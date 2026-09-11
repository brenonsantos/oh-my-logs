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

func TestSelectionBatchDelta(t *testing.T) {
	m := newTestModel()
	t0 := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)

	// Ingest 4 records at 0ms, 50ms, 200ms, 1000ms
	records := []record.Record{
		{Raw: "line 0", Fields: map[string]string{"message": "line 0"}, Timestamp: t0},
		{Raw: "line 1", Fields: map[string]string{"message": "line 1"}, Timestamp: t0.Add(50 * time.Millisecond)},
		{Raw: "line 2", Fields: map[string]string{"message": "line 2"}, Timestamp: t0.Add(200 * time.Millisecond)},
		{Raw: "line 3", Fields: map[string]string{"message": "line 3"}, Timestamp: t0.Add(1000 * time.Millisecond)},
	}
	for _, r := range records {
		m.ingestRecord(r)
	}

	// 1. Multi-row selection from index 1 to 3 (3 records total, elapsed: 950ms)
	m.selectionStart = 1
	m.selectionEnd = 3

	d, ok := m.selectionDelta()
	if !ok {
		t.Fatalf("expected selectionDelta to be available")
	}
	if d != 950*time.Millisecond {
		t.Errorf("expected delta 950ms, got %v", d)
	}

	// 2. Status bar badge
	statusBar := m.viewStatusBar()
	if !strings.Contains(statusBar, "3 selected (Δt: +950.0ms)") {
		t.Errorf("expected status bar to contain '3 selected (Δt: +950.0ms)', got:\n%s", statusBar)
	}

	// 3. Selection message format
	msg := m.selectionMessage(3, "press y to copy")
	if !strings.Contains(msg, "3 rows selected · Δt: +950.0ms (press y to copy)") {
		t.Errorf("unexpected selection message: %q", msg)
	}

	// 4. Press 'y' to copy -> check message
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = updated.(Model)
	if !strings.Contains(m.message, "✓ Copied 3 rows (Δt: +950.0ms) to clipboard") {
		t.Errorf("expected copy message with delta, got %q", m.message)
	}

	// 5. Without timestamps: graceful fallback
	m2 := newTestModel()
	m2.visible = []record.Record{
		{Raw: "raw1", Fields: map[string]string{"message": "raw1"}},
		{Raw: "raw2", Fields: map[string]string{"message": "raw2"}},
	}
	m2.selectionStart = 0
	m2.selectionEnd = 1
	if _, ok := m2.selectionDelta(); ok {
		t.Errorf("expected selectionDelta to be false without timestamps/deltas")
	}
	sb2 := m2.viewStatusBar()
	if !strings.Contains(sb2, "2 selected") || strings.Contains(sb2, "Δt:") {
		t.Errorf("expected status bar to contain '2 selected' without delta, got:\n%s", sb2)
	}
}

func TestLogcatTimestampAndDelta(t *testing.T) {
	m := newTestModel()
	m.tsField = "time"
	m.tsMode = TSModeBoth
	m.showTimestamp = true
	m.columns = []record.Column{
		{Field: "time", Title: "Time", Width: 18, Style: "timestamp"},
		{Field: "pid", Title: "PID", Width: 6, Style: "muted"},
		{Field: "tid", Title: "TID", Width: 6, Style: "muted"},
		{Field: "level", Title: "Lvl", Width: 5, Style: "level"},
		{Field: "tag", Title: "Tag", Width: 20, Style: "identifier"},
		{Field: "message", Title: "Message", Width: 0, Style: "primary"},
	}

	// Ingest two records with Logcat time format
	r1 := record.Record{
		Raw: "08-10 05:34:48.669   559   559 I SyntheticService: initialized",
		Fields: map[string]string{
			"time":    "08-10 05:34:48.669",
			"pid":     "559",
			"tid":     "559",
			"level":   "I",
			"tag":     "SyntheticService",
			"message": "initialized",
		},
	}
	m.ingestRecord(r1)

	r2 := record.Record{
		Raw: "08-10 05:34:48.719   559   559 I SyntheticService: ready",
		Fields: map[string]string{
			"time":    "08-10 05:34:48.719",
			"pid":     "559",
			"tid":     "559",
			"level":   "I",
			"tag":     "SyntheticService",
			"message": "ready",
		},
	}
	m.ingestRecord(r2)

	// Record 0 timestamp should be parsed from "08-10 05:34:48.669"
	if m.visible[0].Timestamp.Month() != 8 || m.visible[0].Timestamp.Day() != 10 {
		t.Errorf("expected record 0 to parse month 8, day 10, got %v", m.visible[0].Timestamp)
	}

	// Delta between .669 and .719 is exactly 50ms
	if m.visible[1].Delta != 50*time.Millisecond {
		t.Errorf("expected delta 50ms, got %v", m.visible[1].Delta)
	}
	if m.visible[1].Fields["_delta"] != "+50.0ms" {
		t.Errorf("expected formatted delta '+50.0ms', got %q", m.visible[1].Fields["_delta"])
	}

	// Columns should retain width 18 for Time
	cols := m.effectiveColumns()
	var timeCol *record.Column
	for i := range cols {
		if cols[i].Field == "time" {
			timeCol = &cols[i]
			break
		}
	}
	if timeCol == nil {
		t.Fatalf("expected effectiveColumns to contain 'time' column")
	}
	if timeCol.Width != 18 {
		t.Errorf("expected Time column width 18, got %d", timeCol.Width)
	}
}



