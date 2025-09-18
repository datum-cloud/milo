package quota

import (
	"context"
	"encoding/json"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	quotav1alpha1 "go.miloapis.com/milo/pkg/apis/quota/v1alpha1"
)

// ValidateResourceClaimAgainstRegistrations validates that the ResourceClaim's
// ResourceRef is allowed to claim each of the requested resource types.
func ValidateResourceClaimAgainstRegistrations(ctx context.Context, dynamicClient dynamic.Interface, claim *quotav1alpha1.ResourceClaim) error {
	// Get the resource type that's claiming (from ResourceRef)
	claimingResource := claim.Spec.ResourceRef

	// For each request in the claim, verify it's allowed
	for _, request := range claim.Spec.Requests {
		// Find the ResourceRegistration for this resource type
		registration, err := findResourceRegistrationWithDynamicClient(ctx, dynamicClient, request.ResourceType, claim.Spec.ConsumerRef)
		if err != nil {
			return fmt.Errorf("failed to find ResourceRegistration for %s: %w", request.ResourceType, err)
		}
		if registration == nil {
			return fmt.Errorf("no ResourceRegistration found for resource type %s with consumer type %s/%s",
				request.ResourceType, claim.Spec.ConsumerRef.APIGroup, claim.Spec.ConsumerRef.Kind)
		}

		// Check if the claiming resource is allowed
		if !registration.IsClaimingResourceAllowed(claimingResource.APIGroup, claimingResource.Kind) {
			// Build helpful error message
			claimingResourceStr := claimingResource.Kind
			if claimingResource.APIGroup != "" {
				claimingResourceStr = fmt.Sprintf("%s/%s", claimingResource.APIGroup, claimingResource.Kind)
			}

			if len(registration.Spec.ClaimingResources) == 0 {
				return fmt.Errorf("resource type %s is not allowed to claim quota for %s. No ClaimingResources configured in ResourceRegistration %s",
					claimingResourceStr, request.ResourceType, registration.Name)
			}

			allowedList := ""
			for i, ar := range registration.Spec.ClaimingResources {
				if i > 0 {
					allowedList += ", "
				}
				if ar.APIGroup == "" {
					allowedList += fmt.Sprintf("core/%s", ar.Kind)
				} else {
					allowedList += fmt.Sprintf("%s/%s", ar.APIGroup, ar.Kind)
				}
			}

			return fmt.Errorf("resource type %s is not allowed to claim quota for %s. Allowed claiming resources: [%s]",
				claimingResourceStr, request.ResourceType, allowedList)
		}
	}

	return nil
}

// findResourceRegistrationWithDynamicClient finds the ResourceRegistration for a given resource type and consumer using dynamic client
func findResourceRegistrationWithDynamicClient(ctx context.Context, dynamicClient dynamic.Interface, resourceType string, consumerRef quotav1alpha1.ConsumerRef) (*quotav1alpha1.ResourceRegistration, error) {
	// List all ResourceRegistrations using dynamic client
	gvr := schema.GroupVersionResource{
		Group:    "quota.miloapis.com",
		Version:  "v1alpha1",
		Resource: "resourceregistrations",
	}

	list, err := dynamicClient.Resource(gvr).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list ResourceRegistrations: %w", err)
	}

	// Find matching registration
	for _, item := range list.Items {
		// Convert unstructured to ResourceRegistration
		registrationBytes, err := item.MarshalJSON()
		if err != nil {
			continue // Skip malformed items
		}

		var registration quotav1alpha1.ResourceRegistration
		if err := json.Unmarshal(registrationBytes, &registration); err != nil {
			continue // Skip malformed items
		}

		// Check if ResourceType matches
		if registration.Spec.ResourceType != resourceType {
			continue
		}

		// Check if ConsumerTypeRef matches
		if registration.Spec.ConsumerTypeRef.APIGroup != consumerRef.APIGroup ||
			registration.Spec.ConsumerTypeRef.Kind != consumerRef.Kind {
			continue
		}

		// Found a match
		return &registration, nil
	}

	return nil, nil
}

// ValidateClaimingResourcesConfiguration validates that all ClaimingResources
// in a ResourceRegistration are valid and that there are no duplicates.
func ValidateClaimingResourcesConfiguration(registration *quotav1alpha1.ResourceRegistration) error {
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
