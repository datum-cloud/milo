package v1alpha1

import (
	"context"
	"fmt"

	iamv1alpha1 "go.miloapis.com/milo/pkg/apis/iam/v1alpha1"
	notificationv1alpha1 "go.miloapis.com/milo/pkg/apis/notification/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var ownershipLog = logf.Log.WithName("contact-ownership-validator")

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
	ownershipLog.Info("Starting contact ownership validation",
		"resource", resource.String(),
		"resourceName", resourceName,
		"operation", operation,
		"contactName", contact.Name,
		"contactNamespace", contact.Namespace)

	// Only enforce ownership validation if the request was made through a user context
	// (i.e., via /apis/iam.miloapis.com/v1alpha1/users/{user}/control-plane)
	req, err := admission.RequestFromContext(ctx)
	if err != nil {
		// No admission request in context, skip validation
		ownershipLog.Info("No admission request in context, skipping ownership validation",
			"resource", resource.String(),
			"resourceName", resourceName)
		return nil
	}

	_, hasUserContext := req.UserInfo.Extra[iamv1alpha1.ParentNameExtraKey]
	if !hasUserContext {
		// Not in user context, skip validation
		ownershipLog.Info("Not in user context, skipping ownership validation",
			"resource", resource.String(),
			"resourceName", resourceName,
			"userUID", req.UserInfo.UID)
		return nil
	}

	ownershipLog.Info("User context detected, validating ownership",
		"resource", resource.String(),
		"resourceName", resourceName,
		"userUID", req.UserInfo.UID)

	// Validate that the user can only perform operations for their own contact
	// The Contact's subject.name should match the authenticated user's UID
	if contact.Spec.SubjectRef == nil {
		ownershipLog.Info("Ownership validation failed: contact has no subject reference in user context",
			"resource", resource.String(),
			"resourceName", resourceName,
			"operation", operation,
			"userUID", req.UserInfo.UID,
			"contactName", contact.Name)
		return errors.NewForbidden(
			resource,
			resourceName,
			fmt.Errorf("you do not have permission to %s for this contact", operation),
		)
	}

	if contact.Spec.SubjectRef.Name != string(req.UserInfo.UID) {
		ownershipLog.Info("Ownership validation failed: user does not own the contact",
			"resource", resource.String(),
			"resourceName", resourceName,
			"operation", operation,
			"userUID", req.UserInfo.UID,
			"contactSubjectName", contact.Spec.SubjectRef.Name)
		return errors.NewForbidden(
			resource,
			resourceName,
			fmt.Errorf("you do not have permission to %s for this contact", operation),
		)
	}

	ownershipLog.V(1).Info("Ownership validation succeeded",
		"resource", resource.String(),
		"resourceName", resourceName,
		"operation", operation,
		"userUID", req.UserInfo.UID)

	return nil
}
