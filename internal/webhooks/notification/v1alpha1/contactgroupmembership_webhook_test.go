package v1alpha1

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	iamv1alpha1 "go.miloapis.com/milo/pkg/apis/iam/v1alpha1"
	notificationv1alpha1 "go.miloapis.com/milo/pkg/apis/notification/v1alpha1"
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var cgmTestScheme = runtime.NewScheme()

func init() {
	utilruntime.Must(notificationv1alpha1.AddToScheme(cgmTestScheme))
}

func TestContactGroupMembershipValidator(t *testing.T) {
	makeMembership := func(name, contact, group string) *notificationv1alpha1.ContactGroupMembership {
		return &notificationv1alpha1.ContactGroupMembership{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
			Spec: notificationv1alpha1.ContactGroupMembershipSpec{
				ContactRef: notificationv1alpha1.ContactReference{
					Name:      contact,
					Namespace: "default",
				},
				ContactGroupRef: notificationv1alpha1.ContactGroupReference{
					Name:      group,
					Namespace: "default",
				},
			},
		}
	}

	makeContact := func(name string) *notificationv1alpha1.Contact {
		return &notificationv1alpha1.Contact{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"}}
	}

	makeGroup := func(name string) *notificationv1alpha1.ContactGroup {
		return &notificationv1alpha1.ContactGroup{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"}}
	}

	tests := map[string]struct {
		newObj        *notificationv1alpha1.ContactGroupMembership
		seedObjects   []client.Object
		expectError   bool
		errorContains string
	}{
		"valid membership": {
			newObj:      makeMembership("m1", "c1", "g1"),
			seedObjects: []client.Object{makeContact("c1"), makeGroup("g1")},
			expectError: false,
		},
		"duplicate membership": {
			newObj: makeMembership("m2", "c1", "g1"),
			seedObjects: []client.Object{
				makeContact("c1"),
				makeGroup("g1"),
				makeMembership("existing", "c1", "g1"),
			},
			expectError:   true,
			errorContains: "membership already exists",
		},
		"removal already exists": {
			newObj: makeMembership("m7", "c1", "g1"),
			seedObjects: []client.Object{
				makeContact("c1"),
				makeGroup("g1"),
				&notificationv1alpha1.ContactGroupMembershipRemoval{
					ObjectMeta: metav1.ObjectMeta{Name: "rm1", Namespace: "default"},
					Spec: notificationv1alpha1.ContactGroupMembershipRemovalSpec{
						ContactRef:      notificationv1alpha1.ContactReference{Name: "c1", Namespace: "default"},
						ContactGroupRef: notificationv1alpha1.ContactGroupReference{Name: "g1", Namespace: "default"},
					},
				},
			},
			expectError:   true,
			errorContains: "cannot create membership",
		},
		"different group ok": {
			newObj: makeMembership("m3", "c1", "g2"),
			seedObjects: []client.Object{
				makeContact("c1"),
				makeGroup("g1"),
				makeGroup("g2"),
				makeMembership("existing", "c1", "g1"),
			},
			expectError: false,
		},
		"contact not found": {
			newObj:        makeMembership("m4", "c-missing", "g1"),
			seedObjects:   []client.Object{makeGroup("g1")},
			expectError:   true,
			errorContains: "not found",
		},
		"group not found": {
			newObj:        makeMembership("m5", "c1", "g-missing"),
			seedObjects:   []client.Object{makeContact("c1")},
			expectError:   true,
			errorContains: "not found",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			builder := fake.NewClientBuilder().WithScheme(cgmTestScheme).
				WithIndex(&notificationv1alpha1.ContactGroupMembership{}, contactMembershipCompositeKey, func(o client.Object) []string {
					c := o.(*notificationv1alpha1.ContactGroupMembership)
					return []string{buildContactGroupTupleKey(c.Spec.ContactRef, c.Spec.ContactGroupRef)}
				}).
				WithIndex(&notificationv1alpha1.ContactGroupMembershipRemoval{}, contactMembershipRemovalCompositeKey, func(o client.Object) []string {
					r := o.(*notificationv1alpha1.ContactGroupMembershipRemoval)
					return []string{buildContactGroupTupleKey(r.Spec.ContactRef, r.Spec.ContactGroupRef)}
				})
			if len(tt.seedObjects) > 0 {
				builder = builder.WithObjects(tt.seedObjects...)
			}
			fakeClient := builder.Build()
			v := &ContactGroupMembershipValidator{Client: fakeClient}
			_, err := v.ValidateCreate(context.Background(), tt.newObj)
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

func TestContactGroupMembershipValidator_UserContext(t *testing.T) {
	makeMembership := func(name, contact, group string) *notificationv1alpha1.ContactGroupMembership {
		return &notificationv1alpha1.ContactGroupMembership{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
			Spec: notificationv1alpha1.ContactGroupMembershipSpec{
				ContactRef: notificationv1alpha1.ContactReference{
					Name:      contact,
					Namespace: "default",
				},
				ContactGroupRef: notificationv1alpha1.ContactGroupReference{
					Name:      group,
					Namespace: "default",
				},
			},
		}
	}

	makeContactWithSubject := func(name, subjectName string) *notificationv1alpha1.Contact {
		return &notificationv1alpha1.Contact{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
			Spec: notificationv1alpha1.ContactSpec{
				SubjectRef: &notificationv1alpha1.SubjectReference{
					APIGroup: "iam.miloapis.com",
					Kind:     "User",
					Name:     subjectName,
				},
			},
		}
	}

	makeGroup := func(name string) *notificationv1alpha1.ContactGroup {
		return &notificationv1alpha1.ContactGroup{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"}}
	}

	tests := map[string]struct {
		newObj        *notificationv1alpha1.ContactGroupMembership
		seedObjects   []client.Object
		userID        string
		hasUserCtx    bool
		expectError   bool
		errorContains string
	}{
		"user context: user creates membership for own contact": {
			newObj:      makeMembership("m1", "alice-contact", "g1"),
			seedObjects: []client.Object{makeContactWithSubject("alice-contact", "alice"), makeGroup("g1")},
			userID:      "alice",
			hasUserCtx:  true,
			expectError: false,
		},
		"user context: user tries to create membership for other user's contact": {
			newObj:        makeMembership("m2", "bob-contact", "g1"),
			seedObjects:   []client.Object{makeContactWithSubject("bob-contact", "bob"), makeGroup("g1")},
			userID:        "alice",
			hasUserCtx:    true,
			expectError:   true,
			errorContains: "you do not have permission to create membership for this contact",
		},
		"no user context: admin creates membership for any user": {
			newObj:      makeMembership("m3", "bob-contact", "g1"),
			seedObjects: []client.Object{makeContactWithSubject("bob-contact", "bob"), makeGroup("g1")},
			userID:      "admin",
			hasUserCtx:  false,
			expectError: false,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			builder := fake.NewClientBuilder().WithScheme(cgmTestScheme).
				WithIndex(&notificationv1alpha1.ContactGroupMembership{}, contactMembershipCompositeKey, func(o client.Object) []string {
					c := o.(*notificationv1alpha1.ContactGroupMembership)
					return []string{buildContactGroupTupleKey(c.Spec.ContactRef, c.Spec.ContactGroupRef)}
				}).
				WithIndex(&notificationv1alpha1.ContactGroupMembershipRemoval{}, contactMembershipRemovalCompositeKey, func(o client.Object) []string {
					r := o.(*notificationv1alpha1.ContactGroupMembershipRemoval)
					return []string{buildContactGroupTupleKey(r.Spec.ContactRef, r.Spec.ContactGroupRef)}
				})
			if len(tt.seedObjects) > 0 {
				builder = builder.WithObjects(tt.seedObjects...)
			}
			fakeClient := builder.Build()
			v := &ContactGroupMembershipValidator{Client: fakeClient}

			// Create admission request with user context
			req := admission.Request{
				AdmissionRequest: admissionv1.AdmissionRequest{
					UserInfo: authenticationv1.UserInfo{
						UID: tt.userID,
					},
				},
			}

			// Add user context extra data if needed
			if tt.hasUserCtx {
				req.UserInfo.Extra = map[string]authenticationv1.ExtraValue{
					iamv1alpha1.ParentNameExtraKey: []string{tt.userID},
				}
			}

			ctx := admission.NewContextWithRequest(context.Background(), req)
			_, err := v.ValidateCreate(ctx, tt.newObj)
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
