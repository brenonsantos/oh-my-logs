package config_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

	// Verify that profiles directory is initialized
	if _, err := os.Stat(appCfg.ProfilesDir); err != nil {
		t.Fatalf("profiles directory not created: %v", err)
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

func TestCopyProfilesFrom(t *testing.T) {
	srcDir := t.TempDir()
	targetDir := t.TempDir()

	// Write mock yaml profiles in srcDir
	_ = os.WriteFile(filepath.Join(srcDir, "mock1.yaml"), []byte("name: mock1\n"), 0o644)
	_ = os.WriteFile(filepath.Join(srcDir, "mock2.yml"), []byte("name: mock2\n"), 0o644)
	_ = os.WriteFile(filepath.Join(srcDir, "ignored.txt"), []byte("not a yaml\n"), 0o644)

	copied := config.CopyProfilesFrom(srcDir, targetDir)
	if copied != 2 {
		t.Fatalf("expected 2 profiles copied, got %d", copied)
	}

	// Second copy should not overwrite existing files
	copiedAgain := config.CopyProfilesFrom(srcDir, targetDir)
	if copiedAgain != 0 {
		t.Errorf("expected 0 profiles copied on second run, got %d", copiedAgain)
	}
}

func TestUninstallBinary_CustomTargetNotFound(t *testing.T) {
	emptyDir := t.TempDir()
	_, err := config.UninstallBinary(emptyDir)
	if err == nil || !strings.Contains(err.Error(), "binary not found") {
		t.Errorf("expected binary not found error, got: %v", err)
	}
}

