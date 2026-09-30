package tui

import (
	"testing"
	"time"

	"github.com/brenoniehues/oh-my-logs/internal/parser"
)

func TestParseLineWithTimestampFallback_BracketedPrefix(t *testing.T) {
	// A regex parser that expects Zephyr-style device uptime: [uptime] <level> message
	p, err := parser.NewRegexParser(`^\[\s*(?P<uptime>[^\]]+?)\s*\]\s+<(?P<level>[a-zA-Z]+)>\s+(?P<message>.*)$`)
	if err != nil {
		t.Fatalf("failed to create parser: %v", err)
	}

	// 1. Normal line without host timestamp:
	normalLine := `[  15860.203] <inf> Hello device`
	rec, ts := parseLineWithTimestampFallback(p, normalLine)
	if rec.Fields["level"] != "inf" {
		t.Errorf("expected level=inf, got %q", rec.Fields["level"])
	}
	if rec.Fields["uptime"] != "15860.203" {
		t.Errorf("expected uptime=15860.203, got %q", rec.Fields["uptime"])
	}
	if rec.Fields["message"] != "Hello device" {
		t.Errorf("expected message='Hello device', got %q", rec.Fields["message"])
	}
	if !ts.IsZero() {
		t.Errorf("expected ts=zero for normal line, got %v", ts)
	}

	// 2. Line saved with disk logger host timestamp:
	savedLine := `[2026-09-30T15:52:37.379] [  15860.203] <inf> Hello device`
	rec2, ts2 := parseLineWithTimestampFallback(p, savedLine)
	if rec2.Fields["level"] != "inf" {
		t.Errorf("expected level=inf, got %q", rec2.Fields["level"])
	}
	if rec2.Fields["uptime"] != "15860.203" {
		t.Errorf("expected uptime=15860.203, got %q", rec2.Fields["uptime"])
	}
	if rec2.Fields["message"] != "Hello device" {
		t.Errorf("expected message='Hello device', got %q", rec2.Fields["message"])
	}
	if rec2.Raw != savedLine {
		t.Errorf("expected raw preserved %q, got %q", savedLine, rec2.Raw)
	}
	if ts2.IsZero() {
		t.Fatalf("expected non-zero parsedTs")
	}
	expectedTs, _ := time.Parse("2006-01-02T15:04:05.000", "2026-09-30T15:52:37.379")
	if !ts2.Equal(expectedTs) {
		t.Errorf("expected ts=%v, got %v", expectedTs, ts2)
	}
}

func TestParseLineWithTimestampFallback_PassThroughOnNoMatch(t *testing.T) {
	p, err := parser.NewRegexParser(`^\[(?P<uptime>\d+)\]\s+(?P<message>.*)$`)
	if err != nil {
		t.Fatalf("failed to create parser: %v", err)
	}

	unmatched := `Completely unparseable line`
	rec, ts := parseLineWithTimestampFallback(p, unmatched)
	if rec.Fields["message"] != unmatched {
		t.Errorf("expected fallback message=%q, got %q", unmatched, rec.Fields["message"])
	}
	if !ts.IsZero() {
		t.Errorf("expected zero ts, got %v", ts)
	}
}
