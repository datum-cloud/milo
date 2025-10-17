package lifecycle

import (
	"context"
	"fmt"
	"os"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	quotav1alpha1 "go.miloapis.com/milo/pkg/apis/quota/v1alpha1"
)

// OrphanStatus represents the states of a potentially orphaned ResourceClaim
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

// ResourceClaimOwnershipController establishes owner references on ResourceClaims and
// performs safety-net cleanup for true orphans.
//
//   - Fast path: When a claim is Granted=True and has no ownerRefs, resolve and set a
//     single controller ownerRef via Server-Side Apply.
//   - Safety net: After a grace period, rescue claims whose owner now exists; delete
//     claims past a max age if the owner still doesn’t exist.
type ResourceClaimOwnershipController struct {
	client.Client
	DynamicClient dynamic.Interface
	Scheme        *runtime.Scheme

	// RESTMapper for reliable GVK<->GVR resolution and scope detection
	restMapper meta.RESTMapper
}

// +kubebuilder:rbac:groups=quota.miloapis.com,resources=resourceclaims,verbs=get;list;watch;update;patch;delete
// +kubebuilder:rbac:groups=*,resources=*,verbs=get;list;watch

// Reconcile identifies and cleans up orphaned ResourceClaims.
// This controller focuses on safety-net functionality rather than immediate ownership creation.
func (r *ResourceClaimOwnershipController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithName("resourceclaim-ownership")

	// Get the ResourceClaim
	var claim quotav1alpha1.ResourceClaim
	if err := r.Get(ctx, req.NamespacedName, &claim); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to get ResourceClaim: %w", err)
	}

	// Skip if being deleted
	if !claim.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	// Only process granted claims
	if !isResourceClaimGranted(&claim) {
		logger.V(2).Info("Claim not granted; skipping", "name", claim.Name, "ns", claim.Namespace)
		return ctrl.Result{}, nil
	}

	// Skip if already has owner references
	if len(claim.OwnerReferences) > 0 {
		logger.V(2).Info("ResourceClaim has owner references, skipping", "claim", claim.Name)
		return ctrl.Result{}, nil
	}

	claimAge := time.Since(claim.CreationTimestamp.Time)

	// Fast path: attempt to resolve owner immediately and set ownerRef
	ownerObj, _, _, err := r.resolveOwner(ctx, &claim)
	if err == nil && ownerObj != nil {
		if err := r.applyOwnerReferenceSSA(ctx, &claim, ownerObj); err != nil {
			logger.Error(err, "Failed to set ownerReference via SSA; requeue")
			return ctrl.Result{RequeueAfter: 500 * time.Millisecond}, nil
		}
		return ctrl.Result{}, nil
	}

	// If owner not found, apply safety net logic based on age thresholds
	if apierrors.IsNotFound(err) || err == nil {
		// Owner missing; evaluate grace and max age
		grace := r.getOwnershipGracePeriod()
		if claimAge < grace {
			// Short requeue to check again soon
			return ctrl.Result{RequeueAfter: 2 * time.Second}, nil
		}

		// Beyond grace: try once more to resolve owner and rescue
		ownerObj2, _, _, err2 := r.resolveOwner(ctx, &claim)
		if err2 == nil && ownerObj2 != nil {
			if err := r.applyOwnerReferenceSSA(ctx, &claim, ownerObj2); err != nil {
				logger.Error(err, "Failed to rescue owner reference via SSA; requeue")
				return ctrl.Result{RequeueAfter: time.Second}, nil
			}
			return ctrl.Result{}, nil
		}

		// If max age exceeded without owner, delete the claim
		maxAge := r.getOrphanMaxAge()
		if claimAge > maxAge {
			logger.Info("Deleting orphaned ResourceClaim after max age", "claim", claim.Name, "age", claimAge, "maxAge", maxAge)
			return ctrl.Result{}, r.Delete(ctx, &claim)
		}

		// Not beyond max age; requeue to check later
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Unexpected error when resolving owner
	return ctrl.Result{}, err
}

// rescueOrphanedClaim adds an owner reference via SSA
func (r *ResourceClaimOwnershipController) rescueOrphanedClaim(ctx context.Context, claim *quotav1alpha1.ResourceClaim, claimingResource *unstructured.Unstructured) error {
	return r.applyOwnerReferenceSSA(ctx, claim, claimingResource)
}

// findTargetResource finds the target resource referenced by the ResourceClaim using RESTMapper.
func (r *ResourceClaimOwnershipController) findTargetResource(ctx context.Context, claim *quotav1alpha1.ResourceClaim) (*unstructured.Unstructured, error) {
	gk := schema.GroupKind{Group: claim.Spec.ResourceRef.APIGroup, Kind: claim.Spec.ResourceRef.Kind}
	mapping, err := r.restMapper.RESTMapping(gk)
	if err != nil {
		return nil, err
	}
	gvr := mapping.Resource

	// Determine namespace based on scope
	namespace := ""
	if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
		if claim.Spec.ResourceRef.Namespace != "" {
			namespace = claim.Spec.ResourceRef.Namespace
		} else {
			namespace = claim.Namespace
		}
	}

	resource, err := r.DynamicClient.Resource(gvr).Namespace(namespace).Get(ctx, claim.Spec.ResourceRef.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return resource, nil
}

// getOwnershipGracePeriod returns the grace period before considering a claim for orphan analysis.
func (r *ResourceClaimOwnershipController) getOwnershipGracePeriod() time.Duration {
	if envVal := os.Getenv("RESOURCECLAIM_GRACE_PERIOD"); envVal != "" {
		if duration, err := time.ParseDuration(envVal); err == nil {
			return duration
		}
	}
	return 30 * time.Second
}

// getOrphanMaxAge returns the maximum age before deleting truly orphaned claims.
func (r *ResourceClaimOwnershipController) getOrphanMaxAge() time.Duration {
	if envVal := os.Getenv("RESOURCECLAIM_MAX_ORPHAN_AGE"); envVal != "" {
		if duration, err := time.ParseDuration(envVal); err == nil {
			return duration
		}
	}
	return 30 * time.Second
}

// SetupWithManager sets up the controller with the Manager.
func (r *ResourceClaimOwnershipController) SetupWithManager(mgr ctrl.Manager) error {
	r.restMapper = mgr.GetRESTMapper()

	onlyGrantedMissingOwner := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			claim, ok := e.Object.(*quotav1alpha1.ResourceClaim)
			if !ok {
				return false
			}
			return isResourceClaimGranted(claim) && len(claim.GetOwnerReferences()) == 0
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			newClaim, ok := e.ObjectNew.(*quotav1alpha1.ResourceClaim)
			if !ok {
				return false
			}
			if !(isResourceClaimGranted(newClaim) && len(newClaim.GetOwnerReferences()) == 0) {
				return false
			}
			oldClaim, ok := e.ObjectOld.(*quotav1alpha1.ResourceClaim)
			if !ok {
				return true
			}
			return !(isResourceClaimGranted(oldClaim) && len(oldClaim.GetOwnerReferences()) == 0)
		},
		DeleteFunc:  func(event.DeleteEvent) bool { return false },
		GenericFunc: func(event.GenericEvent) bool { return false },
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&quotav1alpha1.ResourceClaim{}, builder.WithPredicates(onlyGrantedMissingOwner)).
		Named("resource-claim-ownership").
		Complete(r)
}

// resolveOwner resolves the owning object and returns it, along with its GVK and namespace used for GET.
func (r *ResourceClaimOwnershipController) resolveOwner(ctx context.Context, claim *quotav1alpha1.ResourceClaim) (*unstructured.Unstructured, schema.GroupVersionKind, string, error) {
	if r.restMapper == nil {
		return nil, schema.GroupVersionKind{}, "", fmt.Errorf("RESTMapper not initialized")
	}
	gk := schema.GroupKind{Group: claim.Spec.ResourceRef.APIGroup, Kind: claim.Spec.ResourceRef.Kind}
	mapping, err := r.restMapper.RESTMapping(gk)
	if err != nil {
		return nil, schema.GroupVersionKind{}, "", err
	}
	gvr := mapping.Resource
	gvk := mapping.GroupVersionKind

	ownerNS := ""
	if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
		if claim.Spec.ResourceRef.Namespace != "" {
			ownerNS = claim.Spec.ResourceRef.Namespace
		} else {
			ownerNS = claim.Namespace
		}
	}

	obj, err := r.DynamicClient.Resource(gvr).Namespace(ownerNS).Get(ctx, claim.Spec.ResourceRef.Name, metav1.GetOptions{})
	if err != nil {
		return nil, schema.GroupVersionKind{}, ownerNS, err
	}
	if obj.GetAPIVersion() == "" {
		obj.SetAPIVersion(gvk.GroupVersion().String())
	}
	if obj.GetKind() == "" {
		obj.SetKind(gvk.Kind)
	}
	return obj, gvk, ownerNS, nil
}

// applyOwnerReferenceSSA sets a single controller ownerReference on the claim using Server-Side Apply.
func (r *ResourceClaimOwnershipController) applyOwnerReferenceSSA(ctx context.Context, claim *quotav1alpha1.ResourceClaim, owner *unstructured.Unstructured) error {
	// Use unstructured to ensure we only manage metadata.ownerReferences via SSA.
	patch := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "quota.miloapis.com/v1alpha1",
		"kind":       "ResourceClaim",
		"metadata": map[string]interface{}{
			"name":      claim.Name,
			"namespace": claim.Namespace,
			"ownerReferences": []interface{}{
				map[string]interface{}{
					"apiVersion":         owner.GetAPIVersion(),
					"kind":               owner.GetKind(),
					"name":               owner.GetName(),
					"uid":                string(owner.GetUID()),
					"controller":         false,
					"blockOwnerDeletion": false,
				},
			},
		},
	}}
	// Use a dedicated field manager that never touched spec to avoid SSA conflicts
	return r.Patch(ctx, patch, client.Apply, client.FieldOwner("resourceclaim-ownership-metadata"))
}

// isResourceClaimGranted checks if the claim has Granted=True.
func isResourceClaimGranted(claim *quotav1alpha1.ResourceClaim) bool {
	return meta.IsStatusConditionTrue(claim.Status.Conditions, quotav1alpha1.ResourceClaimGranted)
}
