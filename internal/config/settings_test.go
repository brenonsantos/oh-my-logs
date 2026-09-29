package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSettingsSaveAndLoad(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "oml-settings-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cfg := &AppConfig{
		ConfigDir:   tempDir,
		ProfilesDir: filepath.Join(tempDir, "profiles"),
		LogsDir:     filepath.Join(tempDir, "logs"),
	}

	// 1. Initial load when file does not exist
	s, err := cfg.LoadSettings()
	if err != nil {
		t.Fatalf("LoadSettings failed on missing file: %v", err)
	}
	if s.Baud != 115200 {
		t.Errorf("expected default baud 115200, got %d", s.Baud)
	}
	if s.BufferCapacity != 50000 {
		t.Errorf("expected default buffer capacity 50000, got %d", s.BufferCapacity)
	}
	if !s.DefaultFollow {
		t.Errorf("expected default follow true")
	}
	if s.Theme != "dark-slate" {
		t.Errorf("expected default theme dark-slate, got %s", s.Theme)
	}

	// 2. Save settings
	s.Port = "/dev/ttyUSB0"
	s.Baud = 921600
	s.Profile = "STM32-PDM"
	s.ShowTimestamp = true
	s.BufferCapacity = 100000
	s.DefaultFollow = false
	s.DirectToDisk = true
	s.Theme = "monokai"

	if err := cfg.SaveSettings(s); err != nil {
		t.Fatalf("SaveSettings failed: %v", err)
	}

	// 3. Load saved settings
	loaded, err := cfg.LoadSettings()
	if err != nil {
		t.Fatalf("LoadSettings failed: %v", err)
	}

	if loaded.Port != "/dev/ttyUSB0" {
		t.Errorf("expected port /dev/ttyUSB0, got %s", loaded.Port)
	}
	if loaded.Baud != 921600 {
		t.Errorf("expected baud 921600, got %d", loaded.Baud)
	}
	if loaded.Profile != "STM32-PDM" {
		t.Errorf("expected profile STM32-PDM, got %s", loaded.Profile)
	}
	if !loaded.ShowTimestamp {
		t.Errorf("expected ShowTimestamp to be true")
	}
	if loaded.BufferCapacity != 100000 {
		t.Errorf("expected buffer capacity 100000, got %d", loaded.BufferCapacity)
	}
	if loaded.DefaultFollow {
		t.Errorf("expected DefaultFollow false")
	}
	if !loaded.DirectToDisk {
		t.Errorf("expected DirectToDisk true")
	}
	if loaded.Theme != "monokai" {
		t.Errorf("expected theme monokai, got %s", loaded.Theme)
	}
}

func TestSettings_ColumnCustomization(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &AppConfig{
		ConfigDir:   tempDir,
		ProfilesDir: filepath.Join(tempDir, "profiles"),
		LogsDir:     filepath.Join(tempDir, "logs"),
	}

	s, err := cfg.LoadSettings()
	if err != nil {
		t.Fatalf("LoadSettings failed: %v", err)
	}

	// Verify empty initially
	emptyCust := s.GetColumnCustomization("CustomProfile")
	if len(emptyCust.HiddenColumns) != 0 || len(emptyCust.ColumnWidths) != 0 {
		t.Errorf("expected empty customization, got %+v", emptyCust)
	}

	// Set customization
	s.SetColumnCustomization("CustomProfile", ColumnCustomization{
		HiddenColumns: []string{"module"},
		ColumnWidths:  map[string]int{"uptime": 25},
	})

	if err := cfg.SaveSettings(s); err != nil {
		t.Fatalf("SaveSettings failed: %v", err)
	}

	// Reload from disk
	loaded, err := cfg.LoadSettings()
	if err != nil {
		t.Fatalf("LoadSettings failed: %v", err)
	}

	cust := loaded.GetColumnCustomization("customprofile") // test case-insensitivity
	if len(cust.HiddenColumns) != 1 || cust.HiddenColumns[0] != "module" {
		t.Errorf("expected HiddenColumns [module], got %v", cust.HiddenColumns)
	}
	if cust.ColumnWidths["uptime"] != 25 {
		t.Errorf("expected ColumnWidths[uptime] == 25, got %d", cust.ColumnWidths["uptime"])
	}

	// Clear customization
	loaded.SetColumnCustomization("customprofile", ColumnCustomization{})
	if err := cfg.SaveSettings(loaded); err != nil {
		t.Fatalf("SaveSettings failed: %v", err)
	}

	reloaded, err := cfg.LoadSettings()
	if err != nil {
		t.Fatalf("LoadSettings failed: %v", err)
	}
	clearedCust := reloaded.GetColumnCustomization("CustomProfile")
	if len(clearedCust.HiddenColumns) != 0 || len(clearedCust.ColumnWidths) != 0 {
		t.Errorf("expected cleared customization, got %+v", clearedCust)
	}
}

func TestResolveProfilePath_DirectoryConflict(t *testing.T) {
	tempDir := t.TempDir()
	profilesDir := filepath.Join(tempDir, "profiles")
	if err := os.MkdirAll(profilesDir, 0o755); err != nil {
		t.Fatalf("failed to create profilesDir: %v", err)
	}

	// Create an installed profile in appCfg.ProfilesDir
	profileContent := `name: DeviceProfile
description: Test Device Profile
parser:
  type: raw
`
	profileYAML := filepath.Join(profilesDir, "deviceprofile.yaml")
	if err := os.WriteFile(profileYAML, []byte(profileContent), 0o644); err != nil {
		t.Fatalf("failed to write profile file: %v", err)
	}

	appCfg := &AppConfig{
		ConfigDir:   tempDir,
		ProfilesDir: profilesDir,
		LogsDir:     filepath.Join(tempDir, "logs"),
	}

	// Create a local directory with the same name "DeviceProfile" in temp working directory
	workDir := filepath.Join(tempDir, "workspace")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("failed to create workspace: %v", err)
	}
	conflictDir := filepath.Join(workDir, "DeviceProfile")
	if err := os.MkdirAll(conflictDir, 0o755); err != nil {
		t.Fatalf("failed to create conflict dir: %v", err)
	}

	// Change working directory to workDir during this test
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd error: %v", err)
	}
	defer func() {
		_ = os.Chdir(origWd)
	}()
	if err := os.Chdir(workDir); err != nil {
		t.Fatalf("Chdir error: %v", err)
	}

	// 1. ResolveProfilePath("DeviceProfile"):
	// Even though a local directory named "DeviceProfile" exists,
	// it must NOT return the directory. It must resolve to the installed profile YAML.
	resolved := ResolveProfilePath(appCfg, "DeviceProfile")
	if resolved != profileYAML {
		t.Fatalf("expected resolved path %q, got %q", profileYAML, resolved)
	}

	// 2. Resolve by lowercase/slug name
	resolvedSlug := ResolveProfilePath(appCfg, "deviceprofile")
	if resolvedSlug != profileYAML {
		t.Fatalf("expected resolved slug %q, got %q", profileYAML, resolvedSlug)
	}

	// 3. Directly pointing to a directory that has NO profile inside should not return the directory
	nonProfileDir := filepath.Join(workDir, "SomeOtherDir")
	if err := os.MkdirAll(nonProfileDir, 0o755); err != nil {
		t.Fatalf("failed to create nonProfileDir: %v", err)
	}
	if res := ResolveProfilePath(appCfg, "SomeOtherDir"); res != "" {
		t.Fatalf("expected empty string for directory without profiles, got %q", res)
	}
}

func TestResolveProfilePath_BundleDirectory(t *testing.T) {
	tempDir := t.TempDir()
	appCfg := &AppConfig{
		ConfigDir:   tempDir,
		ProfilesDir: filepath.Join(tempDir, "profiles"),
		LogsDir:     filepath.Join(tempDir, "logs"),
	}

	// Create bundle directory with profile.yaml inside
	bundleDir := filepath.Join(tempDir, "bundle_test")
	if err := os.MkdirAll(bundleDir, 0o755); err != nil {
		t.Fatalf("failed to create bundleDir: %v", err)
	}
	bundleProfile := filepath.Join(bundleDir, "profile.yaml")
	if err := os.WriteFile(bundleProfile, []byte("name: BundleProfile\nparser:\n  type: raw\n"), 0o644); err != nil {
		t.Fatalf("failed to write bundle profile: %v", err)
	}

	resolved := ResolveProfilePath(appCfg, bundleDir)
	if resolved != bundleProfile {
		t.Fatalf("expected %q, got %q", bundleProfile, resolved)
	}
}
