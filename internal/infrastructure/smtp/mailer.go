package smtp

import (
	"fmt"
	"net/smtp"

	"github.com/go-api-nosql/internal/config"
)

// Mailer sends emails.
type Mailer interface {
	SendEmail(to, subject, body string) error
}

type mailer struct {
	host     string
	port     string
	from     string
	username string
	password string
}

func NewMailer(cfg *config.Config) Mailer {
	return &mailer{
		host:     cfg.SMTPHost,
		port:     cfg.SMTPPort,
		from:     cfg.SMTPFrom,
		username: cfg.SMTPUsername,
		password: cfg.SMTPPassword,
	}
}

func (m *mailer) SendEmail(to, subject, body string) error {
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s", m.from, to, subject, body)
	addr := fmt.Sprintf("%s:%s", m.host, m.port)

	var auth smtp.Auth
	if m.username != "" {
		auth = smtp.PlainAuth("", m.username, m.password, m.host)
	}

	return smtp.SendMail(addr, auth, m.from, []string{to}, []byte(msg))
}
