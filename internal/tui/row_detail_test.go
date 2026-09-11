package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/brenoniehues/oh-my-logs/internal/record"
)

func TestOrderedRecordFields(t *testing.T) {
	fields := map[string]string{
		"zebra":   "last",
		"message": "skip me",
		"level":   "ERROR",
		"caller":  "main.c:10",
		"uptime":  "12.34s",
		"alpha":   "first",
	}

	ordered := OrderedRecordFields(fields)
	if len(ordered) != 5 {
		t.Fatalf("expected 5 fields (excluding message), got %d", len(ordered))
	}

	// Priority order: level, uptime, caller, alpha, zebra
	expectedOrder := []string{"level", "uptime", "caller", "alpha", "zebra"}
	for i, exp := range expectedOrder {
		if ordered[i][0] != exp {
			t.Errorf("at index %d: expected key %q, got %q", i, exp, ordered[i][0])
		}
	}
}

func TestFormatHexPreview(t *testing.T) {
	raw := []byte("Hello World! 123")
	lines := FormatHexPreview(raw, 32)
	if len(lines) == 0 {
		t.Fatal("expected formatted hex preview lines")
	}
	if !strings.Contains(lines[0], "48 65 6c 6c 6f") {
		t.Errorf("expected hex bytes in line, got: %s", lines[0])
	}
	if !strings.Contains(lines[0], "|Hello World! 123|") {
		t.Errorf("expected ascii gutter in line, got: %s", lines[0])
	}
}

func TestFormattedRecordDetail(t *testing.T) {
	rec := record.Record{
		ID:        42,
		Timestamp: time.Date(2026, 9, 11, 10, 0, 0, 0, time.UTC),
		Delta:     150 * time.Millisecond,
		Fields: map[string]string{
			"level":   "WARN",
			"module":  "wifi",
			"message": `{"event":"disconnect","reason":3}`,
		},
		Raw: "2026-09-11 10:00:00 [WARN] wifi: {\"event\":\"disconnect\",\"reason\":3}",
	}

	detail := FormattedRecordDetail(rec, "time", true)
	if !strings.Contains(detail, "Record ID: #42") {
		t.Errorf("missing Record ID in detail:\n%s", detail)
	}
	if !strings.Contains(detail, "Level: WARN") {
		t.Errorf("missing Level in detail:\n%s", detail)
	}
	if !strings.Contains(detail, "Pinned: Yes") {
		t.Errorf("missing Pinned status in detail:\n%s", detail)
	}
	if !strings.Contains(detail, "  module: wifi") {
		t.Errorf("missing fields in detail:\n%s", detail)
	}
	if !strings.Contains(detail, "  \"event\": \"disconnect\"") {
		t.Errorf("expected formatted JSON in message:\n%s", detail)
	}
	if !strings.Contains(detail, "Raw Log:") {
		t.Errorf("missing raw log in detail:\n%s", detail)
	}
}
