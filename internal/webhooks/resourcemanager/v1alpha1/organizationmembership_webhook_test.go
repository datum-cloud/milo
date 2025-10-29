package v1alpha1

import (
	"context"
	"testing"

	iamv1alpha1 "go.miloapis.com/milo/pkg/apis/iam/v1alpha1"
	resourcemanagerv1alpha1 "go.miloapis.com/milo/pkg/apis/resourcemanager/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// getWebhookTestScheme returns a runtime.Scheme for webhook testing
func getWebhookTestScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = iamv1alpha1.AddToScheme(scheme)
	_ = resourcemanagerv1alpha1.AddToScheme(scheme)
	return scheme
}

func TestOrganizationMembershipValidator_ValidateCreate_Success(t *testing.T) {
	ctx := context.TODO()
	scheme := getWebhookTestScheme()

	// Create test role
	role := &iamv1alpha1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "viewer-role",
			Namespace: "organization-test",
		},
		Spec: iamv1alpha1.RoleSpec{
			LaunchStage: "Stable",
			IncludedPermissions: []string{
				"resourcemanager.miloapis.com/organizations.get",
			},
		},
	}

	// Create membership with valid role
	membership := &resourcemanagerv1alpha1.OrganizationMembership{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-membership",
			Namespace: "organization-test",
		},
		Spec: resourcemanagerv1alpha1.OrganizationMembershipSpec{
			OrganizationRef: resourcemanagerv1alpha1.OrganizationReference{
				Name: "test",
			},
			UserRef: resourcemanagerv1alpha1.MemberReference{
				Name: "test-user",
			},
			Roles: []resourcemanagerv1alpha1.RoleReference{
				{
					Name:      "viewer-role",
					Namespace: "organization-test",
				},
			},
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(role).
		Build()

	validator := &OrganizationMembershipValidator{
		client: c,
	}

	// Test validation
	warnings, err := validator.ValidateCreate(ctx, membership)
	if err != nil {
		t.Fatalf("ValidateCreate failed: %v", err)
	}
	if len(warnings) > 0 {
		t.Errorf("Expected no warnings, got %d", len(warnings))
	}
}

func TestOrganizationMembershipValidator_ValidateCreate_DuplicateRoles(t *testing.T) {
	ctx := context.TODO()
	scheme := getWebhookTestScheme()

	// Create membership with duplicate roles
	membership := &resourcemanagerv1alpha1.OrganizationMembership{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-membership",
			Namespace: "organization-test",
		},
		Spec: resourcemanagerv1alpha1.OrganizationMembershipSpec{
			OrganizationRef: resourcemanagerv1alpha1.OrganizationReference{
				Name: "test",
			},
			UserRef: resourcemanagerv1alpha1.MemberReference{
				Name: "test-user",
			},
			Roles: []resourcemanagerv1alpha1.RoleReference{
				{
					Name:      "viewer-role",
					Namespace: "organization-test",
				},
				{
					Name:      "viewer-role",
					Namespace: "organization-test",
				},
			},
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	validator := &OrganizationMembershipValidator{
		client: c,
	}

	// Test validation - should fail with duplicate roles
	_, err := validator.ValidateCreate(ctx, membership)
	if err == nil {
		t.Fatal("Expected validation to fail with duplicate roles, but it passed")
	}
	if err.Error() != "duplicate role reference detected: viewer-role in namespace organization-test" {
		t.Errorf("Expected duplicate role error, got: %v", err)
	}
}

func TestOrganizationMembershipValidator_ValidateCreate_NonexistentRole(t *testing.T) {
	ctx := context.TODO()
	scheme := getWebhookTestScheme()

	// Create membership with nonexistent role
	membership := &resourcemanagerv1alpha1.OrganizationMembership{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-membership",
			Namespace: "organization-test",
		},
		Spec: resourcemanagerv1alpha1.OrganizationMembershipSpec{
			OrganizationRef: resourcemanagerv1alpha1.OrganizationReference{
				Name: "test",
			},
			UserRef: resourcemanagerv1alpha1.MemberReference{
				Name: "test-user",
			},
			Roles: []resourcemanagerv1alpha1.RoleReference{
				{
					Name:      "nonexistent-role",
					Namespace: "organization-test",
				},
			},
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	validator := &OrganizationMembershipValidator{
		client: c,
	}

	// Test validation - should fail with nonexistent role
	_, err := validator.ValidateCreate(ctx, membership)
	if err == nil {
		t.Fatal("Expected validation to fail with nonexistent role, but it passed")
	}
	if err.Error() != "role 'nonexistent-role' not found in namespace 'organization-test'" {
		t.Errorf("Expected nonexistent role error, got: %v", err)
	}
}

func TestOrganizationMembershipValidator_ValidateCreate_EmptyRoleName(t *testing.T) {
	ctx := context.TODO()
	scheme := getWebhookTestScheme()

	// Create membership with empty role name
	membership := &resourcemanagerv1alpha1.OrganizationMembership{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-membership",
			Namespace: "organization-test",
		},
		Spec: resourcemanagerv1alpha1.OrganizationMembershipSpec{
			OrganizationRef: resourcemanagerv1alpha1.OrganizationReference{
				Name: "test",
			},
			UserRef: resourcemanagerv1alpha1.MemberReference{
				Name: "test-user",
			},
			Roles: []resourcemanagerv1alpha1.RoleReference{
				{
					Name:      "",
					Namespace: "organization-test",
				},
			},
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	validator := &OrganizationMembershipValidator{
		client: c,
	}

	// Test validation - should fail with empty role name
	_, err := validator.ValidateCreate(ctx, membership)
	if err == nil {
		t.Fatal("Expected validation to fail with empty role name, but it passed")
	}
	if err.Error() != "role name cannot be empty" {
		t.Errorf("Expected empty role name error, got: %v", err)
	}
}

func TestOrganizationMembershipValidator_ValidateCreate_MultipleRoles(t *testing.T) {
	ctx := context.TODO()
	scheme := getWebhookTestScheme()

	// Create test roles
	viewerRole := &iamv1alpha1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "viewer-role",
			Namespace: "organization-test",
		},
		Spec: iamv1alpha1.RoleSpec{
			LaunchStage: "Stable",
		},
	}

	editorRole := &iamv1alpha1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "editor-role",
			Namespace: "organization-test",
		},
		Spec: iamv1alpha1.RoleSpec{
			LaunchStage: "Stable",
		},
	}

	// Create membership with multiple valid roles
	membership := &resourcemanagerv1alpha1.OrganizationMembership{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-membership",
			Namespace: "organization-test",
		},
		Spec: resourcemanagerv1alpha1.OrganizationMembershipSpec{
			OrganizationRef: resourcemanagerv1alpha1.OrganizationReference{
				Name: "test",
			},
			UserRef: resourcemanagerv1alpha1.MemberReference{
				Name: "test-user",
			},
			Roles: []resourcemanagerv1alpha1.RoleReference{
				{
					Name:      "viewer-role",
					Namespace: "organization-test",
				},
				{
					Name:      "editor-role",
					Namespace: "organization-test",
				},
			},
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(viewerRole, editorRole).
		Build()

	validator := &OrganizationMembershipValidator{
		client: c,
	}

	// Test validation - should pass with multiple valid roles
	warnings, err := validator.ValidateCreate(ctx, membership)
	if err != nil {
		t.Fatalf("ValidateCreate failed with multiple roles: %v", err)
	}
	if len(warnings) > 0 {
		t.Errorf("Expected no warnings, got %d", len(warnings))
	}
}

func TestOrganizationMembershipValidator_ValidateCreate_CrossNamespaceRole(t *testing.T) {
	ctx := context.TODO()
	scheme := getWebhookTestScheme()

	// Create role in different namespace
	role := &iamv1alpha1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "shared-role",
			Namespace: "milo-system",
		},
		Spec: iamv1alpha1.RoleSpec{
			LaunchStage: "Stable",
		},
	}

	// Create membership referencing cross-namespace role
	membership := &resourcemanagerv1alpha1.OrganizationMembership{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-membership",
			Namespace: "organization-test",
		},
		Spec: resourcemanagerv1alpha1.OrganizationMembershipSpec{
			OrganizationRef: resourcemanagerv1alpha1.OrganizationReference{
				Name: "test",
			},
			UserRef: resourcemanagerv1alpha1.MemberReference{
				Name: "test-user",
			},
			Roles: []resourcemanagerv1alpha1.RoleReference{
				{
					Name:      "shared-role",
					Namespace: "milo-system",
				},
			},
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(role).
		Build()

	validator := &OrganizationMembershipValidator{
		client: c,
	}

	// Test validation - should pass with cross-namespace role
	warnings, err := validator.ValidateCreate(ctx, membership)
	if err != nil {
		t.Fatalf("ValidateCreate failed with cross-namespace role: %v", err)
	}
	if len(warnings) > 0 {
		t.Errorf("Expected no warnings, got %d", len(warnings))
	}
}

func TestOrganizationMembershipValidator_ValidateUpdate(t *testing.T) {
	ctx := context.TODO()
	scheme := getWebhookTestScheme()

	// Create test role
	role := &iamv1alpha1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "viewer-role",
			Namespace: "organization-test",
		},
		Spec: iamv1alpha1.RoleSpec{
			LaunchStage: "Stable",
		},
	}

	oldMembership := &resourcemanagerv1alpha1.OrganizationMembership{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-membership",
			Namespace: "organization-test",
		},
		Spec: resourcemanagerv1alpha1.OrganizationMembershipSpec{
			OrganizationRef: resourcemanagerv1alpha1.OrganizationReference{
				Name: "test",
			},
			UserRef: resourcemanagerv1alpha1.MemberReference{
				Name: "test-user",
			},
		},
	}

	newMembership := &resourcemanagerv1alpha1.OrganizationMembership{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-membership",
			Namespace: "organization-test",
		},
		Spec: resourcemanagerv1alpha1.OrganizationMembershipSpec{
			OrganizationRef: resourcemanagerv1alpha1.OrganizationReference{
				Name: "test",
			},
			UserRef: resourcemanagerv1alpha1.MemberReference{
				Name: "test-user",
			},
			Roles: []resourcemanagerv1alpha1.RoleReference{
				{
					Name:      "viewer-role",
					Namespace: "organization-test",
				},
			},
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(role).
		Build()

	validator := &OrganizationMembershipValidator{
		client: c,
	}

	// Test update validation
	warnings, err := validator.ValidateUpdate(ctx, oldMembership, newMembership)
	if err != nil {
		t.Fatalf("ValidateUpdate failed: %v", err)
	}
	if len(warnings) > 0 {
		t.Errorf("Expected no warnings, got %d", len(warnings))
	}
}

func TestOrganizationMembershipValidator_CheckDuplicateRoles(t *testing.T) {
	validator := &OrganizationMembershipValidator{}

	tests := []struct {
		name        string
		roles       []resourcemanagerv1alpha1.RoleReference
		namespace   string
		expectError bool
	}{
		{
			name: "no duplicates",
			roles: []resourcemanagerv1alpha1.RoleReference{
				{Name: "role1", Namespace: "ns1"},
				{Name: "role2", Namespace: "ns1"},
			},
			namespace:   "default",
			expectError: false,
		},
		{
			name: "duplicate with same namespace",
			roles: []resourcemanagerv1alpha1.RoleReference{
				{Name: "role1", Namespace: "ns1"},
				{Name: "role1", Namespace: "ns1"},
			},
			namespace:   "default",
			expectError: true,
		},
		{
			name: "same name different namespace",
			roles: []resourcemanagerv1alpha1.RoleReference{
				{Name: "role1", Namespace: "ns1"},
				{Name: "role1", Namespace: "ns2"},
			},
			namespace:   "default",
			expectError: false,
		},
		{
			name: "duplicate with empty namespace",
			roles: []resourcemanagerv1alpha1.RoleReference{
				{Name: "role1"},
				{Name: "role1"},
			},
			namespace:   "default",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			membership := &resourcemanagerv1alpha1.OrganizationMembership{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: tt.namespace,
				},
				Spec: resourcemanagerv1alpha1.OrganizationMembershipSpec{
					Roles: tt.roles,
				},
			}

			err := validator.checkDuplicateRoles(membership)
			if tt.expectError && err == nil {
				t.Error("Expected error for duplicate roles, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
		})
	}
}
