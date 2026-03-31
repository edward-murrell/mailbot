package mailer

import (
	"context"

	"github.com/ekm/mailbot/internal/submission"
)

// Mailer sends a contact form submission by email.
type Mailer interface {
	Send(ctx context.Context, s submission.Submission) error
}
