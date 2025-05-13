package resend

import (
	"fmt"
	"log/slog"

	"go.datum.net/iam/internal/providers/email"

	"github.com/resend/resend-go/v2"
)

// ResendProvider implements the EmailProvider interface using Resend.
type Resend struct {
	client  *resend.Client
	from    string // The 'from' email address
	replyTo string // Optional: The 'reply-to' email address
}

// NewResendProvider creates a new ResendProvider.
// The 'from' address is required. 'replyTo' is optional.
func NewResendProvider(apiKey, fromAddress string, replyToAddress ...string) *Resend {
	client := resend.NewClient(apiKey)
	provider := &Resend{
		client: client,
		from:   fromAddress,
	}
	if len(replyToAddress) > 0 && replyToAddress[0] != "" {
		provider.replyTo = replyToAddress[0]
	}
	return provider
}

// SendEmail sends an email using the Resend provider.
// It uses the HTML body for the email content.
func (r *Resend) SendEmail(params *email.SendEmailParams) error {
	opts := &resend.SendEmailRequest{
		From:    r.from,
		To:      params.To,
		Cc:      params.Cc,
		Subject: params.Subject,
		Html:    params.HTMLBody,
	}

	if r.replyTo != "" {
		opts.ReplyTo = r.replyTo
	}

	sent, err := r.client.Emails.Send(opts)
	if err != nil {
		slog.Error("failed to send email via Resend", "error", err)
		return fmt.Errorf("failed to send email via Resend: %w", err)
	}

	slog.Info("email sent successfully", "id", sent.Id)
	return nil
}
