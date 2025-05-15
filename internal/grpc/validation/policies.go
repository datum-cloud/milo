package validation

import (
	"context"
	"fmt"
	"log/slog"

	iampb "buf.build/gen/go/datum-cloud/iam/protocolbuffers/go/datum/iam/v1alpha"
	"go.datum.net/iam/internal/role"
	"go.datum.net/iam/internal/subject"
	"go.datum.net/iam/internal/validation/field"
	"go.datum.net/iam/internal/validation/meta"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type PolicyValidatorOptions struct {
	// RoleResolver will return an error if a role cannot be resolved or used in
	// an IAM policy.
	RoleResolver role.Resolver
	// SubjectResolver can resolve a subject to validate that a subject may be
	// used in an IAM Policy.
	SubjectResolver subject.Resolver
	// The context of the validation request.
	Context context.Context
}

func ValidatePolicy(policy *iampb.Policy, opts PolicyValidatorOptions) field.ErrorList {
	errs := field.ErrorList{}
	fieldPath := field.NewPath("policy")
	errs = append(errs, validatePolicySpec(fieldPath.Child("spec"), policy.Spec, opts)...)
	errs = append(errs, meta.ValidateAnnotations(fieldPath.Child("annotations"), policy.Annotations)...)
	errs = append(errs, meta.ValidateLabels(fieldPath.Child("labels"), policy.Labels)...)

	return errs
}

func validatePolicySpec(fieldPath *field.Path, spec *iampb.PolicySpec, opts PolicyValidatorOptions) field.ErrorList {
	if spec == nil {
		return field.ErrorList{field.Required(fieldPath, "")}
	}

	errs := field.ErrorList{}

	bindingsPath := fieldPath.Child("bindings")
	for index, binding := range spec.Bindings {
		bindingPath := bindingsPath.Index(index)

		if binding.Role == "" {
			errs = append(errs, field.Required(bindingPath.Child("role"), ""))
		} else if err := opts.RoleResolver(opts.Context, binding.Role); err != nil {
			if code := status.Code(err); code == codes.NotFound || code == codes.PermissionDenied {
				errs = append(errs, field.Invalid(bindingPath.Child("role"), binding.Role, "invalid role provided"))
			} else {
				slog.WarnContext(
					opts.Context,
					"failed to check role access",
					slog.String("role", binding.Role),
					slog.String("error", err.Error()),
				)
				errs = append(errs, field.InternalError(
					bindingPath.Child("role"),
					fmt.Errorf("error validating role '%s' in binding", binding.Role),
				))
			}
		}

		membersPath := bindingPath.Child("members")
		if len(binding.Members) == 0 {
			errs = append(errs, field.Required(membersPath, ""))
		} else {
			for index, member := range binding.Members {
				if member == "allAuthenticatedUsers" {
					continue
				}

				if _, _, err := subject.Parse(member); err != nil {
					errs = append(errs, field.Invalid(membersPath.Index(index), member, "Invalid member provided. Must be in format 'allAuthenticatedUsers', 'user:*', 'serviceAccount:*', or 'group:*'"))
				} else if _, err := opts.SubjectResolver(opts.Context, member); err != nil {
					errs = append(errs, field.Invalid(membersPath.Index(index), member, "Member must be an active user, service account, or group."))
				}
			}
		}
	}

	return errs
}
