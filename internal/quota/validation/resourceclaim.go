package validation

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/client-go/dynamic"

	quotav1alpha1 "go.miloapis.com/milo/pkg/apis/quota/v1alpha1"
)

// ResourceClaimValidator provides validation capabilities for ResourceClaim objects.
type ResourceClaimValidator interface {
	// ValidateResourceClaimAgainstRegistrations validates that a ResourceClaim's
	// ResourceRef is allowed to claim each of the requested resource types.
	ValidateResourceClaimAgainstRegistrations(ctx context.Context, claim *quotav1alpha1.ResourceClaim) error
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
func (v *resourceClaimValidator) ValidateResourceClaimAgainstRegistrations(ctx context.Context, claim *quotav1alpha1.ResourceClaim) error {
	// Get the resource type that's claiming (from ResourceRef)
	claimingResource := claim.Spec.ResourceRef

	// For each request in the claim, verify it's allowed
	for _, request := range claim.Spec.Requests {
		// Validate that the resource type is registered and active using the cached validator
		if err := v.resourceTypeValidator.ValidateResourceType(ctx, request.ResourceType); err != nil {
			return err // This provides user-friendly error messages about registration
		}

		// Check if the claiming resource is allowed using the cached validator
		allowed, allowedList, err := v.resourceTypeValidator.IsClaimingResourceAllowed(ctx, request.ResourceType, claim.Spec.ConsumerRef, claimingResource.APIGroup, claimingResource.Kind)
		if err != nil {
			return fmt.Errorf("failed to check claiming resource permission for %s: %w", request.ResourceType, err)
		}
		if !allowed {
			// Build helpful error message
			claimingResourceStr := claimingResource.Kind
			if claimingResource.APIGroup != "" {
				claimingResourceStr = fmt.Sprintf("%s/%s", claimingResource.APIGroup, claimingResource.Kind)
			}

			if len(allowedList) == 0 {
				return fmt.Errorf("resource type %s is not allowed to claim quota for %s. No ClaimingResources configured",
					claimingResourceStr, request.ResourceType)
			}

			return fmt.Errorf("resource type %s is not allowed to claim quota for %s. Allowed claiming resources: [%s]",
				claimingResourceStr, request.ResourceType, strings.Join(allowedList, ", "))
		}
	}

	return nil
}


