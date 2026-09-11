package payload

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

var testPalette = ColorPalette{
	Cyan:   lipgloss.Color("#06b6d4"),
	Yellow: lipgloss.Color("#eab308"),
	Green:  lipgloss.Color("#22c55e"),
	Purple: lipgloss.Color("#a855f7"),
	Accent: lipgloss.Color("#3b82f6"),
	Muted:  lipgloss.Color("#64748b"),
	Fg:     lipgloss.Color("#f1f5f9"),
}

func TestDetectAndFormat_JSON(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantType  PayloadType
		wantLabel string
		wantSub   string
	}{
		{
			name:      "simple json object",
			input:     `{"status":"ok","code":200}`,
			wantType:  JSON,
			wantLabel: "JSON",
			wantSub:   `"status": "ok"`,
		},
		{
			name:      "embedded json with prefix",
			input:     `12:00:00 [INF] payload: {"user":"alice","active":true}`,
			wantType:  JSON,
			wantLabel: "JSON",
			wantSub:   `"user": "alice"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			det := DetectAndFormat(tc.input, testPalette)
			if det.Type != tc.wantType {
				t.Fatalf("expected type %v, got %v", tc.wantType, det.Type)
			}
			if det.TypeLabel != tc.wantLabel {
				t.Errorf("expected label %s, got %s", tc.wantLabel, det.TypeLabel)
			}
			if !strings.Contains(det.FormattedText, tc.wantSub) {
				t.Errorf("expected formatted text to contain %q, got:\n%s", tc.wantSub, det.FormattedText)
			}
			if len(det.ColorizedLines) == 0 {
				t.Errorf("expected colorized lines to be non-empty")
			}
		})
	}
}

func TestDetectAndFormat_XML(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantType  PayloadType
		wantLabel string
		wantSub   string
	}{
		{
			name:      "compact leaf xml",
			input:     `<response status="ok"><id>42</id></response>`,
			wantType:  XML,
			wantLabel: "XML",
			wantSub:   `<id>42</id>`,
		},
		{
			name:      "nested xml with attributes",
			input:     `<config env="prod"><server host="127.0.0.1" port="8080"><status>running</status></server></config>`,
			wantType:  XML,
			wantLabel: "XML",
			wantSub:   `host="127.0.0.1"`,
		},
		{
			name:      "xml with comment and prefix",
			input:     `Incoming frame: <packet id="1"><!-- heartbeat --><ack>true</ack></packet>`,
			wantType:  XML,
			wantLabel: "XML",
			wantSub:   `<!-- heartbeat -->`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			det := DetectAndFormat(tc.input, testPalette)
			if det.Type != tc.wantType {
				t.Fatalf("expected type %v, got %v", tc.wantType, det.Type)
			}
			if det.TypeLabel != tc.wantLabel {
				t.Errorf("expected label %s, got %s", tc.wantLabel, det.TypeLabel)
			}
			if !strings.Contains(det.FormattedText, tc.wantSub) {
				t.Errorf("expected formatted text to contain %q, got:\n%s", tc.wantSub, det.FormattedText)
			}
			if len(det.ColorizedLines) == 0 {
				t.Errorf("expected colorized lines to be non-empty")
			}
		})
	}
}

func TestDetectAndFormat_YAML(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantType  PayloadType
		wantLabel string
		wantSub   string
	}{
		{
			name:      "multiline yaml mapping",
			input:     "server:\n  port: 8080\n  host: 0.0.0.0\n  enabled: true",
			wantType:  YAML,
			wantLabel: "YAML",
			wantSub:   "port: 8080",
		},
		{
			name:      "yaml list sequence",
			input:     "- item1\n- item2\n- item3",
			wantType:  YAML,
			wantLabel: "YAML",
			wantSub:   "- item1",
		},
		{
			name:      "escaped newline yaml mapping",
			input:     "config:\\n  database: postgres\\n  pool: 10\\n  ssl: true",
			wantType:  YAML,
			wantLabel: "YAML",
			wantSub:   "database: postgres",
		},
		{
			name:      "prefixed log line with embedded yaml",
			input:     `2026-09-11 11:30:02.180 [INF] config: manifest:\n  version: "1.2.0"\n  environment: production\n  services:\n    telemetry:\n      enabled: true`,
			wantType:  YAML,
			wantLabel: "YAML",
			wantSub:   `version: "1.2.0"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			det := DetectAndFormat(tc.input, testPalette)
			if det.Type != tc.wantType {
				t.Fatalf("expected type %v, got %v", tc.wantType, det.Type)
			}
			if det.TypeLabel != tc.wantLabel {
				t.Errorf("expected label %s, got %s", tc.wantLabel, det.TypeLabel)
			}
			if !strings.Contains(det.FormattedText, tc.wantSub) {
				t.Errorf("expected formatted text to contain %q, got:\n%s", tc.wantSub, det.FormattedText)
			}
			if len(det.ColorizedLines) == 0 {
				t.Errorf("expected colorized lines to be non-empty")
			}
		})
	}
}

func TestDetectAndFormat_Logfmt(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantType   PayloadType
		wantLabel  string
		wantPrefix string
		wantSub    string
	}{
		{
			name:       "pure logfmt",
			input:      `level=info msg="task finished" duration_ms=42.5 success=true`,
			wantType:   Logfmt,
			wantLabel:  "LOGFMT",
			wantPrefix: "",
			wantSub:    `msg`,
		},
		{
			name:       "logfmt with timestamp prefix",
			input:      `2026-09-11 14:30:00 level=warn module=auth user=admin attempts=3`,
			wantType:   Logfmt,
			wantLabel:  "LOGFMT",
			wantPrefix: "2026-09-11 14:30:00",
			wantSub:    `module`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			det := DetectAndFormat(tc.input, testPalette)
			if det.Type != tc.wantType {
				t.Fatalf("expected type %v, got %v", tc.wantType, det.Type)
			}
			if det.TypeLabel != tc.wantLabel {
				t.Errorf("expected label %s, got %s", tc.wantLabel, det.TypeLabel)
			}
			if det.Prefix != tc.wantPrefix {
				t.Errorf("expected prefix %q, got %q", tc.wantPrefix, det.Prefix)
			}
			if !strings.Contains(det.FormattedText, tc.wantSub) {
				t.Errorf("expected formatted text to contain %q, got:\n%s", tc.wantSub, det.FormattedText)
			}
			if len(det.ColorizedLines) == 0 {
				t.Errorf("expected colorized lines to be non-empty")
			}
		})
	}
}

func TestDetectAndFormat_PlainFallback(t *testing.T) {
	plainInputs := []string{
		"Simple plain log message without structured data",
		"error: connection timeout",
		"std::vector<int> buffer overflow",
		"GET /api/v1?user=john&group=dev HTTP/1.1",
		"x = 1 + 2 = 3",
		"short",
		"",
	}

	for _, input := range plainInputs {
		det := DetectAndFormat(input, testPalette)
		if det.Type != None {
			t.Errorf("expected input %q to return None, got %v (%s)", input, det.Type, det.TypeLabel)
		}
	}
}

func TestDetectAndFormatJSON_DirectObject(t *testing.T) {
	raw := `{"sensor":"bme280","temp":24.5,"active":true,"reading":null}`
	res := DetectAndFormatJSON(raw)
	if !res.HasJSON {
		t.Fatalf("expected HasJSON=true, got false")
	}
	if !strings.Contains(res.IndentedJSON, "  \"sensor\": \"bme280\"") {
		t.Errorf("expected indented JSON with 2 spaces, got:\n%s", res.IndentedJSON)
	}
}

func TestDetectAndFormatJSON_Array(t *testing.T) {
	raw := `[1, 2, 3, "four"]`
	res := DetectAndFormatJSON(raw)
	if !res.HasJSON {
		t.Fatalf("expected HasJSON=true, got false")
	}
	if !strings.Contains(res.IndentedJSON, "  1,") {
		t.Errorf("expected indented array, got:\n%s", res.IndentedJSON)
	}
}

func TestDetectAndFormatJSON_Embedded(t *testing.T) {
	raw := `2026-09-11 10:00:00 [INF] event: {"status":"connected","code":200} (done)`
	res := DetectAndFormatJSON(raw)
	if !res.HasJSON {
		t.Fatalf("expected HasJSON=true, got false")
	}
	if res.Prefix != "2026-09-11 10:00:00 [INF] event:" {
		t.Errorf("unexpected prefix: %q", res.Prefix)
	}
	if res.Suffix != "(done)" {
		t.Errorf("unexpected suffix: %q", res.Suffix)
	}
	if !strings.Contains(res.IndentedJSON, "  \"status\": \"connected\"") {
		t.Errorf("expected indented JSON, got:\n%s", res.IndentedJSON)
	}
}

func TestDetectAndFormatJSON_NonJSON(t *testing.T) {
	raw := `booting stm32f411 at 100MHz... ready`
	res := DetectAndFormatJSON(raw)
	if res.HasJSON {
		t.Fatalf("expected HasJSON=false, got true")
	}
}

func TestColorizeJSON(t *testing.T) {
	raw := "{\n  \"key\": \"value\",\n  \"num\": 42,\n  \"flag\": true,\n  \"empty\": null\n}"
	out := ColorizeJSON(raw, testPalette)
	if len(out) == 0 {
		t.Fatal("expected colorized output, got empty")
	}
}

func TestWrapTextLines(t *testing.T) {
	text := "The quick brown fox jumps over the lazy dog."
	wrapped := WrapTextLines(text, 15)
	if len(wrapped) <= 1 {
		t.Fatalf("expected text to wrap across multiple lines, got %d lines: %v", len(wrapped), wrapped)
	}
	for _, l := range wrapped {
		if len(l) > 15 {
			t.Errorf("line exceeded max width 15: %q (len %d)", l, len(l))
		}
	}
}
