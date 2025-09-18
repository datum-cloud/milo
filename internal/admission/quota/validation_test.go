package quota

import (
	"sync"
	"testing"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"

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

	tests := []struct {
		name           string
		policy         *quotav1alpha1.ClaimCreationPolicy
		shouldBeLoaded bool
	}{
		{
			name: "ready_and_enabled_policy",
			policy: &quotav1alpha1.ClaimCreationPolicy{
				Spec: quotav1alpha1.ClaimCreationPolicySpec{
					Trigger: quotav1alpha1.ClaimTriggerSpec{
						Resource: quotav1alpha1.TargetResource{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
						},
					},
					Enabled: ptr.To(true),
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
					Trigger: quotav1alpha1.ClaimTriggerSpec{
						Resource: quotav1alpha1.TargetResource{
							APIVersion: "apps/v1",
							Kind:       "StatefulSet",
						},
					},
					Enabled: ptr.To(true),
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
					Trigger: quotav1alpha1.ClaimTriggerSpec{
						Resource: quotav1alpha1.TargetResource{
							APIVersion: "batch/v1",
							Kind:       "Job",
						},
					},
					Enabled: ptr.To(true),
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
			gvk := tt.policy.Spec.Trigger.Resource.GetGVK()
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

	// Create test policies
	deploymentPolicy := &quotav1alpha1.ClaimCreationPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "deployment-policy",
		},
		Spec: quotav1alpha1.ClaimCreationPolicySpec{
			Trigger: quotav1alpha1.ClaimTriggerSpec{
				Resource: quotav1alpha1.TargetResource{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
				},
			},
			Enabled: ptr.To(true),
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
			Trigger: quotav1alpha1.ClaimTriggerSpec{
				Resource: quotav1alpha1.TargetResource{
					APIVersion: "apps/v1",
					Kind:       "StatefulSet",
				},
			},
			Enabled: ptr.To(false),
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

// TestPolicyEngineReconciliation tests policy addition, update, and removal
func TestPolicyEngineReconciliation(t *testing.T) {
	logger := logr.Discard()
	engine := &policyEngine{
		logger:      logger,
		gvkIndex:    sync.Map{},
		initialized: true,
	}

	// Test policy addition
	t.Run("add policy", func(t *testing.T) {
		policy := &quotav1alpha1.ClaimCreationPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-policy",
			},
			Spec: quotav1alpha1.ClaimCreationPolicySpec{
				Trigger: quotav1alpha1.ClaimTriggerSpec{
					Resource: quotav1alpha1.TargetResource{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
					},
				},
				Enabled: ptr.To(true),
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

		err := engine.updatePolicy(policy)
		if err != nil {
			t.Fatalf("updatePolicy() error = %v", err)
		}

		// Verify policy is indexed
		gvk := policy.Spec.Trigger.Resource.GetGVK()
		value, exists := engine.gvkIndex.Load(gvk.String())
		if !exists {
			t.Error("Policy not found in index after update")
		}

		storedPolicy := value.(*quotav1alpha1.ClaimCreationPolicy)
		if storedPolicy.Name != policy.Name {
			t.Errorf("Stored policy name = %s, want %s", storedPolicy.Name, policy.Name)
		}
	})

	// Test policy disable
	t.Run("disable policy", func(t *testing.T) {
		disabledPolicy := &quotav1alpha1.ClaimCreationPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-policy",
			},
			Spec: quotav1alpha1.ClaimCreationPolicySpec{
				Trigger: quotav1alpha1.ClaimTriggerSpec{
					Resource: quotav1alpha1.TargetResource{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
					},
				},
				Enabled: ptr.To(false), // Disabled
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

		err := engine.updatePolicy(disabledPolicy)
		if err != nil {
			t.Fatalf("updatePolicy() error = %v", err)
		}

		// Verify policy is removed from index when disabled
		gvk := disabledPolicy.Spec.Trigger.Resource.GetGVK()
		_, exists := engine.gvkIndex.Load(gvk.String())
		if exists {
			t.Error("Disabled policy should not be in index")
		}
	})

	// Test policy removal
	t.Run("remove policy", func(t *testing.T) {
		// First add a policy
		policy := &quotav1alpha1.ClaimCreationPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name: "policy-to-remove",
			},
			Spec: quotav1alpha1.ClaimCreationPolicySpec{
				Trigger: quotav1alpha1.ClaimTriggerSpec{
					Resource: quotav1alpha1.TargetResource{
						APIVersion: "batch/v1",
						Kind:       "Job",
					},
				},
				Enabled: ptr.To(true),
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

		err := engine.updatePolicy(policy)
		if err != nil {
			t.Fatalf("updatePolicy() error = %v", err)
		}

		// Verify policy is in index
		gvk := policy.Spec.Trigger.Resource.GetGVK()
		_, exists := engine.gvkIndex.Load(gvk.String())
		if !exists {
			t.Error("Policy should be in index before removal")
		}

		// Remove the policy
		engine.removePolicy("policy-to-remove")

		// Verify policy is removed from index
		_, exists = engine.gvkIndex.Load(gvk.String())
		if exists {
			t.Error("Policy should not be in index after removal")
		}
	})
}

// TestPolicyEngineNilPolicy tests handling of nil policy
func TestPolicyEngineNilPolicy(t *testing.T) {
	logger := logr.Discard()
	engine := &policyEngine{
		logger:      logger,
		gvkIndex:    sync.Map{},
		initialized: true,
	}

	err := engine.updatePolicy(nil)
	if err == nil {
		t.Error("Expected error for nil policy, but got none")
	}
	if !containsSubstring(err.Error(), "policy cannot be nil") {
		t.Errorf("Expected error message about nil policy, got: %v", err)
	}
}

// Helper function to check if a string contains a substring (simple implementation for tests)
func containsSubstring(s, substr string) bool {
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
