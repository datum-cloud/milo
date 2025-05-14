package validation

import (
	"context"
	"fmt"
	"regexp"

	iampb "buf.build/gen/go/datum-cloud/iam/protocolbuffers/go/datum/iam/v1alpha"
	"go.datum.net/iam/internal/storage"
	"go.datum.net/iam/internal/validation/field"
	"go.datum.net/iam/internal/validation/meta"
)

type RoleValidator func(fieldPath *field.Path, role string) field.ErrorList

type RoleValidatorOptions struct {
	PermissionValidator PermissionValidator
	RoleValidator       RoleValidator
}

func ValidateRole(role *iampb.Role, opts *RoleValidatorOptions) field.ErrorList {
	errs := field.ErrorList{}

	errs = append(errs, meta.ValidateLabels(field.NewPath("labels"), role.Labels)...)
	errs = append(errs, meta.ValidateAnnotations(field.NewPath("annotations"), role.Annotations)...)
	errs = append(errs, validateRoleSpec(field.NewPath("spec"), role.Spec, opts)...)

	return errs
}

func validateRoleSpec(fieldPath *field.Path, roleSpec *iampb.RoleSpec, opts *RoleValidatorOptions) field.ErrorList {
	errs := field.ErrorList{}

	if roleSpec == nil {
		return append(errs, field.Required(fieldPath, "spec is required"))
	}

	errs = append(errs, validateSpecIncludedPermissions(fieldPath, roleSpec, opts)...)
	errs = append(errs, validateSpecInheritedRoles(fieldPath, roleSpec, opts)...)

	return errs
}

func validateSpecIncludedPermissions(fieldPath *field.Path, roleSpec *iampb.RoleSpec, opts *RoleValidatorOptions) field.ErrorList {
	errs := field.ErrorList{}

	for index, permission := range roleSpec.GetIncludedPermissions() {
		errs = append(errs, opts.PermissionValidator(fieldPath.Child("included_permissions").Index(index), permission)...)
	}

	return errs
}

func validateSpecInheritedRoles(fieldPath *field.Path, roleSpec *iampb.RoleSpec, opts *RoleValidatorOptions) field.ErrorList {
	errs := field.ErrorList{}

	for index, role := range roleSpec.GetInheritedRoles() {
		errs = append(errs, opts.RoleValidator(fieldPath.Child("inherited_roles").Index(index), role)...)
	}

	return errs
}

var roleMatcher *regexp.Regexp

func init() {
	roleMatcher = regexp.MustCompile(`services\/([a-zA-Z0-9\.\-]+)\/roles\/([a-zA-Z0-9\.\-]+)`)
}

func NewRoleValidator(services storage.ResourceGetter[*iampb.Role]) RoleValidator {
	return func(fieldPath *field.Path, role string) field.ErrorList {
		errs := field.ErrorList{}
		matches := roleMatcher.FindStringSubmatch(role)
		if len(matches) != 3 {
			errs = append(errs, field.Invalid(fieldPath, role, "role must be in the format `services/{service_name}/roles/{role_name}`"))
			return errs
		}

		resource, err := services.GetResource(context.Background(), &storage.GetResourceRequest{
			Name: role,
		})
		if len(resource.GetName()) == 0 && err != nil {
			errs = append(errs, field.NotFound(fieldPath, role))
		} else if err != nil {
			errs = append(errs, field.InternalError(fieldPath, fmt.Errorf("internal error when validating role")))
		}

		return errs
	}
}
