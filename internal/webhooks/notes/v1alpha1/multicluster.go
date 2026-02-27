package v1alpha1

import (
	"context"

	miloprovider "go.miloapis.com/milo/pkg/multicluster-runtime/milo"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	mcmanager "sigs.k8s.io/multicluster-runtime/pkg/manager"
)

// ClusterGetter provides access to clusters in the multicluster runtime.
type ClusterGetter interface {
	// GetCluster returns a cluster by name. Empty string returns the local cluster.
	GetCluster(ctx context.Context, name string) (cluster.Cluster, error)
	// ListClusterNames returns names of all engaged clusters including "" for local.
	ListClusterNames() []string
}

// MultiClusterGetter implements ClusterGetter using mcmanager and provider.
type MultiClusterGetter struct {
	Manager  mcmanager.Manager
	Provider *miloprovider.Provider
}

// GetCluster returns a cluster by name. Empty string returns the local cluster.
func (g *MultiClusterGetter) GetCluster(ctx context.Context, name string) (cluster.Cluster, error) {
	return g.Manager.GetCluster(ctx, name)
}

// ListClusterNames returns names of all engaged clusters including "" for local.
func (g *MultiClusterGetter) ListClusterNames() []string {
	// Start with local cluster (empty string)
	names := []string{""}
	// Add all project control planes
	if g.Provider != nil {
		names = append(names, g.Provider.ListEngagedClusters()...)
	}
	return names
}
