package tui

import (
	"strings"
	"testing"
)

func TestDetectAndFormatPayload_JSON(t *testing.T) {
	p := PaletteDarkSlate

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
			wantType:  PayloadJSON,
			wantLabel: "JSON",
			wantSub:   `"status": "ok"`,
		},
		{
			name:      "embedded json with prefix",
			input:     `12:00:00 [INF] payload: {"user":"alice","active":true}`,
			wantType:  PayloadJSON,
			wantLabel: "JSON",
			wantSub:   `"user": "alice"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			det := DetectAndFormatPayload(tc.input, p)
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

func TestDetectAndFormatPayload_XML(t *testing.T) {
	p := PaletteDarkSlate

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
			wantType:  PayloadXML,
			wantLabel: "XML",
			wantSub:   `<id>42</id>`,
		},
		{
			name:      "nested xml with attributes",
			input:     `<config env="prod"><server host="127.0.0.1" port="8080"><status>running</status></server></config>`,
			wantType:  PayloadXML,
			wantLabel: "XML",
			wantSub:   `host="127.0.0.1"`,
		},
		{
			name:      "xml with comment and prefix",
			input:     `Incoming frame: <packet id="1"><!-- heartbeat --><ack>true</ack></packet>`,
			wantType:  PayloadXML,
			wantLabel: "XML",
			wantSub:   `<!-- heartbeat -->`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			det := DetectAndFormatPayload(tc.input, p)
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

func TestDetectAndFormatPayload_YAML(t *testing.T) {
	p := PaletteDarkSlate

	tests := []struct {
		name      string
		input     string
		wantType  PayloadType
		wantLabel string
		wantSub   string
	}{
		{
			name: "multiline yaml mapping",
			input: "server:\n  port: 8080\n  host: 0.0.0.0\n  enabled: true",
			wantType:  PayloadYAML,
			wantLabel: "YAML",
			wantSub:   "port: 8080",
		},
		{
			name: "yaml list sequence",
			input: "- item1\n- item2\n- item3",
			wantType:  PayloadYAML,
			wantLabel: "YAML",
			wantSub:   "- item1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			det := DetectAndFormatPayload(tc.input, p)
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

func TestDetectAndFormatPayload_Logfmt(t *testing.T) {
	p := PaletteDarkSlate

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
			wantType:   PayloadLogfmt,
			wantLabel:  "LOGFMT",
			wantPrefix: "",
			wantSub:    `msg`,
		},
		{
			name:       "logfmt with timestamp prefix",
			input:      `2026-09-11 14:30:00 level=warn module=auth user=admin attempts=3`,
			wantType:   PayloadLogfmt,
			wantLabel:  "LOGFMT",
			wantPrefix: "2026-09-11 14:30:00",
			wantSub:    `module`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			det := DetectAndFormatPayload(tc.input, p)
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

func TestDetectAndFormatPayload_PlainFallback(t *testing.T) {
	p := PaletteDarkSlate

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
		det := DetectAndFormatPayload(input, p)
		if det.Type != PayloadNone {
			t.Errorf("expected input %q to return PayloadNone, got %v (%s)", input, det.Type, det.TypeLabel)
		}
	}
}
