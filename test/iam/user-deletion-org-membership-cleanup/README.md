# Test: `user-deletion-org-membership-cleanup`

Regression test for GitHub issue #536: Orphaned PolicyBindings for deleted
Users are not garbage collected.

When a User is deleted, any OrganizationMembership resources that reference
the user as a subject (in organization namespaces) are NOT cleaned up. The
root cause is that OrganizationMembership resources created by the
organization webhook have no User ownerReference, so Kubernetes garbage
collection has no knowledge of the dependency.

The membership controller owns the PolicyBindings via ownerReference, so
PolicyBindings are owned by the OrganizationMembership. When the User is
deleted without the membership being cleaned up first, the membership (and
its PolicyBindings) become permanently orphaned. The PolicyBindings
subsequently enter a SubjectValidationFailed state because the referenced
User no longer exists.

THIS TEST IS WRITTEN TO DEMONSTRATE THE BUG.
- With the CURRENT buggy code: the final assertions PASS (membership and
  PolicyBindings remain after the User is deleted, confirming the bug).
- After the FIX is applied (adding a User ownerReference to the
  OrganizationMembership, or adding a finalizer on the User that deletes
  memberships): the final assertions must be INVERTED to use `error` instead
  of `assert`, verifying that both the membership and its PolicyBindings
  were deleted.

See also: test/iam/userinvitation-policybinding-gc/chainsaw-test.yaml for
the established pattern this test follows.


## Steps

| # | Name | Bindings | Try | Catch | Finally | Cleanup |
|:-:|---|:-:|:-:|:-:|:-:|:-:|
| 1 | [create-user](#step-create-user) | 0 | 2 | 0 | 0 | 0 |
| 2 | [create-organization](#step-create-organization) | 0 | 2 | 0 | 0 | 0 |
| 3 | [create-role-and-membership](#step-create-role-and-membership) | 0 | 4 | 0 | 0 | 0 |
| 4 | [delete-user-and-assert-membership-orphaned](#step-delete-user-and-assert-membership-orphaned) | 0 | 4 | 0 | 0 | 0 |

### Step: `create-user`

Create the User that will later be deleted. Wait for it to reach Ready
state so the membership controller can successfully reconcile against it.


#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | *No description* |
| 2 | `wait` | 0 | 0 | *No description* |

### Step: `create-organization`

Create the Organization using the admin token (system:masters group). The
organization webhook skips OrganizationMembership creation for
system:masters requests, so no membership is auto-created here. The
namespace organization-om-gc-test-org is provisioned by the webhook.


#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | *No description* |
| 2 | `wait` | 0 | 0 | *No description* |

### Step: `create-role-and-membership`

Create a test role in the organization namespace, then create an
OrganizationMembership that mirrors what the organization webhook creates
today: the membership references the User in spec.userRef but has NO
ownerReference pointing back to the User resource. This is the core of
bug #536 — without the ownerReference the Kubernetes garbage collector
will not cascade-delete the membership when the User is removed.


#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | *No description* |
| 2 | `apply` | 0 | 0 | *No description* |
| 3 | `wait` | 0 | 0 | Wait for the membership to reach Ready state, confirming the controller found the User and Organization and created PolicyBindings. |
| 4 | `script` | 0 | 0 | Verify that the membership controller created at least one PolicyBinding
in the organization namespace, owned by the membership. These PolicyBindings
are what gets orphaned when the User is deleted.
 |

### Step: `delete-user-and-assert-membership-orphaned`

Delete the User and then assert that the OrganizationMembership and its
associated PolicyBindings STILL EXIST in the organization namespace.

THIS STEP DEMONSTRATES THE BUG (issue #536).

Because the OrganizationMembership has no ownerReference pointing to the
User, Kubernetes garbage collection does not know to delete the membership
when the User is removed. The membership — and all PolicyBindings it owns
— are left as orphans. The PolicyBindings will subsequently enter a
SubjectValidationFailed state because the User subject they reference no
longer exists.

After the fix is applied:
  Replace the final two `assert` operations in this step with `error`
  operations. The `delete` and wait-for-deletion steps remain unchanged.

The fix may be implemented as any of the following:
  1. Add a User ownerReference to OrganizationMembership when the
     organization webhook creates it.
  2. Add a finalizer on the User resource that deletes all
     OrganizationMemberships referencing the User before allowing deletion.
  3. A combination of the above approaches.


#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `delete` | 0 | 0 | Delete the User resource. |
| 2 | `error` | 0 | 0 | Wait for the User to be fully removed from the API server. |
| 3 | `assert` | 0 | 0 | BUG DEMONSTRATION: Assert the OrganizationMembership STILL EXISTS after
the User was deleted. If this assertion passes the membership is
confirmed to be orphaned.

After the fix is applied, replace this `assert` with an `error` block
to verify the membership was cleaned up.
 |
| 4 | `script` | 0 | 0 | BUG DEMONSTRATION: Assert that at least one PolicyBinding owned by the
OrganizationMembership STILL EXISTS after the User was deleted. If this
passes, the PolicyBindings are confirmed to be orphaned.

After the fix is applied, replace this script with one that asserts
a count of 0 orphaned PolicyBindings.
 |

---

