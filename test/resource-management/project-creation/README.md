# Test: `project-creation`

Tests basic Project creation and multi-cluster visibility.

This test verifies:
- Projects can be created within an Organization namespace
- Projects reach Ready status
- Projects are visible in both organization and main cluster contexts
- User and OrganizationMembership setup works correctly


## Steps

| # | Name | Bindings | Try | Catch | Finally | Cleanup |
|:-:|---|:-:|:-:|:-:|:-:|:-:|
| 1 | [setup-organization](#step-setup-organization) | 0 | 5 | 0 | 0 | 0 |
| 2 | [test-project-creation-and-ready-status](#step-test-project-creation-and-ready-status) | 0 | 3 | 0 | 0 | 0 |
| 3 | [verify-project-in-main-cluster](#step-verify-project-in-main-cluster) | 0 | 1 | 0 | 0 | 0 |

### Step: `setup-organization`

Create Organization, User, and OrganizationMembership for project testing

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | *No description* |
| 2 | `wait` | 0 | 0 | *No description* |
| 3 | `apply` | 0 | 0 | *No description* |
| 4 | `wait` | 0 | 0 | *No description* |
| 5 | `apply` | 0 | 0 | *No description* |

### Step: `test-project-creation-and-ready-status`

Create Project in organization context and verify it reaches Ready status

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | *No description* |
| 2 | `wait` | 0 | 0 | *No description* |
| 3 | `assert` | 0 | 0 | *No description* |

### Step: `verify-project-in-main-cluster`

Verify Project is visible in main cluster with correct status

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `assert` | 0 | 0 | *No description* |

---

