# Dynamic Vendor Types

This document explains how to manage vendor types dynamically in the Milo system using individual `VendorTypeDefinition` resources.

## Overview

Instead of hardcoded vendor types, the system now supports dynamic vendor types that can be managed by staff users through individual Kubernetes resources. This allows for:

- Adding new vendor types without code changes
- Enabling/disabling individual vendor types
- Customizing display names and descriptions
- Organizing types with sort orders
- Independent lifecycle management for each type
- Rich metadata and validation rules per type

## Architecture

### VendorTypeDefinition CRD

Each vendor type is now a separate `VendorTypeDefinition` resource with its own spec and status:

```yaml
apiVersion: vendors.miloapis.com/v1alpha1
kind: VendorTypeDefinition
metadata:
  name: llc
spec:
  code: "llc"
  displayName: "Limited Liability Company (LLC)"
  description: "A business structure that combines..."
  enabled: true
  sortOrder: 10
  category: "business"
  requiresBusinessFields: true
  requiresTaxVerification: true
  validCountries: ["US"]
  requiredTaxDocuments: ["W-9", "W-8BEN"]
```

### Vendor CRD Updates

The `Vendor` CRD now uses a string field for `vendorType` that references the codes defined in `VendorTypeDefinition` resources:

```yaml
apiVersion: vendors.miloapis.com/v1alpha1
kind: Vendor
metadata:
  name: acme-corp
spec:
  profileType: business
  legalName: "ACME Corporation LLC"
  vendorType: "llc"  # References code from VendorTypeDefinition
  # ... other fields
```

## Usage

### 1. Create Individual Vendor Type Definitions

Create separate `VendorTypeDefinition` resources for each vendor type:

```yaml
apiVersion: vendors.miloapis.com/v1alpha1
kind: VendorTypeDefinition
metadata:
  name: llc
spec:
  code: "llc"
  displayName: "Limited Liability Company (LLC)"
  description: "A business structure that combines..."
  enabled: true
  sortOrder: 10
  category: "business"
  requiresBusinessFields: true
  requiresTaxVerification: true
  validCountries: ["US"]
  requiredTaxDocuments: ["W-9"]
---
apiVersion: vendors.miloapis.com/v1alpha1
kind: VendorTypeDefinition
metadata:
  name: s-corp
spec:
  code: "s-corp"
  displayName: "S Corporation"
  description: "A special type of corporation..."
  enabled: true
  sortOrder: 20
  category: "business"
  requiresBusinessFields: true
  requiresTaxVerification: true
  validCountries: ["US"]
  requiredTaxDocuments: ["W-9"]
```

### 2. Create Vendors with Dynamic Types

When creating vendors, use the codes defined in your `VendorTypeDefinition` resources:

```yaml
apiVersion: vendors.miloapis.com/v1alpha1
kind: Vendor
metadata:
  name: example-vendor
spec:
  profileType: business
  legalName: "Example Business"
  vendorType: "llc"  # Must match a code from VendorTypeDefinition
  # ... other fields
```

### 3. Managing Vendor Types

#### Adding New Types

To add a new vendor type, create a new `VendorTypeDefinition` resource:

```yaml
apiVersion: vendors.miloapis.com/v1alpha1
kind: VendorTypeDefinition
metadata:
  name: new-type
spec:
  code: "new-type"
  displayName: "New Business Type"
  description: "A new type of business structure"
  enabled: true
  sortOrder: 40
  category: "business"
  requiresBusinessFields: true
  requiresTaxVerification: true
  validCountries: ["US"]
  requiredTaxDocuments: ["W-9"]
```

#### Disabling Types

To disable a vendor type, update its `enabled` field:

```yaml
apiVersion: vendors.miloapis.com/v1alpha1
kind: VendorTypeDefinition
metadata:
  name: old-type
spec:
  code: "old-type"
  displayName: "Old Business Type"
  enabled: false  # Disable this type
  sortOrder: 50
  # ... other fields
```

#### Reordering Types

Use the `sortOrder` field to control display order (lower numbers appear first):

```yaml
apiVersion: vendors.miloapis.com/v1alpha1
kind: VendorTypeDefinition
metadata:
  name: priority-type
spec:
  code: "priority-type"
  displayName: "Priority Type"
  enabled: true
  sortOrder: 5  # Will appear first
  # ... other fields
```

## Validation

The system validates that:

1. Vendor type codes must be unique across all `VendorTypeDefinition` resources
2. Vendor type codes must match the pattern `^[a-z0-9-]+$`
3. Vendor `vendorType` values must reference valid, enabled `VendorTypeDefinition` resources
4. Business fields are required when `requiresBusinessFields` is true
5. Tax verification is required when `requiresTaxVerification` is true

## API Functions

The system provides helper functions for validation and display:

```go
// Validate a vendor type against a specific definition
err := ValidateVendorType(vendor.Spec.VendorType, definition)

// Validate a vendor type against a list of definitions
err := ValidateVendorTypeFromList(vendor.Spec.VendorType, definitions)

// Get display name for a vendor type
displayName := GetVendorTypeDisplayName(vendor.Spec.VendorType, definition)

// Get display name from a list of definitions
displayName := GetVendorTypeDisplayNameFromList(vendor.Spec.VendorType, definitions)

// Get all available vendor types
availableTypes := GetAvailableVendorTypes(definitions)

// Find a specific vendor type definition
definition := FindVendorTypeDefinition("llc", definitions)
```

## Migration from Hardcoded Types

If you have existing vendors with hardcoded vendor types, you'll need to:

1. Create individual `VendorTypeDefinition` resources for each type
2. Update existing vendor resources to use the new string codes
3. Remove the old hardcoded enum from the codebase

## Best Practices

1. **Use descriptive codes**: Use lowercase, hyphenated codes like `"s-corp"` instead of `"scorp"`
2. **Keep codes stable**: Once vendors are using a code, avoid changing it
3. **Use sort orders**: Organize types logically for UI display
4. **Provide descriptions**: Help users understand what each type means
5. **Test changes**: Always test vendor type changes in a development environment first
6. **Use categories**: Group related types together (business, nonprofit, international, etc.)
7. **Set validation rules**: Use `requiresBusinessFields` and `requiresTaxVerification` appropriately
8. **Specify countries**: Use `validCountries` to restrict types to specific regions

## Advanced Features

### Categories

Use the `category` field to group vendor types:

- `business` - Standard business entities
- `nonprofit` - Non-profit organizations
- `international` - International business structures
- `individual` - Individual contractors
- `other` - Miscellaneous types

### Validation Rules

Each vendor type can specify validation requirements:

- `requiresBusinessFields` - Whether business-specific fields are required
- `requiresTaxVerification` - Whether tax verification is mandatory
- `validCountries` - Countries where this type is valid
- `requiredTaxDocuments` - Required tax document types

### Status Tracking

The status field tracks usage statistics:

- `vendorCount` - Number of vendors using this type
- `lastUsed` - Last time this type was used

## Examples

See the sample files:
- `config/samples/vendors/v1alpha1/vendortypedefinition-example.yaml`
- `config/samples/vendors/v1alpha1/vendor-example.yaml`
