package mailer

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"

	"github.com/ekm/mailbot/internal/submission"
)

// SMTPConfig holds connection parameters for the SMTP mailer.
type SMTPConfig struct {
	Host     string
	Port     string
	User     string
	Pass     string
	From     string
	To       string
	StartTLS bool
}

// SMTPMailer sends email via an authenticated SMTP server.
// It is context-aware: dial honours context deadlines and cancellation.
type SMTPMailer struct {
	cfg SMTPConfig
}

// NewSMTPMailer constructs an SMTPMailer.
func NewSMTPMailer(cfg SMTPConfig) *SMTPMailer {
	return &SMTPMailer{cfg: cfg}
}

func (m *SMTPMailer) Send(ctx context.Context, s submission.Submission) error {
	addr := net.JoinHostPort(m.cfg.Host, m.cfg.Port)

	var (
		conn net.Conn
		err  error
	)

	if m.cfg.StartTLS {
		// Plain TCP connection; STARTTLS upgrade happens after EHLO.
		conn, err = (&net.Dialer{}).DialContext(ctx, "tcp", addr)
	} else {
		// Implicit TLS (e.g. port 465 / SMTPS): TLS before any SMTP traffic.
		conn, err = (&tls.Dialer{
			NetDialer: &net.Dialer{},
			Config:    &tls.Config{ServerName: m.cfg.Host},
		}).DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("smtp dial %s: %w", addr, err)
	}

	// Apply a hard deadline on the underlying connection so the SMTP
	// transaction does not block indefinitely if the context has no deadline.
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(30 * time.Second)
	}
	if err := conn.SetDeadline(deadline); err != nil {
		conn.Close()
		return fmt.Errorf("smtp set deadline: %w", err)
	}

	client, err := smtp.NewClient(conn, m.cfg.Host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("smtp new client: %w", err)
	}
	defer client.Close()

	if m.cfg.StartTLS {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(&tls.Config{ServerName: m.cfg.Host}); err != nil {
				return fmt.Errorf("smtp starttls: %w", err)
			}
		}
	}

	auth := smtp.PlainAuth("", m.cfg.User, m.cfg.Pass, m.cfg.Host)
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("smtp auth: %w", err)
	}
	if err := client.Mail(m.cfg.From); err != nil {
		return fmt.Errorf("smtp mail from: %w", err)
	}
	if err := client.Rcpt(m.cfg.To); err != nil {
		return fmt.Errorf("smtp rcpt to: %w", err)
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}

	if _, err := fmt.Fprint(w, buildMessage(m.cfg.From, m.cfg.To, emailSubject(s), submission.Format(s))); err != nil {
		return fmt.Errorf("smtp write message: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp close data: %w", err)
	}
	if err := client.Quit(); err != nil {
		return fmt.Errorf("smtp quit: %w", err)
	}
	return nil
}

// emailSubject derives a descriptive subject line from the submission.
func emailSubject(s submission.Submission) string {
	switch {
	case s.Subject != "":
		return "Contact: " + s.Subject
	case s.Reason != "":
		return "Contact: " + s.Reason
	default:
		return "Contact form submission"
	}
}

// buildMessage constructs a minimal RFC 2822 plain-text email message.
// This is a pure function and can be tested independently.
func buildMessage(from, to, subject, body string) string {
	var sb strings.Builder
	sb.WriteString("From: " + from + "\r\n")
	sb.WriteString("To: " + to + "\r\n")
	sb.WriteString("Subject: " + subject + "\r\n")
	sb.WriteString("MIME-Version: 1.0\r\n")
	sb.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	sb.WriteString("\r\n")
	// SMTP requires \r\n line endings in the message body.
	sb.WriteString(strings.ReplaceAll(body, "\n", "\r\n"))
	return sb.String()
}
