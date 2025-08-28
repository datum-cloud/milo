package notification

import (
	"k8s.io/apimachinery/pkg/runtime"

	"go.miloapis.com/milo/pkg/apis/notification/v1alpha1"
)

// Install registers the Notification API types with the given scheme.
func Install(scheme *runtime.Scheme) {
	v1alpha1.AddToScheme(scheme)
}
