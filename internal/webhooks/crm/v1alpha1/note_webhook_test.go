package v1alpha1

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	crmv1alpha1 "go.miloapis.com/milo/pkg/apis/crm/v1alpha1"
	iamv1alpha1 "go.miloapis.com/milo/pkg/apis/iam/v1alpha1"
	notificationv1alpha1 "go.miloapis.com/milo/pkg/apis/notification/v1alpha1"
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var runtimeScheme = runtime.NewScheme()

func init() {
	utilruntime.Must(crmv1alpha1.AddToScheme(runtimeScheme))
	utilruntime.Must(iamv1alpha1.AddToScheme(runtimeScheme))
	utilruntime.Must(notificationv1alpha1.AddToScheme(runtimeScheme))
}

func TestNoteMutator_Default_UserSubject_SetsOwnerAndCreator(t *testing.T) {
	// Subject user that will become the owner reference
	subjectUser := &iamv1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{
			Name: "subject-user",
			UID:  types.UID("subject-user-uid"),
		},
	}

	// Creator user, identified by the admission request UID
	creatorUser := &iamv1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{
			Name: "creator-user",
			UID:  types.UID("creator-user-uid"),
		},
	}

	note := &crmv1alpha1.Note{
		ObjectMeta: metav1.ObjectMeta{
			Name: "note-user-subject",
		},
		Spec: crmv1alpha1.NoteSpec{
			Content: "test note",
			SubjectRef: crmv1alpha1.SubjectReference{
				APIGroup: "iam.miloapis.com",
				Kind:     "User",
				Name:     subjectUser.Name,
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(runtimeScheme).
		WithObjects(subjectUser, creatorUser).
		Build()

	mutator := &NoteMutator{
		Client: fakeClient,
		Scheme: runtimeScheme,
	}

	// Admission request with the creator's UID
	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UserInfo: authenticationv1.UserInfo{
				UID: creatorUser.Name,
			},
		},
	}
	ctx := admission.NewContextWithRequest(context.Background(), req)

	err := mutator.Default(ctx, note)
	assert.NoError(t, err)

	// Owner reference should point to the subject user
	if assert.Len(t, note.OwnerReferences, 1, "expected one owner reference") {
		ref := note.OwnerReferences[0]
		assert.Equal(t, iamv1alpha1.SchemeGroupVersion.String(), ref.APIVersion)
		assert.Equal(t, "User", ref.Kind)
		assert.Equal(t, subjectUser.Name, ref.Name)
		assert.Equal(t, subjectUser.UID, ref.UID)
	}

	// CreatorRef should be populated from the admission user
	assert.Equal(t, creatorUser.Name, note.Spec.CreatorRef.Name)
}

func TestNoteMutator_Default_UserSubject_UserNotFound(t *testing.T) {
	// Note refers to a user that does not exist in the fake client
	note := &crmv1alpha1.Note{
		ObjectMeta: metav1.ObjectMeta{
			Name: "note-missing-user",
		},
		Spec: crmv1alpha1.NoteSpec{
			Content: "test note",
			SubjectRef: crmv1alpha1.SubjectReference{
				APIGroup: "iam.miloapis.com",
				Kind:     "User",
				Name:     "missing-user",
			},
		},
	}

	// Creator user exists so failure is only from missing subject user
	creatorUser := &iamv1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{
			Name: "creator-user",
			UID:  types.UID("creator-user-uid"),
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(runtimeScheme).
		WithObjects(creatorUser).
		Build()

	mutator := &NoteMutator{
		Client: fakeClient,
		Scheme: runtimeScheme,
	}

	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UserInfo: authenticationv1.UserInfo{
				UID: creatorUser.Name,
			},
		},
	}
	ctx := admission.NewContextWithRequest(context.Background(), req)

	err := mutator.Default(ctx, note)
	assert.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "failed to get referenced user"))
}

func TestNoteMutator_Default_ContactSubject_SetsOwnerAndCreator(t *testing.T) {
	contact := &notificationv1alpha1.Contact{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "contact-1",
			Namespace: "contacts-ns",
			UID:       types.UID("contact-uid"),
		},
	}

	creatorUser := &iamv1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{
			Name: "creator-user",
			UID:  types.UID("creator-user-uid"),
		},
	}

	note := &crmv1alpha1.Note{
		ObjectMeta: metav1.ObjectMeta{
			Name: "note-contact-subject",
		},
		Spec: crmv1alpha1.NoteSpec{
			Content: "test note for contact",
			SubjectRef: crmv1alpha1.SubjectReference{
				APIGroup:  "notification.miloapis.com",
				Kind:      "Contact",
				Name:      contact.Name,
				Namespace: contact.Namespace,
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(runtimeScheme).
		WithObjects(contact, creatorUser).
		Build()

	mutator := &NoteMutator{
		Client: fakeClient,
		Scheme: runtimeScheme,
	}

	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UserInfo: authenticationv1.UserInfo{
				UID: creatorUser.Name,
			},
		},
	}
	ctx := admission.NewContextWithRequest(context.Background(), req)

	err := mutator.Default(ctx, note)
	assert.NoError(t, err)

	// For Contact subjects we do NOT set an owner reference because Contact is
	// namespace-scoped and Note is cluster-scoped; Kubernetes does not allow a
	// cluster-scoped resource to have a namespace-scoped owner.
	assert.Len(t, note.OwnerReferences, 0, "expected no owner references for contact subject")

	// CreatorRef should be populated from the admission user
	assert.Equal(t, creatorUser.Name, note.Spec.CreatorRef.Name)
}

func TestNoteMutator_Default_ContactSubject_ContactNotFound(t *testing.T) {
	creatorUser := &iamv1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{
			Name: "creator-user",
			UID:  types.UID("creator-user-uid"),
		},
	}

	note := &crmv1alpha1.Note{
		ObjectMeta: metav1.ObjectMeta{
			Name: "note-missing-contact",
		},
		Spec: crmv1alpha1.NoteSpec{
			Content: "test note",
			SubjectRef: crmv1alpha1.SubjectReference{
				APIGroup:  "notification.miloapis.com",
				Kind:      "Contact",
				Name:      "missing-contact",
				Namespace: "contacts-ns",
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(runtimeScheme).
		WithObjects(creatorUser).
		Build()

	mutator := &NoteMutator{
		Client: fakeClient,
		Scheme: runtimeScheme,
	}

	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UserInfo: authenticationv1.UserInfo{
				UID: creatorUser.Name,
			},
		},
	}
	ctx := admission.NewContextWithRequest(context.Background(), req)

	err := mutator.Default(ctx, note)
	assert.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "failed to get referenced contact"))
}

func TestNoteValidator_ValidateCreate(t *testing.T) {
	now := time.Now().UTC()
	past := metav1.NewTime(now.Add(-1 * time.Hour))
	future := metav1.NewTime(now.Add(1 * time.Hour))

	tests := map[string]struct {
		note          *crmv1alpha1.Note
		seedObjects   []client.Object
		expectError   bool
		errorContains string
	}{
		"valid user subject with no nextActionTime": {
			note: &crmv1alpha1.Note{
				ObjectMeta: metav1.ObjectMeta{
					Name: "valid-user-note",
				},
				Spec: crmv1alpha1.NoteSpec{
					Content: "content",
					SubjectRef: crmv1alpha1.SubjectReference{
						APIGroup: "iam.miloapis.com",
						Kind:     "User",
						Name:     "existing-user",
					},
				},
			},
			seedObjects: []client.Object{
				&iamv1alpha1.User{
					ObjectMeta: metav1.ObjectMeta{Name: "existing-user"},
				},
			},
			expectError: false,
		},
		"error when nextActionTime is in the past": {
			note: &crmv1alpha1.Note{
				ObjectMeta: metav1.ObjectMeta{
					Name: "past-next-action",
				},
				Spec: crmv1alpha1.NoteSpec{
					Content: "content",
					SubjectRef: crmv1alpha1.SubjectReference{
						APIGroup: "iam.miloapis.com",
						Kind:     "User",
						Name:     "existing-user",
					},
					NextActionTime: &past,
				},
			},
			seedObjects: []client.Object{
				&iamv1alpha1.User{
					ObjectMeta: metav1.ObjectMeta{Name: "existing-user"},
				},
			},
			expectError:   true,
			errorContains: "nextActionTime cannot be in the past",
		},
		"valid when nextActionTime is in the future": {
			note: &crmv1alpha1.Note{
				ObjectMeta: metav1.ObjectMeta{
					Name: "future-next-action",
				},
				Spec: crmv1alpha1.NoteSpec{
					Content: "content",
					SubjectRef: crmv1alpha1.SubjectReference{
						APIGroup: "iam.miloapis.com",
						Kind:     "User",
						Name:     "existing-user",
					},
					NextActionTime: &future,
				},
			},
			seedObjects: []client.Object{
				&iamv1alpha1.User{
					ObjectMeta: metav1.ObjectMeta{Name: "existing-user"},
				},
			},
			expectError: false,
		},
		"error when iam apiGroup has non-User kind": {
			note: &crmv1alpha1.Note{
				ObjectMeta: metav1.ObjectMeta{
					Name: "wrong-kind-iam",
				},
				Spec: crmv1alpha1.NoteSpec{
					Content: "content",
					SubjectRef: crmv1alpha1.SubjectReference{
						APIGroup: "iam.miloapis.com",
						Kind:     "Contact",
						Name:     "any",
					},
				},
			},
			expectError:   true,
			errorContains: "kind must be User for iam.miloapis.com",
		},
		"error when user subject does not exist": {
			note: &crmv1alpha1.Note{
				ObjectMeta: metav1.ObjectMeta{
					Name: "missing-user",
				},
				Spec: crmv1alpha1.NoteSpec{
					Content: "content",
					SubjectRef: crmv1alpha1.SubjectReference{
						APIGroup: "iam.miloapis.com",
						Kind:     "User",
						Name:     "nonexistent",
					},
				},
			},
			expectError:   true,
			errorContains: "not found",
		},
		"valid contact subject with existing contact": {
			note: &crmv1alpha1.Note{
				ObjectMeta: metav1.ObjectMeta{
					Name: "contact-note",
				},
				Spec: crmv1alpha1.NoteSpec{
					Content: "content",
					SubjectRef: crmv1alpha1.SubjectReference{
						APIGroup:  "notification.miloapis.com",
						Kind:      "Contact",
						Name:      "contact-1",
						Namespace: "contacts-ns",
					},
				},
			},
			seedObjects: []client.Object{
				&notificationv1alpha1.Contact{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "contact-1",
						Namespace: "contacts-ns",
					},
				},
			},
			expectError: false,
		},
		"error when notification apiGroup has non-Contact kind": {
			note: &crmv1alpha1.Note{
				ObjectMeta: metav1.ObjectMeta{
					Name: "wrong-kind-notification",
				},
				Spec: crmv1alpha1.NoteSpec{
					Content: "content",
					SubjectRef: crmv1alpha1.SubjectReference{
						APIGroup: "notification.miloapis.com",
						Kind:     "User",
						Name:     "any",
					},
				},
			},
			expectError:   true,
			errorContains: "kind must be Contact for notification.miloapis.com",
		},
		"error when contact subject is missing namespace": {
			note: &crmv1alpha1.Note{
				ObjectMeta: metav1.ObjectMeta{
					Name: "contact-missing-namespace",
				},
				Spec: crmv1alpha1.NoteSpec{
					Content: "content",
					SubjectRef: crmv1alpha1.SubjectReference{
						APIGroup: "notification.miloapis.com",
						Kind:     "Contact",
						Name:     "contact-1",
					},
				},
			},
			expectError:   true,
			errorContains: "namespace is required for Contact",
		},
		"error when contact subject does not exist": {
			note: &crmv1alpha1.Note{
				ObjectMeta: metav1.ObjectMeta{
					Name: "missing-contact",
				},
				Spec: crmv1alpha1.NoteSpec{
					Content: "content",
					SubjectRef: crmv1alpha1.SubjectReference{
						APIGroup:  "notification.miloapis.com",
						Kind:      "Contact",
						Name:      "missing",
						Namespace: "contacts-ns",
					},
				},
			},
			expectError:   true,
			errorContains: "not found",
		},
		"error when apiGroup is invalid": {
			note: &crmv1alpha1.Note{
				ObjectMeta: metav1.ObjectMeta{
					Name: "invalid-apigroup",
				},
				Spec: crmv1alpha1.NoteSpec{
					Content: "content",
					SubjectRef: crmv1alpha1.SubjectReference{
						APIGroup: "unknown.group",
						Kind:     "User",
						Name:     "any",
					},
				},
			},
			expectError:   true,
			errorContains: "apiGroup must be iam.miloapis.com or notification.miloapis.com",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			builder := fake.NewClientBuilder().WithScheme(runtimeScheme)
			if len(tt.seedObjects) > 0 {
				builder = builder.WithObjects(tt.seedObjects...)
			}
			fakeClient := builder.Build()

			validator := &NoteValidator{Client: fakeClient}
			_, err := validator.ValidateCreate(context.Background(), tt.note)

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

func TestNoteValidator_ValidateUpdate_UsesSameLogicAsCreate(t *testing.T) {
	// Reuse a simple valid case to ensure update path delegates to validateNote
	existingUser := &iamv1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{Name: "existing-user"},
	}

	noteOld := &crmv1alpha1.Note{
		ObjectMeta: metav1.ObjectMeta{
			Name: "update-note",
		},
		Spec: crmv1alpha1.NoteSpec{
			Content: "old content",
			SubjectRef: crmv1alpha1.SubjectReference{
				APIGroup: "iam.miloapis.com",
				Kind:     "User",
				Name:     "existing-user",
			},
		},
	}

	noteNew := noteOld.DeepCopy()
	noteNew.Spec.Content = "new content"

	fakeClient := fake.NewClientBuilder().
		WithScheme(runtimeScheme).
		WithObjects(existingUser).
		Build()

	validator := &NoteValidator{Client: fakeClient}
	warnings, err := validator.ValidateUpdate(context.Background(), noteOld, noteNew)
	assert.NoError(t, err)
	assert.Empty(t, warnings)
}
