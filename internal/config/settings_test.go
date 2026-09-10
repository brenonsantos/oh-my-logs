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
