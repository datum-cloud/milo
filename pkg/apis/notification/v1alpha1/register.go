package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// SchemeGroupVersion is group version used to register these objects.
var SchemeGroupVersion = schema.GroupVersion{Group: "notification.miloapis.com", Version: "v1alpha1"}

var (
	// SchemeBuilder is used to add go types to the GroupVersionKind scheme
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	// AddToScheme allows addition of this group to a scheme
	AddToScheme = SchemeBuilder.AddToScheme
)

// addKnownTypes adds the set of types defined in this package to the supplied scheme.
func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&EmailTemplate{},
		&EmailTemplateList{},
		&Email{},
		&EmailList{},
		&Contact{},
		&ContactList{},
		&ContactGroup{},
		&ContactGroupList{},
		&ContactGroupMembership{},
		&ContactGroupMembershipList{},
		&EmailBroadcast{},
		&EmailBroadcastList{},
		&ContactGroupMembershipRemoval{},
		&ContactGroupMembershipRemovalList{},
	)
	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}
