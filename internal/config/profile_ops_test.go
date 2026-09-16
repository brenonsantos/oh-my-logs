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
			if p.ID <= 0 {
				t.Errorf("expected positive profile ID, got %d", p.ID)
			}
			break
		}
	}
	if !found {
		t.Errorf("expected CustomTest profile to be found and marked global in %v", list)
	}
}

func TestResolveProfile_ByIDAndName(t *testing.T) {
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

	// Resolve by name
	byName, err := config.ResolveProfile(appCfg, "CustomTest")
	if err != nil {
		t.Fatalf("failed to resolve by name: %v", err)
	}
	if byName.Name != "CustomTest" {
		t.Errorf("expected CustomTest, got %q", byName.Name)
	}

	// Resolve by numeric ID
	byID, err := config.ResolveProfile(appCfg, "1")
	if err != nil {
		t.Fatalf("failed to resolve by ID 1: %v", err)
	}
	if byID.Name != "CustomTest" {
		t.Errorf("expected CustomTest, got %q", byID.Name)
	}

	// Resolve by formatted ID "#1"
	byFormattedID, err := config.ResolveProfile(appCfg, "#1")
	if err != nil {
		t.Fatalf("failed to resolve by #1: %v", err)
	}
	if byFormattedID.Name != "CustomTest" {
		t.Errorf("expected CustomTest, got %q", byFormattedID.Name)
	}
}

func TestUninstallProfile_GlobalAndDecoders(t *testing.T) {
	tmpDir := t.TempDir()
	profilesDir := filepath.Join(tmpDir, "profiles")
	decodersDir := filepath.Join(tmpDir, "decoders")
	_ = os.MkdirAll(profilesDir, 0o755)
	_ = os.MkdirAll(decodersDir, 0o755)

	profFile := filepath.Join(profilesDir, "customtest.yaml")
	_ = os.WriteFile(profFile, []byte(sampleProfileYAML), 0o644)

	// Companion decoders dir
	compDir := filepath.Join(decodersDir, "customtest")
	_ = os.MkdirAll(compDir, 0o755)
	_ = os.WriteFile(filepath.Join(compDir, "script.py"), []byte("#!/usr/bin/env python3\n"), 0o755)

	appCfg := &config.AppConfig{
		ConfigDir:   tmpDir,
		ProfilesDir: profilesDir,
		DecodersDir: decodersDir,
		LogsDir:     filepath.Join(tmpDir, "logs"),
	}

	// Uninstall by ID
	info, err := config.UninstallProfile(appCfg, "1", true)
	if err != nil {
		t.Fatalf("UninstallProfile failed: %v", err)
	}
	if info.Name != "CustomTest" {
		t.Errorf("expected CustomTest uninstalled, got %s", info.Name)
	}

	// Verify profile file was deleted
	if _, err := os.Stat(profFile); !os.IsNotExist(err) {
		t.Errorf("expected profile file to be deleted")
	}

	// Verify companion decoder dir was deleted
	if _, err := os.Stat(compDir); !os.IsNotExist(err) {
		t.Errorf("expected companion decoders dir to be deleted")
	}
}

func TestInspectProfile_Success(t *testing.T) {
	tmpDir := t.TempDir()
	profilesDir := filepath.Join(tmpDir, "profiles")
	_ = os.MkdirAll(profilesDir, 0o755)
	profFile := filepath.Join(profilesDir, "customtest.yaml")
	_ = os.WriteFile(profFile, []byte(sampleProfileYAML), 0o644)

	appCfg := &config.AppConfig{
		ConfigDir:   tmpDir,
		ProfilesDir: profilesDir,
		LogsDir:     filepath.Join(tmpDir, "logs"),
	}

	insp, err := config.InspectProfile(appCfg, "CustomTest")
	if err != nil {
		t.Fatalf("InspectProfile failed: %v", err)
	}
	if insp.Profile.Name != "CustomTest" {
		t.Errorf("expected CustomTest, got %s", insp.Profile.Name)
	}
	if len(insp.Profile.Columns) != 1 {
		t.Errorf("expected 1 column, got %d", len(insp.Profile.Columns))
	}
}

