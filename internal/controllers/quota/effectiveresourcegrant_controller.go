package quota

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"time"

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

// EffectiveResourceGrantController reconciles a EffectiveResourceGrant object
type EffectiveResourceGrantController struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=quota.miloapis.com,resources=effectiveresourcegrants,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=quota.miloapis.com,resources=effectiveresourcegrants/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=quota.miloapis.com,resources=resourcegrants,verbs=get;list;watch
// +kubebuilder:rbac:groups=quota.miloapis.com,resources=resourceclaims,verbs=get;list;watch
// +kubebuilder:rbac:groups=quota.miloapis.com,resources=allowancebuckets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=quota.miloapis.com,resources=allowancebuckets/status,verbs=get;update;patch

// Reconcile manages the lifecycle of EffectiveResourceGrant objects.
func (r *EffectiveResourceGrantController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Get the object that triggered this reconciliation
	var effectiveGrant quotav1alpha1.EffectiveResourceGrant
	if err := r.Get(ctx, req.NamespacedName, &effectiveGrant); err != nil {
		if errors.IsNotFound(err) {
			// EffectiveResourceGrant doesn't exist - check if it should be created
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

		if updateErr := r.Status().Update(ctx, &effectiveGrant); updateErr != nil {
			if !errors.IsConflict(updateErr) {
				logger.Error(updateErr, "Failed to update EffectiveResourceGrant status after bucket creation failure")
			}
		}
		return ctrl.Result{}, fmt.Errorf("failed to create AllowanceBuckets: %w", err)
	}

	// Aggregate total limit from all active ResourceGrants in the same namespace with matching resourceType and ownerInstanceRef
	totalLimit, contributingGrantRefs, err := r.aggregateTotalLimit(ctx, effectiveGrant.Namespace, effectiveGrant.Spec.ResourceType, effectiveGrant.Spec.OwnerInstanceRef)
	if err != nil {
		readyCondition.Status = metav1.ConditionFalse
		readyCondition.Reason = quotav1alpha1.EffectiveResourceGrantAggregationFailedReason
		readyCondition.Message = fmt.Sprintf("Failed to aggregate total limit: %v", err)
		apimeta.SetStatusCondition(&effectiveGrant.Status.Conditions, *readyCondition)

		if updateErr := r.Status().Update(ctx, &effectiveGrant); updateErr != nil {
			if !errors.IsConflict(updateErr) {
				logger.Error(updateErr, "Failed to update EffectiveResourceGrant status after aggregation failure")
			}
		}

		// If a race condition is detected, requeue with a short delay to avoid
		// excessive retries.
		if strings.Contains(err.Error(), "no contributing ResourceGrants found") {
			logger.Info("Requeuing due to timing issue with ResourceGrant activation", "retryAfter", "5s")
			return ctrl.Result{RequeueAfter: time.Second * 5}, nil
		}

		return ctrl.Result{}, fmt.Errorf("failed to aggregate total limit: %w", err)
	}

	// Calculate total allocated from all granted ResourceClaims
	totalAllocated, contributingClaimRefs, err := r.calculateTotalAllocated(ctx, &effectiveGrant)
	if err != nil {
		readyCondition.Status = metav1.ConditionFalse
		readyCondition.Reason = quotav1alpha1.EffectiveResourceGrantAggregationFailedReason
		readyCondition.Message = fmt.Sprintf("Failed to calculate total allocated: %v", err)
		apimeta.SetStatusCondition(&effectiveGrant.Status.Conditions, *readyCondition)

		if updateErr := r.Status().Update(ctx, &effectiveGrant); updateErr != nil {
			if !errors.IsConflict(updateErr) {
				logger.Error(updateErr, "Failed to update EffectiveResourceGrant status after calculation failure")
			}
		}
		return ctrl.Result{}, fmt.Errorf("failed to calculate total allocated: %w", err)
	}

	// Update EffectiveResourceGrant status
	effectiveGrant.Status.TotalLimit = totalLimit
	effectiveGrant.Status.TotalAllocated = totalAllocated
	effectiveGrant.Status.Available = totalLimit - totalAllocated
	effectiveGrant.Status.ContributingGrantRefs = contributingGrantRefs
	effectiveGrant.Status.ContributingClaimRefs = contributingClaimRefs

	// Set ready condition to true
	readyCondition.Status = metav1.ConditionTrue
	readyCondition.Reason = quotav1alpha1.EffectiveResourceGrantAggregationCompleteReason
	readyCondition.Message = "EffectiveResourceGrant aggregation completed successfully"

	apimeta.SetStatusCondition(&effectiveGrant.Status.Conditions, *readyCondition)

	// Only update the status if something has actually changed
	if !equality.Semantic.DeepEqual(originalStatus, &effectiveGrant.Status) {
		if err := r.Status().Update(ctx, &effectiveGrant); err != nil {
			if errors.IsConflict(err) {
				// Conflict errors are expected when multiple reconciliation loops run simultaneously
				// This is not a fatal error - just requeue to retry with the latest version
				logger.Info("Conflict updating EffectiveResourceGrant status, will retry", "error", err)
				return ctrl.Result{Requeue: true}, nil
			}
			logger.Error(err, "Failed to update EffectiveResourceGrant status")
			return ctrl.Result{}, err
		}
		logger.Info("Successfully updated EffectiveResourceGrant",
			"totalLimit", totalLimit,
			"totalAllocated", totalAllocated,
			"available", totalLimit-totalAllocated,
			"contributingGrants", len(contributingGrantRefs),
			"contributingClaims", len(contributingClaimRefs))
	}

	return ctrl.Result{}, nil
}

// Handles the case where an EffectiveResourceGrant doesn't exist but should be
// created based on the ResourceGrant that triggered this reconciliation.
func (r *EffectiveResourceGrantController) createEffectiveResourceGrants(ctx context.Context, namespacedName types.NamespacedName) (ctrl.Result, error) {
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
			expectedName := r.generateEffectiveResourceGrantName(grant.Namespace, allowance.ResourceType, grant.Spec.OwnerInstanceRef)
			if expectedName == namespacedName.Name {
				// Found a matching ResourceGrant - create the EffectiveResourceGrant
				effectiveGrant := quotav1alpha1.EffectiveResourceGrant{
					ObjectMeta: metav1.ObjectMeta{
						Name:      namespacedName.Name,
						Namespace: namespacedName.Namespace,
					},
					Spec: quotav1alpha1.EffectiveResourceGrantSpec{
						ResourceType: allowance.ResourceType,
						OwnerInstanceRef: quotav1alpha1.OwnerInstanceRef{
							Kind: grant.Spec.OwnerInstanceRef.Kind,
							Name: grant.Spec.OwnerInstanceRef.Name,
						},
					},
					Status: quotav1alpha1.EffectiveResourceGrantStatus{
						TotalLimit:            0,
						TotalAllocated:        0,
						Available:             0,
						ContributingGrantRefs: make([]quotav1alpha1.ContributingResourceRef, 0),
						ContributingClaimRefs: make([]quotav1alpha1.ContributingResourceRef, 0),
						Conditions:            make([]metav1.Condition, 0),
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

// createAllowanceBucketsForGrantedClaims creates AllowanceBuckets for all granted ResourceClaims.
func (r *EffectiveResourceGrantController) createAllowanceBucketsForGrantedClaims(ctx context.Context, effectiveGrant *quotav1alpha1.EffectiveResourceGrant) error {
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
			if request.ResourceType == effectiveGrant.Spec.ResourceType {
				if err := r.createAllowanceBucket(ctx, effectiveGrant, request.ResourceType, request.Dimensions); err != nil {
					return err
				}
				// Recalculate bucket allocation for this request
				if err := r.recalculateBucketAllocation(ctx, effectiveGrant, request.ResourceType, request.Dimensions); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// aggregateTotalLimit sums allowances from all active ResourceGrants with matching namespace, resourceType, and ownerInstanceRef
func (r *EffectiveResourceGrantController) aggregateTotalLimit(ctx context.Context, namespace, resourceType string, ownerInstanceRef quotav1alpha1.OwnerInstanceRef) (int64, []quotav1alpha1.ContributingResourceRef, error) {
	logger := log.FromContext(ctx)

	var grants quotav1alpha1.ResourceGrantList
	if err := r.List(ctx, &grants, client.InNamespace(namespace)); err != nil {
		return 0, []quotav1alpha1.ContributingResourceRef{}, fmt.Errorf("failed to list ResourceGrants: %w", err)
	}

	logger.Info("Aggregating total limit",
		"namespace", namespace,
		"resourceType", resourceType,
		"ownerInstanceRef", ownerInstanceRef,
		"totalResourceGrants", len(grants.Items))

	var totalLimit int64
	contributingGrantRefs := make([]quotav1alpha1.ContributingResourceRef, 0)

	for _, grant := range grants.Items {
		logger.Info("Checking ResourceGrant",
			"grantName", grant.Name,
			"grantOwnerRef", grant.Spec.OwnerInstanceRef,
			"isActive", r.isResourceGrantActive(&grant),
			"conditions", grant.Status.Conditions)

		// Check if grant is active
		if !r.isResourceGrantActive(&grant) {
			logger.Info("Skipping inactive ResourceGrant", "grantName", grant.Name)
			continue
		}

		// Only consider grants with matching OwnerInstanceRef
		if grant.Spec.OwnerInstanceRef.Kind != ownerInstanceRef.Kind ||
			grant.Spec.OwnerInstanceRef.Name != ownerInstanceRef.Name {
			logger.Info("Skipping ResourceGrant with non-matching owner",
				"grantName", grant.Name,
				"grantOwner", grant.Spec.OwnerInstanceRef,
				"expectedOwner", ownerInstanceRef)
			continue
		}

		// Check if this grant has allowances for the matching resource type
		var hasMatchingResourceType bool
		for _, allowance := range grant.Spec.Allowances {
			if allowance.ResourceType == resourceType {
				hasMatchingResourceType = true
				for _, bucket := range allowance.Buckets {
					totalLimit += bucket.Amount
				}
			}
		}

		// If this grant contributed to the total limit, add it to contributing refs
		if hasMatchingResourceType {
			logger.Info("Found contributing ResourceGrant",
				"grantName", grant.Name,
				"generation", grant.Generation)
			contributingGrantRefs = append(contributingGrantRefs, quotav1alpha1.ContributingResourceRef{
				Name:               grant.Name,
				ObservedGeneration: grant.Generation,
			})
		} else {
			logger.Info("ResourceGrant has no matching resource type",
				"grantName", grant.Name,
				"expectedResourceType", resourceType)
		}
	}

	logger.Info("Aggregation complete",
		"totalLimit", totalLimit,
		"contributingGrants", len(contributingGrantRefs))

	// Since EffectiveResourceGrants are created when ResourceGrants are activated,
	// we should always find at least one contributing ResourceGrant
	if len(contributingGrantRefs) == 0 {
		logger.Info("No contributing ResourceGrants found - this indicates a timing issue where the ResourceGrant hasn't been marked as active yet")
		return 0, contributingGrantRefs, fmt.Errorf("no contributing ResourceGrants found - ResourceGrant may not be active yet")
	}

	return totalLimit, contributingGrantRefs, nil
}

// calculateTotalAllocated sums allocated amounts from all granted ResourceClaims for this resource type and owner instance
func (r *EffectiveResourceGrantController) calculateTotalAllocated(ctx context.Context, effectiveGrant *quotav1alpha1.EffectiveResourceGrant) (int64, []quotav1alpha1.ContributingResourceRef, error) {
	var claims quotav1alpha1.ResourceClaimList
	if err := r.List(ctx, &claims, client.InNamespace(effectiveGrant.Namespace)); err != nil {
		return 0, []quotav1alpha1.ContributingResourceRef{}, fmt.Errorf("failed to list ResourceClaims: %w", err)
	}

	var totalAllocated int64
	contributingClaimRefs := make([]quotav1alpha1.ContributingResourceRef, 0)

	for _, claim := range claims.Items {
		// Only consider granted claims
		if !r.isResourceClaimGranted(&claim) {
			continue
		}

		// Check if this claim has any requests for the matching resource type
		var hasMatchingRequest bool
		var claimTotal int64

		for _, request := range claim.Spec.Requests {
			if request.ResourceType == effectiveGrant.Spec.ResourceType {
				hasMatchingRequest = true
				claimTotal += request.Amount
			}
		}

		// If this claim contributed to the total allocated, add it to contributing refs
		if hasMatchingRequest {
			totalAllocated += claimTotal
			contributingClaimRefs = append(contributingClaimRefs, quotav1alpha1.ContributingResourceRef{
				Name:               claim.Name,
				ObservedGeneration: claim.Generation,
			})
		}
	}

	return totalAllocated, contributingClaimRefs, nil
}

// isResourceGrantActive checks if a ResourceGrant has an Active condition with
// a status of True
func (r *EffectiveResourceGrantController) isResourceGrantActive(grant *quotav1alpha1.ResourceGrant) bool {
	for _, condition := range grant.Status.Conditions {
		if condition.Type == quotav1alpha1.ResourceGrantActive && condition.Status == metav1.ConditionTrue {
			return true
		}
	}
	return false
}

// Determines if a ResourceClaim has been granted
func (r *EffectiveResourceGrantController) isResourceClaimGranted(claim *quotav1alpha1.ResourceClaim) bool {
	for _, condition := range claim.Status.Conditions {
		if condition.Type == quotav1alpha1.ResourceClaimGranted && condition.Status == metav1.ConditionTrue {
			return true
		}
	}
	return false
}

// Creates an AllowanceBucket when a new ResourceClaim is granted.
func (r *EffectiveResourceGrantController) createAllowanceBucket(ctx context.Context, effectiveGrant *quotav1alpha1.EffectiveResourceGrant, resourceType string, dimensions map[string]string) error {
	log := log.FromContext(ctx)

	log.Info("Running createAllowanceBucket", "effectiveGrant", effectiveGrant, "resourceType", resourceType, "dimensions", dimensions)
	bucketName := r.generateAllowanceBucketName(effectiveGrant.Namespace, resourceType, dimensions)

	var bucket quotav1alpha1.AllowanceBucket
	err := r.Get(ctx, types.NamespacedName{
		Name:      bucketName,
		Namespace: effectiveGrant.Namespace,
	}, &bucket)

	if errors.IsNotFound(err) {
		log.Info("Creating new AllowanceBucket", "effectiveGrant", effectiveGrant, "resourceType", resourceType, "dimensions", dimensions)

		// Create new AllowanceBucket
		bucket = quotav1alpha1.AllowanceBucket{
			ObjectMeta: metav1.ObjectMeta{
				Name:      bucketName,
				Namespace: effectiveGrant.Namespace,
			},
			Spec: quotav1alpha1.AllowanceBucketSpec{
				OwnerInstanceRef: quotav1alpha1.OwnerInstanceRef{
					Kind: effectiveGrant.Kind,
					Name: effectiveGrant.Name,
				},
				ResourceType: resourceType,
				Dimensions:   dimensions,
			},
			Status: quotav1alpha1.AllowanceBucketStatus{
				Allocated:             0,
				ContributingClaimRefs: make([]quotav1alpha1.ContributingClaimRef, 0),
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
func (r *EffectiveResourceGrantController) recalculateBucketAllocation(ctx context.Context, effectiveGrant *quotav1alpha1.EffectiveResourceGrant, resourceType string, dimensions map[string]string) error {
	log := log.FromContext(ctx)
	bucketName := r.generateAllowanceBucketName(effectiveGrant.Namespace, resourceType, dimensions)

	var bucket quotav1alpha1.AllowanceBucket
	if err := r.Get(ctx, types.NamespacedName{Name: bucketName, Namespace: effectiveGrant.Namespace}, &bucket); err != nil {
		if errors.IsNotFound(err) {
			log.Info("AllowanceBucket not found during recalculation, skipping", "name", bucketName)
			return nil
		}
		return fmt.Errorf("failed to get AllowanceBucket %s: %w", bucketName, err)
	}

	originalAllocated := bucket.Status.Allocated
	originalContributingRefs := bucket.Status.ContributingClaimRefs

	// Recalculate allocation from scratch
	var newAllocated int64
	newContributingRefs := make([]quotav1alpha1.ContributingClaimRef, 0)

	var allClaims quotav1alpha1.ResourceClaimList
	if err := r.List(ctx, &allClaims, client.InNamespace(effectiveGrant.Namespace)); err != nil {
		return fmt.Errorf("failed to list ResourceClaims for bucket recalculation: %w", err)
	}

	log.Info("Recalculating bucket allocation",
		"bucketName", bucketName,
		"resourceType", resourceType,
		"dimensions", dimensions,
		"totalClaims", len(allClaims.Items))

	for _, claim := range allClaims.Items {
		// Only consider granted claims
		if !r.isResourceClaimGranted(&claim) {
			continue
		}

		// Check if this claim has any requests that match this bucket
		var claimContributesToBucket bool

		// Find requests in the claim that match this bucket
		for _, request := range claim.Spec.Requests {
			if request.ResourceType == resourceType &&
				r.dimensionsMatch(request.Dimensions, dimensions) {
				newAllocated += request.Amount
				claimContributesToBucket = true
			}
		}

		// If this claim contributed to the bucket, add it to the contributing refs
		if claimContributesToBucket {
			log.V(1).Info("Found contributing ResourceClaim for bucket",
				"bucketName", bucketName,
				"claimName", claim.Name,
				"generation", claim.Generation)
			newContributingRefs = append(newContributingRefs, quotav1alpha1.ContributingClaimRef{
				Name:                   claim.Name,
				LastObservedGeneration: claim.Generation,
			})
		}
	}

	log.Info("Bucket recalculation complete",
		"bucketName", bucketName,
		"newAllocated", newAllocated,
		"contributingClaims", len(newContributingRefs))

	// Check if anything changed (allocation amount or contributing claims)
	contributingRefsChanged := !r.contributingClaimRefsEqual(originalContributingRefs, newContributingRefs)

	if newAllocated != originalAllocated || contributingRefsChanged {
		bucket.Status.Allocated = newAllocated
		bucket.Status.ContributingClaimRefs = newContributingRefs
		bucket.Status.ObservedGeneration = bucket.Generation

		if err := r.Status().Update(ctx, &bucket); err != nil {
			if errors.IsConflict(err) {
				// Conflict is not fatal for bucket updates - another reconciliation will handle it
				log.Info("Conflict updating AllowanceBucket status, skipping", "bucket", bucketName, "error", err)
			} else {
				return fmt.Errorf("failed to update AllowanceBucket status for %s: %w", bucketName, err)
			}
		}

		log.Info("Updated AllowanceBucket status",
			"bucketName", bucketName,
			"allocated", newAllocated,
			"contributingClaims", len(newContributingRefs))
	}

	return nil
}

// dimensionsMatch compares two dimension maps for equality.
func (r *EffectiveResourceGrantController) dimensionsMatch(d1, d2 map[string]string) bool {
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

// contributingClaimRefsEqual compares two slices of ContributingClaimRef for equality.
func (r *EffectiveResourceGrantController) contributingClaimRefsEqual(refs1, refs2 []quotav1alpha1.ContributingClaimRef) bool {
	if len(refs1) != len(refs2) {
		return false
	}

	// Create maps for O(n) comparison instead of O(n²)
	map1 := make(map[string]int64)
	map2 := make(map[string]int64)

	for _, ref := range refs1 {
		map1[ref.Name] = ref.LastObservedGeneration
	}

	for _, ref := range refs2 {
		map2[ref.Name] = ref.LastObservedGeneration
	}

	// Compare the maps
	for name, gen1 := range map1 {
		if gen2, exists := map2[name]; !exists || gen1 != gen2 {
			return false
		}
	}

	return true
}

// generateEffectiveResourceGrantName creates a deterministic name for EffectiveResourceGrant
func (r *EffectiveResourceGrantController) generateEffectiveResourceGrantName(namespace, resourceType string, ownerRef quotav1alpha1.OwnerInstanceRef) string {
	input := fmt.Sprintf("%s%s%s%s", namespace, resourceType, ownerRef.Kind, ownerRef.Name)
	hash := sha256.Sum256([]byte(input))
	// Use first 8 chars of hash + suffix for readability and uniqueness
	return fmt.Sprintf("erg-%x", hash)[:12]
}

// generateAllowanceBucketName creates a deterministic name for AllowanceBucket using hash
func (r *EffectiveResourceGrantController) generateAllowanceBucketName(namespace, resourceType string, dimensions map[string]string) string {
	// Serialize dimensions for consistent hashing
	dimensionsJson, _ := json.Marshal(dimensions)

	// Create hash of namespace + resourceType + dimensions
	input := fmt.Sprintf("%s%s%s", namespace, resourceType, string(dimensionsJson))
	hash := sha256.Sum256([]byte(input))

	// Return first 12 characters of hex hash for readability
	return fmt.Sprintf("bucket-%x", hash)[:19] // Keep it under 20 chars for k8s names
}

// SetupWithManager sets up the controller with the Manager.
// Watches both ResourceGrant and ResourceClaim objects.
func (r *EffectiveResourceGrantController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&quotav1alpha1.EffectiveResourceGrant{}).
		Watches(
			&quotav1alpha1.ResourceGrant{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
				grant := obj.(*quotav1alpha1.ResourceGrant)
				var requests []reconcile.Request

				// Find all EffectiveResourceGrants affected by this ResourceGrant
				for _, allowance := range grant.Spec.Allowances {
					effectiveGrantName := r.generateEffectiveResourceGrantName(grant.Namespace, allowance.ResourceType, grant.Spec.OwnerInstanceRef)
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
				var grants quotav1alpha1.ResourceGrantList
				if err := r.List(ctx, &grants, client.InNamespace(claim.Namespace)); err != nil {
					return requests
				}

				// Create a map to deduplicate requests
				requestMap := make(map[string]bool)

				for _, request := range claim.Spec.Requests {
					// Find all ResourceGrants that provide quota for this resourceType
					for _, grant := range grants.Items {
						for _, allowance := range grant.Spec.Allowances {
							if allowance.ResourceType == request.ResourceType {
								effectiveGrantName := r.generateEffectiveResourceGrantName(claim.Namespace, request.ResourceType, grant.Spec.OwnerInstanceRef)
								requestKey := fmt.Sprintf("%s/%s", claim.Namespace, effectiveGrantName)
								if !requestMap[requestKey] {
									requestMap[requestKey] = true
									requests = append(requests, reconcile.Request{
										NamespacedName: types.NamespacedName{
											Name:      effectiveGrantName,
											Namespace: claim.Namespace,
										},
									})
								}
							}
						}
					}
				}

				return requests
			}),
		).
		Watches(
			&quotav1alpha1.AllowanceBucket{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
				bucket := obj.(*quotav1alpha1.AllowanceBucket)
				if bucket.Spec.OwnerInstanceRef.Name == "" {
					return nil
				}
				// When a bucket's allocation changes, trigger a reconcile for the owning EffectiveResourceGrant
				return []reconcile.Request{
					{NamespacedName: types.NamespacedName{
						Name:      bucket.Spec.OwnerInstanceRef.Name,
						Namespace: bucket.Namespace,
					}},
				}
			}),
		).
		Named("effective-resource-grant").
		Complete(r)
}
