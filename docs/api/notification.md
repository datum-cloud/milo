## Note
<sup><sup>[↩ Parent](#notificationmiloapiscomv1alpha1 )</sup></sup>

> **Note:** This documentation section describes the Note resource for `notification.miloapis.com/v1alpha1`, which is namespace-scoped and attaches notes to a Contact. A separate Note resource now exists at `crm.miloapis.com/v1alpha1`, which is cluster-scoped and supports both Contacts and Users as subjects; see the CRM API documentation for details on that resource.


Note is the Schema for the notes API.
It represents a note attached to a Contact.

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
      <td>notification.miloapis.com/v1alpha1</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b>kind</b></td>
      <td>string</td>
      <td>Note</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectmeta-v1-meta">metadata</a></b></td>
      <td>object</td>
      <td>Refer to the Kubernetes API documentation for the fields of the `metadata` field.</td>
      <td>true</td>
      </tr><tr>
        <td><b><a href="#notespec">spec</a></b></td>
        <td>object</td>
        <td>
          NoteSpec defines the desired state of Note.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#notestatus">status</a></b></td>
        <td>object</td>
        <td>
          NoteStatus defines the observed state of Note.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Note.spec
<sup><sup>[↩ Parent](#note)</sup></sup>



NoteSpec defines the desired state of Note.

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
        <td><b>contactRef</b></td>
        <td>string</td>
        <td>
          ContactRef is the name of the Contact this note is attached to.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>content</b></td>
        <td>string</td>
        <td>
          Content is the text content of the note.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>action</b></td>
        <td>string</td>
        <td>
          Action is an optional follow-up action.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>actionTime</b></td>
        <td>string</td>
        <td>
          ActionTime is the timestamp for the follow-up action.<br/>
          <br/>
            <i>Format</i>: date-time<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>interactionTime</b></td>
        <td>string</td>
        <td>
          InteractionTime is the timestamp of the interaction.
If not specified, it defaults to the creation timestamp of the note.<br/>
          <br/>
            <i>Format</i>: date-time<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Note.status
<sup><sup>[↩ Parent](#note)</sup></sup>



NoteStatus defines the observed state of Note.

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
        <td><b><a href="#notestatusconditionsindex">conditions</a></b></td>
        <td>[]object</td>
        <td>
          Conditions represent the latest available observations of an object's state<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Note.status.conditions[index]
<sup><sup>[↩ Parent](#notestatus)</sup></sup>



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