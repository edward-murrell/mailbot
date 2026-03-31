package store

import (
	"context"

	"github.com/ekm/mailbot/internal/submission"
)

// Store persists a contact form submission.
type Store interface {
	Save(ctx context.Context, s submission.Submission) error
}
