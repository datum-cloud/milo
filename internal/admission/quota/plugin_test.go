package quota

import (
	"context"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/apis/audit"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	quotav1alpha1 "go.miloapis.com/milo/pkg/apis/quota/v1alpha1"
)

func TestClaimCreationPlugin_Validate(t *testing.T) {
	tests := []struct {
		name        string
		policy      *quotav1alpha1.ClaimCreationPolicy
		obj         *unstructured.Unstructured
		gvk         schema.GroupVersionKind
		user        user.Info
		operation   admission.Operation
		expectClaim bool
		expectError bool
	}{
		{
			name: "basic policy creates claim",
			policy: &quotav1alpha1.ClaimCreationPolicy{
				Spec: quotav1alpha1.ClaimCreationPolicySpec{
					TargetResource: quotav1alpha1.TargetResource{
						APIVersion: "networking.datumapis.com/v1alpha",
						Kind:       "HTTPProxy",
					},
					Enabled: boolPtr(true),
					ResourceClaimTemplate: quotav1alpha1.ResourceClaimTemplateSpec{
						Requests: []quotav1alpha1.ResourceRequestTemplate{
							{
								ResourceType: "networking.datumapis.com/HTTPProxy",
								Amount:       int64Ptr(1),
								Dimensions: map[string]string{
									"service-tier": "basic",
								},
							},
						},
					},
				},
				Status: quotav1alpha1.ClaimCreationPolicyStatus{
					Conditions: []metav1.Condition{
						{
							Type:   quotav1alpha1.ClaimCreationPolicyReady,
							Status: metav1.ConditionTrue,
							Reason: "TestReady",
						},
					},
				},
			},
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "networking.datumapis.com/v1alpha",
					"kind":       "HTTPProxy",
					"metadata": map[string]interface{}{
						"name":      "test-proxy",
						"namespace": "default",
					},
					"spec": map[string]interface{}{
						"virtualhost": map[string]interface{}{
							"fqdn": "example.com",
						},
					},
				},
			},
			gvk: schema.GroupVersionKind{
				Group:   "networking.datumapis.com",
				Version: "v1alpha",
				Kind:    "HTTPProxy",
			},
			user: &user.DefaultInfo{
				Name:   "test-user",
				UID:    "test-uid",
				Groups: []string{"basic-users"},
			},
			operation:   admission.Create,
			expectClaim: true,
			expectError: false,
		},
		{
			name:   "no policy for GVK",
			policy: nil,
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
					"metadata": map[string]interface{}{
						"name":      "test-cm",
						"namespace": "default",
					},
				},
			},
			gvk: schema.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "ConfigMap",
			},
			user: &user.DefaultInfo{
				Name: "test-user",
			},
			operation:   admission.Create,
			expectClaim: false,
			expectError: false,
		},
		{
			name: "disabled policy",
			policy: &quotav1alpha1.ClaimCreationPolicy{
				Spec: quotav1alpha1.ClaimCreationPolicySpec{
					TargetResource: quotav1alpha1.TargetResource{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
					},
					Enabled: boolPtr(false), // Disabled
					ResourceClaimTemplate: quotav1alpha1.ResourceClaimTemplateSpec{
						Requests: []quotav1alpha1.ResourceRequestTemplate{
							{
								ResourceType: "apps/Deployment",
								Amount:       int64Ptr(1),
							},
						},
					},
				},
				Status: quotav1alpha1.ClaimCreationPolicyStatus{
					Conditions: []metav1.Condition{
						{
							Type:   quotav1alpha1.ClaimCreationPolicyReady,
							Status: metav1.ConditionTrue,
							Reason: "TestReady",
						},
					},
				},
			},
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":      "test-deploy",
						"namespace": "default",
					},
				},
			},
			gvk: schema.GroupVersionKind{
				Group:   "apps",
				Version: "v1",
				Kind:    "Deployment",
			},
			user: &user.DefaultInfo{
				Name: "test-user",
			},
			operation:   admission.Create,
			expectClaim: false,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake dynamic client
			scheme := runtime.NewScheme()
			quotav1alpha1.AddToScheme(scheme)

			// Convert policy to unstructured for dynamic client
			var objects []runtime.Object
			if tt.policy != nil {
				unstructuredPolicy, err := runtime.DefaultUnstructuredConverter.ToUnstructured(tt.policy)
				if err != nil {
					t.Fatalf("Failed to convert policy to unstructured: %v", err)
				}
				policyObj := &unstructured.Unstructured{Object: unstructuredPolicy}
				policyObj.SetGroupVersionKind(schema.GroupVersionKind{
					Group:   "quota.miloapis.com",
					Version: "v1alpha1",
					Kind:    "ClaimCreationPolicy",
				})
				objects = append(objects, policyObj)
			}

			fakeDynamicClient := &fakeGrantingDynamicClient{
				FakeDynamicClient: fake.NewSimpleDynamicClient(scheme, objects...),
			}

			// Create logger
			logger := zap.New(zap.UseDevMode(true))

			// Create CEL engine
			celEngine, err := NewCELEngine(logger.WithName("cel"))
			if err != nil {
				t.Fatalf("Failed to create CEL engine: %v", err)
			}

			// Create policy engine
			policyEngine := &testPolicyEngine{
				policy: tt.policy,
				gvk:    tt.gvk,
			}

			// Create template engine
			templateEngine := NewTemplateEngine(celEngine, logger.WithName("template"))

			// Create plugin
			plugin := &ClaimCreationPlugin{
				Handler:        admission.NewHandler(admission.Create, admission.Update),
				dynamicClient:  fakeDynamicClient,
				policyEngine:   policyEngine,
				templateEngine: templateEngine,
				config:         DefaultAdmissionPluginConfig(),
				logger:         logger.WithName("plugin"),
			}

			// Create admission attributes
			attrs := &testAdmissionAttributes{
				operation: tt.operation,
				object:    tt.obj,
				gvk:       tt.gvk,
				name:      tt.obj.GetName(),
				namespace: tt.obj.GetNamespace(),
				userInfo:  tt.user,
				dryRun:    false,
			}

			// Call Validate (not Admit)
			err = plugin.Validate(context.Background(), attrs, nil)

			// Check results - the plugin should never return errors (fail-open strategy)
			if err != nil {
				t.Errorf("Unexpected error (plugin should fail-open): %v", err)
			}

			// For enabled policies, check that ResourceClaim creation was attempted
			if tt.expectClaim {
				// Check if a CREATE action was performed on ResourceClaim
				actions := fakeDynamicClient.Actions()
				found := false
				for _, action := range actions {
					if action.Matches("create", "resourceclaims") {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected ResourceClaim creation but no create action found")
				}
			}
		})
	}
}

// Test helper types

type testPolicyEngine struct {
	policy *quotav1alpha1.ClaimCreationPolicy
	gvk    schema.GroupVersionKind
}

func (e *testPolicyEngine) GetPolicyForGVK(gvk schema.GroupVersionKind) (*quotav1alpha1.ClaimCreationPolicy, error) {
	if e.policy != nil && e.gvk == gvk {
		return e.policy, nil
	}
	return nil, nil
}

func (e *testPolicyEngine) Close() {
	// No-op for test implementation
}

func (e *testPolicyEngine) updatePolicyForTest(policy *quotav1alpha1.ClaimCreationPolicy) error {
	// For testing, store the policy by GVK
	gvk := policy.Spec.TargetResource.GetGVK()
	e.policy = policy
	e.gvk = gvk
	return nil
}

func (e *testPolicyEngine) removePolicyForTest(policyName string) {
	// For testing, just clear the stored policy
	e.policy = nil
}

type testAdmissionAttributes struct {
	operation   admission.Operation
	object      *unstructured.Unstructured
	gvk         schema.GroupVersionKind
	name        string
	namespace   string
	userInfo    user.Info
	dryRun      bool
	subResource string
}

func (a *testAdmissionAttributes) GetOperation() admission.Operation { return a.operation }
func (a *testAdmissionAttributes) GetObject() runtime.Object         { return a.object }
func (a *testAdmissionAttributes) GetOldObject() runtime.Object      { return nil }
func (a *testAdmissionAttributes) GetKind() schema.GroupVersionKind  { return a.gvk }
func (a *testAdmissionAttributes) GetName() string                   { return a.name }
func (a *testAdmissionAttributes) GetNamespace() string              { return a.namespace }
func (a *testAdmissionAttributes) GetResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{}
}
func (a *testAdmissionAttributes) GetSubresource() string                { return a.subResource }
func (a *testAdmissionAttributes) GetUserInfo() user.Info                { return a.userInfo }
func (a *testAdmissionAttributes) IsDryRun() bool                        { return a.dryRun }
func (a *testAdmissionAttributes) GetOperationOptions() runtime.Object   { return nil }
func (a *testAdmissionAttributes) AddAnnotation(key, value string) error { return nil }
func (a *testAdmissionAttributes) AddAnnotationWithLevel(key, value string, level audit.Level) error {
	return nil
}
func (a *testAdmissionAttributes) GetReinvocationContext() admission.ReinvocationContext {
	return nil
}

// Helper functions

func boolPtr(b bool) *bool {
	return &b
}

func int64Ptr(i int64) *int64 {
	return &i
}

// fakeGrantingDynamicClient wraps the fake dynamic client and automatically grants ResourceClaims
type fakeGrantingDynamicClient struct {
	*fake.FakeDynamicClient
}

func (f *fakeGrantingDynamicClient) Resource(resource schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
	return &fakeGrantingNamespaceableResource{
		NamespaceableResourceInterface: f.FakeDynamicClient.Resource(resource),
		gvr:                            resource,
	}
}

type fakeGrantingNamespaceableResource struct {
	dynamic.NamespaceableResourceInterface
	gvr schema.GroupVersionResource
}

func (f *fakeGrantingNamespaceableResource) Namespace(namespace string) dynamic.ResourceInterface {
	return &fakeGrantingResource{
		ResourceInterface: f.NamespaceableResourceInterface.Namespace(namespace),
		gvr:               f.gvr,
		namespace:         namespace,
	}
}

type fakeGrantingResource struct {
	dynamic.ResourceInterface
	gvr       schema.GroupVersionResource
	namespace string
}

func (f *fakeGrantingResource) Create(ctx context.Context, obj *unstructured.Unstructured, options metav1.CreateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	created, err := f.ResourceInterface.Create(ctx, obj, options, subresources...)
	if err != nil {
		return nil, err
	}

	// If this is a ResourceClaim, automatically set it to granted
	if f.gvr.Resource == "resourceclaims" && f.gvr.Group == "quota.miloapis.com" {
		// Add granted condition
		conditions := []interface{}{
			map[string]interface{}{
				"type":    quotav1alpha1.ResourceClaimGranted,
				"status":  string(metav1.ConditionTrue),
				"reason":  "TestGranted",
				"message": "Automatically granted for testing",
			},
		}

		unstructured.SetNestedSlice(created.Object, conditions, "status", "conditions")
	}

	return created, nil
}

func (f *fakeGrantingResource) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	// Return a fake watcher that immediately signals the resource is granted
	return &fakeGrantingWatcher{
		gvr:       f.gvr,
		namespace: f.namespace,
		name:      opts.FieldSelector, // Should contain metadata.name=claim-name
	}, nil
}

type fakeGrantingWatcher struct {
	gvr       schema.GroupVersionResource
	namespace string
	name      string
	sent      bool
}

func (f *fakeGrantingWatcher) Stop() {
	// No-op
}

func (f *fakeGrantingWatcher) ResultChan() <-chan watch.Event {
	ch := make(chan watch.Event, 1)

	go func() {
		defer close(ch)

		if !f.sent {
			// Send a granted event after a small delay
			time.Sleep(100 * time.Millisecond)

			// Create a fake ResourceClaim with granted status
			claim := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": f.gvr.GroupVersion().String(),
					"kind":       "ResourceClaim",
					"metadata": map[string]interface{}{
						"name":      "test-claim",
						"namespace": f.namespace,
					},
					"status": map[string]interface{}{
						"conditions": []interface{}{
							map[string]interface{}{
								"type":    quotav1alpha1.ResourceClaimGranted,
								"status":  string(metav1.ConditionTrue),
								"reason":  "TestGranted",
								"message": "Automatically granted for testing",
							},
						},
					},
				},
			}

			ch <- watch.Event{
				Type:   watch.Modified,
				Object: claim,
			}
			f.sent = true
		}
	}()

	return ch
}
