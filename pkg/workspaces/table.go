package workspaces

import (
	"context"
	"net/http"
	"sync"

	genericapiserver "k8s.io/apiserver/pkg/server"
)

// Builder spawns (or re-uses) a workspace server for <projectID>.
type Builder func(ctx context.Context, projectID string) (*genericapiserver.GenericAPIServer, error)

type wsEntry struct {
	server *genericapiserver.GenericAPIServer
	stop   context.CancelFunc
}

type Table struct {
	sync.RWMutex
	m     map[string]wsEntry
	build Builder
}

func NewTable(b Builder) *Table {
	return &Table{
		m:     map[string]wsEntry{},
		build: b,
	}
}

func (t *Table) Ensure(parent context.Context, id string) (*genericapiserver.GenericAPIServer, error) {
	t.RLock()
	if e, ok := t.m[id]; ok {
		t.RUnlock()
		return e.server, nil
	}
	t.RUnlock()

	t.Lock()
	defer t.Unlock()

	if e, ok := t.m[id]; ok {
		return e.server, nil
	}

	wsCtx, cancel := context.WithCancel(parent)
	server, err := t.build(wsCtx, id)
	if err != nil {
		cancel()
		return nil, err
	}

	t.m[id] = wsEntry{server: server, stop: cancel}
	return server, nil
}

func (t *Table) Handler(id string) http.Handler {
	t.RLock()
	defer t.RUnlock()
	if e, ok := t.m[id]; ok {
		return e.server.Handler
	}
	return nil
}

func (t *Table) Delete(id string) {
	t.Lock()
	defer t.Unlock()
	if e, ok := t.m[id]; ok {
		e.stop() // cancels wsCtx → workspace exits
		delete(t.m, id)
	}
}
