package quota

import (
	"context"
	"fmt"
	"strings"
	"sync"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	quotav1alpha1 "go.miloapis.com/milo/pkg/apis/quota/v1alpha1"
)

// PolicyEngine maintains an in-memory index of active GrantCreationPolicy resources
// and provides efficient lookup by trigger resource type.
type PolicyEngine struct {
	client.Client
	// policiesByResourceType maps GVK strings to active policies
	policiesByResourceType map[string][]*quotav1alpha1.GrantCreationPolicy
	// allPolicies keeps references to all active policies for easy management
	allPolicies map[string]*quotav1alpha1.GrantCreationPolicy
	// mutex protects concurrent access to the maps
	mu sync.RWMutex
	// started indicates if the engine has been started
	started bool
	// eventRecorder for recording events
	eventRecorder func(object client.Object, eventType, reason, message string)
}

// NewPolicyEngine creates a new PolicyEngine instance.
func NewPolicyEngine(client client.Client) *PolicyEngine {
	return &PolicyEngine{
		Client:                 client,
		policiesByResourceType: make(map[string][]*quotav1alpha1.GrantCreationPolicy),
		allPolicies:            make(map[string]*quotav1alpha1.GrantCreationPolicy),
	}
}

// SetEventRecorder sets the event recorder function for recording Kubernetes events.
func (e *PolicyEngine) SetEventRecorder(recorder func(object client.Object, eventType, reason, message string)) {
	e.eventRecorder = recorder
}

// Start initializes the PolicyEngine by loading existing policies.
func (e *PolicyEngine) Start(ctx context.Context) error {
	logger := log.FromContext(ctx)

	if e.started {
		return fmt.Errorf("policy engine already started")
	}

	logger.Info("Starting PolicyEngine")

	// Load existing policies
	if err := e.loadExistingPolicies(ctx); err != nil {
		return fmt.Errorf("failed to load existing policies: %w", err)
	}

	e.started = true
	logger.Info("PolicyEngine started successfully", "activePolicies", len(e.allPolicies))

	return nil
}

// IsStarted returns true if the policy engine has been started.
func (e *PolicyEngine) IsStarted() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.started
}

// RefreshPolicies reloads all policies from the API server.
func (e *PolicyEngine) RefreshPolicies(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Clear existing policies
	e.policiesByResourceType = make(map[string][]*quotav1alpha1.GrantCreationPolicy)
	e.allPolicies = make(map[string]*quotav1alpha1.GrantCreationPolicy)

	// Reload policies
	return e.loadExistingPolicies(ctx)
}

// GetPoliciesForResource returns all active policies that target the given resource type.
func (e *PolicyEngine) GetPoliciesForResource(gvk schema.GroupVersionKind) []*quotav1alpha1.GrantCreationPolicy {
	e.mu.RLock()
	defer e.mu.RUnlock()

	gvkString := gvk.String()
	policies := e.policiesByResourceType[gvkString]

	// Return a copy to avoid concurrent modification issues
	result := make([]*quotav1alpha1.GrantCreationPolicy, len(policies))
	copy(result, policies)

	return result
}

// GetAllActivePolicies returns all currently active policies.
func (e *PolicyEngine) GetAllActivePolicies() []*quotav1alpha1.GrantCreationPolicy {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*quotav1alpha1.GrantCreationPolicy, 0, len(e.allPolicies))
	for _, policy := range e.allPolicies {
		result = append(result, policy)
	}

	return result
}

// GetPolicyByName returns a specific policy by name.
func (e *PolicyEngine) GetPolicyByName(name string) (*quotav1alpha1.GrantCreationPolicy, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	policy, exists := e.allPolicies[name]
	return policy, exists
}

// GetActiveResourceTypes returns all resource types that have active policies.
func (e *PolicyEngine) GetActiveResourceTypes() []schema.GroupVersionKind {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var result []schema.GroupVersionKind
	for gvkString := range e.policiesByResourceType {
		if gvk, err := parseGVKString(gvkString); err == nil {
			result = append(result, gvk)
		}
	}

	return result
}

// loadExistingPolicies loads all existing GrantCreationPolicy resources.
func (e *PolicyEngine) loadExistingPolicies(ctx context.Context) error {
	logger := log.FromContext(ctx)

	var policyList quotav1alpha1.GrantCreationPolicyList
	if err := e.List(ctx, &policyList); err != nil {
		return fmt.Errorf("failed to list GrantCreationPolicies: %w", err)
	}

	logger.Info("Loading existing policies", "count", len(policyList.Items))

	for i := range policyList.Items {
		policy := &policyList.Items[i]
		if e.isPolicyActive(policy) {
			e.addPolicy(policy)
			logger.V(2).Info("Loaded active policy", "policy", policy.Name, "triggerResource", policy.Spec.Trigger.Resource.Kind)
		} else {
			logger.V(2).Info("Skipped inactive policy", "policy", policy.Name, "enabled", policy.Spec.Enabled)
		}
	}

	return nil
}

// UpdatePolicy updates a specific policy in the engine.
func (e *PolicyEngine) UpdatePolicy(ctx context.Context, policyName string) error {
	logger := log.FromContext(ctx).WithValues("policy", policyName)

	// Fetch the latest version of the policy
	var policy quotav1alpha1.GrantCreationPolicy
	if err := e.Get(ctx, client.ObjectKey{Name: policyName}, &policy); err != nil {
		if client.IgnoreNotFound(err) == nil {
			// Policy was deleted
			logger.Info("Policy deleted, removing from engine")
			e.removePolicy(policyName)
			return nil
		}
		return fmt.Errorf("failed to fetch policy: %w", err)
	}

	// Check if policy is now active or inactive
	if e.isPolicyActive(&policy) {
		logger.Info("Policy is active, adding/updating in engine")
		e.addPolicy(&policy)
		e.recordEvent(&policy, "Normal", "PolicyActivated", "Policy activated and added to engine")
	} else {
		logger.Info("Policy is inactive, removing from engine")
		e.removePolicy(policy.Name)
		e.recordEvent(&policy, "Normal", "PolicyDeactivated", "Policy deactivated and removed from engine")
	}

	return nil
}

// addPolicy adds or updates a policy in the engine.
func (e *PolicyEngine) addPolicy(policy *quotav1alpha1.GrantCreationPolicy) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Remove from old resource type if it existed
	if existingPolicy, exists := e.allPolicies[policy.Name]; exists {
		e.removePolicyFromResourceTypeIndex(existingPolicy)
	}

	// Add to new resource type
	gvk := policy.Spec.Trigger.Resource.GetGVK()
	gvkString := gvk.String()

	// Store a copy of the policy
	policyCopy := policy.DeepCopy()
	e.allPolicies[policy.Name] = policyCopy

	// Add to resource type index
	e.policiesByResourceType[gvkString] = append(e.policiesByResourceType[gvkString], policyCopy)
}

// removePolicy removes a policy from the engine.
func (e *PolicyEngine) removePolicy(policyName string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if policy, exists := e.allPolicies[policyName]; exists {
		e.removePolicyFromResourceTypeIndex(policy)
		delete(e.allPolicies, policyName)
	}
}

// removePolicyFromResourceTypeIndex removes a policy from the resource type index.
func (e *PolicyEngine) removePolicyFromResourceTypeIndex(policy *quotav1alpha1.GrantCreationPolicy) {
	gvk := policy.Spec.Trigger.Resource.GetGVK()
	gvkString := gvk.String()

	policies := e.policiesByResourceType[gvkString]
	for i, p := range policies {
		if p.Name == policy.Name {
			// Remove this policy from the slice
			e.policiesByResourceType[gvkString] = append(policies[:i], policies[i+1:]...)
			break
		}
	}

	// Clean up empty slices
	if len(e.policiesByResourceType[gvkString]) == 0 {
		delete(e.policiesByResourceType, gvkString)
	}
}

// isPolicyActive checks if a policy is active (enabled and ready).
func (e *PolicyEngine) isPolicyActive(policy *quotav1alpha1.GrantCreationPolicy) bool {
	// Check if policy is enabled
	if policy.Spec.Enabled != nil && !*policy.Spec.Enabled {
		return false
	}

	// Check if policy is ready
	readyCondition := apimeta.FindStatusCondition(policy.Status.Conditions, quotav1alpha1.GrantCreationPolicyReady)
	if readyCondition == nil || readyCondition.Status != metav1.ConditionTrue {
		return false
	}

	return true
}

// recordEvent records a Kubernetes event if an event recorder is configured.
func (e *PolicyEngine) recordEvent(obj client.Object, eventType, reason, message string) {
	if e.eventRecorder != nil {
		e.eventRecorder(obj, eventType, reason, message)
	}
}

// parseGVKString parses a GroupVersionKind string back to schema.GroupVersionKind.
func parseGVKString(gvkString string) (schema.GroupVersionKind, error) {
	// GVK string format is typically "Group/Version, Kind=Kind"
	// We'll use a simple parsing approach
	parts := strings.Split(gvkString, ", ")
	if len(parts) != 2 {
		return schema.GroupVersionKind{}, fmt.Errorf("invalid GVK string format: %s", gvkString)
	}

	gvPart := parts[0]
	kindPart := parts[1]

	// Parse group/version
	gvParts := strings.Split(gvPart, "/")
	if len(gvParts) != 2 {
		return schema.GroupVersionKind{}, fmt.Errorf("invalid group/version format: %s", gvPart)
	}

	// Parse kind
	if !strings.HasPrefix(kindPart, "Kind=") {
		return schema.GroupVersionKind{}, fmt.Errorf("invalid kind format: %s", kindPart)
	}
	kind := strings.TrimPrefix(kindPart, "Kind=")

	return schema.GroupVersionKind{
		Group:   gvParts[0],
		Version: gvParts[1],
		Kind:    kind,
	}, nil
}
