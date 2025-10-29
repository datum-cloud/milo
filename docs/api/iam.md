# API Reference

Packages:

- [iam.miloapis.com/v1alpha1](#iammiloapiscomv1alpha1)

# iam.miloapis.com/v1alpha1

Resource Types:

- [GroupMembership](#groupmembership)

- [Group](#group)

- [MachineAccountKey](#machineaccountkey)

- [MachineAccount](#machineaccount)

- [PolicyBinding](#policybinding)

- [ProtectedResource](#protectedresource)

- [Role](#role)

- [UserDeactivation](#userdeactivation)

- [UserInvitation](#userinvitation)

- [UserPreference](#userpreference)

- [User](#user)




## GroupMembership
<sup><sup>[↩ Parent](#iammiloapiscomv1alpha1 )</sup></sup>

[... the rest of the document remains unchanged ...]

## UserPreference
<sup><sup>[↩ Parent](#iammiloapiscomv1alpha1 )</sup></sup>






UserPreference is the Schema for the userpreferences API

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
      <td>UserPreference</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectmeta-v1-meta">metadata</a></b></td>
      <td>object</td>
      <td>Refer to the Kubernetes API documentation for the fields of the `metadata` field.</td>
      <td>true</td>
      </tr><tr>
        <td><b><a href="#userpreferencespec">spec</a></b></td>
        <td>object</td>
        <td>
          UserPreferenceSpec defines the desired state of UserPreference<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#userpreferencestatus">status</a></b></td>
        <td>object</td>
        <td>
          UserPreferenceStatus defines the observed state of UserPreference<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### UserPreference.spec
<sup><sup>[↩ Parent](#userpreference)</sup></sup>



UserPreferenceSpec defines the desired state of UserPreference

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
        <td><b><a href="#userpreferencespecuserref">userRef</a></b></td>
        <td>object</td>
        <td>
          Reference to the user these preferences belong to.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>theme</b></td>
        <td>enum</td>
        <td>
          The user's theme preference.<br/>
          <br/>
            <i>Enum</i>: light, dark, system<br/>
            <i>Default</i>: system<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>displayName</b></td>
        <td>string</td>
        <td>
          DisplayName is the user's preferred display name.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>title</b></td>
        <td>string</td>
        <td>
          Title is the user's title or role.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### UserPreference.spec.userRef
<sup><sup>[↩ Parent](#userpreferencespec)</sup></sup>



Reference to the user these preferences belong to.

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
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name is the name of the User being referenced.<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### UserPreference.status
<sup><sup>[↩ Parent](#userpreference)</sup></sup>



UserPreferenceStatus defines the observed state of UserPreference

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
        <td><b><a href="#userpreferencestatusconditionsindex">conditions</a></b></td>
        <td>[]object</td>
        <td>
          Conditions provide conditions that represent the current status of the UserPreference.<br/>
          <br/>
            <i>Default</i>: [map[lastTransitionTime:1970-01-01T00:00:00Z message:Waiting for control plane to reconcile reason:Unknown status:Unknown type:Ready]]<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### UserPreference.status.conditions[index]
<sup><sup>[↩ Parent](#userpreferencestatus)</sup></sup>



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

## User
<sup><sup>[↩ Parent](#iammiloapiscomv1alpha1 )</sup></sup>

[... the rest of the document remains unchanged ...]

