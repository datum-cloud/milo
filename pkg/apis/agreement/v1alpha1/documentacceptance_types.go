package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	documentationmiloapiscomv1alpha1 "go.miloapis.com/milo/pkg/apis/documentation/v1alpha1"
)

// Create conditions
const (
	// DocumentAcceptanceReadyCondition is the condition Type that tracks document acceptance status.
	DocumentAcceptanceReadyCondition = "Ready"
	// DocumentAcceptanceCreatedReason is used when document creation succeeds.
	DocumentAcceptanceCreatedReason = "CreateSuccessful"
)

// ResourceReference contains information that points to the Resource being referenced.
// +kubebuilder:validation:Type=object
type ResourceReference struct {
	// APIGroup is the group for the resource being referenced.
	// +kubebuilder:validation:Required
	APIGroup string `json:"apiGroup"`

	// Kind is the type of resource being referenced.
	// +kubebuilder:validation:Required
	Kind string `json:"kind"`

	// Name is the name of the Resource being referenced.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Namespace is the namespace of the Resource being referenced.
	// +kubebuilder:validation:Optional
	Namespace string `json:"namespace,omitempty"`
}

// DocumentAcceptanceContext contains the context of the document acceptance.
// +kubebuilder:validation:Type=object
type DocumentAcceptanceContext struct {
	// Method is the method of the document acceptance.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=web;email;cli
	Method string `json:"method"`

	// IPAddress is the IP address of the accepter.
	// +kubebuilder:validation:Optional
	IPAddress string `json:"ipAddress,omitempty"`

	// UserAgent is the user agent of the accepter.
	// +kubebuilder:validation:Optional
	UserAgent string `json:"userAgent,omitempty"`

	// AcceptanceLanguage is the language of the document acceptance.
	// +kubebuilder:validation:Optional
	AcceptanceLanguage string `json:"acceptanceLanguage,omitempty"`
}

// DocumentAcceptanceSignature contains the signature of the document acceptance.
// +kubebuilder:validation:Type=object
type DocumentAcceptanceSignature struct {
	// Type specifies the signature mechanism used for the document acceptance.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=checkbox
	Type string `json:"type"`

	// Timestamp is the timestamp of the document acceptance.
	// +kubebuilder:validation:Required
	Timestamp metav1.Time `json:"timestamp"`
}

// DocumentAcceptanceSpec defines the desired state of DocumentAcceptance.
// +kubebuilder:validation:Type=object
// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="spec is immutable"
type DocumentAcceptanceSpec struct {
	// DocumentRevisionRef is a reference to the document revision that is being accepted.
	// +kubebuilder:validation:Required
	DocumentRevisionRef documentationmiloapiscomv1alpha1.DocumentRevisionReference `json:"documentRevisionRef"`

	// SubjectRef is a reference to the subject that this document acceptance applies to.
	// +kubebuilder:validation:Required
	SubjectRef ResourceReference `json:"subjectRef"`

	// AccepterRef is a reference to the accepter that this document acceptance applies to.
	// +kubebuilder:validation:Required
	AccepterRef ResourceReference `json:"accepterRef"`

	// AcceptanceContext is the context of the document acceptance.
	// +kubebuilder:validation:Required
	AcceptanceContext DocumentAcceptanceContext `json:"acceptanceContext"`

	// Signature is the signature of the document acceptance.
	// +kubebuilder:validation:Required
	Signature DocumentAcceptanceSignature `json:"signature"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// DocumentAcceptance is the Schema for the documentacceptances API.
// It represents a document acceptance.
// +kubebuilder:resource:scope=Namespaced
type DocumentAcceptance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DocumentAcceptanceSpec   `json:"spec,omitempty"`
	Status DocumentAcceptanceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DocumentAcceptanceList contains a list of DocumentAcceptance.
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type DocumentAcceptanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DocumentAcceptance `json:"items"`
}

// DocumentAcceptanceStatus defines the observed state of DocumentAcceptance.
// +kubebuilder:validation:Type=object
type DocumentAcceptanceStatus struct {
	// Conditions represent the latest available observations of an object's current state.
	// +kubebuilder:default={{type: "Ready", status: "Unknown", reason: "Unknown", message: "Waiting for control plane to reconcile", lastTransitionTime: "1970-01-01T00:00:00Z"}}
	// +kubebuilder:validation:Optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}
