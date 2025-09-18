// +kubebuilder:object:generate=true
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CorporationTypeConfigSpec defines the desired state of CorporationTypeConfig
// +k8s:protobuf=true
type CorporationTypeConfigSpec struct {
	// Available corporation types that can be selected for vendors
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	CorporationTypes []CorporationTypeDefinition `json:"corporationTypes"`

	// Whether this configuration is active
	// +kubebuilder:validation:Required
	// +kubebuilder:default=true
	Active bool `json:"active"`
}

// CorporationTypeDefinition defines a single corporation type option
type CorporationTypeDefinition struct {
	// The unique identifier for this corporation type
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=^[a-z0-9-]+$
	Code string `json:"code"`

	// Human-readable display name
	// +kubebuilder:validation:Required
	DisplayName string `json:"displayName"`

	// Optional description of this corporation type
	// +optional
	Description string `json:"description,omitempty"`

	// Whether this corporation type is currently available for selection
	// +kubebuilder:validation:Required
	// +kubebuilder:default=true
	Enabled bool `json:"enabled"`

	// Sort order for display purposes (lower numbers appear first)
	// +kubebuilder:validation:Required
	// +kubebuilder:default=100
	SortOrder int32 `json:"sortOrder"`
}

// CorporationTypeConfigStatus defines the observed state of CorporationTypeConfig
// +k8s:protobuf=true
type CorporationTypeConfigStatus struct {
	// ObservedGeneration is the most recent generation observed for this CorporationTypeConfig by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions represents the observations of a corporation type config's current state.
	// Known condition types are: "Ready"
	// +kubebuilder:default={{type: "Ready", status: "Unknown", reason: "Unknown", message: "Waiting for control plane to reconcile", lastTransitionTime: "1970-01-01T00:00:00Z"}}
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// Number of active corporation types
	// +optional
	ActiveTypeCount int32 `json:"activeTypeCount,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:protobuf=true

// +kubebuilder:subresource:status
// +kubebuilder:resource:path=corporationtypeconfigs,scope=Cluster,categories=datum,singular=corporationtypeconfig
// +kubebuilder:printcolumn:name="Active",type="boolean",JSONPath=".spec.active"
// +kubebuilder:printcolumn:name="Type Count",type="integer",JSONPath=".status.activeTypeCount"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=".metadata.creationTimestamp"
// CorporationTypeConfig is the Schema for the CorporationTypeConfigs API
// +kubebuilder:object:root=true
type CorporationTypeConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:Required
	Spec   CorporationTypeConfigSpec   `json:"spec,omitempty"`
	Status CorporationTypeConfigStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:protobuf=true

// +kubebuilder:object:root=true
// CorporationTypeConfigList contains a list of CorporationTypeConfig
type CorporationTypeConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CorporationTypeConfig `json:"items"`
}
