#!/usr/bin/env python3
import argparse
import csv
import hashlib
import re
import sys
from typing import Any, Dict, List, Optional

try:
    import yaml  # PyYAML
except ImportError:
    print("Missing dependency: PyYAML. Install with: pip install pyyaml", file=sys.stderr)
    sys.exit(1)


DEFAULT_NAMESPACE = "milo-system"
NEWSLETTER_GROUP_NAME = "emailnewsletter-contact-group-3a3ar9"
CONTACT_GROUP_NAMESPACE = "default"


def slugify(value: str) -> str:
    """
    Lowercase, trim, replace non-alphanumeric with '-', collapse repeats, strip '-'.
    """
    value = (value or "").strip().lower()
    value = re.sub(r"[^a-z0-9]+", "-", value)
    value = re.sub(r"-{2,}", "-", value)
    return value.strip("-")


def short_id_from_email(email: str, length: int = 5) -> str:
    """
    Deterministic short suffix from email for stable names.
    """
    digest = hashlib.sha1((email or "").strip().lower().encode("utf-8")).hexdigest()
    return digest[:length]


def build_contact_name(given_name: str, family_name: str, email: str) -> str:
    """
    Produce a DNS-safe name for the Contact resource.
    Pattern: 'contact-<id>' where <id> is a short deterministic suffix.
    """
    suffix = short_id_from_email(email)
    return f"contact-{suffix}"


def make_contact_doc(fname: str, lname: str, email: str) -> Dict[str, Any]:
    """
    Build a Contact CRD manifest from basic person fields.
    Returns a dict ready to be dumped as YAML.
    """
    name = build_contact_name(fname, lname, email)
    doc: Dict[str, Any] = {
        "apiVersion": "notification.miloapis.com/v1alpha1",
        "kind": "Contact",
        "metadata": {
            "name": name,
            "namespace": DEFAULT_NAMESPACE,
        },
        "spec": {
            "email": email,
        },
    }
    if fname:
        doc["spec"]["givenName"] = fname
    if lname:
        doc["spec"]["familyName"] = lname
    return doc


def make_membership_doc(contact_name: str) -> Dict[str, Any]:
    """
    Build a ContactGroupMembership CRD manifest that links the given contact
    to the default Email Newsletter contact group.
    """
    doc: Dict[str, Any] = {
        "apiVersion": "notification.miloapis.com/v1alpha1",
        "kind": "ContactGroupMembership",
        "metadata": {
            # We could also use generateName: 'contact-group-membership-' and omit name.
            "name": f"contact-group-membership-{hashlib.sha1(contact_name.encode('utf-8')).hexdigest()[:5]}",
            "namespace": DEFAULT_NAMESPACE,
        },
        "spec": {
            "contactGroupRef": {
                "name": NEWSLETTER_GROUP_NAME,
                "namespace": CONTACT_GROUP_NAMESPACE,
            },
            "contactRef": {
                "name": contact_name,
                "namespace": DEFAULT_NAMESPACE,
            },
        },
    }
    return doc


def parse_bool_newsletter(value: str) -> bool:
    """
    Returns True if the CSV 'List' column indicates Newsletter.
    Accepts case-insensitive 'newsletter'.
    """
    if value is None:
        return False
    return value.strip().lower() == "newsletter"


def read_csv(path: str) -> List[Dict[str, str]]:
    """
    Read the input CSV and normalize the columns we care about.
    Returns a list of dict rows with keys: fname, lname, email, list, company, notes.
    """
    with open(path, "r", encoding="utf-8") as f:
        reader = csv.DictReader(f)
        rows: List[Dict[str, str]] = []
        for row in reader:
            # Normalize keys we care about and trim whitespace
            normalized = {
                "fname": (row.get("fname") or "").strip(),
                "lname": (row.get("lname") or "").strip(),
                "email": (row.get("Email Address") or "").strip(),
                "list": (row.get("List") or "").strip(),
                "company": (row.get("Company") or "").strip(),
                "notes": (row.get("Notes") or "").strip(),
            }
            rows.append(normalized)
        return rows


def dump_yaml_docs(docs: List[Dict[str, Any]]) -> None:
    """
    Print a sequence of YAML documents to stdout, separated by '---'.
    """
    for idx, doc in enumerate(docs):
        print(yaml.safe_dump(doc, sort_keys=False).rstrip())
        if idx != len(docs) - 1:
            print("---")


def write_yaml_file(docs: List[Dict[str, Any]], out_path: str) -> None:
    """
    Write the YAML documents to a file as a multi-document stream.
    """
    with open(out_path, "w", encoding="utf-8") as f:
        for idx, doc in enumerate(docs):
            f.write(yaml.safe_dump(doc, sort_keys=False))
            if idx != len(docs) - 1:
                f.write("---\n")


def process_csv(path: str, contacts_out_path: Optional[str], memberships_out_path: Optional[str]) -> None:
    """
    End-to-end CSV processing:
    - Reads rows
    - Builds Contact documents
    - Adds ContactGroupMembership documents for Newsletter rows
    - Prints all documents as YAML
    """
    rows = read_csv(path)
    contact_docs: List[Dict[str, Any]] = []
    membership_docs: List[Dict[str, Any]] = []

    for row in rows:
        email = row["email"]
        if not email:
            # Skip rows without email
            continue
        # Clean stray commas/spaces that sometimes appear in CSV cells
        email = email.strip().rstrip(",")

        contact_doc = make_contact_doc(row["fname"], row["lname"], email)
        contact_docs.append(contact_doc)

        if parse_bool_newsletter(row["list"]):
            membership_doc = make_membership_doc(contact_doc["metadata"]["name"])
            membership_docs.append(membership_doc)

    # Print to console: contacts first, then memberships
    dump_yaml_docs(contact_docs)
    if membership_docs:
        print("---")
        dump_yaml_docs(membership_docs)

    # Also write to files if requested
    if contacts_out_path:
        write_yaml_file(contact_docs, contacts_out_path)
    if memberships_out_path:
        write_yaml_file(membership_docs, memberships_out_path)


def parse_args(argv: Optional[List[str]] = None) -> argparse.Namespace:
    """
    Parse CLI arguments.
    """
    parser = argparse.ArgumentParser(
        description="Read CSV, build Contact and ContactGroupMembership YAML. Prints to stdout and writes files."
    )
    parser.add_argument(
        "-i",
        "--input",
        default="./contacts.csv",
        help="Path to input CSV. Default: 'contacts.csv'",
    )
    parser.add_argument(
        "--contacts-output",
        "-oc",
        default="contacts_from_csv.yaml",
        help="Path to write Contacts YAML. Default: contacts_from_csv.yaml",
    )
    parser.add_argument(
        "--memberships-output",
        "-om",
        default="contact_group_memberships_from_csv.yaml",
        help="Path to write ContactGroupMemberships YAML. Default: contact_group_memberships_from_csv.yaml",
    )
    return parser.parse_args(argv)


def main() -> None:
    """
    Program entry point: parse args and process the CSV file.
    """
    args = parse_args()
    process_csv(args.input, args.contacts_output, args.memberships_output)


if __name__ == "__main__":
    main()


