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
	// If false, no **ResourceGrants** will be created for matching resources.
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
	// Constraints are CEL expressions that must evaluate to true for grant creation.
	// All constraints must pass for the policy to trigger.
	// The 'object' variable contains the trigger resource being evaluated.
	//
	// +optional
	// +kubebuilder:validation:MaxItems=10
	Constraints []ConditionExpression `json:"constraints,omitempty"`
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

// ConditionExpression defines a CEL expression that determines when the policy should trigger.
// All expressions in a policy's trigger conditions must evaluate to true for the policy to activate.
type ConditionExpression struct {
	// Expression specifies the CEL expression to evaluate against the trigger resource.
	// Must return a boolean value (true to match, false to skip).
	// Maximum 1024 characters.
	//
	// Available variables in GrantCreationPolicy context:
	// - object: The complete resource being watched (map[string]any)
	//   - object.metadata.name, object.spec.*, object.status.*, etc.
	//
	// Common expression patterns:
	// - object.spec.tier == "premium" (check resource field)
	// - object.metadata.labels["environment"] == "prod" (check labels)
	// - object.status.phase == "Active" (check status)
	// - object.metadata.namespace == "production" (check namespace)
	// - has(object.spec.quotaProfile) (check field existence)
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=1024
	Expression string `json:"expression"`

	// Message provides a human-readable description explaining when this condition applies.
	// Used for documentation and debugging. Maximum 256 characters.
	//
	// Examples:
	// - "Applies only to premium tier organizations"
	// - "Matches organizations in production environment"
	// - "Triggers when quota profile is specified"
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
	// ResourceGrantTemplate defines how to create **ResourceGrants**.
	// String fields support Go template syntax for dynamic content.
	//
	// +kubebuilder:validation:Required
	ResourceGrantTemplate ResourceGrantTemplate `json:"resourceGrantTemplate"`
}

// ParentContextSpec enables cross-cluster grant creation by targeting a parent control plane.
// Used to create grants in infrastructure clusters when policies run in child clusters.
type ParentContextSpec struct {
	// APIGroup specifies the API group of the parent context resource.
	// Must follow DNS subdomain format. Maximum 253 characters.
	//
	// Examples:
	// - "resourcemanager.miloapis.com" (for Organization parent context)
	// - "infrastructure.miloapis.com" (for Cluster parent context)
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`
	APIGroup string `json:"apiGroup"`

	// Kind specifies the resource type that represents the parent context.
	// Must be a valid Kubernetes resource Kind. Maximum 63 characters.
	//
	// Examples:
	// - "Organization" (create grants in organization's parent control plane)
	// - "Cluster" (create grants in cluster's parent infrastructure)
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=`^[A-Z][a-zA-Z0-9]*$`
	Kind string `json:"kind"`

	// NameExpression is a CEL expression that resolves the name of the parent context resource.
	// Must return a string value that identifies the specific parent context instance.
	// Maximum 512 characters.
	//
	// Available variables:
	// - object: The trigger resource being evaluated (complete object)
	//
	// Common expression patterns:
	// - object.spec.organization (direct field reference)
	// - object.metadata.labels["parent-org"] (label-based resolution)
	// - object.metadata.namespace.split("-")[0] (derived from namespace naming)
	//
	// Examples:
	// - "acme-corp" (literal parent name)
	// - object.spec.parentOrganization (field from trigger resource)
	// - object.metadata.labels["quota.miloapis.com/organization"] (label value)
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=512
	NameExpression string `json:"nameExpression"`
}

// ResourceGrantTemplate defines the template for creating ResourceGrants using actual ResourceGrant structure.
type ResourceGrantTemplate struct {
	// Metadata for the created ResourceGrant.
	// String fields support Go template syntax.
	//
	// +kubebuilder:validation:Required
	Metadata ObjectMetaTemplate `json:"metadata"`
	// Spec for the created ResourceGrant.
	// String fields support Go template syntax.
	//
	// +kubebuilder:validation:Required
	Spec ResourceGrantSpec `json:"spec"`
}

// ObjectMetaTemplate defines a minimal, templatable subset of ObjectMeta for use in templates.
// Only safe, user-controlled fields are exposed.
// ObjectMetaTemplate is declared in claimcreationpolicy_types.go

// GrantCreationPolicyStatus defines the observed state of GrantCreationPolicy.
//
// Status fields
// - conditions[type=Ready]: True when the policy is validated and active.
// - conditions[type=ParentContextReady]: True when cross‑cluster targeting is resolvable.
// - observedGeneration: Latest spec generation processed by the quota system.
//
// See also
// - [ResourceGrant](#resourcegrant): The object created by this policy.
// - [ResourceRegistration](#resourceregistration): Resource types for which grants are issued.
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

// GrantCreationPolicy automates ResourceGrant creation when observed resources meet conditions.
// Use it to provision quota based on resource lifecycle events and attributes.
//
// ### How It Works
// - Watch the kind in `spec.trigger.resource` and evaluate all `spec.trigger.constraints[]`.
// - When all constraints are true, render `spec.target.resourceGrantTemplate` and create a `ResourceGrant`.
// - Optionally target a parent control plane via `spec.target.parentContext` (CEL-resolved name) for cross-cluster allocation.
// - Templating supports variables `.trigger`, `.requestInfo`, `.user` and functions `lower`, `upper`, `title`, `default`, `contains`, `join`, `split`, `replace`, `trim`, `toInt`, `toString`.
// - Allowances (resource types and amounts) are static in `v1alpha1`.
//
// ### Works With
// - Creates [ResourceGrant](#resourcegrant) objects whose `allowances[].resourceType` must exist in a [ResourceRegistration](#resourceregistration).
// - May target a parent control plane via `spec.target.parentContext` for cross-plane quota allocation.
// - Policy readiness (`status.conditions[type=Ready]`) signals template/constraint validity.
//
// ### Status
// - `status.conditions[type=Ready]`: Policy validated and active.
// - `status.conditions[type=ParentContextReady]`: Cross‑cluster targeting is resolvable.
// - `status.observedGeneration`: Latest spec generation processed.
//
// ### Selectors and Filtering
//   - Field selectors (server-side):
//     `spec.trigger.resource.kind`, `spec.trigger.resource.apiVersion`,
//     `spec.target.parentContext.kind`, `spec.target.parentContext.apiGroup`.
//   - Label selectors (add your own):
//   - `quota.miloapis.com/trigger-kind`: `Organization`
//   - `quota.miloapis.com/environment`: `prod`
//   - Common queries:
//   - All policies for a trigger kind: label selector `quota.miloapis.com/trigger-kind`.
//   - All enabled policies: field selector `spec.enabled=true`.
//
// ### Defaults and Limits
// - Resource grant allowances are static (no expression-based amounts) in `v1alpha1`.
//
// ### Notes
// - If `ParentContextReady=False`, verify `nameExpression` and referenced attributes.
// - Disabled policies (`spec.enabled=false`) do not create grants.
//
// ### See Also
// - [ResourceGrant](#resourcegrant): The object created by this policy.
// - [ResourceRegistration](#resourceregistration): Resource types that grants must reference.
// - [ClaimCreationPolicy](#claimcreationpolicy): Creates claims at admission for enforcement.
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
// +kubebuilder:selectablefield:JSONPath=".spec.trigger.resource.kind"
// +kubebuilder:selectablefield:JSONPath=".spec.trigger.resource.apiVersion"
// +kubebuilder:selectablefield:JSONPath=".spec.target.parentContext.kind"
// +kubebuilder:selectablefield:JSONPath=".spec.target.parentContext.apiGroup"
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
// +kubebuilder:validation:XValidation:rule="!has(self.spec.enabled) || self.spec.enabled == true || size(self.spec.trigger.constraints) == 0",message="disabled policies should not have trigger constraints"
// +kubebuilder:validation:XValidation:rule="!has(self.spec.target.parentContext) || size(self.spec.target.parentContext.nameExpression) > 0",message="parent context must have a name expression"
// +kubebuilder:validation:XValidation:rule="size(self.spec.target.resourceGrantTemplate.spec.allowances) <= 20",message="maximum 20 allowances per policy"
