package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Role defines a collection of permissions that can be granted to users or groups in the Milo IAM system.
// Roles are namespaced resources that serve as the primary mechanism for defining and organizing
// permissions within the access control framework.
//
// Roles can contain two types of permissions:
// 1. Direct permissions: Explicit permissions listed in the includedPermissions field
// 2. Inherited permissions: Permissions from other roles specified in inheritedRoles
//
// The system includes predefined roles that are automatically available, and administrators
// can create custom roles tailored to specific needs. Roles support inheritance, allowing
// for hierarchical permission structures where complex roles can be built from simpler ones.
//
// Key characteristics:
// - Namespaced: Roles exist within a specific namespace/project context
// - Permission collections: Define sets of permissions using the format {service}/{resource}.{action}
// - Inheritance support: Can inherit permissions from other roles with no depth limit
// - Launch stage tracking: Indicates the stability level of the role (Early Access, Alpha, Beta, Stable, Deprecated)
// - PolicyBinding target: Referenced by PolicyBindings to grant permissions to users/groups
//
// Permission format:
// All permissions follow the format: {service}/{resource}.{action}
// Examples:
// - "compute.datumapis.com/workloads.create" - Create workloads in the compute service
// - "iam.miloapis.com/users.get" - Get user information in the IAM service
// - "storage.miloapis.com/buckets.delete" - Delete storage buckets
//
// Common usage patterns:
// - Predefined system roles: Use built-in roles for common access patterns
// - Custom business roles: Create roles that match organizational responsibilities
// - Hierarchical permissions: Use inheritance to build complex roles from simple ones
// - Environment-specific roles: Create different roles for dev, staging, production
//
// Best practices:
// - Follow principle of least privilege when defining permissions
// - Use descriptive names that clearly indicate the role's purpose
// - Leverage inheritance to avoid permission duplication
// - Set appropriate launch stages to indicate role stability
// - Group related permissions logically within roles
//
// Example - Basic role with direct permissions:
//
//	apiVersion: iam.miloapis.com/v1alpha1
//	kind: Role
//	metadata:
//	  name: workload-viewer
//	  namespace: project-alpha
//	spec:
//	  launchStage: Stable
//	  includedPermissions:
//	  - "compute.datumapis.com/workloads.read"
//	  - "compute.datumapis.com/workloads.list"
//
// Example - Role with inheritance:
//
//	apiVersion: iam.miloapis.com/v1alpha1
//	kind: Role
//	metadata:
//	  name: workload-admin
//	  namespace: project-alpha
//	spec:
//	  launchStage: Stable
//	  includedPermissions:
//	  - "compute.datumapis.com/workloads.create"
//	  - "compute.datumapis.com/workloads.delete"
//	  inheritedRoles:
//	  - name: workload-viewer
//	    namespace: project-alpha
//
// Related resources:
// - PolicyBinding: Binds this role to users/groups on specific resources
// - ProtectedResource: Defines the permissions that can be included in roles
//
// Role is the Schema for the roles API
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Display Name",type="string",JSONPath=".spec.displayName"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Launch Stage",type="string",JSONPath=".spec.launchStage"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Namespaced
type Role struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec RoleSpec `json:"spec,omitempty"`

	// +kubebuilder:default={conditions: {{type: "Ready", status: "Unknown", reason: "Unknown", message: "Waiting for control plane to reconcile", lastTransitionTime: "1970-01-01T00:00:00Z"}}}
	Status RoleStatus `json:"status,omitempty"`
}

// RoleSpec defines the desired state of Role, specifying the permissions and inheritance
// configuration that determines what actions users with this role can perform.
type RoleSpec struct {
	// IncludedPermissions defines the explicit permissions that this role grants.
	// Each permission must follow the format: {service}/{resource}.{action}
	//
	// Examples:
	// - "compute.datumapis.com/workloads.create" - Permission to create workloads
	// - "iam.miloapis.com/users.get" - Permission to read user information
	// - "storage.miloapis.com/buckets.delete" - Permission to delete storage buckets
	//
	// These permissions are in addition to any permissions inherited from other roles
	// specified in the inheritedRoles field.
	//
	// +kubebuilder:validation:Optional
	IncludedPermissions []string `json:"includedPermissions,omitempty"`

	// LaunchStage indicates the stability and maturity level of this IAM role.
	// This helps users understand whether the role is stable for production use
	// or still in development.
	//
	// Valid values:
	// - "Early Access": New role with limited availability, subject to breaking changes
	// - "Alpha": Experimental role that may change significantly
	// - "Beta": Pre-release role that is feature-complete but may have minor changes
	// - "Stable": Production-ready role with backwards compatibility guarantees
	// - "Deprecated": Role scheduled for removal, use alternatives when possible
	//
	// +kubebuilder:validation:Required
	LaunchStage string `json:"launchStage"`

	// InheritedRoles specifies other roles from which this role should inherit permissions.
	// This enables building complex roles from simpler ones and promotes reusability
	// of common permission sets.
	//
	// There is no limit to inheritance depth - roles can inherit from roles that
	// themselves inherit from other roles. The system will resolve the complete
	// permission set by following the inheritance chain.
	//
	// Each inherited role must exist in the same namespace as this role, or specify
	// a different namespace explicitly. If namespace is omitted, it defaults to
	// the current role's namespace.
	//
	// Example:
	//   inheritedRoles:
	//   - name: base-viewer  # inherits from base-viewer in same namespace
	//   - name: admin-tools
	//     namespace: milo-system  # inherits from admin-tools in milo-system namespace
	//
	// +kubebuilder:validation:Optional
	// +listType=map
	// +listMapKey=name
	InheritedRoles []ScopedRoleReference `json:"inheritedRoles,omitempty"`
}

// RoleStatus defines the observed state of Role, indicating the current status
// of the role's validation, inheritance resolution, and overall readiness.
type RoleStatus struct {
	// Parent indicates the resource name of the parent under which this role was created.
	// This field is typically used for system roles that are automatically created
	// as part of resource provisioning or service initialization.
	//
	// Example: "projects/my-project" or "organizations/my-org"
	// +kubebuilder:validation:Optional
	Parent string `json:"parent,omitempty"`

	// Conditions provide detailed status information about the Role resource.
	// The primary condition type is "Ready" which indicates whether the role
	// has been successfully validated and is ready for use in PolicyBindings.
	//
	// Common condition types:
	// - Ready: Indicates the role is validated and ready for use
	// - PermissionsValid: Indicates all specified permissions are valid
	// - InheritanceResolved: Indicates inherited roles have been successfully resolved
	//
	// Example condition:
	//   - type: Ready
	//     status: "True"
	//     reason: RoleReady
	//     message: Role successfully validated and ready for use
	//
	// +kubebuilder:validation:Optional
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// ObservedGeneration represents the most recent generation that has been
	// observed and processed by the role controller. This is used to track
	// whether the controller has processed the latest changes to the role spec.
	//
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RoleList contains a list of Role
type RoleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Role `json:"items"`
}
