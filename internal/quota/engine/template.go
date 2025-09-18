package engine

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"text/template"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apiserver/pkg/endpoints/request"

	quotav1alpha1 "go.miloapis.com/milo/pkg/apis/quota/v1alpha1"
)

// TemplateEngine handles ResourceClaim and ResourceGrant generation from policy templates.
type TemplateEngine interface {
	// RenderResourceClaim renders a ResourceClaimTemplate into a ResourceClaimSpec.
	RenderResourceClaim(ctx context.Context, template quotav1alpha1.ResourceClaimTemplate, evalContext *EvaluationContext) (*quotav1alpha1.ResourceClaimSpec, error)

	// RenderClaimMetadata renders name/generateName/namespace and annotations for claim metadata.
	RenderClaimMetadata(metadata quotav1alpha1.ObjectMetaTemplate, evalContext *EvaluationContext) (string, string, string, map[string]string, map[string]string, error)

	// RenderResourceGrant renders a ResourceGrantTemplate into a ResourceGrantSpec.
	RenderResourceGrant(ctx context.Context, template quotav1alpha1.ResourceGrantTemplate, evalContext *GrantEvaluationContext) (*quotav1alpha1.ResourceGrantSpec, error)

	// RenderGrantMetadata renders name/generateName/namespace and annotations for grant metadata.
	RenderGrantMetadata(metadata quotav1alpha1.ObjectMetaTemplate, evalContext *GrantEvaluationContext) (string, string, string, map[string]string, map[string]string, error)

	// RenderGrant renders a complete ResourceGrant from a GrantCreationPolicy.
	RenderGrant(policy *quotav1alpha1.GrantCreationPolicy, triggerObj *unstructured.Unstructured, targetNamespace string) (*quotav1alpha1.ResourceGrant, error)

	// EvaluateConditions evaluates trigger conditions against a resource object.
	EvaluateConditions(conditions []quotav1alpha1.ConditionExpression, obj *unstructured.Unstructured) (bool, error)

	// EvaluateParentContextName evaluates a parent context name expression.
	EvaluateParentContextName(expression string, obj *unstructured.Unstructured) (string, error)

	// GenerateGrantName generates a consistent grant name for a policy and trigger object.
	GenerateGrantName(policy *quotav1alpha1.GrantCreationPolicy, triggerObj *unstructured.Unstructured) (string, error)
}

// EvaluationContext provides context for template evaluation in admission scenarios.
type EvaluationContext struct {
	Object      *unstructured.Unstructured
	User        UserContext
	RequestInfo *request.RequestInfo
	Namespace   string
	GVK         struct {
		Group   string
		Version string
		Kind    string
	}
}

// GrantEvaluationContext provides context for template evaluation in grant creation scenarios.
type GrantEvaluationContext struct {
	Object         *unstructured.Unstructured
	ParentContext  map[string]interface{}
	ResourceType   string
	ConsumerRef    quotav1alpha1.ConsumerRef
	RequestContext map[string]interface{}
}

// UserContext provides user information for template evaluation.
type UserContext struct {
	Name   string
	UID    string
	Groups []string
	Extra  map[string][]string
}

// templateEngine implements TemplateEngine.
type templateEngine struct {
	logger        logr.Logger
	templateCache sync.Map // string -> *template.Template
	funcMap       template.FuncMap
	celEngine     CELEngine
}

// NewTemplateEngine creates a new template engine.
func NewTemplateEngine(celEngine CELEngine, logger logr.Logger) TemplateEngine {
	engine := &templateEngine{
		logger:    logger.WithName("template-engine"),
		celEngine: celEngine,
	}

	// Initialize template function map
	engine.funcMap = template.FuncMap{
		"lower":    strings.ToLower,
		"upper":    strings.ToUpper,
		"title":    strings.Title,
		"default":  engine.defaultFunc,
		"contains": engine.containsFunc,
		"join":     strings.Join,
		"split":    strings.Split,
		"replace":  strings.ReplaceAll,
		"trim":     strings.TrimSpace,
		"toInt":    engine.toIntFunc,
		"toString": engine.toStringFunc,
	}

	return engine
}

// RenderResourceClaim renders a ResourceClaimTemplate into a ResourceClaimSpec.
func (e *templateEngine) RenderResourceClaim(ctx context.Context, template quotav1alpha1.ResourceClaimTemplate, evalContext *EvaluationContext) (*quotav1alpha1.ResourceClaimSpec, error) {
	// Process resource requests using CEL expressions and field suffixes
	var resourceRequests []quotav1alpha1.ResourceRequest
	for _, requestTemplate := range template.Spec.Requests {
		// Render ResourceType using CEL if it's an expression, otherwise use literal value
		resourceType := requestTemplate.ResourceType
		if strings.HasPrefix(resourceType, "{{") && strings.HasSuffix(resourceType, "}}") {
			// This is a CEL expression
			celExpr := strings.TrimSpace(resourceType[2 : len(resourceType)-2])
			renderedType, err := e.celEngine.EvaluateNameExpression(celExpr, evalContext.Object)
			if err != nil {
				return nil, fmt.Errorf("failed to evaluate ResourceType CEL expression %q: %w", celExpr, err)
			}
			resourceType = renderedType
		}

		// Use the amount directly from the template
		amount := requestTemplate.Amount

		resourceRequests = append(resourceRequests, quotav1alpha1.ResourceRequest{
			ResourceType: resourceType,
			Amount:       amount,
		})
	}

	// Render ConsumerRef using CEL
	consumerRef := template.Spec.ConsumerRef
	if template.Spec.ConsumerRef.Name != "" {
		if strings.HasPrefix(template.Spec.ConsumerRef.Name, "{{") && strings.HasSuffix(template.Spec.ConsumerRef.Name, "}}") {
			// This is a CEL expression
			celExpr := strings.TrimSpace(template.Spec.ConsumerRef.Name[2 : len(template.Spec.ConsumerRef.Name)-2])
			renderedName, err := e.celEngine.EvaluateNameExpression(celExpr, evalContext.Object)
			if err != nil {
				return nil, fmt.Errorf("failed to evaluate ConsumerRef.Name CEL expression %q: %w", celExpr, err)
			}
			consumerRef.Name = renderedName
		}
	}

	return &quotav1alpha1.ResourceClaimSpec{
		Requests:    resourceRequests,
		ConsumerRef: consumerRef,
	}, nil
}

// RenderClaimMetadata renders name/generateName/namespace and annotations for claim metadata.
func (e *templateEngine) RenderClaimMetadata(metadata quotav1alpha1.ObjectMetaTemplate, evalContext *EvaluationContext) (string, string, string, map[string]string, map[string]string, error) {
	contextMap := e.buildAdmissionContext(evalContext)

	// Render name
	name := ""
	if metadata.Name != "" {
		rendered, err := e.renderString(metadata.Name, contextMap)
		if err != nil {
			return "", "", "", nil, nil, fmt.Errorf("failed to render name: %w", err)
		}
		name = rendered
	}

	// Render generateName
	generateName := ""
	if metadata.GenerateName != "" {
		rendered, err := e.renderString(metadata.GenerateName, contextMap)
		if err != nil {
			return "", "", "", nil, nil, fmt.Errorf("failed to render generateName: %w", err)
		}
		generateName = rendered
	}

	// Render namespace (default to object namespace if not specified)
	namespace := evalContext.Namespace
	if metadata.Namespace != "" {
		rendered, err := e.renderString(metadata.Namespace, contextMap)
		if err != nil {
			return "", "", "", nil, nil, fmt.Errorf("failed to render namespace: %w", err)
		}
		namespace = rendered
	}

	// Render labels (literal values, not templates)
	labels := make(map[string]string)
	for key, value := range metadata.Labels {
		labels[key] = value
	}

	// Render annotations (support templates)
	annotations := make(map[string]string)
	for key, value := range metadata.Annotations {
		rendered, err := e.renderString(value, contextMap)
		if err != nil {
			return "", "", "", nil, nil, fmt.Errorf("failed to render annotation %q: %w", key, err)
		}
		annotations[key] = rendered
	}

	return name, generateName, namespace, labels, annotations, nil
}

// RenderResourceGrant renders a ResourceGrantTemplate into a ResourceGrantSpec.
func (e *templateEngine) RenderResourceGrant(ctx context.Context, template quotav1alpha1.ResourceGrantTemplate, evalContext *GrantEvaluationContext) (*quotav1alpha1.ResourceGrantSpec, error) {
	// Build template context for Go templates
	contextMap := e.buildGrantContext(evalContext)

	// Process ConsumerRef
	consumerRef := template.Spec.ConsumerRef
	if consumerRef.Name != "" {
		// Check if it's a Go template expression
		if strings.Contains(consumerRef.Name, "{{") && strings.Contains(consumerRef.Name, "}}") {
			renderedName, err := e.renderString(consumerRef.Name, contextMap)
			if err != nil {
				return nil, fmt.Errorf("failed to render ConsumerRef.Name: %w", err)
			}
			consumerRef.Name = renderedName
		}
	}

	// Process Allowances (currently just copy them as-is, but could add templating in future)
	allowances := make([]quotav1alpha1.Allowance, len(template.Spec.Allowances))
	copy(allowances, template.Spec.Allowances)

	return &quotav1alpha1.ResourceGrantSpec{
		ConsumerRef: consumerRef,
		Allowances:  allowances,
	}, nil
}

// RenderGrantMetadata renders name/generateName/namespace and annotations for grant metadata.
func (e *templateEngine) RenderGrantMetadata(metadata quotav1alpha1.ObjectMetaTemplate, evalContext *GrantEvaluationContext) (string, string, string, map[string]string, map[string]string, error) {
	contextMap := e.buildGrantContext(evalContext)

	// Render name
	name := ""
	if metadata.Name != "" {
		rendered, err := e.renderString(metadata.Name, contextMap)
		if err != nil {
			return "", "", "", nil, nil, fmt.Errorf("failed to render name: %w", err)
		}
		name = rendered
	}

	// Render generateName
	generateName := ""
	if metadata.GenerateName != "" {
		rendered, err := e.renderString(metadata.GenerateName, contextMap)
		if err != nil {
			return "", "", "", nil, nil, fmt.Errorf("failed to render generateName: %w", err)
		}
		generateName = rendered
	}

	// Render namespace
	namespace := ""
	if metadata.Namespace != "" {
		rendered, err := e.renderString(metadata.Namespace, contextMap)
		if err != nil {
			return "", "", "", nil, nil, fmt.Errorf("failed to render namespace: %w", err)
		}
		namespace = rendered
	}

	// Render labels (literal values, not templates)
	labels := make(map[string]string)
	for key, value := range metadata.Labels {
		labels[key] = value
	}

	// Render annotations (support templates)
	annotations := make(map[string]string)
	for key, value := range metadata.Annotations {
		rendered, err := e.renderString(value, contextMap)
		if err != nil {
			return "", "", "", nil, nil, fmt.Errorf("failed to render annotation %q: %w", key, err)
		}
		annotations[key] = rendered
	}

	return name, generateName, namespace, labels, annotations, nil
}

// buildAdmissionContext creates a template context map for Go template rendering from EvaluationContext.
func (e *templateEngine) buildAdmissionContext(evalContext *EvaluationContext) map[string]interface{} {
	// Build GVK string
	gvk := fmt.Sprintf("%s.%s.%s", evalContext.GVK.Group, evalContext.GVK.Version, evalContext.GVK.Kind)

	contextMap := map[string]interface{}{
		"gvk": gvk,
	}

	// Include trigger object (the resource being admitted)
	if evalContext.Object != nil {
		contextMap["trigger"] = evalContext.Object.Object
	}

	// Only include userInfo if it contains meaningful data
	if evalContext.User.Name != "" || evalContext.User.UID != "" || len(evalContext.User.Groups) > 0 || len(evalContext.User.Extra) > 0 {
		contextMap["userInfo"] = map[string]interface{}{
			"username": evalContext.User.Name,
			"uid":      evalContext.User.UID,
			"groups":   evalContext.User.Groups,
			"extra":    evalContext.User.Extra,
		}
	}

	// Only include requestInfo if it's not nil
	if evalContext.RequestInfo != nil {
		contextMap["requestInfo"] = buildRequestInfoMap(evalContext.RequestInfo)
	}

	return contextMap
}

// buildGrantContext creates a template context map for Go template rendering from GrantEvaluationContext.
func (e *templateEngine) buildGrantContext(evalContext *GrantEvaluationContext) map[string]interface{} {
	// Debug log the trigger object structure
	triggerObj := evalContext.Object.Object

	return map[string]interface{}{
		"gvk":     fmt.Sprintf("%s.%s", evalContext.Object.GetAPIVersion(), evalContext.Object.GetKind()),
		"trigger": triggerObj,
	}
}

// renderString renders a template string with the given context map.
func (e *templateEngine) renderString(templateStr string, contextMap map[string]interface{}) (string, error) {
	if templateStr == "" {
		return "", nil
	}

	// Debug log the template context
	e.logger.V(2).Info("Template rendering debug", "template", templateStr, "context", contextMap)

	// Check cache first
	if cached, ok := e.templateCache.Load(templateStr); ok {
		tmpl := cached.(*template.Template)
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, contextMap); err != nil {
			return "", fmt.Errorf("failed to execute template: %w", err)
		}
		return buf.String(), nil
	}

	// Parse and cache template
	tmpl, err := template.New("template").Funcs(e.funcMap).Parse(templateStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	e.templateCache.Store(templateStr, tmpl)

	// Execute template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, contextMap); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// buildRequestInfoMap creates a request info map from a RequestInfo pointer, handling nil case.
func buildRequestInfoMap(requestInfo *request.RequestInfo) map[string]interface{} {
	if requestInfo == nil {
		return map[string]interface{}{
			"verb":              "",
			"resource":          "",
			"subresource":       "",
			"namespace":         "",
			"name":              "",
			"apiGroup":          "",
			"apiVersion":        "",
			"isResourceRequest": false,
			"path":              "",
			"parts":             []string{},
		}
	}

	return map[string]interface{}{
		"verb":              requestInfo.Verb,
		"resource":          requestInfo.Resource,
		"subresource":       requestInfo.Subresource,
		"namespace":         requestInfo.Namespace,
		"name":              requestInfo.Name,
		"apiGroup":          requestInfo.APIGroup,
		"apiVersion":        requestInfo.APIVersion,
		"isResourceRequest": requestInfo.IsResourceRequest,
		"path":              requestInfo.Path,
		"parts":             requestInfo.Parts,
	}
}

// Template function implementations
func (e *templateEngine) defaultFunc(defaultValue, value interface{}) interface{} {
	if value == nil || value == "" {
		return defaultValue
	}
	return value
}

func (e *templateEngine) containsFunc(substr, str string) bool {
	return strings.Contains(str, substr)
}

func (e *templateEngine) toIntFunc(value interface{}) (int, error) {
	switch v := value.(type) {
	case int:
		return v, nil
	case string:
		return strconv.Atoi(v)
	default:
		return 0, fmt.Errorf("cannot convert %T to int", value)
	}
}

func (e *templateEngine) toStringFunc(value interface{}) string {
	return fmt.Sprintf("%v", value)
}

// EvaluateConditions delegates to the CEL engine to evaluate trigger conditions.
func (e *templateEngine) EvaluateConditions(conditions []quotav1alpha1.ConditionExpression, obj *unstructured.Unstructured) (bool, error) {
	return e.celEngine.EvaluateConditions(conditions, obj)
}

// EvaluateParentContextName delegates to the CEL engine to evaluate a parent context name expression.
func (e *templateEngine) EvaluateParentContextName(expression string, obj *unstructured.Unstructured) (string, error) {
	return e.celEngine.EvaluateNameExpression(expression, obj)
}

// GenerateGrantName generates a consistent grant name for a policy and trigger object.
func (e *templateEngine) GenerateGrantName(policy *quotav1alpha1.GrantCreationPolicy, triggerObj *unstructured.Unstructured) (string, error) {
	// Create evaluation context
	evalContext := &GrantEvaluationContext{
		Object:       triggerObj,
		ResourceType: triggerObj.GetKind(),
	}

	// Try to render the grant name from the template
	name, _, _, _, _, err := e.RenderGrantMetadata(policy.Spec.Target.ResourceGrantTemplate.Metadata, evalContext)
	if err != nil {
		return "", fmt.Errorf("failed to generate grant name: %w", err)
	}

	// If no name was specified in template, generate a default one
	if name == "" {
		name = fmt.Sprintf("%s-%s-grant", policy.Name, triggerObj.GetName())
	}

	return name, nil
}

// RenderGrant renders a complete ResourceGrant from a GrantCreationPolicy.
func (e *templateEngine) RenderGrant(policy *quotav1alpha1.GrantCreationPolicy, triggerObj *unstructured.Unstructured, targetNamespace string) (*quotav1alpha1.ResourceGrant, error) {
	// Create evaluation context for grant rendering
	evalContext := &GrantEvaluationContext{
		Object:       triggerObj,
		ResourceType: triggerObj.GetKind(),
	}

	// Render the grant spec
	spec, err := e.RenderResourceGrant(context.Background(), policy.Spec.Target.ResourceGrantTemplate, evalContext)
	if err != nil {
		return nil, fmt.Errorf("failed to render grant spec: %w", err)
	}

	// Render metadata
	name, generateName, namespace, labels, annotations, err := e.RenderGrantMetadata(policy.Spec.Target.ResourceGrantTemplate.Metadata, evalContext)
	if err != nil {
		return nil, fmt.Errorf("failed to render grant metadata: %w", err)
	}

	// Use target namespace if specified (for parent context scenarios)
	if targetNamespace != "" {
		namespace = targetNamespace
	}

	// Create the ResourceGrant object
	grant := &quotav1alpha1.ResourceGrant{
		ObjectMeta: metav1.ObjectMeta{
			Name:         name,
			GenerateName: generateName,
			Namespace:    namespace,
			Labels:       labels,
			Annotations:  annotations,
		},
		Spec: *spec,
	}

	// Set TypeMeta
	grant.APIVersion = "quota.miloapis.com/v1alpha1"
	grant.Kind = "ResourceGrant"

	return grant, nil
}
