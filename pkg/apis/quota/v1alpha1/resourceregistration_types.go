package v1alpha1

import (
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ConsumerTypeRef identifies the resource type that consumes quota.
// This type receives grants and creates claims against registered resources.
type ConsumerTypeRef struct {
	// API group of the quota consumer resource type
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`
	APIGroup string `json:"apiGroup"`
	// Resource type that consumes quota from this registration
	//
	// +kubebuilder:validation:Required
	Kind string `json:"kind"`
}

// ResourceRegistrationSpec defines the desired state of ResourceRegistration.
type ResourceRegistrationSpec struct {
	// ConsumerTypeRef identifies the resource type that receives grants and creates claims.
	// For example, when registering "Projects per Organization", the ConsumerTypeRef
	// would be Organization, which can then receive ResourceGrants allocating Project quota.
	//
	// +kubebuilder:validation:Required
	ConsumerTypeRef ConsumerTypeRef `json:"consumerTypeRef"`
	// Type classifies how the system measures this registration.
	// Entity: Tracks the count of object instances (for example, number of Projects).
	// Allocation: Tracks numeric capacity (for example, bytes of storage, CPU millicores).
	//
	// +kubebuilder:validation:Enum=Entity;Allocation
	// +kubebuilder:validation:Required
	Type string `json:"type"`
	// ResourceType identifies the Kubernetes resource to track with quota.
	// Must match an existing resource type accessible in the cluster.
	// Format: apiGroup/resource (plural), with optional subresource path
	// (for example, "resourcemanager.miloapis.com/projects" or
	// "core/pods/cpu").
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^[a-z]([-a-z]*[a-z])?(\.[a-z]([-a-z]*[a-z])?)*\/[a-zA-Z][a-zA-Z]*(\/*[a-zA-Z][a-zA-Z]*)*$`
	ResourceType string `json:"resourceType"`
	// Description provides context about what this registration tracks
	//
	// +kubebuilder:validation:Optional +kubebuilder:validation:MaxLength=500
	// +kubebuilder:validation:MinLength=1
	Description string `json:"description,omitempty"`
	// BaseUnit defines the internal measurement unit for quota calculations.
	// Examples: "projects", "millicores", "bytes"
	//
	// +kubebuilder:validation:Required +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=50
	BaseUnit string `json:"baseUnit"`
	// DisplayUnit defines the unit shown in user interfaces.
	// Examples: "projects", "cores", "GiB"
	//
	// +kubebuilder:validation:Required +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=50
	DisplayUnit string `json:"displayUnit"`
	// UnitConversionFactor converts baseUnit to displayUnit.
	// Formula: displayValue = baseValue / unitConversionFactor
	// Examples: 1 (no conversion), 1073741824 (bytes to GiB), 1000 (millicores to cores)
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=1
	UnitConversionFactor int64 `json:"unitConversionFactor"`
	// ClaimingResources specifies which resource types can create ResourceClaims
	// for this registered resource type. When a ResourceClaim includes a resourceRef,
	// the referenced resource's type must be in this list for the claim to be valid.
	// If empty, no resources can claim this quota - administrators must explicitly
	// configure which resources can claim quota for security.
	//
	// This field also signals to the ownership controller which resource types
	// to watch for automatic owner reference creation.
	//
	// Uses unversioned references to support API version upgrades without
	// requiring ResourceRegistration updates.
	//
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MaxItems=20
	ClaimingResources []ClaimingResource `json:"claimingResources,omitempty"`
}

// ClaimingResource identifies a resource type that can create ResourceClaims
// for a registered resource type using an unversioned reference.
type ClaimingResource struct {
	// APIGroup is the group for the resource being referenced.
	// If APIGroup is not specified, the specified Kind must be in the core API group.
	// For any other third-party types, APIGroup is required.
	//
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Pattern=`^$|^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`
	APIGroup string `json:"apiGroup,omitempty"`
	// Kind of the referent.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	Kind string `json:"kind"`
}

// ResourceRegistrationStatus reports whether the registration is usable and the
// latest spec generation processed. When Active, grants and claims may be created
// for the registered type. See the schema for exact fields and condition reasons.
// Related objects include [ResourceGrant](#resourcegrant) and
// [ResourceClaim](#resourceclaim).
type ResourceRegistrationStatus struct {
	// Most recent generation observed by the controller.
	//
	// +kubebuilder:validation:Optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Current status conditions. Known condition types: "Active" below marker
	// ensures controllers set a correct and standardized status and an external
	// client can't set the status to bypass validation.
	//
	// +kubebuilder:validation:XValidation:rule="self.all(c, c.type == 'Active' ? c.reason in ['RegistrationActive', 'ValidationFailed', 'RegistrationPending'] : true)",message="Active condition reason must be valid"
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// Condition type constants for ResourceRegistration
const (
	// Indicates that the resource registration is active and ResourceGrants and
	// ResourceClaims can be created to set limits and claim resources.
	ResourceRegistrationActive = "Active"
)

// Condition reason constants for ResourceRegistration
const (
	// Indicates that the registration has been validated and is active.
	ResourceRegistrationActiveReason = "RegistrationActive"
	// Indicates that the registration validation failed.
	ResourceRegistrationValidationFailedReason = "ValidationFailed"
	// Indicates that the registration was not found.
	RegistrationNotFoundReason = "RegistrationNotFound"
	// Indicates that the registration is pending validation.
	ResourceRegistrationPendingReason = "RegistrationPending"
)

// ResourceRegistration defines which resource types the quota system manages and how to measure them.
// Registrations enable grants and claims for a specific resource type, using clear units and ownership rules.
//
// ### How It Works
// - Administrators create registrations to opt resource types into quota.
// - After activation, ResourceGrants allocate capacity and ResourceClaims consume it for the type.
//
// ### Works With
// - [ResourceGrant](#resourcegrant) `allowances[].resourceType` must match `spec.resourceType`.
// - [ResourceClaim](#resourceclaim) `spec.requests[].resourceType` must match `spec.resourceType`.
// - The triggering kind must be listed in `spec.claimingResources` for claims to be valid.
// - Consumers in grants/claims must match `spec.consumerTypeRef`.
//
// ### Selectors and Filtering
// - Field selectors (server-side): `spec.consumerTypeRef.kind`, `spec.consumerTypeRef.apiGroup`, `spec.resourceType`.
// - Label selectors (add your own):
//   - `quota.miloapis.com/resource-kind`: `<Kind>`
//   - `quota.miloapis.com/resource-apigroup`: `<API group>`
//   - `quota.miloapis.com/consumer-kind`: `<Kind>`
//
// - Common queries:
//   - All registrations for a resource kind: label selector `quota.miloapis.com/resource-kind` (+ `quota.miloapis.com/resource-apigroup` when needed).
//   - All registrations for a consumer kind: label selector `quota.miloapis.com/consumer-kind`.
//
// ### Defaults and Limits
// - `spec.type`: `Entity` (count objects) or `Allocation` (numeric capacity).
// - `spec.claimingResources`: up to 20 entries; unversioned references (`apiGroup`, `kind`).
// - `spec.resourceType`: must follow `group/resource` with optional subresource path.
//
// ### Notes
// - `claimingResources` are unversioned; kind matching is case-insensitive and apiGroup must align.
// - Grants and claims use `baseUnit`; `displayUnit` and `unitConversionFactor` affect presentation only.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:subresource:status
// +k8s:openapi-gen=true
// +kubebuilder:printcolumn:name="Type",type="string",JSONPath=".spec.type"
// +kubebuilder:printcolumn:name="Active",type="string",JSONPath=".status.conditions[?(@.type=='Active')].status"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:selectablefield:JSONPath=".spec.consumerTypeRef.kind"
// +kubebuilder:selectablefield:JSONPath=".spec.consumerTypeRef.apiGroup"
// +kubebuilder:selectablefield:JSONPath=".spec.resourceType"
type ResourceRegistration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// +kubebuilder:validation:Required
	Spec   ResourceRegistrationSpec   `json:"spec"`
	Status ResourceRegistrationStatus `json:"status,omitempty"`
}

// ResourceRegistrationList contains a list of ResourceRegistration.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
type ResourceRegistrationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ResourceRegistration `json:"items"`
}

// IsClaimingResourceAllowed checks if the given resource type is allowed to claim
// quota for this registered resource type. Since ClaimingResource uses unversioned
// references, the version parameter is ignored.
func (r *ResourceRegistration) IsClaimingResourceAllowed(apiGroup, kind string) bool {
	// If no claiming resources are specified, no default is assumed
	// The ClaimingResources field must be explicitly configured
	if len(r.Spec.ClaimingResources) == 0 {
		// When not specified, deny by default for security
		// Administrators must explicitly configure which resources can claim quota
		return false
	}
	// Check against the explicit list
	for _, allowedResource := range r.Spec.ClaimingResources {
		// Check APIGroup match (empty string matches core API group)
		if allowedResource.APIGroup != apiGroup {
			continue
		}
		// Check Kind match (case-insensitive)
		if !strings.EqualFold(allowedResource.Kind, kind) {
			continue
		}
		// Match found (version agnostic)
		return true
	}

	return false
}

// MatchesReference checks if this ClaimingResource matches the given
// unversioned object reference.
func (c *ClaimingResource) MatchesReference(ref UnversionedObjectReference) bool {
	return c.APIGroup == ref.APIGroup && strings.EqualFold(c.Kind, ref.Kind)
}
