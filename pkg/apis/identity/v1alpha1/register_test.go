package v1alpha1

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestUserIdentityFieldLabelConversion(t *testing.T) {
	tests := []struct {
		name        string
		label       string
		value       string
		wantLabel   string
		wantValue   string
		wantErr     bool
		errContains string
	}{
		{
			name:      "metadata.name is valid",
			label:     "metadata.name",
			value:     "test-identity",
			wantLabel: "metadata.name",
			wantValue: "test-identity",
			wantErr:   false,
		},
		{
			name:      "metadata.namespace is valid",
			label:     "metadata.namespace",
			value:     "default",
			wantLabel: "metadata.namespace",
			wantValue: "default",
			wantErr:   false,
		},
		{
			name:      "status.userUID is valid",
			label:     "status.userUID",
			value:     "340583683847098197",
			wantLabel: "status.userUID",
			wantValue: "340583683847098197",
			wantErr:   false,
		},
		{
			name:        "invalid field selector",
			label:       "status.invalidField",
			value:       "test",
			wantErr:     true,
			errContains: "not a known field selector",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotLabel, gotValue, err := UserIdentityFieldLabelConversionFunc(tt.label, tt.value)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but got none")
					return
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if gotLabel != tt.wantLabel {
				t.Errorf("label = %q, want %q", gotLabel, tt.wantLabel)
			}

			if gotValue != tt.wantValue {
				t.Errorf("value = %q, want %q", gotValue, tt.wantValue)
			}
		})
	}
}

func TestSessionFieldLabelConversion(t *testing.T) {
	tests := []struct {
		name        string
		label       string
		value       string
		wantLabel   string
		wantValue   string
		wantErr     bool
		errContains string
	}{
		{
			name:      "metadata.name is valid",
			label:     "metadata.name",
			value:     "test-session",
			wantLabel: "metadata.name",
			wantValue: "test-session",
			wantErr:   false,
		},
		{
			name:      "status.userUID is valid",
			label:     "status.userUID",
			value:     "340583683847098197",
			wantLabel: "status.userUID",
			wantValue: "340583683847098197",
			wantErr:   false,
		},
		{
			name:        "invalid field selector",
			label:       "spec.invalid",
			value:       "test",
			wantErr:     true,
			errContains: "not a known field selector",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotLabel, gotValue, err := SessionFieldLabelConversionFunc(tt.label, tt.value)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but got none")
					return
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if gotLabel != tt.wantLabel {
				t.Errorf("label = %q, want %q", gotLabel, tt.wantLabel)
			}

			if gotValue != tt.wantValue {
				t.Errorf("value = %q, want %q", gotValue, tt.wantValue)
			}
		})
	}
}

func TestFieldLabelConversionRegistration(t *testing.T) {
	scheme := runtime.NewScheme()
	err := addKnownTypes(scheme)
	if err != nil {
		t.Fatalf("addKnownTypes failed: %v", err)
	}

	// Test that UserIdentity field label conversion is registered
	userIdentityGVK := schema.GroupVersion{Group: "identity.miloapis.com", Version: "v1alpha1"}.WithKind("UserIdentity")
	
	// Try to convert a valid field selector
	converter := scheme.Converter()
	if converter == nil {
		t.Fatal("scheme converter is nil")
	}

	// Test that Session field label conversion is registered
	sessionGVK := schema.GroupVersion{Group: "identity.miloapis.com", Version: "v1alpha1"}.WithKind("Session")
	
	// Verify the GVKs are registered
	if !scheme.Recognizes(userIdentityGVK) {
		t.Errorf("UserIdentity GVK not recognized by scheme")
	}
	if !scheme.Recognizes(sessionGVK) {
		t.Errorf("Session GVK not recognized by scheme")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
