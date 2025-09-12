package quota

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/utils/ptr"

	quotav1alpha1 "go.miloapis.com/milo/pkg/apis/quota/v1alpha1"
)

// PolicyEngine manages ClaimCreationPolicy resources and provides fast GVK-based lookups.
// It is designed for use by the admission plugin and continuously loads policies in the background.
type PolicyEngine interface {
	// GetPolicyForGVK returns the active policy for a given GroupVersionKind.
	// Returns nil if no policy is found.
	GetPolicyForGVK(gvk schema.GroupVersionKind) (*quotav1alpha1.ClaimCreationPolicy, error)

	// Close stops the policy engine and cleans up resources like watchers.
	Close()
}

// policyEngine implements PolicyEngine for admission plugin use with lazy loading
type policyEngine struct {
	dynamicClient dynamic.Interface
	logger        logr.Logger
	mu            sync.RWMutex
	gvkIndex      sync.Map // map[string]*quotav1alpha1.ClaimCreationPolicy
	initialized   bool

	// Watch management
	watchStarted sync.Once
	watchCtx     context.Context
	watchCancel  context.CancelFunc
	watcher      watch.Interface
}

// NewPolicyEngine creates a policy engine suitable for admission plugin use.
// This version starts loading policies immediately and establishes a watch for changes.
func NewPolicyEngine(dynamicClient dynamic.Interface, logger logr.Logger) (PolicyEngine, error) {
	engine := &policyEngine{
		dynamicClient: dynamicClient,
		logger:        logger.WithName("policy-engine"),
		gvkIndex:      sync.Map{},
		initialized:   false,
	}

	// Start loading policies immediately in background
	go engine.startPolicyLoadingLoop()

	return engine, nil
}

// GetPolicyForGVK returns the active policy for a given GroupVersionKind.
func (e *policyEngine) GetPolicyForGVK(gvk schema.GroupVersionKind) (*quotav1alpha1.ClaimCreationPolicy, error) {
	e.logger.V(1).Info("Looking up policy for GVK", "gvk", gvk.String())

	if value, ok := e.gvkIndex.Load(gvk.String()); ok {
		if policy, ok := value.(*quotav1alpha1.ClaimCreationPolicy); ok {
			// Check if policy is enabled
			if policy.Spec.Enabled != nil && !*policy.Spec.Enabled {
				return nil, nil // Policy exists but is disabled
			}
			e.logger.V(1).Info("Found policy for GVK", "gvk", gvk.String(), "policy", policy.Name)
			return policy, nil
		}
	}

	e.logger.V(3).Info("No policy found for GVK", "gvk", gvk.String())
	return nil, nil // No policy found for this GVK
}

// startPolicyLoadingLoop starts the background loop to continuously load policies and establish watch
func (e *policyEngine) startPolicyLoadingLoop() {
	e.logger.Info("Starting policy loading loop in background")

	// Create a context for the loading loop
	ctx := context.Background()

	for {
		e.logger.V(1).Info("Attempting to load ClaimCreationPolicies")

		if err := e.loadPolicies(ctx); err != nil {
			e.logger.Error(err, "Failed to load policies, retrying in 30 seconds")

			// Wait before retrying
			select {
			case <-time.After(30 * time.Second):
				continue
			case <-ctx.Done():
				e.logger.Info("Policy loading loop context cancelled")
				return
			}
		}

		e.logger.Info("Successfully loaded policies and established watch")

		// Mark as initialized
		e.mu.Lock()
		e.initialized = true
		e.mu.Unlock()

		// Wait for the watch context to be done before restarting
		if e.watchCtx != nil {
			<-e.watchCtx.Done()
			e.logger.Info("Watch context ended, restarting policy loading loop")

			// Reset initialization status to allow reloading
			e.mu.Lock()
			e.initialized = false
			e.mu.Unlock()
		}
	}
}

// updatePolicy adds or updates a policy in the cache.
func (e *policyEngine) updatePolicy(policy *quotav1alpha1.ClaimCreationPolicy) error {
	if policy == nil {
		return fmt.Errorf("policy cannot be nil")
	}

	// Only process policies with Ready=True status
	if !e.isPolicyReady(policy) {
		e.logger.V(1).Info("Policy not ready, skipping update", "policy", policy.Name)
		e.removePolicy(policy.Name)
		return nil
	}

	gvk := policy.Spec.TargetResource.GetGVK()
	gvkKey := gvk.String()

	// Check if policy is disabled
	if policy.Spec.Enabled != nil && !*policy.Spec.Enabled {
		// Remove disabled policy from cache
		e.removePolicy(policy.Name)
		e.logger.V(1).Info("Policy disabled, removed from cache", "policy", policy.Name, "gvk", gvk)
		return nil
	}

	// Check for conflicts - only one policy per GVK is allowed
	if existing, exists := e.gvkIndex.Load(gvkKey); exists {
		existingPolicy := existing.(*quotav1alpha1.ClaimCreationPolicy)
		if existingPolicy.Name != policy.Name {
			e.logger.Error(nil, "Multiple policies found for same GVK, replacing existing",
				"gvk", gvk,
				"existing", existingPolicy.Name,
				"new", policy.Name)
		}
	}

	// Store the policy in the cache
	e.gvkIndex.Store(gvkKey, policy.DeepCopy())

	e.logger.V(1).Info("Policy updated in cache",
		"policy", policy.Name,
		"gvk", gvk,
		"ready", true,
		"enabled", policy.Spec.Enabled == nil || *policy.Spec.Enabled)

	return nil
}

// removePolicy removes a policy from the cache by name.
func (e *policyEngine) removePolicy(policyName string) {
	// Since we need to find the policy by name but our index is by GVK,
	// we need to iterate through the cache to find the policy with the matching name
	var gvkKeyToRemove *string

	e.gvkIndex.Range(func(key, value interface{}) bool {
		gvkKey := key.(string)
		policy := value.(*quotav1alpha1.ClaimCreationPolicy)

		if policy.Name == policyName {
			gvkKeyToRemove = &gvkKey
			return false // Stop iteration
		}
		return true // Continue iteration
	})

	if gvkKeyToRemove != nil {
		e.gvkIndex.Delete(*gvkKeyToRemove)
		e.logger.V(1).Info("Policy removed from cache", "policy", policyName, "gvkKey", *gvkKeyToRemove)
	}
}

// isPolicyReady checks if a ClaimCreationPolicy has Ready=True status condition
func (e *policyEngine) isPolicyReady(policy *quotav1alpha1.ClaimCreationPolicy) bool {
	for _, condition := range policy.Status.Conditions {
		if condition.Type == quotav1alpha1.ClaimCreationPolicyReady {
			return condition.Status == metav1.ConditionTrue
		}
	}
	// No Ready condition found means policy is not ready
	return false
}

// loadPolicies loads only ClaimCreationPolicies with Ready=True status from the API server using dynamic client
func (e *policyEngine) loadPolicies(ctx context.Context) error {
	gvr := schema.GroupVersionResource{
		Group:    "quota.miloapis.com",
		Version:  "v1alpha1",
		Resource: "claimcreationpolicies",
	}

	// List all ClaimCreationPolicies
	list, err := e.dynamicClient.Resource(gvr).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list ClaimCreationPolicies: %w", err)
	}

	// Clear existing index
	e.gvkIndex = sync.Map{}

	count := 0
	readyCount := 0
	for _, item := range list.Items {
		// Convert unstructured to ClaimCreationPolicy
		var policy quotav1alpha1.ClaimCreationPolicy
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(item.Object, &policy); err != nil {
			e.logger.Error(err, "Failed to convert policy", "name", item.GetName())
			continue
		}

		count++

		// Only load policies with Ready=True status condition
		if !e.isPolicyReady(&policy) {
			e.logger.V(1).Info("Skipping policy - not ready", "name", policy.Name)
			continue
		}

		if err := e.updatePolicy(&policy); err != nil {
			e.logger.Error(err, "Failed to index policy", "name", policy.Name)
			continue
		}
		readyCount++
	}

	e.logger.Info("Loaded ClaimCreationPolicies", "totalPolicies", count, "readyPolicies", readyCount)

	// Debug: Log the current state of the policy cache
	e.logger.V(1).Info("Policy cache state after loading", "totalPoliciesLoaded", readyCount)
	e.gvkIndex.Range(func(key, value interface{}) bool {
		gvkKey := key.(string)
		policy := value.(*quotav1alpha1.ClaimCreationPolicy)
		e.logger.V(1).Info("Policy in cache", "gvkKey", gvkKey, "policy", policy.Name, "enabled", policy.Spec.Enabled == nil || *policy.Spec.Enabled)
		return true
	})

	// Start watching for policy changes after initial load
	e.startWatch()

	return nil
}

// startWatch establishes a watch for ClaimCreationPolicy changes after initial load
func (e *policyEngine) startWatch() {
	e.watchStarted.Do(func() {
		e.logger.V(1).Info("Starting watch for ClaimCreationPolicy changes")

		// Create a context for the watch that can be cancelled
		e.watchCtx, e.watchCancel = context.WithCancel(context.Background())

		go e.runWatch()
	})
}

// runWatch runs the watch loop in a separate goroutine
func (e *policyEngine) runWatch() {
	gvr := schema.GroupVersionResource{
		Group:    "quota.miloapis.com",
		Version:  "v1alpha1",
		Resource: "claimcreationpolicies",
	}

	for {
		select {
		case <-e.watchCtx.Done():
			e.logger.V(1).Info("Watch context cancelled, stopping policy watch")
			return
		default:
			// Start the watch with initial events - this eliminates the need for separate list operation
			// and removes the race condition between list and watch
			watchOptions := metav1.ListOptions{
				Watch:                true,
				SendInitialEvents:    ptr.To(true),
				ResourceVersionMatch: "NotOlderThan",
			}

			watcher, err := e.dynamicClient.Resource(gvr).Watch(e.watchCtx, watchOptions)
			if err != nil {
				e.logger.Error(err, "Failed to start policy watch, retrying in 30 seconds")
				select {
				case <-time.After(30 * time.Second):
					continue
				case <-e.watchCtx.Done():
					return
				}
			}

			e.logger.V(1).Info("Policy watch established successfully with initial events")
			e.watcher = watcher

			// Process watch events
			e.processWatchEvents(watcher)

			// Watch ended, clean up and retry
			watcher.Stop()
			e.watcher = nil

			select {
			case <-time.After(5 * time.Second):
				e.logger.V(1).Info("Restarting policy watch after connection loss")
				continue
			case <-e.watchCtx.Done():
				return
			}
		}
	}
}

// processWatchEvents processes events from the watch stream
func (e *policyEngine) processWatchEvents(watcher watch.Interface) {
	for {
		select {
		case <-e.watchCtx.Done():
			return
		case event, ok := <-watcher.ResultChan():
			if !ok {
				e.logger.V(1).Info("Watch channel closed, will restart watch")
				return
			}

			switch event.Type {
			case watch.Added, watch.Modified:
				if obj, ok := event.Object.(*unstructured.Unstructured); ok {
					// Convert unstructured to ClaimCreationPolicy
					var policy quotav1alpha1.ClaimCreationPolicy
					if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &policy); err != nil {
						e.logger.Error(err, "Failed to convert policy from watch event", "name", obj.GetName())
						continue
					}

					e.logger.V(1).Info("Policy watch event", "type", event.Type, "policy", policy.Name)

					// Update the policy in cache
					if err := e.updatePolicy(&policy); err != nil {
						e.logger.Error(err, "Failed to update policy from watch event", "policy", policy.Name)
					}
				}

			case watch.Deleted:
				if obj, ok := event.Object.(*unstructured.Unstructured); ok {
					policyName := obj.GetName()
					e.logger.V(1).Info("Policy deleted", "policy", policyName)
					e.removePolicy(policyName)
				}

			case watch.Error:
				e.logger.Error(fmt.Errorf("watch error"), "Watch error received", "error", event.Object)
				return
			}
		}
	}
}

// Close stops the policy engine and cleans up resources
func (e *policyEngine) Close() {
	e.logger.V(1).Info("Closing policy engine")

	if e.watchCancel != nil {
		e.watchCancel()
	}

	if e.watcher != nil {
		e.watcher.Stop()
	}
}
