package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ResourceRequest defines a single resource request within a claim
type ResourceRequest struct {
	// Fully qualified name of the resource type being claimed
	//
	// +kubebuilder:validation:Required
	ResourceType string `json:"resourceType"`
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
	// Reference to the owner resource specific object instance.
	//
	// +kubebuilder:validation:Required
	ConsumerRef ConsumerRef `json:"consumerRef"`
	// List of resource requests defined by this claim
	// Resource types must be unique within the requests array
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=20
	Requests []ResourceRequest `json:"requests"`
	// Reference to the resource that claimed this quota (automatically populated by admission plugin).
	// This is an unversioned reference that persists across API version upgrades.
	//
	// +kubebuilder:validation:Required
	ResourceRef UnversionedObjectReference `json:"resourceRef"`
}

// RequestAllocation tracks the allocation status of a specific resource request within a claim.
type RequestAllocation struct {
	// Resource type that this allocation status refers to
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	ResourceType string `json:"resourceType"`
	// Status of this specific request allocation
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=Granted;Denied;Pending
	Status string `json:"status"`
	// Reason for the current allocation status
	//
	// +kubebuilder:validation:Optional
	Reason string `json:"reason,omitempty"`
	// Human-readable message describing the allocation result
	//
	// +kubebuilder:validation:Optional
	Message string `json:"message,omitempty"`
	// Amount actually allocated for this request (may be less than requested in some scenarios)
	//
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=0
	AllocatedAmount int64 `json:"allocatedAmount,omitempty"`
	// Timestamp of the last status transition for this allocation
	//
	// +kubebuilder:validation:Required
	LastTransitionTime metav1.Time `json:"lastTransitionTime"`
}

// ResourceClaimStatus defines the observed state of ResourceClaim.
type ResourceClaimStatus struct {
	// Most recent generation observed.
	//
	// +kubebuilder:validation:Optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Total amount of resources allocated by this claim across all requests
	//
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=0
	Allocated int64 `json:"allocated,omitempty"`
	// Per-request allocation status tracking. Each entry corresponds to a resource type in spec.requests[]
	//
	// +kubebuilder:validation:Optional
	// +listType=map
	// +listMapKey=resourceType
	Allocations []RequestAllocation `json:"allocations,omitempty"`
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

// Request allocation status constants
const (
	// Request allocation is granted and resources are reserved
	RequestAllocationGranted = "Granted"
	// Request allocation is denied due to insufficient quota
	RequestAllocationDenied = "Denied"
	// Request allocation is pending evaluation
	RequestAllocationPending = "Pending"
)

// ResourceClaim is the Schema for the resourceclaims API.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Granted",type="string",JSONPath=".status.conditions[?(@.type=='Granted')].status"
// +kubebuilder:printcolumn:name="Resource",type="string",JSONPath=".spec.requests[0].resourceType",priority=1
// +kubebuilder:printcolumn:name="Allocated",type="integer",JSONPath=".status.allocated"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +k8s:openapi-gen=true
type ResourceClaim struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:Required
	Spec ResourceClaimSpec `json:"spec"`
	// +kubebuilder:default={conditions: {{type:"Granted",status:"False",reason:"PendingEvaluation",message:"Awaiting capacity evaluation", lastTransitionTime: "1970-01-01T00:00:00Z"}}}
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
