# API Reference

Packages:

- [documentation.miloapis.com/v1alpha1](#documentationmiloapiscomv1alpha1)

# documentation.miloapis.com/v1alpha1

Resource Types:

- [DocumentRevision](#documentrevision)

- [Document](#document)




## DocumentRevision
<sup><sup>[↩ Parent](#documentationmiloapiscomv1alpha1 )</sup></sup>






DocumentRevision is the Schema for the documentrevisions API.
It represents a revision of a document.

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
      <td>documentation.miloapis.com/v1alpha1</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b>kind</b></td>
      <td>string</td>
      <td>DocumentRevision</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectmeta-v1-meta">metadata</a></b></td>
      <td>object</td>
      <td>Refer to the Kubernetes API documentation for the fields of the `metadata` field.</td>
      <td>true</td>
      </tr><tr>
        <td><b><a href="#documentrevisionspec">spec</a></b></td>
        <td>object</td>
        <td>
          DocumentRevisionSpec defines the desired state of DocumentRevision.<br/>
          <br/>
            <i>Validations</i>:<li>self == oldSelf: spec is immutable</li>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#documentrevisionstatus">status</a></b></td>
        <td>object</td>
        <td>
          DocumentRevisionStatus defines the observed state of DocumentRevision.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### DocumentRevision.spec
<sup><sup>[↩ Parent](#documentrevision)</sup></sup>



DocumentRevisionSpec defines the desired state of DocumentRevision.

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
        <td><b>changesSummary</b></td>
        <td>string</td>
        <td>
          ChangesSummary is the summary of the changes in the document revision.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#documentrevisionspeccontent">content</a></b></td>
        <td>object</td>
        <td>
          Content is the content of the document revision.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#documentrevisionspecdocumentref">documentRef</a></b></td>
        <td>object</td>
        <td>
          DocumentRef is a reference to the document that this revision is based on.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>effectiveDate</b></td>
        <td>string</td>
        <td>
          EffectiveDate is the date in which the document revision starts to be effective.<br/>
          <br/>
            <i>Format</i>: date-time<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#documentrevisionspecexpectedaccepterkindsindex">expectedAccepterKinds</a></b></td>
        <td>[]object</td>
        <td>
          ExpectedAccepterKinds is the resource kinds that are expected to accept this revision.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#documentrevisionspecexpectedsubjectkindsindex">expectedSubjectKinds</a></b></td>
        <td>[]object</td>
        <td>
          ExpectedSubjectKinds is the resource kinds that this revision affects to.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>version</b></td>
        <td>string</td>
        <td>
          Version is the version of the document revision.<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### DocumentRevision.spec.content
<sup><sup>[↩ Parent](#documentrevisionspec)</sup></sup>



Content is the content of the document revision.

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
        <td><b>data</b></td>
        <td>string</td>
        <td>
          Data is the data of the document revision.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>format</b></td>
        <td>enum</td>
        <td>
          Format is the format of the document revision.<br/>
          <br/>
            <i>Enum</i>: html, markdown<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### DocumentRevision.spec.documentRef
<sup><sup>[↩ Parent](#documentrevisionspec)</sup></sup>



DocumentRef is a reference to the document that this revision is based on.

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
          Name is the name of the Document being referenced.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          Namespace of the referenced Document.<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### DocumentRevision.spec.expectedAccepterKinds[index]
<sup><sup>[↩ Parent](#documentrevisionspec)</sup></sup>



DocumentRevisionExpectedAccepterKind is the kind of the resource that is expected to accept this revision.

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
          <br/>
            <i>Validations</i>:<li>self == 'iam.miloapis.com': apiGroup must be iam.miloapis.com</li>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>kind</b></td>
        <td>enum</td>
        <td>
          Kind is the type of resource being referenced.<br/>
          <br/>
            <i>Enum</i>: User, MachineAccount<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### DocumentRevision.spec.expectedSubjectKinds[index]
<sup><sup>[↩ Parent](#documentrevisionspec)</sup></sup>



DocumentRevisionExpectedSubjectKind is the kind of the resource that is expected to reference this revision.

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
      </tr></tbody>
</table>


### DocumentRevision.status
<sup><sup>[↩ Parent](#documentrevision)</sup></sup>



DocumentRevisionStatus defines the observed state of DocumentRevision.

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
        <td><b><a href="#documentrevisionstatusconditionsindex">conditions</a></b></td>
        <td>[]object</td>
        <td>
          Conditions represent the latest available observations of an object's current state.<br/>
          <br/>
            <i>Default</i>: [map[lastTransitionTime:1970-01-01T00:00:00Z message:Waiting for control plane to reconcile reason:Unknown status:Unknown type:Ready]]<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>contentHash</b></td>
        <td>string</td>
        <td>
          ContentHash is the hash of the content of the document revision.
This is used to detect if the content of the document revision has changed.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### DocumentRevision.status.conditions[index]
<sup><sup>[↩ Parent](#documentrevisionstatus)</sup></sup>



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

## Document
<sup><sup>[↩ Parent](#documentationmiloapiscomv1alpha1 )</sup></sup>






Document is the Schema for the documents API.
It represents a document that can be used to create a document revision.

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
      <td>documentation.miloapis.com/v1alpha1</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b>kind</b></td>
      <td>string</td>
      <td>Document</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectmeta-v1-meta">metadata</a></b></td>
      <td>object</td>
      <td>Refer to the Kubernetes API documentation for the fields of the `metadata` field.</td>
      <td>true</td>
      </tr><tr>
        <td><b><a href="#documentdocumentmetadata">documentMetadata</a></b></td>
        <td>object</td>
        <td>
          DocumentMetadata defines the metadata of the Document.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#documentspec">spec</a></b></td>
        <td>object</td>
        <td>
          DocumentSpec defines the desired state of Document.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#documentstatus">status</a></b></td>
        <td>object</td>
        <td>
          DocumentStatus defines the observed state of Document.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Document.documentMetadata
<sup><sup>[↩ Parent](#document)</sup></sup>



DocumentMetadata defines the metadata of the Document.

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
        <td>string</td>
        <td>
          Category is the category of the Document.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>jurisdiction</b></td>
        <td>string</td>
        <td>
          Jurisdiction is the jurisdiction of the Document.<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Document.spec
<sup><sup>[↩ Parent](#document)</sup></sup>



DocumentSpec defines the desired state of Document.

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
        <td><b>description</b></td>
        <td>string</td>
        <td>
          Description is the description of the Document.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>documentType</b></td>
        <td>string</td>
        <td>
          DocumentType is the type of the document.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>title</b></td>
        <td>string</td>
        <td>
          Title is the title of the Document.<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Document.status
<sup><sup>[↩ Parent](#document)</sup></sup>



DocumentStatus defines the observed state of Document.

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
        <td><b><a href="#documentstatusconditionsindex">conditions</a></b></td>
        <td>[]object</td>
        <td>
          Conditions represent the latest available observations of an object's current state.<br/>
          <br/>
            <i>Default</i>: [map[lastTransitionTime:1970-01-01T00:00:00Z message:Waiting for control plane to reconcile reason:Unknown status:Unknown type:Ready]]<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#documentstatuslatestrevisionref">latestRevisionRef</a></b></td>
        <td>object</td>
        <td>
          LatestRevisionRef is a reference to the latest revision of the document.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Document.status.conditions[index]
<sup><sup>[↩ Parent](#documentstatus)</sup></sup>



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


### Document.status.latestRevisionRef
<sup><sup>[↩ Parent](#documentstatus)</sup></sup>



LatestRevisionRef is a reference to the latest revision of the document.

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
          <br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>publishedAt</b></td>
        <td>string</td>
        <td>
          <br/>
          <br/>
            <i>Format</i>: date-time<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>version</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>
