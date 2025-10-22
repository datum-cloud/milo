# Test: `organizationmembership-fieldselector`

Tests field selector functionality for OrganizationMemberships.

This test verifies:
- Field selectors work on spec.userRef.name
- Field selectors work on spec.organizationRef.name
- Field selectors correctly filter memberships within organization namespaces
- Non-matching field selectors return zero results


## Steps

| # | Name | Bindings | Try | Catch | Finally | Cleanup |
|:-:|---|:-:|:-:|:-:|:-:|:-:|
| 1 | [setup](#step-setup) | 0 | 5 | 0 | 0 | 0 |
| 2 | [direct-field-selector-on-org-cluster](#step-direct-field-selector-on-org-cluster) | 0 | 1 | 0 | 0 | 0 |
| 3 | [org-scoped-fieldselector-behavior](#step-org-scoped-fieldselector-behavior) | 0 | 1 | 0 | 0 | 0 |

### Step: `setup`

Create Organization and OrganizationMemberships for field selector testing

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | *No description* |
| 2 | `wait` | 0 | 0 | *No description* |
| 3 | `apply` | 0 | 0 | *No description* |
| 4 | `apply` | 0 | 0 | *No description* |
| 5 | `wait` | 0 | 0 | *No description* |

### Step: `direct-field-selector-on-org-cluster`

Test field selector filtering on userRef.name and organizationRef.name

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `script` | 0 | 0 | *No description* |

### Step: `org-scoped-fieldselector-behavior`

Verify field selector correctly narrows results within organization namespace

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `script` | 0 | 0 | *No description* |

---

