package validation

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/client-go/dynamic"

	quotav1alpha1 "go.miloapis.com/milo/pkg/apis/quota/v1alpha1"
)

// ResourceClaimValidator provides validation capabilities for ResourceClaim objects.
type ResourceClaimValidator interface {
	// ValidateResourceClaimAgainstRegistrations validates that a ResourceClaim's
	// ResourceRef is allowed to claim each of the requested resource types.
	// Returns a field.ErrorList for structured error reporting, consistent with Kubernetes conventions.
	ValidateResourceClaimAgainstRegistrations(ctx context.Context, claim *quotav1alpha1.ResourceClaim) field.ErrorList
}

// resourceClaimValidator implements ResourceClaimValidator using dynamic client for resource access
// and ResourceTypeValidator for fast cached resource type validation.
type resourceClaimValidator struct {
	dynamicClient         dynamic.Interface
	resourceTypeValidator ResourceTypeValidator
}

// NewResourceClaimValidator creates a new ResourceClaim validator with dynamic client support and
// a required ResourceTypeValidator for optimized resource type validation.
func NewResourceClaimValidator(dynamicClient dynamic.Interface, resourceTypeValidator ResourceTypeValidator) ResourceClaimValidator {
	return &resourceClaimValidator{
		dynamicClient:         dynamicClient,
		resourceTypeValidator: resourceTypeValidator,
	}
}

// ValidateResourceClaimAgainstRegistrations validates that the ResourceClaim's
// ResourceRef is allowed to claim each of the requested resource types.
// Returns a field.ErrorList containing all validation errors found.
func (v *resourceClaimValidator) ValidateResourceClaimAgainstRegistrations(ctx context.Context, claim *quotav1alpha1.ResourceClaim) field.ErrorList {
	var allErrs field.ErrorList

	// Get the resource type that's claiming (from ResourceRef)
	claimingResource := claim.Spec.ResourceRef
	specPath := field.NewPath("spec")

	// For each request in the claim, verify it's allowed
	for i, request := range claim.Spec.Requests {
		requestPath := specPath.Child("requests").Index(i)
		resourceTypePath := requestPath.Child("resourceType")

		// Validate that the resource type is registered and active using the cached validator
		if err := v.resourceTypeValidator.ValidateResourceType(ctx, request.ResourceType); err != nil {
			allErrs = append(allErrs, field.Invalid(resourceTypePath, request.ResourceType, err.Error()))
			continue // Skip further validation for this request
		}

		// Check if the claiming resource is allowed using the cached validator
		allowed, allowedList, err := v.resourceTypeValidator.IsClaimingResourceAllowed(ctx, request.ResourceType, claim.Spec.ConsumerRef, claimingResource.APIGroup, claimingResource.Kind)
		if err != nil {
			allErrs = append(allErrs, field.InternalError(resourceTypePath, fmt.Errorf("failed to check claiming resource permission: %w", err)))
			continue
		}

		if !allowed {
			// Build helpful error message
			claimingResourceStr := claimingResource.Kind
			if claimingResource.APIGroup != "" {
				claimingResourceStr = fmt.Sprintf("%s/%s", claimingResource.APIGroup, claimingResource.Kind)
			}

			var errMsg string
			if len(allowedList) == 0 {
				errMsg = fmt.Sprintf("resource type %s is not allowed to claim quota for %s. No ClaimingResources configured",
					claimingResourceStr, request.ResourceType)
			} else {
				errMsg = fmt.Sprintf("resource type %s is not allowed to claim quota for %s. Allowed claiming resources: [%s]",
					claimingResourceStr, request.ResourceType, strings.Join(allowedList, ", "))
			}

			allErrs = append(allErrs, field.Forbidden(resourceTypePath, errMsg))
		}
	}

	return allErrs
}
