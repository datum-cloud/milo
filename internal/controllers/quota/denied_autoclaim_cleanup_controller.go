package quota

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	quotav1alpha1 "go.miloapis.com/milo/pkg/apis/quota/v1alpha1"
)

// DeniedAutoClaimCleanupController automatically deletes denied ResourceClaims
// that were created by the admission plugin, while leaving manually created claims untouched.
type DeniedAutoClaimCleanupController struct {
	client.Client
	Scheme *runtime.Scheme
	logger logr.Logger
}

// NewDeniedAutoClaimCleanupController creates a new DeniedAutoClaimCleanupController.
func NewDeniedAutoClaimCleanupController(
	client client.Client,
	scheme *runtime.Scheme,
) *DeniedAutoClaimCleanupController {
	return &DeniedAutoClaimCleanupController{
		Client: client,
		Scheme: scheme,
		logger: ctrl.Log.WithName("denied-autoclaim-cleanup"),
	}
}

// +kubebuilder:rbac:groups=quota.miloapis.com,resources=resourceclaims,verbs=get;list;watch;delete

// Reconcile processes ResourceClaims and deletes those that are:
// 1. Auto-created by the admission plugin
// 2. Denied (status.conditions[type=Granted,status=False,reason=QuotaExceeded])
func (r *DeniedAutoClaimCleanupController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("claim", req.Name, "namespace", req.Namespace)

	var claim quotav1alpha1.ResourceClaim
	if err := r.Get(ctx, req.NamespacedName, &claim); err != nil {
		// Claim was deleted or doesn't exist - nothing to do
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	logger.V(2).Info("Processing ResourceClaim for cleanup evaluation")

	// Filter 1: Only process auto-created claims
	if !r.isAutoCreatedClaim(&claim) {
		logger.V(3).Info("Skipping manually created claim")
		return ctrl.Result{}, nil
	}

	// Filter 2: Only process denied claims
	if !r.isClaimDenied(&claim) {
		logger.V(3).Info("Skipping non-denied claim")
		return ctrl.Result{}, nil
	}

	// Delete the denied auto-created claim immediately
	logger.Info("Deleting denied auto-created ResourceClaim",
		"policy", claim.Labels["quota.miloapis.com/policy"],
		"resourceName", claim.Annotations["quota.miloapis.com/resource-name"],
		"denialReason", r.getClaimDenialReason(&claim))

	if err := r.Delete(ctx, &claim); err != nil {
		logger.Error(err, "Failed to delete denied auto-created ResourceClaim")
		return ctrl.Result{}, fmt.Errorf("failed to delete denied auto-created ResourceClaim: %w", err)
	}

	logger.V(1).Info("Successfully deleted denied auto-created ResourceClaim")
	return ctrl.Result{}, nil
}

// isAutoCreatedClaim checks if a ResourceClaim was automatically created by the admission plugin.
// Returns true only if both the label and annotation markers are present.
func (r *DeniedAutoClaimCleanupController) isAutoCreatedClaim(claim *quotav1alpha1.ResourceClaim) bool {
	// Check both label and annotation for safety
	autoCreatedLabel := claim.Labels["quota.miloapis.com/auto-created"] == "true"
	createdByPlugin := claim.Annotations["quota.miloapis.com/created-by"] == "claim-creation-plugin"

	r.logger.V(3).Info("Checking auto-created markers",
		"claim", claim.Name,
		"autoCreatedLabel", autoCreatedLabel,
		"createdByPlugin", createdByPlugin)

	return autoCreatedLabel && createdByPlugin
}

// isClaimDenied checks if a ResourceClaim has been denied due to quota exceeded.
func (r *DeniedAutoClaimCleanupController) isClaimDenied(claim *quotav1alpha1.ResourceClaim) bool {
	for _, condition := range claim.Status.Conditions {
		if condition.Type == quotav1alpha1.ResourceClaimGranted &&
			condition.Status == metav1.ConditionFalse &&
			condition.Reason == quotav1alpha1.ResourceClaimDeniedReason {
			return true
		}
	}
	return false
}

// getClaimDenialReason returns the reason why a ResourceClaim was denied.
func (r *DeniedAutoClaimCleanupController) getClaimDenialReason(claim *quotav1alpha1.ResourceClaim) string {
	for _, condition := range claim.Status.Conditions {
		if condition.Type == quotav1alpha1.ResourceClaimGranted &&
			condition.Status == metav1.ConditionFalse {
			if condition.Message != "" {
				return condition.Message
			}
			return condition.Reason
		}
	}
	return "unknown"
}

// SetupWithManager sets up the controller with the Manager and configures efficient filtering.
func (r *DeniedAutoClaimCleanupController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&quotav1alpha1.ResourceClaim{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 5,
		}).
		// Use predicate to filter at the watch level for efficiency
		WithEventFilter(predicate.NewPredicateFuncs(func(obj client.Object) bool {
			claim, ok := obj.(*quotav1alpha1.ResourceClaim)
			if !ok {
				return false
			}

			// Only watch auto-created claims to reduce controller load
			autoCreated := claim.Labels["quota.miloapis.com/auto-created"] == "true"
			createdByPlugin := claim.Annotations["quota.miloapis.com/created-by"] == "claim-creation-plugin"

			return autoCreated && createdByPlugin
		})).
		Named("denied-auto-claim-cleanup").
		Complete(r)
}
