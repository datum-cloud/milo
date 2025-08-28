package v1alpha1

import (
	"context"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	notificationv1alpha1 "go.miloapis.com/milo/pkg/apis/notification/v1alpha1"
)

func TestEmailTemplateValidator_ValidateCreate(t *testing.T) {
	validator := &EmailTemplateValidator{}

	tests := []struct {
		name      string
		htmlBody  string
		textBody  string
		variables []notificationv1alpha1.TemplateVariable
		wantValid bool
		wantErrs  int // expected number of field errors when invalid
	}{
		{
			name:     "valid template",
			htmlBody: `<p>Hello {{.UserName}}</p>`,
			textBody: `Hello {{.UserName}}`,
			variables: []notificationv1alpha1.TemplateVariable{{
				Name:     "UserName",
				Required: true,
				Type:     notificationv1alpha1.EmailTemplateVariableTypeString,
			}},
			wantValid: true,
		},
		{
			name:     "html parse error",
			htmlBody: `<p>Hello {{.UserName}</p>`, // missing closing }}
			textBody: `Hello {{.UserName}}`,
			variables: []notificationv1alpha1.TemplateVariable{{
				Name:     "UserName",
				Required: true,
				Type:     notificationv1alpha1.EmailTemplateVariableTypeString,
			}},
			wantValid: false,
			wantErrs:  1,
		},
		{
			name:      "undeclared variable in html",
			htmlBody:  `<p>Hello {{.UserName}}</p>`,
			textBody:  `Hi there`,
			variables: nil,
			wantValid: false,
			wantErrs:  1,
		},
		{
			name:     "required variable not used",
			htmlBody: `<p>Hello world</p>`,
			textBody: `Hi there`,
			variables: []notificationv1alpha1.TemplateVariable{{
				Name:     "UserName",
				Required: true,
				Type:     notificationv1alpha1.EmailTemplateVariableTypeString,
			}},
			wantValid: false,
			wantErrs:  2, // one for each body
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl := buildEmailTemplate(tt.htmlBody, tt.textBody, tt.variables)
			_, err := validator.ValidateCreate(context.Background(), tmpl)
			if tt.wantValid {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
			} else {
				if err == nil {
					t.Fatalf("expected validation error, got nil")
				}
				if !apierrors.IsInvalid(err) {
					t.Fatalf("expected invalid error, got %v", err)
				}
				statusErr := err.(*apierrors.StatusError)
				if len(statusErr.ErrStatus.Details.Causes) != tt.wantErrs {
					t.Fatalf("expected %d causes, got %d: %v", tt.wantErrs, len(statusErr.ErrStatus.Details.Causes), statusErr.ErrStatus.Details.Causes)
				}
			}
		})
	}
}

func buildEmailTemplate(html, text string, vars []notificationv1alpha1.TemplateVariable) *notificationv1alpha1.EmailTemplate {
	return &notificationv1alpha1.EmailTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name: "sample",
		},
		Spec: notificationv1alpha1.EmailTemplateSpec{
			Subject:   "Subject",
			HTMLBody:  html,
			TextBody:  text,
			Variables: vars,
		},
	}
}
