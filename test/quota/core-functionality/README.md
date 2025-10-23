# Test: `quota-core-functionality`

Tests the core functionality of the quota system including resource registration,
claim creation policies, automatic claim generation, quota enforcement, and cleanup.

This test verifies:
- ResourceRegistrations become active and track claiming resources
- ClaimCreationPolicies configure automatic claim generation
- Projects automatically create ResourceClaims when created
- Quota limits are enforced (projects beyond quota are denied)
- Denied claims are automatically cleaned up
- Successful claims are cleaned up when resources are deleted


## Steps

| # | Name | Bindings | Try | Catch | Finally | Cleanup |
|:-:|---|:-:|:-:|:-:|:-:|:-:|
| 1 | [setup-base-infrastructure](#step-setup-base-infrastructure) | 0 | 3 | 0 | 0 | 0 |
| 2 | [setup-claim-creation-policy](#step-setup-claim-creation-policy) | 0 | 2 | 0 | 0 | 0 |
| 3 | [setup-basic-organization](#step-setup-basic-organization) | 0 | 3 | 0 | 0 | 0 |
| 4 | [create-basic-resource-grant](#step-create-basic-resource-grant) | 0 | 2 | 0 | 0 | 0 |
| 5 | [test-projects-within-quota](#step-test-projects-within-quota) | 0 | 4 | 0 | 0 | 0 |
| 6 | [verify-resource-claims](#step-verify-resource-claims) | 0 | 1 | 0 | 0 | 0 |
| 7 | [test-quota-enforcement](#step-test-quota-enforcement) | 0 | 1 | 0 | 0 | 0 |
| 8 | [verify-denied-claim-cleanup](#step-verify-denied-claim-cleanup) | 0 | 1 | 0 | 0 | 0 |
| 9 | [delete-successful-projects](#step-delete-successful-projects) | 0 | 2 | 0 | 0 | 0 |
| 10 | [verify-complete-cleanup](#step-verify-complete-cleanup) | 0 | 1 | 0 | 0 | 0 |

### Step: `setup-base-infrastructure`

Register the 'projects-per-org' resource type in the quota system and verify it becomes active.
This ResourceRegistration defines what resource is being tracked and how claims are evaluated.


#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `create` | 0 | 0 | Create ResourceRegistration for projects |
| 2 | `wait` | 0 | 0 | Wait for ResourceRegistration to become active |
| 3 | `assert` | 0 | 0 | Verify ResourceRegistration status and configuration |

### Step: `setup-claim-creation-policy`

Create a ClaimCreationPolicy that automatically generates ResourceClaims when Projects are created.
The policy specifies which resource to claim and the parent context for quota enforcement.


#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `create` | 0 | 0 | Create ClaimCreationPolicy for automatic project claim generation |
| 2 | `wait` | 0 | 0 | Wait for ClaimCreationPolicy to be ready |

### Step: `setup-basic-organization`

Create an Organization with a test User and OrganizationMembership.
The Organization will serve as the parent context for quota enforcement.


#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `create` | 0 | 0 | Create test Organization |
| 2 | `wait` | 0 | 0 | Wait for Organization namespace to be active |
| 3 | `assert` | 0 | 0 | Verify Organization was created successfully |

### Step: `create-basic-resource-grant`

Create a ResourceGrant that allocates 2 projects to the Organization.
This grant establishes the quota limit that will be enforced.


#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `create` | 0 | 0 | Create ResourceGrant with quota allowance |
| 2 | `wait` | 0 | 0 | Wait for ResourceGrant to become active |

### Step: `test-projects-within-quota`

Create 2 projects within the quota limit (grant allows 2 projects).
Verify that both projects are created successfully and ResourceClaims are auto-generated.


#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `create` | 0 | 0 | Create 2 projects within quota limit |
| 2 | `wait` | 0 | 0 | Wait for first project to be ready |
| 3 | `wait` | 0 | 0 | Wait for second project to be ready |
| 4 | `assert` | 0 | 0 | Verify both projects were created successfully |

### Step: `verify-resource-claims`

Verify that ResourceClaims were automatically created for both projects.
Claims should be in Granted state and linked to the ResourceGrant.


#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `assert` | 0 | 0 | Verify ResourceClaims exist and are granted |

### Step: `test-quota-enforcement`

Attempt to create a 3rd project that exceeds the quota limit.
The admission webhook should deny the project creation.


#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `create` | 0 | 0 | Attempt to create project beyond quota (should fail) |

### Step: `verify-denied-claim-cleanup`

Verify that no ResourceClaim exists for the denied project.
The system should not create claims for resources that fail admission.


#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `error` | 0 | 0 | Verify no ResourceClaim exists for denied project |

### Step: `delete-successful-projects`

Delete the 2 successful projects to verify ResourceClaim cleanup.
Claims should be automatically deleted when their owning resources are removed.


#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `delete` | 0 | 0 | Delete first project |
| 2 | `delete` | 0 | 0 | Delete second project |

### Step: `verify-complete-cleanup`

Verify that all ResourceClaims for the test scenario have been cleaned up.
This confirms garbage collection works correctly via owner references.


#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `error` | 0 | 0 | Verify no ResourceClaims remain for this test |

---

