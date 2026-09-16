package config_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/brenoniehues/oh-my-logs/internal/config"
)

func TestInstallSource_LocalFile(t *testing.T) {
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "local_test.yaml")
	if err := os.WriteFile(srcFile, []byte(sampleProfileYAML), 0o644); err != nil {
		t.Fatalf("failed to write profile: %v", err)
	}

	appCfg := &config.AppConfig{
		ConfigDir:   tmpDir,
		ProfilesDir: filepath.Join(tmpDir, "installed_profiles"),
		DecodersDir: filepath.Join(tmpDir, "installed_decoders"),
		LogsDir:     filepath.Join(tmpDir, "logs"),
	}

	results, err := config.InstallSource(appCfg, srcFile, config.InstallOptions{})
	if err != nil {
		t.Fatalf("InstallSource failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Name != "CustomTest" {
		t.Errorf("expected name CustomTest, got %s", results[0].Name)
	}
	if _, err := os.Stat(results[0].TargetPath); err != nil {
		t.Errorf("target file not found: %v", err)
	}

	// Test overwrite protection without force
	_, err = config.InstallSource(appCfg, srcFile, config.InstallOptions{Force: false})
	if err == nil {
		t.Fatalf("expected error when installing existing profile without Force, got nil")
	}

	// Test overwrite with force
	results2, err := config.InstallSource(appCfg, srcFile, config.InstallOptions{Force: true})
	if err != nil {
		t.Fatalf("expected success when installing with Force: %v", err)
	}
	if !results2[0].Overwritten {
		t.Errorf("expected Overwritten = true")
	}
}

func TestInstallSource_DirectoryWithDecoders(t *testing.T) {
	tmpDir := t.TempDir()
	bundleDir := filepath.Join(tmpDir, "bundle")
	profilesSub := filepath.Join(bundleDir, "profiles")
	decodersSub := filepath.Join(bundleDir, "decoders")

	_ = os.MkdirAll(profilesSub, 0o755)
	_ = os.MkdirAll(decodersSub, 0o755)

	_ = os.WriteFile(filepath.Join(profilesSub, "app.yaml"), []byte(sampleProfileYAML), 0o644)
	_ = os.WriteFile(filepath.Join(decodersSub, "worker.py"), []byte("#!/usr/bin/env python3\nprint('ok')"), 0o755)

	appCfg := &config.AppConfig{
		ConfigDir:   tmpDir,
		ProfilesDir: filepath.Join(tmpDir, "profiles"),
		DecodersDir: filepath.Join(tmpDir, "decoders"),
		LogsDir:     filepath.Join(tmpDir, "logs"),
	}

	results, err := config.InstallSource(appCfg, bundleDir, config.InstallOptions{})
	if err != nil {
		t.Fatalf("InstallSource bundle failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].DecodersInstalled != 1 {
		t.Errorf("expected 1 companion decoder installed, got %d", results[0].DecodersInstalled)
	}
	if results[0].CompanionDecoders == "" {
		t.Errorf("expected CompanionDecoders path set")
	}

	// Check installed decoder script exists
	installedScript := filepath.Join(results[0].CompanionDecoders, "worker.py")
	if _, err := os.Stat(installedScript); err != nil {
		t.Errorf("installed decoder script not found: %v", err)
	}
}

func TestInstallSource_HTTP(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer secret-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(sampleProfileYAML))
	}))
	defer ts.Close()

	tmpDir := t.TempDir()
	appCfg := &config.AppConfig{
		ConfigDir:   tmpDir,
		ProfilesDir: filepath.Join(tmpDir, "profiles"),
		DecodersDir: filepath.Join(tmpDir, "decoders"),
		LogsDir:     filepath.Join(tmpDir, "logs"),
	}

	// Test without token -> should fail with 401
	_, err := config.InstallSource(appCfg, ts.URL, config.InstallOptions{})
	if err == nil {
		t.Fatalf("expected failure without token, got nil")
	}

	// Test with token -> should succeed
	results, err := config.InstallSource(appCfg, ts.URL, config.InstallOptions{
		Token: "secret-token",
	})
	if err != nil {
		t.Fatalf("InstallSource from HTTP failed: %v", err)
	}
	if len(results) != 1 || results[0].Name != "CustomTest" {
		t.Errorf("unexpected results: %+v", results)
	}
}
