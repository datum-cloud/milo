package v1alpha1

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"
)

// SchemeGroupVersion is group version used to register these objects
var SchemeGroupVersion = schema.GroupVersion{Group: "identity.miloapis.com", Version: "v1alpha1"}

var (
	// SchemeBuilder initializes a scheme builder
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	// AddToScheme is a global function that registers this API group & version to a scheme
	AddToScheme = SchemeBuilder.AddToScheme
)

// Kind takes an unqualified kind and returns back a Group qualified GroupKind
func Kind(kind string) schema.GroupKind {
	return SchemeGroupVersion.WithKind(kind).GroupKind()
}

// Resource takes an unqualified resource and returns a Group qualified GroupResource
func Resource(resource string) schema.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}

// Adds the list of known types to Scheme.
func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&Session{},
		&SessionList{},
		&UserIdentity{},
		&UserIdentityList{},
	)
	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)

	// Register field label conversions for UserIdentity
	// This enables field selectors like status.userUID=<user-id> for staff users
	userIdentityGVK := SchemeGroupVersion.WithKind("UserIdentity")
	klog.InfoS("Registering field label conversion for UserIdentity", "gvk", userIdentityGVK.String())
	if err := scheme.AddFieldLabelConversionFunc(userIdentityGVK,
		UserIdentityFieldLabelConversionFunc); err != nil {
		klog.ErrorS(err, "Failed to register UserIdentity field label conversion")
		return err
	}

	// Register field label conversions for Session
	// This enables field selectors like status.userUID=<user-id> for staff users
	sessionGVK := SchemeGroupVersion.WithKind("Session")
	klog.InfoS("Registering field label conversion for Session", "gvk", sessionGVK.String())
	if err := scheme.AddFieldLabelConversionFunc(sessionGVK,
		SessionFieldLabelConversionFunc); err != nil {
		klog.ErrorS(err, "Failed to register Session field label conversion")
		return err
	}

	return nil
}

// UserIdentityFieldLabelConversionFunc converts field selectors for UserIdentity resources.
// This allows staff users to filter user identities by fields beyond the default metadata.name.
func UserIdentityFieldLabelConversionFunc(label, value string) (string, string, error) {
	klog.V(2).InfoS("UserIdentity field label conversion called", "label", label, "value", value)
	switch label {
	// Metadata fields (default Kubernetes fields)
	case "metadata.name",
		"metadata.namespace":
		klog.V(2).InfoS("UserIdentity field label conversion: accepted metadata field", "label", label)
		return label, value, nil

	// Status fields (custom field selector for staff users)
	case "status.userUID":
		klog.V(2).InfoS("UserIdentity field label conversion: accepted status.userUID", "label", label, "value", value)
		return label, value, nil

	default:
		err := fmt.Errorf("%q is not a known field selector: only %q are supported",
			label, []string{"metadata.name", "metadata.namespace", "status.userUID"})
		klog.V(2).InfoS("UserIdentity field label conversion: rejected field", "label", label, "error", err)
		return "", "", err
	}
}

// SessionFieldLabelConversionFunc converts field selectors for Session resources.
// This allows staff users to filter sessions by fields beyond the default metadata.name.
func SessionFieldLabelConversionFunc(label, value string) (string, string, error) {
	klog.V(2).InfoS("Session field label conversion called", "label", label, "value", value)
	switch label {
	// Metadata fields (default Kubernetes fields)
	case "metadata.name",
		"metadata.namespace":
		klog.V(2).InfoS("Session field label conversion: accepted metadata field", "label", label)
		return label, value, nil

	// Status fields (custom field selector for staff users)
	case "status.userUID":
		klog.V(2).InfoS("Session field label conversion: accepted status.userUID", "label", label, "value", value)
		return label, value, nil

	default:
		err := fmt.Errorf("%q is not a known field selector: only %q are supported",
			label, []string{"metadata.name", "metadata.namespace", "status.userUID"})
		klog.V(2).InfoS("Session field label conversion: rejected field", "label", label, "error", err)
		return "", "", err
	}
}
