package quota

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	quotav1alpha1 "go.miloapis.com/milo/pkg/apis/quota/v1alpha1"
)

type ResourceGrantController struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=quota.miloapis.com,resources=resourcegrants,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=quota.miloapis.com,resources=resourcegrants/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=quota.miloapis.com,resources=resourceregistrations,verbs=get;list;watch
func (r *ResourceGrantController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the ResourceGrant instance
	var grant quotav1alpha1.ResourceGrant
	if err := r.Get(ctx, types.NamespacedName{Name: req.Name, Namespace: quotav1alpha1.MiloSystemNamespace}, &grant); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("ResourceGrant not found")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to get ResourceGrant: %w", err)
	}

	// If the resource is being deleted, stop reconciliation.
	if !grant.DeletionTimestamp.IsZero() {
		logger.Info("ResourceGrant is being deleted")
		return ctrl.Result{}, nil
	}

	// Log that reconciliation is proceeding
	logger.Info("reconciling ResourceGrant",
		"name", grant.Name,
		"namespace", grant.Namespace,
		"generation", grant.Generation)

	return ctrl.Result{}, r.updateResourceGrantStatus(ctx, &grant)
}

// updateResourceGrantStatus updates the status of the ResourceGrant
func (r *ResourceGrantController) updateResourceGrantStatus(ctx context.Context, grant *quotav1alpha1.ResourceGrant) error {
	logger := log.FromContext(ctx)

	// Create a deep copy of the original status to compare against later
	originalStatus := grant.Status.DeepCopy()

	// Always update the observed generation to match the current spec
	grant.Status.ObservedGeneration = grant.Generation

	// Get the current active condition from the status or create a new one
	activeCondition := apimeta.FindStatusCondition(grant.Status.Conditions, quotav1alpha1.ResourceGrantActive)
	if activeCondition == nil {
		activeCondition = &metav1.Condition{
			Type:               quotav1alpha1.ResourceGrantActive,
			Status:             metav1.ConditionFalse,
			Reason:             quotav1alpha1.ResourceGrantPendingReason,
			ObservedGeneration: grant.Generation,
		}
	} else {
		activeCondition = activeCondition.DeepCopy()
		activeCondition.ObservedGeneration = grant.Generation
	}

	// Validate that all required registrations exist before marking the grant as active
	if err := r.validateResourceRegistrationsForGrant(ctx, grant); err != nil {
		logger.Info("ResourceGrant validation failed", "error", err)
		activeCondition.Status = metav1.ConditionFalse
		activeCondition.Reason = quotav1alpha1.ResourceGrantValidationFailedReason
		activeCondition.Message = fmt.Sprintf("Validation failed: %v", err)
	} else {
		// Set the grant status as active
		activeCondition.Status = metav1.ConditionTrue
		activeCondition.Reason = quotav1alpha1.ResourceGrantActiveReason
		activeCondition.Message = "The grant has been successfully activated and will now be taken into account when evaluating future claims."
	}

	// Set the condition on the status
	apimeta.SetStatusCondition(&grant.Status.Conditions, *activeCondition)

	// Only call the API to update the status if something has actually changed
	if !equality.Semantic.DeepEqual(originalStatus, &grant.Status) {
		if err := r.Status().Update(ctx, grant); err != nil {
			return fmt.Errorf("failed to update ResourceGrant status: %w", err)
		}
		logger.Info("ResourceGrant status updated",
			"name", grant.Name,
			"namespace", grant.Namespace,
			"active", activeCondition.Status)
	}

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

	return ValidateResourceRegistrations(ctx, r.Client, resourceTypes)
}

// SetupWithManager sets up the controller with the Manager.
// Uses GenerationChangedPredicate to avoid reconciling on status-only updates.
func (r *ResourceGrantController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&quotav1alpha1.ResourceGrant{}).
		Named("resource-grant").
		Complete(r)
}
