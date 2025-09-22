package v1alpha1

import (
	iamv1alpha1 "go.miloapis.com/milo/pkg/apis/iam/v1alpha1"
	resourcemanagerv1alpha1 "go.miloapis.com/milo/pkg/apis/resourcemanager/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Create conditions
const (
	// ContactReadyCondition is the condition Type that tracks contact creation status.
	ContactReadyCondition = "Ready"
	// ContactCreatePendingReason is used when contact creation is in progress.
	ContactCreatePendingReason = "CreatePending"
	// ContactCreatedReason is used when contact creation succeeds.
	ContactCreatedReason = "CreateSuccessful"
)

// Delete conditions
const (
	// ContactDeletedCondition is the condition Type that tracks contact deletion status.
	ContactDeletedCondition = "Delete"
	// ContactDeletePendingReason is used when contact deletion is in progress.
	ContactDeletePendingReason = "DeletePending"
	// ContactDeletedReason is used when contact deletion succeeds.
	ContactDeletedReason = "DeleteSuccessful"
)

// Update conditions
const (
	// ContactUpdatedCondition is the condition Type that tracks contact update status.
	ContactUpdatedCondition = "Update"
	// ContactUpdatePendingReason is used when contact update is in progress.
	ContactUpdatePendingReason = "UpdatePending"
	// ContactUpdatedReason is used when contact update succeeds.
	ContactUpdatedReason = "UpdateSuccessful"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Contact is the Schema for the contacts API.
// It represents a contact for a user.
// +kubebuilder:printcolumn:name="UserRef",type="string",JSONPath=".spec.subject.userRef.name"
// +kubebuilder:printcolumn:name="OrganizationRef",type="string",JSONPath=".spec.subject.organizationRef.name"
// +kubebuilder:printcolumn:name="ProjectRef",type="string",JSONPath=".spec.subject.projectRef.name"
// +kubebuilder:printcolumn:name="EmailRef",type="string",JSONPath=".spec.subject.email"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Namespaced
type Contact struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ContactSpec   `json:"spec,omitempty"`
	Status ContactStatus `json:"status,omitempty"`
}

// ContactSpec defines the desired state of Contact.
// +kubebuilder:validation:Type=object
type ContactSpec struct {
	// Subject is a reference to the subject of the contact.
	// +kubebuilder:validation:Optional
	Subject *SubjectReference `json:"subject,omitempty"`

	// +kubebuilder:validation:Required
	FamilyName string `json:"familyName,omitempty"`

	// +kubebuilder:validation:Required
	GivenName string `json:"givenName,omitempty"`

	// +kubebuilder:validation:Required
	Email string `json:"email,omitempty"`
}

// SubjectReference is a reference to the subject of the contact.
// +kubebuilder:validation:XValidation:rule="has(self.userRef) != has(self.organizationRef) != has(self.projectRef)",message="exactly one of userRef, organizationRef projectRef must be provided"
// +kubebuilder:validation:Type=object
type SubjectReference struct {
	// UserRef is a reference to the User that the contact is for.
	// It is mutually exclusive with OrganizationRef and ProjectRef.
	// +kubebuilder:validation:Optional
	UserRef *iamv1alpha1.UserReference `json:"userRef,omitempty"`

	// OrganizationRef is a reference to the Organization that the contact is for.
	// It is mutually exclusive with UserRef and ProjectRef.
	// +kubebuilder:validation:Optional
	OrganizationRef *resourcemanagerv1alpha1.OrganizationReference `json:"organizationRef,omitempty"`

	// ProjectRef is a reference to the Project that the contact is for.
	// It is mutually exclusive with UserRef and OrganizationRef.
	// +kubebuilder:validation:Optional
	ProjectRef *ProjectReference `json:"projectRef,omitempty"`
}

// ProjectReference is a reference to the Project that the contact is for.
// +kubebuilder:validation:Type=object
type ProjectReference struct {
	// Name is the name of the Project that the contact is for.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// OrganizationRef is a reference to the Organization that the Project is in.
	// +kubebuilder:validation:Required
	OrganizationRef *resourcemanagerv1alpha1.OrganizationReference `json:"organizationRef,omitempty"`
}

// +kubebuilder:object:root=true

// ContactList contains a list of Contact.
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ContactList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Contact `json:"items"`
}

type ContactStatus struct {
	// Conditions represent the latest available observations of an object's current state.
	// Standard condition is "Ready" which tracks contact creation status and sync to the contact provider.
	// +kubebuilder:default={{type: "Ready", status: "Unknown", reason: "CreatePending", message: "Waiting for contact to be created", lastTransitionTime: "1970-01-01T00:00:00Z"}}
	// +kubebuilder:validation:Optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ProviderID is the identifier returned by the underlying contact provider
	// (e.g. Resend) when the contact is created. It is usually
	// used to track the contact creation status (e.g. provider webhooks).
	// +optional
	ProviderID string `json:"providerID,omitempty"`
}
