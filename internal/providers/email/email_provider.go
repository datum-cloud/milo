package email

// SendEmailParams defines the parameters for sending an email.
type SendEmailParams struct {
	To       []string
	Cc       []string
	Subject  string
	HTMLBody string
}

// Provider defines the interface for sending emails.
type Provider interface {
	SendEmail(params *SendEmailParams) error
}
