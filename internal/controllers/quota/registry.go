package quota

import (
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	ctrl "sigs.k8s.io/controller-runtime"
)

// ControllerSetup defines a function that sets up a controller with a manager.
type ControllerSetup func(mgr ctrl.Manager, dynamicClient dynamic.Interface) error

// QuotaControllerRegistry provides a centralized way to register and setup all quota controllers.
type QuotaControllerRegistry struct {
	controllers map[string]ControllerSetup
	logger      logr.Logger
}

// NewQuotaControllerRegistry creates a new registry with all quota controllers.
func NewQuotaControllerRegistry(logger logr.Logger) *QuotaControllerRegistry {
	registry := &QuotaControllerRegistry{
		controllers: make(map[string]ControllerSetup),
		logger:      logger,
	}

	// Register all quota controllers
	registry.registerControllers()
	return registry
}

// registerControllers registers all quota controllers with their setup functions.
func (r *QuotaControllerRegistry) registerControllers() {
	// ResourceRegistration controller
	r.controllers["ResourceRegistrationController"] = func(mgr ctrl.Manager, dynamicClient dynamic.Interface) error {
		controller := &ResourceRegistrationController{
			Client: mgr.GetClient(),
		}
		return controller.SetupWithManager(mgr)
	}

	// ResourceGrant controller
	r.controllers["ResourceGrantController"] = func(mgr ctrl.Manager, dynamicClient dynamic.Interface) error {
		controller := &ResourceGrantController{
			Client: mgr.GetClient(),
		}
		return controller.SetupWithManager(mgr)
	}

    // ResourceQuotaSummary removed in current architecture

    // ResourceClaim controller
    r.controllers["ResourceClaimController"] = func(mgr ctrl.Manager, dynamicClient dynamic.Interface) error {
        controller := &ResourceClaimController{
            Client: mgr.GetClient(),
        }
        return controller.SetupWithManager(mgr)
    }

    // AllowanceBucket controller (single source of aggregated quota data)
    r.controllers["AllowanceBucketController"] = func(mgr ctrl.Manager, dynamicClient dynamic.Interface) error {
        controller := &AllowanceBucketController{
            Client: mgr.GetClient(),
            Scheme: mgr.GetScheme(),
        }
        return controller.SetupWithManager(mgr)
    }

	// ClaimCreationPolicy controller
	r.controllers["ClaimCreationPolicyReconciler"] = func(mgr ctrl.Manager, dynamicClient dynamic.Interface) error {
		controller := &ClaimCreationPolicyReconciler{
			Client: mgr.GetClient(),
			Scheme: mgr.GetScheme(),
		}
		return controller.SetupWithManager(mgr)
	}

	// ResourceClaimOwnership controller (requires dynamic client)
	r.controllers["ResourceClaimOwnershipController"] = func(mgr ctrl.Manager, dynamicClient dynamic.Interface) error {
		// Get discovery client from the REST config
		restConfig := mgr.GetConfig()

		// Create discovery client
		discoveryClient, err := discovery.NewDiscoveryClientForConfig(restConfig)
		if err != nil {
			// Log warning but continue without discovery client
			r.logger.Info("Failed to create discovery client, will use default versions",
				"error", err)
		}

		controller := &ResourceClaimOwnershipController{
			Client:          mgr.GetClient(),
			DynamicClient:   dynamicClient,
			DiscoveryClient: discoveryClient,
			Scheme:          mgr.GetScheme(),
		}
		return controller.SetupWithManager(mgr)
	}
}

// SetupAllControllers registers all quota controllers with the provided manager.
func (r *QuotaControllerRegistry) SetupAllControllers(mgr ctrl.Manager, dynamicClient dynamic.Interface) error {
	r.logger.Info("Setting up quota controllers", "count", len(r.controllers))

	for name, setupFunc := range r.controllers {
		r.logger.V(1).Info("Setting up quota controller", "controller", name)

		if err := setupFunc(mgr, dynamicClient); err != nil {
			return fmt.Errorf("failed to setup %s: %w", name, err)
		}

		r.logger.V(1).Info("Successfully set up quota controller", "controller", name)
	}

	r.logger.Info("All quota controllers set up successfully")
	return nil
}

// SetupController registers a specific controller by name.
func (r *QuotaControllerRegistry) SetupController(name string, mgr ctrl.Manager, dynamicClient dynamic.Interface) error {
	setupFunc, exists := r.controllers[name]
	if !exists {
		return fmt.Errorf("controller %s not found in registry", name)
	}

	r.logger.Info("Setting up specific quota controller", "controller", name)
	if err := setupFunc(mgr, dynamicClient); err != nil {
		return fmt.Errorf("failed to setup %s: %w", name, err)
	}

	r.logger.Info("Successfully set up quota controller", "controller", name)
	return nil
}

// ListControllers returns the names of all registered controllers.
func (r *QuotaControllerRegistry) ListControllers() []string {
	names := make([]string, 0, len(r.controllers))
	for name := range r.controllers {
		names = append(names, name)
	}
	return names
}

// GetControllerCount returns the number of registered controllers.
func (r *QuotaControllerRegistry) GetControllerCount() int {
	return len(r.controllers)
}
