// ResourceClaimController evaluates quota requests.
//
// This controller reads aggregated AllowanceBucket state to keep decisions O(1).
// To tolerate eventual consistency between controllers, it falls back to
// recomputing limit/allocated when a bucket is not yet available. Bucket
// lifecycle is centralized in AllowanceBucketController to avoid write races;
// this controller never writes buckets.
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
	"sigs.k8s.io/controller-runtime/pkg/log"

	quotav1alpha1 "go.miloapis.com/milo/pkg/apis/quota/v1alpha1"
)

// ResourceClaimController reconciles a ResourceClaim object and is
// responsible for evaluating resource claims against available quota.
type ResourceClaimController struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=quota.miloapis.com,resources=resourceclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=quota.miloapis.com,resources=resourceclaims/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=quota.miloapis.com,resources=resourcegrants,verbs=get;list;watch
// +kubebuilder:rbac:groups=quota.miloapis.com,resources=resourceregistrations,verbs=get;list;watch
// +kubebuilder:rbac:groups=quota.miloapis.com,resources=allowancebuckets,verbs=get;list;watch

// Reconciles a ResourceClaim object by evaluating the requests against the available quota
// and updating the overall Granted condition based on individual request allocations.
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

	// Evaluate each resource request within the claim's spec for basic validation
	// AllowanceBucketControllers are responsible for actual granting/denying individual requests
	for i, request := range claim.Spec.Requests {
		logger.V(1).Info("evaluating resource request",
			"requestIndex", i,
			"resourceType", request.ResourceType,
			"amount", request.Amount,
			"dimensions", request.Dimensions)

		// Basic validation and eligibility check
		_, message, err := r.evaluateResourceRequest(ctx, &claim, request)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to evaluate resource request %d: %w", i, err)
		}

		_ = message // result used only for logging above
	}

	// Update the overall claim condition based on individual request allocations
	if err := r.updateOverallClaimConditionFromAllocations(ctx, &claim); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update overall claim condition: %w", err)
	}

	return ctrl.Result{}, nil
}

// evaluateResourceRequest evaluates a single resource request against the available quota
// by finding or creating the appropriate AllowanceBucket and checking available quota.
// It returns a boolean indicating if the request was granted, a message describing the result,
// and an error if the evaluation fails.
func (r *ResourceClaimController) evaluateResourceRequest(ctx context.Context, claim *quotav1alpha1.ResourceClaim, request quotav1alpha1.ResourceRequest) (bool, string, error) {
	logger := log.FromContext(ctx)

	// Prefer reading the aggregated bucket for O(1) decisions
	bucketName := GenerateAllowanceBucketName(claim.Namespace, request.ResourceType, claim.Spec.ConsumerRef, request.Dimensions)
	var bucket quotav1alpha1.AllowanceBucket
	if err := r.Get(ctx, client.ObjectKey{Namespace: claim.Namespace, Name: bucketName}, &bucket); err == nil {
		available := bucket.Status.Limit - bucket.Status.Allocated
		if available < 0 {
			available = 0
		}
		if available >= request.Amount {
			message := fmt.Sprintf("Granted %d units of %s (current usage: %d, limit: %d, available: %d)", request.Amount, request.ResourceType, bucket.Status.Allocated, bucket.Status.Limit, available)
			logger.Info("resource claim request granted",
				"resourceType", request.ResourceType,
				"requestAmount", request.Amount,
				"bucketAllocated", bucket.Status.Allocated,
				"bucketLimit", bucket.Status.Limit,
				"bucketAvailable", available)
			return true, message, nil
		}
		message := fmt.Sprintf("Denied %d units of %s - would exceed quota (current usage: %d, limit: %d, available: %d)", request.Amount, request.ResourceType, bucket.Status.Allocated, bucket.Status.Limit, available)
		logger.Info("resource claim request denied",
			"resourceType", request.ResourceType,
			"requestAmount", request.Amount,
			"bucketAllocated", bucket.Status.Allocated,
			"bucketLimit", bucket.Status.Limit,
			"bucketAvailable", available)
		return false, message, nil
	} else if !apierrors.IsNotFound(err) {
		return false, "", fmt.Errorf("failed to get AllowanceBucket: %w", err)
	}

	// Fallback: recompute limits/usage when a bucket hasn't been reconciled yet
	limit, allocated, ferr := r.computeLimitAndAllocated(ctx, claim, request)
	if ferr != nil {
		return false, "", ferr
	}
	available := limit - allocated
	if available < 0 {
		available = 0
	}
	if available >= request.Amount {
		message := fmt.Sprintf("Granted %d units of %s (current usage: %d, limit: %d, available: %d)", request.Amount, request.ResourceType, allocated, limit, available)
		logger.Info("resource claim request granted (fallback)", "resourceType", request.ResourceType, "allocated", allocated, "limit", limit)
		return true, message, nil
	}
	message := fmt.Sprintf("Denied %d units of %s - would exceed quota (current usage: %d, limit: %d, available: %d)", request.Amount, request.ResourceType, allocated, limit, available)
	logger.Info("resource claim request denied (fallback)", "resourceType", request.ResourceType, "allocated", allocated, "limit", limit)
	return false, message, nil
}

// Buckets are not created in this controller. Centralizing writes in
// AllowanceBucketController avoids write races across controllers.

// computeLimitAndAllocated recomputes limits/usage as a consistency safety net.
// Used only when a bucket hasn't been reconciled yet; not the hot path.
// for the given owner/resourceType/dimensions combination.
func (r *ResourceClaimController) computeLimitAndAllocated(ctx context.Context, claim *quotav1alpha1.ResourceClaim, request quotav1alpha1.ResourceRequest) (int64, int64, error) {
	// Compute limit
	var grants quotav1alpha1.ResourceGrantList
	if err := r.List(ctx, &grants, client.InNamespace(claim.Namespace)); err != nil {
		return 0, 0, fmt.Errorf("failed to list ResourceGrants: %w", err)
	}
	var totalLimit int64
	for _, grant := range grants.Items {
		if !r.isResourceGrantActive(&grant) {
			continue
		}
		if grant.Spec.ConsumerRef.Kind != claim.Spec.ConsumerRef.Kind || grant.Spec.ConsumerRef.Name != claim.Spec.ConsumerRef.Name {
			continue
		}
		for _, allowance := range grant.Spec.Allowances {
			if allowance.ResourceType != request.ResourceType {
				continue
			}
			for _, ab := range allowance.Buckets {
				if r.dimensionSelectorMatches(ab.DimensionSelector, request.Dimensions) {
					totalLimit += ab.Amount
				}
			}
		}
	}
	// Compute allocated
	var claims quotav1alpha1.ResourceClaimList
	if err := r.List(ctx, &claims, client.InNamespace(claim.Namespace)); err != nil {
		return 0, 0, fmt.Errorf("failed to list ResourceClaims: %w", err)
	}
	var allocated int64
	for _, c := range claims.Items {
		// Granted only
		granted := false
		for _, cond := range c.Status.Conditions {
			if cond.Type == quotav1alpha1.ResourceClaimGranted && cond.Status == metav1.ConditionTrue {
				granted = true
				break
			}
		}
		if !granted {
			continue
		}
		if c.Spec.ConsumerRef.Kind != claim.Spec.ConsumerRef.Kind || c.Spec.ConsumerRef.Name != claim.Spec.ConsumerRef.Name {
			continue
		}
		for _, req := range c.Spec.Requests {
			if req.ResourceType != request.ResourceType {
				continue
			}
			// Dimensions must match exactly
			if len(req.Dimensions) != len(request.Dimensions) {
				continue
			}
			match := true
			for k, v := range request.Dimensions {
				if req.Dimensions[k] != v {
					match = false
					break
				}
			}
			if match {
				allocated += req.Amount
			}
		}
	}
	return totalLimit, allocated, nil
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

// generateAllowanceBucketName creates a deterministic name for AllowanceBucket
// Deprecated: Use GenerateAllowanceBucketName from allowancebucket_controller.go instead
func (r *ResourceClaimController) generateAllowanceBucketName(namespace, resourceType string, dimensions map[string]string) string {
	// For backwards compatibility during transition
	ownerRef := quotav1alpha1.ConsumerRef{
		Kind: "Unknown",
		Name: "unknown",
	}
	return GenerateAllowanceBucketName(namespace, resourceType, ownerRef, dimensions)
}

// updateOverallClaimConditionFromAllocations updates the overall Granted condition
// based on the status of individual request allocations
func (r *ResourceClaimController) updateOverallClaimConditionFromAllocations(ctx context.Context, claim *quotav1alpha1.ResourceClaim) error {
	logger := log.FromContext(ctx)

	// Initialize allocation map for tracking which requests have been processed
	allocationMap := make(map[string]quotav1alpha1.RequestAllocation)
	for _, allocation := range claim.Status.Allocations {
		allocationMap[allocation.ResourceType] = allocation
	}

	var grantedCount, deniedCount, pendingCount int
	var totalRequests = len(claim.Spec.Requests)

	// Check the status of each request by resource type
	for _, request := range claim.Spec.Requests {
		allocation, exists := allocationMap[request.ResourceType]
		if !exists {
			// No allocation status exists for this request - mark as pending
			pendingCount++
			continue
		}

		switch allocation.Status {
		case quotav1alpha1.RequestAllocationGranted:
			grantedCount++
		case quotav1alpha1.RequestAllocationDenied:
			deniedCount++
		case quotav1alpha1.RequestAllocationPending:
			pendingCount++
		default:
			// Unknown status - treat as pending
			pendingCount++
		}
	}

	logger.V(1).Info("Allocation summary",
		"claimName", claim.Name,
		"totalRequests", totalRequests,
		"granted", grantedCount,
		"denied", deniedCount,
		"pending", pendingCount)

	// Determine overall condition based on allocation results
	var conditionStatus metav1.ConditionStatus
	var reason, message string

	if grantedCount == totalRequests {
		// All requests granted
		conditionStatus = metav1.ConditionTrue
		reason = quotav1alpha1.ResourceClaimGrantedReason
		message = fmt.Sprintf("All %d resource requests have been granted", totalRequests)
	} else if deniedCount > 0 {
		// At least one request denied
		conditionStatus = metav1.ConditionFalse
		reason = quotav1alpha1.ResourceClaimDeniedReason
		message = fmt.Sprintf("Resource quota exceeded: %d granted, %d denied, %d pending", grantedCount, deniedCount, pendingCount)
	} else {
		// Some requests still pending
		conditionStatus = metav1.ConditionFalse
		reason = quotav1alpha1.ResourceClaimPendingReason
		message = fmt.Sprintf("Awaiting capacity evaluation: %d granted, %d pending", grantedCount, pendingCount)
	}

	return r.updateOverallClaimCondition(ctx, claim, conditionStatus, reason, message)
}

// updateOverallClaimCondition updates the overall Granted condition using Server Side Apply
func (r *ResourceClaimController) updateOverallClaimCondition(ctx context.Context, claim *quotav1alpha1.ResourceClaim,
	status metav1.ConditionStatus, reason, message string) error {

	logger := log.FromContext(ctx)

	// Check if condition needs updating
	existingCondition := apimeta.FindStatusCondition(claim.Status.Conditions, quotav1alpha1.ResourceClaimGranted)
	if existingCondition != nil &&
		existingCondition.Status == status &&
		existingCondition.Reason == reason &&
		existingCondition.Message == message {
		// No change needed
		logger.V(2).Info("Overall claim condition unchanged, skipping update",
			"claimName", claim.Name, "status", status, "reason", reason)
		return nil
	}

	// Create condition update
	condition := metav1.Condition{
		Type:               quotav1alpha1.ResourceClaimGranted,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	}

	// Create minimal claim object for Server Side Apply
	patchClaim := &quotav1alpha1.ResourceClaim{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "quota.miloapis.com/v1alpha1",
			Kind:       "ResourceClaim",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      claim.Name,
			Namespace: claim.Namespace,
		},
		Status: quotav1alpha1.ResourceClaimStatus{
			ObservedGeneration: claim.Generation,
			Conditions:         []metav1.Condition{condition},
		},
	}

	// Apply the patch using Server Side Apply with our field manager
	fieldManagerName := "resource-claim-controller"
	if err := r.Status().Patch(ctx, patchClaim, client.Apply, client.FieldOwner(fieldManagerName), client.ForceOwnership); err != nil {
		return fmt.Errorf("failed to apply overall claim condition: %w", err)
	}

	logger.Info("Successfully updated overall claim condition",
		"claimName", claim.Name,
		"status", status,
		"reason", reason,
		"message", message)

	return nil
}

// Create and register controller with controller manager
func (r *ResourceClaimController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		// Specify that this controller watches the ResourceClaim resource type as its primary resource.
		For(&quotav1alpha1.ResourceClaim{}).
		Named("resource-claim").
		Complete(r)
}
