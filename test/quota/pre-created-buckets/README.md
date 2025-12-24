# Test: `quota-pre-created-buckets`

Tests that allowance buckets are pre-created when ResourceGrants become active,
allowing consumers to see their quota limits immediately without waiting for claims.

This test verifies:
- AllowanceBuckets are pre-created when ResourceGrants become active
- Pre-created buckets have the correct spec and labels
- Pre-created buckets show accurate limits from grants
- Claims can use pre-created buckets successfully
- Multiple grants contribute to the same bucket's limit


## Steps

| # | Name | Bindings | Try | Catch | Finally | Cleanup |
|:-:|---|:-:|:-:|:-:|:-:|:-:|
| 1 | [setup-base-infrastructure](#step-setup-base-infrastructure) | 0 | 4 | 0 | 0 | 0 |
| 2 | [setup-test-organization](#step-setup-test-organization) | 0 | 2 | 0 | 0 | 0 |
| 3 | [create-first-resource-grant](#step-create-first-resource-grant) | 0 | 2 | 0 | 0 | 0 |
| 4 | [verify-buckets-pre-created](#step-verify-buckets-pre-created) | 0 | 2 | 0 | 0 | 0 |
| 5 | [create-additional-grant](#step-create-additional-grant) | 0 | 2 | 0 | 0 | 0 |
| 6 | [verify-aggregated-limits](#step-verify-aggregated-limits) | 0 | 2 | 0 | 0 | 0 |
| 7 | [test-claim-with-pre-created-bucket](#step-test-claim-with-pre-created-bucket) | 0 | 3 | 0 | 0 | 0 |
| 8 | [verify-bucket-usage](#step-verify-bucket-usage) | 0 | 1 | 0 | 0 | 0 |

### Step: `setup-base-infrastructure`

Register resource types in the quota system and verify they become active.


#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `create` | 0 | 0 | Create ResourceRegistration for projects |
| 2 | `wait` | 0 | 0 | Wait for ResourceRegistration to become active |
| 3 | `create` | 0 | 0 | Create ResourceRegistration for CPU |
| 4 | `wait` | 0 | 0 | Wait for CPU ResourceRegistration to become active |

### Step: `setup-test-organization`

Create an Organization that will receive quota grants.


#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `create` | 0 | 0 | Create test Organization |
| 2 | `wait` | 0 | 0 | Wait for Organization namespace to be active |

### Step: `create-first-resource-grant`

Create a ResourceGrant that allocates resources to the Organization.
This should trigger pre-creation of AllowanceBuckets.


#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `create` | 0 | 0 | Create ResourceGrant with multiple resource types |
| 2 | `wait` | 0 | 0 | Wait for ResourceGrant to become active |

### Step: `verify-buckets-pre-created`

Verify that AllowanceBuckets were pre-created for both resource types
when the grant became active, without requiring any claims.


#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `assert` | 0 | 0 | Verify project bucket exists |
| 2 | `assert` | 0 | 0 | Verify CPU bucket exists |

### Step: `create-additional-grant`

Create an additional ResourceGrant for the same consumer and resource types.
The existing buckets should be updated with new limits.


#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `create` | 0 | 0 | Create second ResourceGrant |
| 2 | `wait` | 0 | 0 | Wait for second ResourceGrant to become active |

### Step: `verify-aggregated-limits`

Verify that the pre-created buckets now show aggregated limits
from both ResourceGrants.


#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `assert` | 0 | 0 | Verify project bucket has aggregated limits |
| 2 | `assert` | 0 | 0 | Verify CPU bucket has aggregated limits |

### Step: `test-claim-with-pre-created-bucket`

Create a ResourceClaim and verify it successfully uses the pre-created bucket.


#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `create` | 0 | 0 | Create ResourceClaim for projects |
| 2 | `wait` | 0 | 0 | Wait for ResourceClaim to be granted |
| 3 | `assert` | 0 | 0 | Verify claim was granted using pre-created bucket |

### Step: `verify-bucket-usage`

Verify that the pre-created bucket correctly shows usage from the claim.


#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `assert` | 0 | 0 | Verify project bucket shows allocated resources |

---

