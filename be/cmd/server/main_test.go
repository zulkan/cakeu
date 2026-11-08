package main

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewServerSetsSecurityTimeouts(t *testing.T) {
	t.Parallel()

	server := newServer(":8080", http.NewServeMux())

	if server.ReadHeaderTimeout != 5*time.Second {
		t.Fatalf("expected read header timeout 5s, got %s", server.ReadHeaderTimeout)
	}

	if server.ReadTimeout != 60*time.Second {
		t.Fatalf("expected read timeout 60s, got %s", server.ReadTimeout)
	}

	// The write timeout must exceed the LLM timeout because chat handlers wait
	// on a model call before they can write the response.
	if server.WriteTimeout != 300*time.Second {
		t.Fatalf("expected write timeout 300s, got %s", server.WriteTimeout)
	}

	if server.IdleTimeout != 60*time.Second {
		t.Fatalf("expected idle timeout 60s, got %s", server.IdleTimeout)
	}
}

func TestLoadDotEnvLoadsMissingEnvironmentVariables(t *testing.T) {
	originalDatabaseURL, hadDatabaseURL := os.LookupEnv("DATABASE_URL")
	originalPort, hadPort := os.LookupEnv("PORT")
	if err := os.Unsetenv("DATABASE_URL"); err != nil {
		t.Fatalf("expected DATABASE_URL unset to succeed, got error: %v", err)
	}
	if err := os.Unsetenv("PORT"); err != nil {
		t.Fatalf("expected PORT unset to succeed, got error: %v", err)
	}
	t.Cleanup(func() {
		if hadDatabaseURL {
			if err := os.Setenv("DATABASE_URL", originalDatabaseURL); err != nil {
				t.Fatalf("expected DATABASE_URL restore to succeed, got error: %v", err)
			}
		} else if err := os.Unsetenv("DATABASE_URL"); err != nil {
			t.Fatalf("expected DATABASE_URL cleanup unset to succeed, got error: %v", err)
		}

		if hadPort {
			if err := os.Setenv("PORT", originalPort); err != nil {
				t.Fatalf("expected PORT restore to succeed, got error: %v", err)
			}
		} else if err := os.Unsetenv("PORT"); err != nil {
			t.Fatalf("expected PORT cleanup unset to succeed, got error: %v", err)
		}
	})

	workDir := t.TempDir()
	filePath := filepath.Join(workDir, ".env")
	if err := os.WriteFile(filePath, []byte("DATABASE_URL=postgres://from-dotenv\nPORT=9090\n"), 0o600); err != nil {
		t.Fatalf("expected env file creation to succeed, got error: %v", err)
	}

	previousDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("expected to get working directory, got error: %v", err)
	}
	defer func() {
		if err := os.Chdir(previousDir); err != nil {
			t.Fatalf("expected to restore working directory, got error: %v", err)
		}
	}()

	if err := os.Chdir(workDir); err != nil {
		t.Fatalf("expected to change working directory, got error: %v", err)
	}

	if err := loadDotEnv(); err != nil {
		t.Fatalf("expected dotenv load to succeed, got error: %v", err)
	}

	if got := os.Getenv("DATABASE_URL"); got != "postgres://from-dotenv" {
		t.Fatalf("expected DATABASE_URL from .env, got %q", got)
	}

	if got := os.Getenv("PORT"); got != "9090" {
		t.Fatalf("expected PORT from .env, got %q", got)
	}
}

func TestLoadDotEnvDoesNotOverrideExistingEnvironmentVariables(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://from-env")
	t.Setenv("PORT", "8088")

	workDir := t.TempDir()
	filePath := filepath.Join(workDir, ".env")
	if err := os.WriteFile(filePath, []byte("DATABASE_URL=postgres://from-dotenv\nPORT=9090\n"), 0o600); err != nil {
		t.Fatalf("expected env file creation to succeed, got error: %v", err)
	}

	previousDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("expected to get working directory, got error: %v", err)
	}
	defer func() {
		if err := os.Chdir(previousDir); err != nil {
			t.Fatalf("expected to restore working directory, got error: %v", err)
		}
	}()

	if err := os.Chdir(workDir); err != nil {
		t.Fatalf("expected to change working directory, got error: %v", err)
	}

	if err := loadDotEnv(); err != nil {
		t.Fatalf("expected dotenv load to succeed, got error: %v", err)
	}

	if got := os.Getenv("DATABASE_URL"); got != "postgres://from-env" {
		t.Fatalf("expected DATABASE_URL to keep existing env var, got %q", got)
	}

	if got := os.Getenv("PORT"); got != "8088" {
		t.Fatalf("expected PORT to keep existing env var, got %q", got)
	}
}
