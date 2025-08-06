package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GroupMembershipSpec defines the desired state of GroupMembership, establishing
// the relationship between a specific user and a group within the IAM system.
type GroupMembershipSpec struct {
	// UserRef is a reference to the User that should be a member of the specified Group.
	// Users are cluster-scoped resources, so only the name is required for identification.
	// The referenced user must exist in the cluster before the GroupMembership can be
	// successfully reconciled.
	//
	// Example: { name: "jane-doe" }
	// +kubebuilder:validation:Required
	UserRef UserReference `json:"userRef"`

	// GroupRef is a reference to the Group that the user should be added to.
	// Groups are namespaced resources, so both name and namespace are required.
	// The referenced group must exist in the specified namespace before the
	// GroupMembership can be successfully reconciled.
	//
	// Example: { name: "developers", namespace: "project-alpha" }
	// +kubebuilder:validation:Required
	GroupRef GroupReference `json:"groupRef"`
}

// UserReference contains information that points to the User being referenced.
// Since Users are cluster-scoped resources, only the name is required for identification.
type UserReference struct {
	// Name is the name of the User being referenced. This must match the metadata.name
	// of an existing User resource in the cluster.
	//
	// Example: "jane-doe"
	// +kubebuilder:validation:Required
	Name string `json:"name"`
}

// GroupReference contains information that points to the Group being referenced.
// Since Groups are namespaced resources, both name and namespace are required.
type GroupReference struct {
	// Name is the name of the Group being referenced. This must match the metadata.name
	// of an existing Group resource in the specified namespace.
	//
	// Example: "developers"
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Namespace is the namespace where the referenced Group exists. This must match
	// the metadata.namespace of an existing Group resource.
	//
	// Example: "project-alpha"
	// +kubebuilder:validation:Required
	Namespace string `json:"namespace"`
}

// GroupMembershipStatus defines the observed state of GroupMembership, indicating
// whether the user has been successfully added to the group.
type GroupMembershipStatus struct {
	// Conditions represent the latest available observations of the GroupMembership's current state.
	// The primary condition type is "Ready" which indicates whether the user has been
	// successfully added to the group and the membership is active.
	//
	// Common condition types:
	// - Ready: Indicates the user is successfully a member of the group
	// - UserFound: Indicates the referenced user exists
	// - GroupFound: Indicates the referenced group exists
	//
	// Example condition:
	//   - type: Ready
	//     status: "True"
	//     reason: MembershipActive
	//     message: User successfully added to group
	//
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:name="User",type="string",JSONPath=".spec.userRef.name"
// +kubebuilder:printcolumn:name="Group",type="string",JSONPath=".spec.groupRef.name"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// GroupMembership establishes a relationship between a User and a Group in the Milo IAM system.
// This resource is the primary mechanism for adding users to groups, enabling organized
// permission management through group-based role assignments.
//
// GroupMembership resources are namespaced and should typically be created in the same
// namespace as the target group. Each GroupMembership represents a single user-to-group
// relationship - to add multiple users to a group, create multiple GroupMembership resources.
//
// Key characteristics:
// - Namespaced: Created in the same namespace as the target group
// - One-to-one relationship: Each resource links exactly one user to one group
// - Cross-namespace references: Can reference cluster-scoped users from any namespace
// - Bidirectional effect: Affects both user's group memberships and group's member list
//
// Common usage patterns:
// - Team onboarding: Add new team members to appropriate groups
// - Role changes: Move users between groups as their responsibilities change
// - Project assignments: Add users to project-specific groups
// - Temporary access: Grant temporary group membership for specific tasks
//
// Best practices:
// - Use descriptive names that indicate the user-group relationship
// - Create memberships in the same namespace as the target group
// - Monitor membership status through conditions before relying on permissions
// - Use groups rather than direct user-role bindings for scalability
//
// Example:
//
//	apiVersion: iam.miloapis.com/v1alpha1
//	kind: GroupMembership
//	metadata:
//	  name: jane-doe-developers
//	  namespace: project-alpha
//	spec:
//	  userRef:
//	    name: jane-doe
//	  groupRef:
//	    name: developers
//	    namespace: project-alpha
//
// Related resources:
// - User: The cluster-scoped user being added to the group
// - Group: The namespaced group that will contain the user
// - PolicyBinding: Can reference the group to grant roles to all members
//
// GroupMembership is the Schema for the groupmemberships API
type GroupMembership struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GroupMembershipSpec   `json:"spec,omitempty"`
	Status GroupMembershipStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// GroupMembershipList contains a list of GroupMembership
type GroupMembershipList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GroupMembership `json:"items"`
}
