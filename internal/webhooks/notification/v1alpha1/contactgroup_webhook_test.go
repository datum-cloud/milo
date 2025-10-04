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

var cgScheme = runtime.NewScheme()

func init() {
	utilruntime.Must(notificationv1alpha1.AddToScheme(cgScheme))
}

func TestContactGroupValidator_ValidateCreate(t *testing.T) {
	tests := map[string]struct {
		newCG         *notificationv1alpha1.ContactGroup
		seedObjects   []client.Object
		expectError   bool
		errorContains string
	}{
		"unique display name": {
			newCG: &notificationv1alpha1.ContactGroup{
				ObjectMeta: metav1.ObjectMeta{Name: "cg1"},
				Spec: notificationv1alpha1.ContactGroupSpec{
					DisplayName: "Engineering",
					Visibility:  notificationv1alpha1.ContactGroupVisibilityPublic,
				},
			},
			expectError: false,
		},
		"duplicate display and visibility": {
			newCG: &notificationv1alpha1.ContactGroup{
				ObjectMeta: metav1.ObjectMeta{Name: "cg2"},
				Spec: notificationv1alpha1.ContactGroupSpec{
					DisplayName: "Engineering",
					Visibility:  notificationv1alpha1.ContactGroupVisibilityPublic,
				},
			},
			seedObjects: []client.Object{
				&notificationv1alpha1.ContactGroup{
					ObjectMeta: metav1.ObjectMeta{Name: "cg-existing"},
					Spec: notificationv1alpha1.ContactGroupSpec{
						DisplayName: "Engineering",
						Visibility:  notificationv1alpha1.ContactGroupVisibilityPublic,
					},
				},
			},
			expectError:   true,
			errorContains: "has this display name and visibility in the same namespace",
		},
		"same name diff visibility allowed": {
			newCG: &notificationv1alpha1.ContactGroup{
				ObjectMeta: metav1.ObjectMeta{Name: "cg3"},
				Spec: notificationv1alpha1.ContactGroupSpec{
					DisplayName: "Engineering",
					Visibility:  notificationv1alpha1.ContactGroupVisibilityPrivate,
				},
			},
			seedObjects: []client.Object{
				&notificationv1alpha1.ContactGroup{
					ObjectMeta: metav1.ObjectMeta{Name: "cg-existing"},
					Spec: notificationv1alpha1.ContactGroupSpec{
						DisplayName: "Engineering",
						Visibility:  notificationv1alpha1.ContactGroupVisibilityPublic,
					},
				},
			},
			expectError: false,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			builder := fake.NewClientBuilder().WithScheme(cgScheme).
				WithIndex(&notificationv1alpha1.ContactGroup{}, contactGroupSpecKey, func(o client.Object) []string {
					cg := o.(*notificationv1alpha1.ContactGroup)
					return []string{buildContactGroupSpecKey(*cg)}
				})
			if len(tt.seedObjects) > 0 {
				builder = builder.WithObjects(tt.seedObjects...)
			}
			fakeClient := builder.Build()

			validator := &ContactGroupValidator{Client: fakeClient}
			_, err := validator.ValidateCreate(context.Background(), tt.newCG)

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
