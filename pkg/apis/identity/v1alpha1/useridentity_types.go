package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// UserIdentity represents a user's linked identity within an external identity provider.
//
// This resource describes the connection between a Milo user and their account in an
// external authentication provider (e.g., GitHub, Google, Microsoft). It is NOT the
// identity provider itself, but rather the user's specific identity within that provider.
//
// Use cases:
//   - Display all authentication methods linked to a user account in the UI
//   - Show which external accounts a user has connected
//   - Provide visibility into federated identity mappings
//
// Important notes:
//   - This is a read-only resource for display purposes only
//   - Identity management (linking/unlinking providers) is handled by the external
//     authentication provider (e.g., Zitadel), not through this API
//   - No sensitive credentials or tokens are exposed through this resource
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type UserIdentity struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Status UserIdentityStatus `json:"status,omitempty"`
}

// UserIdentityStatus contains the details of a user's identity within an external provider.
// All fields are read-only and populated by the authentication provider.
type UserIdentityStatus struct {
	// UserUID is the unique identifier of the Milo user who owns this identity.
	UserUID string `json:"userUID"`

	// ProviderID is the unique identifier of the external identity provider instance.
	// This is typically an internal ID from the authentication system.
	ProviderID string `json:"providerID"`

	// ProviderName is the human-readable name of the identity provider.
	// Examples: "GitHub", "Google", "Microsoft", "GitLab"
	ProviderName string `json:"providerName"`

	// Username is the user's username or identifier within the external identity provider.
	// This is the name the user is known by in the external system (e.g., GitHub username).
	Username string `json:"username"`
}

// UserIdentityList is a list of UserIdentity resources.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type UserIdentityList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []UserIdentity `json:"items"`
}
