package quota

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/admission/initializer"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/warning"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"

	quotavalidation "go.miloapis.com/milo/internal/validation/quota"
	quotav1alpha1 "go.miloapis.com/milo/pkg/apis/quota/v1alpha1"
)

const (
	// PluginName is the name of the admission plugin.
	PluginName = "ClaimCreationQuota"

	// ClaimWaitTimeout is the maximum time to wait for a ResourceClaim to be granted
	ClaimWaitTimeout = 30 * time.Second
)

// ClaimCreationPlugin implements admission.Interface for automatic ResourceClaim creation.
type ClaimCreationPlugin struct {
	*admission.Handler
	dynamicClient  dynamic.Interface
	policyEngine   PolicyEngine
	templateEngine TemplateEngine
	watchManager   ClaimWatchManager
	config         *AdmissionPluginConfig
	logger         logr.Logger
}

// Ensure ClaimCreationPlugin implements the required initializer interfaces
var _ initializer.WantsDynamicClient = &ClaimCreationPlugin{}
var _ admission.ValidationInterface = &ClaimCreationPlugin{}
var _ admission.InitializationValidator = &ClaimCreationPlugin{}

// NewClaimCreationPlugin creates a new ClaimCreationPlugin.
func NewClaimCreationPlugin() (*ClaimCreationPlugin, error) {
	logger := klog.NewKlogr().WithName("claim-creation-plugin")
	klog.V(1).InfoS("Creating ClaimCreationQuota admission plugin instance")

	// Create the admission plugin - engines will be initialized when dependencies are injected
	plugin := &ClaimCreationPlugin{
		Handler: admission.NewHandler(admission.Create),
		config:  DefaultAdmissionPluginConfig(),
		logger:  logger,
	}

	return plugin, nil
}

// SetDynamicClient implements initializer.WantsDynamicClient
func (p *ClaimCreationPlugin) SetDynamicClient(dynamicClient dynamic.Interface) {
	p.dynamicClient = dynamicClient
	p.logger.V(2).Info("Dynamic client set", "plugin", PluginName)

	// Initialize engines and watch manager now that we have the dynamic client
	if dynamicClient != nil {
		p.initializeEngines()
		p.initializeWatchManager()
	}
}

// ValidateInitialization implements admission.InitializationValidator
func (p *ClaimCreationPlugin) ValidateInitialization() error {
	if p.dynamicClient == nil {
		return fmt.Errorf("dynamic client not initialized")
	}
	if p.policyEngine == nil {
		return fmt.Errorf("policy engine not initialized")
	}
	if p.templateEngine == nil {
		return fmt.Errorf("template engine not initialized")
	}
	if p.watchManager == nil && !p.config.DisableSharedWatch {
		return fmt.Errorf("watch manager not initialized")
	}
	return nil
}

// Validate implements admission.ValidationInterface - we use this for ResourceClaim creation
func (p *ClaimCreationPlugin) Validate(ctx context.Context, attrs admission.Attributes, o admission.ObjectInterfaces) error {
	return p.handleAdmission(ctx, attrs, o)
}

// initializeEngines initializes the policy and template engines
func (p *ClaimCreationPlugin) initializeEngines() {
	p.logger.V(1).Info("Initializing engines for admission plugin")

	// Create CEL engine
	celEngine, err := NewCELEngine(p.logger.WithName("cel"))
	if err != nil {
		p.logger.Error(err, "Failed to create CEL engine")
		return
	}

	// Create template engine
	p.templateEngine = NewTemplateEngine(celEngine, p.logger.WithName("template"))

	// Create policy engine for admission plugin use
	p.policyEngine, err = NewPolicyEngine(p.dynamicClient, p.logger)
	if err != nil {
		p.logger.Error(err, "Failed to create policy engine")
		return
	}

	// Don't load policies during initialization to avoid circular dependency
	// Policies will be loaded lazily when first needed
	p.logger.V(1).Info("Policy engine will load policies lazily to avoid circular dependency")

	p.logger.V(1).Info("Engines initialized successfully")
}

// initializeWatchManager initializes the shared watch manager
func (p *ClaimCreationPlugin) initializeWatchManager() {
	if p.config.DisableSharedWatch {
		p.logger.Info("Shared watch manager disabled by configuration")
		return
	}

	p.logger.V(1).Info("Initializing shared watch manager for admission plugin")

	// Create shared watch manager
	p.watchManager = NewClaimWatchManager(p.dynamicClient, p.logger.WithName("watch-manager"))

	// Start the watch manager in the background
	// Note: We use context.Background() here because the watch manager needs to outlive individual requests
	go func() {
		if err := p.watchManager.Start(context.Background()); err != nil {
			p.logger.Error(err, "Failed to start shared watch manager")
		}
	}()

	p.logger.V(1).Info("Shared watch manager initialized successfully")
}

// handleAdmission is the main admission logic - called for each API request.
func (p *ClaimCreationPlugin) handleAdmission(ctx context.Context, attrs admission.Attributes, o admission.ObjectInterfaces) error {
	p.logger.V(2).Info("ClaimCreationQuota admission plugin triggered",
		"operation", attrs.GetOperation(),
		"name", attrs.GetName(),
		"namespace", attrs.GetNamespace(),
		"kind", attrs.GetKind(),
		"user", attrs.GetUserInfo().GetName(),
		"dryRun", attrs.IsDryRun())

	// Check if this is a direct ResourceClaim creation
	if attrs.GetKind().Group == "quota.miloapis.com" && attrs.GetKind().Kind == "ResourceClaim" {
		return p.validateResourceClaim(ctx, attrs)
	}

	// Only handle CREATE and UPDATE operations for other resources
	if attrs.GetOperation() != admission.Create {
		p.logger.V(3).Info("Skipping non-CREATE operation", "operation", attrs.GetOperation())
		return nil
	}

	// Skip dry run requests to avoid creating ResourceClaims during validation
	if attrs.IsDryRun() {
		p.logger.V(2).Info("Skipping ResourceClaim creation for dry run request",
			"name", attrs.GetName(),
			"namespace", attrs.GetNamespace(),
			"gvk", attrs.GetKind())
		return nil
	}

	// Get the GVK from admission attributes
	gvk := schema.GroupVersionKind{
		Group:   attrs.GetKind().Group,
		Version: attrs.GetKind().Version,
		Kind:    attrs.GetKind().Kind,
	}

	p.logger.Info("Looking up policy for GVK", "gvk", gvk)

	// Get the policy for this GVK (O(1) lookup)
	policy, err := p.policyEngine.GetPolicyForGVK(gvk)
	if err != nil {
		p.logger.Error(err, "Failed to get policy for GVK", "gvk", gvk)
		warning.AddWarning(ctx, "", fmt.Sprintf("Failed to get ClaimCreationPolicy for %v: %v", gvk, err))
		return err
	}

	p.logger.Info("Policy lookup completed", "gvk", gvk, "policyFound", policy != nil)

	if policy == nil {
		// No policy for this resource type - allow without ResourceClaim creation
		p.logger.V(3).Info("No policy found for GVK, skipping ResourceClaim creation", "gvk", gvk)
		return nil
	}

	// Check if policy is enabled
	if policy.Spec.Enabled == nil || !*policy.Spec.Enabled {
		p.logger.V(2).Info("Policy is disabled, skipping ResourceClaim creation",
			"policy", policy.Name,
			"gvk", gvk)
		return nil
	}

	p.logger.V(1).Info("Found policy for resource type",
		"policy", policy.Name,
		"gvk", gvk,
		"resourceName", attrs.GetName())

	// Convert the object to unstructured for easier access
	obj, err := p.convertToUnstructured(attrs.GetObject())
	if err != nil {
		p.logger.Error(err, "Failed to convert object to unstructured")
		warning.AddWarning(ctx, "", fmt.Sprintf("Failed to process object for ResourceClaim creation: %v", err))
		return nil // Don't block resource creation
	}

	// Build evaluation context
	evalContext := p.buildEvaluationContext(attrs, obj)

	p.logger.V(1).Info("Creating ResourceClaim based on policy",
		"policy", policy.Name,
		"resourceName", attrs.GetName())

	// Create the ResourceClaim and wait for it to be granted
	if err := p.createAndWaitForResourceClaim(ctx, policy, evalContext); err != nil {
		// ResourceClaim creation or granting failed - block the resource creation
		p.logger.Error(err, "ResourceClaim not granted, denying resource creation",
			"policy", policy.Name,
			"resourceName", attrs.GetName(),
			"gvk", gvk)

		// Return proper admission error with 422 status code
		return errors.NewInvalid(
			schema.GroupKind{Group: gvk.Group, Kind: gvk.Kind},
			attrs.GetName(),
			field.ErrorList{
				field.Invalid(field.NewPath("spec"), nil, fmt.Sprintf("Resource quota exceeded: %v", err)),
			})
	}

	p.logger.V(1).Info("ResourceClaim granted, allowing resource creation",
		"policy", policy.Name,
		"resourceName", attrs.GetName())

	return nil // Allow original resource creation only if claim is granted
}

// createAndWaitForResourceClaim creates a ResourceClaim and waits for it to be granted.
func (p *ClaimCreationPlugin) createAndWaitForResourceClaim(ctx context.Context, policy *quotav1alpha1.ClaimCreationPolicy, evalContext *EvaluationContext) error {
	claimName, namespace, err := p.createResourceClaim(ctx, policy, evalContext)
	if err != nil {
		return fmt.Errorf("failed to create ResourceClaim: %w", err)
	}

	// Wait for the ResourceClaim to be granted
	return p.waitForClaimGranted(ctx, claimName, namespace)
}

// createResourceClaim creates a ResourceClaim based on the policy and context.
// Returns the claim name and namespace for watching.
func (p *ClaimCreationPlugin) createResourceClaim(ctx context.Context, policy *quotav1alpha1.ClaimCreationPolicy, evalContext *EvaluationContext) (string, string, error) {
	// Note: Resource type validation is done by the controller which sets the Ready status
	// We only receive policies with Ready=True, so no validation needed here

	// Render the ResourceClaim from the policy template
	spec, err := p.templateEngine.RenderResourceClaim(ctx, policy.Spec.ResourceClaimTemplate, evalContext, p.policyEngine)
	if err != nil {
		return "", "", fmt.Errorf("failed to render ResourceClaim spec: %w", err)
	}

	// Generate ResourceClaim name prefix for GenerateName
	claimNamePrefix := p.generateResourceClaimNamePrefix(evalContext)

	// Determine namespace
	namespace := quotav1alpha1.MiloSystemNamespace
	if policy.Spec.ResourceClaimTemplate.Namespace != "" {
		namespace = policy.Spec.ResourceClaimTemplate.Namespace
	}

	gvr := schema.GroupVersionResource{
		Group:    "quota.miloapis.com",
		Version:  "v1alpha1",
		Resource: "resourceclaims",
	}

	// Prepare labels and annotations
	labels := map[string]string{
		"quota.miloapis.com/auto-created": "true",
		"quota.miloapis.com/policy":       policy.Name,
		"quota.miloapis.com/gvk":          fmt.Sprintf("%s.%s.%s", evalContext.GVK.Group, evalContext.GVK.Version, evalContext.GVK.Kind),
	}

	// Add template labels
	for key, value := range policy.Spec.ResourceClaimTemplate.Labels {
		labels[key] = value
	}

	annotations := map[string]string{
		"quota.miloapis.com/created-by":    "claim-creation-plugin",
		"quota.miloapis.com/created-at":    time.Now().Format(time.RFC3339),
		"quota.miloapis.com/resource-name": evalContext.Object.GetName(),
		"quota.miloapis.com/policy":        policy.Name,
	}

	// Add template annotations
	for key, value := range policy.Spec.ResourceClaimTemplate.Annotations {
		annotations[key] = value
	}

	// Note: UID removed from ConsumerRef for name/kind-only matching

	// Populate the ResourceRef with the unversioned reference to the resource being created
	spec.ResourceRef = quotav1alpha1.UnversionedObjectReference{
		APIGroup:  evalContext.GVK.Group,
		Kind:      evalContext.GVK.Kind,
		Name:      evalContext.Object.GetName(),
		Namespace: evalContext.Object.GetNamespace(), // Will be empty for cluster-scoped resources
	}

	// Create the ResourceClaim with GenerateName for automatic unique naming
	claim := &quotav1alpha1.ResourceClaim{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: claimNamePrefix,
			Namespace:    namespace,
			Labels:       labels,
			Annotations:  annotations,
		},
		Spec: *spec,
	}

	// Debug: Log the claim before conversion
	p.logger.V(1).Info("Creating ResourceClaim",
		"claimName", claimNamePrefix,
		"consumerRef", claim.Spec.ConsumerRef,
		"consumerRefKind", claim.Spec.ConsumerRef.Kind,
		"consumerRefName", claim.Spec.ConsumerRef.Name,
		"resourceRef", claim.Spec.ResourceRef)

	// Convert ResourceClaim to unstructured using JSON marshaling to preserve types
	claimBytes, err := json.Marshal(claim)
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal ResourceClaim: %w", err)
	}

	var unstructuredClaim map[string]interface{}
	if err := json.Unmarshal(claimBytes, &unstructuredClaim); err != nil {
		return "", "", fmt.Errorf("failed to unmarshal ResourceClaim to unstructured: %w", err)
	}

	// Debug: Log the consumerRef
	p.logger.V(1).Info("Creating ResourceClaim with consumerRef",
		"claimName", claimNamePrefix,
		"consumerRefKind", spec.ConsumerRef.Kind,
		"consumerRefName", spec.ConsumerRef.Name)

	unstructuredObj := &unstructured.Unstructured{Object: unstructuredClaim}
	unstructuredObj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "quota.miloapis.com",
		Version: "v1alpha1",
		Kind:    "ResourceClaim",
	})

	// Create the ResourceClaim using dynamic client
	createdClaim, err := p.dynamicClient.Resource(gvr).Namespace(namespace).Create(ctx, unstructuredObj, metav1.CreateOptions{})
	if err != nil {
		return "", "", fmt.Errorf("failed to create ResourceClaim: %w", err)
	}

	// Get the generated name from the created ResourceClaim
	claimName := createdClaim.GetName()

	p.logger.Info("ResourceClaim created successfully",
		"claimName", claimName,
		"namespace", namespace,
		"policy", policy.Name,
		"resourceName", evalContext.Object.GetName(),
		"requestCount", len(spec.Requests))

	return claimName, namespace, nil
}

// generateResourceClaimNamePrefix creates a name prefix for ResourceClaim GenerateName.
func (p *ClaimCreationPlugin) generateResourceClaimNamePrefix(evalContext *EvaluationContext) string {
	// Generate a descriptive prefix for GenerateName
	// Kubernetes will append a random suffix to ensure uniqueness
	resourceName := evalContext.Object.GetName()
	kind := evalContext.GVK.Kind

	// Format: {resourceName}-{kind}-claim-
	// The trailing dash is important for GenerateName
	return fmt.Sprintf("%s-%s-claim-", resourceName, strings.ToLower(kind))
}

// convertToUnstructured converts a runtime.Object to *unstructured.Unstructured.
func (p *ClaimCreationPlugin) convertToUnstructured(obj runtime.Object) (*unstructured.Unstructured, error) {
	if obj == nil {
		return nil, fmt.Errorf("object is nil")
	}

	// If already unstructured, return as-is
	if u, ok := obj.(*unstructured.Unstructured); ok {
		return u, nil
	}

	// Convert to unstructured
	unstructuredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, fmt.Errorf("failed to convert to unstructured: %w", err)
	}

	return &unstructured.Unstructured{Object: unstructuredMap}, nil
}

// buildEvaluationContext creates an EvaluationContext from admission attributes.
func (p *ClaimCreationPlugin) buildEvaluationContext(attrs admission.Attributes, obj *unstructured.Unstructured) *EvaluationContext {
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
		Resource:          strings.ToLower(attrs.GetKind().Kind) + "s", // Pluralize kind for resource
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

// waitForClaimGranted watches a ResourceClaim and waits for it to be granted or denied.
func (p *ClaimCreationPlugin) waitForClaimGranted(ctx context.Context, claimName, namespace string) error {
	// Use configured timeout
	timeout := p.config.WatchManager.DefaultTimeout

	if p.config.DisableSharedWatch || p.watchManager == nil {
		// Fallback to individual watch (old behavior)
		p.logger.V(1).Info("Using fallback individual watch for ResourceClaim status",
			"name", claimName, "namespace", namespace, "reason", "shared watch disabled")
		return p.waitForClaimGrantedIndividual(ctx, claimName, namespace, timeout)
	}

	p.logger.V(1).Info("Registering with shared watch manager for ResourceClaim status",
		"name", claimName, "namespace", namespace, "timeout", timeout)

	// Register with shared watch manager
	resultChan, cancelFunc := p.watchManager.RegisterClaimWaiter(claimName, namespace, timeout)
	defer cancelFunc()

	// Wait for result from shared watch manager
	select {
	case result, ok := <-resultChan:
		if !ok {
			// Channel was closed, likely due to cancellation
			return fmt.Errorf("watch was cancelled")
		}

		if result.Error != nil {
			return result.Error
		}

		if result.Granted {
			p.logger.Info("ResourceClaim granted via shared watch manager",
				"name", claimName, "namespace", namespace)
			return nil
		} else {
			p.logger.Info("ResourceClaim denied via shared watch manager",
				"name", claimName, "namespace", namespace, "reason", result.Reason)
			return fmt.Errorf("ResourceClaim was denied: %s", result.Reason)
		}

	case <-ctx.Done():
		// Request context was cancelled
		p.watchManager.UnregisterClaimWaiter(claimName, namespace)
		return ctx.Err()
	}
}

// waitForClaimGrantedIndividual is the fallback individual watch method (original implementation)
func (p *ClaimCreationPlugin) waitForClaimGrantedIndividual(ctx context.Context, claimName, namespace string, timeout time.Duration) error {
	gvr := schema.GroupVersionResource{
		Group:    "quota.miloapis.com",
		Version:  "v1alpha1",
		Resource: "resourceclaims",
	}

	// Create a timeout context
	watchCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	p.logger.V(1).Info("Starting individual watch for ResourceClaim status", "name", claimName, "namespace", namespace)

	// Start watching for changes
	watcher, err := p.dynamicClient.Resource(gvr).Namespace(namespace).Watch(watchCtx, metav1.ListOptions{
		FieldSelector:        fmt.Sprintf("metadata.name=%s", claimName),
		Watch:                true,
		SendInitialEvents:    ptr.To(true),
		ResourceVersionMatch: "NotOlderThan",
	})
	if err != nil {
		return fmt.Errorf("failed to start individual watch: %w", err)
	}
	defer watcher.Stop()

	for {
		select {
		case <-watchCtx.Done():
			return fmt.Errorf("timeout waiting for ResourceClaim to be granted after %v", timeout)

		case event, ok := <-watcher.ResultChan():
			if !ok {
				return fmt.Errorf("watch channel closed unexpectedly")
			}

			switch event.Type {
			case watch.Added, watch.Modified:
				claimObj, ok := event.Object.(*unstructured.Unstructured)
				if !ok {
					p.logger.Error(fmt.Errorf("unexpected object type"), "Expected unstructured object")
					continue
				}

				p.logger.V(2).Info("ResourceClaim status updated", "name", claimName, "namespace", namespace)

				// Check if granted (using utility functions)
				if isClaimGranted(claimObj) {
					p.logger.Info("ResourceClaim granted", "name", claimName, "namespace", namespace)
					return nil
				}

				// Check if denied
				if isClaimDenied(claimObj) {
					reason := getClaimDenialReason(claimObj)
					p.logger.Info("ResourceClaim denied, rejecting resource creation",
						"name", claimName, "reason", reason)
					return fmt.Errorf("ResourceClaim was denied: %s", reason)
				}

			case watch.Deleted:
				return fmt.Errorf("ResourceClaim was deleted while waiting for grant")

			case watch.Error:
				return fmt.Errorf("watch error: %v", event.Object)
			}
		}
	}
}

// isClaimGranted checks if a ResourceClaim has been granted (utility function)
func isClaimGranted(claim *unstructured.Unstructured) bool {
	conditions, found, err := unstructured.NestedSlice(claim.Object, "status", "conditions")
	if err != nil || !found {
		return false
	}

	for _, conditionInterface := range conditions {
		condition, ok := conditionInterface.(map[string]interface{})
		if !ok {
			continue
		}

		conditionType, _, _ := unstructured.NestedString(condition, "type")
		conditionStatus, _, _ := unstructured.NestedString(condition, "status")

		if conditionType == quotav1alpha1.ResourceClaimGranted && conditionStatus == string(metav1.ConditionTrue) {
			return true
		}
	}
	return false
}

// isClaimDenied checks if a ResourceClaim has been denied (utility function)
func isClaimDenied(claim *unstructured.Unstructured) bool {
	conditions, found, err := unstructured.NestedSlice(claim.Object, "status", "conditions")
	if err != nil || !found {
		return false
	}

	for _, conditionInterface := range conditions {
		condition, ok := conditionInterface.(map[string]interface{})
		if !ok {
			continue
		}

		conditionType, _, _ := unstructured.NestedString(condition, "type")
		conditionStatus, _, _ := unstructured.NestedString(condition, "status")
		conditionReason, _, _ := unstructured.NestedString(condition, "reason")

		if conditionType == quotav1alpha1.ResourceClaimGranted &&
			conditionStatus == string(metav1.ConditionFalse) &&
			conditionReason == quotav1alpha1.ResourceClaimDeniedReason {
			return true
		}
	}
	return false
}

// getClaimDenialReason returns the reason why a ResourceClaim was denied (utility function)
func getClaimDenialReason(claim *unstructured.Unstructured) string {
	conditions, found, err := unstructured.NestedSlice(claim.Object, "status", "conditions")
	if err != nil || !found {
		return "unknown reason"
	}

	for _, conditionInterface := range conditions {
		condition, ok := conditionInterface.(map[string]interface{})
		if !ok {
			continue
		}

		conditionType, _, _ := unstructured.NestedString(condition, "type")
		conditionStatus, _, _ := unstructured.NestedString(condition, "status")
		conditionMessage, _, _ := unstructured.NestedString(condition, "message")

		if conditionType == quotav1alpha1.ResourceClaimGranted && conditionStatus == string(metav1.ConditionFalse) {
			if conditionMessage != "" {
				return conditionMessage
			}
			return "quota exceeded"
		}
	}
	return "unknown reason"
}

// validateResourceClaim validates ResourceClaim objects when they are created directly
func (p *ClaimCreationPlugin) validateResourceClaim(ctx context.Context, attrs admission.Attributes) error {
	// Only validate CREATE operations for ResourceClaims
	if attrs.GetOperation() != admission.Create {
		p.logger.V(3).Info("Skipping non-CREATE operation for ResourceClaim", "operation", attrs.GetOperation())
		return nil
	}

	// Get the ResourceClaim object
	obj := attrs.GetObject()
	if obj == nil {
		return nil
	}

	claim, ok := obj.(*quotav1alpha1.ResourceClaim)
	if !ok {
		// Try to convert from unstructured
		unstructuredObj, ok := obj.(*unstructured.Unstructured)
		if !ok {
			p.logger.V(2).Info("Could not convert object to ResourceClaim", "type", fmt.Sprintf("%T", obj))
			return nil
		}

		// Convert unstructured to ResourceClaim
		claimBytes, err := unstructuredObj.MarshalJSON()
		if err != nil {
			return fmt.Errorf("failed to marshal unstructured object: %w", err)
		}

		claim = &quotav1alpha1.ResourceClaim{}
		if err := json.Unmarshal(claimBytes, claim); err != nil {
			return fmt.Errorf("failed to unmarshal to ResourceClaim: %w", err)
		}
	}

	p.logger.V(1).Info("Validating ResourceClaim",
		"name", claim.Name,
		"namespace", claim.Namespace,
		"requestCount", len(claim.Spec.Requests))

	// Validate the resource claim using field-based validation
	if errs := p.validateResourceClaimFields(claim); len(errs) > 0 {
		p.logger.Info("ResourceClaim validation failed",
			"name", claim.Name,
			"namespace", claim.Namespace,
			"errors", errs)
		return admission.NewForbidden(attrs, errors.NewInvalid(
			quotav1alpha1.GroupVersion.WithKind("ResourceClaim").GroupKind(),
			claim.Name,
			errs))
	}

	p.logger.V(1).Info("ResourceClaim validation passed",
		"name", claim.Name,
		"namespace", claim.Namespace)

	return nil
}

// validateResourceClaimFields performs comprehensive field-based validation of ResourceClaim
func (p *ClaimCreationPlugin) validateResourceClaimFields(claim *quotav1alpha1.ResourceClaim) field.ErrorList {
	var errs field.ErrorList
	requestsPath := field.NewPath("spec", "requests")
	resourceRefPath := field.NewPath("spec", "resourceRef")

	// Track resource types to detect duplicates
	seenResourceTypes := make(map[string]int)

	// Validate each request
	for i, request := range claim.Spec.Requests {
		requestPath := requestsPath.Index(i)

		// Validate resource type is not empty
		if request.ResourceType == "" {
			errs = append(errs, field.Required(requestPath.Child("resourceType"),
				"resource type is required"))
			continue
		}

		// Check for duplicate resource types
		if firstIndex, exists := seenResourceTypes[request.ResourceType]; exists {
			errs = append(errs, field.Duplicate(requestPath.Child("resourceType"),
				fmt.Sprintf("resource type '%s' is already specified in request %d",
					request.ResourceType, firstIndex)))
		} else {
			seenResourceTypes[request.ResourceType] = i
		}

		// Validate amount is positive
		if request.Amount <= 0 {
			errs = append(errs, field.Invalid(requestPath.Child("amount"), request.Amount,
				"amount must be greater than 0"))
		}
	}

	// Validate that the claiming resource (ResourceRef) is allowed to claim these resources
	// This requires checking against ResourceRegistrations' ClaimingResources field
	ctx := context.Background()
	if err := p.validateClaimingResource(ctx, claim); err != nil {
		errs = append(errs, field.Invalid(resourceRefPath,
			fmt.Sprintf("%s/%s", claim.Spec.ResourceRef.APIGroup, claim.Spec.ResourceRef.Kind),
			err.Error()))
	}

	return errs
}

// validateClaimingResource validates that the ResourceRef is allowed to claim the requested resources
func (p *ClaimCreationPlugin) validateClaimingResource(ctx context.Context, claim *quotav1alpha1.ResourceClaim) error {
	// Use the shared validation logic from the validation package
	return quotavalidation.ValidateResourceClaimAgainstRegistrations(ctx, p.dynamicClient, claim)
}
