package v1alpha1

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	iamv1alpha1 "go.miloapis.com/milo/pkg/apis/iam/v1alpha1"
	notificationv1alpha1 "go.miloapis.com/milo/pkg/apis/notification/v1alpha1"
	resourcemanagerv1alpha1 "go.miloapis.com/milo/pkg/apis/resourcemanager/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var runtimeScheme = runtime.NewScheme()

func init() {
	utilruntime.Must(iamv1alpha1.AddToScheme(runtimeScheme))
	utilruntime.Must(notificationv1alpha1.AddToScheme(runtimeScheme))
	utilruntime.Must(resourcemanagerv1alpha1.AddToScheme(runtimeScheme))
}

func TestContactMutator_Default(t *testing.T) {
	// Prepare referenced User
	user := &iamv1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-user",
			UID:  types.UID("uid-test-user"),
		},
		Spec: iamv1alpha1.UserSpec{Email: "test@example.com"},
	}

	// Contact referencing the User
	contact := &notificationv1alpha1.Contact{
		ObjectMeta: metav1.ObjectMeta{Name: "contact-test"},
		Spec: notificationv1alpha1.ContactSpec{
			GivenName:  "Test",
			FamilyName: "User",
			Email:      "contact@example.com",
			SubjectRef: &notificationv1alpha1.SubjectReference{
				APIGroup:  "iam.miloapis.com",
				Kind:      "User",
				Name:      user.Name,
				Namespace: "", // cluster-scoped
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(runtimeScheme).WithObjects(user).Build()
	mutator := &ContactMutator{client: fakeClient, scheme: runtimeScheme}

	err := mutator.Default(context.Background(), contact)
	assert.NoError(t, err, "mutator should not return error")

	// Expect owner reference to be set
	if assert.Len(t, contact.OwnerReferences, 1, "expected one owner reference") {
		ref := contact.OwnerReferences[0]
		assert.Equal(t, iamv1alpha1.SchemeGroupVersion.String(), ref.APIVersion)
		assert.Equal(t, "User", ref.Kind)
		assert.Equal(t, user.Name, ref.Name)
		assert.Equal(t, user.UID, ref.UID)
	}
}

func TestContactValidator_ValidateCreate(t *testing.T) {
	tests := map[string]struct {
		contact       *notificationv1alpha1.Contact
		seedObjects   []client.Object
		expectError   bool
		errorContains string
	}{
		"valid newsletter contact": {
			contact: &notificationv1alpha1.Contact{
				ObjectMeta: metav1.ObjectMeta{Name: "newsletter"},
				Spec: notificationv1alpha1.ContactSpec{
					GivenName:  "News",
					FamilyName: "Letter",
					Email:      "news@example.com",
				},
			},
			expectError: false,
		},
		"valid user contact": {
			contact: func() *notificationv1alpha1.Contact {
				return &notificationv1alpha1.Contact{
					ObjectMeta: metav1.ObjectMeta{Name: "user-contact"},
					Spec: notificationv1alpha1.ContactSpec{
						GivenName:  "Test",
						FamilyName: "User",
						Email:      "user@example.com",
						SubjectRef: &notificationv1alpha1.SubjectReference{
							APIGroup: "iam.miloapis.com",
							Kind:     "User",
							Name:     "test-user",
						},
					},
				}
			}(),
			seedObjects: []client.Object{&iamv1alpha1.User{
				ObjectMeta: metav1.ObjectMeta{Name: "test-user"},
				Spec:       iamv1alpha1.UserSpec{Email: "test@example.com"},
			}},
			expectError: false,
		},
		"invalid email format": {
			contact: &notificationv1alpha1.Contact{
				ObjectMeta: metav1.ObjectMeta{Name: "bad-email"},
				Spec: notificationv1alpha1.ContactSpec{
					GivenName:  "Bad",
					FamilyName: "Email",
					Email:      "not-an-email",
				},
			},
			expectError:   true,
			errorContains: "invalid email",
		},
		"invalid kind for iam api group": {
			contact: &notificationv1alpha1.Contact{
				ObjectMeta: metav1.ObjectMeta{Name: "wrong-kind"},
				Spec: notificationv1alpha1.ContactSpec{
					GivenName:  "Bad",
					FamilyName: "Kind",
					Email:      "kind@example.com",
					SubjectRef: &notificationv1alpha1.SubjectReference{
						APIGroup: "iam.miloapis.com",
						Kind:     "Organization",
						Name:     "ignored",
						UID:      "xyz",
					},
				},
			},
			expectError:   true,
			errorContains: "kind must be User",
		},
		"user not found": {
			contact: &notificationv1alpha1.Contact{
				ObjectMeta: metav1.ObjectMeta{Name: "missing-user"},
				Spec: notificationv1alpha1.ContactSpec{
					GivenName:  "Miss",
					FamilyName: "User",
					Email:      "missing@example.com",
					SubjectRef: &notificationv1alpha1.SubjectReference{
						APIGroup: "iam.miloapis.com",
						Kind:     "User",
						Name:     "nonexistent",
						UID:      "zzz",
					},
				},
			},
			expectError:   true,
			errorContains: "not found",
		},
		"user with namespace should error": {
			contact: &notificationv1alpha1.Contact{
				ObjectMeta: metav1.ObjectMeta{Name: "user-with-ns"},
				Spec: notificationv1alpha1.ContactSpec{
					GivenName:  "Bad",
					FamilyName: "Namespace",
					Email:      "userns@example.com",
					SubjectRef: &notificationv1alpha1.SubjectReference{
						APIGroup:  "iam.miloapis.com",
						Kind:      "User",
						Name:      "test-user",
						Namespace: "some-namespace",
					},
				},
			},
			seedObjects:   []client.Object{&iamv1alpha1.User{ObjectMeta: metav1.ObjectMeta{Name: "test-user"}}},
			expectError:   true,
			errorContains: "namespace must be empty for User",
		},
		"valid organization contact": {
			contact: &notificationv1alpha1.Contact{
				ObjectMeta: metav1.ObjectMeta{Name: "org-contact"},
				Spec: notificationv1alpha1.ContactSpec{
					GivenName:  "Org",
					FamilyName: "Member",
					Email:      "orgmember@example.com",
					SubjectRef: &notificationv1alpha1.SubjectReference{
						APIGroup:  "resourcemanager.miloapis.com",
						Kind:      "Organization",
						Name:      "org1",
						Namespace: "organization-org1",
					},
				},
			},
			seedObjects: []client.Object{&resourcemanagerv1alpha1.Organization{ObjectMeta: metav1.ObjectMeta{Name: "org1"}}},
			expectError: false,
		},
		"organization namespace mismatch": {
			contact: &notificationv1alpha1.Contact{
				ObjectMeta: metav1.ObjectMeta{Name: "org-mismatch"},
				Spec: notificationv1alpha1.ContactSpec{
					GivenName:  "Org",
					FamilyName: "Mismatch",
					Email:      "mismatch@example.com",
					SubjectRef: &notificationv1alpha1.SubjectReference{
						APIGroup:  "resourcemanager.miloapis.com",
						Kind:      "Organization",
						Name:      "org1",
						Namespace: "wrong-namespace",
					},
				},
			},
			seedObjects:   []client.Object{&resourcemanagerv1alpha1.Organization{ObjectMeta: metav1.ObjectMeta{Name: "org1"}}},
			expectError:   true,
			errorContains: "namespace must be the organization namespace",
		},
		"organization not found": {
			contact: &notificationv1alpha1.Contact{
				ObjectMeta: metav1.ObjectMeta{Name: "org-notfound"},
				Spec: notificationv1alpha1.ContactSpec{
					GivenName:  "Org",
					FamilyName: "Missing",
					Email:      "missingorg@example.com",
					SubjectRef: &notificationv1alpha1.SubjectReference{
						APIGroup:  "resourcemanager.miloapis.com",
						Kind:      "Organization",
						Name:      "org-missing",
						Namespace: "organization-org-missing",
					},
				},
			},
			expectError:   true,
			errorContains: "not found",
		},
		"valid project contact": {
			contact: &notificationv1alpha1.Contact{
				ObjectMeta: metav1.ObjectMeta{Name: "proj-contact"},
				Spec: notificationv1alpha1.ContactSpec{
					GivenName:  "Proj",
					FamilyName: "Member",
					Email:      "projmember@example.com",
					SubjectRef: &notificationv1alpha1.SubjectReference{
						APIGroup:  "resourcemanager.miloapis.com",
						Kind:      "Project",
						Name:      "proj1",
						Namespace: "organization-org1",
					},
				},
			},
			seedObjects: []client.Object{
				&resourcemanagerv1alpha1.Project{
					ObjectMeta: metav1.ObjectMeta{Name: "proj1"},
					Spec:       resourcemanagerv1alpha1.ProjectSpec{OwnerRef: resourcemanagerv1alpha1.OwnerReference{Kind: "Organization", Name: "org1"}},
				},
			},
			expectError: false,
		},
		"project namespace mismatch": {
			contact: &notificationv1alpha1.Contact{
				ObjectMeta: metav1.ObjectMeta{Name: "proj-mismatch"},
				Spec: notificationv1alpha1.ContactSpec{
					GivenName:  "Proj",
					FamilyName: "Mismatch",
					Email:      "projmismatch@example.com",
					SubjectRef: &notificationv1alpha1.SubjectReference{
						APIGroup:  "resourcemanager.miloapis.com",
						Kind:      "Project",
						Name:      "proj1",
						Namespace: "organization-wrong",
					},
				},
			},
			seedObjects: []client.Object{
				&resourcemanagerv1alpha1.Project{
					ObjectMeta: metav1.ObjectMeta{Name: "proj1"},
					Spec:       resourcemanagerv1alpha1.ProjectSpec{OwnerRef: resourcemanagerv1alpha1.OwnerReference{Kind: "Organization", Name: "org1"}},
				},
			},
			expectError:   true,
			errorContains: "namespace must be the project owner's namespace",
		},
		"project not found": {
			contact: &notificationv1alpha1.Contact{
				ObjectMeta: metav1.ObjectMeta{Name: "proj-notfound"},
				Spec: notificationv1alpha1.ContactSpec{
					GivenName:  "Proj",
					FamilyName: "Missing",
					Email:      "missingproj@example.com",
					SubjectRef: &notificationv1alpha1.SubjectReference{
						APIGroup:  "resourcemanager.miloapis.com",
						Kind:      "Project",
						Name:      "proj-missing",
						Namespace: "organization-org1",
					},
				},
			},
			expectError:   true,
			errorContains: "not found",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			builder := fake.NewClientBuilder().WithScheme(runtimeScheme)
			if len(tt.seedObjects) > 0 {
				builder = builder.WithObjects(tt.seedObjects...)
			}
			fakeClient := builder.Build()

			validator := &ContactValidator{Client: fakeClient}
			_, err := validator.ValidateCreate(context.Background(), tt.contact)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, strings.ToLower(err.Error()), strings.ToLower(tt.errorContains))
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
