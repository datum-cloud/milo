// Package policy implements controllers for quota policy management.
// It contains controllers for ClaimCreationPolicy and GrantCreationPolicy resources
// that validate policy configurations and manage grant creation workflows.
package policy

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"go.miloapis.com/milo/internal/informer"
	"go.miloapis.com/milo/internal/quota/engine"
	quotav1alpha1 "go.miloapis.com/milo/pkg/apis/quota/v1alpha1"
)

// GrantCreationController watches trigger resources and creates grants based on active policies.
type GrantCreationController struct {
	client.Client
	// Scheme is the runtime scheme for object serialization.
	Scheme *runtime.Scheme
	// TemplateEngine renders grant templates with trigger resource data.
	TemplateEngine engine.TemplateEngine
	// ParentContextResolver resolves cross-cluster clients for grant creation.
	ParentContextResolver *ParentContextResolver
	// EventRecorder records events for processed resources.
	EventRecorder record.EventRecorder

	// informerManager manages dynamic watches for trigger resources.
	informerManager informer.Manager
	// logger is the controller's logger instance.
	logger logr.Logger
}

// grantCreationHandler implements informer.ResourceEventHandler for grant creation.
type grantCreationHandler struct {
	// controller is the parent GrantCreationController.
	controller *GrantCreationController
	// policyName is the name of the policy this handler processes.
	policyName string
}

// OnAdd implements informer.ResourceEventHandler.
func (h *grantCreationHandler) OnAdd(obj *unstructured.Unstructured) {
	h.controller.processTriggerResource(obj, h.policyName, "ADD")
}

// OnUpdate implements informer.ResourceEventHandler.
func (h *grantCreationHandler) OnUpdate(old, new *unstructured.Unstructured) {
	h.controller.processTriggerResource(new, h.policyName, "UPDATE")
}

// OnDelete implements informer.ResourceEventHandler.
func (h *grantCreationHandler) OnDelete(obj *unstructured.Unstructured) {
	h.controller.processTriggerResource(obj, h.policyName, "DELETE")
}

// NewGrantCreationController creates a new GrantCreationController.
func NewGrantCreationController(
	client client.Client,
	scheme *runtime.Scheme,
	templateEngine engine.TemplateEngine,
	parentContextResolver *ParentContextResolver,
	eventRecorder record.EventRecorder,
	informerManager informer.Manager,
) *GrantCreationController {
	logger := ctrl.Log.WithName("grant-creation")

	return &GrantCreationController{
		Client:                client,
		Scheme:                scheme,
		TemplateEngine:        templateEngine,
		ParentContextResolver: parentContextResolver,
		EventRecorder:         eventRecorder,
		informerManager:       informerManager,
		logger:                logger,
	}
}

// +kubebuilder:rbac:groups=*,resources=*,verbs=get;list;watch
// +kubebuilder:rbac:groups=quota.miloapis.com,resources=resourcegrants,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=quota.miloapis.com,resources=grantcreationpolicies,verbs=get;list;watch

// Reconcile processes GrantCreationPolicy changes.

// processTriggerResource processes a trigger resource event.
func (r *GrantCreationController) processTriggerResource(obj *unstructured.Unstructured, policyName, eventType string) {
	ctx := context.Background()
	logger := r.logger.WithValues(
		"triggerResource", obj.GetName(),
		"triggerKind", obj.GetKind(),
		"namespace", obj.GetNamespace(),
		"policy", policyName,
		"eventType", eventType,
	)

	// Skip DELETE events for now (we handle cleanup via owner references)
	if eventType == "DELETE" {
		logger.V(2).Info("Skipping DELETE event")
		return
	}

	// Get the specific policy
	policy, err := r.getPolicyByName(ctx, policyName)
	if err != nil {
		logger.Error(err, "Failed to get policy")
		return
	}

	if policy == nil {
		logger.V(2).Info("Policy not found, removing watch")
		r.removeWatchForPolicy(ctx, policyName)
		return
	}

	logger.Info("Processing trigger resource for grant creation")

	// Process the policy
	if err := r.processPolicy(ctx, policy, obj); err != nil {
		logger.Error(err, "Failed to process policy")
		r.EventRecorder.Eventf(obj, "Warning", "PolicyProcessingFailed",
			"Failed to process grant creation policy %s: %v", policy.Name, err)
	}
}

// Reconcile handles GrantCreationPolicy changes.
func (r *GrantCreationController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("policyName", req.Name)

	// Fetch the policy
	var policy quotav1alpha1.GrantCreationPolicy
	if err := r.Get(ctx, req.NamespacedName, &policy); err != nil {
		if apierrors.IsNotFound(err) {
			// Policy was deleted
			logger.Info("Policy deleted, removing watch")

			// Remove dynamic watch
			if err := r.removeWatchForPolicy(ctx, req.Name); err != nil {
				logger.Error(err, "Failed to remove watch for deleted policy")
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Check if policy is Ready (has Ready=True condition)
	isReady := r.isPolicyReady(&policy)
	logger.V(1).Info("Policy reconciled", "ready", isReady)

	if isReady {
		// Policy is ready - set up dynamic watch for the trigger resource
		if err := r.addWatchForPolicy(ctx, &policy); err != nil {
			logger.Error(err, "Failed to add watch for policy")
			return ctrl.Result{RequeueAfter: time.Second * 10}, nil
		}
	} else {
		// Policy not ready - clean up watch
		if err := r.removeWatchForPolicy(ctx, req.Name); err != nil {
			logger.Error(err, "Failed to remove watch for not-ready policy")
		}
	}

	logger.Info("Successfully processed policy change")
	return ctrl.Result{}, nil
}

// isPolicyReady checks if a GrantCreationPolicy has Ready=True status condition.
func (r *GrantCreationController) isPolicyReady(policy *quotav1alpha1.GrantCreationPolicy) bool {
	for _, condition := range policy.Status.Conditions {
		if condition.Type == quotav1alpha1.GrantCreationPolicyReady && condition.Status == "True" {
			return true
		}
	}
	return false
}

// addWatchForPolicy adds a dynamic watch for a policy's trigger resource.
func (r *GrantCreationController) addWatchForPolicy(ctx context.Context, policy *quotav1alpha1.GrantCreationPolicy) error {
	gvk := policy.Spec.Trigger.Resource.GetGVK()
	consumerID := fmt.Sprintf("grant-creation-policy-%s", policy.Name)

	handler := &grantCreationHandler{
		controller: r,
		policyName: policy.Name,
	}

	req := informer.WatchRequest{
		GVK:        gvk,
		ConsumerID: consumerID,
		Handler:    handler,
	}

	return r.informerManager.AddWatch(ctx, req)
}

// removeWatchForPolicy removes a dynamic watch for a policy.
func (r *GrantCreationController) removeWatchForPolicy(ctx context.Context, policyName string) error {
	// We need to get the policy to know what GVK to remove
	policy, err := r.getPolicyByName(ctx, policyName)
	if err != nil {
		return err
	}

	if policy == nil {
		// Policy doesn't exist, nothing to remove
		return nil
	}

	gvk := policy.Spec.Trigger.Resource.GetGVK()
	consumerID := fmt.Sprintf("grant-creation-policy-%s", policyName)

	return r.informerManager.RemoveWatch(ctx, gvk, consumerID)
}

// getPolicyByName retrieves a GrantCreationPolicy by name.
func (r *GrantCreationController) getPolicyByName(ctx context.Context, name string) (*quotav1alpha1.GrantCreationPolicy, error) {
	policy := &quotav1alpha1.GrantCreationPolicy{}
	key := client.ObjectKey{Name: name}

	if err := r.Get(ctx, key, policy); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	return policy, nil
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
		return r.Client, policy.Spec.Target.ResourceGrantTemplate.Metadata.Namespace, nil
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
		APIGroup: parentContext.APIGroup,
		Kind:     parentContext.Kind,
		Name:     parentContextName, // Use resolved name directly
	}, triggerObj)
	if err != nil {
		return nil, "", fmt.Errorf("failed to resolve parent context client: %w", err)
	}

	return targetClient, policy.Spec.Target.ResourceGrantTemplate.Metadata.Namespace, nil
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
			// Set owner reference to trigger object
			if err := r.setOwnerReference(grant, triggerObj); err != nil {
				return fmt.Errorf("failed to set owner reference: %w", err)
			}

			// Create new grant
			logger.Info("Creating new ResourceGrant")
			if err := targetClient.Create(ctx, grant); err != nil {
				return fmt.Errorf("failed to create grant: %w", err)
			}

			r.EventRecorder.Eventf(triggerObj, "Normal", "GrantCreated",
				"Created ResourceGrant %s/%s from policy %s", grant.Namespace, grant.Name, policy.Name)

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

// SetupWithManager sets up the controller with the Manager.
func (r *GrantCreationController) SetupWithManager(mgr ctrl.Manager) error {
	r.logger.Info("Setting up GrantCreationController")

	// Watch GrantCreationPolicies to update dynamic watches when policies change
	controller := ctrl.NewControllerManagedBy(mgr).
		For(&quotav1alpha1.GrantCreationPolicy{}).
		Named("grant-creation-controller")

	r.logger.Info("GrantCreationController setup completed successfully")

	return controller.Complete(r)
}

// ParentContextSpec is a simplified version for the resolver.
type ParentContextSpec struct {
	// APIGroup is the API group of the parent context resource.
	APIGroup string
	// Kind is the kind of the parent context resource.
	Kind string
	// Name is the resolved name of the parent context resource.
	Name string
}

// setOwnerReference sets an owner reference from the trigger object to the ResourceGrant.
func (r *GrantCreationController) setOwnerReference(grant *quotav1alpha1.ResourceGrant, triggerObj *unstructured.Unstructured) error {
	// Create owner reference
	controller := true
	ownerRef := metav1.OwnerReference{
		APIVersion: triggerObj.GetAPIVersion(),
		Kind:       triggerObj.GetKind(),
		Name:       triggerObj.GetName(),
		UID:        triggerObj.GetUID(),
		Controller: &controller, // Set as controlling owner
	}

	// Add owner reference to grant
	grant.SetOwnerReferences([]metav1.OwnerReference{ownerRef})
	return nil
}
