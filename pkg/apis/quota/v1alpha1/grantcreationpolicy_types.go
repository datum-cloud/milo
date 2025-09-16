package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// GrantCreationPolicySpec defines the desired state of GrantCreationPolicy.
type GrantCreationPolicySpec struct {
	// Trigger defines what resource changes should trigger grant creation.
	//
	// +kubebuilder:validation:Required
	Trigger TriggerSpec `json:"trigger"`

	// Target defines where and how grants should be created.
	//
	// +kubebuilder:validation:Required
	Target TargetSpec `json:"target"`

	// Enabled determines if this policy is active.
	// If false, no ResourceGrants will be created for matching resources.
	//
	// +kubebuilder:default=true
	// +optional
	Enabled *bool `json:"enabled,omitempty"`
}

// TriggerSpec defines the resource and conditions that trigger grant creation.
type TriggerSpec struct {
	// Resource specifies which resource type triggers this policy.
	//
	// +kubebuilder:validation:Required
	Resource TriggerResource `json:"resource"`

	// Conditions are CEL expressions that must evaluate to true for grant creation.
	// All conditions must pass for the policy to trigger.
	// The 'object' variable contains the trigger resource being evaluated.
	//
	// +optional
	// +kubebuilder:validation:MaxItems=10
	Conditions []ConditionExpression `json:"conditions,omitempty"`
}

// TriggerResource identifies the resource type that triggers grant creation.
type TriggerResource struct {
	// APIVersion of the trigger resource in the format "group/version".
	// For core resources, use "v1".
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^([a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*\/)?v[0-9]+((alpha|beta)[0-9]*)?$`
	APIVersion string `json:"apiVersion"`

	// Kind is the kind of the trigger resource.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=`^[A-Z][a-zA-Z0-9]*$`
	Kind string `json:"kind"`
}

// ConditionExpression defines a CEL expression for condition evaluation.
type ConditionExpression struct {
	// Expression is the CEL expression to evaluate against the trigger resource.
	// The expression must return a boolean value.
	// Available variables:
	// - object: The trigger resource being evaluated (same as .trigger in Go templates)
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=1024
	Expression string `json:"expression"`

	// Message provides a human-readable description of the condition requirement.
	//
	// +optional
	// +kubebuilder:validation:MaxLength=256
	Message string `json:"message,omitempty"`
}

// TargetSpec defines where and how grants are created.
type TargetSpec struct {
	// ParentContext defines cross-control-plane targeting.
	// If specified, grants will be created in the target parent context
	// instead of the current control plane.
	//
	// +optional
	ParentContext *ParentContextSpec `json:"parentContext,omitempty"`

	// ResourceGrantTemplate defines how to create ResourceGrants.
	//
	// +kubebuilder:validation:Required
	ResourceGrantTemplate ResourceGrantTemplateSpec `json:"resourceGrantTemplate"`
}

// ParentContextSpec defines parent context resolution for cross-cluster operations.
type ParentContextSpec struct {
	// APIGroup of the parent context resource.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`
	APIGroup string `json:"apiGroup"`

	// Kind of the parent context resource.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=`^[A-Z][a-zA-Z0-9]*$`
	Kind string `json:"kind"`

	// NameExpression is a CEL expression to resolve the parent context name.
	// The expression must return a string value.
	// Available variables:
	// - object: The trigger resource being evaluated (same as .trigger in Go templates)
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=512
	NameExpression string `json:"nameExpression"`
}

// ResourceGrantTemplateSpec defines the template for creating ResourceGrants.
type ResourceGrantTemplateSpec struct {
	// Metadata template for the created ResourceGrant.
	//
	// +kubebuilder:validation:Required
	Metadata GrantMetadataTemplate `json:"metadata"`

	// Spec template for the created ResourceGrant.
	//
	// +kubebuilder:validation:Required
	Spec GrantSpecTemplate `json:"spec"`
}

// GrantMetadataTemplate defines metadata template fields using standard Kubernetes metadata structure.
type GrantMetadataTemplate struct {
	// Name for grant names using Go template syntax.
	// Available variables: .ResourceName, .ResourceKind, .Namespace, .PolicyName, .trigger
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Name string `json:"name"`

	// Namespace where grants are created.
	// Supports Go template syntax.
	// Available variables: .ResourceName, .ResourceKind, .Namespace, .PolicyName, .trigger
	// If not specified, defaults to "milo-system".
	//
	// +optional
	// +kubebuilder:validation:MaxLength=253
	Namespace string `json:"namespace,omitempty"`

	// Labels to add to created grants.
	// Template variables are not supported in label values.
	//
	// +optional
	// +kubebuilder:validation:MaxProperties=64
	Labels map[string]string `json:"labels,omitempty"`

	// Annotations to add to created grants.
	// Supports Go template syntax in values.
	// Available variables: .ResourceName, .ResourceKind, .Namespace, .PolicyName, .trigger
	//
	// +optional
	// +kubebuilder:validation:MaxProperties=64
	Annotations map[string]string `json:"annotations,omitempty"`
}

// GrantSpecTemplate defines the spec template for ResourceGrants.
type GrantSpecTemplate struct {
	// ConsumerRef template for the grant consumer.
	//
	// +kubebuilder:validation:Required
	ConsumerRefTemplate ConsumerRefTemplate `json:"consumerRef"`

	// Allowances defines static resource allowances.
	// Dynamic allowance calculation is not supported.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=20
	Allowances []AllowanceTemplate `json:"allowances"`
}

// ConsumerRefTemplate defines the consumer reference template.
type ConsumerRefTemplate struct {
	// APIGroup template for the consumer.
	// Supports Go template syntax.
	// Available variables: .ResourceName, .ResourceKind, .Namespace, .PolicyName, .trigger
	//
	// +optional
	// +kubebuilder:validation:MaxLength=253
	APIGroup string `json:"apiGroup,omitempty"`

	// Kind template for the consumer.
	// Supports Go template syntax.
	// Available variables: .ResourceName, .ResourceKind, .Namespace, .PolicyName, .trigger
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	Kind string `json:"kind"`

	// Name for the consumer name using Go template syntax.
	// Available variables: .ResourceName, .ResourceKind, .Namespace, .PolicyName, .trigger
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Name string `json:"name"`
}

// AllowanceTemplate defines static allowance configuration.
type AllowanceTemplate struct {
	// ResourceType being granted.
	// Must correspond to an active ResourceRegistration.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*\/[A-Z][a-zA-Z0-9]*$`
	ResourceType string `json:"resourceType"`

	// Buckets define quota buckets with static amounts.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=10
	Buckets []BucketTemplate `json:"buckets"`
}

// BucketTemplate defines a static quota bucket.
type BucketTemplate struct {
	// Amount of quota to grant (static value only).
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=1000000000
	Amount int64 `json:"amount"`

	// DimensionSelector for this bucket using Kubernetes LabelSelector.
	// If not specified, the bucket applies to all dimensions.
	//
	// +optional
	DimensionSelector *metav1.LabelSelector `json:"dimensionSelector,omitempty"`
}

// GrantCreationPolicyStatus defines the observed state of GrantCreationPolicy.
type GrantCreationPolicyStatus struct {
	// ObservedGeneration is the most recent generation observed.
	//
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions represent the latest available observations of the policy's current state.
	//
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// Condition type constants for GrantCreationPolicy.
const (
	// GrantCreationPolicyReady indicates the policy is ready for use.
	GrantCreationPolicyReady = "Ready"
	// GrantCreationPolicyParentContextReady indicates parent context resolution is working.
	GrantCreationPolicyParentContextReady = "ParentContextReady"
)

// Condition reason constants for GrantCreationPolicy.
const (
	// GrantCreationPolicyReadyReason indicates the policy is ready.
	GrantCreationPolicyReadyReason = "PolicyReady"
	// GrantCreationPolicyValidationFailedReason indicates validation failed.
	GrantCreationPolicyValidationFailedReason = "ValidationFailed"
	// GrantCreationPolicyDisabledReason indicates the policy is disabled.
	GrantCreationPolicyDisabledReason = "PolicyDisabled"
	// GrantCreationPolicyParentContextReadyReason indicates parent context is ready.
	GrantCreationPolicyParentContextReadyReason = "ParentContextReady"
	// GrantCreationPolicyParentContextFailedReason indicates parent context resolution failed.
	GrantCreationPolicyParentContextFailedReason = "ParentContextFailed"
)

// Helper method to get the GVK for the trigger resource.
func (t *TriggerResource) GetGVK() schema.GroupVersionKind {
	gv, _ := schema.ParseGroupVersion(t.APIVersion)
	return schema.GroupVersionKind{
		Group:   gv.Group,
		Version: gv.Version,
		Kind:    t.Kind,
	}
}

// GrantCreationPolicy is the Schema for the grantcreationpolicies API.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Trigger",type="string",JSONPath=".spec.trigger.resource.kind"
// +kubebuilder:printcolumn:name="Enabled",type="boolean",JSONPath=".spec.enabled"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +k8s:openapi-gen=true
type GrantCreationPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:Required
	Spec   GrantCreationPolicySpec   `json:"spec"`
	Status GrantCreationPolicyStatus `json:"status,omitempty"`
}

// GrantCreationPolicyList contains a list of GrantCreationPolicy.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
type GrantCreationPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GrantCreationPolicy `json:"items"`
}

// Validation rules using kubebuilder CEL expressions
//
// +kubebuilder:validation:XValidation:rule="!has(self.spec.enabled) || self.spec.enabled == true || size(self.spec.trigger.conditions) == 0",message="disabled policies should not have trigger conditions"
// +kubebuilder:validation:XValidation:rule="!has(self.spec.target.parentContext) || size(self.spec.target.parentContext.nameExpression) > 0",message="parent context must have a name expression"
// +kubebuilder:validation:XValidation:rule="size(self.spec.target.resourceGrantTemplate.spec.allowances) <= 20",message="maximum 20 allowances per policy"
