package resourcemanager

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	iamv1alpha1 "go.miloapis.com/milo/pkg/apis/iam/v1alpha1"
	resourcemanagerv1alpha "go.miloapis.com/milo/pkg/apis/resourcemanager/v1alpha1"
)

const (
	OrganizationMembershipUserIndexName = "organizationmembership-user-index"
)

// OrganizationController reconciles an Organization object
type OrganizationController struct {
	Client    client.Client
	APIReader client.Reader
}

// +kubebuilder:rbac:groups=resourcemanager.miloapis.com,resources=organizations,verbs=get;list;watch;delete
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=authorization.k8s.io,resources=subjectaccessreviews,verbs=create
// +kubebuilder:rbac:groups=iam.miloapis.com,resources=users,verbs=get;list;watch
// +kubebuilder:rbac:groups=resourcemanager.miloapis.com,resources=organizationmemberships,verbs=get;list;watch

func (r *OrganizationController) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, err error) {
	logger := log.FromContext(ctx)

	var organization resourcemanagerv1alpha.Organization
	if err := r.Client.Get(ctx, req.NamespacedName, &organization); apierrors.IsNotFound(err) {
		return ctrl.Result{}, nil
	} else if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get organization: %w", err)
	}

	// Don't need to continue if the organization is being deleted from the cluster.
	if !organization.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	logger.Info("reconciling organization")
	defer logger.Info("reconcile complete")

	// Find the namespace for this organization
	namespaceName := fmt.Sprintf("organization-%s", organization.Name)
	var namespace corev1.Namespace
	if err := r.Client.Get(ctx, types.NamespacedName{Name: namespaceName}, &namespace); apierrors.IsNotFound(err) {
		// Namespace doesn't exist, nothing to do
		logger.Info("organization namespace not found", "namespace", namespaceName)
		return ctrl.Result{}, nil
	} else if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get organization namespace: %w", err)
	}

	// Check for last-member condition
	deleted, err := r.ensureOrganizationDeletedIfNoMembers(ctx, &organization)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to ensure organization deleted if no members: %w", err)
	} else if deleted {
		return ctrl.Result{}, nil
	}

	// Check if the organization is already set as the controller owner reference
	hasOwnerRef, err := controllerutil.HasOwnerReference(namespace.OwnerReferences, &organization, r.Client.Scheme())
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to check if organization is owner reference: %w", err)
	} else if hasOwnerRef {
		return ctrl.Result{}, nil
	}

	logger.Info("adding organization as owner reference to namespace", "namespace", namespaceName)

	// Set the organization as the controller owner reference for the namespace
	if err := controllerutil.SetControllerReference(&organization, &namespace, r.Client.Scheme()); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to set controller reference: %w", err)
	}

	// Update the namespace with the owner reference
	if err := r.Client.Update(ctx, &namespace); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update namespace owner references: %w", err)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *OrganizationController) SetupWithManager(mgr ctrl.Manager) error {
	r.Client = mgr.GetClient()
	r.APIReader = mgr.GetAPIReader()

	// Index OrganizationMemberships by spec.userRef.name for efficient lookups
	if err := mgr.GetFieldIndexer().IndexField(context.Background(),
		&resourcemanagerv1alpha.OrganizationMembership{},
		OrganizationMembershipUserIndexName,
		func(rawObj client.Object) []string {
			obj := rawObj.(*resourcemanagerv1alpha.OrganizationMembership)
			if obj.Spec.UserRef.Name == "" {
				return nil
			}
			return []string{obj.Spec.UserRef.Name}
		},
	); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&resourcemanagerv1alpha.Organization{}).
		Watches(&iamv1alpha1.User{},
			handler.EnqueueRequestsFromMapFunc(r.findOrganizationsForUser),
			// Only react on user deletions
			builder.WithPredicates(predicate.Funcs{
				CreateFunc:  func(e event.CreateEvent) bool { return false },
				UpdateFunc:  func(e event.UpdateEvent) bool { return false },
				GenericFunc: func(e event.GenericEvent) bool { return false },
				DeleteFunc:  func(e event.DeleteEvent) bool { return true },
			}),
		).
		Named("organization").
		Complete(r)
}

// findOrganizationsForUser maps a deleted User to the Organizations they were a member of.
func (r *OrganizationController) findOrganizationsForUser(ctx context.Context, obj client.Object) []reconcile.Request {
	user := obj.(*iamv1alpha1.User)
	logger := log.FromContext(ctx)
	logger.Info("user deleted, enqueuing organizations for reconcile", "user", user.Name)

	// List all Organization
	var memberships resourcemanagerv1alpha.OrganizationMembershipList
	if err := r.Client.List(ctx, &memberships, client.MatchingFields{OrganizationMembershipUserIndexName: user.Name}); err != nil {
		logger.Error(err, "failed to list organization memberships for deleted user", "user", user.Name)
		return nil
	}

	var requests []reconcile.Request
	for _, m := range memberships.Items {
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name: m.Spec.OrganizationRef.Name,
			},
		})
	}
	return requests
}

// ensureOrganizationDeletedIfNoMembers deletes the Organization if it no longer has any memberships.
func (r *OrganizationController) ensureOrganizationDeletedIfNoMembers(ctx context.Context, organization *resourcemanagerv1alpha.Organization) (bool, error) {
	// Determine the organization namespace and list memberships within it
	namespaceName := fmt.Sprintf("organization-%s", organization.Name)
	var memberships resourcemanagerv1alpha.OrganizationMembershipList
	if err := r.Client.List(ctx, &memberships, client.InNamespace(namespaceName)); err != nil {
		return false, fmt.Errorf("failed to list organization memberships in namespace %s: %w", namespaceName, err)
	}

	// Filter to memberships that reference this organization
	var filtered []resourcemanagerv1alpha.OrganizationMembership
	for _, m := range memberships.Items {
		if m.Spec.OrganizationRef.Name == organization.Name {
			filtered = append(filtered, m)
		}
	}

	// Len = 0: No memberships reference this organization (organization just created)
	// Len > 1: Multiple memberships reference this organization
	if len(filtered) == 0 || len(filtered) > 1 {
		return false, nil
	}

	// If there is exactly one membership left, check if the referenced user still exists.
	// If the user does not exist (was deleted) while the membership remains, delete the organization.
	// By webhook design, the last membership cannot be deleted.

	userName := filtered[0].Spec.UserRef.Name
	user := &iamv1alpha1.User{}
	// Use live API reader to avoid cache race; also treat terminating users as deleted
	if err := r.APIReader.Get(ctx, client.ObjectKey{Name: userName}, user); err != nil {
		if apierrors.IsNotFound(err) {
			if err := r.deleteOrganization(ctx, organization, "single remaining membership references deleted user; deleting organization"); err != nil {
				return false, err
			}
			return true, nil
		}
		return false, fmt.Errorf("failed to get user %s: %w", userName, err)
	}
	if !user.DeletionTimestamp.IsZero() {
		if err := r.deleteOrganization(ctx, organization, "single remaining membership references terminating user; deleting organization"); err != nil {
			return false, err
		}
		return true, nil
	}

	return false, nil
}

// deleteOrganization deletes the given Organization, logging a reason and ignoring NotFound errors.
func (r *OrganizationController) deleteOrganization(ctx context.Context, organization *resourcemanagerv1alpha.Organization, reason string) error {
	logger := log.FromContext(ctx)
	logger.Info(reason, "organization", organization.Name)
	if err := r.Client.Delete(ctx, organization); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete organization %s: %w", organization.Name, err)
	}
	return nil
}
