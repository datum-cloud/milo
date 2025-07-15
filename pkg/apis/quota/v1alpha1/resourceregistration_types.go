package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ResourceRegistrationSpec defines the desired state of ResourceRegistration.
type ResourceRegistrationSpec struct {
	// Reference to the resource that owns the registration
	//
	// +kubebuilder:validation:Required
	OwnerRef OwnerRef `json:"ownerRef"`
	// Type of resource being registered (Entity, Allocation).
	//
	// +kubebuilder:validation:Enum=Entity;Allocation
	// +kubebuilder:validation:Required
	Type string `json:"type"`
	// Fully qualified name of the resource type being registered
	//
	// This must match the actual resource type name used by the owning service.
	// Format: apiGroup/ResourceType or apiGroup/resourceType/subResource
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^[a-z]([-a-z]*[a-z])?(\.[a-z]([-a-z]*[a-z])?)*\/[a-zA-Z][a-zA-Z]*(\/*[a-zA-Z][a-zA-Z]*)*$`
	ResourceTypeName string `json:"resourceTypeName"`
	// Human-readable description of the registration
	//
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MaxLength=500
	// +kubebuilder:validation:MinLength=1
	Description string `json:"description,omitempty"`
	// Base unit of measurement for the resource (e.g., "projects",
	// "millicores", "bytes")
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=50
	BaseUnit string `json:"baseUnit"`
	// Unit of measurement that user interfaces should present
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=50
	DisplayUnit string `json:"displayUnit"`
	// Factor to convert baseUnit to displayUnit (e.g., 1073741824 for bytes to GiB).
	// Must be a positive integer. Formula: displayValue = baseValue /
	// unitConversionFactor
	//
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Required
	UnitConversionFactor int64 `json:"unitConversionFactor"`
	// List of dimension names that can be used in ResourceGrant selectors for this resource type.
	// Each dimension should be a fully qualified name matching the pattern:
	// apiGroup/resourceTypeName
	//
	// +kubebuilder:validation:Optional
	Dimensions []string `json:"dimensions,omitempty"`
}

// ResourceRegistrationStatus defines the observed state of ResourceRegistration.
type ResourceRegistrationStatus struct {
	// Most recent generation observed by the controller.
	//
	// +kubebuilder:validation:Optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Current status conditions.
	// Known condition types: "Active"
	// below marker ensures controllers set a correct and standardized status
	// and an external client can't set the status to bypass validation.
	//
	// +kubebuilder:validation:XValidation:rule="self.all(c, c.type == 'Active' ? c.reason in ['RegistrationActive', 'RegistrationInactive', 'ValidationFailed', 'RegistrationPending'] : true)",message="Active condition reason must be valid"
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// Condition type constants for ResourceRegistration
const (
	// ResourceRegistrationActive indicates that the resource registration is active and available for use.
	ResourceRegistrationActive = "Active"
)

// Condition reason constants for ResourceRegistration
const (
	// ResourceRegistrationActiveReason indicates that the registration is active and available.
	ResourceRegistrationActiveReason = "RegistrationActive"
	// ResourceRegistrationInactiveReason indicates that the registration is inactive.
	ResourceRegistrationInactiveReason = "RegistrationInactive"
	// ResourceRegistrationValidationFailedReason indicates that validation failed.
	ResourceRegistrationValidationFailedReason = "ValidationFailed"
	// ResourceRegistrationPendingReason indicates that the registration is pending activation.
	ResourceRegistrationPendingReason = "RegistrationPending"
)

// ResourceRegistration is the Schema for the resourceregistrations API.
// Defines quotable resource types, enabling them for quota management
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
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
