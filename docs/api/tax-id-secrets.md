# Tax ID Secret Management

This document explains how tax ID numbers are securely stored and managed using Kubernetes Secrets in the Milo vendor system.

## Overview

Tax ID numbers are sensitive information that should never be stored in plain text in CRDs. Instead, they are stored in Kubernetes Secrets and referenced by the Vendor resource through a `TaxIdReference`.

## Architecture

### TaxIdReference

The `TaxIdReference` type points to a Kubernetes Secret containing the tax ID:

```yaml
taxIdRef:
  secretName: "acme-corp-tax-id"    # Name of the Secret
  secretKey: "tax-id"               # Key within the Secret
  namespace: "default"              # Optional: Secret namespace
```

### Secret Structure

Tax IDs are stored in Kubernetes Secrets with the following structure:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: acme-corp-tax-id
  namespace: default
  labels:
    vendor.miloapis.com/vendor: acme-corp
    vendor.miloapis.com/type: tax-id
type: Opaque
data:
  tax-id: MTItMzQ1Njc4OQ==  # Base64 encoded tax ID
```

## Security Benefits

### 1. **Encryption at Rest**
- Secrets are encrypted at rest by Kubernetes
- No plain text storage in etcd
- Automatic encryption key rotation

### 2. **Access Control**
- RBAC controls who can access secrets
- Fine-grained permissions per secret
- Audit logging for secret access

### 3. **Separation of Concerns**
- Sensitive data separated from business logic
- CRDs contain only references, not actual data
- Easier to manage and audit

### 4. **Compliance**
- Meets data protection requirements
- Audit trail for sensitive data access
- Proper data handling practices

## Usage Examples

### Creating a Tax ID Secret

```bash
# Create a secret with tax ID
kubectl create secret generic acme-corp-tax-id \
  --from-literal=tax-id="12-3456789" \
  --namespace=default

# Add labels for identification
kubectl label secret acme-corp-tax-id \
  vendor.miloapis.com/vendor=acme-corp \
  vendor.miloapis.com/type=tax-id
```

### Referencing in Vendor Resource

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
      namespace: "default"
    country: "United States"
    taxDocument: "W-9"
    taxVerified: true
```

### Multiple Tax IDs

For vendors with multiple tax IDs (e.g., international operations):

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: complex-vendor-tax-ids
  namespace: default
type: Opaque
data:
  ein: MTItMzQ1Njc4OQ==              # US EIN
  vat: R0IxMjM0NTY3ODk=              # EU VAT
  business-number: Q0ExMjM0NTY3ODk=  # Canadian Business Number
```

```yaml
# Reference specific tax ID
taxIdRef:
  secretName: "complex-vendor-tax-ids"
  secretKey: "ein"  # or "vat", "business-number"
  namespace: "default"
```

## API Functions

### Retrieving Tax ID

```go
// Get tax ID from secret
taxId, err := vendorsv1alpha1.GetTaxIdFromSecret(ctx, client, vendor, taxIdRef)
if err != nil {
    return fmt.Errorf("failed to get tax ID: %w", err)
}
```

### Validating Secret Reference

```go
// Validate that secret exists and contains the key
err := vendorsv1alpha1.ValidateTaxIdSecret(ctx, client, vendor, taxIdRef)
if err != nil {
    return fmt.Errorf("invalid tax ID secret: %w", err)
}
```

### Creating/Updating Secrets

```go
// Create new secret
err := vendorsv1alpha1.CreateTaxIdSecret(ctx, client, vendor, taxIdRef, "12-3456789")

// Update existing secret
err := vendorsv1alpha1.UpdateTaxIdSecret(ctx, client, vendor, taxIdRef, "12-3456789")

// Delete secret
err := vendorsv1alpha1.DeleteTaxIdSecret(ctx, client, vendor, taxIdRef)
```

## Best Practices

### 1. **Secret Naming Convention**
Use descriptive names that include the vendor name:
```bash
# Good
acme-corp-tax-id
john-doe-ssn
international-corp-vat

# Avoid
tax-secret-1
secret-abc
```

### 2. **Key Naming Convention**
Use consistent key names within secrets:
```yaml
data:
  tax-id: MTItMzQ1Njc4OQ==      # For EIN
  ssn: MTIzLTQ1LTY3ODk=         # For SSN
  vat-number: R0IxMjM0NTY3ODk=  # For VAT
```

### 3. **Namespace Strategy**
- Use the same namespace as the vendor when possible
- Specify namespace explicitly for cross-namespace references
- Consider using a dedicated namespace for sensitive data

### 4. **Labels and Annotations**
Add proper labels for identification and management:
```yaml
metadata:
  labels:
    vendor.miloapis.com/vendor: acme-corp
    vendor.miloapis.com/type: tax-id
    vendor.miloapis.com/country: us
  annotations:
    vendor.miloapis.com/tax-id-type: ein
    vendor.miloapis.com/created-by: admin
```

### 5. **Owner References**
Set owner references to ensure cleanup:
```yaml
ownerReferences:
  - apiVersion: vendors.miloapis.com/v1alpha1
    kind: Vendor
    name: acme-corp
    uid: 12345678-1234-1234-1234-123456789abc
    controller: true
```

## Migration from Plain Text

If you have existing vendors with plain text tax IDs:

1. **Create secrets** for each vendor's tax ID
2. **Update vendor resources** to use `TaxIdRef` instead of `TaxId`
3. **Verify secrets** are accessible and contain correct data
4. **Test validation** to ensure everything works

### Migration Script Example

```bash
#!/bin/bash
# Extract tax IDs and create secrets
kubectl get vendors -o json | jq -r '.items[] | select(.spec.taxInfo.taxId) | "\(.metadata.name) \(.spec.taxInfo.taxId)"' | while read vendor_name tax_id; do
  # Create secret
  kubectl create secret generic "${vendor_name}-tax-id" \
    --from-literal=tax-id="$tax_id" \
    --namespace=default
  
  # Add labels
  kubectl label secret "${vendor_name}-tax-id" \
    vendor.miloapis.com/vendor="$vendor_name" \
    vendor.miloapis.com/type=tax-id
done
```

## Security Considerations

### 1. **RBAC Configuration**
Ensure proper RBAC for secret access:
```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: vendor-secret-reader
rules:
- apiGroups: [""]
  resources: ["secrets"]
  verbs: ["get", "list"]
  resourceNames: ["*-tax-id"]  # Restrict to tax ID secrets
```

### 2. **Network Policies**
Consider network policies to restrict secret access:
```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: restrict-secret-access
spec:
  podSelector: {}
  policyTypes:
  - Ingress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          name: vendor-controllers
```

### 3. **Audit Logging**
Enable audit logging for secret access:
```yaml
apiVersion: audit.k8s.io/v1
kind: Policy
rules:
- level: Metadata
  resources:
  - group: ""
    resources: ["secrets"]
```

## Troubleshooting

### Common Issues

1. **Secret Not Found**
   ```
   Error: failed to get secret default/acme-corp-tax-id: secrets "acme-corp-tax-id" not found
   ```
   - Check secret name and namespace
   - Verify secret exists: `kubectl get secret acme-corp-tax-id`

2. **Key Not Found**
   ```
   Error: key tax-id not found in secret default/acme-corp-tax-id
   ```
   - Check secret key name
   - Verify key exists: `kubectl get secret acme-corp-tax-id -o yaml`

3. **Permission Denied**
   ```
   Error: secrets "acme-corp-tax-id" is forbidden: User cannot get resource "secrets"
   ```
   - Check RBAC permissions
   - Verify user has access to secrets

### Debugging Commands

```bash
# Check secret exists
kubectl get secret acme-corp-tax-id

# View secret contents (base64 encoded)
kubectl get secret acme-corp-tax-id -o yaml

# Decode secret value
kubectl get secret acme-corp-tax-id -o jsonpath='{.data.tax-id}' | base64 -d

# Check secret labels
kubectl get secret acme-corp-tax-id --show-labels

# Test secret access
kubectl auth can-i get secret acme-corp-tax-id
```

This approach ensures that sensitive tax ID information is properly secured while maintaining the flexibility and functionality of the vendor management system.
