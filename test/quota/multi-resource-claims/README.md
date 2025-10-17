# Test: `multi-resource-claims`

Tests ResourceClaims that request multiple resource types simultaneously.

This test verifies:
- Claims can request multiple different resource types in one claim
- All requested resources must be available for claim to be granted
- Partial availability results in claim denial
- Duplicate resource types in a single claim are rejected
- Multi-resource claims properly consume from multiple grants


## Steps

| # | Name | Bindings | Try | Catch | Finally | Cleanup |
|:-:|---|:-:|:-:|:-:|:-:|:-:|
| 1 | [setup-base-infrastructure](#step-setup-base-infrastructure) | 0 | 5 | 0 | 0 | 0 |
| 2 | [setup-multi-resource-organization](#step-setup-multi-resource-organization) | 0 | 4 | 0 | 0 | 0 |
| 3 | [setup-multi-resource-grants](#step-setup-multi-resource-grants) | 0 | 4 | 0 | 0 | 0 |
| 4 | [multi-test-multi-resource-claim-success](#step-multi-test-multi-resource-claim-success) | 0 | 3 | 0 | 0 | 0 |
| 5 | [multi-test-multi-resource-claim-partial-deny](#step-multi-test-multi-resource-claim-partial-deny) | 0 | 3 | 0 | 0 | 0 |
| 6 | [test-invalid-duplicate-claim](#step-test-invalid-duplicate-claim) | 0 | 1 | 0 | 0 | 0 |

### Step: `setup-base-infrastructure`

Register multiple resource types (projects, users, clusters) for testing multi-resource claims.


#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | *No description* |
| 2 | `wait` | 0 | 0 | *No description* |
| 3 | `apply` | 0 | 0 | *No description* |
| 4 | `wait` | 0 | 0 | *No description* |
| 5 | `wait` | 0 | 0 | *No description* |

### Step: `setup-multi-resource-organization`

Create Organization with User and Membership for testing multi-resource claims.


#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | *No description* |
| 2 | `wait` | 0 | 0 | *No description* |
| 3 | `apply` | 0 | 0 | *No description* |
| 4 | `apply` | 0 | 0 | *No description* |

### Step: `setup-multi-resource-grants`

Create ResourceGrants for multiple resource types (users, clusters, projects).
Each grant provides allowance for a different resource type.


#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | *No description* |
| 2 | `wait` | 0 | 0 | *No description* |
| 3 | `wait` | 0 | 0 | *No description* |
| 4 | `wait` | 0 | 0 | *No description* |

### Step: `multi-test-multi-resource-claim-success`

Create a ResourceClaim requesting multiple resource types where all are available.
The claim should be granted since all requested resources have sufficient quota.


#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | *No description* |
| 2 | `wait` | 0 | 0 | *No description* |
| 3 | `assert` | 0 | 0 | *No description* |

### Step: `multi-test-multi-resource-claim-partial-deny`

Create a ResourceClaim requesting multiple resources where one type is unavailable.
The entire claim should be denied (all-or-nothing semantics).


#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | *No description* |
| 2 | `wait` | 0 | 0 | *No description* |
| 3 | `assert` | 0 | 0 | *No description* |

### Step: `test-invalid-duplicate-claim`

Attempt to create a ResourceClaim with duplicate resource types.
This should be rejected by API validation.


#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | *No description* |

---

