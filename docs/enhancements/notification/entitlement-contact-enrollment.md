# Entitlement Registration Contact Group Enrollment

> Status: Draft, seeking review. Source issue: datum-cloud/cloud-portal#1497.

## Overview

When a customer registers for a service entitlement (e.g. enables the Compute
service via a `ServiceEntitlement` in service-catalog), internal staff want the
requesting user's CRM contact record automatically added to a corresponding
staff-portal Contact Group (e.g. `compute-testers`), without manual list
maintenance.

This design extends Milo's existing `ContactGroupEnrollmentPolicy` /
`ContactGroupEnrollmentController` mechanism (group `notification.miloapis.com`)
with a new trigger type, `EntitlementRegistered`, rather than building a
bespoke controller inside service-catalog. Enrolling a new entitlement type
into a contact group becomes a matter of creating a policy object, not writing
new controller code.

Source: GitHub issue datum-cloud/cloud-portal#1497.

## Requirements

### Functional Requirements

- FR1: An operator can declare, via a `ContactGroupEnrollmentPolicy`, that
  registering for a given service (identified by canonical service name)
  should enroll the registering user's `Contact` into a given `ContactGroup`.
- FR2: When a `ServiceEntitlement` for a mapped service becomes `Active` in any
  project, the corresponding user's `Contact` is enrolled (a
  `ContactGroupMembership` is created), exactly once per (contact, policy)
  pair, matching the idempotency guarantee the existing `ContactCreated`
  trigger provides.
- FR3: If no `Contact` exists yet for the registering user at the time of
  registration, the enrollment is not lost — it completes once the `Contact`
  is created (or once identity resolution succeeds), without requiring the
  user to re-register.
- FR4: Existing `ContactGroupMembershipRemoval` opt-out records continue to
  suppress enrollment for this trigger type, same as `ContactCreated`.
- FR5: The design states explicit, intentional behavior for entitlement
  revocation/rejection (see Edge Cases) — this rollout does NOT remove
  membership automatically; that is called out as a v2 follow-up, not silently
  omitted.

### Non-Functional Requirements

- NFR1: The new trigger must not introduce a hard Go module dependency from
  Milo onto service-catalog's API package (see Decisions Made) so the two
  repos can version independently, matching the existing one-directional
  dependency (service-catalog → milo, never the reverse).
- NFR2: Cross-cluster watch overhead scales with the number of engaged project
  clusters, consistent with existing multicluster-runtime controllers in both
  repos (e.g. `ServiceEntitlementReconciler`).
- NFR3: Enrollment latency target: contact should land in the group within a
  few minutes of the entitlement becoming Active (bounded by cache resync, not
  polling).
- NFR4: No change to today's authorization model: enrollment is a
  system-controller write (`enrollmentFieldOwner`), not something a project
  admin can trigger directly.

## Design

### Resource Types

| Resource | Group | Storage | Description |
|----------|-------|---------|--------------|
| `ContactGroupEnrollmentPolicy` (extended) | notification.miloapis.com | etcd (Milo root) | Adds `EntitlementRegistered` trigger type and an `EntitlementSelector`. Existing type, no new kind. |
| `ServiceEntitlement` (read-only signal source) | services.miloapis.com | etcd (per-project virtual control plane, service-catalog) | Unmodified schema. Its `status.phase == Active` plus a creator annotation (new, see below) drive enrollment. |
| `Contact` (read/write) | notification.miloapis.com | etcd (Milo root) | Enrollment target's identity anchor; may need to be created if missing (see Identity Resolution). |

No new CRDs. Two schema-level additions:

1. Milo: extend `ContactGroupEnrollmentPolicy.spec.trigger` and add an
   `EntitlementSelector` type.
2. service-catalog: add a creator-identity annotation to `ServiceEntitlement`
   at admission time (currently absent — see Identity Resolution below).

### Cross-Cluster / Cross-Repo Signal Design

**Finding**: `ServiceEntitlement` objects live in per-project virtual control
planes, the same physical fleet that Milo's own controller-manager already
connects to via `pkg/multicluster-runtime/milo` (the Milo provider watches
`Project`/`ProjectControlPlane` to discover and engage project clusters — see
`internal/controllers/resourcemanager/project_controller.go` and the
multicluster-runtime README). service-catalog's `ServiceEntitlementReconciler`
uses this same mechanism from the other repo's binary. There is no need for
service-catalog to "emit" anything Milo watches over a queue/webhook — Milo's
controller-manager can engage the same project clusters directly.

**Decision**: Milo's controller-manager gains a new multicluster-runtime
controller, `EntitlementEnrollmentController`, that watches `ServiceEntitlement`
objects across all engaged project clusters, the same way service-catalog's
`ServiceEntitlementReconciler` does (`mcbuilder.WithEngageWithProviderClusters` /
equivalent engagement mode — to confirm which engagement mode fits Milo's
controller-manager topology, since it is not itself a "provider" in
service-catalog's sense; likely the plain per-project engagement Milo already
uses for other cross-cutting controllers such as quota).

**Avoiding a hard Go dependency (NFR1)**: Rather than importing
`go.miloapis.com/service-catalog/api/v1alpha1` into Milo (which would invert
today's one-directional dependency — service-catalog depends on milo, not vice
versa — and couple the two repos' release cadence), the new controller watches
`ServiceEntitlement` as `unstructured.Unstructured` / a locally-declared
minimal typed shim (GVK `services.miloapis.com/v1alpha1, Kind=ServiceEntitlement`,
reading only `status.phase`, `status.serviceName`, and the creator annotation
via unstructured field accessors). This mirrors how cross-group signals are
already read via `client.MatchingFields` field indexes and
`WatchesRawSource`/`handler.TypedEnqueueRequestsFromMapFunc` patterns elsewhere
in both repos, just applied to a dynamic/unstructured client instead of a
typed one. RBAC: Milo's controller-manager service account needs
`get;list;watch` on `services.miloapis.com/serviceentitlements` in every
project cluster it's engaged with — an additive ClusterRole/role binding,
scoped read-only.

This is flagged as an **open decision requiring human/architecture-team
sign-off**: whether Milo may take a compile-time dependency on
service-catalog's generated types instead (simpler code, less duplication,
but couples deploy ordering and versioning). The unstructured approach is
recommended as the default; a light typed GVK-constants package
(`services.miloapis.com` group name + kind string, no schema) can live in
Milo without pulling in service-catalog's module.

### Identity Resolution

**Finding**: `ServiceEntitlement` today carries **no reference at all** to
the requesting user — `ServiceEntitlementSpec` only has `ServiceRef` and
`RequestMessage`; there is no owner reference, no creator annotation, nothing
in `status`. This is a real gap that must be closed in service-catalog before
this feature can work, it is not solvable purely from Milo's side.

The platform has an established pattern for exactly this problem:
`OrganizationCreatorUserUIDAnnotation` (`resourcemanager.miloapis.com/creator-user-uid`)
is stamped by a **mutating webhook** at create time, reading
`req.UserInfo.UID` off the admission request
(`internal/webhooks/resourcemanager/v1alpha1/organization_webhook.go:78`),
and later consumed by a controller
(`internal/controllers/resourcemanager/organization_bootstrap.go`). Quota's
`ClaimCreationPolicy` uses an equivalent `quota.miloapis.com/created-by`
annotation convention.

service-catalog's `ServiceEntitlement` webhook
(`internal/webhook/v1alpha1/serviceentitlement_webhook.go`) is currently
**validating-only** (`ctrl.NewWebhookManagedBy(...).WithValidator(...)`) — no
defaulter/mutator is registered.

**Decision**: Add a mutating admission path to the ServiceEntitlement webhook
that stamps a new annotation, `services.miloapis.com/creator-user-uid`, from
`req.UserInfo.UID` at create time, following the `organization_webhook.go`
pattern exactly (`admission.CustomDefaulter` or equivalent added alongside the
existing `admission.Validator`). `UserInfo.UID` corresponds to the Milo `User`
resource name, matching `SubjectReference{APIGroup: iam.miloapis.com, Kind:
User, Name: <uid>}` used by `Contact.spec.subject`.

**Consequence for FR3 (Contact may not exist yet)**: A `User` signing up for
an entitlement does not guarantee a `Contact` already exists — `Contact` is
presently created by whatever staff-portal/CRM-sync flow exists today
(manually, or by some other onboarding trigger), and this design does not
change that. The `EntitlementEnrollmentController` therefore:

1. Resolves the `Contact` by field-indexing `Contact` on `spec.subject.name`
   (`SubjectRef.Name == creatorUID`, `SubjectRef.Kind == "User"`), mirroring
   the `.spec.subject.name` selectable field already declared on `Contact`.
2. If no `Contact` is found, the controller does **not** create one itself
   (out of scope — Contact creation/CRM-sync ownership stays with the existing
   flow) and instead **requeues with backoff** rather than dropping the
   signal, so that once a `Contact` eventually appears the enrollment
   completes. This bounds worst-case staleness rather than silently losing
   the signal. (Open question below: is there a maximum bound after which
   the controller should give up and surface a condition/event for
   visibility?)

### API Definitions

```go
// milo: pkg/apis/notification/v1alpha1/contactgroupenrollmentpolicy_types.go

const (
    EnrollmentTriggerContactCreated       = "ContactCreated"
    // EnrollmentTriggerEntitlementRegistered fires when a ServiceEntitlement
    // for the referenced service becomes Active in any project.
    EnrollmentTriggerEntitlementRegistered = "EntitlementRegistered"
)

type EnrollmentTrigger struct {
    // +kubebuilder:validation:Enum=ContactCreated;EntitlementRegistered
    // +kubebuilder:validation:Required
    Type string `json:"type"`

    // EntitlementSelector is required when Type is EntitlementRegistered and
    // must be omitted otherwise (enforced by the ContactGroupEnrollmentPolicy
    // validating webhook, mirroring ContactSelector's existing optionality).
    // +kubebuilder:validation:Optional
    EntitlementSelector *EnrollmentEntitlementSelector `json:"entitlementSelector,omitempty"`
}

// EnrollmentEntitlementSelector filters which ServiceEntitlement registrations
// activate this policy.
type EnrollmentEntitlementSelector struct {
    // ServiceName is the canonical service name (ServiceEntitlement's
    // status.serviceName, e.g. "compute.datumapis.com") this policy reacts to.
    // +kubebuilder:validation:Required
    // +kubebuilder:validation:MaxLength=253
    ServiceName string `json:"serviceName"`
}
```

```go
// service-catalog: api/v1alpha1/serviceentitlement_types.go (annotation, not a schema field)

const (
    // ServiceEntitlementCreatorUserUIDAnnotation stores the Milo User resource
    // name of the identity that created this ServiceEntitlement, stamped by
    // the mutating admission webhook from req.UserInfo.UID. Read-only outside
    // the webhook; not defaulted on update.
    ServiceEntitlementCreatorUserUIDAnnotation = "services.miloapis.com/creator-user-uid"
)
```

### Storage Design

No new resource kinds. `ContactGroupEnrollmentPolicy` remains cluster-scoped
in Milo's root etcd. The annotation lives on `ServiceEntitlement` objects in
each project's virtual control plane (service-catalog's existing storage). No
new indexes are required in service-catalog. Milo adds:

- A field index on `Contact` by `spec.subject.name` (if not already present —
  `.spec.subject.name` is declared `+kubebuilder:selectablefield`, but the
  in-process controller-runtime cache index used by
  `client.MatchingFields` is separate from the apiserver-level selectable
  field and must be registered explicitly in `SetupWithManager`, same as
  the existing `removalContactRefNameIndexKey` index).
- No index needed on `ServiceEntitlement` inside Milo's own cache beyond the
  default watch, since the controller reconciles per-object on watch events,
  the same shape as `ContactGroupEnrollmentController` reconciling per-`Contact`.

### Event Processing

**Event sources**:
- `ServiceEntitlement` status updates, watched cross-cluster by the new
  `EntitlementEnrollmentController` (Milo controller-manager), engaging every
  project cluster the same way `ServiceEntitlementReconciler` does.
- `ContactGroupEnrollmentPolicy` create/update, to pick up newly declared
  entitlement-trigger policies against entitlements that already exist and
  are already Active (mirrors `findContactsForPolicy` in the existing
  controller, generalized to also list `ServiceEntitlement`s across engaged
  clusters for `EntitlementRegistered` policies).
- `Contact` creation, to catch up entitlements that registered before the
  Contact existed (the controller re-evaluates outstanding
  `EntitlementRegistered` policies for a newly created Contact whose
  subject UID matches a pending, unenrolled entitlement).

**Event flow** (happy path — see Sequence below for full detail):

1. `ServiceEntitlement` transitions to `status.phase == Active`.
2. `EntitlementEnrollmentController` reconciles that `ServiceEntitlement`
   (keyed by `{cluster, namespace/name}` via `mcreconcile.Request`, following
   `ServiceEntitlementReconciler`'s shape).
3. It lists `ContactGroupEnrollmentPolicy` objects (Milo root client) whose
   `trigger.type == EntitlementRegistered` and whose
   `entitlementSelector.serviceName == entitlement.status.serviceName`.
4. For each matching, unevaluated policy (idempotency annotation check, same
   convention as today: `enrollment.notification.miloapis.com/{policyName}`,
   but stamped on the **ServiceEntitlement**, not the Contact, since this
   trigger's identity anchor is the entitlement, and a Contact does not exist
   at evaluation time in the not-yet-created case):
   - Resolve the creator UID from the entitlement's
     `services.miloapis.com/creator-user-uid` annotation. Missing annotation
     (e.g. pre-existing entitlements created before this feature shipped) is
     treated as "cannot resolve identity" — mark evaluated with a `Skipped`
     reason recorded in an event, do not error/retry forever (see Migration).
   - Resolve the `Contact` by subject UID. Not found → requeue with backoff
     (bounded, see Open Questions), do not mark evaluated yet.
   - Check `ContactGroupMembershipRemoval` opt-out for (contact, policy's
     `contactGroupRef`) — same helper as today.
   - Create `ContactGroupMembership` (same deterministic name scheme:
     `enrollment-{policyName}-{contactName}`), same webhook-rejection
     tolerance (`webhookRejectionReason`) for idempotent duplicate/opt-out
     races.
   - Mark the policy evaluated on the entitlement.

**Handlers / side effects**: identical to today — create-only, no
notification/email sent directly by this controller (contact provider sync,
e.g. Loops/Resend, is `ContactGroupMembership`'s own reconciler's job,
unchanged).

**Retry / failure handling**: standard controller-runtime requeue-on-error.
The one new wrinkle is the "Contact doesn't exist yet" case, which is not an
error — it is treated as "not yet satisfiable" and explicitly requeued
(`RequeueAfter`, not an immediate hot-loop requeue) rather than treated as a
transient failure.

### ContactGroup Creation/Linking

Per scotwells' comment, the issue anticipated a service-catalog controller
that "auto-creates" the contact group. This design keeps `ContactGroup`
creation **out of scope and manual**, matching how `ContactGroupEnrollmentPolicy`
already requires a pre-existing `ContactGroupRef` today (no auto-create exists
for the `ContactCreated` trigger either). Rationale: auto-creating groups from
service metadata conflates "does this service exist" with "should staff track
its testers in CRM" — not every service entitlement should get a group, and
staff-portal's `ContactGroup` has fields (`Visibility`, `Providers`) that
require an operator decision, not a sane default. An operator creates the
`ContactGroup` (e.g. `compute-testers`) via staff-portal's existing manual UI,
then creates a `ContactGroupEnrollmentPolicy` referencing it. This can be
revisited as a v2 (see Future Considerations).

### Platform Capability Integrations

| Capability | Integration Point | Details |
|------------|--------------------|---------|
| IAM | `SubjectReference` on `Contact` | Unchanged; enrollment continues to resolve identity via the existing `iam.miloapis.com/User` subject reference convention. |
| IAM | Creator identity stamping | New: service-catalog's `ServiceEntitlement` mutating webhook reads `req.UserInfo.UID` off the admission request, following the `organization_webhook.go` precedent — no new IAM API surface. |
| Activity | none | No existing activity/event emission hook found in either controller (`ContactGroupEnrollmentController` doesn't emit k8s Events either); this design does not add one, consistent with current practice, but is called out as a NFR gap for operability (see Open Questions). |
| Quota | none | Not applicable; enrollment does not consume quota. |

### Security Considerations

- Authorization model: unchanged from today. `ContactGroupMembership` writes
  remain a system-controller responsibility (`enrollmentFieldOwner`
  equivalent), not exposed to end users or project admins. The new cross-
  cluster read (Milo controller-manager reading `ServiceEntitlement` in every
  project's control plane) is a new, additive read-only RBAC grant scoped to
  `get;list;watch` — no write access to service-catalog resources.
- Data validation: `EntitlementSelector.serviceName` should be validated
  (webhook, mirroring `ValidateServiceEntitlementCreate`'s style) against
  known canonical service names where feasible, though Milo does not have a
  live view of service-catalog's `Service` list without the same
  cross-cluster/unstructured read used for entitlements — likely accept any
  string at admission and let a mismatched name simply never match (silent
  no-op) rather than block policy creation on a live lookup. Flagged as an
  open question below.
- Audit: creator UID stamped by admission is itself an audit-relevant fact;
  no new PII is introduced beyond what `Contact` already stores (email,
  given/family name) sourced from the existing `Contact` object, not from the
  entitlement.
- The creator-UID annotation is attacker-controllable only in the sense that
  any field in an admission request's `UserInfo` is controlled by the
  authenticating identity itself — same trust boundary as
  `OrganizationCreatorUserUIDAnnotation` today, and the annotation is
  webhook-stamped, not client-settable (a client attempting to set it directly
  should have the webhook overwrite it, matching the org webhook's
  unconditional overwrite behavior — confirm this during implementation).

## Edge Cases

- **Entitlement revoked/rejected after enrollment**: **Decision: do not
  auto-remove membership in this iteration.** Membership removal is
  explicitly a policy decision (who decides "no longer a tester"?) that the
  issue thread flagged as a later step ("eventually will want to let users
  set contact info... and let that drive membership"), not this iteration's
  scope. Document this clearly to stakeholders — it is a deliberate scope cut,
  not an oversight. `ContactGroupMembershipRemoval` remains the only sanctioned
  path to remove someone, and it stays a manual/opt-out action. Flagged in
  Open Questions as needing explicit stakeholder sign-off, since silent
  permanent membership after revocation could surprise CRM operators.
- **Missing Contact at registration time**: handled via bounded requeue, see
  Event Processing. Not a bug case, an expected timing gap.
- **Missing creator-UID annotation** (entitlements created before this
  ships, or created via a path that bypasses the webhook, e.g. direct etcd
  restore in tests): treated as permanently unresolvable for that object;
  marked evaluated with a `Skipped` status/event rather than retried forever.
  See Migration/Rollout.
- **Existing membership** (contact already enrolled via another policy or
  manually): `ensureMembership`'s existing get-before-create plus
  `webhookRejectionReason`'s duplicate-tolerance already cover this — no
  change needed.
- **Multiple entitlements for the same service across multiple projects for
  the same user**: idempotency key is (entitlement, policy), not
  (user, policy) — the same user registering the same service in two
  projects evaluates the policy twice, against two different
  `ServiceEntitlement` objects, but `ensureMembership`'s deterministic name
  (`enrollment-{policy}-{contact}`) collapses both into the same
  `ContactGroupMembership`, so the second is a harmless no-op (already-exists
  path).
- **Dependency-origin entitlements** (`status.origin == Dependency`, e.g. a
  service pulled in automatically because another service depends on it):
  the design does **not** distinguish origin — a policy matching the
  dependency service's canonical name enrolls the user regardless of whether
  they explicitly registered for it or got it transitively. This mirrors "the
  user now has access to Compute" regardless of how, which matches the
  issue's framing ("signs up for... entitlement... gets added"). Flagged as
  an open question in case CRM only wants *direct* signups counted.
- **`ContactGroupEnrollmentPolicy` created after entitlements are already
  Active**: covered by the policy-watch catch-up pass (analogous to
  `findContactsForPolicy`), generalized to list `ServiceEntitlement`s too.

## Rollout / Migration Considerations

- No backfill of the creator-UID annotation for pre-existing
  `ServiceEntitlement` objects is planned in this iteration — those simply
  never satisfy `EntitlementRegistered` policies (see Missing creator-UID
  annotation above). If backfilling pre-existing entitlements' user
  attribution is required, that's a one-off migration script (best-effort,
  e.g. from creation-time audit logs if retained) tracked separately, not
  part of this controller's steady-state logic.
- Ship order: service-catalog's webhook change (stamp annotation) must land
  and roll out to all project clusters before Milo's controller depends on
  it; until then, `EntitlementEnrollmentController` simply finds no
  annotation and no-ops (safe to deploy Milo's side first — it fails open
  to "not yet resolvable", never errors).
- The `ContactGroupEnrollmentPolicy` CRD's trigger enum widening
  (`ContactCreated` → `ContactCreated;EntitlementRegistered`) is additive and
  backward compatible; no existing policy object needs migration.
- Feature-flagging: none proposed; the new trigger type is opt-in per-policy
  (an operator must author a new policy object naming
  `EntitlementRegistered` + a service name), so there's no behavior change
  for anyone until an operator creates one.

## Implementation Plan

1. **service-catalog**: Add `ServiceEntitlementCreatorUserUIDAnnotation`
   constant; add a mutating admission path (`admission.CustomDefaulter` or
   equivalent) to `serviceentitlement_webhook.go` that stamps
   `req.UserInfo.UID` on create, following
   `internal/webhooks/resourcemanager/v1alpha1/organization_webhook.go`'s
   pattern. Register the mutating webhook path in `cmd/services/main.go`
   alongside the existing validator registration. Unit test with a fake
   admission request, mirroring `organization_webhook_test.go`.
2. **milo (types)**: Extend `EnrollmentTrigger` with
   `EnrollmentTriggerEntitlementRegistered` and add
   `EnrollmentEntitlementSelector`; regenerate deepcopy/CRD manifests. Add
   validating-webhook rule on `ContactGroupEnrollmentPolicy` requiring
   `EntitlementSelector` iff `trigger.type == EntitlementRegistered` (check
   whether that webhook currently exists — if not, add one; confirm during
   implementation).
3. **milo (light GVK shim)**: Add a small package
   (e.g. `pkg/apis/services/v1alpha1` or an `unstructured`-based helper in the
   new controller's package) declaring the GVK
   `services.miloapis.com/v1alpha1, Kind=ServiceEntitlement` and the field
   names/annotation key needed, without importing service-catalog's module.
4. **milo (controller)**: Implement `EntitlementEnrollmentController` in
   `internal/controllers/notification/`, following
   `ServiceEntitlementReconciler`'s multicluster engagement pattern for the
   watch side and `ContactGroupEnrollmentController`'s evaluate/idempotency
   pattern for the enrollment logic. Add the `Contact` field index on
   `spec.subject.name`. Wire into the controller-manager's
   `SetupWithManager` alongside the existing enrollment controller.
5. **milo (RBAC)**: Add ClusterRole/binding granting Milo's
   controller-manager service account `get;list;watch` on
   `services.miloapis.com/serviceentitlements` in project control planes.
6. **Tests** (test-engineer): table-driven unit tests for policy matching
   (service name selector), idempotency (annotation on entitlement, not
   contact), missing-Contact requeue behavior, opt-out interaction, and an
   integration/envtest exercising the full watch → membership-create flow
   across at least two simulated project clusters.
7. **Docs/rollout**: coordinate deploy order (service-catalog webhook before
   Milo controller enablement), and communicate the "no auto-removal on
   revocation" scope cut to the requesting stakeholders (support/success/
   sales) before shipping.

## Future Considerations

- Auto-creation of a `ContactGroup` per service (scotwells' original
  suggestion) — deferred; revisit once there's a clear default policy for
  visibility/provider settings per service.
- Membership removal on entitlement revocation/rejection — deferred, needs a
  product decision on whether removal should be automatic, and whether it
  should be a hard delete or converted into a suppress/opt-out record.
- Letting users set contact info per project/org to drive membership
  (mentioned in the issue thread) is a materially different data flow
  (project/org-level contact info, not user-identity-level) and is out of
  scope for this design entirely.

## Handoff

### Decisions Made

- Extend `ContactGroupEnrollmentPolicy`'s existing trigger model with
  `EntitlementRegistered` rather than a new CRD or a bespoke service-catalog
  controller — keeps one enrollment mechanism platform-wide (per prior
  product decision, not re-litigated here).
- Milo's new controller reads `ServiceEntitlement` cross-cluster via
  unstructured/GVK, not a compile-time dependency on service-catalog's Go
  module — preserves the current one-directional repo dependency direction.
- Identity resolution requires a new service-catalog webhook change
  (creator-UID annotation stamped at admission), following the exact
  precedent of `OrganizationCreatorUserUIDAnnotation`. This is a **required
  prerequisite change in service-catalog**, not optional — the feature
  cannot work without it, since `ServiceEntitlement` today has zero
  requester identity on the object.
- `ContactGroup` creation stays manual/operator-driven; not auto-created per
  entitlement.
- Entitlement revocation does not auto-remove contact group membership in
  this iteration — explicit, flagged scope cut.
- Idempotency-tracking annotation for this trigger lives on the
  `ServiceEntitlement` object (mirroring where `ContactCreated`'s tracking
  annotation lives on `Contact` — i.e., on the triggering object), not on
  `Contact`, since a `Contact` may not exist yet when the trigger fires.

### Open Questions

- **(Blocking implementation of the cross-cluster read)** Should Milo import
  service-catalog's generated API types directly instead of using
  unstructured/GVK-shim reads? Affects controller code complexity and repo
  coupling. Recommend unstructured per NFR1, but this should be confirmed
  with whoever owns cross-repo dependency conventions before writing the
  controller.
- **(Blocking for a clean "give up" semantic)** What is the maximum backoff
  window before the controller stops retrying a `ServiceEntitlement` whose
  `Contact` never appears, and how should that be surfaced (Kubernetes Event?
  a status condition on the entitlement, which would require yet another
  service-catalog status field this repo doesn't currently own)? Proposed
  default: unbounded requeue at a capped interval (e.g. 10 minutes), since a
  Contact showing up days later should still complete enrollment, but this
  needs explicit sign-off — unbounded retry queues can mask real bugs if a
  Contact will truly never appear for a class of users (e.g. machine
  accounts / non-User subjects, which are already excluded by the
  `SubjectKind: User`-only convention).
- **(Non-blocking, product decision)** Should dependency-origin entitlements
  (`status.origin == Dependency`) count toward `EntitlementRegistered`
  enrollment, or only direct (`status.origin == Direct`) registrations? This
  design enrolls on any Active entitlement regardless of origin; flag for
  product/CRM sign-off since it changes who ends up in, e.g.,
  `compute-testers`.
- **(Non-blocking, scope confirmation)** Confirm with stakeholders that
  "no automatic membership removal on revocation" is acceptable for v1 — the
  issue thread didn't explicitly rule on this.
- **(Non-blocking)** Should `EnrollmentEntitlementSelector.serviceName` be
  validated against a live/cached list of known canonical service names at
  policy-admission time, or accepted as free text? Recommend free text for
  v1 (a mismatched name silently never matches, which is a lesser failure
  mode than blocking valid policy creation on a fragile cross-cluster
  admission-time lookup).

### Implementation Notes

- api-dev: Start with the service-catalog webhook change — it's the
  prerequisite everything else depends on, and it's small, isolated, and
  directly modeled on `organization_webhook.go` (read that file plus
  `organization_webhook_test.go` first).
- api-dev: When building the unstructured GVK shim in Milo, keep it to the
  minimum fields actually read (`status.phase`, `status.serviceName`, the
  creator annotation, `metadata.annotations` for the idempotency marker) —
  do not attempt to mirror the full `ServiceEntitlementStatus` shape.
- api-dev: Reuse `ContactGroupEnrollmentController`'s helper functions
  (`hasOptOut`, `ensureMembership`, `webhookRejectionReason`,
  `membershipName`) rather than duplicating them — consider extracting them
  into trigger-agnostic helpers shared between the two controllers (or two
  reconcilers in the same package) since the enrollment-target logic
  (create-membership-if-absent, respect opt-out) is identical across both
  trigger types; only the "how do we find/wait-for a Contact" step differs.
- test-engineer: The most valuable test is the "Contact created after
  entitlement is already Active" ordering case (FR3) — this is the one path
  with no equivalent in the existing `ContactCreated`-only controller and is
  the most likely place for a bug (e.g. forgetting to also watch `Contact`
  creation and re-scan pending entitlement triggers).
- test-engineer: Also test the cross-project double-registration collapse
  case explicitly (same user, same service, two projects) since the
  deterministic membership name is the only thing preventing a duplicate
  membership object there.
