# Test: `validating-admission-policy-project`

Tests ValidatingAdmissionPolicy for project naming enforcement across organizations.

This test verifies:
- ValidatingAdmissionPolicies can enforce organization-specific naming rules
- Projects with invalid names are rejected
- Projects with valid prefixes matching their organization are accepted
- Policies work correctly across multiple organization contexts


## Steps

| # | Name | Bindings | Try | Catch | Finally | Cleanup |
|:-:|---|:-:|:-:|:-:|:-:|:-:|
| 1 | [apply-policy-and-orgs](#step-apply-policy-and-orgs) | 0 | 4 | 0 | 0 | 0 |
| 2 | [deny-projects-with-bad-names-orgA](#step-deny-projects-with-bad-names-orgA) | 0 | 1 | 0 | 0 | 0 |
| 3 | [deny-projects-with-bad-names-orgB](#step-deny-projects-with-bad-names-orgB) | 0 | 1 | 0 | 0 | 0 |
| 4 | [allow-projects-with-ok-prefix-orgA](#step-allow-projects-with-ok-prefix-orgA) | 0 | 3 | 0 | 0 | 0 |
| 5 | [allow-projects-with-ok-prefix-orgB](#step-allow-projects-with-ok-prefix-orgB) | 0 | 3 | 0 | 0 | 0 |

### Step: `apply-policy-and-orgs`

Create ValidatingAdmissionPolicy and test organizations

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | *No description* |
| 2 | `sleep` | 0 | 0 | *No description* |
| 3 | `apply` | 0 | 0 | *No description* |
| 4 | `apply` | 0 | 0 | *No description* |

### Step: `deny-projects-with-bad-names-orgA`

Verify projects with invalid names are rejected in orgA

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `error` | 0 | 0 | *No description* |

### Step: `deny-projects-with-bad-names-orgB`

Verify projects with invalid names are rejected in orgB

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `error` | 0 | 0 | *No description* |

### Step: `allow-projects-with-ok-prefix-orgA`

Verify projects with correct orgA prefix are accepted

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | *No description* |
| 2 | `wait` | 0 | 0 | *No description* |
| 3 | `assert` | 0 | 0 | *No description* |

### Step: `allow-projects-with-ok-prefix-orgB`

Verify projects with correct orgB prefix are accepted

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | *No description* |
| 2 | `wait` | 0 | 0 | *No description* |
| 3 | `assert` | 0 | 0 | *No description* |

---

