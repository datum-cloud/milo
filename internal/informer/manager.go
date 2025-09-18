package informer

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// DefaultResyncPeriod is the default resync period for informers.
const DefaultResyncPeriod = 30 * time.Second

// dynamicInformerManager implements the Manager interface.
type dynamicInformerManager struct {
	client        client.Client
	dynamicClient dynamic.Interface
	restMapper    meta.RESTMapper
	logger        logr.Logger

	// No CRD watching needed since all CRDs are pre-installed

	// Informer management
	informers    map[schema.GroupVersionKind]*managedInformer
	requirements map[string]map[schema.GroupVersionKind]WatchRequest // consumerID -> GVK -> request

	// Synchronization
	mu      sync.RWMutex
	started bool
	stopCh  chan struct{}
}

// managedInformer tracks a single informer and its consumers.
type managedInformer struct {
	gvk        schema.GroupVersionKind
	informer   cache.SharedIndexInformer
	stopCh     chan struct{}
	handlers   map[string]ResourceEventHandler // consumerID -> handler
	synced     bool
	lastSync   time.Time
	eventCount int64
	mu         sync.RWMutex
}

// NewManager creates a new dynamic informer manager.
func NewManager(client client.Client, dynamicClient dynamic.Interface, restMapper meta.RESTMapper) Manager {
	return &dynamicInformerManager{
		client:        client,
		dynamicClient: dynamicClient,
		restMapper:    restMapper,
		logger:        log.Log.WithName("dynamic-informer-manager"),
		informers:     make(map[schema.GroupVersionKind]*managedInformer),
		requirements:  make(map[string]map[schema.GroupVersionKind]WatchRequest),
	}
}

// Start implements Manager.
func (m *dynamicInformerManager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.started {
		return fmt.Errorf("manager is already started")
	}

	m.logger.Info("Starting dynamic informer manager")
	m.stopCh = make(chan struct{})

	// Create informers for any existing requirements
	// All CRDs are pre-installed in Milo, so we can create informers immediately
	m.reconcileInformers()

	m.started = true
	m.logger.Info("Dynamic informer manager started successfully")

	// Handle context cancellation
	go func() {
		<-ctx.Done()
		m.Stop()
	}()

	return nil
}

// Stop implements Manager.
func (m *dynamicInformerManager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.started {
		return nil
	}

	m.logger.Info("Stopping dynamic informer manager")

	// Stop all managed informers
	for gvk, managedInf := range m.informers {
		m.logger.V(1).Info("Stopping informer", "gvk", gvk)
		close(managedInf.stopCh)
	}

	// Stop main control loop
	close(m.stopCh)

	m.started = false
	m.logger.Info("Dynamic informer manager stopped")
	return nil
}

// AddWatch implements Manager.
func (m *dynamicInformerManager) AddWatch(ctx context.Context, req WatchRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.started {
		return fmt.Errorf("manager is not started")
	}

	m.logger.V(1).Info("Adding watch requirement",
		"gvk", req.GVK,
		"consumer", req.ConsumerID)

	// Track the requirement
	if m.requirements[req.ConsumerID] == nil {
		m.requirements[req.ConsumerID] = make(map[schema.GroupVersionKind]WatchRequest)
	}
	m.requirements[req.ConsumerID][req.GVK] = req

	// Create or update informer immediately (all CRDs are pre-installed)
	return m.ensureInformer(req.GVK)
}

// RemoveWatch implements Manager.
func (m *dynamicInformerManager) RemoveWatch(ctx context.Context, gvk schema.GroupVersionKind, consumerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.started {
		return fmt.Errorf("manager is not started")
	}

	m.logger.V(1).Info("Removing watch requirement",
		"gvk", gvk,
		"consumer", consumerID)

	// Remove the requirement
	if consumers := m.requirements[consumerID]; consumers != nil {
		delete(consumers, gvk)
		if len(consumers) == 0 {
			delete(m.requirements, consumerID)
		}
	}

	// Check if any other consumers still need this informer
	stillNeeded := false
	for _, consumerReqs := range m.requirements {
		if _, exists := consumerReqs[gvk]; exists {
			stillNeeded = true
			break
		}
	}

	// Stop informer if no longer needed
	if !stillNeeded {
		return m.stopInformer(gvk)
	}

	// Remove handler from existing informer
	if managedInf := m.informers[gvk]; managedInf != nil {
		managedInf.mu.Lock()
		delete(managedInf.handlers, consumerID)
		managedInf.mu.Unlock()
	}

	return nil
}

// GetStatus implements Manager.
func (m *dynamicInformerManager) GetStatus() ManagerStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := ManagerStatus{
		Started:         m.started,
		ActiveInformers: make(map[schema.GroupVersionKind]InformerStatus),
		PendingCRDs:     make([]schema.GroupVersionKind, 0),
		Consumers:       make(map[string][]schema.GroupVersionKind),
	}

	// Collect active informers
	for gvk, managedInf := range m.informers {
		managedInf.mu.RLock()
		consumers := make([]string, 0, len(managedInf.handlers))
		for consumerID := range managedInf.handlers {
			consumers = append(consumers, consumerID)
		}
		status.ActiveInformers[gvk] = InformerStatus{
			GVK:          gvk,
			Synced:       managedInf.synced,
			LastSync:     managedInf.lastSync,
			Consumers:    consumers,
			EventCount:   managedInf.eventCount,
			CRDAvailable: true, // All CRDs are pre-installed
		}
		managedInf.mu.RUnlock()
	}

	// No pending CRDs since all are pre-installed

	// Collect consumers
	for consumerID, consumerReqs := range m.requirements {
		gvks := make([]schema.GroupVersionKind, 0, len(consumerReqs))
		for gvk := range consumerReqs {
			gvks = append(gvks, gvk)
		}
		status.Consumers[consumerID] = gvks
	}

	return status
}

// NeedLeaderElection implements Manager.
func (m *dynamicInformerManager) NeedLeaderElection() bool {
	return true
}

// reconcileInformers creates informers for all requirements.
func (m *dynamicInformerManager) reconcileInformers() {
	for _, consumerReqs := range m.requirements {
		for gvk := range consumerReqs {
			if err := m.ensureInformer(gvk); err != nil {
				m.logger.Error(err, "Failed to create informer during reconciliation", "gvk", gvk)
			}
		}
	}
}

// ensureInformer creates an informer for the given GVK if it doesn't exist.
func (m *dynamicInformerManager) ensureInformer(gvk schema.GroupVersionKind) error {
	if managedInf := m.informers[gvk]; managedInf != nil {
		// Informer already exists, just update handlers
		m.addHandlersToInformer(managedInf, gvk)
		return nil
	}

	// Create new informer
	return m.createInformer(gvk)
}

// createInformer creates a new managed informer for the given GVK.
func (m *dynamicInformerManager) createInformer(gvk schema.GroupVersionKind) error {
	m.logger.Info("Creating informer", "gvk", gvk)

	// Convert GVK to GVR
	gvr, err := m.gvkToGVR(gvk)
	if err != nil {
		return fmt.Errorf("failed to convert GVK to GVR: %w", err)
	}

	listWatch := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			return m.dynamicClient.Resource(gvr).List(context.TODO(), options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			return m.dynamicClient.Resource(gvr).Watch(context.TODO(), options)
		},
	}

	informer := cache.NewSharedIndexInformer(
		listWatch,
		&unstructured.Unstructured{},
		DefaultResyncPeriod,
		cache.Indexers{},
	)

	managedInf := &managedInformer{
		gvk:      gvk,
		informer: informer,
		stopCh:   make(chan struct{}),
		handlers: make(map[string]ResourceEventHandler),
	}

	// Add event handler that dispatches to all consumers
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			managedInf.mu.Lock()
			managedInf.eventCount++
			handlers := make([]ResourceEventHandler, 0, len(managedInf.handlers))
			for _, handler := range managedInf.handlers {
				handlers = append(handlers, handler)
			}
			managedInf.mu.Unlock()

			unstrObj := obj.(*unstructured.Unstructured)
			for _, handler := range handlers {
				handler.OnAdd(unstrObj)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			managedInf.mu.Lock()
			managedInf.eventCount++
			handlers := make([]ResourceEventHandler, 0, len(managedInf.handlers))
			for _, handler := range managedInf.handlers {
				handlers = append(handlers, handler)
			}
			managedInf.mu.Unlock()

			oldUnstr := oldObj.(*unstructured.Unstructured)
			newUnstr := newObj.(*unstructured.Unstructured)
			for _, handler := range handlers {
				handler.OnUpdate(oldUnstr, newUnstr)
			}
		},
		DeleteFunc: func(obj interface{}) {
			managedInf.mu.Lock()
			managedInf.eventCount++
			handlers := make([]ResourceEventHandler, 0, len(managedInf.handlers))
			for _, handler := range managedInf.handlers {
				handlers = append(handlers, handler)
			}
			managedInf.mu.Unlock()

			unstrObj := obj.(*unstructured.Unstructured)
			for _, handler := range handlers {
				handler.OnDelete(unstrObj)
			}
		},
	})

	m.informers[gvk] = managedInf
	m.addHandlersToInformer(managedInf, gvk)

	// Start the informer
	go informer.Run(managedInf.stopCh)

	// Wait for sync in background
	go func() {
		if cache.WaitForCacheSync(managedInf.stopCh, informer.HasSynced) {
			managedInf.mu.Lock()
			managedInf.synced = true
			managedInf.lastSync = time.Now()
			managedInf.mu.Unlock()
			m.logger.V(1).Info("Informer synced", "gvk", gvk)
		}
	}()

	return nil
}

// addHandlersToInformer adds event handlers from all consumers that need this GVK.
func (m *dynamicInformerManager) addHandlersToInformer(managedInf *managedInformer, gvk schema.GroupVersionKind) {
	managedInf.mu.Lock()
	defer managedInf.mu.Unlock()

	// Add handlers from all consumers that need this GVK
	for consumerID, consumerReqs := range m.requirements {
		if req, exists := consumerReqs[gvk]; exists {
			managedInf.handlers[consumerID] = req.Handler
			m.logger.V(2).Info("Added handler to informer",
				"gvk", gvk,
				"consumer", consumerID)
		}
	}
}

// stopInformer stops the informer for the given GVK.
func (m *dynamicInformerManager) stopInformer(gvk schema.GroupVersionKind) error {
	managedInf := m.informers[gvk]
	if managedInf == nil {
		return nil // Nothing to stop
	}

	m.logger.Info("Stopping informer", "gvk", gvk)

	// Stop the informer
	close(managedInf.stopCh)

	// Remove from tracking
	delete(m.informers, gvk)

	return nil
}

// gvkToGVR converts a GroupVersionKind to a GroupVersionResource using discovery.
func (m *dynamicInformerManager) gvkToGVR(gvk schema.GroupVersionKind) (schema.GroupVersionResource, error) {
	mapping, err := m.restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return schema.GroupVersionResource{}, err
	}
	return mapping.Resource, nil
}
