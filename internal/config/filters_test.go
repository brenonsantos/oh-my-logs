package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFiltersLoadAndSave(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &AppConfig{
		ConfigDir: tmpDir,
	}

	// 1. Initial load should return defaults
	fc, err := cfg.LoadFilters()
	if err != nil {
		t.Fatalf("unexpected error on initial load: %v", err)
	}
	if len(fc.Presets) == 0 {
		t.Fatalf("expected default presets on initial load, got 0")
	}

	// 2. Add custom preset and history
	fc.Presets = append(fc.Presets, FilterPreset{
		Name:        "Test BLE",
		Description: "BLE logs",
		Filter:      "tag:ble",
	})
	fc.History = []string{"level:err", "tag:ble"}

	if err := cfg.SaveFilters(fc); err != nil {
		t.Fatalf("failed to save filters: %v", err)
	}

	// 3. Reload from disk and verify
	reloaded, err := cfg.LoadFilters()
	if err != nil {
		t.Fatalf("failed to reload filters: %v", err)
	}
	if len(reloaded.Presets) != len(fc.Presets) {
		t.Errorf("expected %d presets, got %d", len(fc.Presets), len(reloaded.Presets))
	}
	if len(reloaded.History) != 2 {
		t.Errorf("expected 2 history entries, got %d", len(reloaded.History))
	}
	lastPreset := reloaded.Presets[len(reloaded.Presets)-1]
	if lastPreset.Name != "Test BLE" || lastPreset.Filter != "tag:ble" {
		t.Errorf("unexpected preset: %+v", lastPreset)
	}
}

func TestFiltersNilConfig(t *testing.T) {
	var cfg *AppConfig
	fc, err := cfg.LoadFilters()
	if err != nil {
		t.Fatalf("unexpected error with nil config: %v", err)
	}
	if len(fc.Presets) == 0 {
		t.Errorf("expected defaults even with nil config")
	}

	if err := cfg.SaveFilters(fc); err != nil {
		t.Errorf("unexpected error saving with nil config: %v", err)
	}

	if p := cfg.FiltersPath(); p != "" {
		t.Errorf("expected empty path for nil config, got %s", p)
	}
}

func TestFiltersCorruptFile(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &AppConfig{
		ConfigDir: tmpDir,
	}
	fPath := filepath.Join(tmpDir, "filters.yaml")
	if err := os.WriteFile(fPath, []byte("invalid: [yaml: broken"), 0o644); err != nil {
		t.Fatalf("failed to write corrupt file: %v", err)
	}

	_, err := cfg.LoadFilters()
	if err == nil {
		t.Errorf("expected error when parsing corrupt YAML, got nil")
	}
}
