// Package core implements the core quota controllers that manage AllowanceBuckets,
// ResourceClaims, ResourceGrants, and ResourceRegistrations.
//
// The ResourceGrantController validates ResourceGrants against ResourceRegistrations
// and manages their Active status condition. It ensures that all resource types
// referenced in grants have valid registrations before marking grants as active.
package core

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

	"go.miloapis.com/milo/internal/quota/validation"
	quotav1alpha1 "go.miloapis.com/milo/pkg/apis/quota/v1alpha1"
)

// ResourceGrantController reconciles a ResourceGrant object.
type ResourceGrantController struct {
	client.Client
	Scheme *runtime.Scheme
	// ResourceTypeValidator validates resource types against ResourceRegistrations.
	ResourceTypeValidator validation.ResourceTypeValidator
}

// +kubebuilder:rbac:groups=quota.miloapis.com,resources=resourcegrants,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=quota.miloapis.com,resources=resourcegrants/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=quota.miloapis.com,resources=resourceregistrations,verbs=get;list;watch

// Reconcile manages the lifecycle of ResourceGrant objects.
func (r *ResourceGrantController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

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

// updateResourceGrantStatus updates the status of the ResourceGrant.
func (r *ResourceGrantController) updateResourceGrantStatus(ctx context.Context, grant *quotav1alpha1.ResourceGrant) error {
	logger := log.FromContext(ctx)
	originalStatus := grant.Status.DeepCopy()

	// Always update the observed generation in the status to match the current generation of the spec.
	grant.Status.ObservedGeneration = grant.Generation

	// Validate that all required registrations exist before marking the grant as active
	if err := r.validateResourceRegistrationsForGrant(ctx, grant); err != nil {
		logger.Info("ResourceGrant validation failed", "error", err)
		return r.setValidationFailedCondition(ctx, grant, err)
	}

	// Set active condition
	r.setActiveCondition(grant)

	// Only update the status if it has changed
	return r.updateStatusIfChanged(ctx, grant, originalStatus)
}

// setValidationFailedCondition sets the validation failed condition and updates status.
func (r *ResourceGrantController) setValidationFailedCondition(ctx context.Context, grant *quotav1alpha1.ResourceGrant, validationErr error) error {
	condition := metav1.Condition{
		Type:    quotav1alpha1.ResourceGrantActive,
		Status:  metav1.ConditionFalse,
		Reason:  quotav1alpha1.ResourceGrantValidationFailedReason,
		Message: fmt.Sprintf("Validation failed: %v", validationErr),
	}
	apimeta.SetStatusCondition(&grant.Status.Conditions, condition)

	if err := r.Status().Update(ctx, grant); err != nil {
		return fmt.Errorf("failed to update ResourceGrant status: %w", err)
	}
	return nil
}

// setActiveCondition sets the active condition on the grant.
func (r *ResourceGrantController) setActiveCondition(grant *quotav1alpha1.ResourceGrant) {
	condition := metav1.Condition{
		Type:               quotav1alpha1.ResourceGrantActive,
		Status:             metav1.ConditionTrue,
		Reason:             quotav1alpha1.ResourceGrantActiveReason,
		Message:            "ResourceGrant is active",
		ObservedGeneration: grant.Generation,
	}
	apimeta.SetStatusCondition(&grant.Status.Conditions, condition)
}

// updateStatusIfChanged updates the status only if it has changed.
func (r *ResourceGrantController) updateStatusIfChanged(ctx context.Context, grant *quotav1alpha1.ResourceGrant, originalStatus *quotav1alpha1.ResourceGrantStatus) error {
	activeCondition := apimeta.FindStatusCondition(grant.Status.Conditions, quotav1alpha1.ResourceGrantActive)
	if activeCondition == nil {
		return nil
	}

	// Check if status actually changed
	if !apimeta.IsStatusConditionPresentAndEqual(originalStatus.Conditions, quotav1alpha1.ResourceGrantActive, activeCondition.Status) ||
		grant.Status.ObservedGeneration != originalStatus.ObservedGeneration {

		if err := r.Status().Update(ctx, grant); err != nil {
			return fmt.Errorf("failed to update ResourceGrant status: %w", err)
		}
	}

	return nil
}

// validateResourceRegistrationsForGrant validates that all resource types in the grant
// have corresponding registrations.
func (r *ResourceGrantController) validateResourceRegistrationsForGrant(ctx context.Context, grant *quotav1alpha1.ResourceGrant) error {
	// Validate each unique resource type using the shared validator
	seen := make(map[string]bool)
	for _, allowance := range grant.Spec.Allowances {
		resourceType := allowance.ResourceType
		if !seen[resourceType] {
			seen[resourceType] = true
			if err := r.ResourceTypeValidator.ValidateResourceType(ctx, resourceType); err != nil {
				return err
			}
		}
	}
	return nil
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
