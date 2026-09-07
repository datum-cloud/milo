# Entitlement Registration Contact Group Enrollment

> Status: Draft, seeking review. Source issue: datum-cloud/cloud-portal#1497.

## Revision History

- **v2 (this revision)**: Supersedes the v1 (Milo-trigger-based) approach.
  Scot (service-catalog owner) reviewed v1 and redirected the design:

  > "I'd drive it from the service catalog side. This way Milo's core system
  > doesn't understand what services are and service centric logic stays
  > contained within the service catalog. Kinda like why billing doesn't know
  > what a service is."

  This is the guiding principle for v2: Milo's `notification.miloapis.com`
  group stays a generic, service-agnostic CRM primitive (`Contact`,
  `ContactGroup`, `ContactGroupMembership`, and the existing
  `ContactGroupEnrollmentPolicy`/`ContactCreated` mechanism). It gains **no**
  new trigger type, no entitlement selector, and no cross-cluster watch of
  `ServiceEntitlement`. Instead, service-catalog — which already understands
  what a "service" and an "entitlement" are — drives enrollment itself,
  writing directly to Milo's `ContactGroupMembership` API the same way a
  human operator or any other Milo API client would. This keeps
  service-specific logic contained in service-catalog, exactly as billing
  doesn't need to understand what a service is to meter and charge against
  it.

  Everything in v1 that was about *what* problem is being solved (identity
  resolution gap, idempotency, edge cases, scope cuts) still applies; only
  *where the controller lives and which direction the read/write happens*
  has changed. v1's Cross-Cluster/Cross-Repo Signal Design section (Milo
  watching `ServiceEntitlement` cross-cluster) is removed entirely, since its
  premise no longer holds.

## Overview

When a customer registers for a service entitlement (e.g. enables the
Compute service via a `ServiceEntitlement` in service-catalog), internal
staff want the requesting user's CRM contact record automatically added to
a corresponding staff-portal Contact Group (e.g. `compute-testers`), without
manual list maintenance.

This design adds a new reconciler **inside service-catalog** —
tentatively `ServiceEntitlementContactEnrollmentReconciler` — that watches
`ServiceEntitlement` objects it already reconciles today, resolves the
`Service` the entitlement is for, looks up a service → `ContactGroup`
mapping, resolves the registering user's `Contact` in Milo, and creates a
`ContactGroupMembership` directly via Milo's API. Milo's
`ContactGroupEnrollmentPolicy`/`ContactGroupEnrollmentController` mechanism
is untouched; it continues to serve only the `ContactCreated` trigger it
already supports. No new Milo CRD, trigger type, or cross-cluster RBAC grant
for Milo's own controller-manager is introduced.

Source: GitHub issue datum-cloud/cloud-portal#1497.

## Requirements

### Functional Requirements

- FR1: An operator can declare, for a given `Service`, which `ContactGroup`
  (in Milo) registering users' contacts should be enrolled into. See
  Service → ContactGroup Mapping below for where this declaration lives.
- FR2: When a `ServiceEntitlement` for a mapped service becomes `Active` in
  any project, the corresponding user's `Contact` is enrolled (a
  `ContactGroupMembership` is created in Milo), exactly once per
  (entitlement, service) pair — equivalently, once per (contact,
  target-group) pair given the deterministic membership name — matching the
  idempotency guarantee `ContactGroupEnrollmentController`'s existing
  `ContactCreated` trigger provides.
- FR3: If no `Contact` exists yet for the registering user at the time of
  registration, the enrollment is not lost — it completes once the `Contact`
  is created (or once identity resolution succeeds), without requiring the
  user to re-register.
- FR4: Existing `ContactGroupMembershipRemoval` opt-out records continue to
  suppress enrollment, exactly as they do for `ContactCreated` — service-
  catalog's reconciler checks the same opt-out records before creating a
  membership.
- FR5: The design states explicit, intentional behavior for entitlement
  revocation/rejection (see Edge Cases) — this rollout does NOT remove
  membership automatically; that is called out as a v2(product) follow-up,
  not silently omitted. (Naming note: "v2" here means "a later product
  iteration," distinct from this document's own v1→v2 design revision.)

### Non-Functional Requirements

- NFR1: No Go module dependency inversion is introduced. service-catalog
  already depends on `go.miloapis.com/milo` (confirmed:
  `service-catalog/go.mod` already requires `go.miloapis.com/milo`), so
  service-catalog importing `go.miloapis.com/milo/pkg/apis/notification/v1alpha1`
  is consistent with — not a change to — today's one-directional dependency
  (service-catalog → milo, never the reverse). This is strictly simpler than
  v1's unstructured/GVK-shim workaround, which existed only to avoid the
  *wrong* direction of dependency (milo → service-catalog).
- NFR2: No new cross-cluster watch machinery. The trigger side of this
  feature (watching `ServiceEntitlement`) reuses the exact reconcile loop
  `ServiceEntitlementReconciler` already runs per engaged project cluster;
  the only new fan-out is a client call to Milo's root/base API to write a
  `ContactGroupMembership`.
- NFR3: Enrollment latency target: contact should land in the group within a
  few minutes of the entitlement becoming Active (bounded by cache resync
  and normal reconcile latency, not polling).
- NFR4: No change to Milo's authorization model: `ContactGroupMembership`
  writes remain a system-controller responsibility. service-catalog's
  controller-manager service account is granted write access to
  `ContactGroupMembership` and read access to `Contact`/`ContactGroup` in
  Milo, analogous to how `enrollmentFieldOwner` already writes these objects
  today, just from a different service account.

## Design

### Resource Types

| Resource | Group | Storage | Description |
|----------|-------|---------|--------------|
| `ServiceEntitlement` (unmodified, read by new reconciler) | services.miloapis.com | etcd (per-project virtual control plane, service-catalog) | Existing schema, no change. `status.phase == Active` plus the creator annotation (new, see below) drive enrollment. |
| `Service` (read-only signal source) | services.miloapis.com | etcd (Milo root / base control plane) | Unmodified schema except for the new mapping field discussed below (see Service → ContactGroup Mapping). |
| `Contact` (read-only from service-catalog's perspective) | notification.miloapis.com | etcd (Milo root) | Enrollment target's identity anchor; **not created by this feature** if missing (see Identity Resolution). |
| `ContactGroupMembership` (write target) | notification.miloapis.com | etcd (Milo root) | Created by service-catalog's new reconciler via Milo's API, using the same create-only/idempotent semantics as `ContactGroupEnrollmentController.ensureMembership`. |

No new CRDs (unless the "separate mapping resource" option below is chosen —
see Service → ContactGroup Mapping). No changes at all to
`ContactGroupEnrollmentPolicy` — the `EntitlementRegistered` trigger type,
`EnrollmentEntitlementSelector`, and the validating-webhook rule proposed in
v1 are dropped in full.

Two schema-level additions remain, both entirely in service-catalog:

1. service-catalog: add a creator-identity annotation to `ServiceEntitlement`
   at admission time (currently absent — see Identity Resolution below).
   Unchanged from v1.
2. service-catalog: add a service → `ContactGroup` mapping field, either on
   `Service` or on a new small mapping resource (see next section — new in
   v2, since v1 didn't need this: the mapping lived implicitly in each
   `ContactGroupEnrollmentPolicy`'s `entitlementSelector.serviceName`, which
   no longer exists).

### Service -> ContactGroup Mapping

v1 didn't need an explicit mapping resource: a `ContactGroupEnrollmentPolicy`
in Milo directly named a `serviceName` string and a `contactGroupRef`. Now
that service-catalog drives enrollment, and Milo has no service-specific
concepts, this mapping needs a new home. Two options:

**Option A — field on `Service.spec` (recommended)**: Add an optional field,
e.g. `Service.spec.contactGroupRef` (`*ContactGroupReference{Name string}`,
naming a Milo `ContactGroup` by name), analogous to how `Service.spec.owner`
already references a Milo `ProducerProjectRef`. The new reconciler resolves
the entitlement's `Service` (exactly as `ServiceEntitlementReconciler`
already does via `resolveService`) and reads this field directly — no extra
lookup, no new CRD, no new controller-runtime watch/cache setup. Tradeoffs:
couples "does this service have a CRM group" into the `Service` object every
consumer-facing client also reads (a small, harmless field addition —
`Service` already carries several optional pointer fields like
`EnablementPolicy`), and it's a live cross-repo reference (service-catalog's
`Service.spec` naming a Milo `ContactGroup`) with no validation short of a
webhook cross-cluster lookup — same "accept any string, silent no-op on
mismatch" tradeoff v1 already accepted for `EnrollmentEntitlementSelector.serviceName`
(see Security Considerations).

**Option B — separate mapping resource in service-catalog** (e.g.
`ServiceContactGroupBinding`, cluster-scoped, one per mapped service):
keeps `Service` untouched and isolates a CRM-specific concern in its own
object with its own lifecycle, RBAC, and CRD version history. Tradeoffs:
another CRD, another controller-runtime watch/index, another object for
operators to discover and manage — more machinery for something that is, in
practice, a 1:1 optional annotation-shaped fact about a `Service`.

**Recommendation: Option A.** The mapping is a simple, optional,
per-`Service` fact ("this service has a CRM tester group") with no
independent lifecycle of its own — it's set once when a service opts in and
rarely changes. A new field costs one CRD schema addition and zero new
watch/index machinery; a separate resource costs a full new kind for what
is functionally an optional field. This mirrors `EnablementPolicy`'s
existing shape on `Service.spec` (optional, pointer, one small struct).
Flagged as a genuinely close call in Open Questions in case the
service-catalog team has a standing convention against embedding
cross-repo-referencing optional fields directly on `Service`.

### ContactGroup Creation/Linking

v1 decided this stays manual/operator-driven and deferred auto-creation.
Now that service-catalog owns this feature end-to-end (removing the
"which repo's controller creates it" ambiguity that made v1 punt), it's
worth revisiting per the issue author's and Scot's original suggestion of a
service-catalog controller that "auto-creates a contact group."

**Recommendation: keep manual-with-reference as the primary design** (an
operator pre-creates the `ContactGroup` in staff-portal's existing UI, e.g.
`compute-testers`, then sets `Service.spec.contactGroupRef` to point at it).
Rationale, largely unchanged from v1: `ContactGroup` has fields
(`Visibility`, `Providers`) that need a real operator decision — not every
service should get a group, and a sane default doesn't exist for whether a
new group syncs to Loops/Resend/etc. or is public vs. private. Auto-creating
groups from service metadata would either (a) require `Service.spec` to
carry enough new fields to fully specify a `ContactGroup` (visibility,
providers — significant schema growth for a feature this narrow) or (b)
create `ContactGroup`s with placeholder defaults an operator then has to
find and fix, which is worse than just creating it themselves once.

**Flagged as a close, non-blocking open question**: an alternative
"auto-create-if-missing" variant where service-catalog creates a minimal
`ContactGroup` (private, no providers configured) the first time a mapped
service enrolls its first contact, and an operator fills in
visibility/providers afterward. This removes one manual step per service but
means service-catalog's controller needs `create` (not just read) RBAC on
Milo's `ContactGroup`, and a `ContactGroup` can transiently exist with no
sync providers configured (silently not reaching Loops/Resend until an
operator notices and configures it) — arguably worse than not having the
group yet. Recommend deferring this variant unless the primary
recommendation proves too much manual toil in practice.

### Identity Resolution

**Finding** (unchanged from v1): `ServiceEntitlement` today carries **no
reference at all** to the requesting user — `ServiceEntitlementSpec` only
has `ServiceRef` and `RequestMessage` (confirmed in
`service-catalog/api/v1alpha1/serviceentitlement_types.go`); there is no
owner reference, no creator annotation, nothing in `status`. This gap must
be closed in service-catalog before this feature can work.

The platform has an established pattern for exactly this problem:
`OrganizationCreatorUserUIDAnnotation` (`resourcemanager.miloapis.com/creator-user-uid`)
is stamped by a **mutating webhook** at create time, reading
`req.UserInfo.UID` off the admission request
(`internal/webhooks/resourcemanager/v1alpha1/organization_webhook.go`, via
an `admission.CustomDefaulter`-style `OrganizationMutator.Default`
registered with `WithDefaulter(...)` alongside `WithValidator(...)`).

service-catalog's `ServiceEntitlement` webhook
(`internal/webhook/v1alpha1/serviceentitlement_webhook.go`) is currently
**validating-only** (`ctrl.NewWebhookManagedBy(mgr, &servicesv1alpha1.ServiceEntitlement{}).WithValidator(webhook).Complete()`)
— no defaulter/mutator is registered.

**Decision (unchanged from v1, now consumed by service-catalog's own
controller instead of Milo's)**: Add a mutating admission path to the
`ServiceEntitlement` webhook that stamps a new annotation,
`services.miloapis.com/creator-user-uid`, from `req.UserInfo.UID` at create
time, following the `organization_webhook.go` pattern
(`WithDefaulter(&serviceEntitlementMutator{})` added to
`SetupServiceEntitlementWebhookWithManager`). `UserInfo.UID` corresponds to
the Milo `User` resource name, matching `SubjectReference{APIGroup:
iam.miloapis.com, Kind: User, Name: <uid>}` used by `Contact.spec.subject`.

The only thing that changes from v1 here is *who reads the annotation*: v1
had Milo's controller-manager read it over a cross-cluster unstructured
watch; v2 has it read in-process by service-catalog's own
`ServiceEntitlementContactEnrollmentReconciler`, which already has the
`ServiceEntitlement` object in hand from its own typed watch — no
unstructured/GVK shim needed at all, since service-catalog already imports
its own `ServiceEntitlement` type natively.

**Consequence for FR3 (Contact may not exist yet)**: unchanged from v1. A
`User` signing up for an entitlement does not guarantee a `Contact` already
exists — `Contact` creation stays owned by the existing staff-portal/CRM-sync
flow, unaffected by this design. `ServiceEntitlementContactEnrollmentReconciler`
therefore:

1. Resolves the `Contact` in Milo by field-indexing on `spec.subject.name`
   (`SubjectRef.Name == creatorUID`, `SubjectRef.Kind == "User"`), the same
   selectable field `Contact` already declares
   (`+kubebuilder:printcolumn:name="SubjectRef",...JSONPath=".spec.subject.name"`).
   Since this lookup now runs from service-catalog against Milo's API rather
   than in Milo's own cache, it is a `List` call with
   `client.MatchingFields` against a controller-runtime cache that
   service-catalog's manager maintains for Milo's `Contact` kind (requires
   registering an index the same way service-catalog already indexes
   `Service.spec.serviceName` in its own `rootMgr`, plus adding
   `notificationv1alpha1.AddToScheme(scheme)` to service-catalog's scheme
   registration in `cmd/services/main.go`).
2. If no `Contact` is found, the controller does **not** create one itself
   (out of scope — Contact creation/CRM-sync ownership stays with the
   existing flow) and instead **requeues with backoff**, so that once a
   `Contact` eventually appears the enrollment completes.

### Cross-Cluster / Cross-Repo Signal Design

v1's premise for this section — that Milo's controller-manager needed to
cross-cluster watch `ServiceEntitlement` objects living in per-project
virtual control planes — is gone entirely under the new direction, because
the reconciler now lives where `ServiceEntitlement` is already reconciled.

**Finding**: `ServiceEntitlementReconciler` (service-catalog,
`internal/controller/serviceentitlement_controller.go`) already runs a
multicluster-runtime controller engaged with every consumer project cluster
(`mcbuilder.WithEngageWithProviderClusters(true)`), and it already holds a
`rootClient` — `rootMgr.GetClient()`, wired in `SetupWithManager(mcMgr
mcmanager.Manager, rootMgr ctrl.Manager)` — used today to read cluster-scoped
`Service` objects. Confirmed in `cmd/services/main.go`: `rootMgr` (`mgr` in
`main()`) is built from `serverConfig.RestConfig()`, which points directly
at Milo's control plane ("Services resources live in the Milo control
plane, not in the cluster that hosts the controller pod. Connect directly
to Milo..." — see the comment above `cfg, err := serverConfig.RestConfig()`
in `cmd/services/main.go`). This is the **same physical API surface** that
hosts Milo's `notification.miloapis.com` group (`Contact`, `ContactGroup`,
`ContactGroupMembership`), per v1's own finding that those live in "etcd
(Milo root)."

**Decision**: No new cross-cluster wiring is needed for the trigger side.
`ServiceEntitlementContactEnrollmentReconciler` either:

- reuses the exact same `rootClient` `ServiceEntitlementReconciler` already
  has (simplest — one client, already RBAC'd for reads against Milo's root
  API, just needs write RBAC on `ContactGroupMembership` and read RBAC on
  `Contact`/`ContactGroup` added, plus registering
  `notificationv1alpha1` in service-catalog's scheme so the client/cache can
  decode those types), or
- runs as a logically separate reconciler sharing the same watch source
  (`For(&servicesv1alpha1.ServiceEntitlement{}, mcbuilder.WithEngageWithProviderClusters(true))`)
  but constructed with its own narrower client, if the team prefers to keep
  `ServiceEntitlementReconciler`'s RBAC surface from growing to include
  `notification.miloapis.com`.

Recommended: a **separate reconciler** (own `Reconcile` function, own
`SetupWithManager` registration, own RBAC kubebuilder markers) rather than
folding this logic into `ServiceEntitlementReconciler.Reconcile` itself —
keeps the two concerns (entitlement lifecycle vs. CRM enrollment)
independently testable and independently retryable, and keeps
`ServiceEntitlementReconciler`'s already-large `Reconcile` function from
growing further. It reuses `rootMgr.GetClient()` (same underlying client
object as `ServiceEntitlementReconciler.rootClient`) rather than opening a
second connection to Milo.

No unstructured/GVK-shim reads are needed anywhere in this design: service-
catalog already has native, generated Go types for `ServiceEntitlement` and
`Service` (they're its own API group), and per NFR1, importing Milo's
`notification.miloapis.com` types is a normal, already-established
dependency direction, not a new one.

### API Definitions

```go
// service-catalog: api/v1alpha1/serviceentitlement_types.go (annotation, not a schema field)
// Unchanged from v1.

const (
    // ServiceEntitlementCreatorUserUIDAnnotation stores the Milo User resource
    // name of the identity that created this ServiceEntitlement, stamped by
    // the mutating admission webhook from req.UserInfo.UID. Read-only outside
    // the webhook; not defaulted on update.
    ServiceEntitlementCreatorUserUIDAnnotation = "services.miloapis.com/creator-user-uid"
)
```

```go
// service-catalog: api/v1alpha1/service_types.go
// New in v2: the service -> ContactGroup mapping (Option A, recommended).

// ContactGroupReference names a Milo ContactGroup (notification.miloapis.com)
// this Service's registrants should be enrolled into. The reference is by
// name only; ContactGroup is cluster-scoped in Milo's root control plane,
// same trust/storage domain Service.spec.owner.ProducerProjectRef already
// crosses into for project references.
type ContactGroupReference struct {
    // Name is the metadata.name of the target notification.miloapis.com
    // ContactGroup in Milo. Not validated against a live ContactGroup list
    // at admission time (see Security Considerations) -- a name with no
    // matching ContactGroup simply never enrolls anyone, silently.
    //
    // +kubebuilder:validation:Required
    // +kubebuilder:validation:MaxLength=253
    Name string `json:"name"`
}

// Added to ServiceSpec:
//
// ContactGroupRef optionally names a Milo ContactGroup that a registering
// user's Contact should be enrolled into once their ServiceEntitlement for
// this Service becomes Active. Nil means no CRM enrollment happens for this
// service.
//
// +kubebuilder:validation:Optional
// ContactGroupRef *ContactGroupReference `json:"contactGroupRef,omitempty"`
```

```go
// service-catalog: internal/controller/serviceentitlement_contact_enrollment_controller.go
// New reconciler, sketch only -- see Implementation Plan for detail.

// ServiceEntitlementContactEnrollmentReconciler enrolls a ServiceEntitlement's
// creator Contact into the Service's mapped ContactGroup once the entitlement
// is Active. It runs alongside ServiceEntitlementReconciler, engaged with the
// same project clusters, and writes to Milo's notification.miloapis.com API
// via rootClient -- the same client ServiceEntitlementReconciler already uses
// to read Service from Milo's root control plane.
type ServiceEntitlementContactEnrollmentReconciler struct {
    rootClient client.Client // Milo root/base API: Service, Contact, ContactGroup, ContactGroupMembership
    Manager    mcmanager.Manager
    Scheme     *runtime.Scheme
}
```

No changes at all to Milo's
`pkg/apis/notification/v1alpha1/contactgroupenrollmentpolicy_types.go`. The
`EnrollmentTriggerEntitlementRegistered` constant and
`EnrollmentEntitlementSelector` type proposed in v1 are dropped.

### Storage Design

No new resource kinds in Milo. `Contact`, `ContactGroup`, and
`ContactGroupMembership` remain exactly as they are today, in Milo's root
etcd. `Service` in service-catalog gains one optional field
(`spec.contactGroupRef`) — no new index needed on it beyond the existing
`spec.serviceName` index `ServiceEntitlementReconciler` already registers,
since the new reconciler resolves `Service` via the same `resolveService`
helper.

service-catalog's manager needs one new field index, registered on
`rootMgr.GetFieldIndexer()` (the same indexer `ServiceEntitlementReconciler.SetupWithManager`
already uses for `Service.spec.serviceName`):

- `Contact` by `spec.subject.name`, mirroring the index
  `ContactGroupEnrollmentController.SetupWithManager` registers in Milo
  today for `ContactGroupMembershipRemoval` — same pattern, different
  process. Note this is a controller-runtime *cache* index local to
  service-catalog's own manager process, separate from Milo's own in-process
  cache index of the same field (both processes maintain independent
  informer caches against the same underlying `Contact` collection).

### Event Processing

**Event sources**:
- `ServiceEntitlement` status updates, watched by
  `ServiceEntitlementContactEnrollmentReconciler` across every engaged
  project cluster — the exact same watch mechanism (and, if implemented as a
  second reconciler on the same manager, the same underlying informer) as
  `ServiceEntitlementReconciler`.
- `Contact` creation in Milo, watched cross-repo by service-catalog's
  `rootMgr` cache (`WatchesRawSource(source.TypedKind(rootMgr.GetCache(),
  &notificationv1alpha1.Contact{}, ...))`, the same pattern
  `ServiceEntitlementReconciler` already uses for `WatchesRawSource` against
  `billingv1alpha1.BillingEntitlement`/`Offer`/etc.), to catch up
  entitlements that registered before the Contact existed.
- `Service.spec.contactGroupRef` create/update, watched the same way
  `ServiceEntitlementReconciler` already watches `Service` via
  `mapServiceToServiceEntitlements`, so a mapping added after entitlements
  are already Active re-evaluates them.

**Event flow** (happy path):

1. `ServiceEntitlement` transitions to `status.phase == Active` (set by
   `ServiceEntitlementReconciler`, a separate reconciler/controller).
2. `ServiceEntitlementContactEnrollmentReconciler` reconciles that
   `ServiceEntitlement` (keyed by `{cluster, namespace/name}` via
   `mcreconcile.Request`, same shape as `ServiceEntitlementReconciler`).
3. It resolves the `Service` via `resolveService(ctx, rootClient,
   entitlement.Spec.ServiceRef.Name)` — the exact helper
   `ServiceEntitlementReconciler` already exports/shares.
4. If `Service.Spec.ContactGroupRef == nil`, no-op (this service isn't
   mapped to a CRM group) — mark evaluated, done.
5. Check an idempotency marker on the `ServiceEntitlement` (annotation,
   e.g. `services.miloapis.com/contact-enrollment-evaluated`, mirroring
   where v1 put the trigger's tracking annotation — on the triggering
   object, since a `Contact` may not exist yet at evaluation time). If
   present, skip.
6. Resolve the creator UID from `services.miloapis.com/creator-user-uid`.
   Missing (pre-existing entitlements, or a bypass path) → mark evaluated
   with a `Skipped` reason, do not retry forever.
7. Resolve the `Contact` by subject UID via Milo's API
   (`rootClient.List(ctx, &contactList, client.MatchingFields{"spec.subject.name": creatorUID})`).
   Not found → requeue with backoff, do not mark evaluated yet.
8. Check for a `ContactGroupMembershipRemoval` opt-out for (contact,
   `Service.Spec.ContactGroupRef`) via Milo's API — same helper shape as
   `ContactGroupEnrollmentController.hasOptOut`, reimplemented (or, if
   feasible, imported — see Implementation Notes) in service-catalog.
9. Create `ContactGroupMembership` in Milo (deterministic name, e.g.
   `enrollment-{serviceName}-{contactName}`, mirroring
   `ContactGroupEnrollmentController.membershipName`'s
   `enrollment-{policyName}-{contactName}` scheme with the service's
   canonical name standing in for the policy name), tolerating the same
   webhook-rejection races (`webhookRejectionReason`-equivalent).
10. Mark the entitlement evaluated.

**Handlers / side effects**: create-only, no notification/email sent
directly by this reconciler — `ContactGroupMembership`'s own reconciler
(unchanged, still in Milo) still owns provider sync (Loops/Resend/etc.).

**Retry / failure handling**: standard controller-runtime requeue-on-error.
"Contact doesn't exist yet" is treated as "not yet satisfiable," explicitly
requeued with `RequeueAfter`, not an immediate hot-loop requeue or a
transient-failure error.

### Platform Capability Integrations

| Capability | Integration Point | Details |
|------------|--------------------|---------|
| IAM | `SubjectReference` on `Contact` | Unchanged; enrollment continues to resolve identity via the existing `iam.miloapis.com/User` subject reference convention, now read by service-catalog's client instead of Milo's own controller-manager. |
| IAM | Creator identity stamping | service-catalog's `ServiceEntitlement` mutating webhook reads `req.UserInfo.UID` off the admission request, following the `organization_webhook.go` precedent. No new IAM API surface. |
| Notification (Milo) | `ContactGroupMembership` write | New: service-catalog's controller-manager service account gains cross-repo write RBAC on `notification.miloapis.com/contactgroupmemberships` and read RBAC on `contacts`/`contactgroups`/`contactgroupmembershipremovals`, in Milo's root control plane. This is the key new dependency this design introduces: service-catalog is now a *client* of Milo's notification API, not just a Go-module dependency. |
| Activity | none | No existing activity/event emission hook found in either the current `ServiceEntitlementReconciler` or `ContactGroupEnrollmentController`; this design does not add one, consistent with current practice, flagged as an operability gap in Open Questions. |
| Quota | none | Not applicable; enrollment does not consume quota. |

### Security Considerations

- Authorization model: `ContactGroupMembership` writes remain a
  system-controller responsibility, now performed by service-catalog's
  controller-manager service account against Milo's API rather than Milo's
  own controller-manager against itself. This is a **new trust boundary
  crossing** worth calling out explicitly: today, only Milo's own
  controllers write `notification.miloapis.com` objects as a system
  identity; after this change, service-catalog's controller-manager service
  account also does. It needs a scoped `ClusterRole`/binding on Milo's side
  (`get;list;watch` on `contacts`, `contactgroups`,
  `contactgroupmembershipremovals`; `get;list;watch;create` on
  `contactgroupmemberships`) — no update/delete, matching
  `ContactGroupEnrollmentController`'s own RBAC markers, which also omit
  update/delete on `contactgroupmemberships`.
- Data validation: `Service.spec.contactGroupRef.name` is not validated
  against a live `ContactGroup` list at admission time (service-catalog's
  webhook would need a cross-repo read to Milo just to validate a string) —
  a mismatched name is accepted and simply never enrolls anyone (silent
  no-op), the same tradeoff v1 accepted for
  `EnrollmentEntitlementSelector.serviceName`, just inverted in which repo
  owns the unvalidated string.
- Audit: creator UID stamped by admission is itself an audit-relevant fact;
  no new PII is introduced beyond what `Contact` already stores, sourced
  from the existing `Contact` object, not from the entitlement.
- The creator-UID annotation is attacker-controllable only to the extent any
  `UserInfo` field on an admission request is controlled by the
  authenticating identity — same trust boundary as
  `OrganizationCreatorUserUIDAnnotation` today; webhook-stamped, not
  client-settable (confirm the webhook unconditionally overwrites a
  client-supplied value during implementation, matching the org webhook's
  behavior).
- New cross-repo credential/connection: service-catalog's controller-manager
  already authenticates to Milo's control plane today (it's the same
  `rootClient`/`cfg` used to read `Service`/`Project`), so this design adds
  **RBAC scope**, not a **new credential or connection** — a materially
  smaller security-review surface than v1's proposal to grant Milo's
  controller-manager new read access into every project's virtual control
  plane for `ServiceEntitlement`.

## Edge Cases

- **Entitlement revoked/rejected after enrollment**: **Decision (unchanged):
  do not auto-remove membership in this iteration.** Membership removal
  remains a policy decision the issue thread flagged as a later step, not
  this iteration's scope. `ContactGroupMembershipRemoval` remains the only
  sanctioned path to remove someone, staying manual/opt-out. Flagged in Open
  Questions as needing explicit stakeholder sign-off.
- **Missing Contact at registration time**: handled via bounded requeue, see
  Event Processing. Expected timing gap, not a bug.
- **Missing creator-UID annotation** (entitlements created before this
  ships, or created via a path that bypasses the webhook): treated as
  permanently unresolvable for that object; marked evaluated with a
  `Skipped` status/event rather than retried forever. See Rollout.
- **Existing membership** (contact already enrolled via another policy or
  manually): the deterministic membership name plus get-before-create plus
  webhook-rejection duplicate-tolerance already cover this — no change
  needed, same as v1.
- **Multiple entitlements for the same service across multiple projects for
  the same user**: idempotency key is (service, contact) via the
  deterministic membership name — the same user registering the same
  service in two projects evaluates twice, against two different
  `ServiceEntitlement` objects, but collapses into the same
  `ContactGroupMembership`; the second is a harmless already-exists no-op.
- **Dependency-origin entitlements** (`status.origin == Dependency`, e.g. a
  service pulled in automatically because another service depends on it):
  unchanged decision from v1 — the design does not distinguish origin; a
  mapped service enrolls the user regardless of whether they explicitly
  registered for it or got it transitively. Flagged as an open question in
  case CRM only wants *direct* signups counted.
- **`Service.spec.contactGroupRef` added after entitlements are already
  Active**: covered by the `Service`-update watch triggering re-evaluation
  of matching entitlements (see Event Processing), analogous to v1's
  policy-watch catch-up pass.

## Rollout / Migration Considerations

- No backfill of the creator-UID annotation for pre-existing
  `ServiceEntitlement` objects is planned in this iteration — those simply
  never satisfy enrollment (see Missing creator-UID annotation above). A
  one-off best-effort migration script (e.g. from creation-time audit logs,
  if retained) is tracked separately if ever needed.
- Ship order, all within service-catalog and its RBAC on Milo (no Milo code
  changes to sequence against at all, which is materially simpler than v1's
  cross-repo ship-order dependency): (1) grant service-catalog's
  controller-manager the new RBAC on Milo's `notification.miloapis.com`
  resources, (2) land the mutating webhook (stamp annotation), (3) land the
  `Service.spec.contactGroupRef` field, (4) land
  `ServiceEntitlementContactEnrollmentReconciler`. Steps 1–3 are individually
  safe to ship with no consumer yet; step 4 is the only behavior-changing
  deploy, and only for services an operator has explicitly opted in via
  `contactGroupRef`.
- No `ContactGroupEnrollmentPolicy` CRD change, so no widening/migration on
  that object at all — a simplification relative to v1.
- Feature-flagging: none proposed; enrollment is opt-in per-`Service` (an
  operator must set `contactGroupRef`), so there is no behavior change for
  any existing service until an operator sets the field.

## Implementation Plan

1. **service-catalog**: Add `ServiceEntitlementCreatorUserUIDAnnotation`
   constant; add a mutating admission path (`WithDefaulter(...)`) to
   `serviceentitlement_webhook.go` that stamps `req.UserInfo.UID` on create,
   following `organization_webhook.go`'s `OrganizationMutator.Default`
   pattern. Unit test with a fake admission request, mirroring
   `organization_webhook_test.go`.
2. **service-catalog (types)**: Add `ContactGroupReference` and
   `Service.spec.contactGroupRef` to `api/v1alpha1/service_types.go`;
   regenerate deepcopy/CRD manifests.
3. **service-catalog (scheme + client)**: Register
   `notificationv1alpha1.AddToScheme(scheme)` in `cmd/services/main.go`'s
   scheme `init()`; confirm `rootMgr`'s cache can list/watch
   `notification.miloapis.com` kinds against Milo's control plane
   (`rootMgr` already points there per the confirmed finding above — no new
   connection, just new registered types).
4. **service-catalog (RBAC)**: Add RBAC (kubebuilder markers +
   ClusterRole/binding on Milo's side) granting service-catalog's
   controller-manager service account `get;list;watch` on
   `notification.miloapis.com/{contacts,contactgroups,contactgroupmembershipremovals}`
   and `get;list;watch;create` on `notification.miloapis.com/contactgroupmemberships`.
5. **service-catalog (index)**: Register a field index on `Contact` by
   `spec.subject.name` in `rootMgr.GetFieldIndexer()`, alongside the
   existing `Service.spec.serviceName` index registration in
   `ServiceEntitlementReconciler.SetupWithManager` (or its own
   `SetupWithManager`, if implemented as a fully separate reconciler — see
   Implementation Notes).
6. **service-catalog (controller)**: Implement
   `ServiceEntitlementContactEnrollmentReconciler` in
   `internal/controller/`, following `ServiceEntitlementReconciler`'s
   `mcbuilder.WithEngageWithProviderClusters(true)` engagement pattern for
   the watch side, and reimplementing (or extracting/sharing, see
   Implementation Notes) `ContactGroupEnrollmentController`'s
   evaluate/opt-out/idempotency helper logic for the enrollment write.
7. **Tests** (test-engineer): table-driven unit tests for the
   `Service.spec.contactGroupRef` resolution, idempotency (annotation on
   entitlement), missing-Contact requeue behavior, opt-out interaction
   against Milo's `ContactGroupMembershipRemoval`, and an integration/
   envtest exercising the full watch → cross-repo membership-create flow
   (a fake/embedded Milo API surface or a real envtest instance running
   Milo's CRDs, matching how `ServiceEntitlementReconciler`'s existing tests
   already need a `Service`-serving root client).
8. **Docs/rollout**: communicate the "no auto-removal on revocation" scope
   cut to requesting stakeholders (support/success/sales) before shipping;
   no Milo-side deploy-ordering coordination is needed (a meaningful
   simplification over v1).

## Future Considerations

- Auto-creation of a `ContactGroup` per service (scotwells'/issue author's
  original suggestion) — evaluated in this revision (see ContactGroup
  Creation/Linking) and still deferred as a variant, pending evidence the
  manual step is actually a meaningful burden.
- Membership removal on entitlement revocation/rejection — deferred, needs
  a product decision on whether removal should be automatic, and whether it
  should be a hard delete or converted into a suppress/opt-out record.
- Letting users set contact info per project/org to drive membership
  (mentioned in the issue thread) is a materially different data flow and
  stays out of scope for this design entirely.
- If service-catalog ends up needing several more Milo-API-writing
  controllers like this one, consider whether a small shared internal
  package of Milo notification-API helpers (opt-out check, deterministic
  naming, webhook-rejection classification) belongs in service-catalog
  itself, rather than each controller reimplementing
  `ContactGroupEnrollmentController`'s helpers independently.

## Handoff

### Decisions Made

- **Direction flip (the core decision this revision makes)**: service-catalog
  drives enrollment end-to-end; Milo's `ContactGroupEnrollmentPolicy`/
  `ContactGroupEnrollmentController` are untouched and gain no new trigger
  type. Per Scot's stated principle, Milo's core stays ignorant of
  "services" and "entitlements," the same way billing doesn't know what a
  service is.
- The service → `ContactGroup` mapping lives on `Service.spec.contactGroupRef`
  (Option A), not a separate mapping CRD — smallest schema addition,
  reuses `Service` resolution `ServiceEntitlementReconciler` already does.
- `ContactGroup` creation stays manual/operator-driven, matching v1's
  decision; auto-create-if-missing is flagged as a close but deferred
  alternative.
- Identity resolution still requires the service-catalog
  `ServiceEntitlement` mutating webhook change (creator-UID annotation),
  unchanged from v1 — this is a **required prerequisite**, not optional.
  The consumer of the annotation is now service-catalog's own new
  reconciler, not Milo's controller-manager.
- No cross-cluster read is added to Milo's controller-manager at all under
  this revision — the entire cross-cluster complexity v1 introduced (Milo
  watching `ServiceEntitlement` in every project cluster via
  unstructured/GVK) is eliminated, since service-catalog already runs that
  watch for its own reconciliation needs.
- Entitlement revocation does not auto-remove contact group membership in
  this iteration — unchanged, explicit scope cut.
- Idempotency-tracking annotation lives on the `ServiceEntitlement` object
  (service-catalog's own object, easy to annotate directly), not on
  `Contact` in Milo, consistent with v1's rationale (a `Contact` may not
  exist yet when the trigger fires).
- Confirmed via code inspection: service-catalog's `ServiceEntitlementReconciler.rootClient`
  (`rootMgr.GetClient()`, wired from a `rest.Config` pointed directly at
  Milo's control plane per `cmd/services/main.go`) is the same client/cluster
  that would host `ContactGroupMembership` — no second connection or
  credential is needed, only expanded RBAC and scheme registration.

### Open Questions

- **(Non-blocking, close call)** Should the service → `ContactGroup` mapping
  live on `Service.spec` (recommended) or a separate mapping resource in
  service-catalog? Flagged as genuinely close in case the service-catalog
  team has a standing convention against cross-repo-referencing optional
  fields on `Service`.
- **(Non-blocking, product decision)** Should `ContactGroup` auto-create-if-
  missing be pursued instead of manual-with-reference? Recommend starting
  manual and revisiting if it proves to be meaningful operator toil.
- **(Non-blocking, product decision)** Should dependency-origin entitlements
  (`status.origin == Dependency`) count toward enrollment, or only direct
  (`status.origin == Direct`) registrations? Unchanged from v1; this design
  enrolls on any Active entitlement regardless of origin.
- **(Non-blocking, scope confirmation)** Confirm with stakeholders that "no
  automatic membership removal on revocation" is acceptable for this
  iteration — the issue thread didn't explicitly rule on this.
- **(Non-blocking)** What is the maximum backoff window before the
  controller stops retrying a `ServiceEntitlement` whose `Contact` never
  appears, and how should that be surfaced (Kubernetes Event, or a status
  condition on the `ServiceEntitlement`, which service-catalog *does* own
  now, unlike v1 where this would have required a new field on an object
  Milo doesn't own)? Being service-catalog's own object removes v1's
  blocking objection here — recommend a capped-interval unbounded requeue
  (e.g. 10 minutes) plus a `ContactEnrollmentPending`-style condition on the
  entitlement's own status, but confirm the exact condition shape during
  implementation.
- **(Non-blocking)** Should `Service.spec.contactGroupRef.name` be validated
  against a live `ContactGroup` list in Milo at admission time (a real
  cross-repo lookup from a webhook), or accepted as free text? Recommend
  free text for this iteration — a mismatched name silently never matches,
  a lesser failure mode than blocking valid `Service` updates on a fragile
  cross-cluster admission-time lookup.

### Implementation Notes

- api-dev: Start with the service-catalog webhook change — it's the
  prerequisite everything else depends on, small and isolated, directly
  modeled on `organization_webhook.go` (read that file plus
  `organization_webhook_test.go` first).
- api-dev: When wiring `rootMgr`'s scheme/cache to also serve
  `notification.miloapis.com` types, double check
  `ClusterOptions: []cluster.Option{func(o *cluster.Options) { o.Scheme = scheme }}`
  in `cmd/services/main.go` — the multicluster provider's per-project
  cluster scheme is set separately from `rootMgr`'s own scheme, and only
  `rootMgr`'s scheme needs the notification types (they only ever live in
  Milo's root control plane, never in a project cluster).
- api-dev: Decide early whether
  `ServiceEntitlementContactEnrollmentReconciler` is a genuinely separate
  reconciler (recommended, see Cross-Cluster/Cross-Repo Signal Design) or
  folded into `ServiceEntitlementReconciler.Reconcile` — this affects
  whether `ContactGroupEnrollmentController`'s opt-out/idempotency helper
  logic gets reimplemented once (separate reconciler, its own small helper
  file) or gets tangled into the existing large `Reconcile` function
  (avoid).
- api-dev: `ContactGroupEnrollmentController`'s helpers (`hasOptOut`,
  `ensureMembership`, `webhookRejectionReason`, `membershipName`) are
  unexported and live in Milo's `internal/controllers/notification` package
  — they cannot be imported directly from service-catalog. Reimplement the
  same logic in service-catalog against the imported `notificationv1alpha1`
  types rather than trying to share code across the module boundary; keep
  the reimplementation intentionally close to the original so a future
  reader can diff them.
- test-engineer: The most valuable test is still "Contact created after
  entitlement is already Active" (FR3) — the one path with no equivalent in
  `ServiceEntitlementReconciler`'s existing tests and the most likely place
  for a bug (e.g. forgetting to also watch `Contact` creation in Milo's
  cache and re-scan pending entitlements).
- test-engineer: Also test the cross-project double-registration collapse
  case explicitly (same user, same service, two projects), since the
  deterministic membership name is the only thing preventing a duplicate
  membership object there.
- test-engineer: Any integration test exercising the cross-repo write needs
  Milo's `notification.miloapis.com` CRDs available in the test environment
  (envtest or equivalent) — confirm whether service-catalog's existing test
  harness already loads any Milo CRDs (it must, for `resourcemanagerv1alpha1.Project`
  reads) and extend that same fixture rather than building a new one.
