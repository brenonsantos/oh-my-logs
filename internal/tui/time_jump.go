package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/brenoniehues/oh-my-logs/internal/record"
	"github.com/brenoniehues/oh-my-logs/internal/timing"
	tea "github.com/charmbracelet/bubbletea"
)

var jumpTimeLayouts = []string{
	"15:04:05.000000000",
	"15:04:05.000000",
	"15:04:05.000",
	"15:04:05",
	"15:04",
	"2006-01-02 15:04:05.000000000",
	"2006-01-02 15:04:05.000000",
	"2006-01-02 15:04:05.000",
	"2006-01-02 15:04:05",
	"2006-01-02 15:04",
	"2006/01/02 15:04:05.000",
	"2006/01/02 15:04:05",
	"2006/01/02 15:04",
	"2006-01-02T15:04:05.000000000",
	"2006-01-02T15:04:05.000000",
	"2006-01-02T15:04:05.000",
	"2006-01-02T15:04:05",
	"01-02 15:04:05.000000000",
	"01-02 15:04:05.000000",
	"01-02 15:04:05.000",
	"01-02 15:04:05",
	"01-02 15:04",
	"01/02 15:04:05.000",
	"01/02 15:04:05",
	"01/02 15:04",
	time.RFC3339Nano,
	time.RFC3339,
}

// parseJumpTime converts user query (relative or absolute) into a target time.Time.
func parseJumpTime(query string, refTime time.Time) (time.Time, error) {
	s := strings.TrimSpace(query)
	if s == "" {
		return time.Time{}, fmt.Errorf("empty time query")
	}

	lower := strings.ToLower(s)

	// 1. Relative "ago" format (e.g. "5m ago", "10s ago", "1h ago")
	if strings.HasSuffix(lower, "ago") {
		durationStr := strings.TrimSpace(strings.TrimSuffix(lower, "ago"))
		d, err := time.ParseDuration(durationStr)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid duration in %q: %w", query, err)
		}
		if refTime.IsZero() {
			refTime = time.Now()
		}
		return refTime.Add(-d), nil
	}

	// 2. Relative "in" format (e.g. "in 5m", "in 30s")
	if strings.HasPrefix(lower, "in ") {
		durationStr := strings.TrimSpace(strings.TrimPrefix(lower, "in "))
		d, err := time.ParseDuration(durationStr)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid duration in %q: %w", query, err)
		}
		if refTime.IsZero() {
			refTime = time.Now()
		}
		return refTime.Add(d), nil
	}

	// 3. Signed relative duration (e.g. "+5m", "-10s", "-1h", "+500ms", "+2m30s")
	if strings.HasPrefix(s, "+") || strings.HasPrefix(s, "-") {
		sign := 1
		durationStr := s
		if strings.HasPrefix(s, "+") {
			durationStr = s[1:]
		} else if strings.HasPrefix(s, "-") {
			sign = -1
			durationStr = s[1:]
		}
		d, err := time.ParseDuration(durationStr)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid duration %q: %w", query, err)
		}
		if refTime.IsZero() {
			refTime = time.Now()
		}
		if sign < 0 {
			return refTime.Add(-d), nil
		}
		return refTime.Add(d), nil
	}

	// 4. Bare relative duration (e.g. "5m", "10s", "1h", "500ms") defaults to looking backward
	if d, err := time.ParseDuration(s); err == nil {
		if refTime.IsZero() {
			refTime = time.Now()
		}
		return refTime.Add(-d), nil
	}

	// 5. UNIX epoch timestamps (seconds >= 1,000,000,000 or milliseconds >= 1,000,000,000,000)
	if sec, err := strconv.ParseFloat(s, 64); err == nil && sec >= 1000000000 {
		if sec >= 1000000000000 {
			return time.UnixMilli(int64(sec)), nil
		}
		intSec := int64(sec)
		nsec := int64((sec - float64(intSec)) * 1e9)
		return time.Unix(intSec, nsec), nil
	}

	// 6. Absolute formatted timestamp
	normalized := strings.ReplaceAll(s, ",", ".")
	for strings.Contains(normalized, "  ") {
		normalized = strings.ReplaceAll(normalized, "  ", " ")
	}

	for _, layout := range jumpTimeLayouts {
		if t, err := time.Parse(layout, normalized); err == nil {
			// If layout did not specify a year (year <= 1) and refTime has a year, graft refTime date
			if t.Year() <= 1 && refTime.Year() > 1 {
				return time.Date(refTime.Year(), refTime.Month(), refTime.Day(),
					t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), refTime.Location()), nil
			}
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unrecognized time format %q (expected e.g. 14:20:00, -5m, +30s)", query)
}

// findClosestRecordByTime searches records for the entry closest to target.
// Returns the index of the closest record and the absolute difference.
func findClosestRecordByTime(records []record.Record, target time.Time) (int, time.Duration) {
	if len(records) == 0 {
		return -1, 0
	}

	// 1. Detect if visible records contain full dates or only time-of-day
	var sampleTS time.Time
	for i := range records {
		if !records[i].Timestamp.IsZero() {
			sampleTS = records[i].Timestamp
			break
		}
	}
	if sampleTS.IsZero() {
		return -1, 0
	}

	// 2. Align target date format with records date format
	if sampleTS.Year() > 1 && target.Year() <= 1 {
		target = time.Date(sampleTS.Year(), sampleTS.Month(), sampleTS.Day(),
			target.Hour(), target.Minute(), target.Second(), target.Nanosecond(), sampleTS.Location())
	} else if sampleTS.Year() <= 1 && target.Year() > 1 {
		target = time.Date(0, 1, 1,
			target.Hour(), target.Minute(), target.Second(), target.Nanosecond(), time.UTC)
	}

	bestIdx := -1
	var minDiff time.Duration = 1<<63 - 1

	for i := range records {
		ts := records[i].Timestamp
		if ts.IsZero() {
			continue
		}
		if sampleTS.Year() <= 1 && ts.Year() > 1 {
			ts = time.Date(0, 1, 1,
				ts.Hour(), ts.Minute(), ts.Second(), ts.Nanosecond(), time.UTC)
		}

		diff := ts.Sub(target)
		if diff < 0 {
			diff = -diff
		}
		if diff < minDiff {
			minDiff = diff
			bestIdx = i
			if diff == 0 {
				break
			}
		}
	}

	// 3. Handle midnight / day-rollover edge case when records have dates but user entered time-of-day
	if sampleTS.Year() > 1 && minDiff > 12*time.Hour {
		for _, shift := range []time.Duration{24 * time.Hour, -24 * time.Hour} {
			shiftedTarget := target.Add(shift)
			for i := range records {
				ts := records[i].Timestamp
				if ts.IsZero() {
					continue
				}
				diff := ts.Sub(shiftedTarget)
				if diff < 0 {
					diff = -diff
				}
				if diff < minDiff {
					minDiff = diff
					bestIdx = i
					if diff == 0 {
						break
					}
				}
			}
		}
	}

	return bestIdx, minDiff
}

// referenceTimeForJump determines the anchor time for relative time seeking (+10s, -5m).
func (m *Model) referenceTimeForJump() time.Time {
	// 1. If a row is currently selected and has a valid timestamp, use it
	if m.selectedRow >= 0 && m.selectedRow < len(m.visible) {
		if t := m.visible[m.selectedRow].Timestamp; !t.IsZero() {
			return t
		}
	}

	// 2. If follow mode is active, anchor to the latest visible record
	if m.follow && len(m.visible) > 0 {
		for i := len(m.visible) - 1; i >= 0; i-- {
			if t := m.visible[i].Timestamp; !t.IsZero() {
				return t
			}
		}
	}

	// 3. If scrolled at a viewport position, anchor to the top visible row
	if m.scrollOffset >= 0 && m.scrollOffset < len(m.visible) {
		for i := m.scrollOffset; i < len(m.visible); i++ {
			if t := m.visible[i].Timestamp; !t.IsZero() {
				return t
			}
		}
	}

	// 4. Any visible record
	for i := len(m.visible) - 1; i >= 0; i-- {
		if t := m.visible[i].Timestamp; !t.IsZero() {
			return t
		}
	}

	return time.Now()
}

// jumpToTime parses query and centers viewport on the closest chronological record.
func (m *Model) jumpToTime(query string) {
	query = strings.TrimSpace(query)
	if query == "" {
		m.mode = modeNormal
		return
	}
	m.timeJumpInput.AddHistory(query)

	if len(m.visible) == 0 {
		m.message = "No visible records to jump to"
		m.mode = modeNormal
		return
	}

	ref := m.referenceTimeForJump()
	target, err := parseJumpTime(query, ref)
	if err != nil {
		m.message = fmt.Sprintf("Invalid time query: %v", err)
		m.mode = modeNormal
		return
	}

	bestIdx, diff := findClosestRecordByTime(m.visible, target)
	if bestIdx < 0 {
		m.message = "No timestamped records found in view"
		m.mode = modeNormal
		return
	}

	m.follow = false
	m.selectedRow = bestIdx
	h := m.activeDataHeight()
	m.scrollOffset = bestIdx - h/2
	m.clampScroll()
	if m.splitMode != SplitNone && m.syncScroll {
		m.syncOtherPaneChronologically()
	}

	recTime := m.visible[bestIdx].Timestamp
	timeStr := recTime.Format("15:04:05.000")
	if recTime.Year() > 1 {
		timeStr = recTime.Format("2006-01-02 15:04:05.000")
	}

	if diff == 0 {
		m.message = fmt.Sprintf("⏱ Jumped to %s (row #%d)", timeStr, bestIdx+1)
	} else {
		m.message = fmt.Sprintf("⏱ Jumped to %s (row #%d, Δ %s)", timeStr, bestIdx+1, timing.FormatDelta(diff))
	}
	m.mode = modeNormal
}

// handleTimeJumpKey processes keyboard input when in modeTimeJump.
func (m Model) handleTimeJumpKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case keyMatches(msg, m.keys.Cancel):
		m.mode = modeNormal
		return m, nil

	case keyMatches(msg, m.keys.Confirm):
		m.jumpToTime(m.timeJumpInput.Value)
		return m, nil

	case msg.String() == "up" || msg.Type == tea.KeyUp:
		m.timeJumpInput.HistoryPrev()
		return m, nil

	case msg.String() == "down" || msg.Type == tea.KeyDown:
		m.timeJumpInput.HistoryNext()
		return m, nil

	default:
		m.timeJumpInput.HandleKey(msg)
		return m, nil
	}
}
