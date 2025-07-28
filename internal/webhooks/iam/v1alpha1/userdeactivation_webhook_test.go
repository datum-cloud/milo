package v1alpha1

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	iamv1alpha1 "go.miloapis.com/milo/pkg/apis/iam/v1alpha1"
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var runtimeScheme = runtime.NewScheme()

func init() {
	utilruntime.Must(iamv1alpha1.AddToScheme(runtimeScheme))
}

func TestUserDeactivationValidator_ValidateCreate(t *testing.T) {
	const (
		systemNamespace = "milo-system"
	)

	// Common user object used in successful validation case
	testUser := &iamv1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-user",
		},
		Spec: iamv1alpha1.UserSpec{
			Email:      "test@example.com",
			GivenName:  "Test",
			FamilyName: "User",
		},
	}

	tests := map[string]struct {
		userDeactivation *iamv1alpha1.UserDeactivation
		includeUser      bool // whether to include the user in the fake client
		expectError      bool
		errorContains    string
	}{
		"valid when referenced user exists": {
			userDeactivation: &iamv1alpha1.UserDeactivation{
				ObjectMeta: metav1.ObjectMeta{
					Name: "deactivate-test-user",
				},
				Spec: iamv1alpha1.UserDeactivationSpec{
					UserRef: iamv1alpha1.UserReference{
						Name: testUser.Name,
					},
					Reason:        "Testing",
					DeactivatedBy: "tester",
				},
			},
			includeUser: true,
			expectError: false,
		},
		"error when userRef.name is empty": {
			userDeactivation: &iamv1alpha1.UserDeactivation{
				ObjectMeta: metav1.ObjectMeta{
					Name: "deactivate-missing-user",
				},
				Spec: iamv1alpha1.UserDeactivationSpec{
					UserRef: iamv1alpha1.UserReference{
						Name: "",
					},
					Reason:        "Testing",
					DeactivatedBy: "tester",
				},
			},
			includeUser:   false,
			expectError:   true,
			errorContains: "userRef.name is required",
		},
		"error when referenced user does not exist": {
			userDeactivation: &iamv1alpha1.UserDeactivation{
				ObjectMeta: metav1.ObjectMeta{
					Name: "deactivate-nonexistent-user",
				},
				Spec: iamv1alpha1.UserDeactivationSpec{
					UserRef: iamv1alpha1.UserReference{
						Name: "nonexistent-user",
					},
					Reason:        "Testing",
					DeactivatedBy: "tester",
				},
			},
			includeUser:   true,
			expectError:   true,
			errorContains: "referenced user 'nonexistent-user' does not exist",
		},
		"error when deactivatedBy does not match requester": {
			userDeactivation: &iamv1alpha1.UserDeactivation{
				ObjectMeta: metav1.ObjectMeta{Name: "deactivate-test-user-bad"},
				Spec: iamv1alpha1.UserDeactivationSpec{
					UserRef:       iamv1alpha1.UserReference{Name: testUser.Name},
					Reason:        "Testing",
					DeactivatedBy: "other-user",
				},
			},
			includeUser:   true,
			expectError:   true,
			errorContains: "spec.deactivatedBy is managed by the system",
		},
		"error when deactivation already exists for user": {
			userDeactivation: &iamv1alpha1.UserDeactivation{
				ObjectMeta: metav1.ObjectMeta{
					Name: "second-deactivate-test-user",
				},
				Spec: iamv1alpha1.UserDeactivationSpec{
					UserRef:       iamv1alpha1.UserReference{Name: testUser.Name},
					Reason:        "Testing duplicate",
					DeactivatedBy: "tester",
				},
			},
			includeUser:   true,
			expectError:   true,
			errorContains: "UserDeactivation already exists",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Build fake client with required objects for the test case
			builder := fake.NewClientBuilder().WithScheme(runtimeScheme)
			if tt.includeUser {
				builder = builder.WithObjects(testUser)
			}

			// If this test case is for duplicate validation, seed an existing UserDeactivation for the same user
			if tt.errorContains == "UserDeactivation already exists" {
				existingUD := &iamv1alpha1.UserDeactivation{
					ObjectMeta: metav1.ObjectMeta{Name: "first-deactivate-test-user"},
					Spec: iamv1alpha1.UserDeactivationSpec{
						UserRef:       iamv1alpha1.UserReference{Name: testUser.Name},
						Reason:        "Existing",
						DeactivatedBy: "tester",
					},
				}
				builder = builder.WithObjects(existingUD)
			}

			fakeClient := builder.Build()

			validator := &UserDeactivationValidator{
				client:          fakeClient,
				systemNamespace: systemNamespace,
			}

			// Build admission request context with authenticated user info
			req := admission.Request{
				AdmissionRequest: admissionv1.AdmissionRequest{
					UserInfo: authenticationv1.UserInfo{
						Username: "tester",
					},
				},
			}
			ctx := admission.NewContextWithRequest(context.Background(), req)

			warnings, err := validator.ValidateCreate(ctx, tt.userDeactivation)

			if tt.expectError {
				assert.Error(t, err, "expected validation error")
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains, "error message should contain expected text")
				}
			} else {
				assert.NoError(t, err, "expected no validation error")
			}

			// No warnings are expected from current validator implementation
			assert.Empty(t, warnings, "expected no validation warnings")
		})
	}
}

// Test the mutator defaulting behavior to ensure deactivatedBy is set correctly before validation.
func TestUserDeactivationMutator_DefaultsAndValidator(t *testing.T) {
	systemNamespace := "milo-system"

	// Prepare UserDeactivation without DeactivatedBy
	ud := &iamv1alpha1.UserDeactivation{
		ObjectMeta: metav1.ObjectMeta{
			Name: "deactivate-test-user",
		},
		Spec: iamv1alpha1.UserDeactivationSpec{
			UserRef: iamv1alpha1.UserReference{
				Name: "test-user",
			},
			Reason: "Testing default",
		},
	}

	// Build fake client including referenced user
	testUser := &iamv1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{Name: "test-user"},
		Spec:       iamv1alpha1.UserSpec{Email: "test@example.com"},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(runtimeScheme).WithObjects(testUser).Build()

	// Create admission request context with user "tester"
	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UserInfo: authenticationv1.UserInfo{Username: "tester"},
		},
	}
	ctx := admission.NewContextWithRequest(context.Background(), req)

	// Run mutator default
	mutator := &UserDeactivationMutator{}
	assert.NoError(t, mutator.Default(ctx, ud))

	// Ensure DeactivatedBy was defaulted to requester username
	assert.Equal(t, "tester", ud.Spec.DeactivatedBy, "deactivatedBy should be defaulted to requester username")

	// Validate create should now pass
	validator := &UserDeactivationValidator{client: fakeClient, systemNamespace: systemNamespace}
	warnings, err := validator.ValidateCreate(ctx, ud)
	assert.NoError(t, err)
	assert.Empty(t, warnings)
}
