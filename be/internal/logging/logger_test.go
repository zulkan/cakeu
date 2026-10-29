package logging_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goskeleton/internal/logging"
)

func TestNewLoggerCreatesLogFileAndWritesMessage(t *testing.T) {
	t.Parallel()

	logDir := filepath.Join(t.TempDir(), "logs")
	logger, cleanup, err := logging.New(logDir)
	if err != nil {
		t.Fatalf("expected logger creation to succeed, got error: %v", err)
	}
	t.Cleanup(func() {
		if err := cleanup(); err != nil {
			t.Fatalf("expected cleanup to succeed, got error: %v", err)
		}
	})

	logger.Info("hello from test")

	logPath := filepath.Join(logDir, "app.log")
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("expected log file to exist, got error: %v", err)
	}

	if !strings.Contains(string(content), "hello from test") {
		t.Fatalf("expected log file to contain written message, got %q", string(content))
	}

	info, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("expected stat on log file to succeed, got error: %v", err)
	}

	if info.Mode().Perm() != 0o600 {
		t.Fatalf("expected log file permissions 0600, got %o", info.Mode().Perm())
	}
}
