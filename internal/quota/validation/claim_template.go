package validation

import (
	"fmt"
	"strings"
	"text/template"

	quotav1alpha1 "go.miloapis.com/milo/pkg/apis/quota/v1alpha1"
)

// ClaimTemplateValidator provides validation for Go templates used in ClaimCreationPolicy metadata.
type ClaimTemplateValidator struct{}

// ValidateClaimTemplate validates the ResourceClaim template structure and template syntax.
// It enforces name/generateName mutual exclusivity and parses templated fields for syntax errors.
func (v *ClaimTemplateValidator) ValidateClaimTemplate(t quotav1alpha1.ResourceClaimTemplate) error {
	// name vs generateName mutual exclusivity
	nameSet := strings.TrimSpace(t.Metadata.Name) != ""
	genSet := strings.TrimSpace(t.Metadata.GenerateName) != ""
	if nameSet && genSet {
		return fmt.Errorf("metadata.name and metadata.generateName are mutually exclusive")
	}

	// Validate templated fields: name, generateName, namespace
	if err := parseTemplateIfNeeded(t.Metadata.Name); err != nil {
		return fmt.Errorf("invalid metadata.name template: %w", err)
	}
	if err := parseTemplateIfNeeded(t.Metadata.GenerateName); err != nil {
		return fmt.Errorf("invalid metadata.generateName template: %w", err)
	}
	if err := parseTemplateIfNeeded(t.Metadata.Namespace); err != nil {
		return fmt.Errorf("invalid metadata.namespace template: %w", err)
	}

	// Validate annotations (values are templated)
	for k, vStr := range t.Metadata.Annotations {
		if err := parseTemplateIfNeeded(vStr); err != nil {
			return fmt.Errorf("invalid annotation template for key %q: %w", k, err)
		}
	}

	return nil
}

func parseTemplateIfNeeded(s string) error {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	if !strings.Contains(s, "{{") {
		return nil
	}
	_, err := template.New("tmpl").Parse(s)
	return err
}

