package v1alpha1

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	iamv1alpha1 "go.miloapis.com/milo/pkg/apis/iam/v1alpha1"
	resourcemanagerv1alpha1 "go.miloapis.com/milo/pkg/apis/resourcemanager/v1alpha1"
)

var organizationmembershiplog = logf.Log.WithName("organizationmembership-resource")

// +kubebuilder:webhook:path=/validate-resourcemanager-miloapis-com-v1alpha1-organizationmembership,mutating=false,failurePolicy=fail,sideEffects=None,groups=resourcemanager.miloapis.com,resources=organizationmemberships,verbs=create;update,versions=v1alpha1,name=vorganizationmembership.datum.net,admissionReviewVersions={v1,v1beta1},serviceName=milo-controller-manager,servicePort=9443,serviceNamespace=milo-system

// SetupOrganizationMembershipWebhooksWithManager sets up OrganizationMembership webhooks
func SetupOrganizationMembershipWebhooksWithManager(mgr ctrl.Manager) error {
	organizationmembershiplog.Info("Setting up resourcemanager.miloapis.com organizationmembership webhooks")

	return ctrl.NewWebhookManagedBy(mgr).
		For(&resourcemanagerv1alpha1.OrganizationMembership{}).
		WithValidator(&OrganizationMembershipValidator{
			client: mgr.GetClient(),
		}).
		Complete()
}

// OrganizationMembershipValidator validates OrganizationMemberships
type OrganizationMembershipValidator struct {
	client  client.Client
	decoder admission.Decoder
}

func (v *OrganizationMembershipValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	membership := obj.(*resourcemanagerv1alpha1.OrganizationMembership)
	organizationmembershiplog.Info("Validating OrganizationMembership create", "name", membership.Name, "namespace", membership.Namespace)

	// Validate roles if specified
	if len(membership.Spec.Roles) > 0 {
		if err := v.validateRoles(ctx, membership); err != nil {
			return nil, err
		}
	}

	return nil, nil
}

func (v *OrganizationMembershipValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	membership := newObj.(*resourcemanagerv1alpha1.OrganizationMembership)
	organizationmembershiplog.Info("Validating OrganizationMembership update", "name", membership.Name, "namespace", membership.Namespace)

	// Validate roles if specified
	if len(membership.Spec.Roles) > 0 {
		if err := v.validateRoles(ctx, membership); err != nil {
			return nil, err
		}
	}

	return nil, nil
}

func (v *OrganizationMembershipValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	// No validation needed for delete
	return nil, nil
}

// validateRoles validates the role references in the membership
func (v *OrganizationMembershipValidator) validateRoles(ctx context.Context, membership *resourcemanagerv1alpha1.OrganizationMembership) error {
	// Check for duplicate roles
	if err := v.checkDuplicateRoles(membership); err != nil {
		return err
	}

	// Validate each role reference
	for _, roleRef := range membership.Spec.Roles {
		if err := v.validateRoleReference(ctx, membership, roleRef); err != nil {
			return err
		}
	}

	return nil
}

// checkDuplicateRoles ensures no duplicate roles are specified
func (v *OrganizationMembershipValidator) checkDuplicateRoles(membership *resourcemanagerv1alpha1.OrganizationMembership) error {
	seen := make(map[string]bool)

	for _, roleRef := range membership.Spec.Roles {
		// Create unique key for role
		roleNamespace := roleRef.Namespace
		if roleNamespace == "" {
			roleNamespace = membership.Namespace
		}
		roleKey := fmt.Sprintf("%s/%s", roleNamespace, roleRef.Name)

		if seen[roleKey] {
			return fmt.Errorf("duplicate role reference detected: %s in namespace %s", roleRef.Name, roleNamespace)
		}
		seen[roleKey] = true
	}

	return nil
}

// validateRoleReference validates a single role reference
func (v *OrganizationMembershipValidator) validateRoleReference(ctx context.Context, membership *resourcemanagerv1alpha1.OrganizationMembership, roleRef resourcemanagerv1alpha1.RoleReference) error {
	// Validate role name is not empty
	if roleRef.Name == "" {
		return fmt.Errorf("role name cannot be empty")
	}

	// Determine the namespace to check
	roleNamespace := roleRef.Namespace
	if roleNamespace == "" {
		roleNamespace = membership.Namespace
	}

	// Verify the role exists
	var role iamv1alpha1.Role
	roleKey := client.ObjectKey{
		Name:      roleRef.Name,
		Namespace: roleNamespace,
	}

	if err := v.client.Get(ctx, roleKey, &role); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return fmt.Errorf("role '%s' not found in namespace '%s'", roleRef.Name, roleNamespace)
		}
		return fmt.Errorf("failed to verify role '%s' in namespace '%s': %w", roleRef.Name, roleNamespace, err)
	}

	// Additional validation: ensure role is ready (if it has a status condition)
	// This is optional but helps catch issues early
	if len(role.Status.Conditions) > 0 {
		var readyCondition *metav1.Condition
		for i := range role.Status.Conditions {
			if role.Status.Conditions[i].Type == "Ready" {
				readyCondition = &role.Status.Conditions[i]
				break
			}
		}

		if readyCondition != nil && readyCondition.Status != metav1.ConditionTrue {
			organizationmembershiplog.Info("Warning: role is not ready",
				"role", roleRef.Name,
				"namespace", roleNamespace,
				"condition", readyCondition)
			// Note: We don't fail here, just log a warning
		}
	}

	return nil
}
