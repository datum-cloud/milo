# Quota Service Configuration

This directory contains the service configuration for Milo's quota management system, organized by functional domain.

## Overview

The quota system provides comprehensive resource quota management for Milo, enabling organizations to control and monitor resource usage across their tenants. It consists of six core resource types working together to provide real-time quota enforcement.

## Directory Structure

```
config/services/quota/
├── README.md                          # This documentation
├── kustomization.yaml                # Main service configuration
└── iam/                              # IAM configurations
    ├── kustomization.yaml            # IAM resource configuration
    ├── protected-resources/          # ProtectedResource CRDs
    │   ├── kustomization.yaml
    │   ├── resourceregistration.yaml
    │   ├── resourcegrant.yaml
    │   ├── resourceclaim.yaml
    │   ├── allowancebucket.yaml
    │   ├── grantcreationpolicy.yaml
    │   └── claimcreationpolicy.yaml
    └── roles/                        # RBAC Role CRDs
        ├── kustomization.yaml
        ├── quota-admin.yaml
        ├── quota-manager.yaml
        ├── quota-viewer.yaml
        ├── quota-operator.yaml
        └── organization-quota-manager.yaml
```

## Resource Types

### Core Quota Resources

1. **ResourceRegistration** (cluster-scoped)
   - Defines which resource types can be managed with quota
   - Specifies measurement units and consumer types
   - Foundation for all quota operations

2. **ResourceGrant** (namespaced)
   - Allocates quota to consumers (organizations, projects)
   - Can be created manually or automatically via GrantCreationPolicy
   - Multiple grants can contribute to the same quota pool

3. **ResourceClaim** (namespaced)
   - Claims quota when resources are created
   - Provides real-time admission control
   - Created automatically via ClaimCreationPolicy

4. **AllowanceBucket** (namespaced)
   - Tracks quota limits and usage per consumer+resource type
   - System-managed aggregation of grants and claims
   - Single source of truth for quota enforcement decisions

### Policy Resources

5. **GrantCreationPolicy** (cluster-scoped)
   - Automates ResourceGrant creation based on trigger conditions
   - Supports CEL expressions and Go templates
   - Enables dynamic quota provisioning

6. **ClaimCreationPolicy** (cluster-scoped)
   - Automates ResourceClaim creation during resource admission
   - Integrates with Kubernetes admission webhooks
   - Provides real-time quota enforcement

## IAM Integration

### Protected Resources

Each quota resource type is registered as a ProtectedResource in Milo's IAM system, enabling:
- Fine-grained permission control
- Integration with Milo's RBAC system
- Audit logging of quota operations

### RBAC Roles

Five predefined roles provide different levels of access:

#### Administrative Roles

- **quota.miloapis.com-admin**: Full access to all quota resources
  - Platform administrators and quota system maintainers
  - Can manage registrations, policies, grants, and claims

- **quota.miloapis.com-manager**: Quota allocation and management
  - Quota administrators who allocate quota to organizations
  - Can create/manage grants and claims, read-only access to policies

#### Operational Roles

- **quota.miloapis.com-operator**: System automation access
  - Automated systems and controllers
  - Can manage AllowanceBuckets and cleanup ResourceClaims
  - Read-only access to grants and registrations

- **quota.miloapis.com-viewer**: Read-only monitoring access
  - Monitoring systems, auditors, support staff
  - Can view all quota resources but not modify them

#### User Roles

- **quota.miloapis.com-organization-quota-manager**: Organization-scoped access
  - Organization administrators
  - Can view quota status and claims for their organizations
  - Read-only access to understand quota limits and usage

## Usage Patterns

### Initial Setup

1. Deploy ResourceRegistrations for resource types requiring quota
2. Configure GrantCreationPolicies for automatic quota allocation
3. Configure ClaimCreationPolicies for admission control
4. Assign appropriate IAM roles to users and service accounts

### Runtime Operations

1. **Automatic Grant Creation**: GrantCreationPolicies create ResourceGrants when organizations are created
2. **Real-time Enforcement**: ClaimCreationPolicies create ResourceClaims during resource admission
3. **Quota Tracking**: AllowanceBuckets aggregate limits and usage automatically
4. **Monitoring**: Use IAM roles to provide appropriate access for monitoring and administration

## Security Considerations

- All quota operations are logged through Milo's IAM system
- Fine-grained permissions enable principle of least privilege
- Organization-scoped roles prevent cross-tenant access
- System-managed resources (AllowanceBuckets) are protected from direct user modification

## Related Documentation

- [Quota System Architecture](../../../docs/quota-system.md)
- [IAM Integration Guide](../../../docs/iam-integration.md)
- [Resource Registration Guide](../../../docs/resource-registration.md)