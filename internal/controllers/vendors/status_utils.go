package vendors

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	vendorsv1alpha1 "go.miloapis.com/milo/pkg/apis/vendors/v1alpha1"
)

// Condition types for Vendor status
const (
	ConditionTypeReady     = "Ready"
	ConditionTypeValidated = "Validated"
	ConditionTypeVerified  = "Verified"
	ConditionTypeActive    = "Active"
)

// Condition reasons
const (
	ReasonVendorActive           = "VendorActive"
	ReasonVendorPending          = "VendorPending"
	ReasonVendorRejected         = "VendorRejected"
	ReasonVendorArchived         = "VendorArchived"
	ReasonValidationPassed       = "ValidationPassed"
	ReasonValidationFailed       = "ValidationFailed"
	ReasonVerificationComplete   = "VerificationComplete"
	ReasonVerificationInProgress = "VerificationInProgress"
	ReasonVerificationFailed     = "VerificationFailed"
	ReasonActivated              = "Activated"
	ReasonNotActivated           = "NotActivated"
)

// SetVendorStatus sets the vendor status and updates related conditions
func SetVendorStatus(vendor *vendorsv1alpha1.Vendor, status vendorsv1alpha1.VendorStatusValue, reason, message string) {
	vendor.Status.Status = status
	now := metav1.Now()

	// Update Ready condition based on status
	readyStatus := metav1.ConditionFalse
	readyReason := reason
	readyMessage := message

	switch status {
	case vendorsv1alpha1.VendorStatusActive:
		readyStatus = metav1.ConditionTrue
		readyReason = ReasonVendorActive
		readyMessage = "Vendor is active and ready for business"
		vendor.Status.ActivatedAt = &now
	case vendorsv1alpha1.VendorStatusRejected:
		readyStatus = metav1.ConditionFalse
		readyReason = ReasonVendorRejected
		readyMessage = "Vendor has been rejected"
		vendor.Status.RejectedAt = &now
	case vendorsv1alpha1.VendorStatusArchived:
		readyStatus = metav1.ConditionFalse
		readyReason = ReasonVendorArchived
		readyMessage = "Vendor has been archived"
	case vendorsv1alpha1.VendorStatusPending:
		readyStatus = metav1.ConditionFalse
		readyReason = ReasonVendorPending
		readyMessage = "Vendor is pending verification or activation"
	}

	SetCondition(vendor, ConditionTypeReady, readyStatus, readyReason, readyMessage)
}

// SetValidationStatus sets the validation condition
func SetValidationStatus(vendor *vendorsv1alpha1.Vendor, passed bool, reason, message string) {
	status := metav1.ConditionFalse
	if passed {
		status = metav1.ConditionTrue
	}
	SetCondition(vendor, ConditionTypeValidated, status, reason, message)
}

// SetVerificationStatus sets the verification condition and updates verification status
func SetVerificationStatus(vendor *vendorsv1alpha1.Vendor, verificationStatus vendorsv1alpha1.VerificationStatus, reason, message string) {
	vendor.Status.VerificationStatus = verificationStatus

	status := metav1.ConditionFalse
	switch verificationStatus {
	case vendorsv1alpha1.VerificationStatusApproved:
		status = metav1.ConditionTrue
		vendor.Status.LastVerifiedAt = &metav1.Time{Time: time.Now()}
	case vendorsv1alpha1.VerificationStatusRejected:
		status = metav1.ConditionFalse
	case vendorsv1alpha1.VerificationStatusInProgress:
		status = metav1.ConditionFalse
	case vendorsv1alpha1.VerificationStatusExpired:
		status = metav1.ConditionFalse
	case vendorsv1alpha1.VerificationStatusPending:
		status = metav1.ConditionFalse
	}

	SetCondition(vendor, ConditionTypeVerified, status, reason, message)
}

// SetActiveStatus sets the active condition
func SetActiveStatus(vendor *vendorsv1alpha1.Vendor, active bool, reason, message string) {
	status := metav1.ConditionFalse
	if active {
		status = metav1.ConditionTrue
	}
	SetCondition(vendor, ConditionTypeActive, status, reason, message)
}

// SetCondition sets a condition on the vendor status
func SetCondition(vendor *vendorsv1alpha1.Vendor, conditionType string, status metav1.ConditionStatus, reason, message string) {
	now := metav1.Now()
	condition := metav1.Condition{
		Type:               conditionType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: now,
	}

	// Find existing condition and update or add new one
	for i, existing := range vendor.Status.Conditions {
		if existing.Type == conditionType {
			if existing.Status != status || existing.Reason != reason {
				condition.LastTransitionTime = now
			} else {
				condition.LastTransitionTime = existing.LastTransitionTime
			}
			vendor.Status.Conditions[i] = condition
			return
		}
	}

	// Add new condition
	vendor.Status.Conditions = append(vendor.Status.Conditions, condition)
}

// GetCondition returns the condition with the given type
func GetCondition(vendor *vendorsv1alpha1.Vendor, conditionType string) *metav1.Condition {
	for i := range vendor.Status.Conditions {
		if vendor.Status.Conditions[i].Type == conditionType {
			return &vendor.Status.Conditions[i]
		}
	}
	return nil
}

// IsConditionTrue returns true if the condition with the given type is True
func IsConditionTrue(vendor *vendorsv1alpha1.Vendor, conditionType string) bool {
	condition := GetCondition(vendor, conditionType)
	return condition != nil && condition.Status == metav1.ConditionTrue
}

// IsConditionFalse returns true if the condition with the given type is False
func IsConditionFalse(vendor *vendorsv1alpha1.Vendor, conditionType string) bool {
	condition := GetCondition(vendor, conditionType)
	return condition != nil && condition.Status == metav1.ConditionFalse
}

// UpdateVerificationCounts updates the verification counts in vendor status
func UpdateVerificationCounts(vendor *vendorsv1alpha1.Vendor, verifications []vendorsv1alpha1.VendorVerification) {
	vendor.Status.RequiredVerifications = 0
	vendor.Status.CompletedVerifications = 0
	vendor.Status.PendingVerifications = 0
	vendor.Status.RejectedVerifications = 0
	vendor.Status.ExpiredVerifications = 0

	for _, verification := range verifications {
		if verification.Spec.Required {
			vendor.Status.RequiredVerifications++
		}

		switch verification.Spec.Status {
		case vendorsv1alpha1.VerificationStatusApproved:
			vendor.Status.CompletedVerifications++
		case vendorsv1alpha1.VerificationStatusPending:
			vendor.Status.PendingVerifications++
		case vendorsv1alpha1.VerificationStatusRejected:
			vendor.Status.RejectedVerifications++
		}

		if IsVerificationExpired(&verification) {
			vendor.Status.ExpiredVerifications++
		}
	}
}

// UpdateVendorStatusFromVerifications updates vendor status based on verification status
func UpdateVendorStatusFromVerifications(ctx context.Context, c client.Client, vendor *vendorsv1alpha1.Vendor) error {
	verifications, err := GetVerificationsForVendor(ctx, c, vendor)
	if err != nil {
		return fmt.Errorf("failed to get verifications: %w", err)
	}

	// Update verification counts
	UpdateVerificationCounts(vendor, verifications)

	// Determine overall verification status
	verificationStatus, err := GetVerificationStatus(ctx, c, vendor)
	if err != nil {
		return fmt.Errorf("failed to get verification status: %w", err)
	}

	// Update verification condition
	SetVerificationStatus(vendor, verificationStatus, ReasonVerificationInProgress, "Verification status updated")

	// Update active condition based on verification status
	active := verificationStatus == vendorsv1alpha1.VerificationStatusApproved
	SetActiveStatus(vendor, active, ReasonActivated, "Vendor activation status updated")

	// Update overall vendor status
	if active && vendor.Status.Status != vendorsv1alpha1.VendorStatusActive {
		SetVendorStatus(vendor, vendorsv1alpha1.VendorStatusActive, ReasonVendorActive, "Vendor activated after verification completion")
	} else if !active && vendor.Status.Status == vendorsv1alpha1.VendorStatusActive {
		SetVendorStatus(vendor, vendorsv1alpha1.VendorStatusPending, ReasonVendorPending, "Vendor deactivated due to verification issues")
	}

	return nil
}

// CanActivateVendor checks if a vendor can be activated
func CanActivateVendor(vendor *vendorsv1alpha1.Vendor) (bool, string) {
	// Check if all required verifications are completed
	if vendor.Status.RequiredVerifications > 0 && vendor.Status.CompletedVerifications < vendor.Status.RequiredVerifications {
		return false, fmt.Sprintf("Required verifications incomplete: %d/%d completed",
			vendor.Status.CompletedVerifications, vendor.Status.RequiredVerifications)
	}

	// Check if there are any rejected verifications
	if vendor.Status.RejectedVerifications > 0 {
		return false, fmt.Sprintf("Vendor has %d rejected verifications", vendor.Status.RejectedVerifications)
	}

	// Check if there are any expired verifications
	if vendor.Status.ExpiredVerifications > 0 {
		return false, fmt.Sprintf("Vendor has %d expired verifications", vendor.Status.ExpiredVerifications)
	}

	// Check if validation passed
	if !IsConditionTrue(vendor, ConditionTypeValidated) {
		return false, "Vendor validation not passed"
	}

	return true, "Vendor can be activated"
}

// GetVendorStatusSummary returns a summary of the vendor status
func GetVendorStatusSummary(vendor *vendorsv1alpha1.Vendor) *VendorStatusSummary {
	return &VendorStatusSummary{
		Status:                 vendor.Status.Status,
		VerificationStatus:     vendor.Status.VerificationStatus,
		RequiredVerifications:  vendor.Status.RequiredVerifications,
		CompletedVerifications: vendor.Status.CompletedVerifications,
		PendingVerifications:   vendor.Status.PendingVerifications,
		RejectedVerifications:  vendor.Status.RejectedVerifications,
		ExpiredVerifications:   vendor.Status.ExpiredVerifications,
		IsReady:                IsConditionTrue(vendor, ConditionTypeReady),
		IsValidated:            IsConditionTrue(vendor, ConditionTypeValidated),
		IsVerified:             IsConditionTrue(vendor, ConditionTypeVerified),
		IsActive:               IsConditionTrue(vendor, ConditionTypeActive),
		LastVerifiedAt:         vendor.Status.LastVerifiedAt,
		ActivatedAt:            vendor.Status.ActivatedAt,
		RejectedAt:             vendor.Status.RejectedAt,
		RejectionReason:        vendor.Status.RejectionReason,
	}
}

// VendorStatusSummary provides a summary of vendor status
type VendorStatusSummary struct {
	Status                 vendorsv1alpha1.VendorStatusValue  `json:"status"`
	VerificationStatus     vendorsv1alpha1.VerificationStatus `json:"verificationStatus"`
	RequiredVerifications  int32                              `json:"requiredVerifications"`
	CompletedVerifications int32                              `json:"completedVerifications"`
	PendingVerifications   int32                              `json:"pendingVerifications"`
	RejectedVerifications  int32                              `json:"rejectedVerifications"`
	ExpiredVerifications   int32                              `json:"expiredVerifications"`
	IsReady                bool                               `json:"isReady"`
	IsValidated            bool                               `json:"isValidated"`
	IsVerified             bool                               `json:"isVerified"`
	IsActive               bool                               `json:"isActive"`
	LastVerifiedAt         *metav1.Time                       `json:"lastVerifiedAt,omitempty"`
	ActivatedAt            *metav1.Time                       `json:"activatedAt,omitempty"`
	RejectedAt             *metav1.Time                       `json:"rejectedAt,omitempty"`
	RejectionReason        string                             `json:"rejectionReason,omitempty"`
}
