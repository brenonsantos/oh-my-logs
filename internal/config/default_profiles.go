package config

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

//go:embed embedded_profiles/*.yaml
var embeddedProfilesFS embed.FS

// EnsureDefaultProfiles copies any missing built-in starter profiles
// (raw.yaml, logcat.yaml, zephyr.yaml) into targetDir.
// It returns the number of profiles written. Existing profiles are never overwritten.
func EnsureDefaultProfiles(targetDir string) int {
	if targetDir == "" {
		return 0
	}
	_ = os.MkdirAll(targetDir, 0o755)

	entries, err := fs.ReadDir(embeddedProfilesFS, "embedded_profiles")
	if err != nil {
		return 0
	}

	copied := 0
	for _, entry := range entries {
		if entry.IsDir() || (!strings.HasSuffix(entry.Name(), ".yaml") && !strings.HasSuffix(entry.Name(), ".yml")) {
			continue
		}
		dest := filepath.Join(targetDir, entry.Name())
		if _, err := os.Stat(dest); err == nil {
			continue // Do not overwrite existing profile
		}

		data, err := embeddedProfilesFS.ReadFile("embedded_profiles/" + entry.Name())
		if err != nil {
			continue
		}

		if err := os.WriteFile(dest, data, 0o644); err == nil {
			copied++
		}
	}
	return copied
}
