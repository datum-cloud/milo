package projectprovider

import (
	"context"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/restmapper"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

type Sink interface {
	AddProject(ctx context.Context, id string, cfg *rest.Config) error
	RemoveProject(id string)
}

type Provider struct {
	root       *rest.Config
	dyn        dynamic.Interface
	sink       Sink
	projectGVR schema.GroupVersionResource
}

func New(root *rest.Config, sink Sink) (*Provider, error) {
	dyn, err := dynamic.NewForConfig(root)
	if err != nil {
		return nil, err
	}
	gvr, err := resolveProjectGVR(root, "resourcemanager.miloapis.com", "v1alpha1")
	if err != nil {
		return nil, err
	}
	return &Provider{root: root, dyn: dyn, sink: sink, projectGVR: gvr}, nil
}

func (p *Provider) cfgForProject(id string) *rest.Config {
	c := rest.CopyConfig(p.root)
	c.Host = strings.TrimSuffix(p.root.Host, "/") + "/projects/" + id + "/control-plane"
	return c
}

func (p *Provider) Run(ctx context.Context) error {
	lw := &cache.ListWatch{
		ListFunc: func(lo metav1.ListOptions) (runtime.Object, error) {
			return p.dyn.Resource(p.projectGVR).List(ctx, lo)
		},
		WatchFunc: func(lo metav1.ListOptions) (watch.Interface, error) {
			return p.dyn.Resource(p.projectGVR).Watch(ctx, lo)
		},
	}
	inf := cache.NewSharedIndexInformer(lw, &unstructured.Unstructured{}, 0, nil)

	inf.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(o interface{}) {
			id := o.(*unstructured.Unstructured).GetName()
			err := p.sink.AddProject(ctx, id, p.cfgForProject(id))
			if err != nil {
				// Log the error but continue processing other projects
				klog.Errorf("Failed to add project %q: %v", id, err)
			}
		},
		DeleteFunc: func(o interface{}) {
			id := o.(*unstructured.Unstructured).GetName()
			p.sink.RemoveProject(id)
		},
	})

	go inf.Run(ctx.Done())
	<-ctx.Done()
	return nil
}

func resolveProjectGVR(cfg *rest.Config, group, preferredVersion string) (schema.GroupVersionResource, error) {
	disc, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return schema.GroupVersionResource{}, err
	}
	rm := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(disc))

	// If preferredVersion == "", RESTMapping will pick the preferred version
	mapping, err := rm.RESTMapping(schema.GroupKind{Group: group, Kind: "Project"}, preferredVersion)
	if err != nil {
		return schema.GroupVersionResource{}, err
	}
	return mapping.Resource, nil
}
