package templating

import (
	"testing"

	"github.com/stretchr/testify/require"
	notificationv1alpha1 "go.miloapis.com/milo/pkg/apis/notification/v1alpha1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func TestValidateTextTemplate(t *testing.T) {
	t.Parallel()

	type wantError struct {
		errType  field.ErrorType
		contains string
	}

	tests := []struct {
		name       string
		textBody   string
		declared   []notificationv1alpha1.TemplateVariable
		wantErrLen int
		wantErrs   []wantError
	}{
		{
			name:     "valid template with declared required variable",
			textBody: "Hello {{ .UserName }}",
			declared: []notificationv1alpha1.TemplateVariable{{
				Name:     "UserName",
				Required: true,
				Type:     notificationv1alpha1.EmailTemplateVariableTypeString,
			}},
			wantErrLen: 0,
		},
		{
			name:     "invalid Go template",
			textBody: "{{ .UserName ", // missing closing braces
			declared: []notificationv1alpha1.TemplateVariable{{
				Name:     "UserName",
				Required: true,
				Type:     notificationv1alpha1.EmailTemplateVariableTypeString,
			}},
			wantErrLen: 1,
			wantErrs: []wantError{{
				errType:  field.ErrorTypeInvalid,
				contains: "textBody is not a valid Go template",
			}},
		},
		{
			name:     "invalid Go template (missing opening braces)",
			textBody: ".UserName }}", // malformed template, missing "{{"
			declared: []notificationv1alpha1.TemplateVariable{{
				Name:     "UserName",
				Required: true,
				Type:     notificationv1alpha1.EmailTemplateVariableTypeString,
			}},
			wantErrLen: 1,
			wantErrs: []wantError{
				{
					errType:  field.ErrorTypeRequired,
					contains: "required variable \"UserName\" is not referenced",
				},
			},
		},
		{
			name:       "undeclared variable referenced",
			textBody:   "Hello {{ .FirstName }}",
			declared:   nil, // no declarations
			wantErrLen: 1,
			wantErrs: []wantError{{
				errType:  field.ErrorTypeNotSupported,
				contains: "declared variables",
			}},
		},
		{
			name:     "required variable not used",
			textBody: "Hello World",
			declared: []notificationv1alpha1.TemplateVariable{{
				Name:     "Greeting",
				Required: true,
				Type:     notificationv1alpha1.EmailTemplateVariableTypeString,
			}},
			wantErrLen: 1,
			wantErrs: []wantError{{
				errType:  field.ErrorTypeRequired,
				contains: "required variable \"Greeting\" is not referenced",
			}},
		},
		{
			name:     "three variables - all referenced, mix of required and optional",
			textBody: "Hello {{ .FirstName }} {{ .LastName }}. Visit {{ .ProfileURL }}",
			declared: []notificationv1alpha1.TemplateVariable{
				{Name: "FirstName", Required: true, Type: notificationv1alpha1.EmailTemplateVariableTypeString},
				{Name: "LastName", Required: true, Type: notificationv1alpha1.EmailTemplateVariableTypeString},
				{Name: "ProfileURL", Required: false, Type: notificationv1alpha1.EmailTemplateVariableTypeURL},
			},
			wantErrLen: 0,
		},
		{
			name:     "three variables - optional not referenced, required referenced",
			textBody: "Hello {{ .FirstName }} {{ .LastName }}",
			declared: []notificationv1alpha1.TemplateVariable{
				{Name: "FirstName", Required: true, Type: notificationv1alpha1.EmailTemplateVariableTypeString},
				{Name: "LastName", Required: true, Type: notificationv1alpha1.EmailTemplateVariableTypeString},
				{Name: "ProfileURL", Required: false, Type: notificationv1alpha1.EmailTemplateVariableTypeURL},
			},
			wantErrLen: 0,
		},
		{
			name:     "three variables - required variable missing",
			textBody: "Hello {{ .FirstName }}. Optional link: {{ .ProfileURL }}",
			declared: []notificationv1alpha1.TemplateVariable{
				{Name: "FirstName", Required: true, Type: notificationv1alpha1.EmailTemplateVariableTypeString},
				{Name: "LastName", Required: true, Type: notificationv1alpha1.EmailTemplateVariableTypeString},
				{Name: "ProfileURL", Required: false, Type: notificationv1alpha1.EmailTemplateVariableTypeURL},
			},
			wantErrLen: 1,
			wantErrs: []wantError{
				{
					errType:  field.ErrorTypeRequired,
					contains: "required variable \"LastName\" is not referenced",
				},
			},
		},
		{
			name:     "two required variables missing, two variables not referenced",
			textBody: "Hello there! {{ .GivenName }} {{ .FamilyName }}",
			declared: []notificationv1alpha1.TemplateVariable{
				{Name: "FirstName", Required: true, Type: notificationv1alpha1.EmailTemplateVariableTypeString},
				{Name: "LastName", Required: true, Type: notificationv1alpha1.EmailTemplateVariableTypeString},
			},
			wantErrLen: 4, // Expect two required-variable errors plus two undeclared-variable errors
		},
		{
			name:       "two variables referenced but undeclared",
			textBody:   "Hello {{ .Foo }} {{ .Bar }}",
			declared:   nil,
			wantErrLen: 2,
		},
	}

	for _, tt := range tests {
		tt := tt // capture range variable
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			errs := ValidateTextTemplate(tt.textBody, tt.declared)
			require.Len(t, errs, tt.wantErrLen, "unexpected number of validation errors: %v", errs)

			// Ensure each expected error type / message appears
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
