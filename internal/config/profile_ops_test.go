package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/brenoniehues/oh-my-logs/internal/config"
)

const sampleProfileYAML = `
name: CustomTest
parser:
  type: raw
columns:
  - field: message
    title: Message
    width: 0
`

const invalidProfileYAML = `
name: Broken
parser:
  type: regex
  pattern: '[invalid(regex'
`

func TestImportProfile_Success(t *testing.T) {
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "source.yaml")
	if err := os.WriteFile(srcFile, []byte(sampleProfileYAML), 0o644); err != nil {
		t.Fatalf("failed to write source profile: %v", err)
	}

	appCfg := &config.AppConfig{
		ConfigDir:   tmpDir,
		ProfilesDir: filepath.Join(tmpDir, "profiles"),
		LogsDir:     filepath.Join(tmpDir, "logs"),
	}

	targetPath, name, err := config.ImportProfile(appCfg, srcFile)
	if err != nil {
		t.Fatalf("ImportProfile failed: %v", err)
	}
	if name != "CustomTest" {
		t.Errorf("expected profile name 'CustomTest', got %q", name)
	}
	if filepath.Base(targetPath) != "customtest.yaml" {
		t.Errorf("expected target filename 'customtest.yaml', got %q", filepath.Base(targetPath))
	}
	if _, err := os.Stat(targetPath); err != nil {
		t.Errorf("target file not found: %v", err)
	}
}

func TestImportProfile_InvalidRegex(t *testing.T) {
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "invalid.yaml")
	if err := os.WriteFile(srcFile, []byte(invalidProfileYAML), 0o644); err != nil {
		t.Fatalf("failed to write invalid profile: %v", err)
	}

	appCfg := &config.AppConfig{
		ConfigDir:   tmpDir,
		ProfilesDir: filepath.Join(tmpDir, "profiles"),
		LogsDir:     filepath.Join(tmpDir, "logs"),
	}

	_, _, err := config.ImportProfile(appCfg, srcFile)
	if err == nil {
		t.Fatal("expected error for invalid regex, got nil")
	}
}

func TestExportProfile_Success(t *testing.T) {
	tmpDir := t.TempDir()
	profilesDir := filepath.Join(tmpDir, "profiles")
	if err := os.MkdirAll(profilesDir, 0o755); err != nil {
		t.Fatalf("failed to create profiles dir: %v", err)
	}
	profFile := filepath.Join(profilesDir, "customtest.yaml")
	if err := os.WriteFile(profFile, []byte(sampleProfileYAML), 0o644); err != nil {
		t.Fatalf("failed to write sample profile: %v", err)
	}

	appCfg := &config.AppConfig{
		ConfigDir:   tmpDir,
		ProfilesDir: profilesDir,
		LogsDir:     filepath.Join(tmpDir, "logs"),
	}

	data, path, err := config.ExportProfile(appCfg, "CustomTest")
	if err != nil {
		t.Fatalf("ExportProfile failed: %v", err)
	}
	if path != profFile {
		t.Errorf("expected path %q, got %q", profFile, path)
	}
	if len(data) == 0 {
		t.Errorf("expected non-empty profile content")
	}
}

func TestListProfiles(t *testing.T) {
	tmpDir := t.TempDir()
	profilesDir := filepath.Join(tmpDir, "profiles")
	if err := os.MkdirAll(profilesDir, 0o755); err != nil {
		t.Fatalf("failed to create profiles dir: %v", err)
	}
	profFile := filepath.Join(profilesDir, "customtest.yaml")
	if err := os.WriteFile(profFile, []byte(sampleProfileYAML), 0o644); err != nil {
		t.Fatalf("failed to write sample profile: %v", err)
	}

	appCfg := &config.AppConfig{
		ConfigDir:   tmpDir,
		ProfilesDir: profilesDir,
		LogsDir:     filepath.Join(tmpDir, "logs"),
	}

	list := config.ListProfiles(appCfg)
	found := false
	for _, p := range list {
		if p.Name == "CustomTest" && p.IsGlobal {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected CustomTest profile to be found and marked global in %v", list)
	}
}
