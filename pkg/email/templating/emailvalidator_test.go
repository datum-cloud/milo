package templating

import (
	"testing"

	"github.com/stretchr/testify/require"
	notificationv1alpha1 "go.miloapis.com/milo/pkg/apis/notification/v1alpha1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func TestValidateEmailVariables(t *testing.T) {
	t.Parallel()

	type wantError struct {
		errType  field.ErrorType
		contains string
	}

	var (
		tmplFirstName = notificationv1alpha1.TemplateVariable{Name: "FirstName", Required: true, Type: notificationv1alpha1.EmailTemplateVariableTypeString}
		tmplLastName  = notificationv1alpha1.TemplateVariable{Name: "LastName", Required: false, Type: notificationv1alpha1.EmailTemplateVariableTypeString}
		tmplProfile   = notificationv1alpha1.TemplateVariable{Name: "ProfileURL", Required: false, Type: notificationv1alpha1.EmailTemplateVariableTypeURL}

		emailFirstName = notificationv1alpha1.EmailVariable{Name: "FirstName", Value: "John"}
		emailLastName  = notificationv1alpha1.EmailVariable{Name: "LastName", Value: "Doe"}
	)

	tests := []struct {
		name         string
		templateVars []notificationv1alpha1.TemplateVariable
		emailVars    []notificationv1alpha1.EmailVariable
		wantErrLen   int
		wantErrs     []wantError
	}{
		{
			name: "valid variables (required & optional, https url)",
			templateVars: []notificationv1alpha1.TemplateVariable{
				tmplFirstName,
				tmplLastName,
				tmplProfile,
			},
			emailVars: []notificationv1alpha1.EmailVariable{
				emailFirstName,
				emailLastName,
				{Name: "ProfileURL", Value: "https://example.com"},
			},
			wantErrLen: 0,
		},
		{
			name: "missing required variable",
			templateVars: []notificationv1alpha1.TemplateVariable{
				tmplFirstName,
				tmplLastName,
			},
			emailVars: []notificationv1alpha1.EmailVariable{
				emailLastName,
			},
			wantErrLen: 1,
			wantErrs:   []wantError{{errType: field.ErrorTypeRequired, contains: "required variable \"FirstName\""}},
		},
		{
			name: "undeclared variable provided",
			templateVars: []notificationv1alpha1.TemplateVariable{
				tmplFirstName,
				tmplLastName,
			},
			emailVars: []notificationv1alpha1.EmailVariable{
				emailFirstName,
				{Name: "Extra", Value: "foo"},
			},
			wantErrLen: 1,
			wantErrs:   []wantError{{errType: field.ErrorTypeNotSupported, contains: "declared variables"}},
		},
		{
			name: "duplicate variable name",
			templateVars: []notificationv1alpha1.TemplateVariable{
				tmplFirstName,
				tmplLastName,
			},
			emailVars: []notificationv1alpha1.EmailVariable{
				emailFirstName,
				{Name: "FirstName", Value: "Johnny"}, // duplicate
				emailLastName,
			},
			wantErrLen: 1,
			wantErrs:   []wantError{{errType: field.ErrorTypeDuplicate, contains: "FirstName"}},
		},
		{
			name: "url not https",
			templateVars: []notificationv1alpha1.TemplateVariable{
				tmplProfile,
				tmplFirstName,
			},
			emailVars: []notificationv1alpha1.EmailVariable{
				emailFirstName,
				{Name: "ProfileURL", Value: "http://example.com"},
			},
			wantErrLen: 1,
			wantErrs:   []wantError{{errType: field.ErrorTypeInvalid, contains: "https"}},
		},
		{
			name: "invalid url format",
			templateVars: []notificationv1alpha1.TemplateVariable{
				tmplProfile,
				tmplFirstName,
			},
			emailVars: []notificationv1alpha1.EmailVariable{
				emailFirstName,
				{Name: "ProfileURL", Value: "//bad-url"},
			},
			wantErrLen: 1,
			wantErrs:   []wantError{{errType: field.ErrorTypeInvalid, contains: "https"}},
		},
		{
			name: "three simultaneous errors (duplicate, invalid https, undeclared)",
			templateVars: []notificationv1alpha1.TemplateVariable{
				tmplFirstName,
				{Name: "ProfileURL", Required: true, Type: notificationv1alpha1.EmailTemplateVariableTypeURL},
			},
			emailVars: []notificationv1alpha1.EmailVariable{
				emailFirstName,
				{Name: "FirstName", Value: "Johnny"},              // duplicate -> Duplicate error
				{Name: "ProfileURL", Value: "http://example.com"}, // invalid scheme -> Invalid error
				{Name: "Extra", Value: "foo"},                     // undeclared -> NotSupported error
			},
			wantErrLen: 3,
			wantErrs: []wantError{
				{errType: field.ErrorTypeDuplicate, contains: "FirstName"},
				{errType: field.ErrorTypeInvalid, contains: "https"},
				{errType: field.ErrorTypeNotSupported, contains: "declared variables"},
			},
		},
	}

	for _, tt := range tests {
		tt := tt // capture range var
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tmpl := notificationv1alpha1.EmailTemplate{
				Spec: notificationv1alpha1.EmailTemplateSpec{Variables: tt.templateVars},
			}
			email := notificationv1alpha1.Email{
				Spec: notificationv1alpha1.EmailSpec{Variables: tt.emailVars},
			}

			errs := ValidateEmailVariables(&email, &tmpl)
			require.Len(t, errs, tt.wantErrLen, "unexpected number of validation errors: %v", errs)

			for i, we := range tt.wantErrs {
				require.Truef(t, len(errs) > i, "missing expected error #%d", i)
				require.Equal(t, we.errType, errs[i].Type, "error type mismatch for error %d", i)
				if we.contains != "" {
					require.Contains(t, errs[i].Error(), we.contains, "error message mismatch for error %d", i)
				}
			}
		})
	}
}
