package milo

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"go.miloapis.com/milo/pkg/apis/resourcemanager/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	mcmanager "sigs.k8s.io/multicluster-runtime/pkg/manager"
	"sigs.k8s.io/multicluster-runtime/pkg/multicluster"
)

// Built following the cluster-api provider as an example.
// See: https://sigs.k8s.io/multicluster-runtime/blob/7abad14c6d65fdaf9b83a2b1d9a2c99140d18e7d/providers/cluster-api/provider.go

var _ multicluster.Provider = &Provider{}

var projectGVK = v1alpha1.GroupVersion.WithKind("Project")
var projectControlPlaneGVK = v1alpha1.GroupVersion.WithKind("ProjectControlPlane")

// Options are the options for the Datum cluster Provider.
type Options struct {
	// ClusterOptions are the options passed to the cluster constructor.
	ClusterOptions []cluster.Option

	// InternalServiceDiscovery will result in the provider to look for
	// ProjectControlPlane resources in the local manager's cluster, and establish
	// a connection via the internal service address. Otherwise, the provider will
	// look for Project resources in the cluster and expect to connect to the
	// external Datum API endpoint.
	InternalServiceDiscovery bool

	// ProjectRestConfig is the rest config to use when connecting to project
	// API endpoints. If not provided, the provider will use the rest config
	// from the local manager.
	ProjectRestConfig *rest.Config

	// LabelSelector is an optional selector to filter projects based on labels.
	// When provided, only projects matching this selector will be reconciled.
	LabelSelector *metav1.LabelSelector
}

// New creates a new Datum cluster Provider.
func New(localMgr manager.Manager, opts Options) (*Provider, error) {
	p := &Provider{
		opts:              opts,
		log:               log.Log.WithName("datum-cluster-provider"),
		client:            localMgr.GetClient(),
		projectRestConfig: opts.ProjectRestConfig,
		projects:          map[string]cluster.Cluster{},
		cancelFns:         map[string]context.CancelFunc{},
	}

	if p.projectRestConfig == nil {
		p.projectRestConfig = localMgr.GetConfig()
	}

	var project unstructured.Unstructured
	if p.opts.InternalServiceDiscovery {
		project.SetGroupVersionKind(projectControlPlaneGVK)
	} else {
		project.SetGroupVersionKind(projectGVK)
	}

	var forOpts []builder.ForOption
	if opts.LabelSelector != nil {
		selector, err := metav1.LabelSelectorAsSelector(opts.LabelSelector)
		if err != nil {
			return nil, fmt.Errorf("failed to create selector from label selector: %w", err)
		}

		labelPredicate := predicate.NewPredicateFuncs(func(obj client.Object) bool {
			return selector.Matches(labels.Set(obj.GetLabels()))
		})

		forOpts = append(forOpts, builder.WithPredicates(labelPredicate))
	}

	controllerBuilder := builder.ControllerManagedBy(localMgr).
		For(&project, forOpts...).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1})

	if err := controllerBuilder.Complete(p); err != nil {
		return nil, fmt.Errorf("failed to create controller: %w", err)
	}

	return p, nil
}

type index struct {
	object       client.Object
	field        string
	extractValue client.IndexerFunc
}

// Provider is a cluster Provider that works with Datum
type Provider struct {
	opts              Options
	log               logr.Logger
	projectRestConfig *rest.Config
	client            client.Client

	lock      sync.Mutex
	mcMgr     mcmanager.Manager
	projects  map[string]cluster.Cluster
	cancelFns map[string]context.CancelFunc
	indexers  []index
}

// Get returns the cluster with the given name, if it is known.
func (p *Provider) Get(_ context.Context, clusterName string) (cluster.Cluster, error) {
	p.lock.Lock()
	defer p.lock.Unlock()
	if cl, ok := p.projects[clusterName]; ok {
		return cl, nil
	}

	return nil, fmt.Errorf("cluster %s not found", clusterName)
}

// Run starts the provider and blocks.
func (p *Provider) Run(ctx context.Context, mgr mcmanager.Manager) error {
	p.log.Info("Starting Datum cluster provider")

	p.lock.Lock()
	p.mcMgr = mgr
	p.lock.Unlock()

	<-ctx.Done()

	return ctx.Err()
}

func (p *Provider) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := p.log.WithValues("project", req.Name)
	log.Info("Reconciling Project")

	key := req.String()
	var project unstructured.Unstructured

	if p.opts.InternalServiceDiscovery {
		project.SetGroupVersionKind(projectControlPlaneGVK)
	} else {
		project.SetGroupVersionKind(projectGVK)
	}

	if err := p.client.Get(ctx, req.NamespacedName, &project); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("removing cluster for project")
			p.lock.Lock()
			defer p.lock.Unlock()

			delete(p.projects, key)
			if cancel, ok := p.cancelFns[key]; ok {
				cancel()
			}

			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("failed to get project: %w", err)
	}

	p.lock.Lock()
	defer p.lock.Unlock()

	// Make sure the manager has started
	// TODO(jreese) what condition would lead to this?
	if p.mcMgr == nil {
		return ctrl.Result{RequeueAfter: time.Second * 2}, nil
	}

	// already engaged?
	if _, ok := p.projects[key]; ok {
		log.Info("Project already engaged")
		return ctrl.Result{}, nil
	}

	// ready and provisioned?
	conditions, err := extractUnstructuredConditions(project.Object)
	if err != nil {
		return ctrl.Result{}, err
	}

	if p.opts.InternalServiceDiscovery {
		if !apimeta.IsStatusConditionTrue(conditions, "ControlPlaneReady") {
			log.Info("ProjectControlPlane is not ready")
			return ctrl.Result{}, nil
		}
	} else {
		if !apimeta.IsStatusConditionTrue(conditions, "Ready") {
			log.Info("Project is not ready")
			return ctrl.Result{}, nil
		}
	}

	cfg := rest.CopyConfig(p.projectRestConfig)
	apiHost, err := url.Parse(cfg.Host)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to parse host from rest config: %w", err)
	}

	if p.opts.InternalServiceDiscovery {
		apiHost.Path = ""
		apiHost.Host = fmt.Sprintf("milo-apiserver.project-%s.svc.cluster.local:6443", project.GetUID())
	} else {
		apiHost.Path = fmt.Sprintf("/apis/resourcemanager.miloapis.com/v1alpha1/projects/%s/control-plane", project.GetName())
	}
	cfg.Host = apiHost.String()

	// create cluster.
	cl, err := cluster.New(cfg, p.opts.ClusterOptions...)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create cluster: %w", err)
	}
	for _, idx := range p.indexers {
		if err := cl.GetCache().IndexField(ctx, idx.object, idx.field, idx.extractValue); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to index field %q: %w", idx.field, err)
		}
	}

	clusterCtx, cancel := context.WithCancel(ctx)
	go func() {
		if err := cl.Start(clusterCtx); err != nil {
			log.Error(err, "failed to start cluster")
			return
		}
	}()

	if !cl.GetCache().WaitForCacheSync(ctx) {
		cancel()
		return ctrl.Result{}, fmt.Errorf("failed to sync cache")
	}

	// store project client
	p.projects[key] = cl
	p.cancelFns[key] = cancel

	p.log.Info("Added new cluster")

	// engage manager.
	if err := p.mcMgr.Engage(clusterCtx, key, cl); err != nil {
		log.Error(err, "failed to engage manager")
		delete(p.projects, key)
		delete(p.cancelFns, key)
		return reconcile.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (p *Provider) IndexField(ctx context.Context, obj client.Object, field string, extractValue client.IndexerFunc) error {
	p.lock.Lock()
	defer p.lock.Unlock()

	// save for future projects.
	p.indexers = append(p.indexers, index{
		object:       obj,
		field:        field,
		extractValue: extractValue,
	})

	// apply to existing projects.
	for name, cl := range p.projects {
		if err := cl.GetCache().IndexField(ctx, obj, field, extractValue); err != nil {
			return fmt.Errorf("failed to index field %q on project %q: %w", field, name, err)
		}
	}
	return nil
}

func extractUnstructuredConditions(
	obj map[string]interface{},
) ([]metav1.Condition, error) {
	conditions, ok, _ := unstructured.NestedSlice(obj, "status", "conditions")
	if !ok {
		return nil, nil
	}

	wrappedConditions := map[string]interface{}{
		"conditions": conditions,
	}

	var typedConditions struct {
		Conditions []metav1.Condition `json:"conditions"`
	}

	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(wrappedConditions, &typedConditions); err != nil {
		return nil, fmt.Errorf("failed converting unstructured conditions: %w", err)
	}

	return typedConditions.Conditions, nil
}
