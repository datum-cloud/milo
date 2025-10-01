package validation

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/client-go/dynamic"

	quotav1alpha1 "go.miloapis.com/milo/pkg/apis/quota/v1alpha1"
)

// ValidationEngine provides validation capabilities for quota resources.
type ValidationEngine interface {
	// ValidateResourceClaimAgainstRegistrations validates that a ResourceClaim's
	// ResourceRef is allowed to claim each of the requested resource types.
	ValidateResourceClaimAgainstRegistrations(ctx context.Context, claim *quotav1alpha1.ResourceClaim) error

	// ValidateClaimingResourcesConfiguration validates that all ClaimingResources
	// in a ResourceRegistration are valid and that there are no duplicates.
	ValidateClaimingResourcesConfiguration(registration *quotav1alpha1.ResourceRegistration) error
}

// validationEngine implements ValidationEngine using dynamic client for resource access
// and ResourceTypeValidator for fast cached resource type validation.
type validationEngine struct {
	dynamicClient         dynamic.Interface
	resourceTypeValidator ResourceTypeValidator
}

// NewValidationEngine creates a new validation engine with dynamic client support and
// a required ResourceTypeValidator for optimized resource type validation.
func NewValidationEngine(dynamicClient dynamic.Interface, resourceTypeValidator ResourceTypeValidator) ValidationEngine {
	return &validationEngine{
		dynamicClient:         dynamicClient,
		resourceTypeValidator: resourceTypeValidator,
	}
}

// ValidateResourceClaimAgainstRegistrations validates that the ResourceClaim's
// ResourceRef is allowed to claim each of the requested resource types.
func (e *validationEngine) ValidateResourceClaimAgainstRegistrations(ctx context.Context, claim *quotav1alpha1.ResourceClaim) error {
	// Get the resource type that's claiming (from ResourceRef)
	claimingResource := claim.Spec.ResourceRef

	// For each request in the claim, verify it's allowed
	for _, request := range claim.Spec.Requests {
		// Validate that the resource type is registered and active using the cached validator
		if err := e.resourceTypeValidator.ValidateResourceType(ctx, request.ResourceType); err != nil {
			return err // This provides user-friendly error messages about registration
		}

		// Check if the claiming resource is allowed using the cached validator
		allowed, allowedList, err := e.resourceTypeValidator.IsClaimingResourceAllowed(ctx, request.ResourceType, claim.Spec.ConsumerRef, claimingResource.APIGroup, claimingResource.Kind)
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

// ValidateClaimingResourcesConfiguration validates that all ClaimingResources
// in a ResourceRegistration are valid and that there are no duplicates.
func (e *validationEngine) ValidateClaimingResourcesConfiguration(registration *quotav1alpha1.ResourceRegistration) error {
	if len(registration.Spec.ClaimingResources) == 0 {
		// Empty is valid - will use defaults
		return nil
	}

	// Check for duplicates
	seen := make(map[string]bool)
	for _, cr := range registration.Spec.ClaimingResources {
		key := fmt.Sprintf("%s/%s", cr.APIGroup, cr.Kind)
		if seen[key] {
			return fmt.Errorf("duplicate ClaimingResource: %s", key)
		}
		seen[key] = true
	}

	// Validate that ClaimingResources don't reference invalid kinds
	for _, cr := range registration.Spec.ClaimingResources {
		if cr.Kind == "" {
			return fmt.Errorf("ClaimingResource must have a Kind")
		}
		// Additional validation could be added here (e.g., checking if the Kind exists in the cluster)
	}

	return nil
}
