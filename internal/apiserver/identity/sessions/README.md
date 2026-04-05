Virtual Sessions API (identity.miloapis.com/v1alpha1)

This package implements Milo's public, virtual Sessions API. It exposes
identity.miloapis.com/v1alpha1, Resource=sessions without persisting anything
to etcd. All operations are delegated to a provider API so RBAC and audit
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
  - --sessions-provider-url=https://identity.miloapis.com
  - --sessions-provider-ca-file=
  - --sessions-provider-client-cert=
  - --sessions-provider-client-key=
  - Optional: --provider-timeout, --provider-retries,
    --forward-extras=org,tenant
- At startup Milo builds a GroupVersionResource from the flags and uses it in
  DynamicProvider for all requests.


Naming & structure
- internal/apiserver/storage/identity/storageprovider.go — group installer
- internal/apiserver/identity/sessions/rest.go — REST storage
- internal/apiserver/identity/sessions/dynamic.go — provider implementation

Field Selector Support for Staff Users
The Sessions API supports field selectors to enable staff users to query
other users' active sessions. This is required for support and administrative
purposes in the staff portal.

Supported field selectors:
- status.userUID=<user-id> — Query sessions for a specific user

Authorization:
- Regular users: Can only list their own sessions (field selectors are ignored)
- Staff users: Can use field selectors to query other users' sessions
  - Must be members of privileged groups in the identity provider (e.g., staff-users, fraud-manager)
  - Authorization is enforced by the backend provider (auth-provider-zitadel)
  - Requires appropriate PolicyBinding in Milo for RBAC

Example usage:
  # Regular user (sees only their own sessions)
  kubectl get sessions

  # Staff user (can query specific user's sessions)
  kubectl get sessions --field-selector=status.userUID=<target-user-id>

Security model:
1. Milo RBAC: PolicyBinding grants access to sessions resource
2. Backend authorization: Provider validates group membership for field selector usage
3. Audit logging: All requests are logged with user context for compliance

