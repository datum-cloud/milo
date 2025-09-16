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
	// Reference to the owner resource specific object instance.
	//
	// +kubebuilder:validation:Required
	ConsumerRef ConsumerRef `json:"consumerRef"`
	// The resource type this bucket tracks quota usage for
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^[a-z]([-a-z]*[a-z])?(\.[a-z]([-a-z]*[a-z])?)*\/[a-zA-Z][a-zA-Z]*(\/*[a-zA-Z][a-zA-Z]*)*$`
	ResourceType string `json:"resourceType"`
	// Dimensions for this bucket as key-value pairs
	//
	// +kubebuilder:validation:Optional
	Dimensions map[string]string `json:"dimensions,omitempty"`
}

// AllowanceBucketStatus defines the observed state of AllowanceBucket.
// Optimized for scalability - tracks only aggregated values, not individual claims.
type AllowanceBucketStatus struct {
	// The specific revision of the AllowanceBucket
	//
	// +kubebuilder:validation:Optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Total quota limit from all applicable ResourceGrants
	//
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Required
	Limit int64 `json:"limit"`
	// Amount of quota currently allocated/used in this bucket
	//
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Required
	Allocated int64 `json:"allocated"`
	// Amount available to be claimed (limit - allocated)
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

// AllowanceBucket is the Schema for the allowancebuckets API.
// Provides the single source of truth for quota limits and usage for specific
// resource and dimension combinations. Optimized for scale by tracking only
// aggregated values rather than individual claim references.
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
