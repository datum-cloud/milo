# API Reference

Packages:

- [resourcemanager.miloapis.com/v1alpha1](#resourcemanagermiloapiscomv1alpha1)

# resourcemanager.miloapis.com/v1alpha1

Resource Types:

- [OrganizationMembership](#organizationmembership)

- [Organization](#organization)

- [Project](#project)




## OrganizationMembership
<sup><sup>[↩ Parent](#resourcemanagermiloapiscomv1alpha1 )</sup></sup>






OrganizationMembership is the Schema for the organizationmemberships API

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
      <td>resourcemanager.miloapis.com/v1alpha1</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b>kind</b></td>
      <td>string</td>
      <td>OrganizationMembership</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectmeta-v1-meta">metadata</a></b></td>
      <td>object</td>
      <td>Refer to the Kubernetes API documentation for the fields of the `metadata` field.</td>
      <td>true</td>
      </tr><tr>
        <td><b><a href="#organizationmembershipspec">spec</a></b></td>
        <td>object</td>
        <td>
          OrganizationMembershipSpec defines the desired state of OrganizationMembership<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#organizationmembershipstatus">status</a></b></td>
        <td>object</td>
        <td>
          OrganizationMembershipStatus defines the observed state of OrganizationMembership<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### OrganizationMembership.spec
<sup><sup>[↩ Parent](#organizationmembership)</sup></sup>



OrganizationMembershipSpec defines the desired state of OrganizationMembership

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
        <td><b><a href="#organizationmembershipspecorganizationref">organizationRef</a></b></td>
        <td>object</td>
        <td>
          OrganizationRef is a reference to the Organization that the user is a member of.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#organizationmembershipspecuserref">userRef</a></b></td>
        <td>object</td>
        <td>
          UserRef is a reference to the User that is a member of the Organization.<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### OrganizationMembership.spec.organizationRef
<sup><sup>[↩ Parent](#organizationmembershipspec)</sup></sup>



OrganizationRef is a reference to the Organization that the user is a member of.

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
      </tr></tbody>
</table>


### OrganizationMembership.spec.userRef
<sup><sup>[↩ Parent](#organizationmembershipspec)</sup></sup>



UserRef is a reference to the User that is a member of the Organization.

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
      </tr></tbody>
</table>


### OrganizationMembership.status
<sup><sup>[↩ Parent](#organizationmembership)</sup></sup>



OrganizationMembershipStatus defines the observed state of OrganizationMembership

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
        <td><b><a href="#organizationmembershipstatusconditionsindex">conditions</a></b></td>
        <td>[]object</td>
        <td>
          Conditions provide conditions that represent the current status of the OrganizationMembership.<br/>
          <br/>
            <i>Default</i>: [map[lastTransitionTime:1970-01-01T00:00:00Z message:Waiting for control plane to reconcile reason:Unknown status:Unknown type:Ready]]<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>observedGeneration</b></td>
        <td>integer</td>
        <td>
          ObservedGeneration is the most recent generation observed for this OrganizationMembership by the controller.<br/>
          <br/>
            <i>Format</i>: int64<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#organizationmembershipstatusorganization">organization</a></b></td>
        <td>object</td>
        <td>
          Organization contains information about the organization in the membership.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#organizationmembershipstatususer">user</a></b></td>
        <td>object</td>
        <td>
          User contains information about the user in the membership.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### OrganizationMembership.status.conditions[index]
<sup><sup>[↩ Parent](#organizationmembershipstatus)</sup></sup>



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


### OrganizationMembership.status.organization
<sup><sup>[↩ Parent](#organizationmembershipstatus)</sup></sup>



Organization contains information about the organization in the membership.

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
        <td><b>displayName</b></td>
        <td>string</td>
        <td>
          DisplayName is the display name of the organization in the membership.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>type</b></td>
        <td>string</td>
        <td>
          Type is the type of the organization in the membership.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### OrganizationMembership.status.user
<sup><sup>[↩ Parent](#organizationmembershipstatus)</sup></sup>



User contains information about the user in the membership.

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
          Email is the email of the user in the membership.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>familyName</b></td>
        <td>string</td>
        <td>
          FamilyName is the family name of the user in the membership.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>givenName</b></td>
        <td>string</td>
        <td>
          GivenName is the given name of the user in the membership.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>

## Organization
<sup><sup>[↩ Parent](#resourcemanagermiloapiscomv1alpha1 )</sup></sup>






Use lowercase for path, which influences plural name. Ensure kind is Organization.
Organization is the Schema for the Organizations API

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
      <td>resourcemanager.miloapis.com/v1alpha1</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b>kind</b></td>
      <td>string</td>
      <td>Organization</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectmeta-v1-meta">metadata</a></b></td>
      <td>object</td>
      <td>Refer to the Kubernetes API documentation for the fields of the `metadata` field.</td>
      <td>true</td>
      </tr><tr>
        <td><b><a href="#organizationspec">spec</a></b></td>
        <td>object</td>
        <td>
          OrganizationSpec defines the desired state of Organization<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#organizationstatus">status</a></b></td>
        <td>object</td>
        <td>
          OrganizationStatus defines the observed state of Organization<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Organization.spec
<sup><sup>[↩ Parent](#organization)</sup></sup>



OrganizationSpec defines the desired state of Organization

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
        <td><b>type</b></td>
        <td>enum</td>
        <td>
          The type of organization.<br/>
          <br/>
            <i>Validations</i>:<li>type(oldSelf) == null_type || self == oldSelf: organization type is immutable</li>
            <i>Enum</i>: Personal, Standard<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Organization.status
<sup><sup>[↩ Parent](#organization)</sup></sup>



OrganizationStatus defines the observed state of Organization

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
        <td><b><a href="#organizationstatusconditionsindex">conditions</a></b></td>
        <td>[]object</td>
        <td>
          Conditions represents the observations of an organization's current state.
Known condition types are: "Ready"<br/>
          <br/>
            <i>Default</i>: [map[lastTransitionTime:1970-01-01T00:00:00Z message:Waiting for control plane to reconcile reason:Unknown status:Unknown type:Ready]]<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>observedGeneration</b></td>
        <td>integer</td>
        <td>
          ObservedGeneration is the most recent generation observed for this Organization by the controller.<br/>
          <br/>
            <i>Format</i>: int64<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Organization.status.conditions[index]
<sup><sup>[↩ Parent](#organizationstatus)</sup></sup>



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

## Project
<sup><sup>[↩ Parent](#resourcemanagermiloapiscomv1alpha1 )</sup></sup>






Project is the Schema for the projects API.

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
      <td>resourcemanager.miloapis.com/v1alpha1</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b>kind</b></td>
      <td>string</td>
      <td>Project</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectmeta-v1-meta">metadata</a></b></td>
      <td>object</td>
      <td>Refer to the Kubernetes API documentation for the fields of the `metadata` field.</td>
      <td>true</td>
      </tr><tr>
        <td><b><a href="#projectspec">spec</a></b></td>
        <td>object</td>
        <td>
          ProjectSpec defines the desired state of Project.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#projectstatus">status</a></b></td>
        <td>object</td>
        <td>
          ProjectStatus defines the observed state of Project.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Project.spec
<sup><sup>[↩ Parent](#project)</sup></sup>



ProjectSpec defines the desired state of Project.

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
        <td><b><a href="#projectspecownerref">ownerRef</a></b></td>
        <td>object</td>
        <td>
          OwnerRef is a reference to the owner of the project. Must be a valid
resource.<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Project.spec.ownerRef
<sup><sup>[↩ Parent](#projectspec)</sup></sup>



OwnerRef is a reference to the owner of the project. Must be a valid
resource.

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
          Kind is the kind of the resource.<br/>
          <br/>
            <i>Enum</i>: Organization<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name is the name of the resource.<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Project.status
<sup><sup>[↩ Parent](#project)</sup></sup>



ProjectStatus defines the observed state of Project.

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
        <td><b><a href="#projectstatusconditionsindex">conditions</a></b></td>
        <td>[]object</td>
        <td>
          Represents the observations of a project's current state.
Known condition types are: "Ready"<br/>
          <br/>
            <i>Default</i>: [map[lastTransitionTime:1970-01-01T00:00:00Z message:Waiting for control plane to reconcile reason:Unknown status:Unknown type:Ready]]<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Project.status.conditions[index]
<sup><sup>[↩ Parent](#projectstatus)</sup></sup>



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
