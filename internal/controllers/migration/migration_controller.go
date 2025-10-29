package migration

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	iamv1alpha1 "go.miloapis.com/milo/pkg/apis/iam/v1alpha1"
	resourcemanagerv1alpha1 "go.miloapis.com/milo/pkg/apis/resourcemanager/v1alpha1"
)

const (
	ManagedByLabel = "resourcemanager.miloapis.com/managed-by"
	ManagedByValue = "organization-membership-controller"
	RolesApplied   = "RolesApplied"
)

// MigrationController backfills OrganizationMembership resources with roles
// from legacy PolicyBindings and cleans up after migration completes.
type MigrationController struct {
	Client client.Client
}

// +kubebuilder:rbac:groups=resourcemanager.miloapis.com,resources=organizationmemberships,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=resourcemanager.miloapis.com,resources=organizations,verbs=get;list;watch
// +kubebuilder:rbac:groups=iam.miloapis.com,resources=users,verbs=get;list;watch
// +kubebuilder:rbac:groups=iam.miloapis.com,resources=roles,verbs=get;list;watch
// +kubebuilder:rbac:groups=iam.miloapis.com,resources=policybindings,verbs=get;list;watch;delete

func (r *MigrationController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var membership resourcemanagerv1alpha1.OrganizationMembership
	if err := r.Client.Get(ctx, req.NamespacedName, &membership); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Skip if roles already exist
	if len(membership.Spec.Roles) > 0 {
		// If roles exist, check if we can clean up legacy bindings
		return r.maybeCleanupLegacyBindings(ctx, &membership)
	}

	// Find legacy PolicyBindings
	logger.Info("checking for legacy policy bindings",
		"membership", membership.Name,
		"organization", membership.Spec.OrganizationRef.Name,
		"user", membership.Spec.UserRef.Name)

	legacyBindings, err := r.findLegacyBindings(ctx, &membership)
	if err != nil {
		logger.Error(err, "failed to find legacy bindings")
		return ctrl.Result{}, err
	}

	if len(legacyBindings) == 0 {
		logger.V(1).Info("no legacy bindings found")
		return ctrl.Result{}, nil
	}

	logger.Info("found legacy policy bindings", "count", len(legacyBindings))

	// Extract and deduplicate roles
	roles := r.extractRoles(ctx, legacyBindings)
	if len(roles) == 0 {
		logger.Info("no valid roles extracted from bindings")
		return ctrl.Result{}, nil
	}

	// Check if roles have changed to avoid unnecessary updates
	if equality.Semantic.DeepEqual(membership.Spec.Roles, roles) {
		logger.V(1).Info("roles already match, no update needed")
		return ctrl.Result{}, nil
	}

	// Update membership with roles
	updated := membership.DeepCopy()
	updated.Spec.Roles = roles

	logger.Info("updating membership with roles", "roleCount", len(roles))
	if err := r.Client.Update(ctx, updated); err != nil {
		logger.Error(err, "failed to update membership")
		return ctrl.Result{}, err
	}

	logger.Info("successfully migrated membership", "roles", len(roles))

	// Update will trigger another reconcile, no need to requeue
	return ctrl.Result{}, nil
}

func (r *MigrationController) maybeCleanupLegacyBindings(ctx context.Context, membership *resourcemanagerv1alpha1.OrganizationMembership) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Check if all roles are applied
	rolesApplied := false
	for _, condition := range membership.Status.Conditions {
		if condition.Type == RolesApplied &&
			condition.Status == "True" &&
			condition.Reason == "AllRolesApplied" {
			rolesApplied = true
			break
		}
	}

	if !rolesApplied {
		// Roles not yet applied, will reconcile again when status updates
		logger.V(1).Info("waiting for roles to be applied")
		return ctrl.Result{}, nil
	}

	// Find and delete legacy bindings
	legacyBindings, err := r.findLegacyBindings(ctx, membership)
	if err != nil {
		logger.Error(err, "failed to find legacy bindings for cleanup")
		return ctrl.Result{}, err
	}

	if len(legacyBindings) == 0 {
		// No legacy bindings, migration complete
		return ctrl.Result{}, nil
	}

	logger.Info("cleaning up legacy policy bindings", "count", len(legacyBindings))

	for _, binding := range legacyBindings {
		logger.Info("deleting legacy policy binding", "binding", binding.Name)
		if err := r.Client.Delete(ctx, &binding); err != nil && !apierrors.IsNotFound(err) {
			logger.Error(err, "failed to delete legacy binding", "binding", binding.Name)
			// Continue with other deletions
			continue
		}
	}

	logger.Info("legacy binding cleanup complete")
	return ctrl.Result{}, nil
}

func (r *MigrationController) findLegacyBindings(ctx context.Context, membership *resourcemanagerv1alpha1.OrganizationMembership) ([]iamv1alpha1.PolicyBinding, error) {
	// Get User
	var user iamv1alpha1.User
	if err := r.Client.Get(ctx, types.NamespacedName{Name: membership.Spec.UserRef.Name}, &user); err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Get Organization
	var org resourcemanagerv1alpha1.Organization
	if err := r.Client.Get(ctx, types.NamespacedName{Name: membership.Spec.OrganizationRef.Name}, &org); err != nil {
		return nil, fmt.Errorf("failed to get organization: %w", err)
	}

	// List all PolicyBindings in namespace
	var allBindings iamv1alpha1.PolicyBindingList
	if err := r.Client.List(ctx, &allBindings, client.InNamespace(membership.Namespace)); err != nil {
		return nil, fmt.Errorf("failed to list policy bindings: %w", err)
	}

	var legacy []iamv1alpha1.PolicyBinding
	for _, binding := range allBindings.Items {
		// Skip managed bindings
		if binding.Labels != nil && binding.Labels[ManagedByLabel] == ManagedByValue {
			continue
		}

		// Must target the organization
		if binding.Spec.ResourceSelector.ResourceRef == nil {
			continue
		}

		ref := binding.Spec.ResourceSelector.ResourceRef
		if ref.Kind != "Organization" ||
			ref.APIGroup != "resourcemanager.miloapis.com" ||
			ref.Name != org.Name ||
			ref.UID != string(org.UID) {
			continue
		}

		// Must include the user
		hasUser := false
		for _, subject := range binding.Spec.Subjects {
			if subject.Kind == "User" &&
				subject.Name == user.Name &&
				subject.UID == string(user.UID) {
				hasUser = true
				break
			}
		}

		if hasUser {
			legacy = append(legacy, binding)
		}
	}

	return legacy, nil
}

func (r *MigrationController) extractRoles(ctx context.Context, bindings []iamv1alpha1.PolicyBinding) []resourcemanagerv1alpha1.RoleReference {
	logger := log.FromContext(ctx)
	seen := make(map[string]bool)
	var roles []resourcemanagerv1alpha1.RoleReference

	for _, binding := range bindings {
		roleRef := binding.Spec.RoleRef
		key := fmt.Sprintf("%s/%s", roleRef.Namespace, roleRef.Name)

		if seen[key] {
			continue
		}

		// Verify role exists
		var role iamv1alpha1.Role
		if err := r.Client.Get(ctx, types.NamespacedName{
			Name:      roleRef.Name,
			Namespace: roleRef.Namespace,
		}, &role); err != nil {
			if apierrors.IsNotFound(err) {
				logger.Info("role not found, skipping",
					"role", roleRef.Name,
					"namespace", roleRef.Namespace)
				continue
			}
			logger.Error(err, "failed to verify role", "role", roleRef.Name)
			continue
		}

		seen[key] = true
		roles = append(roles, resourcemanagerv1alpha1.RoleReference{
			Name:      roleRef.Name,
			Namespace: roleRef.Namespace,
		})
	}

	return roles
}

func (r *MigrationController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&resourcemanagerv1alpha1.OrganizationMembership{}).
		Named("organization-membership-migration").
		Complete(r)
}
