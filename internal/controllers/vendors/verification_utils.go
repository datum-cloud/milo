package vendors

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	vendorsv1alpha1 "go.miloapis.com/milo/pkg/apis/vendors/v1alpha1"
)

// GetVerificationsForVendor retrieves all verifications for a specific vendor
func GetVerificationsForVendor(ctx context.Context, c client.Client, vendor *vendorsv1alpha1.Vendor) ([]vendorsv1alpha1.VendorVerification, error) {
	var verificationList vendorsv1alpha1.VendorVerificationList

	// Determine the namespace for the search
	namespace := vendor.Namespace
	if namespace == "" {
		namespace = "default"
	}

	// List all verifications
	if err := c.List(ctx, &verificationList); err != nil {
		return nil, fmt.Errorf("failed to list verifications: %w", err)
	}

	// Filter verifications for this vendor
	var vendorVerifications []vendorsv1alpha1.VendorVerification
	for _, verification := range verificationList.Items {
		if verification.Spec.VendorRef.Name == vendor.Name {
			// Check namespace match (if specified in verification)
			if verification.Spec.VendorRef.Namespace == "" || verification.Spec.VendorRef.Namespace == namespace {
				vendorVerifications = append(vendorVerifications, verification)
			}
		}
	}

	return vendorVerifications, nil
}

// GetVerificationByType retrieves a specific verification type for a vendor
func GetVerificationByType(ctx context.Context, c client.Client, vendor *vendorsv1alpha1.Vendor, verificationType vendorsv1alpha1.VerificationType) (*vendorsv1alpha1.VendorVerification, error) {
	verifications, err := GetVerificationsForVendor(ctx, c, vendor)
	if err != nil {
		return nil, err
	}

	for _, verification := range verifications {
		if verification.Spec.VerificationType == verificationType {
			return &verification, nil
		}
	}

	return nil, fmt.Errorf("verification of type %s not found for vendor %s", verificationType, vendor.Name)
}

// IsVendorVerified checks if a vendor has all required verifications approved
func IsVendorVerified(ctx context.Context, c client.Client, vendor *vendorsv1alpha1.Vendor) (bool, []string, error) {
	verifications, err := GetVerificationsForVendor(ctx, c, vendor)
	if err != nil {
		return false, nil, err
	}

	var missingVerifications []string
	allVerified := true

	for _, verification := range verifications {
		if verification.Spec.Required {
			if verification.Spec.Status != vendorsv1alpha1.VerificationStatusApproved {
				allVerified = false
				missingVerifications = append(missingVerifications, string(verification.Spec.VerificationType))
			}
		}
	}

	return allVerified, missingVerifications, nil
}

// GetVerificationStatus returns the overall verification status for a vendor
func GetVerificationStatus(ctx context.Context, c client.Client, vendor *vendorsv1alpha1.Vendor) (vendorsv1alpha1.VerificationStatus, error) {
	verifications, err := GetVerificationsForVendor(ctx, c, vendor)
	if err != nil {
		return vendorsv1alpha1.VerificationStatusPending, err
	}

	if len(verifications) == 0 {
		return vendorsv1alpha1.VerificationStatusPending, nil
	}

	// Check if any verification is in progress
	for _, verification := range verifications {
		if verification.Spec.Status == vendorsv1alpha1.VerificationStatusInProgress {
			return vendorsv1alpha1.VerificationStatusInProgress, nil
		}
	}

	// Check if any required verification is rejected
	for _, verification := range verifications {
		if verification.Spec.Required && verification.Spec.Status == vendorsv1alpha1.VerificationStatusRejected {
			return vendorsv1alpha1.VerificationStatusRejected, nil
		}
	}

	// Check if all required verifications are approved
	allApproved := true
	for _, verification := range verifications {
		if verification.Spec.Required && verification.Spec.Status != vendorsv1alpha1.VerificationStatusApproved {
			allApproved = false
			break
		}
	}

	if allApproved {
		return vendorsv1alpha1.VerificationStatusApproved, nil
	}

	return vendorsv1alpha1.VerificationStatusPending, nil
}

// IsVerificationExpired checks if a verification has expired
func IsVerificationExpired(verification *vendorsv1alpha1.VendorVerification) bool {
	if verification.Spec.ExpirationDate == nil {
		return false
	}
	return verification.Spec.ExpirationDate.Time.Before(time.Now())
}

// GetExpiredVerifications returns all expired verifications for a vendor
func GetExpiredVerifications(ctx context.Context, c client.Client, vendor *vendorsv1alpha1.Vendor) ([]vendorsv1alpha1.VendorVerification, error) {
	verifications, err := GetVerificationsForVendor(ctx, c, vendor)
	if err != nil {
		return nil, err
	}

	var expiredVerifications []vendorsv1alpha1.VendorVerification
	for _, verification := range verifications {
		if IsVerificationExpired(&verification) {
			expiredVerifications = append(expiredVerifications, verification)
		}
	}

	return expiredVerifications, nil
}

// CreateVerification creates a new verification for a vendor
func CreateVerification(ctx context.Context, c client.Client, vendor *vendorsv1alpha1.Vendor, verification *vendorsv1alpha1.VendorVerification) error {
	// Set vendor reference
	verification.Spec.VendorRef = vendorsv1alpha1.VendorReference{
		Name:      vendor.Name,
		Namespace: vendor.Namespace,
	}

	// Set creation timestamp
	now := metav1.Now()
	verification.CreationTimestamp = now

	// Set initial status if not set
	if verification.Spec.Status == "" {
		verification.Spec.Status = vendorsv1alpha1.VerificationStatusPending
	}

	// Set owner reference
	verification.OwnerReferences = []metav1.OwnerReference{
		{
			APIVersion: vendor.APIVersion,
			Kind:       vendor.Kind,
			Name:       vendor.Name,
			UID:        vendor.UID,
			Controller: &[]bool{false}[0], // Not a controller, just a reference
		},
	}

	return c.Create(ctx, verification)
}

// UpdateVerificationStatus updates the status of a verification
func UpdateVerificationStatus(ctx context.Context, c client.Client, verification *vendorsv1alpha1.VendorVerification, status vendorsv1alpha1.VerificationStatus, notes string) error {
	verification.Spec.Status = status
	verification.Spec.Notes = notes

	// Update status timestamps
	now := metav1.Now()
	verification.Status.LastUpdatedAt = &now

	if status == vendorsv1alpha1.VerificationStatusApproved || status == vendorsv1alpha1.VerificationStatusRejected {
		verification.Status.CompletedAt = &now
	}

	return c.Update(ctx, verification)
}

// GetVerificationSummary returns a summary of verifications for a vendor
func GetVerificationSummary(ctx context.Context, c client.Client, vendor *vendorsv1alpha1.Vendor) (*VerificationSummary, error) {
	verifications, err := GetVerificationsForVendor(ctx, c, vendor)
	if err != nil {
		return nil, err
	}

	summary := &VerificationSummary{
		VendorName:            vendor.Name,
		TotalVerifications:    len(verifications),
		RequiredVerifications: 0,
		ApprovedVerifications: 0,
		PendingVerifications:  0,
		RejectedVerifications: 0,
		ExpiredVerifications:  0,
		VerificationTypes:     make(map[vendorsv1alpha1.VerificationType]int),
	}

	for _, verification := range verifications {
		if verification.Spec.Required {
			summary.RequiredVerifications++
		}

		summary.VerificationTypes[verification.Spec.VerificationType]++

		switch verification.Spec.Status {
		case vendorsv1alpha1.VerificationStatusApproved:
			summary.ApprovedVerifications++
		case vendorsv1alpha1.VerificationStatusPending:
			summary.PendingVerifications++
		case vendorsv1alpha1.VerificationStatusRejected:
			summary.RejectedVerifications++
		}

		if IsVerificationExpired(&verification) {
			summary.ExpiredVerifications++
		}
	}

	// Calculate overall status
	overallStatus, err := GetVerificationStatus(ctx, c, vendor)
	if err != nil {
		return nil, err
	}
	summary.OverallStatus = overallStatus

	return summary, nil
}

// VerificationSummary provides a summary of verification status for a vendor
type VerificationSummary struct {
	VendorName            string                                   `json:"vendorName"`
	TotalVerifications    int                                      `json:"totalVerifications"`
	RequiredVerifications int                                      `json:"requiredVerifications"`
	ApprovedVerifications int                                      `json:"approvedVerifications"`
	PendingVerifications  int                                      `json:"pendingVerifications"`
	RejectedVerifications int                                      `json:"rejectedVerifications"`
	ExpiredVerifications  int                                      `json:"expiredVerifications"`
	OverallStatus         vendorsv1alpha1.VerificationStatus       `json:"overallStatus"`
	VerificationTypes     map[vendorsv1alpha1.VerificationType]int `json:"verificationTypes"`
}
