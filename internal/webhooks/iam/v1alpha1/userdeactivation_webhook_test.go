package v1alpha1

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	iamv1alpha1 "go.miloapis.com/milo/pkg/apis/iam/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
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
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Build fake client with or without the user object depending on the test case
			builder := fake.NewClientBuilder().WithScheme(runtimeScheme)
			if tt.includeUser {
				builder = builder.WithObjects(testUser)
			}
			fakeClient := builder.Build()

			validator := &UserDeactivationValidator{
				client:          fakeClient,
				systemNamespace: systemNamespace,
			}

			ctx := context.Background()
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
