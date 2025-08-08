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
	"sigs.k8s.io/controller-runtime/pkg/finalizer"
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
	Client     client.Client
	Finalizers finalizer.Finalizers
}

// userFinalizer implements the finalizer.Finalizer interface for User cleanup.
// This is used to clean up associated UserDeactivations when a User is deleted.
type userFinalizer struct {
	Client client.Client
}

// Finalize implements the finalizer.Finalizer interface.
func (f *userFinalizer) Finalize(ctx context.Context, obj client.Object) (finalizer.Result, error) {
	log := log.FromContext(ctx).WithName("user-finalizer")

	user, ok := obj.(*iamv1alpha1.User)
	if !ok {
		log.Error(fmt.Errorf("unexpected object type %T, expected *iamv1alpha1.User", obj), "unexpected object type")
		return finalizer.Result{}, fmt.Errorf("unexpected object type %T, expected *iamv1alpha1.User", obj)
	}
	log.Info("finalizing User", "user", user.Name)

	// Best-effort delete of the associated UserDeactivation, if present.
	var udList iamv1alpha1.UserDeactivationList
	if err := f.Client.List(ctx, &udList, client.MatchingFields{"spec.userRef.name": user.Name}); err != nil {
		log.Error(err, "failed to list UserDeactivations in finalizer")
		return finalizer.Result{}, fmt.Errorf("failed to list UserDeactivations in finalizer: %w", err)
	}

	for i := range udList.Items {
		ud := &udList.Items[i]
		if err := f.Client.Delete(ctx, ud); err != nil && !apierrors.IsNotFound(err) {
			log.Error(err, "failed to delete UserDeactivation during finalization", "userDeactivation", ud.Name)
			return finalizer.Result{}, err
		}
		log.Info("deleted UserDeactivation during finalization", "userDeactivation", ud.Name, "user", user.Name)
	}

	log.Info("finalized User", "user", user.Name)
	return finalizer.Result{}, nil
}

// +kubebuilder:rbac:groups=iam.miloapis.com,resources=users,verbs=get;list;watch;update
// +kubebuilder:rbac:groups=iam.miloapis.com,resources=users/status,verbs=update
// +kubebuilder:rbac:groups=iam.miloapis.com,resources=userdeactivations,verbs=get;list;watch;delete

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

	// Handle finalizer lifecycle and ensure it is set.
	finalizeResult, err := r.Finalizers.Finalize(ctx, user)
	if err != nil {
		log.Error(err, "failed to run finalizers for User")
		return ctrl.Result{}, fmt.Errorf("failed to run finalizers for User: %w", err)
	}
	if finalizeResult.Updated {
		if err := r.Client.Update(ctx, user); err != nil {
			log.Error(err, "failed to update User after finalizer update")
			return ctrl.Result{}, fmt.Errorf("failed to update User after finalizer update: %w", err)
		}
		return ctrl.Result{}, nil
	}

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
	// If there are any UserDeactivations for this user, the user is inactive
	if len(udList.Items) > 0 {
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
		Message:            fmt.Sprintf("User state set to %s based on UserDeactivation presence", desiredState),
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
	r.Finalizers = finalizer.NewFinalizers()
	// Register finalizer implementation
	if err := r.Finalizers.Register(userFinalizerKey, &userFinalizer{Client: mgr.GetClient()}); err != nil {
		return fmt.Errorf("failed to register user finalizer: %w", err)
	}

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
