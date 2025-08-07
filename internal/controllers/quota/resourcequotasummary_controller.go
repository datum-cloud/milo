package quota

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// ResourceQuotaSummaryController reconciles a ResourceQuotaSummary object
type ResourceQuotaSummaryController struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=quota.miloapis.com,resources=resourcequotasummaries,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=quota.miloapis.com,resources=resourcequotasummaries/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=quota.miloapis.com,resources=resourcegrants,verbs=get;list;watch
// +kubebuilder:rbac:groups=quota.miloapis.com,resources=resourceclaims,verbs=get;list;watch
// +kubebuilder:rbac:groups=quota.miloapis.com,resources=allowancebuckets,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=quota.miloapis.com,resources=allowancebuckets/status,verbs=get;update;patch

// Reconcile manages the lifecycle of ResourceQuotaSummary objects.
func (r *ResourceQuotaSummaryController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Get the object that triggered this reconciliation
	var resourceQuotaSummary quotav1alpha1.ResourceQuotaSummary
	if err := r.Get(ctx, req.NamespacedName, &resourceQuotaSummary); err != nil {
		if errors.IsNotFound(err) {
			logger.Error(err, "ResourceQuotaSummary not found")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get ResourceQuotaSummary")
		return ctrl.Result{}, err
	}

	// Create a deep copy of the original status to compare against later
	originalStatus := resourceQuotaSummary.Status.DeepCopy()

	// Always update the observed generation to match the current spec
	resourceQuotaSummary.Status.ObservedGeneration = resourceQuotaSummary.Generation

	// Get the current ready condition from the status or create a new one
	readyCondition := apimeta.FindStatusCondition(resourceQuotaSummary.Status.Conditions, quotav1alpha1.ResourceQuotaSummaryReady)
	if readyCondition == nil {
		readyCondition = &metav1.Condition{
			Type:               quotav1alpha1.ResourceQuotaSummaryReady,
			Status:             metav1.ConditionFalse,
			Reason:             quotav1alpha1.ResourceQuotaSummaryCalculationPendingReason,
			Message:            "Calculation is pending",
			ObservedGeneration: resourceQuotaSummary.Generation,
		}
	} else {
		readyCondition = readyCondition.DeepCopy()
		readyCondition.ObservedGeneration = resourceQuotaSummary.Generation
	}

	// Create AllowanceBuckets for all granted ResourceClaims
	if err := r.createAllowanceBucketsForGrantedClaims(ctx, &resourceQuotaSummary); err != nil {
		readyCondition.Status = metav1.ConditionFalse
		readyCondition.Reason = quotav1alpha1.ResourceQuotaSummaryCalculationFailedReason
		readyCondition.Message = fmt.Sprintf("Failed to create AllowanceBuckets: %v", err)
		apimeta.SetStatusCondition(&resourceQuotaSummary.Status.Conditions, *readyCondition)

		if updateErr := r.Status().Update(ctx, &resourceQuotaSummary); updateErr != nil {
			if !errors.IsConflict(updateErr) {
				logger.Error(updateErr, "Failed to update ResourceQuotaSummary status after bucket creation failure")
			}
		}
		return ctrl.Result{}, fmt.Errorf("failed to create AllowanceBuckets: %w", err)
	}

	// Calculate total limit from all active ResourceGrants in the same namespace with matching resourceType and ownerInstanceRef
	totalLimit, contributingGrantRefs, err := r.calculateTotalLimit(ctx, resourceQuotaSummary.Namespace, resourceQuotaSummary.Spec.ResourceType, resourceQuotaSummary.Spec.OwnerInstanceRef)
	if err != nil {
		readyCondition.Status = metav1.ConditionFalse
		readyCondition.Reason = quotav1alpha1.ResourceQuotaSummaryCalculationFailedReason
		readyCondition.Message = fmt.Sprintf("Failed to calculate total limit: %v", err)
		apimeta.SetStatusCondition(&resourceQuotaSummary.Status.Conditions, *readyCondition)

		if updateErr := r.Status().Update(ctx, &resourceQuotaSummary); updateErr != nil {
			if !errors.IsConflict(updateErr) {
				logger.Error(updateErr, "Failed to update ResourceQuotaSummary status after calculation failure")
			}
		}

		// If a race condition is detected, requeue with a short delay to avoid
		// excessive retries.
		if strings.Contains(err.Error(), "no contributing ResourceGrants found") {
			logger.Info("Requeuing due to timing issue with ResourceGrant activation", "retryAfter", "5s")
			return ctrl.Result{RequeueAfter: time.Second * 5}, nil
		}

		return ctrl.Result{}, fmt.Errorf("failed to calculate total limit: %w", err)
	}

	// Calculate total allocated from all granted ResourceClaims
	totalAllocated, contributingClaimRefs, err := r.calculateTotalAllocated(ctx, &resourceQuotaSummary)
	if err != nil {
		readyCondition.Status = metav1.ConditionFalse
		readyCondition.Reason = quotav1alpha1.ResourceQuotaSummaryCalculationFailedReason
		readyCondition.Message = fmt.Sprintf("Failed to calculate total allocated: %v", err)
		apimeta.SetStatusCondition(&resourceQuotaSummary.Status.Conditions, *readyCondition)

		if updateErr := r.Status().Update(ctx, &resourceQuotaSummary); updateErr != nil {
			if !errors.IsConflict(updateErr) {
				logger.Error(updateErr, "Failed to update ResourceQuotaSummary status after calculation failure")
			}
		}
		return ctrl.Result{}, fmt.Errorf("failed to calculate total allocated: %w", err)
	}

	// Update ResourceQuotaSummary status
	resourceQuotaSummary.Status.TotalLimit = totalLimit
	resourceQuotaSummary.Status.TotalAllocated = totalAllocated
	resourceQuotaSummary.Status.Available = totalLimit - totalAllocated
	resourceQuotaSummary.Status.ContributingGrantRefs = contributingGrantRefs
	resourceQuotaSummary.Status.ContributingClaimRefs = contributingClaimRefs

	// Set ready condition to true
	readyCondition.Status = metav1.ConditionTrue
	readyCondition.Reason = quotav1alpha1.ResourceQuotaSummaryCalculationCompleteReason
	readyCondition.Message = "ResourceQuotaSummary calculation completed successfully"

	apimeta.SetStatusCondition(&resourceQuotaSummary.Status.Conditions, *readyCondition)

	// Only update the status if something has actually changed
	if !equality.Semantic.DeepEqual(originalStatus, &resourceQuotaSummary.Status) {
		if err := r.Status().Update(ctx, &resourceQuotaSummary); err != nil {
			if errors.IsConflict(err) {
				// Conflict errors are expected when multiple reconciliation loops run simultaneously
				// Add requeue delay to reduce repeated conflicts when multiple controllers update simultaneously
				requeueAfter := time.Duration(rand.Intn(500)+100) * time.Millisecond // 100-600ms jitter
				logger.Info("Conflict updating ResourceQuotaSummary status, will retry after delay",
					"error", err, "requeueAfter", requeueAfter)
				return ctrl.Result{RequeueAfter: requeueAfter}, nil
			}
			logger.Error(err, "Failed to update ResourceQuotaSummary status")
			return ctrl.Result{}, err
		}
		// logger.Info("Successfully updated ResourceQuotaSummary",
		// 	"totalLimit", totalLimit,
		// 	"totalAllocated", totalAllocated,
		// 	"available", totalLimit-totalAllocated,
		// 	"contributingGrants", len(contributingGrantRefs),
		// 	"contributingClaims", len(contributingClaimRefs))
	}

	return ctrl.Result{}, nil
}

// createAllowanceBucketsForGrantedClaims creates AllowanceBuckets for all granted ResourceClaims.
func (r *ResourceQuotaSummaryController) createAllowanceBucketsForGrantedClaims(ctx context.Context, resourceQuotaSummary *quotav1alpha1.ResourceQuotaSummary) error {
	// Ensure buckets for ResourceClaims
	var claims quotav1alpha1.ResourceClaimList
	if err := r.List(ctx, &claims, client.InNamespace(resourceQuotaSummary.Namespace)); err != nil {
		return fmt.Errorf("failed to list ResourceClaims: %w", err)
	}

	for _, claim := range claims.Items {
		// Check if claim is granted and continue to next claim if not, as allowance buckets are only created for granted claims
		if !r.isResourceClaimGranted(&claim) {
			continue
		}

		for _, request := range claim.Spec.Requests {
			if request.ResourceType == resourceQuotaSummary.Spec.ResourceType {
				if err := r.createAllowanceBucket(ctx, resourceQuotaSummary, request.ResourceType, request.Dimensions); err != nil {
					return err
				}
				// Calculate bucket allocation for this request
				if err := r.calculateBucketAllocation(ctx, resourceQuotaSummary, request.ResourceType, request.Dimensions); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// calculateTotalLimit sums allowances from all active ResourceGrants with matching namespace, resourceType, and ownerInstanceRef
func (r *ResourceQuotaSummaryController) calculateTotalLimit(ctx context.Context, namespace, resourceType string, ownerInstanceRef quotav1alpha1.OwnerInstanceRef) (int64, []quotav1alpha1.ContributingResourceRef, error) {
	logger := log.FromContext(ctx)

	var grants quotav1alpha1.ResourceGrantList
	if err := r.List(ctx, &grants, client.InNamespace(namespace)); err != nil {
		return 0, []quotav1alpha1.ContributingResourceRef{}, fmt.Errorf("failed to list ResourceGrants: %w", err)
	}

	// logger.Info("Aggregating total limit",
	// 	"namespace", namespace,
	// 	"resourceType", resourceType,
	// 	"ownerInstanceRef", ownerInstanceRef,
	// 	"totalResourceGrants", len(grants.Items))

	var totalLimit int64
	contributingGrantRefs := []quotav1alpha1.ContributingResourceRef{}

	for _, grant := range grants.Items {
		// logger.Info("Checking ResourceGrant",
		// 	"grantName", grant.Name,
		// 	"grantOwnerRef", grant.Spec.OwnerInstanceRef,
		// 	"isActive", r.isResourceGrantActive(&grant),
		// 	"conditions", grant.Status.Conditions)

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
			// logger.Info("Found contributing ResourceGrant",
			// 	"grantName", grant.Name,
			// 	"generation", grant.Generation)
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

	// logger.Info("Aggregation complete",
	// 	"totalLimit", totalLimit,
	// 	"contributingGrants", len(contributingGrantRefs))

	// Since ResourceQuotaSummaries are created when ResourceGrants are activated,
	// we should always find at least one contributing ResourceGrant
	if len(contributingGrantRefs) == 0 {
		logger.Info("No contributing ResourceGrants found - this indicates a race condition where the ResourceGrant hasn't been marked as active yet")
		return 0, contributingGrantRefs, fmt.Errorf("no contributing ResourceGrants found - ResourceGrant may not be active yet")
	}

	return totalLimit, contributingGrantRefs, nil
}

// calculateTotalAllocated sums allocated amounts from all granted ResourceClaims for this resource type and owner instance
func (r *ResourceQuotaSummaryController) calculateTotalAllocated(ctx context.Context, resourceQuotaSummary *quotav1alpha1.ResourceQuotaSummary) (int64, []quotav1alpha1.ContributingResourceRef, error) {
	var claims quotav1alpha1.ResourceClaimList
	if err := r.List(ctx, &claims, client.InNamespace(resourceQuotaSummary.Namespace)); err != nil {
		return 0, []quotav1alpha1.ContributingResourceRef{}, fmt.Errorf("failed to list ResourceClaims: %w", err)
	}

	var totalAllocated int64
	contributingClaimRefs := []quotav1alpha1.ContributingResourceRef{}

	for _, claim := range claims.Items {
		// Only consider granted claims
		if !r.isResourceClaimGranted(&claim) {
			continue
		}

		// Only consider claims with matching ownerInstanceRef and resource type
		if claim.Spec.OwnerInstanceRef.Kind != resourceQuotaSummary.Spec.OwnerInstanceRef.Kind ||
			claim.Spec.OwnerInstanceRef.Name != resourceQuotaSummary.Spec.OwnerInstanceRef.Name {
			continue
		}

		// Check if this claim has any requests for the matching resource type
		var hasMatchingRequest bool
		var claimTotal int64

		for _, request := range claim.Spec.Requests {
			if request.ResourceType == resourceQuotaSummary.Spec.ResourceType {
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
func (r *ResourceQuotaSummaryController) isResourceGrantActive(grant *quotav1alpha1.ResourceGrant) bool {
	for _, condition := range grant.Status.Conditions {
		if condition.Type == quotav1alpha1.ResourceGrantActive && condition.Status == metav1.ConditionTrue {
			return true
		}
	}
	return false
}

// Determines if a ResourceClaim has been granted
func (r *ResourceQuotaSummaryController) isResourceClaimGranted(claim *quotav1alpha1.ResourceClaim) bool {
	for _, condition := range claim.Status.Conditions {
		if condition.Type == quotav1alpha1.ResourceClaimGranted && condition.Status == metav1.ConditionTrue {
			return true
		}
	}
	return false
}

// Creates an AllowanceBucket when a new ResourceClaim is granted.
func (r *ResourceQuotaSummaryController) createAllowanceBucket(ctx context.Context, resourceQuotaSummary *quotav1alpha1.ResourceQuotaSummary, resourceType string, dimensions map[string]string) error {
	log := log.FromContext(ctx)

	// log.Info("Running createAllowanceBucket", "effectiveGrant", effectiveGrant, "resourceType", resourceType, "dimensions", dimensions)
	bucketName := r.generateAllowanceBucketName(resourceQuotaSummary.Namespace, resourceType, dimensions)

	var bucket quotav1alpha1.AllowanceBucket
	err := r.Get(ctx, types.NamespacedName{
		Name:      bucketName,
		Namespace: resourceQuotaSummary.Namespace,
	}, &bucket)

	if errors.IsNotFound(err) {
		log.Info("Creating new AllowanceBucket", "effectiveGrant", resourceQuotaSummary, "resourceType", resourceType, "dimensions", dimensions)

		// Create new AllowanceBucket
		bucket = quotav1alpha1.AllowanceBucket{
			ObjectMeta: metav1.ObjectMeta{
				Name:      bucketName,
				Namespace: resourceQuotaSummary.Namespace,
			},
			Spec: quotav1alpha1.AllowanceBucketSpec{
				OwnerInstanceRef: quotav1alpha1.OwnerInstanceRef{
					Kind: resourceQuotaSummary.Kind,
					Name: resourceQuotaSummary.Name,
				},
				ResourceType: resourceType,
				Dimensions:   dimensions,
			},
			Status: quotav1alpha1.AllowanceBucketStatus{
				Allocated:             0,
				ContributingClaimRefs: []quotav1alpha1.ContributingClaimRef{},
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

// Calculates the entire allocation for a bucket by scanning all claims.
func (r *ResourceQuotaSummaryController) calculateBucketAllocation(ctx context.Context, resourceQuotaSummary *quotav1alpha1.ResourceQuotaSummary, resourceType string, dimensions map[string]string) error {
	log := log.FromContext(ctx)
	bucketName := r.generateAllowanceBucketName(resourceQuotaSummary.Namespace, resourceType, dimensions)

	var bucket quotav1alpha1.AllowanceBucket
	if err := r.Get(ctx, types.NamespacedName{Name: bucketName, Namespace: resourceQuotaSummary.Namespace}, &bucket); err != nil {
		if errors.IsNotFound(err) {
			log.Info("AllowanceBucket not found during recalculation, skipping", "name", bucketName)
			return nil
		}
		return fmt.Errorf("failed to get AllowanceBucket %s: %w", bucketName, err)
	}

	originalAllocated := bucket.Status.Allocated
	originalContributingRefs := bucket.Status.ContributingClaimRefs

	// Calculate allocation from scratch
	var newAllocated int64
	newContributingRefs := []quotav1alpha1.ContributingClaimRef{}

	var allClaims quotav1alpha1.ResourceClaimList
	if err := r.List(ctx, &allClaims, client.InNamespace(resourceQuotaSummary.Namespace)); err != nil {
		return fmt.Errorf("failed to list ResourceClaims for bucket recalculation: %w", err)
	}

	// log.Info("Recalculating bucket allocation",
	// 	"bucketName", bucketName,
	// 	"resourceType", resourceType,
	// 	"dimensions", dimensions,
	// 	"totalClaims", len(allClaims.Items))

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
			// log.Info("Found contributing ResourceClaim for bucket",
			// 	"bucketName", bucketName,
			// 	"claimName", claim.Name,
			// 	"generation", claim.Generation)
			newContributingRefs = append(newContributingRefs, quotav1alpha1.ContributingClaimRef{
				Name:                   claim.Name,
				LastObservedGeneration: claim.Generation,
			})
		}
	}

	// log.Info("Bucket recalculation complete",
	// 	"bucketName", bucketName,
	// 	"newAllocated", newAllocated,
	// 	"contributingClaims", len(newContributingRefs))

	// Check if anything changed (allocation amount or contributing claims)
	contributingRefsChanged := !r.contributingClaimRefsEqual(originalContributingRefs, newContributingRefs)

	if newAllocated != originalAllocated || contributingRefsChanged {
		bucket.Status.Allocated = newAllocated
		bucket.Status.ContributingClaimRefs = newContributingRefs
		bucket.Status.ObservedGeneration = bucket.Generation

		if err := r.Status().Update(ctx, &bucket); err != nil {
			if errors.IsConflict(err) {
				log.Info("Conflict updating AllowanceBucket status, skipping", "bucket", bucketName, "error", err)
			} else {
				return fmt.Errorf("failed to update AllowanceBucket status for %s: %w", bucketName, err)
			}
		}

		// log.Info("Updated AllowanceBucket status",
		// 	"bucketName", bucketName,
		// 	"allocated", newAllocated,
		// 	"contributingClaims", len(newContributingRefs))
	}

	return nil
}

// dimensionsMatch compares two dimension maps for equality.
func (r *ResourceQuotaSummaryController) dimensionsMatch(d1, d2 map[string]string) bool {
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
func (r *ResourceQuotaSummaryController) contributingClaimRefsEqual(refs1, refs2 []quotav1alpha1.ContributingClaimRef) bool {
	if len(refs1) != len(refs2) {
		return false
	}

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

// generateResourceQuotaSummaryName creates a deterministic name for ResourceQuotaSummary
func (r *ResourceQuotaSummaryController) generateResourceQuotaSummaryName(namespace, resourceType string, ownerRef quotav1alpha1.OwnerInstanceRef) string {
	input := fmt.Sprintf("%s%s%s%s", namespace, resourceType, ownerRef.Kind, ownerRef.Name)
	hash := sha256.Sum256([]byte(input))
	// Use first 8 chars of hash + suffix for readability and uniqueness
	return fmt.Sprintf("rqs-%x", hash)[:12]
}

// generateAllowanceBucketName creates a deterministic name for AllowanceBucket using hash
func (r *ResourceQuotaSummaryController) generateAllowanceBucketName(namespace, resourceType string, dimensions map[string]string) string {
	// Serialize dimensions for consistent hashing
	dimensionsJson, _ := json.Marshal(dimensions)

	// Create hash of namespace + resourceType + dimensions
	input := fmt.Sprintf("%s%s%s", namespace, resourceType, string(dimensionsJson))
	hash := sha256.Sum256([]byte(input))

	// Return first 12 characters of hex hash for readability
	return fmt.Sprintf("bucket-%x", hash)[:19] // Keep it under 20 chars for k8s names
}

// SetupWithManager sets up the controller with the Manager.
func (r *ResourceQuotaSummaryController) SetupWithManager(mgr ctrl.Manager) error {
	// Predicate to only reconcile when the 'Active' condition on a ResourceGrant is 'True'
	resourceGrantIsActivePredicate := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		grant, ok := obj.(*quotav1alpha1.ResourceGrant)
		if !ok {
			return false
		}
		return r.isResourceGrantActive(grant)
	})

	resourceClaimStatusPredicate := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			claim, ok := e.Object.(*quotav1alpha1.ResourceClaim)
			if !ok {
				return false
			}
			return r.isResourceClaimGranted(claim)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldClaim, okOld := e.ObjectOld.(*quotav1alpha1.ResourceClaim)
			newClaim, okNew := e.ObjectNew.(*quotav1alpha1.ResourceClaim)
			if !okOld || !okNew {
				return false
			}

			oldIsGranted := r.isResourceClaimGranted(oldClaim)
			newIsGranted := r.isResourceClaimGranted(newClaim)

			// Reconcile if the claim becomes granted, or if a granted claim's spec changes.
			return (!oldIsGranted && newIsGranted) || (newIsGranted && oldClaim.Generation != newClaim.Generation)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			claim, ok := e.Object.(*quotav1alpha1.ResourceClaim)
			if !ok {
				return false
			}
			// Reconcile if a granted claim is deleted.
			return r.isResourceClaimGranted(claim)
		},
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&quotav1alpha1.ResourceQuotaSummary{},
			builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(
			&quotav1alpha1.ResourceGrant{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
				grant := obj.(*quotav1alpha1.ResourceGrant)
				var reconcileRequests []reconcile.Request

				// Directly map ResourceGrant to affected ResourceQuotaSummaries
				requestMap := make(map[string]bool)

				for _, allowance := range grant.Spec.Allowances {
					resourceQuotaSummaryName := r.generateResourceQuotaSummaryName(
						grant.Namespace,
						allowance.ResourceType,
						grant.Spec.OwnerInstanceRef,
					)
					requestKey := fmt.Sprintf("%s/%s", grant.Namespace, resourceQuotaSummaryName)
					if !requestMap[requestKey] {
						requestMap[requestKey] = true
						reconcileRequests = append(reconcileRequests, reconcile.Request{
							NamespacedName: types.NamespacedName{
								Name:      resourceQuotaSummaryName,
								Namespace: grant.Namespace,
							},
						})
					}
				}

				return reconcileRequests
			}),
			builder.WithPredicates(resourceGrantIsActivePredicate),
		).
		Watches(
			&quotav1alpha1.ResourceClaim{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
				claim := obj.(*quotav1alpha1.ResourceClaim)
				var reconcileRequests []reconcile.Request

				// Directly map ResourceClaim to affected ResourceQuotaSummaries
				// based on the claim's ownerInstanceRef and resource types
				requestMap := make(map[string]bool)

				for _, request := range claim.Spec.Requests {
					resourceQuotaSummaryName := r.generateResourceQuotaSummaryName(
						claim.Namespace,
						request.ResourceType,
						claim.Spec.OwnerInstanceRef,
					)
					requestKey := fmt.Sprintf("%s/%s", claim.Namespace, resourceQuotaSummaryName)
					if !requestMap[requestKey] {
						requestMap[requestKey] = true
						reconcileRequests = append(reconcileRequests, reconcile.Request{
							NamespacedName: types.NamespacedName{
								Name:      resourceQuotaSummaryName,
								Namespace: claim.Namespace,
							},
						})
					}
				}

				return reconcileRequests
			}),
			builder.WithPredicates(resourceClaimStatusPredicate),
		).
		Complete(r)
}
