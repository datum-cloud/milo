package openfga

import (
	"fmt"
	"strings"

	iampb "buf.build/gen/go/datum-cloud/iam/protocolbuffers/go/datum/iam/v1alpha"
)

type resourceGraphNode struct {
	// The fully qualified Type of the resource registered in the system. This
	// will always be in the format `<service_name>/<type>` (e.g.
	// compute.datumapis.com/Workload).
	ResourceType string

	// A list of permissions that are supported directly against this resource.
	// This will not contain a list of inherited permissions.
	DirectPermissions []string

	// A list of resources that can be a parent of the current resource in the
	// graph.
	ParentResources []string

	// A list of child resources that are direct child resources of the parent
	// resource.
	ChildResources []*resourceGraphNode
}

// Create a new graph of resources and the permissions that may be granted to
// each resource based on their parent / child relationships defined in the
// resource hierarchy.
//
// Each node in the graph represents a single resource and any permissions that
// may be granted to that resource. A parent resource may be granted any
// permission defined by a child resource to support permission inheritance.
//
// An error will be returned if no root resources were found in the hierarchy. A
// root resource is defined as a resource with no parent resources.
func getResourceGraph(services map[string]*iampb.Service) (*resourceGraphNode, error) {
	if len(services) == 0 {
		return &resourceGraphNode{}, nil
	}

	rootResources := []*iampb.Resource{}
	// Contains a mapping of parent resources to their direct children.
	directChildren := map[string][]string{}
	resources := map[string]*iampb.Resource{}

	for _, service := range services {
		for _, resource := range service.GetSpec().GetResources() {
			// Build a mapping of all resources based on the resource's type.
			resources[resource.Type] = resource

			// Track which resources are the root resources in the hierarchy so we can
			// build a graph.
			if len(resource.ParentResources) == 0 {
				rootResources = append(rootResources, resource)
			}

			// All resources have the root resource as its parent so permissions can
			// be bound to the root resource to grant permissions across all
			// resources.
			resource.ParentResources = append(resource.ParentResources, "iam.datumapis.com/Root")

			for _, parent := range resource.ParentResources {
				// Some resources may parent themselves (e.g. Folders can have a
				// folder as a parent to create a folder tree). We can skip
				// tracking parent / child relationships between the same entity
				// because a resource will already have permissions bound.
				if parent != resource.Type {
					directChildren[parent] = append(directChildren[parent], resource.Type)
				}
			}
		}
	}

	if len(rootResources) == 0 {
		return nil, fmt.Errorf("did not find any root resources in the hierarchy")
	}

	nodes := []*resourceGraphNode{}
	for _, resource := range rootResources {
		node, err := getResourceGraphNode(resource, resources, directChildren)
		if err != nil {
			return nil, fmt.Errorf("could not get root graph node: %v", err)
		}
		nodes = append(nodes, node)
	}

	return &resourceGraphNode{
		ResourceType:   "iam.datumapis.com/Root",
		ChildResources: nodes,
	}, nil
}

func getResourceGraphNode(resource *iampb.Resource, resources map[string]*iampb.Resource, directChildren map[string][]string) (*resourceGraphNode, error) {
	childNodes := []*resourceGraphNode{}
	// Build a tree of nodes based on the direct children of the resource we're
	// treating as the root node.
	for _, child := range directChildren[resource.Type] {
		childResource, found := resources[child]
		if !found {
			return nil, fmt.Errorf("did not find child resource of type '%s'", child)
		}

		childNode, err := getResourceGraphNode(childResource, resources, directChildren)
		if err != nil {
			return nil, fmt.Errorf("failed to create graph node for child resource: %w", err)
		}
		childNodes = append(childNodes, childNode)
	}

	node := &resourceGraphNode{
		ResourceType:    resource.Type,
		ParentResources: resource.ParentResources,
		ChildResources:  childNodes,
	}

	// The resource type will always be in the fully qualified format:
	// `<service_name>/<Resource>`.
	resourceTypeParts := strings.Split(resource.Type, "/")
	if len(resourceTypeParts) != 2 {
		return nil, fmt.Errorf("invalid type provided for resource, expected to be in the format `<service_name>/<resource>`: type %s", resource.Type)
	}

	// Builds the fully qualified resource name based on the service name,
	// resource plural name, and permission name.
	for _, permission := range resource.Permissions {
		node.DirectPermissions = append(node.DirectPermissions, fmt.Sprintf("%s/%s.%s", resourceTypeParts[0], resource.Plural, permission))
	}

	return node, nil
}
