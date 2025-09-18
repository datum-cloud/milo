// QuotaControllerRegistry centralizes setup for quota controllers.
//
// Why: A single registration point reduces boilerplate wiring in controller
// manager and keeps controller lifecycle consistent across the package.
package quota

import (
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/client-go/dynamic"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"go.miloapis.com/milo/internal/informer"
	"go.miloapis.com/milo/internal/validation/quota"
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
			Client:                 mgr.GetClient(),
			Scheme:                 mgr.GetScheme(),
			ClaimTemplateValidator: &quota.ClaimTemplateValidator{},
		}
		return controller.SetupWithManager(mgr)
	}

	// GrantCreationPolicy validation controller
	r.controllers["GrantCreationPolicyReconciler"] = func(mgr ctrl.Manager, dynamicClient dynamic.Interface) error {
		// Create validation engines
		celValidator, err := quota.NewCELValidator()
		if err != nil {
			return fmt.Errorf("failed to create CEL validator: %w", err)
		}

		// Create controller first so we can use it as ResourceTypeValidator
		controller := &GrantCreationPolicyReconciler{
			Client:       mgr.GetClient(),
			Scheme:       mgr.GetScheme(),
			CELValidator: celValidator,
		}

		// Create template validator with controller as ResourceTypeValidator
		templateValidator := quota.NewGrantTemplateValidator(controller)
		controller.TemplateValidator = templateValidator

		return controller.SetupWithManager(mgr)
	}

	// Grant Creation controller
	r.controllers["GrantCreationController"] = func(mgr ctrl.Manager, dynamicClient dynamic.Interface) error {
		// Create validation engines
		celValidator, err := quota.NewCELValidator()
		if err != nil {
			return fmt.Errorf("failed to create CEL validator: %w", err)
		}

		// Create policy engine
		policyEngine := NewPolicyEngine(mgr.GetClient())

		// Create template engine
		templateEngine := NewTemplateEngine(celValidator)

		// Create parent context resolver
		parentContextResolver := NewParentContextResolver(mgr.GetClient())

		// Set up event recorder for policy engine
		eventRecorder := mgr.GetEventRecorderFor("grant-creation-policy")
		policyEngine.SetEventRecorder(func(obj client.Object, eventType, reason, message string) {
			eventRecorder.Event(obj, eventType, reason, message)
		})

		// Add PolicyEngine as a runnable - it will start after cache sync
		if err := mgr.Add(policyEngine); err != nil {
			return fmt.Errorf("failed to add policy engine to controller manager: %w", err)
		}

		// Create dynamic informer manager
		informerManager, err := informer.NewManagerFromManager(mgr)
		if err != nil {
			return fmt.Errorf("failed to create informer manager: %w", err)
		}

		// Add informer manager as a runnable
		if err := mgr.Add(informerManager); err != nil {
			return fmt.Errorf("failed to add informer manager to controller manager: %w", err)
		}

		controller := NewGrantCreationController(
			mgr.GetClient(),
			mgr.GetScheme(),
			policyEngine,
			templateEngine,
			parentContextResolver,
			mgr.GetEventRecorderFor("grant-creation"),
			informerManager,
		)

		// TODO: Start background cleanup for parent context resolver when manager starts
		// Background cleanup will be handled by the controller when it starts reconciling

		return controller.SetupWithManager(mgr)
	}

	// ResourceClaim Ownership controller (fast ownership + orphan cleanup)
	r.controllers["ResourceClaimOwnershipController"] = func(mgr ctrl.Manager, dynamicClient dynamic.Interface) error {
		controller := &ResourceClaimOwnershipController{
			Client:        mgr.GetClient(),
			DynamicClient: dynamicClient,
			Scheme:        mgr.GetScheme(),
		}
		return controller.SetupWithManager(mgr)
	}

	// DeniedAutoClaimCleanup controller (automatically deletes denied auto-created ResourceClaims)
	r.controllers["DeniedAutoClaimCleanupController"] = func(mgr ctrl.Manager, dynamicClient dynamic.Interface) error {
		controller := NewDeniedAutoClaimCleanupController(
			mgr.GetClient(),
			mgr.GetScheme(),
		)
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
