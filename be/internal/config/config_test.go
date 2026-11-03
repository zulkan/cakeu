package config

import "testing"

func TestLoadSmartImportDefaults(t *testing.T) {
	t.Setenv("SMART_IMPORT_MODE", "")
	t.Setenv("SMART_IMPORT_SAMPLE_ROWS", "")

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
