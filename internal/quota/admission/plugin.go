package admission

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/admission/initializer"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/warning"
	"k8s.io/client-go/dynamic"
	"k8s.io/component-base/metrics"
	legacyregistry "k8s.io/component-base/metrics/legacyregistry"
	"k8s.io/klog/v2"

	"go.miloapis.com/milo/internal/quota/engine"
	"go.miloapis.com/milo/internal/quota/validation"
	quotav1alpha1 "go.miloapis.com/milo/pkg/apis/quota/v1alpha1"
)

const (
	// PluginName is the name of the admission plugin.
	PluginName = "ResourceQuotaEnforcement"

	// ClaimWaitTimeout is the maximum time to wait for a ResourceClaim to be granted
	ClaimWaitTimeout = 30 * time.Second
)

// Metrics for quota admission decisions. Registered once at init.
var (
	admissionResultTotal = metrics.NewCounterVec(
		&metrics.CounterOpts{
			Subsystem:      "milo_quota",
			Name:           "admission_result_total",
			Help:           "Total quota admission decisions by outcome, policy, policy namespace, and resource type.",
			StabilityLevel: metrics.ALPHA,
		},
		[]string{"result", "policy_name", "policy_namespace", "resource_group", "resource_kind"},
	)
)

func init() {
	// Register metrics with Kubernetes legacy registry so they are exposed on the apiserver /metrics.
	legacyregistry.MustRegister(admissionResultTotal)
}

// ResourceQuotaEnforcementPlugin implements admission.Interface for resource quota enforcement
// via automatic ResourceClaim creation and quota validation.
type ResourceQuotaEnforcementPlugin struct {
	*admission.Handler
	dynamicClient                 dynamic.Interface
	policyEngine                  engine.PolicyEngine
	templateEngine                engine.TemplateEngine
	resourceClaimValidator        validation.ResourceClaimValidator
	resourceRegistrationValidator *validation.ResourceRegistrationValidator
	claimCreationPolicyValidator  *validation.ClaimCreationPolicyValidator
	grantCreationPolicyValidator  *validation.GrantCreationPolicyValidator
	resourceGrantValidator        *validation.ResourceGrantValidator

	// The plugin uses the resource type validator to prevent the apiserver from
	// being marked ready until the resource type's cache has been synced.
	resourceTypeValidator validation.ResourceTypeValidator

	watchManager ClaimWatchManager
	config       *AdmissionPluginConfig

	logger logr.Logger
}

// Ensure ResourceQuotaEnforcementPlugin implements the required initializer interfaces
var _ initializer.WantsDynamicClient = &ResourceQuotaEnforcementPlugin{}
var _ admission.ValidationInterface = &ResourceQuotaEnforcementPlugin{}
var _ admission.InitializationValidator = &ResourceQuotaEnforcementPlugin{}

// NewResourceQuotaEnforcementPlugin creates a new ResourceQuotaEnforcementPlugin.
func NewResourceQuotaEnforcementPlugin() (*ResourceQuotaEnforcementPlugin, error) {
	logger := klog.NewKlogr().WithName("resource-quota-enforcement-plugin")
	klog.V(1).InfoS("Creating ResourceQuotaEnforcement admission plugin instance")

	// Create the admission plugin - tracer will be initialized when TracerProvider is injected
	plugin := &ResourceQuotaEnforcementPlugin{
		Handler: admission.NewHandler(admission.Create),
		config:  DefaultAdmissionPluginConfig(),
		logger:  logger,
	}

	return plugin, nil
}

// SetDynamicClient implements initializer.WantsDynamicClient
func (p *ResourceQuotaEnforcementPlugin) SetDynamicClient(dynamicClient dynamic.Interface) {
	p.dynamicClient = dynamicClient
	p.logger.V(2).Info("Dynamic client set", "plugin", PluginName)

	// Initialize engines and watch manager now that we have the dynamic client
	if dynamicClient != nil && p.policyEngine == nil {
		p.initializeEngines()
		p.initializeWatchManager()
	}
}

// ValidateInitialization implements admission.InitializationValidator
func (p *ResourceQuotaEnforcementPlugin) ValidateInitialization() error {
	if p.dynamicClient == nil {
		return fmt.Errorf("dynamic client not initialized")
	}
	if p.policyEngine == nil {
		return fmt.Errorf("policy engine not initialized")
	}
	if p.templateEngine == nil {
		return fmt.Errorf("template engine not initialized")
	}
	if p.resourceClaimValidator == nil {
		return fmt.Errorf("resource claim validator not initialized")
	}
	if p.resourceRegistrationValidator == nil {
		return fmt.Errorf("resource registration validator not initialized")
	}
	if p.claimCreationPolicyValidator == nil {
		return fmt.Errorf("claim creation policy validator not initialized")
	}
	if p.grantCreationPolicyValidator == nil {
		return fmt.Errorf("grant creation policy validator not initialized")
	}
	if p.resourceGrantValidator == nil {
		return fmt.Errorf("resource grant validator not initialized")
	}
	if p.watchManager == nil {
		return fmt.Errorf("watch manager not initialized")
	}
	return nil
}

// initializeEngines initializes the policy and template engines
func (p *ResourceQuotaEnforcementPlugin) initializeEngines() {
	p.logger.V(2).Info("Initializing engines for admission plugin")

	celEngine, err := engine.NewCELEngine()
	if err != nil {
		p.logger.Error(err, "Failed to create CEL engine")
		return
	}

	p.templateEngine = engine.NewTemplateEngine(celEngine, p.logger.WithName("template"))
	p.policyEngine = engine.NewPolicyEngine(p.dynamicClient, p.logger)
	p.resourceTypeValidator = validation.NewResourceTypeValidator(p.dynamicClient)
	p.resourceClaimValidator = validation.NewResourceClaimValidator(p.dynamicClient, p.resourceTypeValidator)
	p.resourceRegistrationValidator = validation.NewResourceRegistrationValidator(p.resourceTypeValidator)

	// Initialize policy validators for admission-time validation
	celValidator, err := validation.NewCELValidator()
	if err != nil {
		p.logger.Error(err, "Failed to create CEL validator")
		return
	}

	grantTemplateValidator, err := validation.NewGrantTemplateValidator(p.resourceTypeValidator)
	if err != nil {
		p.logger.Error(err, "Failed to create grant template validator")
		return
	}

	p.claimCreationPolicyValidator = validation.NewClaimCreationPolicyValidator(p.resourceTypeValidator)
	p.grantCreationPolicyValidator = validation.NewGrantCreationPolicyValidator(celValidator, grantTemplateValidator)
	p.resourceGrantValidator = validation.NewResourceGrantValidator(p.resourceTypeValidator)

	go func() {
		if err := p.policyEngine.Start(context.Background()); err != nil {
			p.logger.Error(err, "Failed to start policy engine")
		}
	}()

	p.logger.V(2).Info("Engines initialized successfully")
}

// initializeWatchManager initializes the shared informer-based watch manager
func (p *ResourceQuotaEnforcementPlugin) initializeWatchManager() {
	// Create shared informer-based watch manager with automatic reconnection
	p.watchManager = NewClaimWatchManager(p.dynamicClient, p.logger.WithName("watch-manager"))

	// Start the watch manager in the background
	// Note: We use context.Background() here because the watch manager needs to outlive individual requests
	go func() {
		if err := p.watchManager.Start(context.Background()); err != nil {
			p.logger.Error(err, "Failed to start shared informer watch manager")
		} else {
			p.logger.V(2).Info("Shared informer watch manager started successfully")
		}
	}()
}

// Validate implements admission.ValidationInterface and orchestrates the main admission flow
func (p *ResourceQuotaEnforcementPlugin) Validate(ctx context.Context, attrs admission.Attributes, _ admission.ObjectInterfaces) error {
	p.logger.V(3).Info("ResourceQuotaEnforcement admission plugin triggered",
		"operation", attrs.GetOperation(),
		"resource.group", attrs.GetKind().Group,
		"resource.version", attrs.GetKind().Version,
		"resource.kind", attrs.GetKind().Kind,
		"name", attrs.GetName(),
		"namespace", attrs.GetNamespace(),
		"user", attrs.GetUserInfo().GetName(),
		"dryRun", attrs.IsDryRun(),
	)

	// Route to appropriate handler based on resource type
	if attrs.GetKind().Group == "quota.miloapis.com" {
		switch attrs.GetKind().Kind {
		case "ResourceClaim":
			return p.validateResourceClaim(ctx, attrs)
		case "ResourceRegistration":
			return p.validateResourceRegistration(ctx, attrs)
		case "ClaimCreationPolicy":
			return p.validateClaimCreationPolicy(ctx, attrs)
		case "GrantCreationPolicy":
			return p.validateGrantCreationPolicy(ctx, attrs)
		case "ResourceGrant":
			return p.validateResourceGrant(ctx, attrs)
		}
	}

	// Only handle CREATE operations for other resources
	if attrs.GetOperation() != admission.Create {
		p.logger.V(4).Info("Skipping non-CREATE operation", "operation", attrs.GetOperation())
		return nil
	}

	// Skip dry run requests to avoid creating ResourceClaims during validation
	if attrs.IsDryRun() {
		return nil
	}

	return p.handleResourceQuotaEnforcement(ctx, attrs)
}

// handleResourceQuotaEnforcement enforces resource quotas by creating and validating ResourceClaims
func (p *ResourceQuotaEnforcementPlugin) handleResourceQuotaEnforcement(ctx context.Context, attrs admission.Attributes) error {
	ctx, span := p.startSpan(ctx, "quota.admission.ResourceQuotaEnforcement",
		trace.WithAttributes(
			attribute.String("operation", string(attrs.GetOperation())),
			attribute.String("resource.name", attrs.GetName()),
			attribute.String("resource.namespace", attrs.GetNamespace()),
			attribute.String("resource.group", attrs.GetKind().Group),
			attribute.String("resource.version", attrs.GetKind().Version),
			attribute.String("resource.kind", attrs.GetKind().Kind),
			attribute.String("user.name", attrs.GetUserInfo().GetName()),
			attribute.Bool("dry_run", attrs.IsDryRun()),
		))
	defer span.End()

	// Get the GVK from admission attributes
	gvk := schema.GroupVersionKind{
		Group:   attrs.GetKind().Group,
		Version: attrs.GetKind().Version,
		Kind:    attrs.GetKind().Kind,
	}

	// Look up policy for this resource type
	policy, err := p.lookupPolicyForResource(ctx, gvk)
	if err != nil {
		p.logger.Error(err, "Failed to get policy for GVK", "gvk", gvk)
		warning.AddWarning(ctx, "", fmt.Sprintf("Failed to get ClaimCreationPolicy for %v: %v", gvk, err))
		return err
	}

	if policy == nil {
		// No policy for this resource type - allow without ResourceClaim creation
		p.logger.V(3).Info("No policy found for GVK, skipping ResourceClaim creation", "gvk", gvk)
		return nil
	}

	// Check if policy is disabled
	if policy.Spec.Disabled != nil && *policy.Spec.Disabled {
		// Record policy disabled decision with full context
		admissionResultTotal.WithLabelValues("policy_disabled", policy.Name, policy.Namespace,
			gvk.Group, gvk.Kind).Inc()

		p.logger.V(3).Info("Policy is disabled, skipping ResourceClaim creation",
			"policy", policy.Name,
			"gvk", gvk)
		return nil
	}

	// Process the resource with the policy
	return p.processResourceWithPolicy(ctx, attrs, policy, gvk)
}

// lookupPolicyForResource retrieves the policy for a given GVK with tracing
func (p *ResourceQuotaEnforcementPlugin) lookupPolicyForResource(ctx context.Context, gvk schema.GroupVersionKind) (*quotav1alpha1.ClaimCreationPolicy, error) {

	// Get the policy for this GVK with tracing
	_, policySpan := p.startSpan(ctx, "quota.admission.ResourceQuotaEnforcement.policyLookup",
		trace.WithAttributes(
			attribute.String("gvk.group", gvk.Group),
			attribute.String("gvk.version", gvk.Version),
			attribute.String("gvk.kind", gvk.Kind),
		))
	defer policySpan.End()

	policy, err := p.policyEngine.GetPolicyForGVK(gvk)
	if err != nil {
		policySpan.RecordError(err)
		policySpan.SetStatus(codes.Error, fmt.Sprintf("Failed to get policy for GVK: %v", err))
		return nil, err
	}

	policySpan.SetAttributes(
		attribute.Bool("policy.found", policy != nil),
	)
	if policy != nil {
		policySpan.SetAttributes(
			attribute.String("policy.name", policy.Name),
			attribute.Bool("policy.disabled", policy.Spec.Disabled != nil && *policy.Spec.Disabled),
		)
	}

	return policy, nil
}

// processResourceWithPolicy handles resource creation when a policy is found and enabled
func (p *ResourceQuotaEnforcementPlugin) processResourceWithPolicy(ctx context.Context, attrs admission.Attributes, policy *quotav1alpha1.ClaimCreationPolicy, gvk schema.GroupVersionKind) error {
	// Convert the resource being created to unstructured for CEL evaluation.
	// The CEL engine requires map[string]interface{} (unstructured.Object) to evaluate
	// trigger conditions against arbitrary resource types.
	unstructuredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(attrs.GetObject())
	if err != nil {
		p.logger.Error(err, "Failed to convert object to unstructured")
		warning.AddWarning(ctx, "", fmt.Sprintf("Failed to process object for ResourceClaim creation: %v", err))
		return nil // Don't block resource creation
	}
	obj := &unstructured.Unstructured{Object: unstructuredMap}

	// Build evaluation context
	evalContext := p.buildEvaluationContext(attrs, obj)

	// Evaluate trigger constraints to determine if this resource should trigger the policy
	constraintsMet, err := p.templateEngine.EvaluateConditions(policy.Spec.Trigger.Constraints, obj)
	if err != nil {
		p.logger.Error(err, "Failed to evaluate policy constraints",
			"policy", policy.Name,
			"resourceName", attrs.GetName())
		warning.AddWarning(ctx, "", fmt.Sprintf("Failed to evaluate policy constraints: %v", err))
		return nil // Don't block resource creation on constraint evaluation errors
	}

	if !constraintsMet {
		// Policy constraints not met - skip ResourceClaim creation
		p.logger.V(3).Info("Policy constraints not met, skipping ResourceClaim creation",
			"policy", policy.Name,
			"resourceName", attrs.GetName(),
			"gvk", gvk)
		return nil
	}

	p.logger.V(2).Info("Policy constraints met, creating ResourceClaim based on policy",
		"policy", policy.Name,
		"resourceName", attrs.GetName())

	// Create the ResourceClaim and wait for it to be granted
	if err := p.createAndWaitForResourceClaim(ctx, policy, evalContext); err != nil {
		// ResourceClaim creation or granting failed - block the resource creation

		// Record denied admission decision with full context
		admissionResultTotal.WithLabelValues("denied", policy.Name, policy.Namespace,
			evalContext.GVK.Group, evalContext.GVK.Kind).Inc()

		p.logger.Error(err, "ResourceClaim not granted, denying resource creation",
			"policy", policy.Name,
			"resourceName", attrs.GetName(),
			"gvk", gvk)

		// Return quota exceeded error using Forbidden (403) - consistent with K8s core
		// The error message clearly indicates it's a quota issue, not an auth failure
		gr := schema.GroupResource{Group: gvk.Group, Resource: attrs.GetResource().Resource}

		//lint:ignore ST1005 "Error message intentionally capitalized for user-facing display"
		return errors.NewForbidden(gr, attrs.GetName(), fmt.Errorf("Insufficient quota resources available. Review your quota usage and reach out to support if you need additional resources."))
	}

	// Record granted admission decision with full context
	admissionResultTotal.WithLabelValues("granted", policy.Name, policy.Namespace,
		evalContext.GVK.Group, evalContext.GVK.Kind).Inc()

	p.logger.V(2).Info("ResourceClaim granted, allowing resource creation",
		"policy", policy.Name,
		"resourceName", attrs.GetName())

	return nil // Allow original resource creation only if claim is granted
}

// createAndWaitForResourceClaim creates a ResourceClaim and waits for it to be granted.
func (p *ResourceQuotaEnforcementPlugin) createAndWaitForResourceClaim(ctx context.Context, policy *quotav1alpha1.ClaimCreationPolicy, evalContext *EvaluationContext) error {
	ctx, span := p.startSpan(ctx, "quota.admission.ResourceQuotaEnforcement.createAndWaitForResourceClaim",
		trace.WithAttributes(
			attribute.String("policy.name", policy.Name),
			attribute.String("resource.name", evalContext.Object.GetName()),
			attribute.String("resource.namespace", evalContext.Object.GetNamespace()),
		))
	defer span.End()

	claimName, namespace, err := p.createResourceClaim(ctx, policy, evalContext)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to create ResourceClaim")
		return fmt.Errorf("failed to create ResourceClaim: %w", err)
	}

	span.SetAttributes(
		attribute.String("claim.name", claimName),
		attribute.String("claim.namespace", namespace),
	)
	p.logger.V(2).Info("Creating waiter for resource claim",
		"claimName", claimName,
		"namespace", namespace,
	)

	err = p.waitForClaimGranted(ctx, claimName, namespace)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "ResourceClaim was not granted")
		return err
	}

	span.SetAttributes(attribute.String("claim.status", "granted"))
	return nil
}

// createResourceClaim creates a ResourceClaim based on the policy and context.
// Returns the claim name and namespace for watching.
func (p *ResourceQuotaEnforcementPlugin) createResourceClaim(ctx context.Context, policy *quotav1alpha1.ClaimCreationPolicy, evalContext *EvaluationContext) (string, string, error) {
	ctx, span := p.startSpan(ctx, "quota.admission.ResourceQuotaEnforcement.createResourceClaim",
		trace.WithAttributes(
			attribute.String("policy.name", policy.Name),
			attribute.String("resource.name", evalContext.Object.GetName()),
			attribute.String("resource.namespace", evalContext.Object.GetNamespace()),
		))
	defer span.End()

	// Render the complete ResourceClaim from the policy template
	// Convert admission EvaluationContext to engine EvaluationContext
	engineContext := p.convertToEngineContext(evalContext)
	claim, err := p.templateEngine.RenderClaim(policy, engineContext)
	if err != nil {
		return "", "", fmt.Errorf("failed to render ResourceClaim: %w", err)
	}

	// Populate the ResourceRef with the unversioned reference to the resource being created
	claim.Spec.ResourceRef = quotav1alpha1.UnversionedObjectReference{
		APIGroup:  evalContext.GVK.Group,
		Kind:      evalContext.GVK.Kind,
		Name:      evalContext.Object.GetName(),
		Namespace: evalContext.Object.GetNamespace(), // Will be empty for cluster-scoped resources
	}

	// Default GenerateName if neither name nor generateName provided
	if strings.TrimSpace(claim.Name) == "" && strings.TrimSpace(claim.GenerateName) == "" {
		claim.GenerateName = p.generateResourceClaimNamePrefix(evalContext)
	}

	// Add admission-specific labels and annotations
	if claim.Labels == nil {
		claim.Labels = make(map[string]string)
	}
	if claim.Annotations == nil {
		claim.Annotations = make(map[string]string)
	}

	// Add standard admission labels
	claim.Labels["quota.miloapis.com/auto-created"] = "true"
	claim.Labels["quota.miloapis.com/policy"] = policy.Name
	claim.Labels["quota.miloapis.com/gvk"] = fmt.Sprintf("%s.%s.%s", evalContext.GVK.Group, evalContext.GVK.Version, evalContext.GVK.Kind)

	// Add standard admission annotations
	claim.Annotations["quota.miloapis.com/created-by"] = "claim-creation-plugin"
	claim.Annotations["quota.miloapis.com/created-at"] = time.Now().Format(time.RFC3339)
	claim.Annotations["quota.miloapis.com/resource-name"] = evalContext.Object.GetName()
	claim.Annotations["quota.miloapis.com/policy"] = policy.Name

	gvr := schema.GroupVersionResource{
		Group:    "quota.miloapis.com",
		Version:  "v1alpha1",
		Resource: "resourceclaims",
	}

	unstructuredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(claim)
	if err != nil {
		return "", "", fmt.Errorf("failed to convert ResourceClaim to unstructured: %w", err)
	}
	unstructuredObj := &unstructured.Unstructured{Object: unstructuredMap}

	// Create the ResourceClaim using dynamic client
	createdClaim, err := p.dynamicClient.Resource(gvr).Namespace(claim.Namespace).Create(ctx, unstructuredObj, metav1.CreateOptions{})
	if err != nil {
		return "", "", fmt.Errorf("failed to create ResourceClaim: %w", err)
	}

	// Get the generated name from the created ResourceClaim
	claimName := createdClaim.GetName()

	p.logger.V(2).Info("ResourceClaim created successfully",
		"claimName", claimName,
		"namespace", claim.Namespace,
		"policy", policy.Name,
		"resourceName", evalContext.Object.GetName(),
	)

	return claimName, claim.Namespace, nil
}

// waitForClaimGranted watches a ResourceClaim and waits for it to be granted or denied.
func (p *ResourceQuotaEnforcementPlugin) waitForClaimGranted(ctx context.Context, claimName, namespace string) error {
	ctx, span := p.startSpan(ctx, "quota.admission.ResourceQuotaEnforcement.waitForClaimGranted",
		trace.WithAttributes(
			attribute.String("claim.name", claimName),
			attribute.String("claim.namespace", namespace),
		))
	defer span.End()

	// Use configured timeout
	timeout := p.config.WatchManager.DefaultTimeout
	span.SetAttributes(attribute.String("watch.timeout", timeout.String()))

	p.logger.V(2).Info("Registering with shared watch manager for ResourceClaim status",
		"name", claimName, "namespace", namespace, "timeout", timeout)

	// Wait for the claim to be granted or denied
	resultChan, cancelFunc, err := p.watchManager.RegisterClaimWaiter(ctx, claimName, namespace, timeout)
	if err != nil {
		return fmt.Errorf("failed to wait for claim: %w", err)
	}
	defer cancelFunc()

	select {
	case result, ok := <-resultChan:
		if !ok {
			// Channel was closed, likely due to cancellation
			span.SetStatus(codes.Error, "Watch was cancelled")
			return fmt.Errorf("watch was cancelled")
		}

		if result.Error != nil {
			span.RecordError(result.Error)
			span.SetStatus(codes.Error, "Watch error")
			return result.Error
		}

		if result.Granted {
			span.SetAttributes(
				attribute.String("claim.result", "granted"),
				attribute.String("watch.method", "shared"),
			)
			p.logger.V(2).Info("ResourceClaim granted via shared watch manager",
				"name", claimName, "namespace", namespace)
			return nil
		} else {
			span.SetAttributes(
				attribute.String("claim.result", "denied"),
				attribute.String("claim.denial_reason", result.Reason),
				attribute.String("watch.method", "shared"),
			)
			p.logger.Info("ResourceClaim denied via shared watch manager",
				"name", claimName, "namespace", namespace, "reason", result.Reason)
			return fmt.Errorf("ResourceClaim was denied: %s", result.Reason)
		}

	case <-ctx.Done():
		// Request context was cancelled
		span.SetStatus(codes.Error, "Context cancelled")
		p.watchManager.UnregisterClaimWaiter(claimName, namespace)
		return ctx.Err()
	}
}

// validateResourceClaim validates ResourceClaim objects when they are created directly
func (p *ResourceQuotaEnforcementPlugin) validateResourceClaim(ctx context.Context, attrs admission.Attributes) error {
	ctx, span := p.startSpan(ctx, "quota.admission.ResourceClaimValidation",
		trace.WithAttributes(
			attribute.String("operation", string(attrs.GetOperation())),
			attribute.String("claim.name", attrs.GetName()),
			attribute.String("claim.namespace", attrs.GetNamespace()),
			attribute.String("user.name", attrs.GetUserInfo().GetName()),
			attribute.Bool("dry_run", attrs.IsDryRun()),
		))
	defer span.End()

	// Only validate CREATE operations for ResourceClaims
	if attrs.GetOperation() != admission.Create {
		span.SetAttributes(attribute.String("validation.status", "skipped"))
		p.logger.V(4).Info("Skipping non-CREATE operation for ResourceClaim", "operation", attrs.GetOperation())
		return nil
	}

	// Get the ResourceClaim object
	obj := attrs.GetObject()
	if obj == nil {
		return nil
	}

	claim, ok := obj.(*quotav1alpha1.ResourceClaim)
	if !ok {
		return fmt.Errorf("expected ResourceClaim, got %T", obj)
	}

	span.SetAttributes(
		attribute.String("claim.name", claim.Name),
		attribute.String("claim.namespace", claim.Namespace),
		attribute.Int("claim.request_count", len(claim.Spec.Requests)),
	)

	// Validate the resource claim using field-based validation
	// Validate the ResourceClaim using the complete validator
	if errs := p.resourceClaimValidator.Validate(ctx, claim); len(errs) > 0 {
		span.SetAttributes(
			attribute.String("validation.status", "failed"),
			attribute.Int("validation.error_count", len(errs)),
		)
		span.SetStatus(codes.Error, "ResourceClaim validation failed")
		p.logger.Info("ResourceClaim validation failed",
			"name", claim.Name,
			"namespace", claim.Namespace,
			"errors", errs)
		return admission.NewForbidden(attrs, errors.NewInvalid(
			quotav1alpha1.GroupVersion.WithKind("ResourceClaim").GroupKind(),
			claim.Name,
			errs))
	}

	span.SetAttributes(attribute.String("validation.status", "passed"))

	return nil
}

// startSpan safely starts a span using the tracer provider from the context
func (p *ResourceQuotaEnforcementPlugin) startSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	// Get the tracer provider from the existing span context
	tracerProvider := trace.SpanFromContext(ctx).TracerProvider()
	tracer := tracerProvider.Tracer("go.miloapis.com/milo/admission/quota")
	return tracer.Start(ctx, name, opts...)
}

// buildEvaluationContext creates an EvaluationContext from admission attributes.
func (p *ResourceQuotaEnforcementPlugin) buildEvaluationContext(attrs admission.Attributes, obj *unstructured.Unstructured) *EvaluationContext {
	user := UserContext{
		Name:   attrs.GetUserInfo().GetName(),
		UID:    attrs.GetUserInfo().GetUID(),
		Groups: attrs.GetUserInfo().GetGroups(),
		Extra:  attrs.GetUserInfo().GetExtra(),
	}

	// Build RequestInfo from admission attributes
	// Map admission operation to HTTP verb equivalents
	verb := strings.ToLower(string(attrs.GetOperation()))

	requestInfo := &request.RequestInfo{
		IsResourceRequest: true,
		Verb:              verb,
		APIGroup:          attrs.GetKind().Group,
		APIVersion:        attrs.GetKind().Version,
		Namespace:         attrs.GetNamespace(),
		Resource:          attrs.GetResource().Resource,
		Subresource:       attrs.GetSubresource(),
		Name:              attrs.GetName(),
	}

	return &EvaluationContext{
		Object:      obj,
		User:        user,
		RequestInfo: requestInfo,
		Namespace:   attrs.GetNamespace(),
		GVK: schema.GroupVersionKind{
			Group:   attrs.GetKind().Group,
			Version: attrs.GetKind().Version,
			Kind:    attrs.GetKind().Kind,
		},
	}
}

// convertToEngineContext converts admission EvaluationContext to engine EvaluationContext
func (p *ResourceQuotaEnforcementPlugin) convertToEngineContext(admissionCtx *EvaluationContext) *engine.EvaluationContext {
	return &engine.EvaluationContext{
		Object: admissionCtx.Object,
		User: engine.UserContext{
			Name:   admissionCtx.User.Name,
			UID:    admissionCtx.User.UID,
			Groups: admissionCtx.User.Groups,
			Extra:  admissionCtx.User.Extra,
		},
		RequestInfo: admissionCtx.RequestInfo,
		Namespace:   admissionCtx.Namespace,
		GVK: struct {
			Group   string
			Version string
			Kind    string
		}{
			Group:   admissionCtx.GVK.Group,
			Version: admissionCtx.GVK.Version,
			Kind:    admissionCtx.GVK.Kind,
		},
	}
}

// generateResourceClaimNamePrefix creates a name prefix for ResourceClaim GenerateName.
func (p *ResourceQuotaEnforcementPlugin) generateResourceClaimNamePrefix(evalContext *EvaluationContext) string {
	// Generate a descriptive prefix for GenerateName
	// Kubernetes will append a random suffix to ensure uniqueness
	resourceName := evalContext.Object.GetName()
	kind := evalContext.GVK.Kind

	// Format: {resourceName}-{kind}-claim-
	// The trailing dash is important for GenerateName
	return fmt.Sprintf("%s-%s-claim-", resourceName, strings.ToLower(kind))
}

// validateResourceRegistration validates ResourceRegistration objects for cross-resource duplicates.
func (p *ResourceQuotaEnforcementPlugin) validateResourceRegistration(ctx context.Context, attrs admission.Attributes) error {
	ctx, span := p.startSpan(ctx, "quota.admission.ResourceRegistrationValidation",
		trace.WithAttributes(
			attribute.String("operation", string(attrs.GetOperation())),
			attribute.String("registration.name", attrs.GetName()),
			attribute.String("user.name", attrs.GetUserInfo().GetName()),
		))
	defer span.End()

	// Only validate on CREATE to check for duplicate resourceType
	// Updates are handled by CEL immutability rules
	if attrs.GetOperation() != admission.Create {
		span.SetAttributes(attribute.String("validation.status", "skipped"))
		return nil
	}

	obj := attrs.GetObject()
	if obj == nil {
		return nil
	}

	// Convert to ResourceRegistration
	registration, ok := obj.(*quotav1alpha1.ResourceRegistration)
	if !ok {
		return fmt.Errorf("expected ResourceRegistration, got %T", obj)
	}

	span.SetAttributes(
		attribute.String("registration.name", registration.Name),
		attribute.String("registration.resourceType", registration.Spec.ResourceType),
	)

	// Validate the ResourceRegistration
	if validationErrs := p.resourceRegistrationValidator.Validate(registration); len(validationErrs) > 0 {
		span.SetAttributes(attribute.String("validation.status", "failed"))
		span.SetStatus(codes.Error, "ResourceRegistration validation failed")

		return admission.NewForbidden(attrs, errors.NewInvalid(
			quotav1alpha1.GroupVersion.WithKind("ResourceRegistration").GroupKind(),
			registration.Name,
			validationErrs,
		))
	}

	span.SetAttributes(attribute.String("validation.status", "passed"))
	return nil
}

// validateClaimCreationPolicy validates ClaimCreationPolicy objects for template syntax and resource types.
func (p *ResourceQuotaEnforcementPlugin) validateClaimCreationPolicy(ctx context.Context, attrs admission.Attributes) error {
	ctx, span := p.startSpan(ctx, "quota.admission.ClaimCreationPolicyValidation",
		trace.WithAttributes(
			attribute.String("operation", string(attrs.GetOperation())),
			attribute.String("policy.name", attrs.GetName()),
			attribute.String("policy.namespace", attrs.GetNamespace()),
			attribute.String("user.name", attrs.GetUserInfo().GetName()),
		))
	defer span.End()

	// Only validate on CREATE - updates can be handled by CEL immutability rules
	if attrs.GetOperation() != admission.Create {
		span.SetAttributes(attribute.String("validation.status", "skipped"))
		return nil
	}

	obj := attrs.GetObject()
	if obj == nil {
		return nil
	}

	// Convert to ClaimCreationPolicy
	policy, ok := obj.(*quotav1alpha1.ClaimCreationPolicy)
	if !ok {
		return fmt.Errorf("expected ClaimCreationPolicy, got %T", obj)
	}

	span.SetAttributes(
		attribute.String("policy.name", policy.Name),
		attribute.String("policy.namespace", policy.Namespace),
	)

	// Validate the ClaimCreationPolicy
	if validationErrs := p.claimCreationPolicyValidator.Validate(ctx, policy); len(validationErrs) > 0 {
		span.SetAttributes(attribute.String("validation.status", "failed"))
		span.SetStatus(codes.Error, "ClaimCreationPolicy validation failed")

		p.logger.Info("ClaimCreationPolicy validation failed",
			"name", policy.Name,
			"namespace", policy.Namespace,
			"errors", validationErrs)

		return admission.NewForbidden(attrs, errors.NewInvalid(
			quotav1alpha1.GroupVersion.WithKind("ClaimCreationPolicy").GroupKind(),
			policy.Name,
			validationErrs,
		))
	}

	span.SetAttributes(attribute.String("validation.status", "passed"))
	return nil
}

// validateGrantCreationPolicy validates GrantCreationPolicy objects for CEL expressions and template syntax.
func (p *ResourceQuotaEnforcementPlugin) validateGrantCreationPolicy(ctx context.Context, attrs admission.Attributes) error {
	ctx, span := p.startSpan(ctx, "quota.admission.GrantCreationPolicyValidation",
		trace.WithAttributes(
			attribute.String("operation", string(attrs.GetOperation())),
			attribute.String("policy.name", attrs.GetName()),
			attribute.String("policy.namespace", attrs.GetNamespace()),
			attribute.String("user.name", attrs.GetUserInfo().GetName()),
		))
	defer span.End()

	// Only validate on CREATE - updates can be handled by CEL immutability rules
	if attrs.GetOperation() != admission.Create {
		span.SetAttributes(attribute.String("validation.status", "skipped"))
		return nil
	}

	obj := attrs.GetObject()
	if obj == nil {
		return nil
	}

	// Convert to GrantCreationPolicy
	policy, ok := obj.(*quotav1alpha1.GrantCreationPolicy)
	if !ok {
		return fmt.Errorf("expected GrantCreationPolicy, got %T", obj)
	}

	span.SetAttributes(
		attribute.String("policy.name", policy.Name),
		attribute.String("policy.namespace", policy.Namespace),
	)

	// Validate the GrantCreationPolicy
	if validationErrs := p.grantCreationPolicyValidator.Validate(ctx, policy); len(validationErrs) > 0 {
		span.SetAttributes(attribute.String("validation.status", "failed"))
		span.SetStatus(codes.Error, "GrantCreationPolicy validation failed")

		p.logger.Info("GrantCreationPolicy validation failed",
			"name", policy.Name,
			"namespace", policy.Namespace,
			"errors", validationErrs)

		return admission.NewForbidden(attrs, errors.NewInvalid(
			quotav1alpha1.GroupVersion.WithKind("GrantCreationPolicy").GroupKind(),
			policy.Name,
			validationErrs,
		))
	}

	span.SetAttributes(attribute.String("validation.status", "passed"))
	return nil
}

// validateResourceGrant validates ResourceGrant objects for resource type validity.
func (p *ResourceQuotaEnforcementPlugin) validateResourceGrant(ctx context.Context, attrs admission.Attributes) error {
	ctx, span := p.startSpan(ctx, "quota.admission.ResourceGrantValidation",
		trace.WithAttributes(
			attribute.String("operation", string(attrs.GetOperation())),
			attribute.String("grant.name", attrs.GetName()),
			attribute.String("grant.namespace", attrs.GetNamespace()),
			attribute.String("user.name", attrs.GetUserInfo().GetName()),
		))
	defer span.End()

	// Only validate on CREATE - updates can be handled by CEL immutability rules
	if attrs.GetOperation() != admission.Create {
		span.SetAttributes(attribute.String("validation.status", "skipped"))
		return nil
	}

	obj := attrs.GetObject()
	if obj == nil {
		return nil
	}

	// Convert to ResourceGrant
	grant, ok := obj.(*quotav1alpha1.ResourceGrant)
	if !ok {
		return fmt.Errorf("expected ResourceGrant, got %T", obj)
	}

	span.SetAttributes(
		attribute.String("grant.name", grant.Name),
		attribute.String("grant.namespace", grant.Namespace),
		attribute.Int("grant.allowances_count", len(grant.Spec.Allowances)),
	)

	// Validate the ResourceGrant
	if validationErrs := p.resourceGrantValidator.Validate(ctx, grant); len(validationErrs) > 0 {
		span.SetAttributes(attribute.String("validation.status", "failed"))
		span.SetStatus(codes.Error, "ResourceGrant validation failed")

		p.logger.Info("ResourceGrant validation failed",
			"name", grant.Name,
			"namespace", grant.Namespace,
			"errors", validationErrs)

		return admission.NewForbidden(attrs, errors.NewInvalid(
			quotav1alpha1.GroupVersion.WithKind("ResourceGrant").GroupKind(),
			grant.Name,
			validationErrs,
		))
	}

	span.SetAttributes(attribute.String("validation.status", "passed"))
	return nil
}
