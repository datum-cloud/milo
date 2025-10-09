# Tax ID Management for Vendors

This guide explains how to securely manage tax identification numbers for vendors in Milo, including why we use secure storage and how to set it up.

## Why Secure Tax ID Storage?

Tax identification numbers (EIN, SSN, VAT, etc.) are highly sensitive information that require special handling:

- **Legal Compliance**: Tax IDs are subject to strict data protection regulations
- **Security**: These numbers can be used for identity theft and fraud
- **Audit Requirements**: Access to tax IDs must be logged and controlled
- **Separation of Concerns**: Business logic should be separate from sensitive data

Milo uses secure storage to protect this information while keeping vendor management simple and efficient.

## How It Works

Instead of storing tax IDs directly in vendor records, Milo uses a reference system:

1. **Tax IDs are stored securely** in dedicated secure storage
2. **Vendor records contain only references** to the secure storage
3. **Access is controlled** through role-based permissions
4. **All access is logged** for audit purposes

## Setting Up Tax ID Storage

### Step 1: Create a Secure Tax ID Record

Create a secure record for each vendor's tax ID:

```bash
# For a US business with EIN
kubectl create tax-id acme-corp-tax-id \
  --vendor=acme-corp \
  --type=ein \
  --value="12-3456789" \
  --country=us

# For an individual with SSN
kubectl create tax-id john-doe-ssn \
  --vendor=john-doe-consulting \
  --type=ssn \
  --value="123-45-6789" \
  --country=us
```

### Step 2: Reference in Vendor Record

Update your vendor record to reference the secure tax ID:

```yaml
apiVersion: vendors.miloapis.com/v1alpha1
kind: Vendor
metadata:
  name: acme-corp
spec:
  profileType: business
  legalName: "ACME Corporation LLC"
  taxInfo:
    taxIdType: EIN
    taxIdRef:
      secretName: "acme-corp-tax-id"
      secretKey: "tax-id"
    country: "United States"
    taxDocument: "W-9"
```

## Managing Multiple Tax IDs

Some vendors may have multiple tax IDs (e.g., international operations):

### Create a Multi-ID Record

```bash
# Create a record with multiple tax IDs
kubectl create tax-id complex-vendor-tax-ids \
  --vendor=international-corp \
  --type=multiple \
  --values="ein:12-3456789,vat:GB123456789,business-number:CA123456789"
```

### Reference Specific Tax ID

```yaml
# Reference the US EIN
taxIdRef:
  secretName: "complex-vendor-tax-ids"
  secretKey: "ein"

# Or reference the EU VAT
taxIdRef:
  secretName: "complex-vendor-tax-ids"
  secretKey: "vat"
```

## Best Practices

### Naming Conventions

Use descriptive names that include the vendor name:

```bash
# Good examples
acme-corp-tax-id
john-doe-ssn
international-corp-vat

# Avoid generic names
tax-secret-1
secret-abc
```

### Key Naming

Use consistent key names within records:

```yaml
# Standard keys
tax-id      # For EIN
ssn         # For Social Security Number
vat-number  # For VAT numbers
business-number  # For business registration numbers
```

## Common Operations

### Viewing Tax ID Information

```bash
# List all tax ID records for a vendor
kubctl get tax-ids --vendor=acme-corp

# View specific tax ID (requires appropriate permissions)
kubctl describe tax-id acme-corp-tax-id
```

### Updating Tax IDs

```bash
# Update an existing tax ID
kubctl update tax-id acme-corp-tax-id \
  --value="98-7654321"

# Add additional tax ID to existing record
kubctl update tax-id complex-vendor-tax-ids \
  --add-key="gst:IN123456789"
```

### Removing Tax IDs

```bash
# Remove a tax ID record
kubctl delete tax-id acme-corp-tax-id

# Remove specific key from multi-ID record
kubctl update tax-id complex-vendor-tax-ids \
  --remove-key="vat"
```

## Troubleshooting

### Common Issues

**Tax ID Not Found**
```
Error: Tax ID record 'acme-corp-tax-id' not found
```
- Check the record name in your vendor configuration
- Verify the record exists: `kubctl get tax-ids --vendor=acme-corp`

**Access Denied**
```
Error: Access denied to tax ID record
```
- Check your permissions for tax ID access
- Contact your administrator for access

**Invalid Reference**
```
Error: Invalid tax ID reference in vendor record
```
- Verify the `secretName` and `secretKey` in your vendor configuration
- Check that the referenced record exists

### Getting Help

```bash
# Check vendor tax ID configuration
kubctl describe vendor acme-corp --show-tax-info

# Verify tax ID record exists
kubctl get tax-ids --vendor=acme-corp

# Check your permissions
kubctl auth can-i get tax-ids
```
