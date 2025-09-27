# Test: identity-sessions

This suite validates Milo's virtual Sessions API end-to-end against a fake
provider CRD served in the Milo apiserver. It:
- Installs the fake provider CRD
- Creates two provider Sessions (zitadel.identity.miloapis.com)
- Asserts the public identity API surfaces those Sessions with expected fields
- Deletes the Sessions via the public API and waits for provider deletion

## Steps

| # | Name | Bindings | Try | Catch | Finally | Cleanup |
|:-:|---|:-:|:-:|:-:|:-:|:-:|
| 1 | [install-provider-crd](#step-install-provider-crd) | 0 | 2 | 0 | 0 | 0 |
| 2 | [create-provider-sessions](#step-create-provider-sessions) | 0 | 2 | 0 | 0 | 0 |
| 3 | [verify-public-api](#step-verify-public-api) | 0 | 2 | 0 | 0 | 0 |
| 4 | [delete-via-public-api](#step-delete-via-public-api) | 0 | 4 | 0 | 0 | 0 |

### Step: install-provider-crd

Install the fake provider CRD and wait until it is Established.

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | Apply CRD: resources/provider-crd.yaml |
| 2 | `wait` | 0 | 0 | Wait for CRD Established |

### Step: create-provider-sessions

Create provider Sessions and assert they exist as provider resources.

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | Apply provider Sessions: resources/provider-sessions.yaml |
| 2 | `assert` | 0 | 0 | Assert provider Sessions exist |

### Step: verify-public-api

Assert the public identity API exposes the Sessions with expected status fields.

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `assert` | 0 | 0 | Assert public Session sess-abc123 (identity API) |
| 2 | `assert` | 0 | 0 | Assert public Session sess-def456 (identity API) |

### Step: delete-via-public-api

Delete Sessions via the public identity API and wait for provider-side deletion.

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `delete` | 0 | 0 | Delete via public API: sess-abc123 |
| 2 | `delete` | 0 | 0 | Delete via public API: sess-def456 |
| 3 | `wait` | 0 | 0 | Wait for provider sess-abc123 deletion |
| 4 | `wait` | 0 | 0 | Wait for provider sess-def456 deletion |

---

