package admission

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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/component-base/metrics"
	legacyregistry "k8s.io/component-base/metrics/legacyregistry"

	quotav1alpha1 "go.miloapis.com/milo/pkg/apis/quota/v1alpha1"
)

// claimWaiter represents a waiter for a specific ResourceClaim
type claimWaiter struct {
	claimName  string
	namespace  string
	resultChan chan ClaimResult
	timeout    time.Duration
	cancelFunc context.CancelFunc
	timer      *time.Timer
	startTime  time.Time
}

// sharedInformerClaimWatchManager implements ClaimWatchManager using shared informers
type sharedInformerClaimWatchManager struct {
	dynamicClient dynamic.Interface
	logger        logr.Logger
	config        *WatchManagerConfig

	// Waiters management
	waitersLock sync.RWMutex
	waiters     map[types.NamespacedName]*claimWaiter

	// Shared informer
	informer cache.SharedIndexInformer
	stopCh   chan struct{}
	started  bool

	// Workqueue for processing events
	workqueue workqueue.TypedRateLimitingInterface[types.NamespacedName]

	// Synchronization
	startOnce sync.Once
}

// Metrics for the watch manager. Registered once at init.
var (
	waitersCurrent = metrics.NewGauge(
		&metrics.GaugeOpts{
			Subsystem:      "milo_quota",
			Name:           "waiters_current",
			Help:           "Current number of active ResourceClaim waiters registered in the admission watch manager.",
			StabilityLevel: metrics.ALPHA,
		},
	)

	waitRegisterTotal = metrics.NewCounter(
		&metrics.CounterOpts{
			Subsystem:      "milo_quota",
			Name:           "wait_register_total",
			Help:           "Total number of waiter registrations in the admission watch manager.",
			StabilityLevel: metrics.ALPHA,
		},
	)

	waitUnregisterTotal = metrics.NewCounter(
		&metrics.CounterOpts{
			Subsystem:      "milo_quota",
			Name:           "wait_unregister_total",
			Help:           "Total number of waiter unregistrations in the admission watch manager.",
			StabilityLevel: metrics.ALPHA,
		},
	)

	waitTimeSeconds = metrics.NewHistogramVec(
		&metrics.HistogramOpts{
			Subsystem:      "milo_quota",
			Name:           "wait_time_seconds",
			Help:           "Time from waiter registration to final result.",
			Buckets:        []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10, 20, 30, 60},
			StabilityLevel: metrics.ALPHA,
		},
		[]string{"result"},
	)

	informerEventsTotal = metrics.NewCounterVec(
		&metrics.CounterOpts{
			Subsystem:      "milo_quota",
			Name:           "informer_events_total",
			Help:           "ResourceClaim events observed by the shared informer.",
			StabilityLevel: metrics.ALPHA,
		},
		[]string{"type"}, // add|update|delete|unknown
	)
)

func init() {
	// Register metrics with Kubernetes legacy registry so they are exposed on the apiserver /metrics.
	legacyregistry.MustRegister(waitersCurrent)
	legacyregistry.MustRegister(waitRegisterTotal)
	legacyregistry.MustRegister(waitUnregisterTotal)
	legacyregistry.MustRegister(waitTimeSeconds)
	legacyregistry.MustRegister(informerEventsTotal)
}

// NewClaimWatchManager creates a new ClaimWatchManager using shared informers.
func NewClaimWatchManager(dynamicClient dynamic.Interface, logger logr.Logger) ClaimWatchManager {
	config := DefaultWatchManagerConfig()

	return &sharedInformerClaimWatchManager{
		dynamicClient: dynamicClient,
		logger:        logger,
		config:        config,
		waiters:       make(map[types.NamespacedName]*claimWaiter),
		stopCh:        make(chan struct{}),
		// Name the workqueue so standard client-go workqueue_* metrics are labeled.
		workqueue: workqueue.NewTypedRateLimitingQueueWithConfig(
			workqueue.DefaultTypedControllerRateLimiter[types.NamespacedName](),
			workqueue.TypedRateLimitingQueueConfig[types.NamespacedName]{
				Name: "quota_admission_claim_watch",
			},
		),
	}
}

// Start starts the shared informer and begins watching ResourceClaims
func (w *sharedInformerClaimWatchManager) Start(ctx context.Context) error {
	var startErr error
	w.startOnce.Do(func() {
		w.logger.Info("Starting shared informer claim watch manager")

		// Create GVR for ResourceClaims
		gvr := schema.GroupVersionResource{
			Group:    quotav1alpha1.GroupVersion.Group,
			Version:  quotav1alpha1.GroupVersion.Version,
			Resource: "resourceclaims",
		}

		// Create shared informer
		lw := &cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				return w.dynamicClient.Resource(gvr).List(ctx, options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return w.dynamicClient.Resource(gvr).Watch(ctx, options)
			},
		}

		w.informer = cache.NewSharedIndexInformer(
			lw,
			&unstructured.Unstructured{},
			w.config.InformerResyncPeriod,
			cache.Indexers{},
		)

		// Add event handlers
		_, err := w.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				informerEventsTotal.WithLabelValues("add").Inc()
				w.handleClaimEvent(obj)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				informerEventsTotal.WithLabelValues("update").Inc()
				w.handleClaimEvent(newObj)
			},
			DeleteFunc: func(obj interface{}) {
				informerEventsTotal.WithLabelValues("delete").Inc()
				w.handleClaimEvent(obj)
			},
		})
		if err != nil {
			startErr = fmt.Errorf("failed to add event handler: %w", err)
			return
		}

		// Start the informer
		go w.informer.Run(w.stopCh)

		// Start the workqueue processor
		go w.processWorkItems(ctx)

		// Wait for informer cache to sync
		if !cache.WaitForCacheSync(w.stopCh, w.informer.HasSynced) {
			startErr = fmt.Errorf("timed out waiting for cache to sync")
			return
		}

		w.started = true
		w.logger.Info("Shared informer claim watch manager started successfully")
	})

	return startErr
}

// RegisterClaimWaiter registers a waiter for a specific ResourceClaim
func (w *sharedInformerClaimWatchManager) RegisterClaimWaiter(ctx context.Context, claimName, namespace string, timeout time.Duration) (<-chan ClaimResult, context.CancelFunc, error) {
	if !w.started {
		return nil, nil, fmt.Errorf("watch manager not started")
	}

	key := types.NamespacedName{Namespace: namespace, Name: claimName}

	w.logger.V(4).Info("Registering claim waiter",
		"claimName", claimName,
		"namespace", namespace,
		"timeout", timeout,
	)

	// Check if we've reached the maximum number of waiters
	w.waitersLock.RLock()
	if w.config.MaxWaiters > 0 && len(w.waiters) >= w.config.MaxWaiters {
		w.waitersLock.RUnlock()
		return nil, nil, fmt.Errorf("maximum number of waiters (%d) reached", w.config.MaxWaiters)
	}
	w.waitersLock.RUnlock()

	// Create waiter context that respects both the incoming context and our timeout
	_, cancelFunc := context.WithCancel(ctx)

	resultChan := make(chan ClaimResult, 1)

	// Create the waiter struct (without timer initially)
	waiter := &claimWaiter{
		claimName:  claimName,
		namespace:  namespace,
		resultChan: resultChan,
		timeout:    timeout,
		cancelFunc: cancelFunc,
		timer:      nil, // Will be set later if needed
		startTime:  time.Now(),
	}

	// Register the waiter BEFORE checking cache to avoid race condition
	w.waitersLock.Lock()
	w.waiters[key] = waiter
	w.waitersLock.Unlock()

	// Metrics: track registrations and current waiter count
	waitRegisterTotal.Inc()
	waitersCurrent.Inc()

	// Return a cancel function that cleans up the waiter
	cancelWithCleanup := func() {
		cancelFunc()
		w.UnregisterClaimWaiter(claimName, namespace)
	}

	// Now check if the claim already has a final state
	if existingClaim := w.getClaimFromCache(claimName, namespace); existingClaim != nil {
		if result := w.evaluateClaimStatus(existingClaim); result != nil {
			// Claim already has a final state - send result and record metrics
			outcome := ""
			if result.Granted {
				outcome = "granted"
			} else {
				// Treat any non-granted final as denied unless a specific reason is provided
				if result.Reason == "" {
					outcome = "denied"
				} else {
					outcome = "denied"
				}
			}
			waitTimeSeconds.WithLabelValues(outcome).Observe(time.Since(waiter.startTime).Seconds())

			select {
			case waiter.resultChan <- *result:
			default:
			}
			// Clean up immediately since we're done
			w.UnregisterClaimWaiter(claimName, namespace)
			return resultChan, cancelWithCleanup, nil
		}
	}

	// Only start the timeout timer if the claim is not already complete
	waiter.timer = time.AfterFunc(timeout, func() {
		w.logger.V(3).Info("Claim waiter timed out",
			"claimName", claimName,
			"namespace", namespace,
			"timeout", timeout)

		// Metrics for timeout
		waitTimeSeconds.WithLabelValues("timeout").Observe(time.Since(waiter.startTime).Seconds())

		select {
		case resultChan <- ClaimResult{
			Granted: false,
			Reason:  "timeout",
			Error:   fmt.Errorf("timeout waiting for ResourceClaim %s/%s after %v", namespace, claimName, timeout),
		}:
		default:
		}

		// Clean up the waiter
		w.UnregisterClaimWaiter(claimName, namespace)
	})

	return resultChan, cancelWithCleanup, nil
}

// UnregisterClaimWaiter unregisters a waiter for a specific ResourceClaim
func (w *sharedInformerClaimWatchManager) UnregisterClaimWaiter(claimName, namespace string) {
	key := types.NamespacedName{Namespace: namespace, Name: claimName}

	w.waitersLock.Lock()
	defer w.waitersLock.Unlock()

	if waiter, exists := w.waiters[key]; exists {
		w.logger.V(3).Info("Unregistering claim waiter",
			"claimName", claimName,
			"namespace", namespace)

		if waiter.timer != nil {
			waiter.timer.Stop()
		}
		waiter.cancelFunc()
		close(waiter.resultChan)
		delete(w.waiters, key)

		// Metrics: unregister and current waiter count
		waitUnregisterTotal.Inc()
		waitersCurrent.Dec()
	}
}

// handleClaimEvent handles ResourceClaim events from the shared informer
func (w *sharedInformerClaimWatchManager) handleClaimEvent(obj interface{}) {
	unstructuredObj, ok := obj.(*unstructured.Unstructured)
	if !ok {
		w.logger.Error(nil, "Received non-unstructured object in claim event handler")
		return
	}

	w.workqueue.Add(types.NamespacedName{
		Name:      unstructuredObj.GetName(),
		Namespace: unstructuredObj.GetNamespace(),
	})
}

// processWorkItems processes items from the workqueue
func (w *sharedInformerClaimWatchManager) processWorkItems(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		default:
			item, shutdown := w.workqueue.Get()
			if shutdown {
				return
			}

			func() {
				defer w.workqueue.Done(item)

				if err := w.processClaimEvent(ctx, item); err != nil {
					w.logger.Error(err, "Failed to process claim event", "namespace", item.Namespace, "name", item.Name)
					w.workqueue.AddRateLimited(item)
				} else {
					w.workqueue.Forget(item)
				}
			}()
		}
	}
}

// processClaimEvent processes a single ResourceClaim event
func (w *sharedInformerClaimWatchManager) processClaimEvent(ctx context.Context, key types.NamespacedName) error {
	// Check if we have a waiter for this claim
	w.waitersLock.RLock()
	waiter, exists := w.waiters[key]
	w.waitersLock.RUnlock()

	if !exists {
		// No waiter for this claim, nothing to do
		return nil
	}

	// Get the claim from the informer cache
	claim := w.getClaimFromCache(key.Name, key.Namespace)
	if claim == nil {
		// Claim was deleted or doesn't exist
		waitTimeSeconds.WithLabelValues("not_found").Observe(time.Since(waiter.startTime).Seconds())

		select {
		case waiter.resultChan <- ClaimResult{
			Granted: false,
			Reason:  "claim not found",
			Error:   fmt.Errorf("ResourceClaim %s/%s was deleted or not found", key.Namespace, key.Name),
		}:
		default:
		}
		w.UnregisterClaimWaiter(key.Name, key.Namespace)
		return nil
	}

	// Evaluate the claim status
	if result := w.evaluateClaimStatus(claim); result != nil {
		// Claim has reached a final state
		outcome := ""
		if result.Granted {
			outcome = "granted"
		} else {
			// Denial final state
			outcome = "denied"
		}
		waitTimeSeconds.WithLabelValues(outcome).Observe(time.Since(waiter.startTime).Seconds())

		select {
		case waiter.resultChan <- *result:
		default:
		}
		w.UnregisterClaimWaiter(key.Name, key.Namespace)
	}

	return nil
}

// getClaimFromCache retrieves a ResourceClaim from the informer cache
func (w *sharedInformerClaimWatchManager) getClaimFromCache(name, namespace string) *unstructured.Unstructured {
	key := fmt.Sprintf("%s/%s", namespace, name)
	item, exists, err := w.informer.GetIndexer().GetByKey(key)
	if err != nil || !exists {
		return nil
	}

	unstructuredObj, ok := item.(*unstructured.Unstructured)
	if !ok {
		return nil
	}

	return unstructuredObj
}

// evaluateClaimStatus evaluates a ResourceClaim's status and returns a result if final
func (w *sharedInformerClaimWatchManager) evaluateClaimStatus(claim *unstructured.Unstructured) *ClaimResult {
	// Extract the status from the unstructured object
	status, found, err := unstructured.NestedMap(claim.Object, "status")
	if err != nil || !found {
		// No status yet, claim is still pending
		return nil
	}

	// Check for conditions
	conditions, found, err := unstructured.NestedSlice(status, "conditions")
	if err != nil || !found {
		// No conditions yet, claim is still pending
		return nil
	}

	// Look for final conditions
	for _, conditionInterface := range conditions {
		condition, ok := conditionInterface.(map[string]interface{})
		if !ok {
			continue
		}

		conditionType, found, err := unstructured.NestedString(condition, "type")
		if err != nil || !found {
			continue
		}

		conditionStatus, found, err := unstructured.NestedString(condition, "status")
		if err != nil || !found {
			continue
		}

		reason, _, _ := unstructured.NestedString(condition, "reason")
		message, _, _ := unstructured.NestedString(condition, "message")

		// Check the Granted condition
		if conditionType == string(quotav1alpha1.ResourceClaimGranted) {
			if conditionStatus == string(metav1.ConditionTrue) {
				return &ClaimResult{
					Granted: true,
					Reason:  reason,
				}
			} else if conditionStatus == string(metav1.ConditionFalse) && reason == quotav1alpha1.ResourceClaimDeniedReason {
				return &ClaimResult{
					Granted: false,
					Reason:  reason,
					Error:   fmt.Errorf("ResourceClaim was denied: %s", message),
				}
			}
			// Other false statuses (like PendingEvaluation) are not final
		}
	}

	// No final condition found, claim is still pending
	return nil
}
