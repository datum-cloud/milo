Virtual UserIdentities API (identity.miloapis.com/v1alpha1)

This package implements Milo's public, virtual UserIdentities API. It exposes
identity.miloapis.com/v1alpha1, Resource=useridentities without persisting anything
to etcd. All operations are delegated to a provider API so RBAC and audit
reflect the end-user.

Components and flow
- StorageProvider (internal/apiserver/storage/identity/storageprovider.go)
  - Installs API group identity.miloapis.com/v1alpha1 and wires the useridentities
    resource to the REST storage.
- REST storage (this package, rest.go)
  - Implements List/Get for a cluster-scoped virtual resource.
  - Extracts the request user from context and delegates to a Provider.
  - Implements Table output so kubectl get useridentities.identity.miloapis.com
    works without -o json.
- Provider (this package, dynamic.go)
  - DynamicProvider uses a dynamic client with impersonation to call a
    provider GVR (configured via flags) and converts unstructured ↔ typed
    identity/v1alpha1 objects.
  - Strict pass-through: Milo does not mutate or default provider fields.

Configuration
- Enable the feature and point to a provider GVR via flags on the apiserver:
  - --feature-useridentities=true
  - --useridentities-provider-url=https://identity.miloapis.com
  - --useridentities-provider-ca-file=
  - --useridentities-provider-client-cert=
  - --useridentities-provider-client-key=
  - Optional: --provider-timeout, --provider-retries,
    --forward-extras=org,tenant
- At startup Milo builds a GroupVersionResource from the flags and uses it in
  DynamicProvider for all requests.


Naming & structure
- internal/apiserver/storage/identity/storageprovider.go — group installer
- internal/apiserver/identity/useridentities/rest.go — REST storage
- internal/apiserver/identity/useridentities/dynamic.go — provider implementation

Read-only resource
Unlike sessions, useridentities is a read-only resource. Users cannot create,
update, or delete user identities through the Kubernetes API. Identity linking
and unlinking is managed through the external identity provider (e.g., Zitadel).
