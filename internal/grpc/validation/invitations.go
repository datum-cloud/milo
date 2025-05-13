package validation

import (
	"fmt"
	"time"

	resourcemanagerpb "buf.build/gen/go/datum-cloud/iam/protocolbuffers/go/datum/resourcemanager/v1alpha"
	"go.datum.net/iam/internal/validation/field"
)

func ValidateInvitation(invitation *resourcemanagerpb.Invitation, parent string) field.ErrorList {
	errs := field.ErrorList{}

	if invitation.InvitationId != "" {
		errs = append(errs, validateInvitationId(field.NewPath("invitation_id"), invitation.InvitationId)...)
	}

	if len(invitation.DisplayName) > MaxDisplayNameLength {
		errs = append(errs, field.TooLongMaxLength(field.NewPath("display_name"), invitation.DisplayName, MaxDisplayNameLength))
	}

	specFieldPath := field.NewPath("spec")
	if invitation.Spec == nil {
		errs = append(errs, field.Required(specFieldPath, ""))
	} else {
		errs = append(errs, validateInvitationSpec(specFieldPath, invitation.Spec, parent)...)
	}

	return errs
}

func validateInvitationId(fieldPath *field.Path, invitationId string) field.ErrorList {
	errs := field.ErrorList{}

	if len(invitationId) < MinInvitationIdLength {
		errs = append(errs, field.Invalid(fieldPath, invitationId, fmt.Sprintf("invitation_id must be at least %d character long", MinInvitationIdLength)))
	}

	if len(invitationId) > MaxInvitationIdLength {
		errs = append(errs, field.TooLongMaxLength(fieldPath, invitationId, MaxInvitationIdLength))
	}

	return errs
}

func validateInvitationSpec(fieldPath *field.Path, spec *resourcemanagerpb.Spec, parent string) field.ErrorList {
	errs := field.ErrorList{}

	if spec.RecipientEmailAddress == "" {
		errs = append(errs, field.Required(fieldPath.Child("recipient_email_address"), ""))
	}

	if spec.ExpirationTime.AsTime().Before(time.Now()) {
		errs = append(errs, field.Invalid(fieldPath.Child("expiration_time"), spec.ExpirationTime, "expiration_time must be in the future"))
	}

	if spec.Roles == nil {
		errs = append(errs, field.Required(fieldPath.Child("roles"), ""))
	}

	return errs
}
