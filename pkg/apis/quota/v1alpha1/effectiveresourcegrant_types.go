package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EffectiveResourceGrantSpec defines the desired state of EffectiveResourceGrant.
type EffectiveResourceGrantSpec struct {
	// Reference to the resource that owns the effective grant.
	//
	// +kubebuilder:validation:Required
	OwnerRef OwnerRef `json:"ownerRef"`

	// The resource type this effective grant aggregates quota information for
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^[a-z]([-a-z]*[a-z])?(\.[a-z]([-a-z]*[a-z])?)*\/[a-zA-Z][a-zA-Z]*(\/*[a-zA-Z][a-zA-Z]*)*$`
	ResourceTypeName string `json:"resourceTypeName"`
}

// AllowanceBucketRef references an AllowanceBucket and its observed state
type AllowanceBucketRef struct {
	// Name of the AllowanceBucket
	//
	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// The generation of the AllowanceBucket when this reference was last
	// observed
	//
	// +kubebuilder:validation:Optional
	ObservedGeneration int64 `json:"observedGeneration"`
	// Amount allocated in this bucket
	//
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Required
	Allocated int64 `json:"allocated"`
}

// EffectiveResourceGrantStatus defines the observed state of EffectiveResourceGrant.
type EffectiveResourceGrantStatus struct {
	// The specific revision of the EffectiveResourceGrant
	//
	// +kubebuilder:validation:Optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Total effective quota limit from all applicable ResourceGrants for this resource type
	//
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Required
	TotalLimit int64 `json:"totalLimit"`
	// Total allocated usage across all AllowanceBucket resources for this resource type
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
	// References to AllowanceBuckets used to calculate totalAllocated
	//
	// +kubebuilder:validation:Optional
	AllowanceBucketRefs []AllowanceBucketRef `json:"allowanceBucketRefs,omitempty"`
	// Known condition types: "Ready"
	//
	// +kubebuilder:validation:XValidation:rule="self.all(c, c.type == 'Ready' ? c.reason in ['AggregationComplete', 'AggregationFailed', 'AggregationPending'] : true)",message="Ready condition reason must be valid"
	// +kubebuilder:validation:Optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// Condition type constants for EffectiveResourceGrant
const (
	// Indicates that the EffectiveResourceGrant has completed aggregation and is ready
	EffectiveResourceGrantReady = "Ready"
)

// Condition reason constants for EffectiveResourceGrant
const (
	// Indicates that aggregation has completed successfully
	EffectiveResourceGrantAggregationCompleteReason = "AggregationComplete"
	// Indicates that aggregation failed
	EffectiveResourceGrantAggregationFailedReason = "AggregationFailed"
	// Indicates that aggregation is pending
	EffectiveResourceGrantAggregationPendingReason = "AggregationPending"
)

// EffectiveResourceGrant is the Schema for the effectiveresourcegrants API.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +k8s:openapi-gen=true
// +kubebuilder:printcolumn:name="Resource Type",type=string,JSONPath=`.spec.resourceTypeName`
// +kubebuilder:printcolumn:name="Total Limit",type=integer,JSONPath=`.status.totalLimit`
// +kubebuilder:printcolumn:name="Allocated",type=integer,JSONPath=`.status.totalAllocated`
// +kubebuilder:printcolumn:name="Available",type=integer,JSONPath=`.status.available`
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type EffectiveResourceGrant struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:Required
	Spec   EffectiveResourceGrantSpec   `json:"spec"`
	Status EffectiveResourceGrantStatus `json:"status,omitempty"`
}

// EffectiveResourceGrantList contains a list of EffectiveResourceGrant.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
type EffectiveResourceGrantList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EffectiveResourceGrant `json:"items"`
}
