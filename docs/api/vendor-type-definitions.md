# VendorTypeDefinition Architecture

This document explains the new `VendorTypeDefinition` architecture that replaces the previous `CorporationTypeConfig` approach.

## Overview

The new architecture uses individual `VendorTypeDefinition` resources instead of a single configuration resource. This provides better separation of concerns, independent lifecycle management, and more flexible validation rules.

## Architecture Comparison

| Aspect | Old (CorporationTypeConfig) | New (VendorTypeDefinition) |
|--------|----------------------------|----------------------------|
| **Resource Type** | Single config resource | Individual definition resources |
| **Management** | Bulk configuration | Per-type management |
| **Lifecycle** | Single resource lifecycle | Independent lifecycles |
| **Validation** | Basic enabled/disabled | Rich validation rules |
| **Metadata** | Limited fields | Comprehensive metadata |
| **Scalability** | Monolithic | Microservice-friendly |

## Key Benefits

### 1. **Independent Lifecycle Management**
Each vendor type can be managed independently:
- Enable/disable without affecting other types
- Update individual types without touching others
- Delete unused types without breaking others

### 2. **Rich Validation Rules**
Each type can have its own validation requirements:
```yaml
spec:
  requiresBusinessFields: true
  requiresTaxVerification: true
  validCountries: ["US", "CA"]
  requiredTaxDocuments: ["W-9", "W-8BEN"]
```

### 3. **Better Organization**
Types can be categorized and organized:
```yaml
spec:
  category: "business"
  sortOrder: 10
  displayName: "Limited Liability Company (LLC)"
```

### 4. **Usage Tracking**
Each type tracks its own usage statistics:
```yaml
status:
  vendorCount: 42
  lastUsed: "2024-01-15T10:30:00Z"
```

## Resource Structure

### VendorTypeDefinition

```yaml
apiVersion: vendors.miloapis.com/v1alpha1
kind: VendorTypeDefinition
metadata:
  name: llc
spec:
  code: "llc"                           # Unique identifier
  displayName: "LLC"                    # Human-readable name
  description: "..."                    # Detailed description
  enabled: true                         # Whether type is available
  sortOrder: 10                         # Display order
  category: "business"                  # Type category
  requiresBusinessFields: true          # Business fields required
  requiresTaxVerification: true         # Tax verification required
  validCountries: ["US"]                # Valid countries
  requiredTaxDocuments: ["W-9"]         # Required tax docs
status:
  observedGeneration: 1
  conditions:
    - type: "Ready"
      status: "True"
  vendorCount: 42                       # Usage statistics
  lastUsed: "2024-01-15T10:30:00Z"     # Last usage
```

### Updated Vendor Resource

```yaml
apiVersion: vendors.miloapis.com/v1alpha1
kind: Vendor
metadata:
  name: acme-corp
spec:
  profileType: business
  legalName: "ACME Corporation LLC"
  vendorType: "llc"                     # References VendorTypeDefinition
  # ... other fields
```

## Migration Path

### From CorporationTypeConfig to VendorTypeDefinition

1. **Create individual definitions** for each type:
   ```bash
   # Old approach
   kubectl get corporationtypeconfigs
   
   # New approach
   kubectl get vendortypedefinitions
   ```

2. **Update vendor resources** to use new field:
   ```yaml
   # Old
   corporationType: "llc"
   
   # New
   vendorType: "llc"
   ```

3. **Remove old resources**:
   ```bash
   kubectl delete corporationtypeconfigs --all
   ```

## API Functions

### Validation Functions

```go
// Validate against specific definition
err := ValidateVendorType(vendor.Spec.VendorType, definition)

// Validate against list of definitions
err := ValidateVendorTypeFromList(vendor.Spec.VendorType, definitions)
```

### Display Functions

```go
// Get display name
displayName := GetVendorTypeDisplayName(vendor.Spec.VendorType, definition)

// Get display name from list
displayName := GetVendorTypeDisplayNameFromList(vendor.Spec.VendorType, definitions)
```

### Utility Functions

```go
// Get available types
availableTypes := GetAvailableVendorTypes(definitions)

// Find specific definition
definition := FindVendorTypeDefinition("llc", definitions)
```

## Controller Integration

The validation controller now works with individual definitions:

```go
// Get all VendorTypeDefinitions
var definitionList vendorsv1alpha1.VendorTypeDefinitionList
if err := r.List(ctx, &definitionList); err != nil {
    return ctrl.Result{}, err
}

// Validate vendor type
if err := vendorsv1alpha1.ValidateVendorTypeFromList(
    vendor.Spec.VendorType, 
    definitionList.Items,
); err != nil {
    return ctrl.Result{}, err
}
```

## Best Practices

1. **Use descriptive resource names** that match the code (e.g., `name: llc` for `code: "llc"`)
2. **Set appropriate categories** to group related types
3. **Use sort orders** to control display order
4. **Set validation rules** based on business requirements
5. **Monitor usage statistics** to identify unused types
6. **Keep codes stable** once vendors are using them

## Future Enhancements

The new architecture enables future enhancements:

1. **Per-type controllers** for business logic
2. **Type-specific webhooks** for validation
3. **Usage analytics** and reporting
4. **Type dependencies** and relationships
5. **Custom validation rules** per type
6. **Type-specific UI components**

This architecture provides a much more flexible and maintainable approach to managing vendor types in the Milo system.
