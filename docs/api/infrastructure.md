# API Reference

Packages:

- [infrastructure.miloapis.com/v1alpha1](#infrastructuremiloapiscomv1alpha1)
- [resourcemanager.miloapis.com/v1alpha1](#resourcemanagermiloapiscomv1alpha1)

# infrastructure.miloapis.com/v1alpha1

Resource Types:

- [ProjectControlPlane](#projectcontrolplane)

# resourcemanager.miloapis.com/v1alpha1

Resource Types:


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

  