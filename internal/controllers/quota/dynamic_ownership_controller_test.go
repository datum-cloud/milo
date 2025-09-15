package quota

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	quotav1alpha1 "go.miloapis.com/milo/pkg/apis/quota/v1alpha1"
)

func TestDynamicOwnershipController_ClaimReferencesClaimingResource(t *testing.T) {
	tests := []struct {
		name           string
		claim          *quotav1alpha1.ResourceClaim
		claimingObj    *unstructured.Unstructured
		gvk            schema.GroupVersionKind
		expectedResult bool
	}{
		{
			name: "exact match with namespace",
			claim: &quotav1alpha1.ResourceClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-claim",
					Namespace: "test-ns",
				},
				Spec: quotav1alpha1.ResourceClaimSpec{
					ResourceRef: quotav1alpha1.UnversionedObjectReference{
						APIGroup:  "resourcemanager.miloapis.com",
						Kind:      "Project",
						Name:      "test-project",
						Namespace: "test-ns",
					},
				},
			},
			claimingObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "resourcemanager.miloapis.com/v1alpha1",
					"kind":       "Project",
					"metadata": map[string]interface{}{
						"name":      "test-project",
						"namespace": "test-ns",
						"uid":       "test-uid",
					},
				},
			},
			gvk: schema.GroupVersionKind{
				Group:   "resourcemanager.miloapis.com",
				Version: "v1alpha1",
				Kind:    "Project",
			},
			expectedResult: true,
		},
		{
			name: "no match - different name",
			claim: &quotav1alpha1.ResourceClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-claim",
					Namespace: "test-ns",
				},
				Spec: quotav1alpha1.ResourceClaimSpec{
					ResourceRef: quotav1alpha1.UnversionedObjectReference{
						APIGroup:  "resourcemanager.miloapis.com",
						Kind:      "Project",
						Name:      "different-project",
						Namespace: "test-ns",
					},
				},
			},
			claimingObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "resourcemanager.miloapis.com/v1alpha1",
					"kind":       "Project",
					"metadata": map[string]interface{}{
						"name":      "test-project",
						"namespace": "test-ns",
						"uid":       "test-uid",
					},
				},
			},
			gvk: schema.GroupVersionKind{
				Group:   "resourcemanager.miloapis.com",
				Version: "v1alpha1",
				Kind:    "Project",
			},
			expectedResult: false,
		},
		{
			name: "no match - different kind",
			claim: &quotav1alpha1.ResourceClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-claim",
					Namespace: "test-ns",
				},
				Spec: quotav1alpha1.ResourceClaimSpec{
					ResourceRef: quotav1alpha1.UnversionedObjectReference{
						APIGroup:  "resourcemanager.miloapis.com",
						Kind:      "Organization",
						Name:      "test-project",
						Namespace: "test-ns",
					},
				},
			},
			claimingObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "resourcemanager.miloapis.com/v1alpha1",
					"kind":       "Project",
					"metadata": map[string]interface{}{
						"name":      "test-project",
						"namespace": "test-ns",
						"uid":       "test-uid",
					},
				},
			},
			gvk: schema.GroupVersionKind{
				Group:   "resourcemanager.miloapis.com",
				Version: "v1alpha1",
				Kind:    "Project",
			},
			expectedResult: false,
		},
		{
			name: "match with empty namespace in resourceRef (uses claim namespace)",
			claim: &quotav1alpha1.ResourceClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-claim",
					Namespace: "test-ns",
				},
				Spec: quotav1alpha1.ResourceClaimSpec{
					ResourceRef: quotav1alpha1.UnversionedObjectReference{
						APIGroup:  "resourcemanager.miloapis.com",
						Kind:      "Project",
						Name:      "test-project",
						Namespace: "", // Empty namespace should match claim namespace
					},
				},
			},
			claimingObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "resourcemanager.miloapis.com/v1alpha1",
					"kind":       "Project",
					"metadata": map[string]interface{}{
						"name":      "test-project",
						"namespace": "test-ns",
						"uid":       "test-uid",
					},
				},
			},
			gvk: schema.GroupVersionKind{
				Group:   "resourcemanager.miloapis.com",
				Version: "v1alpha1",
				Kind:    "Project",
			},
			expectedResult: true,
		},
		{
			name: "cluster-scoped resource match",
			claim: &quotav1alpha1.ResourceClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-claim",
					Namespace: "test-ns",
				},
				Spec: quotav1alpha1.ResourceClaimSpec{
					ResourceRef: quotav1alpha1.UnversionedObjectReference{
						APIGroup:  "resourcemanager.miloapis.com",
						Kind:      "Organization",
						Name:      "test-org",
						Namespace: "", // Cluster-scoped
					},
				},
			},
			claimingObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "resourcemanager.miloapis.com/v1alpha1",
					"kind":       "Organization",
					"metadata": map[string]interface{}{
						"name": "test-org",
						"uid":  "test-uid",
					},
				},
			},
			gvk: schema.GroupVersionKind{
				Group:   "resourcemanager.miloapis.com",
				Version: "v1alpha1",
				Kind:    "Organization",
			},
			expectedResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			controller := &DynamicOwnershipController{}
			result := controller.claimReferencesClaimingResource(tt.claim, tt.claimingObj, tt.gvk)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestDynamicOwnershipController_AddOwnerReference(t *testing.T) {
	scheme := runtime.NewScheme()
	err := quotav1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	// Create a test claiming resource
	claimingResource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "resourcemanager.miloapis.com/v1alpha1",
			"kind":       "Project",
			"metadata": map[string]interface{}{
				"name":      "test-project",
				"namespace": "test-ns",
				"uid":       "test-uid-123",
			},
		},
	}

	// Create a test ResourceClaim
	claim := &quotav1alpha1.ResourceClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-claim",
			Namespace: "test-ns",
		},
		Spec: quotav1alpha1.ResourceClaimSpec{
			ResourceRef: quotav1alpha1.UnversionedObjectReference{
				APIGroup:  "resourcemanager.miloapis.com",
				Kind:      "Project",
				Name:      "test-project",
				Namespace: "test-ns",
			},
			ConsumerRef: quotav1alpha1.ConsumerRef{
				APIGroup: "resourcemanager.miloapis.com",
				Kind:     "Organization",
				Name:     "test-org",
			},
			Requests: []quotav1alpha1.ResourceRequest{
				{
					ResourceType: "resourcemanager.miloapis.com/Project",
					Amount:       1,
				},
			},
		},
	}

	// Create fake client
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(claim).
		Build()

	controller := &DynamicOwnershipController{
		Client: fakeClient,
	}

	// Test adding owner reference
	ctx := context.Background()
	err = controller.addOwnerReference(ctx, claim, claimingResource)
	require.NoError(t, err)

	// Verify the owner reference was added
	var updatedClaim quotav1alpha1.ResourceClaim
	err = fakeClient.Get(ctx, client.ObjectKey{Name: "test-claim", Namespace: "test-ns"}, &updatedClaim)
	require.NoError(t, err)

	require.Len(t, updatedClaim.GetOwnerReferences(), 1)
	ownerRef := updatedClaim.GetOwnerReferences()[0]

	assert.Equal(t, "resourcemanager.miloapis.com/v1alpha1", ownerRef.APIVersion)
	assert.Equal(t, "Project", ownerRef.Kind)
	assert.Equal(t, "test-project", ownerRef.Name)
	assert.Equal(t, "test-uid-123", string(ownerRef.UID))
	assert.False(t, *ownerRef.Controller)
	assert.True(t, *ownerRef.BlockOwnerDeletion)
}

func TestDynamicWatchManager_KindToResource(t *testing.T) {
	tests := []struct {
		kind     string
		expected string
	}{
		{"Project", "projects"},
		{"Organization", "organizations"},
		{"Policy", "policies"},
		{"ResourceRegistry", "resourceregistries"}, // ends with 'y'
		{"Status", "statuses"},                     // ends with 's'
		{"Class", "classes"},                       // ends with 's'
	}

	manager := &DynamicWatchManager{}

	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			result := manager.kindToResource(tt.kind)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDynamicWatchManager_DiscoverVersion(t *testing.T) {
	manager := &DynamicWatchManager{
		// No discovery client - should use defaults
		discoveryClient: nil,
	}

	tests := []struct {
		apiGroup string
		kind     string
		expected string
	}{
		{"", "Pod", "v1"}, // Core API group
		{"resourcemanager.miloapis.com", "Project", "v1alpha1"}, // Custom resource
		{"apps", "Deployment", "v1alpha1"},                      // Third-party group
	}

	for _, tt := range tests {
		t.Run(tt.apiGroup+"/"+tt.kind, func(t *testing.T) {
			result, err := manager.discoverVersion(tt.apiGroup, tt.kind)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
