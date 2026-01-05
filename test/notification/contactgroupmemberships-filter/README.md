# Test: `contactgroupmemberships-filter`

End-to-end tests for ContactGroupMembership and ContactGroupMembershipRemoval filtering.

This test verifies:
- User can only see their own memberships when listing
- User can only see their own membership removals when listing
- User cannot see memberships/removals belonging to other users
- Manual field selectors are overridden to enforce user scope


## Steps

| # | Name | Bindings | Try | Catch | Finally | Cleanup |
|:-:|---|:-:|:-:|:-:|:-:|:-:|
| 1 | [setup-contact-group](#step-setup-contact-group) | 0 | 2 | 0 | 0 | 0 |
| 2 | [setup-admin-contact](#step-setup-admin-contact) | 0 | 2 | 0 | 0 | 0 |
| 3 | [setup-test-user-contact](#step-setup-test-user-contact) | 0 | 2 | 0 | 0 | 0 |
| 4 | [create-admin-membership](#step-create-admin-membership) | 0 | 3 | 0 | 0 | 0 |
| 5 | [create-test-user-membership](#step-create-test-user-membership) | 0 | 3 | 0 | 0 | 0 |
| 6 | [verify-membership-filter](#step-verify-membership-filter) | 0 | 1 | 0 | 0 | 0 |
| 7 | [verify-membership-field-selector-override](#step-verify-membership-field-selector-override) | 0 | 1 | 0 | 0 | 0 |
| 8 | [create-admin-removal](#step-create-admin-removal) | 0 | 3 | 0 | 0 | 0 |
| 9 | [create-test-user-removal](#step-create-test-user-removal) | 0 | 3 | 0 | 0 | 0 |
| 10 | [verify-removal-filter](#step-verify-removal-filter) | 0 | 1 | 0 | 0 | 0 |

### Step: `setup-contact-group`

Create the contact group for testing

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | *No description* |
| 2 | `assert` | 0 | 0 | *No description* |

### Step: `setup-admin-contact`

Create the contact for admin user

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | *No description* |
| 2 | `assert` | 0 | 0 | *No description* |

### Step: `setup-test-user-contact`

Create the contact for test-user

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | *No description* |
| 2 | `assert` | 0 | 0 | *No description* |

### Step: `create-admin-membership`

Create membership for admin user and set status.username

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | *No description* |
| 2 | `assert` | 0 | 0 | *No description* |
| 3 | `script` | 0 | 0 | *No description* |

### Step: `create-test-user-membership`

Create membership for test-user and set status.username

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | *No description* |
| 2 | `assert` | 0 | 0 | *No description* |
| 3 | `script` | 0 | 0 | *No description* |

### Step: `verify-membership-filter`

Verify that admin user can only see their own membership

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `script` | 0 | 0 | *No description* |

### Step: `verify-membership-field-selector-override`

Verify that attempting to spy on other users memberships is blocked

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `script` | 0 | 0 | *No description* |

### Step: `create-admin-removal`

Create removal request for admin user and set status.username

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | *No description* |
| 2 | `assert` | 0 | 0 | *No description* |
| 3 | `script` | 0 | 0 | *No description* |

### Step: `create-test-user-removal`

Create removal request for test-user and set status.username

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | *No description* |
| 2 | `assert` | 0 | 0 | *No description* |
| 3 | `script` | 0 | 0 | *No description* |

### Step: `verify-removal-filter`

Verify that admin user can only see their own removal request

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `script` | 0 | 0 | *No description* |

---

