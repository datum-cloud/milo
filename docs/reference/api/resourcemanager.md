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






OrganizationMembership links a user to an organization, establishing the
foundation for role-based access control within organizations. Note that
membership alone does not grant access - a PolicyBinding must also be
created to assign roles and permissions to the user.

OrganizationMemberships are namespaced resources that create relationships
between cluster-scoped users and organizations. They are a prerequisite
for access control but do not grant permissions by themselves.

Key characteristics:
- Namespaced: Created within the organization's namespace
- User-organization linkage: Connects users to organizations
- Access prerequisite: Required before PolicyBindings can grant organization permissions
- Status information: Provides cached details about both user and organization

Common workflows:
1. Ensure both the user and organization exist and are ready
2. Create the membership in the organization's namespace
3. Wait for the Ready condition to become True
4. Create PolicyBinding resources to grant specific roles and permissions
5. User can now access organization resources based on assigned policies

Prerequisites:
- User: The referenced user must exist and be ready
- Organization: The referenced organization must exist and be ready
- Namespace: Must be created in the organization's associated namespace

Example - Adding a user to an organization:

	apiVersion: resourcemanager.miloapis.com/v1alpha1
	kind: OrganizationMembership
	metadata:
	  name: jane-doe-acme-membership
	  namespace: organization-acme-corp
	spec:
	  organizationRef:
	    name: acme-corp
	  userRef:
	    name: jane-doe

Related resources:
- User: Must exist before creating membership
- Organization: Must exist before creating membership
- PolicyBinding: Required to grant actual permissions after membership is established

Troubleshooting:
- Check the Ready condition in status to verify successful membership
- Ensure both user and organization resources exist and are ready
- Verify the membership is created in the correct organization namespace
- Remember that PolicyBinding resources are still needed to grant actual permissions
- List memberships within the organization namespace to verify creation

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
          OrganizationMembershipSpec defines the desired membership relationship
between a user and an organization.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#organizationmembershipstatus">status</a></b></td>
        <td>object</td>
        <td>
          OrganizationMembershipStatus defines the observed state of OrganizationMembership,
indicating whether the membership has been successfully established.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### OrganizationMembership.spec
<sup><sup>[↩ Parent](#organizationmembership)</sup></sup>



OrganizationMembershipSpec defines the desired membership relationship
between a user and an organization.

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
          OrganizationRef identifies the organization to grant membership in.
The organization must exist before creating the membership.

Example:
  organizationRef:
    name: acme-corp<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#organizationmembershipspecuserref">userRef</a></b></td>
        <td>object</td>
        <td>
          UserRef identifies the user to grant organization membership.
The user must exist before creating the membership.

Example:
  userRef:
    name: jane-doe<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### OrganizationMembership.spec.organizationRef
<sup><sup>[↩ Parent](#organizationmembershipspec)</sup></sup>



OrganizationRef identifies the organization to grant membership in.
The organization must exist before creating the membership.

Example:
  organizationRef:
    name: acme-corp

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
          Name is the name of the organization to reference.
Must match an existing organization resource.

Example: "acme-corp"<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### OrganizationMembership.spec.userRef
<sup><sup>[↩ Parent](#organizationmembershipspec)</sup></sup>



UserRef identifies the user to grant organization membership.
The user must exist before creating the membership.

Example:
  userRef:
    name: jane-doe

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
          Name is the name of the user to reference.
Must match an existing user resource.

Example: "jane-doe"<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### OrganizationMembership.status
<sup><sup>[↩ Parent](#organizationmembership)</sup></sup>



OrganizationMembershipStatus defines the observed state of OrganizationMembership,
indicating whether the membership has been successfully established.

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
          Conditions describe the current state of membership establishment.
Check the "Ready" condition to determine if the membership is
active and the user has access to organization resources.

Common condition types:
- Ready: Membership is established and user has organization access

Example ready condition:
  - type: Ready
    status: "True"
    reason: MembershipReady
    message: User successfully added to organization<br/>
          <br/>
            <i>Default</i>: [map[lastTransitionTime:1970-01-01T00:00:00Z message:Waiting for control plane to reconcile reason:Unknown status:Unknown type:Ready]]<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>observedGeneration</b></td>
        <td>integer</td>
        <td>
          ObservedGeneration tracks the most recent membership spec that the
controller has processed. Use this to determine if status reflects
the latest changes.<br/>
          <br/>
            <i>Format</i>: int64<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#organizationmembershipstatusorganization">organization</a></b></td>
        <td>object</td>
        <td>
          Organization contains cached information about the organization in this membership.
This information is populated by the controller from the referenced organization.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#organizationmembershipstatususer">user</a></b></td>
        <td>object</td>
        <td>
          User contains cached information about the user in this membership.
This information is populated by the controller from the referenced user.<br/>
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



Organization contains cached information about the organization in this membership.
This information is populated by the controller from the referenced organization.

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
          DisplayName is the human-readable name of the organization.
Populated from the kubernetes.io/display-name annotation of the organization.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>type</b></td>
        <td>string</td>
        <td>
          Type is the business model of the organization (Personal or Standard).
Populated from the referenced organization resource.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### OrganizationMembership.status.user
<sup><sup>[↩ Parent](#organizationmembershipstatus)</sup></sup>



User contains cached information about the user in this membership.
This information is populated by the controller from the referenced user.

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
          Email is the email address of the user.
Populated from the referenced user resource.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>familyName</b></td>
        <td>string</td>
        <td>
          FamilyName is the last name of the user.
Populated from the referenced user resource.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>givenName</b></td>
        <td>string</td>
        <td>
          GivenName is the first name of the user.
Populated from the referenced user resource.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>

## Organization
<sup><sup>[↩ Parent](#resourcemanagermiloapiscomv1alpha1 )</sup></sup>






Organization represents the top-level tenant boundary in Milo's control plane
for consumers of services. Organizations provide complete isolation and serve
as the root of the resource hierarchy for access control and resource management.

Organizations are cluster-scoped resources that automatically create an
associated namespace named "organization-{name}" for organizing related
resources. All projects must be owned by an organization.

Choose the organization type based on your use case:
- Personal: Individual developers and small projects
- Standard: Teams, businesses, and production workloads

Key characteristics:
- Cluster-scoped: Organizations exist globally across the Milo deployment
- Immutable type: Organization type cannot be changed after creation
- Automatic namespacing: Creates "organization-{name}" namespace
- Resource hierarchy root: Contains projects and user memberships
- Tenant isolation: Complete isolation between different organizations

Common workflows:
1. Create organization for your team or business
2. Add organization members using OrganizationMembership resources
3. Create projects within the organization
4. Deploy resources within organization projects

Prerequisites:
- None (organizations are the root of the resource hierarchy)

Example - Personal organization:

	apiVersion: resourcemanager.miloapis.com/v1alpha1
	kind: Organization
	metadata:
	  name: jane-doe-personal
	  annotations:
	    kubernetes.io/display-name: "Jane's Personal Projects"
	spec:
	  type: Personal

Example - Standard business organization:

	apiVersion: resourcemanager.miloapis.com/v1alpha1
	kind: Organization
	metadata:
	  name: acme-corp
	  annotations:
	    kubernetes.io/display-name: "ACME Corporation"
	spec:
	  type: Standard

Related resources:
- Project: Projects must be owned by an organization
- OrganizationMembership: Links users to organizations
- IAM resources: Inherit permissions from organization level

Troubleshooting:
- Check the Ready condition in status to verify successful creation
- List all organizations to verify creation and status
- Display names are set via the kubernetes.io/display-name annotation

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
          OrganizationSpec defines the desired state of Organization, specifying the
business characteristics that determine how the organization operates.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#organizationstatus">status</a></b></td>
        <td>object</td>
        <td>
          OrganizationStatus defines the observed state of Organization, indicating
whether the organization has been successfully created and is ready for use.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Organization.spec
<sup><sup>[↩ Parent](#organization)</sup></sup>



OrganizationSpec defines the desired state of Organization, specifying the
business characteristics that determine how the organization operates.

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
          Type specifies the business model for this organization.
This field determines resource limits, billing, and available features.

Choose "Personal" for individual users and small projects.
Choose "Standard" for teams and business use cases.

Warning: The type cannot be changed after organization creation.

Example: "Standard"<br/>
          <br/>
            <i>Validations</i>:<li>type(oldSelf) == null_type || self == oldSelf: organization type is immutable</li>
            <i>Enum</i>: Personal, Standard<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Organization.status
<sup><sup>[↩ Parent](#organization)</sup></sup>



OrganizationStatus defines the observed state of Organization, indicating
whether the organization has been successfully created and is ready for use.

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
          Conditions describe the current state of organization provisioning.
Check the "Ready" condition to determine if the organization is
available for creating projects and adding members.

Common condition types:
- Ready: Organization is provisioned and ready for use

Example ready condition:
  - type: Ready
    status: "True"
    reason: OrganizationReady
    message: Organization successfully created<br/>
          <br/>
            <i>Default</i>: [map[lastTransitionTime:1970-01-01T00:00:00Z message:Waiting for control plane to reconcile reason:Unknown status:Unknown type:Ready]]<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>observedGeneration</b></td>
        <td>integer</td>
        <td>
          ObservedGeneration tracks the most recent organization spec that the
controller has processed. Use this to determine if status reflects
the latest changes.<br/>
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






Project represents a logical container for related resources within an
organization. Projects provide resource organization and access control
boundaries for your applications and workloads.

Projects are cluster-scoped resources that must be owned by an organization.
They serve as the primary unit for organizing and managing resources in Milo.

Key characteristics:
- Cluster-scoped: Projects exist globally across the Milo deployment
- Organization-owned: Each project must reference a valid organization
- Resource container: Groups related resources for management
- Access control boundary: Inherits permissions from the owning organization

Common workflows:
1. Ensure the owning organization exists and is ready
2. Create the project with a reference to the organization
3. Wait for the Ready condition to become True
4. Deploy your applications and resources within the project

Prerequisites:
- Organization: The referenced organization must exist and be ready

Example - Development project:

	apiVersion: resourcemanager.miloapis.com/v1alpha1
	kind: Project
	metadata:
	  name: web-app-dev
	  annotations:
	    kubernetes.io/display-name: "Web App Development"
	spec:
	  ownerRef:
	    kind: Organization
	    name: acme-corp

Example - Production project:

	apiVersion: resourcemanager.miloapis.com/v1alpha1
	kind: Project
	metadata:
	  name: web-app-prod
	  annotations:
	    kubernetes.io/display-name: "Web App Production"
	spec:
	  ownerRef:
	    kind: Organization
	    name: acme-corp

Related resources:
- Organization: Must exist before creating projects
- IAM resources: Projects inherit permissions from organizations

Troubleshooting:
- Check the Ready condition in status to verify successful provisioning
- Ensure the referenced organization exists and is ready
- List all projects to verify creation and status
- Display names are set via the kubernetes.io/display-name annotation

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
          ProjectSpec defines the configuration for a project, specifying which
organization owns it.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#projectstatus">status</a></b></td>
        <td>object</td>
        <td>
          ProjectStatus defines the observed state of Project, indicating whether
the project has been successfully provisioned and is ready for use.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Project.spec
<sup><sup>[↩ Parent](#project)</sup></sup>



ProjectSpec defines the configuration for a project, specifying which
organization owns it.

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
          OwnerRef references the organization that owns this project.
Projects must be owned by an organization and inherit its permissions.

The organization must exist before creating the project.
Currently only Organization resources are supported as owners.

Example:
  ownerRef:
    kind: Organization
    name: acme-corp<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Project.spec.ownerRef
<sup><sup>[↩ Parent](#projectspec)</sup></sup>



OwnerRef references the organization that owns this project.
Projects must be owned by an organization and inherit its permissions.

The organization must exist before creating the project.
Currently only Organization resources are supported as owners.

Example:
  ownerRef:
    kind: Organization
    name: acme-corp

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
          Kind specifies the type of resource that owns this project.
Currently only "Organization" is supported.

Example: "Organization"<br/>
          <br/>
            <i>Enum</i>: Organization<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name is the name of the organization that owns this project.
The organization must exist before creating the project.

Example: "acme-corp"<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Project.status
<sup><sup>[↩ Parent](#project)</sup></sup>



ProjectStatus defines the observed state of Project, indicating whether
the project has been successfully provisioned and is ready for use.

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
          Conditions describe the current state of project provisioning.
Check the "Ready" condition to determine if the project is
available for deploying resources.

Common condition types:
- Ready: Project is provisioned and ready for use

Example ready condition:
  - type: Ready
    status: "True"
    reason: ProjectReady
    message: Project successfully provisioned<br/>
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
