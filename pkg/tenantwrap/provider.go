// pkg/tenantwrap/provider.go
package tenantwrap

import (
	"reflect"
	"strings"
	"unsafe"

	generic "k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/generic/registry"
	apiserver "k8s.io/apiserver/pkg/server"
	serverstorage "k8s.io/apiserver/pkg/server/storage"
	"k8s.io/klog/v2"
	controlplaneapiserver "k8s.io/kubernetes/pkg/controlplane/apiserver"
)

// ---------- common patching logic -------------------------------------
type baseDecorator struct {
	inner controlplaneapiserver.RESTStorageProvider
}

func (b baseDecorator) GroupName() string { return b.inner.GroupName() }

func (b baseDecorator) NewRESTStorage(
	cfg serverstorage.APIResourceConfigSource,
	getter generic.RESTOptionsGetter,
) (apiserver.APIGroupInfo, error) {

	agi, err := b.inner.NewRESTStorage(cfg, getter)
	if err != nil {
		return agi, err
	}

	// pkg/tenantwrap/provider.go  – inside baseDecorator.NewRESTStorage
	for _, resMap := range agi.VersionedResourcesStorageMap {
		// look for the three namespace entries
		for resName, obj := range resMap {
			if !strings.HasPrefix(resName, "namespaces") {
				continue
			}

			if s := findStore(obj); s != nil {
				klog.Infof("[tenant] wrapping %q store (%T)", resName, s)
				Wrap(s) // prepend /projects/<id>/registry/…
			} else {
				klog.Warningf("[tenant] %q: Store not found (type %T)", resName, obj)
			}
		}
	}

	return agi, nil
}

// findStore returns the first *registry.Store found inside obj (one level deep),
// including unexported anonymous fields.
func findStore(obj interface{}) *registry.Store {
	// direct hit
	if s, ok := obj.(*registry.Store); ok {
		return s
	}

	v := reflect.ValueOf(obj)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return nil
	}

	for i := 0; i < v.NumField(); i++ {
		fv := v.Field(i)
		ft := fv.Type()

		// ----------------------------------------------------------
		// Case 1: anonymous or named inline struct field (Store)
		// ----------------------------------------------------------
		if fv.Kind() == reflect.Struct && ft == reflect.TypeOf(registry.Store{}) {
			ptr := unsafe.Pointer(fv.UnsafeAddr())
			return (*registry.Store)(ptr)
		}

		// ----------------------------------------------------------
		// Case 2: pointer to Store ( *registry.Store )
		// ----------------------------------------------------------
		if fv.Kind() == reflect.Pointer && ft.Elem() == reflect.TypeOf(registry.Store{}) {
			if fv.IsNil() {
				return nil
			} // shouldn't be nil, but guard
			// if the field is exported, Interface() is safe
			if fv.CanInterface() {
				return fv.Interface().(*registry.Store)
			}
			// unexported pointer field → use unsafe
			ptr := unsafe.Pointer(fv.UnsafeAddr())
			// fv.UnsafeAddr is a *uintptr to the data; dereference and convert
			return *(**registry.Store)(ptr)
		}
	}
	return nil
}

// ---------- wrapper *with* PostStartHook -------------------------------
type withHook struct{ baseDecorator }

func (w withHook) PostStartHook() (string, apiserver.PostStartHookFunc, error) {
	return w.inner.(apiserver.PostStartHookProvider).PostStartHook()
}

// ---------- factory ----------------------------------------------------
func WrapProvider(p controlplaneapiserver.RESTStorageProvider) controlplaneapiserver.RESTStorageProvider {
	if _, ok := p.(apiserver.PostStartHookProvider); ok {
		return withHook{baseDecorator{inner: p}}
	}
	return baseDecorator{inner: p}
}
