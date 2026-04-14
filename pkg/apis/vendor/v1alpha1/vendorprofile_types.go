// +kubebuilder:object:generate=true
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// VendorProfileSpec defines the desired state of VendorProfile.
type VendorProfileSpec struct {
	// Human-readable vendor name (e.g., "Amazon Web Services").
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=256
	DisplayName string `json:"displayName"`

	// Markdown-formatted description of the vendor and its role as a
	// sub-processor.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=5000
	Description string `json:"description"`

	// URL to the vendor's logo image.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MaxLength=2048
	LogoURL string `json:"logoURL,omitempty"`

	// Vendor's primary website.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MaxLength=2048
	WebsiteURL string `json:"websiteURL,omitempty"`

	// Link to the vendor's privacy policy.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MaxLength=2048
	PrivacyPolicyURL string `json:"privacyPolicyURL,omitempty"`

	// Type of service the vendor provides.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=Infrastructure;Security;Analytics;Authentication;DataProcessing;Communication;Monitoring;Other
	Category string `json:"category"`

	// Describes what data this vendor processes and why.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=1000
	Purpose string `json:"purpose"`

	// Countries or regions where the vendor processes data (e.g., "US", "EU").
	// +kubebuilder:validation:Optional
	DataProcessingLocations []string `json:"dataProcessingLocations,omitempty"`

	// Vendor contact email address.
	// +kubebuilder:validation:Optional
	ContactEmail string `json:"contactEmail,omitempty"`
}

// VendorProfileStatus defines the observed state of VendorProfile.
type VendorProfileStatus struct {
	// ObservedGeneration is the most recent generation observed for this
	// VendorProfile by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions represents the observations of a vendor profile's current state.
	// Known condition types are: "Ready"
	// +kubebuilder:default={{type: "Ready", status: "Unknown", reason: "Unknown", message: "Waiting for control plane to reconcile", lastTransitionTime: "1970-01-01T00:00:00Z"}}
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// +kubebuilder:subresource:status
// +kubebuilder:resource:path=vendorprofiles,scope=Cluster,categories=datum,singular=vendorprofile
// +kubebuilder:printcolumn:name="Display Name",type="string",JSONPath=".spec.displayName"
// +kubebuilder:printcolumn:name="Category",type="string",JSONPath=".spec.category"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=".metadata.creationTimestamp"
// VendorProfile is the Schema for the VendorProfiles API.
// It represents a third-party vendor or sub-processor for compliance
// documentation.
// +kubebuilder:object:root=true
type VendorProfile struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:Required
	Spec   VendorProfileSpec   `json:"spec,omitempty"`
	Status VendorProfileStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// +kubebuilder:object:root=true
// VendorProfileList contains a list of VendorProfile.
type VendorProfileList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VendorProfile `json:"items"`
}
