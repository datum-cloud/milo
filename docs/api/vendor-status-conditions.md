# Vendor Status Conditions

This document explains the status conditions pattern used in the Vendor resource to communicate vendor state and verification status.

## Overview

The Vendor resource uses Kubernetes status conditions to provide detailed information about the vendor's current state, verification status, and readiness. This approach provides better observability and follows Kubernetes best practices.

## Status Structure

### Vendor Status Fields

The `VendorStatus` includes both high-level status fields and detailed conditions:

```yaml
status:
  # High-level status
  status: active                    # Overall vendor status
  verificationStatus: approved      # Overall verification status
  
  # Verification counts
  requiredVerifications: 2
  completedVerifications: 2
  pendingVerifications: 0
  rejectedVerifications: 0
  expiredVerifications: 0
  
  # Timestamps
  lastVerifiedAt: "2024-01-15T14:30:00Z"
  activatedAt: "2024-01-15T15:00:00Z"
  rejectedAt: null
  rejectionReason: ""
  
  # Detailed conditions
  conditions:
  - type: "Ready"
    status: "True"
    reason: "VendorActive"
    message: "Vendor is active and verified"
    lastTransitionTime: "2024-01-15T15:00:00Z"
  - type: "Validated"
    status: "True"
    reason: "ValidationPassed"
    message: "All required fields validated"
    lastTransitionTime: "2024-01-15T10:00:00Z"
  - type: "Verified"
    status: "True"
    reason: "VerificationComplete"
    message: "All required verifications completed"
    lastTransitionTime: "2024-01-15T14:30:00Z"
  - type: "Active"
    status: "True"
    reason: "Activated"
    message: "Vendor is active and ready for business"
    lastTransitionTime: "2024-01-15T15:00:00Z"
```

## Condition Types

### 1. Ready
**Purpose**: Overall readiness of the vendor
**Status**: True/False/Unknown
**Reasons**:
- `VendorActive`: Vendor is active and ready for business
- `VendorPending`: Vendor is pending verification or activation
- `VendorRejected`: Vendor has been rejected
- `VendorArchived`: Vendor has been archived

### 2. Validated
**Purpose**: Whether vendor data validation has passed
**Status**: True/False/Unknown
**Reasons**:
- `ValidationPassed`: All required fields validated successfully
- `ValidationFailed`: Validation failed due to missing or invalid data

### 3. Verified
**Purpose**: Whether all required verifications are completed
**Status**: True/False/Unknown
**Reasons**:
- `VerificationComplete`: All required verifications completed
- `VerificationInProgress`: Verification process in progress
- `VerificationFailed`: Verification process failed

### 4. Active
**Purpose**: Whether vendor is active and can conduct business
**Status**: True/False/Unknown
**Reasons**:
- `Activated`: Vendor is active and ready for business
- `NotActivated`: Vendor is not yet activated

## Status Values

### Vendor Status
- `pending`: Vendor is pending verification or activation
- `active`: Vendor is active and ready for business
- `rejected`: Vendor has been rejected
- `archived`: Vendor has been archived

### Verification Status
- `pending`: No verifications started
- `in-progress`: Verifications in progress
- `approved`: All required verifications approved
- `rejected`: Some verifications rejected
- `expired`: Some verifications expired

## API Functions

### Status Management

```go
// Set vendor status and update conditions
SetVendorStatus(vendor, VendorStatusActive, ReasonVendorActive, "Vendor activated")

// Set validation status
SetValidationStatus(vendor, true, ReasonValidationPassed, "All fields validated")

// Set verification status
SetVerificationStatus(vendor, VerificationStatusApproved, ReasonVerificationComplete, "All verifications complete")

// Set active status
SetActiveStatus(vendor, true, ReasonActivated, "Vendor activated")

// Set individual condition
SetCondition(vendor, ConditionTypeReady, metav1.ConditionTrue, ReasonVendorActive, "Vendor is ready")
```

### Status Queries

```go
// Check condition status
isReady := IsConditionTrue(vendor, ConditionTypeReady)
isValidated := IsConditionTrue(vendor, ConditionTypeValidated)
isVerified := IsConditionTrue(vendor, ConditionTypeVerified)
isActive := IsConditionTrue(vendor, ConditionTypeActive)

// Get specific condition
condition := GetCondition(vendor, ConditionTypeReady)

// Check if vendor can be activated
canActivate, reason := CanActivateVendor(vendor)

// Get status summary
summary := GetVendorStatusSummary(vendor)
```

### Verification Integration

```go
// Update vendor status from verifications
err := UpdateVendorStatusFromVerifications(ctx, client, vendor)

// Update verification counts
UpdateVerificationCounts(vendor, verifications)
```

## Usage Examples

### Creating a New Vendor

```go
vendor := &Vendor{
    Spec: VendorSpec{
        ProfileType: VendorProfileTypeBusiness,
        LegalName: "ACME Corp",
        // ... other fields
    },
}

// Set initial status
SetVendorStatus(vendor, VendorStatusPending, ReasonVendorPending, "Vendor created, pending verification")
SetValidationStatus(vendor, true, ReasonValidationPassed, "Initial validation passed")
SetVerificationStatus(vendor, VerificationStatusPending, ReasonVerificationInProgress, "Verification pending")
SetActiveStatus(vendor, false, ReasonNotActivated, "Not yet activated")
```

### Updating Verification Status

```go
// When verifications are completed
SetVerificationStatus(vendor, VerificationStatusApproved, ReasonVerificationComplete, "All verifications approved")

// Update active status
SetActiveStatus(vendor, true, ReasonActivated, "Vendor activated after verification")

// Update overall status
SetVendorStatus(vendor, VendorStatusActive, ReasonVendorActive, "Vendor is active and ready")
```

### Handling Rejection

```go
// When vendor is rejected
SetVendorStatus(vendor, VendorStatusRejected, ReasonVendorRejected, "Vendor rejected due to failed verification")
vendor.Status.RejectionReason = "Tax verification failed"
vendor.Status.RejectedAt = &metav1.Time{Time: time.Now()}
```

## Monitoring and Alerting

### Key Metrics to Monitor

1. **Vendor Status Distribution**
   - Count of vendors by status (pending, active, rejected, archived)
   - Count of vendors by verification status

2. **Verification Metrics**
   - Average time to complete verification
   - Verification success rate
   - Number of expired verifications

3. **Condition Health**
   - Count of vendors with each condition type
   - Condition transition frequency

### Alerting Rules

```yaml
# Alert on high rejection rate
- alert: HighVendorRejectionRate
  expr: rate(vendor_rejections_total[5m]) > 0.1
  for: 5m
  labels:
    severity: warning
  annotations:
    summary: "High vendor rejection rate detected"

# Alert on expired verifications
- alert: ExpiredVerifications
  expr: vendor_expired_verifications > 0
  for: 0m
  labels:
    severity: warning
  annotations:
    summary: "Vendors with expired verifications detected"

# Alert on stuck pending vendors
- alert: StuckPendingVendors
  expr: time() - vendor_created_timestamp > 86400 and vendor_status == "pending"
  for: 1h
  labels:
    severity: warning
  annotations:
    summary: "Vendors stuck in pending status for over 24 hours"
```

## Best Practices

### 1. Condition Management
- Always set conditions with appropriate reasons and messages
- Use consistent reason codes across the system
- Include timestamps for all status changes
- Keep condition messages descriptive and actionable

### 2. Status Updates
- Update verification counts when verifications change
- Set timestamps for significant status changes
- Maintain consistency between high-level status and conditions
- Use atomic updates to prevent race conditions

### 3. Monitoring
- Monitor condition transition frequencies
- Set up alerts for stuck or failed states
- Track verification completion times
- Monitor rejection rates and reasons

### 4. API Design
- Provide clear status summary functions
- Include validation functions for status changes
- Use consistent naming conventions
- Document all condition types and reasons

## Migration from Simple Status

If migrating from a simple status field:

1. **Add status conditions** to existing vendors
2. **Update controllers** to set conditions appropriately
3. **Modify applications** to read from conditions
4. **Add monitoring** for condition health
5. **Test status transitions** end-to-end

## Integration with Controllers

Controllers should update vendor status based on their specific concerns:

```go
func (r *VendorValidationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    // ... validation logic ...
    
    if validationPassed {
        SetValidationStatus(vendor, true, ReasonValidationPassed, "Validation completed")
    } else {
        SetValidationStatus(vendor, false, ReasonValidationFailed, "Validation failed: " + reason)
    }
    
    // Update overall status
    UpdateVendorStatusFromVerifications(ctx, r.Client, vendor)
    
    return ctrl.Result{}, nil
}
```

This status conditions approach provides much better observability and follows Kubernetes best practices for communicating resource state!
