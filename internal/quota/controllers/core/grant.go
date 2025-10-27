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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	mcbuilder "sigs.k8s.io/multicluster-runtime/pkg/builder"
	mcmanager "sigs.k8s.io/multicluster-runtime/pkg/manager"
	mcreconcile "sigs.k8s.io/multicluster-runtime/pkg/reconcile"

	"go.miloapis.com/milo/internal/quota/validation"
	quotav1alpha1 "go.miloapis.com/milo/pkg/apis/quota/v1alpha1"
)

// ResourceGrantController reconciles a ResourceGrant object.
type ResourceGrantController struct {
	Scheme         *runtime.Scheme
	Manager        mcmanager.Manager
	GrantValidator *validation.ResourceGrantValidator
}

// +kubebuilder:rbac:groups=quota.miloapis.com,resources=resourcegrants,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=quota.miloapis.com,resources=resourcegrants/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=quota.miloapis.com,resources=resourceregistrations,verbs=get;list;watch

// Reconcile manages the lifecycle of ResourceGrant objects across all control planes.
func (r *ResourceGrantController) Reconcile(ctx context.Context, req mcreconcile.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	if req.ClusterName != "" {
		logger = logger.WithValues("cluster", req.ClusterName)
	}

	cluster, err := r.Manager.GetCluster(ctx, req.ClusterName)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get cluster %q: %w", req.ClusterName, err)
	}
	clusterClient := cluster.GetClient()

	// Fetch the ResourceGrant
	var grant quotav1alpha1.ResourceGrant
	if err := clusterClient.Get(ctx, req.NamespacedName, &grant); err != nil {
		if apierrors.IsNotFound(err) {
			logger.V(1).Info("ResourceGrant not found")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to get ResourceGrant: %w", err)
	}

	// Update observed generation and conditions
	if err := r.updateResourceGrantStatus(ctx, clusterClient, &grant); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// updateResourceGrantStatus updates the status of the ResourceGrant.
func (r *ResourceGrantController) updateResourceGrantStatus(ctx context.Context, clusterClient client.Client, grant *quotav1alpha1.ResourceGrant) error {
	logger := log.FromContext(ctx)
	originalStatus := grant.Status.DeepCopy()

	// Always update the observed generation in the status to match the current generation of the spec.
	grant.Status.ObservedGeneration = grant.Generation

	if validationErrs := r.GrantValidator.Validate(ctx, grant, validation.ControllerValidationOptions()); len(validationErrs) > 0 {
		logger.Info("ResourceGrant validation failed", "errors", validationErrs.ToAggregate())
		return r.setValidationFailedCondition(ctx, clusterClient, grant, validationErrs.ToAggregate())
	}

	// Set active condition
	r.setActiveCondition(grant)

	// Only update the status if it has changed
	return r.updateStatusIfChanged(ctx, clusterClient, grant, originalStatus)
}

// setValidationFailedCondition sets the validation failed condition and updates status.
func (r *ResourceGrantController) setValidationFailedCondition(ctx context.Context, clusterClient client.Client, grant *quotav1alpha1.ResourceGrant, validationErr error) error {
	condition := metav1.Condition{
		Type:    quotav1alpha1.ResourceGrantActive,
		Status:  metav1.ConditionFalse,
		Reason:  quotav1alpha1.ResourceGrantValidationFailedReason,
		Message: fmt.Sprintf("Validation failed: %v", validationErr),
	}
	apimeta.SetStatusCondition(&grant.Status.Conditions, condition)

	if err := clusterClient.Status().Update(ctx, grant); err != nil {
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
func (r *ResourceGrantController) updateStatusIfChanged(ctx context.Context, clusterClient client.Client, grant *quotav1alpha1.ResourceGrant, originalStatus *quotav1alpha1.ResourceGrantStatus) error {
	activeCondition := apimeta.FindStatusCondition(grant.Status.Conditions, quotav1alpha1.ResourceGrantActive)
	if activeCondition == nil {
		return nil
	}

	// Check if status actually changed
	if !apimeta.IsStatusConditionPresentAndEqual(originalStatus.Conditions, quotav1alpha1.ResourceGrantActive, activeCondition.Status) ||
		grant.Status.ObservedGeneration != originalStatus.ObservedGeneration {

		if err := clusterClient.Status().Update(ctx, grant); err != nil {
			return fmt.Errorf("failed to update ResourceGrant status: %w", err)
		}
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
// ResourceGrants can exist in both the local cluster and project control planes for delegated quota management.
//
// Note: We don't watch ResourceRegistrations because the admission plugin validates that
// all resource types are already registered before allowing grant creation.
func (r *ResourceGrantController) SetupWithManager(mgr mcmanager.Manager) error {
	return mcbuilder.ControllerManagedBy(mgr).
		For(&quotav1alpha1.ResourceGrant{},
			mcbuilder.WithEngageWithLocalCluster(true),
			mcbuilder.WithEngageWithProviderClusters(true),
		).
		Named("resource-grant").
		Complete(r)
}
