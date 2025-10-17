# API Reference

Packages:

- [notification.miloapis.com/v1alpha1](#notificationmiloapiscomv1alpha1)

# notification.miloapis.com/v1alpha1

Resource Types:

- [ContactGroupMembershipRemoval](#contactgroupmembershipremoval)

- [ContactGroupMembership](#contactgroupmembership)

- [ContactGroup](#contactgroup)

- [Contact](#contact)

- [EmailBroadcast](#emailbroadcast)

- [Email](#email)

- [EmailTemplate](#emailtemplate)




## ContactGroupMembershipRemoval
<sup><sup>[↩ Parent](#notificationmiloapiscomv1alpha1 )</sup></sup>







ContactGroupMembershipRemoval is the Schema for the contactgroupmembershipremovals API.
It represents a removal of a Contact from a ContactGroup, it also prevents the Contact from being added to the ContactGroup.

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
      <td>notification.miloapis.com/v1alpha1</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b>kind</b></td>
      <td>string</td>
      <td>ContactGroupMembershipRemoval</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectmeta-v1-meta">metadata</a></b></td>
      <td>object</td>
      <td>Refer to the Kubernetes API documentation for the fields of the `metadata` field.</td>
      <td>true</td>
      </tr><tr>
        <td><b><a href="#contactgroupmembershipremovalspec">spec</a></b></td>
        <td>object</td>
        <td>
          <br/>
          <br/>
            <i>Validations</i>:<li>self == oldSelf: spec is immutable</li>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#contactgroupmembershipremovalstatus">status</a></b></td>
        <td>object</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### ContactGroupMembershipRemoval.spec
<sup><sup>[↩ Parent](#contactgroupmembershipremoval)</sup></sup>




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
        <td><b><a href="#contactgroupmembershipremovalspeccontactgroupref">contactGroupRef</a></b></td>
        <td>object</td>
        <td>
          ContactGroupRef is a reference to the ContactGroup that the Contact does not want to be a member of.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#contactgroupmembershipremovalspeccontactref">contactRef</a></b></td>
        <td>object</td>
        <td>
          ContactRef is a reference to the Contact that prevents the Contact from being part of the ContactGroup.<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### ContactGroupMembershipRemoval.spec.contactGroupRef
<sup><sup>[↩ Parent](#contactgroupmembershipremovalspec)</sup></sup>


ContactGroupRef is a reference to the ContactGroup that the Contact does not want to be a member of.

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
          Name is the name of the ContactGroup being referenced.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          Namespace is the namespace of the ContactGroup being referenced.<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### ContactGroupMembershipRemoval.spec.contactRef
<sup><sup>[↩ Parent](#contactgroupmembershipremovalspec)</sup></sup>


ContactRef is a reference to the Contact that prevents the Contact from being part of the ContactGroup.

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
          Name is the name of the Contact being referenced.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          Namespace is the namespace of the Contact being referenced.<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### ContactGroupMembershipRemoval.status
<sup><sup>[↩ Parent](#contactgroupmembershipremoval)</sup></sup>




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
        <td><b><a href="#contactgroupmembershipremovalstatusconditionsindex">conditions</a></b></td>
        <td>[]object</td>
        <td>
          Conditions represent the latest available observations of an object's current state.
Standard condition is "Ready" which tracks contact group membership removal creation status.<br/>
          <br/>
            <i>Default</i>: [map[lastTransitionTime:1970-01-01T00:00:00Z message:Waiting for contact group membership removal to be created reason:CreatePending status:Unknown type:Ready]]<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### ContactGroupMembershipRemoval.status.conditions[index]
<sup><sup>[↩ Parent](#contactgroupmembershipremovalstatus)</sup></sup>


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

## ContactGroupMembership
<sup><sup>[↩ Parent](#notificationmiloapiscomv1alpha1 )</sup></sup>

[...document content above unchanged...]

## Contact
<sup><sup>[↩ Parent](#notificationmiloapiscomv1alpha1 )</sup></sup>





Contact is the Schema for the contacts API.
It represents a contact for a user.

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
      <td>notification.miloapis.com/v1alpha1</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b>kind</b></td>
      <td>string</td>
      <td>Contact</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectmeta-v1-meta">metadata</a></b></td>
      <td>object</td>
      <td>Refer to the Kubernetes API documentation for the fields of the `metadata` field.</td>
      <td>true</td>
      </tr><tr>
        <td><b><a href="#contactspec">spec</a></b></td>
        <td>object</td>
        <td>
          ContactSpec defines the desired state of Contact.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#contactstatus">status</a></b></td>
        <td>object</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Contact.spec
<sup><sup>[↩ Parent](#contact)</sup></sup>


ContactSpec defines the desired state of Contact.

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
          <br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>familyName</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>givenName</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#contactspecsubject">subject</a></b></td>
        <td>object</td>
        <td>
          Subject is a reference to the subject of the contact.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Contact.spec.subject
<sup><sup>[↩ Parent](#contactspec)</sup></sup>


Subject is a reference to the subject of the contact.

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
        <td><b>apiGroup</b></td>
        <td>enum</td>
        <td>
          APIGroup is the group for the resource being referenced.<br/>
          <br/>
            <i>Enum</i>: iam.miloapis.com<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>kind</b></td>
        <td>enum</td>
        <td>
          Kind is the type of resource being referenced.<br/>
          <br/>
            <i>Enum</i>: User<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name is the name of resource being referenced.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          Namespace is the namespace of resource being referenced.
Required for namespace-scoped resources. Omitted for cluster-scoped resources.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Contact.status
<sup><sup>[↩ Parent](#contact)</sup></sup>

[...remaining document content unchanged...]
