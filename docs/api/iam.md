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

- [UserIdentity](#useridentity)


## GroupMembership
<sup><sup>[↩ Parent](#iammiloapiscomv1alpha1 )</sup></sup>

... (existing content for GroupMembership -- unchanged) ...

## UserIdentity
<sup><sup>[↩ Parent](#iammiloapiscomv1alpha1 )</sup></sup>

UserIdentity represents a user's linked identity within an external identity provider. This resource describes the connection between a Milo user and their account in an external authentication provider (e.g., GitHub, Google, Microsoft). It is NOT the identity provider itself, but rather the user's specific identity within that provider.

Use cases include:
- Display all authentication methods linked to a user account in the UI
- Show which external accounts a user has connected
- Provide visibility into federated identity mappings

**Important notes:**
- This is a read-only resource for display purposes only
- Identity management (linking/unlinking providers) is handled by the external authentication provider (e.g., Zitadel), not through this API
- No sensitive credentials or tokens are exposed through this resource

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody>
      <tr>
        <td><b>apiVersion</b></td>
        <td>string</td>
        <td>identity.miloapis.com/v1alpha1</td>
        <td>true</td>
      </tr>
      <tr>
        <td><b>kind</b></td>
        <td>string</td>
        <td>UserIdentity</td>
        <td>true</td>
      </tr>
      <tr>
        <td><b><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectmeta-v1-meta">metadata</a></b></td>
        <td>object</td>
        <td>Refer to the Kubernetes API documentation for the fields of the <code>metadata</code> field.</td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#useridentitystatus">status</a></b></td>
        <td>object</td>
        <td>
          UserIdentityStatus contains the details of a user's identity within an external provider. All fields are read-only and populated by the authentication provider.<br/>
        </td>
        <td>true</td>
      </tr>
    </tbody>
</table>

### UserIdentity.status
<sup><sup>[↩ Parent](#useridentity)</sup></sup>

UserIdentityStatus contains the details of a user's identity within an external provider. All fields are read-only and populated by the authentication provider.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody>
      <tr>
        <td><b>userUID</b></td>
        <td>string</td>
        <td>
          UserUID is the unique identifier of the Milo user who owns this identity.<br/>
        </td>
        <td>true</td>
      </tr>
      <tr>
        <td><b>providerID</b></td>
        <td>string</td>
        <td>
          ProviderID is the unique identifier of the external identity provider instance. This is typically an internal ID from the authentication system.<br/>
        </td>
        <td>true</td>
      </tr>
      <tr>
        <td><b>providerName</b></td>
        <td>string</td>
        <td>
          ProviderName is the human-readable name of the identity provider. Examples: "GitHub", "Google", "Microsoft", "GitLab"<br/>
        </td>
        <td>true</td>
      </tr>
      <tr>
        <td><b>username</b></td>
        <td>string</td>
        <td>
          Username is the user's username or identifier within the external identity provider. This is the name the user is known by in the external system (e.g., GitHub username).<br/>
        </td>
        <td>true</td>
      </tr>
    </tbody>
</table>

... (rest of the document unchanged) ...
