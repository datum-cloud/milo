// Package policy implements controllers for quota policy management.
// It contains controllers for ClaimCreationPolicy and GrantCreationPolicy resources
// that validate policy configurations and manage grant creation workflows.
package policy

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"go.miloapis.com/milo/internal/quota/validation"
	quotav1alpha1 "go.miloapis.com/milo/pkg/apis/quota/v1alpha1"
)

// GrantCreationPolicyReconciler reconciles a GrantCreationPolicy object.
// Its responsibility is to validate the policy and set the Ready status condition.
// The PolicyEngine watches for policies with Ready=True to include them in grant creation.
type GrantCreationPolicyReconciler struct {
	client.Client
	// Scheme is the runtime scheme for object serialization.
	Scheme *runtime.Scheme
	// PolicyValidator validates GrantCreationPolicy resources.
	PolicyValidator *validation.GrantCreationPolicyValidator
}

// +kubebuilder:rbac:groups=quota.miloapis.com,resources=grantcreationpolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=quota.miloapis.com,resources=grantcreationpolicies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=quota.miloapis.com,resources=resourceregistrations,verbs=get;list;watch

// Reconcile reconciles a GrantCreationPolicy object by validating it and setting its Ready status.
func (r *GrantCreationPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var policy quotav1alpha1.GrantCreationPolicy
	logger.V(1).Info("Reconciling GrantCreationPolicy", "name", req.Name)

	if err := r.Get(ctx, req.NamespacedName, &policy); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("GrantCreationPolicy was deleted", "name", req.Name)
			// Policy was deleted - nothing to do (PolicyEngine will handle via watch)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to get GrantCreationPolicy: %w", err)
	}

	// Store original status to detect changes
	originalStatus := policy.Status.DeepCopy()

	// Validate the policy
	validationErrs := r.PolicyValidator.Validate(ctx, &policy)

	// Update policy status based on validation results
	r.updatePolicyStatus(&policy, validationErrs)

	// Update status if it has changed
	if !equality.Semantic.DeepEqual(&policy.Status, originalStatus) {
		policy.Status.ObservedGeneration = policy.Generation

		if err := r.Status().Update(ctx, &policy); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update GrantCreationPolicy status: %w", err)
		}
		logger.V(1).Info("Updated GrantCreationPolicy status",
			"policy", policy.Name,
			"ready", apimeta.IsStatusConditionTrue(policy.Status.Conditions, quotav1alpha1.GrantCreationPolicyReady))
	}

	return ctrl.Result{}, nil
}

// updatePolicyStatus updates the policy status conditions based on validation results.
func (r *GrantCreationPolicyReconciler) updatePolicyStatus(policy *quotav1alpha1.GrantCreationPolicy, validationErrs field.ErrorList) {
	now := metav1.NewTime(time.Now())

	if len(validationErrs) > 0 {
		// Validation failed - format errors with field paths
		apimeta.SetStatusCondition(&policy.Status.Conditions, metav1.Condition{
			Type:               quotav1alpha1.GrantCreationPolicyReady,
			Status:             metav1.ConditionFalse,
			Reason:             quotav1alpha1.GrantCreationPolicyValidationFailedReason,
			Message:            validationErrs.ToAggregate().Error(),
			LastTransitionTime: now,
		})

		// Clear parent context ready condition if validation failed
		apimeta.RemoveStatusCondition(&policy.Status.Conditions, quotav1alpha1.GrantCreationPolicyParentContextReady)
		return
	}

	if policy.Spec.Disabled != nil && *policy.Spec.Disabled {
		// Policy is disabled
		apimeta.SetStatusCondition(&policy.Status.Conditions, metav1.Condition{
			Type:               quotav1alpha1.GrantCreationPolicyReady,
			Status:             metav1.ConditionFalse,
			Reason:             quotav1alpha1.GrantCreationPolicyDisabledReason,
			Message:            "Policy is disabled",
			LastTransitionTime: now,
		})

		// Clear parent context ready condition if disabled
		apimeta.RemoveStatusCondition(&policy.Status.Conditions, quotav1alpha1.GrantCreationPolicyParentContextReady)
		return
	}

	// Validation passed and policy is enabled
	apimeta.SetStatusCondition(&policy.Status.Conditions, metav1.Condition{
		Type:               quotav1alpha1.GrantCreationPolicyReady,
		Status:             metav1.ConditionTrue,
		Reason:             quotav1alpha1.GrantCreationPolicyReadyReason,
		Message:            "Policy is ready and active",
		LastTransitionTime: now,
	})

	// Set parent context ready condition
	if policy.Spec.Target.ParentContext != nil {
		apimeta.SetStatusCondition(&policy.Status.Conditions, metav1.Condition{
			Type:               quotav1alpha1.GrantCreationPolicyParentContextReady,
			Status:             metav1.ConditionTrue,
			Reason:             quotav1alpha1.GrantCreationPolicyParentContextReadyReason,
			Message:            "Parent context resolution is ready",
			LastTransitionTime: now,
		})
	} else {
		// No parent context specified - remove the condition
		apimeta.RemoveStatusCondition(&policy.Status.Conditions, quotav1alpha1.GrantCreationPolicyParentContextReady)
	}
}

// enqueueAffectedPolicies finds all GrantCreationPolicies that reference a ResourceRegistration
// and enqueues them for reconciliation when the registration changes.
func (r *GrantCreationPolicyReconciler) enqueueAffectedPolicies(ctx context.Context, obj client.Object) []reconcile.Request {
	registration, ok := obj.(*quotav1alpha1.ResourceRegistration)
	if !ok {
		return nil
	}

	// List all GrantCreationPolicies
	var policyList quotav1alpha1.GrantCreationPolicyList
	if err := r.List(ctx, &policyList); err != nil {
		// Log error but don't block - policies will be revalidated on their regular schedule
		log.FromContext(ctx).Error(err, "Failed to list GrantCreationPolicies for ResourceRegistration change",
			"registration", registration.Name)
		return nil
	}

	var requests []reconcile.Request
	// Find policies that reference this resource type
	for _, policy := range policyList.Items {
		for _, allowance := range policy.Spec.Target.ResourceGrantTemplate.Spec.Allowances {
			if allowance.ResourceType == registration.Spec.ResourceType {
				// This policy references the changed ResourceRegistration - trigger reconciliation
				requests = append(requests, reconcile.Request{
					NamespacedName: client.ObjectKeyFromObject(&policy),
				})
				break // Only need to enqueue each policy once
			}
		}
	}

	return requests
}

// SetupWithManager sets up the controller with the Manager.
func (r *GrantCreationPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&quotav1alpha1.GrantCreationPolicy{}).
		// Watch ResourceRegistrations to revalidate policies when registrations change
		Watches(&quotav1alpha1.ResourceRegistration{}, handler.EnqueueRequestsFromMapFunc(
			r.enqueueAffectedPolicies,
		)).
		Named("grant-creation-policy").
		Complete(r)
}
