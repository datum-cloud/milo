// +kubebuilder:object:generate=true
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// VerificationType defines the type of verification being performed
// +kubebuilder:validation:Enum=tax;business;identity;compliance;other
type VerificationType string

const (
	VerificationTypeTax        VerificationType = "tax"
	VerificationTypeBusiness   VerificationType = "business"
	VerificationTypeIdentity   VerificationType = "identity"
	VerificationTypeCompliance VerificationType = "compliance"
	VerificationTypeOther      VerificationType = "other"
)

// VerificationStatus defines the current status of a verification
// +kubebuilder:validation:Enum=pending;in-progress;approved;rejected;expired
type VerificationStatus string

const (
	VerificationStatusPending    VerificationStatus = "pending"
	VerificationStatusInProgress VerificationStatus = "in-progress"
	VerificationStatusApproved   VerificationStatus = "approved"
	VerificationStatusRejected   VerificationStatus = "rejected"
	VerificationStatusExpired    VerificationStatus = "expired"
)

// VendorReference references a Vendor resource
type VendorReference struct {
	// Name of the Vendor resource
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Namespace of the Vendor resource (if empty, uses the same namespace as the VendorVerification)
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// VerifierReference references who performed the verification
type VerifierReference struct {
	// Type of verifier (user, system, external-service, etc.)
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=user;system;external-service;admin
	Type string `json:"type"`

	// Name of the verifier (username, service name, etc.)
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Additional metadata about the verifier
	// +optional
	Metadata map[string]string `json:"metadata,omitempty"`
}

// VerificationDocument represents a document used in verification
type VerificationDocument struct {
	// Type of document (W-9, W-8BEN, business-license, etc.)
	// +kubebuilder:validation:Required
	Type string `json:"type"`

	// Reference to the document (secret name, file path, etc.)
	// +kubebuilder:validation:Required
	Reference string `json:"reference"`

	// Document version or identifier
	// +optional
	Version string `json:"version,omitempty"`

	// Document expiration date
	// +optional
	ExpirationDate *metav1.Time `json:"expirationDate,omitempty"`

	// Whether the document is valid
	// +kubebuilder:default=true
	Valid bool `json:"valid"`
}

// VendorVerificationSpec defines the desired state of VendorVerification
// +k8s:protobuf=true
type VendorVerificationSpec struct {
	// Reference to the vendor being verified
	// +kubebuilder:validation:Required
	VendorRef VendorReference `json:"vendorRef"`

	// Type of verification being performed
	// +kubebuilder:validation:Required
	VerificationType VerificationType `json:"verificationType"`

	// Current status of the verification
	// +kubebuilder:validation:Required
	// +kubebuilder:default=pending
	Status VerificationStatus `json:"status"`

	// Description of what is being verified
	// +optional
	Description string `json:"description,omitempty"`

	// Documents used in this verification
	// +optional
	Documents []VerificationDocument `json:"documents,omitempty"`

	// Reference to who is performing the verification
	// +optional
	VerifierRef *VerifierReference `json:"verifierRef,omitempty"`

	// Additional notes or comments about the verification
	// +optional
	Notes string `json:"notes,omitempty"`

	// Priority of this verification (1-10, higher is more urgent)
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=10
	// +kubebuilder:default=5
	Priority int32 `json:"priority"`

	// Whether this verification is required for vendor activation
	// +kubebuilder:default=true
	Required bool `json:"required"`

	// Expiration date for this verification
	// +optional
	ExpirationDate *metav1.Time `json:"expirationDate,omitempty"`

	// External system reference (if verification is done by external service)
	// +optional
	ExternalReference string `json:"externalReference,omitempty"`
}

// VendorVerificationStatus defines the observed state of VendorVerification
// +k8s:protobuf=true
type VendorVerificationStatus struct {
	// ObservedGeneration is the most recent generation observed for this VendorVerification by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions represents the observations of a vendor verification's current state.
	// Known condition types are: "Ready", "Valid", "Expired"
	// +kubebuilder:default={{type: "Ready", status: "Unknown", reason: "Unknown", message: "Waiting for control plane to reconcile", lastTransitionTime: "1970-01-01T00:00:00Z"}}
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// Timestamp when verification was completed
	// +optional
	CompletedAt *metav1.Time `json:"completedAt,omitempty"`

	// Timestamp when verification was last updated
	// +optional
	LastUpdatedAt *metav1.Time `json:"lastUpdatedAt,omitempty"`

	// Number of verification attempts
	// +optional
	AttemptCount int32 `json:"attemptCount,omitempty"`

	// Last error message if verification failed
	// +optional
	LastError string `json:"lastError,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:protobuf=true

// +kubebuilder:subresource:status
// +kubebuilder:resource:path=vendorverifications,scope=Cluster,categories=datum,singular=vendorverification
// +kubebuilder:printcolumn:name="Vendor",type="string",JSONPath=".spec.vendorRef.name"
// +kubebuilder:printcolumn:name="Type",type="string",JSONPath=".spec.verificationType"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".spec.status"
// +kubebuilder:printcolumn:name="Verifier",type="string",JSONPath=".spec.verifierRef.name"
// +kubebuilder:printcolumn:name="Required",type="boolean",JSONPath=".spec.required"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=".metadata.creationTimestamp"
// VendorVerification is the Schema for the VendorVerifications API
// +kubebuilder:object:root=true
type VendorVerification struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:Required
	Spec   VendorVerificationSpec   `json:"spec,omitempty"`
	Status VendorVerificationStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:protobuf=true

// +kubebuilder:object:root=true
// VendorVerificationList contains a list of VendorVerification
type VendorVerificationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VendorVerification `json:"items"`
}
