package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindProjectConfig_InSameDir(t *testing.T) {
	tempDir := t.TempDir()

	configContent := `
profile: Zephyr
baud: 115200
strip_prefix: '^(?:\w+:~\$\s*)'
decoders:
  - match: '^instruments,\s*(?P<cpu>\d+)'
    format: 'CPU: {cpu}'
`
	configPath := filepath.Join(tempDir, ".oml.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := FindProjectConfig(tempDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatalf("expected project config to be found")
	}
	if cfg.Profile != "Zephyr" {
		t.Errorf("expected profile=Zephyr, got %q", cfg.Profile)
	}
	if cfg.Baud != 115200 {
		t.Errorf("expected baud=115200, got %d", cfg.Baud)
	}
	if cfg.StripPrefix != `^(?:\w+:~\$\s*)` {
		t.Errorf("expected strip_prefix=^(?:\\w+:~\\$\\s*), got %q", cfg.StripPrefix)
	}
	if len(cfg.Decoders) != 1 {
		t.Fatalf("expected 1 decoder, got %d", len(cfg.Decoders))
	}
	if cfg.Decoders[0].Match != `^instruments,\s*(?P<cpu>\d+)` {
		t.Errorf("unexpected match pattern: %q", cfg.Decoders[0].Match)
	}
	if cfg.ConfigPath != configPath {
		t.Errorf("expected config path %q, got %q", configPath, cfg.ConfigPath)
	}
}

func TestFindProjectConfig_InParentDir(t *testing.T) {
	tempDir := t.TempDir()
	subDir := filepath.Join(tempDir, "src", "submodule")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	configContent := `profile: CustomProfile`
	if err := os.WriteFile(filepath.Join(tempDir, ".oml.yml"), []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := FindProjectConfig(subDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatalf("expected project config to be found in parent dir")
	}
	if cfg.Profile != "CustomProfile" {
		t.Errorf("expected profile CustomProfile, got %q", cfg.Profile)
	}
}

func TestFindProjectConfig_NotFound(t *testing.T) {
	tempDir := t.TempDir()
	cfg, err := FindProjectConfig(tempDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg != nil {
		t.Errorf("expected nil config, got %+v", cfg)
	}
}
