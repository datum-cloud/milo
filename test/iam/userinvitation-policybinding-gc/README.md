# Test: `userinvitation-policybinding-gc`

Regression test for GitHub issue #535: Orphaned PolicyBindings for deleted
UserInvitations are not garbage collected.

The UserInvitationController creates PolicyBindings in milo-system granting
the invitee user `getinvitation` and `acceptinvitation` permissions. When a
UserInvitation is deleted (accepted, declined, expired, or manually removed),
the associated PolicyBindings must be cleaned up by the finalizer.

The fix wires r.finalizer.Finalize(ctx, ui) into Reconcile so the finalizer
string is added on first reconcile and the cleanup handler (userInvitationFinalizer.Finalize)
runs on deletion, deleting both PolicyBindings before the object is removed.

See also: test/iam/user-deletion-garbage-collection/chainsaw-test.yaml for
the established pattern this test follows.


## Steps

| # | Name | Bindings | Try | Catch | Finally | Cleanup |
|:-:|---|:-:|:-:|:-:|:-:|:-:|
| 1 | [setup-organization](#step-setup-organization) | 0 | 2 | 0 | 0 | 0 |
| 2 | [setup-users](#step-setup-users) | 0 | 4 | 0 | 0 | 0 |
| 3 | [create-invitation-and-verify-policybindings](#step-create-invitation-and-verify-policybindings) | 0 | 3 | 0 | 0 | 0 |
| 4 | [delete-invitation-and-verify-policybindings-cleaned-up](#step-delete-invitation-and-verify-policybindings-cleaned-up) | 0 | 3 | 0 | 0 | 0 |

### Step: `setup-organization`

Create the Organization and wait for its namespace to be provisioned.
UserInvitations are namespaced resources that live in the
organization-{name} namespace.


#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | *No description* |
| 2 | `wait` | 0 | 0 | *No description* |

### Step: `setup-users`

Create the inviter and invitee User resources and wait for them to become
Ready. The controller looks up the invitee User by email when reconciling
the UserInvitation and will only create PolicyBindings once the User exists.


#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | *No description* |
| 2 | `apply` | 0 | 0 | *No description* |
| 3 | `wait` | 0 | 0 | *No description* |
| 4 | `wait` | 0 | 0 | *No description* |

### Step: `create-invitation-and-verify-policybindings`

Create the UserInvitation and wait for the controller to reconcile it.
The controller creates two PolicyBindings in milo-system:
  - one for the iam.miloapis.com-getinvitation role
  - one for the iam.miloapis.com-acceptinvitation role
Both bindings use deterministic names derived from the UserInvitation UID
and role name: {uid}-{rolename}.
Wait for the Pending condition to be set, which indicates the controller
has finished its first reconcile pass and created the PolicyBindings.


#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | *No description* |
| 2 | `wait` | 0 | 0 | *No description* |
| 3 | `script` | 0 | 0 | Verify both invitation PolicyBindings were created in milo-system |

### Step: `delete-invitation-and-verify-policybindings-cleaned-up`

Delete the UserInvitation and verify the associated PolicyBindings are
cleaned up by the finalizer before the object is removed.

The fix for issue #535 wires r.finalizer.Finalize into Reconcile so that:
  - The finalizer string is added to the UserInvitation on first reconcile,
    preventing Kubernetes from deleting the object immediately.
  - On deletion, the finalizer runs userInvitationFinalizer.Finalize which
    calls deletePolicyBinding for each uiRelatedRoles entry.
  - Both PolicyBindings are deleted before the UserInvitation is removed
    from the API server.


#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `delete` | 0 | 0 | *No description* |
| 2 | `error` | 0 | 0 | Wait for the UserInvitation to be fully removed from the API server |
| 3 | `script` | 0 | 0 | Assert both invitation PolicyBindings were deleted by the finalizer.
A count of 0 confirms the fix for issue #535 is working correctly.
 |

---

