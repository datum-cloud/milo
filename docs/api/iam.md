# API Reference

Packages:

- [iam.miloapis.com/v1alpha1](#iammiloapiscomv1alpha1)
- [identity.miloapis.com/v1alpha1](#identitymiloapiscomv1alpha1)

# iam.miloapis.com/v1alpha1

Resource Types:

- [GroupMembership](#groupmembership)
- [Group](#group)
- [MachineAccount](#machineaccount)
- [PlatformAccessApproval](#platformaccessapproval)
- [PlatformAccessDenial](#platformaccessdenial)
- [PlatformAccessRejection](#platformaccessrejection)
- [PlatformInvitation](#platforminvitation)
- [PolicyBinding](#policybinding)
- [ProtectedResource](#protectedresource)
- [Role](#role)
- [UserDeactivation](#userdeactivation)
- [UserInvitation](#userinvitation)
- [UserPreference](#userpreference)
- [User](#user)

(...unchanged documentation for these resources...)

# identity.miloapis.com/v1alpha1

Resource Types:

- [MachineAccountKey](#identity-machineaccountkey)

## MachineAccountKey
<sup><sup>[↩ Parent](#identitymiloapiscomv1alpha1 )</sup></sup>

MachineAccountKey is the Schema for the machineaccountkeys API

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
      <td>MachineAccountKey</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectmeta-v1-meta">metadata</a></b></td>
      <td>object</td>
      <td>Refer to the Kubernetes API documentation for the fields of the `metadata` field.</td>
      <td>true</td>
      </tr><tr>
        <td><b><a href="#machineaccountkeyspec">spec</a></b></td>
        <td>object</td>
        <td>
          MachineAccountKeySpec defines the desired state of MachineAccountKey<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#machineaccountkeystatus">status</a></b></td>
        <td>object</td>
        <td>
          MachineAccountKeyStatus defines the observed state of MachineAccountKey<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>

### MachineAccountKey.spec
<sup><sup>[↩ Parent](#machineaccountkey)</sup></sup>

MachineAccountKeySpec defines the desired state of MachineAccountKey

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
        <td><b>machineAccountName</b></td>
        <td>string</td>
        <td>
          MachineAccountName is the name of the MachineAccount that owns this key.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>expirationDate</b></td>
        <td>string</td>
        <td>
          ExpirationDate is the date and time when the MachineAccountKey will expire.
If not specified, the MachineAccountKey will never expire.<br/>
          <br/>
            <i>Format</i>: date-time<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>publicKey</b></td>
        <td>string</td>
        <td>
          PublicKey is the public key of the MachineAccountKey.
If not specified, the MachineAccountKey will be created with an auto-generated public key.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>

### MachineAccountKey.status
<sup><sup>[↩ Parent](#machineaccountkey)</sup></sup>

MachineAccountKeyStatus defines the observed state of MachineAccountKey

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
        <td><b>authProviderKeyID</b></td>
        <td>string</td>
        <td>
          AuthProviderKeyID is the unique identifier for the key in the auth provider.
This field is populated by the controller after the key is created in the auth provider.
For example, when using Zitadel, a typical value might be: "326102453042806786"<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>privateKey</b></td>
        <td>string</td>
        <td>
          PrivateKey contains the PEM-encoded RSA private key generated during resource creation. This field is populated only in the creation response and is never persisted to etcd. Any value present on a GET or LIST response indicates a bug in the server implementation.

Note: private key material will appear in API server audit logs for creation events. This matches the behavior of similar systems (GCP service account keys).
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#machineaccountkeystatusconditionsindex">conditions</a></b></td>
        <td>[]object</td>
        <td>
          Conditions provide conditions that represent the current status of the MachineAccountKey.<br/>
          <br/>
            <i>Default</i>: [map[lastTransitionTime:1970-01-01T00:00:00Z message:Waiting for control plane to reconcile reason:Unknown status:Unknown type:Ready]]<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>

### MachineAccountKey.status.conditions[index]
<sup><sup>[↩ Parent](#machineaccountkeystatus)</sup></sup>

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

## MachineAccount
<sup><sup>[↩ Parent](#iammiloapiscomv1alpha1 )</sup></sup>

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
      <td>iam.miloapis.com/v1alpha1</td>
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

(...rest of resource docs unchanged as not affected by the code changes...)