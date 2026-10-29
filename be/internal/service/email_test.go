package service

import (
	"context"
	"log/slog"
	"testing"
)

func TestResetLinkFormat(t *testing.T) {
	t.Parallel()

	s := NewEmailService(EmailConfig{FrontendURL: "https://app.catatku.com/"}, nil)
	link := s.resetLink("abc123")
	want := "https://app.catatku.com/reset-password?token=abc123"
	if link != want {
		t.Fatalf("expected %q, got %q", want, link)
	}
}

type captureHandler struct {
	records []slog.Record
}

func (h *captureHandler) Enabled(ctx context.Context, l slog.Level) bool { return true }
func (h *captureHandler) Handle(ctx context.Context, r slog.Record) error {
	h.records = append(h.records, r)
	return nil
}
func (h *captureHandler) WithAttrs(attrs []slog.Attr) slog.Handler { return h }
func (h *captureHandler) WithGroup(name string) slog.Handler       { return h }

func recordAttr(r slog.Record, key string) string {
	var out string
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == key {
			out = a.Value.String()
			return false
		}
		return true
	})
	return out
}

func TestPlaceholderSMTPNoNetwork(t *testing.T) {
	t.Parallel()

	cap := &captureHandler{}
	logger := slog.New(cap)
	s := NewEmailService(EmailConfig{
		FrontendURL: "http://localhost:3000",
		Host:        "smtp.example.com",
		Port:        587,
		Username:    "your-user",
		Password:    "your-pass",
	}, logger)

	if err := s.SendPasswordResetEmail(context.Background(), "a@example.com", "tok123"); err != nil {
		t.Fatalf("expected nil error for placeholder SMTP, got %v", err)
	}
	if len(cap.records) != 1 {
		t.Fatalf("expected 1 log record, got %d", len(cap.records))
	}
	if cap.records[0].Message != "password reset link (SMTP not configured)" {
		t.Fatalf("expected warn message, got %q", cap.records[0].Message)
	}
	if got := recordAttr(cap.records[0], "to"); got != "a@example.com" {
		t.Fatalf("expected to=a@example.com, got %q", got)
	}
	if got := recordAttr(cap.records[0], "link"); got != "http://localhost:3000/reset-password?token=tok123" {
		t.Fatalf("expected link with token, got %q", got)
	}
}

func TestEmptySMTPNoNetwork(t *testing.T) {
	t.Parallel()

	cap := &captureHandler{}
	logger := slog.New(cap)
	s := NewEmailService(EmailConfig{
		FrontendURL: "http://localhost:3000",
		Host:        "smtp.example.com",
		Port:        587,
	}, logger)

	if err := s.SendPasswordResetEmail(context.Background(), "a@example.com", "tok"); err != nil {
		t.Fatalf("expected nil error for empty SMTP credentials, got %v", err)
	}
	if len(cap.records) != 1 {
		t.Fatalf("expected 1 log record, got %d", len(cap.records))
	}
}
