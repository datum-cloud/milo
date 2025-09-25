// +kubebuilder:object:generate=true
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// VendorTypeDefinitionSpec defines the desired state of VendorTypeDefinition
// +k8s:protobuf=true
type VendorTypeDefinitionSpec struct {
	// The unique identifier for this vendor type (e.g., "llc", "s-corp", "partnership")
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=^[a-z0-9-]+$
	Code string `json:"code"`

	// Human-readable display name
	// +kubebuilder:validation:Required
	DisplayName string `json:"displayName"`

	// Optional description of this vendor type
	// +optional
	Description string `json:"description,omitempty"`

	// Whether this vendor type is currently available for selection
	// +kubebuilder:validation:Required
	// +kubebuilder:default=true
	Enabled bool `json:"enabled"`

	// Category of vendor type (e.g., "business", "nonprofit", "international")
	// +optional
	Category string `json:"category,omitempty"`

	// Whether this type requires additional business-specific fields
	// +kubebuilder:validation:Required
	// +kubebuilder:default=false
	RequiresBusinessFields bool `json:"requiresBusinessFields"`

	// Whether this type requires tax verification
	// +kubebuilder:validation:Required
	// +kubebuilder:default=true
	RequiresTaxVerification bool `json:"requiresTaxVerification"`

	// Countries where this vendor type is valid (empty means all countries)
	// +optional
	ValidCountries []string `json:"validCountries,omitempty"`

	// Required tax document types for this vendor type
	// +optional
	RequiredTaxDocuments []string `json:"requiredTaxDocuments,omitempty"`
}

// VendorTypeDefinitionStatus defines the observed state of VendorTypeDefinition
// +k8s:protobuf=true
type VendorTypeDefinitionStatus struct {
	// ObservedGeneration is the most recent generation observed for this VendorTypeDefinition by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions represents the observations of a vendor type definition's current state.
	// Known condition types are: "Ready", "Valid"
	// +kubebuilder:default={{type: "Ready", status: "Unknown", reason: "Unknown", message: "Waiting for control plane to reconcile", lastTransitionTime: "1970-01-01T00:00:00Z"}}
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// Number of vendors currently using this type
	// +optional
	VendorCount int32 `json:"vendorCount,omitempty"`

	// Last time this type was used in a vendor
	// +optional
	LastUsed *metav1.Time `json:"lastUsed,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:protobuf=true

// +kubebuilder:subresource:status
// +kubebuilder:resource:path=vendortypedefinitions,scope=Cluster,categories=datum,singular=vendortypedefinition
// +kubebuilder:printcolumn:name="Code",type="string",JSONPath=".spec.code"
// +kubebuilder:printcolumn:name="Display Name",type="string",JSONPath=".spec.displayName"
// +kubebuilder:printcolumn:name="Enabled",type="boolean",JSONPath=".spec.enabled"
// +kubebuilder:printcolumn:name="Category",type="string",JSONPath=".spec.category"
// +kubebuilder:printcolumn:name="Vendor Count",type="integer",JSONPath=".status.vendorCount"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=".metadata.creationTimestamp"
// VendorTypeDefinition is the Schema for the VendorTypeDefinitions API
// +kubebuilder:object:root=true
type VendorTypeDefinition struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:Required
	Spec   VendorTypeDefinitionSpec   `json:"spec,omitempty"`
	Status VendorTypeDefinitionStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:protobuf=true

// +kubebuilder:object:root=true
// VendorTypeDefinitionList contains a list of VendorTypeDefinition
type VendorTypeDefinitionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VendorTypeDefinition `json:"items"`
}
