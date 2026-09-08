package parser_test

import (
	"os"
	"path/filepath"
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

const zephyrTestProfileYAML = `
name: Zephyr
parser:
  type: regex
  pattern: '^\[\s*(?P<uptime>[^\]]+?)\s*\]\s+<(?P<level>[a-zA-Z]+)>\s+(?:(?P<module>[a-zA-Z0-9_.-]+):\s+)?(?P<message>.*)$'
columns:
  - field: uptime
    title: Uptime
    width: 19
    style: uptime
  - field: level
    title: Level
    width: 7
    style: level
    colors:
      err: red
      wrn: yellow
      inf: cyan
      dbg: gray
  - field: module
    title: Module
    width: 16
    style: identifier
  - field: message
    title: Message
    width: 0
    style: primary
`

func TestLoadProfile_Zephyr(t *testing.T) {
	path := writeTemp(t, zephyrTestProfileYAML)
	p, err := parser.LoadProfile(path)
	if err != nil {
		t.Fatalf("failed to load zephyr profile: %v", err)
	}
	if p.Name != "Zephyr" {
		t.Errorf("expected profile name Zephyr, got %q", p.Name)
	}
	bp, err := p.BuildParser()
	if err != nil {
		t.Fatalf("failed to build parser: %v", err)
	}

	// 1. Test standard generic Zephyr log lines (both formatted timestamp and seconds format)
	testCases := []struct {
		line    string
		uptime  string
		level   string
		module  string
		message string
	}{
		{
			line:    "[00:00:03.165,977] <err> ext_log_system: critical level log",
			uptime:  "00:00:03.165,977",
			level:   "err",
			module:  "ext_log_system",
			message: "critical level log",
		},
		{
			line:    "[      5.182] <inf> fs_nvs: 16 Sectors of 4096 bytes",
			uptime:  "5.182",
			level:   "inf",
			module:  "fs_nvs",
			message: "16 Sectors of 4096 bytes",
		},
		{
			line:    "[      5.200] <wrn> net_core: Network interface initialization timed out",
			uptime:  "5.200",
			level:   "wrn",
			module:  "net_core",
			message: "Network interface initialization timed out",
		},
		{
			line:    "[00:00:03.166,044] <inf> Booting without module",
			uptime:  "00:00:03.166,044",
			level:   "inf",
			module:  "",
			message: "Booting without module",
		},
	}

	for _, tc := range testCases {
		r, err := bp.Parse(tc.line)
		if err != nil {
			t.Fatalf("failed to parse Zephyr line %q: %v", tc.line, err)
		}
		if r.Fields["uptime"] != tc.uptime {
			t.Errorf("expected uptime %q, got %q", tc.uptime, r.Fields["uptime"])
		}
		if r.Fields["level"] != tc.level {
			t.Errorf("expected level %q, got %q", tc.level, r.Fields["level"])
		}
		if r.Fields["module"] != tc.module {
			t.Errorf("expected module %q, got %q", tc.module, r.Fields["module"])
		}
		if r.Fields["message"] != tc.message {
			t.Errorf("expected message %q, got %q", tc.message, r.Fields["message"])
		}
	}

	// 2. Test non-matching boot line fallback to message
	bootLine := "*** Booting Zephyr OS build v3.5.0 ***"
	rUnmatched, err := bp.Parse(bootLine)
	if err == nil {
		t.Errorf("expected error for non-matching line")
	}
	if rUnmatched.Fields["message"] != bootLine {
		t.Errorf("expected non-matching line to fallback to message, got %q", rUnmatched.Fields["message"])
	}
}
