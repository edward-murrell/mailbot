package mailer

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"

	"github.com/ekm/mailbot/internal/config"
	"github.com/ekm/mailbot/internal/submission"
)

// SMTPMailer sends email via an authenticated SMTP server.
// It is context-aware: dial honours context deadlines and cancellation.
type SMTPMailer struct {
	cfg config.SMTPConfig
}

// NewSMTPMailer constructs an SMTPMailer.
func NewSMTPMailer(cfg config.SMTPConfig) *SMTPMailer {
	return &SMTPMailer{cfg: cfg}
}

func (m *SMTPMailer) Send(ctx context.Context, s submission.Submission) error {
	addr := net.JoinHostPort(m.cfg.Host, m.cfg.Port)

	conn, err := m.dial(ctx, addr)
	if err != nil {
		return fmt.Errorf("smtp dial %s: %w", addr, err)
	}

	// Apply a hard deadline so the SMTP transaction does not block
	// indefinitely if the context has no deadline.
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

	if m.cfg.Security == config.SMTPSecurityStartTLS {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(&tls.Config{ServerName: m.cfg.Host}); err != nil {
				return fmt.Errorf("smtp starttls: %w", err)
			}
		}
	}

	if err := client.Auth(m.auth()); err != nil {
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

// dial opens the appropriate connection based on cfg.Security.
//   - starttls: plain TCP; STARTTLS upgrade happens after EHLO.
//   - ssl:      implicit TLS from the start (port 465 / SMTPS).
//   - none:     plain TCP, no TLS at all (local dev / MailHog).
func (m *SMTPMailer) dial(ctx context.Context, addr string) (net.Conn, error) {
	switch m.cfg.Security {
	case config.SMTPSecuritySSL:
		return (&tls.Dialer{
			NetDialer: &net.Dialer{},
			Config:    &tls.Config{ServerName: m.cfg.Host},
		}).DialContext(ctx, "tcp", addr)
	default: // starttls and none both dial plain TCP
		return (&net.Dialer{}).DialContext(ctx, "tcp", addr)
	}
}

// auth returns the appropriate smtp.Auth for the configured security mode.
// In "none" mode a custom implementation is used that does not require TLS,
// since the user has explicitly opted into an insecure connection.
func (m *SMTPMailer) auth() smtp.Auth {
	if m.cfg.Security == config.SMTPSecurityNone {
		return &insecurePlainAuth{
			user: m.cfg.User,
			pass: m.cfg.Pass,
		}
	}
	return smtp.PlainAuth("", m.cfg.User, m.cfg.Pass, m.cfg.Host)
}

// insecurePlainAuth implements smtp.Auth using PLAIN without requiring a TLS
// connection. Only used when SMTP_SECURITY=none (e.g. local MailHog).
type insecurePlainAuth struct {
	user, pass string
}

func (a *insecurePlainAuth) Start(_ *smtp.ServerInfo) (string, []byte, error) {
	resp := []byte("\x00" + a.user + "\x00" + a.pass)
	return "PLAIN", resp, nil
}

func (a *insecurePlainAuth) Next(_ []byte, more bool) ([]byte, error) {
	if more {
		return nil, fmt.Errorf("unexpected server challenge")
	}
	return nil, nil
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
func buildMessage(from, to, subject, body string) string {
	var sb strings.Builder
	sb.WriteString("From: " + from + "\r\n")
	sb.WriteString("To: " + to + "\r\n")
	sb.WriteString("Subject: " + subject + "\r\n")
	sb.WriteString("MIME-Version: 1.0\r\n")
	sb.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	sb.WriteString("\r\n")
	sb.WriteString(strings.ReplaceAll(body, "\n", "\r\n"))
	return sb.String()
}
