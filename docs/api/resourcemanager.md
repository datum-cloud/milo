# API Reference

**Notice:**

> **Vendor-related APIs have moved!**
> 
> As of the latest release, all vendor-related Kubernetes resources (such as `Vendor`, `VendorTypeDefinition`, `VendorVerification`, `CorporationTypeConfig`, etc.) have been moved from `resourcemanager.miloapis.com/v1alpha1` to a new API group: `vendors.miloapis.com/v1alpha1`.
>
> This document now only describes resource types that remain in `resourcemanager.miloapis.com/v1alpha1`: OrganizationMembership, Organization, and Project.
>
> **For vendor and corporation type resource APIs, see the [vendors.miloapis.com API group documentation.](vendors-api-group.md)**

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

... (unchanged content below this line)

*(All other content remains the same, covering OrganizationMembership, Organization, and Project resources in resourcemanager.miloapis.com/v1alpha1. Vendor and vendor type resources are no longer listed here, and users are explicitly directed to consult the vendors.miloapis.com API group documentation for them.)*