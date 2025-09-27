Virtual Sessions API (identity.miloapis.com/v1alpha1)

This package implements Milo's public, virtual Sessions API. It exposes
identity.miloapis.com/v1alpha1, Resource=sessions without persisting anything
to etcd. All operations are delegated to a provider API (e.g.,
zitadel.identity.miloapis.com) using user impersonation so RBAC and audit
reflect the end-user.

Components and flow
- StorageProvider (internal/apiserver/storage/identity/storageprovider.go)
  - Installs API group identity.miloapis.com/v1alpha1 and wires the sessions
    resource to the REST storage.
- REST storage (this package, rest.go)
  - Implements List/Get/Delete for a cluster-scoped virtual resource.
  - Extracts the request user from context and delegates to a Provider.
  - Implements Table output so kubectl get sessions.identity.miloapis.com
    works without -o json.
- Provider (this package, dynamic.go)
  - DynamicProvider uses a dynamic client with impersonation to call a
    provider GVR (configured via flags) and converts unstructured ↔ typed
    identity/v1alpha1 objects.
  - Strict pass-through: Milo does not mutate or default provider fields.

Configuration
- Enable the feature and point to a provider GVR via flags on the apiserver:
  - --feature-sessions=true
  - --sessions-provider-group=zitadel.identity.miloapis.com
  - --sessions-provider-version=v1alpha1
  - --sessions-provider-resource=sessions
  - Optional: --provider-timeout, --provider-retries,
    --impersonate-forward-extras=org,tenant
- At startup Milo builds a GroupVersionResource from the flags and uses it in
  DynamicProvider for all requests.

RBAC
- Milo apiserver ServiceAccount must be allowed to impersonate identities:
  authentication.k8s.io users, groups, serviceaccounts, userextras/* with
  verb impersonate.
- End-user access is enforced by the provider. Because Milo impersonates the
  caller, end users must have verbs on the provider GVR (e.g.,
  zitadel.identity.miloapis.com/sessions).

Local testing
- Fake provider CRD in Milo (dev-only): apply a CRD and sample Sessions under
  zitadel.identity.miloapis.com/v1alpha1 in the Milo apiserver and point the
  flags to that GVR.
- Chainsaw E2E: see test/identity/sessions/ for a suite that applies provider
  Sessions, verifies the public API, and deletes via the public API.

Naming & structure
- internal/apiserver/storage/identity/storageprovider.go — group installer
- internal/apiserver/identity/sessions/rest.go — REST storage
- internal/apiserver/identity/sessions/dynamic.go — provider implementation

