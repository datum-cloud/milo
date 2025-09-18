# Quota System Test Suites

This directory contains comprehensive end-to-end test suites for the Milo quota system, organized into focused test categories for better maintainability and debugging.

## Test Structure

The quota tests are organized into four main test suites:

### 1. Core Functionality (`core-functionality/`)
**Focus**: Basic quota operations and fundamental workflows
**Execution Time**: ~2-3 minutes

Tests the essential quota system functionality:
- ResourceRegistration creation and activation
- ClaimCreationPolicy setup and validation
- Organization and resource grant setup
- Basic project creation within quota limits
- Automatic ResourceClaim creation and processing
- Simple quota enforcement

**Key Test Steps**:
1. Setup base infrastructure (ResourceRegistration)
2. Setup claim creation policy
3. Create basic organization with user and membership
4. Create resource grant
5. Test project creation within quota
6. Verify automatic resource claims
7. Test basic quota enforcement

### 2. Multi-Resource Claims (`multi-resource-claims/`)
**Focus**: Advanced multi-resource quota functionality
**Execution Time**: ~4-5 minutes

Tests complex quota scenarios involving multiple resource types:
- Multiple ResourceRegistrations (Projects, Users, Clusters)
- Multi-resource ResourceGrants
- Complex ResourceClaims requesting multiple resource types
- Partial denial scenarios
- Resource type validation and edge cases

**Key Test Steps**:
1. Setup multiple resource registrations
2. Create multi-resource organization
3. Setup multiple resource grants
4. Test successful multi-resource claims
5. Test partial denial scenarios
6. Test invalid duplicate claims

### 3. Enforcement Edge Cases (`enforcement-edge-cases/`)
**Focus**: Quota enforcement boundaries and error conditions
**Execution Time**: ~3-4 minutes

Tests quota system edge cases and error handling:
- Strict quota enforcement scenarios
- Concurrent resource creation and race conditions
- Invalid resource claims (zero/negative amounts)
- ClaimCreationPolicy validation edge cases
- Boundary condition testing

**Key Test Steps**:
1. Setup quota enforcement infrastructure
2. Test strict quota enforcement
3. Test concurrent creation scenarios
4. Test boundary conditions (zero/negative amounts)
5. Test policy validation edge cases

### 4. Grant Creation Policy (`grant-creation-policy/`)
**Focus**: Automated ResourceGrant creation based on policy triggers
**Execution Time**: ~3-4 minutes

Tests the GrantCreationPolicy functionality for automated grant management:
- GrantCreationPolicy validation and setup
- Automatic ResourceGrant creation on trigger resource events
- CEL expression evaluation for conditions and name generation
- Go template rendering with different context values
- Owner reference management and cleanup
- Policy priority and event type filtering

**Key Test Steps**:
1. Setup ResourceRegistration for test resources
2. Create and validate GrantCreationPolicy with conditions
3. Create trigger Organizations with different attributes
4. Verify automatic ResourceGrant creation with proper owner references
5. Test condition evaluation with resource updates
6. Test CEL-based dynamic name generation
7. Verify grant cleanup when trigger resources are deleted
8. Test policy validation and error handling

## Running Tests

### Run Individual Test Suites

```bash
# Run core functionality tests only
task test:end-to-end -- quota/core-functionality

# Run multi-resource claims tests only
task test:end-to-end -- quota/multi-resource-claims

# Run enforcement edge cases tests only
task test:end-to-end -- quota/enforcement-edge-cases

# Run grant creation policy tests only
task test:end-to-end -- quota/grant-creation-policy
```

### Run All Quota Tests

```bash
# Run all quota test suites
task test:end-to-end -- quota
```

### Run Specific Test Steps

Use Chainsaw's test selection capabilities:
```bash
# Run only setup steps from core functionality
KUBECONFIG=.milo/kubeconfig chainsaw test test/quota/core-functionality --include-test-regex "setup.*"

# Run only enforcement tests
KUBECONFIG=.milo/kubeconfig chainsaw test test/quota/enforcement-edge-cases --include-test-regex ".*enforcement.*"
```

## Test Dependencies

### Required Infrastructure
- Milo API server running (via `task dev:setup`)
- Test infrastructure cluster available
- Kubeconfig files configured properly

### Resource Dependencies
Each test suite includes its own dependency setup:
- **ResourceRegistrations**: Define quota-able resource types
- **Organizations**: Test consumers of quota resources
- **ResourceGrants**: Allocate quota to organizations
- **ClaimCreationPolicies**: Automate resource claim creation

### Test Data Organization
```
test/quota/
├── core-functionality/
│   ├── chainsaw-test.yaml          # Main test definition
│   ├── 01-resource-registration.yaml
│   ├── 02-basic-quota-organization.yaml
│   ├── project-creation-policy.yaml
│   ├── test-data/
│   │   ├── projects-within-quota.yaml
│   │   └── project-exceeds-quota.yaml
│   └── assertions/
│       ├── assert-projects-created.yaml
│       └── assert-resource-claims.yaml
├── multi-resource-claims/
│   └── [similar structure with multi-resource test data]
├── enforcement-edge-cases/
│   └── [similar structure with edge case test data]
└── grant-creation-policy/
    ├── chainsaw-test.yaml          # Main test definition
    ├── 01-resource-registration.yaml
    ├── grant-creation-policy.yaml
    ├── cel-name-policy.yaml
    ├── test-data/
    │   ├── premium-organization.yaml
    │   ├── cel-trigger-organization.yaml
    │   └── invalid-policy.yaml
    └── assertions/
        ├── assert-automatic-grant.yaml
        ├── assert-cel-named-grant.yaml
        └── assert-grant-cleanup.yaml
```

## Troubleshooting

### Common Issues

1. **ClaimCreationPolicy Not Ready**
   - **Root Cause**: ResourceRegistration not active before policy creation
   - **Solution**: Ensure ResourceRegistration has `Active: True` status before creating policies
   - **Check**: `task kubectl -- get resourceregistrations`

2. **Resource Claims Not Created**
   - **Root Cause**: ClaimCreationPolicy validation failing
   - **Solution**: Verify all referenced resource types have active ResourceRegistrations
   - **Check**: `task kubectl -- get claimcreationpolicies -o yaml`

3. **Projects Not Created**
   - **Root Cause**: Quota enforcement blocking creation or missing ResourceGrants
   - **Solution**: Check ResourceGrant allocation and AllowanceBucket status
   - **Check**: `task kubectl -- get resourcegrants -A` and `task kubectl -- get allowancebuckets -A`

4. **Test Timeouts**
   - **Root Cause**: Controllers not reconciling resources quickly enough
   - **Solution**: Check controller logs for errors
   - **Check**: `task test-infra:kubectl -- logs -n milo-system -l app.kubernetes.io/name=milo-controller-manager`

### Debug Commands

```bash
# Check quota system status
task kubectl -- get resourceregistrations,claimcreationpolicies,resourcegrants,resourceclaims -A

# Check controller logs
task test-infra:kubectl -- logs -n milo-system -l app.kubernetes.io/name=milo-controller-manager --tail=50

# Check API server logs
task test-infra:kubectl -- logs -n milo-system -l app.kubernetes.io/name=milo-apiserver --tail=50

# Check specific resource status
task kubectl -- get claimcreationpolicy test-project-quota-policy -o yaml
task kubectl -- get resourceregistration test-projects-per-org -o yaml
```

## Test Development Guidelines

### Adding New Test Cases

1. **Choose the Right Suite**: Add tests to the most appropriate suite based on functionality
2. **Follow Naming Conventions**: Use descriptive names that indicate the test purpose
3. **Include Proper Assertions**: Always include assertion files to verify expected outcomes
4. **Test Error Conditions**: Include both positive and negative test cases
5. **Use Proper Cleanup**: Ensure resources are properly cleaned up or use Chainsaw's auto-cleanup

### Test File Organization

- **Setup Files** (`01-`, `02-`, etc.): Infrastructure and dependency resources
- **Test Data** (`test-data/`): Resources that trigger the actual test scenarios
- **Assertions** (`assertions/`): Expected outcomes and validation criteria
- **Kubeconfig**: Organizational context configuration for cross-cluster testing

### Best Practices

1. **Dependency Ordering**: Always create ResourceRegistrations before ClaimCreationPolicies
2. **Wait Conditions**: Use appropriate wait conditions for resource readiness
3. **Timeout Values**: Set reasonable timeouts (30s for most resources)
4. **Test Isolation**: Each test suite should be independently runnable
5. **Clear Naming**: Use descriptive names that indicate test purpose and expected outcome

## Migration from Old Test Structure

The quota test suite has been reorganized from a monolithic test structure into these focused suites for better maintainability and debugging.

### Benefits of New Structure

- **Faster Feedback**: Run only relevant tests during development
- **Easier Debugging**: Smaller test surface area when failures occur
- **Better Organization**: Tests grouped by functional area
- **Parallel Execution**: Different suites can be run concurrently
- **Clearer Intent**: Each suite has a specific focus and purpose