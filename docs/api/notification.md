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
            <i>Validations</i>:<li>self == oldSelf: spec is immutable</li><li>If the referenced Contact's subjectRef.apiGroup is <code>resourcemanager.miloapis.com</code>, the ContactGroupMembershipRemoval namespace must match the Contact's namespace.</li><li>The APIGroup in subjectRef must be <code>resourcemanager.miloapis.com</code>; other API groups are not supported and will be rejected.</li>
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
          <b>Constraint:</b> If the referenced Contact's <code>subjectRef.apiGroup</code> is <code>resourcemanager.miloapis.com</code>, the ContactGroupMembershipRemoval resource namespace <b>must</b> match the Contact's namespace. Only <code>resourcemanager.miloapis.com</code> API group is currently supported; other API groups will be rejected.
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

[...unchanged]

## ContactGroupMembership
<sup><sup>[↩ Parent](#notificationmiloapiscomv1alpha1 )</sup></sup>

ContactGroupMembership is the Schema for the contactgroupmemberships API.
It represents a membership of a Contact in a ContactGroup.

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
      <td>ContactGroupMembership</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectmeta-v1-meta">metadata</a></b></td>
      <td>object</td>
      <td>Refer to the Kubernetes API documentation for the fields of the `metadata` field.</td>
      <td>true</td>
      </tr><tr>
        <td><b><a href="#contactgroupmembershipspec">spec</a></b></td>
        <td>object</td>
        <td>
          ContactGroupMembershipSpec defines the desired state of ContactGroupMembership.<br/>
          <br/>
            <i>Validations</i>:<li>self == oldSelf: spec is immutable</li><li>If the referenced Contact's subjectRef.apiGroup is <code>resourcemanager.miloapis.com</code>, the ContactGroupMembership namespace must match the Contact's namespace.</li><li>The APIGroup in subjectRef must be <code>resourcemanager.miloapis.com</code>; other API groups are not supported and will be rejected.</li>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#contactgroupmembershipstatus">status</a></b></td>
        <td>object</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### ContactGroupMembership.spec
<sup><sup>[↩ Parent](#contactgroupmembership)</sup></sup>

ContactGroupMembershipSpec defines the desired state of ContactGroupMembership.

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
        <td><b><a href="#contactgroupmembershipspeccontactgroupref">contactGroupRef</a></b></td>
        <td>object</td>
        <td>
          ContactGroupRef is a reference to the ContactGroup that the Contact is a member of.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#contactgroupmembershipspeccontactref">contactRef</a></b></td>
        <td>object</td>
        <td>
          ContactRef is a reference to the Contact that is a member of the ContactGroup.<br/>
          <b>Constraint:</b> If the referenced Contact's <code>subjectRef.apiGroup</code> is <code>resourcemanager.miloapis.com</code>, the ContactGroupMembership resource namespace <b>must</b> match the Contact's namespace. Only <code>resourcemanager.miloapis.com</code> API group is currently supported; other API groups will be rejected.
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### ContactGroupMembership.spec.contactGroupRef
<sup><sup>[↩ Parent](#contactgroupmembershipspec)</sup></sup>

[...unchanged]

### ContactGroupMembership.spec.contactRef
<sup><sup>[↩ Parent](#contactgroupmembershipspec)</sup></sup>

[...unchanged]

### ContactGroupMembership.status
<sup><sup>[↩ Parent](#contactgroupmembership)</sup></sup>

[...unchanged]

### ContactGroupMembership.status.conditions[index]
<sup><sup>[↩ Parent](#contactgroupmembershipstatus)</sup></sup>

[...unchanged]

## ContactGroup
<sup><sup>[↩ Parent](#notificationmiloapiscomv1alpha1 )</sup></sup>

[...unchanged]

### ContactGroup.spec
<sup><sup>[↩ Parent](#contactgroup)</sup></sup>

[...unchanged]

### ContactGroup.status
<sup><sup>[↩ Parent](#contactgroup)</sup></sup>

[...unchanged]

### ContactGroup.status.conditions[index]
<sup><sup>[↩ Parent](#contactgroupstatus)</sup></sup>

[...unchanged]

## Contact
<sup><sup>[↩ Parent](#notificationmiloapiscomv1alpha1 )</sup></sup>

[...unchanged]

### Contact.spec
<sup><sup>[↩ Parent](#contact)</sup></sup>

[...unchanged]

### Contact.spec.subject
<sup><sup>[↩ Parent](#contactspec)</sup></sup>

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
            <i>Enum</i>: iam.miloapis.com, resourcemanager.miloapis.com<br/>
            <b>Note:</b> Only <code>resourcemanager.miloapis.com</code> is supported for membership operations; other groups will be rejected if used as a subjectRef in ContactGroupMembership/Removal resources.
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>kind</b></td>
        <td>enum</td>
        <td>
          Kind is the type of resource being referenced.<br/>
          <br/>
            <i>Enum</i>: User, Project<br/>
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

[...unchanged]

### Contact.status.conditions[index]
<sup><sup>[↩ Parent](#contactstatus)</sup></sup>

[...unchanged]

## EmailBroadcast
<sup><sup>[↩ Parent](#notificationmiloapiscomv1alpha1 )</sup></sup>

[...unchanged]

### EmailBroadcast.spec
<sup><sup>[↩ Parent](#emailbroadcast)</sup></sup>

[...unchanged]

### EmailBroadcast.spec.contactGroupRef
<sup><sup>[↩ Parent](#emailbroadcastspec)</sup></sup>

[...unchanged]

### EmailBroadcast.spec.templateRef
<sup><sup>[↩ Parent](#emailbroadcastspec)</sup></sup>

[...unchanged]

### EmailBroadcast.status
<sup><sup>[↩ Parent](#emailbroadcast)</sup></sup>

[...unchanged]

### EmailBroadcast.status.conditions[index]
<sup><sup>[↩ Parent](#emailbroadcaststatus)</sup></sup>

[...unchanged]

## Email
<sup><sup>[↩ Parent](#notificationmiloapiscomv1alpha1 )</sup></sup>

[...unchanged]

### Email.spec
<sup><sup>[↩ Parent](#email)</sup></sup>

[...unchanged]

### Email.spec.recipient
<sup><sup>[↩ Parent](#emailspec)</sup></sup>

[...unchanged]

### Email.spec.recipient.userRef
<sup><sup>[↩ Parent](#emailspecrecipient)</sup></sup>

[...unchanged]

### Email.spec.templateRef
<sup><sup>[↩ Parent](#emailspec)</sup></sup>

[...unchanged]

### Email.spec.variables[index]
<sup><sup>[↩ Parent](#emailspec)</sup></sup>

[...unchanged]

### Email.status
<sup><sup>[↩ Parent](#email)</sup></sup>

[...unchanged]

### Email.status.conditions[index]
<sup><sup>[↩ Parent](#emailstatus)</sup></sup>

[...unchanged]

## EmailTemplate
<sup><sup>[↩ Parent](#notificationmiloapiscomv1alpha1 )</sup></sup>

[...unchanged]

### EmailTemplate.spec
<sup><sup>[↩ Parent](#emailtemplate)</sup></sup>

[...unchanged]

### EmailTemplate.spec.variables[index]
<sup><sup>[↩ Parent](#emailtemplatespec)</sup></sup>

[...unchanged]

### EmailTemplate.status
<sup><sup>[↩ Parent](#emailtemplate)</sup></sup>

[...unchanged]

### EmailTemplate.status.conditions[index]
<sup><sup>[↩ Parent](#emailtemplatestatus)</sup></sup>

[...unchanged]
