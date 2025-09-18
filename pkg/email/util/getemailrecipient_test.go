package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
	notificationv1alpha1 "go.miloapis.com/milo/pkg/apis/notification/v1alpha1"
)

func TestGetEmailRecipient_UserRef(t *testing.T) {
	email := &notificationv1alpha1.Email{
		Spec: notificationv1alpha1.EmailSpec{
			Recipient: notificationv1alpha1.EmailRecipient{
				UserRef: notificationv1alpha1.EmailUserReference{Name: "john"},
			},
		},
	}

	recipient, err := GetEmailRecipient(email)
	assert.NoError(t, err)
	assert.Equal(t, "john", recipient.UserRef.Name)
	assert.Empty(t, recipient.EmailAddress)
}

func TestGetEmailRecipient_EmailAddress(t *testing.T) {
	email := &notificationv1alpha1.Email{
		Spec: notificationv1alpha1.EmailSpec{
			Recipient: notificationv1alpha1.EmailRecipient{
				EmailAddress: "john@example.com",
			},
		},
	}

	recipient, err := GetEmailRecipient(email)
	assert.NoError(t, err)
	assert.Equal(t, "john@example.com", recipient.EmailAddress)
	assert.Empty(t, recipient.UserRef.Name)
}

func TestGetEmailRecipient_NoRecipient(t *testing.T) {
	email := &notificationv1alpha1.Email{}
	recipient, err := GetEmailRecipient(email)
	assert.Error(t, err)
	assert.Equal(t, notificationv1alpha1.EmailRecipient{}, recipient)
}

func TestGetEmailRecipient_BothUserRefAndEmailAddress(t *testing.T) {
	email := &notificationv1alpha1.Email{
		Spec: notificationv1alpha1.EmailSpec{
			Recipient: notificationv1alpha1.EmailRecipient{
				EmailAddress: "john@example.com",
				UserRef:      notificationv1alpha1.EmailUserReference{Name: "john"},
			},
		},
	}
	recipient, err := GetEmailRecipient(email)
	assert.Error(t, err)
	assert.Equal(t, notificationv1alpha1.EmailRecipient{}, recipient)
}
