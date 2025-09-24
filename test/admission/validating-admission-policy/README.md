# Test: `validating-admission-policy-project`

Validates cluster-wide enforcement of a ValidatingAdmissionPolicy on `Project` resources and verifies the webhook populates organization ownership (label and ownerReferences) on successful creates.

- Bad names (not starting with `vap-ok-`) are denied in any org
- Good names are allowed; resulting `Project` has:
  - label `resourcemanager.miloapis.com/organization-name`
  - an `ownerReferences[0]` to the owning `Organization`
  - `spec.ownerRef` set to the owning `Organization`

Run:

```
task test:end-to-end -- admission/validating-admission-policy
```

Regenerate docs:

```
bin/chainsaw build docs --test-dir test/admission/validating-admission-policy
```

## Steps

| # | Name | Bindings | Try | Catch | Finally | Cleanup |
|:-:|---|:-:|:-:|:-:|:-:|:-:|
| 1 | [apply-policy-and-orgs](#step-apply-policy-and-orgs) | 0 | 4 | 0 | 0 | 0 |
| 2 | [deny-projects-with-bad-names-orgA](#step-deny-projects-with-bad-names-orga) | 0 | 1 | 0 | 0 | 0 |
| 3 | [deny-projects-with-bad-names-orgB](#step-deny-projects-with-bad-names-orgb) | 0 | 1 | 0 | 0 | 0 |
| 4 | [allow-projects-with-ok-prefix-orgA](#step-allow-projects-with-ok-prefix-orga) | 0 | 3 | 0 | 0 | 0 |
| 5 | [allow-projects-with-ok-prefix-orgB](#step-allow-projects-with-ok-prefix-orgb) | 0 | 3 | 0 | 0 | 0 |

### Step: `apply-policy-and-orgs`

Installs a `ValidatingAdmissionPolicy` and `ValidatingAdmissionPolicyBinding` that require project names to start with `vap-ok-`, then creates two organizations used by subsequent steps.

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | Apply policy and binding |
| 2 | `sleep` | 0 | 0 | Give the admission chain time to reconcile |
| 3 | `apply` | 0 | 0 | Create org A |
| 4 | `apply` | 0 | 0 | Create org B |

### Step: `deny-projects-with-bad-names-orgA`

Ensures a badly named project is denied when submitted to org A.

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `error` | 0 | 0 | Expect create of `bad-vap-proj-a` to fail |

### Step: `deny-projects-with-bad-names-orgB`

Ensures a badly named project is denied when submitted to org B.

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `error` | 0 | 0 | Expect create of `bad-vap-proj-b` to fail |

### Step: `allow-projects-with-ok-prefix-orgA`

Creates a valid project in org A, waits for Ready, and asserts ownership metadata populated by the webhook.

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | Create `vap-ok-good-a` in org A |
| 2 | `wait` | 0 | 0 | Wait for `Ready=True` |
| 3 | `assert` | 0 | 0 | Check label, ownerReferences, and spec.ownerRef |

### Step: `allow-projects-with-ok-prefix-orgB`

Creates a valid project in org B, waits for Ready, and asserts ownership metadata populated by the webhook.

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | Create `vap-ok-good-b` in org B |
| 2 | `wait` | 0 | 0 | Wait for `Ready=True` |
| 3 | `assert` | 0 | 0 | Check label, ownerReferences, and spec.ownerRef |

---

