package projectstorage

import (
	"context"
	"strings"
	"sync"

	"go.miloapis.com/milo/pkg/request"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/storage/cacher"

	generic "k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/storage"

	"k8s.io/apimachinery/pkg/fields"

	storagebackend "k8s.io/apiserver/pkg/storage/storagebackend"
	factory "k8s.io/apiserver/pkg/storage/storagebackend/factory"

	"k8s.io/client-go/tools/cache"
)

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

// projectMux implements storage.Interface and routes to root vs project-scoped storage.
type projectMux struct {
	mu        sync.RWMutex
	children  map[string]*child // "" => root, "*" => union(/projects)
	versioner storage.Versioner

	inner generic.StorageDecorator
	cfg   storagebackend.ConfigForResource
	args  decoratorArgs
}

func (m *projectMux) Versioner() storage.Versioner { return m.versioner }

// rootChild returns/creates the default (non-project) storage.
func (m *projectMux) rootChild() (storage.Interface, error) {
	m.mu.RLock()
	if c := m.children[""]; c != nil {
		m.mu.RUnlock()
		return c.s, nil
	}
	m.mu.RUnlock()

	// Should already exist (built in decorator), but build defensively if missing.
	m.mu.Lock()
	defer m.mu.Unlock()
	if c := m.children[""]; c != nil {
		return c.s, nil
	}
	s, destroy, err := m.inner(
		&m.cfg, m.args.resourcePrefix, m.args.keyFunc, m.args.newFunc, m.args.newListFunc,
		m.args.getAttrs, m.args.triggerFn, m.args.indexers,
	)
	if err != nil {
		return nil, err
	}
	if m.versioner == nil {
		m.versioner = s.Versioner()
	}
	if m.children == nil {
		m.children = make(map[string]*child, 2)
	}
	m.children[""] = &child{s: s, destroy: destroy}
	return s, nil
}

// unionChild returns/creates the shared storage rooted at /projects.
func (m *projectMux) unionChild() (storage.Interface, error) {
	m.mu.RLock()
	if c := m.children["*"]; c != nil {
		m.mu.RUnlock()
		return c.s, nil
	}
	m.mu.RUnlock()
	m.mu.Lock()
	defer m.mu.Unlock()
	if c := m.children["*"]; c != nil {
		return c.s, nil
	}

	cfg2 := m.cfg
	cfg2.Config.Prefix = "/projects"

	// 1) raw store (same as StorageWithCacher does internally)
	raw, rawDestroy, err := generic.NewRawStorage(&cfg2, m.args.newFunc, m.args.newListFunc, "")
	if err != nil {
		return nil, err
	}

	// 2) adapter
	adapter := NewProjectUnionStore(raw, m.args.resourcePrefix, m.args.newFunc, m.args.newListFunc)

	cli, err := newEtcdClientFrom(cfg2.Config) // endpoints + TLS from your storage config
	if err != nil {
		rawDestroy()
		return nil, err
	}
	adapter.WithEtcdClient(cli)

	// Ensure non-nil indexers/maps for the cacher
	triggers := m.args.triggerFn
	if triggers == nil {
		triggers = storage.IndexerFuncs{}
	}
	indexers := m.args.indexers
	if indexers == nil {
		indexers = &cache.Indexers{}
	}

	c, err := cacher.NewCacherFromConfig(cacher.Config{
		Storage:        adapter,
		Versioner:      storage.APIObjectVersioner{},
		GroupResource:  cfg2.GroupResource,
		ResourcePrefix: m.args.resourcePrefix,
		KeyFunc:        m.args.keyFunc,
		NewFunc:        m.args.newFunc,
		NewListFunc:    m.args.newListFunc,
		GetAttrsFunc:   adapter.GetAttrs, // virtual field "project"
		IndexerFuncs:   triggers,
		Indexers:       indexers,
		Codec:          cfg2.Codec,
	})
	if err != nil {
		rawDestroy()
		adapter.Stop()
		return nil, err
	}

	destroy := func() {
		c.Stop()
		adapter.Stop()
		if rawDestroy != nil {
			rawDestroy()
		}
	}

	if m.versioner == nil {
		m.versioner = c.Versioner()
	}
	if m.children == nil {
		m.children = make(map[string]*child, 2)
	}
	m.children["*"] = &child{s: c, destroy: destroy}
	return c, nil

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

// pickWithProject returns the storage to use and the project ("" for root requests).
func (m *projectMux) pickWithProject(ctx context.Context) (storage.Interface, string, error) {
	if proj, ok := request.ProjectID(ctx); ok && proj != "" {
		s, err := m.unionChild()
		return s, proj, err
	}
	s, err := m.rootChild()
	return s, "", err
}

// /projects/<project>/<original-key>
func addProjectToKey(_ string, project, key string) string {
	if project == "" {
		return key // root untouched
	}
	k := strings.TrimPrefix(key, "/")
	if strings.HasPrefix(k, project+"/") { // avoid double prefix
		return k
	}
	return project + "/" + k
}

// ---------- storage.Interface forwarding ----------

func (m *projectMux) Create(ctx context.Context, key string, obj, out runtime.Object, ttl uint64) error {
	s, proj, err := m.pickWithProject(ctx)
	if err != nil {
		return err
	}
	return s.Create(ctx, addProjectToKey(m.args.resourcePrefix, proj, key), obj, out, ttl)
}

func (m *projectMux) Delete(ctx context.Context, key string, out runtime.Object, precond *storage.Preconditions,
	validateDeletion storage.ValidateObjectFunc, cachedExistingObject runtime.Object, opts storage.DeleteOptions) error {
	s, proj, err := m.pickWithProject(ctx)
	if err != nil {
		return err
	}
	return s.Delete(ctx, addProjectToKey(m.args.resourcePrefix, proj, key), out, precond, validateDeletion, cachedExistingObject, opts)
}

func (m *projectMux) Watch(ctx context.Context, key string, opts storage.ListOptions) (watch.Interface, error) {
	s, proj, err := m.pickWithProject(ctx)
	if err != nil {
		return nil, err
	}
	if proj != "" {
		opts.Predicate.Field = andField(opts.Predicate.Field, fields.OneTermEqualSelector("project", proj))
	}
	// IMPORTANT: do NOT rewrite `key` for watches; let the cacher use its fixed root.
	return s.Watch(ctx, key, opts)
}

func (m *projectMux) GetList(ctx context.Context, key string, opts storage.ListOptions, listObj runtime.Object) error {
	s, proj, err := m.pickWithProject(ctx)
	if err != nil {
		return err
	}
	if proj != "" {
		opts.Predicate.Field = andField(opts.Predicate.Field, fields.OneTermEqualSelector("project", proj))
	}
	return s.GetList(ctx, key, opts, listObj)
}

func (m *projectMux) Get(ctx context.Context, key string, opts storage.GetOptions, objPtr runtime.Object) error {
	s, proj, err := m.pickWithProject(ctx)
	if err != nil {
		return err
	}
	return s.Get(ctx, addProjectToKey(m.args.resourcePrefix, proj, key), opts, objPtr)
}

func (m *projectMux) GuaranteedUpdate(ctx context.Context, key string, out runtime.Object, ignoreNotFound bool,
	precond *storage.Preconditions, tryUpdate storage.UpdateFunc, suggestion runtime.Object) error {
	s, proj, err := m.pickWithProject(ctx)
	if err != nil {
		return err
	}
	return s.GuaranteedUpdate(ctx, addProjectToKey(m.args.resourcePrefix, proj, key), out, ignoreNotFound, precond, tryUpdate, suggestion)
}

// If your k8s minor *doesn't* include Count in storage.Interface, delete this.
func (m *projectMux) Count(key string) (int64, error) {
	// Count is a root-space operation in your setup; use the root child.
	s, err := m.rootChild()
	if err != nil {
		return 0, err
	}
	return s.Count(key)
}

// ReadinessCheck proxies to the root child (defaults to the "" project / global).
func (m *projectMux) ReadinessCheck() error {
	s, err := m.rootChild()
	if err != nil {
		return err
	}
	return s.ReadinessCheck()
}

func (m *projectMux) RequestWatchProgress(ctx context.Context) error {
	s, _, err := m.pickWithProject(ctx)
	if err != nil {
		return err
	}
	return s.RequestWatchProgress(ctx)
}

func andField(a, b fields.Selector) fields.Selector {
	if a == nil || a.Empty() {
		return b
	}
	if b == nil || b.Empty() {
		return a
	}
	return fields.AndSelectors(a, b)
}
