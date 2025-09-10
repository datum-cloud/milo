package quota

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
// quota limits and usage aggregates.
type AllowanceBucketController struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=quota.miloapis.com,resources=allowancebuckets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=quota.miloapis.com,resources=allowancebuckets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=quota.miloapis.com,resources=resourcegrants,verbs=get;list;watch
// +kubebuilder:rbac:groups=quota.miloapis.com,resources=resourceclaims,verbs=get;list;watch;update;patch

// Reconcile maintains AllowanceBucket limits and usage aggregates by watching
// ResourceGrants and ResourceClaims.
func (r *AllowanceBucketController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Get the AllowanceBucket
	var bucket quotav1alpha1.AllowanceBucket
	if err := r.Get(ctx, req.NamespacedName, &bucket); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("AllowanceBucket not found, may have been deleted")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to get AllowanceBucket: %w", err)
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

	// Calculate available quota
	bucket.Status.Available = bucket.Status.Limit - bucket.Status.Allocated

	// Update last reconciliation time
	now := metav1.Now()
	bucket.Status.LastReconciliation = &now

	// Update status if it has changed
	if !statusEqual(originalStatus, &bucket.Status) {
		if err := r.Status().Update(ctx, &bucket); err != nil {
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

		// Check if this grant applies to this bucket
		for _, allowance := range grant.Spec.Allowances {
			if allowance.ResourceType != bucket.Spec.ResourceType {
				continue
			}

			// Check each bucket in the allowance
			for _, allowanceBucket := range allowance.Buckets {
				if r.dimensionSelectorMatches(allowanceBucket.DimensionSelector, bucket.Spec.Dimensions) {
					totalLimit += allowanceBucket.Amount

					// Track contributing grant
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

		// Check if this claim uses this bucket
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

// generateAllowanceBucketName creates a deterministic name for an AllowanceBucket
func GenerateAllowanceBucketName(namespace, resourceType string, ownerRef quotav1alpha1.OwnerInstanceRef, dimensions map[string]string) string {
	dimensionsBytes, _ := json.Marshal(dimensions)
	input := fmt.Sprintf("%s%s%s%s%s", namespace, resourceType, ownerRef.Kind, ownerRef.Name, string(dimensionsBytes))
	hash := sha256.Sum256([]byte(input))
	return fmt.Sprintf("bucket-%x", hash)[:19]
}

// SetupWithManager sets up the controller with the Manager.
func (r *AllowanceBucketController) SetupWithManager(mgr ctrl.Manager) error {
	// Watch AllowanceBuckets directly
	return ctrl.NewControllerManagedBy(mgr).
		For(&quotav1alpha1.AllowanceBucket{}).
		// Watch ResourceGrants that affect bucket limits
		Watches(
			&quotav1alpha1.ResourceGrant{},
			handler.EnqueueRequestsFromMapFunc(r.enqueueAffectedBuckets),
			builder.WithPredicates(predicate.Funcs{
				UpdateFunc: func(e event.UpdateEvent) bool {
					// Only trigger on status changes (active/inactive)
					return e.ObjectOld.GetGeneration() != e.ObjectNew.GetGeneration()
				},
			}),
		).
		// Watch ResourceClaims that affect bucket usage
		Watches(
			&quotav1alpha1.ResourceClaim{},
			handler.EnqueueRequestsFromMapFunc(r.enqueueAffectedBuckets),
			builder.WithPredicates(predicate.Funcs{
				UpdateFunc: func(e event.UpdateEvent) bool {
					// Trigger on generation or status changes
					oldClaim := e.ObjectOld.(*quotav1alpha1.ResourceClaim)
					newClaim := e.ObjectNew.(*quotav1alpha1.ResourceClaim)
					return oldClaim.Generation != newClaim.Generation ||
						r.isResourceClaimGranted(oldClaim) != r.isResourceClaimGranted(newClaim)
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
			buckets, err := r.findBucketsForResourceType(ctx, o.Namespace, allowance.ResourceType, o.Spec.OwnerInstanceRef)
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
			bucketName := GenerateAllowanceBucketName(o.Namespace, request.ResourceType, o.Spec.OwnerInstanceRef, request.Dimensions)
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
func (r *AllowanceBucketController) findBucketsForResourceType(ctx context.Context, namespace, resourceType string, ownerRef quotav1alpha1.OwnerInstanceRef) ([]quotav1alpha1.AllowanceBucket, error) {
	var buckets quotav1alpha1.AllowanceBucketList
	if err := r.List(ctx, &buckets, client.InNamespace(namespace)); err != nil {
		return nil, err
	}

	var matchingBuckets []quotav1alpha1.AllowanceBucket
	for _, bucket := range buckets.Items {
		if bucket.Spec.ResourceType == resourceType &&
			bucket.Spec.OwnerInstanceRef.Kind == ownerRef.Kind &&
			bucket.Spec.OwnerInstanceRef.Name == ownerRef.Name {
			matchingBuckets = append(matchingBuckets, bucket)
		}
	}

	return matchingBuckets, nil
}