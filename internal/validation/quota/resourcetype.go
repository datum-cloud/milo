package quota

import "context"

// ResourceTypeValidator provides an interface for validating resource types against ResourceRegistrations.
type ResourceTypeValidator interface {
	// ValidateResourceType validates a single resource type against ResourceRegistrations.
	// Returns an error if the resource type is not registered or not active.
	// Returns nil if the resource type is valid and active.
	ValidateResourceType(ctx context.Context, resourceType string) error
}