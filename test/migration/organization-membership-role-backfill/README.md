# Test: `organization-membership-role-backfill`

Tests the organization membership migration controller that backfills roles
from existing PolicyBindings.

This test verifies:
- OrganizationMemberships without roles are discovered
- Legacy PolicyBindings are correctly identified
- Roles are extracted and added to OrganizationMembership.spec.roles
- OrganizationMembership controller creates new managed PolicyBindings
- Legacy PolicyBindings are cleaned up after migration
- Users retain access throughout the migration process


## Steps

| # | Name | Bindings | Try | Catch | Finally | Cleanup |
|:-:|---|:-:|:-:|:-:|:-:|:-:|
| 1 | [setup-test-resources](#step-setup-test-resources) | 0 | 7 | 0 | 0 | 0 |
| 2 | [create-legacy-setup](#step-create-legacy-setup) | 0 | 7 | 0 | 0 | 0 |
| 3 | [verify-pre-migration-state](#step-verify-pre-migration-state) | 0 | 3 | 0 | 0 | 0 |
| 4 | [run-migration-controller](#step-run-migration-controller) | 0 | 4 | 0 | 0 | 0 |
| 5 | [cleanup-legacy-bindings](#step-cleanup-legacy-bindings) | 0 | 6 | 0 | 0 | 0 |
| 6 | [verify-post-migration-state](#step-verify-post-migration-state) | 0 | 4 | 0 | 0 | 0 |

### Step: `setup-test-resources`

Create Organization, User, Roles, and OrganizationMembership without roles

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 1 | *No description* |
| 2 | `wait` | 0 | 0 | *No description* |
| 3 | `apply` | 0 | 1 | *No description* |
| 4 | `wait` | 0 | 0 | *No description* |
| 5 | `apply` | 0 | 0 | *No description* |
| 6 | `wait` | 0 | 0 | *No description* |
| 7 | `wait` | 0 | 0 | *No description* |

### Step: `create-legacy-setup`

Create OrganizationMembership without roles and legacy PolicyBindings

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 1 | *No description* |
| 2 | `wait` | 0 | 0 | *No description* |
| 3 | `assert` | 0 | 0 | *No description* |
| 4 | `apply` | 0 | 0 | *No description* |
| 5 | `wait` | 0 | 0 | *No description* |
| 6 | `wait` | 0 | 0 | *No description* |
| 7 | `assert` | 0 | 0 | *No description* |

### Step: `verify-pre-migration-state`

Verify the pre-migration state is correct

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `assert` | 0 | 0 | *No description* |
| 2 | `assert` | 0 | 0 | *No description* |
| 3 | `script` | 0 | 0 | *No description* |

### Step: `run-migration-controller`

Simulate migration controller discovering and updating membership

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `patch` | 0 | 0 | *No description* |
| 2 | `wait` | 0 | 0 | *No description* |
| 3 | `assert` | 0 | 0 | *No description* |
| 4 | `script` | 0 | 0 | *No description* |

### Step: `cleanup-legacy-bindings`

Simulate cleanup controller removing legacy PolicyBindings

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `assert` | 0 | 0 | *No description* |
| 2 | `delete` | 0 | 0 | *No description* |
| 3 | `delete` | 0 | 0 | *No description* |
| 4 | `wait` | 0 | 0 | *No description* |
| 5 | `wait` | 0 | 0 | *No description* |
| 6 | `script` | 0 | 0 | *No description* |

### Step: `verify-post-migration-state`

Verify the final state after migration

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `assert` | 0 | 0 | *No description* |
| 2 | `assert` | 0 | 0 | *No description* |
| 3 | `script` | 0 | 0 | *No description* |
| 4 | `script` | 0 | 0 | *No description* |

---

