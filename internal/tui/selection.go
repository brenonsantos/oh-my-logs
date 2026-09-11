package tui

import (
	"fmt"
	"time"

	"github.com/brenoniehues/oh-my-logs/internal/timing"
)

// selectionRange returns the normalized [start, end] (inclusive) bounds of the
// active multi-row selection, or (-1, -1) if no multi-row selection is active.
func (m Model) selectionRange() (int, int) {
	if m.selectionStart < 0 || m.selectionEnd < 0 || m.selectionStart == m.selectionEnd {
		return -1, -1
	}
	start, end := m.selectionStart, m.selectionEnd
	if start > end {
		start, end = end, start
	}
	if start < 0 {
		start = 0
	}
	if end >= len(m.visible) {
		end = len(m.visible) - 1
	}
	if start >= end || len(m.visible) == 0 {
		return -1, -1
	}
	return start, end
}

// selectionDelta calculates the elapsed time between the first and last
// record in the currently active multi-row selection, if timestamps/deltas are available.
func (m Model) selectionDelta() (time.Duration, bool) {
	start, end := m.selectionRange()
	if start < 0 || end < 0 {
		return 0, false
	}

	firstRec := m.visible[start]
	lastRec := m.visible[end]

	t0 := firstRec.Timestamp
	if t0.IsZero() {
		t0 = tryParseRecordTimestamp(firstRec, m.tsField)
	}
	t1 := lastRec.Timestamp
	if t1.IsZero() {
		t1 = tryParseRecordTimestamp(lastRec, m.tsField)
	}

	// 1. Both endpoints have valid timestamps
	if !t0.IsZero() && !t1.IsZero() {
		d := t1.Sub(t0)
		if d < 0 && d > -24*time.Hour && t0.Year() == 0 {
			d += 24 * time.Hour
		} else if d < 0 {
			d = -d
		}
		return d, true
	}

	// 2. Fallback: sum inter-record deltas across the selected range
	var sum time.Duration
	hasDelta := false
	for i := start + 1; i <= end; i++ {
		r := m.visible[i]
		if r.Delta > 0 {
			sum += r.Delta
			hasDelta = true
		}
	}
	if hasDelta {
		return sum, true
	}

	return 0, false
}

// selectionMessage formats a user-friendly selection summary with elapsed delta if available.
func (m Model) selectionMessage(count int, baseMsg string) string {
	d, ok := m.selectionDelta()
	suffix := ""
	if baseMsg != "" {
		suffix = fmt.Sprintf(" (%s)", baseMsg)
	}
	if ok {
		return fmt.Sprintf("%d rows selected · Δt: %s%s", count, timing.FormatDelta(d), suffix)
	}
	return fmt.Sprintf("%d rows selected%s", count, suffix)
}
