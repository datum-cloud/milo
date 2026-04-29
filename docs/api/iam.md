# API Reference

Packages:

- [iam.miloapis.com/v1alpha1](#iammiloapiscomv1alpha1)

# iam.miloapis.com/v1alpha1

Resource Types:

- [GroupMembership](#groupmembership)

- [Group](#group)

- [ServiceAccount](#serviceaccount)

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




## GroupMembership
<sup><sup>[↩ Parent](#iammiloapiscomv1alpha1 )</sup></sup>

...[unchanged documentation for GroupMembership and Group]...

## ServiceAccount
<sup><sup>[↩ Parent](#iammiloapiscomv1alpha1 )</sup></sup>





ServiceAccount is the Schema for the service accounts API

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
      <td>ServiceAccount</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectmeta-v1-meta">metadata</a></b></td>
      <td>object</td>
      <td>Refer to the Kubernetes API documentation for the fields of the `metadata` field.</td>
      <td>true</td>
      </tr><tr>
        <td><b><a href="#serviceaccountspec">spec</a></b></td>
        <td>object</td>
        <td>
          ServiceAccountSpec defines the desired state of ServiceAccount<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#serviceaccountstatus">status</a></b></td>
        <td>object</td>
        <td>
          ServiceAccountStatus defines the observed state of ServiceAccount<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### ServiceAccount.spec
<sup><sup>[↩ Parent](#serviceaccount)</sup></sup>



ServiceAccountSpec defines the desired state of ServiceAccount

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
          The state of the service account. This state can be safely changed as needed.
States:
  - Active: The service account can be used to authenticate.
  - Inactive: The service account is prohibited to be used to authenticate, and revokes all existing sessions.<br/>
          <br/>
            <i>Enum</i>: Active, Inactive<br/>
            <i>Default</i>: Active<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### ServiceAccount.status
<sup><sup>[↩ Parent](#serviceaccount)</sup></sup>



ServiceAccountStatus defines the observed state of ServiceAccount

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
        <td><b><a href="#serviceaccountstatusconditionsindex">conditions</a></b></td>
        <td>[]object</td>
        <td>
          Conditions provide conditions that represent the current status of the ServiceAccount.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>email</b></td>
        <td>string</td>
        <td>
          The computed email of the service account following the pattern:
{metadata.name}@{metadata.namespace}.{project.metadata.name}.{global-suffix}<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>state</b></td>
        <td>enum</td>
        <td>
          State represents the current activation state of the service account from the auth provider.
This field tracks the state from the previous generation and is updated when state changes
are successfully propagated to the auth provider. It helps optimize performance by only
updating the auth provider when a state change is detected.<br/>
          <br/>
            <i>Enum</i>: Active, Inactive<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### ServiceAccount.status.conditions[index]
<sup><sup>[↩ Parent](#serviceaccountstatus)</sup></sup>



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

...[unchanged documentation for all other resources]...

### PolicyBinding.spec.subjects[index]
<sup><sup>[↩ Parent](#policybindingspec)</sup></sup>

Subject contains a reference to the object or user identities a role binding applies to.
This can be a User, Group, or ServiceAccount.

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
        <td><b>kind</b></td>
        <td>enum</td>
        <td>
          Kind of object being referenced. Values defined in Kind constants.<br/>
          <br/>
            <i>Enum</i>: User, Group, ServiceAccount<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name of the object being referenced. A special group name of
"system:authenticated-users" can be used to refer to all authenticated
users.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          Namespace of the referenced object.
If not specified for a Group, User or ServiceAccount, it is ignored.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>uid</b></td>
        <td>string</td>
        <td>
          UID of the referenced object. Optional for system groups (groups with names starting with "system:").<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>

...[unchanged documentation for all other resources]...