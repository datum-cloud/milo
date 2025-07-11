package iam

import (
	"k8s.io/apimachinery/pkg/runtime"

	"go.miloapis.com/milo/pkg/apis/iam/v1alpha1"
)

func Install(scheme *runtime.Scheme) error {
	return v1alpha1.AddToScheme(scheme)
}
