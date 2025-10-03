package validation

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"text/template"

	quotav1alpha1 "go.miloapis.com/milo/pkg/apis/quota/v1alpha1"
	"k8s.io/apimachinery/pkg/util/validation"
)

// GrantTemplateValidator provides validation for Go templates used in GrantCreationPolicy.
type GrantTemplateValidator struct {
	// validVariables defines the allowed template variables
	validVariables map[string]bool
	// resourceTypeValidator validates resource types against ResourceRegistrations
	resourceTypeValidator ResourceTypeValidator
}

// NewGrantTemplateValidator creates a new grant template validator.
func NewGrantTemplateValidator(resourceTypeValidator ResourceTypeValidator) *GrantTemplateValidator {
	return &GrantTemplateValidator{
		validVariables: map[string]bool{
			"trigger": true,
		},
		resourceTypeValidator: resourceTypeValidator,
	}
}

// ValidateGrantTemplate validates the entire ResourceGrant template structure.
func (v *GrantTemplateValidator) ValidateGrantTemplate(ctx context.Context, grantTemplate quotav1alpha1.ResourceGrantTemplate) error {
	// Validate metadata template
	if err := v.ValidateMetadataTemplate(grantTemplate.Metadata); err != nil {
		return fmt.Errorf("metadata template validation failed: %w", err)
	}

	// Validate spec template (including resource type validation)
	if err := v.ValidateSpecTemplate(ctx, grantTemplate.Spec); err != nil {
		return fmt.Errorf("spec template validation failed: %w", err)
	}

	return nil
}

// ValidateMetadataTemplate validates metadata template fields.
func (v *GrantTemplateValidator) ValidateMetadataTemplate(metadata quotav1alpha1.ObjectMetaTemplate) error {
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
	}

	// Validate annotations (keys must be valid, values can contain template variables)
	for key, value := range metadata.Annotations {
		if err := v.validateAnnotationKey(key); err != nil {
			return fmt.Errorf("annotation key '%s' validation failed: %w", key, err)
		}
		// Annotation values can contain template variables, so validate as template if needed
		if v.containsTemplateVariables(value) {
			if err := v.ValidateTemplate(value); err != nil {
				return fmt.Errorf("annotation value template validation failed for key '%s': %w", key, err)
			}
		}
	}

	return nil
}

// ValidateSpecTemplate validates the spec template structure.
func (v *GrantTemplateValidator) ValidateSpecTemplate(ctx context.Context, spec quotav1alpha1.ResourceGrantSpec) error {
	// Validate consumer ref template
	if err := v.ValidateConsumerRefTemplate(spec.ConsumerRef); err != nil {
		return fmt.Errorf("consumer ref template validation failed: %w", err)
	}

	// Validate allowances
	if len(spec.Allowances) == 0 {
		return fmt.Errorf("at least one allowance must be specified")
	}

	// Validate each allowance (including resource type validation)
	for i, allowance := range spec.Allowances {
		if allowance.ResourceType == "" {
			return fmt.Errorf("allowance %d: resource type cannot be empty", i)
		}

		// Validate resource type against ResourceRegistrations
		if err := v.resourceTypeValidator.ValidateResourceType(ctx, allowance.ResourceType); err != nil {
			return fmt.Errorf("allowance %d resource type validation failed: %w", i, err)
		}

		// Validate allowance template structure
		if err := v.ValidateAllowanceTemplate(allowance); err != nil {
			return fmt.Errorf("allowance %d validation failed: %w", i, err)
		}
	}

	return nil
}

// ValidateConsumerRefTemplate validates the consumer reference template.
func (v *GrantTemplateValidator) ValidateConsumerRefTemplate(consumerRef quotav1alpha1.ConsumerRef) error {
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
func (v *GrantTemplateValidator) ValidateAllowanceTemplate(allowance quotav1alpha1.Allowance) error {
	// Validate resource type is not empty
	if allowance.ResourceType == "" {
		return fmt.Errorf("resource type cannot be empty")
	}

	// Note: Resource type registration validation is handled separately by the controller
	// through validateResourceTypes() which checks against ResourceRegistrations

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
func (v *GrantTemplateValidator) ValidateBucketTemplate(bucket quotav1alpha1.Bucket) error {
	// Validate amount (must be non-negative)
	if bucket.Amount < 0 {
		return fmt.Errorf("amount cannot be negative: %d", bucket.Amount)
	}

	return nil
}

// ValidateNameTemplate validates a name template.
func (v *GrantTemplateValidator) ValidateNameTemplate(nameTemplate string) error {
	if nameTemplate == "" {
		return fmt.Errorf("name template cannot be empty")
	}

	// If it contains template variables, validate as template
	if v.containsTemplateVariables(nameTemplate) {
		if err := v.ValidateTemplate(nameTemplate); err != nil {
			return fmt.Errorf("template validation failed: %w", err)
		}
	} else {
		// If it doesn't contain template variables, validate as Kubernetes name
		if err := v.validateKubernetesName(nameTemplate); err != nil {
			return fmt.Errorf("Kubernetes name validation failed: %w", err)
		}
	}

	return nil
}

// ValidateConsumerRefNameTemplate validates a consumer reference name template.
// This is more permissive than regular name templates since consumer refs reference existing resources.
func (v *GrantTemplateValidator) ValidateConsumerRefNameTemplate(nameTemplate string) error {
	if nameTemplate == "" {
		return fmt.Errorf("name template cannot be empty")
	}

	// If it contains template variables, validate as template
	if v.containsTemplateVariables(nameTemplate) {
		if err := v.ValidateTemplate(nameTemplate); err != nil {
			return fmt.Errorf("template validation failed: %w", err)
		}
	} else {
		// If it doesn't contain template variables, validate as Kubernetes name
		if err := v.validateKubernetesName(nameTemplate); err != nil {
			return fmt.Errorf("Kubernetes name validation failed: %w", err)
		}
	}

	return nil
}

// ValidateTemplate validates a Go template string.
func (v *GrantTemplateValidator) ValidateTemplate(templateStr string) error {
	if templateStr == "" {
		return nil
	}

	// Parse the template to check for syntax errors
	tmpl, err := template.New("validation").Parse(templateStr)
	if err != nil {
		return fmt.Errorf("template syntax error: %w", err)
	}

	// Extract and validate template variables
	variables := v.extractTemplateVariables(templateStr)
	for _, variable := range variables {
		if !v.validVariables[variable] {
			validVars := v.getValidVariablesList()
			return fmt.Errorf("invalid template variable '%s', valid variables are: %v", variable, validVars)
		}
	}

	// Execute template with dummy data to check for runtime errors
	dummyData := map[string]interface{}{
		"trigger": map[string]interface{}{
			"metadata": map[string]interface{}{
				"name":      "test-name",
				"namespace": "test-namespace",
			},
			"spec": map[string]interface{}{},
		},
	}

	var result strings.Builder
	err = tmpl.Execute(&result, dummyData)
	if err != nil {
		return fmt.Errorf("template execution error: %w", err)
	}

	return nil
}

// containsTemplateVariables checks if a string contains Go template variables.
func (v *GrantTemplateValidator) containsTemplateVariables(str string) bool {
	return strings.Contains(str, "{{") && strings.Contains(str, "}}")
}

// extractTemplateVariables extracts template variables from a template string.
func (v *GrantTemplateValidator) extractTemplateVariables(templateStr string) []string {
	// Simple extraction - looks for {{.variableName}} patterns
	re := regexp.MustCompile(`\{\{\s*\.(\w+)`)
	matches := re.FindAllStringSubmatch(templateStr, -1)

	var variables []string
	seen := make(map[string]bool)
	for _, match := range matches {
		if len(match) > 1 {
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
func (v *GrantTemplateValidator) getValidVariablesList() []string {
	var variables []string
	for variable := range v.validVariables {
		variables = append(variables, variable)
	}
	return variables
}

// validateKubernetesName validates a Kubernetes name.
func (v *GrantTemplateValidator) validateKubernetesName(name string) error {
	if len(name) == 0 {
		return fmt.Errorf("name cannot be empty")
	}
	if len(name) > 253 {
		return fmt.Errorf("name cannot be longer than 253 characters")
	}

	// Kubernetes names must be lowercase alphanumeric with hyphens
	re := regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
	if !re.MatchString(name) {
		return fmt.Errorf("name must consist of lowercase alphanumeric characters or '-', and must start and end with an alphanumeric character")
	}

	return nil
}

// validateLabelKey validates a label key using official Kubernetes validation.
func (v *GrantTemplateValidator) validateLabelKey(key string) error {
	if len(key) == 0 {
		return fmt.Errorf("label key cannot be empty")
	}

	// Kubernetes metadata expects labels to support the prefix/name format (e.g.,
	// "quota.miloapis.com/auto-created")
	if errs := validation.IsQualifiedName(key); len(errs) > 0 {
		return fmt.Errorf("label key must be a valid qualified name: %s", strings.Join(errs, "; "))
	}

	return nil
}

// validateLabelValue validates a Kubernetes label value using official Kubernetes validation.
func (v *GrantTemplateValidator) validateLabelValue(value string) error {
	// Use Kubernetes official validation for label values
	if errs := validation.IsValidLabelValue(value); len(errs) > 0 {
		return fmt.Errorf("label value must be valid: %s", strings.Join(errs, "; "))
	}

	return nil
}

// validateAnnotationKey validates a Kubernetes annotation key.
func (v *GrantTemplateValidator) validateAnnotationKey(key string) error {
	// Similar to label key validation but more permissive
	return v.validateLabelKey(key)
}

// validateAPIGroup validates a Kubernetes API group.
func (v *GrantTemplateValidator) validateAPIGroup(apiGroup string) error {
	if len(apiGroup) == 0 {
		// Empty API group is valid (core group)
		return nil
	}
	if len(apiGroup) > 253 {
		return fmt.Errorf("API group cannot be longer than 253 characters")
	}

	// API groups are DNS names
	re := regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`)
	if !re.MatchString(apiGroup) {
		return fmt.Errorf("API group must be a valid DNS name")
	}

	return nil
}

// validateKind validates a Kubernetes Kind.
func (v *GrantTemplateValidator) validateKind(kind string) error {
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
