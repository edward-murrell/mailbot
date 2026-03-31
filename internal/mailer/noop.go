package mailer

import (
	"context"
	"log/slog"

	"github.com/ekm/mailbot/internal/submission"
)

// NoopMailer logs instead of sending email.
// Used when SMTP_ENABLED=false to allow local development without an SMTP server.
type NoopMailer struct {
	logger *slog.Logger
}

// NewNoopMailer constructs a NoopMailer.
func NewNoopMailer(logger *slog.Logger) *NoopMailer {
	return &NoopMailer{logger: logger}
}

func (m *NoopMailer) Send(ctx context.Context, s submission.Submission) error {
	m.logger.InfoContext(ctx, "smtp disabled: skipping send",
		"name", s.Name,
		"email", s.Email,
		"subject", s.Subject,
	)
	return nil
}
