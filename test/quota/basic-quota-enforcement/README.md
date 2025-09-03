# Basic Quota Enforcement Test

This directory contains a Chainsaw end-to-end test that validates the core Milo quota enforcement functionality.

## Directory Structure

```
basic-quota-enforcement/
├── chainsaw-test.yaml               # Main test definition
├── kubeconfig-main                  # Main API kubeconfig
├── kubeconfig-org-template          # Organizational context kubeconfig
├── 01-resource-registration.yaml    # Base quota infrastructure
├── 02-test-organization.yaml        # Test organization setup
├── 03-test-user.yaml                # Test user creation
├── 04-organization-membership.yaml  # User-organization association
├── project-creation-policy.yaml     # Quota enforcement policy
├── standard-org-grant-template.yaml # Resource allocation template
├── test-data/                       # Test input data
│   ├── projects-within-quota.yaml
│   └── project-exceeds-quota.yaml
└── assertions/                      # Test validations
    ├── assert-organization.yaml
    ├── assert-projects.yaml
    ├── assert-resource-claims.yaml
    ├── assert-resource-registration.yaml
    └── assert-projects-created.yaml
```

## Test Scenario

This test validates the complete quota enforcement flow:

1. **Setup Phase**: Creates base quota infrastructure (ResourceRegistration, Organization, User, OrganizationMembership)
2. **Grant Phase**: Allocates quota resources to the test organization via ResourceGrant
3. **Policy Phase**: Establishes quota enforcement via ClaimCreationPolicy
4. **Within-Quota Test**: Creates projects within quota limits (should succeed)
5. **Quota Verification**: Verifies ResourceClaims are created and allocated correctly
6. **Enforcement Test**: Attempts to exceed quota limits (should fail with quota error)
7. **Cleanup Phase**: Removes test resources

## Running the Test

### Direct Execution
```bash
# From the test/quota directory
chainsaw test basic-quota-enforcement/

# With additional options
chainsaw test basic-quota-enforcement/ --fail-fast
```

### Via Task System
```bash
# From the project root
TASK_X_REMOTE_TASKFILES=1 task test:end-to-end -- quota
```

## Cluster Contexts

The test uses two kubeconfig contexts:

- **main**: Standard API operations against `https://localhost:30443`
- **org**: Organizational context API operations against `https://localhost:30443/apis/resourcemanager.miloapis.com/v1alpha1/organizations/test-quota-org/control-plane`

Project creation must go through the organizational context API to properly trigger quota enforcement.

## Resource Labeling

All test resources are labeled for identification and cleanup:
```yaml
labels:
  test.miloapis.com/quota: "true"
  test.miloapis.com/scenario: "basic-quota-enforcement"
```

## Debugging

### Resource Inspection
```bash
# Check quota system resources
kubectl get resourceregistrations,resourcegrants,resourceclaims,claimcreationpolicies -A

# Check test organization and projects
kubectl get organizations,projects -A

# Check test-specific resources
kubectl get all -A -l test.miloapis.com/scenario=basic-quota-enforcement
```

### Manual Cleanup
```bash
# Remove test resources if cleanup fails
kubectl delete all -A -l test.miloapis.com/scenario=basic-quota-enforcement

# Remove quota-specific test resources
kubectl delete resourceregistrations,resourcegrants,resourceclaims,claimcreationpolicies -A -l test.miloapis.com/scenario=basic-quota-enforcement
```

## Expected Behavior

- **Projects within quota**: Should create successfully and show `Active` phase
- **Projects exceeding quota**: Should fail with quota enforcement error
- **ResourceClaims**: Should be created automatically and show `Allocated` state
- **ClaimCreationPolicy**: Should be `Ready` and actively enforcing limits
