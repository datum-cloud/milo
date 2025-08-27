package projectstorage

import (
	"bytes"
	"context"
	"strconv"
	"strings"
	"sync"
	"time"

	v3 "go.etcd.io/etcd/client/v3"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/storage"
	"k8s.io/klog/v2"
)

type projectUnionStore struct {
	// raw union store (Prefix="/projects") for CRUD; we don't use it for list/watch.
	delegate storage.Interface

	// resource prefix for this GVR, e.g. "gateway.networking.k8s.io/gatewayclasses" or "secrets"
	rp string

	// object factories
	newFunc     func() runtime.Object
	newListFunc func() runtime.Object

	// etcd client watching /projects/** (single stream)
	cli     *v3.Client
	stopCh  chan struct{}
	stopped sync.Once

	// map object UID -> project (virtual tenancy field; used by GetAttrs)
	uidToProject sync.Map // map[types.UID]string

	decoder func(ctx context.Context, raw []byte, into runtime.Object) error // optional

}

// NOTE: pass in your etcd client (recommended). If you can't yet, keep cli=nil and
// we'll fall back to delegate.List/Watch (less ideal).
func NewProjectUnionStore(delegate storage.Interface, resourcePrefix string, newFunc func() runtime.Object, newListFunc func() runtime.Object) *projectUnionStore {
	return &projectUnionStore{
		delegate:    delegate,
		rp:          trimSlash(resourcePrefix),
		newFunc:     newFunc,
		newListFunc: newListFunc,
		stopCh:      make(chan struct{}),
	}
}

func (s *projectUnionStore) WithDecoder(fn func(context.Context, []byte, runtime.Object) error) *projectUnionStore {
	s.decoder = fn
	return s
}

func (s *projectUnionStore) decodeFromWatch(ctx context.Context, raw []byte, rel string, rv int64, into runtime.Object) error {
	if s.decoder != nil && len(raw) > 0 {
		return s.decoder(ctx, raw, into)
	}
	// fallback: exact-rev re-GET if decoder not available
	return s.delegate.Get(ctx, rel, storage.GetOptions{ResourceVersion: strconv.FormatInt(rv, 10)}, into)
}

// Optional helper if you want to inject the etcd client after construction.
func (s *projectUnionStore) WithEtcdClient(cli *v3.Client) *projectUnionStore {
	s.cli = cli
	return s
}

func (s *projectUnionStore) Stop() {
	s.stopped.Do(func() {
		close(s.stopCh)
		if s.cli != nil {
			_ = s.cli.Close()
		}
	})
}

func trimSlash(sv string) string {
	for len(sv) > 0 && sv[0] == '/' {
		sv = sv[1:]
	}
	return sv
}

// -------- storage.Interface: delegate CRUD to the raw union store --------

func (s *projectUnionStore) Versioner() storage.Versioner { return s.delegate.Versioner() }

func (s *projectUnionStore) Create(ctx context.Context, key string, obj, out runtime.Object, ttl uint64) error {
	return s.delegate.Create(ctx, key, obj, out, ttl)
}
func (s *projectUnionStore) Delete(ctx context.Context, key string, out runtime.Object, precond *storage.Preconditions,
	validateDeletion storage.ValidateObjectFunc, cachedExistingObject runtime.Object, opts storage.DeleteOptions) error {
	return s.delegate.Delete(ctx, key, out, precond, validateDeletion, cachedExistingObject, opts)
}
func (s *projectUnionStore) Get(ctx context.Context, key string, opts storage.GetOptions, objPtr runtime.Object) error {
	return s.delegate.Get(ctx, key, opts, objPtr)
}
func (s *projectUnionStore) GuaranteedUpdate(ctx context.Context, key string, out runtime.Object, ignoreNotFound bool,
	precond *storage.Preconditions, tryUpdate storage.UpdateFunc, suggestion runtime.Object) error {
	return s.delegate.GuaranteedUpdate(ctx, key, out, ignoreNotFound, precond, tryUpdate, suggestion)
}
func (s *projectUnionStore) Count(key string) (int64, error) { return s.delegate.Count(key) }
func (s *projectUnionStore) ReadinessCheck() error           { return s.delegate.ReadinessCheck() }
func (s *projectUnionStore) RequestWatchProgress(ctx context.Context) error {
	if s.cli != nil {
		return s.cli.RequestProgress(ctx)
	}
	return s.delegate.RequestWatchProgress(ctx)
}

// -------- List/Watch used by the stock Cacher population pipeline --------

// GetList: scan /projects/** and keep only keys for this resource prefix.
// We fetch decoded objects via the delegate store to reuse its codec/transformer.
func (s *projectUnionStore) GetList(ctx context.Context, key string, opts storage.ListOptions, out runtime.Object) error {
	if s.cli == nil {
		return s.delegate.GetList(ctx, key, opts, out) // forward key, not ""
	}
	resp, err := s.cli.Get(ctx, "/projects", v3.WithPrefix())
	if err != nil {
		return err
	}

	items := []runtime.Object{}
	for _, kv := range resp.Kvs {
		if !s.matchesResource(kv.Key) {
			continue
		}

		proj, ok := s.projectFromKey(kv.Key)
		if !ok {
			continue
		}

		// Decode via delegate to honor codec/transformer
		rel := relKeyFromEtcdKey(kv.Key)
		obj := s.newFunc()
		if err := s.delegate.Get(ctx, rel, storage.GetOptions{}, obj); err != nil {
			klog.V(4).InfoS("delegate Get during list failed", "key", rel, "err", err)
			continue
		}

		// Populate UID→project for future predicate checks
		if uid := objectUID(obj); uid != "" {
			s.uidToProject.Store(uid, proj)
		}

		// ---- APPLY PREDICATE using a virtual "project" field ----
		lbls, flds, _ := s.GetAttrs(obj)
		if _, ok := flds["project"]; !ok {
			// inject virtual field so SelectionPredicate matches now
			f2 := fields.Set{}
			for k, v := range flds {
				f2[k] = v
			}
			f2["project"] = proj
			flds = f2
		}
		if !opts.Predicate.Label.Empty() && !opts.Predicate.Label.Matches(lbls) {
			continue
		}
		if !opts.Predicate.Field.Empty() && !opts.Predicate.Field.Matches(flds) {
			continue
		}
		// ---------------------------------------------------------

		items = append(items, obj)
	}

	if err := meta.SetList(out, items); err != nil {
		return err
	}
	if acc, err := meta.ListAccessor(out); err == nil && acc.GetResourceVersion() == "" {
		acc.SetResourceVersion(strconv.FormatInt(resp.Header.Revision, 10))
	}
	return nil
}

// Watch: single backend watch on /projects/**, filter by resource prefix before decode and forward.
func (s *projectUnionStore) Watch(ctx context.Context, key string, opts storage.ListOptions) (watch.Interface, error) {
	if s.cli == nil {
		// Fallback: delegate (may miss nested keys)
		return s.delegate.Watch(ctx, key, opts)
	}
	w := watch.NewRaceFreeFake()
	go func() {
		defer w.Stop()
		var startRev int64
		if rv := opts.ResourceVersion; rv != "" {
			startRev = parseRV(rv)
		}

		optsV3 := []v3.OpOption{v3.WithPrefix(), v3.WithRev(startRev), v3.WithPrevKV()}
		if opts.ProgressNotify {
			optsV3 = append(optsV3, v3.WithProgressNotify())
		}

		rch := s.cli.Watch(ctx, "/projects", optsV3...)
		for {
			select {
			case <-ctx.Done():
				return
			case <-s.stopCh:
				return
			case wr, ok := <-rch:
				if !ok {
					return
				}
				if err := wr.Err(); err != nil {
					// If compaction or canceled: exit so caller (cacher) can re-list.
					if wr.CompactRevision != 0 || wr.Canceled {
						klog.V(3).InfoS("projects watch canceled", "compactRev", wr.CompactRevision, "err", err)
						return
					}
					// transient error: short backoff and continue
					klog.V(3).InfoS("projects watch transient error", "err", err)
					time.Sleep(200 * time.Millisecond)
					continue
				}
				for _, ev := range wr.Events {
					key := ev.Kv.Key
					if !s.matchesResource(key) {
						continue
					}
					proj, ok := s.projectFromKey(key)
					if !ok {
						continue
					}
					rel := relKeyFromEtcdKey(key)

					switch ev.Type {
					case v3.EventTypePut:
						obj := s.newFunc()
						if err := s.decodeFromWatch(ctx, ev.Kv.Value, rel, ev.Kv.ModRevision, obj); err != nil {
							continue
						}
						if uid := objectUID(obj); uid != "" {
							s.uidToProject.Store(uid, proj)
						}
						if ev.Kv.CreateRevision == ev.Kv.ModRevision {
							w.Action(watch.Added, obj)
						} else {
							w.Action(watch.Modified, obj)
						}
					case v3.EventTypeDelete:
						obj := s.newFunc()
						if ev.PrevKv != nil {
							_ = s.decodeFromWatch(ctx, ev.PrevKv.Value, rel, ev.PrevKv.ModRevision, obj)
							if uid := objectUID(obj); uid != "" {
								s.uidToProject.Delete(uid)
							}
						}
						w.Action(watch.Deleted, obj)
					}
				}
			}
		}
	}()
	return w, nil
}

// ---- helpers ----

func (s *projectUnionStore) matchesResource(key []byte) bool {
	// /projects/<project>/(namespaces/<ns>/)?<rp>(/...)?
	pfx := []byte("/projects/")
	if !bytes.HasPrefix(key, pfx) {
		return false
	}
	rest := key[len(pfx):]

	// cut <project>/
	i := bytes.IndexByte(rest, '/')
	if i < 0 {
		return false
	}
	afterProj := rest[i+1:]

	rp := []byte(s.rp)

	// Special-case the "namespaces" resource itself:
	// /projects/<project>/namespaces/<name>
	if bytes.Equal(rp, []byte("namespaces")) {
		return bytes.HasPrefix(afterProj, []byte("namespaces/"))
	}

	// Namespaced resources:
	// /projects/<project>/namespaces/<ns>/<rp>(/...)?
	if bytes.HasPrefix(afterProj, []byte("namespaces/")) {
		// skip "namespaces/"
		afterNs := afterProj[len("namespaces/"):]
		// skip "<ns>/"
		j := bytes.IndexByte(afterNs, '/')
		if j < 0 {
			return false
		} // we've got only "namespaces/<ns>"
		afterNs = afterNs[j+1:] // now at "<rp>(/...)?"
		if !bytes.HasPrefix(afterNs, rp) {
			return false
		}
		return len(afterNs) == len(rp) || afterNs[len(rp)] == '/'
	}

	// Cluster-scoped resources (not namespaces):
	// /projects/<project>/<rp>(/...)?
	if !bytes.HasPrefix(afterProj, rp) {
		return false
	}
	return len(afterProj) == len(rp) || afterProj[len(rp)] == '/'
}

func (s *projectUnionStore) projectFromKey(key []byte) (string, bool) {
	pfx := []byte("/projects/")
	if !bytes.HasPrefix(key, pfx) {
		return "", false
	}
	rest := key[len(pfx):]
	i := bytes.IndexByte(rest, '/')
	if i < 0 {
		return "", false
	}
	return string(rest[:i]), true
}

func relKeyFromEtcdKey(full []byte) string {
	// convert etcd absolute key to delegate-relative key (strip "/projects/")
	s := string(full)
	return strings.TrimPrefix(strings.TrimPrefix(s, "/"), "projects/") // handles "/projects/..." and "projects/..."
}

func objectUID(obj runtime.Object) types.UID {
	acc, err := meta.Accessor(obj)
	if err != nil {
		return ""
	}
	return acc.GetUID()
}

func parseRV(rv string) int64 {
	n, err := strconv.ParseInt(rv, 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// GetAttrs is used by the Cacher to evaluate predicates. We add a virtual "project" field.
func (s *projectUnionStore) GetAttrs(obj runtime.Object) (labels.Set, fields.Set, error) {
	l := labels.Set(nil)
	f := fields.Set{}
	if acc, err := meta.Accessor(obj); err == nil {
		if acc.GetLabels() != nil {
			l = labels.Set(acc.GetLabels())
		}
		if ns := acc.GetNamespace(); ns != "" {
			f["metadata.namespace"] = ns
		}
		f["metadata.name"] = acc.GetName()
		if uid := acc.GetUID(); uid != "" {
			if p, ok := s.uidToProject.Load(uid); ok {
				f["project"] = p.(string) // virtual field used for per-project filtering
			}
		}
	}
	return l, f, nil
}
