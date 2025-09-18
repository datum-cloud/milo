package validation

import (
	"context"
	"fmt"
	"testing"

	quotav1alpha1 "go.miloapis.com/milo/pkg/apis/quota/v1alpha1"
)

// MockResourceTypeValidator for testing
type MockResourceTypeValidator struct {
	validResourceTypes map[string]bool
}

func (m *MockResourceTypeValidator) ValidateResourceType(ctx context.Context, resourceType string) error {
	if m.validResourceTypes[resourceType] {
		return nil
	}
	return fmt.Errorf("resource type '%s' is not registered", resourceType)
}

func (m *MockResourceTypeValidator) IsClaimingResourceAllowed(ctx context.Context, resourceType string, consumerRef quotav1alpha1.ConsumerRef, claimingAPIGroup, claimingKind string) (bool, []string, error) {
	// Simple mock implementation - allow all claiming resources for valid resource types
	if m.validResourceTypes[resourceType] {
		return true, []string{fmt.Sprintf("%s/%s", claimingAPIGroup, claimingKind)}, nil
	}
	return false, nil, fmt.Errorf("resource type '%s' is not registered", resourceType)
}

func (m *MockResourceTypeValidator) IsReady() bool {
	// Mock is always ready for testing
	return true
}

func TestValidateLabelKey(t *testing.T) {
	validator := NewGrantTemplateValidator(&MockResourceTypeValidator{
		validResourceTypes: map[string]bool{
			"test.example.com/projects": true,
		},
	})

	tests := []struct {
		name        string
		key         string
		expectError bool
		description string
	}{
		{
			name:        "valid simple key",
			key:         "environment",
			expectError: false,
			description: "Simple alphanumeric label key should be valid",
		},
		{
			name:        "valid key with hyphens",
			key:         "app-name",
			expectError: false,
			description: "Label key with hyphens should be valid",
		},
		{
			name:        "valid key with dots",
			key:         "app.name",
			expectError: false,
			description: "Label key with dots should be valid",
		},
		{
			name:        "valid key with underscores",
			key:         "app_name",
			expectError: false,
			description: "Label key with underscores should be valid",
		},
		{
			name:        "valid prefixed key with slash",
			key:         "quota.miloapis.com/auto-created",
			expectError: false,
			description: "Prefixed label key with forward slash should be valid (this was the bug)",
		},
		{
			name:        "valid kubernetes.io prefix",
			key:         "kubernetes.io/arch",
			expectError: false,
			description: "Kubernetes.io prefixed label should be valid",
		},
		{
			name:        "valid app.kubernetes.io prefix",
			key:         "app.kubernetes.io/name",
			expectError: false,
			description: "app.kubernetes.io prefixed label should be valid",
		},
		{
			name:        "valid prefixed key with subdomain",
			key:         "example.com/component",
			expectError: false,
			description: "Prefixed label with subdomain should be valid",
		},
		{
			name:        "empty key",
			key:         "",
			expectError: true,
			description: "Empty label key should be invalid",
		},
		{
			name:        "key starting with hyphen",
			key:         "-invalid",
			expectError: true,
			description: "Label key starting with hyphen should be invalid",
		},
		{
			name:        "key ending with hyphen",
			key:         "invalid-",
			expectError: true,
			description: "Label key ending with hyphen should be invalid",
		},
		{
			name:        "key with invalid characters",
			key:         "invalid@key",
			expectError: true,
			description: "Label key with @ symbol should be invalid",
		},
		{
			name:        "key too long",
			key:         "this-is-a-very-long-label-key-that-exceeds-the-maximum-length-allowed-by-kubernetes-and-should-fail-validation",
			expectError: true,
			description: "Label key longer than 63 characters should be invalid",
		},
		{
			name:        "key with multiple slashes",
			key:         "invalid/multiple/slashes",
			expectError: true,
			description: "Label key with multiple slashes should be invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateLabelKey(tt.key)
			if tt.expectError && err == nil {
				t.Errorf("Expected error for key '%s', but got none. %s", tt.key, tt.description)
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for key '%s': %v. %s", tt.key, err, tt.description)
			}
		})
	}
}

func TestValidateLabelValue(t *testing.T) {
	validator := NewGrantTemplateValidator(&MockResourceTypeValidator{
		validResourceTypes: map[string]bool{
			"test.example.com/projects": true,
		},
	})

	tests := []struct {
		name        string
		value       string
		expectError bool
		description string
	}{
		{
			name:        "valid simple value",
			value:       "production",
			expectError: false,
			description: "Simple alphanumeric label value should be valid",
		},
		{
			name:        "valid value with hyphens",
			value:       "my-app",
			expectError: false,
			description: "Label value with hyphens should be valid",
		},
		{
			name:        "valid value with dots",
			value:       "1.0.0",
			expectError: false,
			description: "Label value with dots should be valid",
		},
		{
			name:        "valid value with underscores",
			value:       "my_app",
			expectError: false,
			description: "Label value with underscores should be valid",
		},
		{
			name:        "valid empty value",
			value:       "",
			expectError: false,
			description: "Empty label value should be valid",
		},
		{
			name:        "valid numeric value",
			value:       "123",
			expectError: false,
			description: "Numeric label value should be valid",
		},
		{
			name:        "valid boolean-like value",
			value:       "true",
			expectError: false,
			description: "Boolean-like label value should be valid",
		},
		{
			name:        "value starting with hyphen",
			value:       "-invalid",
			expectError: true,
			description: "Label value starting with hyphen should be invalid",
		},
		{
			name:        "value ending with hyphen",
			value:       "invalid-",
			expectError: true,
			description: "Label value ending with hyphen should be invalid",
		},
		{
			name:        "value with invalid characters",
			value:       "invalid@value",
			expectError: true,
			description: "Label value with @ symbol should be invalid",
		},
		{
			name:        "value too long",
			value:       "this-is-a-very-long-label-value-that-exceeds-the-maximum-length-allowed-by-kubernetes-and-should-fail",
			expectError: true,
			description: "Label value longer than 63 characters should be invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateLabelValue(tt.value)
			if tt.expectError && err == nil {
				t.Errorf("Expected error for value '%s', but got none. %s", tt.value, tt.description)
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for value '%s': %v. %s", tt.value, err, tt.description)
			}
		})
	}
}

func TestValidateMetadataTemplateLabels(t *testing.T) {
	validator := NewGrantTemplateValidator(&MockResourceTypeValidator{
		validResourceTypes: map[string]bool{
			"test.example.com/projects": true,
		},
	})

	tests := []struct {
		name        string
		metadata    quotav1alpha1.ObjectMetaTemplate
		expectError bool
		description string
	}{
		{
			name: "valid labels with prefixed keys",
			metadata: quotav1alpha1.ObjectMetaTemplate{
				Name:      "test-grant",
				Namespace: "test-namespace",
				Labels: map[string]string{
					"quota.miloapis.com/auto-created": "true",
					"quota.miloapis.com/policy":       "test-policy",
					"app.kubernetes.io/name":          "milo",
					"app.kubernetes.io/component":     "quota-system",
					"environment":                     "production",
					"version":                         "1.0.0",
				},
			},
			expectError: false,
			description: "All production-like labels should be valid",
		},
		{
			name: "invalid label key",
			metadata: quotav1alpha1.ObjectMetaTemplate{
				Name:      "test-grant",
				Namespace: "test-namespace",
				Labels: map[string]string{
					"invalid@key": "value",
				},
			},
			expectError: true,
			description: "Label with invalid key should fail validation",
		},
		{
			name: "invalid label value",
			metadata: quotav1alpha1.ObjectMetaTemplate{
				Name:      "test-grant",
				Namespace: "test-namespace",
				Labels: map[string]string{
					"valid-key": "invalid@value",
				},
			},
			expectError: true,
			description: "Label with invalid value should fail validation",
		},
		{
			name: "valid annotations with templating",
			metadata: quotav1alpha1.ObjectMetaTemplate{
				Name:      "test-grant",
				Namespace: "test-namespace",
				Annotations: map[string]string{
					"quota.miloapis.com/description":  "Auto-generated grant for {{.trigger.metadata.name}}",
					"quota.miloapis.com/created-by":   "grant-creation-policy",
					"quota.miloapis.com/organization": "{{.trigger.metadata.name}}",
				},
			},
			expectError: false,
			description: "Valid annotations with template variables should be valid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateMetadataTemplate(tt.metadata)
			if tt.expectError && err == nil {
				t.Errorf("Expected error for metadata, but got none. %s", tt.description)
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for metadata: %v. %s", err, tt.description)
			}
		})
	}
}
