package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:object:root=true

// MachineAccount is the Schema for the machine accounts API
// +kubebuilder:printcolumn:name="Email",type="string",JSONPath=".status.email"
// +kubebuilder:printcolumn:name="Description",type="string",JSONPath=".metadata.annotations['kubernetes\\.io/description']"
// +kubebuilder:printcolumn:name="State",type="string",JSONPath=".spec.state"
// +kubebuilder:printcolumn:name="Access Token Type",type="string",JSONPath=".spec.accessTokenType"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Namespaced
type MachineAccount struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MachineAccountSpec   `json:"spec,omitempty"`
	Status MachineAccountStatus `json:"status,omitempty"`
}

// MachineAccountSpec defines the desired state of MachineAccount
type MachineAccountSpec struct {
	// The state of the machine account. This state can be safely changed as needed.
	// States:
	//   - Active: The machine account can be used to authenticate.
	//   - Inactive: The machine account is prohibited to be used to authenticate, and revokes all existing sessions.
	// +kubebuilder:validation:Enum=Active;Inactive
	// +kubebuilder:default=Active
	// +kubebuilder:validation:Optional
	State string `json:"state,omitempty"`
}

// MachineAccountStatus defines the observed state of MachineAccount
// +k8s:openapi-gen=false
type MachineAccountStatus struct {
	// The computed email of the machine account following the pattern:
	// {metadata.name}@{metadata.namespace}.{project.metadata.name}.{global-suffix}
	Email string `json:"email,omitempty"`

	// State represents the current activation state of the machine account from the auth provider.
	// This field tracks the state from the previous generation and is updated when state changes
	// are successfully propagated to the auth provider. It helps optimize performance by only
	// updating the auth provider when a state change is detected.
	// +kubebuilder:validation:Enum=Active;Inactive
	State string `json:"state,omitempty"`

	// Conditions provide conditions that represent the current status of the MachineAccount.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true

// MachineAccountList contains a list of MachineAccount
type MachineAccountList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MachineAccount `json:"items"`
}
