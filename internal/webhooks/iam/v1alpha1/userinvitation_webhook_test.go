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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

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
			UserInfo: authenticationv1.UserInfo{Username: "requester", UID: "requester"},
		},
	}
	ctx := admission.NewContextWithRequest(context.Background(), req)

	// Build a fake client containing the inviter User so the mutator's lookup succeeds.
	inviterUser := &iamv1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{Name: "requester", UID: "requester"},
	}
	fakeClient := fake.NewClientBuilder().WithScheme(runtimeScheme).WithObjects(inviterUser).Build()

	mutator := &UserInvitationMutator{client: fakeClient}
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
		existing       []client.Object
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
		"error when duplicate invitation exists": {
			invitation: &iamv1alpha1.UserInvitation{
				ObjectMeta: metav1.ObjectMeta{Name: "duplicate-invitation"},
				Spec: iamv1alpha1.UserInvitationSpec{
					Email:           "duplicate@example.com",
					State:           "Pending",
					OrganizationRef: resourcemanagerv1alpha1.OrganizationReference{Name: "testorg"},
				},
			},
			existing: []client.Object{
				&iamv1alpha1.UserInvitation{
					ObjectMeta: metav1.ObjectMeta{Name: "existing-invitation"},
					Spec: iamv1alpha1.UserInvitationSpec{
						Email:           "duplicate@example.com",
						State:           "Pending",
						OrganizationRef: resourcemanagerv1alpha1.OrganizationReference{Name: "testorg"},
					},
				},
			},
			expectError:    true,
			errorSubstring: "organizationRef",
		},
	}

	// Common admission request used across sub-tests
	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Namespace: "organization-testorg",
			UserInfo:  authenticationv1.UserInfo{Username: "tester"},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Build fake client with any existing objects for this test case
			builder := fake.NewClientBuilder().WithScheme(runtimeScheme)
			if len(tc.existing) > 0 {
				builder = builder.WithObjects(tc.existing...)
			}
			// Add composite key index to mimic real indexer behaviour
			builder = builder.WithIndex(&iamv1alpha1.UserInvitation{}, userInvitationCompositeKey, func(raw client.Object) []string {
				ui := raw.(*iamv1alpha1.UserInvitation)
				return []string{buildUserInvitationCompositeKey(*ui)}
			})
			fakeClient := builder.Build()

			validator := &UserInvitationValidator{client: fakeClient}

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
