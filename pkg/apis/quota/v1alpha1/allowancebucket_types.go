package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ContributingGrantRef references a ResourceGrant that contributes to
// the total limit in the bucket's status.
type ContributingGrantRef struct {
	// Name of the ResourceGrant
	//
	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// The generation of the ResourceGrant when this bucket last processed it
	//
	// +kubebuilder:validation:Required
	LastObservedGeneration int64 `json:"lastObservedGeneration"`
	// Amount granted
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=0
	Amount int64 `json:"amount"`
}

// AllowanceBucketSpec defines the desired state of AllowanceBucket.
type AllowanceBucketSpec struct {
	// ConsumerRef identifies the quota consumer this bucket tracks
	//
	// +kubebuilder:validation:Required
	ConsumerRef ConsumerRef `json:"consumerRef"`
	// ResourceType specifies which resource type this bucket tracks.
	// Must match a registered resource type from ResourceRegistration.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^[a-z]([-a-z]*[a-z])?(\.[a-z]([-a-z]*[a-z])?)*\/[a-zA-Z][a-zA-Z]*(\/*[a-zA-Z][a-zA-Z]*)*$`
	ResourceType string `json:"resourceType"`
}

// AllowanceBucketStatus is the controller‑computed snapshot for a single
// (`spec.consumerRef`, `spec.resourceType`). The controller aggregates capacity
// from Active [ResourceGrant](#resourcegrant)s and usage from Granted
// [ResourceClaim](#resourceclaim)s, then derives availability as capacity minus
// usage (never negative). It also records provenance for how capacity was
// composed, simple cardinalities to aid troubleshooting at scale, and a
// reconciliation timestamp. Values may lag briefly after underlying grants or
// claims change. See the schema for exact field names and constraints.
type AllowanceBucketStatus struct {
	// The specific revision of the AllowanceBucket
	//
	// +kubebuilder:validation:Optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Total quota limit from all applicable ResourceGrants, measured in the
	// BaseUnit defined by the ResourceRegistration for this resource type.
	//
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Required
	Limit int64 `json:"limit"`
	// Amount of quota currently allocated/used in this bucket, measured in the
	// BaseUnit defined by the ResourceRegistration.
	//
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Required
	Allocated int64 `json:"allocated"`
	// Amount available to be claimed (limit - allocated), measured in the
	// BaseUnit defined by the ResourceRegistration.
	//
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Required
	Available int64 `json:"available"`
	// Count of claims consuming quota from this bucket
	//
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Required
	ClaimCount int32 `json:"claimCount"`
	// Count of grants contributing to this bucket's limit
	//
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Required
	GrantCount int32 `json:"grantCount"`
	// A list of all the grants that contribute to the limit for this bucket.
	// Grants are tracked individually as they are typically few in number.
	//
	// +kubebuilder:validation:Optional
	ContributingGrantRefs []ContributingGrantRef `json:"contributingGrantRefs,omitempty"`
	// Last time the bucket was reconciled
	//
	// +kubebuilder:validation:Optional
	LastReconciliation *metav1.Time `json:"lastReconciliation,omitempty"`
}

// AllowanceBucket tracks the effective quota for a single consumer and resource type.
// The system aggregates capacity from ResourceGrants and consumption from ResourceClaims
// to support real-time admission decisions.
//
// ### How It Works
// - Scope: One bucket per (`consumerRef`, `resourceType`) pair.
// - Inputs: Active `ResourceGrant`s increase `status.limit`; granted `ResourceClaim`s increase `status.allocated`.
// - Decision: Admission grants a claim only when `status.available >= requested amount`.
// - Scale: Status stores aggregates, not per-claim entries, to keep object size bounded.
//
// ### Works With
// - Aggregates active [ResourceGrant](#resourcegrant) amounts into `status.limit` for the matching (`spec.consumerRef`, `spec.resourceType`).
// - Aggregates granted [ResourceClaim](#resourceclaim) amounts into `status.allocated`.
// - Used by admission decisions: a claim is granted only if `status.available >= requested amount`.
// - Labeled by the controller to simplify queries by consumer and resource kind.
//
// ### Selectors and Filtering
// - Field selectors (server-side): `spec.consumerRef.kind`, `spec.consumerRef.name`, `spec.resourceType`.
// - Built-in labels (set by controller):
//   - `quota.miloapis.com/resource-kind`
//   - `quota.miloapis.com/resource-apigroup` (omitted for core kinds)
//   - `quota.miloapis.com/consumer-kind`
//   - `quota.miloapis.com/consumer-name`
//
// - Common queries:
//   - All buckets for a consumer: label selector `quota.miloapis.com/consumer-kind` + `quota.miloapis.com/consumer-name`.
//   - All buckets for a resource kind: label selector `quota.miloapis.com/resource-kind` (and `quota.miloapis.com/resource-apigroup` if needed).
//   - Buckets for a resourceType: field selector `spec.resourceType`.
//
// ### Notes
// - A dedicated controller is the single writer for status to avoid races.
// - Aggregates may lag briefly after grant/claim updates (eventual consistency).
// - `status.available` never goes negative.
//
// ### See Also
// - [ResourceGrant](#resourcegrant): Supplies capacity that increases `status.limit`.
// - [ResourceClaim](#resourceclaim): Consumes capacity that increases `status.allocated`.
// - [ClaimCreationPolicy](#claimcreationpolicy): Drives creation of claims during admission.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +k8s:openapi-gen=true
// +kubebuilder:printcolumn:name="Resource Type",type="string",JSONPath=".spec.resourceType"
// +kubebuilder:printcolumn:name="Limit",type="integer",JSONPath=".status.limit"
// +kubebuilder:printcolumn:name="Allocated",type="integer",JSONPath=".status.allocated"
// +kubebuilder:printcolumn:name="Available",type="integer",JSONPath=".status.available"
// +kubebuilder:printcolumn:name="Claims",type="integer",JSONPath=".status.claimCount"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:selectablefield:JSONPath=".spec.consumerRef.kind"
// +kubebuilder:selectablefield:JSONPath=".spec.consumerRef.name"
// +kubebuilder:selectablefield:JSONPath=".spec.resourceType"
type AllowanceBucket struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// +kubebuilder:validation:Required
	Spec   AllowanceBucketSpec   `json:"spec"`
	Status AllowanceBucketStatus `json:"status,omitempty"`
}

// AllowanceBucketList contains a list of AllowanceBucket.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
type AllowanceBucketList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AllowanceBucket `json:"items"`
}
