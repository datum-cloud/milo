package quota

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"

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

// ResourceClaimController reconciles a ResourceClaim object and is
// responsible for evaluating resource claims against available quota.
type ResourceClaimController struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=quota.miloapis.com,resources=resourcegrants,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=quota.miloapis.com,resources=resourceclaims/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=quota.miloapis.com,resources=resourcegrants,verbs=get;list;watch
// +kubebuilder:rbac:groups=quota.miloapis.com,resources=resourceregistrations,verbs=get;list;watch
// +kubebuilder:rbac:groups=quota.miloapis.com,resources=allowancebuckets,verbs=get;list;watch
//
// Reconciles a ResourceClaim object by evaluating the requests against the available quota.
func (r *ResourceClaimController) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, err error) {
	logger := log.FromContext(ctx)

	// Fetch the ResourceClaim
	var claim quotav1alpha1.ResourceClaim
	if err := r.Get(ctx, types.NamespacedName{Name: req.Name, Namespace: req.Namespace}, &claim); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("ResourceClaim not found")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to get ResourceClaim: %w", err)
	}

	if !claim.DeletionTimestamp.IsZero() {
		logger.Info("ResourceClaim is being deleted, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	// Create a deep copy of the original status to compare against later
	originalStatus := claim.Status.DeepCopy()

	// Always update the observed generation in the status to match the current generation of the spec.
	claim.Status.ObservedGeneration = claim.Generation

	// Validate that all required registrations exist before evaluating the claim
	if err := r.validateResourceRegistrationsForClaim(ctx, &claim); err != nil {
		logger.Info("ResourceClaim validation failed", "error", err)

		// Set the condition to indicate validation failed
		validationFailedCondition := metav1.Condition{
			Type:    quotav1alpha1.ResourceClaimGranted,
			Status:  metav1.ConditionFalse,
			Reason:  quotav1alpha1.ResourceClaimValidationFailedReason,
			Message: fmt.Sprintf("Validation failed: %v", err),
		}
		apimeta.SetStatusCondition(&claim.Status.Conditions, validationFailedCondition)

		// Update status and return
		if err := r.Status().Update(ctx, &claim); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update ResourceClaim status: %w", err)
		}
		return ctrl.Result{}, nil
	}

	// Evaluate each resource request within the claim's spec.
	allRequestsGranted := true
	// Variable to store the outcome message for each request evaluation.
	var evaluationMessages []string

	// Iterate through the requests in the claim's spec and evaluate them
	for i, request := range claim.Spec.Requests {
		// Log the start of the evaluation for the current request
		logger.Info("evaluating resource request",
			"requestIndex", i,
			"resourceType", request.ResourceType,
			"amount", request.Amount,
			"dimensions", request.Dimensions)

		// Evaluate the request to determine if there is enough quota available
		granted, message, err := r.evaluateResourceRequest(ctx, &claim, request)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to evaluate resource request %d: %w", i, err)
		}

		// If any single request is not granted, the entire claim is considered not granted.
		if !granted {
			allRequestsGranted = false
		}
		// Append the result message from the evaluation to the list of messages.
		evaluationMessages = append(evaluationMessages, message)
	}

	// Set the 'Granted' condition on the status based on the overall evaluation result.
	var grantedCondition metav1.Condition
	// If all requests were granted, set the condition to True.
	if allRequestsGranted {
		grantedCondition = metav1.Condition{
			Type:    quotav1alpha1.ResourceClaimGranted,
			Status:  metav1.ConditionTrue,
			Reason:  quotav1alpha1.ResourceClaimGrantedReason,
			Message: "Claim granted due to quota availability",
		}
	} else {
		// If any of the requests were denied, set the condition to False.
		grantedCondition = metav1.Condition{
			Type:    quotav1alpha1.ResourceClaimGranted,
			Status:  metav1.ConditionFalse,
			Reason:  quotav1alpha1.ResourceClaimDeniedReason,
			Message: "Claim denied as it would exceed the currently set quota limit.",
		}
	}

	apimeta.SetStatusCondition(&claim.Status.Conditions, grantedCondition)

	statusChanged := claim.Status.ObservedGeneration != originalStatus.ObservedGeneration

	// Check if the 'Granted' condition has changed
	currentGrantedCondition := apimeta.FindStatusCondition(claim.Status.Conditions, quotav1alpha1.ResourceClaimGranted)
	originalGrantedCondition := apimeta.FindStatusCondition(originalStatus.Conditions, quotav1alpha1.ResourceClaimGranted)

	conditionChanged := false
	// Compare current and original conditions
	if currentGrantedCondition != nil && originalGrantedCondition != nil {
		conditionChanged = currentGrantedCondition.Status != originalGrantedCondition.Status ||
			currentGrantedCondition.Reason != originalGrantedCondition.Reason ||
			currentGrantedCondition.Message != originalGrantedCondition.Message
	} else if currentGrantedCondition != originalGrantedCondition {
		conditionChanged = true
	}

	// If the status has changed, update it
	if statusChanged || conditionChanged {
		if err := r.Status().Update(ctx, &claim); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update ResourceClaim status: %w", err)
		}

		logger.Info("updated ResourceClaim status",
			"name", claim.Name,
			"namespace", claim.Namespace,
			"granted", allRequestsGranted,
			"requestCount", len(claim.Spec.Requests))
	}

	return ctrl.Result{}, nil
}

// evaluateResourceRequest evaluates a single resource request against the available quota
// by reading AllowanceBuckets for usage and active ResourceGrants for limits.
// It returns a boolean indicating if the request was granted, a message describing the result,
// and an error if the evaluation fails.
func (r *ResourceClaimController) evaluateResourceRequest(ctx context.Context, claim *quotav1alpha1.ResourceClaim, request quotav1alpha1.ResourceRequest) (bool, string, error) {
	// Get a logger for this context.
	logger := log.FromContext(ctx)

	// Determine current usage from the specific AllowanceBucket that matches
	// the resource claim request
	bucketName := r.generateAllowanceBucketName(claim.Namespace, request.ResourceType, request.Dimensions)
	var bucket quotav1alpha1.AllowanceBucket
	currentUsage := int64(0)

	if err := r.Get(ctx, client.ObjectKey{
		Namespace: claim.Namespace,
		Name:      bucketName,
	}, &bucket); err != nil {
		if !apierrors.IsNotFound(err) {
			return false, "", fmt.Errorf("failed to get AllowanceBucket: %w", err)
		}
		logger.Info("AllowanceBucket not found as resources have not been used", "bucketName", bucketName)
	} else {
		// Set the allocated amount as current usage
		currentUsage = bucket.Status.Allocated
	}

	// Calculate applicable limit from active ResourceGrants
	var grants quotav1alpha1.ResourceGrantList
	if err := r.List(ctx, &grants, client.InNamespace(claim.Namespace)); err != nil {
		return false, "", fmt.Errorf("failed to list ResourceGrants: %w", err)
	}

	var totalEffectiveLimit int64
	for _, grant := range grants.Items {
		// Only consider active grants
		if !r.isResourceGrantActive(&grant) {
			continue
		}

		// Check each allowance in the grant
		for _, allowance := range grant.Spec.Allowances {
			if allowance.ResourceType != request.ResourceType {
				continue
			}

			// Check each bucket in the allowance
			for _, allowanceBucket := range allowance.Buckets {
				// Check if this allowance's dimensionSelector matches the claim's dimensions
				if r.dimensionSelectorMatches(allowanceBucket.DimensionSelector, request.Dimensions) {
					totalEffectiveLimit += allowanceBucket.Amount
				}
			}
		}
	}

	// Evaluate quota
	logger.Info("quota evaluation",
		"resourceType", request.ResourceType,
		"requestAmount", request.Amount,
		"currentUsage", currentUsage,
		"totalEffectiveLimit", totalEffectiveLimit,
		"available", totalEffectiveLimit-currentUsage)

	if currentUsage+request.Amount <= totalEffectiveLimit {
		// Claim can be granted due to quota availability
		message := fmt.Sprintf("Granted %d units of %s (current usage: %d, applicable limit: %d, available: %d)",
			request.Amount, request.ResourceType, currentUsage, totalEffectiveLimit, totalEffectiveLimit-currentUsage)

		logger.Info("resource claim request granted",
			"resourceType", request.ResourceType,
			"requestAmount", request.Amount,
			"currentUsage", currentUsage,
			"totalEffectiveLimit", totalEffectiveLimit)

		return true, message, nil
	} else {
		// Request exceeds available quota
		available := totalEffectiveLimit - currentUsage
		message := fmt.Sprintf("Denied %d units of %s - would exceed quota (current usage: %d, applicable limit: %d, available: %d)",
			request.Amount, request.ResourceType, currentUsage, totalEffectiveLimit, available)

		logger.Info("resource claim request denied",
			"resourceType", request.ResourceType,
			"requestAmount", request.Amount,
			"currentUsage", currentUsage,
			"totalEffectiveLimit", totalEffectiveLimit)

		return false, message, nil
	}
}

// isResourceGrantActive checks if a ResourceGrant has an Active condition with status True
func (r *ResourceClaimController) isResourceGrantActive(grant *quotav1alpha1.ResourceGrant) bool {
	for _, condition := range grant.Status.Conditions {
		if condition.Type == quotav1alpha1.ResourceGrantActive && condition.Status == metav1.ConditionTrue {
			return true
		}
	}
	return false
}

// Checks if a dimension selector matches the given dimensions.
// Empty selector matches all dimensions
func (r *ResourceClaimController) dimensionSelectorMatches(selector metav1.LabelSelector, dimensions map[string]string) bool {
	// Empty selector matches everything
	if len(selector.MatchLabels) == 0 && len(selector.MatchExpressions) == 0 {
		return true
	}

	// Check MatchLabels
	for key, value := range selector.MatchLabels {
		if dimensions[key] != value {
			return false
		}
	}

	return true
}

// Creates a deterministic name for AllowanceBucket using hash,
// which should match the logic in EffectiveResourceGrant controller.
func (r *ResourceClaimController) generateAllowanceBucketName(namespace, resourceType string, dimensions map[string]string) string {
	dimensionsBytes, _ := json.Marshal(dimensions)

	// Create hash of namespace + resourceType + dimensions
	input := fmt.Sprintf("%s%s%s", namespace, resourceType, string(dimensionsBytes))
	hash := sha256.Sum256([]byte(input))

	// Return first part of hex hash for readability
	return fmt.Sprintf("bucket-%x", hash)[:19]
}

// validateResourceRegistrations validates that all resource types in the claim
// have corresponding registrations.
func (r *ResourceClaimController) validateResourceRegistrationsForClaim(ctx context.Context, claim *quotav1alpha1.ResourceClaim) error {
	// Collect all unique resource type names from requests to be passed into
	// the ValidateResourceRegistrations function.
	var resourceTypes []string
	for _, request := range claim.Spec.Requests {
		resourceTypes = append(resourceTypes, request.ResourceType)
	}

	return ValidateResourceRegistrations(ctx, r.Client, resourceTypes)
}

// Create and register controller with controller manager
func (r *ResourceClaimController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		// Specify that this controller watches the ResourceClaim resource type as its primary resource.
		For(&quotav1alpha1.ResourceClaim{}).
		Named("resource-claim").
		Complete(r)
}
