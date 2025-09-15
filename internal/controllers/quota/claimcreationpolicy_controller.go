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
)

// ClaimCreationPolicyReconciler reconciles a ClaimCreationPolicy object
// Its sole responsibility is to validate the policy and set the Ready status condition.
// The PolicyEngine (used only by the admission plugin) watches for policies with Ready=True.
type ClaimCreationPolicyReconciler struct {
	client.Client
	Scheme *runtime.Scheme
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

	// Set defaults
	if policy.Spec.Enabled == nil {
		enabled := true
		policy.Spec.Enabled = &enabled
	}

	// Validate resource types against ResourceRegistrations
	validationErr := r.validateResourceTypes(ctx, &policy)

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

	// Requeue after a reasonable interval for periodic validation
	return ctrl.Result{RequeueAfter: time.Minute * 10}, nil
}

// validateResourceTypes validates that all resource types in the policy correspond to active ResourceRegistrations
func (r *ClaimCreationPolicyReconciler) validateResourceTypes(ctx context.Context, policy *quotav1alpha1.ClaimCreationPolicy) error {
	logger := log.FromContext(ctx)

	for _, requestTemplate := range policy.Spec.ResourceClaimTemplate.Requests {
		resourceType := requestTemplate.ResourceType

		// Find the ResourceRegistration for this resource type
		var registrations quotav1alpha1.ResourceRegistrationList
		if err := r.List(ctx, &registrations); err != nil {
			return fmt.Errorf("failed to list ResourceRegistrations: %w", err)
		}

		var registration *quotav1alpha1.ResourceRegistration
		for _, reg := range registrations.Items {
			if reg.Spec.ResourceType == resourceType {
				registration = &reg
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

		logger.V(2).Info("Resource type validation passed", "resourceType", resourceType, "registration", registration.Name)
	}

	return nil
}

// updatePolicyStatus updates the policy status conditions based on validation results
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

	if policy.Spec.Enabled != nil && !*policy.Spec.Enabled {
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
// and enqueues them for reconciliation when the registration changes
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
		for _, request := range policy.Spec.ResourceClaimTemplate.Requests {
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
