# API Reference

Packages:

- [infrastructure.miloapis.com/v1alpha1](#infrastructuremiloapiscomv1alpha1)
- [resourcemanager.miloapis.com/v1alpha1](#resourcemanagermiloapiscomv1alpha1)

# infrastructure.miloapis.com/v1alpha1

Resource Types:

- [ProjectControlPlane](#projectcontrolplane)

# resourcemanager.miloapis.com/v1alpha1

Resource Types:

- [Vendor](#vendor)
- [CorporationTypeConfig](#corporationtypeconfig)



---

## ProjectControlPlane
<sup><sup>[↩ Parent](#infrastructuremiloapiscomv1alpha1 )</sup></sup>

ProjectControlPlane is the Schema for the projectcontrolplanes API.

<table>
<thead>
<tr><th>Name</th><th>Type</th><th>Description</th><th>Required</th></tr>
</thead>
<tbody>
<tr><td><b>apiVersion</b></td><td>string</td><td>infrastructure.miloapis.com/v1alpha1</td><td>true</td></tr>
<tr><td><b>kind</b></td><td>string</td><td>ProjectControlPlane</td><td>true</td></tr>
<tr><td><b><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectmeta-v1-meta">metadata</a></b></td><td>object</td><td>Refer to the Kubernetes API documentation for the fields of the `metadata` field.</td><td>true</td></tr>
<tr><td><b>spec</b></td><td>object</td><td>ProjectControlPlaneSpec defines the desired state of ProjectControlPlane.</td><td>true</td></tr>
<tr><td><b><a href="#projectcontrolplanestatus">status</a></b></td><td>object</td><td>ProjectControlPlaneStatus defines the observed state of ProjectControlPlane.<br/><br/><i>Default</i>: map[conditions:[map[lastTransitionTime:1970-01-01T00:00:00Z message:Creating a new control plane for the project reason:Creating status:False type:ControlPlaneReady]]]</td><td>false</td></tr>
</tbody>
</table>

### ProjectControlPlane.status
<sup><sup>[↩ Parent](#projectcontrolplane)</sup></sup>

ProjectControlPlaneStatus defines the observed state of ProjectControlPlane.

<table>
<thead>
<tr><th>Name</th><th>Type</th><th>Description</th><th>Required</th></tr>
</thead>
<tbody><tr>
<td><b><a href="#projectcontrolplanestatusconditionsindex">conditions</a></b></td>
<td>[]object</td>
<td>
Represents the observations of a project control plane's current state. Known condition types are: "Ready"<br/>
</td>
<td>false</td>
</tr></tbody>
</table>

### ProjectControlPlane.status.conditions[index]
<sup><sup>[↩ Parent](#projectcontrolplanestatus)</sup></sup>

Condition contains details for one aspect of the current state of this API Resource.

<table>
<thead>
<tr><th>Name</th><th>Type</th><th>Description</th><th>Required</th></tr>
</thead>
<tbody>
<tr>
<td><b>lastTransitionTime</b></td>
<td>string</td>
<td>lastTransitionTime is the last time the condition transitioned from one status to another.<br/>This should be when the underlying condition changed. If that is not known, then using the time when the API field changed is acceptable.<br/><i>Format</i>: date-time</td>
<td>true</td>
</tr><tr>
<td><b>message</b></td>
<td>string</td>
<td>message is a human readable message indicating details about the transition. This may be an empty string.<br/></td>
<td>true</td>
</tr><tr>
<td><b>reason</b></td>
<td>string</td>
<td>reason contains a programmatic identifier indicating the reason for the condition's last transition. Producers of specific condition types may define expected values and meanings for this field, and whether the values are considered a guaranteed API. The value should be a CamelCase string. This field may not be empty.<br/></td>
<td>true</td>
</tr><tr>
<td><b>status</b></td><td>enum</td><td>status of the condition, one of True, False, Unknown.<br/><i>Enum</i>: True, False, Unknown</td><td>true</td>
</tr><tr>
<td><b>type</b></td><td>string</td><td>type of condition in CamelCase or in foo.example.com/CamelCase.<br/></td><td>true</td>
</tr><tr>
<td><b>observedGeneration</b></td><td>integer</td><td>observedGeneration represents the .metadata.generation that the condition was set based upon. For instance, if .metadata.generation is currently 12, but the .status.conditions[x].observedGeneration is 9, the condition is out of date with respect to the current state of the instance.<br/><i>Format</i>: int64<br/><i>Minimum</i>: 0<br/></td><td>false</td>
</tr></tbody>
</table>

---

## Vendor
<sup><sup>[↩ Parent](#resourcemanagermiloapiscomv1alpha1)</sup></sup>

Vendor is the Schema for the Vendors API.

<table>
<thead>
<tr><th>Name</th><th>Type</th><th>Description</th><th>Required</th></tr>
</thead>
<tbody>
<tr><td><b>apiVersion</b></td><td>string</td><td>resourcemanager.miloapis.com/v1alpha1</td><td>true</td></tr>
<tr><td><b>kind</b></td><td>string</td><td>Vendor</td><td>true</td></tr>
<tr><td><b>metadata</b></td><td>object</td><td>Kubernetes standard object metadata.</td><td>true</td></tr>
<tr><td><b>spec</b></td><td>object</td><td>VendorSpec defines the desired state of Vendor.</td><td>true</td></tr>
<tr><td><b>status</b></td><td>object</td><td>VendorStatus defines the observed state of Vendor.</td><td>false</td></tr>
</tbody>
</table>

### Vendor.spec

<table>
<thead>
<tr><th>Name</th><th>Type</th><th>Description</th><th>Required</th></tr>
</thead>
<tbody>
<tr><td><b>profileType</b></td><td>string (enum: person, business)</td><td>Profile type - person or business</td><td>true</td></tr>
<tr><td><b>legalName</b></td><td>string</td><td>Legal name of the vendor</td><td>true</td></tr>
<tr><td><b>nickname</b></td><td>string</td><td>Nickname or display name</td><td>false</td></tr>
<tr><td><b>billingAddress</b></td><td>object</td><td>Billing address (fields: street, street2, city, state, postalCode, country)</td><td>true</td></tr>
<tr><td><b>mailingAddress</b></td><td>object</td><td>Mailing address (same fields as billing) if different from billing</td><td>false</td></tr>
<tr><td><b>description</b></td><td>string</td><td>Description of the vendor</td><td>false</td></tr>
<tr><td><b>website</b></td><td>string</td><td>Website URL</td><td>false</td></tr>
<tr><td><b>status</b></td><td>string (enum: pending, active, rejected, archived)</td><td>Current status of the vendor (default: pending)</td><td>true</td></tr>
<tr><td><b>corporationType</b></td><td>string</td><td>Business-specific field for "business" profileTypes only; must match a code from CorporationTypeConfig</td><td>false</td></tr>
<tr><td><b>corporationDBA</b></td><td>string</td><td>Doing business as name</td><td>false</td></tr>
<tr><td><b>registrationNumber</b></td><td>string</td><td>Registration number</td><td>false</td></tr>
<tr><td><b>stateOfIncorporation</b></td><td>string</td><td>State of incorporation</td><td>false</td></tr>
<tr><td><b>taxInfo</b></td><td>object</td><td>Tax information (fields: taxIdType, taxId, country, taxDocument, taxVerified, verificationTimestamp)</td><td>true</td></tr>
</tbody>
</table>

### Vendor.status

<table>
<thead>
<tr><th>Name</th><th>Type</th><th>Description</th><th>Required</th></tr>
</thead>
<tbody>
<tr><td><b>observedGeneration</b></td><td>integer</td><td>Most recent generation observed for this Vendor by the controller.</td><td>false</td></tr>
<tr><td><b>conditions</b></td><td>[]object</td><td>Represents the observations of a vendor's current state. Known condition types are: "Ready"</td><td>false</td></tr>
</tbody>
</table>

### Vendor.status.conditions[index]

Refer to the [ProjectControlPlane.status.conditions[index]](#projectcontrolplanestatusconditionsindex) for the schema, as it shares the same structure.

---

## CorporationTypeConfig
<sup><sup>[↩ Parent](#resourcemanagermiloapiscomv1alpha1)</sup></sup>

CorporationTypeConfig is the Schema for the CorporationTypeConfigs API.

<table>
<thead>
<tr><th>Name</th><th>Type</th><th>Description</th><th>Required</th></tr>
</thead>
<tbody>
<tr><td><b>apiVersion</b></td><td>string</td><td>resourcemanager.miloapis.com/v1alpha1</td><td>true</td></tr>
<tr><td><b>kind</b></td><td>string</td><td>CorporationTypeConfig</td><td>true</td></tr>
<tr><td><b>metadata</b></td><td>object</td><td>Kubernetes standard object metadata.</td><td>true</td></tr>
<tr><td><b>spec</b></td><td>object</td><td>CorporationTypeConfigSpec defines the desired state of CorporationTypeConfig.</td><td>true</td></tr>
<tr><td><b>status</b></td><td>object</td><td>CorporationTypeConfigStatus defines the observed state of CorporationTypeConfig.</td><td>false</td></tr>
</tbody>
</table>

### CorporationTypeConfig.spec

<table>
<thead>
<tr><th>Name</th><th>Type</th><th>Description</th><th>Required</th></tr>
</thead>
<tbody>
<tr><td><b>active</b></td><td>boolean (default: true)</td><td>Whether this configuration is active</td><td>true</td></tr>
<tr><td><b>corporationTypes</b></td><td>[]object</td><td>Available corporation types that can be selected for vendors</td><td>true</td></tr>
</tbody>
</table>

### CorporationTypeConfig.spec.corporationTypes[index]

<table>
<thead>
<tr><th>Name</th><th>Type</th><th>Description</th><th>Required</th></tr>
</thead>
<tbody>
<tr><td><b>code</b></td><td>string</td><td>The unique identifier for this corporation type (pattern: ^[a-z0-9-]+$)</td><td>true</td></tr>
<tr><td><b>displayName</b></td><td>string</td><td>Human-readable display name</td><td>true</td></tr>
<tr><td><b>description</b></td><td>string</td><td>Optional description of this corporation type</td><td>false</td></tr>
<tr><td><b>enabled</b></td><td>boolean (default: true)</td><td>Whether this corporation type is currently available for selection</td><td>true</td></tr>
<tr><td><b>sortOrder</b></td><td>integer (default: 100)</td><td>Sort order for display purposes (lower numbers appear first)</td><td>true</td></tr>
</tbody>
</table>

### CorporationTypeConfig.status

<table>
<thead>
<tr><th>Name</th><th>Type</th><th>Description</th><th>Required</th></tr>
</thead>
<tbody>
<tr><td><b>observedGeneration</b></td><td>integer</td><td>Most recent generation observed for this CorporationTypeConfig by the controller.</td><td>false</td></tr>
<tr><td><b>activeTypeCount</b></td><td>integer</td><td>Number of active corporation types.</td><td>false</td></tr>
<tr><td><b>conditions</b></td><td>[]object</td><td>Represents the observations of a corporation type config's current state. Known condition types are: "Ready"</td><td>false</td></tr>
</tbody>
</table>

### CorporationTypeConfig.status.conditions[index]

Refer to the [ProjectControlPlane.status.conditions[index]](#projectcontrolplanestatusconditionsindex) for the schema, as it shares the same structure.

---
