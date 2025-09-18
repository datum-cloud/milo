# API Reference

Packages:

- [quota.miloapis.com/v1alpha1](#quotamiloapiscomv1alpha1)

# quota.miloapis.com/v1alpha1

Resource Types:

- [AllowanceBucket](#allowancebucket)

- [ClaimCreationPolicy](#claimcreationpolicy)

- [GrantCreationPolicy](#grantcreationpolicy)

- [ResourceClaim](#resourceclaim)

- [ResourceGrant](#resourcegrant)

- [ResourceRegistration](#resourceregistration)




## AllowanceBucket
<sup><sup>[↩ Parent](#quotamiloapiscomv1alpha1 )</sup></sup>






AllowanceBucket tracks the effective quota for a single consumer and resource type.
The system aggregates capacity from ResourceGrants and consumption from ResourceClaims
to support real-time admission decisions.

### How It Works
- Scope: One bucket per (`consumerRef`, `resourceType`) pair.
- Inputs: Active `ResourceGrant`s increase `status.limit`; granted `ResourceClaim`s increase `status.allocated`.
- Decision: Admission grants a claim only when `status.available >= requested amount`.
- Scale: Status stores aggregates, not per-claim entries, to keep object size bounded.

### Works With
- Aggregates active [ResourceGrant](#resourcegrant) amounts into `status.limit` for the matching (`spec.consumerRef`, `spec.resourceType`).
- Aggregates granted [ResourceClaim](#resourceclaim) amounts into `status.allocated`.
- Used by admission decisions: a claim is granted only if `status.available >= requested amount`.
- Labeled by the controller to simplify queries by consumer and resource kind.

### Selectors and Filtering
- Field selectors (server-side): `spec.consumerRef.kind`, `spec.consumerRef.name`, `spec.resourceType`.
- Built-in labels (set by controller):
  - `quota.miloapis.com/resource-kind`
  - `quota.miloapis.com/resource-apigroup` (omitted for core kinds)
  - `quota.miloapis.com/consumer-kind`
  - `quota.miloapis.com/consumer-name`

- Common queries:
  - All buckets for a consumer: label selector `quota.miloapis.com/consumer-kind` + `quota.miloapis.com/consumer-name`.
  - All buckets for a resource kind: label selector `quota.miloapis.com/resource-kind` (and `quota.miloapis.com/resource-apigroup` if needed).
  - Buckets for a resourceType: field selector `spec.resourceType`.

### Notes
- A dedicated controller is the single writer for status to avoid races.
- Aggregates may lag briefly after grant/claim updates (eventual consistency).
- `status.available` never goes negative.

### See Also
- [ResourceGrant](#resourcegrant): Supplies capacity that increases `status.limit`.
- [ResourceClaim](#resourceclaim): Consumes capacity that increases `status.allocated`.
- [ClaimCreationPolicy](#claimcreationpolicy): Drives creation of claims during admission.

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
      <td>quota.miloapis.com/v1alpha1</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b>kind</b></td>
      <td>string</td>
      <td>AllowanceBucket</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectmeta-v1-meta">metadata</a></b></td>
      <td>object</td>
      <td>Refer to the Kubernetes API documentation for the fields of the `metadata` field.</td>
      <td>true</td>
      </tr><tr>
        <td><b><a href="#allowancebucketspec">spec</a></b></td>
        <td>object</td>
        <td>
          AllowanceBucketSpec defines the desired state of AllowanceBucket.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#allowancebucketstatus">status</a></b></td>
        <td>object</td>
        <td>
          AllowanceBucketStatus is the controller‑computed snapshot for a single
(`spec.consumerRef`, `spec.resourceType`). The controller aggregates capacity
from Active [ResourceGrant](#resourcegrant)s and usage from Granted
[ResourceClaim](#resourceclaim)s, then derives availability as capacity minus
usage (never negative). It also records provenance for how capacity was
composed, simple cardinalities to aid troubleshooting at scale, and a
reconciliation timestamp. Values may lag briefly after underlying grants or
claims change. See the schema for exact field names and constraints.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### AllowanceBucket.spec
<sup><sup>[↩ Parent](#allowancebucket)</sup></sup>



AllowanceBucketSpec defines the desired state of AllowanceBucket.

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
        <td><b><a href="#allowancebucketspecconsumerref">consumerRef</a></b></td>
        <td>object</td>
        <td>
          ConsumerRef identifies the quota consumer this bucket tracks<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>resourceType</b></td>
        <td>string</td>
        <td>
          ResourceType specifies which resource type this bucket tracks.
Must match a registered resource type from ResourceRegistration.<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### AllowanceBucket.spec.consumerRef
<sup><sup>[↩ Parent](#allowancebucketspec)</sup></sup>



ConsumerRef identifies the quota consumer this bucket tracks

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
        <td><b>kind</b></td>
        <td>string</td>
        <td>
          Kind of the consumer resource (for example, Organization, Project).<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name of the consumer resource object instance.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>apiGroup</b></td>
        <td>string</td>
        <td>
          APIGroup of the target resource (e.g., "resourcemanager.miloapis.com").
Empty string for core API group.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### AllowanceBucket.status
<sup><sup>[↩ Parent](#allowancebucket)</sup></sup>



AllowanceBucketStatus is the controller‑computed snapshot for a single
(`spec.consumerRef`, `spec.resourceType`). The controller aggregates capacity
from Active [ResourceGrant](#resourcegrant)s and usage from Granted
[ResourceClaim](#resourceclaim)s, then derives availability as capacity minus
usage (never negative). It also records provenance for how capacity was
composed, simple cardinalities to aid troubleshooting at scale, and a
reconciliation timestamp. Values may lag briefly after underlying grants or
claims change. See the schema for exact field names and constraints.

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
        <td><b>allocated</b></td>
        <td>integer</td>
        <td>
          Amount of quota currently allocated/used in this bucket, measured in the
BaseUnit defined by the ResourceRegistration.<br/>
          <br/>
            <i>Format</i>: int64<br/>
            <i>Minimum</i>: 0<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>available</b></td>
        <td>integer</td>
        <td>
          Amount available to be claimed (limit - allocated), measured in the
BaseUnit defined by the ResourceRegistration.<br/>
          <br/>
            <i>Format</i>: int64<br/>
            <i>Minimum</i>: 0<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>claimCount</b></td>
        <td>integer</td>
        <td>
          Count of claims consuming quota from this bucket<br/>
          <br/>
            <i>Format</i>: int32<br/>
            <i>Minimum</i>: 0<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>grantCount</b></td>
        <td>integer</td>
        <td>
          Count of grants contributing to this bucket's limit<br/>
          <br/>
            <i>Format</i>: int32<br/>
            <i>Minimum</i>: 0<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>limit</b></td>
        <td>integer</td>
        <td>
          Total quota limit from all applicable ResourceGrants, measured in the
BaseUnit defined by the ResourceRegistration for this resource type.<br/>
          <br/>
            <i>Format</i>: int64<br/>
            <i>Minimum</i>: 0<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#allowancebucketstatuscontributinggrantrefsindex">contributingGrantRefs</a></b></td>
        <td>[]object</td>
        <td>
          A list of all the grants that contribute to the limit for this bucket.
Grants are tracked individually as they are typically few in number.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>lastReconciliation</b></td>
        <td>string</td>
        <td>
          Last time the bucket was reconciled<br/>
          <br/>
            <i>Format</i>: date-time<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>observedGeneration</b></td>
        <td>integer</td>
        <td>
          The specific revision of the AllowanceBucket<br/>
          <br/>
            <i>Format</i>: int64<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### AllowanceBucket.status.contributingGrantRefs[index]
<sup><sup>[↩ Parent](#allowancebucketstatus)</sup></sup>



ContributingGrantRef references a ResourceGrant that contributes to
the total limit in the bucket's status.

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
        <td><b>amount</b></td>
        <td>integer</td>
        <td>
          Amount granted<br/>
          <br/>
            <i>Format</i>: int64<br/>
            <i>Minimum</i>: 0<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>lastObservedGeneration</b></td>
        <td>integer</td>
        <td>
          The generation of the ResourceGrant when this bucket last processed it<br/>
          <br/>
            <i>Format</i>: int64<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name of the ResourceGrant<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>

## ClaimCreationPolicy
<sup><sup>[↩ Parent](#quotamiloapiscomv1alpha1 )</sup></sup>






ClaimCreationPolicy creates ResourceClaims during admission when target resources are created.
Use it to enforce quota in real time at resource creation.

### How It Works
- Admission matches incoming creates against `spec.trigger.resource`.
- It evaluates all CEL expressions in `spec.trigger.conditions[]`.
- When all conditions are true, it renders `resourceClaimTemplate` and creates a claim.
- The system evaluates the claim against [AllowanceBucket](#allowancebucket)s and grants or denies the request.

### Works With
- Creates [ResourceClaim](#resourceclaim) objects; the triggering kind must be allowed by the target [ResourceRegistration](#resourceregistration) `spec.claimingResources`.
- Consumer resolution is automatic at admission; claims are evaluated against [AllowanceBucket](#allowancebucket) capacity.
- Policy readiness (`status.conditions[type=Ready]`) indicates the policy is valid and active.

### Selectors and Filtering
- Field selectors (server-side): `spec.trigger.resource.kind`, `spec.trigger.resource.apiVersion`, `spec.enabled`.
- Label selectors (add your own):
  - `quota.miloapis.com/target-kind`: `Project`
  - `quota.miloapis.com/environment`: `prod`

- Common queries:
  - All policies for a target kind: label selector `quota.miloapis.com/target-kind`.
  - All enabled policies: field selector `spec.enabled=true`.

### Defaults and Limits
- In `v1alpha1`, `spec.requests[]` amounts are static integers (no expression-based amounts).
- `metadata.labels` in the template are literal; annotation values support templating.
- `spec.consumerRef` is resolved automatically by admission (not templated in `v1alpha1`).

### Notes
- Available template variables: `.trigger`, `.requestInfo`, `.user`.
- Template functions: `lower`, `upper`, `title`, `default`, `contains`, `join`, `split`, `replace`, `trim`, `toInt`, `toString`.
- If `Ready=False` with `ValidationFailed`, check expressions and templates for errors.
- Disabled policies (`spec.enabled=false`) do not create claims, even if conditions match.
- For task-oriented steps and examples, see future How-to guides.

### See Also
- [ResourceClaim](#resourceclaim): The object created by this policy.
- [ResourceRegistration](#resourceregistration): Controls which resources can claim quota.

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
      <td>quota.miloapis.com/v1alpha1</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b>kind</b></td>
      <td>string</td>
      <td>ClaimCreationPolicy</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectmeta-v1-meta">metadata</a></b></td>
      <td>object</td>
      <td>Refer to the Kubernetes API documentation for the fields of the `metadata` field.</td>
      <td>true</td>
      </tr><tr>
        <td><b><a href="#claimcreationpolicyspec">spec</a></b></td>
        <td>object</td>
        <td>
          ClaimCreationPolicySpec defines the desired state of ClaimCreationPolicy.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#claimcreationpolicystatus">status</a></b></td>
        <td>object</td>
        <td>
          ClaimCreationPolicyStatus defines the observed state of ClaimCreationPolicy.

Status fields
- conditions[type=Ready]: True when the policy is validated and active.

See also
- [ResourceClaim](#resourceclaim): The object created by this policy.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### ClaimCreationPolicy.spec
<sup><sup>[↩ Parent](#claimcreationpolicy)</sup></sup>



ClaimCreationPolicySpec defines the desired state of ClaimCreationPolicy.

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
        <td><b><a href="#claimcreationpolicyspectarget">target</a></b></td>
        <td>object</td>
        <td>
          Target defines how and where ResourceClaims should be created.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#claimcreationpolicyspectrigger">trigger</a></b></td>
        <td>object</td>
        <td>
          Trigger defines what resource changes should trigger claim creation.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>enabled</b></td>
        <td>boolean</td>
        <td>
          Enabled determines if this policy is active.
If false, no ResourceClaims will be created for matching resources.<br/>
          <br/>
            <i>Default</i>: true<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### ClaimCreationPolicy.spec.target
<sup><sup>[↩ Parent](#claimcreationpolicyspec)</sup></sup>



Target defines how and where ResourceClaims should be created.

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
        <td><b><a href="#claimcreationpolicyspectargetresourceclaimtemplate">resourceClaimTemplate</a></b></td>
        <td>object</td>
        <td>
          ResourceClaimTemplate defines how to create ResourceClaims.
String fields support Go template syntax for dynamic content.<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### ClaimCreationPolicy.spec.target.resourceClaimTemplate
<sup><sup>[↩ Parent](#claimcreationpolicyspectarget)</sup></sup>



ResourceClaimTemplate defines how to create ResourceClaims.
String fields support Go template syntax for dynamic content.

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
        <td><b><a href="#claimcreationpolicyspectargetresourceclaimtemplatemetadata">metadata</a></b></td>
        <td>object</td>
        <td>
          Metadata for the created ResourceClaim.
String fields support Go template syntax.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#claimcreationpolicyspectargetresourceclaimtemplatespec">spec</a></b></td>
        <td>object</td>
        <td>
          Spec for the created ResourceClaim.
String fields support Go template syntax.<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### ClaimCreationPolicy.spec.target.resourceClaimTemplate.metadata
<sup><sup>[↩ Parent](#claimcreationpolicyspectargetresourceclaimtemplate)</sup></sup>



Metadata for the created ResourceClaim.
String fields support Go template syntax.

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
        <td><b>annotations</b></td>
        <td>map[string]string</td>
        <td>
          Annotations to set on the created object. Values support Go templates.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>generateName</b></td>
        <td>string</td>
        <td>
          GenerateName prefix for the created object when Name is empty. Supports Go templates.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>labels</b></td>
        <td>map[string]string</td>
        <td>
          Labels to set on the created object. Literal values only.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name of the created object. Supports Go templates.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          Namespace where the object will be created. Supports Go templates.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### ClaimCreationPolicy.spec.target.resourceClaimTemplate.spec
<sup><sup>[↩ Parent](#claimcreationpolicyspectargetresourceclaimtemplate)</sup></sup>



Spec for the created ResourceClaim.
String fields support Go template syntax.

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
        <td><b><a href="#claimcreationpolicyspectargetresourceclaimtemplatespecconsumerref">consumerRef</a></b></td>
        <td>object</td>
        <td>
          ConsumerRef identifies the quota consumer (the subject that receives
limits and consumes capacity) making this claim. Examples include an
Organization or a Project, depending on how the registration is defined.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#claimcreationpolicyspectargetresourceclaimtemplatespecrequestsindex">requests</a></b></td>
        <td>[]object</td>
        <td>
          Requests specifies the resource types and amounts being claimed.
Each resource type must be unique within the requests array.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#claimcreationpolicyspectargetresourceclaimtemplatespecresourceref">resourceRef</a></b></td>
        <td>object</td>
        <td>
          ResourceRef links to the actual resource that triggered this quota claim.
Automatically populated by the admission plugin.
Uses an unversioned reference to persist across API version upgrades.<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### ClaimCreationPolicy.spec.target.resourceClaimTemplate.spec.consumerRef
<sup><sup>[↩ Parent](#claimcreationpolicyspectargetresourceclaimtemplatespec)</sup></sup>



ConsumerRef identifies the quota consumer (the subject that receives
limits and consumes capacity) making this claim. Examples include an
Organization or a Project, depending on how the registration is defined.

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
        <td><b>kind</b></td>
        <td>string</td>
        <td>
          Kind of the consumer resource (for example, Organization, Project).<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name of the consumer resource object instance.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>apiGroup</b></td>
        <td>string</td>
        <td>
          APIGroup of the target resource (e.g., "resourcemanager.miloapis.com").
Empty string for core API group.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### ClaimCreationPolicy.spec.target.resourceClaimTemplate.spec.requests[index]
<sup><sup>[↩ Parent](#claimcreationpolicyspectargetresourceclaimtemplatespec)</sup></sup>



ResourceRequest defines a single resource request within a claim

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
        <td><b>amount</b></td>
        <td>integer</td>
        <td>
          Amount of the resource being claimed, measured in the BaseUnit
defined by the corresponding ResourceRegistration.<br/>
          <br/>
            <i>Format</i>: int64<br/>
            <i>Minimum</i>: 0<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>resourceType</b></td>
        <td>string</td>
        <td>
          Fully qualified name of the resource type being claimed.
Must match a registered ResourceRegistration.spec.resourceType
(for example, "resourcemanager.miloapis.com/projects" or
"core/persistentvolumeclaims").<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### ClaimCreationPolicy.spec.target.resourceClaimTemplate.spec.resourceRef
<sup><sup>[↩ Parent](#claimcreationpolicyspectargetresourceclaimtemplatespec)</sup></sup>



ResourceRef links to the actual resource that triggered this quota claim.
Automatically populated by the admission plugin.
Uses an unversioned reference to persist across API version upgrades.

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
        <td><b>kind</b></td>
        <td>string</td>
        <td>
          Kind of the referent.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name of the referent.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>apiGroup</b></td>
        <td>string</td>
        <td>
          APIGroup is the group for the resource being referenced.
If APIGroup is not specified, the specified Kind must be in the core API group.
For any other third-party types, APIGroup is required.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          Namespace of the referent.
More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/namespaces/<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### ClaimCreationPolicy.spec.trigger
<sup><sup>[↩ Parent](#claimcreationpolicyspec)</sup></sup>



Trigger defines what resource changes should trigger claim creation.

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
        <td><b><a href="#claimcreationpolicyspectriggerresource">resource</a></b></td>
        <td>object</td>
        <td>
          Resource specifies which resource type triggers this policy.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#claimcreationpolicyspectriggerconditionsindex">conditions</a></b></td>
        <td>[]object</td>
        <td>
          Conditions are CEL expressions that must evaluate to true for claim creation to occur.
Evaluated in the admission context.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### ClaimCreationPolicy.spec.trigger.resource
<sup><sup>[↩ Parent](#claimcreationpolicyspectrigger)</sup></sup>



Resource specifies which resource type triggers this policy.

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
        <td>
          APIVersion of the target resource in the format "group/version".<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>kind</b></td>
        <td>string</td>
        <td>
          Kind is the kind of the target resource.<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### ClaimCreationPolicy.spec.trigger.conditions[index]
<sup><sup>[↩ Parent](#claimcreationpolicyspectrigger)</sup></sup>



ConditionExpression defines a CEL expression for condition evaluation.

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
        <td><b>expression</b></td>
        <td>string</td>
        <td>
          Expression is the CEL expression to evaluate against the trigger resource.
The expression must return a boolean value.
Available variables:
- GrantCreationPolicy (controller): `object` is the trigger resource (map)
- ClaimCreationPolicy (admission): `trigger` is the trigger resource (map);
  also `user.name`, `user.uid`, `user.groups`, `user.extra`, `requestInfo.*`,
  `namespace`, `gvk.group`, `gvk.version`, `gvk.kind`<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>message</b></td>
        <td>string</td>
        <td>
          Message provides a human-readable description of the condition requirement.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### ClaimCreationPolicy.status
<sup><sup>[↩ Parent](#claimcreationpolicy)</sup></sup>



ClaimCreationPolicyStatus defines the observed state of ClaimCreationPolicy.

Status fields
- conditions[type=Ready]: True when the policy is validated and active.

See also
- [ResourceClaim](#resourceclaim): The object created by this policy.

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
        <td><b><a href="#claimcreationpolicystatusconditionsindex">conditions</a></b></td>
        <td>[]object</td>
        <td>
          Conditions represent the latest available observations of the policy's current state.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>observedGeneration</b></td>
        <td>integer</td>
        <td>
          ObservedGeneration is the most recent generation observed.<br/>
          <br/>
            <i>Format</i>: int64<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### ClaimCreationPolicy.status.conditions[index]
<sup><sup>[↩ Parent](#claimcreationpolicystatus)</sup></sup>



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

## GrantCreationPolicy
<sup><sup>[↩ Parent](#quotamiloapiscomv1alpha1 )</sup></sup>






GrantCreationPolicy automates ResourceGrant creation when observed resources meet conditions.
Use it to provision quota based on resource lifecycle events and attributes.

### How It Works
- Watch the kind in `spec.trigger.resource` and evaluate all `spec.trigger.conditions[]`.
- When all conditions are true, render `spec.target.resourceGrantTemplate` and create a `ResourceGrant`.
- Optionally target a parent control plane via `spec.target.parentContext` (CEL-resolved name) for cross-cluster allocation.
- Templating supports variables `.trigger`, `.requestInfo`, `.user` and functions `lower`, `upper`, `title`, `default`, `contains`, `join`, `split`, `replace`, `trim`, `toInt`, `toString`.
- Allowances (resource types and amounts) are static in `v1alpha1`.

### Works With
- Creates [ResourceGrant](#resourcegrant) objects whose `allowances[].resourceType` must exist in a [ResourceRegistration](#resourceregistration).
- May target a parent control plane via `spec.target.parentContext` for cross-plane quota allocation.
- Policy readiness (`status.conditions[type=Ready]`) signals template/condition validity.

### Status
- `status.conditions[type=Ready]`: Policy validated and active.
- `status.conditions[type=ParentContextReady]`: Cross‑cluster targeting is resolvable.
- `status.observedGeneration`: Latest spec generation processed.

### Selectors and Filtering
  - Field selectors (server-side):
    `spec.trigger.resource.kind`, `spec.trigger.resource.apiVersion`,
    `spec.target.parentContext.kind`, `spec.target.parentContext.apiGroup`.
  - Label selectors (add your own):
  - `quota.miloapis.com/trigger-kind`: `Organization`
  - `quota.miloapis.com/environment`: `prod`
  - Common queries:
  - All policies for a trigger kind: label selector `quota.miloapis.com/trigger-kind`.
  - All enabled policies: field selector `spec.enabled=true`.

### Defaults and Limits
- Resource grant allowances are static (no expression-based amounts) in `v1alpha1`.

### Notes
- If `ParentContextReady=False`, verify `nameExpression` and referenced attributes.
- Disabled policies (`spec.enabled=false`) do not create grants.

### See Also
- [ResourceGrant](#resourcegrant): The object created by this policy.
- [ResourceRegistration](#resourceregistration): Resource types that grants must reference.
- [ClaimCreationPolicy](#claimcreationpolicy): Creates claims at admission for enforcement.

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
      <td>quota.miloapis.com/v1alpha1</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b>kind</b></td>
      <td>string</td>
      <td>GrantCreationPolicy</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectmeta-v1-meta">metadata</a></b></td>
      <td>object</td>
      <td>Refer to the Kubernetes API documentation for the fields of the `metadata` field.</td>
      <td>true</td>
      </tr><tr>
        <td><b><a href="#grantcreationpolicyspec">spec</a></b></td>
        <td>object</td>
        <td>
          GrantCreationPolicySpec defines the desired state of GrantCreationPolicy.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#grantcreationpolicystatus">status</a></b></td>
        <td>object</td>
        <td>
          GrantCreationPolicyStatus defines the observed state of GrantCreationPolicy.

Status fields
- conditions[type=Ready]: True when the policy is validated and active.
- conditions[type=ParentContextReady]: True when cross‑cluster targeting is resolvable.
- observedGeneration: Latest spec generation processed by the controller.

See also
- [ResourceGrant](#resourcegrant): The object created by this policy.
- [ResourceRegistration](#resourceregistration): Resource types for which grants are issued.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### GrantCreationPolicy.spec
<sup><sup>[↩ Parent](#grantcreationpolicy)</sup></sup>



GrantCreationPolicySpec defines the desired state of GrantCreationPolicy.

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
        <td><b><a href="#grantcreationpolicyspectarget">target</a></b></td>
        <td>object</td>
        <td>
          Target defines where and how grants should be created.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#grantcreationpolicyspectrigger">trigger</a></b></td>
        <td>object</td>
        <td>
          Trigger defines what resource changes should trigger grant creation.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>enabled</b></td>
        <td>boolean</td>
        <td>
          Enabled determines if this policy is active.
If false, no ResourceGrants will be created for matching resources.<br/>
          <br/>
            <i>Default</i>: true<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### GrantCreationPolicy.spec.target
<sup><sup>[↩ Parent](#grantcreationpolicyspec)</sup></sup>



Target defines where and how grants should be created.

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
        <td><b><a href="#grantcreationpolicyspectargetresourcegranttemplate">resourceGrantTemplate</a></b></td>
        <td>object</td>
        <td>
          ResourceGrantTemplate defines how to create ResourceGrants.
String fields support Go template syntax for dynamic content.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#grantcreationpolicyspectargetparentcontext">parentContext</a></b></td>
        <td>object</td>
        <td>
          ParentContext defines cross-control-plane targeting.
If specified, grants will be created in the target parent context
instead of the current control plane.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### GrantCreationPolicy.spec.target.resourceGrantTemplate
<sup><sup>[↩ Parent](#grantcreationpolicyspectarget)</sup></sup>



ResourceGrantTemplate defines how to create ResourceGrants.
String fields support Go template syntax for dynamic content.

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
        <td><b><a href="#grantcreationpolicyspectargetresourcegranttemplatemetadata">metadata</a></b></td>
        <td>object</td>
        <td>
          Metadata for the created ResourceGrant.
String fields support Go template syntax.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#grantcreationpolicyspectargetresourcegranttemplatespec">spec</a></b></td>
        <td>object</td>
        <td>
          Spec for the created ResourceGrant.
String fields support Go template syntax.<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### GrantCreationPolicy.spec.target.resourceGrantTemplate.metadata
<sup><sup>[↩ Parent](#grantcreationpolicyspectargetresourcegranttemplate)</sup></sup>



Metadata for the created ResourceGrant.
String fields support Go template syntax.

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
        <td><b>annotations</b></td>
        <td>map[string]string</td>
        <td>
          Annotations to set on the created object. Values support Go templates.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>generateName</b></td>
        <td>string</td>
        <td>
          GenerateName prefix for the created object when Name is empty. Supports Go templates.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>labels</b></td>
        <td>map[string]string</td>
        <td>
          Labels to set on the created object. Literal values only.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name of the created object. Supports Go templates.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          Namespace where the object will be created. Supports Go templates.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### GrantCreationPolicy.spec.target.resourceGrantTemplate.spec
<sup><sup>[↩ Parent](#grantcreationpolicyspectargetresourcegranttemplate)</sup></sup>



Spec for the created ResourceGrant.
String fields support Go template syntax.

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
        <td><b><a href="#grantcreationpolicyspectargetresourcegranttemplatespecallowancesindex">allowances</a></b></td>
        <td>[]object</td>
        <td>
          List of allowances this grant contains<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#grantcreationpolicyspectargetresourcegranttemplatespecconsumerref">consumerRef</a></b></td>
        <td>object</td>
        <td>
          ConsumerRef identifies the quota consumer (recipient) that receives
these allowances (for example, an Organization).<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### GrantCreationPolicy.spec.target.resourceGrantTemplate.spec.allowances[index]
<sup><sup>[↩ Parent](#grantcreationpolicyspectargetresourcegranttemplatespec)</sup></sup>



Allowance defines a single resource allowance within a grant

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
        <td><b><a href="#grantcreationpolicyspectargetresourcegranttemplatespecallowancesindexbucketsindex">buckets</a></b></td>
        <td>[]object</td>
        <td>
          List of buckets this allowance contains<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>resourceType</b></td>
        <td>string</td>
        <td>
          Fully qualified name of the resource type being granted.
Must match a registered ResourceRegistration.spec.resourceType
(for example, "resourcemanager.miloapis.com/projects").<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### GrantCreationPolicy.spec.target.resourceGrantTemplate.spec.allowances[index].buckets[index]
<sup><sup>[↩ Parent](#grantcreationpolicyspectargetresourcegranttemplatespecallowancesindex)</sup></sup>





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
        <td><b>amount</b></td>
        <td>integer</td>
        <td>
          Amount of the resource type being granted, measured in the BaseUnit
defined by the corresponding ResourceRegistration for this resource type.<br/>
          <br/>
            <i>Format</i>: int64<br/>
            <i>Minimum</i>: 0<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### GrantCreationPolicy.spec.target.resourceGrantTemplate.spec.consumerRef
<sup><sup>[↩ Parent](#grantcreationpolicyspectargetresourcegranttemplatespec)</sup></sup>



ConsumerRef identifies the quota consumer (recipient) that receives
these allowances (for example, an Organization).

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
        <td><b>kind</b></td>
        <td>string</td>
        <td>
          Kind of the consumer resource (for example, Organization, Project).<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name of the consumer resource object instance.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>apiGroup</b></td>
        <td>string</td>
        <td>
          APIGroup of the target resource (e.g., "resourcemanager.miloapis.com").
Empty string for core API group.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### GrantCreationPolicy.spec.target.parentContext
<sup><sup>[↩ Parent](#grantcreationpolicyspectarget)</sup></sup>



ParentContext defines cross-control-plane targeting.
If specified, grants will be created in the target parent context
instead of the current control plane.

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
          APIGroup of the parent context resource.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>kind</b></td>
        <td>string</td>
        <td>
          Kind of the parent context resource.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>nameExpression</b></td>
        <td>string</td>
        <td>
          NameExpression is a CEL expression to resolve the parent context name.
The expression must return a string value.
Available variables:
- object: The trigger resource being evaluated (same as .trigger in Go templates)<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### GrantCreationPolicy.spec.trigger
<sup><sup>[↩ Parent](#grantcreationpolicyspec)</sup></sup>



Trigger defines what resource changes should trigger grant creation.

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
        <td><b><a href="#grantcreationpolicyspectriggerresource">resource</a></b></td>
        <td>object</td>
        <td>
          Resource specifies which resource type triggers this policy.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#grantcreationpolicyspectriggerconditionsindex">conditions</a></b></td>
        <td>[]object</td>
        <td>
          Conditions are CEL expressions that must evaluate to true for grant creation.
All conditions must pass for the policy to trigger.
The 'object' variable contains the trigger resource being evaluated.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### GrantCreationPolicy.spec.trigger.resource
<sup><sup>[↩ Parent](#grantcreationpolicyspectrigger)</sup></sup>



Resource specifies which resource type triggers this policy.

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
        <td>
          APIVersion of the trigger resource in the format "group/version".
For core resources, use "v1".<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>kind</b></td>
        <td>string</td>
        <td>
          Kind is the kind of the trigger resource.<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### GrantCreationPolicy.spec.trigger.conditions[index]
<sup><sup>[↩ Parent](#grantcreationpolicyspectrigger)</sup></sup>



ConditionExpression defines a CEL expression for condition evaluation.

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
        <td><b>expression</b></td>
        <td>string</td>
        <td>
          Expression is the CEL expression to evaluate against the trigger resource.
The expression must return a boolean value.
Available variables:
- GrantCreationPolicy (controller): `object` is the trigger resource (map)
- ClaimCreationPolicy (admission): `trigger` is the trigger resource (map);
  also `user.name`, `user.uid`, `user.groups`, `user.extra`, `requestInfo.*`,
  `namespace`, `gvk.group`, `gvk.version`, `gvk.kind`<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>message</b></td>
        <td>string</td>
        <td>
          Message provides a human-readable description of the condition requirement.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### GrantCreationPolicy.status
<sup><sup>[↩ Parent](#grantcreationpolicy)</sup></sup>



GrantCreationPolicyStatus defines the observed state of GrantCreationPolicy.

Status fields
- conditions[type=Ready]: True when the policy is validated and active.
- conditions[type=ParentContextReady]: True when cross‑cluster targeting is resolvable.
- observedGeneration: Latest spec generation processed by the controller.

See also
- [ResourceGrant](#resourcegrant): The object created by this policy.
- [ResourceRegistration](#resourceregistration): Resource types for which grants are issued.

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
        <td><b><a href="#grantcreationpolicystatusconditionsindex">conditions</a></b></td>
        <td>[]object</td>
        <td>
          Conditions represent the latest available observations of the policy's current state.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>observedGeneration</b></td>
        <td>integer</td>
        <td>
          ObservedGeneration is the most recent generation observed.<br/>
          <br/>
            <i>Format</i>: int64<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### GrantCreationPolicy.status.conditions[index]
<sup><sup>[↩ Parent](#grantcreationpolicystatus)</sup></sup>



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

## ResourceClaim
<sup><sup>[↩ Parent](#quotamiloapiscomv1alpha1 )</sup></sup>






ResourceClaim represents a quota consumption request tied to the creation of a resource.
ClaimCreationPolicy typically creates these claims during admission to enforce quota in real time.
The system evaluates each claim against [AllowanceBucket](#allowancebucket)s that aggregate available capacity.

### How It Works
- Admission evaluates policies that match the incoming resource and creates a `ResourceClaim`.
- The claim requests one or more resource types and amounts for a specific `consumerRef`.
- The system grants the claim when sufficient capacity is available; otherwise it denies it.
- `resourceRef` links back to the triggering resource to enable cleanup and auditing.

### Works With
- Created by [ClaimCreationPolicy](#claimcreationpolicy) at admission when trigger conditions match.
- Evaluated against [AllowanceBucket](#allowancebucket) capacity for the matching `spec.consumerRef` + `spec.requests[].resourceType`.
- Must target a registered `resourceType`; the triggering kind must be allowed by the target [ResourceRegistration](#resourceregistration) `spec.claimingResources`.
- Controllers set owner references where possible and clean up denied auto‑created claims.

### Notes
- Auto-created claims set owner references when possible; a fallback path updates ownership asynchronously.
- Auto-created claims denied by policy are cleaned up automatically; manual claims are not.

### Selectors and Filtering
  - Field selectors (server-side):
    `spec.consumerRef.kind`, `spec.consumerRef.name`,
    `spec.resourceRef.apiGroup`, `spec.resourceRef.kind`, `spec.resourceRef.name`, `spec.resourceRef.namespace`.
  - Built-in labels (on auto-created claims):
  - `quota.miloapis.com/auto-created`: `"true"`
  - `quota.miloapis.com/policy`: `<ClaimCreationPolicy name>`
  - `quota.miloapis.com/gvk`: `<group.version.kind of the triggering resource>`
  - Built-in annotations (on auto-created claims):
  - `quota.miloapis.com/created-by`: `claim-creation-plugin`
  - `quota.miloapis.com/created-at`: `RFC3339` timestamp
  - `quota.miloapis.com/resource-name`: name of the triggering resource
  - `quota.miloapis.com/policy`: `<ClaimCreationPolicy name>`
  - Common queries:
  - All auto-created claims for a policy: label selector `quota.miloapis.com/policy`.
  - All claims for a consumer: add labels for `consumer-kind` and `consumer-name` via policy templates and filter by label.
  - All claims for a specific triggering kind: label selector `quota.miloapis.com/gvk`.

### See Also
- [AllowanceBucket](#allowancebucket): Aggregates limits and usage that drive claim evaluation.
- [ResourceGrant](#resourcegrant): Supplies capacity aggregated by buckets.
- [ClaimCreationPolicy](#claimcreationpolicy): Automates creation of ResourceClaims at admission.
- [ResourceRegistration](#resourceregistration): Defines claimable resource types.

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
      <td>quota.miloapis.com/v1alpha1</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b>kind</b></td>
      <td>string</td>
      <td>ResourceClaim</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectmeta-v1-meta">metadata</a></b></td>
      <td>object</td>
      <td>Refer to the Kubernetes API documentation for the fields of the `metadata` field.</td>
      <td>true</td>
      </tr><tr>
        <td><b><a href="#resourceclaimspec">spec</a></b></td>
        <td>object</td>
        <td>
          ResourceClaimSpec defines the desired state of ResourceClaim.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#resourceclaimstatus">status</a></b></td>
        <td>object</td>
        <td>
          ResourceClaimStatus captures the controller's evaluation of a claim: an overall
grant decision reported via conditions and per‑resource allocation results. It
also records the most recent observed spec generation. See the schema for exact
fields, condition reasons, and constraints. For capacity context, consult
[AllowanceBucket](#allowancebucket) and for capacity sources see
[ResourceGrant](#resourcegrant).<br/>
          <br/>
            <i>Default</i>: map[conditions:[map[lastTransitionTime:1970-01-01T00:00:00Z message:Awaiting capacity evaluation reason:PendingEvaluation status:False type:Granted]]]<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### ResourceClaim.spec
<sup><sup>[↩ Parent](#resourceclaim)</sup></sup>



ResourceClaimSpec defines the desired state of ResourceClaim.

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
        <td><b><a href="#resourceclaimspecconsumerref">consumerRef</a></b></td>
        <td>object</td>
        <td>
          ConsumerRef identifies the quota consumer (the subject that receives
limits and consumes capacity) making this claim. Examples include an
Organization or a Project, depending on how the registration is defined.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#resourceclaimspecrequestsindex">requests</a></b></td>
        <td>[]object</td>
        <td>
          Requests specifies the resource types and amounts being claimed.
Each resource type must be unique within the requests array.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#resourceclaimspecresourceref">resourceRef</a></b></td>
        <td>object</td>
        <td>
          ResourceRef links to the actual resource that triggered this quota claim.
Automatically populated by the admission plugin.
Uses an unversioned reference to persist across API version upgrades.<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### ResourceClaim.spec.consumerRef
<sup><sup>[↩ Parent](#resourceclaimspec)</sup></sup>



ConsumerRef identifies the quota consumer (the subject that receives
limits and consumes capacity) making this claim. Examples include an
Organization or a Project, depending on how the registration is defined.

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
        <td><b>kind</b></td>
        <td>string</td>
        <td>
          Kind of the consumer resource (for example, Organization, Project).<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name of the consumer resource object instance.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>apiGroup</b></td>
        <td>string</td>
        <td>
          APIGroup of the target resource (e.g., "resourcemanager.miloapis.com").
Empty string for core API group.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### ResourceClaim.spec.requests[index]
<sup><sup>[↩ Parent](#resourceclaimspec)</sup></sup>



ResourceRequest defines a single resource request within a claim

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
        <td><b>amount</b></td>
        <td>integer</td>
        <td>
          Amount of the resource being claimed, measured in the BaseUnit
defined by the corresponding ResourceRegistration.<br/>
          <br/>
            <i>Format</i>: int64<br/>
            <i>Minimum</i>: 0<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>resourceType</b></td>
        <td>string</td>
        <td>
          Fully qualified name of the resource type being claimed.
Must match a registered ResourceRegistration.spec.resourceType
(for example, "resourcemanager.miloapis.com/projects" or
"core/persistentvolumeclaims").<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### ResourceClaim.spec.resourceRef
<sup><sup>[↩ Parent](#resourceclaimspec)</sup></sup>



ResourceRef links to the actual resource that triggered this quota claim.
Automatically populated by the admission plugin.
Uses an unversioned reference to persist across API version upgrades.

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
        <td><b>kind</b></td>
        <td>string</td>
        <td>
          Kind of the referent.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name of the referent.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>apiGroup</b></td>
        <td>string</td>
        <td>
          APIGroup is the group for the resource being referenced.
If APIGroup is not specified, the specified Kind must be in the core API group.
For any other third-party types, APIGroup is required.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          Namespace of the referent.
More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/namespaces/<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### ResourceClaim.status
<sup><sup>[↩ Parent](#resourceclaim)</sup></sup>



ResourceClaimStatus captures the controller's evaluation of a claim: an overall
grant decision reported via conditions and per‑resource allocation results. It
also records the most recent observed spec generation. See the schema for exact
fields, condition reasons, and constraints. For capacity context, consult
[AllowanceBucket](#allowancebucket) and for capacity sources see
[ResourceGrant](#resourcegrant).

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
        <td><b><a href="#resourceclaimstatusallocationsindex">allocations</a></b></td>
        <td>[]object</td>
        <td>
          removed: aggregate allocated total is not tracked; use per-request allocations instead
Per-request allocation status tracking. Each entry corresponds to a resource type in spec.requests[]<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#resourceclaimstatusconditionsindex">conditions</a></b></td>
        <td>[]object</td>
        <td>
          Known condition types: "Granted"<br/>
          <br/>
            <i>Validations</i>:<li>self.all(c, c.type == 'Granted' ? c.reason in ['QuotaAvailable', 'QuotaExceeded', 'ValidationFailed', 'PendingEvaluation'] : true): Granted condition reason must be valid</li>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>observedGeneration</b></td>
        <td>integer</td>
        <td>
          Most recent generation observed.<br/>
          <br/>
            <i>Format</i>: int64<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### ResourceClaim.status.allocations[index]
<sup><sup>[↩ Parent](#resourceclaimstatus)</sup></sup>



RequestAllocation tracks the allocation status of a specific resource request within a claim.

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
          Timestamp of the last status transition for this allocation<br/>
          <br/>
            <i>Format</i>: date-time<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>resourceType</b></td>
        <td>string</td>
        <td>
          Resource type that this allocation status refers to.
Must correspond to a resourceType listed in spec.requests.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>status</b></td>
        <td>enum</td>
        <td>
          Status of this specific request allocation<br/>
          <br/>
            <i>Enum</i>: Granted, Denied, Pending<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>allocatedAmount</b></td>
        <td>integer</td>
        <td>
          Amount actually allocated for this request (may be less than requested in some scenarios),
measured in the BaseUnit defined by the ResourceRegistration.<br/>
          <br/>
            <i>Format</i>: int64<br/>
            <i>Minimum</i>: 0<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>allocatingBucket</b></td>
        <td>string</td>
        <td>
          Name of the AllowanceBucket that provided this allocation (set when status is Granted)<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>message</b></td>
        <td>string</td>
        <td>
          Human-readable message describing the allocation result<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>reason</b></td>
        <td>string</td>
        <td>
          Reason for the current allocation status<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### ResourceClaim.status.conditions[index]
<sup><sup>[↩ Parent](#resourceclaimstatus)</sup></sup>



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

## ResourceGrant
<sup><sup>[↩ Parent](#quotamiloapiscomv1alpha1 )</sup></sup>






ResourceGrant allocates capacity to a consumer for one or more resource types.
AllowanceBuckets aggregate active grants to calculate available quota.
You can create grants manually or automate them with GrantCreationPolicy.

### How It Works
- Allocate allowances for one or more `resourceType`s to a `consumerRef`.
- Only grants with `status.conditions[type=Active]==True` contribute to bucket limits.
- Grants may be created manually or via [GrantCreationPolicy](#grantcreationpolicy).

### Works With
- Increases [AllowanceBucket](#allowancebucket) `status.limit` for matching (`spec.consumerRef`, `allowances[].resourceType`).
- Only grants with `status.conditions[type=Active]=="True"` affect bucket limits.
- Often created by [GrantCreationPolicy](#grantcreationpolicy); manual grants behave the same.
- Cross-plane allocations are possible when policies target a parent context.

### Selectors and Filtering
  - Field selectors (server-side): `spec.consumerRef.kind`, `spec.consumerRef.name`.
  - Label selectors: Add your own labels in metadata to group grants (for example by tier or region).
    Common labels you may add:
  - `quota.miloapis.com/consumer-kind`: `Organization`
  - `quota.miloapis.com/consumer-name`: `<name>`
  - `quota.miloapis.com/resource-kind`: `Project` (repeat per allowance if desired)
  - Common queries:
  - All grants for a consumer: labels `quota.miloapis.com/consumer-kind` + `quota.miloapis.com/consumer-name`.
  - Grants created by a policy: use a policy label your automation adds consistently.

### Notes
- Amounts use the BaseUnit from the corresponding ResourceRegistration.
- Multiple ResourceGrants can contribute to a single bucket; see bucket grantCount and contributingGrantRefs.

### See Also
- [AllowanceBucket](#allowancebucket): Aggregates active grants into a single limit.
- [ResourceRegistration](#resourceregistration): Validates resourceType names and claimability.
- [GrantCreationPolicy](#grantcreationpolicy): Automates grant creation based on observed resources.

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
      <td>quota.miloapis.com/v1alpha1</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b>kind</b></td>
      <td>string</td>
      <td>ResourceGrant</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectmeta-v1-meta">metadata</a></b></td>
      <td>object</td>
      <td>Refer to the Kubernetes API documentation for the fields of the `metadata` field.</td>
      <td>true</td>
      </tr><tr>
        <td><b><a href="#resourcegrantspec">spec</a></b></td>
        <td>object</td>
        <td>
          ResourceGrantSpec defines the desired state of ResourceGrant.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#resourcegrantstatus">status</a></b></td>
        <td>object</td>
        <td>
          ResourceGrantStatus indicates whether a grant is active and the most recent
spec generation processed by the controller. Only Active grants contribute to
bucket limits. See the schema for exact fields and condition reasons. For how
capacity is aggregated, see AllowanceBucket, and for type validity see
ResourceRegistration.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### ResourceGrant.spec
<sup><sup>[↩ Parent](#resourcegrant)</sup></sup>



ResourceGrantSpec defines the desired state of ResourceGrant.

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
        <td><b><a href="#resourcegrantspecallowancesindex">allowances</a></b></td>
        <td>[]object</td>
        <td>
          List of allowances this grant contains<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#resourcegrantspecconsumerref">consumerRef</a></b></td>
        <td>object</td>
        <td>
          ConsumerRef identifies the quota consumer (recipient) that receives
these allowances (for example, an Organization).<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### ResourceGrant.spec.allowances[index]
<sup><sup>[↩ Parent](#resourcegrantspec)</sup></sup>



Allowance defines a single resource allowance within a grant

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
        <td><b><a href="#resourcegrantspecallowancesindexbucketsindex">buckets</a></b></td>
        <td>[]object</td>
        <td>
          List of buckets this allowance contains<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>resourceType</b></td>
        <td>string</td>
        <td>
          Fully qualified name of the resource type being granted.
Must match a registered ResourceRegistration.spec.resourceType
(for example, "resourcemanager.miloapis.com/projects").<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### ResourceGrant.spec.allowances[index].buckets[index]
<sup><sup>[↩ Parent](#resourcegrantspecallowancesindex)</sup></sup>





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
        <td><b>amount</b></td>
        <td>integer</td>
        <td>
          Amount of the resource type being granted, measured in the BaseUnit
defined by the corresponding ResourceRegistration for this resource type.<br/>
          <br/>
            <i>Format</i>: int64<br/>
            <i>Minimum</i>: 0<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### ResourceGrant.spec.consumerRef
<sup><sup>[↩ Parent](#resourcegrantspec)</sup></sup>



ConsumerRef identifies the quota consumer (recipient) that receives
these allowances (for example, an Organization).

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
        <td><b>kind</b></td>
        <td>string</td>
        <td>
          Kind of the consumer resource (for example, Organization, Project).<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name of the consumer resource object instance.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>apiGroup</b></td>
        <td>string</td>
        <td>
          APIGroup of the target resource (e.g., "resourcemanager.miloapis.com").
Empty string for core API group.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### ResourceGrant.status
<sup><sup>[↩ Parent](#resourcegrant)</sup></sup>



ResourceGrantStatus indicates whether a grant is active and the most recent
spec generation processed by the controller. Only Active grants contribute to
bucket limits. See the schema for exact fields and condition reasons. For how
capacity is aggregated, see AllowanceBucket, and for type validity see
ResourceRegistration.

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
        <td><b><a href="#resourcegrantstatusconditionsindex">conditions</a></b></td>
        <td>[]object</td>
        <td>
          Known condition types: "Active"<br/>
          <br/>
            <i>Validations</i>:<li>self.all(c, c.type == 'Active' ? c.reason in ['GrantActive', 'ValidationFailed', 'GrantPending'] : true): Active condition reason must be valid</li>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>observedGeneration</b></td>
        <td>integer</td>
        <td>
          Most recent generation observed.<br/>
          <br/>
            <i>Format</i>: int64<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### ResourceGrant.status.conditions[index]
<sup><sup>[↩ Parent](#resourcegrantstatus)</sup></sup>



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

## ResourceRegistration
<sup><sup>[↩ Parent](#quotamiloapiscomv1alpha1 )</sup></sup>






ResourceRegistration defines which resource types the quota system manages and how to measure them.
Registrations enable grants and claims for a specific resource type, using clear units and ownership rules.

### How It Works
- Administrators create registrations to opt resource types into quota.
- After activation, ResourceGrants allocate capacity and ResourceClaims consume it for the type.

### Works With
- [ResourceGrant](#resourcegrant) `allowances[].resourceType` must match `spec.resourceType`.
- [ResourceClaim](#resourceclaim) `spec.requests[].resourceType` must match `spec.resourceType`.
- The triggering kind must be listed in `spec.claimingResources` for claims to be valid.
- Consumers in grants/claims must match `spec.consumerTypeRef`.

### Selectors and Filtering
- Field selectors (server-side): `spec.consumerTypeRef.kind`, `spec.consumerTypeRef.apiGroup`, `spec.resourceType`.
- Label selectors (add your own):
  - `quota.miloapis.com/resource-kind`: `<Kind>`
  - `quota.miloapis.com/resource-apigroup`: `<API group>`
  - `quota.miloapis.com/consumer-kind`: `<Kind>`

- Common queries:
  - All registrations for a resource kind: label selector `quota.miloapis.com/resource-kind` (+ `quota.miloapis.com/resource-apigroup` when needed).
  - All registrations for a consumer kind: label selector `quota.miloapis.com/consumer-kind`.

### Defaults and Limits
- `spec.type`: `Entity` (count objects) or `Allocation` (numeric capacity).
- `spec.claimingResources`: up to 20 entries; unversioned references (`apiGroup`, `kind`).
- `spec.resourceType`: must follow `group/resource` with optional subresource path.

### Notes
- `claimingResources` are unversioned; kind matching is case-insensitive and apiGroup must align.
- Grants and claims use `baseUnit`; `displayUnit` and `unitConversionFactor` affect presentation only.

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
      <td>quota.miloapis.com/v1alpha1</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b>kind</b></td>
      <td>string</td>
      <td>ResourceRegistration</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectmeta-v1-meta">metadata</a></b></td>
      <td>object</td>
      <td>Refer to the Kubernetes API documentation for the fields of the `metadata` field.</td>
      <td>true</td>
      </tr><tr>
        <td><b><a href="#resourceregistrationspec">spec</a></b></td>
        <td>object</td>
        <td>
          ResourceRegistrationSpec defines the desired state of ResourceRegistration.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#resourceregistrationstatus">status</a></b></td>
        <td>object</td>
        <td>
          ResourceRegistrationStatus reports whether the registration is usable and the
latest spec generation processed. When Active, grants and claims may be created
for the registered type. See the schema for exact fields and condition reasons.
Related objects include [ResourceGrant](#resourcegrant) and
[ResourceClaim](#resourceclaim).<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### ResourceRegistration.spec
<sup><sup>[↩ Parent](#resourceregistration)</sup></sup>



ResourceRegistrationSpec defines the desired state of ResourceRegistration.

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
        <td><b>baseUnit</b></td>
        <td>string</td>
        <td>
          BaseUnit defines the internal measurement unit for quota calculations.
Examples: "projects", "millicores", "bytes"<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#resourceregistrationspecconsumertyperef">consumerTypeRef</a></b></td>
        <td>object</td>
        <td>
          ConsumerTypeRef identifies the resource type that receives grants and creates claims.
For example, when registering "Projects per Organization", the ConsumerTypeRef
would be Organization, which can then receive ResourceGrants allocating Project quota.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>displayUnit</b></td>
        <td>string</td>
        <td>
          DisplayUnit defines the unit shown in user interfaces.
Examples: "projects", "cores", "GiB"<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>resourceType</b></td>
        <td>string</td>
        <td>
          ResourceType identifies the Kubernetes resource to track with quota.
Must match an existing resource type accessible in the cluster.
Format: apiGroup/resource (plural), with optional subresource path
(for example, "resourcemanager.miloapis.com/projects" or
"core/pods/cpu").<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>type</b></td>
        <td>enum</td>
        <td>
          Type classifies how the system measures this registration.
Entity: Tracks the count of object instances (for example, number of Projects).
Allocation: Tracks numeric capacity (for example, bytes of storage, CPU millicores).<br/>
          <br/>
            <i>Enum</i>: Entity, Allocation<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>unitConversionFactor</b></td>
        <td>integer</td>
        <td>
          UnitConversionFactor converts baseUnit to displayUnit.
Formula: displayValue = baseValue / unitConversionFactor
Examples: 1 (no conversion), 1073741824 (bytes to GiB), 1000 (millicores to cores)<br/>
          <br/>
            <i>Format</i>: int64<br/>
            <i>Minimum</i>: 1<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#resourceregistrationspecclaimingresourcesindex">claimingResources</a></b></td>
        <td>[]object</td>
        <td>
          ClaimingResources specifies which resource types can create ResourceClaims
for this registered resource type. When a ResourceClaim includes a resourceRef,
the referenced resource's type must be in this list for the claim to be valid.
If empty, no resources can claim this quota - administrators must explicitly
configure which resources can claim quota for security.

This field also signals to the ownership controller which resource types
to watch for automatic owner reference creation.

Uses unversioned references to support API version upgrades without
requiring ResourceRegistration updates.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>description</b></td>
        <td>string</td>
        <td>
          Description provides context about what this registration tracks<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### ResourceRegistration.spec.consumerTypeRef
<sup><sup>[↩ Parent](#resourceregistrationspec)</sup></sup>



ConsumerTypeRef identifies the resource type that receives grants and creates claims.
For example, when registering "Projects per Organization", the ConsumerTypeRef
would be Organization, which can then receive ResourceGrants allocating Project quota.

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
          API group of the quota consumer resource type<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>kind</b></td>
        <td>string</td>
        <td>
          Resource type that consumes quota from this registration<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### ResourceRegistration.spec.claimingResources[index]
<sup><sup>[↩ Parent](#resourceregistrationspec)</sup></sup>



ClaimingResource identifies a resource type that can create ResourceClaims
for a registered resource type using an unversioned reference.

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
        <td><b>kind</b></td>
        <td>string</td>
        <td>
          Kind of the referent.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>apiGroup</b></td>
        <td>string</td>
        <td>
          APIGroup is the group for the resource being referenced.
If APIGroup is not specified, the specified Kind must be in the core API group.
For any other third-party types, APIGroup is required.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### ResourceRegistration.status
<sup><sup>[↩ Parent](#resourceregistration)</sup></sup>



ResourceRegistrationStatus reports whether the registration is usable and the
latest spec generation processed. When Active, grants and claims may be created
for the registered type. See the schema for exact fields and condition reasons.
Related objects include [ResourceGrant](#resourcegrant) and
[ResourceClaim](#resourceclaim).

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
        <td><b><a href="#resourceregistrationstatusconditionsindex">conditions</a></b></td>
        <td>[]object</td>
        <td>
          Current status conditions. Known condition types: "Active" below marker
ensures controllers set a correct and standardized status and an external
client can't set the status to bypass validation.<br/>
          <br/>
            <i>Validations</i>:<li>self.all(c, c.type == 'Active' ? c.reason in ['RegistrationActive', 'ValidationFailed', 'RegistrationPending'] : true): Active condition reason must be valid</li>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>observedGeneration</b></td>
        <td>integer</td>
        <td>
          Most recent generation observed by the controller.<br/>
          <br/>
            <i>Format</i>: int64<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### ResourceRegistration.status.conditions[index]
<sup><sup>[↩ Parent](#resourceregistrationstatus)</sup></sup>



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
