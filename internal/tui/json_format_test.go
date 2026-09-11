package tui

import (
	"strings"
	"testing"
)

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
	out := ColorizeJSON(raw, PaletteDarkSlate)
	if len(out) == 0 {
		t.Fatal("expected colorized output, got empty")
	}
	if !strings.Contains(out, "key") || !strings.Contains(out, "value") {
		t.Errorf("output missing text content:\n%s", out)
	}
}

func TestWrapTextLines(t *testing.T) {
	longText := "The quick brown fox jumps over the lazy dog near the riverbank on a sunny morning"
	wrapped := wrapTextLines(longText, 25)
	if len(wrapped) < 3 {
		t.Errorf("expected at least 3 wrapped lines, got %d", len(wrapped))
	}
	for _, l := range wrapped {
		if len([]rune(l)) > 25 {
			t.Errorf("wrapped line exceeds 25 runes: %q (len=%d)", l, len([]rune(l)))
		}
	}
	rejoined := strings.Join(wrapped, " ")
	if rejoined != longText {
		t.Errorf("rejoined text mismatch: got %q, want %q", rejoined, longText)
	}
}
