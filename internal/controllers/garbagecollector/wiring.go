// pkg/controller/garbagecollector/wiring.go
package garbagecollector

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	metadatainformer "k8s.io/client-go/metadata/metadatainformer"
	"k8s.io/controller-manager/pkg/informerfactory"

	"k8s.io/client-go/informers"
	clientset "k8s.io/client-go/kubernetes"

	"k8s.io/client-go/discovery"
	"k8s.io/client-go/metadata"
	"k8s.io/client-go/rest"
)

type GCSink struct {
	GC                *GarbageCollector
	RootRESTMapper    meta.ResettableRESTMapper // meta.ResettableRESTMapper under the hood
	Ignored           map[schema.GroupResource]struct{}
	InformersStarted  <-chan struct{}
	InitialSyncPeriod time.Duration
}

func (s *GCSink) AddProject(ctx context.Context, id string, cfg *rest.Config) error {
	k8sProj := clientset.NewForConfigOrDie(cfg)
	mdProj := metadata.NewForConfigOrDie(cfg)
	discProj := discovery.NewDiscoveryClientForConfigOrDie(cfg)

	// Per-project factories (separate caches per partition)
	coreFact := informers.NewSharedInformerFactory(k8sProj, ResourceResyncTime)
	metaFact := metadatainformer.NewSharedInformerFactory(mdProj, ResourceResyncTime)
	composite := informerfactory.NewInformerFactory(coreFact, metaFact)

	return s.GC.AddProject(
		ctx,
		id,
		mdProj,
		s.RootRESTMapper,
		s.Ignored,
		composite,
		s.InformersStarted,
		discProj,
		s.InitialSyncPeriod,
	)
}

func (s *GCSink) RemoveProject(id string) {
	s.GC.RemoveProject(id)
}
