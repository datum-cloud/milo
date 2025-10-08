package identity

import (
	"k8s.io/apimachinery/pkg/runtime"

	"go.miloapis.com/milo/pkg/apis/identity/v1alpha1"
)

// Install registers the identity API group versions into the provided scheme.
func Install(scheme *runtime.Scheme) {
	v1alpha1.AddToScheme(scheme)
}
