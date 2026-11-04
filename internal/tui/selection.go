package tui

import (
	"fmt"
	"strings"
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

// selectedText extracts the plain text content corresponding to the current selection.
// If character-level mouse drag bounds are present, it slices the start and end lines
// at the exact selected character columns.
func (m Model) selectedText() string {
	if len(m.visible) == 0 {
		return ""
	}

	// Case 1: Single-row character selection
	if (m.selectionStart < 0 || m.selectionEnd < 0 || m.selectionStart == m.selectionEnd) &&
		m.charSelStart >= 0 && m.charSelEnd >= 0 && m.charSelStart != m.charSelEnd {
		targetIdx := m.selectedRow
		if targetIdx < 0 || targetIdx >= len(m.visible) {
			targetIdx = m.selectionStart
		}
		if targetIdx < 0 || targetIdx >= len(m.visible) {
			targetIdx = m.scrollOffset
		}
		if targetIdx >= 0 && targetIdx < len(m.visible) {
			plainText := strings.TrimRight(m.selectedRowPlainText(targetIdx), " ")
			runes := []rune(plainText)
			minSel, maxSel := m.charSelStart, m.charSelEnd
			if minSel > maxSel {
				minSel, maxSel = maxSel, minSel
			}
			if minSel < 0 {
				minSel = 0
			}
			if minSel < len(runes) {
				if maxSel > len(runes) {
					maxSel = len(runes)
				}
				return string(runes[minSel:maxSel])
			}
		}
		return ""
	}

	// Case 2: Multi-row selection
	if m.selectionStart >= 0 && m.selectionEnd >= 0 && m.selectionStart != m.selectionEnd {
		topRow := m.selectionStart
		bottomRow := m.selectionEnd
		if topRow > bottomRow {
			topRow, bottomRow = bottomRow, topRow
		}
		if topRow < 0 {
			topRow = 0
		}
		if bottomRow >= len(m.visible) {
			bottomRow = len(m.visible) - 1
		}

		// Whole-row selection (Shift+Arrows or Ctrl+A) where columns are not tracked
		if m.charSelStart < 0 || m.charSelEnd < 0 || m.charSelStart == m.charSelEnd {
			var b strings.Builder
			for i := topRow; i <= bottomRow; i++ {
				r := m.visible[i]
				b.WriteString(r.Raw)
				if decoded, ok := r.DecodedMessage(); ok {
					b.WriteString("\n  ↳ ")
					b.WriteString(decoded)
				}
				if i < bottomRow {
					b.WriteByte('\n')
				}
			}
			return b.String()
		}

		// Rectangular block selection across rows: [minCol : maxCol] for each row!
		minCol := m.charSelStart
		maxCol := m.charSelEnd
		if minCol > maxCol {
			minCol, maxCol = maxCol, minCol
		}

		var lines []string
		for rIdx := topRow; rIdx <= bottomRow; rIdx++ {
			plainText := m.selectedRowPlainText(rIdx)
			runes := []rune(plainText)
			if minCol >= len(runes) {
				lines = append(lines, "")
				continue
			}
			end := maxCol
			if end > len(runes) {
				end = len(runes)
			}
			lines = append(lines, string(runes[minCol:end]))
		}
		return strings.Join(lines, "\n")
	}

	return ""
}
