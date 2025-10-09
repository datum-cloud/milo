# VendorVerification Architecture

This document explains the `VendorVerification` resource that manages verification processes for vendors, providing better separation of concerns and simpler RBAC.

## Overview

The `VendorVerification` resource separates verification concerns from vendor information, allowing for:
- Independent verification lifecycle management
- Granular RBAC permissions
- Rich verification metadata and tracking
- Multiple verification types per vendor
- Audit trails and compliance tracking

## Architecture Benefits

### 1. **Separation of Concerns**
- Vendor resource contains only vendor information
- Verification resource handles all verification processes
- Clear boundaries between data and processes

### 2. **Simplified RBAC**
- Different permissions for vendor data vs verification
- Verification teams can manage verifications without vendor access
- Audit teams can access verification data independently

### 3. **Rich Verification Metadata**
- Detailed tracking of verification processes
- Document management and validation
- Verifier information and timestamps
- Priority and requirement settings

## Resource Structure

### VendorVerification

```yaml
apiVersion: vendors.miloapis.com/v1alpha1
kind: VendorVerification
metadata:
  name: acme-corp-tax-verification
spec:
  vendorRef:
    name: acme-corp
    namespace: default
  verificationType: tax
  status: approved
  description: "Tax ID verification for EIN 12-3456789"
  documents:
    - type: "W-9"
      reference: "acme-corp-w9-document"
      version: "2024-01-15"
      valid: true
  verifierRef:
    type: admin
    name: "admin@company.com"
    metadata:
      department: "Finance"
      role: "Tax Specialist"
  notes: "Tax ID verified through IRS database"
  priority: 8
  required: true
  expirationDate: "2025-01-15T00:00:00Z"
status:
  completedAt: "2024-01-15T14:30:00Z"
  lastUpdatedAt: "2024-01-15T14:30:00Z"
```

## Verification Types

### Tax Verification
- **Purpose**: Verify tax identification and compliance
- **Documents**: W-9, W-8BEN, tax certificates
- **Verifiers**: Tax specialists, finance team
- **Priority**: High (required for payment processing)

### Business Verification
- **Purpose**: Verify business registration and legitimacy
- **Documents**: Articles of incorporation, business licenses
- **Verifiers**: Business verification services, legal team
- **Priority**: High (required for business relationships)

### Identity Verification
- **Purpose**: Verify individual identity and credentials
- **Documents**: Government ID, background checks
- **Verifiers**: HR team, background check services
- **Priority**: Medium (required for individual contractors)

### Compliance Verification
- **Purpose**: Verify regulatory compliance
- **Documents**: Compliance certificates, regulatory approvals
- **Verifiers**: Compliance team, legal team
- **Priority**: High (required for regulated industries)

## Verification Status Lifecycle

```
Pending → In-Progress → Approved
   ↓           ↓           ↓
Rejected   Expired    Expired
```

### Status Descriptions

- **Pending**: Verification created but not yet started
- **In-Progress**: Verification is actively being processed
- **Approved**: Verification completed successfully
- **Rejected**: Verification failed or was denied
- **Expired**: Verification has expired and needs renewal

## RBAC Examples

### Verification Team Role
```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: vendor-verification-manager
rules:
- apiGroups: ["vendors.miloapis.com"]
  resources: ["vendorverifications"]
  verbs: ["get", "list", "create", "update", "patch"]
- apiGroups: ["vendors.miloapis.com"]
  resources: ["vendors"]
  verbs: ["get", "list"]  # Read-only access to vendor data
```

### Vendor Management Role
```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: vendor-manager
rules:
- apiGroups: ["vendors.miloapis.com"]
  resources: ["vendors"]
  verbs: ["get", "list", "create", "update", "patch", "delete"]
- apiGroups: ["vendors.miloapis.com"]
  resources: ["vendorverifications"]
  verbs: ["get", "list"]  # Read-only access to verification status
```

### Audit Team Role
```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: vendor-auditor
rules:
- apiGroups: ["vendors.miloapis.com"]
  resources: ["vendorverifications"]
  verbs: ["get", "list"]  # Read-only access for auditing
- apiGroups: ["vendors.miloapis.com"]
  resources: ["vendors"]
  verbs: ["get", "list"]  # Read-only access to vendor data
```

## API Functions

### Basic Operations

```go
// Get all verifications for a vendor
verifications, err := GetVerificationsForVendor(ctx, client, vendor)

// Get specific verification type
verification, err := GetVerificationByType(ctx, client, vendor, VerificationTypeTax)

// Check if vendor is fully verified
isVerified, missing, err := IsVendorVerified(ctx, client, vendor)

// Get overall verification status
status, err := GetVerificationStatus(ctx, client, vendor)
```

### Verification Management

```go
// Create new verification
err := CreateVerification(ctx, client, vendor, verification)

// Update verification status
err := UpdateVerificationStatus(ctx, client, verification, VerificationStatusApproved, "Approved by tax specialist")

// Check for expired verifications
expired, err := GetExpiredVerifications(ctx, client, vendor)

// Get verification summary
summary, err := GetVerificationSummary(ctx, client, vendor)
```

## Usage Examples

### Creating a Tax Verification

```yaml
apiVersion: vendors.miloapis.com/v1alpha1
kind: VendorVerification
metadata:
  name: acme-corp-tax-verification
spec:
  vendorRef:
    name: acme-corp
  verificationType: tax
  status: pending
  description: "Tax ID verification for EIN 12-3456789"
  documents:
    - type: "W-9"
      reference: "acme-corp-w9-document"
      valid: true
  verifierRef:
    type: admin
    name: "admin@company.com"
  priority: 8
  required: true
```

### Updating Verification Status

```bash
# Approve verification
kubectl patch vendorverification acme-corp-tax-verification \
  --type='merge' \
  -p='{"spec":{"status":"approved"},"status":{"completedAt":"2024-01-15T14:30:00Z"}}'

# Reject verification
kubectl patch vendorverification acme-corp-tax-verification \
  --type='merge' \
  -p='{"spec":{"status":"rejected","notes":"Invalid tax ID format"}}'
```

### Querying Verification Status

```bash
# Get all verifications for a vendor
kubectl get vendorverifications -l vendor.miloapis.com/vendor=acme-corp

# Get pending verifications
kubectl get vendorverifications --field-selector spec.status=pending

# Get verifications by type
kubectl get vendorverifications --field-selector spec.verificationType=tax
```

## Integration with Vendor Status

The vendor status can be derived from verification status:

```go
// Check if vendor can be activated
isVerified, missingVerifications, err := IsVendorVerified(ctx, client, vendor)
if err != nil {
    return err
}

if !isVerified {
    return fmt.Errorf("vendor cannot be activated, missing verifications: %v", missingVerifications)
}

// Update vendor status based on verification
if isVerified {
    vendor.Spec.Status = VendorStatusActive
} else {
    vendor.Spec.Status = VendorStatusPending
}
```

## Best Practices

### 1. **Verification Naming**
Use descriptive names that include vendor and type:
```yaml
metadata:
  name: acme-corp-tax-verification
  name: john-doe-identity-verification
  name: international-corp-compliance-verification
```

### 2. **Document References**
Use consistent document reference formats:
```yaml
documents:
  - type: "W-9"
    reference: "acme-corp-w9-document"
    version: "2024-01-15"
  - type: "Business-License"
    reference: "acme-corp-license-2024"
    version: "2024-01-01"
```

### 3. **Verifier Information**
Include detailed verifier metadata:
```yaml
verifierRef:
  type: admin
  name: "admin@company.com"
  metadata:
    department: "Finance"
    role: "Tax Specialist"
    employee-id: "EMP-12345"
```

### 4. **Priority and Requirements**
Set appropriate priorities and requirements:
```yaml
priority: 8        # High priority (1-10 scale)
required: true     # Required for vendor activation
```

### 5. **Expiration Management**
Set appropriate expiration dates:
```yaml
expirationDate: "2025-01-15T00:00:00Z"  # 1 year from verification
```

## Monitoring and Alerting

### Verification Metrics
- Total verifications by status
- Average verification time
- Expired verifications count
- Verification success rate

### Alerts
- Verification approaching expiration
- Verification rejected
- Verification stuck in pending status
- Missing required verifications

## Migration from Embedded Verification

If you have existing vendors with embedded verification fields:

1. **Extract verification data** from vendor resources
2. **Create VendorVerification resources** for each verification
3. **Remove verification fields** from vendor resources
4. **Update applications** to use verification resources
5. **Test verification workflows** end-to-end

This architecture provides a much cleaner separation of concerns and enables more sophisticated verification workflows while maintaining simple RBAC and better audit capabilities.
