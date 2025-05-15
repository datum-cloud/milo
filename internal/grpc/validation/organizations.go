package validation

import (
	"fmt"

	resourcemanagerpb "buf.build/gen/go/datum-cloud/iam/protocolbuffers/go/datum/resourcemanager/v1alpha"
	"go.datum.net/iam/internal/validation/field"
)

func ValidateOrganization(organization *resourcemanagerpb.Organization) field.ErrorList {
	errs := field.ErrorList{}

	if organization.OrganizationId != "" {
		errs = append(errs, validateOrganizationId(field.NewPath("organization_id"), organization.OrganizationId)...)
	}

	if len(organization.DisplayName) > MaxDisplayNameLength {
		errs = append(errs, field.TooLongMaxLength(field.NewPath("display_name"), organization.DisplayName, MaxDisplayNameLength))
	}

	specFieldPath := field.NewPath("spec")
	if organization.Spec == nil {
		errs = append(errs, field.Required(specFieldPath, ""))
	} else {
		errs = append(errs, validateOrganizationSpec(specFieldPath, organization.Spec)...)
	}

	return errs

}

func validateOrganizationId(fieldPath *field.Path, organizationId string) field.ErrorList {
	errs := field.ErrorList{}

	if len(organizationId) < MinOrganizationIdLength {
		errs = append(errs, field.Invalid(fieldPath, organizationId, fmt.Sprintf("organization_id must be at least %d character long", MinOrganizationIdLength)))
	}

	if len(organizationId) > MaxOrganizationIdLength {
		errs = append(errs, field.TooLongMaxLength(fieldPath, organizationId, MaxOrganizationIdLength))
	}

	return errs
}

func validateOrganizationSpec(fieldPath *field.Path, spec *resourcemanagerpb.Organization_Spec) field.ErrorList {
	errs := field.ErrorList{}
	if len(spec.Description) > MaxDescriptionLength {
		errs = append(errs, field.TooLongMaxLength(fieldPath.Child("description"), spec.Description, MaxDescriptionLength))
	}

	return errs
}
