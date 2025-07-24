package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ProtectedResourceSpec defines the desired state of ProtectedResource, specifying
// how a resource type should be registered with the IAM system and what permissions
// are available for instances of that resource type.
type ProtectedResourceSpec struct {
	// ServiceRef identifies the service that owns this protected resource type.
	// This creates a logical grouping of related resource types under their
	// owning service, helping with organization and management. The service name
	// should be the API group of the service.
	//
	// Example:
	//   serviceRef:
	//     name: compute.datumapis.com
	//
	// +kubebuilder:validation:Required
	ServiceRef ServiceReference `json:"serviceRef"`

	// Kind specifies the Kubernetes-style kind name for this resource type.
	// This should match the kind field used in the actual resource definitions
	// and follow PascalCase naming conventions.
	//
	// Examples: "Workload", "Database", "StorageBucket"
	// +kubebuilder:validation:Required
	Kind string `json:"kind"`

	// Singular specifies the singular form of the resource name, used in API
	// paths and CLI commands. This should follow camelCase naming conventions
	// and be the lowercase, singular version of the Kind.
	//
	// Examples: "workload", "database", "storageBucket"
	// +kubebuilder:validation:Required
	Singular string `json:"singular"`

	// Plural specifies the plural form of the resource name, used in API paths
	// and resource listings. This should follow camelCase naming conventions
	// and be the lowercase, plural version of the Kind.
	//
	// Examples: "workloads", "databases", "storageBuckets"
	// +kubebuilder:validation:Required
	Plural string `json:"plural"`

	// ParentResources defines the resource types that can serve as parents to
	// this resource type in the permission hierarchy. When permissions are
	// granted on a parent resource, they can be inherited by child resources.
	//
	// This enables powerful permission models where, for example, granting
	// permissions on an Organization automatically applies to all Projects
	// within that organization, and all resources within those projects.
	//
	// Each parent resource reference must specify the apiGroup and kind of
	// the parent resource type. The parent resource types must also be
	// registered as ProtectedResources for the inheritance to work properly.
	//
	// Example hierarchy: Project -> Workload
	//   parentResources:
	//   - apiGroup: resourcemanager.miloapis.com
	//     kind: Project
	//
	// +kubebuilder:validation:Optional
	ParentResources []ParentResourceRef `json:"parentResources,omitempty"`

	// Permissions defines the complete set of permissions that can be granted
	// on instances of this resource type. Each permission should follow the
	// standard format: {service}/{resource}.{action}
	//
	// These permissions become available for use in Role definitions and
	// determine what actions users can perform on resources of this type
	// when granted appropriate roles through PolicyBindings.
	//
	// Common permission patterns:
	// - CRUD operations: create, read, update, delete
	// - Listing operations: list
	// - Administrative operations: admin, manage
	// - Resource-specific operations: scale, backup, restore, etc.
	//
	// Examples:
	//   permissions:
	//   - "compute.datumapis.com/workloads.create"
	//   - "compute.datumapis.com/workloads.get"
	//   - "compute.datumapis.com/workloads.update"
	//   - "compute.datumapis.com/workloads.delete"
	//   - "compute.datumapis.com/workloads.list"
	//   - "compute.datumapis.com/workloads.scale"
	//   - "compute.datumapis.com/workloads.logs"
	//
	// +kubebuilder:validation:Required
	Permissions []string `json:"permissions"`
}

// ProtectedResourceStatus defines the observed state of ProtectedResource, indicating
// whether the resource type has been successfully registered with the IAM system.
type ProtectedResourceStatus struct {
	// Conditions provide detailed status information about the ProtectedResource registration.
	// The primary condition type is "Ready" which indicates whether the resource type
	// has been successfully registered and is available for use in the IAM system.
	//
	// Common condition types:
	// - Ready: Indicates the resource type is registered and available for protection
	// - ServiceValid: Indicates the referenced service exists
	// - PermissionsValid: Indicates all specified permissions follow the correct format
	// - ParentResourcesValid: Indicates all parent resource references are valid
	//
	// Example condition:
	//   - type: Ready
	//     status: "True"
	//     reason: ResourceRegistered
	//     message: Resource type successfully registered with IAM system
	//
	// +kubebuilder:default={{type: "Ready", status: "Unknown", reason: "Unknown", message: "Waiting for control plane to reconcile", lastTransitionTime: "1970-01-01T00:00:00Z"}}
	// +kubebuilder:validation:Optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration represents the most recent generation that has been
	// observed and processed by the ProtectedResource controller. This corresponds
	// to the resource's metadata.generation and is used to track whether the
	// controller has processed the latest changes to the spec.
	//
	// +kubebuilder:validation:Optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ProtectedResource registers a resource type with the Milo IAM system, making it available
// for access control through roles and policy bindings. This is a cluster-scoped resource
// that defines which resource types can be protected by the IAM system and what permissions
// are available for those resources.
//
// ProtectedResources serve as the registry that makes the IAM system aware of different
// resource types that exist in the platform. By registering a resource type, system
// administrators define the complete set of permissions that can be granted on instances
// of that resource type, enabling fine-grained access control.
//
// Key characteristics:
// - Cluster-scoped: ProtectedResources exist globally across the control plane
// - Administrator-managed: Typically created by system administrators, not end users
// - Permission registry: Defines all possible permissions for a resource type
// - Hierarchy support: Can specify parent resources to enable permission inheritance
// - Service integration: Links resources to their owning services for organization
//
// Permission inheritance through parent resources:
// When parent resources are specified, permissions can be granted at higher levels
// in the resource hierarchy and automatically apply to child resources. For example,
// granting permissions on an Organization can automatically apply to all Projects
// within that organization.
//
// Common usage patterns:
// - New service integration: Register resource types when adding new services to Milo
// - Permission modeling: Define the complete permission set for each resource type
// - Hierarchy establishment: Set up parent-child relationships between resource types
// - Access control preparation: Make resources available for PolicyBinding targeting
//
// Best practices:
// - Use consistent permission naming across similar resource types
// - Define comprehensive permission sets that cover all necessary operations
// - Establish clear parent-child relationships for logical permission inheritance
// - Link resources to appropriate services for proper organization
// - Document permission semantics for developers and administrators
//
// Example - Register a Workload resource type:
//
//	apiVersion: iam.miloapis.com/v1alpha1
//	kind: ProtectedResource
//	metadata:
//	  name: workloads
//	spec:
//	  serviceRef:
//	    name: compute.datumapis.com
//	  kind: Workload
//	  singular: workload
//	  plural: workloads
//	  permissions:
//	  - "compute.datumapis.com/workloads.create"
//	  - "compute.datumapis.com/workloads.get"
//	  - "compute.datumapis.com/workloads.update"
//	  - "compute.datumapis.com/workloads.delete"
//	  - "compute.datumapis.com/workloads.list"
//	  - "compute.datumapis.com/workloads.scale"
//	  parentResources:
//	  - apiGroup: resourcemanager.miloapis.com
//	    kind: Project
//
// Example - Register a Database resource with organization-level inheritance:
//
//	apiVersion: iam.miloapis.com/v1alpha1
//	kind: ProtectedResource
//	metadata:
//	  name: databases
//	spec:
//	  serviceRef:
//	    name: sql.datumapis.com
//	  kind: Database
//	  singular: database
//	  plural: databases
//	  permissions:
//	  - "sql.datumapis.com/databases.create"
//	  - "sql.datumapis.com/databases.read"
//	  - "sql.datumapis.com/databases.update"
//	  - "sql.datumapis.com/databases.delete"
//	  - "sql.datumapis.com/databases.backup"
//	  - "sql.datumapis.com/databases.restore"
//	  parentResources:
//	  - apiGroup: resourcemanager.miloapis.com
//	    kind: Project
//
// Related resources:
// - Role: Can include permissions defined in ProtectedResource
// - PolicyBinding: Can target resource types registered as ProtectedResource
//
// ProtectedResource is the Schema for the protectedresources API
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Kind",type="string",JSONPath=".spec.kind"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:path=protectedresources,scope=Cluster,singular=protectedresource
type ProtectedResource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProtectedResourceSpec   `json:"spec,omitempty"`
	Status ProtectedResourceStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ProtectedResourceList contains a list of ProtectedResource
type ProtectedResourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProtectedResource `json:"items"`
}

// ParentResourceRef defines the reference to a parent resource
type ParentResourceRef struct {
	// APIGroup is the group for the resource being referenced.
	// If APIGroup is not specified, the specified Kind must be in the core API group.
	// For any other third-party types, APIGroup is required.
	// +kubebuilder:validation:Optional
	APIGroup string `json:"apiGroup,omitempty"`
	// Kind is the type of resource being referenced.
	// +kubebuilder:validation:Required
	Kind string `json:"kind"`
}

// ServiceReference holds a reference to a service definition.
type ServiceReference struct {
	// Name is the resource name of the service definition.
	// +kubebuilder:validation:Required
	Name string `json:"name"`
}
