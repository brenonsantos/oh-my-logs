package parser

import (
	"fmt"
	"os"
	"time"

	"github.com/brenoniehues/oh-my-logs/internal/record"
	"github.com/brenoniehues/oh-my-logs/internal/timing"
	"gopkg.in/yaml.v3"
)

// ParserConfig describes which parser to use and its parameters.
type ParserConfig struct {
	Type     string   `yaml:"type"`               // "raw" or "regex"
	Pattern  string   `yaml:"pattern,omitempty"`  // single regex pattern (when Type == "regex")
	Patterns []string `yaml:"patterns,omitempty"` // ordered list of fallback regex patterns (when Type == "regex")
}

// ColumnConfig describes a single column as defined in a YAML profile.
type ColumnConfig struct {
	Field  string            `yaml:"field"`
	Title  string            `yaml:"title"`
	Width  int               `yaml:"width"`
	Style  string            `yaml:"style"`
	Colors map[string]string `yaml:"colors"`
}


// IngestTimestamp, when enabled, stamps every received record with the system
// clock time at which the line arrived. Useful for architectures that do not
// include a timestamp in their serial output.
type IngestTimestamp struct {
	Enabled bool   `yaml:"enabled"`
	Field   string `yaml:"field"`  // field name to inject; defaults to "_ts"
	Format  string `yaml:"format"` // Go time layout;  defaults to "15:04:05.000"
}

// TimestampField returns the field name to use, falling back to "_ts".
func (t IngestTimestamp) TimestampField() string {
	if t.Field == "" {
		return "_ts"
	}
	return t.Field
}

// TimestampFormat returns the Go time format string, falling back to millisecond time.
func (t IngestTimestamp) TimestampFormat() string {
	if t.Format == "" {
		return "15:04:05.000"
	}
	return t.Format
}

// IngestConfig controls what the pipeline automatically adds to every record,
// independent of what the parser extracts from the raw line.
type IngestConfig struct {
	Timestamp IngestTimestamp `yaml:"timestamp"`
}

// TimingConfig defines profile-level latency thresholds and moving-average parameters.
type TimingConfig struct {
	WarnThreshold  string  `yaml:"warn_threshold"`  // e.g. "50ms", "100ms"
	AlertThreshold string  `yaml:"alert_threshold"` // e.g. "200ms", "1s"
	WarnRatio      float64 `yaml:"warn_ratio"`      // dynamic ratio override (e.g. 3.0)
	AlertRatio     float64 `yaml:"alert_ratio"`     // dynamic ratio override (e.g. 6.0)
}

// Profile is a parsed YAML profile file. It defines how serial output from a
// particular architecture is parsed and displayed.
type Profile struct {
	Name    string         `yaml:"name"`
	Parser  ParserConfig   `yaml:"parser"`
	Columns []ColumnConfig `yaml:"columns"`
	Ingest  IngestConfig   `yaml:"ingest"`
	Timing  TimingConfig   `yaml:"timing"`
}

// TimingParameters converts TimingConfig to timing.Config, parsing duration strings.
func (p *Profile) TimingParameters() timing.Config {
	cfg := timing.DefaultConfig()
	if p == nil {
		return cfg
	}
	if p.Timing.WarnRatio > 0 {
		cfg.WarnRatio = p.Timing.WarnRatio
	}
	if p.Timing.AlertRatio > 0 {
		cfg.AlertRatio = p.Timing.AlertRatio
	}
	if p.Timing.WarnThreshold != "" {
		if d, err := time.ParseDuration(p.Timing.WarnThreshold); err == nil {
			cfg.WarnThreshold = d
		}
	}
	if p.Timing.AlertThreshold != "" {
		if d, err := time.ParseDuration(p.Timing.AlertThreshold); err == nil {
			cfg.AlertThreshold = d
		}
	}
	return cfg
}


// ParseProfile unmarshals raw YAML bytes into a Profile.
func ParseProfile(data []byte) (*Profile, error) {
	var p Profile
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("profile: invalid YAML: %w", err)
	}
	return &p, nil
}

// LoadProfile reads a YAML file at path and unmarshals it into a Profile.
func LoadProfile(path string) (*Profile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("profile: cannot read %q: %w", path, err)
	}
	p, err := ParseProfile(data)
	if err != nil {
		return nil, fmt.Errorf("profile: invalid YAML in %q: %w", path, err)
	}
	return p, nil
}

// BuildParser instantiates the correct Parser implementation based on the
// profile's parser configuration.
func (p *Profile) BuildParser() (Parser, error) {
	switch p.Parser.Type {
	case "raw", "":
		return NewRawParser(), nil
	case "regex":
		var patterns []string
		if len(p.Parser.Patterns) > 0 {
			patterns = append(patterns, p.Parser.Patterns...)
		} else if p.Parser.Pattern != "" {
			patterns = append(patterns, p.Parser.Pattern)
		}
		if len(patterns) == 0 {
			return nil, fmt.Errorf("profile %q: regex parser requires a pattern or patterns", p.Name)
		}
		return NewRegexParser(patterns...)
	default:
		return nil, fmt.Errorf("profile %q: unknown parser type %q", p.Name, p.Parser.Type)
	}
}

// ToColumns converts the profile's column configs into record.Column values.
func (p *Profile) ToColumns() []record.Column {
	cols := make([]record.Column, len(p.Columns))
	for i, c := range p.Columns {
		cols[i] = record.Column{
			Field:  c.Field,
			Title:  c.Title,
			Width:  c.Width,
			Style:  c.Style,
			Colors: c.Colors,
		}
	}
	return cols
}

