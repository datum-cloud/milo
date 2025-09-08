package iam

import (
	"context"
	"fmt"
	"strings"
	"time"

	iamv1alpha1 "go.miloapis.com/milo/pkg/apis/iam/v1alpha1"
	resourcemanagerv1alpha1 "go.miloapis.com/milo/pkg/apis/resourcemanager/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/finalizer"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	userInvitationFinalizerKey = "iam.miloapis.com/userinvitation"
)

var uiRelatedRoles []iamv1alpha1.RoleReference

type UserInvitationController struct {
	client                   client.Client
	finalizer                finalizer.Finalizers
	systemNamespace          string
	getInvitationRoleName    string
	acceptInvitationRoleName string
}

type userInvitationFinalizer struct {
	client client.Client
}

func (f *userInvitationFinalizer) Finalize(ctx context.Context, obj client.Object) (finalizer.Result, error) {
	log := logf.FromContext(ctx).WithName("userinvitation-finalizer")
	log.Info("Finalizing UserInvitation", "name", obj.GetName())

	ui, ok := obj.(*iamv1alpha1.UserInvitation)
	if !ok {
		return finalizer.Result{}, fmt.Errorf("unexpected object type %T, expected UserInvitation", obj)
	}

	// Delete the PolicyBindings invitation-related roles
	for _, roleRe := range uiRelatedRoles {
		if err := deletePolicyBinding(ctx, f.client, &iamv1alpha1.RoleReference{
			Name:      roleRe.Name,
			Namespace: roleRe.Namespace,
		}, *ui); err != nil {
			log.Error(err, "Failed to delete PolicyBinding for invitation-related role on UserInvitation finalization", "role", roleRe)
			return finalizer.Result{}, fmt.Errorf("failed to delete PolicyBinding for invitation-related role on UserInvitation finalization: %w", err)
		}
	}

	log.Info("Successfully finalized UserInvitation (cleaned up ui PolicyBindings)")

	return finalizer.Result{}, nil
}

func (r *UserInvitationController) SetupController(mgr ctrl.Manager, systemNamespace, getInvitationRoleName, acceptInvitationRoleName string) error {
	r.client = mgr.GetClient()
	r.systemNamespace = systemNamespace
	r.getInvitationRoleName = getInvitationRoleName
	r.acceptInvitationRoleName = acceptInvitationRoleName
	return nil
}

const (
	userEmailIndexKey = "spec.email"
)

// +kubebuilder:rbac:groups=iam.miloapis.com,resources=userinvitations,verbs=get;list;watch;update
// +kubebuilder:rbac:groups=iam.miloapis.com,resources=userinvitations/status,verbs=update
// +kubebuilder:rbac:groups=iam.miloapis.com,resources=users,verbs=get;list;watch
// +kubebuilder:rbac:groups=iam.miloapis.com,resources=policybindings,verbs=get;list;watch;create;delete
// +kubebuilder:rbac:groups=resourcemanager.miloapis.com,resources=organizationmemberships,verbs=get;list;watch;create
// +kubebuilder:rbac:groups=resourcemanager.miloapis.com,resources=organizations,verbs=get;list;watch

func (r *UserInvitationController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx).WithName("userinvitation-reconciler")
	log.Info("Starting reconciliation", "name", req.Name)

	// Get the UserInvitation
	ui := &iamv1alpha1.UserInvitation{}
	if err := r.client.Get(ctx, req.NamespacedName, ui); err != nil {
		if errors.IsNotFound(err) {
			log.Info("UserInvitation not found, probably deleted. Skipping reconciliation")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get UserInvitation")
		return ctrl.Result{}, fmt.Errorf("failed to get UserInvitation: %w", err)
	}

	log.Info("reconciling UserInvitation", "name", ui.Name, "email", ui.Spec.Email)

	// Check if the UserInvitation is ready
	if meta.IsStatusConditionTrue(ui.Status.Conditions, string(iamv1alpha1.UserInvitationReadyCondition)) {
		log.Info("UserInvitation is ready, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	// Check if the UserInvitation is not expired
	if meta.IsStatusConditionTrue(ui.Status.Conditions, string(iamv1alpha1.UserInvitationExpiredCondition)) {
		log.Info("UserInvitation is expired, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	// Check that the UserInvitation is not expired
	// Expiration is checked in the validationwebhook, but we check here in case some UserInvitation got
	// stuck in the controller loop for a long time, and we want to prevent giving roles to a user that is no longer valid.
	now := metav1.NewTime(time.Now().UTC())
	if ui.Spec.ExpirationDate != nil && ui.Spec.ExpirationDate.Before(&now) {
		if err := r.updateUserInvitationStatus(ctx, ui.DeepCopy(), metav1.Condition{
			Type:    string(iamv1alpha1.UserInvitationExpiredCondition),
			Status:  metav1.ConditionTrue,
			Reason:  string(iamv1alpha1.UserInvitationStateExpiredReason),
			Message: fmt.Sprintf("UserInvitation %s is expired", ui.GetName()),
		}); err != nil {
			log.Error(err, "Failed to update expired UserInvitation status")
			return ctrl.Result{}, fmt.Errorf("failed to update expired UserInvitation status: %w", err)
		}
		log.Info("ExpiredUserInvitation status updated", "name", ui.Name)
		return ctrl.Result{}, nil
	}

	// Get the User that was invited by the UserInvitation
	var users iamv1alpha1.UserList
	if err := r.client.List(ctx, &users, client.MatchingFields{userEmailIndexKey: strings.ToLower(ui.Spec.Email)}); err != nil {
		log.Error(err, "Failed to list Users by email")
		return ctrl.Result{}, fmt.Errorf("failed to list Users by email: %w", err)
	}
	if len(users.Items) == 0 {
		log.Info("User not found for invitation, skipping reconciliation. Reconciliation will be triggered again when the User is created.")
		return ctrl.Result{}, nil
	}
	user := &users.Items[0]

	// Grant roles to the invitee user for the organization if the invitation is accepted
	if ui.Spec.State == iamv1alpha1.UserInvitationStateAccepted {
		log.Info("Granting roles to the invitee user for the organization, as the invitation is accepted", "user", user.Name, "roles", ui.Spec.Roles)

		// Create the OrganizationMembership
		if err := r.createOrganizationMembership(ctx, user, ui); err != nil {
			log.Error(err, "Failed to create OrganizationMembership for userInvitation")
			return ctrl.Result{}, fmt.Errorf("failed to create OrganizationMembership for userInvitation: %w", err)
		}

		// Create the PolicyBindings
		for _, roleRef := range ui.Spec.Roles {
			err := r.createPolicyBinding(ctx, user, ui, &iamv1alpha1.RoleReference{
				Name:      roleRef.Name,
				Namespace: roleRef.Namespace,
			})
			if err != nil {
				log.Error(err, "Failed to create policy binding with %s role", roleRef.Name)
				return ctrl.Result{}, fmt.Errorf("failed to create policy binding with %s role: %w", roleRef.Name, err)
			}
		}

		// Update the UserInvitation status
		if err := r.updateUserInvitationStatus(ctx, ui.DeepCopy(), metav1.Condition{
			Type:    string(iamv1alpha1.UserInvitationReadyCondition),
			Status:  metav1.ConditionTrue,
			Reason:  string(iamv1alpha1.UserInvitationStateAcceptedReason),
			Message: fmt.Sprintf("User accepted the invitation %s", ui.GetName()),
		}); err != nil {
			return ctrl.Result{}, err
		}

		log.Info("UserInvitation reconciled. User accepted the invitation", "userInvitation", ui.GetName())
		return ctrl.Result{}, nil
	}

	if ui.Spec.State == iamv1alpha1.UserInvitationStateDeclined {
		// Delete the PolicyBindings for the invitation-related roles
		log.Info("Deleting PolicyBindings for accepting the invitation, as the invitation is declined", "userInvitation", ui.GetName())
		if err := deletePolicyBinding(ctx, r.client, &iamv1alpha1.RoleReference{
			Name:      r.acceptInvitationRoleName,
			Namespace: r.systemNamespace,
		}, *ui); err != nil {
			log.Error(err, "Failed to delete PolicyBinding for accepting the invitation")
			return ctrl.Result{}, err
		}

		// Update the UserInvitation status
		if err := r.updateUserInvitationStatus(ctx, ui.DeepCopy(), metav1.Condition{
			Type:    string(iamv1alpha1.UserInvitationReadyCondition),
			Status:  metav1.ConditionTrue,
			Reason:  string(iamv1alpha1.UserInvitationStateDeclinedReason),
			Message: fmt.Sprintf("User declined the invitation %s", ui.GetName()),
		}); err != nil {
			return ctrl.Result{}, err
		}

		log.Info("UserInvitation reconciled. User declined the invitation", "userInvitation", ui.GetName())
		return ctrl.Result{}, nil
	}

	// Check if the UserInvitation is pending
	if meta.IsStatusConditionTrue(ui.Status.Conditions, string(iamv1alpha1.UserInvitationPendingCondition)) {
		log.Info("UserInvitation is pending, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	// Grant permissions to the invitee user so they can accept the invitation
	for _, role := range uiRelatedRoles {
		err := r.createPolicyBinding(ctx, user, ui, &iamv1alpha1.RoleReference{
			Name:      role.Name,
			Namespace: role.Namespace,
		})
		if err != nil {
			log.Error(err, "Failed to create policy binding with %s role", role, "userInvitation", ui.GetName())
			return ctrl.Result{}, fmt.Errorf("failed to create policy binding with %s role: %w", role, err)
		}
	}

	// Send an email to the invitee user to accept the invitation
	if err := r.sendInvitationEmail(ctx, user, ui); err != nil {
		log.Error(err, "Failed to send invitation email to user", "userInvitation", ui.GetName())
		return ctrl.Result{}, fmt.Errorf("failed to send invitation email to user: %w", err)
	}

	// Update the UserInvitation status
	if err := r.updateUserInvitationStatus(ctx, ui.DeepCopy(), metav1.Condition{
		Type:    string(iamv1alpha1.UserInvitationPendingCondition),
		Status:  metav1.ConditionTrue,
		Reason:  string(iamv1alpha1.UserInvitationStatePendingReason),
		Message: fmt.Sprintf("User invitation is pending, ui: %s", ui.GetName()),
	}); err != nil {
		return ctrl.Result{}, err
	}

	log.Info("UserInvitation reconciled", "userInvitation", ui.GetName())

	return ctrl.Result{}, nil
}

func (r *UserInvitationController) SetupWithManager(mgr ctrl.Manager) error {
	uiRelatedRoles = append(uiRelatedRoles, iamv1alpha1.RoleReference{
		Name:      r.getInvitationRoleName,
		Namespace: r.systemNamespace,
	}, iamv1alpha1.RoleReference{
		Name:      r.acceptInvitationRoleName,
		Namespace: r.systemNamespace,
	})

	r.finalizer = finalizer.NewFinalizers()
	if err := r.finalizer.Register(userInvitationFinalizerKey, &userInvitationFinalizer{
		client: r.client,
	}); err != nil {
		return fmt.Errorf("failed to register user invitation finalizer: %w", err)
	}

	// Register field indexer for User email for efficient lookup
	if err := mgr.GetFieldIndexer().IndexField(context.Background(),
		&iamv1alpha1.User{}, userEmailIndexKey,
		func(obj client.Object) []string {
			user := obj.(*iamv1alpha1.User)
			return []string{strings.ToLower(user.Spec.Email)}
		}); err != nil {
		return fmt.Errorf("failed to set field index on User by .spec.email: %w", err)
	}

	// Register field indexer for UserInvitation email for efficient lookup
	if err := mgr.GetFieldIndexer().IndexField(context.Background(),
		&iamv1alpha1.UserInvitation{}, userEmailIndexKey,
		func(obj client.Object) []string {
			ui := obj.(*iamv1alpha1.UserInvitation)
			return []string{strings.ToLower(ui.Spec.Email)}
		}); err != nil {
		return fmt.Errorf("failed to set field index on UserInvitation by .spec.email: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&iamv1alpha1.UserInvitation{}).
		Watches(
			&iamv1alpha1.User{},
			handler.EnqueueRequestsFromMapFunc(r.findUserInvitationsForUser),
			builder.WithPredicates(userCreateOnlyPredicate),
		).
		Named("userinvitation").
		Complete(r)
}

// updateUserInvitationStatus updates the status of the UserInvitation
func (r *UserInvitationController) updateUserInvitationStatus(ctx context.Context, ui *iamv1alpha1.UserInvitation, condition metav1.Condition) error {
	log := logf.FromContext(ctx).WithName("userinvitation-update-status")
	log.Info("Updating UserInvitation status", "status", ui.Status)

	meta.SetStatusCondition(&ui.Status.Conditions, condition)

	if err := r.client.Status().Update(ctx, ui); err != nil {
		log.Error(err, "failed to update UserInvitation status", "userInvitation", ui.Name)
		return fmt.Errorf("failed to update UserInvitation status: %w", err)
	}
	log.Info("UserInvitation status updated")

	return nil
}

// createPolicyBinding creates a PolicyBinding for the invitee user to the organization.
// This is an idempotent operation.
func (r *UserInvitationController) createPolicyBinding(
	ctx context.Context,
	user *iamv1alpha1.User,
	invitation *iamv1alpha1.UserInvitation,
	roleRef *iamv1alpha1.RoleReference) error {

	log := logf.FromContext(ctx).WithName("userinvitation-create-invitee-policy-binding")
	log.Info("Attempting to create PolicyBinding for invitee user", "user", user.Name)

	// Check if the PolicyBinding already exists
	policyBinding := &iamv1alpha1.PolicyBinding{}
	if err := r.client.Get(ctx, client.ObjectKey{Name: getDeterministicResourceName(roleRef.Name, *invitation), Namespace: roleRef.Namespace}, policyBinding); err != nil {
		if errors.IsNotFound(err) {
			log.Info("PolicyBinding not found, creating")
		} else {
			return fmt.Errorf("failed to get PolicyBinding: %w", err)
		}
	} else {
		log.Info("PolicyBinding found, skipping creation")
		return nil
	}

	// Generate the ResourceRef
	resourceRef, err := r.getResourceRef(ctx, roleRef, *invitation)
	if err != nil {
		return fmt.Errorf("failed to generate ResourceRef: %w", err)
	}

	// Build the PolicyBinding
	policyBinding = &iamv1alpha1.PolicyBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getDeterministicResourceName(roleRef.Name, *invitation),
			Namespace: roleRef.Namespace,
		},
		Spec: iamv1alpha1.PolicyBindingSpec{
			RoleRef: iamv1alpha1.RoleReference{
				Name:      roleRef.Name,
				Namespace: roleRef.Namespace,
			},
			Subjects: []iamv1alpha1.Subject{
				{
					Kind: "User",
					Name: user.Name,
					UID:  string(user.GetUID()),
				},
			},
			ResourceSelector: iamv1alpha1.ResourceSelector{
				ResourceRef: &resourceRef,
			},
		},
	}

	// Create the PolicyBinding
	if err := r.client.Create(ctx, policyBinding); err != nil {
		return fmt.Errorf("failed to create policy binding resource: %w", err)
	}

	log.Info("PolicyBinding created", "name", policyBinding.GetName())

	return nil
}

// getDeterministicResourceName generates a deterministic name for a resource to create based on the UserInvitation.
// This can be used in order to get/create the PolicyBinding, or other resources.
func getDeterministicResourceName(name string, ui iamv1alpha1.UserInvitation) string {
	return fmt.Sprintf("%s-%s", string(ui.GetUID()), name)
}

// getResourceRef generates a ResourceRef for the PolicyBinding. As the ResourceRef depends on the roleRef
func (r *UserInvitationController) getResourceRef(ctx context.Context, roleRef *iamv1alpha1.RoleReference, ui iamv1alpha1.UserInvitation) (iamv1alpha1.ResourceReference, error) {
	log := logf.FromContext(ctx).WithName("userinvitation-generate-resource-ref")
	log.Info("Generating ResourceRef for UserInvitation", "roleRef", roleRef, "uiName", ui.GetName())

	for _, role := range uiRelatedRoles {
		if role.Name == roleRef.Name && role.Namespace == roleRef.Namespace {
			// If the roleRef contains the invitation-related roles, then the resourceRef is the UserInvitation
			return iamv1alpha1.ResourceReference{
				APIGroup:  iamv1alpha1.SchemeGroupVersion.Group,
				Kind:      "UserInvitation",
				Name:      ui.GetName(),
				UID:       string(ui.GetUID()),
				Namespace: ui.GetNamespace(),
			}, nil
		}
	}

	// If the roleRef is the organization role, then the resourceRef is the Organization

	// Get the Organization
	org := &resourcemanagerv1alpha1.Organization{}
	if err := r.client.Get(ctx, client.ObjectKey{Name: ui.Spec.OrganizationRef.Name}, org); err != nil {
		return iamv1alpha1.ResourceReference{}, fmt.Errorf("failed to get Organization: %w", err)
	}

	return iamv1alpha1.ResourceReference{
		APIGroup: resourcemanagerv1alpha1.GroupVersion.Group,
		Kind:     "Organization",
		Name:     org.GetName(),
		UID:      string(org.GetUID()),
	}, nil
}

// deletePolicyBinding deletes a PolicyBinding for the invitee user to the organization.
// This is an idempotent operation.
func deletePolicyBinding(ctx context.Context, c client.Client, roleRef *iamv1alpha1.RoleReference, ui iamv1alpha1.UserInvitation) error {
	log := logf.FromContext(ctx).WithName("userinvitation-delete-policy-binding")
	log.Info("Deleting PolicyBinding for UserInvitation", "roleRef", roleRef, "uiName", ui.GetName())

	// Check if the PolicyBinding exists
	policyBinding := &iamv1alpha1.PolicyBinding{}
	if err := c.Get(ctx, client.ObjectKey{Name: getDeterministicResourceName(roleRef.Name, ui), Namespace: roleRef.Namespace}, policyBinding); err != nil {
		if errors.IsNotFound(err) {
			log.Info("PolicyBinding not found, skipping deletion")
			return nil
		}
		log.Error(err, "Failed to get PolicyBinding")
		return fmt.Errorf("failed to get PolicyBinding: %w", err)
	}

	// Delete the PolicyBinding
	if err := c.Delete(ctx, &iamv1alpha1.PolicyBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getDeterministicResourceName(roleRef.Name, ui),
			Namespace: roleRef.Namespace,
		},
	}); err != nil {
		return fmt.Errorf("failed to delete policy binding resource: %w", err)
	}

	log.Info("PolicyBinding deleted", "name", getDeterministicResourceName(roleRef.Name, ui))

	return nil
}

// sendInvitationEmail sends an email to the invitee user to accept the invitation.
// This is an idempotent operation.
func (r *UserInvitationController) sendInvitationEmail(ctx context.Context, user *iamv1alpha1.User, ui *iamv1alpha1.UserInvitation) error {
	log := logf.FromContext(ctx).WithName("userinvitation-send-invitation-email")
	log.Info("Sending invitation email to user", "userInvitation", ui.GetName(), "user", user.GetName())

	emailName := fmt.Sprintf("user-invitation-%s", ui.GetName())

	log.Info("Email name", "emailName", emailName)

	// TODO: Get Email

	// TODO: Send email if not already sent

	return nil
}

// createOrganizationMembership creates an OrganizationMembership for the invitee user. This is an idempotent operation.
func (r *UserInvitationController) createOrganizationMembership(ctx context.Context, user *iamv1alpha1.User, ui *iamv1alpha1.UserInvitation) error {
	log := logf.FromContext(ctx).WithName("userinvitation-create-organization-membership")
	log.Info("Creating OrganizationMembership for userInvitation", "userInvitation", ui.GetName(), "user", user.GetName())

	// Check if the OrganizationMembership already exists
	organizationMembership := &resourcemanagerv1alpha1.OrganizationMembership{}
	if err := r.client.Get(ctx, client.ObjectKey{Name: fmt.Sprintf("member-%s", user.Name), Namespace: fmt.Sprintf("organization-%s", ui.Spec.OrganizationRef.Name)}, organizationMembership); err != nil {
		if errors.IsNotFound(err) {
			log.Info("OrganizationMembership not found, creating")
		} else {
			return fmt.Errorf("failed to get OrganizationMembership: %w", err)
		}
	} else {
		log.Info("OrganizationMembership found, skipping creation")
		return nil
	}

	// Build the OrganizationMembership
	organizationMembership = &resourcemanagerv1alpha1.OrganizationMembership{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("member-%s", user.Name),
			Namespace: fmt.Sprintf("organization-%s", ui.Spec.OrganizationRef.Name),
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: iamv1alpha1.SchemeGroupVersion.Group,
					Kind:       "User",
					Name:       user.GetName(),
					UID:        user.GetUID(),
				},
			},
		},
		Spec: resourcemanagerv1alpha1.OrganizationMembershipSpec{
			OrganizationRef: resourcemanagerv1alpha1.OrganizationReference{
				Name: ui.Spec.OrganizationRef.Name,
			},
			UserRef: resourcemanagerv1alpha1.MemberReference{
				Name: user.Name,
			},
		},
	}

	// Create the OrganizationMembership
	if err := r.client.Create(ctx, organizationMembership); err != nil {
		return fmt.Errorf("failed to create organization membership resource: %w", err)
	}

	log.Info("OrganizationMembership created", "name", organizationMembership.GetName())

	return nil
}

// findUserInvitationsForUser finds all UserInvitation resources that reference a given User email.
// This is used to reconcile the UserInvitation resources when a User is created, in the case that the User was invited by a UserInvitation
// and the User was created after the UserInvitation was created.
func (r *UserInvitationController) findUserInvitationsForUser(ctx context.Context, obj client.Object) []reconcile.Request {
	log := logf.FromContext(ctx).WithName("find-userinvitations-for-user")

	user, ok := obj.(*iamv1alpha1.User)
	if !ok {
		log.Error(fmt.Errorf("unexpected object type %T, expected *iamv1alpha1.User", obj), "unexpected object type")
		return nil
	}

	if user.Spec.Email == "" {
		log.Error(fmt.Errorf("user has no email"), "user has no email")
		return nil
	}

	// List UserInvitations matching this user's email (case-insensitive)
	var uiList iamv1alpha1.UserInvitationList
	if err := r.client.List(ctx, &uiList, client.MatchingFields{userEmailIndexKey: strings.ToLower(user.Spec.Email)}); err != nil {
		log.Error(err, "failed to list UserInvitations by email")
		return nil
	}

	requests := make([]reconcile.Request, 0, len(uiList.Items))
	for i := range uiList.Items {
		ui := uiList.Items[i]
		requests = append(requests, reconcile.Request{NamespacedName: client.ObjectKey{Name: ui.GetName(), Namespace: ui.GetNamespace()}})
	}

	log.Info("Found UserInvitations for user", "Number of UserInvitations", len(requests), "user", user.GetName())

	return requests
}

// userCreateOnlyPredicate triggers only on User create events.
var userCreateOnlyPredicate = predicate.Funcs{
	CreateFunc:  func(e event.CreateEvent) bool { return true },
	UpdateFunc:  func(e event.UpdateEvent) bool { return false },
	DeleteFunc:  func(e event.DeleteEvent) bool { return false },
	GenericFunc: func(e event.GenericEvent) bool { return false },
}
