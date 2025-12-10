# 001 – Import contacts and newsletter memberships from CSV

## Overview

This migration imports a set of **marketing contacts** into Milo and, for a subset of them, creates **newsletter list memberships**. It is intended for one‑off/batch imports where we are working from an existing CSV export.

The migration lives in the directory `docs/migrations/001-contacts-from-csv` and is implemented by the script `create_contacts_from_csv.py` in that directory. A minimal example CSV with the expected headers (`contacts.csv`) is also provided there; it is for structure only, the real data remains private.

At a high level:

1. We prepare a CSV file with basic contact details.
2. We convert the CSV into Milo CRDs.
3. The script generates two YAML files:
   - one containing a series of `Contact` resources.
   - one containing a series of `ContactGroupMembership` resources.
4. We apply those YAML files directly to the target cluster.

---

### CSV shape

The CSV itself is private, but the **shape** (headers and expected values) is:

- **fname** – contact’s given/first name (optional).
- **lname** – contact’s family/last name (optional).
- **Email Address** – contact’s email address (required; rows without this are skipped).
- **List** – text label describing the mailing list for the contact.
  - Rows with the case‑insensitive value `newsletter` will be added to the
    default email newsletter contact group.
- **Company** – company/organization name (optional; currently not written to the CRDs).
- **Notes** – free‑form notes (optional; currently not written to the CRDs).

Example (sanitized) header and row:

```text
fname,lname,Email Address,List,Company,Notes
Jane,Doe,jane@example.com,Newsletter,Example Corp,Met at conference
```

The script normalizes whitespace and ignores rows that do not include an email.

---

### Running the migration

#### 1. Prerequisites

- Python 3 available on your workstation.
- The `PyYAML` package installed:

  ```bash
  pip3 install pyyaml
  ```

- Your CSV file present on disk (for example: `contacts.csv`).
- `kubectl` configured to point at the target Milo cluster (`KUBECONFIG`
  or current context).

#### 2. Generate YAML from the CSV

From the migration directory:

```bash
cd docs/migrations/001-contacts-from-csv
python3 create_contacts_from_csv.py \
  --input ./contacts.csv \
  --contacts-output ./contacts_from_csv.yaml \
  --memberships-output ./contact_group_memberships_from_csv.yaml
```

If you omit the flags, the script defaults to:

- **Input CSV**: `contacts.csv` (expected to live in this directory)
- **Contacts output**: `contacts_from_csv.yaml`
- **Memberships output**: `contact_group_memberships_from_csv.yaml`

The script:

- Reads the CSV and, for each row with an email, creates a `Contact` resource.
- For rows where the `List` column is `Newsletter` (case‑insensitive), also
  creates a `ContactGroupMembership` resource.
- Prints all generated manifests to `stdout` (for inspection) and writes them
  to the two output YAML files.

#### 3. What the generated files contain

- **`contacts_from_csv.yaml`**
  - A multi‑document YAML file.
  - Each document is a `Contact` (`notification.miloapis.com/v1alpha1`).
  - Each contact:
    - Lives in the `milo-system` namespace.
    - Has a deterministic name derived from the email
      (pattern: `contact-<short-id>`).
    - Includes the email, and where present, `givenName` and `familyName`.

- **`contact_group_memberships_from_csv.yaml`**
  - A multi‑document YAML file.
  - Each document is a `ContactGroupMembership`
    (`notification.miloapis.com/v1alpha1`).
  - Each membership:
    - Lives in the `milo-system` namespace.
    - References the default email newsletter contact group:
      - `spec.contactGroupRef.name`: `emailnewsletter-contact-group-3a3ar9`
      - `spec.contactGroupRef.namespace`: `default`
    - References one of the `Contact` resources generated in
      `contacts_from_csv.yaml`.

Together, **each file represents a series of CRDs**:

- `contacts_from_csv.yaml` – a series of `Contact` resources.
- `contact_group_memberships_from_csv.yaml` – a series of `ContactGroupMembership` resources.

---

### 4. Applying the migration to the cluster

Ensure your `kubectl` context is pointing at the target Milo cluster (for
example, by exporting `KUBECONFIG`):

```bash
export KUBECONFIG=.milo/kubeconfig   # or another kubeconfig pointing to the target cluster
```

Then apply the generated YAML files (from the same migration directory):
