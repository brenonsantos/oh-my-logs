package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/brenoniehues/oh-my-logs/internal/record"
	tea "github.com/charmbracelet/bubbletea"
)

func TestParseJumpTime_Relative(t *testing.T) {
	ref := time.Date(2026, 9, 22, 14, 30, 0, 0, time.UTC)

	tests := []struct {
		name     string
		query    string
		expected time.Time
	}{
		{
			name:     "plus minutes",
			query:    "+5m",
			expected: ref.Add(5 * time.Minute),
		},
		{
			name:     "minus minutes",
			query:    "-10m",
			expected: ref.Add(-10 * time.Minute),
		},
		{
			name:     "plus seconds",
			query:    "+30s",
			expected: ref.Add(30 * time.Second),
		},
		{
			name:     "minus seconds",
			query:    "-15s",
			expected: ref.Add(-15 * time.Second),
		},
		{
			name:     "compound duration",
			query:    "+1m30s",
			expected: ref.Add(90 * time.Second),
		},
		{
			name:     "ago suffix minutes",
			query:    "5m ago",
			expected: ref.Add(-5 * time.Minute),
		},
		{
			name:     "ago suffix seconds",
			query:    "45s ago",
			expected: ref.Add(-45 * time.Second),
		},
		{
			name:     "in prefix minutes",
			query:    "in 10m",
			expected: ref.Add(10 * time.Minute),
		},
		{
			name:     "bare duration defaults to backward",
			query:    "5m",
			expected: ref.Add(-5 * time.Minute),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseJumpTime(tc.query, ref)
			if err != nil {
				t.Fatalf("unexpected error parsing %q: %v", tc.query, err)
			}
			if !got.Equal(tc.expected) {
				t.Errorf("query %q: expected %v, got %v", tc.query, tc.expected, got)
			}
		})
	}
}

func TestParseJumpTime_Absolute(t *testing.T) {
	ref := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)

	t.Run("clock with date grafting", func(t *testing.T) {
		got, err := parseJumpTime("14:20:00", ref)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := time.Date(2026, 9, 22, 14, 20, 0, 0, time.UTC)
		if !got.Equal(expected) {
			t.Errorf("expected %v, got %v", expected, got)
		}
	})

	t.Run("clock with subseconds", func(t *testing.T) {
		got, err := parseJumpTime("14:20:00.500", ref)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := time.Date(2026, 9, 22, 14, 20, 0, 500000000, time.UTC)
		if !got.Equal(expected) {
			t.Errorf("expected %v, got %v", expected, got)
		}
	})

	t.Run("clock hour and minute", func(t *testing.T) {
		got, err := parseJumpTime("14:20", ref)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := time.Date(2026, 9, 22, 14, 20, 0, 0, time.UTC)
		if !got.Equal(expected) {
			t.Errorf("expected %v, got %v", expected, got)
		}
	})

	t.Run("full date and time", func(t *testing.T) {
		got, err := parseJumpTime("2026-09-22 15:45:00", ref)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := time.Date(2026, 9, 22, 15, 45, 0, 0, time.UTC)
		if !got.Equal(expected) {
			t.Errorf("expected %v, got %v", expected, got)
		}
	})

	t.Run("unix epoch seconds", func(t *testing.T) {
		got, err := parseJumpTime("1700000000", ref)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := time.Unix(1700000000, 0)
		if !got.Equal(expected) {
			t.Errorf("expected %v, got %v", expected, got)
		}
	})

	t.Run("invalid queries", func(t *testing.T) {
		invalid := []string{"", "   ", "not-a-time", "invalid:time:format:here"}
		for _, q := range invalid {
			if _, err := parseJumpTime(q, ref); err == nil {
				t.Errorf("expected error for %q, got nil", q)
			}
		}
	})
}

func TestFindClosestRecordByTime(t *testing.T) {
	t.Run("empty records", func(t *testing.T) {
		idx, diff := findClosestRecordByTime(nil, time.Now())
		if idx != -1 || diff != 0 {
			t.Errorf("expected -1, 0, got %d, %v", idx, diff)
		}
	})

	t.Run("exact match", func(t *testing.T) {
		base := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)
		records := []record.Record{
			{ID: 1, Timestamp: base},
			{ID: 2, Timestamp: base.Add(1 * time.Minute)},
			{ID: 3, Timestamp: base.Add(2 * time.Minute)},
		}

		idx, diff := findClosestRecordByTime(records, base.Add(1*time.Minute))
		if idx != 1 {
			t.Errorf("expected index 1, got %d", idx)
		}
		if diff != 0 {
			t.Errorf("expected diff 0, got %v", diff)
		}
	})

	t.Run("closest match between entries", func(t *testing.T) {
		base := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)
		records := []record.Record{
			{ID: 1, Timestamp: base},
			{ID: 2, Timestamp: base.Add(10 * time.Second)},
			{ID: 3, Timestamp: base.Add(30 * time.Second)},
		}

		// Target is 25s, closer to 30s (diff 5s) than to 10s (diff 15s)
		target := base.Add(25 * time.Second)
		idx, diff := findClosestRecordByTime(records, target)
		if idx != 2 {
			t.Errorf("expected index 2, got %d", idx)
		}
		if diff != 5*time.Second {
			t.Errorf("expected diff 5s, got %v", diff)
		}
	})

	t.Run("records have dates but query is clock time only", func(t *testing.T) {
		base := time.Date(2026, 9, 22, 14, 0, 0, 0, time.UTC)
		records := []record.Record{
			{ID: 1, Timestamp: base},
			{ID: 2, Timestamp: base.Add(20 * time.Minute)}, // 14:20:00
			{ID: 3, Timestamp: base.Add(40 * time.Minute)}, // 14:40:00
		}

		// Query time has year 0
		clockQuery := time.Date(0, 1, 1, 14, 21, 0, 0, time.UTC)
		idx, diff := findClosestRecordByTime(records, clockQuery)
		if idx != 1 {
			t.Errorf("expected index 1, got %d", idx)
		}
		if diff != 1*time.Minute {
			t.Errorf("expected diff 1m, got %v", diff)
		}
	})

	t.Run("records have clock only (year 0) but query has date", func(t *testing.T) {
		records := []record.Record{
			{ID: 1, Timestamp: time.Date(0, 1, 1, 8, 0, 0, 0, time.UTC)},
			{ID: 2, Timestamp: time.Date(0, 1, 1, 8, 30, 0, 0, time.UTC)},
			{ID: 3, Timestamp: time.Date(0, 1, 1, 9, 0, 0, 0, time.UTC)},
		}

		query := time.Date(2026, 9, 22, 8, 29, 0, 0, time.UTC)
		idx, diff := findClosestRecordByTime(records, query)
		if idx != 1 {
			t.Errorf("expected index 1, got %d", idx)
		}
		if diff != 1*time.Minute {
			t.Errorf("expected diff 1m, got %v", diff)
		}
	})

	t.Run("midnight day rollover", func(t *testing.T) {
		d1 := time.Date(2026, 9, 22, 23, 58, 0, 0, time.UTC)
		d2 := time.Date(2026, 9, 23, 0, 2, 0, 0, time.UTC)
		records := []record.Record{
			{ID: 1, Timestamp: d1},
			{ID: 2, Timestamp: d2},
		}

		// User queries "00:03:00" with sampleTS on day 22
		target := time.Date(0, 1, 1, 0, 3, 0, 0, time.UTC)
		idx, diff := findClosestRecordByTime(records, target)
		if idx != 1 {
			t.Errorf("expected index 1 (next day 00:02), got %d", idx)
		}
		if diff != 1*time.Minute {
			t.Errorf("expected diff 1m, got %v", diff)
		}
	})
}

func TestModel_TimeJumpWorkflow(t *testing.T) {
	m := newTestModel()
	m.width = 100
	m.height = 30
	m.recalcLayout()

	base := time.Date(2026, 9, 22, 10, 0, 0, 0, time.Local)
	for i := 0; i < 50; i++ {
		r := record.Record{
			ID:        uint64(i + 1),
			Timestamp: base.Add(time.Duration(i*10) * time.Second),
			Raw:       fmt.Sprintf("log line %d at %s", i+1, base.Add(time.Duration(i*10)*time.Second).Format("15:04:05")),
			Fields: map[string]string{
				"time":    base.Add(time.Duration(i*10) * time.Second).Format("15:04:05"),
				"message": fmt.Sprintf("event %d", i+1),
			},
		}
		m.ingestRecord(r)
	}

	if len(m.visible) != 50 {
		t.Fatalf("expected 50 visible records, got %d", len(m.visible))
	}

	// 1. Press J in normal mode -> opens modeTimeJump
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'J'}})
	m = updated.(Model)
	if m.mode != modeTimeJump {
		t.Fatalf("expected modeTimeJump, got %v", m.mode)
	}

	// 2. Press Esc -> cancels back to modeNormal without changing selection
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = updated.(Model)
	if m.mode != modeNormal {
		t.Fatalf("expected modeNormal after Esc, got %v", m.mode)
	}

	// 3. Press J again, type "+2m", and press Enter
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'J'}})
	m = updated.(Model)
	for _, r := range "+2m" {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(Model)
	}
	if m.timeJumpInput.Value != "+2m" {
		t.Fatalf("expected timeJumpInput '+2m', got %q", m.timeJumpInput.Value)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	// Mode returns to normal
	if m.mode != modeNormal {
		t.Fatalf("expected modeNormal after jump, got %v", m.mode)
	}

	// Follow mode must be disabled
	if m.follow {
		t.Errorf("expected follow mode to be false after jump")
	}

	// Selected row must be set
	if m.selectedRow < 0 {
		t.Errorf("expected selectedRow to be >= 0 after jump, got %d", m.selectedRow)
	}

	// Message should confirm jump
	if !strings.Contains(m.message, "Jumped to") {
		t.Errorf("expected status message to contain 'Jumped to', got %q", m.message)
	}

	// 4. Test history recall: open J again, press Up arrow -> recalls "+2m"
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'J'}})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = updated.(Model)
	if m.timeJumpInput.Value != "+2m" {
		t.Errorf("expected recalled history '+2m', got %q", m.timeJumpInput.Value)
	}

	// 5. Test jump to absolute time (e.g. 10:02:00 -> row at 120s = index 12)
	targetTimeStr := base.Add(120 * time.Second).Format("15:04:05")
	m.jumpToTime(targetTimeStr)
	if m.selectedRow != 12 {
		t.Errorf("expected jump to row 12 for %s, got %d", targetTimeStr, m.selectedRow)
	}
}

func BenchmarkFindClosestRecordByTime(b *testing.B) {
	base := time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC)
	const count = 50000
	records := make([]record.Record, count)
	for i := 0; i < count; i++ {
		records[i] = record.Record{
			ID:        uint64(i + 1),
			Timestamp: base.Add(time.Duration(i) * 100 * time.Millisecond),
		}
	}
	target := base.Add(2500 * time.Second)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		idx, _ := findClosestRecordByTime(records, target)
		if idx < 0 {
			b.Fatalf("failed to find match")
		}
	}
}
