package mock

import (
	"log/slog"

	"go.datum.net/iam/internal/providers/email"
)

// Client is a mocked email client.
type Client struct{}

// NewClient creates a new mock email client.
func NewClient() *Client {
	return &Client{}
}

// SendEmail logs the email sending attempt.
func (c *Client) SendEmail(params *email.SendEmailParams) error {
	slog.Info("Mock SendEmail", "to", params.To, "subject", params.Subject, "body", params.HTMLBody, "cc", params.Cc)
	return nil
}

// Ensure Client implements the email.Provider interface.
var _ email.Provider = (*Client)(nil)
