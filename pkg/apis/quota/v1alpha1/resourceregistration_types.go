package v1alpha1

import (
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OwnerTypeRef defines the resource type that owns the registration,
// and the grants and claims that will be created for it.
type ConsumerTypeRef struct {
	// API group of the resource type that owns the registration
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`
	APIGroup string `json:"apiGroup"`

	// Resource type that owns the registration.
	//
	// +kubebuilder:validation:Required
	Kind string `json:"kind"`
}

// ResourceRegistrationSpec defines the desired state of ResourceRegistration.
type ResourceRegistrationSpec struct {
	// Reference to the owning resource type that will create grant and claim objects
	// for this registration.
	// For example, if creating a registration that defines the max number of
	// Projects per Organization, the OwnerTypeRef would be the Organization resource type.
	// No Name field is included as ResourceRegistrations are cluster-scoped and
	// not owned by any specific object instance.
	//
	// +kubebuilder:validation:Required
	ConsumerTypeRef ConsumerTypeRef `json:"consumerTypeRef"`
	// Type is the type of registration (Entity, Allocation).
	//
	// +kubebuilder:validation:Enum=Entity;Allocation
	// +kubebuilder:validation:Required
	Type string `json:"type"`
	// Fully qualified name of the resource type being registered
	//
	// Format: apiGroup/ResourceType or apiGroup/resourceType/subResource
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^[a-z]([-a-z]*[a-z])?(\.[a-z]([-a-z]*[a-z])?)*\/[a-zA-Z][a-zA-Z]*(\/*[a-zA-Z][a-zA-Z]*)*$`
	ResourceType string `json:"resourceType"`
	// Human-readable description of the registration
	//
	// +kubebuilder:validation:Optional +kubebuilder:validation:MaxLength=500
	// +kubebuilder:validation:MinLength=1
	Description string `json:"description,omitempty"`
	// Base unit of measurement for the resource (e.g., "projects",
	// "millicores", "bytes")
	//
	// +kubebuilder:validation:Required +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=50
	BaseUnit string `json:"baseUnit"`
	// Unit of measurement that user interfaces should present
	//
	// +kubebuilder:validation:Required +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=50
	DisplayUnit string `json:"displayUnit"`
	// Factor to convert baseUnit to displayUnit (e.g., 1073741824 for bytes to
	// GiB). Must be a positive integer. Formula: displayValue = baseValue /
	// unitConversionFactor
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=1
	UnitConversionFactor int64 `json:"unitConversionFactor"`
	// List of dimension names that can be used in ResourceGrant selectors for
	// this resource type. Each dimension should be a fully qualified name
	// matching the pattern: apiGroup/resourceType
	//
	// +kubebuilder:validation:Optional
	Dimensions []string `json:"dimensions,omitempty"`
	// ClaimingResources defines which resource types are allowed to create
	// ResourceClaims for this registered resource type. When a ResourceClaim is
	// created with a ResourceRef, that ResourceRef's type must be in this list
	// for the claim to be valid. If not specified, defaults to allowing the
	// resource type itself (e.g., Projects can claim Project quota).
	//
	// This field also signals to the ownership controller which resource types
	// should be watched for immediate owner reference creation.
	//
	// Uses unversioned references to allow API version upgrades without
	// needing to update the ResourceRegistration.
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

// ResourceRegistrationStatus defines the observed state of
// ResourceRegistration.
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

// ResourceRegistration is the Schema for the resourceregistrations API.
// Defines quotable resource types, enabling them for quota management.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:subresource:status
// +k8s:openapi-gen=true
// +kubebuilder:printcolumn:name="Type",type="string",JSONPath=".spec.type"
// +kubebuilder:printcolumn:name="Active",type="string",JSONPath=".status.conditions[?(@.type=='Active')].status"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
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
