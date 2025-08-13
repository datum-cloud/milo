package v1alpha1

// OwnerInstanceRef is a reference to the specific owning resource object
// instance.
type OwnerInstanceRef struct {
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

	// UID of the owning resource object instance.
	//
	// +kubebuilder:validation:Required
	UID string `json:"uid"`
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

const (
	// MiloSystemNamespace is the namespace where the Milo system components are
	// deployed in the project control-plane.
	MiloSystemNamespace = "milo-system"
)
