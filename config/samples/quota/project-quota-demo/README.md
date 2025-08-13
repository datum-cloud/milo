# Project Quota Management Demo

This directory contains a complete example of how to implement project quota management in Milo using ResourceRegistrations, ResourceGrants, and ClaimCreationPolicies.

## Overview

This demo shows how to:
1. Register a quota resource type for projects per organization
2. Grant quota allowances for project creation
3. Automatically create ResourceClaims when projects are created
4. Enforce quota limits through admission control

## Components

### 1. ResourceRegistration (`01-resource-registration.yaml`)
Defines the quota resource type for tracking "projects per organization". This establishes:
- Resource type: `resourcemanager.miloapis.com/Project`
- Owner type: Organization
- Unit of measurement: Projects
- Dimensions for categorization (tier, region, etc.)

### 2. ResourceGrant (`02-resource-grant.yaml`)
Provides quota allowances for a specific organization. This grants:
- 5 projects for standard tier
- 2 projects for development tier
- 10 projects for production tier
- Different limits based on region

### 3. ClaimCreationPolicy (`03-claim-creation-policy.yaml`)
Automatically creates ResourceClaims when Project resources are created. This policy:
- Targets Project creation/updates
- Creates claims based on project tier and region
- Uses CEL expressions for dynamic quota calculation
- Ensures quota enforcement before project creation

### 4. Test Resources
- `04-test-organization.yaml`: Sample organization for testing
- `05-test-projects.yaml`: Sample projects to test quota enforcement

## Setup Instructions

1. **Apply the ResourceRegistration first:**
   ```bash
   kubectl apply -f 01-resource-registration.yaml
   ```

2. **Apply the ResourceGrant to provide quota:**
   ```bash
   kubectl apply -f 02-resource-grant.yaml
   ```

3. **Apply the ClaimCreationPolicy to enable enforcement:**
   ```bash
   kubectl apply -f 03-claim-creation-policy.yaml
   ```

4. **Create the test organization:**
   ```bash
   kubectl apply -f 04-test-organization.yaml
   ```

5. **Try creating test projects (some should succeed, others should fail due to quota):**
   ```bash
   kubectl apply -f 05-test-projects.yaml
   ```

## Expected Behavior

- **First 5 standard projects**: Should be created successfully
- **6th standard project**: Should be rejected due to quota exceeded
- **Development projects**: First 2 should succeed, 3rd should fail
- **Production projects**: First 10 should succeed, 11th should fail

## Monitoring

Check the quota status with:
```bash
# View ResourceClaims created
kubectl get resourceclaims -n milo-system

# Check ResourceGrant status
kubectl get resourcegrants -n milo-system -o yaml

# Monitor ClaimCreationPolicy status
kubectl get claimcreationpolicies -o wide
```

## Troubleshooting

If projects are being created without quota checks:
1. Verify the ClaimCreationPolicy is Ready: `kubectl get claimcreationpolicies`
2. Check admission plugin logs: Look for "ClaimCreationQuota" entries
3. Ensure ResourceRegistration is Active: `kubectl get resourceregistrations -o wide`
4. Verify ResourceGrant provides sufficient quota: `kubectl describe resourcegrant`