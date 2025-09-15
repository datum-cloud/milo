# Dynamic Corporation Types

This document explains how to manage corporation types dynamically in the Milo system using the `CorporationTypeConfig` CRD.

## Overview

Instead of hardcoded corporation types, the system now supports dynamic corporation types that can be managed by staff users through Kubernetes resources. This allows for:

- Adding new corporation types without code changes
- Enabling/disabling corporation types
- Customizing display names and descriptions
- Organizing types with sort orders

## Architecture

### CorporationTypeConfig CRD

The `CorporationTypeConfig` CRD allows staff users to define available corporation types:

```yaml
apiVersion: resourcemanager.miloapis.com/v1alpha1
kind: CorporationTypeConfig
metadata:
  name: default-corporation-types
spec:
  active: true
  corporationTypes:
    - code: "llc"
      displayName: "Limited Liability Company (LLC)"
      description: "A business structure that combines..."
      enabled: true
      sortOrder: 10
```

### Vendor CRD Updates

The `Vendor` CRD now uses a string field for `corporationType` that references the codes defined in `CorporationTypeConfig`:

```yaml
apiVersion: resourcemanager.miloapis.com/v1alpha1
kind: Vendor
metadata:
  name: acme-corp
spec:
  profileType: business
  legalName: "ACME Corporation LLC"
  corporationType: "llc"  # References code from CorporationTypeConfig
  # ... other fields
```

## Usage

### 1. Create Corporation Type Configuration

Create a `CorporationTypeConfig` resource with your desired corporation types:

```yaml
apiVersion: resourcemanager.miloapis.com/v1alpha1
kind: CorporationTypeConfig
metadata:
  name: my-corporation-types
spec:
  active: true
  corporationTypes:
    - code: "llc"
      displayName: "LLC"
      description: "Limited Liability Company"
      enabled: true
      sortOrder: 10
    - code: "s-corp"
      displayName: "S Corporation"
      description: "S Corporation"
      enabled: true
      sortOrder: 20
    - code: "custom-type"
      displayName: "Custom Business Type"
      description: "Our custom business structure"
      enabled: true
      sortOrder: 30
```

### 2. Create Vendors with Dynamic Types

When creating vendors, use the codes defined in your `CorporationTypeConfig`:

```yaml
apiVersion: resourcemanager.miloapis.com/v1alpha1
kind: Vendor
metadata:
  name: example-vendor
spec:
  profileType: business
  legalName: "Example Business"
  corporationType: "llc"  # Must match a code from CorporationTypeConfig
  # ... other fields
```

### 3. Managing Corporation Types

#### Adding New Types

To add a new corporation type, update your `CorporationTypeConfig`:

```yaml
spec:
  corporationTypes:
    # ... existing types
    - code: "new-type"
      displayName: "New Business Type"
      description: "A new type of business structure"
      enabled: true
      sortOrder: 40
```

#### Disabling Types

To disable a corporation type without removing it:

```yaml
spec:
  corporationTypes:
    - code: "old-type"
      displayName: "Old Business Type"
      enabled: false  # Disable this type
      sortOrder: 50
```

#### Reordering Types

Use the `sortOrder` field to control display order (lower numbers appear first):

```yaml
spec:
  corporationTypes:
    - code: "priority-type"
      displayName: "Priority Type"
      enabled: true
      sortOrder: 5  # Will appear first
    - code: "other-type"
      displayName: "Other Type"
      enabled: true
      sortOrder: 100  # Will appear last
```

## Validation

The system validates that:

1. Only one `CorporationTypeConfig` can be active at a time
2. Corporation type codes must be unique within a config
3. Corporation type codes must match the pattern `^[a-z0-9-]+$`
4. Vendor `corporationType` values must reference valid, enabled codes

## API Functions

The system provides helper functions for validation and display:

```go
// Validate a corporation type against a config
err := ValidateCorporationType(vendor.Spec.CorporationType, config)

// Get display name for a corporation type
displayName := GetCorporationTypeDisplayName(vendor.Spec.CorporationType, config)

// Get all available corporation types
types := GetAvailableCorporationTypes(config)
```

## Migration from Hardcoded Types

If you have existing vendors with hardcoded corporation types, you'll need to:

1. Create a `CorporationTypeConfig` with the old hardcoded values
2. Update existing vendor resources to use the new string codes
3. Remove the old hardcoded enum from the codebase

## Best Practices

1. **Use descriptive codes**: Use lowercase, hyphenated codes like `"s-corp"` instead of `"scorp"`
2. **Keep codes stable**: Once vendors are using a code, avoid changing it
3. **Use sort orders**: Organize types logically for UI display
4. **Provide descriptions**: Help users understand what each type means
5. **Test changes**: Always test corporation type changes in a development environment first

## Examples

See the sample files:
- `config/samples/resourcemanager/v1alpha1/corporationtypeconfig-example.yaml`
- `config/samples/resourcemanager/v1alpha1/vendor-example.yaml`
