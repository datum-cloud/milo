package projectstorage

import (
	"context"
	"testing"

	"go.miloapis.com/milo/pkg/request"

	"k8s.io/apiserver/pkg/storage"
	storagebackend "k8s.io/apiserver/pkg/storage/storagebackend"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	generic "k8s.io/apiserver/pkg/registry/generic"
	factory "k8s.io/apiserver/pkg/storage/storagebackend/factory"
	"k8s.io/client-go/tools/cache"
)

type recorder struct {
	rootCreates, unionCreates       int
	rootDestroyed, unionDestroyed   bool
	calls                           []string // e.g. "root:Get alpha/secrets/x"
}

func (r *recorder) record(where, what, key string) {
	r.calls = append(r.calls, where+":"+what+" "+key)
}

type fakeStorage struct {
	where string // "root" or "union"
	rec   *recorder
}

func (f *fakeStorage) Versioner() storage.Versioner { return nil }
func (f *fakeStorage) Create(ctx context.Context, key string, obj, out runtime.Object, ttl uint64) error {
	f.rec.record(f.where, "Create", key); return nil
}
func (f *fakeStorage) Delete(ctx context.Context, key string, out runtime.Object, precond *storage.Preconditions,
	validateDeletion storage.ValidateObjectFunc, cachedExistingObject runtime.Object, opts storage.DeleteOptions) error {
	f.rec.record(f.where, "Delete", key); return nil
}
func (f *fakeStorage) Watch(ctx context.Context, key string, opts storage.ListOptions) (watch.Interface, error) {
	f.rec.record(f.where, "Watch", key); return watch.NewFake(), nil
}
func (f *fakeStorage) Get(ctx context.Context, key string, opts storage.GetOptions, objPtr runtime.Object) error {
	f.rec.record(f.where, "Get", key); return nil
}
func (f *fakeStorage) GetList(ctx context.Context, key string, opts storage.ListOptions, listObj runtime.Object) error {
	f.rec.record(f.where, "GetList", key); return nil
}
func (f *fakeStorage) GuaranteedUpdate(ctx context.Context, key string, out runtime.Object, ignoreNotFound bool,
	precond *storage.Preconditions, tryUpdate storage.UpdateFunc, suggestion runtime.Object) error {
	f.rec.record(f.where, "GuaranteedUpdate", key); return nil
}
func (f *fakeStorage) Count(key string) (int64, error) { f.rec.record(f.where, "Count", key); return 0, nil }
func (f *fakeStorage) ReadinessCheck() error           { f.rec.record(f.where, "ReadinessCheck", ""); return nil }
func (f *fakeStorage) RequestWatchProgress(ctx context.Context) error {
	f.rec.record(f.where, "RequestWatchProgress", ""); return nil
}

func fakeDecorator(rec *recorder) generic.StorageDecorator {
	return func(cfg *storagebackend.ConfigForResource, resourcePrefix string,
		keyFunc func(obj runtime.Object) (string, error), newFunc func() runtime.Object,
		newListFunc func() runtime.Object, getAttrs storage.AttrFunc,
		triggerFn storage.IndexerFuncs, indexers *cache.Indexers,
	) (storage.Interface, factory.DestroyFunc, error) {

		if cfg.Config.Prefix == "/projects" {
			rec.unionCreates++
			fs := &fakeStorage{where: "union", rec: rec}
			return fs, func() { rec.unionDestroyed = true }, nil
		}
		rec.rootCreates++
		fs := &fakeStorage{where: "root", rec: rec}
		return fs, func() { rec.rootDestroyed = true }, nil
	}
}

func newMuxForTest(rec *recorder) *projectMux {
	return &projectMux{
		children: map[string]*child{}, // start empty; lazy-create
		inner:    fakeDecorator(rec),
		cfg:      storagebackend.ConfigForResource{Config: storagebackend.Config{Prefix: "/root"}},
		args:     decoratorArgs{},
	}
}

func TestRootRoutesToRootChild(t *testing.T) {
	rec := &recorder{}
	m := newMuxForTest(rec)

	if err := m.Get(context.Background(), "secrets/x", storage.GetOptions{}, nil); err != nil {
		t.Fatal(err)
	}
	if rec.rootCreates != 1 || rec.unionCreates != 0 {
		t.Fatalf("expected root=1, union=0 creates; got %d, %d", rec.rootCreates, rec.unionCreates)
	}
	if want, got := "root:Get secrets/x", rec.calls[0]; got != want {
		t.Fatalf("want %q got %q", want, got)
	}
}

func TestProjectRoutesToUnionAndRewritesKey(t *testing.T) {
	rec := &recorder{}
	m := newMuxForTest(rec)
	ctx := request.WithProject(context.Background(), "alpha")

	if err := m.Get(ctx, "secrets/x", storage.GetOptions{}, nil); err != nil {
		t.Fatal(err)
	}
	if rec.unionCreates != 1 {
		t.Fatalf("expected one union create, got %d", rec.unionCreates)
	}
	if want, got := "union:Get alpha/secrets/x", rec.calls[0]; got != want {
		t.Fatalf("want %q got %q", want, got)
	}

	// Leading slash is trimmed
	if err := m.Get(ctx, "/secrets/y", storage.GetOptions{}, nil); err != nil {
		t.Fatal(err)
	}
	if want, got := "union:Get alpha/secrets/y", rec.calls[1]; got != want {
		t.Fatalf("want %q got %q", want, got)
	}
}

func TestUnionChildCreatedOnceAcrossProjects(t *testing.T) {
	rec := &recorder{}
	m := newMuxForTest(rec)

	if err := m.Get(request.WithProject(context.Background(), "alpha"), "k/x", storage.GetOptions{}, nil); err != nil {
		t.Fatal(err)
	}
	if err := m.Get(request.WithProject(context.Background(), "beta"), "k/y", storage.GetOptions{}, nil); err != nil {
		t.Fatal(err)
	}
	if rec.unionCreates != 1 {
		t.Fatalf("expected union created once, got %d", rec.unionCreates)
	}
}

func TestCountAndReadinessUseRoot(t *testing.T) {
	rec := &recorder{}
	m := newMuxForTest(rec)

	if _, err := m.Count("anything"); err != nil {
		t.Fatal(err)
	}
	if err := m.ReadinessCheck(); err != nil {
		t.Fatal(err)
	}
	if rec.rootCreates != 1 {
		t.Fatalf("root should be created once; got %d", rec.rootCreates)
	}
	// Ensure calls were recorded on root
	foundCount, foundReady := false, false
	for _, c := range rec.calls {
		if c == "root:Count anything" {
			foundCount = true
		}
		if c == "root:ReadinessCheck " {
			foundReady = true
		}
	}
	if !foundCount || !foundReady {
		t.Fatalf("root Count/ReadinessCheck not invoked as expected: %v", rec.calls)
	}
}

func TestDestroyAllInvokesDestroys(t *testing.T) {
	rec := &recorder{}
	m := newMuxForTest(rec)

	// Touch both children
	if err := m.Get(context.Background(), "k/x", storage.GetOptions{}, nil); err != nil {
		t.Fatal(err)
	}
	if err := m.Get(request.WithProject(context.Background(), "alpha"), "k/y", storage.GetOptions{}, nil); err != nil {
		t.Fatal(err)
	}

	m.destroyAll()
	if !rec.rootDestroyed || !rec.unionDestroyed {
		t.Fatalf("expected both destroys true; root=%v union=%v", rec.rootDestroyed, rec.unionDestroyed)
	}
}
