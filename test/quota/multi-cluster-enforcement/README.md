# Test: `multi-cluster-enforcement`

Tests quota system for Secrets across multiple control planes using Milo's storage filtering.

Validates: GrantCreationPolicy creates grants for projects, ClaimCreationPolicy creates claims
for Secrets, quota enforced across project control planes, control plane isolation.


## Steps

| # | Name | Bindings | Try | Catch | Finally | Cleanup |
|:-:|---|:-:|:-:|:-:|:-:|:-:|
| 1 | [setup-resource-registration](#step-setup-resource-registration) | 0 | 2 | 0 | 0 | 0 |
| 2 | [setup-grant-creation-policy](#step-setup-grant-creation-policy) | 0 | 2 | 0 | 0 | 0 |
| 3 | [setup-test-organization](#step-setup-test-organization) | 0 | 2 | 0 | 0 | 0 |
| 4 | [create-project-1-in-org-control-plane](#step-create-project-1-in-org-control-plane) | 0 | 2 | 0 | 0 | 0 |
| 5 | [verify-grant-for-project-1](#step-verify-grant-for-project-1) | 0 | 1 | 0 | 0 | 0 |
| 6 | [verify-bucket-pre-created](#step-verify-bucket-pre-created) | 0 | 1 | 0 | 0 | 0 |
| 7 | [setup-claim-creation-policy](#step-setup-claim-creation-policy) | 0 | 2 | 0 | 0 | 0 |
| 8 | [create-secret-1-in-project-1](#step-create-secret-1-in-project-1) | 0 | 1 | 0 | 0 | 0 |
| 9 | [verify-claim-for-secret-1](#step-verify-claim-for-secret-1) | 0 | 1 | 0 | 0 | 0 |
| 10 | [verify-bucket-usage-1-of-5](#step-verify-bucket-usage-1-of-5) | 0 | 1 | 0 | 0 | 0 |
| 11 | [create-secret-2-in-project-1](#step-create-secret-2-in-project-1) | 0 | 1 | 0 | 0 | 0 |
| 12 | [verify-claim-for-secret-2](#step-verify-claim-for-secret-2) | 0 | 1 | 0 | 0 | 0 |
| 13 | [verify-bucket-usage-2-of-5](#step-verify-bucket-usage-2-of-5) | 0 | 1 | 0 | 0 | 0 |
| 14 | [delete-secret-1](#step-delete-secret-1) | 0 | 1 | 0 | 0 | 0 |
| 15 | [verify-bucket-usage-after-deletion](#step-verify-bucket-usage-after-deletion) | 0 | 1 | 0 | 0 | 0 |

### Step: `setup-resource-registration`

Register unique resource type for secrets claimed by Projects

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | *No description* |
| 2 | `wait` | 0 | 0 | *No description* |

### Step: `setup-grant-creation-policy`

Create GrantCreationPolicy to grant secret quota to projects

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | *No description* |
| 2 | `wait` | 0 | 0 | *No description* |

### Step: `setup-test-organization`

Create test organization

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | *No description* |
| 2 | `wait` | 0 | 0 | *No description* |

### Step: `create-project-1-in-org-control-plane`

Create project in org control plane

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | *No description* |
| 2 | `wait` | 0 | 0 | *No description* |

### Step: `verify-grant-for-project-1`

Confirm grant is created in project control plane after provisioning

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `wait` | 0 | 0 | *No description* |

### Step: `verify-bucket-pre-created`

Verify AllowanceBucket is pre-created in project control plane when grant becomes active

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `assert` | 0 | 0 | *No description* |

### Step: `setup-claim-creation-policy`

Register ClaimCreationPolicy for Secrets

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | *No description* |
| 2 | `wait` | 0 | 0 | *No description* |

### Step: `create-secret-1-in-project-1`

Create secret in project control plane

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | *No description* |

### Step: `verify-claim-for-secret-1`

Confirm resource claim is created for secret in project

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `wait` | 0 | 0 | *No description* |

### Step: `verify-bucket-usage-1-of-5`

Verify bucket shows 1 secret allocated

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `assert` | 0 | 0 | *No description* |

### Step: `create-secret-2-in-project-1`

Create second secret in project control plane

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | *No description* |

### Step: `verify-claim-for-secret-2`

Verify second claim created and granted

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `wait` | 0 | 0 | *No description* |

### Step: `verify-bucket-usage-2-of-5`

Verify bucket shows 2 secrets allocated

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `assert` | 0 | 0 | *No description* |

### Step: `delete-secret-1`

Delete first secret to free quota

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `delete` | 0 | 0 | *No description* |

### Step: `verify-bucket-usage-after-deletion`

Verify bucket shows quota freed after deletion

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `assert` | 0 | 0 | *No description* |

---

