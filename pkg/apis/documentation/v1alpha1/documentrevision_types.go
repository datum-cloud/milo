package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Create conditions
const (
	// DocumentRevisionReadyCondition is the condition Type that tracks document revision creation status.
	DocumentRevisionReadyCondition = "Ready"
	// DocumentRevisionCreatedReason is used when document revision creation succeeds.
	DocumentRevisionCreatedReason = "CreateSuccessful"
)

// DocumentReference contains information that points to the Document being referenced.
// Document is a namespaced resource.
// +kubebuilder:validation:Type=object
type DocumentReference struct {
	// Name is the name of the Document being referenced.
	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// Namespace of the referenced Document.
	// +kubebuilder:validation:Required
	Namespace string `json:"namespace"`
}

// DocumentRevisionContent contains the content of the document revision.
// +kubebuilder:validation:Type=object
type DocumentRevisionContent struct {
	// Format is the format of the document revision.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=html;markdown
	Format string `json:"format"`

	// Data is the data of the document revision.
	// +kubebuilder:validation:Required
	Data string `json:"data"`
}

// DocumentRevisionExpectedSubjectKind is the kind of the resource that is expected to reference this revision.
// +kubebuilder:validation:Type=object
type DocumentRevisionExpectedSubjectKind struct {
	// APIGroup is the group for the resource being referenced.
	// +kubebuilder:validation:Required
	APIGroup string `json:"apiGroup"`

	// Kind is the type of resource being referenced.
	// +kubebuilder:validation:Required
	Kind string `json:"kind"`
}

// DocumentRevisionExpectedAccepterKind is the kind of the resource that is expected to accept this revision.
// +kubebuilder:validation:Type=object
type DocumentRevisionExpectedAccepterKind struct {
	// APIGroup is the group for the resource being referenced.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="self == 'iam.miloapis.com'",message="apiGroup must be iam.miloapis.com"
	APIGroup string `json:"apiGroup"`

	// Kind is the type of resource being referenced.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=User;MachineAccount
	Kind string `json:"kind"`
}

// DocumentRevisionSpec defines the desired state of DocumentRevision.
// +kubebuilder:validation:Type=object
// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="spec is immutable"
type DocumentRevisionSpec struct {
	// DocumentRef is a reference to the document that this revision is based on.
	// +kubebuilder:validation:Required
	DocumentRef DocumentReference `json:"documentRef"`

	// Version is the version of the document revision.
	// +kubebuilder:validation:Required
	Version DocumentVersion `json:"version"`

	// Content is the content of the document revision.
	// +kubebuilder:validation:Required
	Content DocumentRevisionContent `json:"content"`

	// EffectiveDate is the date in which the document revision starts to be effective.
	// +kubebuilder:validation:Required
	EffectiveDate metav1.Time `json:"effectiveDate"`

	// ChangesSummary is the summary of the changes in the document revision.
	// +kubebuilder:validation:Required
	ChangesSummary string `json:"changesSummary"`

	// ExpectedSubjectKinds is the resource kinds that this revision affects to.
	// +kubebuilder:validation:Required
	ExpectedSubjectKinds []DocumentRevisionExpectedSubjectKind `json:"expectedSubjectKinds"`

	// ExpectedAccepterKinds is the resource kinds that are expected to accept this revision.
	// +kubebuilder:validation:Required
	ExpectedAccepterKinds []DocumentRevisionExpectedAccepterKind `json:"expectedAccepterKinds"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// DocumentRevision is the Schema for the documentrevisions API.
// It represents a revision of a document.
// +kubebuilder:resource:scope=Namespaced
type DocumentRevision struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DocumentRevisionSpec   `json:"spec,omitempty"`
	Status DocumentRevisionStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DocumentRevisionList contains a list of DocumentRevision.
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type DocumentRevisionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DocumentRevision `json:"items"`
}

// DocumentRevisionStatus defines the observed state of DocumentRevision.
// +kubebuilder:validation:Type=object
type DocumentRevisionStatus struct {
	// Conditions represent the latest available observations of an object's current state.
	// +kubebuilder:default={{type: "Ready", status: "Unknown", reason: "Unknown", message: "Waiting for control plane to reconcile", lastTransitionTime: "1970-01-01T00:00:00Z"}}
	// +kubebuilder:validation:Optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ContentHash is the hash of the content of the document revision.
	// This is used to detect if the content of the document revision has changed.
	// +kubebuilder:validation:Optional
	ContentHash string `json:"contentHash,omitempty"`
}

// DocumentRevisionReference contains information that points to the DocumentRevision being referenced.
// +kubebuilder:validation:Type=object
type DocumentRevisionReference struct {
	// Name is the name of the DocumentRevision being referenced.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Namespace of the referenced document revision.
	// +kubebuilder:validation:Required
	Namespace string `json:"namespace"`

	// Version is the version of the DocumentRevision being referenced.
	// +kubebuilder:validation:Required
	Version DocumentVersion `json:"version"`
}
