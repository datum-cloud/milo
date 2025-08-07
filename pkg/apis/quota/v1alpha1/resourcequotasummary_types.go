package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ResourceQuotaSummarySpec defines the desired state of ResourceQuotaSummary.
type ResourceQuotaSummarySpec struct {
	// Reference to the owner resource specific object instance.
	//
	// +kubebuilder:validation:Required
	OwnerInstanceRef OwnerInstanceRef `json:"ownerInstanceRef"`

	// The resource type this summary aggregates quota information for
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^[a-z]([-a-z]*[a-z])?(\.[a-z]([-a-z]*[a-z])?)*\/[a-zA-Z][a-zA-Z]*(\/*[a-zA-Z][a-zA-Z]*)*$`
	ResourceType string `json:"resourceType"`
}

// ResourceQuotaSummaryStatus defines the observed state of ResourceQuotaSummary.
type ResourceQuotaSummaryStatus struct {
	// The specific revision of the ResourceQuotaSummary
	//
	// +kubebuilder:validation:Optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Total effective quota limit from all applicable ResourceGrants for this resource type
	//
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Required
	TotalLimit int64 `json:"totalLimit"`
	// Total allocated usage across all granted ResourceClaims for this resource type
	//
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Required
	TotalAllocated int64 `json:"totalAllocated"`
	// The amount available that can be claimed
	// Available = (totalLimit - totalAllocated)
	//
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Required
	Available int64 `json:"available"`
	// References to the granted ResourceClaims that have contributed to the
	// totalAllocated field
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Default={}
	ContributingClaimRefs []ContributingResourceRef `json:"contributingClaimRefs"`
	// A list of all the grants that have contributed to the totalLimit field
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Default={}
	ContributingGrantRefs []ContributingResourceRef `json:"contributingGrantRefs"`
	// Known condition types: "Ready"
	//
	// +kubebuilder:validation:XValidation:rule="self.all(c, c.type == 'Ready' ? c.reason in ['CalculationComplete', 'CalculationFailed', 'CalculationPending'] : true)",message="Ready condition reason must be valid"
	// +kubebuilder:validation:Optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// Condition type constants for ResourceQuotaSummary
const (
	// Indicates that the ResourceQuotaSummary has completed calculation of
	// status valuesand is ready
	ResourceQuotaSummaryReady = "Ready"
)

// Condition reason constants for ResourceQuotaSummary
const (
	// Indicates that calculation has completed successfully
	ResourceQuotaSummaryCalculationCompleteReason = "CalculationComplete"
	// Indicates that calculation failed
	ResourceQuotaSummaryCalculationFailedReason = "CalculationFailed"
	// Indicates that calculation is pending
	ResourceQuotaSummaryCalculationPendingReason = "CalculationPending"
)

// ResourceQuotaSummary is the Schema for the resourcequotasummaries API.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +k8s:openapi-gen=true
// +kubebuilder:printcolumn:name="Resource Type",type=string,JSONPath=`.spec.resourceType`
// +kubebuilder:printcolumn:name="Total Limit",type=integer,JSONPath=`.status.totalLimit`
// +kubebuilder:printcolumn:name="Allocated",type=integer,JSONPath=`.status.totalAllocated`
// +kubebuilder:printcolumn:name="Available",type=integer,JSONPath=`.status.available`
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type ResourceQuotaSummary struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:Required
	Spec   ResourceQuotaSummarySpec   `json:"spec"`
	Status ResourceQuotaSummaryStatus `json:"status,omitempty"`
}

// ResourceQuotaSummaryList contains a list of ResourceQuotaSummary.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
type ResourceQuotaSummaryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ResourceQuotaSummary `json:"items"`
}
