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

var cgrTestScheme = runtime.NewScheme()

func init() {
	utilruntime.Must(notificationv1alpha1.AddToScheme(cgrTestScheme))
}

func TestContactGroupMembershipRemovalValidator(t *testing.T) {
	makeRemoval := func(name, contact, group string) *notificationv1alpha1.ContactGroupMembershipRemoval {
		return &notificationv1alpha1.ContactGroupMembershipRemoval{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
			Spec: notificationv1alpha1.ContactGroupMembershipRemovalSpec{
				ContactRef:      notificationv1alpha1.ContactReference{Name: contact, Namespace: "default"},
				ContactGroupRef: notificationv1alpha1.ContactGroupReference{Name: group, Namespace: "default"},
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
		newObj        *notificationv1alpha1.ContactGroupMembershipRemoval
		seedObjects   []client.Object
		expectError   bool
		errorContains string
	}{
		"valid removal": {
			newObj:      makeRemoval("rm1", "c1", "g1"),
			seedObjects: []client.Object{makeContact("c1"), makeGroup("g1")},
		},
		"duplicate removal": {
			newObj:        makeRemoval("rm2", "c1", "g1"),
			seedObjects:   []client.Object{makeContact("c1"), makeGroup("g1"), makeRemoval("existing", "c1", "g1")},
			expectError:   true,
			errorContains: "already exists",
		},
		"contact missing": {
			newObj:        makeRemoval("rm3", "c-miss", "g1"),
			seedObjects:   []client.Object{makeGroup("g1")},
			expectError:   true,
			errorContains: "not found",
		},
		"group missing": {
			newObj:        makeRemoval("rm4", "c1", "g-miss"),
			seedObjects:   []client.Object{makeContact("c1")},
			expectError:   true,
			errorContains: "not found",
		},
		"same group different contact": {
			newObj: makeRemoval("rm5", "c2", "g1"),
			seedObjects: []client.Object{
				makeContact("c1"), makeContact("c2"), makeGroup("g1"),
				makeRemoval("existing", "c1", "g1"),
			},
			expectError: false,
		},
		"same contact different group": {
			newObj: makeRemoval("rm6", "c1", "g2"),
			seedObjects: []client.Object{
				makeContact("c1"), makeGroup("g1"), makeGroup("g2"),
				makeRemoval("existing", "c1", "g1"),
			},
			expectError: false,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			builder := fake.NewClientBuilder().WithScheme(cgrTestScheme).
				WithIndex(&notificationv1alpha1.ContactGroupMembershipRemoval{}, removalContactIndexKey, func(o client.Object) []string {
					r := o.(*notificationv1alpha1.ContactGroupMembershipRemoval)
					return []string{r.Spec.ContactRef.Name}
				})
			if len(tt.seedObjects) > 0 {
				builder = builder.WithObjects(tt.seedObjects...)
			}
			cl := builder.Build()
			v := &ContactGroupMembershipRemovalValidator{Client: cl}
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

	t.Run("update rejected", func(t *testing.T) {
		cl := fake.NewClientBuilder().WithScheme(cgrTestScheme).Build()
		v := &ContactGroupMembershipRemovalValidator{Client: cl}
		obj := makeRemoval("rm5", "c1", "g1")
		_, err := v.ValidateUpdate(context.Background(), obj, obj)
		assert.Error(t, err)
		assert.Contains(t, strings.ToLower(err.Error()), "immutable")
	})
}
