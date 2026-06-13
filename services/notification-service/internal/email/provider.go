package email

import (
	"bytes"
	"context"
	"fmt"
	"net/smtp"
	"strings"

	"go.uber.org/zap"
)

// Provider abstracts email delivery so tests can mock it.
type Provider interface {
	Send(ctx context.Context, to, subject, body string) error
}

// MaskEmail masks an email address for safe logging.
func MaskEmail(email string) string {
	at := strings.Index(email, "@")
	if at <= 0 {
		if len(email) <= 3 {
			return strings.Repeat("*", len(email))
		}
		return email[:3] + strings.Repeat("*", len(email)-3)
	}
	local := email[:at]
	if len(local) <= 3 {
		return strings.Repeat("*", len(local)) + email[at:]
	}
	return local[:3] + strings.Repeat("*", len(local)-3) + email[at:]
}

// SMTPProvider sends emails via SMTP.
type SMTPProvider struct {
	addr     string
	from     string
	username string
	password string
}

// NewSMTPProvider creates a new SMTP email provider.
func NewSMTPProvider(host string, port int, from, username, password string) *SMTPProvider {
	return &SMTPProvider{
		addr:     fmt.Sprintf("%s:%d", host, port),
		from:     from,
		username: username,
		password: password,
	}
}

func (p *SMTPProvider) Send(ctx context.Context, to, subject, body string) error {
	msg := buildMessage(p.from, to, subject, body)

	var auth smtp.Auth
	if p.username != "" {
		auth = smtp.PlainAuth("", p.username, p.password, strings.Split(p.addr, ":")[0])
	}

	done := make(chan error, 1)
	go func() {
		done <- smtp.SendMail(p.addr, auth, p.from, []string{to}, msg)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		return err
	}
}

func buildMessage(from, to, subject, body string) []byte {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "From: %s\r\n", from)
	fmt.Fprintf(&buf, "To: %s\r\n", to)
	fmt.Fprintf(&buf, "Subject: %s\r\n", subject)
	fmt.Fprint(&buf, "MIME-Version: 1.0\r\n")
	fmt.Fprint(&buf, "Content-Type: text/plain; charset=\"utf-8\"\r\n")
	fmt.Fprint(&buf, "\r\n")
	fmt.Fprint(&buf, body)
	return buf.Bytes()
}

// LogProvider logs the email instead of sending it. Useful for local development.
type LogProvider struct {
	log *zap.Logger
}

// NewLogProvider creates a provider that only logs emails.
func NewLogProvider(log *zap.Logger) *LogProvider {
	return &LogProvider{log: log}
}

func (p *LogProvider) Send(ctx context.Context, to, subject, body string) error {
	p.log.Info("email logged",
		zap.String("to", MaskEmail(to)),
		zap.String("subject", subject),
	)
	return nil
}
