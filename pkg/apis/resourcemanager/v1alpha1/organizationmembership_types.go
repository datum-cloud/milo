package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// OrganizationMembership links a user to an organization, establishing the
// foundation for role-based access control within organizations. Note that
// membership alone does not grant access - a PolicyBinding must also be
// created to assign roles and permissions to the user.
//
// OrganizationMemberships are namespaced resources that create relationships
// between cluster-scoped users and organizations. They are a prerequisite
// for access control but do not grant permissions by themselves.
//
// Key characteristics:
// - Namespaced: Created within the organization's namespace
// - User-organization linkage: Connects users to organizations
// - Access prerequisite: Required before PolicyBindings can grant organization permissions
// - Status information: Provides cached details about both user and organization
//
// Common workflows:
// 1. Ensure both the user and organization exist and are ready
// 2. Create the membership in the organization's namespace
// 3. Wait for the Ready condition to become True
// 4. Create PolicyBinding resources to grant specific roles and permissions
// 5. User can now access organization resources based on assigned policies
//
// Prerequisites:
// - User: The referenced user must exist and be ready
// - Organization: The referenced organization must exist and be ready
// - Namespace: Must be created in the organization's associated namespace
//
// Example - Adding a user to an organization:
//
//	apiVersion: resourcemanager.miloapis.com/v1alpha1
//	kind: OrganizationMembership
//	metadata:
//	  name: jane-doe-acme-membership
//	  namespace: organization-acme-corp
//	spec:
//	  organizationRef:
//	    name: acme-corp
//	  userRef:
//	    name: jane-doe
//
// Related resources:
// - User: Must exist before creating membership
// - Organization: Must exist before creating membership  
// - PolicyBinding: Required to grant actual permissions after membership is established
//
// Troubleshooting:
// - Check the Ready condition in status to verify successful membership
// - Ensure both user and organization resources exist and are ready
// - Verify the membership is created in the correct organization namespace
// - Remember that PolicyBinding resources are still needed to grant actual permissions
// - List memberships within the organization namespace to verify creation
//
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

// OrganizationMembershipSpec defines the desired membership relationship
// between a user and an organization.
//
// +k8s:protobuf=true
type OrganizationMembershipSpec struct {
	// OrganizationRef identifies the organization to grant membership in.
	// The organization must exist before creating the membership.
	//
	// Example:
	//   organizationRef:
	//     name: acme-corp
	//
	// +kubebuilder:validation:Required
	OrganizationRef OrganizationReference `json:"organizationRef"`

	// UserRef identifies the user to grant organization membership.
	// The user must exist before creating the membership.
	//
	// Example:
	//   userRef:
	//     name: jane-doe
	//
	// +kubebuilder:validation:Required
	UserRef MemberReference `json:"userRef"`
}

// OrganizationReference identifies a specific organization by name.
type OrganizationReference struct {
	// Name is the name of the organization to reference.
	// Must match an existing organization resource.
	//
	// Example: "acme-corp"
	//
	// +kubebuilder:validation:Required
	Name string `json:"name"`
}

// MemberReference identifies a specific user by name.
type MemberReference struct {
	// Name is the name of the user to reference.
	// Must match an existing user resource.
	//
	// Example: "jane-doe"
	//
	// +kubebuilder:validation:Required
	Name string `json:"name"`
}

// OrganizationMembershipStatus defines the observed state of OrganizationMembership,
// indicating whether the membership has been successfully established.
//
// +k8s:protobuf=true
type OrganizationMembershipStatus struct {
	// ObservedGeneration tracks the most recent membership spec that the
	// controller has processed. Use this to determine if status reflects
	// the latest changes.
	//
	// +kubebuilder:validation:Optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions describe the current state of membership establishment.
	// Check the "Ready" condition to determine if the membership is
	// active and the user has access to organization resources.
	//
	// Common condition types:
	// - Ready: Membership is established and user has organization access
	//
	// Example ready condition:
	//   - type: Ready
	//     status: "True"
	//     reason: MembershipReady
	//     message: User successfully added to organization
	//
	// +kubebuilder:default={{type: "Ready", status: "Unknown", reason: "Unknown", message: "Waiting for control plane to reconcile", lastTransitionTime: "1970-01-01T00:00:00Z"}}
	// +kubebuilder:validation:Optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// User contains cached information about the user in this membership.
	// This information is populated by the controller from the referenced user.
	//
	// +kubebuilder:validation:Optional
	User OrganizationMembershipUserStatus `json:"user,omitempty"`

	// Organization contains cached information about the organization in this membership.
	// This information is populated by the controller from the referenced organization.
	//
	// +kubebuilder:validation:Optional
	Organization OrganizationMembershipOrganizationStatus `json:"organization,omitempty"`
}

// OrganizationMembershipUserStatus contains cached information about
// the user in a membership relationship.
type OrganizationMembershipUserStatus struct {
	// Email is the email address of the user.
	// Populated from the referenced user resource.
	//
	// +kubebuilder:validation:Optional
	Email string `json:"email,omitempty"`

	// GivenName is the first name of the user.
	// Populated from the referenced user resource.
	//
	// +kubebuilder:validation:Optional
	GivenName string `json:"givenName,omitempty"`

	// FamilyName is the last name of the user.
	// Populated from the referenced user resource.
	//
	// +kubebuilder:validation:Optional
	FamilyName string `json:"familyName,omitempty"`
}

// OrganizationMembershipOrganizationStatus contains cached information about
// the organization in a membership relationship.
type OrganizationMembershipOrganizationStatus struct {
	// Type is the business model of the organization (Personal or Standard).
	// Populated from the referenced organization resource.
	//
	// +kubebuilder:validation:Optional
	Type string `json:"type,omitempty"`

	// DisplayName is the human-readable name of the organization.
	// Populated from the kubernetes.io/display-name annotation of the organization.
	//
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
