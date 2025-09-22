# Test: `organizationmembership-fieldselector`

Validates `fieldSelector` behavior for `OrganizationMembership` within an org-scoped session.

It covers:
- Filtering by `spec.userRef.name` (single result for a specific user, zero for non-existent)
- Filtering by `spec.organizationRef.name` (both memberships in the org)
- Plain list returns both, adding `spec.userRef.name` narrows to one

Run:

```
task test:end-to-end -- resource-management/organizationmembership-fieldselector
```

Regenerate docs:

```
bin/chainsaw build docs --test-dir test/resource-management/organizationmembership-fieldselector
```

## Steps

| # | Name | Bindings | Try | Catch | Finally | Cleanup |
|:-:|---|:-:|:-:|:-:|:-:|:-:|
| 1 | [setup](#step-setup) | 0 | 5 | 0 | 0 | 0 |
| 2 | [direct-field-selector-on-org-cluster](#step-direct-field-selector-on-org-cluster) | 0 | 1 | 0 | 0 | 0 |
| 3 | [org-scoped-fieldselector-behavior](#step-org-scoped-fieldselector-behavior) | 0 | 1 | 0 | 0 | 0 |

### Step: `setup`

Creates the org and the two `OrganizationMembership` resources targeting users `admin` and `test-user`, then waits for one to become Ready.

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | Create org |
| 2 | `wait` | 0 | 0 | Wait for org namespace to be Active |
| 3 | `apply` | 0 | 0 | Create membership for admin |
| 4 | `apply` | 0 | 0 | Create membership for test-user |
| 5 | `wait` | 0 | 0 | Wait for membership Ready |

### Step: `direct-field-selector-on-org-cluster`

Runs shell assertions against the API using `--field-selector`.

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `script` | 0 | 0 | Check user filter (1 match and 0 for non-existent) and org filter (2 matches) |

### Step: `org-scoped-fieldselector-behavior`

Verifies plain list vs user-filter interaction in org scope.

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|---|
| 1 | `script` | 0 | 0 | Plain list returns 2; filtering by `test-user` returns 1 |

---

