package quota

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"text/template"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	quotav1alpha1 "go.miloapis.com/milo/pkg/apis/quota/v1alpha1"
	"go.miloapis.com/milo/internal/validation/quota"
)

// TemplateEngine provides rendering capabilities for Go templates used in GrantCreationPolicy.
type TemplateEngine struct {
	celValidator *quota.CELValidator
}

// NewTemplateEngine creates a new template engine.
func NewTemplateEngine(celValidator *quota.CELValidator) *TemplateEngine {
	return &TemplateEngine{
		celValidator: celValidator,
	}
}

// TemplateContext creates a map of variables available during template rendering.
// Uses lowercase keys to match template convention (e.g., .trigger).
func CreateTemplateContext(resourceName, resourceKind, namespace, policyName string, triggerObj map[string]interface{}, apiVersion string) map[string]interface{} {
	// Build requestInfo similar to ClaimCreationPolicy (best effort in controller context)
	apiGroup := ""
	version := ""
	if strings.Contains(apiVersion, "/") {
		parts := strings.SplitN(apiVersion, "/", 2)
		apiGroup = parts[0]
		version = parts[1]
	} else {
		version = apiVersion
	}

	requestInfo := map[string]interface{}{
		"verb":        "reconcile",
		"resource":    "",
		"subresource": "",
		"name":        resourceName,
		"namespace":   namespace,
		"apiGroup":    apiGroup,
		"apiVersion":  version,
	}

	user := map[string]interface{}{
		"name":   "system:controller:grant-creation-policy",
		"uid":    "",
		"groups": []string{"system:authenticated"},
		"extra":  map[string][]string{},
	}

	return map[string]interface{}{
		// Supported variables
		"trigger":     triggerObj,
		"requestInfo": requestInfo,
		"user":        user,
	}
}

// RenderGrant renders a complete ResourceGrant from a template and context.
func (e *TemplateEngine) RenderGrant(
	policy *quotav1alpha1.GrantCreationPolicy,
	triggerObj *unstructured.Unstructured,
	targetNamespace string,
) (*quotav1alpha1.ResourceGrant, error) {
	// Create template context
	context := CreateTemplateContext(
		triggerObj.GetName(),
		triggerObj.GetKind(),
		triggerObj.GetNamespace(),
		policy.Name,
		triggerObj.Object,
		triggerObj.GetAPIVersion(),
	)

	// If no explicit target namespace provided, default to the trigger object's namespace
	if strings.TrimSpace(targetNamespace) == "" {
		targetNamespace = triggerObj.GetNamespace()
	}

	template := policy.Spec.Target.ResourceGrantTemplate

	// Render metadata
	metadata, err := e.renderMetadata(template.Metadata, context, targetNamespace)
	if err != nil {
		return nil, fmt.Errorf("failed to render metadata: %w", err)
	}

	// Render spec
	spec, err := e.renderSpec(template.Spec, context)
	if err != nil {
		return nil, fmt.Errorf("failed to render spec: %w", err)
	}

	// Create the ResourceGrant
	grant := &quotav1alpha1.ResourceGrant{
		TypeMeta: metav1.TypeMeta{
			APIVersion: quotav1alpha1.GroupVersion.String(),
			Kind:       "ResourceGrant",
		},
		ObjectMeta: *metadata,
		Spec:       *spec,
	}

	// Set owner reference to the trigger resource
	grant.OwnerReferences = []metav1.OwnerReference{
		{
			APIVersion: triggerObj.GetAPIVersion(),
			Kind:       triggerObj.GetKind(),
			Name:       triggerObj.GetName(),
			UID:        triggerObj.GetUID(),
			Controller: &[]bool{true}[0],
		},
	}

	// Add management labels
	if grant.Labels == nil {
		grant.Labels = make(map[string]string)
	}
	grant.Labels["quota.miloapis.com/managed-by"] = "grant-creation-policy"
	grant.Labels["quota.miloapis.com/policy"] = policy.Name
	grant.Labels["quota.miloapis.com/trigger-resource"] = triggerObj.GetKind()

	return grant, nil
}

// EvaluateConditions evaluates all trigger conditions against a resource object.
func (e *TemplateEngine) EvaluateConditions(
	conditions []quotav1alpha1.ConditionExpression,
	triggerObj *unstructured.Unstructured,
) (bool, error) {
	return e.celValidator.EvaluateConditions(conditions, triggerObj)
}

// EvaluateParentContextName evaluates a parent context name expression.
func (e *TemplateEngine) EvaluateParentContextName(
	expression string,
	triggerObj *unstructured.Unstructured,
) (string, error) {
	return e.celValidator.EvaluateNameExpression(expression, triggerObj)
}

// renderMetadata renders the metadata template.
func (e *TemplateEngine) renderMetadata(
	metadataTemplate quotav1alpha1.ObjectMetaTemplate,
	context map[string]interface{},
	targetNamespace string,
) (*metav1.ObjectMeta, error) {
	// Render name
	name, err := e.renderTemplate(metadataTemplate.Name, context)
	if err != nil {
		return nil, fmt.Errorf("failed to render name template: %w", err)
	}

	// Determine namespace
	namespace := targetNamespace
	if metadataTemplate.Namespace != "" {
		// Render namespace template if it contains template variables
		if e.containsTemplateVariables(metadataTemplate.Namespace) {
			renderedNamespace, err := e.renderTemplate(metadataTemplate.Namespace, context)
			if err != nil {
				return nil, fmt.Errorf("failed to render namespace template: %w", err)
			}
			namespace = renderedNamespace
		} else {
			namespace = metadataTemplate.Namespace
		}
	}
	if namespace == "" {
		namespace = quotav1alpha1.MiloSystemNamespace
	}

	// Render labels (no template variables allowed)
	labels := make(map[string]string)
	for key, value := range metadataTemplate.Labels {
		labels[key] = value
	}

	// Render annotations (template variables allowed in values)
	annotations := make(map[string]string)
	for key, value := range metadataTemplate.Annotations {
		renderedValue, err := e.renderTemplate(value, context)
		if err != nil {
			return nil, fmt.Errorf("failed to render annotation value for key %s: %w", key, err)
		}
		annotations[key] = renderedValue
	}

	return &metav1.ObjectMeta{
		Name:        name,
		Namespace:   namespace,
		Labels:      labels,
		Annotations: annotations,
	}, nil
}

// renderSpec renders the spec template.
func (e *TemplateEngine) renderSpec(
	specTemplate quotav1alpha1.ResourceGrantSpec,
	context map[string]interface{},
) (*quotav1alpha1.ResourceGrantSpec, error) {
	// Render consumer ref
	consumerRef, err := e.renderConsumerRef(specTemplate.ConsumerRef, context)
	if err != nil {
		return nil, fmt.Errorf("failed to render consumer ref: %w", err)
	}

	// Allowances are used as-is (no template conversion needed)
	return &quotav1alpha1.ResourceGrantSpec{
		ConsumerRef: *consumerRef,
		Allowances:  specTemplate.Allowances,
	}, nil
}

// renderConsumerRef renders the consumer reference template.
func (e *TemplateEngine) renderConsumerRef(
	consumerRef quotav1alpha1.ConsumerRef,
	context map[string]interface{},
) (*quotav1alpha1.ConsumerRef, error) {
	// Render API group
	apiGroup := consumerRef.APIGroup
	if e.containsTemplateVariables(apiGroup) {
		rendered, err := e.renderTemplate(apiGroup, context)
		if err != nil {
			return nil, fmt.Errorf("failed to render apiGroup: %w", err)
		}
		apiGroup = rendered
	}

	// Render kind
	kind := consumerRef.Kind
	if e.containsTemplateVariables(kind) {
		rendered, err := e.renderTemplate(kind, context)
		if err != nil {
			return nil, fmt.Errorf("failed to render kind: %w", err)
		}
		kind = rendered
	}

	// Render name
	name, err := e.renderTemplate(consumerRef.Name, context)
	if err != nil {
		return nil, fmt.Errorf("failed to render name: %w", err)
	}

	return &quotav1alpha1.ConsumerRef{
		APIGroup: apiGroup,
		Kind:     kind,
		Name:     name,
	}, nil
}

// renderTemplate renders a Go template string with the given context.
func (e *TemplateEngine) renderTemplate(templateStr string, context map[string]interface{}) (string, error) {
	if !e.containsTemplateVariables(templateStr) {
		// No template variables, return as-is
		return templateStr, nil
	}

	// Parse and execute the template
	tmpl, err := template.New("grant").Funcs(e.templateFuncMap()).Parse(templateStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, context); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// containsTemplateVariables checks if a string contains Go template variables.
func (e *TemplateEngine) containsTemplateVariables(str string) bool {
	return strings.Contains(str, "{{") && strings.Contains(str, "}}")
}

// GenerateGrantName generates a unique name for a ResourceGrant.
func (e *TemplateEngine) GenerateGrantName(
	policy *quotav1alpha1.GrantCreationPolicy,
	triggerObj *unstructured.Unstructured,
) (string, error) {
	context := CreateTemplateContext(
		triggerObj.GetName(),
		triggerObj.GetKind(),
		triggerObj.GetNamespace(),
		policy.Name,
		triggerObj.Object,
		triggerObj.GetAPIVersion(),
	)

	nameTemplate := policy.Spec.Target.ResourceGrantTemplate.Metadata.Name
	return e.renderTemplate(nameTemplate, context)
}

// templateFuncMap provides helper functions aligned with ClaimCreationPolicy template engine.
func (e *TemplateEngine) templateFuncMap() template.FuncMap {
	return template.FuncMap{
		"lower":    strings.ToLower,
		"upper":    strings.ToUpper,
		"title":    strings.Title,
		"default":  defaultFunc,
		"contains": containsFunc,
		"join":     strings.Join,
		"split":    strings.Split,
		"replace":  strings.ReplaceAll,
		"trim":     strings.TrimSpace,
		"toInt":    toIntFunc,
		"toString": toStringFunc,
	}
}

func defaultFunc(defaultValue, value interface{}) interface{} {
	if value == nil || fmt.Sprintf("%v", value) == "" {
		return defaultValue
	}
	return value
}

func containsFunc(substr, str string) bool {
	return strings.Contains(str, substr)
}

func toIntFunc(value interface{}) int64 {
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

func toStringFunc(value interface{}) string {
	return fmt.Sprintf("%v", value)
}
