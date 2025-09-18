package quota

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/go-logr/logr"

	iamv1alpha1 "go.miloapis.com/milo/pkg/apis/iam/v1alpha1"
	quotav1alpha1 "go.miloapis.com/milo/pkg/apis/quota/v1alpha1"
)

// TemplateEngine handles ResourceClaim generation from ClaimCreationPolicy templates.
type TemplateEngine interface {
	// RenderResourceClaim renders a ResourceClaimTemplate into a ResourceClaimSpec.
	RenderResourceClaim(ctx context.Context, template quotav1alpha1.ResourceClaimTemplate, evalContext *EvaluationContext, policyEngine PolicyEngine) (*quotav1alpha1.ResourceClaimSpec, error)

	// BuildTemplateContext creates a template context for Go template rendering.
	BuildTemplateContext(evalContext *EvaluationContext) TemplateContext

	// RenderClaimMetadata renders name/generateName/namespace and annotations for claim metadata.
	RenderClaimMetadata(metadata quotav1alpha1.ObjectMetaTemplate, evalContext *EvaluationContext) (string, string, string, map[string]string, map[string]string, error)

	// EvaluateConditions evaluates policy trigger conditions using CEL in the admission context.
	EvaluateConditions(ctx context.Context, conditions []quotav1alpha1.ConditionExpression, evalContext *EvaluationContext) (bool, error)
}

// TemplateContext provides variables for Go template rendering.
type TemplateContext struct {
	// Resource information
	ResourceName string
	Namespace    string
	Kind         string
	APIVersion   string

	// User information
	UserName   string
	UserUID    string
	UserGroups []string

	// Request information
	Verb        string
	Resource    string
	Subresource string

	// Context information
	GVK          string
	RandomSuffix string
	Timestamp    string
}

// templateEngine implements TemplateEngine.
type templateEngine struct {
	logger        logr.Logger
	templateCache sync.Map // string -> *template.Template
	funcMap       template.FuncMap
	celEngine     CELEngine
	random        *rand.Rand
}

// NewTemplateEngine creates a new template engine.
func NewTemplateEngine(celEngine CELEngine, logger logr.Logger) TemplateEngine {
	engine := &templateEngine{
		logger:    logger.WithName("template-engine"),
		celEngine: celEngine,
		random:    rand.New(rand.NewSource(time.Now().UnixNano())),
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
func (e *templateEngine) RenderResourceClaim(ctx context.Context, template quotav1alpha1.ResourceClaimTemplate, evalContext *EvaluationContext, policyEngine PolicyEngine) (*quotav1alpha1.ResourceClaimSpec, error) {
	// Build template context for Go templates (used by CEL resolution and legacy helpers)
	templateContext := e.BuildTemplateContext(evalContext)

	// Process resource requests using CEL expressions and field suffixes
	var resourceRequests []quotav1alpha1.ResourceRequest
	for i, requestTemplate := range template.Spec.Requests {
		request, err := e.renderResourceRequest(ctx, &requestTemplate, evalContext, templateContext, policyEngine)
		if err != nil {
			return nil, fmt.Errorf("failed to render resource request %d: %w", i, err)
		}

		// Skip request if condition evaluation results in false
		if request == nil {
			e.logger.V(1).Info("Skipping resource request due to condition",
				"requestIndex", i,
				"resourceName", templateContext.ResourceName)
			continue
		}

		resourceRequests = append(resourceRequests, *request)
	}

	if len(resourceRequests) == 0 {
		return nil, fmt.Errorf("no resource requests generated after template evaluation")
	}

	// Resolve consumer reference - try user extra data first, then fallback to resource-based
	consumerRef, err := e.resolveConsumerRef(evalContext, templateContext)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve consumer reference: %w", err)
	}

	// Build the ResourceClaimSpec
	spec := &quotav1alpha1.ResourceClaimSpec{
		ConsumerRef: *consumerRef,
		Requests:    resourceRequests,
	}

	e.logger.V(1).Info("Successfully rendered ResourceClaim spec",
		"requestCount", len(resourceRequests),
		"resourceName", templateContext.ResourceName)

	return spec, nil
}

// renderResourceRequest renders a single ResourceRequestTemplate.
func (e *templateEngine) renderResourceRequest(ctx context.Context, requestTemplate *quotav1alpha1.ResourceRequest, evalContext *EvaluationContext, templateContext TemplateContext, policyEngine PolicyEngine) (*quotav1alpha1.ResourceRequest, error) {
	// Render resource type
	resourceType, err := e.renderResourceType(ctx, requestTemplate, evalContext, templateContext)
	if err != nil {
		return nil, fmt.Errorf("failed to render resource type: %w", err)
	}

	// Render amount
	amount, err := e.renderAmount(ctx, requestTemplate, evalContext, templateContext)
	if err != nil {
		return nil, fmt.Errorf("failed to render amount: %w", err)
	}

	return &quotav1alpha1.ResourceRequest{
		ResourceType: resourceType,
		Amount:       amount,
	}, nil
}

// renderResourceType renders the resource type using static value.
func (e *templateEngine) renderResourceType(ctx context.Context, requestTemplate *quotav1alpha1.ResourceRequest, evalContext *EvaluationContext, templateContext TemplateContext) (string, error) {
	// Resource type is now always static and required
	return requestTemplate.ResourceType, nil
}

// renderAmount renders the amount using static value or CEL expression.
func (e *templateEngine) renderAmount(ctx context.Context, requestTemplate *quotav1alpha1.ResourceRequest, evalContext *EvaluationContext, templateContext TemplateContext) (int64, error) {
	return requestTemplate.Amount, nil
}

// BuildTemplateContext creates a template context for Go template rendering.
func (e *templateEngine) BuildTemplateContext(evalContext *EvaluationContext) TemplateContext {
	// Generate random suffix for unique naming
	randomSuffix := e.generateRandomSuffix(6)

	// Extract resource information
	resourceName := ""
	if evalContext.Object != nil {
		resourceName = evalContext.Object.GetName()
	}

	// Build user groups string
	userGroups := evalContext.User.Groups
	if userGroups == nil {
		userGroups = []string{}
	}

	return TemplateContext{
		ResourceName: resourceName,
		Namespace:    evalContext.Namespace,
		Kind:         evalContext.GVK.Kind,
		APIVersion:   fmt.Sprintf("%s/%s", evalContext.GVK.Group, evalContext.GVK.Version),

		UserName:   evalContext.User.Name,
		UserUID:    evalContext.User.UID,
		UserGroups: userGroups,

		Verb:        evalContext.RequestInfo.Verb,
		Resource:    evalContext.RequestInfo.Resource,
		Subresource: evalContext.RequestInfo.Subresource,

		GVK:          fmt.Sprintf("%s/%s/%s", evalContext.GVK.Group, evalContext.GVK.Version, evalContext.GVK.Kind),
		RandomSuffix: randomSuffix,
		Timestamp:    time.Now().Format("20060102150405"),
	}
}

// renderGoTemplate renders a Go template string with the given context.
func (e *templateEngine) renderGoTemplate(templateStr string, context TemplateContext) (string, error) {
	// Check if template is cached
	if cachedTemplate, ok := e.templateCache.Load(templateStr); ok {
		tmpl := cachedTemplate.(*template.Template)
		return e.executeTemplate(tmpl, context)
	}

	// Parse and cache the template
	tmpl, err := template.New("template").Funcs(e.funcMap).Parse(templateStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse template '%s': %w", templateStr, err)
	}

	e.templateCache.Store(templateStr, tmpl)
	return e.executeTemplate(tmpl, context)
}

// renderGoTemplateAny renders a Go template string with an arbitrary context (map or struct).
func (e *templateEngine) renderGoTemplateAny(templateStr string, context interface{}) (string, error) {
	// Check cache
	if cachedTemplate, ok := e.templateCache.Load(templateStr); ok {
		tmpl := cachedTemplate.(*template.Template)
		return e.executeTemplateAny(tmpl, context)
	}

	tmpl, err := template.New("template").Funcs(e.funcMap).Parse(templateStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse template '%s': %w", templateStr, err)
	}
	e.templateCache.Store(templateStr, tmpl)
	return e.executeTemplateAny(tmpl, context)
}

// executeTemplate executes a template with the given context.
func (e *templateEngine) executeTemplate(tmpl *template.Template, context TemplateContext) (string, error) {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, context); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}
	return buf.String(), nil
}

// executeTemplateAny executes a template with an arbitrary context (map or struct).
func (e *templateEngine) executeTemplateAny(tmpl *template.Template, context interface{}) (string, error) {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, context); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}
	return buf.String(), nil
}

// CreateTemplateContextForClaim builds the map context used by claim metadata templates.
// Exposes well-known lowercase objects: .trigger, .requestInfo, .user
func (e *templateEngine) CreateTemplateContextForClaim(evalContext *EvaluationContext) map[string]interface{} {
	ctx := map[string]interface{}{}
	// trigger: full unstructured object map
	if evalContext != nil && evalContext.Object != nil {
		ctx["trigger"] = evalContext.Object.Object
	} else {
		ctx["trigger"] = map[string]interface{}{}
	}
	// requestInfo: selected fields
	if evalContext != nil && evalContext.RequestInfo != nil {
		ctx["requestInfo"] = map[string]interface{}{
			"verb":        evalContext.RequestInfo.Verb,
			"resource":    evalContext.RequestInfo.Resource,
			"subresource": evalContext.RequestInfo.Subresource,
			"name":        evalContext.RequestInfo.Name,
			"namespace":   evalContext.RequestInfo.Namespace,
			"apiGroup":    evalContext.RequestInfo.APIGroup,
			"apiVersion":  evalContext.RequestInfo.APIVersion,
		}
	} else {
		ctx["requestInfo"] = map[string]interface{}{}
	}
	// user: name, uid, groups, extra
	if evalContext != nil {
		ctx["user"] = map[string]interface{}{
			"name":   evalContext.User.Name,
			"uid":    evalContext.User.UID,
			"groups": evalContext.User.Groups,
			"extra":  evalContext.User.Extra,
		}
	} else {
		ctx["user"] = map[string]interface{}{}
	}
	return ctx
}

// RenderClaimMetadata renders name/generateName/namespace and annotation values for the claim metadata.
// Returns rendered values and literal labels.
func (e *templateEngine) RenderClaimMetadata(metadata quotav1alpha1.ObjectMetaTemplate, evalContext *EvaluationContext) (string, string, string, map[string]string, map[string]string, error) {
	ctx := e.CreateTemplateContextForClaim(evalContext)

	// Render name and generateName (templated)
	var name, generateName, namespace string
	var err error

	if strings.TrimSpace(metadata.Name) != "" {
		name, err = e.renderGoTemplateAny(metadata.Name, ctx)
		if err != nil {
			return "", "", "", nil, nil, fmt.Errorf("failed to render metadata.name: %w", err)
		}
	}
	if strings.TrimSpace(metadata.GenerateName) != "" {
		generateName, err = e.renderGoTemplateAny(metadata.GenerateName, ctx)
		if err != nil {
			return "", "", "", nil, nil, fmt.Errorf("failed to render metadata.generateName: %w", err)
		}
	}
	if strings.TrimSpace(metadata.Namespace) != "" {
		namespace, err = e.renderGoTemplateAny(metadata.Namespace, ctx)
		if err != nil {
			return "", "", "", nil, nil, fmt.Errorf("failed to render metadata.namespace: %w", err)
		}
	}

	// Render annotations values
	renderedAnnotations := map[string]string{}
	for k, v := range metadata.Annotations {
		if strings.Contains(v, "{{") {
			rv, err := e.renderGoTemplateAny(v, ctx)
			if err != nil {
				return "", "", "", nil, nil, fmt.Errorf("failed to render annotation %q: %w", k, err)
			}
			renderedAnnotations[k] = rv
		} else {
			renderedAnnotations[k] = v
		}
	}

	// Labels are literal (no templating)
	labels := map[string]string{}
	for k, v := range metadata.Labels {
		labels[k] = v
	}

	return name, generateName, namespace, labels, renderedAnnotations, nil
}

// generateRandomSuffix generates a random alphanumeric suffix.
func (e *templateEngine) generateRandomSuffix(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	suffix := make([]byte, length)
	for i := range suffix {
		suffix[i] = charset[e.random.Intn(len(charset))]
	}
	return string(suffix)
}

// resolveConsumerRef determines the quota consumer reference from user extra data or resource-based fallback
func (e *templateEngine) resolveConsumerRef(evalContext *EvaluationContext, templateContext TemplateContext) (*quotav1alpha1.ConsumerRef, error) {
	// Try user extra data first (primary method)
	consumerRef, err := e.extractConsumerFromUserExtra(evalContext)
	if err == nil {
		e.logger.V(1).Info("Resolved consumer from user extra data",
			"consumer", consumerRef.Kind+"/"+consumerRef.Name,
			"apiGroup", consumerRef.APIGroup)
		return consumerRef, nil
	}

	// Fallback to resource-based resolution for backward compatibility
	e.logger.V(1).Info("User extra data not available, using resource-based consumer resolution", "error", err)

	return &quotav1alpha1.ConsumerRef{
		APIGroup: evalContext.GVK.Group,
		Kind:     templateContext.Kind,
		Name:     templateContext.ResourceName,
	}, nil
}

// extractConsumerFromUserExtra extracts consumer information from user authentication extra data
func (e *templateEngine) extractConsumerFromUserExtra(evalContext *EvaluationContext) (*quotav1alpha1.ConsumerRef, error) {
	userInfo := evalContext.User
	e.logger.V(1).Info("Checking user extra data for consumer resolution",
		"userName", userInfo.Name,
		"extraKeys", func() []string {
			if userInfo.Extra == nil {
				return nil
			}
			keys := make([]string, 0, len(userInfo.Extra))
			for k := range userInfo.Extra {
				keys = append(keys, k)
			}
			return keys
		}())

	if userInfo.Extra == nil {
		return nil, fmt.Errorf("user extra data is nil")
	}

	// Extract parent/consumer information
	parentName := getExtraValue(userInfo.Extra, iamv1alpha1.ParentNameExtraKey)
	parentKind := getExtraValue(userInfo.Extra, iamv1alpha1.ParentKindExtraKey)
	parentAPIGroup := getExtraValue(userInfo.Extra, iamv1alpha1.ParentAPIGroupExtraKey)

	e.logger.V(1).Info("Extracted parent information from user extra data",
		"parentName", parentName,
		"parentKind", parentKind,
		"parentAPIGroup", parentAPIGroup,
		"parentNameKey", iamv1alpha1.ParentNameExtraKey,
		"parentKindKey", iamv1alpha1.ParentKindExtraKey,
		"parentAPIGroupKey", iamv1alpha1.ParentAPIGroupExtraKey)

	if parentName == "" || parentKind == "" {
		return nil, fmt.Errorf("required parent information missing from user extra data")
	}

	return &quotav1alpha1.ConsumerRef{
		APIGroup: parentAPIGroup,
		Kind:     parentKind,
		Name:     parentName,
	}, nil
}

// getExtraValue safely extracts a single value from user extra data
func getExtraValue(extra map[string][]string, key string) string {
	if values, exists := extra[key]; exists && len(values) > 0 {
		return values[0]
	}
	return ""
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

func (e *templateEngine) toIntFunc(value interface{}) int64 {
	switch v := value.(type) {
	case int:
		return int64(v)
	case int32:
		return int64(v)
	case int64:
		return v
	case float32:
		return int64(v)
	case float64:
		return int64(v)
	case string:
		if i, err := strconv.ParseInt(v, 10, 64); err == nil {
			return i
		}
	}
	return 0
}

func (e *templateEngine) toStringFunc(value interface{}) string {
	return fmt.Sprintf("%v", value)
}

// EvaluateConditions evaluates all conditions; returns true only if all are true.
func (e *templateEngine) EvaluateConditions(ctx context.Context, conditions []quotav1alpha1.ConditionExpression, evalContext *EvaluationContext) (bool, error) {
	for i, c := range conditions {
		if strings.TrimSpace(c.Expression) == "" {
			continue
		}
		result, err := e.celEngine.EvaluateExpression(ctx, c.Expression, evalContext)
		if err != nil {
			return false, fmt.Errorf("failed to evaluate condition %d: %w", i, err)
		}
		b, ok := result.(bool)
		if !ok {
			return false, fmt.Errorf("condition %d did not return boolean", i)
		}
		if !b {
			return false, nil
		}
	}
	return true, nil
}
