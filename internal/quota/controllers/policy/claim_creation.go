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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"go.miloapis.com/milo/internal/quota/validation"
	quotav1alpha1 "go.miloapis.com/milo/pkg/apis/quota/v1alpha1"
)

// ClaimCreationPolicyReconciler reconciles a ClaimCreationPolicy object.
// Its sole responsibility is to validate the policy and set the Ready status condition.
// The PolicyEngine (used only by the admission plugin) watches for policies with Ready=True.
type ClaimCreationPolicyReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	// ClaimTemplateValidator validates claim metadata templates for syntax and basic constraints.
	ClaimTemplateValidator *validation.ClaimTemplateValidator
	// ResourceTypeValidator validates resource types against ResourceRegistrations.
	ResourceTypeValidator validation.ResourceTypeValidator
}

// +kubebuilder:rbac:groups=quota.miloapis.com,resources=claimcreationpolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=quota.miloapis.com,resources=claimcreationpolicies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=quota.miloapis.com,resources=resourceregistrations,verbs=get;list;watch

// Reconcile reconciles a ClaimCreationPolicy object by validating it and setting its Ready status.
// The controller's sole responsibility is validation - the PolicyEngine (in the admission plugin)
// watches for policies with Ready=True status condition.
func (r *ClaimCreationPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var policy quotav1alpha1.ClaimCreationPolicy
	logger.V(1).Info("Reconciling ClaimCreationPolicy", "name", req.Name)

	if err := r.Get(ctx, req.NamespacedName, &policy); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("ClaimCreationPolicy was deleted", "name", req.Name)
			// Policy was deleted - nothing to do (PolicyEngine will handle via watch)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to get ClaimCreationPolicy: %w", err)
	}

	// Store original status to detect changes
	originalStatus := policy.Status.DeepCopy()

	// Validate full policy (templates and resource types)
	validationErr := r.validatePolicy(ctx, &policy)

	// Update policy status based on validation results
	r.updatePolicyStatus(&policy, validationErr)

	// Update status if it has changed
	if !equality.Semantic.DeepEqual(&policy.Status, originalStatus) {
		policy.Status.ObservedGeneration = policy.Generation
		if err := r.Status().Update(ctx, &policy); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update ClaimCreationPolicy status: %w", err)
		}
		logger.V(1).Info("Updated ClaimCreationPolicy status",
			"policy", policy.Name,
			"ready", apimeta.IsStatusConditionTrue(policy.Status.Conditions, quotav1alpha1.ClaimCreationPolicyReady))
	}

	return ctrl.Result{}, nil
}

// validateResourceTypes validates that all resource types in the policy correspond to active ResourceRegistrations.
func (r *ClaimCreationPolicyReconciler) validateResourceTypes(ctx context.Context, policy *quotav1alpha1.ClaimCreationPolicy) error {
	// Validate each unique resource type using the shared validator
	seen := make(map[string]bool)
	for _, requestTemplate := range policy.Spec.Target.ResourceClaimTemplate.Spec.Requests {
		resourceType := requestTemplate.ResourceType
		if !seen[resourceType] {
			seen[resourceType] = true
			if err := r.ResourceTypeValidator.ValidateResourceType(ctx, resourceType); err != nil {
				return err
			}
		}
	}
	return nil
}

// updatePolicyStatus updates the policy status conditions based on validation results.
func (r *ClaimCreationPolicyReconciler) updatePolicyStatus(policy *quotav1alpha1.ClaimCreationPolicy, validationErr error) {
	now := metav1.NewTime(time.Now())

	if validationErr != nil {
		// Validation failed
		apimeta.SetStatusCondition(&policy.Status.Conditions, metav1.Condition{
			Type:               quotav1alpha1.ClaimCreationPolicyReady,
			Status:             metav1.ConditionFalse,
			Reason:             quotav1alpha1.ClaimCreationPolicyValidationFailedReason,
			Message:            validationErr.Error(),
			LastTransitionTime: now,
		})
		return
	}

	if policy.Spec.Disabled != nil && *policy.Spec.Disabled {
		// Policy is disabled
		apimeta.SetStatusCondition(&policy.Status.Conditions, metav1.Condition{
			Type:               quotav1alpha1.ClaimCreationPolicyReady,
			Status:             metav1.ConditionFalse,
			Reason:             quotav1alpha1.ClaimCreationPolicyDisabledReason,
			Message:            "Policy is disabled",
			LastTransitionTime: now,
		})
		return
	}

	// Validation passed and policy is enabled
	apimeta.SetStatusCondition(&policy.Status.Conditions, metav1.Condition{
		Type:               quotav1alpha1.ClaimCreationPolicyReady,
		Status:             metav1.ConditionTrue,
		Reason:             quotav1alpha1.ClaimCreationPolicyReadyReason,
		Message:            "Policy is ready and active",
		LastTransitionTime: now,
	})
}

// enqueueAffectedPolicies finds all ClaimCreationPolicies that reference a ResourceRegistration
// and enqueues them for reconciliation when the registration changes.
func (r *ClaimCreationPolicyReconciler) enqueueAffectedPolicies(ctx context.Context, obj client.Object) []reconcile.Request {
	registration, ok := obj.(*quotav1alpha1.ResourceRegistration)
	if !ok {
		return nil
	}

	// List all ClaimCreationPolicies
	var policyList quotav1alpha1.ClaimCreationPolicyList
	if err := r.List(ctx, &policyList); err != nil {
		// Log error but don't block - policies will be revalidated on their regular schedule
		return nil
	}

	var requests []reconcile.Request
	// Find policies that reference this resource type
	for _, policy := range policyList.Items {
		for _, request := range policy.Spec.Target.ResourceClaimTemplate.Spec.Requests {
			if request.ResourceType == registration.Spec.ResourceType {
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
func (r *ClaimCreationPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&quotav1alpha1.ClaimCreationPolicy{}).
		// Watch ResourceRegistrations to revalidate policies when registrations change
		Watches(&quotav1alpha1.ResourceRegistration{}, handler.EnqueueRequestsFromMapFunc(
			r.enqueueAffectedPolicies,
		)).
		Named("claim-creation-policy").
		Complete(r)
}

// validatePolicy performs validation of templates and resource types.
func (r *ClaimCreationPolicyReconciler) validatePolicy(ctx context.Context, policy *quotav1alpha1.ClaimCreationPolicy) error {
	// Validate claim template structure and template syntax
	if r.ClaimTemplateValidator != nil {
		if err := r.ClaimTemplateValidator.ValidateClaimTemplate(policy.Spec.Target.ResourceClaimTemplate); err != nil {
			return fmt.Errorf("claim template validation failed: %v", err)
		}
	}
	// Validate resource types against ResourceRegistrations
	if err := r.validateResourceTypes(ctx, policy); err != nil {
		return err
	}
	return nil
}
