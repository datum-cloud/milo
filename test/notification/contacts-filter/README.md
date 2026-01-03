# Test: `contacts-filter`

End-to-end tests for Contact filtering.

This test verifies:
- User can only see their own contacts when listing
- User cannot see contacts belonging to other users
- Manual field selectors are overridden to enforce user scope


## Steps

| # | Name | Bindings | Try | Catch | Finally | Cleanup |
|:-:|---|:-:|:-:|:-:|:-:|:-:|
| 1 | [setup-admin-contact](#step-setup-admin-contact) | 0 | 2 | 0 | 0 | 0 |
| 2 | [setup-test-user-contact](#step-setup-test-user-contact) | 0 | 2 | 0 | 0 | 0 |
| 3 | [verify-filter-only-shows-test-user-contact](#step-verify-filter-only-shows-test-user-contact) | 0 | 1 | 0 | 0 | 0 |
| 4 | [verify-field-selector-override](#step-verify-field-selector-override) | 0 | 1 | 0 | 0 | 0 |

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

### Step: `verify-filter-only-shows-test-user-contact`

Verify that test-user can only see their own contact

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `script` | 0 | 0 | *No description* |

### Step: `verify-field-selector-override`

Verify that attempting to spy on other users is blocked by filter override

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `script` | 0 | 0 | *No description* |

---

