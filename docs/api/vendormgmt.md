# API Reference

Packages:

- [vendor.miloapis.com/v1alpha1](#vendormiloapiscomv1alpha1)

# vendor.miloapis.com/v1alpha1

Resource Types:

- [VendorProfile](#vendorprofile)




## VendorProfile
<sup><sup>[↩ Parent](#vendormiloapiscomv1alpha1 )</sup></sup>






VendorProfile is the Schema for the VendorProfiles API.
It represents a third-party vendor or sub-processor for compliance
documentation.

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
      <td>vendor.miloapis.com/v1alpha1</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b>kind</b></td>
      <td>string</td>
      <td>VendorProfile</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectmeta-v1-meta">metadata</a></b></td>
      <td>object</td>
      <td>Refer to the Kubernetes API documentation for the fields of the `metadata` field.</td>
      <td>true</td>
      </tr><tr>
        <td><b><a href="#vendorprofilespec">spec</a></b></td>
        <td>object</td>
        <td>
          VendorProfileSpec defines the desired state of VendorProfile.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#vendorprofilestatus">status</a></b></td>
        <td>object</td>
        <td>
          VendorProfileStatus defines the observed state of VendorProfile.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### VendorProfile.spec
<sup><sup>[↩ Parent](#vendorprofile)</sup></sup>



VendorProfileSpec defines the desired state of VendorProfile.

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
        <td><b>category</b></td>
        <td>enum</td>
        <td>
          Type of service the vendor provides.<br/>
          <br/>
            <i>Enum</i>: Infrastructure, Security, Analytics, Authentication, DataProcessing, Communication, Monitoring, Other<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>description</b></td>
        <td>string</td>
        <td>
          Markdown-formatted description of the vendor and its role as a
sub-processor.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>displayName</b></td>
        <td>string</td>
        <td>
          Human-readable vendor name (e.g., "Amazon Web Services").<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>purpose</b></td>
        <td>string</td>
        <td>
          Describes what data this vendor processes and why.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>contactEmail</b></td>
        <td>string</td>
        <td>
          Vendor contact email address.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>dataProcessingLocations</b></td>
        <td>[]string</td>
        <td>
          Countries or regions where the vendor processes data (e.g., "US", "EU").<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>logoURL</b></td>
        <td>string</td>
        <td>
          URL to the vendor's logo image.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>privacyPolicyURL</b></td>
        <td>string</td>
        <td>
          Link to the vendor's privacy policy.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>websiteURL</b></td>
        <td>string</td>
        <td>
          Vendor's primary website.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### VendorProfile.status
<sup><sup>[↩ Parent](#vendorprofile)</sup></sup>



VendorProfileStatus defines the observed state of VendorProfile.

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
        <td><b><a href="#vendorprofilestatusconditionsindex">conditions</a></b></td>
        <td>[]object</td>
        <td>
          Conditions represents the observations of a vendor profile's current state.
Known condition types are: "Ready"<br/>
          <br/>
            <i>Default</i>: [map[lastTransitionTime:1970-01-01T00:00:00Z message:Waiting for control plane to reconcile reason:Unknown status:Unknown type:Ready]]<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>observedGeneration</b></td>
        <td>integer</td>
        <td>
          ObservedGeneration is the most recent generation observed for this
VendorProfile by the controller.<br/>
          <br/>
            <i>Format</i>: int64<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### VendorProfile.status.conditions[index]
<sup><sup>[↩ Parent](#vendorprofilestatus)</sup></sup>



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
