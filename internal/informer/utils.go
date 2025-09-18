package informer

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlmanager "sigs.k8s.io/controller-runtime/pkg/manager"
)

// NewManagerFromConfig creates a new dynamic informer manager from a REST config.
func NewManagerFromConfig(config *rest.Config) (Manager, error) {
	// Create a copy of the config with increased rate limits for informer operations
	informerConfig := *config
	if informerConfig.QPS == 0 || informerConfig.QPS < 200 {
		informerConfig.QPS = 200   // High QPS for informer operations
		informerConfig.Burst = 400 // High burst for informer operations
	}

	// Create Kubernetes client
	kubeClient, err := client.New(&informerConfig, client.Options{})
	if err != nil {
		return nil, err
	}

	// Create dynamic client
	dynamicClient, err := dynamic.NewForConfig(&informerConfig)
	if err != nil {
		return nil, err
	}

	// Create discovery client for REST mapping
	_, err = discovery.NewDiscoveryClientForConfig(&informerConfig)
	if err != nil {
		return nil, err
	}

	// Create REST mapper
	restMapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{})
	// Note: In practice, you might want to use a more sophisticated REST mapper
	// that can discover mappings dynamically. For now, we'll rely on the
	// manager's REST mapper when using NewManagerFromManager.

	return NewManager(kubeClient, dynamicClient, restMapper), nil
}

// NewManagerFromManager creates a new dynamic informer manager from a controller-runtime manager.
func NewManagerFromManager(mgr ctrlmanager.Manager) (Manager, error) {
	// Create a copy of the manager's config with increased rate limits
	managerConfig := *mgr.GetConfig()
	if managerConfig.QPS == 0 || managerConfig.QPS < 200 {
		managerConfig.QPS = 200   // High QPS for dynamic informer operations
		managerConfig.Burst = 400 // High burst for dynamic informer operations
	}

	// Create dynamic client from manager's config with higher rate limits
	dynamicClient, err := dynamic.NewForConfig(&managerConfig)
	if err != nil {
		return nil, err
	}

	return NewManager(mgr.GetClient(), dynamicClient, mgr.GetRESTMapper()), nil
}

// AddManagerToManager adds the dynamic informer manager as a runnable to a controller-runtime manager.
// This ensures the informer manager starts and stops with the controller manager.
func AddManagerToManager(mgr ctrlmanager.Manager, infMgr Manager) error {
	return mgr.Add(infMgr)
}
