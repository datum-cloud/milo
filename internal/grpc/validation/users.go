package validation

import (
	iampb "buf.build/gen/go/datum-cloud/iam/protocolbuffers/go/datum/iam/v1alpha"
	"fmt"
	"go.datum.net/iam/internal/validation/field"
	"go.datum.net/iam/internal/validation/meta"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"reflect"
	"strings"
)

type UserAnnotationValidator struct {
	RequiredKey string
}

func (v *UserAnnotationValidator) Validate(fieldPath *field.Path, annotations map[string]string) field.ErrorList {
	errs := field.ErrorList{}

	if _, exists := annotations[v.RequiredKey]; !exists {
		return append(errs, field.Required(fieldPath.Key(v.RequiredKey), fmt.Sprintf("missing required annotation key: %s", v.RequiredKey)))
	}

	providerId := annotations[v.RequiredKey]
	if providerId == "" {
		errs = append(errs, field.Required(fieldPath.Key(v.RequiredKey), fmt.Sprintf("missing required annotation value: %s", v.RequiredKey)))
	}

	return errs
}

func (v *UserAnnotationValidator) GetProviderKey() string {
	return v.RequiredKey
}

// TODO: update this to initialize validator on serve
var UsersAnnotationValidator = &UserAnnotationValidator{
	RequiredKey: "internal.iam.datumapis.com/zitadel-id",
}

func ValidateUser(user *iampb.User) field.ErrorList {
	errs := field.ErrorList{}

	if len(user.DisplayName) > MaxDisplayNameLength {
		errs = append(errs, field.TooLongMaxLength(field.NewPath("display_name"), user.DisplayName, MaxDisplayNameLength))
	}

	errs = append(errs, meta.ValidateAnnotations(field.NewPath("annotations"), user.Annotations)...)
	errs = append(errs, UsersAnnotationValidator.Validate(field.NewPath("annotations"), user.Annotations)...)
	errs = append(errs, meta.ValidateLabels(field.NewPath("labels"), user.Labels)...)

	if user.UserId != "" {
		errs = append(errs, validateUserId(field.NewPath("user_id"), user.UserId)...)
	}

	specFieldPath := field.NewPath("spec")
	if user.Spec == nil {
		errs = append(errs, field.Required(specFieldPath, ""))
	} else {
		errs = append(errs, validateUserSpec(specFieldPath, user.Spec)...)
	}

	return errs
}

func validateUserSpec(fieldPath *field.Path, spec *iampb.UserSpec) field.ErrorList {
	errs := field.ErrorList{}

	if spec.Email == "" {
		errs = append(errs, field.Required(fieldPath.Child("email"), ""))
	}
	// We thrust on the Auth Provider that the email is valid, so no further validation is needed

	if len(spec.GivenName) > MaxGivenNameLength {
		errs = append(errs, field.TooLongMaxLength(fieldPath.Child("given_name"), spec.GivenName, MaxGivenNameLength))
	}

	if len(spec.FamilyName) > MaxFamilyNameLength {
		errs = append(errs, field.TooLongMaxLength(fieldPath.Child("family_name"), spec.FamilyName, MaxFamilyNameLength))
	}

	return errs

}

func validateUserId(fieldPath *field.Path, userId string) field.ErrorList {
	errs := field.ErrorList{}

	if len(userId) < MinUserIdLength {
		errs = append(errs, field.Invalid(fieldPath, userId, fmt.Sprintf("user_id must be at least %d character long", MinUserIdLength)))
	}

	if len(userId) > MaxUserIdLength {
		errs = append(errs, field.TooLongMaxLength(fieldPath, userId, MaxUserIdLength))
	}

	return errs
}

func ValidateListUsersRequest(req *iampb.ListUsersRequest) field.ErrorList {
	errs := field.ErrorList{}

	pageSize := req.PageSize
	pageSizeErrorMessage := fmt.Sprintf("page_size must be greater than 0 and less than %d", MaxUsersPageSize)
	if pageSize < 0 {
		errs = append(errs, field.Invalid(field.NewPath("page_size"), pageSize, pageSizeErrorMessage))
	}
	if pageSize > MaxUsersPageSize {
		errs = append(errs, field.Invalid(field.NewPath("page_size"), pageSize, pageSizeErrorMessage))
	}

	return errs
}

func ValidateUserUpdate(immutablePaths []string, existing *iampb.User, updated *iampb.User, req *iampb.UpdateUserRequest) field.ErrorList {
	errs := field.ErrorList{}
	providerKey := UsersAnnotationValidator.GetProviderKey()

	// Validate that annotations do not include the "provider id" key
	if req.User.Annotations != nil {
		if _, exists := req.User.Annotations[providerKey]; exists {
			errs = append(errs, field.Invalid(field.NewPath(fmt.Sprintf("annotations[%s]", providerKey)), providerKey, "immutable field has been modified"))
		}
	}

	// Check that immutable paths have not been changed
	for _, path := range immutablePaths {
		originalValue := getFieldValue(path, existing)
		updatedValue := getFieldValue(path, updated)

		if !reflect.DeepEqual(originalValue, updatedValue) {
			errs = append(errs, field.Invalid(field.NewPath(path), updatedValue, "immutable field has been modified"))
		}
	}

	return errs
}

func getFieldValue(path string, message proto.Message) interface{} {
	components := strings.Split(path, ".")
	current := message.ProtoReflect()

	for i, component := range components {
		field := current.Descriptor().Fields().ByName(protoreflect.Name(component))
		if field == nil || !current.Has(field) {
			return nil
		}

		value := current.Get(field)

		if i == len(components)-1 {
			return value.Interface()
		}

		if field.Kind() != protoreflect.MessageKind && field.Kind() != protoreflect.GroupKind {
			return nil
		}

		current = value.Message()
	}

	return nil
}
