package config

import (
	"os"
	"path/filepath"

	"github.com/brenoniehues/oh-my-logs/internal/decoder"
	"gopkg.in/yaml.v3"
)

// ProjectConfig represents a repository-local .oml.yaml configuration.
type ProjectConfig struct {
	Profile     string           `yaml:"profile,omitempty"`
	Port        string           `yaml:"port,omitempty"`
	Baud        int              `yaml:"baud,omitempty"`
	StripPrefix string           `yaml:"strip_prefix,omitempty"`
	Decoders    []decoder.Config `yaml:"decoders,omitempty"`

	// ConfigPath stores the absolute path to the loaded configuration file.
	ConfigPath string `yaml:"-"`
}

// FindProjectConfig searches for a project configuration starting from startDir
// and ascending up parent directories until a git root (.git) or filesystem root is reached.
// Candidate file names: ".oml.yaml", ".oml.yml", ".oml/config.yaml", ".oml/config.yml".
func FindProjectConfig(startDir string) (*ProjectConfig, error) {
	if startDir == "" {
		var err error
		startDir, err = os.Getwd()
		if err != nil {
			return nil, err
		}
	}

	absDir, err := filepath.Abs(startDir)
	if err != nil {
		absDir = startDir
	}

	candidates := []string{
		".oml.yaml",
		".oml.yml",
		filepath.Join(".oml", "config.yaml"),
		filepath.Join(".oml", "config.yml"),
	}

	curr := absDir
	for {
		for _, cand := range candidates {
			p := filepath.Join(curr, cand)
			if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
				data, err := os.ReadFile(p)
				if err != nil {
					return nil, err
				}
				var proj ProjectConfig
				if err := yaml.Unmarshal(data, &proj); err != nil {
					return nil, err
				}
				proj.ConfigPath = p
				return &proj, nil
			}
		}

		// If at git root, stop walking up
		gitDir := filepath.Join(curr, ".git")
		if fi, err := os.Stat(gitDir); err == nil && fi.IsDir() {
			break
		}

		parent := filepath.Dir(curr)
		if parent == curr {
			// Reached root of filesystem
			break
		}
		curr = parent
	}

	return nil, nil
}
