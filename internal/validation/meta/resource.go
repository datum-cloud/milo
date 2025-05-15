package meta

import (
	"fmt"
	"strings"

	"go.datum.net/iam/internal/validation/field"
	"k8s.io/apimachinery/pkg/util/validation"
)

const (
	MaxDisplayNameLength = 150
	MaxDescriptionLength = 1000
)

func ValidateResourceID(path *field.Path, resourceID string) field.ErrorList {
	if resourceID == "" {
		return field.ErrorList{field.Required(path, "resourceID is required")}
	}

	if errs := validation.IsDNS1123Label(resourceID); len(errs) > 0 {
		return field.ErrorList{field.Invalid(path, resourceID, fmt.Sprintf("resourceID must be a valid DNS label: %s", strings.Join(errs, ", ")))}
	}

	return nil
}

func ValidateResourceDisplayName(path *field.Path, displayName string) field.ErrorList {
	if displayName == "" {
		// Display name is optional.
		return nil
	}

	if len(displayName) > MaxDisplayNameLength {
		return field.ErrorList{field.TooLongMaxLength(path, displayName, MaxDisplayNameLength)}
	}

	return nil
}

func ValidateResourceDescription(path *field.Path, description string) field.ErrorList {
	if description == "" {
		// Description is optional.
		return nil
	}

	if len(description) > MaxDescriptionLength {
		return field.ErrorList{field.TooLongMaxLength(path, description, MaxDescriptionLength)}
	}

	return nil
}
