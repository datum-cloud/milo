package quota

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	quotav1alpha1 "go.miloapis.com/milo/pkg/apis/quota/v1alpha1"
)

// EffectiveResourceGrantReconciler reconciles a EffectiveResourceGrant object
type EffectiveResourceGrantReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=quota.miloapis.com,resources=effectiveresourcegrants,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=quota.miloapis.com,resources=effectiveresourcegrants/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=quota.miloapis.com,resources=resourcegrants,verbs=get;list;watch
// +kubebuilder:rbac:groups=quota.miloapis.com,resources=resourceclaims,verbs=get;list;watch
// +kubebuilder:rbac:groups=quota.miloapis.com,resources=allowancebuckets,verbs=get;list;watch;create;update;patch;delete

// Reconcile manages the lifecycle of EffectiveResourceGrant objects.
func (r *EffectiveResourceGrantReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Get the object that triggered this reconciliation
	var effectiveGrant quotav1alpha1.EffectiveResourceGrant
	if err := r.Get(ctx, req.NamespacedName, &effectiveGrant); err != nil {
		if errors.IsNotFound(err) {
			// EffectiveResourceGrant doesn't exist - check if we should create it
			// This can happen when a ResourceGrant references a new resource type
			return r.createEffectiveResourceGrants(ctx, req.NamespacedName)
		}
		logger.Error(err, "Failed to get EffectiveResourceGrant")
		return ctrl.Result{}, err
	}

	// Create a deep copy of the original status to compare against later
	originalStatus := effectiveGrant.Status.DeepCopy()

	// Always update the observed generation to match the current spec
	effectiveGrant.Status.ObservedGeneration = effectiveGrant.Generation

	// Get the current ready condition from the status or create a new one
	readyCondition := apimeta.FindStatusCondition(effectiveGrant.Status.Conditions, quotav1alpha1.EffectiveResourceGrantReady)
	if readyCondition == nil {
		readyCondition = &metav1.Condition{
			Type:               quotav1alpha1.EffectiveResourceGrantReady,
			Status:             metav1.ConditionFalse,
			Reason:             quotav1alpha1.EffectiveResourceGrantAggregationPendingReason,
			Message:            "Aggregation is pending",
			ObservedGeneration: effectiveGrant.Generation,
		}
	} else {
		readyCondition = readyCondition.DeepCopy()
		readyCondition.ObservedGeneration = effectiveGrant.Generation
	}

	// Create AllowanceBuckets for all granted ResourceClaims
	if err := r.createAllowanceBucketsForGrantedClaims(ctx, &effectiveGrant); err != nil {
		readyCondition.Status = metav1.ConditionFalse
		readyCondition.Reason = quotav1alpha1.EffectiveResourceGrantAggregationFailedReason
		readyCondition.Message = fmt.Sprintf("Failed to create AllowanceBuckets: %v", err)
		apimeta.SetStatusCondition(&effectiveGrant.Status.Conditions, *readyCondition)

		if err := r.Status().Update(ctx, &effectiveGrant); err != nil {
			logger.Error(err, "Failed to update EffectiveResourceGrant status after bucket creation failure")
		}
		return ctrl.Result{}, fmt.Errorf("failed to create AllowanceBuckets: %w", err)
	}

	// Aggregate total limit from all active ResourceGrants in the same namespace with matching resourceTypeName
	totalLimit, err := r.aggregateTotalLimit(ctx, effectiveGrant.Namespace, effectiveGrant.Spec.ResourceTypeName)
	if err != nil {
		readyCondition.Status = metav1.ConditionFalse
		readyCondition.Reason = quotav1alpha1.EffectiveResourceGrantAggregationFailedReason
		readyCondition.Message = fmt.Sprintf("Failed to aggregate total limit: %v", err)
		apimeta.SetStatusCondition(&effectiveGrant.Status.Conditions, *readyCondition)

		if err := r.Status().Update(ctx, &effectiveGrant); err != nil {
			logger.Error(err, "Failed to update EffectiveResourceGrant status after aggregation failure")
		}
		return ctrl.Result{}, fmt.Errorf("failed to aggregate total limit: %w", err)
	}

	// Calculate total allocated from all owned AllowanceBuckets
	totalAllocated, bucketRefs, err := r.calculateTotalAllocated(ctx, &effectiveGrant)
	if err != nil {
		readyCondition.Status = metav1.ConditionFalse
		readyCondition.Reason = quotav1alpha1.EffectiveResourceGrantAggregationFailedReason
		readyCondition.Message = fmt.Sprintf("Failed to calculate total allocated: %v", err)
		apimeta.SetStatusCondition(&effectiveGrant.Status.Conditions, *readyCondition)

		if err := r.Status().Update(ctx, &effectiveGrant); err != nil {
			logger.Error(err, "Failed to update EffectiveResourceGrant status after calculation failure")
		}
		return ctrl.Result{}, fmt.Errorf("failed to calculate total allocated: %w", err)
	}

	// Update EffectiveResourceGrant status
	effectiveGrant.Status.TotalLimit = totalLimit
	effectiveGrant.Status.TotalAllocated = totalAllocated
	effectiveGrant.Status.Available = totalLimit - totalAllocated
	effectiveGrant.Status.AllowanceBucketRefs = bucketRefs

	// Set ready condition to true
	readyCondition.Status = metav1.ConditionTrue
	readyCondition.Reason = quotav1alpha1.EffectiveResourceGrantAggregationCompleteReason
	readyCondition.Message = "Aggregation completed successfully"

	apimeta.SetStatusCondition(&effectiveGrant.Status.Conditions, *readyCondition)

	// Only update the status if something has actually changed
	if !equality.Semantic.DeepEqual(originalStatus, &effectiveGrant.Status) {
		if err := r.Status().Update(ctx, &effectiveGrant); err != nil {
			logger.Error(err, "Failed to update EffectiveResourceGrant status")
			return ctrl.Result{}, err
		}
		logger.Info("Successfully updated EffectiveResourceGrant",
			"totalLimit", totalLimit,
			"totalAllocated", totalAllocated,
			"available", totalLimit-totalAllocated)
	}

	return ctrl.Result{}, nil
}

// Handles the case where an EffectiveResourceGrant doesn't exist but should be
// created based on existing ResourceGrants
func (r *EffectiveResourceGrantReconciler) createEffectiveResourceGrants(ctx context.Context, namespacedName types.NamespacedName) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Extract resource type name from the EffectiveResourceGrant name
	// This is a reverse lookup - we need to find which ResourceGrants might need this EffectiveResourceGrant
	var grants quotav1alpha1.ResourceGrantList
	if err := r.List(ctx, &grants, client.InNamespace(namespacedName.Namespace)); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to list ResourceGrants: %w", err)
	}

	// Check if any ResourceGrant would create this EffectiveResourceGrant
	for _, grant := range grants.Items {
		for _, allowance := range grant.Spec.Allowances {
			expectedName := r.generateEffectiveResourceGrantName(grant.Namespace, allowance.ResourceTypeName)
			if expectedName == namespacedName.Name {
				// Found a matching ResourceGrant - create the EffectiveResourceGrant
				effectiveGrant := quotav1alpha1.EffectiveResourceGrant{
					ObjectMeta: metav1.ObjectMeta{
						Name:      namespacedName.Name,
						Namespace: namespacedName.Namespace,
					},
					Spec: quotav1alpha1.EffectiveResourceGrantSpec{
						ResourceTypeName: allowance.ResourceTypeName,
					},
				}

				if err := r.Create(ctx, &effectiveGrant); err != nil {
					logger.Error(err, "Failed to create EffectiveResourceGrant", "name", namespacedName.Name)
					return ctrl.Result{}, err
				}
				logger.Info("Created EffectiveResourceGrant", "name", namespacedName.Name)
				// Trigger a reconcile of the newly created object
				return ctrl.Result{Requeue: true}, nil
			}
		}
	}

	// No matching ResourceGrant found, nothing to do
	return ctrl.Result{}, nil
}

// ensureRequiredAllowanceBuckets ensures that all necessary AllowanceBuckets exist
// for granted ResourceClaims affecting this EffectiveResourceGrant
func (r *EffectiveResourceGrantReconciler) createAllowanceBucketsForGrantedClaims(ctx context.Context, effectiveGrant *quotav1alpha1.EffectiveResourceGrant) error {
	// Ensure buckets for ResourceClaims
	var claims quotav1alpha1.ResourceClaimList
	if err := r.List(ctx, &claims, client.InNamespace(effectiveGrant.Namespace)); err != nil {
		return fmt.Errorf("failed to list ResourceClaims: %w", err)
	}

	for _, claim := range claims.Items {
		// Check if claim is granted and continue to next claim if not, as allowance buckets are only created for granted claims
		if !r.isResourceClaimGranted(&claim) {
			continue
		}

		for _, request := range claim.Spec.Requests {
			if request.ResourceTypeName == effectiveGrant.Spec.ResourceTypeName {
				if err := r.createAllowanceBucket(ctx, effectiveGrant, request.ResourceTypeName, request.Dimensions); err != nil {
					return err
				}
				// Recalculate bucket allocation for this request
				if err := r.recalculateBucketAllocation(ctx, effectiveGrant, request.ResourceTypeName, request.Dimensions); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// aggregateTotalLimit sums allowances from all active ResourceGrants with matching namespace and resourceTypeName
func (r *EffectiveResourceGrantReconciler) aggregateTotalLimit(ctx context.Context, namespace, resourceTypeName string) (int64, error) {
	var grants quotav1alpha1.ResourceGrantList
	if err := r.List(ctx, &grants, client.InNamespace(namespace)); err != nil {
		return 0, fmt.Errorf("failed to list ResourceGrants: %w", err)
	}

	var totalLimit int64
	for _, grant := range grants.Items {
		// Check if grant is active
		if !r.isResourceGrantActive(&grant) {
			continue
		}

		// Sum allowances for matching resource types
		for _, allowance := range grant.Spec.Allowances {
			if allowance.ResourceTypeName == resourceTypeName {
				for _, bucket := range allowance.Buckets {
					totalLimit += bucket.Amount
				}
			}
		}
	}

	return totalLimit, nil
}

// calculateTotalAllocated sums allocated amounts from all AllowanceBuckets owned by this EffectiveResourceGrant
func (r *EffectiveResourceGrantReconciler) calculateTotalAllocated(ctx context.Context, effectiveGrant *quotav1alpha1.EffectiveResourceGrant) (int64, []quotav1alpha1.AllowanceBucketRef, error) {
	var buckets quotav1alpha1.AllowanceBucketList
	if err := r.List(ctx, &buckets, client.InNamespace(effectiveGrant.Namespace)); err != nil {
		return 0, nil, fmt.Errorf("failed to list AllowanceBuckets: %w", err)
	}

	var totalAllocated int64
	var bucketRefs []quotav1alpha1.AllowanceBucketRef

	for _, bucket := range buckets.Items {
		// Only count buckets for this resource type that are owned by this EffectiveResourceGrant
		if bucket.Spec.ResourceTypeName == effectiveGrant.Spec.ResourceTypeName &&
			bucket.Spec.OwnerRef.Name == effectiveGrant.Name {
			totalAllocated += bucket.Status.Allocated
			bucketRefs = append(bucketRefs, quotav1alpha1.AllowanceBucketRef{
				Name:               bucket.Name,
				ObservedGeneration: bucket.Status.ObservedGeneration,
				Allocated:          bucket.Status.Allocated,
			})
		}
	}

	return totalAllocated, bucketRefs, nil
}

// isResourceGrantActive checks if a ResourceGrant has an Active condition with
// a status of True
func (r *EffectiveResourceGrantReconciler) isResourceGrantActive(grant *quotav1alpha1.ResourceGrant) bool {
	for _, condition := range grant.Status.Conditions {
		if condition.Type == quotav1alpha1.ResourceGrantActive && condition.Status == metav1.ConditionTrue {
			return true
		}
	}
	return false
}

// Determines if a ResourceClaim has been granted
func (r *EffectiveResourceGrantReconciler) isResourceClaimGranted(claim *quotav1alpha1.ResourceClaim) bool {
	for _, condition := range claim.Status.Conditions {
		if condition.Type == quotav1alpha1.ResourceClaimGranted && condition.Status == metav1.ConditionTrue {
			return true
		}
	}
	return false
}

// Creates an AllowanceBucket if it doesn't exist
func (r *EffectiveResourceGrantReconciler) createAllowanceBucket(ctx context.Context, effectiveGrant *quotav1alpha1.EffectiveResourceGrant, resourceTypeName string, dimensions map[string]string) error {
	log := log.FromContext(ctx)

	bucketName := r.generateAllowanceBucketName(effectiveGrant.Namespace, resourceTypeName, dimensions)

	var bucket quotav1alpha1.AllowanceBucket
	err := r.Get(ctx, types.NamespacedName{
		Name:      bucketName,
		Namespace: effectiveGrant.Namespace,
	}, &bucket)

	if errors.IsNotFound(err) {
		// Create new AllowanceBucket
		bucket = quotav1alpha1.AllowanceBucket{
			ObjectMeta: metav1.ObjectMeta{
				Name:      bucketName,
				Namespace: effectiveGrant.Namespace,
			},
			Spec: quotav1alpha1.AllowanceBucketSpec{
				OwnerRef: quotav1alpha1.OwnerRef{
					APIGroup: "quota.miloapis.com",
					Kind:     "EffectiveResourceGrant",
					Name:     effectiveGrant.Name,
				},
				ResourceTypeName: resourceTypeName,
				Dimensions:       dimensions,
			},
			Status: quotav1alpha1.AllowanceBucketStatus{
				Allocated: 0,
			},
		}

		if err := r.Create(ctx, &bucket); err != nil {
			log.Error(err, "Failed to create AllowanceBucket", "name", bucketName)
			return err
		}
		log.Info("Created AllowanceBucket", "name", bucketName)
	} else if err != nil {
		return err
	}

	return nil
}

// Recalculates the entire allocation for a bucket by scanning all claims.
func (r *EffectiveResourceGrantReconciler) recalculateBucketAllocation(ctx context.Context, effectiveGrant *quotav1alpha1.EffectiveResourceGrant, resourceTypeName string, dimensions map[string]string) error {
	log := log.FromContext(ctx)
	bucketName := r.generateAllowanceBucketName(effectiveGrant.Namespace, resourceTypeName, dimensions)

	var bucket quotav1alpha1.AllowanceBucket
	if err := r.Get(ctx, types.NamespacedName{Name: bucketName, Namespace: effectiveGrant.Namespace}, &bucket); err != nil {
		if errors.IsNotFound(err) {
			log.Info("AllowanceBucket not found during recalculation, skipping", "name", bucketName)
			return nil
		}
		return fmt.Errorf("failed to get AllowanceBucket %s: %w", bucketName, err)
	}

	originalAllocated := bucket.Status.Allocated

	// Recalculate allocation from scratch
	var newAllocated int64
	var allClaims quotav1alpha1.ResourceClaimList
	if err := r.List(ctx, &allClaims, client.InNamespace(effectiveGrant.Namespace)); err != nil {
		return fmt.Errorf("failed to list ResourceClaims for bucket recalculation: %w", err)
	}

	for _, claim := range allClaims.Items {
		// Only consider granted claims
		if !r.isResourceClaimGranted(&claim) {
			continue
		}

		// Find requests in the claim that match this bucket
		for _, request := range claim.Spec.Requests {
			if request.ResourceTypeName == resourceTypeName &&
				r.dimensionsMatch(request.Dimensions, dimensions) {
				newAllocated += request.Amount
			}
		}
	}

	if newAllocated != originalAllocated {
		bucket.Status.Allocated = newAllocated
		bucket.Status.ObservedGeneration = bucket.Generation
		if err := r.Status().Update(ctx, &bucket); err != nil {
			return fmt.Errorf("failed to update AllowanceBucket status for %s: %w", bucketName, err)
		}
	}

	return nil
}

// dimensionsMatch compares two dimension maps for equality.
func (r *EffectiveResourceGrantReconciler) dimensionsMatch(d1, d2 map[string]string) bool {
	if len(d1) != len(d2) {
		return false
	}
	for k, v1 := range d1 {
		if v2, ok := d2[k]; !ok || v1 != v2 {
			return false
		}
	}
	return true
}

// generateEffectiveResourceGrantName creates a deterministic name for EffectiveResourceGrant
func (r *EffectiveResourceGrantReconciler) generateEffectiveResourceGrantName(namespace, resourceTypeName string) string {
	// Create a hash of the resourceTypeName to ensure valid k8s names
	// e.g., "resourcemanager.miloapis.com/Project" -> hash
	input := fmt.Sprintf("%s%s", namespace, resourceTypeName)
	hash := sha256.Sum256([]byte(input))
	// Use first 8 chars of hash + suffix for readability and uniqueness
	return fmt.Sprintf("erg-%x", hash)[:12]
}

// generateAllowanceBucketName creates a deterministic name for AllowanceBucket using hash
func (r *EffectiveResourceGrantReconciler) generateAllowanceBucketName(namespace, resourceTypeName string, dimensions map[string]string) string {
	// Serialize dimensions for consistent hashing
	dimensionsJson, _ := json.Marshal(dimensions)

	// Create hash of namespace + resourceTypeName + dimensions
	input := fmt.Sprintf("%s%s%s", namespace, resourceTypeName, string(dimensionsJson))
	hash := sha256.Sum256([]byte(input))

	// Return first 12 characters of hex hash for readability
	return fmt.Sprintf("bucket-%x", hash)[:19] // Keep it under 20 chars for k8s names
}

// SetupWithManager sets up the controller with the Manager.
// Watches both ResourceGrant and ResourceClaim objects.
func (r *EffectiveResourceGrantReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&quotav1alpha1.EffectiveResourceGrant{}).
		Watches(
			&quotav1alpha1.ResourceGrant{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
				grant := obj.(*quotav1alpha1.ResourceGrant)
				var requests []reconcile.Request

				// Find all EffectiveResourceGrants affected by this ResourceGrant
				for _, allowance := range grant.Spec.Allowances {
					effectiveGrantName := r.generateEffectiveResourceGrantName(grant.Namespace, allowance.ResourceTypeName)
					requests = append(requests, reconcile.Request{
						NamespacedName: types.NamespacedName{
							Name:      effectiveGrantName,
							Namespace: grant.Namespace,
						},
					})
				}

				return requests
			}),
		).
		Watches(
			&quotav1alpha1.ResourceClaim{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
				claim := obj.(*quotav1alpha1.ResourceClaim)
				var requests []reconcile.Request

				// Find all EffectiveResourceGrants affected by this ResourceClaim
				for _, request := range claim.Spec.Requests {
					effectiveGrantName := r.generateEffectiveResourceGrantName(claim.Namespace, request.ResourceTypeName)
					requests = append(requests, reconcile.Request{
						NamespacedName: types.NamespacedName{
							Name:      effectiveGrantName,
							Namespace: claim.Namespace,
						},
					})
				}

				return requests
			}),
		).
		Watches(
			&quotav1alpha1.AllowanceBucket{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
				bucket := obj.(*quotav1alpha1.AllowanceBucket)
				if bucket.Spec.OwnerRef.Name == "" {
					return nil
				}
				// When a bucket's allocation changes, trigger a reconcile for the owning EffectiveResourceGrant
				return []reconcile.Request{
					{NamespacedName: types.NamespacedName{
						Name:      bucket.Spec.OwnerRef.Name,
						Namespace: bucket.Namespace,
					}},
				}
			}),
		).
		Named("effective-resource-grant").
		Complete(r)
}
