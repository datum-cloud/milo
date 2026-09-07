# Entitlement Registration Contact Group Enrollment

> Status: Draft, seeking review. Source issue: datum-cloud/cloud-portal#1497.

## Revision History

**v2 (this revision)**: Supersedes v1. In review, Scot (who owns service
catalog) redirected the design:

> "I'd drive it from the service catalog side. This way Milo's core system
> doesn't understand what services are and service centric logic stays
> contained within the service catalog. Kinda like why billing doesn't know
> what a service is."

Service catalog now owns this feature end to end. Milo's contact system
stays generic: it knows about contacts and groups, not about services or
entitlements.

## Overview

When a customer registers for a service (for example, Compute), internal
staff want that person's CRM contact automatically added to the matching
Contact Group (for example, `compute-testers`), so they don't have to
maintain that list by hand.

Service catalog will own this: when a customer's registration for a service
goes active, and that service has been set up with a Contact Group, service
catalog adds the customer's contact to that group.

## How It Works

An operator sets up a service to opt into this behavior by linking it to a
Contact Group. Services that don't set this up stay unaffected; nothing
changes for them.

When a customer registers for that service and the registration goes active,
service catalog adds their CRM contact to the linked group.

- If the customer doesn't have a CRM contact yet, the enrollment isn't lost.
  It completes as soon as their contact record shows up.
- If someone has already opted out of a group, that opt-out is respected;
  they won't be re-added.
- If a customer registers for the same service more than once (for example,
  across two projects), they only end up in the group once.
- If an operator links a Contact Group to a service after customers have
  already registered, those existing registrations are picked up and
  enrolled too, not just new ones going forward.
- If a service is linked to a group that doesn't exist yet, service catalog
  creates it automatically rather than requiring an operator to create it
  first (see Group Creation below).

## Architecture

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
- **Group Creation**: service catalog creates the linked Contact Group
  automatically the first time it's needed, using sensible defaults. An
  operator can adjust the group's settings (for example, visibility, or
  which external systems it syncs to) afterward in staff-portal, the same
  way they manage any other Contact Group today.

## What This Does Not Do

- **No automatic removal.** If a customer's registration for a service is
  later revoked or rejected, they are not automatically removed from the
  Contact Group. Removing someone from a group stays a manual action, same
  as it is today. This is a deliberate scope cut for this iteration, not an
  oversight, and should be confirmed with stakeholders (support/success/
  sales) before shipping.

## Open Questions

- Should someone who got a service automatically (as a side effect of
  registering for something else it depends on) be enrolled the same as
  someone who registered for it directly? This design currently treats them
  the same. Flag if CRM only wants to count direct signups.
- Confirm with stakeholders that no automatic removal on revocation is
  acceptable for this first iteration.
- Is there a point at which we should stop waiting for a customer's CRM
  contact to show up and flag it instead of waiting indefinitely?
- What defaults should an auto-created Contact Group start with (visibility,
  external sync destinations), given no operator has made that call yet?

## Out of Scope

- Removing group membership automatically on revocation — deferred, needs a
  product decision on whether that should even happen automatically.
- Letting users set contact info per project/org to drive group membership
  (raised in the issue thread) is a different feature entirely and is not
  part of this design.
