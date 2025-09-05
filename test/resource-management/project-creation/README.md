# Project Ready Status Test

This end-to-end test focuses specifically on validating that projects can be created successfully and reach the `Ready` status condition.

## Test Scope

This test verifies:

1. **Project Creation via Organizational Context**: Projects can be created through the organizational control-plane API endpoint
2. **Project Controller Functionality**: The project controller processes projects correctly without TLS certificate issues
3. **Ready Status Condition**: Projects reach the `Ready` status condition with `status: "True"`
4. **Cross-Cluster Visibility**: Projects created via organizational context are visible in the main cluster

## Test Structure

### Setup Phase
- Creates a test organization (`test-project-ready-org`)
- Creates a test user (`2001`)
- Creates organization membership to link user to organization

### Test Phase
- Creates a project through the organizational context API (using `kubeconfig-org-template`)
- Waits for the project to reach `Ready` status condition (60-second timeout)
- Verifies the project status using assertions
- Confirms the project is visible from the main cluster context

### Cleanup Phase
- Deletes the test project
- Waits for proper deletion

## Key Differences from Other Tests

Unlike the quota enforcement tests which focus on resource allocation and limits, this test:

- **Focuses solely on project lifecycle**: Creation → Ready → Deletion
- **Uses correct status format**: Tests for `status.conditions[].type: Ready` instead of deprecated `status.phase`
- **Validates TLS fix**: Ensures project controller can communicate with project control-plane endpoints
- **Tests organizational context**: Verifies the organizational control-plane API endpoints work correctly

## Files

- `01-test-organization.yaml`: Test organization resource
- `02-test-user.yaml`: Test user resource
- `03-organization-membership.yaml`: Organization membership linking user to org
- `04-test-project.yaml`: Test project to be created
- `kubeconfig-org-template`: Kubeconfig for organizational context access
- `kubeconfig-main`: Kubeconfig for main cluster access
- `assertions/assert-project-ready.yaml`: Validates project reaches Ready status
- `assertions/assert-project-exists-main.yaml`: Validates project visibility in main cluster

## Running the Test

```bash
# Run the project-ready test specifically
task test:end-to-end -- project-ready

# Or run with chainsaw directly
chainsaw test test/project-ready/ --test-timeout=120s
```

## Expected Results

✅ **Success Criteria:**
- Project is created successfully through organizational context
- Project reaches `Ready` status within 60 seconds
- No TLS certificate errors in controller logs
- Project is visible and accessible from both organizational and main cluster contexts
- Clean deletion of test resources

❌ **Failure Scenarios:**
- Project creation fails (admission webhook issues)
- Project never reaches Ready status (controller issues, TLS problems)
- Timeout waiting for Ready condition
- Cross-cluster visibility issues
