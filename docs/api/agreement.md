# API Reference

Packages:

- [agreement.miloapis.com/v1alpha1](#agreementmiloapiscomv1alpha1)

# agreement.miloapis.com/v1alpha1

Resource Types:

- [DocumentAcceptance](#documentacceptance)




## DocumentAcceptance
<sup><sup>[↩ Parent](#agreementmiloapiscomv1alpha1 )</sup></sup>






DocumentAcceptance is the Schema for the documentacceptances API.
It represents a document acceptance.

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
      <td>agreement.miloapis.com/v1alpha1</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b>kind</b></td>
      <td>string</td>
      <td>DocumentAcceptance</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectmeta-v1-meta">metadata</a></b></td>
      <td>object</td>
      <td>Refer to the Kubernetes API documentation for the fields of the `metadata` field.</td>
      <td>true</td>
      </tr><tr>
        <td><b><a href="#documentacceptancespec">spec</a></b></td>
        <td>object</td>
        <td>
          DocumentAcceptanceSpec defines the desired state of DocumentAcceptance.<br/>
          <br/>
            <i>Validations</i>:<li>self == oldSelf: spec is immutable</li>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#documentacceptancestatus">status</a></b></td>
        <td>object</td>
        <td>
          DocumentAcceptanceStatus defines the observed state of DocumentAcceptance.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### DocumentAcceptance.spec
<sup><sup>[↩ Parent](#documentacceptance)</sup></sup>



DocumentAcceptanceSpec defines the desired state of DocumentAcceptance.

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
        <td><b><a href="#documentacceptancespecacceptancecontext">acceptanceContext</a></b></td>
        <td>object</td>
        <td>
          AcceptanceContext is the context of the document acceptance.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#documentacceptancespecaccepterref">accepterRef</a></b></td>
        <td>object</td>
        <td>
          AccepterRef is a reference to the accepter that this document acceptance applies to.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#documentacceptancespecdocumentrevisionref">documentRevisionRef</a></b></td>
        <td>object</td>
        <td>
          DocumentRevisionRef is a reference to the document revision that is being accepted.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#documentacceptancespecsignature">signature</a></b></td>
        <td>object</td>
        <td>
          Signature is the signature of the document acceptance.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#documentacceptancespecsubjectref">subjectRef</a></b></td>
        <td>object</td>
        <td>
          SubjectRef is a reference to the subject that this document acceptance applies to.<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### DocumentAcceptance.spec.acceptanceContext
<sup><sup>[↩ Parent](#documentacceptancespec)</sup></sup>



AcceptanceContext is the context of the document acceptance.

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
        <td><b>method</b></td>
        <td>enum</td>
        <td>
          Method is the method of the document acceptance.<br/>
          <br/>
            <i>Enum</i>: web, email, cli<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>acceptanceLanguage</b></td>
        <td>string</td>
        <td>
          AcceptanceLanguage is the language of the document acceptance.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>ipAddress</b></td>
        <td>string</td>
        <td>
          IPAddress is the IP address of the accepter.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>userAgent</b></td>
        <td>string</td>
        <td>
          UserAgent is the user agent of the accepter.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### DocumentAcceptance.spec.accepterRef
<sup><sup>[↩ Parent](#documentacceptancespec)</sup></sup>



AccepterRef is a reference to the accepter that this document acceptance applies to.

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
        <td><b>apiGroup</b></td>
        <td>string</td>
        <td>
          APIGroup is the group for the resource being referenced.<br/>
        </td>
        <td>true</td>
      </tr><tr>
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
          Name is the name of the Resource being referenced.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          Namespace is the namespace of the Resource being referenced.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### DocumentAcceptance.spec.documentRevisionRef
<sup><sup>[↩ Parent](#documentacceptancespec)</sup></sup>



DocumentRevisionRef is a reference to the document revision that is being accepted.

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
          Name is the name of the DocumentRevision being referenced.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          Namespace of the referenced document revision.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>version</b></td>
        <td>string</td>
        <td>
          Version is the version of the DocumentRevision being referenced.<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### DocumentAcceptance.spec.signature
<sup><sup>[↩ Parent](#documentacceptancespec)</sup></sup>



Signature is the signature of the document acceptance.

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
        <td><b>timestamp</b></td>
        <td>string</td>
        <td>
          Timestamp is the timestamp of the document acceptance.<br/>
          <br/>
            <i>Format</i>: date-time<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>type</b></td>
        <td>enum</td>
        <td>
          Type specifies the signature mechanism used for the document acceptance.<br/>
          <br/>
            <i>Enum</i>: checkbox<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### DocumentAcceptance.spec.subjectRef
<sup><sup>[↩ Parent](#documentacceptancespec)</sup></sup>



SubjectRef is a reference to the subject that this document acceptance applies to.

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
        <td><b>apiGroup</b></td>
        <td>string</td>
        <td>
          APIGroup is the group for the resource being referenced.<br/>
        </td>
        <td>true</td>
      </tr><tr>
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
          Name is the name of the Resource being referenced.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          Namespace is the namespace of the Resource being referenced.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### DocumentAcceptance.status
<sup><sup>[↩ Parent](#documentacceptance)</sup></sup>



DocumentAcceptanceStatus defines the observed state of DocumentAcceptance.

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
        <td><b><a href="#documentacceptancestatusconditionsindex">conditions</a></b></td>
        <td>[]object</td>
        <td>
          Conditions represent the latest available observations of an object's current state.<br/>
          <br/>
            <i>Default</i>: [map[lastTransitionTime:1970-01-01T00:00:00Z message:Waiting for control plane to reconcile reason:Unknown status:Unknown type:Ready]]<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### DocumentAcceptance.status.conditions[index]
<sup><sup>[↩ Parent](#documentacceptancestatus)</sup></sup>



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
