package v1alpha1

import (
	"context"
	"fmt"

	iamv1alpha1 "go.miloapis.com/milo/pkg/apis/iam/v1alpha1"
	notificationv1alpha1 "go.miloapis.com/milo/pkg/apis/notification/v1alpha1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// ValidateContactOwnership checks if the authenticated user (when in user context) owns the contact.
// This validation only applies when the request is made through a user context
// (i.e., via /apis/iam.miloapis.com/v1alpha1/users/{user}/control-plane).
//
// Parameters:
//   - ctx: the request context
//   - contact: the Contact resource to validate ownership for
//   - fieldPath: the field path to use in the error message (e.g., "spec", "contactRef")
//   - operation: description of the operation being performed (e.g., "create membership", "delete membership removal")
//
// Returns:
//   - A field.Error if the ownership check fails
//   - nil if the ownership is valid or no user context is present
func ValidateContactOwnership(ctx context.Context, contact *notificationv1alpha1.Contact, fieldPath *field.Path, operation string) *field.Error {
	// Only enforce ownership validation if the request was made through a user context
	// (i.e., via /apis/iam.miloapis.com/v1alpha1/users/{user}/control-plane)
	req, err := admission.RequestFromContext(ctx)
	if err != nil {
		// No admission request in context, skip validation
		return nil
	}

	_, hasUserContext := req.UserInfo.Extra[iamv1alpha1.ParentNameExtraKey]
	if !hasUserContext {
		// Not in user context, skip validation
		return nil
	}

	// Validate that the user can only perform operations for their own contact
	// The Contact's subject.name should match the authenticated user's UID
	if contact.Spec.SubjectRef == nil {
		return nil
	}

	if contact.Spec.SubjectRef.Name != string(req.UserInfo.UID) {
		return field.Forbidden(
			fieldPath,
			fmt.Sprintf("cannot %s for contact '%s' owned by user '%s': you can only %ss for your own contacts",
				operation, contact.Name, contact.Spec.SubjectRef.Name, operation))
	}

	return nil
}
