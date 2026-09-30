package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/brenoniehues/oh-my-logs/internal/config"
	"github.com/brenoniehues/oh-my-logs/internal/parser"
)

func TestEnsureDefaultProfiles(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. Initial seeding into empty directory
	copied := config.EnsureDefaultProfiles(tmpDir)
	if copied != 3 {
		t.Fatalf("expected 3 default profiles seeded into empty dir, got %d", copied)
	}

	expectedProfiles := []string{"raw.yaml", "logcat.yaml", "zephyr.yaml"}
	for _, name := range expectedProfiles {
		p := filepath.Join(tmpDir, name)
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("expected seeded profile %s to exist: %v", name, err)
		}
		prof, err := parser.ParseProfile(data)
		if err != nil {
			t.Fatalf("seeded profile %s failed to parse: %v", name, err)
		}
		if prof.Name == "" {
			t.Fatalf("seeded profile %s has empty name", name)
		}
	}

	// 2. Second call should not overwrite existing files
	secondCopied := config.EnsureDefaultProfiles(tmpDir)
	if secondCopied != 0 {
		t.Fatalf("expected 0 profiles copied on second call, got %d", secondCopied)
	}

	// 3. Modifying a profile should be preserved
	zephyrPath := filepath.Join(tmpDir, "zephyr.yaml")
	customContent := []byte("name: CustomZephyr\nversion: 9.9.9\n")
	if err := os.WriteFile(zephyrPath, customContent, 0o644); err != nil {
		t.Fatalf("failed to write custom profile: %v", err)
	}

	thirdCopied := config.EnsureDefaultProfiles(tmpDir)
	if thirdCopied != 0 {
		t.Fatalf("expected 0 profiles copied when custom file exists, got %d", thirdCopied)
	}

	dataAfter, _ := os.ReadFile(zephyrPath)
	if string(dataAfter) != string(customContent) {
		t.Fatalf("expected existing custom profile to be preserved, got %q", string(dataAfter))
	}

	// 4. Deleting one profile should re-seed only the missing profile
	if err := os.Remove(filepath.Join(tmpDir, "logcat.yaml")); err != nil {
		t.Fatalf("failed to remove logcat.yaml: %v", err)
	}

	reseedCopied := config.EnsureDefaultProfiles(tmpDir)
	if reseedCopied != 1 {
		t.Fatalf("expected 1 profile reseeded after deletion, got %d", reseedCopied)
	}

	if _, err := os.Stat(filepath.Join(tmpDir, "logcat.yaml")); err != nil {
		t.Fatalf("expected logcat.yaml to be restored: %v", err)
	}
}
