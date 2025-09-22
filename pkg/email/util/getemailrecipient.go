package util

import (
	"fmt"
	notificationv1alpha1 "go.miloapis.com/milo/pkg/apis/notification/v1alpha1"
)

func GetEmailRecipient(email *notificationv1alpha1.Email) (notificationv1alpha1.EmailRecipient, error) {
	if HasUserRef(email) && HasEmailAddress(email) {
		return notificationv1alpha1.EmailRecipient{}, fmt.Errorf("email must have either a userRef or emailAddress")
	}

	if HasUserRef(email) {
		return notificationv1alpha1.EmailRecipient{
			UserRef: email.Spec.Recipient.UserRef,
		}, nil
	}

	if HasEmailAddress(email) {
		return notificationv1alpha1.EmailRecipient{
			EmailAddress: email.Spec.Recipient.EmailAddress,
		}, nil
	}

	return notificationv1alpha1.EmailRecipient{}, fmt.Errorf("no recipient found")
}

// HasUserRef checks if the Email has a user reference as recipient
func HasUserRef(email *notificationv1alpha1.Email) bool {
	return email.Spec.Recipient.UserRef.Name != ""
}

// HasEmailAddress checks if the Email has an email address as recipient
func HasEmailAddress(email *notificationv1alpha1.Email) bool {
	return email.Spec.Recipient.EmailAddress != ""
}
