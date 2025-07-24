package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ProjectSpec defines the configuration for a project, specifying which
// organization owns it.
//
// +k8s:protobuf=true
type ProjectSpec struct {
	// OwnerRef references the organization that owns this project.
	// Projects must be owned by an organization and inherit its permissions.
	//
	// The organization must exist before creating the project.
	// Currently only Organization resources are supported as owners.
	//
	// Example:
	//   ownerRef:
	//     kind: Organization
	//     name: acme-corp
	//
	// +kubebuilder:validation:Required
	OwnerRef OwnerReference `json:"ownerRef"`
}

// ProjectStatus defines the observed state of Project, indicating whether
// the project has been successfully provisioned and is ready for use.
//
// +k8s:protobuf=true
type ProjectStatus struct {
	// Conditions describe the current state of project provisioning.
	// Check the "Ready" condition to determine if the project is
	// available for deploying resources.
	//
	// Common condition types:
	// - Ready: Project is provisioned and ready for use
	//
	// Example ready condition:
	//   - type: Ready
	//     status: "True"
	//     reason: ProjectReady
	//     message: Project successfully provisioned
	//
	// +kubebuilder:default={{type: "Ready", status: "Unknown", reason: "Unknown", message: "Waiting for control plane to reconcile", lastTransitionTime: "1970-01-01T00:00:00Z"}}
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

const (
	// ProjectReady indicates that the project has been provisioned and is ready
	// for use.
	ProjectReady = "Ready"
)

const (
	// ProjectReadyReason indicates that the project is ready for use.
	ProjectReadyReason = "Ready"

	// ProjectProvisioningReason indicates that the project is provisioning.
	ProjectProvisioningReason = "Provisioning"

	// ProjectNameConflict indicates that the project name already exists
	ProjectNameConflictReason = "ProjectNameConflict"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster

// Project represents a logical container for related resources within an
// organization. Projects provide resource organization and access control
// boundaries for your applications and workloads.
//
// Projects are cluster-scoped resources that must be owned by an organization.
// They serve as the primary unit for organizing and managing resources in Milo.
//
// Key characteristics:
// - Cluster-scoped: Projects exist globally across the Milo deployment
// - Organization-owned: Each project must reference a valid organization
// - Resource container: Groups related resources for management
// - Access control boundary: Inherits permissions from the owning organization
//
// Common workflows:
// 1. Ensure the owning organization exists and is ready
// 2. Create the project with a reference to the organization
// 3. Wait for the Ready condition to become True
// 4. Deploy your applications and resources within the project
//
// Prerequisites:
// - Organization: The referenced organization must exist and be ready
//
// Example - Development project:
//
//	apiVersion: resourcemanager.miloapis.com/v1alpha1
//	kind: Project
//	metadata:
//	  name: web-app-dev
//	  annotations:
//	    kubernetes.io/display-name: "Web App Development"
//	spec:
//	  ownerRef:
//	    kind: Organization
//	    name: acme-corp
//
// Example - Production project:
//
//	apiVersion: resourcemanager.miloapis.com/v1alpha1
//	kind: Project
//	metadata:
//	  name: web-app-prod
//	  annotations:
//	    kubernetes.io/display-name: "Web App Production"
//	spec:
//	  ownerRef:
//	    kind: Organization
//	    name: acme-corp
//
// Related resources:
// - Organization: Must exist before creating projects
// - IAM resources: Projects inherit permissions from organizations
//
// Troubleshooting:
// - Check the Ready condition in status to verify successful provisioning
// - Ensure the referenced organization exists and is ready
// - List all projects to verify creation and status
// - Display names are set via the kubernetes.io/display-name annotation
//
// Project is the Schema for the projects API.
// +kubebuilder:printcolumn:name="Owner",type="string",JSONPath=".spec.ownerRef.name"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type Project struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:Required
	Spec   ProjectSpec   `json:"spec,omitempty"`
	Status ProjectStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ProjectList contains a list of Project.
type ProjectList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Project `json:"items"`
}

// OwnerReference identifies the organization that owns a project.
// Projects inherit permissions and billing from their owning organization.
type OwnerReference struct {
	// Kind specifies the type of resource that owns this project.
	// Currently only "Organization" is supported.
	//
	// Example: "Organization"
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=Organization
	Kind string `json:"kind"`

	// Name is the name of the organization that owns this project.
	// The organization must exist before creating the project.
	//
	// Example: "acme-corp"
	//
	// +kubebuilder:validation:Required
	Name string `json:"name"`
}
