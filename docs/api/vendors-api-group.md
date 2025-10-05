# Vendors API Group

This document describes the new `vendors.miloapis.com` API group that was created to manage vendor-related resources separately from generic resource management.

## Overview

The vendors API group (`vendors.miloapis.com/v1alpha1`) contains all resources related to vendor management, providing a focused and organized approach to handling vendor data and configuration.

## API Group Structure

```
vendors.miloapis.com/v1alpha1/
├── Vendor                    # Main vendor resource
├── VendorList               # List of vendors
├── CorporationTypeConfig    # Configuration for corporation types
└── CorporationTypeConfigList # List of corporation type configs
```

## Resources

### Vendor

The main resource for managing vendor information.

**API Version:** `vendors.miloapis.com/v1alpha1`  
**Kind:** `Vendor`  
**Scope:** `Cluster`

**Key Features:**
- Support for both person and business profiles
- Comprehensive address management (billing and mailing)
- Secure tax information tracking (stored in Kubernetes Secrets)
- Business-specific fields (vendor type, DBA, etc.)
- Status management (pending, active, rejected, archived)

### VendorTypeDefinition

Individual resource for managing each vendor type definition.

**API Version:** `vendors.miloapis.com/v1alpha1`  
**Kind:** `VendorTypeDefinition`  
**Scope:** `Cluster`

**Key Features:**
- Individual vendor type definitions
- Enable/disable types without code changes
- Rich validation rules per type
- Customizable display names and descriptions
- Sort ordering and categorization
- Usage tracking and statistics

## Migration from ResourceManager

The vendor-related resources were moved from the `resourcemanager.miloapis.com` API group to the new `vendors.miloapis.com` API group for better organization and separation of concerns.

### Changes Made

1. **New API Group Created:**
   - `vendors.miloapis.com/v1alpha1`
   - Dedicated scheme and registration
   - Separate CRD manifests

2. **Resources Moved:**
   - `Vendor` → `vendors.miloapis.com/v1alpha1`
   - `CorporationTypeConfig` → `vendors.miloapis.com/v1alpha1`

3. **Files Updated:**
   - Controller manager scheme registration
   - Kustomization files
   - Sample resources
   - Validation controllers

4. **Old Resources Removed:**
   - Vendor types removed from `resourcemanager.miloapis.com`
   - Old CRD manifests cleaned up
   - Old sample files removed

## Usage Examples

### Creating a Vendor

```yaml
apiVersion: vendors.miloapis.com/v1alpha1
kind: Vendor
metadata:
  name: acme-corp
spec:
  profileType: business
  legalName: "ACME Corporation LLC"
  corporationType: llc
  status: active
  # ... other fields
```

### Managing Corporation Types

```yaml
apiVersion: vendors.miloapis.com/v1alpha1
kind: CorporationTypeConfig
metadata:
  name: default-corporation-types
spec:
  active: true
  corporationTypes:
    - code: "llc"
      displayName: "Limited Liability Company (LLC)"
      enabled: true
      sortOrder: 10
```

## Benefits of Separate API Group

1. **Clear Separation of Concerns:**
   - Vendors are a distinct business domain
   - Separate from generic resource management
   - Easier to understand and maintain

2. **Focused API Management:**
   - Dedicated API versioning
   - Independent evolution of vendor features
   - Clearer RBAC and permissions

3. **Better Organization:**
   - Logical grouping of related resources
   - Easier to find and manage vendor-related code
   - Cleaner API documentation

4. **Scalability:**
   - Can add vendor-specific features without affecting resource management
   - Independent scaling and deployment
   - Easier to add vendor-specific controllers and webhooks

## File Structure

```
pkg/apis/vendors/
├── scheme.go
└── v1alpha1/
    ├── doc.go
    ├── register.go
    ├── vendor_types.go
    ├── corporationtypeconfig_types.go
    ├── validation.go
    └── zz_generated.deepcopy.go

config/
├── crd/bases/vendors/
│   ├── kustomization.yaml
│   ├── vendors.miloapis.com_vendors.yaml
│   └── vendors.miloapis.com_corporationtypeconfigs.yaml
└── samples/vendors/v1alpha1/
    ├── vendor-example.yaml
    └── corporationtypeconfig-example.yaml
```
