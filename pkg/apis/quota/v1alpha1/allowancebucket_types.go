package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ContributingGrantRef references a ResourceGrant that contributes allowances to this bucket
type ContributingGrantRef struct {
	// Name of the ResourceGrant
	//
	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// The generation of the ResourceGrant when this bucket last processed it
	//
	// +kubebuilder:validation:Optional
	LastObservedGeneration int64 `json:"lastObservedGeneration"`
}

// AllowanceBucketSpec defines the desired state of AllowanceBucket.
type AllowanceBucketSpec struct {
	// Reference to the EffectiveResourceGrant that owns this bucket
	//
	// +kubebuilder:validation:Required
	OwnerRef OwnerRef `json:"ownerRef"`
	// The resource type this bucket tracks quota usage for
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^[a-z]([-a-z]*[a-z])?(\.[a-z]([-a-z]*[a-z])?)*\/[a-zA-Z][a-zA-Z]*(\/*[a-zA-Z][a-zA-Z]*)*$`
	ResourceTypeName string `json:"resourceTypeName"`
	// Dimensions for this bucket as key-value pairs
	//
	// +kubebuilder:validation:Optional
	Dimensions map[string]string `json:"dimensions,omitempty"`
}

// AllowanceBucketStatus defines the observed state of AllowanceBucket.
type AllowanceBucketStatus struct {
	// The specific revision of the AllowanceBucket
	//
	// +kubebuilder:validation:Optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Amount of quota currently allocated/used in this bucket
	//
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Required
	Allocated int64 `json:"allocated"`
	// A list of all the claims that have been allocated quota from this bucket.
	//
	// +kubebuilder:validation:Optional
	Allocations []Allocation `json:"allocations,omitempty"`
}

// AllowanceBucket is the Schema for the allowancebuckets API.
// Provides the single source of truth for usage accounting for specific resource and dimension combinations.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +k8s:openapi-gen=true
// +kubebuilder:printcolumn:name="Resource Type",type=string,JSONPath=`.spec.resourceTypeName`
// +kubebuilder:printcolumn:name="Allocated",type=integer,JSONPath=`.status.allocated`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
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
