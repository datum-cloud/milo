package projectstorage

import (
	"k8s.io/apimachinery/pkg/runtime"

	generic "k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/storage"
	storagebackend "k8s.io/apiserver/pkg/storage/storagebackend"
	factory "k8s.io/apiserver/pkg/storage/storagebackend/factory"

	"k8s.io/client-go/tools/cache"
)

// ProjectAwareDecorator builds (and reuses) a child cacher per project prefix.
func ProjectAwareDecorator(inner generic.StorageDecorator) generic.StorageDecorator {
	return func(
		cfg *storagebackend.ConfigForResource,
		resourcePrefix string,
		keyFunc func(obj runtime.Object) (string, error),
		newFunc func() runtime.Object,
		newListFunc func() runtime.Object,
		getAttrs storage.AttrFunc,
		triggerFn storage.IndexerFuncs, // <— changed type
		indexers *cache.Indexers, // <— from client-go/tools/cache
	) (storage.Interface, factory.DestroyFunc, error) {

		// Build default child (no project in ctx).
		defS, defDestroy, err := inner(cfg, resourcePrefix, keyFunc, newFunc, newListFunc, getAttrs, triggerFn, indexers)
		if err != nil {
			return nil, nil, err
		}

		mux := &projectMux{
			inner: inner,
			cfg:   *cfg, // copy
			args:  decoratorArgs{resourcePrefix, keyFunc, newFunc, newListFunc, getAttrs, triggerFn, indexers},
			children: map[string]*child{
				"": {s: defS, destroy: defDestroy},
			},
			versioner: defS.Versioner(),
		}
		return mux, mux.destroyAll, nil
	}
}
