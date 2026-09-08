package config_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/brenoniehues/oh-my-logs/internal/config"
)

func TestDefaultInstallDir(t *testing.T) {
	dir, err := config.DefaultInstallDir()
	if err != nil {
		t.Fatalf("DefaultInstallDir returned error: %v", err)
	}
	if dir == "" {
		t.Fatalf("DefaultInstallDir returned empty string")
	}
}

func TestInstallAndUninstallBinary(t *testing.T) {
	tmpDir := t.TempDir()
	installTargetDir := filepath.Join(tmpDir, "bin")

	appCfg := &config.AppConfig{
		ConfigDir:   tmpDir,
		ProfilesDir: filepath.Join(tmpDir, "profiles"),
		LogsDir:     filepath.Join(tmpDir, "logs"),
	}

	dest, err := config.InstallBinary(appCfg, installTargetDir)
	if err != nil {
		t.Fatalf("InstallBinary failed: %v", err)
	}

	binaryName := "oml"
	if runtime.GOOS == "windows" {
		binaryName = "oml.exe"
	}
	expectedDest := filepath.Join(installTargetDir, binaryName)
	if dest != expectedDest {
		t.Errorf("expected destination %q, got %q", expectedDest, dest)
	}

	if _, err := os.Stat(dest); err != nil {
		t.Fatalf("installed binary not found: %v", err)
	}

	// Test Uninstall
	uninstalledDest, err := config.UninstallBinary(installTargetDir)
	if err != nil {
		t.Fatalf("UninstallBinary failed: %v", err)
	}
	if uninstalledDest != expectedDest {
		t.Errorf("expected uninstalled path %q, got %q", expectedDest, uninstalledDest)
	}

	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Errorf("binary still exists after uninstall")
	}
}

func TestCopyDefaultProfiles(t *testing.T) {
	tmpDir := t.TempDir()
	targetProfiles := filepath.Join(tmpDir, "profiles")

	copied := config.CopyDefaultProfiles(targetProfiles)
	// If profiles/examples exists in working dir, it should copy
	if _, err := os.Stat(filepath.Join("profiles", "examples")); err == nil {
		if copied < 2 {
			t.Errorf("expected at least 2 default profiles copied, got %d", copied)
		}
	}
}
