package quota

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/utils/ptr"

	quotav1alpha1 "go.miloapis.com/milo/pkg/apis/quota/v1alpha1"
)

// ClaimResult represents the result of a ResourceClaim evaluation
type ClaimResult struct {
	Granted bool
	Reason  string
	Error   error
}

// ClaimWaiter represents a request waiting for a ResourceClaim result
type ClaimWaiter struct {
	ClaimName    string
	Namespace    string
	ResultChan   chan<- ClaimResult
	TimeoutCtx   context.Context
	CancelFunc   context.CancelFunc
	RegisteredAt time.Time
}

// ClaimWatchManager manages a single shared watcher for all ResourceClaim events
// and routes events to the appropriate waiting requests
type ClaimWatchManager interface {
	// RegisterClaimWaiter registers a request to wait for a specific ResourceClaim result
	RegisterClaimWaiter(claimName, namespace string, timeout time.Duration) (<-chan ClaimResult, context.CancelFunc)

	// UnregisterClaimWaiter removes a waiting request (called on timeout/cancellation)
	UnregisterClaimWaiter(claimName, namespace string)

	// Start begins the shared watch loop
	Start(ctx context.Context) error

	// Stop gracefully shuts down the watch manager
	Stop()

	// GetStats returns statistics about the watch manager
	GetStats() WatchManagerStats
}

// WatchManagerStats provides metrics about the watch manager
type WatchManagerStats struct {
	ActiveWaiters    int
	TotalProcessed   int64
	WatchRestarts    int64
	LastWatchRestart time.Time
}

// sharedClaimWatchManager implements ClaimWatchManager
type sharedClaimWatchManager struct {
	dynamicClient dynamic.Interface
	logger        logr.Logger

	// Waiter management
	mu      sync.RWMutex
	waiters map[string]*ClaimWaiter // key: namespace/claimName

	// Watch management
	watchCtx    context.Context
	watchCancel context.CancelFunc
	watcher     watch.Interface

	// Statistics
	stats WatchManagerStats

	// Lifecycle
	started   bool
	stopped   bool
	startOnce sync.Once
	stopOnce  sync.Once
}

// NewClaimWatchManager creates a new shared watch manager
func NewClaimWatchManager(dynamicClient dynamic.Interface, logger logr.Logger) ClaimWatchManager {
	return &sharedClaimWatchManager{
		dynamicClient: dynamicClient,
		logger:        logger.WithName("claim-watch-manager"),
		waiters:       make(map[string]*ClaimWaiter),
	}
}

// RegisterClaimWaiter registers a request to wait for a specific ResourceClaim result
func (m *sharedClaimWatchManager) RegisterClaimWaiter(claimName, namespace string, timeout time.Duration) (<-chan ClaimResult, context.CancelFunc) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := fmt.Sprintf("%s/%s", namespace, claimName)

	// Create timeout context for this waiter
	timeoutCtx, cancelFunc := context.WithTimeout(context.Background(), timeout)

	// Create result channel
	resultChan := make(chan ClaimResult, 1)

	// Create waiter
	waiter := &ClaimWaiter{
		ClaimName:    claimName,
		Namespace:    namespace,
		ResultChan:   resultChan,
		TimeoutCtx:   timeoutCtx,
		CancelFunc:   cancelFunc,
		RegisteredAt: time.Now(),
	}

	// Store waiter
	m.waiters[key] = waiter

	m.logger.V(2).Info("Registered claim waiter",
		"claimName", claimName,
		"namespace", namespace,
		"activeWaiters", len(m.waiters))

	// Start timeout handler
	go m.handleWaiterTimeout(key, waiter)

	return resultChan, cancelFunc
}

// UnregisterClaimWaiter removes a waiting request
func (m *sharedClaimWatchManager) UnregisterClaimWaiter(claimName, namespace string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := fmt.Sprintf("%s/%s", namespace, claimName)

	if waiter, exists := m.waiters[key]; exists {
		// Cancel the timeout context
		waiter.CancelFunc()

		// Close result channel to signal cancellation
		close(waiter.ResultChan)

		// Remove from map
		delete(m.waiters, key)

		m.logger.V(2).Info("Unregistered claim waiter",
			"claimName", claimName,
			"namespace", namespace,
			"activeWaiters", len(m.waiters))
	}
}

// Start begins the shared watch loop
func (m *sharedClaimWatchManager) Start(ctx context.Context) error {
	var err error
	m.startOnce.Do(func() {
		m.logger.Info("Starting shared claim watch manager")

		m.watchCtx, m.watchCancel = context.WithCancel(ctx)
		m.started = true

		// Start the watch loop in a goroutine
		go m.runWatchLoop()
	})
	return err
}

// Stop gracefully shuts down the watch manager
func (m *sharedClaimWatchManager) Stop() {
	m.stopOnce.Do(func() {
		m.logger.Info("Stopping shared claim watch manager")

		m.stopped = true

		// Cancel watch context
		if m.watchCancel != nil {
			m.watchCancel()
		}

		// Stop current watcher
		if m.watcher != nil {
			m.watcher.Stop()
		}

		// Cancel all waiting requests
		m.mu.Lock()
		for key, waiter := range m.waiters {
			waiter.CancelFunc()
			close(waiter.ResultChan)
			delete(m.waiters, key)
		}
		m.mu.Unlock()

		m.logger.Info("Shared claim watch manager stopped")
	})
}

// GetStats returns statistics about the watch manager
func (m *sharedClaimWatchManager) GetStats() WatchManagerStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := m.stats
	stats.ActiveWaiters = len(m.waiters)
	return stats
}

// runWatchLoop runs the main watch loop with automatic reconnection
func (m *sharedClaimWatchManager) runWatchLoop() {
	m.logger.Info("Starting watch loop for ResourceClaims")

	for {
		select {
		case <-m.watchCtx.Done():
			m.logger.Info("Watch context cancelled, stopping watch loop")
			return
		default:
			if err := m.establishWatch(); err != nil {
				m.logger.Error(err, "Failed to establish watch, retrying in 30 seconds")

				select {
				case <-time.After(30 * time.Second):
					continue
				case <-m.watchCtx.Done():
					return
				}
			}

			// Watch was established successfully, process events
			m.processWatchEvents()

			// If we reach here, the watch ended unexpectedly
			m.logger.Info("Watch ended, restarting in 5 seconds")
			m.stats.WatchRestarts++
			m.stats.LastWatchRestart = time.Now()

			select {
			case <-time.After(5 * time.Second):
				continue
			case <-m.watchCtx.Done():
				return
			}
		}
	}
}

// establishWatch creates a new watch connection for ResourceClaims
func (m *sharedClaimWatchManager) establishWatch() error {
	gvr := schema.GroupVersionResource{
		Group:    "quota.miloapis.com",
		Version:  "v1alpha1",
		Resource: "resourceclaims",
	}

	// Start watch with initial events - this eliminates the need for separate list operation
	// and removes the race condition between list and watch
	watchOptions := metav1.ListOptions{
		Watch:             true,
		SendInitialEvents: ptr.To(true),
		ResourceVersionMatch: "NotOlderThan",
	}

	watcher, err := m.dynamicClient.Resource(gvr).Watch(m.watchCtx, watchOptions)
	if err != nil {
		return fmt.Errorf("failed to start ResourceClaim watch: %w", err)
	}

	m.watcher = watcher
	m.logger.Info("ResourceClaim watch established successfully with initial events")

	return nil
}

// processWatchEvents processes events from the shared watch
func (m *sharedClaimWatchManager) processWatchEvents() {
	defer func() {
		if m.watcher != nil {
			m.watcher.Stop()
			m.watcher = nil
		}
	}()

	for {
		select {
		case <-m.watchCtx.Done():
			return
		case event, ok := <-m.watcher.ResultChan():
			if !ok {
				m.logger.V(1).Info("Watch channel closed")
				return
			}

			switch event.Type {
			case watch.Added, watch.Modified:
				if obj, ok := event.Object.(*unstructured.Unstructured); ok {
					m.handleClaimEvent(obj)
				}
			case watch.Deleted:
				if obj, ok := event.Object.(*unstructured.Unstructured); ok {
					m.handleClaimDeletion(obj)
				}
			case watch.Error:
				m.logger.Error(fmt.Errorf("watch error"), "Watch error received", "error", event.Object)
				return
			}
		}
	}
}

// handleClaimEvent processes a ResourceClaim event and routes it to waiting requests
func (m *sharedClaimWatchManager) handleClaimEvent(claim *unstructured.Unstructured) {
	claimName := claim.GetName()
	namespace := claim.GetNamespace()
	key := fmt.Sprintf("%s/%s", namespace, claimName)

	m.logger.V(3).Info("Processing claim event", "claimName", claimName, "namespace", namespace)

	m.mu.RLock()
	waiter, exists := m.waiters[key]
	m.mu.RUnlock()

	if !exists {
		// No one is waiting for this claim, skip
		m.logger.V(3).Info("No waiter found for claim", "claimName", claimName, "namespace", namespace)
		return
	}

	// Check if the claim is granted or denied
	if granted := m.isClaimGranted(claim); granted {
		m.sendResult(key, waiter, ClaimResult{
			Granted: true,
			Reason:  "ResourceClaim granted",
			Error:   nil,
		})
	} else if denied := m.isClaimDenied(claim); denied {
		reason := m.getClaimDenialReason(claim)
		m.sendResult(key, waiter, ClaimResult{
			Granted: false,
			Reason:  reason,
			Error:   fmt.Errorf("ResourceClaim was denied: %s", reason),
		})
	}
	// If neither granted nor denied, continue waiting
}

// handleClaimDeletion processes a ResourceClaim deletion
func (m *sharedClaimWatchManager) handleClaimDeletion(claim *unstructured.Unstructured) {
	claimName := claim.GetName()
	namespace := claim.GetNamespace()
	key := fmt.Sprintf("%s/%s", namespace, claimName)

	m.mu.RLock()
	waiter, exists := m.waiters[key]
	m.mu.RUnlock()

	if exists {
		m.sendResult(key, waiter, ClaimResult{
			Granted: false,
			Reason:  "ResourceClaim was deleted",
			Error:   fmt.Errorf("ResourceClaim was deleted while waiting"),
		})
	}
}

// sendResult sends a result to a waiting request and cleans up
func (m *sharedClaimWatchManager) sendResult(key string, waiter *ClaimWaiter, result ClaimResult) {
	m.logger.V(2).Info("Sending result to waiter",
		"claimName", waiter.ClaimName,
		"namespace", waiter.Namespace,
		"granted", result.Granted,
		"reason", result.Reason)

	// Send result (non-blocking)
	select {
	case waiter.ResultChan <- result:
		// Result sent successfully
	default:
		// Channel full or closed, waiter may have timed out
		m.logger.V(1).Info("Failed to send result to waiter, channel full or closed",
			"claimName", waiter.ClaimName)
	}

	// Cancel timeout and clean up
	waiter.CancelFunc()

	// Remove from waiters map
	m.mu.Lock()
	delete(m.waiters, key)
	m.stats.TotalProcessed++
	m.mu.Unlock()
}

// handleWaiterTimeout handles timeout for a specific waiter
func (m *sharedClaimWatchManager) handleWaiterTimeout(key string, waiter *ClaimWaiter) {
	select {
	case <-waiter.TimeoutCtx.Done():
		// Timeout occurred, send timeout result
		m.mu.RLock()
		_, stillWaiting := m.waiters[key]
		m.mu.RUnlock()

		if stillWaiting {
			m.sendResult(key, waiter, ClaimResult{
				Granted: false,
				Reason:  "timeout waiting for ResourceClaim",
				Error:   fmt.Errorf("timeout waiting for ResourceClaim to be granted"),
			})
		}
	}
}

// isClaimGranted checks if a ResourceClaim has been granted
func (m *sharedClaimWatchManager) isClaimGranted(claim *unstructured.Unstructured) bool {
	conditions, found, err := unstructured.NestedSlice(claim.Object, "status", "conditions")
	if err != nil || !found {
		return false
	}

	for _, conditionInterface := range conditions {
		condition, ok := conditionInterface.(map[string]interface{})
		if !ok {
			continue
		}

		conditionType, _, _ := unstructured.NestedString(condition, "type")
		conditionStatus, _, _ := unstructured.NestedString(condition, "status")

		if conditionType == quotav1alpha1.ResourceClaimGranted && conditionStatus == string(metav1.ConditionTrue) {
			return true
		}
	}
	return false
}

// isClaimDenied checks if a ResourceClaim has been denied
func (m *sharedClaimWatchManager) isClaimDenied(claim *unstructured.Unstructured) bool {
	conditions, found, err := unstructured.NestedSlice(claim.Object, "status", "conditions")
	if err != nil || !found {
		return false
	}

	for _, conditionInterface := range conditions {
		condition, ok := conditionInterface.(map[string]interface{})
		if !ok {
			continue
		}

		conditionType, _, _ := unstructured.NestedString(condition, "type")
		conditionStatus, _, _ := unstructured.NestedString(condition, "status")
		conditionReason, _, _ := unstructured.NestedString(condition, "reason")

		if conditionType == quotav1alpha1.ResourceClaimGranted &&
			conditionStatus == string(metav1.ConditionFalse) &&
			conditionReason == quotav1alpha1.ResourceClaimDeniedReason {
			return true
		}
	}
	return false
}

// getClaimDenialReason returns the reason why a ResourceClaim was denied
func (m *sharedClaimWatchManager) getClaimDenialReason(claim *unstructured.Unstructured) string {
	conditions, found, err := unstructured.NestedSlice(claim.Object, "status", "conditions")
	if err != nil || !found {
		return "unknown reason"
	}

	for _, conditionInterface := range conditions {
		condition, ok := conditionInterface.(map[string]interface{})
		if !ok {
			continue
		}

		conditionType, _, _ := unstructured.NestedString(condition, "type")
		conditionStatus, _, _ := unstructured.NestedString(condition, "status")
		conditionMessage, _, _ := unstructured.NestedString(condition, "message")

		if conditionType == quotav1alpha1.ResourceClaimGranted && conditionStatus == string(metav1.ConditionFalse) {
			if conditionMessage != "" {
				return conditionMessage
			}
			return "quota exceeded"
		}
	}
	return "unknown reason"
}
