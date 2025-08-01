# API Reference

Packages:

- [iam.miloapis.com/v1alpha1](#iamdatumapiscomv1alpha1)

# iam.miloapis.com/v1alpha1

Resource Types:

- [UserInvitation](#userinvitation)




## UserInvitation
<sup><sup>[↩ Parent](#iamdatumapiscomv1alpha1 )</sup></sup>






UserInvitation is the Schema for the userinvitations API

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
      <td>UserInvitation</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectmeta-v1-meta">metadata</a></b></td>
      <td>object</td>
      <td>Refer to the Kubernetes API documentation for the fields of the `metadata` field.</td>
      <td>true</td>
      </tr><tr>
        <td><b><a href="#userinvitationspec">spec</a></b></td>
        <td>object</td>
        <td>
          UserInvitationSpec defines the desired state of UserInvitation<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#userinvitationstatus">status</a></b></td>
        <td>object</td>
        <td>
          UserInvitationStatus defines the observed state of UserInvitation<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### UserInvitation.spec
<sup><sup>[↩ Parent](#userinvitation)</sup></sup>



UserInvitationSpec defines the desired state of UserInvitation

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
        <td><b>email</b></td>
        <td>string</td>
        <td>
          The email of the user being invited.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>familyName</b></td>
        <td>string</td>
        <td>
          The last name of the user being invited.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>givenName</b></td>
        <td>string</td>
        <td>
          The first name of the user being invited.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#userinvitationspecrolesindex">roles</a></b></td>
        <td>[]object</td>
        <td>
          The roles that will be assigned to the user when they accept the invitation.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### UserInvitation.spec.roles[index]
<sup><sup>[↩ Parent](#userinvitationspec)</sup></sup>



RoleReference contains information that points to the Role being used

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
          Name is the name of resource being referenced<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          Namespace of the referenced Role. If empty, it is assumed to be in the PolicyBinding's namespace.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### UserInvitation.status
<sup><sup>[↩ Parent](#userinvitation)</sup></sup>



UserInvitationStatus defines the observed state of UserInvitation

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
        <td><b><a href="#userinvitationstatusconditionsindex">conditions</a></b></td>
        <td>[]object</td>
        <td>
          Conditions provide conditions that represent the current status of the UserInvitation.<br/>
          <br/>
            <i>Default</i>: [map[lastTransitionTime:1970-01-01T00:00:00Z message:Waiting for control plane to reconcile reason:Unknown status:Unknown type:Ready]]<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### UserInvitation.status.conditions[index]
<sup><sup>[↩ Parent](#userinvitationstatus)</sup></sup>



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
