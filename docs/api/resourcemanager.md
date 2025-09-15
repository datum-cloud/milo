
# API Reference

Packages:

- [resourcemanager.miloapis.com/v1alpha1](#resourcemanagermiloapiscomv1alpha1)

# resourcemanager.miloapis.com/v1alpha1

Resource Types:

- [OrganizationMembership](#organizationmembership)
- [Organization](#organization)
- [Project](#project)
- [Vendor](#vendor)
- [CorporationTypeConfig](#corporationtypeconfig)

## OrganizationMembership
<sup><sup>[↩ Parent](#resourcemanagermiloapiscomv1alpha1 )</sup></sup>

... [OrganizationMembership, Organization, Project documentation unchanged] ...

## Vendor
<sup><sup>[↩ Parent](#resourcemanagermiloapiscomv1alpha1 )</sup></sup>

Vendor is the Schema for the Vendors API.

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
      <td>Vendor</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectmeta-v1-meta">metadata</a></b></td>
      <td>object</td>
      <td>Refer to the Kubernetes API documentation for the fields of the `metadata` field.</td>
      <td>true</td>
      </tr><tr>
        <td><b><a href="#vendorspec">spec</a></b></td>
        <td>object</td>
        <td>
          VendorSpec defines the desired state of Vendor<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#vendorstatus">status</a></b></td>
        <td>object</td>
        <td>
          VendorStatus defines the observed state of Vendor<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>

### Vendor.spec
<sup><sup>[↩ Parent](#vendor)</sup></sup>

VendorSpec defines the desired state of Vendor.

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
      <tr><td><b>profileType</b></td><td>enum</td><td>Profile type - person or business<br/><i>Enum</i>: person, business</td><td>true</td></tr>
      <tr><td><b>legalName</b></td><td>string</td><td>Legal name of the vendor</td><td>true</td></tr>
      <tr><td><b>nickname</b></td><td>string</td><td>Nickname or display name</td><td>false</td></tr>
      <tr><td><b>billingAddress</b></td><td>object</td><td>Billing address of the vendor</td><td>true</td></tr>
      <tr><td><b>mailingAddress</b></td><td>object</td><td>Mailing address (if different from billing)</td><td>false</td></tr>
      <tr><td><b>description</b></td><td>string</td><td>Description of the vendor</td><td>false</td></tr>
      <tr><td><b>website</b></td><td>string</td><td>Website URL</td><td>false</td></tr>
      <tr><td><b>status</b></td><td>enum</td><td>Current status of the vendor<br/><i>Enum</i>: pending, active, rejected, archived</td><td>true</td></tr>
      <tr><td><b>corporationType</b></td><td>string</td><td>Reference to a corporation type code defined in CorporationTypeConfig (only for business profileType)</td><td>false</td></tr>
      <tr><td><b>corporationDBA</b></td><td>string</td><td>Doing business as name</td><td>false</td></tr>
      <tr><td><b>registrationNumber</b></td><td>string</td><td>Registration number</td><td>false</td></tr>
      <tr><td><b>stateOfIncorporation</b></td><td>string</td><td>State of incorporation</td><td>false</td></tr>
      <tr><td><b>taxInfo</b></td><td>object</td><td>Tax information for the vendor</td><td>true</td></tr>
    </tbody>
</table>

### Vendor.spec.billingAddress / Vendor.spec.mailingAddress
Address object fields:
<table><thead><tr><th>Name</th><th>Type</th><th>Description</th><th>Required</th></tr></thead><tbody>
<tr><td>street</td><td>string</td><td>Street address line 1</td><td>true</td></tr>
<tr><td>street2</td><td>string</td><td>Street address line 2 (optional)</td><td>false</td></tr>
<tr><td>city</td><td>string</td><td>City</td><td>true</td></tr>
<tr><td>state</td><td>string</td><td>State or province</td><td>true</td></tr>
<tr><td>postalCode</td><td>string</td><td>Postal or ZIP code</td><td>true</td></tr>
<tr><td>country</td><td>string</td><td>Country</td><td>true</td></tr>
</tbody></table>

### Vendor.spec.taxInfo
TaxInfo object fields:
<table><thead><tr><th>Name</th><th>Type</th><th>Description</th><th>Required</th></tr></thead><tbody>
<tr><td>taxIdType</td><td>enum</td><td>Type of tax identification<br/><i>Enum</i>: SSN, EIN, ITIN, UNSPECIFIED</td><td>true</td></tr>
<tr><td>taxId</td><td>string</td><td>Tax identification number</td><td>true</td></tr>
<tr><td>country</td><td>string</td><td>Country for tax purposes</td><td>true</td></tr>
<tr><td>taxDocument</td><td>string</td><td>Tax document reference (e.g., W-9, W-8BEN)</td><td>true</td></tr>
<tr><td>taxVerified</td><td>boolean</td><td>Whether tax information has been verified</td><td>false</td></tr>
<tr><td>verificationTimestamp</td><td>string</td><td>Timestamp of tax verification<br/><i>Format</i>: date-time</td><td>false</td></tr>
</tbody></table>

### Vendor.status
<sup><sup>[↩ Parent](#vendor)</sup></sup>

VendorStatus defines the observed state of Vendor

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
      <tr><td><b>conditions</b></td><td>[]object</td><td>Conditions representing the vendor's current state.<br/>Default: [map[lastTransitionTime:1970-01-01T00:00:00Z message:Waiting for control plane to reconcile reason:Unknown status:Unknown type:Ready]]</td><td>false</td></tr>
      <tr><td><b>observedGeneration</b></td><td>integer</td><td>ObservedGeneration is the most recent generation observed for this Vendor by the controller.<br/><i>Format</i>: int64</td><td>false</td></tr>
    </tbody>
</table>

### Vendor.status.conditions[index]
See condition details in previous sections (Organization/Project Conditions).

## CorporationTypeConfig
<sup><sup>[↩ Parent](#resourcemanagermiloapiscomv1alpha1 )</sup></sup>

CorporationTypeConfig is the Schema for the CorporationTypeConfigs API

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
      <td>CorporationTypeConfig</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectmeta-v1-meta">metadata</a></b></td>
      <td>object</td>
      <td>Refer to the Kubernetes API documentation for the fields of the `metadata` field.</td>
      <td>true</td>
      </tr><tr>
        <td><b><a href="#corporationtypeconfigspec">spec</a></b></td>
        <td>object</td>
        <td>
          CorporationTypeConfigSpec defines the desired state of CorporationTypeConfig<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#corporationtypeconfigstatus">status</a></b></td>
        <td>object</td>
        <td>
          CorporationTypeConfigStatus defines the observed state of CorporationTypeConfig<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>

### CorporationTypeConfig.spec
<sup><sup>[↩ Parent](#corporationtypeconfig)</sup></sup>

CorporationTypeConfigSpec defines the desired state of CorporationTypeConfig

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
      <tr><td><b>active</b></td><td>boolean</td><td>Whether this configuration is active</td><td>true</td></tr>
      <tr><td><b>corporationTypes</b></td><td>[]object</td><td>Array of corporation type definitions available</td><td>true</td></tr>
    </tbody>
</table>

#### CorporationTypeConfig.spec.corporationTypes[]
CorporationTypeDefinition fields:
<table><thead><tr><th>Name</th><th>Type</th><th>Description</th><th>Required</th></tr></thead><tbody>
<tr><td>code</td><td>string</td><td>The unique identifier for this corporation type<br/>Must match ^[a-z0-9-]+$</td><td>true</td></tr>
<tr><td>displayName</td><td>string</td><td>Human-readable display name</td><td>true</td></tr>
<tr><td>description</td><td>string</td><td>Optional description of this corporation type</td><td>false</td></tr>
<tr><td>enabled</td><td>boolean</td><td>Whether this corporation type is currently available</td><td>true</td></tr>
<tr><td>sortOrder</td><td>integer</td><td>Sort order for display purposes (lower numbers appear first)</td><td>true</td></tr>
</tbody></table>

### CorporationTypeConfig.status
<sup><sup>[↩ Parent](#corporationtypeconfig)</sup></sup>

CorporationTypeConfigStatus defines the observed state of CorporationTypeConfig

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
      <tr><td><b>activeTypeCount</b></td><td>integer</td><td>Number of active corporation types</td><td>false</td></tr>
      <tr><td><b>conditions</b></td><td>[]object</td><td>Conditions representing the corporation type config's current state.<br/>Default: [map[lastTransitionTime:1970-01-01T00:00:00Z message:Waiting for control plane to reconcile reason:Unknown status:Unknown type:Ready]]</td><td>false</td></tr>
      <tr><td><b>observedGeneration</b></td><td>integer</td><td>ObservedGeneration is the most recent generation observed for this CorporationTypeConfig by the controller.<br/><i>Format</i>: int64</td><td>false</td></tr>
    </tbody>
</table>

### CorporationTypeConfig.status.conditions[index]
See condition details in previous condition sections (Organization/Project Conditions).

