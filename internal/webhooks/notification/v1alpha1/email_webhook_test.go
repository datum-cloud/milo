package v1alpha1

import (
	"context"
	"strings"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	iamv1alpha1 "go.miloapis.com/milo/pkg/apis/iam/v1alpha1"
	notificationv1alpha1 "go.miloapis.com/milo/pkg/apis/notification/v1alpha1"
)

var emailRuntimeScheme = runtime.NewScheme()

func init() {
	utilruntime.Must(iamv1alpha1.AddToScheme(emailRuntimeScheme))
	utilruntime.Must(notificationv1alpha1.AddToScheme(emailRuntimeScheme))
}

func TestEmailValidator_ValidateCreate(t *testing.T) {
	// Common objects
	user := &iamv1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{Name: "john"},
		Spec:       iamv1alpha1.UserSpec{Email: "[email protected]"},
	}

	tmpl := &notificationv1alpha1.EmailTemplate{
		ObjectMeta: metav1.ObjectMeta{Name: "welcome"},
		Spec: notificationv1alpha1.EmailTemplateSpec{
			Subject:  "Welcome",
			HTMLBody: "<p>Hello {{.FirstName}}</p>",
			TextBody: "Hello {{.FirstName}}",
			Variables: []notificationv1alpha1.TemplateVariable{
				{Name: "FirstName", Required: true, Type: notificationv1alpha1.EmailTemplateVariableTypeString},
			},
		},
	}

	validEmail := &notificationv1alpha1.Email{
		ObjectMeta: metav1.ObjectMeta{Name: "welcome-john"},
		Spec: notificationv1alpha1.EmailSpec{
			TemplateRef: notificationv1alpha1.TemplateReference{Name: tmpl.Name},
			UserRef:     notificationv1alpha1.EmailUserReference{Name: user.Name},
			Variables: []notificationv1alpha1.EmailVariable{
				{Name: "FirstName", Value: "John"},
			},
		},
	}

	tests := map[string]struct {
		includeUser   bool
		includeTmpl   bool
		email         *notificationv1alpha1.Email
		expectErr     bool
		errorContains string
	}{
		"valid create": {
			includeUser: true, includeTmpl: true, email: validEmail, expectErr: false,
		},
		"missing user": {
			includeUser: false, includeTmpl: true, email: validEmail, expectErr: true, errorContains: "Not found",
		},
		"missing template": {
			includeUser: true, includeTmpl: false, email: validEmail, expectErr: true, errorContains: "templateRef",
		},
		"variable error": {
			includeUser: true, includeTmpl: true, email: func() *notificationv1alpha1.Email {
				e := validEmail.DeepCopy()
				e.Spec.Variables = []notificationv1alpha1.EmailVariable{ // missing required FirstName
					{Name: "Extra", Value: "foo"},
				}
				return e
			}(), expectErr: true, errorContains: "required variable",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			builder := fake.NewClientBuilder().WithScheme(emailRuntimeScheme)
			if tt.includeUser {
				builder = builder.WithObjects(user)
			}
			if tt.includeTmpl {
				builder = builder.WithObjects(tmpl)
			}
			fakeClient := builder.Build()

			validator := &EmailValidator{Client: fakeClient}
			_, err := validator.ValidateCreate(context.Background(), tt.email)
			if tt.expectErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tt.errorContains != "" && !contains(err.Error(), tt.errorContains) {
					t.Fatalf("error message %q does not contain %q", err.Error(), tt.errorContains)
				}
				if !apierrors.IsInvalid(err) {
					t.Fatalf("expected invalid error, got %v", err)
				}
			} else {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
			}
		})
	}
}

func TestEmailValidator_ValidateUpdateDelete(t *testing.T) {
	fakeClient := fake.NewClientBuilder().WithScheme(emailRuntimeScheme).Build()
	validator := &EmailValidator{Client: fakeClient}

	email := &notificationv1alpha1.Email{ObjectMeta: metav1.ObjectMeta{Name: "sample"}}

	// Update should be rejected
	_, err := validator.ValidateUpdate(context.Background(), email, email)
	if err == nil || !apierrors.IsMethodNotSupported(err) {
		t.Fatalf("expected MethodNotSupported error on update, got %v", err)
	}
	if err.Error() != "update is not supported on resources of kind \"emails.notification.miloapis.com\"" {
		t.Fatalf("expected error message 'updates to Email resources are not allowed', got %v", err)
	}

	// Delete should be rejected
	_, err = validator.ValidateDelete(context.Background(), email)
	if err == nil || !apierrors.IsMethodNotSupported(err) {
		t.Fatalf("expected MethodNotSupported error on delete, got %v", err)
	}
	if err.Error() != "delete is not supported on resources of kind \"emails.notification.miloapis.com\"" {
		t.Fatalf("expected error message 'updates to Email resources are not allowed', got %v", err)
	}
}

// helper contains substring (avoid strings import collision)
func contains(s, substr string) bool { return strings.Contains(s, substr) }
