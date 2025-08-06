// virtual_router.go
package app

import (
	"context"

	"go.miloapis.com/milo/pkg/server/filters"
	"go.miloapis.com/milo/pkg/workspaces"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/client-go/discovery"
	controlplaneapiserver "k8s.io/kubernetes/pkg/controlplane/apiserver"
)

func attachRouterToServer(
	rootCtx context.Context,
	srv *genericapiserver.GenericAPIServer,
	factory *workspaces.Factory,
) {
	table := workspaces.NewTable(func(_ context.Context, id string) (*genericapiserver.GenericAPIServer, error) {
		return factory.Build(rootCtx, id)
	})

	// Wrap only the outer handler chain.
	srv.Handler.FullHandlerChain = filters.ProjectRouter(table)(srv.Handler.FullHandlerChain)
}

func BuildProjectStorage(
	discovery discovery.DiscoveryInterface,
	cfg *CompletedConfig, // reuse existing method
) ([]controlplaneapiserver.RESTStorageProvider, error) {
	return cfg.GenericStorageProviders(discovery)
}
