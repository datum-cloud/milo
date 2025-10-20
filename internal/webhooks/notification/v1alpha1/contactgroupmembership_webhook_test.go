package v1alpha1

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	notificationv1alpha1 "go.miloapis.com/milo/pkg/apis/notification/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
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
		"subjectref same namespace": {
			newObj: &notificationv1alpha1.ContactGroupMembership{
				ObjectMeta: metav1.ObjectMeta{Name: "m6", Namespace: "default"},
				Spec: notificationv1alpha1.ContactGroupMembershipSpec{
					ContactRef:      notificationv1alpha1.ContactReference{Name: "c-sub", Namespace: "default"},
					ContactGroupRef: notificationv1alpha1.ContactGroupReference{Name: "g-sub", Namespace: "default"},
				},
			},
			seedObjects: []client.Object{
				&notificationv1alpha1.Contact{
					ObjectMeta: metav1.ObjectMeta{Name: "c-sub", Namespace: "default"},
					Spec: notificationv1alpha1.ContactSpec{
						SubjectRef: &notificationv1alpha1.SubjectReference{APIGroup: "resourcemanager.miloapis.com", Kind: "Project", Name: "proj1"},
					},
				},
				&notificationv1alpha1.ContactGroup{ObjectMeta: metav1.ObjectMeta{Name: "g-sub", Namespace: "default"}},
			},
			expectError: false,
		},
		"subjectref namespace mismatch": {
			newObj: &notificationv1alpha1.ContactGroupMembership{
				ObjectMeta: metav1.ObjectMeta{Name: "m7", Namespace: "other"},
				Spec: notificationv1alpha1.ContactGroupMembershipSpec{
					ContactRef:      notificationv1alpha1.ContactReference{Name: "c-sub", Namespace: "default"},
					ContactGroupRef: notificationv1alpha1.ContactGroupReference{Name: "g-sub", Namespace: "other"},
				},
			},
			seedObjects: []client.Object{
				&notificationv1alpha1.Contact{
					ObjectMeta: metav1.ObjectMeta{Name: "c-sub", Namespace: "default"},
					Spec: notificationv1alpha1.ContactSpec{
						SubjectRef: &notificationv1alpha1.SubjectReference{APIGroup: "resourcemanager.miloapis.com", Kind: "Project", Name: "proj1"},
					},
				},
				&notificationv1alpha1.ContactGroup{ObjectMeta: metav1.ObjectMeta{Name: "g-sub", Namespace: "other"}},
			},
			expectError:   true,
			errorContains: "namespace must be the same",
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
