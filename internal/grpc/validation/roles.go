package validation

import (
	iampb "buf.build/gen/go/datum-cloud/iam/protocolbuffers/go/datum/iam/v1alpha"
	"go.datum.net/iam/internal/validation/field"
	"go.datum.net/iam/internal/validation/meta"
)

type RoleValidatorOptions struct {
	PermissionValidator PermissionValidator
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
		return append(errs, field.Required(fieldPath.Child("spec"), "spec is required"))
	}

	for index, permission := range roleSpec.GetIncludedPermissions() {
		errs = append(errs, opts.PermissionValidator(fieldPath.Child("included_permissions").Index(index), permission)...)
	}

	return errs
}
