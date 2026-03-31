package submission

import (
	"fmt"
	"math/rand/v2"
	"net/mail"
	"strings"
	"time"
)

// Submission is an immutable contact form submission.
// After construction via New, its fields must not be modified.
type Submission struct {
	ReceivedAt time.Time
	Name       string
	Email      string
	Phone      string
	Subject    string
	Body       string
	Reason     string
}

// New constructs a Submission stamped with the given time.
// All string inputs are whitespace-trimmed. Validation is a separate step: call Validate.
func New(name, email, phone, subject, body, reason string, receivedAt time.Time) Submission {
	return Submission{
		ReceivedAt: receivedAt,
		Name:       strings.TrimSpace(name),
		Email:      strings.TrimSpace(email),
		Phone:      strings.TrimSpace(phone),
		Subject:    strings.TrimSpace(subject),
		Body:       strings.TrimSpace(body),
		Reason:     strings.TrimSpace(reason),
	}
}

// ValidationErrors maps field names to human-readable error messages.
// A non-nil, non-empty value means validation failed.
type ValidationErrors map[string]string

func (e ValidationErrors) Error() string {
	parts := make([]string, 0, len(e))
	for k, v := range e {
		parts = append(parts, k+": "+v)
	}
	return strings.Join(parts, "; ")
}

// Validate checks the submission against the contact form rules.
// Returns nil if valid, or a ValidationErrors map if not.
//
// Rules:
//   - At least one of Email or Phone must be non-empty.
//   - If Email is provided, it must be a valid RFC 5322 address.
//   - At least one of Subject, Body, or Reason must be non-empty.
//     Errors are keyed on "email" and "subject" respectively.
func Validate(s Submission) ValidationErrors {
	errs := make(ValidationErrors)

	if s.Email == "" && s.Phone == "" {
		errs["email"] = "email or phone is required"
	} else if s.Email != "" {
		if _, err := mail.ParseAddress(s.Email); err != nil {
			errs["email"] = "invalid email address"
		}
	}

	if s.Subject == "" && s.Body == "" && s.Reason == "" {
		errs["subject"] = "at least one of subject, body, or reason is required"
	}

	if len(errs) == 0 {
		return nil
	}
	return errs
}

// Format returns a plain-text representation of the submission,
// used for both file storage and email body.
func Format(s Submission) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Date:    %s\n", s.ReceivedAt.UTC().Format(time.RFC3339))
	fmt.Fprintf(&sb, "Name:    %s\n", s.Name)
	fmt.Fprintf(&sb, "Email:   %s\n", s.Email)
	fmt.Fprintf(&sb, "Phone:   %s\n", s.Phone)
	fmt.Fprintf(&sb, "Subject: %s\n", s.Subject)
	fmt.Fprintf(&sb, "Reason:  %s\n", s.Reason)
	if s.Body != "" {
		fmt.Fprintf(&sb, "Body:\n%s\n", s.Body)
	}
	return sb.String()
}

const charset = "abcdefghijklmnopqrstuvwxyz0123456789"

// Filename returns a unique storage filename for the submission.
// Format: YYYYMMDD-HHMMSS-<6 random alphanumeric chars>.txt (UTC)
func Filename(s Submission) string {
	b := make([]byte, 6)
	for i := range b {
		b[i] = charset[rand.IntN(len(charset))]
	}
	return fmt.Sprintf("%s-%s.txt", s.ReceivedAt.UTC().Format("20060102-150405"), string(b))
}
