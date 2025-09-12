package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ClaimCreationPolicySpec defines the desired state of ClaimCreationPolicy.
type ClaimCreationPolicySpec struct {
	// TargetResource specifies which resource type this policy applies to.
	// Each ClaimCreationPolicy targets exactly one resource type (GVK).
	//
	// +kubebuilder:validation:Required
	TargetResource TargetResource `json:"targetResource"`

	// ResourceClaimTemplate defines how to create ResourceClaims for the target resource.
	//
	// +kubebuilder:validation:Required
	ResourceClaimTemplate ResourceClaimTemplateSpec `json:"resourceClaimTemplate"`

	// Enabled determines if this policy is active.
	// If false, no ResourceClaims will be created for matching resources.
	//
	// +kubebuilder:default=true
	// +optional
	Enabled *bool `json:"enabled,omitempty"`
}

// TargetResource identifies the resource type that this policy applies to.
type TargetResource struct {
	// APIVersion of the target resource in the format "group/version".
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*\/v[0-9]+((alpha|beta)[0-9]*)?$`
	APIVersion string `json:"apiVersion"`

	// Kind is the kind of the target resource.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Kind string `json:"kind"`
}

// ResourceClaimTemplateSpec defines how to create ResourceClaims.
type ResourceClaimTemplateSpec struct {
	// Requests defines the resource requests to include in the ResourceClaim.
	// Multiple requests enable claiming different resource types simultaneously.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Requests []ResourceRequestTemplate `json:"requests"`

	// NameTemplate for generating ResourceClaim names.
	// Supports Go templating with variables like {{.ResourceName}}, {{.Namespace}}, {{.Kind}}.
	// If not specified, a default template will be used.
	//
	// +optional
	NameTemplate string `json:"nameTemplate,omitempty"`

	// Namespace where the ResourceClaim should be created.
	// If not specified, defaults to the milo-system namespace.
	//
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Labels to add to the created ResourceClaim.
	//
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Annotations to add to the created ResourceClaim.
	//
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// ResourceRequestTemplate defines how to create individual resource requests.
// Supports both static values and CEL expressions using field name suffixes.
type ResourceRequestTemplate struct {
	// ResourceType to use in the ResourceClaim.
	// Must correspond to an active ResourceRegistration.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	ResourceType string `json:"resourceType"`

	// Amount is the quota amount to request (static value).
	// Mutually exclusive with AmountExpression.
	//
	// +optional
	Amount *int64 `json:"amount,omitempty"`

	// AmountExpression - CEL expression to calculate amount dynamically.
	// Mutually exclusive with Amount.
	//
	// +optional
	AmountExpression string `json:"amountExpression,omitempty"`

	// Dimensions for the resource claim (static key-value pairs).
	// Can be combined with DimensionExpressions.
	//
	// +optional
	Dimensions map[string]string `json:"dimensions,omitempty"`

	// DimensionExpressions - CEL expressions for dynamic dimension values.
	// Merged with static Dimensions, expressions take precedence for duplicate keys.
	//
	// +optional
	DimensionExpressions map[string]string `json:"dimensionExpressions,omitempty"`

	// ConditionExpression - CEL expression to determine if this request should be created.
	// If empty or evaluates to true, the request is included.
	//
	// +optional
	ConditionExpression string `json:"conditionExpression,omitempty"`
}

// ClaimCreationPolicyStatus defines the observed state of ClaimCreationPolicy.
type ClaimCreationPolicyStatus struct {
	// ObservedGeneration is the most recent generation observed.
	//
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions represent the latest available observations of the policy's current state.
	//
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ResourceClaimsCreated is the total number of ResourceClaims created by this policy.
	//
	// +optional
	ResourceClaimsCreated int64 `json:"resourceClaimsCreated,omitempty"`

	// LastResourceClaimCreated is the timestamp of the most recent ResourceClaim creation.
	//
	// +optional
	LastResourceClaimCreated *metav1.Time `json:"lastResourceClaimCreated,omitempty"`
}

// Condition type constants for ClaimCreationPolicy.
const (
	// ClaimCreationPolicyReady indicates the policy is ready for use.
	ClaimCreationPolicyReady = "Ready"
	// ClaimCreationPolicyValidationFailed indicates policy validation failed.
	ClaimCreationPolicyValidationFailed = "ValidationFailed"
)

// Condition reason constants for ClaimCreationPolicy.
const (
	// ClaimCreationPolicyReadyReason indicates the policy is ready.
	ClaimCreationPolicyReadyReason = "PolicyReady"
	// ClaimCreationPolicyValidationFailedReason indicates validation failed.
	ClaimCreationPolicyValidationFailedReason = "ValidationFailed"
	// ClaimCreationPolicyDisabledReason indicates the policy is disabled.
	ClaimCreationPolicyDisabledReason = "PolicyDisabled"
)

// Helper method to get the GVK for the target resource.
func (t *TargetResource) GetGVK() schema.GroupVersionKind {
	gv, _ := schema.ParseGroupVersion(t.APIVersion)
	return schema.GroupVersionKind{
		Group:   gv.Group,
		Version: gv.Version,
		Kind:    t.Kind,
	}
}

// ClaimCreationPolicy is the Schema for the claimcreationpolicies API.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Target",type="string",JSONPath=".spec.targetResource.apiVersion/.spec.targetResource.kind"
// +kubebuilder:printcolumn:name="Enabled",type="boolean",JSONPath=".spec.enabled"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Created Claims",type="integer",JSONPath=".status.resourceClaimsCreated"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +k8s:openapi-gen=true
type ClaimCreationPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:Required
	Spec   ClaimCreationPolicySpec   `json:"spec"`
	Status ClaimCreationPolicyStatus `json:"status,omitempty"`
}

// ClaimCreationPolicyList contains a list of ClaimCreationPolicy.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
type ClaimCreationPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClaimCreationPolicy `json:"items"`
}

// Validation rules using kubebuilder markers
//
// +kubebuilder:validation:XValidation:rule="!has(self.amount) || !has(self.amountExpression)",message="amount and amountExpression are mutually exclusive"
// +kubebuilder:validation:XValidation:rule="has(self.amount) || has(self.amountExpression)",message="either amount or amountExpression must be specified"
