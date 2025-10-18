package admission

import (
	"context"
	"errors"
	"fmt"
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
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"go.miloapis.com/milo/internal/quota/engine"
	"go.miloapis.com/milo/internal/quota/validation"
	quotav1alpha1 "go.miloapis.com/milo/pkg/apis/quota/v1alpha1"
	milorequest "go.miloapis.com/milo/pkg/request"
)

// testResourceTypeValidator provides deterministic resource type validation for tests.
type testResourceTypeValidator struct {
	validResourceTypes map[string]bool
}

func (t *testResourceTypeValidator) ValidateResourceType(ctx context.Context, resourceType string) error {
	if t.validResourceTypes[resourceType] {
		return nil
	}
	return fmt.Errorf("Resource type '%s' is not available for quota management. Enable quota tracking for this resource type by registering it with the quota system", resourceType)
}

func (t *testResourceTypeValidator) IsClaimingResourceAllowed(ctx context.Context, resourceType string, consumerRef quotav1alpha1.ConsumerRef, claimingAPIGroup, claimingKind string) (bool, []string, error) {
	if !t.validResourceTypes[resourceType] {
		return false, nil, fmt.Errorf("no ResourceRegistration found for resource type %s", resourceType)
	}
	return true, []string{fmt.Sprintf("%s/%s", claimingAPIGroup, claimingKind)}, nil
}

func (t *testResourceTypeValidator) IsResourceTypeRegistered(resourceType string) bool {
	return false
}

func (t *testResourceTypeValidator) HasSynced() bool { return true }

func TestResourceQuotaEnforcementPlugin_Validate(t *testing.T) {
	tests := []struct {
		name        string
		policy      *quotav1alpha1.ClaimCreationPolicy
		obj         *unstructured.Unstructured
		gvk         schema.GroupVersionKind
		user        user.Info
		operation   admission.Operation
		expectClaim bool
		expectError bool
		dryRun      bool
	}{
		{
			name: "basic policy creates claim",
			policy: &quotav1alpha1.ClaimCreationPolicy{
				Spec: quotav1alpha1.ClaimCreationPolicySpec{
					Trigger: quotav1alpha1.ClaimTriggerSpec{
						Resource: quotav1alpha1.ClaimTriggerResource{
							APIVersion: "networking.datumapis.com/v1alpha",
							Kind:       "HTTPProxy",
						},
					},
					Disabled: ptr.To(false),
					Target: quotav1alpha1.ClaimTargetSpec{
						ResourceClaimTemplate: quotav1alpha1.ResourceClaimTemplate{
							Metadata: quotav1alpha1.ObjectMetaTemplate{},
							Spec: quotav1alpha1.ResourceClaimSpec{
								ConsumerRef: quotav1alpha1.ConsumerRef{
									APIGroup: "resourcemanager.miloapis.com",
									Kind:     "Organization",
									Name:     "test-org",
								},
								Requests: []quotav1alpha1.ResourceRequest{
									{
										ResourceType: "networking.datumapis.com/HTTPProxy",
										Amount:       1,
									},
								},
								ResourceRef: quotav1alpha1.UnversionedObjectReference{
									APIGroup:  "networking.datumapis.com",
									Kind:      "HTTPProxy",
									Name:      "test-proxy",
									Namespace: "default",
								},
							},
						},
					},
				},
				Status: quotav1alpha1.ClaimCreationPolicyStatus{
					Conditions: []metav1.Condition{{
						Type:   quotav1alpha1.ClaimCreationPolicyReady,
						Status: metav1.ConditionTrue,
						Reason: "TestReady",
					}},
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
					Trigger: quotav1alpha1.ClaimTriggerSpec{
						Resource: quotav1alpha1.ClaimTriggerResource{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
						},
					},
					Disabled: ptr.To(true),
					Target: quotav1alpha1.ClaimTargetSpec{
						ResourceClaimTemplate: quotav1alpha1.ResourceClaimTemplate{
							Metadata: quotav1alpha1.ObjectMetaTemplate{},
							Spec: quotav1alpha1.ResourceClaimSpec{
								ConsumerRef: quotav1alpha1.ConsumerRef{
									APIGroup: "resourcemanager.miloapis.com",
									Kind:     "Organization",
									Name:     "test-org",
								},
								Requests: []quotav1alpha1.ResourceRequest{
									{
										ResourceType: "apps/Deployment",
										Amount:       1,
									},
								},
								ResourceRef: quotav1alpha1.UnversionedObjectReference{
									APIGroup:  "apps",
									Kind:      "Deployment",
									Name:      "test-deploy",
									Namespace: "default",
								},
							},
						},
					},
				},
				Status: quotav1alpha1.ClaimCreationPolicyStatus{
					Conditions: []metav1.Condition{{
						Type:   quotav1alpha1.ClaimCreationPolicyReady,
						Status: metav1.ConditionTrue,
						Reason: "TestReady",
					}},
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
		{
			name: "dry run request skipped",
			policy: &quotav1alpha1.ClaimCreationPolicy{
				Spec: quotav1alpha1.ClaimCreationPolicySpec{
					Trigger: quotav1alpha1.ClaimTriggerSpec{
						Resource: quotav1alpha1.ClaimTriggerResource{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
						},
					},
					Disabled: ptr.To(false),
					Target: quotav1alpha1.ClaimTargetSpec{
						ResourceClaimTemplate: quotav1alpha1.ResourceClaimTemplate{
							Metadata: quotav1alpha1.ObjectMetaTemplate{},
							Spec: quotav1alpha1.ResourceClaimSpec{
								ConsumerRef: quotav1alpha1.ConsumerRef{
									APIGroup: "resourcemanager.miloapis.com",
									Kind:     "Organization",
									Name:     "test-org",
								},
								Requests: []quotav1alpha1.ResourceRequest{
									{
										ResourceType: "apps/Deployment",
										Amount:       1,
									},
								},
								ResourceRef: quotav1alpha1.UnversionedObjectReference{
									APIGroup:  "apps",
									Kind:      "Deployment",
									Name:      "test-deploy",
									Namespace: "default",
								},
							},
						},
					},
				},
				Status: quotav1alpha1.ClaimCreationPolicyStatus{
					Conditions: []metav1.Condition{{
						Type:   quotav1alpha1.ClaimCreationPolicyReady,
						Status: metav1.ConditionTrue,
						Reason: "TestReady",
					}},
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
			expectClaim: false, // Should skip due to dry run
			expectError: false,
			dryRun:      true,
		},
		{
			name: "non-create operation skipped",
			policy: &quotav1alpha1.ClaimCreationPolicy{
				Spec: quotav1alpha1.ClaimCreationPolicySpec{
					Trigger: quotav1alpha1.ClaimTriggerSpec{
						Resource: quotav1alpha1.ClaimTriggerResource{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
						},
					},
					Disabled: ptr.To(false),
					Target: quotav1alpha1.ClaimTargetSpec{
						ResourceClaimTemplate: quotav1alpha1.ResourceClaimTemplate{
							Metadata: quotav1alpha1.ObjectMetaTemplate{},
							Spec: quotav1alpha1.ResourceClaimSpec{
								ConsumerRef: quotav1alpha1.ConsumerRef{
									APIGroup: "resourcemanager.miloapis.com",
									Kind:     "Organization",
									Name:     "test-org",
								},
								Requests: []quotav1alpha1.ResourceRequest{
									{
										ResourceType: "apps/Deployment",
										Amount:       1,
									},
								},
								ResourceRef: quotav1alpha1.UnversionedObjectReference{
									APIGroup:  "apps",
									Kind:      "Deployment",
									Name:      "test-deployment",
									Namespace: "default",
								},
							},
						},
					},
				},
				Status: quotav1alpha1.ClaimCreationPolicyStatus{
					Conditions: []metav1.Condition{{
						Type:   quotav1alpha1.ClaimCreationPolicyReady,
						Status: metav1.ConditionTrue,
						Reason: "TestReady",
					}},
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
			operation:   admission.Update,
			expectClaim: false,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			quotav1alpha1.AddToScheme(scheme)

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

			logger := zap.New(zap.UseDevMode(true))

			celEngine, err := engine.NewCELEngine()
			if err != nil {
				t.Fatalf("Failed to create CEL engine: %v", err)
			}

			policyEngine := &testPolicyEngine{
				policy: tt.policy,
				gvk:    tt.gvk,
			}

			templateEngine := engine.NewTemplateEngine(celEngine, logger.WithName("template"))

			plugin := &ResourceQuotaEnforcementPlugin{
				Handler:        admission.NewHandler(admission.Create, admission.Update),
				dynamicClient:  fakeDynamicClient,
				policyEngine:   policyEngine,
				templateEngine: templateEngine,
				config:         DefaultAdmissionPluginConfig(),
				logger:         logger.WithName("plugin"),
			}
			plugin.watchManagers.Store("", &testWatchManager{behavior: "grant"})

			attrs := &testAdmissionAttributes{
				operation: tt.operation,
				object:    tt.obj,
				gvk:       tt.gvk,
				name:      tt.obj.GetName(),
				namespace: tt.obj.GetNamespace(),
				userInfo:  tt.user,
				dryRun:    tt.dryRun,
			}

			err = plugin.Validate(context.Background(), attrs, nil)

			if err != nil {
				t.Errorf("Unexpected error (plugin should fail-open): %v", err)
			}

			if tt.expectClaim {
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

func (e *testPolicyEngine) Start(ctx context.Context) error { return nil }
func (e *testPolicyEngine) Close()                          {}

func (e *testPolicyEngine) updatePolicyForTest(policy *quotav1alpha1.ClaimCreationPolicy) error {
	gvk := policy.Spec.Trigger.Resource.GetGVK()
	e.policy = policy
	e.gvk = gvk
	return nil
}

func (e *testPolicyEngine) removePolicyForTest(policyName string) {
	e.policy = nil
}

type testAdmissionAttributes struct {
	operation   admission.Operation
	object      runtime.Object
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

// fakeGrantingDynamicClient wraps fake.FakeDynamicClient to automatically grant ResourceClaims on create.
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
	if obj.GetName() == "" && obj.GetGenerateName() != "" {
		obj.SetName(obj.GetGenerateName() + "test-123")
	}

	created, err := f.ResourceInterface.Create(ctx, obj, options, subresources...)
	if err != nil {
		return nil, err
	}

	if f.gvr.Resource == "resourceclaims" && f.gvr.Group == "quota.miloapis.com" {
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
	return &fakeGrantingWatcher{
		gvr:       f.gvr,
		namespace: f.namespace,
		name:      opts.FieldSelector,
	}, nil
}

type fakeGrantingWatcher struct {
	gvr       schema.GroupVersionResource
	namespace string
	name      string
	sent      bool
}

func (f *fakeGrantingWatcher) Stop() {}

func (f *fakeGrantingWatcher) ResultChan() <-chan watch.Event {
	ch := make(chan watch.Event, 1)

	go func() {
		defer close(ch)

		if !f.sent {
			time.Sleep(100 * time.Millisecond)

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

func TestResourceQuotaEnforcementPlugin_ResourceClaimValidation(t *testing.T) {
	tests := []struct {
		name        string
		claim       *quotav1alpha1.ResourceClaim
		expectError bool
		errorSubstr string
	}{
		{
			name: "resource claim without ResourceRegistration",
			claim: &quotav1alpha1.ResourceClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-claim",
					Namespace: "default",
				},
				Spec: quotav1alpha1.ResourceClaimSpec{
					ConsumerRef: quotav1alpha1.ConsumerRef{
						APIGroup: "resourcemanager.miloapis.com",
						Kind:     "Organization",
						Name:     "test-org",
					},
					Requests: []quotav1alpha1.ResourceRequest{
						{
							ResourceType: "apps/Deployment",
							Amount:       5,
						},
					},
					ResourceRef: quotav1alpha1.UnversionedObjectReference{
						APIGroup:  "apps",
						Kind:      "Deployment",
						Name:      "test-deployment",
						Namespace: "default",
					},
				},
			},
			expectError: true,
			errorSubstr: "Resource type 'apps/Deployment' is not available for quota management",
		},
		{
			name: "empty resource type",
			claim: &quotav1alpha1.ResourceClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-claim",
					Namespace: "default",
				},
				Spec: quotav1alpha1.ResourceClaimSpec{
					ConsumerRef: quotav1alpha1.ConsumerRef{
						APIGroup: "resourcemanager.miloapis.com",
						Kind:     "Organization",
						Name:     "test-org",
					},
					Requests: []quotav1alpha1.ResourceRequest{
						{
							ResourceType: "",
							Amount:       5,
						},
					},
					ResourceRef: quotav1alpha1.UnversionedObjectReference{
						APIGroup:  "apps",
						Kind:      "Deployment",
						Name:      "test-deployment",
						Namespace: "default",
					},
				},
			},
			expectError: true,
			errorSubstr: "resource type is required",
		},
		{
			name: "negative amount",
			claim: &quotav1alpha1.ResourceClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-claim",
					Namespace: "default",
				},
				Spec: quotav1alpha1.ResourceClaimSpec{
					ConsumerRef: quotav1alpha1.ConsumerRef{
						APIGroup: "resourcemanager.miloapis.com",
						Kind:     "Organization",
						Name:     "test-org",
					},
					Requests: []quotav1alpha1.ResourceRequest{
						{
							ResourceType: "apps/Deployment",
							Amount:       -1,
						},
					},
					ResourceRef: quotav1alpha1.UnversionedObjectReference{
						APIGroup:  "apps",
						Kind:      "Deployment",
						Name:      "test-deployment",
						Namespace: "default",
					},
				},
			},
			expectError: true,
			errorSubstr: "amount must be greater than 0",
		},
		{
			name: "duplicate resource types",
			claim: &quotav1alpha1.ResourceClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-claim",
					Namespace: "default",
				},
				Spec: quotav1alpha1.ResourceClaimSpec{
					ConsumerRef: quotav1alpha1.ConsumerRef{
						APIGroup: "resourcemanager.miloapis.com",
						Kind:     "Organization",
						Name:     "test-org",
					},
					Requests: []quotav1alpha1.ResourceRequest{
						{
							ResourceType: "apps/Deployment",
							Amount:       3,
						},
						{
							ResourceType: "apps/Deployment",
							Amount:       2,
						},
					},
					ResourceRef: quotav1alpha1.UnversionedObjectReference{
						APIGroup:  "apps",
						Kind:      "Deployment",
						Name:      "test-deployment",
						Namespace: "default",
					},
				},
			},
			expectError: true,
			errorSubstr: "is already specified",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			quotav1alpha1.AddToScheme(scheme)
			fakeDynamicClient := fake.NewSimpleDynamicClient(scheme)

			logger := zap.New(zap.UseDevMode(true))

			mockValidator := &testResourceTypeValidator{
				validResourceTypes: make(map[string]bool),
			}
			resourceClaimValidator := validation.NewResourceClaimValidator(fakeDynamicClient, mockValidator)

			plugin := &ResourceQuotaEnforcementPlugin{
				Handler:                admission.NewHandler(admission.Create),
				dynamicClient:          fakeDynamicClient,
				resourceTypeValidator:  mockValidator,
				resourceClaimValidator: resourceClaimValidator,
				config:                 DefaultAdmissionPluginConfig(),
				logger:                 logger.WithName("plugin"),
			}
			plugin.watchManagers.Store("", &testWatchManager{behavior: "grant"})

			// Convert typed ResourceClaim to unstructured (as CRDs are always unstructured in admission)
			unstructuredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(tt.claim)
			if err != nil {
				t.Fatalf("Failed to convert claim to unstructured: %v", err)
			}
			unstructuredClaim := &unstructured.Unstructured{Object: unstructuredMap}

			attrs := &testAdmissionAttributes{
				operation: admission.Create,
				object:    unstructuredClaim,
				gvk: schema.GroupVersionKind{
					Group:   "quota.miloapis.com",
					Version: "v1alpha1",
					Kind:    "ResourceClaim",
				},
				name:      tt.claim.Name,
				namespace: tt.claim.Namespace,
				userInfo: &user.DefaultInfo{
					Name: "test-user",
				},
				dryRun: false,
			}

			err = plugin.Validate(context.Background(), attrs, nil)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if tt.errorSubstr != "" && !contains(err.Error(), tt.errorSubstr) {
					t.Errorf("Expected error to contain '%s' but got: %v", tt.errorSubstr, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestResourceQuotaEnforcementPlugin_InitializationValidation(t *testing.T) {
	tests := []struct {
		name           string
		setupPlugin    func() *ResourceQuotaEnforcementPlugin
		expectError    bool
		expectedErrMsg string
	}{
		{
			name: "missing dynamic client",
			setupPlugin: func() *ResourceQuotaEnforcementPlugin {
				return &ResourceQuotaEnforcementPlugin{
					Handler: admission.NewHandler(admission.Create),
					config:  DefaultAdmissionPluginConfig(),
					logger:  zap.New(zap.UseDevMode(true)).WithName("plugin"),
				}
			},
			expectError:    true,
			expectedErrMsg: "dynamic client not initialized",
		},
		{
			name: "valid initialization",
			setupPlugin: func() *ResourceQuotaEnforcementPlugin {
				scheme := runtime.NewScheme()
				quotav1alpha1.AddToScheme(scheme)
				fakeDynamicClient := fake.NewSimpleDynamicClient(scheme)
				logger := zap.New(zap.UseDevMode(true))

				plugin := &ResourceQuotaEnforcementPlugin{
					Handler:       admission.NewHandler(admission.Create),
					dynamicClient: fakeDynamicClient,
					config:        DefaultAdmissionPluginConfig(),
					logger:        logger.WithName("plugin"),
				}

				celEngine, _ := engine.NewCELEngine()
				plugin.templateEngine = engine.NewTemplateEngine(celEngine, logger.WithName("template"))
				plugin.policyEngine = &testPolicyEngine{}

				mockValidator := &testResourceTypeValidator{
					validResourceTypes: make(map[string]bool),
				}
				plugin.resourceTypeValidator = mockValidator
				plugin.resourceClaimValidator = validation.NewResourceClaimValidator(fakeDynamicClient, mockValidator)
				plugin.resourceRegistrationValidator = validation.NewResourceRegistrationValidator(mockValidator)
				plugin.claimCreationPolicyValidator = validation.NewClaimCreationPolicyValidator(mockValidator)
				celValidator, _ := validation.NewCELValidator()
				grantTemplateValidator, _ := validation.NewGrantTemplateValidator(mockValidator)
				plugin.grantCreationPolicyValidator = validation.NewGrantCreationPolicyValidator(celValidator, grantTemplateValidator)
				plugin.resourceGrantValidator = validation.NewResourceGrantValidator(mockValidator)

				plugin.watchManagers.Store("", &testWatchManager{behavior: "grant"})

				return plugin
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plugin := tt.setupPlugin()
			err := plugin.ValidateInitialization()

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if tt.expectedErrMsg != "" && !contains(err.Error(), tt.expectedErrMsg) {
					t.Errorf("Expected error to contain '%s' but got: %v", tt.expectedErrMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestResourceQuotaEnforcementPlugin_PolicyEngineFailure(t *testing.T) {
	failingPolicyEngine := &failingPolicyEngine{
		err: errors.New("policy engine failure"),
	}

	scheme := runtime.NewScheme()
	quotav1alpha1.AddToScheme(scheme)
	fakeDynamicClient := fake.NewSimpleDynamicClient(scheme)

	logger := zap.New(zap.UseDevMode(true))

	celEngine, err := engine.NewCELEngine()
	if err != nil {
		t.Fatalf("Failed to create CEL engine: %v", err)
	}

	plugin := &ResourceQuotaEnforcementPlugin{
		Handler:        admission.NewHandler(admission.Create),
		dynamicClient:  fakeDynamicClient,
		policyEngine:   failingPolicyEngine,
		templateEngine: engine.NewTemplateEngine(celEngine, logger.WithName("template")),
		config:         DefaultAdmissionPluginConfig(),
		logger:         logger.WithName("plugin"),
	}
	plugin.watchManagers.Store("", &testWatchManager{behavior: "grant"})

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":      "test-deployment",
				"namespace": "default",
			},
		},
	}

	attrs := &testAdmissionAttributes{
		operation: admission.Create,
		object:    obj,
		gvk: schema.GroupVersionKind{
			Group:   "apps",
			Version: "v1",
			Kind:    "Deployment",
		},
		name:      obj.GetName(),
		namespace: obj.GetNamespace(),
		userInfo: &user.DefaultInfo{
			Name: "test-user",
		},
		dryRun: false,
	}

	err = plugin.Validate(context.Background(), attrs, nil)
	if err == nil {
		t.Error("Expected error when policy engine fails, but got none")
	}
	if !contains(err.Error(), "policy engine failure") {
		t.Errorf("Expected error to contain 'policy engine failure' but got: %v", err)
	}
}

func TestClaimWaitScenarios(t *testing.T) {
	tests := []struct {
		name          string
		claimBehavior string
		expectError   bool
		errorSubstr   string
	}{
		{
			name:          "claim granted",
			claimBehavior: "granted",
			expectError:   false,
		},
		{
			name:          "claim denied",
			claimBehavior: "denied",
			expectError:   true,
			errorSubstr:   "Insufficient quota resources available",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			quotav1alpha1.AddToScheme(scheme)

			var fakeDynamicClient dynamic.Interface
			switch tt.claimBehavior {
			case "granted":
				fakeDynamicClient = &fakeGrantingDynamicClient{
					FakeDynamicClient: fake.NewSimpleDynamicClient(scheme),
				}
			case "denied":
				fakeDynamicClient = &fakeDenyingDynamicClient{
					FakeDynamicClient: fake.NewSimpleDynamicClient(scheme),
				}
			default:
				fakeDynamicClient = fake.NewSimpleDynamicClient(scheme)
			}

			logger := zap.New(zap.UseDevMode(true))

			celEngine, err := engine.NewCELEngine()
			if err != nil {
				t.Fatalf("Failed to create CEL engine: %v", err)
			}

			policy := &quotav1alpha1.ClaimCreationPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-policy",
				},
				Spec: quotav1alpha1.ClaimCreationPolicySpec{
					Trigger: quotav1alpha1.ClaimTriggerSpec{
						Resource: quotav1alpha1.ClaimTriggerResource{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
						},
					},
					Disabled: ptr.To(false),
					Target: quotav1alpha1.ClaimTargetSpec{
						ResourceClaimTemplate: quotav1alpha1.ResourceClaimTemplate{
							Metadata: quotav1alpha1.ObjectMetaTemplate{},
							Spec: quotav1alpha1.ResourceClaimSpec{
								ConsumerRef: quotav1alpha1.ConsumerRef{
									APIGroup: "resourcemanager.miloapis.com",
									Kind:     "Organization",
									Name:     "test-org",
								},
								Requests: []quotav1alpha1.ResourceRequest{
									{
										ResourceType: "apps/Deployment",
										Amount:       1,
									},
								},
								ResourceRef: quotav1alpha1.UnversionedObjectReference{
									APIGroup:  "apps",
									Kind:      "Deployment",
									Name:      "test-deployment",
									Namespace: "default",
								},
							},
						},
					},
				},
				Status: quotav1alpha1.ClaimCreationPolicyStatus{
					Conditions: []metav1.Condition{{
						Type:   quotav1alpha1.ClaimCreationPolicyReady,
						Status: metav1.ConditionTrue,
						Reason: "TestReady",
					}},
				},
			}

			plugin := &ResourceQuotaEnforcementPlugin{
				Handler:       admission.NewHandler(admission.Create),
				dynamicClient: fakeDynamicClient,
				policyEngine: &testPolicyEngine{
					policy: policy,
					gvk: schema.GroupVersionKind{
						Group:   "apps",
						Version: "v1",
						Kind:    "Deployment",
					},
				},
				templateEngine: engine.NewTemplateEngine(celEngine, logger.WithName("template")),
				config:         DefaultAdmissionPluginConfig(),
				logger:         logger.WithName("plugin"),
			}

			var watchManager ClaimWatchManager
			if tt.claimBehavior == "granted" {
				watchManager = &testWatchManager{behavior: "grant"}
			} else if tt.claimBehavior == "denied" {
				watchManager = &testWatchManager{behavior: "deny"}
			} else {
				watchManager = &testWatchManager{behavior: "timeout"}
			}
			plugin.watchManagers.Store("", watchManager)

			obj := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":      "test-deployment",
						"namespace": "default",
					},
				},
			}

			attrs := &testAdmissionAttributes{
				operation: admission.Create,
				object:    obj,
				gvk: schema.GroupVersionKind{
					Group:   "apps",
					Version: "v1",
					Kind:    "Deployment",
				},
				name:      obj.GetName(),
				namespace: obj.GetNamespace(),
				userInfo: &user.DefaultInfo{
					Name: "test-user",
				},
				dryRun: false,
			}

			err = plugin.Validate(context.Background(), attrs, nil)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if tt.errorSubstr != "" && !contains(err.Error(), tt.errorSubstr) {
					t.Errorf("Expected error to contain '%s' but got: %v", tt.errorSubstr, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

type failingPolicyEngine struct {
	err error
}

func (e *failingPolicyEngine) GetPolicyForGVK(gvk schema.GroupVersionKind) (*quotav1alpha1.ClaimCreationPolicy, error) {
	return nil, e.err
}

func (e *failingPolicyEngine) Start(ctx context.Context) error { return nil }
func (e *failingPolicyEngine) Close()                          {}

// fakeDenyingDynamicClient wraps fake.FakeDynamicClient to automatically deny ResourceClaims on create.
type fakeDenyingDynamicClient struct {
	*fake.FakeDynamicClient
}

func (f *fakeDenyingDynamicClient) Resource(resource schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
	return &fakeDenyingNamespaceableResource{
		NamespaceableResourceInterface: f.FakeDynamicClient.Resource(resource),
		gvr:                            resource,
	}
}

type fakeDenyingNamespaceableResource struct {
	dynamic.NamespaceableResourceInterface
	gvr schema.GroupVersionResource
}

func (f *fakeDenyingNamespaceableResource) Namespace(namespace string) dynamic.ResourceInterface {
	return &fakeDenyingResource{
		ResourceInterface: f.NamespaceableResourceInterface.Namespace(namespace),
		gvr:               f.gvr,
		namespace:         namespace,
	}
}

type fakeDenyingResource struct {
	dynamic.ResourceInterface
	gvr       schema.GroupVersionResource
	namespace string
}

func (f *fakeDenyingResource) Create(ctx context.Context, obj *unstructured.Unstructured, options metav1.CreateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	if obj.GetName() == "" && obj.GetGenerateName() != "" {
		obj.SetName(obj.GetGenerateName() + "test-456")
	}

	created, err := f.ResourceInterface.Create(ctx, obj, options, subresources...)
	if err != nil {
		return nil, err
	}

	if f.gvr.Resource == "resourceclaims" && f.gvr.Group == "quota.miloapis.com" {
		conditions := []interface{}{
			map[string]interface{}{
				"type":    quotav1alpha1.ResourceClaimGranted,
				"status":  string(metav1.ConditionFalse),
				"reason":  quotav1alpha1.ResourceClaimDeniedReason,
				"message": "Quota exceeded for testing",
			},
		}

		unstructured.SetNestedSlice(created.Object, conditions, "status", "conditions")
	}

	return created, nil
}

func (f *fakeDenyingResource) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	return &fakeDenyingWatcher{
		gvr:       f.gvr,
		namespace: f.namespace,
		name:      opts.FieldSelector,
	}, nil
}

type fakeDenyingWatcher struct {
	gvr       schema.GroupVersionResource
	namespace string
	name      string
	sent      bool
}

func (f *fakeDenyingWatcher) Stop() {}

func (f *fakeDenyingWatcher) ResultChan() <-chan watch.Event {
	ch := make(chan watch.Event, 1)

	go func() {
		defer close(ch)

		if !f.sent {
			time.Sleep(100 * time.Millisecond)

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
								"status":  string(metav1.ConditionFalse),
								"reason":  quotav1alpha1.ResourceClaimDeniedReason,
								"message": "Quota exceeded for testing",
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

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (substr == "" ||
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())
}

type testWatchManager struct {
	behavior string
}

func (m *testWatchManager) RegisterClaimWaiter(ctx context.Context, claimName, namespace string, timeout time.Duration) (<-chan ClaimResult, context.CancelFunc, error) {
	resultChan := make(chan ClaimResult, 1)
	cancel := func() {}

	go func() {
		time.Sleep(10 * time.Millisecond)
		switch m.behavior {
		case "grant":
			resultChan <- ClaimResult{Granted: true, Reason: "test granted"}
		case "deny":
			resultChan <- ClaimResult{Granted: false, Reason: "quota exceeded", Error: fmt.Errorf("ResourceClaim was denied: quota exceeded")}
		}
		close(resultChan)
	}()

	return resultChan, cancel, nil
}

func (m *testWatchManager) UnregisterClaimWaiter(claimName, namespace string) {}
func (m *testWatchManager) Start(ctx context.Context) error                   { return nil }
func (m *testWatchManager) Stop()                                             {}

func TestProjectContextExtraction(t *testing.T) {
	tests := []struct {
		name           string
		projectID      string
		wantProjectID  string
		wantHasProject bool
	}{
		{
			name:           "with project context",
			projectID:      "test-project",
			wantProjectID:  "test-project",
			wantHasProject: true,
		},
		{
			name:           "without project context (root)",
			projectID:      "",
			wantProjectID:  "",
			wantHasProject: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.projectID != "" {
				ctx = milorequest.WithProject(ctx, tt.projectID)
			}

			gotProjectID, gotHasProject := milorequest.ProjectID(ctx)

			if gotProjectID != tt.wantProjectID {
				t.Errorf("ProjectID() = %v, want %v", gotProjectID, tt.wantProjectID)
			}
			if gotHasProject != tt.wantHasProject {
				t.Errorf("ProjectID() hasProject = %v, want %v", gotHasProject, tt.wantHasProject)
			}
		})
	}
}

func TestGetClientForContext(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := quotav1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	fakeDynamicClient := fake.NewSimpleDynamicClient(scheme)

	plugin := &ResourceQuotaEnforcementPlugin{
		Handler:       admission.NewHandler(admission.Create),
		dynamicClient: fakeDynamicClient,
		logger:        zap.New(zap.UseDevMode(true)),
	}

	tests := []struct {
		name             string
		projectID        string
		wantRootClient   bool
		wantError        bool
		setupLoopbackCfg bool
	}{
		{
			name:             "root context returns root client",
			projectID:        "",
			wantRootClient:   true,
			wantError:        false,
			setupLoopbackCfg: false,
		},
		{
			name:             "project context without loopback config returns error",
			projectID:        "test-project",
			wantRootClient:   false,
			wantError:        true,
			setupLoopbackCfg: false,
		},
		{
			name:             "project context with loopback config creates project client",
			projectID:        "test-project",
			wantRootClient:   false,
			wantError:        false,
			setupLoopbackCfg: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupLoopbackCfg {
				cfg := &rest.Config{
					Host: "http://localhost:8080",
				}
				plugin.SetLoopbackConfig(cfg)
			}

			ctx := context.Background()
			if tt.projectID != "" {
				ctx = milorequest.WithProject(ctx, tt.projectID)
			}

			client, err := plugin.getClient(ctx)

			if tt.wantError {
				if err == nil {
					t.Error("getClient() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("getClient() unexpected error = %v", err)
				return
			}

			if client == nil {
				t.Error("getClient() returned nil client")
				return
			}

			if tt.wantRootClient {
				if client != fakeDynamicClient {
					t.Error("getClient() root context did not return root client")
				}
			} else if client == fakeDynamicClient {
				t.Error("getClient() project context returned root client")
			}
		})
	}
}

func TestProjectClientCaching(t *testing.T) {
	plugin := &ResourceQuotaEnforcementPlugin{
		Handler: admission.NewHandler(admission.Create),
		logger:  zap.New(zap.UseDevMode(true)),
	}

	cfg := &rest.Config{
		Host: "http://localhost:8080",
	}
	plugin.SetLoopbackConfig(cfg)

	projectID := "test-project"

	client1, err := plugin.getProjectClient(projectID)
	if err != nil {
		t.Fatalf("getProjectClient() error = %v", err)
	}
	if client1 == nil {
		t.Fatal("getProjectClient() returned nil client")
	}

	client2, err := plugin.getProjectClient(projectID)
	if err != nil {
		t.Fatalf("getProjectClient() error = %v", err)
	}
	if client2 == nil {
		t.Fatal("getProjectClient() returned nil client")
	}

	if client1 != client2 {
		t.Error("getProjectClient() did not return cached client on second call")
	}

	client3, err := plugin.getProjectClient("different-project")
	if err != nil {
		t.Fatalf("getProjectClient() error = %v", err)
	}
	if client3 == nil {
		t.Fatal("getProjectClient() returned nil client")
	}

	if client1 == client3 {
		t.Error("getProjectClient() returned same client for different project")
	}
}
