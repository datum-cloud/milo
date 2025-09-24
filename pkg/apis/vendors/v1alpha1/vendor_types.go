// +kubebuilder:object:generate=true
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// VendorProfileType defines the type of vendor profile
// +kubebuilder:validation:Enum=person;business
type VendorProfileType string

const (
	VendorProfileTypePerson   VendorProfileType = "person"
	VendorProfileTypeBusiness VendorProfileType = "business"
)

// VendorStatusValue defines the current status of a vendor
// +kubebuilder:validation:Enum=pending;active;rejected;archived
type VendorStatusValue string

const (
	VendorStatusPending  VendorStatusValue = "pending"
	VendorStatusActive   VendorStatusValue = "active"
	VendorStatusRejected VendorStatusValue = "rejected"
	VendorStatusArchived VendorStatusValue = "archived"
)

// VendorType defines the type of vendor
// This should reference a valid vendor type from VendorTypeDefinition
// +kubebuilder:validation:Pattern=^[a-z0-9-]+$
type VendorType string

// TaxIdType defines the type of tax identification
// +kubebuilder:validation:Enum=SSN;EIN;ITIN;UNSPECIFIED
type TaxIdType string

const (
	TaxIdTypeSSN         TaxIdType = "SSN"
	TaxIdTypeEIN         TaxIdType = "EIN"
	TaxIdTypeITIN        TaxIdType = "ITIN"
	TaxIdTypeUnspecified TaxIdType = "UNSPECIFIED"
)

// Address represents a physical address
type Address struct {
	// Street address line 1
	// +kubebuilder:validation:Required
	Street string `json:"street"`

	// Street address line 2 (optional)
	// +optional
	Street2 string `json:"street2,omitempty"`

	// City
	// +kubebuilder:validation:Required
	City string `json:"city"`

	// State or province
	// +kubebuilder:validation:Required
	State string `json:"state"`

	// Postal or ZIP code
	// +kubebuilder:validation:Required
	PostalCode string `json:"postalCode"`

	// Country
	// +kubebuilder:validation:Required
	Country string `json:"country"`
}

// TaxIdReference references a tax ID stored in a Kubernetes Secret
type TaxIdReference struct {
	// Name of the Secret containing the tax ID
	// +kubebuilder:validation:Required
	SecretName string `json:"secretName"`

	// Key within the Secret that contains the tax ID
	// +kubebuilder:validation:Required
	SecretKey string `json:"secretKey"`

	// Namespace of the Secret (if empty, uses the same namespace as the Vendor)
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// TaxInfo represents tax-related information
type TaxInfo struct {
	// Type of tax identification
	// +kubebuilder:validation:Required
	TaxIdType TaxIdType `json:"taxIdType"`

	// Reference to the tax identification number stored in a Secret
	// +kubebuilder:validation:Required
	TaxIdRef TaxIdReference `json:"taxIdRef"`

	// Country for tax purposes
	// +kubebuilder:validation:Required
	Country string `json:"country"`

	// Tax document reference (e.g., W-9, W-8BEN)
	// +kubebuilder:validation:Required
	TaxDocument string `json:"taxDocument"`
}

// VendorSpec defines the desired state of Vendor
// +k8s:protobuf=true
type VendorSpec struct {
	// Profile type - person or business
	// +kubebuilder:validation:Required
	ProfileType VendorProfileType `json:"profileType"`

	// Legal name of the vendor (required)
	// +kubebuilder:validation:Required
	LegalName string `json:"legalName"`

	// Nickname or display name
	// +optional
	Nickname string `json:"nickname,omitempty"`

	// Billing address
	// +kubebuilder:validation:Required
	BillingAddress Address `json:"billingAddress"`

	// Mailing address (if different from billing)
	// +optional
	MailingAddress *Address `json:"mailingAddress,omitempty"`

	// Description of the vendor
	// +optional
	Description string `json:"description,omitempty"`

	// Website URL
	// +optional
	Website string `json:"website,omitempty"`

	// Business-specific fields (only applicable when profileType is business)
	// +optional
	VendorType VendorType `json:"vendorType,omitempty"`

	// Doing business as name
	// +optional
	CorporationDBA string `json:"corporationDBA,omitempty"`

	// Registration number (optional)
	// +optional
	RegistrationNumber string `json:"registrationNumber,omitempty"`

	// State of incorporation
	// +optional
	StateOfIncorporation string `json:"stateOfIncorporation,omitempty"`

	// Tax information
	// +kubebuilder:validation:Required
	TaxInfo TaxInfo `json:"taxInfo"`
}

// VendorStatus defines the observed state of Vendor
// +k8s:protobuf=true
type VendorStatus struct {
	// ObservedGeneration is the most recent generation observed for this Vendor by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions represents the observations of a vendor's current state.
	// Known condition types are: "Ready", "Validated", "Verified", "Active"
	// +kubebuilder:default={{type: "Ready", status: "Unknown", reason: "Unknown", message: "Waiting for control plane to reconcile", lastTransitionTime: "1970-01-01T00:00:00Z"}}
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// Current status of the vendor
	// +optional
	Status VendorStatusValue `json:"status,omitempty"`

	// Overall verification status
	// +optional
	VerificationStatus VerificationStatus `json:"verificationStatus,omitempty"`

	// Number of required verifications
	// +optional
	RequiredVerifications int32 `json:"requiredVerifications,omitempty"`

	// Number of completed verifications
	// +optional
	CompletedVerifications int32 `json:"completedVerifications,omitempty"`

	// Number of pending verifications
	// +optional
	PendingVerifications int32 `json:"pendingVerifications,omitempty"`

	// Number of rejected verifications
	// +optional
	RejectedVerifications int32 `json:"rejectedVerifications,omitempty"`

	// Number of expired verifications
	// +optional
	ExpiredVerifications int32 `json:"expiredVerifications,omitempty"`

	// Timestamp when vendor was last verified
	// +optional
	LastVerifiedAt *metav1.Time `json:"lastVerifiedAt,omitempty"`

	// Timestamp when vendor was activated
	// +optional
	ActivatedAt *metav1.Time `json:"activatedAt,omitempty"`

	// Timestamp when vendor was rejected
	// +optional
	RejectedAt *metav1.Time `json:"rejectedAt,omitempty"`

	// Reason for rejection (if applicable)
	// +optional
	RejectionReason string `json:"rejectionReason,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:protobuf=true

// +kubebuilder:subresource:status
// +kubebuilder:resource:path=vendors,scope=Cluster,categories=datum,singular=vendor
// +kubebuilder:printcolumn:name="Legal Name",type="string",JSONPath=".spec.legalName"
// +kubebuilder:printcolumn:name="Profile Type",type="string",JSONPath=".spec.profileType"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.status"
// +kubebuilder:printcolumn:name="Verification",type="string",JSONPath=".status.verificationStatus"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=".metadata.creationTimestamp"
// Vendor is the Schema for the Vendors API
// +kubebuilder:object:root=true
type Vendor struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:Required
	Spec   VendorSpec   `json:"spec,omitempty"`
	Status VendorStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:protobuf=true

// +kubebuilder:object:root=true
// VendorList contains a list of Vendor
type VendorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Vendor `json:"items"`
}
