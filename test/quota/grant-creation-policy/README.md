# Grant Creation Policy End-to-End Test

This test scenario comprehensively exercises the GrantCreationPolicy functionality, including:

## Test Coverage

### Core Functionality
- **Policy Validation**: Tests that valid policies are accepted and invalid ones are rejected
- **Automatic Grant Creation**: Verifies grants are automatically created when trigger conditions are met
- **Condition Evaluation**: Tests CEL expression evaluation with different organization attributes
- **Template Rendering**: Verifies Go template rendering with different context values
- **Owner References**: Ensures proper cleanup when trigger resources are deleted

### Advanced Features
- **CEL Name Expressions**: Tests dynamic name generation using CEL expressions
- **Conditional Logic**: Template conditions based on trigger resource attributes
- **Policy Priority**: Multiple policies with different priorities
- **Event Type Filtering**: Create and update event processing

## Test Scenarios

1. **setup-base-infrastructure**: Creates ResourceRegistration for test quota
2. **setup-grant-creation-policy**: Creates and validates the main GrantCreationPolicy
3. **setup-test-organization**: Creates Organization that will trigger grant creation
4. **verify-automatic-grant-creation**: Confirms grant was created automatically with correct owner references
5. **test-condition-evaluation**: Updates organization type and verifies grant is updated accordingly
6. **test-template-rendering**: Tests template with different organization attributes
7. **test-cel-name-expression**: Tests CEL-based dynamic name generation
8. **test-grant-cleanup**: Verifies grants are cleaned up when trigger resources are deleted
9. **test-policy-validation**: Confirms invalid policies are rejected

## Key Test Resources

- **Organizations**: Different types (Standard, Business) with various attributes
- **Grants**: Auto-generated with proper owner references and dimension selectors
- **Policies**: Multiple policies with different conditions and templates

## Usage

Run the test with:
```bash
task test:end-to-end -- test/quota/grant-creation-policy
```

Or directly with chainsaw:
```bash
KUBECONFIG=.milo/kubeconfig chainsaw test test/quota/grant-creation-policy --test-file chainsaw-test.yaml
```