package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Bucket struct {
	// Amount of the resource type being granted, measured in the BaseUnit
	// defined by the corresponding ResourceRegistration for this resource type.
	//
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Required
	Amount int64 `json:"amount"`
}

// Allowance defines a single resource allowance within a grant
type Allowance struct {
	// Fully qualified name of the resource type being granted.
	// Must match a registered ResourceRegistration.spec.resourceType
	// (for example, "resourcemanager.miloapis.com/projects").
	//
	// +kubebuilder:validation:Required
	ResourceType string `json:"resourceType"`
	// List of buckets this allowance contains
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Buckets []Bucket `json:"buckets"`
}

// ResourceGrantSpec defines the desired state of ResourceGrant.
type ResourceGrantSpec struct {
	// ConsumerRef identifies the quota consumer (recipient) that receives
	// these allowances (for example, an Organization).
	//
	// +kubebuilder:validation:Required
	ConsumerRef ConsumerRef `json:"consumerRef"`
	// List of allowances this grant contains
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Allowances []Allowance `json:"allowances"`
}

// ResourceGrantStatus indicates whether a grant is active and the most recent
// spec generation processed by the controller. Only Active grants contribute to
// bucket limits. See the schema for exact fields and condition reasons. For how
// capacity is aggregated, see AllowanceBucket, and for type validity see
// ResourceRegistration.
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

// ResourceGrant allocates capacity to a consumer for one or more resource types.
// AllowanceBuckets aggregate active grants to calculate available quota.
// You can create grants manually or automate them with GrantCreationPolicy.
//
// ### How It Works
// - Allocate allowances for one or more `resourceType`s to a `consumerRef`.
// - Only grants with `status.conditions[type=Active]==True` contribute to bucket limits.
// - Grants may be created manually or via [GrantCreationPolicy](#grantcreationpolicy).
//
// ### Works With
// - Increases [AllowanceBucket](#allowancebucket) `status.limit` for matching (`spec.consumerRef`, `allowances[].resourceType`).
// - Only grants with `status.conditions[type=Active]=="True"` affect bucket limits.
// - Often created by [GrantCreationPolicy](#grantcreationpolicy); manual grants behave the same.
// - Cross-plane allocations are possible when policies target a parent context.
//
// ### Selectors and Filtering
//   - Field selectors (server-side): `spec.consumerRef.kind`, `spec.consumerRef.name`.
//   - Label selectors: Add your own labels in metadata to group grants (for example by tier or region).
//     Common labels you may add:
//   - `quota.miloapis.com/consumer-kind`: `Organization`
//   - `quota.miloapis.com/consumer-name`: `<name>`
//   - `quota.miloapis.com/resource-kind`: `Project` (repeat per allowance if desired)
//   - Common queries:
//   - All grants for a consumer: labels `quota.miloapis.com/consumer-kind` + `quota.miloapis.com/consumer-name`.
//   - Grants created by a policy: use a policy label your automation adds consistently.
//
// ### Notes
// - Amounts use the BaseUnit from the corresponding ResourceRegistration.
// - Multiple ResourceGrants can contribute to a single bucket; see bucket grantCount and contributingGrantRefs.
//
// ### See Also
// - [AllowanceBucket](#allowancebucket): Aggregates active grants into a single limit.
// - [ResourceRegistration](#resourceregistration): Validates resourceType names and claimability.
// - [GrantCreationPolicy](#grantcreationpolicy): Automates grant creation based on observed resources.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Active",type="string",JSONPath=".status.conditions[?(@.type=='Active')].status"
// +kubebuilder:printcolumn:name="Consumer Group",type="string",JSONPath=".spec.consumerRef.apiGroup",priority=1
// +kubebuilder:printcolumn:name="Consumer Type",type="string",JSONPath=".spec.consumerRef.kind",priority=1
// +kubebuilder:printcolumn:name="Consumer",type="string",JSONPath=".spec.consumerRef.name",priority=1
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +k8s:openapi-gen=true
// +kubebuilder:selectablefield:JSONPath=".spec.consumerRef.kind"
// +kubebuilder:selectablefield:JSONPath=".spec.consumerRef.name"
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
