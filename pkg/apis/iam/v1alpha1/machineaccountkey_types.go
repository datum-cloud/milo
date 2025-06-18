package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:object:root=true

// MachineAccountKey is the Schema for the machineaccountkeys API
// +k8s:openapi-gen=true
// +kubebuilder:printcolumn:name="Machine Account",type="string",JSONPath=".spec.ownerRef.name"
// +kubebuilder:printcolumn:name="Expiration Date",type="string",JSONPath=".spec.expirationDate"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Namespaced
type MachineAccountKey struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MachineAccountKeySpec   `json:"spec,omitempty"`
	Status MachineAccountKeyStatus `json:"status,omitempty"`
}

// MachineAccountKeySpec defines the desired state of MachineAccountKey
// +k8s:openapi-gen=true
type MachineAccountKeySpec struct {
	// OwnerRef is a reference to the resource that owns the MachineAccountKey.
	// MachineAccountKey is a namespaced resource.
	// +kubebuilder:validation:Required
	OwnerRef OwnerReference `json:"ownerRef"`

	// ExpirationDate is the date and time when the MachineAccountKey will expire.
	// If not specified, the MachineAccountKey will never expire.
	// +kubebuilder:validation:Optional
	ExpirationDate *metav1.Time `json:"expirationDate,omitempty"`

	// PublicKey is the public key of the MachineAccountKey.
	// If not specified, the MachineAccountKey will be created with an auto-generated public key.
	// +kubebuilder:validation:Optional
	PublicKey string `json:"publicKey,omitempty"`
}

// MachineAccountKeyStatus defines the observed state of MachineAccountKey
// +k8s:openapi-gen=true
type MachineAccountKeyStatus struct {
	// AuthProviderKeyID is the unique identifier for the key in the auth provider.
	// This field is populated by the controller after the key is created in the auth provider.
	// +kubebuilder:validation:Optional
	AuthProviderKeyID string `json:"authProviderKeyId,omitempty"`

	// Conditions provide conditions that represent the current status of the MachineAccountKey.
	// +kubebuilder:default={{type: "Ready", status: "Unknown", reason: "Unknown", message: "Waiting for control plane to reconcile", lastTransitionTime: "1970-01-01T00:00:00Z"}}
	// +kubebuilder:validation:Optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// OwnerReference contains information that points to the MachineAccount being referenced.
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
	// +kubebuilder:validation:Enum=MachineAccount
	Kind string `json:"kind"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true

// MachineAccountKeyList contains a list of MachineAccountKey
type MachineAccountKeyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MachineAccountKey `json:"items"`
}
