package v1alpha1

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	iamv1alpha1 "go.miloapis.com/milo/pkg/apis/iam/v1alpha1"
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// TestPlatformAccessRejectionMutator_Default ensures that RejecterRef defaults to the requesting user.
func TestPlatformAccessRejectionMutator_Default(t *testing.T) {
	// Create rejecter user existing in cluster
	rejecter := &iamv1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{Name: "rejecter-user"},
		Spec:       iamv1alpha1.UserSpec{Email: "rejecter@example.com"},
	}

	cl := fake.NewClientBuilder().WithScheme(runtimeScheme).WithObjects(rejecter).Build()
	mutator := &PlatformAccessRejectionMutator{client: cl}

	par := &iamv1alpha1.PlatformAccessRejection{
		ObjectMeta: metav1.ObjectMeta{Name: "reject-access"},
		Spec: iamv1alpha1.PlatformAccessRejectionSpec{
			UserRef: iamv1alpha1.UserReference{Name: "target-user"},
			Reason:  "Not authorized",
		},
	}

	// Admission context with UID == rejecter-user
	req := admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{UserInfo: authenticationv1.UserInfo{UID: "rejecter-user"}}}
	ctx := admission.NewContextWithRequest(context.Background(), req)

	assert.NoError(t, mutator.Default(ctx, par))
	if assert.NotNil(t, par.Spec.RejecterRef) {
		assert.Equal(t, "rejecter-user", par.Spec.RejecterRef.Name)
	}
}

// TestPlatformAccessRejectionValidator_ValidateCreate covers validation scenarios.
func TestPlatformAccessRejectionValidator_ValidateCreate(t *testing.T) {
	existingUser := &iamv1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{Name: "existing-user"},
		Spec:       iamv1alpha1.UserSpec{Email: "existing@example.com"},
	}

	tests := map[string]struct {
		par          *iamv1alpha1.PlatformAccessRejection
		preObjects   []client.Object
		expectError  bool
		errSubstring string
	}{
		"valid when user exists": {
			par: &iamv1alpha1.PlatformAccessRejection{
				ObjectMeta: metav1.ObjectMeta{Name: "reject-existing-user"},
				Spec: iamv1alpha1.PlatformAccessRejectionSpec{
					UserRef: iamv1alpha1.UserReference{Name: "existing-user"},
					Reason:  "Not allowed",
				},
			},
			preObjects:  []client.Object{existingUser},
			expectError: false,
		},
		"user not found": {
			par: &iamv1alpha1.PlatformAccessRejection{
				ObjectMeta: metav1.ObjectMeta{Name: "reject-missing"},
				Spec: iamv1alpha1.PlatformAccessRejectionSpec{
					UserRef: iamv1alpha1.UserReference{Name: "ghost"},
					Reason:  "Missing user",
				},
			},
			expectError:  true,
			errSubstring: "Not found",
		},
		"duplicate rejection exists": {
			par: &iamv1alpha1.PlatformAccessRejection{
				ObjectMeta: metav1.ObjectMeta{Name: "reject-dup"},
				Spec: iamv1alpha1.PlatformAccessRejectionSpec{
					UserRef: iamv1alpha1.UserReference{Name: "existing-user"},
					Reason:  "Duplicate",
				},
			},
			preObjects: []client.Object{existingUser, &iamv1alpha1.PlatformAccessRejection{
				ObjectMeta: metav1.ObjectMeta{Name: "existing-rejection"},
				Spec: iamv1alpha1.PlatformAccessRejectionSpec{
					UserRef: iamv1alpha1.UserReference{Name: "existing-user"},
					Reason:  "First",
				},
			}},
			expectError:  true,
			errSubstring: "platformaccessrejection",
		},
		"platformaccessapproval exists": {
			par: &iamv1alpha1.PlatformAccessRejection{
				ObjectMeta: metav1.ObjectMeta{Name: "approval-exists"},
				Spec: iamv1alpha1.PlatformAccessRejectionSpec{
					UserRef: iamv1alpha1.UserReference{Name: "existing-user"},
					Reason:  "Approval already",
				},
			},
			preObjects: []client.Object{existingUser, &iamv1alpha1.PlatformAccessApproval{
				ObjectMeta: metav1.ObjectMeta{Name: "existing-approval"},
				Spec: iamv1alpha1.PlatformAccessApprovalSpec{
					SubjectRef: iamv1alpha1.SubjectReference{UserRef: &iamv1alpha1.UserReference{Name: "existing-user"}},
				},
			}},
			expectError:  true,
			errSubstring: "platformaccessapproval already exists",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			builder := fake.NewClientBuilder().WithScheme(runtimeScheme)
			if len(tc.preObjects) > 0 {
				builder = builder.WithObjects(tc.preObjects...)
			}
			// Register field indexes
			builder = builder.
				WithIndex(&iamv1alpha1.PlatformAccessRejection{}, platformAccessRejectionIndexKey, func(rawObj client.Object) []string {
					par := rawObj.(*iamv1alpha1.PlatformAccessRejection)
					return []string{par.Spec.UserRef.Name}
				}).
				WithIndex(&iamv1alpha1.PlatformAccessApproval{}, platformAccessApprovalIndexKey, func(rawObj client.Object) []string {
					paa := rawObj.(*iamv1alpha1.PlatformAccessApproval)
					return []string{buildPlatformAccessIndexKey(&paa.Spec.SubjectRef)}
				})
			cl := builder.Build()
			validator := &PlatformAccessRejectionValidator{client: cl}

			warnings, err := validator.ValidateCreate(context.Background(), tc.par)
			if tc.expectError {
				assert.Error(t, err)
				if tc.errSubstring != "" {
					assert.Contains(t, err.Error(), tc.errSubstring)
				}
			} else {
				assert.NoError(t, err)
			}
			assert.Empty(t, warnings)
		})
	}
}
