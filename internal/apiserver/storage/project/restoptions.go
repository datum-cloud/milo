package projectstorage

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	generic "k8s.io/apiserver/pkg/registry/generic"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"
)

// Wrap the upstream RESTOptionsGetter to install a per-project decorator.
func WithProjectAwareDecorator(inner generic.RESTOptionsGetter) generic.RESTOptionsGetter {
	return roGetter{inner: inner}
}

type roGetter struct {
	inner generic.RESTOptionsGetter
}

// NOTE: matches your two-arg signature (GroupResource, runtime.Object).
func (g roGetter) GetRESTOptions(gr schema.GroupResource, example runtime.Object) (generic.RESTOptions, error) {
	opts, err := g.inner.GetRESTOptions(gr, example)
	if err != nil {
		return opts, err
	}
	// 🔒 Leave CRD *definitions* global so discovery is shared cluster-wide
	if gr.Group == "apiextensions.k8s.io" && gr.Resource == "customresourcedefinitions" {
		return opts, nil
	}

	// Ensure we always wrap with our project-aware decorator.
	if opts.Decorator == nil {
		opts.Decorator = ProjectAwareDecorator(genericregistry.StorageWithCacher())
	} else {
		opts.Decorator = ProjectAwareDecorator(opts.Decorator)
	}
	return opts, nil
}
