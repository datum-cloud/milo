# API Reference

Packages:

- [notification.miloapis.com/v1alpha1](#notificationmiloapiscomv1alpha1)

# notification.miloapis.com/v1alpha1

Resource Types:

- [Email](#email)

- [EmailTemplate](#emailtemplate)




## Email
<sup><sup>[↩ Parent](#notificationmiloapiscomv1alpha1 )</sup></sup>






Email is the Schema for the emails API.
It represents a concrete e-mail that should be sent to the referenced users.
For idempotency purposes, controllers can use metadata.uid as a unique identifier
to prevent duplicate email delivery, since it's guaranteed to be unique per resource instance.

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
      <td>Email</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectmeta-v1-meta">metadata</a></b></td>
      <td>object</td>
      <td>Refer to the Kubernetes API documentation for the fields of the `metadata` field.</td>
      <td>true</td>
      </tr><tr>
        <td><b><a href="#emailspec">spec</a></b></td>
        <td>object</td>
        <td>
          EmailSpec defines the desired state of Email.
It references a template, recipients, and any variables required to render the final message.<br/>
          <br/>
            <i>Validations</i>:<li>has(self.emailAddress) != has(self.userRef): exactly one of emailAddress or userRef must be provided</li>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#emailstatus">status</a></b></td>
        <td>object</td>
        <td>
          EmailStatus captures the observed state of an Email.
Uses standard Kubernetes conditions to track both processing and delivery state.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Email.spec
<sup><sup>[↩ Parent](#email)</sup></sup>



EmailSpec defines the desired state of Email.
It references a template, recipients, and any variables required to render the final message.

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
        <td><b><a href="#emailspectemplateref">templateRef</a></b></td>
        <td>object</td>
        <td>
          TemplateRef references the EmailTemplate that should be rendered.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>bcc</b></td>
        <td>[]string</td>
        <td>
          BCC contains e-mail addresses that will receive a blind-carbon copy of the message.
Maximum 10 addresses.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>cc</b></td>
        <td>[]string</td>
        <td>
          CC contains additional e-mail addresses that will receive a carbon copy of the message.
Maximum 10 addresses.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>emailAddress</b></td>
        <td>string</td>
        <td>
          EmailAddress allows specifying a literal e-mail address for the recipient instead of referencing a User resource.
It is mutually exclusive with UserRef: exactly one of them must be specified.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>priority</b></td>
        <td>enum</td>
        <td>
          Priority influences the order in which pending e-mails are processed.<br/>
          <br/>
            <i>Enum</i>: low, normal, high<br/>
            <i>Default</i>: normal<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#emailspecuserref">userRef</a></b></td>
        <td>object</td>
        <td>
          UserRef references the User resource that will receive the message.
It is mutually exclusive with EmailAddress: exactly one of them must be specified.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#emailspecvariablesindex">variables</a></b></td>
        <td>[]object</td>
        <td>
          Variables supplies the values that will be substituted in the template.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Email.spec.templateRef
<sup><sup>[↩ Parent](#emailspec)</sup></sup>



TemplateRef references the EmailTemplate that should be rendered.

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
          Name is the name of the EmailTemplate being referenced.<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Email.spec.userRef
<sup><sup>[↩ Parent](#emailspec)</sup></sup>



UserRef references the User resource that will receive the message.
It is mutually exclusive with EmailAddress: exactly one of them must be specified.

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
          Name contain the name of the User resource that will receive the email.<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Email.spec.variables[index]
<sup><sup>[↩ Parent](#emailspec)</sup></sup>



EmailVariable represents a name/value pair that will be injected into the template.

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
          Name of the variable as declared in the associated EmailTemplate.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>value</b></td>
        <td>string</td>
        <td>
          Value provided for this variable.<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Email.status
<sup><sup>[↩ Parent](#email)</sup></sup>



EmailStatus captures the observed state of an Email.
Uses standard Kubernetes conditions to track both processing and delivery state.

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
        <td><b><a href="#emailstatusconditionsindex">conditions</a></b></td>
        <td>[]object</td>
        <td>
          Conditions represent the latest available observations of an object's current state.
Standard condition is "Delivered" which tracks email delivery status.<br/>
          <br/>
            <i>Default</i>: [map[lastTransitionTime:1970-01-01T00:00:00Z message:Waiting for email delivery reason:DeliveryPending status:Unknown type:Delivered]]<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>providerID</b></td>
        <td>string</td>
        <td>
          ProviderID is the identifier returned by the underlying email provider
(e.g. Resend) when the e-mail is accepted for delivery. It is usually
used to track the email delivery status (e.g. provider webhooks).<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Email.status.conditions[index]
<sup><sup>[↩ Parent](#emailstatus)</sup></sup>



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

## EmailTemplate
<sup><sup>[↩ Parent](#notificationmiloapiscomv1alpha1 )</sup></sup>






EmailTemplate is the Schema for the email templates API.
It represents a reusable e-mail template that can be rendered by substituting
the declared variables.

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
      <td>EmailTemplate</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectmeta-v1-meta">metadata</a></b></td>
      <td>object</td>
      <td>Refer to the Kubernetes API documentation for the fields of the `metadata` field.</td>
      <td>true</td>
      </tr><tr>
        <td><b><a href="#emailtemplatespec">spec</a></b></td>
        <td>object</td>
        <td>
          EmailTemplateSpec defines the desired state of EmailTemplate.
It contains the subject, content, and declared variables.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#emailtemplatestatus">status</a></b></td>
        <td>object</td>
        <td>
          EmailTemplateStatus captures the observed state of an EmailTemplate.
Right now we only expose standard Kubernetes conditions so callers can
determine whether the template is ready for use.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### EmailTemplate.spec
<sup><sup>[↩ Parent](#emailtemplate)</sup></sup>



EmailTemplateSpec defines the desired state of EmailTemplate.
It contains the subject, content, and declared variables.

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
        <td><b>htmlBody</b></td>
        <td>string</td>
        <td>
          HTMLBody is the string for the HTML representation of the message.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>subject</b></td>
        <td>string</td>
        <td>
          Subject is the string that composes the email subject line.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>textBody</b></td>
        <td>string</td>
        <td>
          TextBody is the Go template string for the plain-text representation of the message.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#emailtemplatespecvariablesindex">variables</a></b></td>
        <td>[]object</td>
        <td>
          Variables enumerates all variables that can be referenced inside the template expressions.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### EmailTemplate.spec.variables[index]
<sup><sup>[↩ Parent](#emailtemplatespec)</sup></sup>



TemplateVariable declares a variable that can be referenced in the template body or subject.
Each variable must be listed here so that callers know which parameters are expected.

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
          Name is the identifier of the variable as it appears inside the Go template (e.g. {{.UserName}}).<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>required</b></td>
        <td>boolean</td>
        <td>
          Required indicates whether the variable must be provided when rendering the template.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>type</b></td>
        <td>enum</td>
        <td>
          Type provides a hint about the expected value of this variable (e.g. plain string or URL).<br/>
          <br/>
            <i>Enum</i>: string, url<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### EmailTemplate.status
<sup><sup>[↩ Parent](#emailtemplate)</sup></sup>



EmailTemplateStatus captures the observed state of an EmailTemplate.
Right now we only expose standard Kubernetes conditions so callers can
determine whether the template is ready for use.

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
        <td><b><a href="#emailtemplatestatusconditionsindex">conditions</a></b></td>
        <td>[]object</td>
        <td>
          Conditions represent the latest available observations of an object's current state.<br/>
          <br/>
            <i>Default</i>: [map[lastTransitionTime:1970-01-01T00:00:00Z message:Waiting for control plane to reconcile reason:Unknown status:Unknown type:Ready]]<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### EmailTemplate.status.conditions[index]
<sup><sup>[↩ Parent](#emailtemplatestatus)</sup></sup>



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
