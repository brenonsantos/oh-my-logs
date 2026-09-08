package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const appName = "oh-my-logs"

// AppConfig holds global application configuration.
type AppConfig struct {
	// ConfigDir is the OS-appropriate configuration directory.
	ConfigDir string
	// ProfilesDir is the directory where profile YAML files are stored.
	ProfilesDir string
	// LogsDir is the default directory for saved logs.
	LogsDir string
}

// Load determines configuration paths and ensures they exist.
func Load() (*AppConfig, error) {
	dir, err := configDir()
	if err != nil {
		return nil, fmt.Errorf("config: cannot determine config dir: %w", err)
	}

	cfg := &AppConfig{
		ConfigDir:   dir,
		ProfilesDir: filepath.Join(dir, "profiles"),
		LogsDir:     filepath.Join(dir, "logs"),
	}

	// Ensure directories exist.
	for _, d := range []string{cfg.ProfilesDir, cfg.LogsDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return nil, fmt.Errorf("config: cannot create directory %q: %w", d, err)
		}
	}

	return cfg, nil
}

// configDir returns the OS-appropriate configuration directory for the app.
// On Linux/macOS this follows XDG / os.UserConfigDir conventions.
// On Windows it falls back to APPDATA.
func configDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		// Last resort: ~/.oh-my-logs
		home, herr := os.UserHomeDir()
		if herr != nil {
			return "", fmt.Errorf("cannot find home dir: %w", herr)
		}
		return filepath.Join(home, "."+appName), nil
	}
	return filepath.Join(base, appName), nil
}

// ListProfileFiles returns all .yaml / .yml files in ProfilesDir.
func (c *AppConfig) ListProfileFiles() ([]string, error) {
	entries, err := os.ReadDir(c.ProfilesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var paths []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml") {
			paths = append(paths, filepath.Join(c.ProfilesDir, name))
		}
	}
	return paths, nil
}

// FindAllProfilePaths discovers all YAML profile files across the OS config
// directory and the current working directory's profiles/ hierarchy.
func FindAllProfilePaths(c *AppConfig) []string {
	seen := make(map[string]bool)
	var paths []string

	addFile := func(path string) {
		abs, err := filepath.Abs(path)
		if err != nil {
			abs = path
		}
		if !seen[abs] {
			seen[abs] = true
			paths = append(paths, path)
		}
	}

	// 1. AppConfig profiles dir
	if c != nil {
		if cfgFiles, err := c.ListProfileFiles(); err == nil {
			for _, f := range cfgFiles {
				addFile(f)
			}
		}
	}

	// 2. Local profiles and subdirectories
	localDirs := []string{"profiles", filepath.Join("profiles", "examples")}
	for _, dir := range localDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml") {
				addFile(filepath.Join(dir, name))
			}
		}
	}

	return paths
}

