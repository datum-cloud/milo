package quota

import (
	"context"
	"fmt"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	quotav1alpha1 "go.miloapis.com/milo/pkg/apis/quota/v1alpha1"
)

// ValidateResourceRegistrations validates that all provided resource type names
// have corresponding active ResourceRegistrations in the cluster. This is
// reused between the ResourceClaim, ResourceGrant, and DefaultResourceGrant
// controllers.
func ValidateResourceRegistrations(ctx context.Context, c client.Client, resourceTypeNames []string) error {
	logger := log.FromContext(ctx)

	if len(resourceTypeNames) == 0 {
		return nil
	}

	// Create a set of unique resource type names
	resourceTypeSet := make(map[string]bool)
	for _, resourceTypeName := range resourceTypeNames {
		resourceTypeSet[resourceTypeName] = true
	}

	// List all ResourceRegistrations in the cluster (non-namespaced)
	var registrationList quotav1alpha1.ResourceRegistrationList
	if err := c.List(ctx, &registrationList); err != nil {
		return fmt.Errorf("failed to list ResourceRegistrations: %w", err)
	}

	// Create a map of resourceTypeName to registration
	registrationMap := make(map[string]*quotav1alpha1.ResourceRegistration)
	for i := range registrationList.Items {
		registration := &registrationList.Items[i]
		registrationMap[registration.Spec.ResourceTypeName] = registration
	}

	// Check each resource type for a corresponding registration
	for resourceTypeName := range resourceTypeSet {
		registration, found := registrationMap[resourceTypeName]
		if !found {
			return fmt.Errorf("ResourceRegistration not found for resource type %q", resourceTypeName)
		}

		// Ensure the registration is active
		activeCondition := apimeta.FindStatusCondition(registration.Status.Conditions, quotav1alpha1.ResourceRegistrationActive)
		if activeCondition == nil || activeCondition.Status != metav1.ConditionTrue {
			return fmt.Errorf("ResourceRegistration %q is not active", registration.Name)
		}

		logger.Info("Validated ResourceRegistration",
			"registrationName", registration.Name,
			"resourceTypeName", resourceTypeName)
	}

	return nil
}
