# API Reference

Packages:

- [iam.miloapis.com/v1alpha1](#iammiloapiscomv1alpha1)

# iam.miloapis.com/v1alpha1

Resource Types:

- [GroupMembership](#groupmembership)
- [Group](#group)
- [MachineAccountKey](#machineaccountkey)
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

... (all above unchanged)

## PolicyBinding
<sup><sup>[↩ Parent](#iammiloapiscomv1alpha1 )</sup></sup>

PolicyBinding is the Schema for the policybindings API

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
      <td>PolicyBinding</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectmeta-v1-meta">metadata</a></b></td>
      <td>object</td>
      <td>Refer to the Kubernetes API documentation for the fields of the `metadata` field.</td>
      <td>true</td>
      </tr><tr>
        <td><b><a href="#policybindingspec">spec</a></b></td>
        <td>object</td>
        <td>
          PolicyBindingSpec defines the desired state of PolicyBinding<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#policybindingstatus">status</a></b></td>
        <td>object</td>
        <td>
          PolicyBindingStatus defines the observed state of PolicyBinding<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### PolicyBinding.spec
<sup><sup>[↩ Parent](#policybinding)</sup></sup>


PolicyBindingSpec defines the desired state of PolicyBinding

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
        <td><b><a href="#policybindingspecresourceselector">resourceSelector</a></b></td>
        <td>object</td>
        <td>
          ResourceSelector defines which resources the subjects in the policy binding
should have the role applied to. Options within this struct are mutually
exclusive.<br/>
          <br/>
            <i>Validations</i>:<li>oldSelf == null || self == oldSelf: ResourceSelector is immutable and cannot be changed after creation</li><li>has(self.resourceRef) != has(self.resourceKind): exactly one of resourceRef or resourceKind must be specified, but not both</li>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#policybindingspecroleref">roleRef</a></b></td>
        <td>object</td>
        <td>
          RoleRef is a reference to the Role that is being bound.
This can be a reference to a Role custom resource.<br/>
          <br/>
            <i>Validations</i>:<li>oldSelf == null || self == oldSelf: RoleRef is immutable and cannot be changed after creation</li>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#policybindingspecsubjectsindex">subjects</a></b></td>
        <td>[]object</td>
        <td>
          Subjects holds references to the objects the role applies to.<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


...

### PolicyBinding.spec.subjects[index]
<sup><sup>[↩ Parent](#policybindingspec)</sup></sup>


Subject contains a reference to the object or user identities a role binding applies to.
This can be a User, Group, or MachineAccount.

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
            <i>Enum</i>: User, Group, MachineAccount<br/>
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
          Namespace of the referenced object. Required for MachineAccount subjects.
If not specified for a Group or User, it is ignored.<br/>
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

...(all below unchanged)
