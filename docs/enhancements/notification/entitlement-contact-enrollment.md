---
status: provisional
stage: alpha
latest-milestone: "v0.1"
---

# Entitlement Registration Contact Group Enrollment

Source issue: datum-cloud/cloud-portal#1497.

- [Summary](#summary)
- [Motivation](#motivation)
  - [Goals](#goals)
  - [Non-Goals](#non-goals)
- [Proposal](#proposal)
  - [User Stories](#user-stories)
  - [Notes/Constraints/Caveats](#notesconstraintscaveats)
  - [Risks and Mitigations](#risks-and-mitigations)
- [Design Details](#design-details)
- [Production Readiness Review Questionnaire](#production-readiness-review-questionnaire)
- [Implementation History](#implementation-history)
- [Drawbacks](#drawbacks)
- [Alternatives](#alternatives)

## Summary

When a customer registers for a service (for example, Compute), internal
staff want that person's CRM contact automatically added to the matching
Contact Group (for example, `compute-testers`), so they don't have to
maintain that list by hand. This keeps internal contact lists in sync with
real product usage instead of relying on someone remembering to update them.

Service catalog will own this end to end: when a customer's registration for
a service goes active, and that service has been set up with a Contact
Group, service catalog adds the customer's contact to that group,
auto-creating the group the first time it's needed.

## Motivation

Today, keeping CRM contact lists (like `compute-testers`) aligned with who
has actually signed up for a service is a manual process. Nobody is notified
when a customer registers, so lists drift out of date, and support/success/
sales lose a reliable signal of real product usage.

### Goals

- A customer registering for a service they've opted into CRM tracking for
  gets added to the right Contact Group automatically, without a human
  remembering to do it.
- The mapping from a service to its Contact Group is something an operator
  sets up once per service.
- This work is contained to service catalog. Milo's contact system does not
  need to understand what a service or an entitlement is — the same
  separation billing already has.

### Non-Goals

- Automatically removing someone from a Contact Group when their
  registration is revoked or rejected — that stays a manual/opt-out action
  in this iteration.
- Letting customers set contact info per project/org to drive group
  membership (raised in the issue thread) — a materially different feature.
- Any change to how Contact Groups are used or configured outside of this
  enrollment flow.

## Proposal

An operator sets up a service to opt into this behavior by linking it to a
Contact Group. Services that don't set this up stay unaffected.

When a customer registers for that service and the registration goes
active, service catalog adds their CRM contact to the linked group. If the
group doesn't exist yet, service catalog creates it automatically rather
than requiring an operator to create it first.

### User Stories

#### Story 1: A customer signs up for a tracked service

As an internal user (support/success/sales), when a customer registers for
Compute, I want their contact automatically added to `compute-testers` so
our CRM list reflects real usage without me having to update it by hand.

#### Story 2: An operator opts a new service into tracking

As an operator, I want to link a service to a Contact Group once, and have
every customer who registers for that service — past and future — get
enrolled, without having to backfill the list myself.

### Notes/Constraints/Caveats

- If the customer doesn't have a CRM contact yet, the enrollment isn't
  lost — it completes as soon as their contact record shows up.
- If someone has already opted out of a group, that opt-out is respected;
  they won't be re-added.
- If a customer registers for the same service more than once (for example,
  across two projects), they only end up in the group once.
- If an operator links a Contact Group to a service after customers have
  already registered, those existing registrations are picked up and
  enrolled too, not just new ones going forward.
- Whether someone who received a service automatically (as a side effect of
  registering for something else it depends on) should be enrolled the same
  as someone who registered directly is an open question — this proposal
  currently treats them the same.

### Risks and Mitigations

| Risk | Impact | Mitigation |
| --- | --- | --- |
| No automatic removal on revocation | A revoked customer stays listed as a tester until someone manually removes them | Explicit, called-out scope cut; confirm acceptable with support/success/sales before shipping |
| Auto-created Contact Group has no sensible default for visibility/sync settings | A newly created group may not sync anywhere until an operator notices and configures it | Operator reviews and adjusts auto-created groups' settings after the fact; revisit if this proves to be a recurring gap |
| Dependency-origin registrations counted the same as direct ones | CRM lists may include people who didn't directly opt in | Flagged as an open product question, not silently decided |

## Design Details

This stays entirely within service catalog; Milo's contact system is a
passive API it writes to, not a participant in the decision-making.

- Service catalog already tracks when a customer's registration for a
  service goes active. A new piece of logic in service catalog watches for
  that, and for each newly-active registration, checks whether the service
  it's for has a Contact Group linked.
- If so, it resolves the customer's CRM contact and adds them to that group
  by writing directly to Milo's existing contact API — the same API a human
  operator or any other client would use. No new integration point is added
  on Milo's side, and Milo's own enrollment mechanism (used for other,
  non-service-specific cases) is untouched.
- Service registrations today don't record who requested them. Closing that
  gap is a prerequisite: we need to capture the requesting customer's
  identity at registration time so service catalog can look up their
  contact later. This is a small, self-contained addition to how
  registrations are created.
- Service catalog creates the linked Contact Group automatically the first
  time it's needed, using sensible defaults. An operator can adjust the
  group's settings (for example, visibility, or which external systems it
  syncs to) afterward in staff-portal, the same way they manage any other
  Contact Group today.

## Production Readiness Review Questionnaire

TBD — this is a low-risk, internal-tooling feature (no customer-facing
behavior changes, opt-in per service). A full PRR pass will be done before
this graduates past an initial rollout; noting the key points now:

- **Enablement**: opt-in per service (an operator links a Contact Group).
  No behavior change for any service until an operator does this.
- **Rollback**: disabling is as simple as unlinking the Contact Group from
  a service; no data is deleted, enrollment just stops happening for new
  registrations.
- **Failure mode**: if a customer's CRM contact doesn't exist yet, or the
  contact API is briefly unavailable, enrollment is retried rather than
  dropped.

## Implementation History

- 2026-09-07: Initial design proposed (Milo-side trigger approach).
- 2026-09-07: Redirected to a service-catalog-owned approach per review
  feedback (Milo's core should stay agnostic to what a service is, the same
  way billing doesn't need to know what a service is).

## Drawbacks

- Adds a new cross-repo dependency: service-catalog becomes a direct writer
  to Milo's contact API, not just a reader of it.
- Auto-creating Contact Groups means a group can exist with no sync
  destinations configured until an operator notices, rather than always
  being deliberately set up first.

## Alternatives

- **Drive this from Milo instead of service-catalog** (the original
  direction): rejected because it would require Milo's core contact system
  to understand what a service and an entitlement are, breaking the
  separation Milo currently has (and that billing already follows) between
  generic platform concepts and service-specific ones.
- **Require an operator to pre-create the Contact Group** rather than
  auto-creating it: considered and initially recommended, but revised after
  feedback in favor of removing that manual step; an operator can still
  adjust the auto-created group's settings afterward.
