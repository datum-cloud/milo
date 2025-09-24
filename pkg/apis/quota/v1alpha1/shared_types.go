package v1alpha1

// ConsumerRef identifies a quota consumer - the entity that receives quota grants
// and creates quota claims. Consumers are typically hierarchical (Organization > Project > User).
type ConsumerRef struct {
	// APIGroup specifies the API group of the consumer resource.
	// Use full group name for Milo resources.
	//
	// Examples:
	// - "resourcemanager.miloapis.com" (Organization/Project resources)
	// - "iam.miloapis.com" (User/Group resources)
	// - "infrastructure.miloapis.com" (infrastructure resources)
	//
	// +kubebuilder:validation:Optional
	APIGroup string `json:"apiGroup,omitempty"`

	// Kind specifies the type of consumer resource.
	// Must match an existing Kubernetes resource type that can receive quota grants.
	//
	// Common consumer types:
	// - "Organization" (top-level quota consumer)
	// - "Project" (project-level quota consumer)
	// - "User" (user-level quota consumer)
	//
	// +kubebuilder:validation:Required
	Kind string `json:"kind"`

	// Name identifies the specific consumer resource instance.
	// Must match the name of an existing consumer resource in the cluster.
	//
	// Examples:
	// - "acme-corp" (Organization name)
	// - "web-application" (Project name)
	// - "john.doe" (User name)
	//
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Namespace identifies the namespace of the consumer resource.
	// Required for namespaced consumer resources (e.g., Projects).
	// Leave empty for cluster-scoped consumer resources (e.g., Organizations).
	//
	// Examples:
	// - "" (empty for cluster-scoped Organizations)
	// - "organization-acme-corp" (namespace for Projects within an organization)
	// - "project-web-app" (namespace for resources within a project)
	//
	// +kubebuilder:validation:Optional
	Namespace string `json:"namespace,omitempty"`
}

type ContributingResourceRef struct {
	// Name of the resource
	//
	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// ObservedGeneration is the generation of the resource
	// that was used to populate this ContributingResourceRef.
	//
	// +kubebuilder:validation:Required
	ObservedGeneration int64 `json:"observedGeneration"`
}

// UnversionedObjectReference provides a stable reference to a Kubernetes resource
// that remains valid across API version changes. Used to link ResourceClaims
// to their triggering resources for lifecycle management.
type UnversionedObjectReference struct {
	// APIGroup specifies the API group of the referenced resource.
	// Use full group name for Milo resources.
	//
	// Examples:
	// - "resourcemanager.miloapis.com" (Project, Organization)
	// - "iam.miloapis.com" (User, Group)
	// - "infrastructure.miloapis.com" (infrastructure resources)
	//
	// +kubebuilder:validation:Optional
	APIGroup string `json:"apiGroup,omitempty"`

	// Kind specifies the type of the referenced resource.
	// Must match an existing Kubernetes resource type.
	//
	// Examples:
	// - "Project" (Project resource that triggered quota claim)
	// - "User" (User resource that triggered quota claim)
	// - "Organization" (Organization resource that triggered quota claim)
	//
	// +kubebuilder:validation:Required
	Kind string `json:"kind"`

	// Name identifies the specific resource instance that triggered the quota claim.
	// Used for linking claims back to their triggering resources.
	//
	// Examples:
	// - "web-app-project" (Project that triggered Project quota claim)
	// - "john.doe" (User that triggered User quota claim)
	//
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Namespace specifies the namespace containing the referenced resource.
	// Required for namespaced resources, omitted for cluster-scoped resources.
	//
	// Examples:
	// - "acme-corp" (organization namespace containing Project)
	// - "team-alpha" (project namespace containing User)
	// - "" or omitted (for cluster-scoped resources like Organization)
	//
	// +kubebuilder:validation:Optional
	Namespace string `json:"namespace,omitempty"`
}

// MatchesConsumer checks if two ConsumerRef instances refer to the same consumer.
// Matching is based on name/kind/apiGroup.
func (c ConsumerRef) MatchesConsumer(other ConsumerRef) bool {
	return c.Name == other.Name &&
		c.Kind == other.Kind &&
		c.APIGroup == other.APIGroup
}

const (
	// MiloSystemNamespace is the namespace where the Milo system components are
	// deployed in the project control-plane.
	MiloSystemNamespace = "milo-system"
)
