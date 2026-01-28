package v1alpha1

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	iamv1alpha1 "go.miloapis.com/milo/pkg/apis/iam/v1alpha1"
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func init() {
	utilruntime.Must(iamv1alpha1.AddToScheme(runtimeScheme))
}

// TestPlatformAccessApprovalMutator_Default verifies that the approverRef is defaulted to the requesting user.
func TestPlatformAccessApprovalMutator_Default(t *testing.T) {
	// Create an approver user that exists in the cluster
	approver := &iamv1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{
			Name: "approver-user",
		},
		Spec: iamv1alpha1.UserSpec{
			Email: "approver@example.com",
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(runtimeScheme).WithObjects(approver).Build()

	mutator := &PlatformAccessApprovalMutator{client: fakeClient}

	paa := &iamv1alpha1.PlatformAccessApproval{
		ObjectMeta: metav1.ObjectMeta{Name: "approve-access"},
		Spec: iamv1alpha1.PlatformAccessApprovalSpec{
			SubjectRef: iamv1alpha1.SubjectReference{Email: "subject@example.com"},
		},
	}

	// Inject admission request context where Username == approver-user
	req := admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{UserInfo: authenticationv1.UserInfo{UID: "approver-user"}}}
	ctx := admission.NewContextWithRequest(context.Background(), req)

	assert.NoError(t, mutator.Default(ctx, paa))
	assert.NotNil(t, paa.Spec.ApproverRef)
	assert.Equal(t, "approver-user", paa.Spec.ApproverRef.Name)
}

// TestPlatformAccessApprovalValidator_ValidateCreate covers email and userRef validation paths.
func TestPlatformAccessApprovalValidator_ValidateCreate(t *testing.T) {
	validUser := &iamv1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{Name: "valid-user"},
		Spec: iamv1alpha1.UserSpec{
			Email: "valid@example.com",
		},
	}

	tests := map[string]struct {
		paa          *iamv1alpha1.PlatformAccessApproval
		preObjects   []client.Object
		expectError  bool
		errSubstring string
	}{
		"valid email only": {
			paa: &iamv1alpha1.PlatformAccessApproval{
				ObjectMeta: metav1.ObjectMeta{Name: "email-only"},
				Spec: iamv1alpha1.PlatformAccessApprovalSpec{
					SubjectRef: iamv1alpha1.SubjectReference{Email: "user@example.com"},
				},
			},
			expectError: false,
		},
		"invalid email": {
			paa: &iamv1alpha1.PlatformAccessApproval{
				ObjectMeta: metav1.ObjectMeta{Name: "bad-email"},
				Spec: iamv1alpha1.PlatformAccessApprovalSpec{
					SubjectRef: iamv1alpha1.SubjectReference{Email: "not-an-email"},
				},
			},
			expectError:  true,
			errSubstring: "invalid email address",
		},
		"valid userRef present": {
			paa: &iamv1alpha1.PlatformAccessApproval{
				ObjectMeta: metav1.ObjectMeta{Name: "userref"},
				Spec: iamv1alpha1.PlatformAccessApprovalSpec{
					SubjectRef: iamv1alpha1.SubjectReference{UserRef: &iamv1alpha1.UserReference{Name: "valid-user"}},
				},
			},
			preObjects:  []client.Object{validUser},
			expectError: false,
		},
		"userRef not found": {
			paa: &iamv1alpha1.PlatformAccessApproval{
				ObjectMeta: metav1.ObjectMeta{Name: "missing-user"},
				Spec: iamv1alpha1.PlatformAccessApprovalSpec{
					SubjectRef: iamv1alpha1.SubjectReference{UserRef: &iamv1alpha1.UserReference{Name: "ghost"}},
				},
			},
			expectError:  true,
			errSubstring: "Not found",
		},
		"platformaccessapproval already exists": {
			paa: &iamv1alpha1.PlatformAccessApproval{
				ObjectMeta: metav1.ObjectMeta{Name: "dup-approval"},
				Spec: iamv1alpha1.PlatformAccessApprovalSpec{
					SubjectRef: iamv1alpha1.SubjectReference{Email: "dup@example.com"},
				},
			},
			preObjects: []client.Object{&iamv1alpha1.PlatformAccessApproval{
				ObjectMeta: metav1.ObjectMeta{Name: "existing-dup"},
				Spec: iamv1alpha1.PlatformAccessApprovalSpec{
					SubjectRef: iamv1alpha1.SubjectReference{Email: "dup@example.com"},
				},
			}},
			expectError:  true,
			errSubstring: "an existing platformaccessapproval",
		},
		"platformaccessrejection already exists": {
			paa: &iamv1alpha1.PlatformAccessApproval{
				ObjectMeta: metav1.ObjectMeta{Name: "rejection-exists"},
				Spec: iamv1alpha1.PlatformAccessApprovalSpec{
					SubjectRef: iamv1alpha1.SubjectReference{UserRef: &iamv1alpha1.UserReference{Name: "reject-user"}},
				},
			},
			preObjects: []client.Object{&iamv1alpha1.PlatformAccessRejection{
				ObjectMeta: metav1.ObjectMeta{Name: "existing-rej"},
				Spec: iamv1alpha1.PlatformAccessRejectionSpec{
					UserRef: iamv1alpha1.UserReference{Name: "reject-user"},
				},
			}},
			expectError:  true,
			errSubstring: "platformaccessrejection",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			builder := fake.NewClientBuilder().WithScheme(runtimeScheme)
			if len(tc.preObjects) > 0 {
				builder = builder.WithObjects(tc.preObjects...)
			}
			// Register the same field indexes used by the real manager so MatchingFields selectors work in tests.
			builder = builder.
				WithIndex(&iamv1alpha1.PlatformAccessApproval{}, platformAccessApprovalIndexKey, func(rawObj client.Object) []string {
					paa := rawObj.(*iamv1alpha1.PlatformAccessApproval)
					return []string{buildPlatformAccessIndexKey(&paa.Spec.SubjectRef)}
				}).
				WithIndex(&iamv1alpha1.PlatformAccessRejection{}, platformAccessRejectionIndexKey, func(rawObj client.Object) []string {
					par := rawObj.(*iamv1alpha1.PlatformAccessRejection)
					return []string{par.Spec.UserRef.Name}
				}).
				WithIndex(&iamv1alpha1.User{}, userEmailIndexKey, func(rawObj client.Object) []string {
					user := rawObj.(*iamv1alpha1.User)
					return []string{strings.ToLower(user.Spec.Email)}
				})
			cl := builder.Build()
			validator := &PlatformAccessApprovalValidator{client: cl}

			warnings, err := validator.ValidateCreate(context.Background(), tc.paa)
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
