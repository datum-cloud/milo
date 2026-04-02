package identity

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"go.miloapis.com/milo/pkg/apis/identity/v1alpha1"
)

// Install registers the identity API group versions into the provided scheme.
func Install(scheme *runtime.Scheme) {
	v1alpha1.AddToScheme(scheme)

	// Register valid field selectors for MachineAccountKey so the generic API
	// server passes them through to the REST handler instead of rejecting them.
	_ = scheme.AddFieldLabelConversionFunc(
		schema.GroupVersionKind{
			Group:   v1alpha1.SchemeGroupVersion.Group,
			Version: v1alpha1.SchemeGroupVersion.Version,
			Kind:    "MachineAccountKey",
		},
		func(label, value string) (string, string, error) {
			switch label {
			case "spec.machineAccountUserName", "metadata.name", "metadata.namespace":
				return label, value, nil
			default:
				return "", "", nil
			}
		},
	)
}
