package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:object:root=true

// MachineAccount is the Schema for the machine accounts API
// +k8s:openapi-gen=true
// +kubebuilder:printcolumn:name="Email",type="string",JSONPath=".spec.email"
// +kubebuilder:printcolumn:name="Project Name",type="string",JSONPath=".spec.ownerRef.name"
// +kubebuilder:printcolumn:name="Description",type="string",JSONPath=".spec.description"
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
// +k8s:openapi-gen=true
type MachineAccountSpec struct {
	// OwnerRef is a reference to the Project where the machine will be used.
	// Project is a cluster-scoped resource.
	// +kubebuilder:validation:Required
	OwnerRef OwnerReference `json:"ownerRef"`

	// The email of the machine account.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=200
	// +kubebuilder:validation:Format=email
	Email string `json:"email"`

	// The description of the machine account.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MaxLength=500
	Description string `json:"description,omitempty"`

	// The access token type for the machine account.
	// JWT is the only supported access token type at the moment.
	// +kubebuilder:validation:Enum=jwt
	// +kubebuilder:default=jwt
	// +kubebuilder:validation:Optional
	AccessTokenType string `json:"accessTokenType,omitempty"`

	// The state of the machine account.
	// +kubebuilder:validation:Enum=Active;Inactive
	// +kubebuilder:default=Active
	// +kubebuilder:validation:Optional
	State string `json:"state,omitempty"`
}

// MachineAccountStatus defines the observed state of MachineAccount
// +k8s:openapi-gen=true
type MachineAccountStatus struct {
	// Conditions provide conditions that represent the current status of the MachineAccount.
	// +kubebuilder:default={{type: "Ready", status: "Unknown", reason: "Unknown", message: "Waiting for control plane to reconcile", lastTransitionTime: "1970-01-01T00:00:00Z"}}
	// +kubebuilder:validation:Optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// OwnerReference contains information that points to the Project being referenced.
// Project is a cluster-scoped resource, so Namespace is not needed.
// +k8s:openapi-gen=true
type OwnerReference struct {
	// Name is the name of the resource being referenced.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// UID is the UID of the resource being referenced.
	// +kubebuilder:validation:Required
	UID string `json:"uid"`

	// Kind is the kind of the resource.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=Project
	Kind string `json:"kind"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true

// MachineAccountList contains a list of MachineAccount
// +k8s:openapi-gen=true
type MachineAccountList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MachineAccount `json:"items"`
}
