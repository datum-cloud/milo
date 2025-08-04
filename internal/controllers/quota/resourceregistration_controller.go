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

type ResourceRegistrationController struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=quota.miloapis.com,resources=resourceregistrations,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=quota.miloapis.com,resources=resourceregistrations/status,verbs=get;update;patch
//
// Reconciles a ResourceRegistration object by validating it and updating the
// status to reflect whether the registration is active and the resource type
// can be managed by the quota system.
//
// The current implementation has no cross-system validation to determine if the
// resource type and dimensions being registered exist in the system overall.
// Once a common service is created that tracks all existing resource types,
// additional validation can be added.
func (r *ResourceRegistrationController) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, err error) {
	logger := log.FromContext(ctx)
	var registration quotav1alpha1.ResourceRegistration
	// Fetch the cluster-scoped ResourceRegistration by name and check for
	// errors
	if err := r.Get(ctx, types.NamespacedName{Name: req.Name}, &registration); err != nil {
		// Stop reconciliation if the object isn't found
		if apierrors.IsNotFound(err) {
			logger.Info("ResourceRegistration not found")
			return ctrl.Result{}, nil
		}
		// Requeue the reconciliation and retry if an unexpected error occurs
		// (e.g. a network failure)
		return ctrl.Result{}, fmt.Errorf("failed to get ResourceRegistration: %w", err)
	}

	// If the resource is being deleted, stop reconciliation. TODO: add
	// finalizer to delete grants, claims, effective grants, allowance buckets
	// that are tied to this registration so they aren't orphaned. It is
	// unlikely registrations will be deleted, so this can be deferred to a
	// follow up PR. For now, configured RBAC for the controller does not allow
	// deletion of registrations.
	if !registration.DeletionTimestamp.IsZero() {
		logger.Info("ResourceRegistration is being deleted")
		return ctrl.Result{}, nil
	}

	// Log that reconciliation is proceeding and the details of the registration
	// for debugging purposes.
	logger.Info("Reconciling ResourceRegistration",
		"name", registration.Name,
		"resourceType", registration.Spec.ResourceType,
		"type", registration.Spec.Type,
		"generation", registration.Generation)

	// Create a deep copy of the original status to determine if any changes
	// have occurred during reconciliation
	originalStatus := registration.Status.DeepCopy()

	// Update the observed generation to reflect that the current spec has been
	// processed
	registration.Status.ObservedGeneration = registration.Generation

	// Get the current active condition from the status and create a new one in
	// a pending state if it doesn't already exist. Otherwise, update the
	// observed generation to match the current one being processed.
	activeCondition := apimeta.FindStatusCondition(registration.Status.Conditions, quotav1alpha1.ResourceRegistrationActive)
	if activeCondition == nil {
		// Since the active condition is not set, this means that reconciliation
		// was triggered by the creation of the registration.
		activeCondition = &metav1.Condition{
			Type:               quotav1alpha1.ResourceRegistrationActive,
			Status:             metav1.ConditionFalse,
			Reason:             quotav1alpha1.ResourceRegistrationPendingReason,
			Message:            "The registration is pending validation",
			ObservedGeneration: registration.Generation,
		}
	} else {
		activeCondition = activeCondition.DeepCopy()
		activeCondition.ObservedGeneration = registration.Generation
	}

	// If the registration is already active, skip the rest of the
	// reconciliation.
	if activeCondition.Status == metav1.ConditionTrue {
		return ctrl.Result{}, nil
	}

	// In phase 1, this is a placeholder that always passes. Phase 2 will add
	// cross-system validation to ensure that the resource type is valid and
	// that the owner ref is valid.
	if err := r.validateRegistration(ctx, &registration); err != nil {
		// Update condition to reflect the failure
		logger.Info("resource registration validation failed", "error", err)
		activeCondition.Status = metav1.ConditionFalse
		activeCondition.Reason = quotav1alpha1.ResourceRegistrationValidationFailedReason
		activeCondition.Message = fmt.Sprintf("Validation failed: %v. Please check the resourceType, ownerRef, and dimensions fields to ensure they are valid.", err)
	} else {
		// Set the condition to true to reflect that validation passed and the
		// registration is now active..
		activeCondition.Status = metav1.ConditionTrue
		activeCondition.Reason = quotav1alpha1.ResourceRegistrationActiveReason
		activeCondition.Message = "The registration is active and resource grants and claims can now be created for this resource type."
	}

	// Apply the updated condition to the status
	apimeta.SetStatusCondition(&registration.Status.Conditions, *activeCondition)

	// Compare the original status to the updated one and attempt to persist the
	// change if they are different.
	if !equality.Semantic.DeepEqual(originalStatus, &registration.Status) {
		if err := r.Status().Update(ctx, &registration); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update ResourceRegistration status: %w", err)
		}
		// Log the successful status update
		logger.Info("resource registration status updated",
			"name", registration.Name,
			"namespace", registration.Namespace,
			"active", activeCondition.Status)
	}

	return ctrl.Result{}, nil
}

// Placeholder function that will validate the registration by ensuring the
// resource type, owner ref, and dimensions are valid and exist in the system.
func (r *ResourceRegistrationController) validateRegistration(_ context.Context, _ *quotav1alpha1.ResourceRegistration) error {
	return nil
}

func (r *ResourceRegistrationController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&quotav1alpha1.ResourceRegistration{}).
		Named("resource-registration").
		Complete(r)
}
