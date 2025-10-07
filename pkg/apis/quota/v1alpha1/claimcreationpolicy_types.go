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
	// Target defines how and where **ResourceClaims** should be created.
	//
	// +kubebuilder:validation:Required
	Target ClaimTargetSpec `json:"target"`
	// Enabled determines if this policy is active.
	// If false, no **ResourceClaims** will be created for matching resources.
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
	// Constraints are CEL expressions that must evaluate to true for claim creation to occur.
	// Evaluated in the admission context.
	//
	// +optional
	// +kubebuilder:validation:MaxItems=10
	Constraints []ConditionExpression `json:"constraints,omitempty"`
}

// ClaimTargetSpec defines how **ResourceClaims** are created for a matched trigger.
type ClaimTargetSpec struct {
	// ResourceClaimTemplate defines how to create **ResourceClaims**.
	// String fields support Go template syntax for dynamic content.
	//
	// +kubebuilder:validation:Required
	ResourceClaimTemplate ResourceClaimTemplate `json:"resourceClaimTemplate"`
}

// ResourceClaimTemplate defines how to create **ResourceClaims** using actual **ResourceClaim** structure.
type ResourceClaimTemplate struct {
	// Metadata for the created **ResourceClaim**.
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

// ObjectMetaTemplate defines metadata fields that support Go template rendering for created objects.
// Templates can access trigger resource data to generate dynamic names, namespaces, and annotations.
type ObjectMetaTemplate struct {
	// Name specifies the exact name for the created ResourceClaim.
	// Supports Go template syntax with access to template variables.
	// Leave empty to use GenerateName for auto-generated names.
	//
	// Template variables available:
	// - .trigger: The resource triggering claim creation
	// - .requestInfo: Request details (verb, resource, name, etc.)
	// - .user: User information (name, uid, groups, extra)
	//
	// Example: "{{.trigger.metadata.name}}-quota-claim"
	//
	// +optional
	Name string `json:"name,omitempty"`

	// GenerateName specifies a prefix for auto-generated names when Name is empty.
	// Kubernetes appends random characters to create unique names.
	// Supports Go template syntax.
	//
	// Example: "{{.trigger.spec.type}}-claim-"
	//
	// +optional
	GenerateName string `json:"generateName,omitempty"`

	// Namespace specifies where the ResourceClaim will be created.
	// Supports Go template syntax to derive namespace from trigger resource.
	// Leave empty to create in the same namespace as the trigger resource.
	//
	// Examples:
	// - "{{.trigger.metadata.namespace}}" (same namespace as trigger)
	// - "milo-system" (fixed system namespace)
	// - "{{.trigger.spec.organization}}-claims" (derived namespace)
	//
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Labels specifies static labels to apply to the created ResourceClaim.
	// Values are literal strings (no template processing).
	// The system automatically adds standard labels for policy tracking.
	//
	// Useful for:
	// - Organizing claims by policy or resource type
	// - Adding environment or tier indicators
	// - Enabling label-based queries and monitoring
	//
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Annotations specifies annotations to apply to the created ResourceClaim.
	// Values support Go template syntax for dynamic content.
	// The system automatically adds standard annotations for tracking.
	//
	// Template variables available:
	// - .trigger: The resource triggering claim creation
	// - .requestInfo: Request details
	// - .user: User information
	//
	// Examples:
	// - created-for: "{{.trigger.metadata.name}}"
	// - requested-by: "{{.user.name}}"
	// - trigger-kind: "{{.trigger.kind}}"
	//
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

// ClaimCreationPolicy automatically creates ResourceClaims during admission to enforce quota in real-time.
// Policies intercept resource creation requests, evaluate trigger conditions, and generate
// quota claims that prevent resource creation when quota limits are exceeded.
//
// ### How It Works
// 1. **Trigger Matching**: Admission webhook matches incoming resource creates against spec.trigger.resource
// 2. **Constraint Evaluation**: All CEL expressions in spec.trigger.constraints must evaluate to true
// 3. **Template Rendering**: Policy renders spec.target.resourceClaimTemplate using available template variables
// 4. **Claim Creation**: System creates the rendered ResourceClaim in the specified namespace
// 5. **Quota Evaluation**: Claim is immediately evaluated against AllowanceBucket capacity
// 6. **Admission Decision**: Original resource creation succeeds or fails based on claim result
//
// ### Policy Processing Flow
// **Enabled Policies** (spec.enabled=true):
// 1. Admission webhook receives resource creation request
// 2. Finds all ClaimCreationPolicies matching the resource type
// 3. Evaluates trigger constraints for each matching policy
// 4. Creates ResourceClaim for each policy where all constraints are true
// 5. Evaluates all created claims against quota buckets
// 6. Allows resource creation only if all claims are granted
//
// **Disabled Policies** (spec.enabled=false):
// - Completely ignored during admission processing
// - No constraints evaluated, no claims created
// - Useful for temporarily disabling quota enforcement
//
// ### Template System
// The template system transforms static ResourceClaim specifications into dynamic claims that reflect
// the context of each admission request. When a policy triggers, the template engine receives rich
// contextual information about the resource being created, the user making the request, and details
// about the admission operation itself.
//
// The most important template variable is `.trigger`, which contains the complete structure of the
// resource that triggered the policy. This includes all metadata like labels and annotations, the
// entire spec section, and any status information if the resource already exists. You can navigate
// this structure using standard template dot notation: `.trigger.metadata.name` gives you the
// resource's name, while `.trigger.spec.replicas` might tell you how many instances are requested.
//
// Authentication context comes through the `.user` variable, providing access to the requester's
// name, unique identifier, group memberships, and any additional attributes. This enables policies
// to create claims that track who requested resources and potentially apply different quota rules
// based on user attributes. The `.requestInfo` variable adds operational context like the specific
// API verb being performed and which resource type is being manipulated.
//
// Template functions help transform and manipulate these values. The `default` function proves
// particularly useful for providing fallback values when template variables might be empty.
// String manipulation functions like `lower`, `upper`, and `trim` help normalize names and values,
// while `replace` enables pattern substitution for complex naming schemes. For example, you might
// use `{{default "milo-system" .trigger.metadata.namespace}}` to place claims in a system namespace
// when the triggering resource doesn't specify one.
//
// ### CEL Expression System
// CEL expressions act as the gatekeepers that determine whether a policy should create a quota claim
// for a particular resource. These expressions have access to the same rich contextual information
// as templates but focus on making boolean decisions rather than generating content. Each expression
// must evaluate to either true (activate the policy) or false (skip this resource), and all expressions
// in a policy's constraint list must return true for the policy to trigger.
//
// The expression environment includes the triggering resource under the `trigger` variable, letting
// you examine any field in the resource's structure. This enables sophisticated filtering based on
// resource specifications, labels, annotations, or even status conditions. You might write
// `trigger.spec.tier == "premium"` to only apply quota policies to premium resources, or use
// `trigger.metadata.labels["environment"] == "prod"` to restrict enforcement to production workloads.
//
// User context through the `user` variable enables authorization-based policies. The expression
// `user.groups.exists(g, g == "admin")` would limit quota enforcement to resources created by
// administrators, while `user.name.startsWith("service-")` might target service accounts.
// Combined with resource filtering, you can create nuanced policies that apply different quota
// rules based on who is creating what types of resources in which contexts.
//
// ### Consumer Resolution
// The system automatically resolves spec.consumerRef for created claims:
// - Uses parent context resolution to find the appropriate consumer
// - Typically resolves to Organization for Project resources, Project for User resources, etc.
// - Consumer must match the ResourceRegistration.spec.consumerTypeRef for the requested resource type
//
// ### Validation and Dependencies
// **Policy Validation:**
// - Target resource type must exist and be accessible
// - All resource types in claim template must have active ResourceRegistrations
// - Consumer resolution must be resolvable for target resources
// - CEL expressions and Go templates must be syntactically valid
//
// **Runtime Dependencies:**
// - ResourceRegistration must be Active for each requested resource type
// - Triggering resource kind must be listed in ResourceRegistration.spec.claimingResources
// - AllowanceBucket must exist (created automatically when ResourceGrants are active)
//
// ### Policy Lifecycle
// 1. **Creation**: Administrator creates ClaimCreationPolicy
// 2. **Validation**: Controller validates target resource, expressions, and templates
// 3. **Activation**: Controller sets Ready=True when validation passes
// 4. **Operation**: Admission webhook uses active policies to create claims
// 5. **Updates**: Changes trigger re-validation; only Ready policies are used
//
// ### Status Conditions
// - **Ready=True**: Policy is validated and actively creating claims
// - **Ready=False, reason=ValidationFailed**: Configuration errors prevent activation (check message)
// - **Ready=False, reason=PolicyDisabled**: Policy is disabled (spec.enabled=false)
//
// ### Automatic Claim Features
// Claims created by ClaimCreationPolicy include:
// - **Standard Labels**: quota.miloapis.com/auto-created=true, quota.miloapis.com/policy=<policy-name>
// - **Standard Annotations**: quota.miloapis.com/created-by=claim-creation-plugin, timestamps
// - **Owner References**: Set to triggering resource when possible for lifecycle management
// - **Cleanup**: Automatically cleaned up when denied to prevent accumulation
//
// ### Field Constraints and Limits
// - Maximum 10 constraints per trigger (spec.trigger.constraints)
// - Static amounts only in v1alpha1 (no expression-based quota amounts)
// - Template metadata labels are literal strings (no template processing)
// - Template annotation values support templating
//
// ### Selectors and Filtering
// - **Field selectors**: spec.trigger.resource.kind, spec.trigger.resource.apiVersion, spec.enabled
// - **Recommended labels** (add manually):
//   - quota.miloapis.com/target-kind: Project
//   - quota.miloapis.com/environment: production
//   - quota.miloapis.com/tier: premium
//
// ### Common Queries
// - All policies for a resource kind: label selector quota.miloapis.com/target-kind=<kind>
// - Enabled policies only: field selector spec.enabled=true
// - Environment-specific policies: label selector quota.miloapis.com/environment=<env>
// - Failed policies: filter by status.conditions[type=Ready].status=False
//
// ### Troubleshooting
// - **Policy not triggering**: Check spec.enabled=true and status.conditions[type=Ready]=True
// - **Template errors**: Review status condition message for template syntax issues
// - **CEL expression failures**: Validate expression syntax and available variables
// - **Claims not created**: Verify trigger constraints match the incoming resource
// - **Consumer resolution errors**: Check parent context resolution and ResourceRegistration setup
//
// ### Performance Considerations
// - Policies are evaluated synchronously during admission (affects API latency)
// - Complex CEL expressions can impact admission performance
// - Template rendering occurs for every matching admission request
// - Consider using specific trigger constraints to limit policy evaluation scope
//
// ### Security Considerations
// - Templates can access complete trigger resource data (sensitive field exposure)
// - CEL expressions have access to user information and request details
// - Only trusted administrators should create or modify policies
// - Review template output to ensure no sensitive data leakage in claim metadata
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
