package quota

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	quotav1alpha1 "go.miloapis.com/milo/pkg/apis/quota/v1alpha1"
	"go.miloapis.com/milo/internal/validation/quota"
)

// ResourceGrantController reconciles a ResourceGrant object
type ResourceGrantController struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=quota.miloapis.com,resources=resourcegrants,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=quota.miloapis.com,resources=resourcegrants/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=quota.miloapis.com,resources=resourceregistrations,verbs=get;list;watch

// Reconcile manages the lifecycle of ResourceGrant objects.
func (r *ResourceGrantController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling ResourceGrant")

	// Fetch the ResourceGrant
	var grant quotav1alpha1.ResourceGrant
	if err := r.Get(ctx, req.NamespacedName, &grant); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("ResourceGrant not found")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to get ResourceGrant: %w", err)
	}

	// Update observed generation and conditions
	if err := r.updateResourceGrantStatus(ctx, &grant); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// Update the status of the ResourceGrant
func (r *ResourceGrantController) updateResourceGrantStatus(ctx context.Context, grant *quotav1alpha1.ResourceGrant) error {
	logger := log.FromContext(ctx)

	// Create a deep copy of the original status to compare against later
	originalStatus := grant.Status.DeepCopy()

	// Always update the observed generation in the status to match the current generation of the spec.
	grant.Status.ObservedGeneration = grant.Generation

	// Validate that all required registrations exist before marking the grant as active
	if err := r.validateResourceRegistrationsForGrant(ctx, grant); err != nil {
		logger.Info("ResourceGrant validation failed", "error", err)

		// Set the condition to indicate validation failed
		validationFailedCondition := metav1.Condition{
			Type:    quotav1alpha1.ResourceGrantActive,
			Status:  metav1.ConditionFalse,
			Reason:  quotav1alpha1.ResourceGrantValidationFailedReason,
			Message: fmt.Sprintf("Validation failed: %v", err),
		}
		apimeta.SetStatusCondition(&grant.Status.Conditions, validationFailedCondition)

		// Update status and return
		if err := r.Status().Update(ctx, grant); err != nil {
			return fmt.Errorf("failed to update ResourceGrant status: %w", err)
		}
		return nil
	}

	// Get the current active condition from the status or create a new one
	activeCondition := apimeta.FindStatusCondition(grant.Status.Conditions, quotav1alpha1.ResourceGrantActive)
	if activeCondition == nil {
		activeCondition = &metav1.Condition{
			Type:               quotav1alpha1.ResourceGrantActive,
			Status:             metav1.ConditionTrue,
			Reason:             quotav1alpha1.ResourceGrantActiveReason,
			Message:            "ResourceGrant is active",
			ObservedGeneration: grant.Generation,
		}
	} else {
		activeCondition = activeCondition.DeepCopy()
		activeCondition.Status = metav1.ConditionTrue
		activeCondition.Reason = quotav1alpha1.ResourceGrantActiveReason
		activeCondition.Message = "ResourceGrant is active"
		activeCondition.ObservedGeneration = grant.Generation
	}

	apimeta.SetStatusCondition(&grant.Status.Conditions, *activeCondition)

	// Only update the status if it has changed
	if !apimeta.IsStatusConditionPresentAndEqual(originalStatus.Conditions, quotav1alpha1.ResourceGrantActive, activeCondition.Status) ||
		grant.Status.ObservedGeneration != originalStatus.ObservedGeneration {

		if err := r.Status().Update(ctx, grant); err != nil {
			return fmt.Errorf("failed to update ResourceGrant status: %w", err)
		}

		logger.Info("Updated ResourceGrant status",
			"name", grant.Name,
			"namespace", grant.Namespace,
			"active", activeCondition.Status)
	}

	// AllowanceBuckets will be created/updated by a separate AllowanceBucket controller
	// that watches both ResourceGrants and ResourceClaims to maintain limit and usage

	return nil
}

// validateResourceRegistrations validates that all resource types in the grant
// have corresponding registrations.
func (r *ResourceGrantController) validateResourceRegistrationsForGrant(ctx context.Context, grant *quotav1alpha1.ResourceGrant) error {
	// Collect all unique resource type names from allowances to be passed into
	// the ValidateResourceRegistrations function.
	var resourceTypes []string
	for _, allowance := range grant.Spec.Allowances {
		resourceTypes = append(resourceTypes, allowance.ResourceType)
	}

	return quota.ValidateResourceRegistrations(ctx, r.Client, resourceTypes)
}

// SetupWithManager sets up the controller with the Manager.
// Watches ResourceRegistrations to trigger reconciliation when new resource types are registered.
func (r *ResourceGrantController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&quotav1alpha1.ResourceGrant{}).
		Watches(
			&quotav1alpha1.ResourceRegistration{},
			// Trigger reconciliation of all ResourceGrants when ResourceRegistrations change
			// This ensures ResourceGrants get re-validated when new resource types are registered
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, a client.Object) []reconcile.Request {
				// List all ResourceGrants and enqueue them for reconciliation
				var grants quotav1alpha1.ResourceGrantList
				if err := r.List(ctx, &grants); err != nil {
					// Log error but don't fail the watch setup
					return nil
				}

				requests := make([]reconcile.Request, 0, len(grants.Items))
				for _, grant := range grants.Items {
					requests = append(requests, reconcile.Request{
						NamespacedName: types.NamespacedName{
							Name:      grant.Name,
							Namespace: grant.Namespace,
						},
					})
				}
				return requests
			}),
		).
		Named("resource-grant").
		Complete(r)
}
