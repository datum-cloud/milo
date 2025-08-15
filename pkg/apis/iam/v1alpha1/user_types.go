package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type UserState string

const (
	UserStateActive   UserState = "Active"
	UserStateInactive UserState = "Inactive"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// User is the Schema for the users API
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Email",type="string",JSONPath=".spec.email"
// +kubebuilder:printcolumn:name="Given Name",type="string",JSONPath=".spec.givenName"
// +kubebuilder:printcolumn:name="Family Name",type="string",JSONPath=".spec.familyName"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="State",type="string",JSONPath=".status.state"
// +kubebuilder:resource:path=users,scope=Cluster
type User struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UserSpec   `json:"spec,omitempty"`
	Status UserStatus `json:"status,omitempty"`
}

// UserSpec defines the desired state of User
type UserSpec struct {
	// The email of the user.
	// +kubebuilder:validation:Required
	Email string `json:"email"`
	// The first name of the user.
	// +kubebuilder:validation:Optional
	GivenName string `json:"givenName,omitempty"`
	// The last name of the user.
	// +kubebuilder:validation:Optional
	FamilyName string `json:"familyName,omitempty"`
}

// UserStatus defines the observed state of User
type UserStatus struct {
	// Conditions provide conditions that represent the current status of the User.
	// +kubebuilder:default={{type: "Ready", status: "Unknown", reason: "Unknown", message: "Waiting for control plane to reconcile", lastTransitionTime: "1970-01-01T00:00:00Z"}}
	// +kubebuilder:validation:Optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// State represents the current activation state of the user account from the
	// auth provider. This field is managed exclusively by the UserDeactivation CRD
	// and cannot be changed directly by the user. When a UserDeactivation resource
	// is created for the user, the user is deactivated in the auth provider; when
	// the UserDeactivation is deleted, the user is reactivated.
	// States:
	//   - Active: The user can be used to authenticate.
	//   - Inactive: The user is prohibited to be used to authenticate, and revokes all existing sessions.
	// +kubebuilder:default=Active
	// +kubebuilder:validation:Enum=Active;Inactive
	State UserState `json:"state,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// UserList contains a list of User
type UserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []User `json:"items"`
}
