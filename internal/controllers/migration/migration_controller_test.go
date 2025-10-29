package migration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	iamv1alpha1 "go.miloapis.com/milo/pkg/apis/iam/v1alpha1"
	resourcemanagerv1alpha1 "go.miloapis.com/milo/pkg/apis/resourcemanager/v1alpha1"
)

func TestFindLegacyBindings(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = resourcemanagerv1alpha1.AddToScheme(scheme)
	_ = iamv1alpha1.AddToScheme(scheme)

	user := &iamv1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-user",
			UID:  "user-123",
		},
	}

	org := &resourcemanagerv1alpha1.Organization{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-org",
			UID:  "org-123",
		},
	}

	membership := &resourcemanagerv1alpha1.OrganizationMembership{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-membership",
			Namespace: "org-test-org",
		},
		Spec: resourcemanagerv1alpha1.OrganizationMembershipSpec{
			OrganizationRef: resourcemanagerv1alpha1.OrganizationReference{Name: "test-org"},
			UserRef:         resourcemanagerv1alpha1.MemberReference{Name: "test-user"},
		},
	}

	// Legacy binding
	legacyBinding := &iamv1alpha1.PolicyBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "legacy",
			Namespace: "org-test-org",
		},
		Spec: iamv1alpha1.PolicyBindingSpec{
			RoleRef: iamv1alpha1.RoleReference{Name: "owner", Namespace: "org-test-org"},
			Subjects: []iamv1alpha1.Subject{
				{Kind: "User", Name: "test-user", UID: "user-123"},
			},
			ResourceSelector: iamv1alpha1.ResourceSelector{
				ResourceRef: &iamv1alpha1.ResourceReference{
					APIGroup: "resourcemanager.miloapis.com",
					Kind:     "Organization",
					Name:     "test-org",
					UID:      "org-123",
				},
			},
		},
	}

	// Managed binding (should be skipped)
	managedBinding := &iamv1alpha1.PolicyBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "managed",
			Namespace: "org-test-org",
			Labels: map[string]string{
				ManagedByLabel: ManagedByValue,
			},
		},
		Spec: iamv1alpha1.PolicyBindingSpec{
			RoleRef: iamv1alpha1.RoleReference{Name: "viewer", Namespace: "org-test-org"},
			Subjects: []iamv1alpha1.Subject{
				{Kind: "User", Name: "test-user", UID: "user-123"},
			},
			ResourceSelector: iamv1alpha1.ResourceSelector{
				ResourceRef: &iamv1alpha1.ResourceReference{
					APIGroup: "resourcemanager.miloapis.com",
					Kind:     "Organization",
					Name:     "test-org",
					UID:      "org-123",
				},
			},
		},
	}

	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(user, org, membership, legacyBinding, managedBinding).
		Build()

	controller := &MigrationController{Client: client}

	bindings, err := controller.findLegacyBindings(context.Background(), membership)

	require.NoError(t, err)
	assert.Len(t, bindings, 1)
	assert.Equal(t, "legacy", bindings[0].Name)
}

func TestExtractRoles(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = resourcemanagerv1alpha1.AddToScheme(scheme)
	_ = iamv1alpha1.AddToScheme(scheme)

	ownerRole := &iamv1alpha1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "owner",
			Namespace: "org-test",
		},
	}

	viewerRole := &iamv1alpha1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "viewer",
			Namespace: "org-test",
		},
	}

	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(ownerRole, viewerRole).
		Build()

	controller := &MigrationController{Client: client}

	bindings := []iamv1alpha1.PolicyBinding{
		{
			Spec: iamv1alpha1.PolicyBindingSpec{
				RoleRef: iamv1alpha1.RoleReference{Name: "owner", Namespace: "org-test"},
			},
		},
		{
			Spec: iamv1alpha1.PolicyBindingSpec{
				RoleRef: iamv1alpha1.RoleReference{Name: "viewer", Namespace: "org-test"},
			},
		},
		{
			// Duplicate - should be deduplicated
			Spec: iamv1alpha1.PolicyBindingSpec{
				RoleRef: iamv1alpha1.RoleReference{Name: "owner", Namespace: "org-test"},
			},
		},
	}

	roles := controller.extractRoles(context.Background(), bindings)

	assert.Len(t, roles, 2)
	roleNames := map[string]bool{}
	for _, r := range roles {
		roleNames[r.Name] = true
	}
	assert.True(t, roleNames["owner"])
	assert.True(t, roleNames["viewer"])
}
