package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Create conditions
const (
	// DocumentReadyCondition is the condition Type that tracks document creation status.
	DocumentReadyCondition = "Ready"
	// DocumentCreatedReason is used when document creation succeeds.
	DocumentCreatedReason = "CreateSuccessful"
)

// +kubebuilder:validation:Pattern=`^v\d+\.\d+\.\d+$`
type DocumentVersion string

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Document is the Schema for the documents API.
// It represents a document that can be used to create a document revision.
// +kubebuilder:printcolumn:name="Title",type="string",JSONPath=".spec.title"
// +kubebuilder:printcolumn:name="Category",type="string",JSONPath=".metadata.documentMetadata.category"
// +kubebuilder:printcolumn:name="Jurisdiction",type="string",JSONPath=".metadata.documentMetadata.jurisdiction"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Namespaced
type Document struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec     DocumentSpec     `json:"spec,omitempty"`
	Metadata DocumentMetadata `json:"documentMetadata,omitempty"`
	Status   DocumentStatus   `json:"status,omitempty"`
}

// DocumentSpec defines the desired state of Document.
// +kubebuilder:validation:Type=object
type DocumentSpec struct {
	// Title is the title of the Document.
	// +kubebuilder:validation:Required
	Title string `json:"title"`

	// Description is the description of the Document.
	// +kubebuilder:validation:Required
	Description string `json:"description"`

	// DocumentType is the type of the document.
	// +kubebuilder:validation:Required
	DocumentType string `json:"documentType"`
}

// DocumentMetadata defines the metadata of the Document.
// +kubebuilder:validation:Type=object
type DocumentMetadata struct {
	// Category is the category of the Document.
	// +kubebuilder:validation:Required
	Category string `json:"category"`

	// Jurisdiction is the jurisdiction of the Document.
	// +kubebuilder:validation:Required
	Jurisdiction string `json:"jurisdiction"`
}

// +kubebuilder:object:root=true

// DocumentList contains a list of Document.
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type DocumentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Document `json:"items"`
}

// DocumentStatus defines the observed state of Document.
// +kubebuilder:validation:Type=object
type DocumentStatus struct {
	// Conditions represent the latest available observations of an object's current state.
	// +kubebuilder:default={{type: "Ready", status: "Unknown", reason: "Unknown", message: "Waiting for control plane to reconcile", lastTransitionTime: "1970-01-01T00:00:00Z"}}
	// +kubebuilder:validation:Optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// +kubebuilder:validation:Optional
	LatestRevisionRef *LatestRevisionRef `json:"latestRevisionRef,omitempty"`
}

// LatestRevisionRef is a reference to the latest revision of the document.
// +kubebuilder:validation:Type=object
type LatestRevisionRef struct {
	// +kubebuilder:validation:Optional
	Name string `json:"name,omitempty"`
	// +kubebuilder:validation:Optional
	Namespace string `json:"namespace,omitempty"`
	// +kubebuilder:validation:Optional
	Version DocumentVersion `json:"version,omitempty"`
	// +kubebuilder:validation:Optional
	PublishedAt metav1.Time `json:"publishedAt,omitempty"`
}
