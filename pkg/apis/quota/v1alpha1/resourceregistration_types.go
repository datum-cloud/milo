package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ObjectRef struct {
	APIGroup string `json:"apiGroup"`
	Kind     string `json:"kind"`
}

// ResourceRegistrationSpec defines the desired state of ResourceRegistration.
type ResourceRegistrationSpec struct {
	// Reference to the resource type that will create grant and claim objects
	// for this registration.
	// For example, if creating a registration that defines the max number of
	// "Project"s, the object ref will reference the "Organization" resource
	// type as projects are created within an organization.
	//
	// +kubebuilder:validation:Required
	ObjectRef ObjectRef `json:"objectRef"`
	// Type of resource being registered (Entity, Allocation).
	//
	// +kubebuilder:validation:Enum=Entity;Allocation
	// +kubebuilder:validation:Required
	Type string `json:"type"`
	// Fully qualified name of the resource type being registered
	//
	// Format: apiGroup/ResourceType or apiGroup/resourceType/subResource
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^[a-z]([-a-z]*[a-z])?(\.[a-z]([-a-z]*[a-z])?)*\/[a-zA-Z][a-zA-Z]*(\/*[a-zA-Z][a-zA-Z]*)*$`
	ResourceTypeName string `json:"resourceTypeName"`
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
	// matching the pattern: apiGroup/resourceTypeName
	//
	// +kubebuilder:validation:Optional
	Dimensions []string `json:"dimensions,omitempty"`
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
