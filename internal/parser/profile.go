package parser

import (
	"fmt"
	"os"

	"github.com/brenoniehues/oh-my-logs/internal/record"
	"gopkg.in/yaml.v3"
)

// ParserConfig describes which parser to use and its parameters.
type ParserConfig struct {
	Type    string `yaml:"type"`    // "raw" or "regex"
	Pattern string `yaml:"pattern"` // only used when Type == "regex"
}

// ColumnConfig describes a single column as defined in a YAML profile.
type ColumnConfig struct {
	Field string `yaml:"field"`
	Title string `yaml:"title"`
	Width int    `yaml:"width"`
}

// Profile is a parsed YAML profile file. It defines how serial output from a
// particular architecture is parsed and displayed.
type Profile struct {
	Name    string         `yaml:"name"`
	Parser  ParserConfig   `yaml:"parser"`
	Columns []ColumnConfig `yaml:"columns"`
}

// LoadProfile reads a YAML file at path and unmarshals it into a Profile.
func LoadProfile(path string) (*Profile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("profile: cannot read %q: %w", path, err)
	}
	var p Profile
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("profile: invalid YAML in %q: %w", path, err)
	}
	return &p, nil
}

// BuildParser instantiates the correct Parser implementation based on the
// profile's parser configuration.
func (p *Profile) BuildParser() (Parser, error) {
	switch p.Parser.Type {
	case "raw", "":
		return NewRawParser(), nil
	case "regex":
		if p.Parser.Pattern == "" {
			return nil, fmt.Errorf("profile %q: regex parser requires a pattern", p.Name)
		}
		return NewRegexParser(p.Parser.Pattern)
	default:
		return nil, fmt.Errorf("profile %q: unknown parser type %q", p.Name, p.Parser.Type)
	}
}

// ToColumns converts the profile's column configs into record.Column values.
func (p *Profile) ToColumns() []record.Column {
	cols := make([]record.Column, len(p.Columns))
	for i, c := range p.Columns {
		cols[i] = record.Column{
			Field: c.Field,
			Title: c.Title,
			Width: c.Width,
		}
	}
	return cols
}
