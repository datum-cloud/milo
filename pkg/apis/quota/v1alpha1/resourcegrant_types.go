package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Bucket struct {
	// Amount of the resource type being granted
	//
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Required
	Amount int64 `json:"amount"`
	// Dimension selector for this allowance using Kubernetes LabelSelector
	//
	// +kubebuilder:validation:Optional
	DimensionSelector metav1.LabelSelector `json:"dimensionSelector,omitempty"`
}

// Allowance defines a single resource allowance within a grant
type Allowance struct {
	// Fully qualified name of the resource type being granted
	//
	// +kubebuilder:validation:Required
	ResourceTypeName string `json:"resourceTypeName"`
	// List of buckets this allowance contains
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Buckets []Bucket `json:"buckets"`
}

// ResourceGrantSpec defines the desired state of ResourceGrant.
type ResourceGrantSpec struct {
	// Reference to the owning resource of the grant
	//
	// +kubebuilder:validation:Required
	OwnerRef OwnerRef `json:"ownerRef"`
	// List of allowances this grant contains
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Allowances []Allowance `json:"allowances"`
}

// ResourceGrantStatus defines the observed state of ResourceGrant.
type ResourceGrantStatus struct {
	// Most recent generation observed.
	//
	// +kubebuilder:validation:Optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Known condition types: "Active"
	// +kubebuilder:validation:XValidation:rule="self.all(c, c.type == 'Active' ? c.reason in ['GrantActive', 'ValidationFailed', 'GrantPending'] : true)",message="Active condition reason must be valid"
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

const (
	// Indicates that the resource grant is active and available for usage.
	ResourceGrantActive = "Active"
)

const (
	// Indicates the ResourceGrant is active and its
	// allowances will be taken into account in claim evaluation.
	ResourceGrantActiveReason = "GrantActive"
	// Indicates that the status update validation failed.
	ResourceGrantValidationFailedReason = "ValidationFailed"
	// Indicates that the grant is pending activation.
	ResourceGrantPendingReason = "GrantPending"
)

// ResourceGrant is the Schema for the resourcegrants API.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Active",type="string",JSONPath=".status.conditions[?(@.type=='Active')].status"
// +k8s:openapi-gen=true
type ResourceGrant struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// +kubebuilder:validation:Required
	Spec   ResourceGrantSpec   `json:"spec"`
	Status ResourceGrantStatus `json:"status,omitempty"`
}

// ResourceGrantList contains a list of ResourceGrant.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
type ResourceGrantList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ResourceGrant `json:"items"`
}
