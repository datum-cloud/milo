package quota

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	quotav1alpha1 "go.miloapis.com/milo/pkg/apis/quota/v1alpha1"
)

// GrantCreationController watches trigger resources and creates grants based on active policies.
type GrantCreationController struct {
	client.Client
	Scheme                *runtime.Scheme
	PolicyEngine          *PolicyEngine
	TemplateEngine        *TemplateEngine
	ParentContextResolver *ParentContextResolver
	EventRecorder         record.EventRecorder

	// Dynamic watching management (following DynamicOwnershipController pattern)
	DynamicClient   dynamic.Interface
	DiscoveryClient discovery.DiscoveryInterface
	logger          logr.Logger
	informerFactory dynamicinformer.DynamicSharedInformerFactory
	stopCh          chan struct{}
	watchesMutex    sync.RWMutex
	activeWatches   map[schema.GroupVersionResource]*grantWatchInfo
	watchManager    *GrantWatchManager
}

// grantWatchInfo tracks information about an active watch for grant creation
type grantWatchInfo struct {
	gvr      schema.GroupVersionResource
	gvk      schema.GroupVersionKind
	informer cache.SharedIndexInformer
	stopFunc func()
}

// GrantWatchManager handles the lifecycle of dynamic watches for grant creation
type GrantWatchManager struct {
	client          client.Client
	dynamicClient   dynamic.Interface
	discoveryClient discovery.DiscoveryInterface
	logger          logr.Logger
	versionCache    sync.Map // cache for discovered versions
}

// NewGrantCreationController creates a new GrantCreationController.
func NewGrantCreationController(
	client client.Client,
	scheme *runtime.Scheme,
	policyEngine *PolicyEngine,
	templateEngine *TemplateEngine,
	parentContextResolver *ParentContextResolver,
	eventRecorder record.EventRecorder,
	dynamicClient dynamic.Interface,
	discoveryClient discovery.DiscoveryInterface,
) *GrantCreationController {
	logger := ctrl.Log.WithName("grant-creation")

	controller := &GrantCreationController{
		Client:                client,
		Scheme:                scheme,
		PolicyEngine:          policyEngine,
		TemplateEngine:        templateEngine,
		ParentContextResolver: parentContextResolver,
		EventRecorder:         eventRecorder,
		DynamicClient:         dynamicClient,
		DiscoveryClient:       discoveryClient,
		logger:                logger,
		stopCh:                make(chan struct{}),
		activeWatches:         make(map[schema.GroupVersionResource]*grantWatchInfo),
		watchManager: &GrantWatchManager{
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

// +kubebuilder:rbac:groups=*,resources=*,verbs=get;list;watch
// +kubebuilder:rbac:groups=quota.miloapis.com,resources=resourcegrants,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=quota.miloapis.com,resources=grantcreationpolicies,verbs=get;list;watch

// Reconcile processes GrantCreationPolicy changes.
func (r *GrantCreationController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Check if policy engine is ready
	if !r.PolicyEngine.IsStarted() {
		logger.V(2).Info("Policy engine not yet started, requeuing")
		return ctrl.Result{RequeueAfter: time.Second * 5}, nil
	}

	// This controller only watches GrantCreationPolicy resources
	return r.ReconcilePolicy(ctx, req)
}

// ReconcileTriggerResource processes trigger resources and creates/updates/deletes grants as needed.
func (r *GrantCreationController) ReconcileTriggerResource(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Check if policy engine is ready
	if !r.PolicyEngine.IsStarted() {
		logger.V(2).Info("Policy engine not yet started, requeuing")
		return ctrl.Result{RequeueAfter: time.Second * 5}, nil
	}

	// The request doesn't contain GVK information, so we need to determine it
	// by checking which policies are active and attempting to fetch the resource
	// for each possible trigger resource type.
	activeGVKs := r.PolicyEngine.GetActiveResourceTypes()

	var triggerObj *unstructured.Unstructured
	var matchingGVK schema.GroupVersionKind
	var found bool

	// Try to fetch the resource using each active GVK
	for _, gvk := range activeGVKs {
		obj := &unstructured.Unstructured{}
		obj.SetGroupVersionKind(gvk)

		if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
			if apierrors.IsNotFound(err) {
				continue // Try next GVK
			}
			logger.Error(err, "Failed to fetch resource", "gvk", gvk)
			continue
		}

		// Found the resource
		triggerObj = obj
		matchingGVK = gvk
		found = true
		break
	}

	if !found {
		// Resource might have been deleted or we don't have policies for it
		logger.V(2).Info("No matching trigger resource found", "request", req)
		return ctrl.Result{}, nil
	}

	logger = logger.WithValues(
		"triggerResource", triggerObj.GetName(),
		"triggerKind", matchingGVK.Kind,
		"namespace", triggerObj.GetNamespace(),
	)

	// Get policies for this resource type
	policies := r.PolicyEngine.GetPoliciesForResource(matchingGVK)
	if len(policies) == 0 {
		logger.V(2).Info("No active policies for resource type")
		return ctrl.Result{}, nil
	}

	logger.Info("Processing trigger resource", "activePolicies", len(policies))

	// Process each applicable policy
	for _, policy := range policies {
		if err := r.processPolicy(ctx, policy, triggerObj); err != nil {
			logger.Error(err, "Failed to process policy", "policy", policy.Name)
			r.EventRecorder.Eventf(triggerObj, "Warning", "PolicyProcessingFailed",
				"Failed to process grant creation policy %s: %v", policy.Name, err)
			// Continue with other policies even if one fails
		}
	}

	return ctrl.Result{}, nil
}

// ReconcilePolicy handles GrantCreationPolicy changes.
func (r *GrantCreationController) ReconcilePolicy(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("policyName", req.Name)

	// Fetch the policy
	var policy quotav1alpha1.GrantCreationPolicy
	if err := r.Get(ctx, req.NamespacedName, &policy); err != nil {
		if apierrors.IsNotFound(err) {
			// Policy was deleted - update the policy engine
			logger.Info("Policy deleted, updating policy engine")
			if err := r.PolicyEngine.UpdatePolicy(ctx, req.Name); err != nil {
				logger.Error(err, "Failed to update policy engine for deleted policy")
			}
			// Update dynamic watches after policy changes
			if err := r.updateDynamicWatches(ctx); err != nil {
				logger.Error(err, "Failed to update dynamic watches after policy deletion")
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Update the policy in the policy engine
	if err := r.PolicyEngine.UpdatePolicy(ctx, policy.Name); err != nil {
		logger.Error(err, "Failed to update policy in policy engine")
		return ctrl.Result{RequeueAfter: time.Second * 10}, nil
	}

	// Update dynamic watches after policy changes
	if err := r.updateDynamicWatches(ctx); err != nil {
		logger.Error(err, "Failed to update dynamic watches after policy update")
		return ctrl.Result{RequeueAfter: time.Second * 10}, nil
	}

	logger.Info("Successfully processed policy change")
	return ctrl.Result{}, nil
}

// processPolicy processes a single policy against a trigger resource.
func (r *GrantCreationController) processPolicy(
	ctx context.Context,
	policy *quotav1alpha1.GrantCreationPolicy,
	triggerObj *unstructured.Unstructured,
) error {
	logger := log.FromContext(ctx).WithValues("policy", policy.Name)

	// Evaluate trigger conditions
	conditionsMet, err := r.TemplateEngine.EvaluateConditions(policy.Spec.Trigger.Conditions, triggerObj)
	if err != nil {
		return fmt.Errorf("failed to evaluate conditions: %w", err)
	}

	if !conditionsMet {
		logger.V(2).Info("Trigger conditions not met, skipping grant creation")
		// Check if there's an existing grant that should be cleaned up
		return r.cleanupGrant(ctx, policy, triggerObj)
	}

	logger.Info("Trigger conditions met, creating/updating grant")

	// Determine target client (same cluster or cross-cluster)
	targetClient, targetNamespace, err := r.resolveTargetClient(ctx, policy, triggerObj)
	if err != nil {
		return fmt.Errorf("failed to resolve target client: %w", err)
	}

	// Render the grant
	grant, err := r.TemplateEngine.RenderGrant(policy, triggerObj, targetNamespace)
	if err != nil {
		return fmt.Errorf("failed to render grant: %w", err)
	}

	// Create or update the grant
	if err := r.createOrUpdateGrant(ctx, targetClient, grant, policy, triggerObj); err != nil {
		return fmt.Errorf("failed to create/update grant: %w", err)
	}

	logger.Info("Successfully processed policy", "grantName", grant.Name, "grantNamespace", grant.Namespace)
	return nil
}

// resolveTargetClient determines the target client and namespace for grant creation.
func (r *GrantCreationController) resolveTargetClient(
	ctx context.Context,
	policy *quotav1alpha1.GrantCreationPolicy,
	triggerObj *unstructured.Unstructured,
) (client.Client, string, error) {
	// If no parent context is specified, use the current client
	if policy.Spec.Target.ParentContext == nil {
		namespace := policy.Spec.Target.ResourceGrantTemplate.Metadata.Namespace
		if namespace == "" {
			namespace = quotav1alpha1.MiloSystemNamespace
		}
		return r.Client, namespace, nil
	}

	// Resolve parent context name
	parentContextName, err := r.TemplateEngine.EvaluateParentContextName(
		policy.Spec.Target.ParentContext.NameExpression,
		triggerObj,
	)
	if err != nil {
		return nil, "", fmt.Errorf("failed to evaluate parent context name: %w", err)
	}

	// Get client for parent context
	parentContext := policy.Spec.Target.ParentContext
	targetClient, err := r.ParentContextResolver.ResolveClient(ctx, &ParentContextSpec{
		APIGroup:       parentContext.APIGroup,
		Kind:           parentContext.Kind,
		NameExpression: parentContextName, // Use resolved name directly
	}, triggerObj)
	if err != nil {
		return nil, "", fmt.Errorf("failed to resolve parent context client: %w", err)
	}

	namespace := policy.Spec.Target.ResourceGrantTemplate.Metadata.Namespace
	if namespace == "" {
		namespace = quotav1alpha1.MiloSystemNamespace
	}

	return targetClient, namespace, nil
}

// createOrUpdateGrant creates or updates a ResourceGrant.
func (r *GrantCreationController) createOrUpdateGrant(
	ctx context.Context,
	targetClient client.Client,
	grant *quotav1alpha1.ResourceGrant,
	policy *quotav1alpha1.GrantCreationPolicy,
	triggerObj *unstructured.Unstructured,
) error {
	logger := log.FromContext(ctx).WithValues("grantName", grant.Name, "grantNamespace", grant.Namespace)

	// Check if grant already exists
	existingGrant := &quotav1alpha1.ResourceGrant{}
	err := targetClient.Get(ctx, client.ObjectKey{
		Name:      grant.Name,
		Namespace: grant.Namespace,
	}, existingGrant)

	if err != nil {
		if apierrors.IsNotFound(err) {
			// Create new grant
			logger.Info("Creating new ResourceGrant")
			if err := targetClient.Create(ctx, grant); err != nil {
				return fmt.Errorf("failed to create grant: %w", err)
			}

			r.EventRecorder.Eventf(triggerObj, "Normal", "GrantCreated",
				"Created ResourceGrant %s/%s from policy %s", grant.Namespace, grant.Name, policy.Name)

			// Update policy statistics
			r.updatePolicyStatistics(ctx, policy, true)
			return nil
		}
		return fmt.Errorf("failed to check existing grant: %w", err)
	}

	// Update existing grant if needed
	logger.Info("Updating existing ResourceGrant")
	existingGrant.Spec = grant.Spec
	existingGrant.Labels = grant.Labels
	existingGrant.Annotations = grant.Annotations

	if err := targetClient.Update(ctx, existingGrant); err != nil {
		return fmt.Errorf("failed to update grant: %w", err)
	}

	r.EventRecorder.Eventf(triggerObj, "Normal", "GrantUpdated",
		"Updated ResourceGrant %s/%s from policy %s", grant.Namespace, grant.Name, policy.Name)

	return nil
}

// cleanupGrant removes a grant if conditions are no longer met.
func (r *GrantCreationController) cleanupGrant(
	ctx context.Context,
	policy *quotav1alpha1.GrantCreationPolicy,
	triggerObj *unstructured.Unstructured,
) error {
	logger := log.FromContext(ctx).WithValues("policy", policy.Name)

	// Generate the grant name to check for cleanup
	grantName, err := r.TemplateEngine.GenerateGrantName(policy, triggerObj)
	if err != nil {
		return fmt.Errorf("failed to generate grant name: %w", err)
	}

	// Determine target client
	targetClient, targetNamespace, err := r.resolveTargetClient(ctx, policy, triggerObj)
	if err != nil {
		return fmt.Errorf("failed to resolve target client: %w", err)
	}

	// Check if grant exists
	grant := &quotav1alpha1.ResourceGrant{}
	err = targetClient.Get(ctx, client.ObjectKey{
		Name:      grantName,
		Namespace: targetNamespace,
	}, grant)

	if err != nil {
		if apierrors.IsNotFound(err) {
			// Grant doesn't exist, nothing to clean up
			return nil
		}
		return fmt.Errorf("failed to check existing grant: %w", err)
	}

	// Check if this grant was created by our policy
	if grant.Labels["quota.miloapis.com/policy"] == policy.Name {
		logger.Info("Cleaning up grant due to unmet conditions", "grantName", grantName)

		if err := targetClient.Delete(ctx, grant); err != nil {
			return fmt.Errorf("failed to delete grant: %w", err)
		}

		r.EventRecorder.Eventf(triggerObj, "Normal", "GrantDeleted",
			"Deleted ResourceGrant %s/%s due to unmet conditions", grant.Namespace, grant.Name)
	}

	return nil
}

// updatePolicyStatistics updates policy status with creation statistics.
func (r *GrantCreationController) updatePolicyStatistics(
	ctx context.Context,
	policy *quotav1alpha1.GrantCreationPolicy,
	increment bool,
) {
	// This would require updating the policy status, but since we don't want to
	// conflict with the policy controller, we'll just log for now
	logger := log.FromContext(ctx)
	if increment {
		logger.V(1).Info("Grant created for policy", "policy", policy.Name)
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *GrantCreationController) SetupWithManager(mgr ctrl.Manager) error {
	r.logger.Info("Setting up GrantCreationController")

	// Start the policy engine immediately
	ctx := ctrl.SetupSignalHandler()
	if err := r.PolicyEngine.Start(ctx); err != nil {
		return fmt.Errorf("failed to start policy engine during setup: %w", err)
	}
	r.logger.Info("Policy engine started successfully during setup")

	// Start the informer factory
	r.informerFactory.Start(r.stopCh)

	// Watch GrantCreationPolicies to update dynamic watches when policies change
	controller := ctrl.NewControllerManagedBy(mgr).
		For(&quotav1alpha1.GrantCreationPolicy{}).
		Named("grant-creation-controller")

	r.logger.Info("GrantCreationController setup completed successfully")

	return controller.Complete(r)
}

// updateDynamicWatches updates the set of dynamic watches based on current GrantCreationPolicies
func (r *GrantCreationController) updateDynamicWatches(ctx context.Context) error {
	r.logger.V(1).Info("Updating dynamic watches for grant creation")

	// Get all active resource types from policy engine
	activeGVKs := r.PolicyEngine.GetActiveResourceTypes()
	r.logger.Info("Retrieved active resource types from PolicyEngine", "activeGVKs", activeGVKs, "count", len(activeGVKs))

	// Build set of required GVRs from active trigger resource types
	requiredGVRs := make(map[schema.GroupVersionResource]schema.GroupVersionKind)

	for _, gvk := range activeGVKs {
		// Discover the version and resource name for this trigger resource
		gvr, discoveredGVK, err := r.watchManager.discoverGVR(gvk.Group, gvk.Kind)
		if err != nil {
			r.logger.Error(err, "Failed to discover GVR for trigger resource",
				"apiGroup", gvk.Group,
				"kind", gvk.Kind)
			continue
		}

		requiredGVRs[gvr] = discoveredGVK
		r.logger.V(2).Info("Required watch for trigger resource",
			"gvr", gvr.String(),
			"gvk", discoveredGVK.String())
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
func (r *GrantCreationController) addDynamicWatch(gvr schema.GroupVersionResource, gvk schema.GroupVersionKind) error {
	// Create informer for this GVR
	informer := r.informerFactory.ForResource(gvr).Informer()

	// Add event handlers
	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			r.handleTriggerResourceEvent("ADD", obj, gvr, gvk)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			r.handleTriggerResourceEvent("UPDATE", newObj, gvr, gvk)
		},
		DeleteFunc: func(obj interface{}) {
			r.handleTriggerResourceEvent("DELETE", obj, gvr, gvk)
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
	r.activeWatches[gvr] = &grantWatchInfo{
		gvr:      gvr,
		gvk:      gvk,
		informer: informer,
		stopFunc: func() { close(stopCh) },
	}

	return nil
}

// handleTriggerResourceEvent handles events from trigger resources and creates grants
func (r *GrantCreationController) handleTriggerResourceEvent(eventType string, obj interface{}, gvr schema.GroupVersionResource, gvk schema.GroupVersionKind) {
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

	r.logger.V(1).Info("Trigger resource event received",
		"eventType", eventType,
		"gvk", gvk.String(),
		"name", unstructuredObj.GetName(),
		"namespace", unstructuredObj.GetNamespace())

	// Process grant creation for this trigger resource
	ctx := context.Background()
	if err := r.processTriggerResourceForGrantCreation(ctx, unstructuredObj, gvk); err != nil {
		r.logger.Error(err, "Failed to process trigger resource for grant creation",
			"gvk", gvk.String(),
			"name", unstructuredObj.GetName(),
			"namespace", unstructuredObj.GetNamespace())
	}
}

// processTriggerResourceForGrantCreation processes a trigger resource and creates grants based on active policies
func (r *GrantCreationController) processTriggerResourceForGrantCreation(ctx context.Context, triggerObj *unstructured.Unstructured, gvk schema.GroupVersionKind) error {
	// Get all policies that target this resource type
	policies := r.PolicyEngine.GetPoliciesForResource(gvk)
	if len(policies) == 0 {
		r.logger.V(2).Info("No active policies found for trigger resource",
			"gvk", gvk.String(),
			"name", triggerObj.GetName())
		return nil
	}

	r.logger.Info("Processing trigger resource for grant creation",
		"gvk", gvk.String(),
		"name", triggerObj.GetName(),
		"namespace", triggerObj.GetNamespace(),
		"activePolicies", len(policies))

	// Process each applicable policy
	for _, policy := range policies {
		if err := r.processPolicy(ctx, policy, triggerObj); err != nil {
			r.logger.Error(err, "Failed to process policy", "policy", policy.Name)
			r.EventRecorder.Eventf(triggerObj, "Warning", "PolicyProcessingFailed",
				"Failed to process grant creation policy %s: %v", policy.Name, err)
			// Continue with other policies even if one fails
		}
	}

	return nil
}

// discoverGVR discovers the GroupVersionResource for a given API group and kind
func (m *GrantWatchManager) discoverGVR(apiGroup, kind string) (schema.GroupVersionResource, schema.GroupVersionKind, error) {
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
func (m *GrantWatchManager) discoverVersion(apiGroup, kind string) (string, error) {
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
func (m *GrantWatchManager) kindToResource(kind string) string {
	lower := strings.ToLower(kind)
	if strings.HasSuffix(lower, "s") {
		return lower + "es"
	}
	if strings.HasSuffix(lower, "y") {
		return strings.TrimSuffix(lower, "y") + "ies"
	}
	return lower + "s"
}

// Start starts the grant creation controller dynamic watch system
func (r *GrantCreationController) Start(ctx context.Context) error {
	r.logger.Info("Starting grant creation controller dynamic watch system")

	// Start the policy engine first
	r.logger.Info("Starting policy engine")
	if err := r.PolicyEngine.Start(ctx); err != nil {
		return fmt.Errorf("failed to start policy engine: %w", err)
	}
	r.logger.Info("Policy engine started successfully")

	// Start the informer factory
	r.informerFactory.Start(r.stopCh)

	// Initial setup of dynamic watches
	if err := r.updateDynamicWatches(ctx); err != nil {
		r.logger.Error(err, "Failed to setup initial dynamic watches")
		return err
	}

	// Wait for context cancellation
	<-ctx.Done()

	r.logger.Info("Stopping grant creation controller dynamic watch system")
	close(r.stopCh)

	return nil
}

// ParentContextSpec is a simplified version for the resolver.
type ParentContextSpec struct {
	APIGroup       string
	Kind           string
	NameExpression string // This will be the resolved name, not an expression
}
