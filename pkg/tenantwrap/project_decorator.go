package tenantwrap

import (
	"context"
	"path"
	"strings"
	"sync"

	"go.miloapis.com/milo/pkg/request"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"

	generic "k8s.io/apiserver/pkg/registry/generic"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/storage"
	storagebackend "k8s.io/apiserver/pkg/storage/storagebackend"
	factory "k8s.io/apiserver/pkg/storage/storagebackend/factory"

	"k8s.io/client-go/tools/cache"
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
	// Ensure we always wrap with our project-aware decorator.
	if opts.Decorator == nil {
		opts.Decorator = ProjectAwareDecorator(genericregistry.StorageWithCacher())
	} else {
		opts.Decorator = ProjectAwareDecorator(opts.Decorator)
	}
	return opts, nil
}

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

type child struct {
	s       storage.Interface
	destroy factory.DestroyFunc
}

type decoratorArgs struct {
	resourcePrefix string
	keyFunc        func(obj runtime.Object) (string, error)
	newFunc        func() runtime.Object
	newListFunc    func() runtime.Object
	getAttrs       storage.AttrFunc
	triggerFn      storage.IndexerFuncs
	indexers       *cache.Indexers
}

// projectMux implements storage.Interface and routes to a per-project child.
type projectMux struct {
	mu        sync.RWMutex
	children  map[string]*child
	versioner storage.Versioner

	inner generic.StorageDecorator
	cfg   storagebackend.ConfigForResource
	args  decoratorArgs
}

func (m *projectMux) Versioner() storage.Versioner { return m.versioner }

func (m *projectMux) childForProject(project string) (storage.Interface, error) {
	m.mu.RLock()
	if c, ok := m.children[project]; ok {
		m.mu.RUnlock()
		return c.s, nil
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()
	if c, ok := m.children[project]; ok {
		return c.s, nil
	}

	cfg2 := m.cfg // copy
	base := strings.TrimPrefix(cfg2.Config.Prefix, "/")
	cfg2.Config.Prefix = "/" + path.Join("projects", project, base)

	s, destroy, err := m.inner(
		&cfg2,
		m.args.resourcePrefix,
		m.args.keyFunc,
		m.args.newFunc,
		m.args.newListFunc,
		m.args.getAttrs,
		m.args.triggerFn,
		m.args.indexers,
	)
	if err != nil {
		return nil, err
	}
	if m.versioner == nil {
		m.versioner = s.Versioner()
	}
	if m.children == nil {
		m.children = make(map[string]*child, 1)
	}
	m.children[project] = &child{s: s, destroy: destroy}
	return s, nil
}

func (m *projectMux) destroyAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, c := range m.children {
		if c.destroy != nil {
			c.destroy()
		}
		delete(m.children, k)
	}
}

func (m *projectMux) pick(ctx context.Context) (storage.Interface, error) {
	if proj, ok := request.ProjectID(ctx); ok && proj != "" {
		return m.childForProject(proj)
	}
	return m.childForProject("")
}

// ---------- storage.Interface forwarding ----------

func (m *projectMux) Create(ctx context.Context, key string, obj, out runtime.Object, ttl uint64) error {
	s, err := m.pick(ctx)
	if err != nil {
		return err
	}
	return s.Create(ctx, key, obj, out, ttl)
}

func (m *projectMux) Delete(
	ctx context.Context,
	key string,
	out runtime.Object,
	precond *storage.Preconditions,
	validateDeletion storage.ValidateObjectFunc,
	cachedExistingObject runtime.Object,
	opts storage.DeleteOptions,
) error {
	s, err := m.pick(ctx)
	if err != nil {
		return err
	}
	return s.Delete(ctx, key, out, precond, validateDeletion, cachedExistingObject, opts)
}

func (m *projectMux) Watch(ctx context.Context, key string, opts storage.ListOptions) (watch.Interface, error) {
	s, err := m.pick(ctx)
	if err != nil {
		return nil, err
	}
	return s.Watch(ctx, key, opts)
}

func (m *projectMux) Get(ctx context.Context, key string, opts storage.GetOptions, objPtr runtime.Object) error {
	s, err := m.pick(ctx)
	if err != nil {
		return err
	}
	return s.Get(ctx, key, opts, objPtr)
}

func (m *projectMux) GetList(ctx context.Context, key string, opts storage.ListOptions, listObj runtime.Object) error {
	s, err := m.pick(ctx)
	if err != nil {
		return err
	}
	return s.GetList(ctx, key, opts, listObj)
}

func (m *projectMux) GuaranteedUpdate(
	ctx context.Context,
	key string,
	out runtime.Object,
	ignoreNotFound bool,
	precond *storage.Preconditions,
	tryUpdate storage.UpdateFunc,
	suggestion runtime.Object,
) error {
	s, err := m.pick(ctx)
	if err != nil {
		return err
	}
	return s.GuaranteedUpdate(ctx, key, out, ignoreNotFound, precond, tryUpdate, suggestion)
}

// If your k8s minor *doesn't* include Count in storage.Interface, delete this.
func (m *projectMux) Count(key string) (int64, error) {
	m.mu.RLock()
	c := m.children[""]
	m.mu.RUnlock()
	if c == nil {
		if _, err := m.childForProject(""); err != nil {
			return 0, err
		}
		m.mu.RLock()
		c = m.children[""]
		m.mu.RUnlock()
	}
	return c.s.Count(key)
}

// ReadinessCheck proxies to the appropriate child (defaults to the "" project).
func (m *projectMux) ReadinessCheck() error {
	m.mu.RLock()
	c := m.children[""]
	m.mu.RUnlock()
	if c == nil {
		if _, err := m.childForProject(""); err != nil {
			return err
		}
		m.mu.RLock()
		c = m.children[""]
		m.mu.RUnlock()
	}
	return c.s.ReadinessCheck()
}

func (m *projectMux) RequestWatchProgress(ctx context.Context) error {
	s, err := m.pick(ctx)
	if err != nil {
		return err
	}
	return s.RequestWatchProgress(ctx)
}
