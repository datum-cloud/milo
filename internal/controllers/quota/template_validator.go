package quota

import (
	"fmt"
	"regexp"
	"strings"
	"text/template"

	quotav1alpha1 "go.miloapis.com/milo/pkg/apis/quota/v1alpha1"
)

// TemplateValidator provides validation for Go templates used in GrantCreationPolicy.
type TemplateValidator struct {
	// validVariables defines the allowed template variables
	validVariables map[string]bool
}

// NewTemplateValidator creates a new template validator.
func NewTemplateValidator() *TemplateValidator {
	return &TemplateValidator{
		validVariables: map[string]bool{
			"resourceName": true,
			"resourceKind": true,
			"namespace":    true,
			"policyName":   true,
			"trigger":      true,
		},
	}
}

// ValidateGrantTemplate validates the entire ResourceGrant template structure.
func (v *TemplateValidator) ValidateGrantTemplate(grantTemplate quotav1alpha1.ResourceGrantTemplateSpec) error {
	// Validate metadata template
	if err := v.ValidateMetadataTemplate(grantTemplate.Metadata); err != nil {
		return fmt.Errorf("metadata template validation failed: %w", err)
	}

	// Validate spec template
	if err := v.ValidateSpecTemplate(grantTemplate.Spec); err != nil {
		return fmt.Errorf("spec template validation failed: %w", err)
	}

	return nil
}

// ValidateMetadataTemplate validates metadata template fields.
func (v *TemplateValidator) ValidateMetadataTemplate(metadata quotav1alpha1.GrantMetadataTemplate) error {
	// Validate name template
	if err := v.ValidateNameTemplate(metadata.Name); err != nil {
		return fmt.Errorf("name template validation failed: %w", err)
	}

	// Validate namespace if provided
	if metadata.Namespace != "" {
		// If namespace contains template variables, validate as template, otherwise validate as Kubernetes name
		if v.containsTemplateVariables(metadata.Namespace) {
			if err := v.ValidateTemplate(metadata.Namespace); err != nil {
				return fmt.Errorf("namespace template validation failed: %w", err)
			}
		} else {
			if err := v.validateKubernetesName(metadata.Namespace); err != nil {
				return fmt.Errorf("namespace validation failed: %w", err)
			}
		}
	}

	// Validate labels (no template variables allowed in keys or values for labels)
	for key, value := range metadata.Labels {
		if err := v.validateLabelKey(key); err != nil {
			return fmt.Errorf("label key '%s' validation failed: %w", key, err)
		}
		if err := v.validateLabelValue(value); err != nil {
			return fmt.Errorf("label value '%s' validation failed: %w", value, err)
		}
		// Labels should not contain template variables for consistency
		if v.containsTemplateVariables(key) || v.containsTemplateVariables(value) {
			return fmt.Errorf("label key/value should not contain template variables for consistency: %s=%s", key, value)
		}
	}

	// Validate annotations (template variables allowed in values)
	for key, value := range metadata.Annotations {
		if err := v.validateAnnotationKey(key); err != nil {
			return fmt.Errorf("annotation key '%s' validation failed: %w", key, err)
		}
		// Validate annotation value as template
		if v.containsTemplateVariables(value) {
			if err := v.ValidateTemplate(value); err != nil {
				return fmt.Errorf("annotation value template '%s' validation failed: %w", value, err)
			}
		}
	}

	return nil
}

// ValidateSpecTemplate validates the spec template structure.
func (v *TemplateValidator) ValidateSpecTemplate(spec quotav1alpha1.GrantSpecTemplate) error {
	// Validate consumer ref template
	if err := v.ValidateConsumerRefTemplate(spec.ConsumerRefTemplate); err != nil {
		return fmt.Errorf("consumer ref template validation failed: %w", err)
	}

	// Validate allowances
	if len(spec.Allowances) == 0 {
		return fmt.Errorf("at least one allowance must be specified")
	}

	for i, allowance := range spec.Allowances {
		if err := v.ValidateAllowanceTemplate(allowance); err != nil {
			return fmt.Errorf("allowance %d validation failed: %w", i, err)
		}
	}

	return nil
}

// ValidateConsumerRefTemplate validates the consumer reference template.
func (v *TemplateValidator) ValidateConsumerRefTemplate(consumerRef quotav1alpha1.ConsumerRefTemplate) error {
	// Validate APIGroup template if provided
	if consumerRef.APIGroup != "" {
		if v.containsTemplateVariables(consumerRef.APIGroup) {
			if err := v.ValidateTemplate(consumerRef.APIGroup); err != nil {
				return fmt.Errorf("apiGroup template validation failed: %w", err)
			}
		} else {
			if err := v.validateAPIGroup(consumerRef.APIGroup); err != nil {
				return fmt.Errorf("apiGroup validation failed: %w", err)
			}
		}
	}

	// Validate Kind template
	if v.containsTemplateVariables(consumerRef.Kind) {
		if err := v.ValidateTemplate(consumerRef.Kind); err != nil {
			return fmt.Errorf("kind template validation failed: %w", err)
		}
	} else {
		if err := v.validateKind(consumerRef.Kind); err != nil {
			return fmt.Errorf("kind validation failed: %w", err)
		}
	}

	// Validate Name (for ConsumerRef, we allow templates that start with variables since they reference existing resources)
	if err := v.ValidateConsumerRefNameTemplate(consumerRef.Name); err != nil {
		return fmt.Errorf("name template validation failed: %w", err)
	}

	return nil
}

// ValidateAllowanceTemplate validates an allowance template.
func (v *TemplateValidator) ValidateAllowanceTemplate(allowance quotav1alpha1.AllowanceTemplate) error {
	// Validate resource type format
	if err := v.validateResourceType(allowance.ResourceType); err != nil {
		return fmt.Errorf("resource type validation failed: %w", err)
	}

	// Validate buckets
	if len(allowance.Buckets) == 0 {
		return fmt.Errorf("at least one bucket must be specified")
	}

	for i, bucket := range allowance.Buckets {
		if err := v.ValidateBucketTemplate(bucket); err != nil {
			return fmt.Errorf("bucket %d validation failed: %w", i, err)
		}
	}

	return nil
}

// ValidateBucketTemplate validates a bucket template.
func (v *TemplateValidator) ValidateBucketTemplate(bucket quotav1alpha1.BucketTemplate) error {
	// Validate amount (must be non-negative)
	if bucket.Amount < 0 {
		return fmt.Errorf("amount cannot be negative: %d", bucket.Amount)
	}

	// Validate dimension selector if provided
	if bucket.DimensionSelector != nil {
		if err := v.validateLabelSelector(bucket.DimensionSelector); err != nil {
			return fmt.Errorf("dimension selector validation failed: %w", err)
		}
	}

	return nil
}

// ValidateNameTemplate validates a name template for Kubernetes resource names.
func (v *TemplateValidator) ValidateNameTemplate(nameTemplate string) error {
	if strings.TrimSpace(nameTemplate) == "" {
		return fmt.Errorf("name template cannot be empty")
	}

	// Validate template syntax
	if err := v.ValidateTemplate(nameTemplate); err != nil {
		return fmt.Errorf("template syntax validation failed: %w", err)
	}

	// Additional validation for name constraints
	// Names should not start or end with template variables to ensure valid Kubernetes names
	if strings.HasPrefix(strings.TrimSpace(nameTemplate), "{{") {
		return fmt.Errorf("name template should not start with a template variable to ensure valid Kubernetes names")
	}

	return nil
}

// ValidateConsumerRefNameTemplate validates a name template for ConsumerRef.
// Unlike ValidateNameTemplate, this allows templates that start with variables
// since ConsumerRef references existing resources by their exact names.
func (v *TemplateValidator) ValidateConsumerRefNameTemplate(nameTemplate string) error {
	if strings.TrimSpace(nameTemplate) == "" {
		return fmt.Errorf("name template cannot be empty")
	}

	// Validate template syntax
	if err := v.ValidateTemplate(nameTemplate); err != nil {
		return fmt.Errorf("template syntax validation failed: %w", err)
	}

	// For ConsumerRef, we don't enforce the "no starting with {{" rule
	// since we're referencing existing resources by their exact names
	return nil
}

// ValidateTemplate validates Go template syntax and variable usage.
func (v *TemplateValidator) ValidateTemplate(templateStr string) error {
	if strings.TrimSpace(templateStr) == "" {
		return fmt.Errorf("template cannot be empty")
	}

	// Parse the template to check syntax
	tmpl, err := template.New("test").Parse(templateStr)
	if err != nil {
		return fmt.Errorf("template parse error: %w", err)
	}

	// Extract and validate variables used in the template
	variables := v.extractTemplateVariables(templateStr)
	for _, variable := range variables {
		if !v.validVariables[variable] {
			return fmt.Errorf("invalid template variable: %s. Valid variables are: %v",
				variable, v.getValidVariablesList())
		}
	}

	// Test template execution with dummy data to catch runtime errors
	testData := map[string]interface{}{
		"resourceName": "test-resource",
		"resourceKind": "TestKind",
		"namespace":    "test-namespace",
		"policyName":   "test-policy",
		"trigger": map[string]interface{}{
			"metadata": map[string]interface{}{
				"name": "test-trigger-resource",
			},
			"spec": map[string]interface{}{
				"type": "Standard",
			},
		},
	}

	var result strings.Builder
	if err := tmpl.Execute(&result, testData); err != nil {
		return fmt.Errorf("template execution test failed: %w", err)
	}

	return nil
}

// containsTemplateVariables checks if a string contains Go template variables.
func (v *TemplateValidator) containsTemplateVariables(str string) bool {
	return strings.Contains(str, "{{") && strings.Contains(str, "}}")
}

// extractTemplateVariables extracts variable names from a Go template string.
func (v *TemplateValidator) extractTemplateVariables(templateStr string) []string {
	// Regex to extract template variables like {{ .Variable }} or {{ .trigger.metadata.name }}
	re := regexp.MustCompile(`{{\s*\.(\w+)(?:\.\w+)*\s*}}`)
	matches := re.FindAllStringSubmatch(templateStr, -1)

	var variables []string
	seen := make(map[string]bool)

	for _, match := range matches {
		if len(match) > 1 {
			// Extract the root variable (first part before any dots)
			variable := match[1]
			if !seen[variable] {
				variables = append(variables, variable)
				seen[variable] = true
			}
		}
	}

	return variables
}

// getValidVariablesList returns a list of valid template variables.
func (v *TemplateValidator) getValidVariablesList() []string {
	var variables []string
	for variable := range v.validVariables {
		variables = append(variables, variable)
	}
	return variables
}

// Kubernetes validation helpers

func (v *TemplateValidator) validateKubernetesName(name string) error {
	if len(name) == 0 {
		return fmt.Errorf("name cannot be empty")
	}
	if len(name) > 253 {
		return fmt.Errorf("name cannot be longer than 253 characters")
	}

	// DNS subdomain name validation
	re := regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
	if !re.MatchString(name) {
		return fmt.Errorf("name must be a valid DNS subdomain name")
	}

	return nil
}

func (v *TemplateValidator) validateLabelKey(key string) error {
	if len(key) == 0 {
		return fmt.Errorf("label key cannot be empty")
	}
	if len(key) > 253 {
		return fmt.Errorf("label key cannot be longer than 253 characters")
	}

	// Basic label key validation
	re := regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-_.]*[a-zA-Z0-9])?(/[a-zA-Z0-9]([a-zA-Z0-9\-_.]*[a-zA-Z0-9])?)?$`)
	if !re.MatchString(key) {
		return fmt.Errorf("invalid label key format")
	}

	return nil
}

func (v *TemplateValidator) validateLabelValue(value string) error {
	if len(value) > 63 {
		return fmt.Errorf("label value cannot be longer than 63 characters")
	}

	if value != "" {
		re := regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-_.]*[a-zA-Z0-9])?$`)
		if !re.MatchString(value) {
			return fmt.Errorf("invalid label value format")
		}
	}

	return nil
}

func (v *TemplateValidator) validateAnnotationKey(key string) error {
	return v.validateLabelKey(key) // Same validation rules
}

func (v *TemplateValidator) validateAPIGroup(apiGroup string) error {
	if apiGroup == "" {
		return nil // Empty API group is valid for core resources
	}

	re := regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`)
	if !re.MatchString(apiGroup) {
		return fmt.Errorf("invalid API group format")
	}

	return nil
}

func (v *TemplateValidator) validateKind(kind string) error {
	if len(kind) == 0 {
		return fmt.Errorf("kind cannot be empty")
	}
	if len(kind) > 63 {
		return fmt.Errorf("kind cannot be longer than 63 characters")
	}

	re := regexp.MustCompile(`^[A-Z][a-zA-Z0-9]*$`)
	if !re.MatchString(kind) {
		return fmt.Errorf("kind must start with uppercase letter and contain only alphanumeric characters")
	}

	return nil
}

func (v *TemplateValidator) validateResourceType(resourceType string) error {
	// Resource type format: apiGroup/Kind
	parts := strings.Split(resourceType, "/")
	if len(parts) != 2 {
		return fmt.Errorf("resource type must be in format 'apiGroup/Kind'")
	}

	if err := v.validateAPIGroup(parts[0]); err != nil {
		return fmt.Errorf("invalid API group in resource type: %w", err)
	}

	if err := v.validateKind(parts[1]); err != nil {
		return fmt.Errorf("invalid kind in resource type: %w", err)
	}

	return nil
}

func (v *TemplateValidator) validateLabelSelector(selector interface{}) error {
	// Basic validation - would need more comprehensive validation in real implementation
	// This is a placeholder for label selector validation
	return nil
}
