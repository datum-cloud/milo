// +kubebuilder:object:generate=true
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OrganizationSpec defines the desired state of Organization, specifying the
// business characteristics that determine how the organization operates.
//
// +k8s:protobuf=true
type OrganizationSpec struct {
	// Type specifies the business model for this organization.
	// This field determines resource limits, billing, and available features.
	// 
	// Choose "Personal" for individual users and small projects.
	// Choose "Standard" for teams and business use cases.
	//
	// Warning: The type cannot be changed after organization creation.
	//
	// Example: "Standard"
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=Personal;Standard
	// +kubebuilder:validation:XValidation:rule="type(oldSelf) == null_type || self == oldSelf",message="organization type is immutable"
	Type string `json:"type"`
}

// OrganizationStatus defines the observed state of Organization, indicating
// whether the organization has been successfully created and is ready for use.
//
// +k8s:protobuf=true
type OrganizationStatus struct {
	// ObservedGeneration tracks the most recent organization spec that the
	// controller has processed. Use this to determine if status reflects
	// the latest changes.
	//
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions describe the current state of organization provisioning.
	// Check the "Ready" condition to determine if the organization is
	// available for creating projects and adding members.
	//
	// Common condition types:
	// - Ready: Organization is provisioned and ready for use
	//
	// Example ready condition:
	//   - type: Ready
	//     status: "True"
	//     reason: OrganizationReady
	//     message: Organization successfully created
	//
	// +kubebuilder:default={{type: "Ready", status: "Unknown", reason: "Unknown", message: "Waiting for control plane to reconcile", lastTransitionTime: "1970-01-01T00:00:00Z"}}
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:protobuf=true

// +kubebuilder:subresource:status
// Use lowercase for path, which influences plural name. Ensure kind is Organization.
// +kubebuilder:resource:path=organizations,scope=Cluster,categories=datum,singular=organization
// +kubebuilder:printcolumn:name="Display Name",type="string",JSONPath=".metadata.annotations.kubernetes\\.io\\/display-name"
// +kubebuilder:printcolumn:name="Type",type="string",JSONPath=".spec.type"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=".metadata.creationTimestamp"

// Organization represents the top-level tenant boundary in Milo's control plane
// for consumers of services. Organizations provide complete isolation and serve
// as the root of the resource hierarchy for access control and resource management.
//
// Organizations are cluster-scoped resources that automatically create an
// associated namespace named "organization-{name}" for organizing related
// resources. All projects must be owned by an organization.
//
// Choose the organization type based on your use case:
// - Personal: Individual developers and small projects
// - Standard: Teams, businesses, and production workloads
//
// Key characteristics:
// - Cluster-scoped: Organizations exist globally across the Milo deployment
// - Immutable type: Organization type cannot be changed after creation
// - Automatic namespacing: Creates "organization-{name}" namespace
// - Resource hierarchy root: Contains projects and user memberships
// - Tenant isolation: Complete isolation between different organizations
//
// Common workflows:
// 1. Create organization for your team or business
// 2. Add organization members using OrganizationMembership resources
// 3. Create projects within the organization
// 4. Deploy resources within organization projects
//
// Prerequisites:
// - None (organizations are the root of the resource hierarchy)
//
// Example - Personal organization:
//
//	apiVersion: resourcemanager.miloapis.com/v1alpha1
//	kind: Organization
//	metadata:
//	  name: jane-doe-personal
//	  annotations:
//	    kubernetes.io/display-name: "Jane's Personal Projects"
//	spec:
//	  type: Personal
//
// Example - Standard business organization:
//
//	apiVersion: resourcemanager.miloapis.com/v1alpha1
//	kind: Organization
//	metadata:
//	  name: acme-corp
//	  annotations:
//	    kubernetes.io/display-name: "ACME Corporation"
//	spec:
//	  type: Standard
//
// Related resources:
// - Project: Projects must be owned by an organization
// - OrganizationMembership: Links users to organizations
// - IAM resources: Inherit permissions from organization level
//
// Troubleshooting:
// - Check the Ready condition in status to verify successful creation
// - List all organizations to verify creation and status
// - Display names are set via the kubernetes.io/display-name annotation
//
// Organization is the Schema for the Organizations API
// +kubebuilder:object:root=true
type Organization struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:Required
	Spec   OrganizationSpec   `json:"spec,omitempty"`
	Status OrganizationStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:protobuf=true

// +kubebuilder:object:root=true
// OrganizationList contains a list of Organization
type OrganizationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Organization `json:"items"`
}
