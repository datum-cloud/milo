package resourcemanager

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	iamv1alpha1 "go.miloapis.com/milo/pkg/apis/iam/v1alpha1"
	resourcemanagerv1alpha "go.miloapis.com/milo/pkg/apis/resourcemanager/v1alpha1"
)

const (
	// OrganizationMembershipReady indicates that the organization membership status has been populated
	OrganizationMembershipReady = "Ready"
)

const (
	// OrganizationMembershipReadyReason indicates that the organization membership is ready
	OrganizationMembershipReadyReason = "Ready"
	// OrganizationNotFoundReason indicates that the referenced organization was not found
	OrganizationNotFoundReason = "OrganizationNotFound"
	// UserNotFoundReason indicates that the referenced user was not found
	UserNotFoundReason = "UserNotFound"
	// ReconcileErrorReason indicates an error occurred during reconciliation
	ReconcileErrorReason = "ReconcileError"
)

// OrganizationMembershipController reconciles an OrganizationMembership object
type OrganizationMembershipController struct {
	Client client.Client
}

// +kubebuilder:rbac:groups=resourcemanager.miloapis.com,resources=organizationmemberships,verbs=get;list;watch;create
// +kubebuilder:rbac:groups=resourcemanager.miloapis.com,resources=organizationmemberships/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=resourcemanager.miloapis.com,resources=organizations,verbs=get;list;watch
// +kubebuilder:rbac:groups=iam.miloapis.com,resources=users,verbs=get;list;watch

func (r *OrganizationMembershipController) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, err error) {
	logger := log.FromContext(ctx)

	var organizationMembership resourcemanagerv1alpha.OrganizationMembership
	if err := r.Client.Get(ctx, req.NamespacedName, &organizationMembership); apierrors.IsNotFound(err) {
		return ctrl.Result{}, nil
	} else if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get organization membership: %w", err)
	}

	logger.Info("reconciling organization membership",
		"organization", organizationMembership.Spec.OrganizationRef.Name,
		"user", organizationMembership.Spec.UserRef.Name)

	// Get the current ready condition or create a new one
	readyCondition := apimeta.FindStatusCondition(organizationMembership.Status.Conditions, OrganizationMembershipReady)
	if readyCondition == nil {
		readyCondition = &metav1.Condition{
			Type:               OrganizationMembershipReady,
			Status:             metav1.ConditionFalse,
			Reason:             "Unknown",
			ObservedGeneration: organizationMembership.Generation,
		}
	} else {
		readyCondition = readyCondition.DeepCopy()
		readyCondition.ObservedGeneration = organizationMembership.Generation
	}

	// Fetch the referenced Organization
	var organization resourcemanagerv1alpha.Organization
	organizationKey := types.NamespacedName{
		Name: organizationMembership.Spec.OrganizationRef.Name,
	}

	if err := r.Client.Get(ctx, organizationKey, &organization); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("referenced organization not found", "organization", organizationMembership.Spec.OrganizationRef.Name)
			readyCondition.Status = metav1.ConditionFalse
			readyCondition.Reason = OrganizationNotFoundReason
			readyCondition.Message = fmt.Sprintf("Organization '%s' does not exist. Please ensure the organization name is correct and the organization has been created.", organizationMembership.Spec.OrganizationRef.Name)
		} else {
			logger.Error(err, "failed to get organization")
			readyCondition.Status = metav1.ConditionFalse
			readyCondition.Reason = ReconcileErrorReason
			readyCondition.Message = "Unable to retrieve organization information. Please try again later or contact support if the problem persists."
		}

		if apimeta.SetStatusCondition(&organizationMembership.Status.Conditions, *readyCondition) {
			if err := r.Client.Status().Update(ctx, &organizationMembership); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to update organization membership status: %w", err)
			}
		}
		return ctrl.Result{}, nil
	}

	// Fetch the referenced User
	var user iamv1alpha1.User
	userKey := types.NamespacedName{
		Name: organizationMembership.Spec.UserRef.Name,
	}

	if err := r.Client.Get(ctx, userKey, &user); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("referenced user not found", "user", organizationMembership.Spec.UserRef.Name)
			readyCondition.Status = metav1.ConditionFalse
			readyCondition.Reason = UserNotFoundReason
			readyCondition.Message = fmt.Sprintf("User '%s' does not exist. Please ensure the user name is correct and the user account has been created.", organizationMembership.Spec.UserRef.Name)
		} else {
			logger.Error(err, "failed to get user")
			readyCondition.Status = metav1.ConditionFalse
			readyCondition.Reason = ReconcileErrorReason
			readyCondition.Message = "Unable to retrieve user information. Please try again later or contact the support team if the problem persists."
		}

		if apimeta.SetStatusCondition(&organizationMembership.Status.Conditions, *readyCondition) {
			if err := r.Client.Status().Update(ctx, &organizationMembership); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to update organization membership status: %w", err)
			}
		}
		return ctrl.Result{}, nil
	}

	// Update the status with information from the Organization and User
	originalStatus := organizationMembership.Status.DeepCopy()

	// Update observed generation
	organizationMembership.Status.ObservedGeneration = organizationMembership.Generation

	// Update organization status information
	organizationMembership.Status.Organization = resourcemanagerv1alpha.OrganizationMembershipOrganizationStatus{
		Type:        organization.Spec.Type,
		DisplayName: organization.Annotations["kubernetes.io/display-name"],
	}

	// Update user status information
	organizationMembership.Status.User = resourcemanagerv1alpha.OrganizationMembershipUserStatus{
		Email:      user.Spec.Email,
		GivenName:  user.Spec.GivenName,
		FamilyName: user.Spec.FamilyName,
	}

	// Set ready condition to true
	readyCondition.Status = metav1.ConditionTrue
	readyCondition.Reason = OrganizationMembershipReadyReason
	readyCondition.Message = "Organization membership status has been populated"

	apimeta.SetStatusCondition(&organizationMembership.Status.Conditions, *readyCondition)

	// Update the status only if something changed
	if !equality.Semantic.DeepEqual(originalStatus, organizationMembership.Status) {
		if err := r.Client.Status().Update(ctx, &organizationMembership); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update organization membership status: %w", err)
		}
		logger.Info("organization membership status updated")
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *OrganizationMembershipController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&resourcemanagerv1alpha.OrganizationMembership{}).
		Watches(&resourcemanagerv1alpha.Organization{},
			handler.EnqueueRequestsFromMapFunc(r.findOrganizationMembershipsForOrganization)).
		Watches(&iamv1alpha1.User{},
			handler.EnqueueRequestsFromMapFunc(r.findOrganizationMembershipsForUser)).
		Named("organization-membership").
		Complete(r)
}

// findOrganizationMembershipsForOrganization finds all OrganizationMembership resources that reference a given Organization
func (r *OrganizationMembershipController) findOrganizationMembershipsForOrganization(ctx context.Context, obj client.Object) []reconcile.Request {
	organization := obj.(*resourcemanagerv1alpha.Organization)

	var organizationMemberships resourcemanagerv1alpha.OrganizationMembershipList
	if err := r.Client.List(ctx, &organizationMemberships); err != nil {
		return nil
	}

	var requests []reconcile.Request
	for _, membership := range organizationMemberships.Items {
		if membership.Spec.OrganizationRef.Name == organization.Name {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      membership.Name,
					Namespace: membership.Namespace,
				},
			})
		}
	}

	return requests
}

// findOrganizationMembershipsForUser finds all OrganizationMembership resources that reference a given User
func (r *OrganizationMembershipController) findOrganizationMembershipsForUser(ctx context.Context, obj client.Object) []reconcile.Request {
	user := obj.(*iamv1alpha1.User)

	var organizationMemberships resourcemanagerv1alpha.OrganizationMembershipList
	if err := r.Client.List(ctx, &organizationMemberships); err != nil {
		return nil
	}

	var requests []reconcile.Request
	for _, membership := range organizationMemberships.Items {
		if membership.Spec.UserRef.Name == user.Name {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      membership.Name,
					Namespace: membership.Namespace,
				},
			})
		}
	}

	return requests
}
