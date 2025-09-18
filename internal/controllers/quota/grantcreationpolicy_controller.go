package quota

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

	quotav1alpha1 "go.miloapis.com/milo/pkg/apis/quota/v1alpha1"
	"go.miloapis.com/milo/internal/validation/quota"
)

// GrantCreationPolicyReconciler reconciles a GrantCreationPolicy object.
// Its responsibility is to validate the policy and set the Ready status condition.
// The PolicyEngine watches for policies with Ready=True to include them in grant creation.
type GrantCreationPolicyReconciler struct {
	client.Client
	Scheme            *runtime.Scheme
	CELValidator      *quota.CELValidator
	TemplateValidator *quota.GrantTemplateValidator
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

	// Set defaults
	if policy.Spec.Enabled == nil {
		enabled := true
		policy.Spec.Enabled = &enabled
	}

	// Perform comprehensive validation
	validationErr := r.validatePolicy(ctx, &policy)

	// Update policy status based on validation results
	r.updatePolicyStatus(&policy, validationErr)

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

	// Requeue after a reasonable interval for periodic validation
	return ctrl.Result{RequeueAfter: time.Minute * 10}, nil
}

// validatePolicy performs comprehensive validation of the GrantCreationPolicy.
func (r *GrantCreationPolicyReconciler) validatePolicy(ctx context.Context, policy *quotav1alpha1.GrantCreationPolicy) error {
	logger := log.FromContext(ctx)

	// Skip validation if policy is disabled
	if policy.Spec.Enabled != nil && !*policy.Spec.Enabled {
		logger.V(2).Info("Policy is disabled, skipping validation", "policy", policy.Name)
		return nil
	}

	// Validate CEL expressions in trigger conditions
	if err := r.CELValidator.ValidateConditions(policy.Spec.Trigger.Conditions); err != nil {
		return fmt.Errorf("trigger condition validation failed: %w", err)
	}

	// Validate parent context name expression if specified
	if policy.Spec.Target.ParentContext != nil {
		if err := r.CELValidator.ValidateNameExpression(policy.Spec.Target.ParentContext.NameExpression); err != nil {
			return fmt.Errorf("parent context name expression validation failed: %w", err)
		}
	}

	// Validate grant template structure (including resource type validation)
	if err := r.TemplateValidator.ValidateGrantTemplate(ctx, policy.Spec.Target.ResourceGrantTemplate); err != nil {
		return fmt.Errorf("grant template validation failed: %w", err)
	}

	logger.V(2).Info("Policy validation passed", "policy", policy.Name)
	return nil
}

// ValidateResourceType implements the ResourceTypeValidator interface.
// It validates that a single resource type corresponds to an active ResourceRegistration.
func (r *GrantCreationPolicyReconciler) ValidateResourceType(ctx context.Context, resourceType string) error {
	logger := log.FromContext(ctx)

	// Get all resource registrations
	var registrations quotav1alpha1.ResourceRegistrationList
	if err := r.List(ctx, &registrations); err != nil {
		return fmt.Errorf("failed to list ResourceRegistrations: %w", err)
	}

	// Find the registration for this resource type
	var registration *quotav1alpha1.ResourceRegistration
	for i := range registrations.Items {
		reg := &registrations.Items[i]
		if reg.Spec.ResourceType == resourceType {
			registration = reg
			break
		}
	}

	if registration == nil {
		return fmt.Errorf("resource type '%s' is not registered - please create a ResourceRegistration for this resource type", resourceType)
	}

	// Check if the ResourceRegistration is active
	activeCondition := apimeta.FindStatusCondition(registration.Status.Conditions, quotav1alpha1.ResourceRegistrationActive)
	if activeCondition == nil {
		return fmt.Errorf("resource type '%s' is not ready - the ResourceRegistration is missing status conditions", resourceType)
	}

	if activeCondition.Status != metav1.ConditionTrue {
		return fmt.Errorf("resource type '%s' is not active - ResourceRegistration status: %s (%s)",
			resourceType, activeCondition.Status, activeCondition.Reason)
	}

	logger.V(2).Info("Resource type validation passed",
		"resourceType", resourceType,
		"registration", registration.Name)

	return nil
}

// updatePolicyStatus updates the policy status conditions based on validation results.
func (r *GrantCreationPolicyReconciler) updatePolicyStatus(policy *quotav1alpha1.GrantCreationPolicy, validationErr error) {
	now := metav1.NewTime(time.Now())

	if validationErr != nil {
		// Validation failed
		apimeta.SetStatusCondition(&policy.Status.Conditions, metav1.Condition{
			Type:               quotav1alpha1.GrantCreationPolicyReady,
			Status:             metav1.ConditionFalse,
			Reason:             quotav1alpha1.GrantCreationPolicyValidationFailedReason,
			Message:            validationErr.Error(),
			LastTransitionTime: now,
		})

		// Clear parent context ready condition if validation failed
		apimeta.RemoveStatusCondition(&policy.Status.Conditions, quotav1alpha1.GrantCreationPolicyParentContextReady)
		return
	}

	if policy.Spec.Enabled != nil && !*policy.Spec.Enabled {
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
