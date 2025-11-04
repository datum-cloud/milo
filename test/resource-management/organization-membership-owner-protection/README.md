# Test: `organization-membership-owner-protection`

Validates safeguarding organization owners when deleting memberships.

Ensures deletion is allowed when another owner remains and blocked when
attempting to delete the final owner. Confirms the webhook returns clear
guidance for remediation.


## Steps

| # | Name | Bindings | Try | Catch | Finally | Cleanup |
|:-:|---|:-:|:-:|:-:|:-:|:-:|
| 1 | [setup-environment](#step-setup-environment) | 0 | 11 | 0 | 0 | 0 |
| 2 | [create-primary-owner](#step-create-primary-owner) | 0 | 4 | 0 | 0 | 0 |
| 3 | [create-secondary-owner](#step-create-secondary-owner) | 0 | 4 | 0 | 0 | 0 |
| 4 | [delete-secondary-owner](#step-delete-secondary-owner) | 0 | 2 | 0 | 0 | 0 |
| 5 | [prevent-last-owner-deletion](#step-prevent-last-owner-deletion) | 0 | 3 | 0 | 0 | 0 |
| 6 | [delete-organization](#step-delete-organization) | 0 | 4 | 0 | 0 | 0 |

### Step: `setup-environment`

Creates organization namespace and two users. Waits for namespace to
become active and for both users to reach Ready so memberships can be
created safely.


#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `delete` | 0 | 0 | Remove any leftover owner organization from earlier runs |
| 2 | `delete` | 0 | 0 | Ensure previous organization namespace is cleaned up |
| 3 | `wait` | 0 | 0 | Wait for organization namespace to be fully removed |
| 4 | `sleep` | 0 | 0 | Allow webhook server time to become ready |
| 5 | `apply` | 0 | 0 | Create test organization |
| 6 | `delete` | 0 | 0 | Remove auto-created admin membership |
| 7 | `wait` | 0 | 0 | Wait for organization namespace to become Active |
| 8 | `apply` | 0 | 0 | Create owner test users |
| 9 | `apply` | 0 | 0 | Ensure owner role exists in milo-system |
| 10 | `wait` | 0 | 0 | Wait for Alice to be Ready |
| 11 | `wait` | 0 | 0 | Wait for Bob to be Ready |

### Step: `create-primary-owner`

Creates the first owner membership and waits for the controller to apply
the owner role successfully.


#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | Create owner membership for Alice |
| 2 | `wait` | 0 | 0 | Wait for membership to reach Ready |
| 3 | `script` | 0 | 0 | Mark PolicyBindings as Ready (test workaround) |
| 4 | `wait` | 0 | 0 | Wait for owner role to be applied |

### Step: `create-secondary-owner`

Adds a second owner to validate that deletions succeed when another
owner remains in the organization.


#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | Create owner membership for Bob |
| 2 | `wait` | 0 | 0 | Wait for membership to reach Ready |
| 3 | `script` | 0 | 0 | Mark PolicyBindings as Ready (test workaround) |
| 4 | `wait` | 0 | 0 | Wait for owner role to be applied |

### Step: `delete-secondary-owner`

Deletes the first owner membership. With another owner still present the
operation should succeed.


#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `delete` | 0 | 0 | Delete Alice's membership (should succeed) |
| 2 | `wait` | 0 | 0 | Ensure Alice's membership was removed |

### Step: `prevent-last-owner-deletion`

Attempts to delete the remaining owner. The webhook should block the
deletion and return actionable guidance.


#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `patch` | 0 | 0 | Deny removal of owner role from final owner |
| 2 | `delete` | 0 | 0 | Deny deletion of final owner and verify error messaging |
| 3 | `wait` | 0 | 0 | Confirm Bob's membership still exists |

### Step: `delete-organization`

Deletes the organization and confirms both the namespace and remaining
membership are cleaned up automatically.


#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `delete` | 0 | 0 | Delete organization to trigger namespace cleanup |
| 2 | `wait` | 0 | 0 | Ensure organization namespace no longer exists |
| 3 | `delete` | 0 | 0 | Delete owner test users |
| 4 | `delete` | 0 | 0 | Delete owner role assigned in setup |

---

