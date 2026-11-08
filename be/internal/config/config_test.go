package config

import (
	"strings"
	"testing"
)

func TestLoadSmartImportDefaults(t *testing.T) {
	t.Setenv("SMART_IMPORT_MODE", "")
	t.Setenv("SMART_IMPORT_SAMPLE_ROWS", "")
	t.Setenv("JWT_SECRET", strings.Repeat("x", 32))

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.SmartImportMode != "ai" {
		t.Fatalf("expected ai, got %q", cfg.SmartImportMode)
	}
	if cfg.SmartImportSampleRows != 30 {
		t.Fatalf("expected 30, got %d", cfg.SmartImportSampleRows)
	}
}

func TestLoadSmartImportClamps(t *testing.T) {
	t.Setenv("SMART_IMPORT_MODE", "nonsense")
	t.Setenv("SMART_IMPORT_SAMPLE_ROWS", "999")
	t.Setenv("JWT_SECRET", strings.Repeat("x", 32))

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.SmartImportMode != "ai" {
		t.Fatalf("expected ai fallback, got %q", cfg.SmartImportMode)
	}
	if cfg.SmartImportSampleRows != 50 {
		t.Fatalf("expected clamp to 50, got %d", cfg.SmartImportSampleRows)
	}
}

func TestLoadRejectsMissingJWTSecret(t *testing.T) {
	t.Setenv("JWT_SECRET", "")

	if _, err := Load(); err == nil {
		t.Fatal("expected an error when JWT_SECRET is empty")
	}
}

func TestLoadRejectsPlaceholderJWTSecret(t *testing.T) {
	t.Setenv("JWT_SECRET", defaultJWTSecret)

	if _, err := Load(); err == nil {
		t.Fatal("expected an error when JWT_SECRET is the default placeholder")
	}
}

func TestLoadWarnsOnShortJWTSecret(t *testing.T) {
	t.Setenv("JWT_SECRET", "short-secret")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Warnings) == 0 {
		t.Fatal("expected a warning for a short JWT secret")
	}
}

func TestLoadAcceptsLongJWTSecretWithoutWarnings(t *testing.T) {
	t.Setenv("JWT_SECRET", strings.Repeat("x", 40))

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", cfg.Warnings)
	}
}
