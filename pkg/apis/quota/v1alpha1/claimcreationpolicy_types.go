package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ClaimCreationPolicySpec defines the desired state of ClaimCreationPolicy.
type ClaimCreationPolicySpec struct {
	// Trigger defines what resource changes should trigger claim creation.
	//
	// +kubebuilder:validation:Required
	Trigger ClaimTriggerSpec `json:"trigger"`
	// Target defines how and where ResourceClaims should be created.
	//
	// +kubebuilder:validation:Required
	Target ClaimTargetSpec `json:"target"`
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

// ClaimTriggerSpec defines the resource type and optional conditions for triggering claim creation.
type ClaimTriggerSpec struct {
	// Resource specifies which resource type triggers this policy.
	//
	// +kubebuilder:validation:Required
	Resource TargetResource `json:"resource"`
	// Conditions are CEL expressions that must evaluate to true for claim creation to occur.
	// Evaluated in the admission context.
	//
	// +optional
	// +kubebuilder:validation:MaxItems=10
	Conditions []ConditionExpression `json:"conditions,omitempty"`
}

// ClaimTargetSpec defines how ResourceClaims are created for a matched trigger.
type ClaimTargetSpec struct {
	// ResourceClaimTemplate defines how to create ResourceClaims.
	// String fields support Go template syntax for dynamic content.
	//
	// +kubebuilder:validation:Required
	ResourceClaimTemplate ResourceClaimTemplate `json:"resourceClaimTemplate"`
}

// ResourceClaimTemplate defines how to create ResourceClaims using actual ResourceClaim structure.
type ResourceClaimTemplate struct {
	// Metadata for the created ResourceClaim.
	// String fields support Go template syntax.
	//
	// +kubebuilder:validation:Required
	Metadata ObjectMetaTemplate `json:"metadata"`
	// Spec for the created ResourceClaim.
	// String fields support Go template syntax.
	//
	// +kubebuilder:validation:Required
	Spec ResourceClaimSpec `json:"spec"`
}

// ObjectMetaTemplate defines a minimal, templatable subset of ObjectMeta for use in templates.
// Only safe, user-controlled fields are exposed.
type ObjectMetaTemplate struct {
	// Name of the created object. Supports Go templates.
	// +optional
	Name string `json:"name,omitempty"`
	// GenerateName prefix for the created object when Name is empty. Supports Go templates.
	// +optional
	GenerateName string `json:"generateName,omitempty"`
	// Namespace where the object will be created. Supports Go templates.
	// +optional
	Namespace string `json:"namespace,omitempty"`
	// Labels to set on the created object. Literal values only.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
	// Annotations to set on the created object. Values support Go templates.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// ClaimCreationPolicyStatus defines the observed state of ClaimCreationPolicy.
//
// Status fields
// - conditions[type=Ready]: True when the policy is validated and active.
//
// See also
// - [ResourceClaim](#resourceclaim): The object created by this policy.
type ClaimCreationPolicyStatus struct {
	// ObservedGeneration is the most recent generation observed.
	//
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Conditions represent the latest available observations of the policy's current state.
	//
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
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

// ClaimCreationPolicy creates ResourceClaims during admission when target resources are created.
// Use it to enforce quota in real time at resource creation.
//
// ### How It Works
// - Admission matches incoming creates against `spec.trigger.resource`.
// - It evaluates all CEL expressions in `spec.trigger.conditions[]`.
// - When all conditions are true, it renders `resourceClaimTemplate` and creates a claim.
// - The system evaluates the claim against [AllowanceBucket](#allowancebucket)s and grants or denies the request.
//
// ### Works With
// - Creates [ResourceClaim](#resourceclaim) objects; the triggering kind must be allowed by the target [ResourceRegistration](#resourceregistration) `spec.claimingResources`.
// - Consumer resolution is automatic at admission; claims are evaluated against [AllowanceBucket](#allowancebucket) capacity.
// - Policy readiness (`status.conditions[type=Ready]`) indicates the policy is valid and active.
//
// ### Selectors and Filtering
// - Field selectors (server-side): `spec.trigger.resource.kind`, `spec.trigger.resource.apiVersion`, `spec.enabled`.
// - Label selectors (add your own):
//   - `quota.miloapis.com/target-kind`: `Project`
//   - `quota.miloapis.com/environment`: `prod`
//
// - Common queries:
//   - All policies for a target kind: label selector `quota.miloapis.com/target-kind`.
//   - All enabled policies: field selector `spec.enabled=true`.
//
// ### Defaults and Limits
// - In `v1alpha1`, `spec.requests[]` amounts are static integers (no expression-based amounts).
// - `metadata.labels` in the template are literal; annotation values support templating.
// - `spec.consumerRef` is resolved automatically by admission (not templated in `v1alpha1`).
//
// ### Notes
// - Available template variables: `.trigger`, `.requestInfo`, `.user`.
// - Template functions: `lower`, `upper`, `title`, `default`, `contains`, `join`, `split`, `replace`, `trim`, `toInt`, `toString`.
// - If `Ready=False` with `ValidationFailed`, check expressions and templates for errors.
// - Disabled policies (`spec.enabled=false`) do not create claims, even if conditions match.
// - For task-oriented steps and examples, see future How-to guides.
//
// ### See Also
// - [ResourceClaim](#resourceclaim): The object created by this policy.
// - [ResourceRegistration](#resourceregistration): Controls which resources can claim quota.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Target",type="string",JSONPath=".spec.trigger.resource.kind"
// +kubebuilder:printcolumn:name="Enabled",type="boolean",JSONPath=".spec.enabled"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +k8s:openapi-gen=true
// +kubebuilder:selectablefield:JSONPath=".spec.trigger.resource.kind"
// +kubebuilder:selectablefield:JSONPath=".spec.trigger.resource.apiVersion"
// +kubebuilder:selectablefield:JSONPath=".spec.enabled"
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

// Validation rules
//
// Note: In v1alpha1, ResourceClaim amounts are static integers. Expression-based amounts
// are not supported in the template.
