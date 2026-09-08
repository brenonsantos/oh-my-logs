package parser_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brenoniehues/oh-my-logs/internal/parser"
)

// ── RawParser ────────────────────────────────────────────────────────────────

func TestRawParser_Normal(t *testing.T) {
	p := parser.NewRawParser()
	r, err := p.Parse("Hello world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Fields["message"] != "Hello world" {
		t.Errorf("message = %q, want %q", r.Fields["message"], "Hello world")
	}
	if r.Raw != "Hello world" {
		t.Errorf("raw = %q, want %q", r.Raw, "Hello world")
	}
}

func TestRawParser_Empty(t *testing.T) {
	p := parser.NewRawParser()
	r, err := p.Parse("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Fields["message"] != "" {
		t.Errorf("message = %q, want empty", r.Fields["message"])
	}
}

func TestRawParser_SpecialChars(t *testing.T) {
	p := parser.NewRawParser()
	line := "\x00\xff invalid UTF-8ish \t\n"
	r, err := p.Parse(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Fields["message"] != line {
		t.Error("special chars not preserved")
	}
}

// ── RegexParser ───────────────────────────────────────────────────────────────

const stm32Pattern = `^\[(?P<time>[^\]]+)\]\[(?P<level>[^\]]+)\]\[(?P<module>[^\]]+)\]\s+(?P<message>.*)$`

func TestRegexParser_ValidMatch(t *testing.T) {
	p, err := parser.NewRegexParser(stm32Pattern)
	if err != nil {
		t.Fatalf("NewRegexParser error: %v", err)
	}
	r, err := p.Parse("[15:42:31.102][INFO][SYS] System initialized")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	cases := map[string]string{
		"time":    "15:42:31.102",
		"level":   "INFO",
		"module":  "SYS",
		"message": "System initialized",
	}
	for field, want := range cases {
		if got := r.Fields[field]; got != want {
			t.Errorf("field %q = %q, want %q", field, got, want)
		}
	}
}

func TestRegexParser_NoMatch(t *testing.T) {
	p, err := parser.NewRegexParser(stm32Pattern)
	if err != nil {
		t.Fatalf("NewRegexParser error: %v", err)
	}
	r, err := p.Parse("this line does not match")
	if err == nil {
		t.Fatal("expected non-nil error for non-matching line")
	}
	if r.Fields["_raw"] != "this line does not match" {
		t.Errorf("_raw = %q, want original line", r.Fields["_raw"])
	}
}

func TestRegexParser_EmptyNamedGroup(t *testing.T) {
	// Pattern where 'level' group can be empty.
	p, err := parser.NewRegexParser(`^(?P<time>\d+):(?P<level>[^:]*)$`)
	if err != nil {
		t.Fatalf("NewRegexParser error: %v", err)
	}
	r, err := p.Parse("123:")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Fields["time"] != "123" {
		t.Errorf("time = %q, want 123", r.Fields["time"])
	}
	if _, ok := r.Fields["level"]; !ok {
		t.Error("level field should be present even if empty")
	}
}

func TestRegexParser_InvalidPattern(t *testing.T) {
	_, err := parser.NewRegexParser(`(unclosed`)
	if err == nil {
		t.Fatal("expected error for invalid pattern")
	}
}

// ── Profile loading ───────────────────────────────────────────────────────────

const validProfileYAML = `
name: STM32-PDM
parser:
  type: regex
  pattern: '^\[(?P<time>[^\]]+)\]\[(?P<level>[^\]]+)\]\[(?P<module>[^\]]+)\]\s+(?P<message>.*)$'
columns:
  - field: time
    title: Time
    width: 14
  - field: level
    title: Level
    width: 8
  - field: module
    title: Module
    width: 10
  - field: message
    title: Message
    width: 0
`

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "profile.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

func TestLoadProfile_Valid(t *testing.T) {
	path := writeTemp(t, validProfileYAML)
	p, err := parser.LoadProfile(path)
	if err != nil {
		t.Fatalf("LoadProfile error: %v", err)
	}
	if p.Name != "STM32-PDM" {
		t.Errorf("Name = %q, want STM32-PDM", p.Name)
	}
	if len(p.Columns) != 4 {
		t.Errorf("Columns count = %d, want 4", len(p.Columns))
	}
	parser_, err := p.BuildParser()
	if err != nil {
		t.Fatalf("BuildParser error: %v", err)
	}
	r, err := parser_.Parse("[15:42:31.102][INFO][SYS] System initialized")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if r.Fields["level"] != "INFO" {
		t.Errorf("level = %q, want INFO", r.Fields["level"])
	}
}

func TestLoadProfile_InvalidYAML(t *testing.T) {
	path := writeTemp(t, "{{invalid yaml{{")
	_, err := parser.LoadProfile(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoadProfile_InvalidRegex(t *testing.T) {
	path := writeTemp(t, `
name: Bad
parser:
  type: regex
  pattern: '(unclosed'
columns: []
`)
	p, err := parser.LoadProfile(path)
	if err != nil {
		t.Fatalf("LoadProfile error: %v", err)
	}
	_, err = p.BuildParser()
	if err == nil {
		t.Fatal("expected error for invalid regex")
	}
}

func TestLoadProfile_UnknownType(t *testing.T) {
	path := writeTemp(t, `
name: Bad
parser:
  type: json
columns: []
`)
	p, err := parser.LoadProfile(path)
	if err != nil {
		t.Fatalf("LoadProfile error: %v", err)
	}
	_, err = p.BuildParser()
	if err == nil {
		t.Fatal("expected error for unknown parser type")
	}
}

func TestLoadProfile_RawType(t *testing.T) {
	path := writeTemp(t, `
name: Raw
parser:
  type: raw
columns:
  - field: message
    title: Message
    width: 0
`)
	p, err := parser.LoadProfile(path)
	if err != nil {
		t.Fatalf("LoadProfile error: %v", err)
	}
	parser_, err := p.BuildParser()
	if err != nil {
		t.Fatalf("BuildParser error: %v", err)
	}
	r, _ := parser_.Parse("hello")
	if r.Fields["message"] != "hello" {
		t.Errorf("message = %q, want hello", r.Fields["message"])
	}
}

func TestLoadProfile_Zephyr(t *testing.T) {
	path := filepath.Join("..", "..", "profiles", "examples", "zephyr.yaml")
	p, err := parser.LoadProfile(path)
	if err != nil {
		t.Fatalf("failed to load zephyr.yaml: %v", err)
	}
	if p.Name != "Zephyr" {
		t.Errorf("expected profile name Zephyr, got %q", p.Name)
	}
	bp, err := p.BuildParser()
	if err != nil {
		t.Fatalf("failed to build parser: %v", err)
	}

	// 1. Test standard Zephyr log line from Zcore_Init.log
	r, err := bp.Parse("[      5.182] <inf> fs_nvs: 16 Sectors of 4096 bytes")
	if err != nil {
		t.Fatalf("failed to parse Zephyr log line: %v", err)
	}
	if r.Fields["uptime"] != "5.182" {
		t.Errorf("expected uptime 5.182, got %q", r.Fields["uptime"])
	}
	if r.Fields["level"] != "inf" {
		t.Errorf("expected level inf, got %q", r.Fields["level"])
	}
	if r.Fields["message"] != "fs_nvs: 16 Sectors of 4096 bytes" {
		t.Errorf("expected message 'fs_nvs: 16 Sectors of 4096 bytes', got %q", r.Fields["message"])
	}

	// 2. Test non-matching boot line fallback to message
	rUnmatched, err := bp.Parse("*** Booting Zephyr OS build nxp-v4.1.0-48847-ge5a841432a62 ***")
	if err == nil {
		t.Errorf("expected error for non-matching line")
	}
	if rUnmatched.Fields["message"] != "*** Booting Zephyr OS build nxp-v4.1.0-48847-ge5a841432a62 ***" {
		t.Errorf("expected non-matching line to fallback to message, got %q", rUnmatched.Fields["message"])
	}
}

func TestParse_ZcoreInitLog(t *testing.T) {
	path := filepath.Join("..", "..", "profiles", "examples", "zephyr.yaml")
	p, err := parser.LoadProfile(path)
	if err != nil {
		t.Fatalf("LoadProfile error: %v", err)
	}
	bp, err := p.BuildParser()
	if err != nil {
		t.Fatalf("BuildParser error: %v", err)
	}

	logPath := filepath.Join("..", "..", "Zcore_Init.log")
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Skip("Zcore_Init.log not found, skipping")
	}

	lines := strings.Split(string(data), "\n")
	matched := 0
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			continue
		}
		r, err := bp.Parse(l)
		if err == nil {
			matched++
			if r.Fields["uptime"] == "" || r.Fields["level"] == "" {
				t.Errorf("expected uptime and level to be populated on match: %q", l)
			}
		} else {
			if r.Fields["message"] != l {
				t.Errorf("expected unmatched line to have message = raw line: %q", l)
			}
		}
	}
	if matched < 350 {
		t.Errorf("expected at least 350 matched lines, got %d", matched)
	}
}
