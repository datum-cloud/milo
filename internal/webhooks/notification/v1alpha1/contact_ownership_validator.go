package v1alpha1

import (
	"context"
	"fmt"

	iamv1alpha1 "go.miloapis.com/milo/pkg/apis/iam/v1alpha1"
	notificationv1alpha1 "go.miloapis.com/milo/pkg/apis/notification/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// ValidateContactOwnership checks if the authenticated user (when in user context) owns the contact.
// This validation only applies when the request is made through a user context
// (i.e., via /apis/iam.miloapis.com/v1alpha1/users/{user}/control-plane).
//
// Parameters:
//   - ctx: the request context
//   - contact: the Contact resource to validate ownership for
//   - resource: the GroupResource for the error message (e.g., "contactgroupmemberships")
//   - resourceName: the name of the resource being validated
//   - operation: description of the operation being performed (e.g., "create membership", "delete membership removal")
//
// Returns:
//   - A Forbidden error if the ownership check fails
//   - nil if the ownership is valid or no user context is present
func ValidateContactOwnership(ctx context.Context, contact *notificationv1alpha1.Contact, resource schema.GroupResource, resourceName string, operation string) error {
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
		return errors.NewForbidden(
			resource,
			resourceName,
			fmt.Errorf("you do not have permission to %s for this contact", operation),
		)
	}

	return nil
}
