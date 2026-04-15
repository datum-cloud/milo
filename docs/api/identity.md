# API Reference

## Packages

- [identity.miloapis.com/v1alpha1](#identitymiloapis.comv1alpha1)

## identity.miloapis.com/v1alpha1

Package v1alpha1 contains API types for identity-related resources.

### Resource Types

- [UserIdentity](#useridentity)
- [Session](#session)
- [MachineAccountKey](#machineaccountkey)
- [MachineAccount](#machineaccount)

---

### UserIdentity

UserIdentity represents a user's linked identity within an external identity provider.

This resource describes the connection between a Milo user and their account in an external authentication provider (e.g., GitHub, Google, Microsoft). It is NOT the identity provider itself, but rather the user's specific identity within that provider.

**Use cases:**

- Display all authentication methods linked to a user account in the UI
- Show which external accounts a user has connected
- Provide visibility into federated identity mappings

**Important notes:**

- This is a read-only resource for display purposes only
- Identity management (linking/unlinking providers) is handled by the external authentication provider (e.g., Zitadel), not through this API
- No sensitive credentials or tokens are exposed through this resource

#### UserIdentityStatus

| Field | Type | Description |
| :--- | :--- | :--- |
| `userUID` | string | The unique identifier of the Milo user who owns this identity. |
| `providerID` | string | The unique identifier of the external identity provider instance. This is typically an internal ID from the authentication system. |
| `providerName` | string | The human-readable name of the identity provider. Examples: "GitHub", "Google", "Microsoft", "GitLab" |
| `username` | string | The user's username or identifier within the external identity provider. This is the name the user is known by in the external system (e.g., GitHub username). |

---

### Session

Session represents an active user session in the system.

This resource provides information about user authentication sessions, including the provider used for authentication and session metadata.

**Use cases:**

- Display active sessions for a user
- Monitor session activity
- Provide session management capabilities in the UI

**Important notes:**

- This is a read-only resource
- Session lifecycle is managed by the authentication provider
- No sensitive session tokens are exposed

#### SessionStatus

| Field | Type | Description |
| :--- | :--- | :--- |
| `userUID` | string | The unique identifier of the user who owns this session. |
| `provider` | string | The authentication provider used for this session. |
| `ip` | string | The IP address from which the session was created (optional). |
| `fingerprintID` | string | A fingerprint identifier for the session (optional). |
| `createdAt` | metav1.Time | The timestamp when the session was created. |
| `expiresAt` | *metav1.Time | The timestamp when the session expires (optional). |

---

### MachineAccountKey

MachineAccountKey represents a credential for a MachineAccount.

This resource allows users to manage API keys for machine-to-machine authentication. When a MachineAccountKey is created, the system generates a private key that is returned in the status only once.

**Use cases:**

- Authenticating external services and automation scripts
- Managing key rotation and expiration
- Auditing machine account activity

**Important notes:**

- The `privateKey` is ONLY available in the creation response and is NEVER persisted in the Milo API server.
- Keys can have an optional expiration date.
- Each key is associated with a specific `MachineAccount` identified by its email.

#### MachineAccountKeySpec

| Field | Type | Description |
| :--- | :--- | :--- |
| `machineAccountUserName` | string | The email address of the MachineAccount that owns this key. |
| `expirationDate` | metav1.Time | Optional date and time when the key will expire. |
| `publicKey` | string | Optional public key to be registered. If not provided, one will be auto-generated. |

#### MachineAccountKeyStatus

| Field | Type | Description |
| :--- | :--- | :--- |
| `authProviderKeyID` | string | Unique identifier for the key in the authentication provider (e.g. Zitadel ID). |
| `privateKey` | string | PEM-encoded RSA private key. Only present in the response of a creation event. |
| `conditions` | []metav1.Condition | Standard Kubernetes conditions for resource status. |

---

## MachineAccount
<sup><sup>[↩ Parent](#identitymiloapis.comv1alpha1)</sup></sup>

MachineAccount is the Schema for the machine accounts API

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
      <td><b>apiVersion</b></td>
      <td>string</td>
      <td>identity.miloapis.com/v1alpha1</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b>kind</b></td>
      <td>string</td>
      <td>MachineAccount</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectmeta-v1-meta">metadata</a></b></td>
      <td>object</td>
      <td>Refer to the Kubernetes API documentation for the fields of the `metadata` field.</td>
      <td>true</td>
      </tr><tr>
        <td><b><a href="#machineaccountspec">spec</a></b></td>
        <td>object</td>
        <td>
          MachineAccountSpec defines the desired state of MachineAccount<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#machineaccountstatus">status</a></b></td>
        <td>object</td>
        <td>
          MachineAccountStatus defines the observed state of MachineAccount<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### MachineAccount.spec
<sup><sup>[↩ Parent](#machineaccount)</sup></sup>

MachineAccountSpec defines the desired state of MachineAccount

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>state</b></td>
        <td>enum</td>
        <td>
          The state of the machine account. This state can be safely changed as needed.
States:
  - Active: The machine account can be used to authenticate.
  - Inactive: The machine account is prohibited to be used to authenticate, and revokes all existing sessions.<br/>
          <br/>
            <i>Enum</i>: Active, Inactive<br/>
            <i>Default</i>: Active<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### MachineAccount.status
<sup><sup>[↩ Parent](#machineaccount)</sup></sup>

MachineAccountStatus defines the observed state of MachineAccount

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#machineaccountstatusconditionsindex">conditions</a></b></td>
        <td>[]object</td>
        <td>
          Conditions provide conditions that represent the current status of the MachineAccount.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>email</b></td>
        <td>string</td>
        <td>
          The computed email of the machine account following the pattern:
{metadata.name}@{project-name}.{email-address-suffix}<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>state</b></td>
        <td>enum</td>
        <td>
          State represents the current activation state of the machine account from the auth provider.
This field tracks the state from the previous generation and is updated when state changes
are successfully propagated to the auth provider. It helps optimize performance by only
updating the auth provider when a state change is detected.<br/>
          <br/>
            <i>Enum</i>: Active, Inactive<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### MachineAccount.status.conditions[index]
<sup><sup>[↩ Parent](#machineaccountstatus)</sup></sup>

Condition contains details for one aspect of the current state of this API Resource.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>lastTransitionTime</b></td>
        <td>string</td>
        <td>
          lastTransitionTime is the last time the condition transitioned from one status to another.
This should be when the underlying condition changed.  If that is not known, then using the time when the API field changed is acceptable.<br/>
          <br/>
            <i>Format</i>: date-time<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>message</b></td>
        <td>string</td>
        <td>
          message is a human readable message indicating details about the transition.
This may be an empty string.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>reason</b></td>
        <td>string</td>
        <td>
          reason contains a programmatic identifier indicating the reason for the condition's last transition.
Producers of specific condition types may define expected values and meanings for this field,
and whether the values are considered a guaranteed API.
The value should be a CamelCase string.
This field may not be empty.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>status</b></td>
        <td>enum</td>
        <td>
          status of the condition, one of True, False, Unknown.<br/>
          <br/>
            <i>Enum</i>: True, False, Unknown<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>type</b></td>
        <td>string</td>
        <td>
          type of condition in CamelCase or in foo.example.com/CamelCase.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>observedGeneration</b></td>
        <td>integer</td>
        <td>
          observedGeneration represents the .metadata.generation that the condition was set based upon.
For instance, if .metadata.generation is currently 12, but the .status.conditions[x].observedGeneration is 9, the condition is out of date
with respect to the current state of the instance.<br/>
          <br/>
            <i>Format</i>: int64<br/>
            <i>Minimum</i>: 0<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>
