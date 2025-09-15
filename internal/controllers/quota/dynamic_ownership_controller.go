// DynamicOwnershipController provides immediate ownership reference management through dynamic watches.
//
// This controller dynamically manages watches for claiming resource types based on ResourceRegistrations.
// When a claiming resource is created, it immediately finds related ResourceClaims and adds owner references.
// This provides instant ownership tracking without waiting for periodic reconciliation.
package quota

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	quotav1alpha1 "go.miloapis.com/milo/pkg/apis/quota/v1alpha1"
)

// DynamicOwnershipController manages immediate ownership references through dynamic watches
type DynamicOwnershipController struct {
	client.Client
	DynamicClient   dynamic.Interface
	DiscoveryClient discovery.DiscoveryInterface
	Scheme          *runtime.Scheme
	logger          logr.Logger

	// Dynamic watch management
	watchManager    *DynamicWatchManager
	informerFactory dynamicinformer.DynamicSharedInformerFactory
	stopCh          chan struct{}
	watchesMutex    sync.RWMutex
	activeWatches   map[schema.GroupVersionResource]*watchInfo
}

// watchInfo tracks information about an active watch
type watchInfo struct {
	gvr      schema.GroupVersionResource
	gvk      schema.GroupVersionKind
	informer cache.SharedIndexInformer
	stopFunc func()
}

// DynamicWatchManager handles the lifecycle of dynamic watches
type DynamicWatchManager struct {
	client          client.Client
	dynamicClient   dynamic.Interface
	discoveryClient discovery.DiscoveryInterface
	logger          logr.Logger
	versionCache    sync.Map // cache for discovered versions
}

// NewDynamicOwnershipController creates a new dynamic ownership controller
func NewDynamicOwnershipController(
	client client.Client,
	dynamicClient dynamic.Interface,
	discoveryClient discovery.DiscoveryInterface,
	scheme *runtime.Scheme,
) *DynamicOwnershipController {
	logger := ctrl.Log.WithName("dynamic-ownership")

	controller := &DynamicOwnershipController{
		Client:          client,
		DynamicClient:   dynamicClient,
		DiscoveryClient: discoveryClient,
		Scheme:          scheme,
		logger:          logger,
		stopCh:          make(chan struct{}),
		activeWatches:   make(map[schema.GroupVersionResource]*watchInfo),
		watchManager: &DynamicWatchManager{
			client:          client,
			dynamicClient:   dynamicClient,
			discoveryClient: discoveryClient,
			logger:          logger,
		},
	}

	// Create dynamic informer factory with a reasonable resync period
	controller.informerFactory = dynamicinformer.NewDynamicSharedInformerFactory(
		dynamicClient,
		30*time.Second, // Resync period
	)

	return controller
}

// +kubebuilder:rbac:groups=quota.miloapis.com,resources=resourceclaims,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=quota.miloapis.com,resources=resourceregistrations,verbs=get;list;watch
// +kubebuilder:rbac:groups=*,resources=*,verbs=get;list;watch

// Reconcile handles ResourceRegistration changes to update dynamic watches
func (r *DynamicOwnershipController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithName("dynamic-ownership")
	r.logger = logger

	// Get the ResourceRegistration
	var registration quotav1alpha1.ResourceRegistration
	if err := r.Get(ctx, req.NamespacedName, &registration); err != nil {
		if apierrors.IsNotFound(err) {
			// Registration was deleted - update watches
			logger.Info("ResourceRegistration deleted, updating dynamic watches", "registration", req.Name)
			if err := r.updateDynamicWatches(ctx); err != nil {
				logger.Error(err, "Failed to update dynamic watches after registration deletion")
				return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to get ResourceRegistration: %w", err)
	}

	logger.V(1).Info("Processing ResourceRegistration change",
		"registration", registration.Name,
		"claimingResources", len(registration.Spec.ClaimingResources))

	// Update dynamic watches based on current registrations
	if err := r.updateDynamicWatches(ctx); err != nil {
		logger.Error(err, "Failed to update dynamic watches")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	return ctrl.Result{}, nil
}

// updateDynamicWatches updates the set of dynamic watches based on current ResourceRegistrations
func (r *DynamicOwnershipController) updateDynamicWatches(ctx context.Context) error {
	r.logger.V(1).Info("Updating dynamic watches")

	// Get all ResourceRegistrations
	var registrations quotav1alpha1.ResourceRegistrationList
	if err := r.List(ctx, &registrations); err != nil {
		return fmt.Errorf("failed to list ResourceRegistrations: %w", err)
	}

	// Build set of required GVRs from ClaimingResources
	requiredGVRs := make(map[schema.GroupVersionResource]schema.GroupVersionKind)

	for _, registration := range registrations.Items {
		for _, claimingResource := range registration.Spec.ClaimingResources {
			// Discover the version and resource name for this claiming resource
			gvr, gvk, err := r.watchManager.discoverGVR(claimingResource.APIGroup, claimingResource.Kind)
			if err != nil {
				r.logger.Error(err, "Failed to discover GVR for claiming resource",
					"apiGroup", claimingResource.APIGroup,
					"kind", claimingResource.Kind)
				continue
			}

			requiredGVRs[gvr] = gvk
			r.logger.V(2).Info("Required watch for claiming resource",
				"gvr", gvr.String(),
				"gvk", gvk.String())
		}
	}

	r.watchesMutex.Lock()
	defer r.watchesMutex.Unlock()

	// Remove watches that are no longer needed
	for gvr, watchInfo := range r.activeWatches {
		if _, required := requiredGVRs[gvr]; !required {
			r.logger.Info("Removing dynamic watch", "gvr", gvr.String())
			watchInfo.stopFunc()
			delete(r.activeWatches, gvr)
		}
	}

	// Add new required watches
	for gvr, gvk := range requiredGVRs {
		if _, exists := r.activeWatches[gvr]; !exists {
			r.logger.Info("Adding dynamic watch", "gvr", gvr.String())
			if err := r.addDynamicWatch(gvr, gvk); err != nil {
				r.logger.Error(err, "Failed to add dynamic watch", "gvr", gvr.String())
				continue
			}
		}
	}

	r.logger.Info("Dynamic watches updated",
		"activeWatches", len(r.activeWatches),
		"requiredWatches", len(requiredGVRs))

	return nil
}

// addDynamicWatch adds a new dynamic watch for the specified GVR
func (r *DynamicOwnershipController) addDynamicWatch(gvr schema.GroupVersionResource, gvk schema.GroupVersionKind) error {
	// Create informer for this GVR
	informer := r.informerFactory.ForResource(gvr).Informer()

	// Add event handlers
	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			r.handleClaimingResourceEvent("ADD", obj, gvr, gvk)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			r.handleClaimingResourceEvent("UPDATE", newObj, gvr, gvk)
		},
		DeleteFunc: func(obj interface{}) {
			r.handleClaimingResourceEvent("DELETE", obj, gvr, gvk)
		},
	})
	if err != nil {
		return fmt.Errorf("failed to add event handler: %w", err)
	}

	// Create stop channel for this specific watch
	stopCh := make(chan struct{})

	// Start the informer
	go informer.Run(stopCh)

	// Wait for initial sync
	go func() {
		if !cache.WaitForCacheSync(stopCh, informer.HasSynced) {
			r.logger.Error(fmt.Errorf("failed to sync cache"), "Failed to sync cache for GVR", "gvr", gvr.String())
		} else {
			r.logger.V(1).Info("Cache synced for dynamic watch", "gvr", gvr.String())
		}
	}()

	// Store watch info
	r.activeWatches[gvr] = &watchInfo{
		gvr:      gvr,
		gvk:      gvk,
		informer: informer,
		stopFunc: func() { close(stopCh) },
	}

	return nil
}

// handleClaimingResourceEvent handles events from claiming resources
func (r *DynamicOwnershipController) handleClaimingResourceEvent(eventType string, obj interface{}, gvr schema.GroupVersionResource, gvk schema.GroupVersionKind) {
	unstructuredObj, ok := obj.(*unstructured.Unstructured)
	if !ok {
		r.logger.Error(fmt.Errorf("unexpected object type"), "Expected unstructured.Unstructured",
			"actualType", fmt.Sprintf("%T", obj),
			"gvr", gvr.String())
		return
	}

	// Only process ADD and UPDATE events (not DELETE)
	if eventType == "DELETE" {
		return
	}

	r.logger.V(1).Info("Claiming resource event received",
		"eventType", eventType,
		"gvk", gvk.String(),
		"name", unstructuredObj.GetName(),
		"namespace", unstructuredObj.GetNamespace())

	// Process ownership for this claiming resource
	ctx := context.Background()
	if err := r.processClaimingResourceForOwnership(ctx, unstructuredObj, gvk); err != nil {
		r.logger.Error(err, "Failed to process claiming resource for ownership",
			"gvk", gvk.String(),
			"name", unstructuredObj.GetName(),
			"namespace", unstructuredObj.GetNamespace())
	}
}

// processClaimingResourceForOwnership finds ResourceClaims that reference this claiming resource and adds owner references
func (r *DynamicOwnershipController) processClaimingResourceForOwnership(ctx context.Context, claimingObj *unstructured.Unstructured, gvk schema.GroupVersionKind) error {
	// Determine the namespace to search for ResourceClaims
	searchNamespace := claimingObj.GetNamespace()
	if searchNamespace == "" {
		// For cluster-scoped resources, we need to search all namespaces
		// We'll list all ResourceClaims and filter them
		searchNamespace = ""
	}

	// List ResourceClaims
	var claims quotav1alpha1.ResourceClaimList
	listOptions := []client.ListOption{}
	if searchNamespace != "" {
		listOptions = append(listOptions, client.InNamespace(searchNamespace))
	}

	if err := r.List(ctx, &claims, listOptions...); err != nil {
		return fmt.Errorf("failed to list ResourceClaims: %w", err)
	}

	r.logger.V(2).Info("Searching for ResourceClaims that reference this claiming resource",
		"claimingResource", fmt.Sprintf("%s/%s", gvk.Kind, claimingObj.GetName()),
		"totalClaims", len(claims.Items))

	// Process each claim to see if it references this claiming resource
	processed := 0
	for i := range claims.Items {
		claim := &claims.Items[i]

		// Skip claims that already have owner references
		if len(claim.GetOwnerReferences()) > 0 {
			continue
		}

		// Check if this claim references the claiming resource
		if r.claimReferencesClaimingResource(claim, claimingObj, gvk) {
			r.logger.Info("Found ResourceClaim that references claiming resource, adding owner reference",
				"claim", claim.Name,
				"claimNamespace", claim.Namespace,
				"claimingResource", fmt.Sprintf("%s/%s", gvk.Kind, claimingObj.GetName()))

			if err := r.addOwnerReference(ctx, claim, claimingObj); err != nil {
				r.logger.Error(err, "Failed to add owner reference",
					"claim", claim.Name,
					"claimingResource", claimingObj.GetName())
				// Continue processing other claims
			} else {
				processed++
			}
		}
	}

	if processed > 0 {
		r.logger.Info("Successfully processed claiming resource for ownership",
			"claimingResource", fmt.Sprintf("%s/%s", gvk.Kind, claimingObj.GetName()),
			"processedClaims", processed)
	}

	return nil
}

// claimReferencesClaimingResource checks if a ResourceClaim references the given claiming resource
func (r *DynamicOwnershipController) claimReferencesClaimingResource(claim *quotav1alpha1.ResourceClaim, claimingObj *unstructured.Unstructured, gvk schema.GroupVersionKind) bool {
	resourceRef := claim.Spec.ResourceRef

	// Check if the ResourceRef matches the claiming resource
	return resourceRef.APIGroup == gvk.Group &&
		strings.EqualFold(resourceRef.Kind, gvk.Kind) &&
		resourceRef.Name == claimingObj.GetName() &&
		(resourceRef.Namespace == claimingObj.GetNamespace() ||
			(resourceRef.Namespace == "" && claimingObj.GetNamespace() == "") ||
			(resourceRef.Namespace == "" && claim.Namespace == claimingObj.GetNamespace()))
}

// addOwnerReference adds an owner reference to a ResourceClaim
func (r *DynamicOwnershipController) addOwnerReference(ctx context.Context, claim *quotav1alpha1.ResourceClaim, owner *unstructured.Unstructured) error {
	// Create owner reference
	ownerRef := metav1.OwnerReference{
		APIVersion: owner.GetAPIVersion(),
		Kind:       owner.GetKind(),
		Name:       owner.GetName(),
		UID:        owner.GetUID(),
		// Set controller to false to avoid conflicts with other controllers
		Controller: func() *bool { b := false; return &b }(),
		// Block deletion to ensure proper cleanup order
		BlockOwnerDeletion: func() *bool { b := true; return &b }(),
	}

	// Create a copy of the claim for updating
	updatedClaim := claim.DeepCopy()
	updatedClaim.SetOwnerReferences([]metav1.OwnerReference{ownerRef})

	// Update the claim with the new owner reference
	if err := r.Update(ctx, updatedClaim); err != nil {
		return fmt.Errorf("failed to update ResourceClaim with owner reference: %w", err)
	}

	r.logger.Info("Successfully added owner reference to ResourceClaim",
		"claim", claim.Name,
		"owner", owner.GetName(),
		"ownerKind", owner.GetKind(),
		"ownerUID", owner.GetUID())

	return nil
}

// discoverGVR discovers the GroupVersionResource for a given API group and kind
func (m *DynamicWatchManager) discoverGVR(apiGroup, kind string) (schema.GroupVersionResource, schema.GroupVersionKind, error) {
	// Check cache first
	cacheKey := fmt.Sprintf("%s/%s", apiGroup, kind)
	if cached, ok := m.versionCache.Load(cacheKey); ok {
		gvr := cached.(schema.GroupVersionResource)
		gvk := schema.GroupVersionKind{
			Group:   gvr.Group,
			Version: gvr.Version,
			Kind:    kind,
		}
		return gvr, gvk, nil
	}

	// Use discovery to find the correct version and resource name
	version, err := m.discoverVersion(apiGroup, kind)
	if err != nil {
		return schema.GroupVersionResource{}, schema.GroupVersionKind{}, err
	}

	// Convert kind to resource name (simple pluralization)
	resource := m.kindToResource(kind)

	gvr := schema.GroupVersionResource{
		Group:    apiGroup,
		Version:  version,
		Resource: resource,
	}

	gvk := schema.GroupVersionKind{
		Group:   apiGroup,
		Version: version,
		Kind:    kind,
	}

	// Cache the result
	m.versionCache.Store(cacheKey, gvr)

	return gvr, gvk, nil
}

// discoverVersion discovers the correct API version for a given group and kind
func (m *DynamicWatchManager) discoverVersion(apiGroup, kind string) (string, error) {
	// If discovery client is not available, fall back to defaults
	if m.discoveryClient == nil {
		if apiGroup == "" {
			return "v1", nil // Core API group
		}
		return "v1alpha1", nil // Default for custom resources
	}

	// Use discovery to find the preferred version
	apiGroupList, err := m.discoveryClient.ServerGroups()
	if err != nil {
		m.logger.Error(err, "Failed to discover server groups, using defaults",
			"apiGroup", apiGroup, "kind", kind)
		// Fall back to defaults on discovery failure
		if apiGroup == "" {
			return "v1", nil
		}
		return "v1alpha1", nil
	}

	// Find the group in the list
	for _, group := range apiGroupList.Groups {
		if group.Name == apiGroup {
			// Use the preferred version
			if group.PreferredVersion.Version != "" {
				return group.PreferredVersion.Version, nil
			}
			// If no preferred version, use the first available version
			if len(group.Versions) > 0 {
				return group.Versions[0].Version, nil
			}
		}
	}

	// Special handling for core API group (empty string)
	if apiGroup == "" {
		return "v1", nil
	}

	// If group not found, use a reasonable default
	m.logger.Info("API group not found in discovery, using default version",
		"apiGroup", apiGroup,
		"kind", kind,
		"defaultVersion", "v1alpha1")
	return "v1alpha1", nil
}

// kindToResource converts a Kind to its corresponding resource name using simple pluralization
func (m *DynamicWatchManager) kindToResource(kind string) string {
	lower := strings.ToLower(kind)
	if strings.HasSuffix(lower, "s") {
		return lower + "es"
	}
	if strings.HasSuffix(lower, "y") {
		return strings.TrimSuffix(lower, "y") + "ies"
	}
	return lower + "s"
}

// Start starts the dynamic ownership controller
func (r *DynamicOwnershipController) Start(ctx context.Context) error {
	r.logger.Info("Starting dynamic ownership controller")

	// Start the informer factory
	r.informerFactory.Start(r.stopCh)

	// Initial setup of dynamic watches
	if err := r.updateDynamicWatches(ctx); err != nil {
		r.logger.Error(err, "Failed to setup initial dynamic watches")
		return err
	}

	// Wait for context cancellation
	<-ctx.Done()

	r.logger.Info("Stopping dynamic ownership controller")
	close(r.stopCh)

	return nil
}

// SetupWithManager sets up the controller with the Manager
func (r *DynamicOwnershipController) SetupWithManager(mgr ctrl.Manager) error {
	// Watch ResourceRegistrations to update dynamic watches
	controller := ctrl.NewControllerManagedBy(mgr).
		For(&quotav1alpha1.ResourceRegistration{}).
		Named("dynamic-ownership-controller")

	// Add the controller as a runnable to start the dynamic watch system
	if err := mgr.Add(r); err != nil {
		return fmt.Errorf("failed to add dynamic ownership controller as runnable: %w", err)
	}

	return controller.Complete(r)
}
