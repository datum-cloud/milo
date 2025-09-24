package iam

import (
	"context"
	"strings"
	"testing"

	iamv1alpha1 "go.miloapis.com/milo/pkg/apis/iam/v1alpha1"
	notificationv1alpha1 "go.miloapis.com/milo/pkg/apis/notification/v1alpha1"
	resourcemanagerv1alpha1 "go.miloapis.com/milo/pkg/apis/resourcemanager/v1alpha1"

	apierr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// getTestScheme returns a runtime.Scheme with all Milo APIs registered.
func getTestScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = iamv1alpha1.AddToScheme(scheme)
	_ = resourcemanagerv1alpha1.AddToScheme(scheme)
	_ = notificationv1alpha1.AddToScheme(scheme)
	return scheme
}

// TestUserInvitationController_createPolicyBinding verifies that createPolicyBinding creates a PolicyBinding CR.
func TestUserInvitationController_createPolicyBinding(t *testing.T) {
	ctx := context.TODO()
	scheme := getTestScheme()

	// Arrange test objects
	user := &iamv1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-user",
			UID:  types.UID("user-uid"),
		},
		Spec: iamv1alpha1.UserSpec{
			Email: "test@example.com",
		},
	}

	ui := &iamv1alpha1.UserInvitation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-invitation",
			Namespace: "default",
			UID:       types.UID("ui-uid"),
		},
		Spec: iamv1alpha1.UserInvitationSpec{
			Email: user.Spec.Email,
			OrganizationRef: resourcemanagerv1alpha1.OrganizationReference{
				Name: "test-org",
			},
		},
	}

	// Role that grants the ability to view the invitation. This must be included in uiRelatedRoles so that getResourceRef
	// points the PolicyBinding to the UserInvitation CR.
	roleRef := iamv1alpha1.RoleReference{
		Name:      "get-invitation-role",
		Namespace: "milo-system",
	}

	// Build fake client with initial objects.
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(user, ui).Build()

	uic := &UserInvitationController{
		Client:         c,
		uiRelatedRoles: []iamv1alpha1.RoleReference{roleRef},
	}

	// Act
	if err := uic.createPolicyBinding(ctx, user, ui, &roleRef); err != nil {
		t.Fatalf("createPolicyBinding returned error: %v", err)
	}

	// Assert – the PolicyBinding should exist with deterministic name
	expectedName := getDeterministicRoleName(&roleRef, *ui)
	pb := &iamv1alpha1.PolicyBinding{}
	if err := c.Get(ctx, types.NamespacedName{Name: expectedName, Namespace: roleRef.Namespace}, pb); err != nil {
		t.Fatalf("expected PolicyBinding %s to be created: %v", expectedName, err)
	}

	// Verify key fields
	if pb.Spec.RoleRef.Name != roleRef.Name || pb.Spec.RoleRef.Namespace != roleRef.Namespace {
		t.Errorf("PolicyBinding has unexpected RoleRef: %+v", pb.Spec.RoleRef)
	}
	if len(pb.Spec.Subjects) != 1 || pb.Spec.Subjects[0].Name != user.Name {
		t.Errorf("PolicyBinding has unexpected Subjects: %+v", pb.Spec.Subjects)
	}

	// Call createPolicyBinding again to ensure idempotency
	if err := uic.createPolicyBinding(ctx, user, ui, &roleRef); err != nil {
		t.Fatalf("second createPolicyBinding call returned error: %v", err)
	}

	// List PolicyBindings in the namespace to ensure only one exists
	var pbList iamv1alpha1.PolicyBindingList
	if err := c.List(ctx, &pbList, client.InNamespace(roleRef.Namespace)); err != nil {
		t.Fatalf("failed to list PolicyBindings: %v", err)
	}
	if len(pbList.Items) != 1 {
		t.Errorf("expected 1 PolicyBinding after idempotent call, got %d", len(pbList.Items))
	}
}

// TestUserInvitationController_createOrganizationMembership verifies that createOrganizationMembership creates an OrganizationMembership CR.
func TestUserInvitationController_createOrganizationMembership(t *testing.T) {
	ctx := context.TODO()
	scheme := getTestScheme()

	// Arrange test objects
	user := &iamv1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-user",
			UID:  types.UID("user-uid"),
		},
		Spec: iamv1alpha1.UserSpec{
			Email: "test@example.com",
		},
	}

	ui := &iamv1alpha1.UserInvitation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-invitation",
			Namespace: "default",
			UID:       types.UID("ui-uid"),
		},
		Spec: iamv1alpha1.UserInvitationSpec{
			Email: user.Spec.Email,
			OrganizationRef: resourcemanagerv1alpha1.OrganizationReference{
				Name: "test-org",
			},
		},
	}

	// Pre-create Organization so that OrganizationMembership namespace ("organization-<name>") is valid in case the test environment validates namespaces.
	org := &resourcemanagerv1alpha1.Organization{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-org",
			UID:  types.UID("org-uid"),
		},
	}

	// Build fake client with initial objects.
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(user, ui, org).Build()

	uic := &UserInvitationController{Client: c}

	// Act
	if err := uic.createOrganizationMembership(ctx, user, ui); err != nil {
		t.Fatalf("createOrganizationMembership returned error: %v", err)
	}

	// Assert – the OrganizationMembership should exist
	expectedName := "member-" + user.Name
	expectedNamespace := "organization-" + ui.Spec.OrganizationRef.Name
	om := &resourcemanagerv1alpha1.OrganizationMembership{}
	if err := c.Get(ctx, types.NamespacedName{Name: expectedName, Namespace: expectedNamespace}, om); err != nil {
		t.Fatalf("expected OrganizationMembership %s/%s to be created: %v", expectedNamespace, expectedName, err)
	}

	// Verify basic fields
	if om.Spec.UserRef.Name != user.Name {
		t.Errorf("OrganizationMembership has unexpected UserRef: %+v", om.Spec.UserRef)
	}
	if om.Spec.OrganizationRef.Name != ui.Spec.OrganizationRef.Name {
		t.Errorf("OrganizationMembership has unexpected OrganizationRef: %+v", om.Spec.OrganizationRef)
	}

	// Call createOrganizationMembership again to ensure idempotency
	if err := uic.createOrganizationMembership(ctx, user, ui); err != nil {
		t.Fatalf("second createOrganizationMembership call returned error: %v", err)
	}

	// List OrganizationMemberships in the namespace to ensure only one exists
	var omList resourcemanagerv1alpha1.OrganizationMembershipList
	if err := c.List(ctx, &omList, client.InNamespace(expectedNamespace)); err != nil {
		t.Fatalf("failed to list OrganizationMemberships: %v", err)
	}
	if len(omList.Items) != 1 {
		t.Errorf("expected 1 OrganizationMembership after idempotent call, got %d", len(omList.Items))
	}
}

// TestGetDeterministicResourceName verifies that the helper produces a stable deterministic name.
func TestUserInvitationController_getDeterministicResourceName(t *testing.T) {
	ui := iamv1alpha1.UserInvitation{
		ObjectMeta: metav1.ObjectMeta{
			UID:  types.UID("abc-123"),
			Name: "invitation-name",
		},
	}

	name1 := getDeterministicResourceName("role-a", ui)
	expected := "abc-123-role-a"
	if name1 != expected {
		t.Fatalf("expected %s, got %s", expected, name1)
	}

	// Calling again with same inputs should yield identical result (determinism)
	name2 := getDeterministicResourceName("role-a", ui)
	if name2 != name1 {
		t.Fatalf("deterministic function returned different results: %s vs %s", name1, name2)
	}

	// A different role name should change the output but still include the UID prefix
	name3 := getDeterministicResourceName("role-b", ui)
	if name3 == name1 {
		t.Fatalf("expected different names for different role inputs, got same %s", name3)
	}
	if wantPrefix := "abc-123-"; len(name3) <= len(wantPrefix) || name3[:len(wantPrefix)] != wantPrefix {
		t.Fatalf("expected name to start with %s, got %s", wantPrefix, name3)
	}
}

func TestUserInvitationController_getResourceRef(t *testing.T) {
	ctx := context.TODO()
	scheme := getTestScheme()

	// Shared objects
	org := &resourcemanagerv1alpha1.Organization{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-org",
			UID:  types.UID("org-uid"),
		},
	}

	ui := iamv1alpha1.UserInvitation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "inv",
			Namespace: "default",
			UID:       types.UID("ui-uid"),
		},
		Spec: iamv1alpha1.UserInvitationSpec{
			OrganizationRef: resourcemanagerv1alpha1.OrganizationReference{Name: org.Name},
			Email:           "test@example.com",
		},
	}

	invitationRole := iamv1alpha1.RoleReference{Name: "get-invitation-role", Namespace: "milo-system"}
	orgRole := iamv1alpha1.RoleReference{Name: "org-admin", Namespace: "milo-system"}

	cases := []struct {
		name          string
		roleRef       iamv1alpha1.RoleReference
		uiRelated     []iamv1alpha1.RoleReference
		withOrg       bool
		wantErr       bool
		wantKind      string
		wantName      string
		wantUID       string
		wantNamespace string
	}{
		{
			name:          "invitation related role",
			roleRef:       invitationRole,
			uiRelated:     []iamv1alpha1.RoleReference{invitationRole},
			withOrg:       true,
			wantKind:      "UserInvitation",
			wantName:      ui.Name,
			wantUID:       string(ui.UID),
			wantNamespace: ui.Namespace,
			wantErr:       false,
		},
		{
			name:          "organization role",
			roleRef:       orgRole,
			uiRelated:     []iamv1alpha1.RoleReference{invitationRole},
			withOrg:       true,
			wantKind:      "Organization",
			wantName:      org.Name,
			wantUID:       string(org.UID),
			wantNamespace: "",
			wantErr:       false,
		},
		{
			name:      "organization role but org missing",
			roleRef:   orgRole,
			uiRelated: []iamv1alpha1.RoleReference{invitationRole},
			withOrg:   false,
			wantErr:   true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			builder := fake.NewClientBuilder().WithScheme(scheme)
			if tc.withOrg {
				builder = builder.WithObjects(org)
			}
			c := builder.Build()

			uic := &UserInvitationController{
				Client:         c,
				uiRelatedRoles: tc.uiRelated,
			}

			ref, err := uic.getResourceRef(ctx, &tc.roleRef, ui)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error but got none, ref=%+v", ref)
				}
				return
			}
			if err != nil {
				t.Fatalf("getResourceRef returned error: %v", err)
			}

			if ref.Kind != tc.wantKind || ref.Name != tc.wantName || ref.UID != tc.wantUID || ref.Namespace != tc.wantNamespace {
				t.Fatalf("unexpected ResourceRef: %+v, want kind=%s name=%s uid=%s namespace=%s", ref, tc.wantKind, tc.wantName, tc.wantUID, tc.wantNamespace)
			}
		})
	}
}

// TestUserInvitationController_findUserInvitationsForUser tests mapping from User to related UserInvitations.
func TestUserInvitationController_findUserInvitationsForUser(t *testing.T) {
	ctx := context.TODO()
	scheme := getTestScheme()

	user := &iamv1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-user",
		},
		Spec: iamv1alpha1.UserSpec{Email: "Test@Example.com"},
	}

	ui1 := &iamv1alpha1.UserInvitation{
		ObjectMeta: metav1.ObjectMeta{Name: "inv1", Namespace: "default"},
		Spec: iamv1alpha1.UserInvitationSpec{
			Email:           "test@example.com", // lower-case matches user email case-insensitively
			OrganizationRef: resourcemanagerv1alpha1.OrganizationReference{Name: "org"},
		},
	}
	ui2 := &iamv1alpha1.UserInvitation{
		ObjectMeta: metav1.ObjectMeta{Name: "inv2", Namespace: "other-ns"},
		Spec: iamv1alpha1.UserInvitationSpec{
			Email:           "TEST@example.com", // upper-case variant
			OrganizationRef: resourcemanagerv1alpha1.OrganizationReference{Name: "org"},
		},
	}
	ui3 := &iamv1alpha1.UserInvitation{
		ObjectMeta: metav1.ObjectMeta{Name: "inv3", Namespace: "other-ns"},
		Spec: iamv1alpha1.UserInvitationSpec{
			Email:           "notused@example.com",
			OrganizationRef: resourcemanagerv1alpha1.OrganizationReference{Name: "org"},
		},
	}

	// Build fake client with userinvitations; user object does not need to be in the client for the list operation.
	builder := fake.NewClientBuilder().WithScheme(scheme).WithObjects(ui1, ui2, ui3)
	builder = builder.WithIndex(&iamv1alpha1.UserInvitation{}, userEmailIndexKey, func(obj client.Object) []string {
		ui := obj.(*iamv1alpha1.UserInvitation)
		return []string{strings.ToLower(ui.Spec.Email)}
	})
	c := builder.Build()

	uic := &UserInvitationController{Client: c}

	// Case 1: normal mapping
	reqs := uic.findUserInvitationsForUser(ctx, user)
	if len(reqs) != 2 {
		t.Fatalf("expected 2 reconcile requests, got %d", len(reqs))
	}

	// Ensure names collected are inv1 and inv2 regardless of order
	got := map[string]struct{}{}
	for _, r := range reqs {
		got[r.Name] = struct{}{}
	}
	if _, ok := got["inv1"]; !ok {
		t.Errorf("inv1 not found in requests: %v", reqs)
	}
	if _, ok := got["inv2"]; !ok {
		t.Errorf("inv2 not found in requests: %v", reqs)
	}

	// Case 2: user without email should return nil/empty slice
	userNoEmail := &iamv1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{Name: "no-email"},
		Spec:       iamv1alpha1.UserSpec{Email: "nonui@test.com"},
	}
	if r := uic.findUserInvitationsForUser(ctx, userNoEmail); len(r) != 0 {
		t.Errorf("expected 0 requests for user without email, got %d", len(r))
	}

	// Case 3: user with different email should return 0 requests
	userOther := &iamv1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{Name: "other"},
		Spec:       iamv1alpha1.UserSpec{Email: "other@example.com"},
	}
	if r := uic.findUserInvitationsForUser(ctx, userOther); len(r) != 0 {
		t.Errorf("expected 0 requests for user with different email, got %d", len(r))
	}

	// Case 3: unexpected object type returns nil
	dummy := &iamv1alpha1.UserInvitation{}
	if r := uic.findUserInvitationsForUser(ctx, dummy); r != nil {
		t.Errorf("expected nil for unexpected type, got %v", r)
	}
}

// Test_deletePolicyBinding verifies deletion behavior and idempotency.
func Test_deletePolicyBinding(t *testing.T) {
	ctx := context.TODO()
	scheme := getTestScheme()

	roleRef := &iamv1alpha1.RoleReference{Name: "get-invitation-role", Namespace: "milo-system"}

	ui := iamv1alpha1.UserInvitation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "inv",
			Namespace: "default",
			UID:       types.UID("ui-uid"),
		},
	}

	// Build PolicyBinding that should be deleted
	pbName := getDeterministicRoleName(roleRef, ui)
	pb := &iamv1alpha1.PolicyBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pbName,
			Namespace: roleRef.Namespace,
		},
	}

	// Case 1: resource exists then deleted, second delete is no-op
	clientWithPB := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pb).Build()

	if err := deletePolicyBinding(ctx, clientWithPB, roleRef, ui); err != nil {
		t.Fatalf("unexpected error deleting existing PolicyBinding: %v", err)
	}

	// Verify it is gone
	if err := clientWithPB.Get(ctx, types.NamespacedName{Name: pbName, Namespace: roleRef.Namespace}, &iamv1alpha1.PolicyBinding{}); !apierr.IsNotFound(err) {
		t.Fatalf("expected PolicyBinding to be deleted, got err=%v", err)
	}

	// Second call should still succeed (idempotent)
	if err := deletePolicyBinding(ctx, clientWithPB, roleRef, ui); err != nil {
		t.Fatalf("second deletePolicyBinding call returned error: %v", err)
	}

	// Case 2: resource never existed
	clientNoPB := fake.NewClientBuilder().WithScheme(scheme).Build()
	if err := deletePolicyBinding(ctx, clientNoPB, roleRef, ui); err != nil {
		t.Fatalf("deletePolicyBinding should succeed when resource absent, got: %v", err)
	}
}

// TestUserInvitationController_updateUserInvitationStatus verifies that status conditions are correctly updated and that repeated calls remain idempotent.
func TestUserInvitationController_updateUserInvitationStatus(t *testing.T) {
	ctx := context.TODO()
	scheme := getTestScheme()

	initialUI := &iamv1alpha1.UserInvitation{
		ObjectMeta: metav1.ObjectMeta{Name: "inv", Namespace: "default"},
		Spec: iamv1alpha1.UserInvitationSpec{
			Email:           "test2@example.com",
			OrganizationRef: resourcemanagerv1alpha1.OrganizationReference{Name: "org"},
			State:           iamv1alpha1.UserInvitationStatePending,
		},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).
		WithStatusSubresource(&iamv1alpha1.UserInvitation{}).
		WithObjects(initialUI.DeepCopy()).Build()

	uic := &UserInvitationController{Client: c}

	cond := metav1.Condition{
		Type:   string(iamv1alpha1.UserInvitationReadyCondition),
		Status: metav1.ConditionTrue,
		Reason: string(iamv1alpha1.UserInvitationStateExpiredReason),
	}

	// Fetch object from client to ensure ResourceVersion populated
	ui := &iamv1alpha1.UserInvitation{}
	_ = c.Get(ctx, types.NamespacedName{Name: initialUI.Name, Namespace: initialUI.Namespace}, ui)

	if meta.IsStatusConditionTrue(ui.Status.Conditions, string(iamv1alpha1.UserInvitationReadyCondition)) {
		t.Fatalf("Ready condition unexpectedly true before status update")
	}

	if err := uic.updateUserInvitationStatus(ctx, ui, cond); err != nil {
		t.Fatalf("updateUserInvitationStatus returned error: %v", err)
	}

	// Fetch updated resource
	updated := &iamv1alpha1.UserInvitation{}
	if err := c.Get(ctx, types.NamespacedName{Name: ui.Name, Namespace: ui.Namespace}, updated); err != nil {
		t.Fatalf("failed to get updated UserInvitation: %v", err)
	}

	readyCond := meta.FindStatusCondition(updated.Status.Conditions, string(iamv1alpha1.UserInvitationReadyCondition))
	if readyCond == nil {
		t.Fatalf("Ready condition missing after update: %+v", updated.Status.Conditions)
	}
	if readyCond.Status != metav1.ConditionTrue {
		t.Fatalf("Ready condition Status expected True, got %s", readyCond.Status)
	}
	if readyCond.Reason != string(iamv1alpha1.UserInvitationStateExpiredReason) {
		t.Fatalf("Ready condition Reason expected %s, got %s", iamv1alpha1.UserInvitationStateExpiredReason, readyCond.Reason)
	}

	// Call again with same condition to ensure idempotency (no duplicate conditions should be added)
	if err := uic.updateUserInvitationStatus(ctx, updated, cond); err != nil {
		t.Fatalf("second updateUserInvitationStatus call errored: %v", err)
	}

	again := &iamv1alpha1.UserInvitation{}
	_ = c.Get(ctx, types.NamespacedName{Name: ui.Name, Namespace: ui.Namespace}, again)
	count := 0
	for _, cnd := range again.Status.Conditions {
		if cnd.Type == string(iamv1alpha1.UserInvitationReadyCondition) {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 Ready condition, got %d", count)
	}
}

// Test_userInvitationFinalizer_Finalize verifies that PolicyBindings for uiRelatedRoles are deleted and that the operation is idempotent.
func Test_userInvitationFinalizer_Finalize(t *testing.T) {
	ctx := context.TODO()
	scheme := getTestScheme()

	// Prepare UserInvitation
	ui := &iamv1alpha1.UserInvitation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "inv",
			Namespace: "default",
			UID:       types.UID("ui-uid"),
		},
		Spec: iamv1alpha1.UserInvitationSpec{
			Email: "test@example.com",
			OrganizationRef: resourcemanagerv1alpha1.OrganizationReference{
				Name: "org",
			},
		},
	}

	// Two invitation-related roles
	roleA := iamv1alpha1.RoleReference{Name: "get-invitation-role", Namespace: "milo-system"}
	roleB := iamv1alpha1.RoleReference{Name: "accept-invitation-role", Namespace: "milo-system"}

	// Corresponding PolicyBindings that should be deleted by the finalizer
	pbA := &iamv1alpha1.PolicyBinding{ObjectMeta: metav1.ObjectMeta{Name: getDeterministicRoleName(&roleA, *ui), Namespace: roleA.Namespace}}
	pbB := &iamv1alpha1.PolicyBinding{ObjectMeta: metav1.ObjectMeta{Name: getDeterministicRoleName(&roleB, *ui), Namespace: roleB.Namespace}}

	// Build fake client with PBs present
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pbA, pbB).Build()

	f := &userInvitationFinalizer{client: c, uiRelatedRoles: []iamv1alpha1.RoleReference{roleA, roleB}}

	// First call should delete both PolicyBindings
	if _, err := f.Finalize(ctx, ui); err != nil {
		t.Fatalf("Finalize returned error: %v", err)
	}

	// Verify deletion
	for _, r := range []iamv1alpha1.RoleReference{roleA, roleB} {
		name := getDeterministicRoleName(&r, *ui)
		if err := c.Get(ctx, types.NamespacedName{Name: name, Namespace: r.Namespace}, &iamv1alpha1.PolicyBinding{}); !apierr.IsNotFound(err) {
			t.Fatalf("expected PolicyBinding %s to be deleted, err=%v", name, err)
		}
	}

	// Second call should still succeed (idempotent)
	if _, err := f.Finalize(ctx, ui); err != nil {
		t.Fatalf("Finalize second call errored: %v", err)
	}
}

// TestUserInvitationController_Reconcile_StateTransitionCreatesBindings performs two full reconciliation cycles for a pending UserInvitation
// and then when the invitation is updated to Accepted a new PolicyBinding (for organization role) is created.
func TestUserInvitationController_Reconcile_StateTransitionCreatesBindings(t *testing.T) {
	ctx := context.TODO()
	scheme := getTestScheme()

	// Objects
	user := &iamv1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{Name: "test-user", UID: types.UID("u-uid")},
		Spec:       iamv1alpha1.UserSpec{Email: "test@example.com"},
	}

	inviter := &iamv1alpha1.User{ObjectMeta: metav1.ObjectMeta{Name: "inviter", UID: types.UID("inviter-uid")}, Spec: iamv1alpha1.UserSpec{Email: "inviter@example.com"}}

	ui := &iamv1alpha1.UserInvitation{
		ObjectMeta: metav1.ObjectMeta{Name: "inv", Namespace: "default", UID: types.UID("ui-uid")},
		Spec: iamv1alpha1.UserInvitationSpec{
			Email:           user.Spec.Email,
			OrganizationRef: resourcemanagerv1alpha1.OrganizationReference{Name: "org"},
			State:           iamv1alpha1.UserInvitationStatePending,
			Roles:           []iamv1alpha1.RoleReference{{Name: "org-admin", Namespace: "milo-system"}},
			InvitedBy:       iamv1alpha1.UserReference{Name: inviter.Name},
		},
	}

	org := &resourcemanagerv1alpha1.Organization{
		ObjectMeta: metav1.ObjectMeta{Name: "org", UID: types.UID("org-uid")},
	}

	// Invitation-related role needed so that controller grants access to accept invitation.
	invitationRoleRef := iamv1alpha1.RoleReference{Name: "get-invitation-role", Namespace: "milo-system"}

	// Build fake client with status subresource enabled for UserInvitation so status updates work.
	builder := fake.NewClientBuilder().WithScheme(scheme).
		WithStatusSubresource(&iamv1alpha1.UserInvitation{}).
		WithObjects(user.DeepCopy(), ui.DeepCopy(), org.DeepCopy(), inviter.DeepCopy())

	// add indexes required by reconciler
	builder = builder.WithIndex(&iamv1alpha1.User{}, userEmailIndexKey, func(obj client.Object) []string {
		u := obj.(*iamv1alpha1.User)
		return []string{strings.ToLower(u.Spec.Email)}
	})
	builder = builder.WithIndex(&iamv1alpha1.UserInvitation{}, userEmailIndexKey, func(obj client.Object) []string {
		inv := obj.(*iamv1alpha1.UserInvitation)
		return []string{strings.ToLower(inv.Spec.Email)}
	})

	c := builder.Build()

	uic := &UserInvitationController{
		Client:          c,
		SystemNamespace: "milo-system",
		uiRelatedRoles:  []iamv1alpha1.RoleReference{invitationRoleRef},
	}

	// First reconcile (Pending)
	if _, err := uic.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: ui.Name, Namespace: ui.Namespace}}); err != nil {
		t.Fatalf("first reconcile error: %v", err)
	}

	// Verify invitation-related PolicyBinding exists
	pbInviteName := getDeterministicRoleName(&invitationRoleRef, *ui)
	if err := c.Get(ctx, types.NamespacedName{Name: pbInviteName, Namespace: invitationRoleRef.Namespace}, &iamv1alpha1.PolicyBinding{}); err != nil {
		t.Fatalf("expected invitation PolicyBinding created: %v", err)
	}

	// Check Pending condition true, Ready false
	afterFirst := &iamv1alpha1.UserInvitation{}
	_ = c.Get(ctx, types.NamespacedName{Name: ui.Name, Namespace: ui.Namespace}, afterFirst)
	if !meta.IsStatusConditionTrue(afterFirst.Status.Conditions, string(iamv1alpha1.UserInvitationPendingCondition)) {
		t.Fatalf("Pending condition should be true after first reconcile")
	}
	if meta.IsStatusConditionTrue(afterFirst.Status.Conditions, string(iamv1alpha1.UserInvitationReadyCondition)) {
		t.Fatalf("Ready condition should not be true before acceptance")
	}

	// Ensure organization role PolicyBinding does NOT exist yet
	orgRoleRef := ui.Spec.Roles[0]
	pbOrgName := getDeterministicRoleName(&orgRoleRef, *ui)
	if err := c.Get(ctx, types.NamespacedName{Name: pbOrgName, Namespace: orgRoleRef.Namespace}, &iamv1alpha1.PolicyBinding{}); err == nil {
		t.Fatalf("organization PolicyBinding should not exist before acceptance")
	}

	// Update state to Accepted
	refreshed := &iamv1alpha1.UserInvitation{}
	_ = c.Get(ctx, types.NamespacedName{Name: ui.Name, Namespace: ui.Namespace}, refreshed)
	refreshed.Spec.State = iamv1alpha1.UserInvitationStateAccepted
	if err := c.Update(ctx, refreshed); err != nil {
		t.Fatalf("failed to update UI state: %v", err)
	}

	// Second reconcile after state change
	if _, err := uic.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: ui.Name, Namespace: ui.Namespace}}); err != nil {
		t.Fatalf("second reconcile error: %v", err)
	}

	// Verify organization role PolicyBinding created
	if err := c.Get(ctx, types.NamespacedName{Name: pbOrgName, Namespace: orgRoleRef.Namespace}, &iamv1alpha1.PolicyBinding{}); err != nil {
		t.Fatalf("expected organization PolicyBinding created: %v", err)
	}

	// Ready condition should now be true, Pending may remain true
	final := &iamv1alpha1.UserInvitation{}
	_ = c.Get(ctx, types.NamespacedName{Name: ui.Name, Namespace: ui.Namespace}, final)
	if !meta.IsStatusConditionTrue(final.Status.Conditions, string(iamv1alpha1.UserInvitationReadyCondition)) {
		t.Fatalf("Ready condition should be true after acceptance")
	}
}

// Test when UserInvitation exists before User resource; controller should act once user appears and then on acceptance.
func TestUserInvitationController_Reconcile_UserCreatedLater(t *testing.T) {
	ctx := context.TODO()
	scheme := getTestScheme()

	// Initial objects: UserInvitation only
	inviter := &iamv1alpha1.User{ObjectMeta: metav1.ObjectMeta{Name: "inviter", UID: types.UID("inviter-uid")}, Spec: iamv1alpha1.UserSpec{Email: "inviter@example.com"}}
	ui := &iamv1alpha1.UserInvitation{
		ObjectMeta: metav1.ObjectMeta{Name: "inv", Namespace: "default", UID: types.UID("ui-uid")},
		Spec: iamv1alpha1.UserInvitationSpec{
			Email:           "later@example.com",
			OrganizationRef: resourcemanagerv1alpha1.OrganizationReference{Name: "org"},
			State:           iamv1alpha1.UserInvitationStatePending,
			Roles:           []iamv1alpha1.RoleReference{{Name: "org-admin", Namespace: "milo-system"}},
			InvitedBy:       iamv1alpha1.UserReference{Name: inviter.Name},
		},
	}

	org := &resourcemanagerv1alpha1.Organization{ObjectMeta: metav1.ObjectMeta{Name: "org", UID: types.UID("org-uid")}}

	invitationRoleRef := iamv1alpha1.RoleReference{Name: "get-invitation-role", Namespace: "milo-system"}

	builder := fake.NewClientBuilder().WithScheme(scheme).
		WithStatusSubresource(&iamv1alpha1.UserInvitation{}).
		WithObjects(ui.DeepCopy(), org.DeepCopy(), inviter.DeepCopy())

	// indexes
	builder = builder.WithIndex(&iamv1alpha1.User{}, userEmailIndexKey, func(obj client.Object) []string {
		u := obj.(*iamv1alpha1.User)
		return []string{strings.ToLower(u.Spec.Email)}
	})
	builder = builder.WithIndex(&iamv1alpha1.UserInvitation{}, userEmailIndexKey, func(obj client.Object) []string {
		inv := obj.(*iamv1alpha1.UserInvitation)
		return []string{strings.ToLower(inv.Spec.Email)}
	})
	c := builder.Build()

	uic := &UserInvitationController{Client: c, SystemNamespace: "milo-system", uiRelatedRoles: []iamv1alpha1.RoleReference{invitationRoleRef}}

	// First reconcile: no User yet
	if _, err := uic.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: ui.Name, Namespace: ui.Namespace}}); err != nil {
		t.Fatalf("first reconcile error: %v", err)
	}

	// Expect no PolicyBindings created
	pbInviteName := getDeterministicRoleName(&invitationRoleRef, *ui)
	if err := c.Get(ctx, types.NamespacedName{Name: pbInviteName, Namespace: invitationRoleRef.Namespace}, &iamv1alpha1.PolicyBinding{}); err == nil {
		t.Fatalf("PolicyBinding should not exist when User absent")
	}

	// Create User now
	user := &iamv1alpha1.User{ObjectMeta: metav1.ObjectMeta{Name: "later-user", UID: types.UID("u-uid")}, Spec: iamv1alpha1.UserSpec{Email: "later@example.com"}}
	if err := c.Create(ctx, user); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// Second reconcile: should create invitation PB and Pending condition
	if _, err := uic.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: ui.Name, Namespace: ui.Namespace}}); err != nil {
		t.Fatalf("second reconcile error: %v", err)
	}

	if err := c.Get(ctx, types.NamespacedName{Name: pbInviteName, Namespace: invitationRoleRef.Namespace}, &iamv1alpha1.PolicyBinding{}); err != nil {
		t.Fatalf("expected invitation PolicyBinding created after user appears: %v", err)
	}

	// Update state to Accepted
	refreshed := &iamv1alpha1.UserInvitation{}
	_ = c.Get(ctx, types.NamespacedName{Name: ui.Name, Namespace: ui.Namespace}, refreshed)
	refreshed.Spec.State = iamv1alpha1.UserInvitationStateAccepted
	if err := c.Update(ctx, refreshed); err != nil {
		t.Fatalf("update UI state: %v", err)
	}

	// Third reconcile
	if _, err := uic.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: ui.Name, Namespace: ui.Namespace}}); err != nil {
		t.Fatalf("third reconcile error: %v", err)
	}

	orgRoleRef := ui.Spec.Roles[0]
	pbOrgName := getDeterministicRoleName(&orgRoleRef, *ui)
	if err := c.Get(ctx, types.NamespacedName{Name: pbOrgName, Namespace: orgRoleRef.Namespace}, &iamv1alpha1.PolicyBinding{}); err != nil {
		t.Fatalf("expected org role PolicyBinding after acceptance: %v", err)
	}

	final := &iamv1alpha1.UserInvitation{}
	_ = c.Get(ctx, types.NamespacedName{Name: ui.Name, Namespace: ui.Namespace}, final)
	if !meta.IsStatusConditionTrue(final.Status.Conditions, string(iamv1alpha1.UserInvitationReadyCondition)) {
		t.Fatalf("Ready condition should be true after acceptance")
	}
}

func TestUserInvitationController_createInvitationEmail(t *testing.T) {
	ctx := context.TODO()
	scheme := getTestScheme()

	// Objects
	invitee := &iamv1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{Name: "invitee", UID: types.UID("u-invitee")},
		Spec:       iamv1alpha1.UserSpec{Email: "invitee@example.com", GivenName: "Invite", FamilyName: "E"},
	}

	inviter := &iamv1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{Name: "inviter", UID: types.UID("u-inviter")},
		Spec:       iamv1alpha1.UserSpec{Email: "inviter@example.com", GivenName: "John", FamilyName: "Doe"},
	}

	org := &resourcemanagerv1alpha1.Organization{
		ObjectMeta: metav1.ObjectMeta{Name: "test-org", UID: types.UID("org-uid"), Annotations: map[string]string{"kubernetes.io/display-name": "Test Org"}},
	}

	ui := &iamv1alpha1.UserInvitation{
		ObjectMeta: metav1.ObjectMeta{Name: "inv", Namespace: "default", UID: types.UID("ui-uid")},
		Spec: iamv1alpha1.UserInvitationSpec{
			GivenName:       invitee.Spec.GivenName,
			FamilyName:      invitee.Spec.FamilyName,
			Email:           invitee.Spec.Email,
			InvitedBy:       iamv1alpha1.UserReference{Name: inviter.Name},
			OrganizationRef: resourcemanagerv1alpha1.OrganizationReference{Name: org.Name},
			Roles:           []iamv1alpha1.RoleReference{{Name: "org-admin", Namespace: "milo-system"}},
		},
	}

	template := &notificationv1alpha1.EmailTemplate{ObjectMeta: metav1.ObjectMeta{Name: "template"}}

	// Build fake client with status subresource for Email so that create works.
	c := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(invitee.DeepCopy(), inviter.DeepCopy(), org.DeepCopy(), template.DeepCopy()).Build()

	uic := &UserInvitationController{
		Client:                          c,
		UserInvitationEmailTemplateName: template.Name,
	}

	// Act
	if err := uic.createInvitationEmail(ctx, ui); err != nil {
		t.Fatalf("createInvitationEmail error: %v", err)
	}

	// Assert Email exists
	emailName := getDeterministicEmailName(*ui)
	email := &notificationv1alpha1.Email{}
	if err := c.Get(ctx, types.NamespacedName{Name: emailName, Namespace: ui.Namespace}, email); err != nil {
		t.Fatalf("expected Email created: %v", err)
	}

	if email.Spec.TemplateRef.Name != template.Name {
		t.Errorf("unexpected TemplateRef.Name, got %s", email.Spec.TemplateRef.Name)
	}
	if email.Spec.Recipient.EmailAddress != invitee.Spec.Email {
		t.Errorf("unexpected Recipient.EmailAddress, got %s", email.Spec.Recipient.EmailAddress)
	}

	// Check variables map for a few key vars
	vars := map[string]string{}
	for _, v := range email.Spec.Variables {
		vars[v.Name] = v.Value
	}
	if vars["UserName"] != "Invite E" {
		t.Errorf("UserName variable mismatch, got %s", vars["UserName"])
	}
	if vars["CompanyName"] != "Test Org" {
		t.Errorf("CompanyNam variable mismatch, got %s", vars["CompanyNam"])
	}
	if vars["InviteLink"] != "https://cloud.staging.env.datum.net/invitation/inv/accept" {
		t.Errorf("InviteLink variable mismatch, got %s", vars["InviteLink"])
	}

	// Idempotency: second call should not error and should not create duplicate Email (still one)
	if err := uic.createInvitationEmail(ctx, ui); err != nil {
		t.Fatalf("idempotent createInvitationEmail error: %v", err)
	}

	var emailList notificationv1alpha1.EmailList
	if err := c.List(ctx, &emailList); err != nil {
		t.Fatalf("list emails: %v", err)
	}
	if len(emailList.Items) != 1 {
		t.Errorf("expected 1 Email after idempotent call, got %d", len(emailList.Items))
	}
}
