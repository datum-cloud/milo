package v1alpha1

type OwnerRef struct {
	// API group of the resource
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`
	APIGroup string `json:"apiGroup"`

	// Resource type
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^[A-Z][a-zA-Z]*$`
	Kind string `json:"kind"`

	// Name of the owning resource if there is one
	//
	// +kubebuilder:validation:Optional
	Name string `json:"name"`
}
