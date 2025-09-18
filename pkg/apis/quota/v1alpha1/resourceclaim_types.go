package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ResourceRequest defines a single resource request within a claim
type ResourceRequest struct {
	// Fully qualified name of the resource type being claimed.
	// Must match a registered ResourceRegistration.spec.resourceType
	// (for example, "resourcemanager.miloapis.com/projects" or
	// "core/persistentvolumeclaims").
	//
	// +kubebuilder:validation:Required
	ResourceType string `json:"resourceType"`
	// Amount of the resource being claimed, measured in the BaseUnit
	// defined by the corresponding ResourceRegistration.
	//
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Required
	Amount int64 `json:"amount"`
}

// ResourceClaimSpec defines the desired state of ResourceClaim.
type ResourceClaimSpec struct {
	// ConsumerRef identifies the quota consumer (the subject that receives
	// limits and consumes capacity) making this claim. Examples include an
	// Organization or a Project, depending on how the registration is defined.
	//
	// +kubebuilder:validation:Required
	ConsumerRef ConsumerRef `json:"consumerRef"`
	// Requests specifies the resource types and amounts being claimed.
	// Each resource type must be unique within the requests array.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=20
	Requests []ResourceRequest `json:"requests"`
	// ResourceRef links to the actual resource that triggered this quota claim.
	// Automatically populated by the admission plugin.
	// Uses an unversioned reference to persist across API version upgrades.
	//
	// +kubebuilder:validation:Required
	ResourceRef UnversionedObjectReference `json:"resourceRef"`
}

// RequestAllocation tracks the allocation status of a specific resource request within a claim.
type RequestAllocation struct {
	// Resource type that this allocation status refers to.
	// Must correspond to a resourceType listed in spec.requests.
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
	// Amount actually allocated for this request (may be less than requested in some scenarios),
	// measured in the BaseUnit defined by the ResourceRegistration.
	//
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=0
	AllocatedAmount int64 `json:"allocatedAmount,omitempty"`
	// Name of the AllowanceBucket that provided this allocation (set when status is Granted)
	//
	// +kubebuilder:validation:Optional
	AllocatingBucket string `json:"allocatingBucket,omitempty"`
	// Timestamp of the last status transition for this allocation
	//
	// +kubebuilder:validation:Required
	LastTransitionTime metav1.Time `json:"lastTransitionTime"`
}

// ResourceClaimStatus captures the controller's evaluation of a claim: an overall
// grant decision reported via conditions and per‑resource allocation results. It
// also records the most recent observed spec generation. See the schema for exact
// fields, condition reasons, and constraints. For capacity context, consult
// [AllowanceBucket](#allowancebucket) and for capacity sources see
// [ResourceGrant](#resourcegrant).
type ResourceClaimStatus struct {
	// Most recent generation observed.
	//
	// +kubebuilder:validation:Optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// removed: aggregate allocated total is not tracked; use per-request allocations instead
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

// ResourceClaim represents a quota consumption request tied to the creation of a resource.
// ClaimCreationPolicy typically creates these claims during admission to enforce quota in real time.
// The system evaluates each claim against [AllowanceBucket](#allowancebucket)s that aggregate available capacity.
//
// ### How It Works
// - Admission evaluates policies that match the incoming resource and creates a `ResourceClaim`.
// - The claim requests one or more resource types and amounts for a specific `consumerRef`.
// - The system grants the claim when sufficient capacity is available; otherwise it denies it.
// - `resourceRef` links back to the triggering resource to enable cleanup and auditing.
//
// ### Works With
// - Created by [ClaimCreationPolicy](#claimcreationpolicy) at admission when trigger conditions match.
// - Evaluated against [AllowanceBucket](#allowancebucket) capacity for the matching `spec.consumerRef` + `spec.requests[].resourceType`.
// - Must target a registered `resourceType`; the triggering kind must be allowed by the target [ResourceRegistration](#resourceregistration) `spec.claimingResources`.
// - Controllers set owner references where possible and clean up denied auto‑created claims.
//
// ### Notes
// - Auto-created claims set owner references when possible; a fallback path updates ownership asynchronously.
// - Auto-created claims denied by policy are cleaned up automatically; manual claims are not.
//
// ### Selectors and Filtering
//   - Field selectors (server-side):
//     `spec.consumerRef.kind`, `spec.consumerRef.name`,
//     `spec.resourceRef.apiGroup`, `spec.resourceRef.kind`, `spec.resourceRef.name`, `spec.resourceRef.namespace`.
//   - Built-in labels (on auto-created claims):
//   - `quota.miloapis.com/auto-created`: `"true"`
//   - `quota.miloapis.com/policy`: `<ClaimCreationPolicy name>`
//   - `quota.miloapis.com/gvk`: `<group.version.kind of the triggering resource>`
//   - Built-in annotations (on auto-created claims):
//   - `quota.miloapis.com/created-by`: `claim-creation-plugin`
//   - `quota.miloapis.com/created-at`: `RFC3339` timestamp
//   - `quota.miloapis.com/resource-name`: name of the triggering resource
//   - `quota.miloapis.com/policy`: `<ClaimCreationPolicy name>`
//   - Common queries:
//   - All auto-created claims for a policy: label selector `quota.miloapis.com/policy`.
//   - All claims for a consumer: add labels for `consumer-kind` and `consumer-name` via policy templates and filter by label.
//   - All claims for a specific triggering kind: label selector `quota.miloapis.com/gvk`.
//
// ### See Also
// - [AllowanceBucket](#allowancebucket): Aggregates limits and usage that drive claim evaluation.
// - [ResourceGrant](#resourcegrant): Supplies capacity aggregated by buckets.
// - [ClaimCreationPolicy](#claimcreationpolicy): Automates creation of ResourceClaims at admission.
// - [ResourceRegistration](#resourceregistration): Defines claimable resource types.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Granted",type="string",JSONPath=".status.conditions[?(@.type=='Granted')].status"
// +kubebuilder:printcolumn:name="Resource",type="string",JSONPath=".spec.requests[0].resourceType",priority=1
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +k8s:openapi-gen=true
// +kubebuilder:selectablefield:JSONPath=".spec.consumerRef.kind"
// +kubebuilder:selectablefield:JSONPath=".spec.consumerRef.name"
// +kubebuilder:selectablefield:JSONPath=".spec.resourceRef.apiGroup"
// +kubebuilder:selectablefield:JSONPath=".spec.resourceRef.kind"
// +kubebuilder:selectablefield:JSONPath=".spec.resourceRef.name"
// +kubebuilder:selectablefield:JSONPath=".spec.resourceRef.namespace"
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
