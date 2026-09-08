package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/brenoniehues/oh-my-logs/internal/config"
)

func TestConfigLoad(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load failed: %v", err)
	}
	if cfg.ConfigDir == "" {
		t.Errorf("expected non-empty ConfigDir")
	}
	if cfg.ProfilesDir == "" {
		t.Errorf("expected non-empty ProfilesDir")
	}
	if cfg.LogsDir == "" {
		t.Errorf("expected non-empty LogsDir")
	}
}

func TestFindAllProfilePaths(t *testing.T) {
	tmpDir := t.TempDir()
	profilesDir := filepath.Join(tmpDir, "profiles")
	if err := os.MkdirAll(profilesDir, 0o755); err != nil {
		t.Fatalf("failed to create temp profiles dir: %v", err)
	}
	dummyFile := filepath.Join(profilesDir, "test.yaml")
	if err := os.WriteFile(dummyFile, []byte("name: Test\n"), 0o644); err != nil {
		t.Fatalf("failed to write dummy profile: %v", err)
	}

	cfg := &config.AppConfig{
		ConfigDir:   tmpDir,
		ProfilesDir: profilesDir,
		LogsDir:     filepath.Join(tmpDir, "logs"),
	}

	paths := config.FindAllProfilePaths(cfg)
	found := false
	for _, p := range paths {
		if p == dummyFile {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected %q to be found in %v", dummyFile, paths)
	}
}
