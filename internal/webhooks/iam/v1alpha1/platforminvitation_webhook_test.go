package v1alpha1

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	iamv1alpha1 "go.miloapis.com/milo/pkg/apis/iam/v1alpha1"
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// TestPlatformInvitationMutator_Default verifies that InvitedBy defaults to the requesting user.
func TestPlatformInvitationMutator_Default(t *testing.T) {
	inviter := &iamv1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{
			Name: "requester",
		},
		Spec: iamv1alpha1.UserSpec{
			Email: "requester@example.com",
		},
	}

	cl := fake.NewClientBuilder().WithScheme(runtimeScheme).WithObjects(inviter).Build()
	mutator := &PlatformInvitationMutator{client: cl}

	pi := &iamv1alpha1.PlatformInvitation{
		ObjectMeta: metav1.ObjectMeta{Name: "invite-platform-user"},
		Spec:       iamv1alpha1.PlatformInvitationSpec{Email: "invitee@example.com"},
	}

	req := admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{UserInfo: authenticationv1.UserInfo{UID: "requester"}}}
	ctx := admission.NewContextWithRequest(context.Background(), req)

	assert.NoError(t, mutator.Default(ctx, pi))
	assert.Equal(t, "requester", pi.Spec.InvitedBy.Name)
}

// TestPlatformInvitationValidator_ValidateCreate validates scenarios for email format, scheduleAt, user existence and duplicate invitations.
func TestPlatformInvitationValidator_ValidateCreate(t *testing.T) {
	now := time.Now().UTC()
	past := metav1.NewTime(now.Add(-1 * time.Hour))
	future := metav1.NewTime(now.Add(1 * time.Hour))

	existingUser := &iamv1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{Name: "existing-user"},
		Spec:       iamv1alpha1.UserSpec{Email: "dupe@example.com"},
	}

	existingInvitation := &iamv1alpha1.PlatformInvitation{
		ObjectMeta: metav1.ObjectMeta{Name: "existing-invite"},
		Spec:       iamv1alpha1.PlatformInvitationSpec{Email: "alreadyinvited@example.com"},
	}

	tests := map[string]struct {
		obj         *iamv1alpha1.PlatformInvitation
		preObjects  []client.Object
		expectError bool
		contains    string
	}{
		"valid invitation": {
			obj: &iamv1alpha1.PlatformInvitation{
				ObjectMeta: metav1.ObjectMeta{Name: "valid"},
				Spec:       iamv1alpha1.PlatformInvitationSpec{Email: "new@example.com"},
			},
			expectError: false,
		},
		"invalid email": {
			obj:         &iamv1alpha1.PlatformInvitation{ObjectMeta: metav1.ObjectMeta{Name: "bad-email"}, Spec: iamv1alpha1.PlatformInvitationSpec{Email: "not-an-email"}},
			expectError: true,
			contains:    "invalid email address",
		},
		"scheduleAt in the past": {
			obj:         &iamv1alpha1.PlatformInvitation{ObjectMeta: metav1.ObjectMeta{Name: "past"}, Spec: iamv1alpha1.PlatformInvitationSpec{Email: "past@example.com", ScheduleAt: &past}},
			expectError: true,
			contains:    "scheduleAt must be in the future",
		},
		"user with same email exists": {
			obj:         &iamv1alpha1.PlatformInvitation{ObjectMeta: metav1.ObjectMeta{Name: "user-exists"}, Spec: iamv1alpha1.PlatformInvitationSpec{Email: "dupe@example.com"}},
			preObjects:  []client.Object{existingUser},
			expectError: true,
			contains:    "a user with this email already exists",
		},
		"invitation with same email exists": {
			obj:         &iamv1alpha1.PlatformInvitation{ObjectMeta: metav1.ObjectMeta{Name: "invite-exists"}, Spec: iamv1alpha1.PlatformInvitationSpec{Email: "alreadyinvited@example.com"}},
			preObjects:  []client.Object{existingInvitation},
			expectError: true,
			contains:    "platforminvitation with this email already exists",
		},
		"valid when scheduleAt is in future": {
			obj:         &iamv1alpha1.PlatformInvitation{ObjectMeta: metav1.ObjectMeta{Name: "future"}, Spec: iamv1alpha1.PlatformInvitationSpec{Email: "future@example.com", ScheduleAt: &future}},
			expectError: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			builder := fake.NewClientBuilder().WithScheme(runtimeScheme)
			if len(tc.preObjects) > 0 {
				builder = builder.WithObjects(tc.preObjects...)
			}
			// Register the same field indexes used by the real manager
			builder = builder.
				WithIndex(&iamv1alpha1.User{}, platformInvitationUserEmailIndexKey, func(rawObj client.Object) []string {
					u := rawObj.(*iamv1alpha1.User)
					return []string{strings.ToLower(u.Spec.Email)}
				}).
				WithIndex(&iamv1alpha1.PlatformInvitation{}, platformInvitationEmailIndexKey, func(rawObj client.Object) []string {
					pi := rawObj.(*iamv1alpha1.PlatformInvitation)
					return []string{strings.ToLower(pi.Spec.Email)}
				})

			cl := builder.Build()
			v := &PlatformInvitationValidator{client: cl}

			warnings, err := v.ValidateCreate(context.Background(), tc.obj)
			if tc.expectError {
				assert.Error(t, err)
				if tc.contains != "" {
					assert.Contains(t, err.Error(), tc.contains)
				}
			} else {
				assert.NoError(t, err)
			}
			assert.Empty(t, warnings)
		})
	}
}
