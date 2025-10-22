package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// Create conditions
const (
	// PlatformAccessRejectionReadyCondition is the condition Type that tracks platform access rejection creation status.
	PlatformAccessRejectionReadyCondition = "Ready"
	// PlatformAccessRejectionReconciledReason is used when platform access rejection reconciliation succeeds.
	PlatformAccessRejectionReconciledReason = "Reconciled"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// PlatformAccessRejection is the Schema for the platformaccessrejections API.
// It represents a formal denial of platform access for a user. Once the rejection is created, a notification can be sent to the user.
// +kubebuilder:resource:scope=Cluster
type PlatformAccessRejection struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PlatformAccessRejectionSpec   `json:"spec,omitempty"`
	Status PlatformAccessRejectionStatus `json:"status,omitempty"`
}

// PlatformAccessRejectionSpec defines the desired state of PlatformAccessRejection.
// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="spec is immutable"
// +kubebuilder:validation:Type=object
type PlatformAccessRejectionSpec struct {
	// SubjectRef is the reference to the subject being rejected.
	// +kubebuilder:validation:Required
	SubjectRef SubjectReference `json:"subjectRef"`
	// RejecterRef is the reference to the actor who issued the rejection.
	// If not specified, the rejection was made by the system.
	// +kubebuilder:validation:Optional
	RejecterRef *UserReference `json:"rejecterRef,omitempty"`
}

// +kubebuilder:object:root=true

// PlatformAccessRejectionList contains a list of PlatformAccessRejection.
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type PlatformAccessRejectionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PlatformAccessRejection `json:"items"`
}

type PlatformAccessRejectionStatus struct {
	// Conditions provide conditions that represent the current status of the PlatformAccessRejection.
	// +kubebuilder:default={{type: "Ready", status: "Unknown", reason: "ReconcilePending", message: "Platform access rejection reconciliation is pending", lastTransitionTime: "1970-01-01T00:00:00Z"}}
	// +kubebuilder:validation:Optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// RejecterUser contains information about the user who rejected the access request.
	// +kubebuilder:validation:Optional
	RejecterUser *PlatformAccessRejectionUserStatus `json:"rejecterUser,omitempty"`
}

type PlatformAccessRejectionUserStatus struct {
	// Email is the email of the User being referenced.
	// +kubebuilder:validation:Optional
	Email string `json:"email,omitempty"`
}
