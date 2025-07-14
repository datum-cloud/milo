package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// SchemeGroupVersion is group version used to register these objects
var SchemeGroupVersion = schema.GroupVersion{Group: "iam.miloapis.com", Version: "v1alpha1"}

var (
	// SchemeBuilder initializes a scheme builder. We register both the external
	// v1alpha1 types and internal (unversioned) types so that the generic
	// apiserver storage layer, which converts objects to the internal version
	// before persisting them, can encode/decode MachineAccountKey resources.
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes, addInternalKnownTypes)
	// AddToScheme is a global function that registers this API group & version to a scheme
	AddToScheme = SchemeBuilder.AddToScheme
)

// addInternalKnownTypes registers the IAM types with the scheme using the
// "internal" API version. We currently only need MachineAccountKey and its
// list to satisfy the storage codec used by the apiserver, but additional
// types can be added here as they get first-class storage implementations.
func addInternalKnownTypes(scheme *runtime.Scheme) error {
	internalGV := schema.GroupVersion{Group: SchemeGroupVersion.Group, Version: runtime.APIVersionInternal}

	scheme.AddKnownTypes(internalGV,
		&MachineAccountKey{},
		&MachineAccountKeyList{},
	)
	return nil
}

// Adds the list of known types to Scheme.
func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&Group{},
		&GroupList{},
		&GroupMembership{},
		&GroupMembershipList{},
		&PolicyBinding{},
		&PolicyBindingList{},
		&UserInvitation{},
		&UserInvitationList{},
		&Role{},
		&RoleList{},
		&User{},
		&UserList{},
		&ProtectedResource{},
		&ProtectedResourceList{},
		&MachineAccount{},
		&MachineAccountList{},
		&MachineAccountKey{},
		&MachineAccountKeyList{},
	)
	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}
