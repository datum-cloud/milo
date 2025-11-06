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



## ...

## User
<sup><sup>[↩ Parent](#iammiloapiscomv1alpha1 )</sup></sup>

User is the Schema for the users API

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
          UserSpec defines the desired state of User<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#userstatus">status</a></b></td>
        <td>object</td>
        <td>
          UserStatus defines the observed state of User<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### User.status
<sup><sup>[↩ Parent](#user)</sup></sup>

UserStatus defines the observed state of User

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
          Conditions provide conditions that represent the current status of the User.<br/>
          <br/>
            <i>Default</i>: [map[lastTransitionTime:1970-01-01T00:00:00Z message:Waiting for control plane to reconcile reason:Unknown status:Unknown type:Ready]]<br/>
          <br/>
          <b>Waitlist Email Status Conditions:</b> The platform sets a condition of type <tt>WaitlistPendingEmailSent</tt>, <tt>WaitlistApprovedEmailSent</tt>, or <tt>WaitlistRejectedEmailSent</tt> (see below) to <tt>True</tt> after it sends the respective waitlist email notification to a user's <tt>spec.email</tt>. These conditions are managed automatically according to <tt>status.registrationApproval</tt> ("Pending", "Approved", or "Rejected") and track one delivery per state transition.
          <ul>
            <li><b>WaitlistPendingEmailSent</b>: The pending waitlist email was sent.</li>
            <li><b>WaitlistApprovedEmailSent</b>: The approved waitlist email was sent.</li>
            <li><b>WaitlistRejectedEmailSent</b>: The rejected waitlist email was sent.</li>
          </ul>
          The <tt>reason</tt> field for these conditions will be <tt>EmailSent</tt> if the notification was successfully sent.
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>registrationApproval</b></td>
        <td>enum</td>
        <td>
          RegistrationApproval represents the administrator’s decision on the user’s registration request.
States:
  - Pending:  The user is awaiting review by an administrator.
  - Approved: The user registration has been approved.
  - Rejected: The user registration has been rejected.
The User resource is always created regardless of this value, but the
ability for the person to sign into the platform and access resources is
governed by this status: only *Approved* users are granted access, while
*Pending* and *Rejected* users are prevented for interacting with resources.<br/>
          <br/>
            <i>Enum</i>: Pending, Approved, Rejected<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>state</b></td>
        <td>enum</td>
        <td>
          State represents the current activation state of the user account from the
auth provider. This field is managed exclusively by the UserDeactivation CRD
and cannot be changed directly by the user. When a UserDeactivation resource
is created for the user, the user is deactivated in the auth provider; when
the UserDeactivation is deleted, the user is reactivated.
States:
  - Active: The user can be used to authenticate.
  - Inactive: The user is prohibited to be used to authenticate, and revokes all existing sessions.<br/>
          <br/>
            <i>Enum</i>: Active, Inactive<br/>
            <i>Default</i>: Active<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### User.status.conditions[index]
<sup><sup>[↩ Parent](#userstatus)</sup></sup>

Condition contains details for one aspect of the current state of this API Resource. Several condition <tt>type</tt> values exist, notably:
<ul>
<li><b>Ready</b></li>
<li><b>WaitlistPendingEmailSent</b></li>
<li><b>WaitlistApprovedEmailSent</b></li>
<li><b>WaitlistRejectedEmailSent</b></li>
</ul>

The three "Waitlist...EmailSent" types indicate that the platform has attempted to send (and, on status True, has sent) the matching waitlist-related notification email. See the description above for when these are set.

The <tt>reason</tt> for these conditions may be:
<ul>
<li><b>EmailSent</b>: The waitlist email notification was sent successfully.</li>
</ul>

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
Common values for waitlist email conditions are:
<ul>
  <li><b>EmailSent</b>: The waitlist notification email was sent successfully.</li>
</ul>
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
          type of condition in CamelCase or in foo.example.com/CamelCase.
Waitlist-related types handled by the platform:
<ul>
  <li><b>WaitlistPendingEmailSent</b></li>
  <li><b>WaitlistApprovedEmailSent</b></li>
  <li><b>WaitlistRejectedEmailSent</b></li>
</ul>
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

<!-- The rest of the document is unchanged -->

