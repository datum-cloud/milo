# Test: `quota-enforcement-edge-cases`

Tests edge cases and boundary conditions in the quota enforcement system.

This test verifies:
- Zero and negative claim amounts are rejected
- Invalid ClaimCreationPolicy configurations are detected
- Policy validation catches missing resource references
- Boundary conditions are properly handled


## Steps

| # | Name | Bindings | Try | Catch | Finally | Cleanup |
|:-:|---|:-:|:-:|:-:|:-:|:-:|
| 1 | [setup-base-infrastructure](#step-setup-base-infrastructure) | 0 | 2 | 0 | 0 | 0 |
| 2 | [setup-claim-creation-policy](#step-setup-claim-creation-policy) | 0 | 2 | 0 | 0 | 0 |
| 3 | [setup-quota-enforcement-organization](#step-setup-quota-enforcement-organization) | 0 | 2 | 0 | 0 | 0 |
| 4 | [create-limited-resource-grant](#step-create-limited-resource-grant) | 0 | 2 | 0 | 0 | 0 |
| 5 | [test-boundary-conditions](#step-test-boundary-conditions) | 0 | 2 | 0 | 0 | 0 |
| 6 | [test-policy-validation](#step-test-policy-validation) | 0 | 1 | 0 | 0 | 0 |

### Step: `setup-base-infrastructure`

Register the resource type and wait for it to become active.


#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | Create ResourceRegistration |
| 2 | `wait` | 0 | 0 | Wait for ResourceRegistration to become active |

### Step: `setup-claim-creation-policy`

Create ClaimCreationPolicy for automatic claim generation.


#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | Create ClaimCreationPolicy |
| 2 | `wait` | 0 | 0 | Wait for policy to be ready |

### Step: `setup-quota-enforcement-organization`

Create Organization, User, and OrganizationMembership for testing.


#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | Create Organization |
| 2 | `wait` | 0 | 0 | Wait for Organization namespace to be active |

### Step: `create-limited-resource-grant`

Create ResourceGrant with a limited allowance for edge case testing.


#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | Create ResourceGrant |
| 2 | `wait` | 0 | 0 | Wait for ResourceGrant to become active |

### Step: `test-boundary-conditions`

Test that invalid ResourceClaim configurations are rejected.
Zero and negative amounts should be prevented by API validation.


#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | Attempt to create claim with zero amount (should fail) |
| 2 | `apply` | 0 | 0 | Attempt to create claim with negative amount (should fail) |

### Step: `test-policy-validation`

Test that ClaimCreationPolicy validation catches invalid configurations.
Policies referencing non-existent resources should be rejected at admission time.


#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `create` | 0 | 0 | Attempt to create policy with missing resource reference (should fail) |

---

