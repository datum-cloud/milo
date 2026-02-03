# Test: `contactgroup-visibility`

End-to-end tests for ContactGroup visibility filtering.

This test verifies:
- Public groups are visible to all users when listing
- Private groups are NOT visible to users without membership
- Private groups become visible once the user has a ContactGroupMembership


## Steps

| # | Name | Bindings | Try | Catch | Finally | Cleanup |
|:-:|---|:-:|:-:|:-:|:-:|:-:|
| 1 | [setup-contact](#step-setup-contact) | 0 | 2 | 0 | 0 | 0 |
| 2 | [create-first-public-group-and-verify-visible](#step-create-first-public-group-and-verify-visible) | 0 | 3 | 0 | 0 | 0 |
| 3 | [create-second-public-group-and-verify-both-visible](#step-create-second-public-group-and-verify-both-visible) | 0 | 3 | 0 | 0 | 0 |
| 4 | [create-private-groups-and-verify-not-visible](#step-create-private-groups-and-verify-not-visible) | 0 | 5 | 0 | 0 | 0 |
| 5 | [create-first-membership-and-verify-private-still-hidden](#step-create-first-membership-and-verify-private-still-hidden) | 0 | 3 | 0 | 0 | 0 |
| 6 | [create-second-membership-and-verify-private-still-hidden](#step-create-second-membership-and-verify-private-still-hidden) | 0 | 3 | 0 | 0 | 0 |

### Step: `setup-contact`

Create the contact associated with admin user

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | *No description* |
| 2 | `assert` | 0 | 0 | *No description* |

### Step: `create-first-public-group-and-verify-visible`

Create first public group and verify user can see it

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | *No description* |
| 2 | `assert` | 0 | 0 | *No description* |
| 3 | `script` | 0 | 0 | *No description* |

### Step: `create-second-public-group-and-verify-both-visible`

Create second public group and verify user can see both

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | *No description* |
| 2 | `assert` | 0 | 0 | *No description* |
| 3 | `script` | 0 | 0 | *No description* |

### Step: `create-private-groups-and-verify-not-visible`

Create two private groups and verify user cannot see them (only public groups)

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | *No description* |
| 2 | `apply` | 0 | 0 | *No description* |
| 3 | `assert` | 0 | 0 | *No description* |
| 4 | `assert` | 0 | 0 | *No description* |
| 5 | `script` | 0 | 0 | *No description* |

### Step: `create-first-membership-and-verify-private-still-hidden`

Create membership for first private group but verify user still ONLY sees public groups (strict public filtering)

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | *No description* |
| 2 | `assert` | 0 | 0 | *No description* |
| 3 | `script` | 0 | 0 | *No description* |

### Step: `create-second-membership-and-verify-private-still-hidden`

Create membership for second private group and verify user still ONLY sees public groups

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | *No description* |
| 2 | `assert` | 0 | 0 | *No description* |
| 3 | `script` | 0 | 0 | *No description* |

---

