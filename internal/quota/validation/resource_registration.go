package validation

import (
	"fmt"

	quotav1alpha1 "go.miloapis.com/milo/pkg/apis/quota/v1alpha1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ResourceRegistrationValidator validates ResourceRegistration resources.
type ResourceRegistrationValidator struct{}

// NewResourceRegistrationValidator creates a new ResourceRegistrationValidator.
func NewResourceRegistrationValidator() *ResourceRegistrationValidator {
	return &ResourceRegistrationValidator{}
}

// Validate performs complete validation of a ResourceRegistration.
func (v *ResourceRegistrationValidator) Validate(registration *quotav1alpha1.ResourceRegistration) field.ErrorList {
	var allErrs field.ErrorList

	if errs := v.validateClaimingResourcesDuplicates(registration); len(errs) > 0 {
		allErrs = append(allErrs, errs...)
	}

	return allErrs
}

// validateClaimingResourcesDuplicates checks for duplicate entries in the claimingResources array.
// Moved from CEL validation due to cost limits with nested loops.
func (v *ResourceRegistrationValidator) validateClaimingResourcesDuplicates(registration *quotav1alpha1.ResourceRegistration) field.ErrorList {
	var allErrs field.ErrorList

	if len(registration.Spec.ClaimingResources) <= 1 {
		return nil
	}

	claimingResourcesPath := field.NewPath("spec", "claimingResources")
	seen := make(map[string]int)

	for i, cr := range registration.Spec.ClaimingResources {
		key := fmt.Sprintf("%s/%s", cr.APIGroup, cr.Kind)
		if firstIndex, exists := seen[key]; exists {
			allErrs = append(allErrs, field.Duplicate(
				claimingResourcesPath.Index(i),
				fmt.Sprintf("duplicate claiming resource '%s' (first occurrence at index %d)", key, firstIndex),
			))
		}
		seen[key] = i
	}

	return allErrs
}
