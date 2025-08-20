package iam

import (
	"context"
	"fmt"

	iamv1alpha1 "go.miloapis.com/milo/pkg/apis/iam/v1alpha1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	userFinalizerKey       = "iam.miloapis.com/user"
	userReadyConditionType = "Ready"
)

// UserController reconciles a User object
type UserController struct {
	Client client.Client
}

// +kubebuilder:rbac:groups=iam.miloapis.com,resources=users,verbs=get;list;watch;update
// +kubebuilder:rbac:groups=iam.miloapis.com,resources=users/status,verbs=update
// +kubebuilder:rbac:groups=iam.miloapis.com,resources=userdeactivations,verbs=get;list;watch

// Reconcile is the main reconciliation loop for the UserController.
func (r *UserController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx).WithName("user-controller")
	log.Info("Starting reconciliation", "request", req.Name)

	user := &iamv1alpha1.User{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: req.Name}, user); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to get User: %w", err)
	}
	log.Info("reconciling User", "user", user.Name)

	// Stop reconciling if deletion in progress.
	if !user.DeletionTimestamp.IsZero() {
		log.Info("User is being deleted, skipping reconciliation", "user", user.Name)
		return ctrl.Result{}, nil
	}

	// Determine desired state based on existence of any UserDeactivation for this user
	var udList iamv1alpha1.UserDeactivationList
	if err := r.Client.List(ctx, &udList, client.MatchingFields{"spec.userRef.name": user.Name}); err != nil {
		log.Error(err, "failed to list UserDeactivations")
		return ctrl.Result{}, fmt.Errorf("failed to list UserDeactivations: %w", err)
	}

	// Capture the current status to detect changes later
	oldUserStatus := user.Status.DeepCopy()

	// Defining the desired user state
	var desiredState iamv1alpha1.UserState
	// Only mark the user Inactive if there is at least one processed (Ready=True) UserDeactivation
	hasProcessedDeactivation := false
	for i := range udList.Items {
		ud := udList.Items[i]
		if meta.IsStatusConditionTrue(ud.Status.Conditions, iamv1alpha1.UserDeactivationReadyCondition) {
			hasProcessedDeactivation = true
			break
		}
	}
	if hasProcessedDeactivation {
		desiredState = iamv1alpha1.UserStateInactive
	} else {
		desiredState = iamv1alpha1.UserStateActive
	}
	user.Status.State = desiredState

	// Also set/refresh Ready condition to reflect change
	userCondition := metav1.Condition{
		Type:               userReadyConditionType,
		Status:             metav1.ConditionTrue,
		Reason:             "Reconciled",
		Message:            fmt.Sprintf("User state set to %s based on processed UserDeactivation presence", desiredState),
		LastTransitionTime: metav1.Now(),
	}
	meta.SetStatusCondition(&user.Status.Conditions, userCondition)
	// Update or set condition
	// Only update the status if it actually changed to avoid unnecessary API calls
	if !equality.Semantic.DeepEqual(oldUserStatus, &user.Status) {
		log.Info("Updating User status", "userName", user.GetName())
		if err := r.Client.Status().Update(ctx, user); err != nil {
			log.Error(err, "Failed to update User status")
			return ctrl.Result{}, fmt.Errorf("failed to update User status: %w", err)
		}
	} else {
		log.Info("User status unchanged, skipping update", "user", user.GetName())
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *UserController) SetupWithManager(mgr ctrl.Manager) error {
	// Index UserDeactivation by spec.userRef.name for efficient lookups
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &iamv1alpha1.UserDeactivation{}, "spec.userRef.name", func(obj client.Object) []string {
		ud, ok := obj.(*iamv1alpha1.UserDeactivation)
		if !ok {
			return nil
		}
		if ud.Spec.UserRef.Name == "" {
			// This should never happen, as the there is a webhook that validates the UserDeactivation
			return nil
		}
		return []string{ud.Spec.UserRef.Name}
	}); err != nil {
		return fmt.Errorf("failed to set field index on UserDeactivation: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&iamv1alpha1.User{}).
		Watches(&iamv1alpha1.UserDeactivation{}, handler.EnqueueRequestsFromMapFunc(r.findUserDeactivationsForUser)).
		Named("user").
		Complete(r)
}

// findUserDeactivationsForUser finds all UserDeactivation resources that reference a given User
func (r *UserController) findUserDeactivationsForUser(ctx context.Context, obj client.Object) []reconcile.Request {
	log := log.FromContext(ctx).WithName("find-user-deactivations-for-user")

	userDeactivation, ok := obj.(*iamv1alpha1.UserDeactivation)
	if !ok {
		log.Error(fmt.Errorf("unexpected object type %T, expected *iamv1alpha1.UserDeactivation", obj), "unexpected object type")
		return nil
	}
	if userDeactivation.Spec.UserRef.Name == "" {
		// This should never happen, as the there is a webhook that validates the UserDeactivation
		log.Error(fmt.Errorf("user deactivation has no user reference"), "user deactivation has no user reference")
		return nil
	}
	log.Info("found UserDeactivation for user", "user", userDeactivation.Spec.UserRef.Name, "userDeactivation", userDeactivation.Name)

	return []reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Name:      userDeactivation.Spec.UserRef.Name,
				Namespace: userDeactivation.Namespace,
			},
		},
	}
}
