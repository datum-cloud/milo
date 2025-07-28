package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ResourceRequest defines a single resource request within a claim
type ResourceRequest struct {
	// Fully qualified name of the resource type being claimed
	//
	// +kubebuilder:validation:Required
	ResourceTypeName string `json:"resourceTypeName"`
	// Amount of the resource being claimed
	//
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Required
	Amount int64 `json:"amount"`
	// Dimensions for this resource request as key-value pairs
	//
	// +kubebuilder:validation:Optional
	Dimensions map[string]string `json:"dimensions,omitempty"`
}

// ResourceClaimSpec defines the desired state of ResourceClaim.
type ResourceClaimSpec struct {
	// Reference to the resource that owns the claim request.
	//
	// +kubebuilder:validation:Required
	OwnerRef OwnerRef `json:"ownerRef"`
	// List of resource requests defined by this claim
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Requests []ResourceRequest `json:"requests"`
}

// ResourceClaimStatus defines the observed state of ResourceClaim.
type ResourceClaimStatus struct {
	// Most recent generation observed.
	//
	// +kubebuilder:validation:Optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Known condition types: "Granted"
	//
	// +kubebuilder:validation:XValidation:rule="self.all(c, c.type == 'Granted' ? c.reason in ['QuotaAvailable', 'QuotaExceeded', 'ValidationFailed', 'PendingEvaluation'] : true)",message="Granted condition reason must be valid"
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// Condition type constants for ResourceClaim
const (
	// Indicates whether the ResourceClaim was granted after evaluation
	ResourceClaimGranted = "Granted"
)

// Condition reason constants for ResourceClaim status updates
const (
	// Granted due to quota being available
	ResourceClaimGrantedReason = "QuotaAvailable"
	// Denied due to it exceeding the quota limit
	ResourceClaimDeniedReason = "QuotaExceeded"
	// Indicates that status update validation failed.
	ResourceClaimValidationFailedReason = "ValidationFailed"
	// Indicates that the ResourceClaim has not finished being evaluated against the total effective quota limit
	ResourceClaimPendingReason = "PendingEvaluation"
)

// ResourceClaim is the Schema for the resourceclaims API.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Granted",type="string",JSONPath=".status.conditions[?(@.type=='Granted')].status"
// +k8s:openapi-gen=true
type ResourceClaim struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:Required
	Spec   ResourceClaimSpec   `json:"spec"`
	Status ResourceClaimStatus `json:"status,omitempty"`
}

// ResourceClaimList contains a list of ResourceClaim.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
type ResourceClaimList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ResourceClaim `json:"items"`
}
