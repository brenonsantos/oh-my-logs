package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// FilterPreset represents a named saved filter configuration.
type FilterPreset struct {
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	Filter      string `yaml:"filter" json:"filter"`
}

// FiltersConfig stores saved filter presets and persistent filter history.
type FiltersConfig struct {
	Presets []FilterPreset `yaml:"presets" json:"presets"`
	History []string       `yaml:"history,omitempty" json:"history,omitempty"`
}

// DefaultFilterPresets returns standard starter filter presets.
func DefaultFilterPresets() []FilterPreset {
	return []FilterPreset{
		{
			Name:        "Errors & Critical",
			Description: "Only error, critical, and fatal level records",
			Filter:      "level:err,fatal,crit,error",
		},
		{
			Name:        "Warnings & Above",
			Description: "Warnings, errors, and fatal records",
			Filter:      "level:warn,err,fatal,error,warning",
		},
		{
			Name:        "Exclude Debug/Trace",
			Description: "Silence noisy debug and trace log chatter",
			Filter:      "-debug -trace",
		},
		{
			Name:        "Boot & Init",
			Description: "System startup, bootloader, and init events",
			Filter:      "boot init start ready",
		},
	}
}

// FiltersPath returns the path to ~/.config/oh-my-logs/filters.yaml.
func (c *AppConfig) FiltersPath() string {
	if c == nil || c.ConfigDir == "" {
		return ""
	}
	return filepath.Join(c.ConfigDir, "filters.yaml")
}

// LoadFilters loads filter presets and history from disk.
// If the file does not exist, it initializes with DefaultFilterPresets.
func (c *AppConfig) LoadFilters() (*FiltersConfig, error) {
	fc := &FiltersConfig{
		Presets: DefaultFilterPresets(),
		History: []string{},
	}
	if c == nil || c.ConfigDir == "" {
		return fc, nil
	}

	path := c.FiltersPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			_ = c.SaveFilters(fc)
			return fc, nil
		}
		return fc, err
	}

	if err := yaml.Unmarshal(data, fc); err != nil {
		return nil, fmt.Errorf("parse filters.yaml: %w", err)
	}

	if len(fc.Presets) == 0 {
		fc.Presets = DefaultFilterPresets()
	}

	return fc, nil
}

// SaveFilters saves presets and history to ~/.config/oh-my-logs/filters.yaml atomically.
func (c *AppConfig) SaveFilters(fc *FiltersConfig) error {
	if c == nil || c.ConfigDir == "" || fc == nil {
		return nil
	}

	if err := os.MkdirAll(c.ConfigDir, 0o755); err != nil {
		return err
	}

	data, err := yaml.Marshal(fc)
	if err != nil {
		return fmt.Errorf("marshal filters: %w", err)
	}

	path := c.FiltersPath()
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return err
	}

	return os.Rename(tmpPath, path)
}
