package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:object:root=true
// +kubebuilder:storageversion

// MachineAccountKey is the Schema for the machineaccountkeys API
// +kubebuilder:printcolumn:name="Machine Account",type="string",JSONPath=".spec.machineAccountName"
// +kubebuilder:printcolumn:name="Expiration Date",type="string",JSONPath=".spec.expirationDate"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:selectablefield:JSONPath=".spec.machineAccountName"
// +kubebuilder:resource:scope=Namespaced
type MachineAccountKey struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MachineAccountKeySpec   `json:"spec,omitempty"`
	Status MachineAccountKeyStatus `json:"status,omitempty"`
}

// MachineAccountKeySpec defines the desired state of MachineAccountKey
type MachineAccountKeySpec struct {
	// MachineAccountName is the name of the MachineAccount that owns this key.
	// +kubebuilder:validation:Required
	MachineAccountName string `json:"machineAccountName"`

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
type MachineAccountKeyStatus struct {
	// AuthProviderKeyID is the unique identifier for the key in the auth provider.
	// This field is populated by the controller after the key is created in the auth provider.
	// For example, when using Zitadel, a typical value might be: "326102453042806786"
	AuthProviderKeyID string `json:"authProviderKeyID,omitempty"`

	// PrivateKey contains the PEM-encoded RSA private key generated during resource
	// creation. This field is populated only in the creation response and is never
	// persisted to etcd. Any value present on a GET or LIST response indicates a
	// bug in the server implementation.
	//
	// Note: private key material will appear in API server audit logs for creation
	// events. This matches the behavior of similar systems (GCP service account keys).
	//
	// +kubebuilder:validation:Optional
	PrivateKey string `json:"privateKey,omitempty"`

	// Conditions provide conditions that represent the current status of the MachineAccountKey.
	// +kubebuilder:default={{type: "Ready", status: "Unknown", reason: "Unknown", message: "Waiting for control plane to reconcile", lastTransitionTime: "1970-01-01T00:00:00Z"}}
	// +kubebuilder:validation:Optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true

// MachineAccountKeyList contains a list of MachineAccountKey
type MachineAccountKeyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MachineAccountKey `json:"items"`
}
