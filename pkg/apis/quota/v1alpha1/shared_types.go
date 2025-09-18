package v1alpha1

// ConsumerRef references the quota consumer (the subject that receives limits
// and consumes capacity). Historically named OwnerInstanceRef.
type ConsumerRef struct {
	// APIGroup of the target resource (e.g., "resourcemanager.miloapis.com").
	// Empty string for core API group.
	//
	// +kubebuilder:validation:Optional
	APIGroup string `json:"apiGroup,omitempty"`
	// Kind of the consumer resource (for example, Organization, Project).
	//
	// +kubebuilder:validation:Required
	Kind string `json:"kind"`
	// Name of the consumer resource object instance.
	//
	// +kubebuilder:validation:Required
	Name string `json:"name"`
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

// UnversionedObjectReference contains enough information to let you inspect or modify the referred object.
// This is an unversioned reference that persists across API version upgrades, containing only
// the API group, kind, name, and namespace (when applicable).
type UnversionedObjectReference struct {
	// APIGroup is the group for the resource being referenced.
	// If APIGroup is not specified, the specified Kind must be in the core API group.
	// For any other third-party types, APIGroup is required.
	//
	// +kubebuilder:validation:Optional
	APIGroup string `json:"apiGroup,omitempty"`
	// Kind of the referent.
	//
	// +kubebuilder:validation:Required
	Kind string `json:"kind"`
	// Name of the referent.
	//
	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// Namespace of the referent.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/namespaces/
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
