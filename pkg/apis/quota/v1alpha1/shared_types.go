package v1alpha1

// ConsumerRef references the quota consumer (the subject that receives limits
// and consumes capacity). Historically named OwnerInstanceRef.
type ConsumerRef struct {
	// APIGroup of the target resource (e.g., "resourcemanager.miloapis.com").
	// Empty string for core API group.
	//
	// +kubebuilder:validation:Optional
	APIGroup string `json:"apiGroup,omitempty"`

	// Resource type
	//
	// +kubebuilder:validation:Required
	Kind string `json:"kind"`

	// Name of the owning resource object instance.
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
