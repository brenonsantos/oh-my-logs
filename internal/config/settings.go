package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/brenoniehues/oh-my-logs/internal/parser"
)

// Settings stores user configuration that persists across application restarts.
type Settings struct {
	Port           string   `json:"port,omitempty"`
	Baud           int      `json:"baud,omitempty"`
	Profile        string   `json:"profile,omitempty"`
	ShowTimestamp  bool     `json:"show_timestamp"`
	TimestampMode  string   `json:"timestamp_mode,omitempty"`
	TXEnding       string   `json:"tx_ending,omitempty"`
	TXHistory      []string `json:"tx_history,omitempty"`
	BufferCapacity int      `json:"buffer_capacity,omitempty"`
	DefaultFollow  bool     `json:"default_follow"`
	DirectToDisk   bool     `json:"direct_to_disk"`
	LogDir         string   `json:"log_dir,omitempty"`
	LogPrefix      string   `json:"log_prefix,omitempty"`
	Theme          string   `json:"theme,omitempty"`
	DisplayFormat  string   `json:"display_format,omitempty"`
}

// SettingsPath returns the absolute path to settings.json in the config directory.
func (c *AppConfig) SettingsPath() string {
	if c == nil {
		return ""
	}
	return filepath.Join(c.ConfigDir, "settings.json")
}

// LoadSettings reads saved settings from disk. If the file does not exist,
// it returns a default Settings struct with Baud set to 115200 and BufferCapacity set to 50000.
func (c *AppConfig) LoadSettings() (*Settings, error) {
	s := &Settings{
		Baud:           115200,
		BufferCapacity: 50000,
		DefaultFollow:  true,
		Theme:          "dark-slate",
	}
	if c == nil || c.ConfigDir == "" {
		return s, nil
	}

	path := c.SettingsPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return s, err
	}

	if err := json.Unmarshal(data, s); err != nil {
		return s, err
	}

	if s.Baud <= 0 {
		s.Baud = 115200
	}
	if s.BufferCapacity <= 0 {
		s.BufferCapacity = 50000
	}
	if s.Theme == "" {
		s.Theme = "dark-slate"
	}
	return s, nil
}

// SaveSettings writes the settings struct to disk atomically.
func (c *AppConfig) SaveSettings(s *Settings) error {
	if c == nil || c.ConfigDir == "" || s == nil {
		return nil
	}

	if err := os.MkdirAll(c.ConfigDir, 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	path := c.SettingsPath()
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return err
	}

	return os.Rename(tmpPath, path)
}

// ResolveProfilePath looks up a profile by name or file path across all discovered profile locations.
func ResolveProfilePath(appCfg *AppConfig, nameOrPath string) string {
	if nameOrPath == "" || strings.EqualFold(nameOrPath, "raw") {
		return ""
	}

	// 1. Direct path check
	if _, err := os.Stat(nameOrPath); err == nil {
		return nameOrPath
	}

	// 2. Search all discovered profile paths
	candidates := FindAllProfilePaths(appCfg)
	for _, p := range candidates {
		// Check by exact filename without extension
		base := strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
		if strings.EqualFold(base, nameOrPath) {
			return p
		}

		// Check by loaded profile Name
		prof, err := parser.LoadProfile(p)
		if err != nil {
			continue
		}
		if strings.EqualFold(prof.Name, nameOrPath) {
			return p
		}
	}

	return ""
}
