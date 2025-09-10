package v1alpha1

// Allocation represents a single allocation of quota from a bucket to a claim.
type Allocation struct {
	// Reference to the claim's specific object instance.
	//
	// +kubebuilder:validation:Required
	ClaimRef OwnerInstanceRef `json:"claimRef"`
	// The amount of quota allocated.
	///
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=0
	Amount int64 `json:"amount"`
}
