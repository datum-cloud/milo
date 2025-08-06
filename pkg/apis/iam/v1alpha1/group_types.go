package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GroupStatus defines the observed state of Group, tracking the readiness and
// synchronization status of the group resource.
type GroupStatus struct {
	// Conditions represent the latest available observations of a group's current state.
	// The primary condition type is "Ready" which indicates whether the group
	// is properly initialized and ready for use in the IAM system.
	//
	// Common condition types:
	// - Ready: Indicates the group is available for membership operations
	//
	// Example condition:
	//   - type: Ready
	//     status: "True"
	//     reason: GroupReady
	//     message: Group successfully created and ready for members
	//
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// Group represents a collection of users for simplified permission management in the Milo IAM system.
// Groups are namespaced resources that serve as containers for organizing users with similar access needs.
//
// Groups themselves have no configuration options - they exist purely as organizational units.
// Users are added to groups through GroupMembership resources, which create the actual relationship
// between users and groups. Groups cannot be nested within other groups in the current implementation,
// though this may be supported in future versions.
//
// Key characteristics:
// - Namespaced: Groups exist within a specific namespace/project context
// - User organization: Primary purpose is to organize users for easier permission management
// - No direct configuration: Groups have no spec fields, only metadata and status
// - PolicyBinding target: Groups can be referenced in PolicyBindings to grant roles to all members
//
// Common usage patterns:
// - Team organization (e.g., "developers", "qa-team", "project-managers")
// - Role-based groupings (e.g., "admins", "viewers", "editors")
// - Department-based access (e.g., "engineering", "marketing", "finance")
// - Project-specific teams (e.g., "project-alpha-team", "infrastructure-team")
//
// Best practices:
// - Use descriptive names that clearly indicate the group's purpose
// - Organize groups by function or team rather than individual permissions
// - Bind roles to groups rather than individual users for easier management
// - Use groups consistently across projects for similar roles
//
// Example:
//
//	apiVersion: iam.miloapis.com/v1alpha1
//	kind: Group
//	metadata:
//	  name: developers
//	  namespace: project-alpha
//	  annotations:
//	    description: "Developers working on project alpha with read/write access"
//
// Related resources:
// - GroupMembership: Links users to this group
// - PolicyBinding: Can reference this group as a subject for role assignments
//
// Group is the Schema for the groups API
type Group struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Status GroupStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// GroupList contains a list of Group
type GroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Group `json:"items"`
}
