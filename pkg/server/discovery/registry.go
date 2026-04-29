package discovery

import (
	"context"
	"fmt"
	"sync"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsinformers "k8s.io/apiextensions-apiserver/pkg/client/informers/externalversions"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

// Registry is the source of truth for "which parent contexts is this resource
// visible in." It merges two inputs:
//
//  1. CRD annotations — for resources installed through the apiextensions API
//     (both Milo's bundled CRDs and any CRDs installed by external services
//     that build on Milo). Watched live via an informer.
//
//  2. Static registrations — for built-in/aggregated APIs (e.g. core/v1,
//     identity.miloapis.com sessions) that aren't backed by CRDs. Registered
//     once at apiserver startup with RegisterStatic.
//
// Lookups return the union: a static registration sets a baseline that a CRD
// annotation can extend but not contradict. (In practice no resource has
// both, since static is for non-CRD types.)
//
// Resources with no registration in either source are treated as visible in
// all contexts, so existing CRDs and external CRDs that haven't adopted the
// marker continue to behave as before.
type Registry struct {
	mu      sync.RWMutex
	crd     map[schema.GroupResource][]ParentContext
	static  map[schema.GroupResource][]ParentContext
	hasInit bool
}

// NewRegistry creates an empty registry. Call RegisterStatic for any built-in
// APIs, then Run with an informer factory to populate the CRD-derived map.
func NewRegistry() *Registry {
	return &Registry{
		crd:    map[schema.GroupResource][]ParentContext{},
		static: map[schema.GroupResource][]ParentContext{},
	}
}

// RegisterStatic records the parent contexts for a built-in or aggregated API
// that is not backed by a CRD. Safe to call before Run.
func (r *Registry) RegisterStatic(gr schema.GroupResource, contexts ...ParentContext) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(contexts) == 0 {
		delete(r.static, gr)
		return
	}
	r.static[gr] = append([]ParentContext(nil), contexts...)
}

// AllowedContexts returns the parent contexts a resource should be visible
// in, or nil if it should be visible everywhere.
func (r *Registry) AllowedContexts(gr schema.GroupResource) []ParentContext {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if v, ok := r.static[gr]; ok {
		return v
	}
	if v, ok := r.crd[gr]; ok {
		return v
	}
	return nil
}

// IsVisible is a convenience wrapper combining AllowedContexts + Matches.
func (r *Registry) IsVisible(gr schema.GroupResource, current ParentContext) bool {
	return Matches(r.AllowedContexts(gr), current)
}

// HasSynced reports whether the CRD informer has completed its initial list.
// Discovery filtering should fall open (visible) until this is true to avoid
// hiding resources during apiserver startup.
func (r *Registry) HasSynced() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.hasInit
}

// Run starts watching CRDs from the supplied informer factory. It blocks
// until ctx is cancelled. Caller must invoke factory.Start(...) separately
// (or use the same factory for other consumers).
func (r *Registry) Run(ctx context.Context, factory apiextensionsinformers.SharedInformerFactory) error {
	informer := factory.Apiextensions().V1().CustomResourceDefinitions().Informer()

	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj any) { r.upsertFromObj(obj) },
		UpdateFunc: func(_, obj any) { r.upsertFromObj(obj) },
		DeleteFunc: func(obj any) { r.deleteFromObj(obj) },
	})
	if err != nil {
		return fmt.Errorf("registering CRD event handler: %w", err)
	}

	if !cache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		return fmt.Errorf("CRD informer cache failed to sync")
	}

	r.mu.Lock()
	r.hasInit = true
	r.mu.Unlock()

	klog.InfoS("Discovery context registry synced", "crdEntries", len(r.crd))
	<-ctx.Done()
	return nil
}

func (r *Registry) upsertFromObj(obj any) {
	crd, ok := obj.(*apiextensionsv1.CustomResourceDefinition)
	if !ok {
		return
	}
	gr := schema.GroupResource{Group: crd.Spec.Group, Resource: crd.Spec.Names.Plural}
	contexts := ParseContexts(crd.Annotations[ParentContextsAnnotation])

	r.mu.Lock()
	defer r.mu.Unlock()
	if contexts == nil {
		// Wildcard / unset — drop any prior entry so lookup falls through
		// to "visible everywhere".
		delete(r.crd, gr)
		return
	}
	r.crd[gr] = contexts
}

func (r *Registry) deleteFromObj(obj any) {
	var crd *apiextensionsv1.CustomResourceDefinition
	switch v := obj.(type) {
	case *apiextensionsv1.CustomResourceDefinition:
		crd = v
	case cache.DeletedFinalStateUnknown:
		crd, _ = v.Obj.(*apiextensionsv1.CustomResourceDefinition)
	}
	if crd == nil {
		return
	}
	gr := schema.GroupResource{Group: crd.Spec.Group, Resource: crd.Spec.Names.Plural}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.crd, gr)
}
