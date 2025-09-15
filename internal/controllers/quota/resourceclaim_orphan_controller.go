package quota

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	quotav1alpha1 "go.miloapis.com/milo/pkg/apis/quota/v1alpha1"
)

// OrphanStatus represents the different states of a potentially orphaned ResourceClaim
type OrphanStatus int

const (
	// OrphanStatusKeepWaiting indicates the claim is not yet considered orphaned
	OrphanStatusKeepWaiting OrphanStatus = iota
	// OrphanStatusCanBeRescued indicates the claiming resource exists and we can add owner reference
	OrphanStatusCanBeRescued
	// OrphanStatusShouldBeDeleted indicates the claim is truly orphaned and should be cleaned up
	OrphanStatusShouldBeDeleted
)

// OrphanAnalysis contains the result of analyzing a potentially orphaned ResourceClaim
type OrphanAnalysis struct {
	Status           OrphanStatus
	Reason           string
	ClaimingResource *unstructured.Unstructured // Only set if Status is CanBeRescued
}

// ResourceClaimOrphanController identifies and cleans up orphaned ResourceClaims.
//
// This controller serves as a safety net for ResourceClaims that don't have owner references
// after sufficient time has passed. It focuses on orphan detection and cleanup rather than
// immediate ownership creation (which is handled by DynamicOwnershipController).
type ResourceClaimOrphanController struct {
	client.Client
	DynamicClient   dynamic.Interface
	DiscoveryClient discovery.DiscoveryInterface
	Scheme          *runtime.Scheme
	logger          logr.Logger
	// Cache for resource versions to avoid repeated discovery calls
	versionCache sync.Map // key: group/kind -> value: version
}

// +kubebuilder:rbac:groups=quota.miloapis.com,resources=resourceclaims,verbs=get;list;watch;update;patch;delete
// +kubebuilder:rbac:groups=quota.miloapis.com,resources=claimcreationpolicies,verbs=get;list;watch
// +kubebuilder:rbac:groups=*,resources=*,verbs=get

// Reconcile identifies and cleans up orphaned ResourceClaims.
// This controller focuses on safety-net functionality rather than immediate ownership creation.
func (r *ResourceClaimOrphanController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithName("resourceclaim-orphan-cleanup")
	r.logger = logger

	// Get the ResourceClaim
	var claim quotav1alpha1.ResourceClaim
	if err := r.Get(ctx, req.NamespacedName, &claim); err != nil {
		if errors.IsNotFound(err) {
			logger.V(1).Info("ResourceClaim not found")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to get ResourceClaim: %w", err)
	}

	// Skip if already has owner references (DynamicOwnershipController succeeded)
	if len(claim.OwnerReferences) > 0 {
		logger.V(2).Info("ResourceClaim has owner references, skipping", "claim", claim.Name)
		return ctrl.Result{}, nil
	}

	// Skip if being deleted
	if !claim.DeletionTimestamp.IsZero() {
		logger.V(1).Info("ResourceClaim is being deleted, skipping", "claim", claim.Name)
		return ctrl.Result{}, nil
	}

	claimAge := time.Since(claim.CreationTimestamp.Time)

	// Wait longer before processing to give DynamicOwnershipController time to work
	gracePeriod := r.getGracePeriod()
	if claimAge < gracePeriod {
		remainingWait := gracePeriod - claimAge
		logger.V(2).Info("ResourceClaim still within grace period, waiting for DynamicOwnershipController",
			"claim", claim.Name,
			"age", claimAge,
			"gracePeriod", gracePeriod,
			"remainingWait", remainingWait)
		return ctrl.Result{RequeueAfter: remainingWait}, nil
	}

	logger.V(1).Info("Processing potentially orphaned ResourceClaim",
		"claim", claim.Name,
		"age", claimAge,
		"resourceRef", claim.Spec.ResourceRef)

	// Check if this is a legitimate orphan or if we should try to rescue it
	orphanStatus := r.analyzeOrphanStatus(ctx, &claim)

	switch orphanStatus.Status {
	case OrphanStatusCanBeRescued:
		logger.Info("Attempting to rescue ResourceClaim by adding missing owner reference",
			"claim", claim.Name,
			"age", claimAge,
			"reason", orphanStatus.Reason)

		if err := r.rescueOrphanedClaim(ctx, &claim, orphanStatus.ClaimingResource); err != nil {
			logger.Error(err, "Failed to rescue orphaned ResourceClaim", "claim", claim.Name)
			return ctrl.Result{RequeueAfter: 60 * time.Second}, nil
		}

		logger.Info("Successfully rescued orphaned ResourceClaim", "claim", claim.Name)
		return ctrl.Result{}, nil

	case OrphanStatusShouldBeDeleted:
		logger.Info("Deleting orphaned ResourceClaim",
			"claim", claim.Name,
			"age", claimAge,
			"reason", orphanStatus.Reason)

		return ctrl.Result{}, r.Delete(ctx, &claim)

	case OrphanStatusKeepWaiting:
		logger.V(1).Info("ResourceClaim not yet considered orphaned, continuing to wait",
			"claim", claim.Name,
			"age", claimAge,
			"reason", orphanStatus.Reason)

		return ctrl.Result{RequeueAfter: 120 * time.Second}, nil

	default:
		logger.Error(fmt.Errorf("unknown orphan status"), "Unknown orphan status",
			"claim", claim.Name, "status", orphanStatus.Status)
		return ctrl.Result{RequeueAfter: 60 * time.Second}, nil
	}
}

// analyzeOrphanStatus determines if a ResourceClaim is orphaned and what action to take
func (r *ResourceClaimOrphanController) analyzeOrphanStatus(ctx context.Context, claim *quotav1alpha1.ResourceClaim) OrphanAnalysis {
	claimAge := time.Since(claim.CreationTimestamp.Time)
	maxAge := r.getMaxOrphanAge()

	// Try to find the claiming resource
	claimingResource, err := r.findTargetResource(ctx, claim)
	if err == nil && claimingResource != nil {
		// Claiming resource exists - this claim can be rescued
		return OrphanAnalysis{
			Status:           OrphanStatusCanBeRescued,
			Reason:           "Claiming resource exists but owner reference is missing (likely due to timing or missed event)",
			ClaimingResource: claimingResource,
		}
	}

	// Claiming resource doesn't exist - check if it's beyond the max age
	if claimAge > maxAge {
		return OrphanAnalysis{
			Status: OrphanStatusShouldBeDeleted,
			Reason: fmt.Sprintf("ResourceClaim has no claiming resource after %v (max age: %v)", claimAge, maxAge),
		}
	}

	// Still within acceptable time window - keep waiting
	return OrphanAnalysis{
		Status: OrphanStatusKeepWaiting,
		Reason: fmt.Sprintf("ResourceClaim age %v is within max orphan age %v, continuing to wait for claiming resource", claimAge, maxAge),
	}
}

// rescueOrphanedClaim adds an owner reference to a ResourceClaim that has a claiming resource
func (r *ResourceClaimOrphanController) rescueOrphanedClaim(ctx context.Context, claim *quotav1alpha1.ResourceClaim, claimingResource *unstructured.Unstructured) error {
	// Create owner reference
	ownerRef := metav1.OwnerReference{
		APIVersion:         claimingResource.GetAPIVersion(),
		Kind:               claimingResource.GetKind(),
		Name:               claimingResource.GetName(),
		UID:                claimingResource.GetUID(),
		Controller:         func() *bool { b := false; return &b }(), // Not a controller reference
		BlockOwnerDeletion: func() *bool { b := true; return &b }(),  // Block deletion until claim is cleaned up
	}

	// Update the claim with owner reference
	updatedClaim := claim.DeepCopy()
	updatedClaim.SetOwnerReferences([]metav1.OwnerReference{ownerRef})

	if err := r.Update(ctx, updatedClaim); err != nil {
		return fmt.Errorf("failed to update ResourceClaim with owner reference: %w", err)
	}

	r.logger.Info("Rescued orphaned ResourceClaim by adding owner reference",
		"claim", claim.Name,
		"claimingResource", claimingResource.GetName(),
		"claimingResourceKind", claimingResource.GetKind(),
		"claimingResourceUID", claimingResource.GetUID())

	return nil
}

// findTargetResource finds the target resource referenced by the ResourceClaim.
func (r *ResourceClaimOrphanController) findTargetResource(ctx context.Context, claim *quotav1alpha1.ResourceClaim) (*unstructured.Unstructured, error) {
	// Get the API group from ResourceRef (the claiming resource)
	apiGroup := claim.Spec.ResourceRef.APIGroup
	kind := claim.Spec.ResourceRef.Kind

	// Get the correct version for this resource using discovery
	version, err := r.discoverResourceVersion(apiGroup, kind)
	if err != nil {
		return nil, fmt.Errorf("failed to discover version for %s/%s: %w", apiGroup, kind, err)
	}

	// Build GVR for dynamic client
	gvr := schema.GroupVersionResource{
		Group:    apiGroup,
		Version:  version,
		Resource: r.kindToResource(kind),
	}

	// Determine namespace - use ResourceRef namespace if specified, otherwise use claim's namespace
	namespace := claim.Spec.ResourceRef.Namespace
	gvk := schema.GroupVersionKind{
		Group:   apiGroup,
		Version: version,
		Kind:    claim.Spec.ResourceRef.Kind,
	}
	if r.isClusterScoped(gvk) {
		// Cluster-scoped resources always have empty namespace
		namespace = ""
	} else if namespace == "" {
		// For namespaced resources without explicit namespace, use the claim's namespace
		namespace = claim.Namespace
	}

	// Get target resource using the name from ResourceRef
	resource, err := r.DynamicClient.Resource(gvr).
		Namespace(namespace).
		Get(ctx, claim.Spec.ResourceRef.Name, metav1.GetOptions{})

	if err != nil {
		return nil, err
	}

	// Note: UID verification removed for quota scenarios to support name/kind matching

	return resource, nil
}

// kindToResource converts a Kind to its corresponding resource name.
func (r *ResourceClaimOrphanController) kindToResource(kind string) string {
	// Simple pluralization - this could be enhanced with a proper mapping
	lower := strings.ToLower(kind)
	if strings.HasSuffix(lower, "s") {
		return lower + "es"
	}
	if strings.HasSuffix(lower, "y") {
		return strings.TrimSuffix(lower, "y") + "ies"
	}
	return lower + "s"
}

// isClusterScoped determines if a GVK represents a cluster-scoped resource.
func (r *ResourceClaimOrphanController) isClusterScoped(gvk schema.GroupVersionKind) bool {
	// Common cluster-scoped resources
	clusterScopedKinds := map[string]bool{
		"Namespace":            true,
		"Node":                 true,
		"ClusterRole":          true,
		"ClusterRoleBinding":   true,
		"PersistentVolume":     true,
		"StorageClass":         true,
		"Organization":         true, // Milo-specific
		"ClaimCreationPolicy":  true, // Milo-specific
		"ResourceRegistration": true, // Milo-specific
	}

	return clusterScopedKinds[gvk.Kind]
}

// getGracePeriod returns the grace period before considering a claim for orphan analysis.
func (r *ResourceClaimOrphanController) getGracePeriod() time.Duration {
	if envVal := os.Getenv("RESOURCECLAIM_GRACE_PERIOD"); envVal != "" {
		if duration, err := time.ParseDuration(envVal); err == nil {
			return duration
		}
	}
	return 5 * time.Minute // Longer grace period to let DynamicOwnershipController work
}

// getMaxOrphanAge returns the maximum age before deleting truly orphaned claims.
func (r *ResourceClaimOrphanController) getMaxOrphanAge() time.Duration {
	if envVal := os.Getenv("RESOURCECLAIM_MAX_ORPHAN_AGE"); envVal != "" {
		if duration, err := time.ParseDuration(envVal); err == nil {
			return duration
		}
	}
	return 30 * time.Second
}

// discoverResourceVersion discovers the correct API version for a given group and kind.
func (r *ResourceClaimOrphanController) discoverResourceVersion(apiGroup, kind string) (string, error) {
	// Check cache first
	cacheKey := fmt.Sprintf("%s/%s", apiGroup, kind)
	if cached, ok := r.versionCache.Load(cacheKey); ok {
		return cached.(string), nil
	}

	// If discovery client is not available, fall back to defaults
	if r.DiscoveryClient == nil {
		// Default versions for known resources
		if apiGroup == "" {
			return "v1", nil // Core API group
		}
		return "v1alpha1", nil // Default for custom resources
	}

	// Use discovery to find the preferred version
	apiGroupList, err := r.DiscoveryClient.ServerGroups()
	if err != nil {
		r.logger.Error(err, "Failed to discover server groups, using defaults",
			"apiGroup", apiGroup, "kind", kind)
		// Fall back to defaults on discovery failure
		if apiGroup == "" {
			return "v1", nil
		}
		return "v1alpha1", nil
	}

	// Find the group in the list
	for _, group := range apiGroupList.Groups {
		if group.Name == apiGroup {
			// Use the preferred version
			if group.PreferredVersion.Version != "" {
				version := group.PreferredVersion.Version
				// Cache the result
				r.versionCache.Store(cacheKey, version)
				r.logger.V(1).Info("Discovered resource version",
					"apiGroup", apiGroup,
					"kind", kind,
					"version", version)
				return version, nil
			}
			// If no preferred version, use the first available version
			if len(group.Versions) > 0 {
				version := group.Versions[0].Version
				// Cache the result
				r.versionCache.Store(cacheKey, version)
				r.logger.V(1).Info("Using first available version",
					"apiGroup", apiGroup,
					"kind", kind,
					"version", version)
				return version, nil
			}
		}
	}

	// Special handling for core API group (empty string)
	if apiGroup == "" {
		r.versionCache.Store(cacheKey, "v1")
		return "v1", nil
	}

	// If group not found, try to use a reasonable default
	r.logger.Info("API group not found in discovery, using default version",
		"apiGroup", apiGroup,
		"kind", kind,
		"defaultVersion", "v1alpha1")
	r.versionCache.Store(cacheKey, "v1alpha1")
	return "v1alpha1", nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ResourceClaimOrphanController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&quotav1alpha1.ResourceClaim{}).
		Named("resource-claim-orphan-cleanup").
		Complete(r)
}
