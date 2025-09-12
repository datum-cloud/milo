package quota

import (
	"sync"
	"testing"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	quotav1alpha1 "go.miloapis.com/milo/pkg/apis/quota/v1alpha1"
)

// TestPolicyEngineReadyFiltering tests that the policy engine only loads policies with Ready=True
func TestPolicyEngineReadyFiltering(t *testing.T) {
	// Create a mock logger for testing
	logger := logr.Discard()

	// Create a policy engine
	engine := &policyEngine{
		logger:   logger,
		gvkIndex: sync.Map{},
	}

	enabled := true

	tests := []struct {
		name           string
		policy         *quotav1alpha1.ClaimCreationPolicy
		shouldBeLoaded bool
	}{
		{
			name: "ready_and_enabled_policy",
			policy: &quotav1alpha1.ClaimCreationPolicy{
				Spec: quotav1alpha1.ClaimCreationPolicySpec{
					TargetResource: quotav1alpha1.TargetResource{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
					},
					Enabled: &enabled,
				},
				Status: quotav1alpha1.ClaimCreationPolicyStatus{
					Conditions: []metav1.Condition{
						{
							Type:   quotav1alpha1.ClaimCreationPolicyReady,
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			shouldBeLoaded: true,
		},
		{
			name: "not_ready_policy",
			policy: &quotav1alpha1.ClaimCreationPolicy{
				Spec: quotav1alpha1.ClaimCreationPolicySpec{
					TargetResource: quotav1alpha1.TargetResource{
						APIVersion: "apps/v1",
						Kind:       "StatefulSet",
					},
					Enabled: &enabled,
				},
				Status: quotav1alpha1.ClaimCreationPolicyStatus{
					Conditions: []metav1.Condition{
						{
							Type:   quotav1alpha1.ClaimCreationPolicyReady,
							Status: metav1.ConditionFalse,
						},
					},
				},
			},
			shouldBeLoaded: false,
		},
		{
			name: "no_status_condition",
			policy: &quotav1alpha1.ClaimCreationPolicy{
				Spec: quotav1alpha1.ClaimCreationPolicySpec{
					TargetResource: quotav1alpha1.TargetResource{
						APIVersion: "batch/v1",
						Kind:       "Job",
					},
					Enabled: &enabled,
				},
				Status: quotav1alpha1.ClaimCreationPolicyStatus{
					Conditions: []metav1.Condition{},
				},
			},
			shouldBeLoaded: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test isPolicyReady
			isReady := engine.isPolicyReady(tt.policy)
			if isReady != tt.shouldBeLoaded {
				t.Errorf("isPolicyReady() = %v, want %v", isReady, tt.shouldBeLoaded)
			}

			// Test updatePolicy behavior
			err := engine.updatePolicy(tt.policy)
			if err != nil {
				t.Fatalf("updatePolicy() error = %v", err)
			}

			// Check if policy was indexed
			gvk := tt.policy.Spec.TargetResource.GetGVK()
			_, exists := engine.gvkIndex.Load(gvk.String())

			if exists != tt.shouldBeLoaded {
				t.Errorf("Policy indexed = %v, want %v", exists, tt.shouldBeLoaded)
			}
		})
	}
}

// TestPolicyEngineGVKLookup tests GVK-based policy lookups
func TestPolicyEngineGVKLookup(t *testing.T) {
	logger := logr.Discard()
	engine := &policyEngine{
		logger:      logger,
		gvkIndex:    sync.Map{},
		initialized: true, // Mark as initialized to skip lazy loading
	}

	enabled := true
	disabled := false

	// Create test policies
	deploymentPolicy := &quotav1alpha1.ClaimCreationPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "deployment-policy",
		},
		Spec: quotav1alpha1.ClaimCreationPolicySpec{
			TargetResource: quotav1alpha1.TargetResource{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
			},
			Enabled: &enabled,
		},
		Status: quotav1alpha1.ClaimCreationPolicyStatus{
			Conditions: []metav1.Condition{
				{
					Type:   quotav1alpha1.ClaimCreationPolicyReady,
					Status: metav1.ConditionTrue,
				},
			},
		},
	}

	disabledPolicy := &quotav1alpha1.ClaimCreationPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "disabled-policy",
		},
		Spec: quotav1alpha1.ClaimCreationPolicySpec{
			TargetResource: quotav1alpha1.TargetResource{
				APIVersion: "apps/v1",
				Kind:       "StatefulSet",
			},
			Enabled: &disabled,
		},
		Status: quotav1alpha1.ClaimCreationPolicyStatus{
			Conditions: []metav1.Condition{
				{
					Type:   quotav1alpha1.ClaimCreationPolicyReady,
					Status: metav1.ConditionTrue,
				},
			},
		},
	}

	// Add policies
	engine.updatePolicy(deploymentPolicy)
	engine.updatePolicy(disabledPolicy)

	// Test lookups
	tests := []struct {
		name         string
		gvk          schema.GroupVersionKind
		expectPolicy bool
		expectedName string
	}{
		{
			name: "find_enabled_policy",
			gvk: schema.GroupVersionKind{
				Group:   "apps",
				Version: "v1",
				Kind:    "Deployment",
			},
			expectPolicy: true,
			expectedName: "deployment-policy",
		},
		{
			name: "disabled_policy_not_returned",
			gvk: schema.GroupVersionKind{
				Group:   "apps",
				Version: "v1",
				Kind:    "StatefulSet",
			},
			expectPolicy: false,
		},
		{
			name: "no_policy_for_gvk",
			gvk: schema.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "Pod",
			},
			expectPolicy: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy, err := engine.GetPolicyForGVK(tt.gvk)
			if err != nil {
				t.Fatalf("GetPolicyForGVK() error = %v", err)
			}

			if tt.expectPolicy {
				if policy == nil {
					t.Error("Expected policy but got nil")
				} else if policy.Name != tt.expectedName {
					t.Errorf("Got policy %s, want %s", policy.Name, tt.expectedName)
				}
			} else {
				if policy != nil {
					t.Errorf("Expected no policy but got %s", policy.Name)
				}
			}
		})
	}
}
