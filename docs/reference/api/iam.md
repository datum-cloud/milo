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

- [UserInvitation](#userinvitation)

- [User](#user)




## GroupMembership
<sup><sup>[↩ Parent](#iammiloapiscomv1alpha1 )</sup></sup>






GroupMembership establishes a relationship between a User and a Group in the Milo IAM system.
This resource is the primary mechanism for adding users to groups, enabling organized
permission management through group-based role assignments.

GroupMembership resources are namespaced and should typically be created in the same
namespace as the target group. Each GroupMembership represents a single user-to-group
relationship - to add multiple users to a group, create multiple GroupMembership resources.

Key characteristics:
- Namespaced: Created in the same namespace as the target group
- One-to-one relationship: Each resource links exactly one user to one group
- Cross-namespace references: Can reference cluster-scoped users from any namespace
- Bidirectional effect: Affects both user's group memberships and group's member list

Common usage patterns:
- Team onboarding: Add new team members to appropriate groups
- Role changes: Move users between groups as their responsibilities change
- Project assignments: Add users to project-specific groups
- Temporary access: Grant temporary group membership for specific tasks

Best practices:
- Use descriptive names that indicate the user-group relationship
- Create memberships in the same namespace as the target group
- Monitor membership status through conditions before relying on permissions
- Use groups rather than direct user-role bindings for scalability

Example:

	apiVersion: iam.miloapis.com/v1alpha1
	kind: GroupMembership
	metadata:
	  name: jane-doe-developers
	  namespace: project-alpha
	spec:
	  userRef:
	    name: jane-doe
	  groupRef:
	    name: developers
	    namespace: project-alpha

Related resources:
- User: The cluster-scoped user being added to the group
- Group: The namespaced group that will contain the user
- PolicyBinding: Can reference the group to grant roles to all members

GroupMembership is the Schema for the groupmemberships API

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
      <td>GroupMembership</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectmeta-v1-meta">metadata</a></b></td>
      <td>object</td>
      <td>Refer to the Kubernetes API documentation for the fields of the `metadata` field.</td>
      <td>true</td>
      </tr><tr>
        <td><b><a href="#groupmembershipspec">spec</a></b></td>
        <td>object</td>
        <td>
          GroupMembershipSpec defines the desired state of GroupMembership, establishing
the relationship between a specific user and a group within the IAM system.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#groupmembershipstatus">status</a></b></td>
        <td>object</td>
        <td>
          GroupMembershipStatus defines the observed state of GroupMembership, indicating
whether the user has been successfully added to the group.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### GroupMembership.spec
<sup><sup>[↩ Parent](#groupmembership)</sup></sup>



GroupMembershipSpec defines the desired state of GroupMembership, establishing
the relationship between a specific user and a group within the IAM system.

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
        <td><b><a href="#groupmembershipspecgroupref">groupRef</a></b></td>
        <td>object</td>
        <td>
          GroupRef is a reference to the Group that the user should be added to.
Groups are namespaced resources, so both name and namespace are required.
The referenced group must exist in the specified namespace before the
GroupMembership can be successfully reconciled.

Example: { name: "developers", namespace: "project-alpha" }<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#groupmembershipspecuserref">userRef</a></b></td>
        <td>object</td>
        <td>
          UserRef is a reference to the User that should be a member of the specified Group.
Users are cluster-scoped resources, so only the name is required for identification.
The referenced user must exist in the cluster before the GroupMembership can be
successfully reconciled.

Example: { name: "jane-doe" }<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### GroupMembership.spec.groupRef
<sup><sup>[↩ Parent](#groupmembershipspec)</sup></sup>



GroupRef is a reference to the Group that the user should be added to.
Groups are namespaced resources, so both name and namespace are required.
The referenced group must exist in the specified namespace before the
GroupMembership can be successfully reconciled.

Example: { name: "developers", namespace: "project-alpha" }

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
          Name is the name of the Group being referenced. This must match the metadata.name
of an existing Group resource in the specified namespace.

Example: "developers"<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          Namespace is the namespace where the referenced Group exists. This must match
the metadata.namespace of an existing Group resource.

Example: "project-alpha"<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### GroupMembership.spec.userRef
<sup><sup>[↩ Parent](#groupmembershipspec)</sup></sup>



UserRef is a reference to the User that should be a member of the specified Group.
Users are cluster-scoped resources, so only the name is required for identification.
The referenced user must exist in the cluster before the GroupMembership can be
successfully reconciled.

Example: { name: "jane-doe" }

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
          Name is the name of the User being referenced. This must match the metadata.name
of an existing User resource in the cluster.

Example: "jane-doe"<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### GroupMembership.status
<sup><sup>[↩ Parent](#groupmembership)</sup></sup>



GroupMembershipStatus defines the observed state of GroupMembership, indicating
whether the user has been successfully added to the group.

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
        <td><b><a href="#groupmembershipstatusconditionsindex">conditions</a></b></td>
        <td>[]object</td>
        <td>
          Conditions represent the latest available observations of the GroupMembership's current state.
The primary condition type is "Ready" which indicates whether the user has been
successfully added to the group and the membership is active.

Common condition types:
- Ready: Indicates the user is successfully a member of the group
- UserFound: Indicates the referenced user exists
- GroupFound: Indicates the referenced group exists

Example condition:
  - type: Ready
    status: "True"
    reason: MembershipActive
    message: User successfully added to group<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### GroupMembership.status.conditions[index]
<sup><sup>[↩ Parent](#groupmembershipstatus)</sup></sup>



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

## Group
<sup><sup>[↩ Parent](#iammiloapiscomv1alpha1 )</sup></sup>






Group represents a collection of users for simplified permission management in the Milo IAM system.
Groups are namespaced resources that serve as containers for organizing users with similar access needs.

Groups themselves have no configuration options - they exist purely as organizational units.
Users are added to groups through GroupMembership resources, which create the actual relationship
between users and groups. Groups cannot be nested within other groups in the current implementation,
though this may be supported in future versions.

Key characteristics:
- Namespaced: Groups exist within a specific namespace/project context
- User organization: Primary purpose is to organize users for easier permission management
- No direct configuration: Groups have no spec fields, only metadata and status
- PolicyBinding target: Groups can be referenced in PolicyBindings to grant roles to all members

Common usage patterns:
- Team organization (e.g., "developers", "qa-team", "project-managers")
- Role-based groupings (e.g., "admins", "viewers", "editors")
- Department-based access (e.g., "engineering", "marketing", "finance")
- Project-specific teams (e.g., "project-alpha-team", "infrastructure-team")

Best practices:
- Use descriptive names that clearly indicate the group's purpose
- Organize groups by function or team rather than individual permissions
- Bind roles to groups rather than individual users for easier management
- Use groups consistently across projects for similar roles

Example:

	apiVersion: iam.miloapis.com/v1alpha1
	kind: Group
	metadata:
	  name: developers
	  namespace: project-alpha
	  annotations:
	    description: "Developers working on project alpha with read/write access"

Related resources:
- GroupMembership: Links users to this group
- PolicyBinding: Can reference this group as a subject for role assignments

Group is the Schema for the groups API

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
      <td>Group</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectmeta-v1-meta">metadata</a></b></td>
      <td>object</td>
      <td>Refer to the Kubernetes API documentation for the fields of the `metadata` field.</td>
      <td>true</td>
      </tr><tr>
        <td><b><a href="#groupstatus">status</a></b></td>
        <td>object</td>
        <td>
          GroupStatus defines the observed state of Group, tracking the readiness and
synchronization status of the group resource.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Group.status
<sup><sup>[↩ Parent](#group)</sup></sup>



GroupStatus defines the observed state of Group, tracking the readiness and
synchronization status of the group resource.

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
        <td><b><a href="#groupstatusconditionsindex">conditions</a></b></td>
        <td>[]object</td>
        <td>
          Conditions represent the latest available observations of a group's current state.
The primary condition type is "Ready" which indicates whether the group
is properly initialized and ready for use in the IAM system.

Common condition types:
- Ready: Indicates the group is available for membership operations

Example condition:
  - type: Ready
    status: "True"
    reason: GroupReady
    message: Group successfully created and ready for members<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Group.status.conditions[index]
<sup><sup>[↩ Parent](#groupstatus)</sup></sup>



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

## MachineAccountKey
<sup><sup>[↩ Parent](#iammiloapiscomv1alpha1 )</sup></sup>






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
      <td>iam.miloapis.com/v1alpha1</td>
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
        <td><b>authProviderKeyId</b></td>
        <td>string</td>
        <td>
          AuthProviderKeyID is the unique identifier for the key in the auth provider.
This field is populated by the controller after the key is created in the auth provider.
For example, when using Zitadel, a typical value might be: "326102453042806786"<br/>
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
{metadata.name}@{metadata.namespace}.{project.metadata.name}.{global-suffix}<br/>
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

## PolicyBinding
<sup><sup>[↩ Parent](#iammiloapiscomv1alpha1 )</sup></sup>






PolicyBinding grants roles to users or groups on specific resources in the Milo IAM system.
This is the central resource that connects the three core IAM concepts: subjects (users/groups),
roles (permission sets), and resources (the things being protected).

PolicyBindings are the mechanism through which access control is actually enforced. They
specify which users or groups should receive which permissions (via roles) on which resources
or resource types. This follows the "who can do what on which resource" model of access control.

Key characteristics:
- Namespaced: PolicyBindings exist within a specific namespace context
- Immutable references: Role and resource references cannot be changed after creation
- Flexible resource targeting: Can target specific resource instances or all resources of a type
- Cross-namespace capability: Can reference roles from any namespace
- Multiple subjects: Can grant the same role to multiple users/groups in a single binding

Resource targeting modes:
1. Specific resource (resourceRef): Grants permissions on a single, specific resource instance
2. Resource kind (resourceKind): Grants permissions on ALL resources of a particular type

Common usage patterns:
- Project access: Grant team members access to all resources in a project
- Resource-specific permissions: Grant access to individual workloads, databases, etc.
- Administrative access: Grant admin roles on resource types for operational teams
- Temporary access: Create time-limited bindings for contractor or temporary access

Best practices:
- Use groups as subjects rather than individual users for easier management
- Prefer resource kind bindings for broad access, specific resource refs for targeted access
- Use descriptive names that indicate the purpose of the binding
- Regularly audit PolicyBindings to ensure appropriate access levels
- Leverage the principle of least privilege when designing role assignments

Example - Grant developers access to all workloads in a project:

	apiVersion: iam.miloapis.com/v1alpha1
	kind: PolicyBinding
	metadata:
	  name: developers-workload-access
	  namespace: project-alpha
	spec:
	  roleRef:
	    name: workload-developer
	    namespace: project-alpha
	  subjects:
	  - kind: Group
	    name: developers
	  resourceSelector:
	    resourceKind:
	      apiGroup: compute.miloapis.com
	      kind: Workload

Example - Grant specific user access to a specific database:

	apiVersion: iam.miloapis.com/v1alpha1
	kind: PolicyBinding
	metadata:
	  name: alice-prod-db-access
	  namespace: production
	spec:
	  roleRef:
	    name: database-admin
	  subjects:
	  - kind: User
	    name: alice-smith
	    uid: user-123-abc
	  resourceSelector:
	    resourceRef:
	      apiGroup: data.miloapis.com
	      kind: Database
	      name: production-primary
	      uid: db-456-def
	      namespace: production

Related resources:
- Role: Defines the permissions being granted
- User/Group: The subjects receiving the permissions
- Resource: The target resource(s) being protected

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
          PolicyBindingSpec defines the desired state of PolicyBinding, specifying which
subjects (users/groups) should receive which role on which resources.

This spec contains three key components that together define the complete
access control policy:
1. RoleRef: The role being granted (defines the permissions)
2. Subjects: Who is receiving the role (users and/or groups)
3. ResourceSelector: What resources the role applies to (specific or by type)<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#policybindingstatus">status</a></b></td>
        <td>object</td>
        <td>
          PolicyBindingStatus defines the observed state of PolicyBinding, indicating
whether the access control policy has been successfully applied and is active.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### PolicyBinding.spec
<sup><sup>[↩ Parent](#policybinding)</sup></sup>



PolicyBindingSpec defines the desired state of PolicyBinding, specifying which
subjects (users/groups) should receive which role on which resources.

This spec contains three key components that together define the complete
access control policy:
1. RoleRef: The role being granted (defines the permissions)
2. Subjects: Who is receiving the role (users and/or groups)
3. ResourceSelector: What resources the role applies to (specific or by type)

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
          ResourceSelector specifies which resources the role should be applied to.
This is an immutable field that cannot be changed after creation.

Exactly one of the following must be specified:
- resourceRef: Grants permissions on a specific resource instance
- resourceKind: Grants permissions on all resources of a specific type

Use resourceRef for targeted access to individual resources.
Use resourceKind for broad access across all resources of a type.

Examples:
  # Grant access to all workloads
  resourceSelector:
    resourceKind:
      apiGroup: compute.miloapis.com
      kind: Workload

  # Grant access to specific workload
  resourceSelector:
    resourceRef:
      apiGroup: compute.miloapis.com
      kind: Workload
      name: my-workload
      uid: workload-456-def<br/>
          <br/>
            <i>Validations</i>:<li>oldSelf == null || self == oldSelf: ResourceSelector is immutable and cannot be changed after creation</li><li>has(self.resourceRef) != has(self.resourceKind): exactly one of resourceRef or resourceKind must be specified, but not both</li>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#policybindingspecroleref">roleRef</a></b></td>
        <td>object</td>
        <td>
          RoleRef specifies the Role that should be granted to the subjects.
This is an immutable field that cannot be changed after the PolicyBinding
is created - to change the role, you must delete and recreate the binding.

The role can exist in any namespace, enabling cross-namespace role sharing.
If no namespace is specified, it defaults to the PolicyBinding's namespace.

Example:
  roleRef:
    name: workload-developer
    namespace: shared-roles  # optional, defaults to current namespace<br/>
          <br/>
            <i>Validations</i>:<li>oldSelf == null || self == oldSelf: RoleRef is immutable and cannot be changed after creation</li>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#policybindingspecsubjectsindex">subjects</a></b></td>
        <td>[]object</td>
        <td>
          Subjects specifies the users and/or groups that should receive the role.
Multiple subjects can be listed to grant the same role to multiple entities
in a single PolicyBinding.

Each subject must specify:
- kind: Either "User" or "Group"
- name: The name of the user or group
- uid: The unique identifier (required for users, optional for system groups)

Special group "system:authenticated-users" can be used to grant access
to all authenticated users in the system.

Examples:
  subjects:
  - kind: User
    name: alice-smith
    uid: user-123-abc
  - kind: Group
    name: developers
  - kind: Group
    name: system:authenticated-users  # special system group<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### PolicyBinding.spec.resourceSelector
<sup><sup>[↩ Parent](#policybindingspec)</sup></sup>



ResourceSelector specifies which resources the role should be applied to.
This is an immutable field that cannot be changed after creation.

Exactly one of the following must be specified:
- resourceRef: Grants permissions on a specific resource instance
- resourceKind: Grants permissions on all resources of a specific type

Use resourceRef for targeted access to individual resources.
Use resourceKind for broad access across all resources of a type.

Examples:
  # Grant access to all workloads
  resourceSelector:
    resourceKind:
      apiGroup: compute.miloapis.com
      kind: Workload

  # Grant access to specific workload
  resourceSelector:
    resourceRef:
      apiGroup: compute.miloapis.com
      kind: Workload
      name: my-workload
      uid: workload-456-def

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
        <td><b><a href="#policybindingspecresourceselectorresourcekind">resourceKind</a></b></td>
        <td>object</td>
        <td>
          ResourceKind specifies that the policy binding should apply to all resources of a specific kind.
Mutually exclusive with resourceRef.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#policybindingspecresourceselectorresourceref">resourceRef</a></b></td>
        <td>object</td>
        <td>
          ResourceRef provides a reference to a specific resource instance.
Mutually exclusive with resourceKind.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### PolicyBinding.spec.resourceSelector.resourceKind
<sup><sup>[↩ Parent](#policybindingspecresourceselector)</sup></sup>



ResourceKind specifies that the policy binding should apply to all resources of a specific kind.
Mutually exclusive with resourceRef.

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
        <td>string</td>
        <td>
          Kind is the type of resource being referenced.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>apiGroup</b></td>
        <td>string</td>
        <td>
          APIGroup is the group for the resource type being referenced. If APIGroup
is not specified, the specified Kind must be in the core API group.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### PolicyBinding.spec.resourceSelector.resourceRef
<sup><sup>[↩ Parent](#policybindingspecresourceselector)</sup></sup>



ResourceRef provides a reference to a specific resource instance.
Mutually exclusive with resourceKind.

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
        <td>string</td>
        <td>
          Kind is the type of resource being referenced.<br/>
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
        <td><b>uid</b></td>
        <td>string</td>
        <td>
          UID is the unique identifier of the resource being referenced.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>apiGroup</b></td>
        <td>string</td>
        <td>
          APIGroup is the group for the resource being referenced.
If APIGroup is not specified, the specified Kind must be in the core API group.
For any other third-party types, APIGroup is required.<br/>
        </td>
        <td>false</td>
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


### PolicyBinding.spec.roleRef
<sup><sup>[↩ Parent](#policybindingspec)</sup></sup>



RoleRef specifies the Role that should be granted to the subjects.
This is an immutable field that cannot be changed after the PolicyBinding
is created - to change the role, you must delete and recreate the binding.

The role can exist in any namespace, enabling cross-namespace role sharing.
If no namespace is specified, it defaults to the PolicyBinding's namespace.

Example:
  roleRef:
    name: workload-developer
    namespace: shared-roles  # optional, defaults to current namespace

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


### PolicyBinding.spec.subjects[index]
<sup><sup>[↩ Parent](#policybindingspec)</sup></sup>



Subject contains a reference to the object or user identities a role binding applies to.
This can be a User or Group.

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
            <i>Enum</i>: User, Group<br/>
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
          Namespace of the referenced object. If DNE, then for an SA it refers to the PolicyBinding resource's namespace.
For a User or Group, it is ignored.<br/>
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


### PolicyBinding.status
<sup><sup>[↩ Parent](#policybinding)</sup></sup>



PolicyBindingStatus defines the observed state of PolicyBinding, indicating
whether the access control policy has been successfully applied and is active.

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
        <td><b><a href="#policybindingstatusconditionsindex">conditions</a></b></td>
        <td>[]object</td>
        <td>
          Conditions provide detailed status information about the PolicyBinding resource.
The primary condition type is "Ready" which indicates whether the policy
binding has been successfully applied and is actively enforcing access control.

Common condition types:
- Ready: Indicates the policy binding is active and enforcing access
- RoleFound: Indicates the referenced role exists and is valid
- SubjectsValid: Indicates all referenced subjects (users/groups) exist
- ResourceValid: Indicates the target resource or resource type is valid

Example condition:
  - type: Ready
    status: "True"
    reason: PolicyActive
    message: Policy binding successfully applied and enforcing access<br/>
          <br/>
            <i>Default</i>: [map[lastTransitionTime:1970-01-01T00:00:00Z message:Waiting for control plane to reconcile reason:Unknown status:Unknown type:Ready]]<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>observedGeneration</b></td>
        <td>integer</td>
        <td>
          ObservedGeneration represents the most recent generation that has been
observed and processed by the PolicyBinding controller. This is used to
track whether the controller has processed the latest changes to the spec.<br/>
          <br/>
            <i>Format</i>: int64<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### PolicyBinding.status.conditions[index]
<sup><sup>[↩ Parent](#policybindingstatus)</sup></sup>



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

## ProtectedResource
<sup><sup>[↩ Parent](#iammiloapiscomv1alpha1 )</sup></sup>






ProtectedResource registers a resource type with the Milo IAM system, making it available
for access control through roles and policy bindings. This is a cluster-scoped resource
that defines which resource types can be protected by the IAM system and what permissions
are available for those resources.

ProtectedResources serve as the registry that makes the IAM system aware of different
resource types that exist in the platform. By registering a resource type, system
administrators define the complete set of permissions that can be granted on instances
of that resource type, enabling fine-grained access control.

Key characteristics:
- Cluster-scoped: ProtectedResources exist globally across the control plane
- Administrator-managed: Typically created by system administrators, not end users
- Permission registry: Defines all possible permissions for a resource type
- Hierarchy support: Can specify parent resources to enable permission inheritance
- Service integration: Links resources to their owning services for organization

Permission inheritance through parent resources:
When parent resources are specified, permissions can be granted at higher levels
in the resource hierarchy and automatically apply to child resources. For example,
granting permissions on an Organization can automatically apply to all Projects
within that organization.

Common usage patterns:
- New service integration: Register resource types when adding new services to Milo
- Permission modeling: Define the complete permission set for each resource type
- Hierarchy establishment: Set up parent-child relationships between resource types
- Access control preparation: Make resources available for PolicyBinding targeting

Best practices:
- Use consistent permission naming across similar resource types
- Define comprehensive permission sets that cover all necessary operations
- Establish clear parent-child relationships for logical permission inheritance
- Link resources to appropriate services for proper organization
- Document permission semantics for developers and administrators

Example - Register a Workload resource type:

	apiVersion: iam.miloapis.com/v1alpha1
	kind: ProtectedResource
	metadata:
	  name: workloads
	spec:
	  serviceRef:
	    name: compute.datumapis.com
	  kind: Workload
	  singular: workload
	  plural: workloads
	  permissions:
	  - "compute.datumapis.com/workloads.create"
	  - "compute.datumapis.com/workloads.get"
	  - "compute.datumapis.com/workloads.update"
	  - "compute.datumapis.com/workloads.delete"
	  - "compute.datumapis.com/workloads.list"
	  - "compute.datumapis.com/workloads.scale"
	  parentResources:
	  - apiGroup: resourcemanager.miloapis.com
	    kind: Project

Example - Register a Database resource with organization-level inheritance:

	apiVersion: iam.miloapis.com/v1alpha1
	kind: ProtectedResource
	metadata:
	  name: databases
	spec:
	  serviceRef:
	    name: sql.datumapis.com
	  kind: Database
	  singular: database
	  plural: databases
	  permissions:
	  - "sql.datumapis.com/databases.create"
	  - "sql.datumapis.com/databases.read"
	  - "sql.datumapis.com/databases.update"
	  - "sql.datumapis.com/databases.delete"
	  - "sql.datumapis.com/databases.backup"
	  - "sql.datumapis.com/databases.restore"
	  parentResources:
	  - apiGroup: resourcemanager.miloapis.com
	    kind: Project

Related resources:
- Role: Can include permissions defined in ProtectedResource
- PolicyBinding: Can target resource types registered as ProtectedResource

ProtectedResource is the Schema for the protectedresources API

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
      <td>ProtectedResource</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectmeta-v1-meta">metadata</a></b></td>
      <td>object</td>
      <td>Refer to the Kubernetes API documentation for the fields of the `metadata` field.</td>
      <td>true</td>
      </tr><tr>
        <td><b><a href="#protectedresourcespec">spec</a></b></td>
        <td>object</td>
        <td>
          ProtectedResourceSpec defines the desired state of ProtectedResource, specifying
how a resource type should be registered with the IAM system and what permissions
are available for instances of that resource type.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#protectedresourcestatus">status</a></b></td>
        <td>object</td>
        <td>
          ProtectedResourceStatus defines the observed state of ProtectedResource, indicating
whether the resource type has been successfully registered with the IAM system.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### ProtectedResource.spec
<sup><sup>[↩ Parent](#protectedresource)</sup></sup>



ProtectedResourceSpec defines the desired state of ProtectedResource, specifying
how a resource type should be registered with the IAM system and what permissions
are available for instances of that resource type.

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
        <td>string</td>
        <td>
          Kind specifies the Kubernetes-style kind name for this resource type.
This should match the kind field used in the actual resource definitions
and follow PascalCase naming conventions.

Examples: "Workload", "Database", "StorageBucket"<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>permissions</b></td>
        <td>[]string</td>
        <td>
          Permissions defines the complete set of permissions that can be granted
on instances of this resource type. Each permission should follow the
standard format: {service}/{resource}.{action}

These permissions become available for use in Role definitions and
determine what actions users can perform on resources of this type
when granted appropriate roles through PolicyBindings.

Common permission patterns:
- CRUD operations: create, read, update, delete
- Listing operations: list
- Administrative operations: admin, manage
- Resource-specific operations: scale, backup, restore, etc.

Examples:
  permissions:
  - "compute.datumapis.com/workloads.create"
  - "compute.datumapis.com/workloads.get"
  - "compute.datumapis.com/workloads.update"
  - "compute.datumapis.com/workloads.delete"
  - "compute.datumapis.com/workloads.list"
  - "compute.datumapis.com/workloads.scale"
  - "compute.datumapis.com/workloads.logs"<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>plural</b></td>
        <td>string</td>
        <td>
          Plural specifies the plural form of the resource name, used in API paths
and resource listings. This should follow camelCase naming conventions
and be the lowercase, plural version of the Kind.

Examples: "workloads", "databases", "storageBuckets"<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#protectedresourcespecserviceref">serviceRef</a></b></td>
        <td>object</td>
        <td>
          ServiceRef identifies the service that owns this protected resource type.
This creates a logical grouping of related resource types under their
owning service, helping with organization and management. The service name
should be the API group of the service.

Example:
  serviceRef:
    name: compute.datumapis.com<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>singular</b></td>
        <td>string</td>
        <td>
          Singular specifies the singular form of the resource name, used in API
paths and CLI commands. This should follow camelCase naming conventions
and be the lowercase, singular version of the Kind.

Examples: "workload", "database", "storageBucket"<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#protectedresourcespecparentresourcesindex">parentResources</a></b></td>
        <td>[]object</td>
        <td>
          ParentResources defines the resource types that can serve as parents to
this resource type in the permission hierarchy. When permissions are
granted on a parent resource, they can be inherited by child resources.

This enables powerful permission models where, for example, granting
permissions on an Organization automatically applies to all Projects
within that organization, and all resources within those projects.

Each parent resource reference must specify the apiGroup and kind of
the parent resource type. The parent resource types must also be
registered as ProtectedResources for the inheritance to work properly.

Example hierarchy: Project -> Workload
  parentResources:
  - apiGroup: resourcemanager.miloapis.com
    kind: Project<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### ProtectedResource.spec.serviceRef
<sup><sup>[↩ Parent](#protectedresourcespec)</sup></sup>



ServiceRef identifies the service that owns this protected resource type.
This creates a logical grouping of related resource types under their
owning service, helping with organization and management. The service name
should be the API group of the service.

Example:
  serviceRef:
    name: compute.datumapis.com

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
          Name is the resource name of the service definition.<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### ProtectedResource.spec.parentResources[index]
<sup><sup>[↩ Parent](#protectedresourcespec)</sup></sup>



ParentResourceRef defines the reference to a parent resource

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
        <td>string</td>
        <td>
          Kind is the type of resource being referenced.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>apiGroup</b></td>
        <td>string</td>
        <td>
          APIGroup is the group for the resource being referenced.
If APIGroup is not specified, the specified Kind must be in the core API group.
For any other third-party types, APIGroup is required.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### ProtectedResource.status
<sup><sup>[↩ Parent](#protectedresource)</sup></sup>



ProtectedResourceStatus defines the observed state of ProtectedResource, indicating
whether the resource type has been successfully registered with the IAM system.

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
        <td><b><a href="#protectedresourcestatusconditionsindex">conditions</a></b></td>
        <td>[]object</td>
        <td>
          Conditions provide detailed status information about the ProtectedResource registration.
The primary condition type is "Ready" which indicates whether the resource type
has been successfully registered and is available for use in the IAM system.

Common condition types:
- Ready: Indicates the resource type is registered and available for protection
- ServiceValid: Indicates the referenced service exists
- PermissionsValid: Indicates all specified permissions follow the correct format
- ParentResourcesValid: Indicates all parent resource references are valid

Example condition:
  - type: Ready
    status: "True"
    reason: ResourceRegistered
    message: Resource type successfully registered with IAM system<br/>
          <br/>
            <i>Default</i>: [map[lastTransitionTime:1970-01-01T00:00:00Z message:Waiting for control plane to reconcile reason:Unknown status:Unknown type:Ready]]<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>observedGeneration</b></td>
        <td>integer</td>
        <td>
          ObservedGeneration represents the most recent generation that has been
observed and processed by the ProtectedResource controller. This corresponds
to the resource's metadata.generation and is used to track whether the
controller has processed the latest changes to the spec.<br/>
          <br/>
            <i>Format</i>: int64<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### ProtectedResource.status.conditions[index]
<sup><sup>[↩ Parent](#protectedresourcestatus)</sup></sup>



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

## Role
<sup><sup>[↩ Parent](#iammiloapiscomv1alpha1 )</sup></sup>






Role defines a collection of permissions that can be granted to users or groups in the Milo IAM system.
Roles are namespaced resources that serve as the primary mechanism for defining and organizing
permissions within the access control framework.

Roles can contain two types of permissions:
1. Direct permissions: Explicit permissions listed in the includedPermissions field
2. Inherited permissions: Permissions from other roles specified in inheritedRoles

The system includes predefined roles that are automatically available, and administrators
can create custom roles tailored to specific needs. Roles support inheritance, allowing
for hierarchical permission structures where complex roles can be built from simpler ones.

Key characteristics:
- Namespaced: Roles exist within a specific namespace/project context
- Permission collections: Define sets of permissions using the format {service}/{resource}.{action}
- Inheritance support: Can inherit permissions from other roles with no depth limit
- Launch stage tracking: Indicates the stability level of the role (Early Access, Alpha, Beta, Stable, Deprecated)
- PolicyBinding target: Referenced by PolicyBindings to grant permissions to users/groups

Permission format:
All permissions follow the format: {service}/{resource}.{action}
Examples:
- "compute.datumapis.com/workloads.create" - Create workloads in the compute service
- "iam.miloapis.com/users.get" - Get user information in the IAM service
- "storage.miloapis.com/buckets.delete" - Delete storage buckets

Common usage patterns:
- Predefined system roles: Use built-in roles for common access patterns
- Custom business roles: Create roles that match organizational responsibilities
- Hierarchical permissions: Use inheritance to build complex roles from simple ones
- Environment-specific roles: Create different roles for dev, staging, production

Best practices:
- Follow principle of least privilege when defining permissions
- Use descriptive names that clearly indicate the role's purpose
- Leverage inheritance to avoid permission duplication
- Set appropriate launch stages to indicate role stability
- Group related permissions logically within roles

Example - Basic role with direct permissions:

	apiVersion: iam.miloapis.com/v1alpha1
	kind: Role
	metadata:
	  name: workload-viewer
	  namespace: project-alpha
	spec:
	  launchStage: Stable
	  includedPermissions:
	  - "compute.datumapis.com/workloads.read"
	  - "compute.datumapis.com/workloads.list"

Example - Role with inheritance:

	apiVersion: iam.miloapis.com/v1alpha1
	kind: Role
	metadata:
	  name: workload-admin
	  namespace: project-alpha
	spec:
	  launchStage: Stable
	  includedPermissions:
	  - "compute.datumapis.com/workloads.create"
	  - "compute.datumapis.com/workloads.delete"
	  inheritedRoles:
	  - name: workload-viewer
	    namespace: project-alpha

Related resources:
- PolicyBinding: Binds this role to users/groups on specific resources
- ProtectedResource: Defines the permissions that can be included in roles

Role is the Schema for the roles API

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
      <td>Role</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectmeta-v1-meta">metadata</a></b></td>
      <td>object</td>
      <td>Refer to the Kubernetes API documentation for the fields of the `metadata` field.</td>
      <td>true</td>
      </tr><tr>
        <td><b><a href="#rolespec">spec</a></b></td>
        <td>object</td>
        <td>
          RoleSpec defines the desired state of Role, specifying the permissions and inheritance
configuration that determines what actions users with this role can perform.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#rolestatus">status</a></b></td>
        <td>object</td>
        <td>
          RoleStatus defines the observed state of Role, indicating the current status
of the role's validation, inheritance resolution, and overall readiness.<br/>
          <br/>
            <i>Default</i>: map[conditions:[map[lastTransitionTime:1970-01-01T00:00:00Z message:Waiting for control plane to reconcile reason:Unknown status:Unknown type:Ready]]]<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Role.spec
<sup><sup>[↩ Parent](#role)</sup></sup>



RoleSpec defines the desired state of Role, specifying the permissions and inheritance
configuration that determines what actions users with this role can perform.

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
        <td><b>launchStage</b></td>
        <td>string</td>
        <td>
          LaunchStage indicates the stability and maturity level of this IAM role.
This helps users understand whether the role is stable for production use
or still in development.

Valid values:
- "Early Access": New role with limited availability, subject to breaking changes
- "Alpha": Experimental role that may change significantly
- "Beta": Pre-release role that is feature-complete but may have minor changes
- "Stable": Production-ready role with backwards compatibility guarantees
- "Deprecated": Role scheduled for removal, use alternatives when possible<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>includedPermissions</b></td>
        <td>[]string</td>
        <td>
          IncludedPermissions defines the explicit permissions that this role grants.
Each permission must follow the format: {service}/{resource}.{action}

Examples:
- "compute.datumapis.com/workloads.create" - Permission to create workloads
- "iam.miloapis.com/users.get" - Permission to read user information
- "storage.miloapis.com/buckets.delete" - Permission to delete storage buckets

These permissions are in addition to any permissions inherited from other roles
specified in the inheritedRoles field.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#rolespecinheritedrolesindex">inheritedRoles</a></b></td>
        <td>[]object</td>
        <td>
          InheritedRoles specifies other roles from which this role should inherit permissions.
This enables building complex roles from simpler ones and promotes reusability
of common permission sets.

There is no limit to inheritance depth - roles can inherit from roles that
themselves inherit from other roles. The system will resolve the complete
permission set by following the inheritance chain.

Each inherited role must exist in the same namespace as this role, or specify
a different namespace explicitly. If namespace is omitted, it defaults to
the current role's namespace.

Example:
  inheritedRoles:
  - name: base-viewer  # inherits from base-viewer in same namespace
  - name: admin-tools
    namespace: milo-system  # inherits from admin-tools in milo-system namespace<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Role.spec.inheritedRoles[index]
<sup><sup>[↩ Parent](#rolespec)</sup></sup>



ScopedRoleReference defines a reference to another Role, scoped by namespace.
This is used for role inheritance where one role needs to reference another
role to inherit its permissions. The reference includes both name and optional
namespace for cross-namespace role inheritance.

Example usage in role inheritance:
  inheritedRoles:
  - name: viewer-role        # references viewer-role in same namespace
  - name: admin-base
    namespace: system        # references admin-base in system namespace

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
          Name of the referenced Role. This must match the metadata.name of an
existing Role resource that contains the permissions to be inherited.

Example: "workload-viewer"<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          Namespace of the referenced Role. If not specified, it defaults to the
namespace of the resource containing this reference, enabling same-namespace
role inheritance without explicit namespace specification.

For cross-namespace inheritance, this field must be explicitly set to
the namespace containing the target role.

Example: "system" (for system-wide roles) or "shared-roles"<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Role.status
<sup><sup>[↩ Parent](#role)</sup></sup>



RoleStatus defines the observed state of Role, indicating the current status
of the role's validation, inheritance resolution, and overall readiness.

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
        <td><b><a href="#rolestatusconditionsindex">conditions</a></b></td>
        <td>[]object</td>
        <td>
          Conditions provide detailed status information about the Role resource.
The primary condition type is "Ready" which indicates whether the role
has been successfully validated and is ready for use in PolicyBindings.

Common condition types:
- Ready: Indicates the role is validated and ready for use
- PermissionsValid: Indicates all specified permissions are valid
- InheritanceResolved: Indicates inherited roles have been successfully resolved

Example condition:
  - type: Ready
    status: "True"
    reason: RoleReady
    message: Role successfully validated and ready for use<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>observedGeneration</b></td>
        <td>integer</td>
        <td>
          ObservedGeneration represents the most recent generation that has been
observed and processed by the role controller. This is used to track
whether the controller has processed the latest changes to the role spec.<br/>
          <br/>
            <i>Format</i>: int64<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>parent</b></td>
        <td>string</td>
        <td>
          Parent indicates the resource name of the parent under which this role was created.
This field is typically used for system roles that are automatically created
as part of resource provisioning or service initialization.

Example: "projects/my-project" or "organizations/my-org"<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Role.status.conditions[index]
<sup><sup>[↩ Parent](#rolestatus)</sup></sup>



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

## UserInvitation
<sup><sup>[↩ Parent](#iammiloapiscomv1alpha1 )</sup></sup>






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

## User
<sup><sup>[↩ Parent](#iammiloapiscomv1alpha1 )</sup></sup>






User represents an individual identity in the Milo IAM system. Users are cluster-scoped
resources that exist globally across the entire Milo deployment and serve as the foundation
for identity and access management.

Users are automatically created when a person authenticates or registers with the Milo
platform for the first time, though they can also be created manually by administrators.
Each user is uniquely identified by their email address and integrates with external
identity providers for authentication.

Key characteristics:
- Cluster-scoped: Users exist globally and can be referenced from any namespace
- Email-based identity: Each user is uniquely identified by their email address
- Automatic lifecycle: Created during first authentication/registration
- Cross-namespace access: Can be granted permissions across different projects/namespaces

Common usage patterns:
- New user onboarding when team members join
- Permission management through groups or direct role bindings
- Audit trails for tracking user activities across the system
- Identity foundation for all IAM operations

Example:

	apiVersion: iam.miloapis.com/v1alpha1
	kind: User
	metadata:
	  name: jane-doe
	spec:
	  email: jane.doe@company.com
	  givenName: Jane
	  familyName: Doe

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
      <td>User</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectmeta-v1-meta">metadata</a></b></td>
      <td>object</td>
      <td>Refer to the Kubernetes API documentation for the fields of the `metadata` field.</td>
      <td>true</td>
      </tr><tr>
        <td><b><a href="#userspec">spec</a></b></td>
        <td>object</td>
        <td>
          UserSpec defines the desired state of User, containing the core identity information
that uniquely identifies and describes a user in the system.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#userstatus">status</a></b></td>
        <td>object</td>
        <td>
          UserStatus defines the observed state of User, indicating the current status
of the user's synchronization with external systems and overall readiness.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### User.spec
<sup><sup>[↩ Parent](#user)</sup></sup>



UserSpec defines the desired state of User, containing the core identity information
that uniquely identifies and describes a user in the system.

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
          Email is the unique email address that identifies this user in the system.
This field is required and serves as the primary identifier for the user.
The email must be unique across all users in the cluster.

Example: "jane.doe@company.com"<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>familyName</b></td>
        <td>string</td>
        <td>
          FamilyName is the user's last name or family name. This field is optional
and is used for display purposes and user identification in UI contexts.

Example: "Doe"<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>givenName</b></td>
        <td>string</td>
        <td>
          GivenName is the user's first name or given name. This field is optional
and is used for display purposes and user identification in UI contexts.

Example: "Jane"<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### User.status
<sup><sup>[↩ Parent](#user)</sup></sup>



UserStatus defines the observed state of User, indicating the current status
of the user's synchronization with external systems and overall readiness.

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
        <td><b><a href="#userstatusconditionsindex">conditions</a></b></td>
        <td>[]object</td>
        <td>
          Conditions provide detailed status information about the User resource.
The primary condition type is "Ready" which indicates whether the user
has been successfully synchronized with the authentication provider and
is ready for use in the IAM system.

Common condition types:
- Ready: Indicates the user is properly synchronized and available
- Synced: Indicates successful synchronization with external auth provider

Example condition:
  - type: Ready
    status: "True"
    reason: UserReady
    message: User successfully synchronized with auth provider<br/>
          <br/>
            <i>Default</i>: [map[lastTransitionTime:1970-01-01T00:00:00Z message:Waiting for control plane to reconcile reason:Unknown status:Unknown type:Ready]]<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### User.status.conditions[index]
<sup><sup>[↩ Parent](#userstatus)</sup></sup>



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
