package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// OrganizationMembership is the Schema for the organizationmemberships API
// +k8s:openapi-gen=true
// +kubebuilder:printcolumn:name="Organization",type="string",JSONPath=".spec.organizationRef.name"
// +kubebuilder:printcolumn:name="Organization Type",type="string",JSONPath=".status.organization.type"
// +kubebuilder:printcolumn:name="Organization Display Name",type="string",JSONPath=".status.organization.displayName"
// +kubebuilder:printcolumn:name="User",type="string",JSONPath=".spec.userRef.name"
// +kubebuilder:printcolumn:name="User Email",type="string",JSONPath=".status.user.email",priority=1
// +kubebuilder:printcolumn:name="User Given Name",type="string",JSONPath=".status.user.givenName",priority=1
// +kubebuilder:printcolumn:name="User Family Name",type="string",JSONPath=".status.user.familyName",priority=1
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:path=organizationmemberships,scope=Namespaced,singular=organizationmembership
// +kubebuilder:selectablefield:JSONPath=".spec.userRef.name"
// +kubebuilder:selectablefield:JSONPath=".spec.organizationRef.name"
type OrganizationMembership struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OrganizationMembershipSpec   `json:"spec,omitempty"`
	Status OrganizationMembershipStatus `json:"status,omitempty"`
}

// OrganizationMembershipSpec defines the desired state of OrganizationMembership
type OrganizationMembershipSpec struct {
	// OrganizationRef is a reference to the Organization that the user is a member of.
	// +kubebuilder:validation:Required
	OrganizationRef OrganizationReference `json:"organizationRef"`
	// UserRef is a reference to the User that is a member of the Organization.
	// +kubebuilder:validation:Required
	UserRef MemberReference `json:"userRef"`
}

// OrganizationReference contains information that points to the Organization being referenced.
type OrganizationReference struct {
	// Name is the name of resource being referenced
	// +kubebuilder:validation:Required
	Name string `json:"name"`
}

// MemberReference contains information that points to the User being referenced.
type MemberReference struct {
	// Name is the name of resource being referenced
	// +kubebuilder:validation:Required
	Name string `json:"name"`
}

// OrganizationMembershipStatus defines the observed state of OrganizationMembership
type OrganizationMembershipStatus struct {
	// ObservedGeneration is the most recent generation observed for this OrganizationMembership by the controller.
	// +kubebuilder:validation:Optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions provide conditions that represent the current status of the OrganizationMembership.
	// +kubebuilder:default={{type: "Ready", status: "Unknown", reason: "Unknown", message: "Waiting for control plane to reconcile", lastTransitionTime: "1970-01-01T00:00:00Z"}}
	// +kubebuilder:validation:Optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// User contains information about the user in the membership.
	// +kubebuilder:validation:Optional
	User OrganizationMembershipUserStatus `json:"user,omitempty"`

	// Organization contains information about the organization in the membership.
	// +kubebuilder:validation:Optional
	Organization OrganizationMembershipOrganizationStatus `json:"organization,omitempty"`
}

// OrganizationMembershipUserStatus defines the observed state of a user in a membership.
type OrganizationMembershipUserStatus struct {
	// Email is the email of the user in the membership.
	// +kubebuilder:validation:Optional
	Email string `json:"email,omitempty"`
	// GivenName is the given name of the user in the membership.
	// +kubebuilder:validation:Optional
	GivenName string `json:"givenName,omitempty"`
	// FamilyName is the family name of the user in the membership.
	// +kubebuilder:validation:Optional
	FamilyName string `json:"familyName,omitempty"`
}

// OrganizationMembershipOrganizationStatus defines the observed state of an organization in a membership.
type OrganizationMembershipOrganizationStatus struct {
	// Type is the type of the organization in the membership.
	// +kubebuilder:validation:Optional
	Type string `json:"type,omitempty"`
	// DisplayName is the display name of the organization in the membership.
	// +kubebuilder:validation:Optional
	DisplayName string `json:"displayName,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true

// OrganizationMembershipList contains a list of OrganizationMembership
type OrganizationMembershipList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OrganizationMembership `json:"items"`
}
