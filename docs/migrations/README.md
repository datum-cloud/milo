# Migrations

This section documents **manual migrations** that we run directly against a Milo cluster (for example, bulk data imports or one‑off resource changes).

Each migration is written as a small, repeatable runbook:

- **What** the migration does.
- **Why** it exists / background.
- **How** to run it (exact commands).
- **What** gets created/changed in the cluster.

Individual migrations live in numbered files so we can track the order they
were introduced:

- `001-contacts-from-csv` – import marketing contacts and newsletter memberships from a CSV file.
