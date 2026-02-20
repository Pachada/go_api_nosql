package smtp

import (
	"crypto/tls"
	"fmt"
	"net/smtp"

	"github.com/go-api-nosql/internal/config"
)

// Mailer sends emails.
type Mailer interface {
	SendEmail(to, subject, body string) error
}

type mailer struct {
	host       string
	port       string
	from       string
	username   string
	password   string
	tlsEnabled bool
}

func NewMailer(cfg *config.Config) Mailer {
	return &mailer{
		host:       cfg.SMTPHost,
		port:       cfg.SMTPPort,
		from:       cfg.SMTPFrom,
		username:   cfg.SMTPUsername,
		password:   cfg.SMTPPassword,
		tlsEnabled: cfg.SMTPTLSEnabled,
	}
}

func (m *mailer) SendEmail(to, subject, body string) error {
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s", m.from, to, subject, body)
	addr := fmt.Sprintf("%s:%s", m.host, m.port)

	if !m.tlsEnabled {
		// Local dev path (e.g. MailHog): no TLS required.
		var auth smtp.Auth
		if m.username != "" {
			auth = smtp.PlainAuth("", m.username, m.password, m.host)
		}
		return smtp.SendMail(addr, auth, m.from, []string{to}, []byte(msg))
	}

	// Production path: dial then upgrade to TLS via STARTTLS (fail-secure).
	c, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("smtp dial: %w", err)
	}
	defer c.Close()

	if err := c.StartTLS(&tls.Config{ServerName: m.host}); err != nil {
		return fmt.Errorf("smtp starttls: %w", err)
	}
	if m.username != "" {
		auth := smtp.PlainAuth("", m.username, m.password, m.host)
		if err := c.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}
	if err := c.Mail(m.from); err != nil {
		return fmt.Errorf("smtp mail from: %w", err)
	}
	if err := c.Rcpt(to); err != nil {
		return fmt.Errorf("smtp rcpt: %w", err)
	}
	wc, err := c.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err := fmt.Fprint(wc, msg); err != nil {
		return fmt.Errorf("smtp write body: %w", err)
	}
	if err := wc.Close(); err != nil {
		return fmt.Errorf("smtp close data: %w", err)
	}
	return c.Quit()
}
