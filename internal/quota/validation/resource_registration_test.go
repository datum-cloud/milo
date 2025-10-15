package validation

import (
	"testing"

	quotav1alpha1 "go.miloapis.com/milo/pkg/apis/quota/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestResourceRegistrationValidator_Validate(t *testing.T) {
	tests := []struct {
		name         string
		registration *quotav1alpha1.ResourceRegistration
		wantErrs     bool
		errContains  string
	}{
		{
			name: "valid registration with single claiming resource",
			registration: &quotav1alpha1.ResourceRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-registration",
				},
				Spec: quotav1alpha1.ResourceRegistrationSpec{
					ResourceType: "test-resources",
					ConsumerType: quotav1alpha1.ConsumerType{
						APIGroup: "resourcemanager.miloapis.com",
						Kind:     "Organization",
					},
					ClaimingResources: []quotav1alpha1.ClaimingResource{
						{
							APIGroup: "resourcemanager.miloapis.com",
							Kind:     "Project",
						},
					},
				},
			},
			wantErrs: false,
		},
		{
			name: "valid registration with multiple unique claiming resources",
			registration: &quotav1alpha1.ResourceRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-registration",
				},
				Spec: quotav1alpha1.ResourceRegistrationSpec{
					ResourceType: "test-resources",
					ConsumerType: quotav1alpha1.ConsumerType{
						APIGroup: "resourcemanager.miloapis.com",
						Kind:     "Organization",
					},
					ClaimingResources: []quotav1alpha1.ClaimingResource{
						{
							APIGroup: "resourcemanager.miloapis.com",
							Kind:     "Project",
						},
						{
							APIGroup: "resourcemanager.miloapis.com",
							Kind:     "Workspace",
						},
					},
				},
			},
			wantErrs: false,
		},
		{
			name: "invalid registration with duplicate claiming resources",
			registration: &quotav1alpha1.ResourceRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-registration",
				},
				Spec: quotav1alpha1.ResourceRegistrationSpec{
					ResourceType: "test-resources",
					ConsumerType: quotav1alpha1.ConsumerType{
						APIGroup: "resourcemanager.miloapis.com",
						Kind:     "Organization",
					},
					ClaimingResources: []quotav1alpha1.ClaimingResource{
						{
							APIGroup: "resourcemanager.miloapis.com",
							Kind:     "Project",
						},
						{
							APIGroup: "resourcemanager.miloapis.com",
							Kind:     "Project",
						},
					},
				},
			},
			wantErrs:    true,
			errContains: "duplicate claiming resource",
		},
		{
			name: "invalid registration with duplicate claiming resources at different indices",
			registration: &quotav1alpha1.ResourceRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-registration",
				},
				Spec: quotav1alpha1.ResourceRegistrationSpec{
					ResourceType: "test-resources",
					ConsumerType: quotav1alpha1.ConsumerType{
						APIGroup: "resourcemanager.miloapis.com",
						Kind:     "Organization",
					},
					ClaimingResources: []quotav1alpha1.ClaimingResource{
						{
							APIGroup: "resourcemanager.miloapis.com",
							Kind:     "Project",
						},
						{
							APIGroup: "resourcemanager.miloapis.com",
							Kind:     "Workspace",
						},
						{
							APIGroup: "resourcemanager.miloapis.com",
							Kind:     "Project",
						},
					},
				},
			},
			wantErrs:    true,
			errContains: "first occurrence at index 0",
		},
	}

	validator := NewResourceRegistrationValidator()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := validator.Validate(tt.registration)

			if tt.wantErrs && len(errs) == 0 {
				t.Errorf("expected validation errors, got none")
			}

			if !tt.wantErrs && len(errs) > 0 {
				t.Errorf("expected no validation errors, got: %v", errs)
			}

			if tt.errContains != "" && len(errs) > 0 {
				found := false
				errStr := errs.ToAggregate().Error()
				if contains(errStr, tt.errContains) {
					found = true
				}
				if !found {
					t.Errorf("expected error containing %q, got: %v", tt.errContains, errStr)
				}
			}
		})
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
