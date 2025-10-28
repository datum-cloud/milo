# Test: `organization-membership-role-assignment`

Tests OrganizationMembership role-based access control functionality.

This test validates the enhanced OrganizationMembership feature that
automatically manages PolicyBinding resources for assigned roles.

Key scenarios tested:
- OrganizationMembership creation with role assignments
- Automatic PolicyBinding creation by the controller
- Multiple roles per membership
- Cross-namespace role references
- Role addition via membership update
- Role removal via membership update
- PolicyBinding cleanup when roles are removed
- Validation webhook enforcement (duplicate roles, invalid roles)
- Status tracking with appliedRoles and conditions
- Garbage collection of PolicyBindings via owner references


## Steps

| # | Name | Bindings | Try | Catch | Finally | Cleanup |
|:-:|---|:-:|:-:|:-:|:-:|:-:|
| 1 | [setup-test-environment](#step-setup-test-environment) | 0 | 6 | 0 | 0 | 0 |
| 2 | [test-membership-with-single-role](#step-test-membership-with-single-role) | 0 | 5 | 0 | 0 | 0 |
| 3 | [test-membership-with-multiple-roles](#step-test-membership-with-multiple-roles) | 0 | 5 | 0 | 0 | 0 |
| 4 | [test-add-roles-via-update](#step-test-add-roles-via-update) | 0 | 4 | 0 | 0 | 0 |
| 5 | [test-remove-roles-via-update](#step-test-remove-roles-via-update) | 0 | 4 | 0 | 0 | 0 |
| 6 | [test-membership-without-roles](#step-test-membership-without-roles) | 0 | 4 | 0 | 0 | 0 |
| 7 | [test-cross-namespace-role](#step-test-cross-namespace-role) | 0 | 4 | 0 | 0 | 0 |
| 8 | [test-validation-duplicate-roles](#step-test-validation-duplicate-roles) | 0 | 1 | 0 | 0 | 0 |
| 9 | [test-validation-invalid-role](#step-test-validation-invalid-role) | 0 | 1 | 0 | 0 | 0 |
| 10 | [test-partial-role-application](#step-test-partial-role-application) | 0 | 4 | 0 | 0 | 0 |
| 11 | [test-garbage-collection](#step-test-garbage-collection) | 0 | 3 | 0 | 0 | 0 |

### Step: `setup-test-environment`

Creates test organization, user, and four test roles (organization-admin,
organization-viewer, billing-manager in org namespace; shared-developer in
milo-system). Waits for organization namespace to become Active and user to
reach Ready state before proceeding.


#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | Create test Organization |
| 2 | `wait` | 0 | 0 | Wait for Organization namespace to become Active |
| 3 | `apply` | 0 | 0 | Create test User |
| 4 | `wait` | 0 | 0 | Wait for User to reach Ready state |
| 5 | `apply` | 0 | 0 | Create four test Roles (organization-admin, organization-viewer, billing-manager, shared-developer) |
| 6 | `sleep` | 0 | 0 | Allow time for Roles to be processed |

### Step: `test-membership-with-single-role`

Assigns organization-viewer role to user via OrganizationMembership. Verifies
that the controller automatically creates one PolicyBinding with correct labels
and owner references. Confirms appliedRoles status shows role as "Applied".


#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | Create OrganizationMembership with organization-viewer role |
| 2 | `wait` | 0 | 0 | Wait for membership to reach Ready state |
| 3 | `wait` | 0 | 0 | Wait for controller to apply role and update RolesApplied condition |
| 4 | `assert` | 0 | 0 | Verify membership status shows applied role |
| 5 | `assert` | 0 | 0 | Verify controller created PolicyBinding with correct labels and owner reference |

### Step: `test-membership-with-multiple-roles`

Assigns two roles (organization-admin and billing-manager) to user in single
membership. Verifies controller creates two PolicyBindings, each referencing
the correct role. Confirms both roles appear in appliedRoles status.


#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | Create OrganizationMembership with organization-admin and billing-manager roles |
| 2 | `wait` | 0 | 0 | Wait for membership to reach Ready state |
| 3 | `wait` | 0 | 0 | Wait for controller to apply both roles |
| 4 | `assert` | 0 | 0 | Verify membership status shows both applied roles |
| 5 | `assert` | 0 | 0 | Verify controller created two PolicyBindings, one for each role |

### Step: `test-add-roles-via-update`

Updates existing single-role membership to add billing-manager role. Verifies
controller creates additional PolicyBinding for new role while preserving
existing PolicyBinding. Confirms appliedRoles status reflects both roles.


#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | Update membership to add billing-manager role to existing organization-viewer role |
| 2 | `wait` | 0 | 0 | Wait for controller to process update and apply new role |
| 3 | `sleep` | 0 | 0 | Allow time for PolicyBinding creation to complete |
| 4 | `assert` | 0 | 0 | Verify both roles in appliedRoles and two PolicyBindings exist |

### Step: `test-remove-roles-via-update`

Updates multi-role membership to remove billing-manager role. Verifies controller
deletes corresponding PolicyBinding while preserving PolicyBinding for remaining
organization-admin role. Confirms appliedRoles status shows only active role.


#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | Update membership to remove billing-manager role, keeping only organization-admin |
| 2 | `wait` | 0 | 0 | Wait for controller to process removal and update RolesApplied condition |
| 3 | `sleep` | 0 | 0 | Allow time for PolicyBinding deletion to complete |
| 4 | `assert` | 0 | 0 | Verify only organization-admin role remains in appliedRoles and corresponding PolicyBinding exists |

### Step: `test-membership-without-roles`

Creates membership with empty roles list. Verifies no PolicyBindings are created
and RolesApplied condition shows True with reason "NoRolesSpecified". Confirms
membership remains valid without role assignments.


#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | Create OrganizationMembership with empty roles list |
| 2 | `wait` | 0 | 0 | Wait for membership to reach Ready state |
| 3 | `wait` | 0 | 0 | Wait for RolesApplied condition (should be True with NoRolesSpecified reason) |
| 4 | `assert` | 0 | 0 | Verify no PolicyBindings created and RolesApplied condition reason is NoRolesSpecified |

### Step: `test-cross-namespace-role`

Assigns role from milo-system namespace (shared-developer) to membership in
organization namespace. Verifies PolicyBinding is created in membership namespace
with correct cross-namespace role reference. Confirms appliedRoles shows full
namespace/name format.


#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | Create membership referencing shared-developer role from milo-system namespace |
| 2 | `wait` | 0 | 0 | Wait for membership to reach Ready state |
| 3 | `wait` | 0 | 0 | Wait for controller to apply cross-namespace role |
| 4 | `assert` | 0 | 0 | Verify PolicyBinding created in membership namespace with correct cross-namespace role reference |

### Step: `test-validation-duplicate-roles`

Attempts to create membership with duplicate role reference (organization-viewer
listed twice). Verifies Kubernetes API server rejects request with "Duplicate
value" error before webhook runs. Tests +listType=map kubebuilder marker
enforcement.


#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `create` | 0 | 0 | Attempt to create membership with organization-viewer role listed twice |

### Step: `test-validation-invalid-role`

Attempts to create membership referencing non-existent role (nonexistent-role).
Verifies admission webhook rejects request with "role not found" error. Tests
webhook validation layer that checks role existence.


#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `create` | 0 | 0 | Attempt to create membership with non-existent role reference |

### Step: `test-partial-role-application`

Creates membership with single valid role (organization-viewer). Verifies
controller successfully applies the role and creates PolicyBinding. Confirms
appliedRoles status accurately reflects successful role application.


#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `create` | 0 | 0 | Create membership with organization-viewer role |
| 2 | `wait` | 0 | 0 | Wait for membership to reach Ready state |
| 3 | `sleep` | 0 | 0 | Allow time for role application to complete |
| 4 | `assert` | 0 | 0 | Verify role successfully applied and status accurate |

### Step: `test-garbage-collection`

Deletes membership with active PolicyBindings. Verifies Kubernetes garbage
collection automatically deletes associated PolicyBindings via owner references.
Confirms no orphaned PolicyBindings remain after membership deletion.


#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `delete` | 0 | 0 | Delete membership with active PolicyBinding |
| 2 | `sleep` | 0 | 0 | Allow time for garbage collection to process owner references |
| 3 | `script` | 0 | 0 | Verify PolicyBinding was automatically deleted (should return NotFound) |

---

