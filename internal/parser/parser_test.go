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

func TestRegexParser_EmptyPatterns(t *testing.T) {
	if _, err := parser.NewRegexParser(); err == nil {
		t.Error("expected error when no patterns provided")
	}
	if _, err := parser.NewRegexParser("", ""); err == nil {
		t.Error("expected error when only empty patterns provided")
	}
}

func TestRegexParser_MultiplePatterns(t *testing.T) {
	p, err := parser.NewRegexParser(
		`^(?P<time>\d{2}:\d{2})\s+\[(?P<level>\w+)\]\s+(?P<message>.*)$`,
		`^(?P<level>\w+):(?P<time>\d{2}:\d{2}):(?P<message>.*)$`,
	)
	if err != nil {
		t.Fatalf("NewRegexParser error: %v", err)
	}

	// First pattern match
	r1, err := p.Parse("12:30 [INFO] First message")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r1.Fields["time"] != "12:30" || r1.Fields["level"] != "INFO" || r1.Fields["message"] != "First message" {
		t.Errorf("r1 parsed incorrectly: %+v", r1.Fields)
	}

	// Fallback pattern match
	r2, err := p.Parse("WARN:12:31:Second message")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r2.Fields["time"] != "12:31" || r2.Fields["level"] != "WARN" || r2.Fields["message"] != "Second message" {
		t.Errorf("r2 parsed incorrectly: %+v", r2.Fields)
	}

	// Neither matches
	r3, err := p.Parse("Just an unformatted line")
	if err == nil {
		t.Error("expected error for unmatched line")
	}
	if r3.Fields["message"] != "Just an unformatted line" {
		t.Errorf("expected fallback message, got %q", r3.Fields["message"])
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

const logcatTestProfileYAML = `
name: Logcat
parser:
  type: regex
  patterns:
    # 1. threadtime format: [MM-DD ]HH:MM:SS.mmm PID TID Level Tag: Message
    - '^(?:(?P<time>(?:\d{2}-\d{2}\s+)?\d{2}:\d{2}:\d{2}\.\d{3}))\s+(?P<pid>\d+)\s+(?P<tid>\d+)\s+(?P<level>[VDIWEAFvdiweaf])\s+(?:(?P<tag>[^:\r\n]+?):\s+)?(?P<message>.*)$'
    # 2. time format: [MM-DD ]HH:MM:SS.mmm Level/Tag(PID): Message
    - '^(?:(?P<time>(?:\d{2}-\d{2}\s+)?\d{2}:\d{2}:\d{2}\.\d{3}))\s+(?P<level>[VDIWEAFvdiweaf])/(?P<tag>.+?)\(\s*(?P<pid>\d+)\):\s*(?P<message>.*)$'
columns:
  - field: time
    title: Time
    width: 18
    style: timestamp
  - field: pid
    title: PID
    width: 6
    style: muted
  - field: tid
    title: TID
    width: 6
    style: muted
  - field: level
    title: Lvl
    width: 5
    style: level
  - field: tag
    title: Tag
    width: 20
    style: identifier
  - field: message
    title: Message
    width: 0
    style: primary
`

func TestLoadProfile_Logcat(t *testing.T) {
	path := writeTemp(t, logcatTestProfileYAML)
	p, err := parser.LoadProfile(path)
	if err != nil {
		t.Fatalf("failed to load logcat profile: %v", err)
	}
	if p.Name != "Logcat" {
		t.Errorf("expected profile name Logcat, got %q", p.Name)
	}
	bp, err := p.BuildParser()
	if err != nil {
		t.Fatalf("failed to build parser: %v", err)
	}

	testCases := []struct {
		line    string
		time    string
		pid     string
		tid     string
		level   string
		tag     string
		message string
	}{
		{
			line:    "08-10 05:34:48.669   559   559 I SystemServerTiming: Initializing system service",
			time:    "08-10 05:34:48.669",
			pid:     "559",
			tid:     "559",
			level:   "I",
			tag:     "SystemServerTiming",
			message: "Initializing system service",
		},
		{
			line:    "08-10 12:00:00.123  1000  1001 D WifiService: Connected to network",
			time:    "08-10 12:00:00.123",
			pid:     "1000",
			tid:     "1001",
			level:   "D",
			tag:     "WifiService",
			message: "Connected to network",
		},
		{
			line:    "05:34:48.670   559   620 E AudioFlinger: cannot open hw device",
			time:    "05:34:48.670",
			pid:     "559",
			tid:     "620",
			level:   "E",
			tag:     "AudioFlinger",
			message: "cannot open hw device",
		},
		{
			line:    "12:34:56.789  1234  1234 W ActivityManager: Slow delivery of broadcast",
			time:    "12:34:56.789",
			pid:     "1234",
			tid:     "1234",
			level:   "W",
			tag:     "ActivityManager",
			message: "Slow delivery of broadcast",
		},
		{
			line:    "01-01 00:00:01.000   100   100 V BatteryService: level=100 scale=100",
			time:    "01-01 00:00:01.000",
			pid:     "100",
			tid:     "100",
			level:   "V",
			tag:     "BatteryService",
			message: "level=100 scale=100",
		},
		// -v time format cases: [MM-DD ]HH:MM:SS.mmm Level/Tag(PID): Message
		{
			line:    "10-23 15:19:23.255 I/SystemServerTiming(  546): OnBootPhase_550_com.android.server.UiModeManagerService",
			time:    "10-23 15:19:23.255",
			pid:     "546",
			tid:     "",
			level:   "I",
			tag:     "SystemServerTiming",
			message: "OnBootPhase_550_com.android.server.UiModeManagerService",
		},
		{
			line:    "09-09 13:37:12.574 D/WifiService( 1000): Connected to network",
			time:    "09-09 13:37:12.574",
			pid:     "1000",
			tid:     "",
			level:   "D",
			tag:     "WifiService",
			message: "Connected to network",
		},
		{
			line:    "10-23 15:19:23.276 W/WallpaperManagerService(  546): Invalid wallpaper data",
			time:    "10-23 15:19:23.276",
			pid:     "546",
			tid:     "",
			level:   "W",
			tag:     "WallpaperManagerService",
			message: "Invalid wallpaper data",
		},
		{
			line:    "05:34:48.670 E/AudioFlinger(  559): cannot open hw device",
			time:    "05:34:48.670",
			pid:     "559",
			tid:     "",
			level:   "E",
			tag:     "AudioFlinger",
			message: "cannot open hw device",
		},
		{
			line:    "08-10 05:34:48.670 F/libc(  123): Fatal signal 11 (SIGSEGV)",
			time:    "08-10 05:34:48.670",
			pid:     "123",
			tid:     "",
			level:   "F",
			tag:     "libc",
			message: "Fatal signal 11 (SIGSEGV)",
		},
		{
			line:    "10-23 15:19:23.457 D/UsbHostManager: Sub(  546): USB endpoint registered",
			time:    "10-23 15:19:23.457",
			pid:     "546",
			tid:     "",
			level:   "D",
			tag:     "UsbHostManager: Sub",
			message: "USB endpoint registered",
		},
	}

	for _, tc := range testCases {
		r, err := bp.Parse(tc.line)
		if err != nil {
			t.Fatalf("failed to parse Logcat line %q: %v", tc.line, err)
		}
		if r.Fields["time"] != tc.time {
			t.Errorf("expected time %q, got %q", tc.time, r.Fields["time"])
		}
		if r.Fields["pid"] != tc.pid {
			t.Errorf("expected pid %q, got %q", tc.pid, r.Fields["pid"])
		}
		if r.Fields["tid"] != tc.tid {
			t.Errorf("expected tid %q, got %q", tc.tid, r.Fields["tid"])
		}
		if r.Fields["level"] != tc.level {
			t.Errorf("expected level %q, got %q", tc.level, r.Fields["level"])
		}
		if r.Fields["tag"] != tc.tag {
			t.Errorf("expected tag %q, got %q", tc.tag, r.Fields["tag"])
		}
		if r.Fields["message"] != tc.message {
			t.Errorf("expected message %q, got %q", tc.message, r.Fields["message"])
		}
	}

	// Non-matching logcat line (e.g. system banner) fallbacks to raw message
	banner := "--------- beginning of system"
	rUnmatched, err := bp.Parse(banner)
	if err == nil {
		t.Errorf("expected error for non-matching banner line")
	}
	if rUnmatched.Fields["message"] != banner {
		t.Errorf("expected non-matching banner to fallback to message, got %q", rUnmatched.Fields["message"])
	}
}

