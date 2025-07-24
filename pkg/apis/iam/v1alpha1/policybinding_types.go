package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RoleReference contains information that points to the Role being used
// +k8s:deepcopy-gen=true
type RoleReference struct {
	// Name is the name of resource being referenced
	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// Namespace of the referenced Role. If empty, it is assumed to be in the PolicyBinding's namespace.
	// +kubebuilder:validation:Optional
	Namespace string `json:"namespace,omitempty"`
}

// Subject contains a reference to the object or user identities a role binding applies to.
// This can be a User or Group.
// +k8s:deepcopy-gen=true
// +kubebuilder:validation:XValidation:rule="(self.kind == 'Group' && has(self.name) && self.name.startsWith('system:')) || (has(self.uid) && size(self.uid) > 0)",message="UID is required for all subjects except system groups (groups with names starting with 'system:')"
type Subject struct {
	// Kind of object being referenced. Values defined in Kind constants.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=User;Group
	Kind string `json:"kind"`
	// Name of the object being referenced. A special group name of
	// "system:authenticated-users" can be used to refer to all authenticated
	// users.
	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// Namespace of the referenced object. If DNE, then for an SA it refers to the PolicyBinding resource's namespace.
	// For a User or Group, it is ignored.
	// +kubebuilder:validation:Optional
	Namespace string `json:"namespace,omitempty"`
	// UID of the referenced object. Optional for system groups (groups with names starting with "system:").
	// +kubebuilder:validation:Optional
	UID string `json:"uid,omitempty"`
}

// ResourceReference contains enough information to let you identify a specific
// API resource instance.
// +k8s:deepcopy-gen=true
type ResourceReference struct {
	// APIGroup is the group for the resource being referenced.
	// If APIGroup is not specified, the specified Kind must be in the core API group.
	// For any other third-party types, APIGroup is required.
	// +kubebuilder:validation:Optional
	APIGroup string `json:"apiGroup,omitempty"`
	// Kind is the type of resource being referenced.
	// +kubebuilder:validation:Required
	Kind string `json:"kind"`
	// Name is the name of resource being referenced.
	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// UID is the unique identifier of the resource being referenced.
	// +kubebuilder:validation:Required
	UID string `json:"uid"`
	// Namespace is the namespace of resource being referenced.
	// Required for namespace-scoped resources. Omitted for cluster-scoped resources.
	// +kubebuilder:validation:Optional
	Namespace string `json:"namespace,omitempty"`
}

// ResourceKind contains enough information to identify a resource type.
// +k8s:deepcopy-gen=true
type ResourceKind struct {
	// APIGroup is the group for the resource type being referenced. If APIGroup
	// is not specified, the specified Kind must be in the core API group.
	// +kubebuilder:validation:Optional
	APIGroup string `json:"apiGroup,omitempty"`

	// Kind is the type of resource being referenced.
	// +kubebuilder:validation:Required
	Kind string `json:"kind"`
}

// ResourceSelector defines which resources the policy binding applies to.
// Either resourceRef or resourceKind must be specified, but not both.
// +k8s:deepcopy-gen=true
// +kubebuilder:validation:XValidation:rule="has(self.resourceRef) != has(self.resourceKind)",message="exactly one of resourceRef or resourceKind must be specified, but not both"
type ResourceSelector struct {
	// ResourceRef provides a reference to a specific resource instance.
	// Mutually exclusive with resourceKind.
	// +kubebuilder:validation:Optional
	ResourceRef *ResourceReference `json:"resourceRef,omitempty"`

	// ResourceKind specifies that the policy binding should apply to all resources of a specific kind.
	// Mutually exclusive with resourceRef.
	// +kubebuilder:validation:Optional
	ResourceKind *ResourceKind `json:"resourceKind,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// PolicyBinding grants roles to users or groups on specific resources in the Milo IAM system.
// This is the central resource that connects the three core IAM concepts: subjects (users/groups),
// roles (permission sets), and resources (the things being protected).
//
// PolicyBindings are the mechanism through which access control is actually enforced. They
// specify which users or groups should receive which permissions (via roles) on which resources
// or resource types. This follows the "who can do what on which resource" model of access control.
//
// Key characteristics:
// - Namespaced: PolicyBindings exist within a specific namespace context
// - Immutable references: Role and resource references cannot be changed after creation
// - Flexible resource targeting: Can target specific resource instances or all resources of a type
// - Cross-namespace capability: Can reference roles from any namespace
// - Multiple subjects: Can grant the same role to multiple users/groups in a single binding
//
// Resource targeting modes:
// 1. Specific resource (resourceRef): Grants permissions on a single, specific resource instance
// 2. Resource kind (resourceKind): Grants permissions on ALL resources of a particular type
//
// Common usage patterns:
// - Project access: Grant team members access to all resources in a project
// - Resource-specific permissions: Grant access to individual workloads, databases, etc.
// - Administrative access: Grant admin roles on resource types for operational teams
// - Temporary access: Create time-limited bindings for contractor or temporary access
//
// Best practices:
// - Use groups as subjects rather than individual users for easier management
// - Prefer resource kind bindings for broad access, specific resource refs for targeted access
// - Use descriptive names that indicate the purpose of the binding
// - Regularly audit PolicyBindings to ensure appropriate access levels
// - Leverage the principle of least privilege when designing role assignments
//
// Example - Grant developers access to all workloads in a project:
//
//	apiVersion: iam.miloapis.com/v1alpha1
//	kind: PolicyBinding
//	metadata:
//	  name: developers-workload-access
//	  namespace: project-alpha
//	spec:
//	  roleRef:
//	    name: workload-developer
//	    namespace: project-alpha
//	  subjects:
//	  - kind: Group
//	    name: developers
//	  resourceSelector:
//	    resourceKind:
//	      apiGroup: compute.miloapis.com
//	      kind: Workload
//
// Example - Grant specific user access to a specific database:
//
//	apiVersion: iam.miloapis.com/v1alpha1
//	kind: PolicyBinding
//	metadata:
//	  name: alice-prod-db-access
//	  namespace: production
//	spec:
//	  roleRef:
//	    name: database-admin
//	  subjects:
//	  - kind: User
//	    name: alice-smith
//	    uid: user-123-abc
//	  resourceSelector:
//	    resourceRef:
//	      apiGroup: data.miloapis.com
//	      kind: Database
//	      name: production-primary
//	      uid: db-456-def
//	      namespace: production
//
// Related resources:
// - Role: Defines the permissions being granted
// - User/Group: The subjects receiving the permissions
// - Resource: The target resource(s) being protected
//
// PolicyBinding is the Schema for the policybindings API
// +kubebuilder:printcolumn:name="Role",type="string",JSONPath=".spec.roleRef.name"
// +kubebuilder:printcolumn:name="Resource Kind",type="string",JSONPath=".spec.resourceSelector.resourceRef.kind"
// +kubebuilder:printcolumn:name="Resource Name",type="string",JSONPath=".spec.resourceSelector.resourceRef.name"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:path=policybindings,scope=Namespaced
type PolicyBinding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec   PolicyBindingSpec   `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status PolicyBindingStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// PolicyBindingSpec defines the desired state of PolicyBinding, specifying which
// subjects (users/groups) should receive which role on which resources.
//
// This spec contains three key components that together define the complete
// access control policy:
// 1. RoleRef: The role being granted (defines the permissions)
// 2. Subjects: Who is receiving the role (users and/or groups)
// 3. ResourceSelector: What resources the role applies to (specific or by type)
//
// +k8s:deepcopy-gen=true
type PolicyBindingSpec struct {
	// RoleRef specifies the Role that should be granted to the subjects.
	// This is an immutable field that cannot be changed after the PolicyBinding
	// is created - to change the role, you must delete and recreate the binding.
	//
	// The role can exist in any namespace, enabling cross-namespace role sharing.
	// If no namespace is specified, it defaults to the PolicyBinding's namespace.
	//
	// Example:
	//   roleRef:
	//     name: workload-developer
	//     namespace: shared-roles  # optional, defaults to current namespace
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="oldSelf == null || self == oldSelf",message="RoleRef is immutable and cannot be changed after creation"
	RoleRef RoleReference `json:"roleRef"`

	// Subjects specifies the users and/or groups that should receive the role.
	// Multiple subjects can be listed to grant the same role to multiple entities
	// in a single PolicyBinding.
	//
	// Each subject must specify:
	// - kind: Either "User" or "Group"
	// - name: The name of the user or group
	// - uid: The unique identifier (required for users, optional for system groups)
	//
	// Special group "system:authenticated-users" can be used to grant access
	// to all authenticated users in the system.
	//
	// Examples:
	//   subjects:
	//   - kind: User
	//     name: alice-smith
	//     uid: user-123-abc
	//   - kind: Group
	//     name: developers
	//   - kind: Group
	//     name: system:authenticated-users  # special system group
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Subjects []Subject `json:"subjects"`

	// ResourceSelector specifies which resources the role should be applied to.
	// This is an immutable field that cannot be changed after creation.
	//
	// Exactly one of the following must be specified:
	// - resourceRef: Grants permissions on a specific resource instance
	// - resourceKind: Grants permissions on all resources of a specific type
	//
	// Use resourceRef for targeted access to individual resources.
	// Use resourceKind for broad access across all resources of a type.
	//
	// Examples:
	//   # Grant access to all workloads
	//   resourceSelector:
	//     resourceKind:
	//       apiGroup: compute.miloapis.com
	//       kind: Workload
	//
	//   # Grant access to specific workload
	//   resourceSelector:
	//     resourceRef:
	//       apiGroup: compute.miloapis.com
	//       kind: Workload
	//       name: my-workload
	//       uid: workload-456-def
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="oldSelf == null || self == oldSelf",message="ResourceSelector is immutable and cannot be changed after creation"
	ResourceSelector ResourceSelector `json:"resourceSelector"`
}

// PolicyBindingStatus defines the observed state of PolicyBinding, indicating
// whether the access control policy has been successfully applied and is active.
//
// +k8s:deepcopy-gen=true
type PolicyBindingStatus struct {
	// ObservedGeneration represents the most recent generation that has been
	// observed and processed by the PolicyBinding controller. This is used to
	// track whether the controller has processed the latest changes to the spec.
	//
	// +kubebuilder:validation:Optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions provide detailed status information about the PolicyBinding resource.
	// The primary condition type is "Ready" which indicates whether the policy
	// binding has been successfully applied and is actively enforcing access control.
	//
	// Common condition types:
	// - Ready: Indicates the policy binding is active and enforcing access
	// - RoleFound: Indicates the referenced role exists and is valid
	// - SubjectsValid: Indicates all referenced subjects (users/groups) exist
	// - ResourceValid: Indicates the target resource or resource type is valid
	//
	// Example condition:
	//   - type: Ready
	//     status: "True"
	//     reason: PolicyActive
	//     message: Policy binding successfully applied and enforcing access
	//
	// +kubebuilder:default={{type: "Ready", status: "Unknown", reason: "Unknown", message: "Waiting for control plane to reconcile", lastTransitionTime: "1970-01-01T00:00:00Z"}}
	// +kubebuilder:validation:Optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true

// PolicyBindingList contains a list of PolicyBinding
type PolicyBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PolicyBinding `json:"items"`
}
