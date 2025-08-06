package v1alpha1

// OwnerInstanceRef is a reference to the specific owning resource object
// instance.
type OwnerInstanceRef struct {
	// Resource type
	//
	// +kubebuilder:validation:Required
	Kind string `json:"kind"`

	// Name of the owning resource object instance.
	//
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// ObservedGeneration is the generation of the owning resource object instance
	// that was used to populate this OwnerInstanceRef.
	//
	// +kubebuilder:validation:Optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
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
