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

// ResourceClaimOwnershipController sets owner references on ResourceClaims
// after their target resources are created.
type ResourceClaimOwnershipController struct {
	client.Client
	DynamicClient    dynamic.Interface
	DiscoveryClient  discovery.DiscoveryInterface
	Scheme           *runtime.Scheme
	logger           logr.Logger
	// Cache for resource versions to avoid repeated discovery calls
	versionCache     sync.Map // key: group/kind -> value: version
	versionCacheLock sync.RWMutex
}

// +kubebuilder:rbac:groups=quota.miloapis.com,resources=resourceclaims,verbs=get;list;watch;update;patch;delete
// +kubebuilder:rbac:groups=quota.miloapis.com,resources=claimcreationpolicies,verbs=get;list;watch
// +kubebuilder:rbac:groups=*,resources=*,verbs=get

// Reconcile sets owner references on ResourceClaims when their target resources exist.
func (r *ResourceClaimOwnershipController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithName("resourceclaim-ownership")
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

	// Skip if already has owner references
	if len(claim.OwnerReferences) > 0 {
		logger.V(2).Info("ResourceClaim already has owner references", "claim", claim.Name)
		return ctrl.Result{}, nil
	}

	// Skip if being deleted
	if !claim.DeletionTimestamp.IsZero() {
		logger.V(1).Info("ResourceClaim is being deleted, skipping", "claim", claim.Name)
		return ctrl.Result{}, nil
	}

	// PROTECTION: Minimum age to handle resource creation delays
	claimAge := time.Since(claim.CreationTimestamp.Time)
	minAge := r.getMinAge()

	if claimAge < minAge {
		remainingWait := minAge - claimAge
		logger.V(1).Info("ResourceClaim too young, waiting for minimum age",
			"claim", claim.Name,
			"age", claimAge,
			"minAge", minAge,
			"remainingWait", remainingWait)
		return ctrl.Result{RequeueAfter: remainingWait}, nil
	}

	logger.V(1).Info("Processing ResourceClaim for owner reference",
		"claim", claim.Name,
		"age", claimAge,
		"ownerInstanceRef", claim.Spec.OwnerInstanceRef)

	// Try to find target resource
	targetObj, err := r.findTargetResource(ctx, &claim)
	if err != nil {
		if errors.IsNotFound(err) {
			// Check if claim is very old without a resource
			maxAge := r.getMaxAge()
			if claimAge > maxAge {
				logger.Error(nil, "Deleting very old ResourceClaim without target resource",
					"claim", claim.Name,
					"age", claimAge,
					"maxAge", maxAge,
					"targetName", claim.Spec.OwnerInstanceRef.Name)
				return ctrl.Result{}, r.Delete(ctx, &claim)
			}

			// Resource doesn't exist yet - keep waiting
			logger.V(1).Info("Target resource not found, will retry",
				"claim", claim.Name,
				"targetName", claim.Spec.OwnerInstanceRef.Name,
				"age", claimAge)
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to find target resource: %w", err)
	}

	// Resource exists - set owner reference
	claim.OwnerReferences = []metav1.OwnerReference{{
		APIVersion:         targetObj.GetAPIVersion(),
		Kind:               targetObj.GetKind(),
		Name:               targetObj.GetName(),
		UID:                targetObj.GetUID(),
		Controller:         &[]bool{false}[0], // Not a controller reference
		BlockOwnerDeletion: &[]bool{true}[0],  // Block deletion until claim is cleaned up
	}}

	logger.Info("Set owner reference on ResourceClaim",
		"claim", claim.Name,
		"target", targetObj.GetName(),
		"targetKind", targetObj.GetKind(),
		"targetUID", targetObj.GetUID(),
		"claimAge", claimAge)

	return ctrl.Result{}, r.Update(ctx, &claim)
}

// findTargetResource finds the target resource referenced by the ResourceClaim.
func (r *ResourceClaimOwnershipController) findTargetResource(ctx context.Context, claim *quotav1alpha1.ResourceClaim) (*unstructured.Unstructured, error) {
	// Get the API group from OwnerInstanceRef
	apiGroup := claim.Spec.OwnerInstanceRef.APIGroup
	kind := claim.Spec.OwnerInstanceRef.Kind
	
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

	// Determine namespace - cluster-scoped resources have empty namespace
	namespace := ""
	gvk := schema.GroupVersionKind{
		Group:   apiGroup,
		Version: version,
		Kind:    claim.Spec.OwnerInstanceRef.Kind,
	}
	if !r.isClusterScoped(gvk) {
		// For namespaced resources, use the claim's namespace
		namespace = claim.Namespace
	}

	// Get target resource using the UID for verification
	resource, err := r.DynamicClient.Resource(gvr).
		Namespace(namespace).
		Get(ctx, claim.Spec.OwnerInstanceRef.Name, metav1.GetOptions{})

	if err != nil {
		return nil, err
	}

	// Verify UID matches to ensure we found the correct resource
	if string(resource.GetUID()) != claim.Spec.OwnerInstanceRef.UID {
		return nil, fmt.Errorf("resource UID mismatch: expected %s, found %s",
			claim.Spec.OwnerInstanceRef.UID, resource.GetUID())
	}

	return resource, nil
}

// kindToResource converts a Kind to its corresponding resource name.
func (r *ResourceClaimOwnershipController) kindToResource(kind string) string {
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
func (r *ResourceClaimOwnershipController) isClusterScoped(gvk schema.GroupVersionKind) bool {
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

// getMinAge returns the minimum age before processing claims.
func (r *ResourceClaimOwnershipController) getMinAge() time.Duration {
	if envVal := os.Getenv("RESOURCECLAIM_MIN_AGE"); envVal != "" {
		if duration, err := time.ParseDuration(envVal); err == nil {
			return duration
		}
	}
	return 1 * time.Minute // Conservative default
}

// getMaxAge returns the maximum age before deleting orphaned claims.
func (r *ResourceClaimOwnershipController) getMaxAge() time.Duration {
	if envVal := os.Getenv("RESOURCECLAIM_MAX_AGE"); envVal != "" {
		if duration, err := time.ParseDuration(envVal); err == nil {
			return duration
		}
	}
	return 2 * time.Minute // Default maximum age
}

// discoverResourceVersion discovers the correct API version for a given group and kind.
func (r *ResourceClaimOwnershipController) discoverResourceVersion(apiGroup, kind string) (string, error) {
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
func (r *ResourceClaimOwnershipController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&quotav1alpha1.ResourceClaim{}).
		Complete(r)
}
