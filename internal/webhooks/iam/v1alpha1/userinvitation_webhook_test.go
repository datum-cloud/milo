package v1alpha1

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	iamv1alpha1 "go.miloapis.com/milo/pkg/apis/iam/v1alpha1"
	resourcemanagerv1alpha1 "go.miloapis.com/milo/pkg/apis/resourcemanager/v1alpha1"
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func init() {
	utilruntime.Must(iamv1alpha1.AddToScheme(runtimeScheme))
}

// TestUserInvitationMutator_Default ensures that the InvitedBy field is defaulted to the requesting user.
func TestUserInvitationMutator_Default(t *testing.T) {
	// Prepare basic UserInvitation with no InvitedBy
	ui := &iamv1alpha1.UserInvitation{
		ObjectMeta: metav1.ObjectMeta{Name: "invite-user"},
		Spec: iamv1alpha1.UserInvitationSpec{
			Email: "invitee@example.com",
			State: "Pending",
		},
	}

	// Create admission request context with authenticated user
	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UserInfo: authenticationv1.UserInfo{Username: "requester"},
		},
	}
	ctx := admission.NewContextWithRequest(context.Background(), req)

	mutator := &UserInvitationMutator{}
	assert.NoError(t, mutator.Default(ctx, ui))

	// Check that InvitedBy has been populated correctly
	assert.Equal(t, "requester", ui.Spec.InvitedBy.Name, "invitedBy should be set to the requester username")
}

// TestUserInvitationValidator_ValidateCreate covers expiration date validation.
func TestUserInvitationValidator_ValidateCreate(t *testing.T) {
	now := time.Now().UTC()
	past := metav1.NewTime(now.Add(-1 * time.Hour))
	future := metav1.NewTime(now.Add(1 * time.Hour))

	tests := map[string]struct {
		invitation     *iamv1alpha1.UserInvitation
		expectError    bool
		errorSubstring string
	}{
		"valid when expirationDate is nil": {
			invitation: &iamv1alpha1.UserInvitation{
				ObjectMeta: metav1.ObjectMeta{Name: "no-expiration"},
				Spec: iamv1alpha1.UserInvitationSpec{
					Email:           "abc@example.com",
					State:           "Pending",
					OrganizationRef: resourcemanagerv1alpha1.OrganizationReference{Name: "testorg"},
				},
			},
			expectError: false,
		},
		"valid when expirationDate is in the future": {
			invitation: &iamv1alpha1.UserInvitation{
				ObjectMeta: metav1.ObjectMeta{Name: "future-expiration"},
				Spec: iamv1alpha1.UserInvitationSpec{
					Email:           "future@example.com",
					State:           "Pending",
					ExpirationDate:  &future,
					OrganizationRef: resourcemanagerv1alpha1.OrganizationReference{Name: "testorg"},
				},
			},
			expectError: false,
		},
		"error when expirationDate is in the past": {
			invitation: &iamv1alpha1.UserInvitation{
				ObjectMeta: metav1.ObjectMeta{Name: "past-expiration"},
				Spec: iamv1alpha1.UserInvitationSpec{
					Email:           "past@example.com",
					State:           "Pending",
					ExpirationDate:  &past,
					OrganizationRef: resourcemanagerv1alpha1.OrganizationReference{Name: "testorg"},
				},
			},
			expectError:    true,
			errorSubstring: "expirationDate must be in the future",
		},
		"error when organizationRef is not set": {
			invitation: &iamv1alpha1.UserInvitation{
				ObjectMeta: metav1.ObjectMeta{Name: "no-organization"},
				Spec: iamv1alpha1.UserInvitationSpec{
					Email: "no-org@example.com",
					State: "Pending",
				},
			},
			expectError:    true,
			errorSubstring: "organizationRef must be the same as the requesting user's organization",
		},
		"error when organizationRef is not in the same namespace": {
			invitation: &iamv1alpha1.UserInvitation{
				ObjectMeta: metav1.ObjectMeta{Name: "no-organization"},
				Spec: iamv1alpha1.UserInvitationSpec{
					Email:           "no-org@example.com",
					State:           "Pending",
					OrganizationRef: resourcemanagerv1alpha1.OrganizationReference{Name: "testorg-1"},
				},
			},
			expectError:    true,
			errorSubstring: "organizationRef must be the same as the requesting user's organization",
		},
	}

	validator := &UserInvitationValidator{}

	// Common admission request used across sub-tests
	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Namespace: "organization-testorg",
			UserInfo:  authenticationv1.UserInfo{Username: "tester"},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Ensure OrganizationRef is set so validator passes namespace check
			ctx := admission.NewContextWithRequest(context.Background(), req)

			warnings, err := validator.ValidateCreate(ctx, tc.invitation)
			if tc.expectError {
				assert.Error(t, err)
				if tc.errorSubstring != "" {
					assert.Contains(t, err.Error(), tc.errorSubstring)
				}
			} else {
				assert.NoError(t, err)
			}
			assert.Empty(t, warnings)
		})
	}
}
