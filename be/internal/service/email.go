package service

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/smtp"
	"strings"

	"github.com/jordan-wright/email"
)

type EmailConfig struct {
	Host        string
	Port        int
	Username    string
	Password    string
	From        string
	FrontendURL string
}

type EmailService struct {
	config EmailConfig
	logger *slog.Logger
}

func NewEmailService(config EmailConfig, logger *slog.Logger) *EmailService {
	if logger == nil {
		logger = slog.Default()
	}
	return &EmailService{config: config, logger: logger}
}

func (s *EmailService) resetLink(resetToken string) string {
	return strings.TrimRight(s.config.FrontendURL, "/") + "/reset-password?token=" + resetToken
}

func (s *EmailService) SendPasswordResetEmail(ctx context.Context, to, resetToken string) error {
	link := s.resetLink(resetToken)

	// Dev fallback: placeholder or empty credentials mean SMTP is not configured.
	if s.config.Username == "" || s.config.Password == "" ||
		strings.HasPrefix(s.config.Username, "your-") || strings.HasPrefix(s.config.Password, "your-") {
		s.logger.Log(ctx, slog.LevelWarn, "password reset link (SMTP not configured)", "to", to, "link", link)
		return nil
	}

	e := email.NewEmail()
	e.From = fmt.Sprintf("Catatku <%s>", s.config.From)
	e.To = []string{to}
	e.Subject = "Reset your Catatku password"
	e.Text = []byte(fmt.Sprintf("Click the link below to reset your password:\n\n%s\n\nIf you did not request this, please ignore this email.", link))
	e.HTML = []byte(fmt.Sprintf(`
<p>Click the link below to reset your password:</p>
<p><a href="%s">%s</a></p>
<p>If you did not request this, please ignore this email.</p>
`, link, link))

	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)

	auth := smtp.PlainAuth("", s.config.Username, s.config.Password, s.config.Host)
	if s.config.Port == 465 {
		return e.SendWithTLS(addr, auth, &tls.Config{ServerName: s.config.Host})
	}

	return e.Send(addr, auth)
}
