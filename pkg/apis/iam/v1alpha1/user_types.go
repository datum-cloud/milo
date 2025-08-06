package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// User represents an individual identity in the Milo IAM system. Users are cluster-scoped
// resources that exist globally across the entire Milo deployment and serve as the foundation
// for identity and access management.
//
// Users are automatically created when a person authenticates or registers with the Milo
// platform for the first time, though they can also be created manually by administrators.
// Each user is uniquely identified by their email address and integrates with external
// identity providers for authentication.
//
// Key characteristics:
// - Cluster-scoped: Users exist globally and can be referenced from any namespace
// - Email-based identity: Each user is uniquely identified by their email address
// - Automatic lifecycle: Created during first authentication/registration
// - Cross-namespace access: Can be granted permissions across different projects/namespaces
//
// Common usage patterns:
// - New user onboarding when team members join
// - Permission management through groups or direct role bindings
// - Audit trails for tracking user activities across the system
// - Identity foundation for all IAM operations
//
// Example:
//
//	apiVersion: iam.miloapis.com/v1alpha1
//	kind: User
//	metadata:
//	  name: jane-doe
//	spec:
//	  email: jane.doe@company.com
//	  givenName: Jane
//	  familyName: Doe
//
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Email",type="string",JSONPath=".spec.email"
// +kubebuilder:printcolumn:name="Given Name",type="string",JSONPath=".spec.givenName"
// +kubebuilder:printcolumn:name="Family Name",type="string",JSONPath=".spec.familyName"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:path=users,scope=Cluster
type User struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UserSpec   `json:"spec,omitempty"`
	Status UserStatus `json:"status,omitempty"`
}

// UserSpec defines the desired state of User, containing the core identity information
// that uniquely identifies and describes a user in the system.
type UserSpec struct {
	// Email is the unique email address that identifies this user in the system.
	// This field is required and serves as the primary identifier for the user.
	// The email must be unique across all users in the cluster.
	//
	// Example: "jane.doe@company.com"
	// +kubebuilder:validation:Required
	Email string `json:"email"`

	// GivenName is the user's first name or given name. This field is optional
	// and is used for display purposes and user identification in UI contexts.
	//
	// Example: "Jane"
	// +kubebuilder:validation:Optional
	GivenName string `json:"givenName,omitempty"`

	// FamilyName is the user's last name or family name. This field is optional
	// and is used for display purposes and user identification in UI contexts.
	//
	// Example: "Doe"
	// +kubebuilder:validation:Optional
	FamilyName string `json:"familyName,omitempty"`
}

// UserStatus defines the observed state of User, indicating the current status
// of the user's synchronization with external systems and overall readiness.
type UserStatus struct {
	// Conditions provide detailed status information about the User resource.
	// The primary condition type is "Ready" which indicates whether the user
	// has been successfully synchronized with the authentication provider and
	// is ready for use in the IAM system.
	//
	// Common condition types:
	// - Ready: Indicates the user is properly synchronized and available
	// - Synced: Indicates successful synchronization with external auth provider
	//
	// Example condition:
	//   - type: Ready
	//     status: "True"
	//     reason: UserReady
	//     message: User successfully synchronized with auth provider
	//
	// +kubebuilder:default={{type: "Ready", status: "Unknown", reason: "Unknown", message: "Waiting for control plane to reconcile", lastTransitionTime: "1970-01-01T00:00:00Z"}}
	// +kubebuilder:validation:Optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// UserList contains a list of User
type UserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []User `json:"items"`
}
