package iam

import (
	"context"
	"fmt"
	"strings"
	"time"

	iamv1alpha1 "go.miloapis.com/milo/pkg/apis/iam/v1alpha1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type PlatformInvitationController struct {
	Client client.Client
}

const (
	PlatformInvitationScheludedCondition = "PlatformInvitationScheduledCondition"
	PlatformInvitationScheduledReason    = "Reconciled"
)

const platformInvitationUserEmailIndexKey = "iam.miloapis.com/useremailkey"

// +kubebuilder:rbac:groups=iam.miloapis.com,resources=platforminvitations,verbs=get;list;watch;update
// +kubebuilder:rbac:groups=iam.miloapis.com,resources=platforminvitations/status,verbs=update
// +kubebuilder:rbac:groups=iam.miloapis.com,resources=users,verbs=get;list;watch
// +kubebuilder:rbac:groups=iam.miloapis.com,resources=platformaccessapprovals,verbs=get;list;watch;create
func (r *PlatformInvitationController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx).WithValues("controller", "PlatformInvitationController", "trigger", req.NamespacedName)
	log.Info("Starting reconciliation", "name", req.Name)

	// Get the PlatformInvitation
	pi := &iamv1alpha1.PlatformInvitation{}
	if err := r.Client.Get(ctx, req.NamespacedName, pi); err != nil {
		if errors.IsNotFound(err) {
			log.Info("PlatformInvitation not found, probably deleted. Skipping reconciliation")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get PlatformInvitation")
		return ctrl.Result{}, fmt.Errorf("failed to get PlatformInvitation: %w", err)
	}

	oldStatus := pi.Status.DeepCopy()

	// Check if the PlatformInvitation is ready
	if meta.IsStatusConditionTrue(pi.Status.Conditions, iamv1alpha1.PlatformInvitationReadyCondition) {
		log.Info("PlatformInvitation is ready, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	// Check if the PlatformInvitation should be requeued based on the schedule at
	if pi.Spec.ScheduleAt != nil && pi.Spec.ScheduleAt.After(time.Now()) {
		log.Info("PlatformInvitation should be requeued based on the schedule at", "scheduleAt", pi.Spec.ScheduleAt)
		if err := r.updatePlatformInvitationStatus(ctx, pi, metav1.Condition{
			Type:               PlatformInvitationScheludedCondition,
			Status:             metav1.ConditionTrue,
			Reason:             PlatformInvitationScheduledReason,
			Message:            fmt.Sprintf("PlatformInvitation %s is scheduled, will be sent at %s", pi.Name, pi.Spec.ScheduleAt.Time.Format(time.RFC3339)),
			LastTransitionTime: metav1.NewTime(time.Now()),
		}, oldStatus); err != nil {
			log.Error(err, "Failed to update PlatformInvitation status")
			return ctrl.Result{}, fmt.Errorf("failed to update PlatformInvitation status: %w", err)
		}
		return ctrl.Result{RequeueAfter: time.Until(pi.Spec.ScheduleAt.Time)}, nil
	}

	// Verify that the user does not exists, as the user has may been created before the PlatformInvitation was sent.
	// Likely to happen for scheduled PlatformInvitations.
	userExists := false
	users := &iamv1alpha1.UserList{}
	if err := r.Client.List(ctx, users, client.MatchingFields{platformInvitationUserEmailIndexKey: strings.ToLower(pi.Spec.Email)}); err != nil {
		log.Error(err, "Failed to list Users by email")
		return ctrl.Result{}, fmt.Errorf("failed to list Users by email: %w", err)
	}
	if len(users.Items) > 0 {
		log.Info("User already exists, skipping sending email and creating PlatformAccessApproval")
		userExists = true
	}

	conditionMessage := "PlatformInvitation is ready. Email and PlatformAccessApproval not created as user already exists. "
	if !userExists {
		// If here, the PlatformInvitation should be sent and the PlatformAccessApproval should be created
		conditionMessage = "PlatformInvitation is ready. Email sent and PlatformAccessApproval created."

		// Create the PlatformAccessApproval
		if err := r.createPlatformAccessApproval(ctx, pi); err != nil {
			log.Error(err, "Failed to create PlatformAccessApproval")
			return ctrl.Result{}, fmt.Errorf("failed to create PlatformAccessApproval: %w", err)
		}
	}

	// PlatformInvitation is ready, update the status
	if err := r.updatePlatformInvitationStatus(ctx, pi, metav1.Condition{
		Type:               iamv1alpha1.PlatformInvitationReadyCondition,
		Status:             metav1.ConditionTrue,
		Reason:             iamv1alpha1.PlatformInvitationReconciledReason,
		Message:            conditionMessage,
		LastTransitionTime: metav1.NewTime(time.Now()),
	}, oldStatus); err != nil {
		log.Error(err, "Failed to update PlatformInvitation status")
		return ctrl.Result{}, fmt.Errorf("failed to update PlatformInvitation status: %w", err)
	}

	// Check if the PlatformInvitation is ready
	return reconcile.Result{}, nil
}

func (r *PlatformInvitationController) SetupWithManager(mgr ctrl.Manager) error {
	log := logf.FromContext(context.Background()).WithName("platforminvitation-setup-with-manager")
	log.Info("Setting up PlatformInvitationController with Manager")

	// Register field indexer for User email for efficient lookup
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &iamv1alpha1.User{}, platformInvitationUserEmailIndexKey, func(obj client.Object) []string {
		user := obj.(*iamv1alpha1.User)
		return []string{strings.ToLower(user.Spec.Email)}
	}); err != nil {
		log.Error(err, "Failed to set field index on User by .spec.email")
		return fmt.Errorf("failed to set field index on User by .spec.email: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&iamv1alpha1.PlatformInvitation{}).
		Named("platforminvitation").
		Complete(r)
}

func (r *PlatformInvitationController) updatePlatformInvitationStatus(ctx context.Context, pi *iamv1alpha1.PlatformInvitation, condition metav1.Condition, oldStatus *iamv1alpha1.PlatformInvitationStatus) error {
	log := logf.FromContext(ctx).WithName("platforminvitation-update-status")
	log.Info("Updating PlatformInvitation status", "status", pi.Status)

	meta.SetStatusCondition(&pi.Status.Conditions, condition)

	if !equality.Semantic.DeepEqual(oldStatus, &pi.Status) {
		log.Info("Updating PlatformInvitation status", "name", pi.GetName())
		if err := r.Client.Status().Update(ctx, pi); err != nil {
			log.Error(err, "Failed to update PlatformInvitation status")
			return fmt.Errorf("failed to update PlatformInvitation status: %w", err)
		}
	} else {
		log.Info("PlatformInvitation status unchanged, skipping update", "platformInvitation", pi.GetName())
	}

	return nil
}

// createPlatformAccessApproval creates a PlatformAccessApproval for the PlatformInvitation
// This is an idempotent operation, so it will not create a new PlatformAccessApproval if one already exists
func (r *PlatformInvitationController) createPlatformAccessApproval(ctx context.Context, pi *iamv1alpha1.PlatformInvitation) error {
	log := logf.FromContext(ctx).WithName("platforminvitation-create-platformaccessapproval")
	log.Info("Creating PlatformAccessApproval", "name", pi.Name, "email", pi.Spec.Email)

	deterministicName := getDeterministicPlatformAccessApprovalName(*pi)

	// Check if the PlatformAccessApproval already exists
	existingPaa := &iamv1alpha1.PlatformAccessApproval{}
	if err := r.Client.Get(ctx, client.ObjectKey{Name: deterministicName}, existingPaa); err != nil {
		if errors.IsNotFound(err) {
			log.Info("PlatformAccessApproval not found, creating new one")
		} else {
			log.Error(err, "Failed to get PlatformAccessApproval")
			return err
		}
	} else {
		log.Info("PlatformAccessApproval already exists, skipping creation")
		return nil
	}

	// Create the PlatformAccessApproval
	paa := &iamv1alpha1.PlatformAccessApproval{
		ObjectMeta: metav1.ObjectMeta{
			Name: deterministicName,
		},
		Spec: iamv1alpha1.PlatformAccessApprovalSpec{
			SubjectRef: iamv1alpha1.SubjectReference{Email: pi.Spec.Email},
		},
	}
	if err := r.Client.Create(ctx, paa); err != nil {
		log.Error(err, "Failed to create PlatformAccessApproval")
		return err
	}

	log.Info("PlatformAccessApproval created", "name", paa.Name)

	return nil
}

// getDeterministicPlatformAccessApprovalName generates a deterministic name for a PlatformAccessApproval to create based on the PlatformInvitation.
func getDeterministicPlatformAccessApprovalName(pi iamv1alpha1.PlatformInvitation) string {
	return fmt.Sprintf("%s-%s", string(pi.GetUID()), strings.ToLower(pi.Spec.Email))
}
