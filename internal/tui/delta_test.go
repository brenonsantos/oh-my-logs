package tui

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/brenoniehues/oh-my-logs/internal/config"
	"github.com/brenoniehues/oh-my-logs/internal/parser"
	"github.com/brenoniehues/oh-my-logs/internal/record"
	"github.com/brenoniehues/oh-my-logs/internal/serial"
	"github.com/brenoniehues/oh-my-logs/internal/timing"
	tea "github.com/charmbracelet/bubbletea"
)

func TestTimestampModeCycling(t *testing.T) {
	m := newTestModel()
	if m.tsMode != TSModeOff {
		t.Fatalf("expected initial mode TSModeOff, got %v", m.tsMode)
	}

	// 1st press -> TSModeClock
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	m = updated.(Model)
	if m.tsMode != TSModeClock || !m.showTimestamp {
		t.Fatalf("expected TSModeClock and showTimestamp=true, got %v (%v)", m.tsMode, m.showTimestamp)
	}
	if !strings.Contains(m.viewStatusBar(), "⏱ CLOCK") {
		t.Errorf("expected status bar badge '⏱ CLOCK', got: %s", m.viewStatusBar())
	}

	// 2nd press -> TSModeDelta
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	m = updated.(Model)
	if m.tsMode != TSModeDelta || !m.showTimestamp {
		t.Fatalf("expected TSModeDelta and showTimestamp=true, got %v (%v)", m.tsMode, m.showTimestamp)
	}
	if !strings.Contains(m.viewStatusBar(), "⏱ Δt") {
		t.Errorf("expected status bar badge '⏱ Δt', got: %s", m.viewStatusBar())
	}

	// 3rd press -> TSModeBoth
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	m = updated.(Model)
	if m.tsMode != TSModeBoth || !m.showTimestamp {
		t.Fatalf("expected TSModeBoth and showTimestamp=true, got %v (%v)", m.tsMode, m.showTimestamp)
	}
	if !strings.Contains(m.viewStatusBar(), "⏱ BOTH") {
		t.Errorf("expected status bar badge '⏱ BOTH', got: %s", m.viewStatusBar())
	}

	// 4th press -> TSModeOff
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	m = updated.(Model)
	if m.tsMode != TSModeOff || m.showTimestamp {
		t.Fatalf("expected TSModeOff and showTimestamp=false, got %v (%v)", m.tsMode, m.showTimestamp)
	}
	if !strings.Contains(m.viewStatusBar(), "⏱ OFF") {
		t.Errorf("expected status bar badge '⏱ OFF', got: %s", m.viewStatusBar())
	}
}

func TestEffectiveColumnsTimestampModes(t *testing.T) {
	m := newTestModel()
	colsOff := m.effectiveColumns()
	for _, c := range colsOff {
		if c.Field == "_ts" || c.Field == "_delta" {
			t.Errorf("expected neither _ts nor _delta in TSModeOff, got %s", c.Field)
		}
	}

	m.tsMode = TSModeClock
	colsClock := m.effectiveColumns()
	hasTS, hasDelta := false, false
	for _, c := range colsClock {
		if c.Field == "_ts" {
			hasTS = true
		}
		if c.Field == "_delta" {
			hasDelta = true
		}
	}
	if !hasTS || hasDelta {
		t.Errorf("expected only _ts in TSModeClock, got hasTS=%v hasDelta=%v", hasTS, hasDelta)
	}

	m.tsMode = TSModeDelta
	colsDelta := m.effectiveColumns()
	hasTS, hasDelta = false, false
	for _, c := range colsDelta {
		if c.Field == "_ts" {
			hasTS = true
		}
		if c.Field == "_delta" {
			hasDelta = true
		}
	}
	if hasTS || !hasDelta {
		t.Errorf("expected only _delta in TSModeDelta, got hasTS=%v hasDelta=%v", hasTS, hasDelta)
	}

	m.tsMode = TSModeBoth
	colsBoth := m.effectiveColumns()
	hasTS, hasDelta = false, false
	for _, c := range colsBoth {
		if c.Field == "_ts" {
			hasTS = true
		}
		if c.Field == "_delta" {
			hasDelta = true
		}
	}
	if !hasTS || !hasDelta {
		t.Errorf("expected both _ts and _delta in TSModeBoth, got hasTS=%v hasDelta=%v", hasTS, hasDelta)
	}
}

func TestDeltaIngestAndFormatting(t *testing.T) {
	m := newTestModel()
	m.tsMode = TSModeDelta
	m.showTimestamp = true

	t0 := time.Now()
	rec1 := record.Record{
		Raw:       "first line",
		Fields:    map[string]string{"message": "first line"},
		Timestamp: t0,
	}
	m.ingestRecord(rec1)

	if len(m.visible) != 1 {
		t.Fatalf("expected 1 record ingested, got %d", len(m.visible))
	}
	if m.visible[0].Fields["_delta"] != "---" {
		t.Errorf("expected first record delta to be '---', got %q", m.visible[0].Fields["_delta"])
	}

	// Ingest second record 15 milliseconds later
	m.lastRecordTime = t0
	rec2 := record.Record{
		Raw:       "second line",
		Fields:    map[string]string{"message": "second line"},
		Timestamp: t0.Add(15 * time.Millisecond),
	}
	m.ingestRecord(rec2)

	if len(m.visible) != 2 {
		t.Fatalf("expected 2 records ingested, got %d", len(m.visible))
	}
	if !strings.HasPrefix(m.visible[1].Fields["_delta"], "+15") {
		t.Errorf("expected second record delta to format as +15ms, got %q", m.visible[1].Fields["_delta"])
	}

	// Verify table header contains Δt
	header := m.viewTableHeader()
	if !strings.Contains(header, "Δt") {
		t.Errorf("expected table header to contain 'Δt', got:\n%s", header)
	}

	// Render view and check output contains Δt formatting
	view := m.View()
	if !strings.Contains(view, "---") {
		t.Errorf("expected view to contain '---' for row 0 delta, got:\n%s", view)
	}
	if !strings.Contains(view, "+15") {
		t.Errorf("expected view to contain '+15' for row 1 delta, got:\n%s", view)
	}
}

func TestProfileTimingOverrides(t *testing.T) {
	profYAML := `
name: CAN Bus Protocol
timing:
  warn_threshold: 20ms
  alert_threshold: 100ms
columns:
  - key: message
    label: Message
    width: 30
`
	prof, err := parser.ParseProfile([]byte(profYAML))
	if err != nil {
		t.Fatalf("failed to parse profile YAML: %v", err)
	}

	cfg := prof.TimingParameters()
	if cfg.WarnThreshold != 20*time.Millisecond {
		t.Errorf("expected WarnThreshold 20ms, got %v", cfg.WarnThreshold)
	}
	if cfg.AlertThreshold != 100*time.Millisecond {
		t.Errorf("expected AlertThreshold 100ms, got %v", cfg.AlertThreshold)
	}

	tracker := timing.NewTracker(cfg)
	// Normal: 10ms (< 20ms)
	tracker.Update(10 * time.Millisecond)
	lvl := tracker.Classify(10 * time.Millisecond)
	if lvl != timing.LevelNormal {
		t.Errorf("expected LevelNormal for 10ms, got %v", lvl)
	}

	// Hiccup: 30ms (>= 20ms and < 100ms)
	tracker.Update(30 * time.Millisecond)
	lvl = tracker.Classify(30 * time.Millisecond)
	if lvl != timing.LevelHiccup {
		t.Errorf("expected LevelHiccup for 30ms, got %v", lvl)
	}

	// Alert: 120ms (>= 100ms)
	tracker.Update(120 * time.Millisecond)
	lvl = tracker.Classify(120 * time.Millisecond)
	if lvl != timing.LevelAlert {
		t.Errorf("expected LevelAlert for 120ms, got %v", lvl)
	}
}

func TestTimestampModeSettingsPersistence(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "oml-delta-settings-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	appCfg := &config.AppConfig{
		ConfigDir: tempDir,
	}

	m := newTestModel()
	m.appConfig = appCfg
	m.tsMode = TSModeBoth
	m.saveSettings()

	saved, err := appCfg.LoadSettings()
	if err != nil {
		t.Fatalf("failed to load saved settings: %v", err)
	}
	if saved.TimestampMode != "both" {
		t.Errorf("expected saved TimestampMode to be 'both', got %q", saved.TimestampMode)
	}

	// Re-load into fresh model
	buf := record.NewBuffer(100)
	m2 := New(m.serialCfg, nil, m.parser, buf, nil, appCfg)
	if m2.tsMode != TSModeBoth {
		t.Errorf("expected restored model tsMode to be TSModeBoth, got %v", m2.tsMode)
	}
	if !m2.showTimestamp {
		t.Errorf("expected restored model showTimestamp to be true")
	}
}

func TestParsedTimestampDeltas(t *testing.T) {
	profYAML := `
name: TimestampLog
parser:
  type: regex
  pattern: '^\[(?P<time>[^\]]+)\]\s+(?P<message>.*)$'
columns:
  - field: time
    title: Time
    width: 14
  - field: message
    title: Message
    width: 30
`
	prof, err := parser.ParseProfile([]byte(profYAML))
	if err != nil {
		t.Fatalf("failed to parse profile: %v", err)
	}
	p, err := prof.BuildParser()
	if err != nil {
		t.Fatalf("failed to build parser: %v", err)
	}

	buf := record.NewBuffer(100)
	cfg := serial.Config{Port: "COM1", Baud: 115200}
	m := New(cfg, prof, p, buf, nil, &config.AppConfig{})
	m.tsMode = TSModeBoth
	m.showTimestamp = true

	lines := []string{
		"[10:00:00.000] system boot",
		"[10:00:00.050] init radio",
		"[10:00:00.052] radio burst message 1",
		"[10:00:00.053] radio burst message 2",
		"[10:00:00.500] hiccup waiting for ack",
		"[10:00:02.500] anomaly timeout retransmitted",
	}

	for _, line := range lines {
		rec, parseErr := p.Parse(line)
		if parseErr != nil {
			t.Fatalf("parse error on %q: %v", line, parseErr)
		}
		m.ingestRecord(rec)
	}

	if len(m.visible) != 6 {
		t.Fatalf("expected 6 records, got %d", len(m.visible))
	}

	// Line 0: ---
	if m.visible[0].Fields["_delta"] != "---" {
		t.Errorf("expected record 0 delta to be '---', got %q", m.visible[0].Fields["_delta"])
	}
	// Line 1: +50.0ms
	if m.visible[1].Fields["_delta"] != "+50.0ms" {
		t.Errorf("expected record 1 delta to be '+50.0ms', got %q", m.visible[1].Fields["_delta"])
	}
	// Line 2: +2.00ms
	if m.visible[2].Fields["_delta"] != "+2.00ms" {
		t.Errorf("expected record 2 delta to be '+2.00ms', got %q", m.visible[2].Fields["_delta"])
	}
	// Line 3: +1.00ms
	if m.visible[3].Fields["_delta"] != "+1.00ms" {
		t.Errorf("expected record 3 delta to be '+1.00ms', got %q", m.visible[3].Fields["_delta"])
	}
	// Line 4: +447.0ms
	if m.visible[4].Fields["_delta"] != "+447.0ms" {
		t.Errorf("expected record 4 delta to be '+447.0ms', got %q", m.visible[4].Fields["_delta"])
	}
	// Line 5: +2.00s
	if m.visible[5].Fields["_delta"] != "+2.00s" {
		t.Errorf("expected record 5 delta to be '+2.00s', got %q", m.visible[5].Fields["_delta"])
	}
}

