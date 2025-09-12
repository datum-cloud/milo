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
	// RenderResourceClaim renders a ResourceClaimTemplateSpec into a ResourceClaimSpec.
	RenderResourceClaim(ctx context.Context, template quotav1alpha1.ResourceClaimTemplateSpec, evalContext *EvaluationContext, policyEngine PolicyEngine) (*quotav1alpha1.ResourceClaimSpec, error)

	// BuildTemplateContext creates a template context for Go template rendering.
	BuildTemplateContext(evalContext *EvaluationContext) TemplateContext
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

// RenderResourceClaim renders a ResourceClaimTemplateSpec into a ResourceClaimSpec.
func (e *templateEngine) RenderResourceClaim(ctx context.Context, templateSpec quotav1alpha1.ResourceClaimTemplateSpec, evalContext *EvaluationContext, policyEngine PolicyEngine) (*quotav1alpha1.ResourceClaimSpec, error) {
	// Build template context for Go templates
	templateContext := e.BuildTemplateContext(evalContext)

	// Render name template if specified
	var claimName string
	if templateSpec.NameTemplate != "" {
		name, err := e.renderGoTemplate(templateSpec.NameTemplate, templateContext)
		if err != nil {
			return nil, fmt.Errorf("failed to render name template: %w", err)
		}
		claimName = name
	} else {
		// Use default naming pattern
		claimName = fmt.Sprintf("%s-claim-%s", templateContext.ResourceName, templateContext.RandomSuffix)
	}

	// Process resource requests using CEL expressions and field suffixes
	var resourceRequests []quotav1alpha1.ResourceRequest
	for i, requestTemplate := range templateSpec.Requests {
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

	e.logger.V(1).Info("Successfully rendered ResourceClaim",
		"claimName", claimName,
		"requestCount", len(resourceRequests),
		"resourceName", templateContext.ResourceName)

	return spec, nil
}

// renderResourceRequest renders a single ResourceRequestTemplate.
func (e *templateEngine) renderResourceRequest(ctx context.Context, requestTemplate *quotav1alpha1.ResourceRequestTemplate, evalContext *EvaluationContext, templateContext TemplateContext, policyEngine PolicyEngine) (*quotav1alpha1.ResourceRequest, error) {
	// Evaluate condition expression first (if specified)
	if requestTemplate.ConditionExpression != "" {
		result, err := e.celEngine.EvaluateExpression(ctx, requestTemplate.ConditionExpression, evalContext)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate condition expression '%s': %w", requestTemplate.ConditionExpression, err)
		}

		// Convert result to boolean
		shouldInclude, ok := result.(bool)
		if !ok {
			return nil, fmt.Errorf("condition expression must return boolean, got %T", result)
		}

		if !shouldInclude {
			return nil, nil // Skip this request
		}
	}

	// Render resource type
	resourceType, err := e.renderResourceType(ctx, requestTemplate, evalContext, templateContext)
	if err != nil {
		return nil, fmt.Errorf("failed to render resource type: %w", err)
	}

	// Note: Resource type validation is now done at policy creation time,
	// so we don't need runtime validation here since resource types are static

	// Render amount
	amount, err := e.renderAmount(ctx, requestTemplate, evalContext, templateContext)
	if err != nil {
		return nil, fmt.Errorf("failed to render amount: %w", err)
	}

	// Render dimensions
	dimensions, err := e.renderDimensions(ctx, requestTemplate, evalContext, templateContext)
	if err != nil {
		return nil, fmt.Errorf("failed to render dimensions: %w", err)
	}

	return &quotav1alpha1.ResourceRequest{
		ResourceType: resourceType,
		Amount:       amount,
		Dimensions:   dimensions,
	}, nil
}

// renderResourceType renders the resource type using static value.
func (e *templateEngine) renderResourceType(ctx context.Context, requestTemplate *quotav1alpha1.ResourceRequestTemplate, evalContext *EvaluationContext, templateContext TemplateContext) (string, error) {
	// Resource type is now always static and required
	return requestTemplate.ResourceType, nil
}

// renderAmount renders the amount using static value or CEL expression.
func (e *templateEngine) renderAmount(ctx context.Context, requestTemplate *quotav1alpha1.ResourceRequestTemplate, evalContext *EvaluationContext, templateContext TemplateContext) (int64, error) {
	if requestTemplate.Amount != nil {
		// Static value
		return *requestTemplate.Amount, nil
	}

	if requestTemplate.AmountExpression != "" {
		// CEL expression
		result, err := e.celEngine.EvaluateExpression(ctx, requestTemplate.AmountExpression, evalContext)
		if err != nil {
			return 0, fmt.Errorf("failed to evaluate amount expression '%s': %w", requestTemplate.AmountExpression, err)
		}

		// Convert result to int64
		switch v := result.(type) {
		case int64:
			return v, nil
		case int:
			return int64(v), nil
		case float64:
			return int64(v), nil
		case string:
			amount, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return 0, fmt.Errorf("amount expression returned non-numeric string '%s'", v)
			}
			return amount, nil
		default:
			return 0, fmt.Errorf("amount expression must return numeric value, got %T", result)
		}
	}

	return 0, fmt.Errorf("either amount or amountExpression must be specified")
}

// renderDimensions renders dimensions using static values and/or CEL expressions.
func (e *templateEngine) renderDimensions(ctx context.Context, requestTemplate *quotav1alpha1.ResourceRequestTemplate, evalContext *EvaluationContext, templateContext TemplateContext) (map[string]string, error) {
	dimensions := make(map[string]string)

	// Add static dimensions first
	for key, value := range requestTemplate.Dimensions {
		dimensions[key] = value
	}

	// Add CEL expression dimensions (these take precedence for duplicate keys)
	for key, expression := range requestTemplate.DimensionExpressions {
		result, err := e.celEngine.EvaluateExpression(ctx, expression, evalContext)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate dimension expression for key '%s': %w", key, err)
		}

		// Convert result to string
		dimensions[key] = fmt.Sprintf("%v", result)
	}

	return dimensions, nil
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

// executeTemplate executes a template with the given context.
func (e *templateEngine) executeTemplate(tmpl *template.Template, context TemplateContext) (string, error) {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, context); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}
	return buf.String(), nil
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
