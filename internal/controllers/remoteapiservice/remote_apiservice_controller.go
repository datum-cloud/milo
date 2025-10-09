package remoteapiservice

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apiregv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	apiregv1helper "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1/helper"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// -----------------------------------------------------------------------------
// RBAC (kubebuilder markers)
// -----------------------------------------------------------------------------
// +kubebuilder:rbac:groups=apiregistration.k8s.io,resources=apiservices,verbs=get;list;watch
// +kubebuilder:rbac:groups=apiregistration.k8s.io,resources=apiservices/status,verbs=patch;update

// RemoteAPIServiceAvailabilityReconciler ensures Available=True for remote APIServices.
type RemoteAPIServiceAvailabilityReconciler struct {
	client.Client

	// Optional settings
	Reason      string        // default: "Remote"
	Message     string        // default: "Availability managed by custom controller"
	ResyncEvery time.Duration // default: 0 (disabled)
}

func (r *RemoteAPIServiceAvailabilityReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var apiSvc apiregv1.APIService
	if err := r.Get(ctx, req.NamespacedName, &apiSvc); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Only touch "remote" APIServices
	if apiSvc.Spec.Service == nil {
		return ctrl.Result{}, nil
	}

	reason := r.Reason
	if reason == "" {
		reason = "Remote"
	}
	message := r.Message
	if message == "" {
		message = "Availability managed by custom controller"
	}

	// Desired condition (note: Status type is apiregv1.ConditionStatus)
	desired := apiregv1.APIServiceCondition{
		Type:               apiregv1.Available,
		Status:             apiregv1.ConditionTrue,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	}

	// Prepare patch: only write if status actually changes.
	orig := apiSvc.DeepCopy()
	apiregv1helper.SetAPIServiceCondition(&apiSvc, desired)

	if equality.Semantic.DeepEqual(orig.Status, apiSvc.Status) {
		if r.ResyncEvery > 0 {
			return ctrl.Result{RequeueAfter: r.ResyncEvery}, nil
		}
		return ctrl.Result{}, nil
	}

	if err := r.Status().Patch(ctx, &apiSvc, client.MergeFrom(orig)); err != nil {
		return ctrl.Result{}, err
	}

	if r.ResyncEvery > 0 {
		return ctrl.Result{RequeueAfter: r.ResyncEvery}, nil
	}
	return ctrl.Result{}, nil
}

func (r *RemoteAPIServiceAvailabilityReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Only enqueue "remote" APIServices to reduce churn.
	remoteOnly := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		if as, ok := obj.(*apiregv1.APIService); ok {
			return as.Spec.Service != nil
		}
		return false
	})
	return ctrl.NewControllerManagedBy(mgr).
		For(&apiregv1.APIService{}).
		WithEventFilter(remoteOnly).
		Complete(r)
}
