// Package core implements the core quota controllers that manage AllowanceBuckets,
// ResourceClaims, ResourceGrants, and ResourceRegistrations.
//
// The AllowanceBucketController owns the lifecycle of AllowanceBuckets.
// Centralizing bucket writes in a single controller avoids write races between
// controllers and keeps reconciliation predictable. This controller
// materializes aggregates for efficient quota evaluation by:
// - Computing Limit from Active ResourceGrants (scoped to the same owner)
// - Computing Allocated from Granted ResourceClaims (same owner + shape)
// Buckets are created on demand when claims reference them to minimize churn
// while still handling eventual consistency between controllers.
package core

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

// AllowanceBucketController reconciles AllowanceBucket objects and maintains
// aggregated quota (Limit/Allocated/Available). It is the single writer for
// bucket objects; other controllers are read-only.
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

	// Try to grant pending claims by reserving capacity
	if err := r.processPendingGrants(ctx, &bucket); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed processing pending grants: %w", err)
	}

	// Calculate available quota after any reservations
	// Ensure Available is never negative (validation constraint)
	bucket.Status.Available = max(0, bucket.Status.Limit-bucket.Status.Allocated)

	// Update last reconciliation time
	now := metav1.Now()
	bucket.Status.LastReconciliation = &now

	// Update status (Kubernetes API server efficiently handles no-op updates)
	if err := r.Status().Update(ctx, &bucket); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update AllowanceBucket status: %w", err)
	}

	return ctrl.Result{}, nil
}

// updateLimitsFromGrants calculates the total limit from all applicable ResourceGrants.
func (r *AllowanceBucketController) updateLimitsFromGrants(ctx context.Context, bucket *quotav1alpha1.AllowanceBucket) error {

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
				totalLimit += allowanceBucket.Amount
				contributingGrants = append(contributingGrants, quotav1alpha1.ContributingGrantRef{
					Name:                   grant.Name,
					LastObservedGeneration: grant.Generation,
					Amount:                 allowanceBucket.Amount,
				})
			}
		}
	}

	bucket.Status.Limit = totalLimit
	bucket.Status.GrantCount = int32(len(contributingGrants))
	bucket.Status.ContributingGrantRefs = contributingGrants

	return nil
}

// updateUsageFromClaims calculates the total allocated usage from ResourceClaims
// based on individual request allocations that have been granted.
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
		// Owner must match
		if claim.Spec.ConsumerRef.Kind != bucket.Spec.ConsumerRef.Kind ||
			claim.Spec.ConsumerRef.Name != bucket.Spec.ConsumerRef.Name {
			logger.V(1).Info("Claim consumer does not match bucket consumer, skipping",
				"claimName", claim.Name,
				"claimConsumer", claim.Spec.ConsumerRef.Kind+"/"+claim.Spec.ConsumerRef.Name,
				"bucketConsumer", bucket.Spec.ConsumerRef.Kind+"/"+bucket.Spec.ConsumerRef.Name)
			continue
		}

		// Check allocations for granted requests that match this bucket
		for _, allocation := range claim.Status.Allocations {
			if allocation.Status != quotav1alpha1.RequestAllocationGranted {
				continue
			}

			// Check if this allocation matches the bucket
			if allocation.ResourceType != bucket.Spec.ResourceType {
				continue
			}

			// Find the corresponding request from the spec to check dimensions
			var matchingRequest *quotav1alpha1.ResourceRequest
			for _, req := range claim.Spec.Requests {
				if req.ResourceType == allocation.ResourceType {
					matchingRequest = &req
					break
				}
			}
			if matchingRequest == nil {
				logger.Error(nil, "No matching request found for allocation",
					"claimName", claim.Name,
					"resourceType", allocation.ResourceType)
				continue
			}

			// Use the allocated amount from the allocation status
			totalAllocated += allocation.AllocatedAmount
			claimCount++

		}
	}

	bucket.Status.Allocated = totalAllocated
	bucket.Status.ClaimCount = claimCount

	return nil
}

// ensureBucketFromClaims creates the bucket spec from a referencing claim if found.
// It returns true if a bucket was created, false if no referencing claim was found.
func (r *AllowanceBucketController) ensureBucketFromClaims(ctx context.Context, req ctrl.Request) (bool, error) {
	var claims quotav1alpha1.ResourceClaimList
	if err := r.List(ctx, &claims, client.InNamespace(req.Namespace)); err != nil {
		return false, fmt.Errorf("failed to list ResourceClaims: %w", err)
	}
	for _, claim := range claims.Items {
		for _, request := range claim.Spec.Requests {
			name := GenerateAllowanceBucketName(claim.Namespace, request.ResourceType, claim.Spec.ConsumerRef)
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

// isResourceGrantActive checks if a ResourceGrant has an Active condition with status True.
func (r *AllowanceBucketController) isResourceGrantActive(grant *quotav1alpha1.ResourceGrant) bool {
	return apimeta.IsStatusConditionTrue(grant.Status.Conditions, quotav1alpha1.ResourceGrantActive)
}

// processPendingGrants attempts to grant pending requests that reference this bucket.
// For each eligible claim, it evaluates individual requests that match this bucket,
// reserves capacity, then marks specific request allocations as Granted/Denied.
func (r *AllowanceBucketController) processPendingGrants(ctx context.Context, bucket *quotav1alpha1.AllowanceBucket) error {
	logger := log.FromContext(ctx)
	var claims quotav1alpha1.ResourceClaimList
	if err := r.List(ctx, &claims, client.InNamespace(bucket.Namespace)); err != nil {
		return fmt.Errorf("failed to list ResourceClaims: %w", err)
	}

	// Current state for available calculation during this reconcile loop
	limit := bucket.Status.Limit
	allocated := bucket.Status.Allocated
	fieldManagerName := fmt.Sprintf("allowance-bucket-%s", bucket.Name)

	for _, claim := range claims.Items {

		// Owner must match
		if claim.Spec.ConsumerRef.Kind != bucket.Spec.ConsumerRef.Kind ||
			claim.Spec.ConsumerRef.Name != bucket.Spec.ConsumerRef.Name {
			continue
		}

		// Process each request that matches this bucket
		for _, request := range claim.Spec.Requests {
			// Skip if request doesn't match this bucket
			if request.ResourceType != bucket.Spec.ResourceType {
				continue
			}

			// Check if this request is already processed by looking at allocations
			if r.isRequestAllocationProcessed(&claim, request.ResourceType) {
				logger.V(2).Info("Request allocation already processed, skipping",
					"claimName", claim.Name,
					"resourceType", request.ResourceType)
				continue
			}

			// Check availability using current local view
			if limit-allocated < request.Amount {
				logger.Info("Insufficient quota available for request",
					"claimName", claim.Name,
					"resourceType", request.ResourceType,
					"requestAmount", request.Amount,
					"available", limit-allocated)

				// Mark this specific request as denied
				if err := r.updateRequestAllocation(ctx, &claim, request.ResourceType, quotav1alpha1.RequestAllocationDenied,
					quotav1alpha1.ResourceClaimDeniedReason,
					fmt.Sprintf("Resource quota exceeded: requested %d, available %d", request.Amount, limit-allocated),
					0, "", fieldManagerName); err != nil {
					logger.Error(err, "failed to update request allocation for denial",
						"claimName", claim.Name, "resourceType", request.ResourceType)
				}
				continue
			}

			// Reserve capacity and keep status fields self-consistent for validation
			bucket.Status.Allocated = allocated + request.Amount
			// Recompute Available with clamp to satisfy CRD validation
			bucket.Status.Available = max(0, bucket.Status.Limit-bucket.Status.Allocated)
			bucket.Status.ObservedGeneration = bucket.Generation

			if err := r.Status().Update(ctx, bucket); err != nil {
				if apierrors.IsConflict(err) {
					// Controller runtime will automatically re-queue this resource
					return nil
				}
				return fmt.Errorf("failed to update bucket during reservation: %w", err)
			}

			// Reservation successful; update local allocated for subsequent requests
			allocated = bucket.Status.Allocated

			// Mark this specific request as granted
			if err := r.updateRequestAllocation(ctx, &claim, request.ResourceType, quotav1alpha1.RequestAllocationGranted,
				quotav1alpha1.ResourceClaimGrantedReason,
				"Capacity reserved",
				request.Amount, bucket.Name, fieldManagerName); err != nil {
				logger.Error(err, "failed to update request allocation after reservation",
					"claimName", claim.Name, "resourceType", request.ResourceType)
				// Don't revert the bucket allocation - the capacity has been reserved
				return fmt.Errorf("failed to update request allocation: %w", err)
			}

		}
	}
	return nil
}

// isRequestAllocationProcessed checks if a specific request allocation has already been processed.
func (r *AllowanceBucketController) isRequestAllocationProcessed(claim *quotav1alpha1.ResourceClaim, resourceType string) bool {
	for _, allocation := range claim.Status.Allocations {
		if allocation.ResourceType == resourceType &&
			(allocation.Status == quotav1alpha1.RequestAllocationGranted || allocation.Status == quotav1alpha1.RequestAllocationDenied) {
			return true
		}
	}
	return false
}

// updateRequestAllocation updates or creates a request allocation status using Server Side Apply.
func (r *AllowanceBucketController) updateRequestAllocation(ctx context.Context, claim *quotav1alpha1.ResourceClaim,
	resourceType string, status, reason, message string, allocatedAmount int64, bucketName, fieldManagerName string) error {

	allocation := quotav1alpha1.RequestAllocation{
		ResourceType:       resourceType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		AllocatedAmount:    allocatedAmount,
		LastTransitionTime: metav1.Now(),
	}

	// Set the allocating bucket reference only when status is Granted
	if status == quotav1alpha1.RequestAllocationGranted {
		allocation.AllocatingBucket = bucketName
	}

	// Create a minimal claim object for Server Side Apply
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
			Allocations: []quotav1alpha1.RequestAllocation{allocation},
		},
	}

	// Apply the patch using Server Side Apply with our field manager
	if err := r.Status().Patch(ctx, patchClaim, client.Apply, client.FieldOwner(fieldManagerName), client.ForceOwnership); err != nil {
		return fmt.Errorf("failed to apply request allocation status: %w", err)
	}

	return nil
}

// parseResourceType splits a resourceType like "resourcemanager.miloapis.com/Project"
// into apiGroup and kind components.
func parseResourceType(resourceType string) (apiGroup, kind string) {
	parts := strings.Split(resourceType, "/")
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	// Handle core resources (no API group)
	return "", resourceType
}

// GenerateAllowanceBucketName creates a deterministic name for an AllowanceBucket.
func GenerateAllowanceBucketName(namespace, resourceType string, ownerRef quotav1alpha1.ConsumerRef) string {
	// Why: using namespace|resourceType|owner yields stable, per-owner buckets and
	// avoids cross-owner collisions. The name is truncated to remain DNS-safe.
	input := fmt.Sprintf("%s%s%s%s", namespace, resourceType, ownerRef.Kind, ownerRef.Name)
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
// when ResourceGrants or ResourceClaims change.
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
			bucketName := GenerateAllowanceBucketName(o.Namespace, request.ResourceType, o.Spec.ConsumerRef)
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

// findBucketsForResourceType finds all AllowanceBuckets for a given resource type and owner.
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
