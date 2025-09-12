// AllowanceBucketController owns the lifecycle of AllowanceBuckets.
//
// Centralizing bucket writes in a single controller avoids write races between
// controllers and keeps reconciliation predictable. This controller
// materializes aggregates for fast, O(1) evaluation by:
// - Computing Limit from Active ResourceGrants (scoped to the same owner)
// - Computing Allocated from Granted ResourceClaims (same owner + shape)
// Buckets are created on demand when claims reference them to minimize churn
// while still handling eventual consistency between controllers.
package quota

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	quotav1alpha1 "go.miloapis.com/milo/pkg/apis/quota/v1alpha1"
)

const (
	// BucketNameLabel is added to ResourceClaims to track which bucket they belong to
	BucketNameLabel = "quota.miloapis.com/bucket"
	// BucketGenerationLabel tracks the bucket generation when the claim was processed
	BucketGenerationLabel = "quota.miloapis.com/bucket-generation"
)

// AllowanceBucketController reconciles AllowanceBucket objects and maintains
// aggregated quota (Limit/Allocated/Available). It is the single writer for
// bucket objects; other controllers read-only. See docs/architecture/quota-system.md.
type AllowanceBucketController struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=quota.miloapis.com,resources=allowancebuckets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=quota.miloapis.com,resources=allowancebuckets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=quota.miloapis.com,resources=resourcegrants,verbs=get;list;watch
// +kubebuilder:rbac:groups=quota.miloapis.com,resources=resourceclaims,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=quota.miloapis.com,resources=resourceclaims/status,verbs=get;update;patch

// Reconcile maintains AllowanceBucket limits and usage aggregates by watching
// ResourceGrants and ResourceClaims.
func (r *AllowanceBucketController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Get the AllowanceBucket
	var bucket quotav1alpha1.AllowanceBucket
	if err := r.Get(ctx, req.NamespacedName, &bucket); err != nil {
		if apierrors.IsNotFound(err) {
			// Single-writer: attempt to create the bucket if a claim references it
			if created, cerr := r.ensureBucketFromClaims(ctx, req); cerr != nil {
				return ctrl.Result{}, cerr
			} else if !created {
				logger.V(1).Info("AllowanceBucket not found and no claims reference it; skipping", "bucket", req.NamespacedName)
				return ctrl.Result{}, nil
			}
			// Re-fetch after creation
			if gerr := r.Get(ctx, req.NamespacedName, &bucket); gerr != nil {
				return ctrl.Result{}, fmt.Errorf("failed to get AllowanceBucket after create: %w", gerr)
			}
		} else {
			return ctrl.Result{}, fmt.Errorf("failed to get AllowanceBucket: %w", err)
		}
	}

	// Create a deep copy of the original status
	originalStatus := bucket.Status.DeepCopy()

	// Update observed generation
	bucket.Status.ObservedGeneration = bucket.Generation

	// Calculate limits from ResourceGrants
	if err := r.updateLimitsFromGrants(ctx, &bucket); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update limits from grants: %w", err)
	}

	// Calculate usage from ResourceClaims
	if err := r.updateUsageFromClaims(ctx, &bucket); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update usage from claims: %w", err)
	}

	// Try to grant pending claims by reserving capacity with a CAS update on the bucket
	if err := r.processPendingGrants(ctx, &bucket); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed processing pending grants: %w", err)
	}

	// Calculate available quota after any reservations
	// Ensure Available is never negative (validation constraint)
	available := bucket.Status.Limit - bucket.Status.Allocated
	if available < 0 {
		available = 0
	}
	bucket.Status.Available = available

	// Update last reconciliation time
	now := metav1.Now()
	bucket.Status.LastReconciliation = &now

	// Update status if it has changed
	if !statusEqual(originalStatus, &bucket.Status) {
		if err := r.Status().Update(ctx, &bucket); err != nil {
			if apierrors.IsConflict(err) {
				// Someone else updated the bucket; requeue shortly to reconcile again
				return ctrl.Result{RequeueAfter: 100 * time.Millisecond}, nil
			}
			return ctrl.Result{}, fmt.Errorf("failed to update AllowanceBucket status: %w", err)
		}

		logger.Info("Updated AllowanceBucket status",
			"name", bucket.Name,
			"namespace", bucket.Namespace,
			"limit", bucket.Status.Limit,
			"allocated", bucket.Status.Allocated,
			"available", bucket.Status.Available,
			"claimCount", bucket.Status.ClaimCount,
			"grantCount", bucket.Status.GrantCount)
	}

	return ctrl.Result{}, nil
}

// updateLimitsFromGrants calculates the total limit from all applicable ResourceGrants
func (r *AllowanceBucketController) updateLimitsFromGrants(ctx context.Context, bucket *quotav1alpha1.AllowanceBucket) error {
	logger := log.FromContext(ctx)

	// List all ResourceGrants in the same namespace
	var grants quotav1alpha1.ResourceGrantList
	if err := r.List(ctx, &grants, client.InNamespace(bucket.Namespace)); err != nil {
		return fmt.Errorf("failed to list ResourceGrants: %w", err)
	}

	var totalLimit int64
	var contributingGrants []quotav1alpha1.ContributingGrantRef

	for _, grant := range grants.Items {
		// Only consider active grants
		if !r.isResourceGrantActive(&grant) {
			continue
		}

		// Prevent cross-owner mixing by requiring an exact owner match
		if grant.Spec.ConsumerRef.Kind != bucket.Spec.ConsumerRef.Kind ||
			grant.Spec.ConsumerRef.Name != bucket.Spec.ConsumerRef.Name {
			continue
		}

		// Check if this grant applies to this bucket
		for _, allowance := range grant.Spec.Allowances {
			if allowance.ResourceType != bucket.Spec.ResourceType {
				continue
			}

			// Check each bucket in the allowance
			for _, allowanceBucket := range allowance.Buckets {
				if r.dimensionSelectorMatches(allowanceBucket.DimensionSelector, bucket.Spec.Dimensions) {
					totalLimit += allowanceBucket.Amount
					contributingGrants = append(contributingGrants, quotav1alpha1.ContributingGrantRef{
						Name:                   grant.Name,
						LastObservedGeneration: grant.Generation,
						Amount:                 allowanceBucket.Amount,
					})
					logger.V(1).Info("Grant contributes to bucket",
						"grantName", grant.Name,
						"amount", allowanceBucket.Amount,
						"resourceType", allowance.ResourceType)
				}
			}
		}
	}

	bucket.Status.Limit = totalLimit
	bucket.Status.GrantCount = int32(len(contributingGrants))
	bucket.Status.ContributingGrantRefs = contributingGrants

	return nil
}

// updateUsageFromClaims calculates the total allocated usage from ResourceClaims
func (r *AllowanceBucketController) updateUsageFromClaims(ctx context.Context, bucket *quotav1alpha1.AllowanceBucket) error {
	logger := log.FromContext(ctx)

	// Find all ResourceClaims that use this bucket
	var claims quotav1alpha1.ResourceClaimList
	if err := r.List(ctx, &claims, client.InNamespace(bucket.Namespace)); err != nil {
		return fmt.Errorf("failed to list ResourceClaims: %w", err)
	}

	var totalAllocated int64
	var claimCount int32

	for _, claim := range claims.Items {
		// Only consider granted claims
		if !r.isResourceClaimGranted(&claim) {
			continue
		}

		// Owner must match
		if claim.Spec.ConsumerRef.Kind != bucket.Spec.ConsumerRef.Kind ||
			claim.Spec.ConsumerRef.Name != bucket.Spec.ConsumerRef.Name {
			logger.V(1).Info("Claim consumer does not match bucket consumer, skipping",
				"claimName", claim.Name,
				"claimConsumer", claim.Spec.ConsumerRef.Kind+"/"+claim.Spec.ConsumerRef.Name,
				"bucketConsumer", bucket.Spec.ConsumerRef.Kind+"/"+bucket.Spec.ConsumerRef.Name)
			continue
		}

		logger.V(1).Info("Claim consumer matches bucket, evaluating requests",
			"claimName", claim.Name,
			"consumer", claim.Spec.ConsumerRef.Kind+"/"+claim.Spec.ConsumerRef.Name)

		// Claims must match owner and shape (pre-dimension removal)
		for _, request := range claim.Spec.Requests {
			if request.ResourceType != bucket.Spec.ResourceType {
				continue
			}

			// Check if dimensions match
			if r.dimensionsMatch(bucket.Spec.Dimensions, request.Dimensions) {
				totalAllocated += request.Amount
				claimCount++

				// Label the claim so we can find it later
				if err := r.labelClaimForBucket(ctx, &claim, bucket.Name); err != nil {
					logger.Error(err, "Failed to label claim", "claimName", claim.Name)
					// Don't fail reconciliation for labeling errors
				}

				logger.V(1).Info("Claim contributes to bucket",
					"claimName", claim.Name,
					"amount", request.Amount,
					"resourceType", request.ResourceType)
			}
		}
	}

	bucket.Status.Allocated = totalAllocated
	bucket.Status.ClaimCount = claimCount

	return nil
}

// ensureBucketFromClaims creates the bucket spec from a referencing claim if found.
func (r *AllowanceBucketController) ensureBucketFromClaims(ctx context.Context, req ctrl.Request) (bool, error) {
	var claims quotav1alpha1.ResourceClaimList
	if err := r.List(ctx, &claims, client.InNamespace(req.Namespace)); err != nil {
		return false, fmt.Errorf("failed to list ResourceClaims: %w", err)
	}
	for _, claim := range claims.Items {
		for _, request := range claim.Spec.Requests {
			name := GenerateAllowanceBucketName(claim.Namespace, request.ResourceType, claim.Spec.ConsumerRef, request.Dimensions)
			if name == req.Name {
				// create bucket
				bucket := &quotav1alpha1.AllowanceBucket{
					ObjectMeta: metav1.ObjectMeta{
						Name:      req.Name,
						Namespace: req.Namespace,
						Labels: func() map[string]string {
							apiGroup, kind := parseResourceType(request.ResourceType)
							labels := map[string]string{
								"quota.miloapis.com/resource-kind": kind,
								"quota.miloapis.com/consumer-kind": claim.Spec.ConsumerRef.Kind,
								"quota.miloapis.com/consumer-name": claim.Spec.ConsumerRef.Name,
							}
							if apiGroup != "" {
								labels["quota.miloapis.com/resource-apigroup"] = apiGroup
							}
							return labels
						}(),
					},
					Spec: quotav1alpha1.AllowanceBucketSpec{
						ConsumerRef:  claim.Spec.ConsumerRef,
						ResourceType: request.ResourceType,
						Dimensions:   request.Dimensions,
					},
				}
				if err := r.Create(ctx, bucket); err != nil && !apierrors.IsAlreadyExists(err) {
					return false, fmt.Errorf("failed to create AllowanceBucket %s: %w", req.Name, err)
				}
				return true, nil
			}
		}
	}
	return false, nil
}

// labelClaimForBucket adds a label to track which bucket a claim belongs to
func (r *AllowanceBucketController) labelClaimForBucket(ctx context.Context, claim *quotav1alpha1.ResourceClaim, bucketName string) error {
	if claim.Labels == nil {
		claim.Labels = make(map[string]string)
	}

	// Only update if label is missing or different
	if claim.Labels[BucketNameLabel] != bucketName {
		claim.Labels[BucketNameLabel] = bucketName
		claim.Labels[BucketGenerationLabel] = fmt.Sprintf("%d", claim.Generation)

		return r.Update(ctx, claim)
	}

	return nil
}

// isResourceGrantActive checks if a ResourceGrant has an Active condition with status True
func (r *AllowanceBucketController) isResourceGrantActive(grant *quotav1alpha1.ResourceGrant) bool {
	for _, condition := range grant.Status.Conditions {
		if condition.Type == quotav1alpha1.ResourceGrantActive && condition.Status == metav1.ConditionTrue {
			return true
		}
	}
	return false
}

// isResourceClaimGranted checks if a ResourceClaim has a Granted condition with status True
func (r *AllowanceBucketController) isResourceClaimGranted(claim *quotav1alpha1.ResourceClaim) bool {
	for _, condition := range claim.Status.Conditions {
		if condition.Type == quotav1alpha1.ResourceClaimGranted && condition.Status == metav1.ConditionTrue {
			return true
		}
	}
	return false
}

// dimensionSelectorMatches checks if a dimension selector matches the given dimensions
func (r *AllowanceBucketController) dimensionSelectorMatches(selector metav1.LabelSelector, dimensions map[string]string) bool {
	// Empty selector matches everything
	if len(selector.MatchLabels) == 0 && len(selector.MatchExpressions) == 0 {
		return true
	}

	// Convert to Kubernetes label selector
	labelSelector, err := metav1.LabelSelectorAsSelector(&selector)
	if err != nil {
		return false
	}

	return labelSelector.Matches(labels.Set(dimensions))
}

// dimensionsMatch checks if two dimension maps are equal
func (r *AllowanceBucketController) dimensionsMatch(bucketDimensions, requestDimensions map[string]string) bool {
	if len(bucketDimensions) != len(requestDimensions) {
		return false
	}

	for key, value := range bucketDimensions {
		if requestDimensions[key] != value {
			return false
		}
	}

	return true
}

// statusEqual compares two AllowanceBucketStatus for equality
func statusEqual(a, b *quotav1alpha1.AllowanceBucketStatus) bool {
	return a.ObservedGeneration == b.ObservedGeneration &&
		a.Limit == b.Limit &&
		a.Allocated == b.Allocated &&
		a.Available == b.Available &&
		a.ClaimCount == b.ClaimCount &&
		a.GrantCount == b.GrantCount
}

// processPendingGrants attempts to grant pending claims that reference this bucket.
// For each eligible claim, it first reserves capacity via a CAS status update to the
// bucket, then marks the claim Granted on success.
func (r *AllowanceBucketController) processPendingGrants(ctx context.Context, bucket *quotav1alpha1.AllowanceBucket) error {
	logger := log.FromContext(ctx)
	var claims quotav1alpha1.ResourceClaimList
	if err := r.List(ctx, &claims, client.InNamespace(bucket.Namespace)); err != nil {
		return fmt.Errorf("failed to list ResourceClaims: %w", err)
	}

	// Current state for available calculation during this reconcile loop
	limit := bucket.Status.Limit
	allocated := bucket.Status.Allocated

	for _, claim := range claims.Items {
		logger.V(1).Info("Processing claim in pending grants",
			"claimName", claim.Name,
			"claimConsumer", claim.Spec.ConsumerRef.Kind+"/"+claim.Spec.ConsumerRef.Name,
			"claimAPIGroup", claim.Spec.ConsumerRef.APIGroup,
			"bucketConsumer", bucket.Spec.ConsumerRef.Kind+"/"+bucket.Spec.ConsumerRef.Name,
			"bucketAPIGroup", bucket.Spec.ConsumerRef.APIGroup)

		if r.isResourceClaimGranted(&claim) {
			logger.V(1).Info("Claim already granted, skipping", "claimName", claim.Name)
			continue
		}

		// Skip claims explicitly denied
		if cond := apimeta.FindStatusCondition(claim.Status.Conditions, quotav1alpha1.ResourceClaimGranted); cond != nil {
			if cond.Status == metav1.ConditionFalse && cond.Reason == quotav1alpha1.ResourceClaimDeniedReason {
				logger.V(1).Info("Claim explicitly denied, skipping",
					"claimName", claim.Name,
					"reason", cond.Reason,
					"message", cond.Message)
				continue
			}
		}

		// Owner must match
		if claim.Spec.ConsumerRef.Kind != bucket.Spec.ConsumerRef.Kind ||
			claim.Spec.ConsumerRef.Name != bucket.Spec.ConsumerRef.Name {
			continue
		}

		// Sum only requests that match this bucket's resourceType and dimensions
		var reqTotal int64
		for _, req := range claim.Spec.Requests {
			if req.ResourceType != bucket.Spec.ResourceType {
				logger.V(2).Info("Request resource type does not match bucket",
					"claimName", claim.Name,
					"requestType", req.ResourceType,
					"bucketType", bucket.Spec.ResourceType)
				continue
			}
			if !r.dimensionsMatch(bucket.Spec.Dimensions, req.Dimensions) {
				logger.V(2).Info("Request dimensions do not match bucket",
					"claimName", claim.Name,
					"requestDimensions", req.Dimensions,
					"bucketDimensions", bucket.Spec.Dimensions)
				continue
			}
			reqTotal += req.Amount
			logger.V(1).Info("Adding request to total",
				"claimName", claim.Name,
				"requestAmount", req.Amount,
				"runningTotal", reqTotal)
		}
		if reqTotal == 0 {
			logger.V(1).Info("No matching requests found for claim", "claimName", claim.Name)
			continue
		}

		// Check availability using current local view
		logger.V(1).Info("Evaluating quota availability",
			"claimName", claim.Name,
			"requestTotal", reqTotal,
			"bucketLimit", limit,
			"bucketAllocated", allocated,
			"available", limit-allocated)
		if limit-allocated < reqTotal {
			logger.Info("Insufficient quota available for claim",
				"claimName", claim.Name,
				"requestTotal", reqTotal,
				"available", limit-allocated)
			continue
		}

		// Attempt to reserve capacity with a CAS update; retry a few times on conflict
		logger.V(1).Info("Attempting to reserve quota capacity",
			"claimName", claim.Name,
			"requestTotal", reqTotal,
			"currentAllocated", allocated,
			"newAllocated", allocated+reqTotal)
		for i := 0; i < 3; i++ {
			// Reserve capacity and keep status fields self-consistent for validation
			bucket.Status.Allocated = allocated + reqTotal
			// Recompute Available with clamp to satisfy CRD validation (Minimum=0)
			avail := bucket.Status.Limit - bucket.Status.Allocated
			if avail < 0 {
				avail = 0
			}
			bucket.Status.Available = avail
			bucket.Status.ObservedGeneration = bucket.Generation
			if err := r.Status().Update(ctx, bucket); err != nil {
				if apierrors.IsConflict(err) {
					// Refresh local view and try again
					var latest quotav1alpha1.AllowanceBucket
					if gerr := r.Get(ctx, types.NamespacedName{Name: bucket.Name, Namespace: bucket.Namespace}, &latest); gerr != nil {
						return fmt.Errorf("failed to refetch bucket after conflict: %w", gerr)
					}
					*bucket = latest
					limit = bucket.Status.Limit
					allocated = bucket.Status.Allocated
					if limit-allocated < reqTotal {
						// Capacity gone
						break
					}
					continue
				}
				return fmt.Errorf("failed to update bucket during reservation: %w", err)
			}
			// Reservation successful; update local allocated for subsequent claims
			allocated = bucket.Status.Allocated

			// Mark claim Granted using Server-Side Apply to avoid conflicts
			// Set the granted condition
			apimeta.SetStatusCondition(&claim.Status.Conditions, metav1.Condition{
				Type:    quotav1alpha1.ResourceClaimGranted,
				Status:  metav1.ConditionTrue,
				Reason:  quotav1alpha1.ResourceClaimGrantedReason,
				Message: "Capacity reserved",
			})

			// Use Server-Side Apply to update the claim status
			// This avoids conflicts by using field ownership
			patch := client.MergeFrom(&quotav1alpha1.ResourceClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      claim.Name,
					Namespace: claim.Namespace,
				},
			})

			// Apply the status update with field manager ownership
			if err := r.Status().Patch(ctx, &claim, patch, client.FieldOwner("allowance-bucket-controller")); err != nil {
				logger.Error(err, "failed to patch claim status after reservation",
					"claim", claim.Name)
				// Don't revert the bucket allocation - the capacity has been reserved
				// The claim will be reconciled again and matched with the allocation
				return fmt.Errorf("failed to patch claim status: %w", err)
			}

			// Successfully updated claim status
			logger.V(1).Info("Successfully marked claim as granted using SSA",
				"claim", claim.Name)
			break
		}
	}
	return nil
}

// parseResourceType splits a resourceType like "resourcemanager.miloapis.com/Project"
// into apiGroup and kind components
func parseResourceType(resourceType string) (apiGroup, kind string) {
	parts := strings.Split(resourceType, "/")
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	// Handle core resources (no API group)
	return "", resourceType
}

// generateAllowanceBucketName creates a deterministic name for an AllowanceBucket
func GenerateAllowanceBucketName(namespace, resourceType string, ownerRef quotav1alpha1.ConsumerRef, dimensions map[string]string) string {
	// Why: using namespace|resourceType|owner yields stable, per-owner buckets and
	// avoids cross-owner collisions. Dimensions are included for current
	// compatibility and will be removed once dimensions are dropped from the API.
	// The name is truncated to remain DNS-safe.
	dimensionsBytes, _ := json.Marshal(dimensions)
	input := fmt.Sprintf("%s%s%s%s%s", namespace, resourceType, ownerRef.Kind, ownerRef.Name, string(dimensionsBytes))
	hash := sha256.Sum256([]byte(input))
	return fmt.Sprintf("bucket-%x", hash)[:19]
}

// SetupWithManager sets up the controller with the Manager.
func (r *AllowanceBucketController) SetupWithManager(mgr ctrl.Manager) error {
    // Watch buckets directly; react to grants/claims that change aggregates.
    return ctrl.NewControllerManagedBy(mgr).
        For(&quotav1alpha1.AllowanceBucket{}).
        // Watch ResourceGrants that affect bucket limits
        Watches(
            &quotav1alpha1.ResourceGrant{},
            handler.EnqueueRequestsFromMapFunc(r.enqueueAffectedBuckets),
            builder.WithPredicates(predicate.Funcs{
                CreateFunc: func(e event.CreateEvent) bool {
                    // React on new grants so newly active grants can be picked up
                    return true
                },
                UpdateFunc: func(e event.UpdateEvent) bool {
                    // Trigger on spec (generation) changes or active status flips
                    oldGrant := e.ObjectOld.(*quotav1alpha1.ResourceGrant)
                    newGrant := e.ObjectNew.(*quotav1alpha1.ResourceGrant)
                    if oldGrant.Generation != newGrant.Generation {
                        return true
                    }
                    oldActive := apimeta.IsStatusConditionTrue(oldGrant.Status.Conditions, quotav1alpha1.ResourceGrantActive)
                    newActive := apimeta.IsStatusConditionTrue(newGrant.Status.Conditions, quotav1alpha1.ResourceGrantActive)
                    return oldActive != newActive
                },
            }),
        ).
        // Watch ResourceClaims that affect bucket usage
        Watches(
            &quotav1alpha1.ResourceClaim{},
            handler.EnqueueRequestsFromMapFunc(r.enqueueAffectedBuckets),
            builder.WithPredicates(predicate.Funcs{
                CreateFunc: func(e event.CreateEvent) bool {
                    // React immediately on new claims to create buckets on demand
                    return true
                },
                UpdateFunc: func(e event.UpdateEvent) bool {
                    // Trigger on spec changes or any Granted condition status/reason change
                    oldClaim := e.ObjectOld.(*quotav1alpha1.ResourceClaim)
                    newClaim := e.ObjectNew.(*quotav1alpha1.ResourceClaim)
                    if oldClaim.Generation != newClaim.Generation {
                        return true
                    }
                    oldCond := apimeta.FindStatusCondition(oldClaim.Status.Conditions, quotav1alpha1.ResourceClaimGranted)
                    newCond := apimeta.FindStatusCondition(newClaim.Status.Conditions, quotav1alpha1.ResourceClaimGranted)
                    if (oldCond == nil) != (newCond == nil) {
                        return true
                    }
                    if oldCond != nil && newCond != nil {
                        return oldCond.Status != newCond.Status || oldCond.Reason != newCond.Reason
                    }
                    return false
                },
            }),
        ).
        Named("allowance-bucket").
        Complete(r)
}

// enqueueAffectedBuckets determines which AllowanceBuckets need to be reconciled
// when ResourceGrants or ResourceClaims change
func (r *AllowanceBucketController) enqueueAffectedBuckets(ctx context.Context, obj client.Object) []reconcile.Request {
	logger := log.FromContext(ctx)
	var requests []reconcile.Request

	switch o := obj.(type) {
	case *quotav1alpha1.ResourceGrant:
		// For each allowance in the grant, find matching buckets
		for _, allowance := range o.Spec.Allowances {
			buckets, err := r.findBucketsForResourceType(ctx, o.Namespace, allowance.ResourceType, o.Spec.ConsumerRef)
			if err != nil {
				logger.Error(err, "Failed to find buckets for grant", "grantName", o.Name)
				continue
			}
			for _, bucket := range buckets {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      bucket.Name,
						Namespace: bucket.Namespace,
					},
				})
			}
		}

	case *quotav1alpha1.ResourceClaim:
		// For each request in the claim, find matching buckets
		for _, request := range o.Spec.Requests {
			bucketName := GenerateAllowanceBucketName(o.Namespace, request.ResourceType, o.Spec.ConsumerRef, request.Dimensions)
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      bucketName,
					Namespace: o.Namespace,
				},
			})
		}
	}

	return requests
}

// findBucketsForResourceType finds all AllowanceBuckets for a given resource type and owner
func (r *AllowanceBucketController) findBucketsForResourceType(ctx context.Context, namespace, resourceType string, ownerRef quotav1alpha1.ConsumerRef) ([]quotav1alpha1.AllowanceBucket, error) {
	var buckets quotav1alpha1.AllowanceBucketList
	if err := r.List(ctx, &buckets, client.InNamespace(namespace)); err != nil {
		return nil, err
	}

	var matchingBuckets []quotav1alpha1.AllowanceBucket
	for _, bucket := range buckets.Items {
		if bucket.Spec.ResourceType == resourceType &&
			bucket.Spec.ConsumerRef.Kind == ownerRef.Kind &&
			bucket.Spec.ConsumerRef.Name == ownerRef.Name {
			matchingBuckets = append(matchingBuckets, bucket)
		}
	}

	return matchingBuckets, nil
}
