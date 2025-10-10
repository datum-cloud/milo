package validation

import (
	"strings"

	quotav1alpha1 "go.miloapis.com/milo/pkg/apis/quota/v1alpha1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ClaimTemplateValidator provides validation for CEL expressions used in ClaimCreationPolicy templates.
type ClaimTemplateValidator struct{}

// claimTemplateAllowedVariables defines the allowed template variables for ClaimCreationPolicy
var claimTemplateAllowedVariables = []string{"trigger", "user", "requestInfo"}

// NewClaimTemplateValidator creates a new claim template validator.
func NewClaimTemplateValidator() (*ClaimTemplateValidator, error) {
	return &ClaimTemplateValidator{}, nil
}

// It enforces name/generateName mutual exclusivity and validates CEL expressions.
func (v *ClaimTemplateValidator) ValidateClaimTemplate(t quotav1alpha1.ResourceClaimTemplate) field.ErrorList {
	var allErrs field.ErrorList
	fldPath := field.NewPath("metadata")

	nameSet := strings.TrimSpace(t.Metadata.Name) != ""
	genSet := strings.TrimSpace(t.Metadata.GenerateName) != ""
	if nameSet && genSet {
		allErrs = append(allErrs, field.Invalid(fldPath, t.Metadata, "metadata.name and metadata.generateName are mutually exclusive"))
	}

	// Name can be empty (will use generateName), but if set must be valid
	if t.Metadata.Name != "" {
		if errs := validateTemplateOrKubernetesName(t.Metadata.Name, claimTemplateAllowedVariables, false, fldPath.Child("name")); len(errs) > 0 {
			allErrs = append(allErrs, errs...)
		}
	}
	// GenerateName can be empty, but if set must be valid
	if t.Metadata.GenerateName != "" {
		if errs := validateTemplateOrGenerateName(t.Metadata.GenerateName, claimTemplateAllowedVariables, false, fldPath.Child("generateName")); len(errs) > 0 {
			allErrs = append(allErrs, errs...)
		}
	}
	// Namespace can be empty, but if set must be valid
	if t.Metadata.Namespace != "" {
		if errs := validateTemplateOrKubernetesName(t.Metadata.Namespace, claimTemplateAllowedVariables, false, fldPath.Child("namespace")); len(errs) > 0 {
			allErrs = append(allErrs, errs...)
		}
	}

	// Values can contain CEL expressions
	for k, vStr := range t.Metadata.Annotations {
		if errs := v.validateCELTemplateIfNeeded(vStr, fldPath.Child("annotations").Key(k)); len(errs) > 0 {
			allErrs = append(allErrs, errs...)
		}
	}

	return allErrs
}

// validateCELTemplateIfNeeded validates a string that may contain CEL expressions in {{ }} delimiters.
func (v *ClaimTemplateValidator) validateCELTemplateIfNeeded(s string, fldPath *field.Path) field.ErrorList {
	return validateTemplateOrLiteral(s, claimTemplateAllowedVariables, true, fldPath)
}
