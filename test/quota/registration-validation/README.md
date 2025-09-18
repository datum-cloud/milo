# ResourceRegistration Validation Tests

This directory contains comprehensive end-to-end Chainsaw tests for ResourceRegistration validation.

## Test Coverage

### OpenAPI Schema Validation (CRD-level)
- **Required Fields**: Tests that resourceType, baseUnit, displayUnit, unitConversionFactor are required
- **Pattern Validation**: Tests resourceType pattern validation (group/resource format)
- **Enum Validation**: Tests that `type` field only accepts "Entity" or "Allocation"
- **Numeric Constraints**: Tests that unitConversionFactor must be >= 1
- **Array Limits**: Tests maximum of 20 claimingResources

### CEL Validation Rules (CRD-level)
- **Immutability**: Tests that resourceType, consumerTypeRef, and type fields cannot be changed after creation

### Admission Plugin Validation
- **ClaimingResources Duplicates**: Tests that claimingResources array cannot contain duplicate apiGroup/kind combinations
- **Cross-Resource Duplicates**: Tests that two ResourceRegistrations cannot have the same resourceType

## Running the Tests

```bash
# Run all registration validation tests
task test:end-to-end -- --test-dir test/quota/registration-validation

# Run with verbose output for debugging
task test:end-to-end -- --test-dir test/quota/registration-validation --verbose
```

## Test Files

### Test Data
- `valid-registration.yaml` - A fully valid ResourceRegistration
- `missing-required-fields.yaml` - Missing required fields (should fail)
- `invalid-resource-type-pattern.yaml` - Invalid characters in resourceType
- `invalid-type-enum.yaml` - Invalid enum value for type field
- `invalid-conversion-factor.yaml` - Zero or negative unitConversionFactor
- `duplicate-claiming-resources.yaml` - Duplicate entries in claimingResources
- `first-registration.yaml` - First registration with unique resourceType
- `duplicate-resource-type.yaml` - Second registration with same resourceType (should fail)
- `registration-to-update.yaml` - Registration used for immutability tests
- `patch-*.yaml` - Various patch files to test immutable field updates
- `max-claiming-resources.yaml` - Exceeds 20 item limit for claimingResources

### Assertions
- `assert-valid-registration.yaml` - Verifies successful registration creation
- `assert-updated-registration.yaml` - Verifies mutable fields were updated

## Validation Layers

1. **OpenAPI Schema** - Enforced by Kubernetes API server via CRD
2. **CEL Rules** - Evaluated by Kubernetes API server for complex validations
3. **Admission Plugin** - Custom validation for cross-resource constraints
4. **Controller** - Runtime validation and status updates

## Expected Behavior

- Invalid resources should be rejected at API request time with descriptive errors
- Immutable fields should prevent updates with clear error messages
- Duplicate resourceTypes across registrations should be detected and rejected
- All validation errors should provide field-level context for debugging