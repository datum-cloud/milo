package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Create conditions
const (
	// ContactGroupReadyCondition is the condition Type that tracks contact group creation status.
	ContactGroupReadyCondition = "Ready"
	// ContactGroupCreatePendingReason is used when contact group creation is in progress.
	ContactGroupCreatePendingReason = "CreatePending"
	// ContactGroupCreatedReason is used when contact group creation succeeds.
	ContactGroupCreatedReason = "CreateSuccessful"
)

// Delete conditions
const (
	// ContactGroupDeletedCondition is the condition Type that tracks contact group deletion status.
	ContactGroupDeletedCondition = "Delete"
	// ContactGroupDeletePendingReason is used when contact group deletion is in progress.
	ContactGroupDeletePendingReason = "DeletePending"
	// ContactGroupDeletedReason is used when contact group deletion succeeds.
	ContactGroupDeletedReason = "DeleteSuccessful"
)

// Update conditions
const (
	// ContactGroupUpdatedCondition is the condition Type that tracks contact group update status.
	ContactGroupUpdatedCondition = "Update"
	// ContactGroupUpdatePendingReason is used when contact group update is in progress.
	ContactGroupUpdatePendingReason = "UpdatePending"
	// ContactGroupUpdatedReason is used when contact group update succeeds.
	ContactGroupUpdatedReason = "UpdateSuccessful"
)

// ContactGroupVisibility declares whether a group is open for opt-out.
type ContactGroupVisibility string

const (
	ContactGroupVisibilityPublic  ContactGroupVisibility = "public"
	ContactGroupVisibilityPrivate ContactGroupVisibility = "private"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// ContactGroup is the Schema for the contactgroups API.
// It represents a logical grouping of Contacts.
// +kubebuilder:printcolumn:name="DisplayName",type="string",JSONPath=".spec.displayName"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Namespaced
type ContactGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ContactGroupSpec   `json:"spec,omitempty"`
	Status ContactGroupStatus `json:"status,omitempty"`
}

// ContactGroupSpec defines the desired state of ContactGroup.
// +kubebuilder:validation:Type=object
type ContactGroupSpec struct {
	// DisplayName is the display name of the contact group.
	// +kubebuilder:validation:Required
	DisplayName string `json:"displayName,omitempty"`

	// Visibility determines whether members are allowed opt-in or opt-out of the contactgroup.
	//   • "public"  – members may leave via ContactGroupMembershipRemoval.
	//   • "private" – membership is enforced; opt-out requests are rejected.
	// +kubebuilder:validation:Enum=public;private
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="type(oldSelf) == null_type || self == oldSelf",message="visibility type is immutable"
	Visibility ContactGroupVisibility `json:"visibility,omitempty"`
}

// +kubebuilder:object:root=true

// ContactGroupList contains a list of ContactGroup.
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ContactGroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ContactGroup `json:"items"`
}

type ContactGroupStatus struct {
	// Conditions represent the latest available observations of an object's current state.
	// Standard condition is "Ready" which tracks contact group creation status and sync to the contact group provider.
	// +kubebuilder:default={{type: "Ready", status: "Unknown", reason: "CreatePending", message: "Waiting for contact group to be created", lastTransitionTime: "1970-01-01T00:00:00Z"}}
	// +kubebuilder:validation:Optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ProviderID is the identifier returned by the underlying contact groupprovider
	// (e.g. Resend) when the contact groupis created. It is usually
	// used to track the contact creation status (e.g. provider webhooks).
	// +optional
	ProviderID string `json:"providerID,omitempty"`
}
