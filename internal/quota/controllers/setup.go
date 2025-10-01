// Package controllers provides a simplified setup function for all quota controllers.
//
// Why: A single setup function reduces boilerplate wiring in controller
// manager and keeps controller lifecycle consistent across the package.
package controllers

import (
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/client-go/dynamic"
	ctrl "sigs.k8s.io/controller-runtime"

	"go.miloapis.com/milo/internal/informer"
	"go.miloapis.com/milo/internal/quota/controllers/core"
	"go.miloapis.com/milo/internal/quota/controllers/lifecycle"
	"go.miloapis.com/milo/internal/quota/controllers/policy"
	"go.miloapis.com/milo/internal/quota/engine"
	"go.miloapis.com/milo/internal/quota/validation"
)

// SetupQuotaControllers registers all quota controllers with the provided manager in a single step.
// This replaces the previous two-stage registry approach with a simple, direct setup function.
func SetupQuotaControllers(mgr ctrl.Manager, dynamicClient dynamic.Interface, logger logr.Logger) error {
	logger.Info("Setting up quota controllers")

	// Create shared validation components once using async initialization
	// This prevents blocking controller startup if API server isn't fully ready
	sharedResourceTypeValidator := validation.NewResourceTypeValidator(dynamicClient)
	logger.Info("Shared ResourceTypeValidator created, will sync in background")

	// Create shared CEL engine (used by multiple controllers)
	celEngine, err := engine.NewCELEngine()
	if err != nil {
		return fmt.Errorf("failed to create CEL engine: %w", err)
	}

	// Setup controllers in logical order

	// 1. ResourceRegistration controller (foundational)
	logger.V(1).Info("Setting up ResourceRegistration controller")
	if err := (&core.ResourceRegistrationController{
		Client: mgr.GetClient(),
	}).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("failed to setup ResourceRegistrationController: %w", err)
	}

	// 2. ResourceGrant controller (depends on ResourceRegistrations)
	logger.V(1).Info("Setting up ResourceGrant controller")
	if err := (&core.ResourceGrantController{
		Client:                mgr.GetClient(),
		ResourceTypeValidator: sharedResourceTypeValidator,
	}).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("failed to setup ResourceGrantController: %w", err)
	}

	// 3. ResourceClaim controller
	logger.V(1).Info("Setting up ResourceClaim controller")
	if err := (&core.ResourceClaimController{
		Client: mgr.GetClient(),
	}).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("failed to setup ResourceClaimController: %w", err)
	}

	// 4. AllowanceBucket controller (aggregates quota data)
	logger.V(1).Info("Setting up AllowanceBucket controller")
	if err := (&core.AllowanceBucketController{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("failed to setup AllowanceBucketController: %w", err)
	}

	// 5. ClaimCreationPolicy controller (policy validation)
	logger.V(1).Info("Setting up ClaimCreationPolicy controller")
	if err := (&policy.ClaimCreationPolicyReconciler{
		Client:                 mgr.GetClient(),
		Scheme:                 mgr.GetScheme(),
		ClaimTemplateValidator: &validation.ClaimTemplateValidator{},
		ResourceTypeValidator:  sharedResourceTypeValidator,
	}).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("failed to setup ClaimCreationPolicyReconciler: %w", err)
	}

	// 6. GrantCreationPolicy controller (policy validation)
	logger.V(1).Info("Setting up GrantCreationPolicy controller")
	templateValidator := validation.NewGrantTemplateValidator(sharedResourceTypeValidator)
	if err := (&policy.GrantCreationPolicyReconciler{
		Client:            mgr.GetClient(),
		Scheme:            mgr.GetScheme(),
		CELValidator:      celEngine,
		TemplateValidator: templateValidator,
	}).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("failed to setup GrantCreationPolicyReconciler: %w", err)
	}

	// 7. Grant Creation controller (automatic grant creation)
	logger.V(1).Info("Setting up Grant Creation controller")
	templateEngine := engine.NewTemplateEngine(celEngine, logger)
	parentContextResolver := policy.NewParentContextResolver(mgr.GetClient(), mgr.GetConfig(), policy.ParentContextResolverOptions{})

	informerManager, err := informer.NewManagerFromManager(mgr)
	if err != nil {
		return fmt.Errorf("failed to create informer manager: %w", err)
	}

	if err := mgr.Add(informerManager); err != nil {
		return fmt.Errorf("failed to add informer manager to controller manager: %w", err)
	}

	grantCreationController := policy.NewGrantCreationController(
		mgr.GetClient(),
		mgr.GetScheme(),
		templateEngine,
		parentContextResolver,
		mgr.GetEventRecorderFor("grant-creation"),
		informerManager,
	)
	if err := grantCreationController.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("failed to setup GrantCreationController: %w", err)
	}

	// 8. ResourceClaim Ownership controller (lifecycle management)
	logger.V(1).Info("Setting up ResourceClaim Ownership controller")
	if err := (&lifecycle.ResourceClaimOwnershipController{
		Client:        mgr.GetClient(),
		DynamicClient: dynamicClient,
		Scheme:        mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("failed to setup ResourceClaimOwnershipController: %w", err)
	}

	// 9. DeniedAutoClaim Cleanup controller (lifecycle management)
	logger.V(1).Info("Setting up DeniedAutoClaim Cleanup controller")
	deniedCleanupController := lifecycle.NewDeniedAutoClaimCleanupController(
		mgr.GetClient(),
		mgr.GetScheme(),
	)
	if err := deniedCleanupController.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("failed to setup DeniedAutoClaimCleanupController: %w", err)
	}

	logger.Info("All quota controllers set up successfully")
	return nil
}
