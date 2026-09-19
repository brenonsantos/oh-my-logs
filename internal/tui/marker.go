package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/brenoniehues/oh-my-logs/internal/record"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// activeSelectedRowIndex returns the currently selected row index in the active tab/pane, or -1.
func (m Model) activeSelectedRowIndex() int {
	if m.splitMode != SplitNone {
		t := m.currentTabForPane(m.activePane)
		if t != nil && t.SelectedRow >= 0 && t.SelectedRow < len(t.Visible) {
			return t.SelectedRow
		}
		return -1
	}
	if m.selectedRow >= 0 && m.selectedRow < len(m.visible) {
		return m.selectedRow
	}
	return -1
}

// activeSelectedRecord returns the currently selected record, or false if no row is selected.
func (m Model) activeSelectedRecord() (record.Record, bool) {
	row := m.activeSelectedRowIndex()
	if row < 0 {
		return record.Record{}, false
	}
	if m.splitMode != SplitNone {
		t := m.currentTabForPane(m.activePane)
		if t != nil && row < len(t.Visible) {
			return t.Visible[row], true
		}
		return record.Record{}, false
	}
	if row < len(m.visible) {
		return m.visible[row], true
	}
	return record.Record{}, false
}

// openMarkerPrompt enters the marker note input mode and captures target line context.
func (m *Model) openMarkerPrompt() {
	m.markerInput.Reset()
	m.mode = modeMarkerPrompt
	m.markerTargetRow = -1
	m.markerTargetID = 0
	m.markerTargetText = ""
	m.markerTargetTime = time.Time{}

	if rec, ok := m.activeSelectedRecord(); ok {
		m.markerTargetRow = m.activeSelectedRowIndex()
		m.markerTargetID = rec.ID
		m.markerTargetTime = rec.Timestamp
		m.markerTargetText = rec.Raw
		if m.markerTargetText == "" && rec.Fields != nil {
			m.markerTargetText = rec.Fields["message"]
		}
	}
}

// handleMarkerPromptKey processes keystrokes in the marker input mode.
func (m Model) handleMarkerPromptKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case keyMatches(msg, m.keys.Cancel):
		m.mode = modeNormal
		m.markerInput.Reset()
		return m, nil

	case keyMatches(msg, m.keys.Confirm) || msg.Type == tea.KeyEnter:
		note := strings.TrimSpace(m.markerInput.Value)
		m.recordMarker(note)
		if note != "" {
			m.markerInput.AddHistory(note)
		}
		m.markerInput.Reset()
		m.mode = modeNormal
		return m, nil

	default:
		m.markerInput.HandleKey(msg)
		return m, nil
	}
}

// recordMarker creates and inserts a milestone marker into the log stream at the target row or end of stream.
func (m *Model) recordMarker(note string) {
	ts := m.markerTargetTime
	if ts.IsZero() {
		ts = time.Now()
	}
	r := record.NewMarkerRecord(note, ts)
	if r.Fields == nil {
		r.Fields = make(map[string]string)
	}
	if r.Fields[m.tsField] == "" {
		r.Fields[m.tsField] = ts.Format(m.tsFormat)
	}
	if r.Fields["_ts"] == "" {
		r.Fields["_ts"] = r.Fields[m.tsField]
	}

	m.nextRecordID++
	r.ID = m.nextRecordID

	if m.bookmarks == nil {
		m.bookmarks = make(map[uint64]struct{})
	}
	m.bookmarks[r.ID] = struct{}{}

	// Ingest into buffer
	if m.markerTargetID != 0 {
		m.buffer.InsertBeforeID(m.markerTargetID, r)
	} else {
		m.buffer.Add(r)
	}

	// Persist to disk if active
	if m.diskLogger != nil && m.diskLogger.IsActive() {
		raw := r.Raw
		if raw != "" {
			line := raw
			if !r.Timestamp.IsZero() {
				const saveTimestampFmt = "2006-01-02T15:04:05.000"
				line = "[" + r.Timestamp.Format(saveTimestampFmt) + "] " + raw
			}
			m.diskLogger.WriteLine(line)
		}
	}

	// Dispatch to all tabs
	if len(m.tabs) == 0 {
		_ = m.currentTab()
	}
	for i := range m.tabs {
		if m.markerTargetID != 0 {
			targetIdx := -1
			for idx, rec := range m.tabs[i].Visible {
				if rec.ID == m.markerTargetID {
					targetIdx = idx
					break
				}
			}
			if targetIdx >= 0 {
				newVis := make([]record.Record, 0, len(m.tabs[i].Visible)+1)
				newVis = append(newVis, m.tabs[i].Visible[:targetIdx]...)
				newVis = append(newVis, r)
				newVis = append(newVis, m.tabs[i].Visible[targetIdx:]...)
				m.tabs[i].Visible = newVis

				if i == m.activeTab {
					m.selectedRow = targetIdx
				}
				m.tabs[i].SelectedRow = targetIdx
			} else {
				m.tabs[i].Visible = append(m.tabs[i].Visible, r)
			}
		} else {
			m.tabs[i].Visible = append(m.tabs[i].Visible, r)
			if m.tabs[i].Follow && !m.paused {
				h := m.tableHeight
				if len(m.tabs[i].Visible) > h {
					m.tabs[i].ScrollOffset = len(m.tabs[i].Visible) - h
				} else {
					m.tabs[i].ScrollOffset = 0
				}
			}
		}
	}

	cur := m.currentTab()
	m.visible = cur.Visible
	m.scrollOffset = cur.ScrollOffset
	m.follow = cur.Follow

	if m.markerTargetRow >= 0 {
		if note != "" {
			m.message = fmt.Sprintf("📌 Inserted marker at line %d: %s", m.markerTargetRow+1, note)
		} else {
			m.message = fmt.Sprintf("📌 Inserted marker at line %d", m.markerTargetRow+1)
		}
	} else {
		if note != "" {
			m.message = fmt.Sprintf("📌 Dropped marker: %s", note)
		} else {
			m.message = "📌 Dropped marker milestone"
		}
	}
}

// formatMarkerRow renders a distinctive full-width milestone divider across the table.
func formatMarkerRow(r record.Record, width int, hasBg bool, rowBg lipgloss.TerminalColor) string {
	ts := ""
	if !r.Timestamp.IsZero() {
		ts = r.Timestamp.Format("15:04:05.000")
	} else if r.Fields["time"] != "" {
		ts = r.Fields["time"]
	}

	var bannerText string
	if r.MarkerNote != "" {
		if ts != "" {
			bannerText = fmt.Sprintf("── 📌 MARKER [%s]: %s ", ts, r.MarkerNote)
		} else {
			bannerText = fmt.Sprintf("── 📌 MARKER: %s ", r.MarkerNote)
		}
	} else {
		if ts != "" {
			bannerText = fmt.Sprintf("── 📌 MARKER [%s] ", ts)
		} else {
			bannerText = "── 📌 MARKER "
		}
	}

	bannerW := lipgloss.Width(bannerText)
	trailingDashes := ""
	if width > bannerW {
		trailingDashes = strings.Repeat("─", width-bannerW)
	}

	markerStyle := lipgloss.NewStyle().Foreground(colorYellow).Bold(true)
	dashStyle := lipgloss.NewStyle().Foreground(colorAccent)
	if hasBg {
		markerStyle = markerStyle.Background(rowBg)
		dashStyle = dashStyle.Background(rowBg)
	}

	return markerStyle.Render(bannerText) + dashStyle.Render(trailingDashes)
}

// viewMarkerDrawer renders an interactive docked drawer with selected-row background styling.
func (m Model) viewMarkerDrawer() string {
	w := m.width
	if w <= 0 {
		w = 80
	}

	bg := colorSelected
	baseStyle := lipgloss.NewStyle().Background(bg)

	// 1. Header line: ─── 📌 ADD STREAM MARKER ── Target: Line #137 [11:03:20.322] ─── [Enter: insert · Esc: cancel] ───
	var targetVal string
	if m.markerTargetRow >= 0 {
		tsStr := ""
		if !m.markerTargetTime.IsZero() {
			tsStr = " [" + m.markerTargetTime.Format("15:04:05.000") + "]"
		}
		targetVal = fmt.Sprintf("Target: Line #%d%s", m.markerTargetRow+1, tsStr)
	} else {
		targetVal = "Target: End of Stream (Live Milestone)"
	}

	title := fmt.Sprintf("─── 📌 ADD STREAM MARKER ── %s ", targetVal)
	titleStyled := baseStyle.Foreground(colorYellow).Bold(true).Render(title)
	titleW := lipgloss.Width(titleStyled)

	help := " [Enter: insert · ←/→: cursor · ↑/↓: history · Esc: cancel] ───"
	helpStyled := baseStyle.Foreground(colorMuted).Render(help)
	helpW := lipgloss.Width(helpStyled)

	dashCount := w - titleW - helpW
	if dashCount < 1 {
		dashCount = 1
	}
	middleDashes := baseStyle.Foreground(colorAccent).Render(strings.Repeat("─", dashCount))
	headerLine := titleStyled + middleDashes + helpStyled
	if curW := lipgloss.Width(headerLine); curW < w {
		headerLine += baseStyle.Foreground(colorAccent).Render(strings.Repeat("─", w-curW))
	} else if curW > w {
		headerLine = baseStyle.MaxWidth(w).Render(headerLine)
	}

	// 2. Log preview line:  Log:  <message>
	logLabel := baseStyle.Foreground(colorMuted).Bold(true).Render("  Log:  ")
	var logBody string
	if m.markerTargetText != "" {
		preview := m.markerTargetText
		avail := w - 10
		if avail > 3 && len(preview) > avail {
			preview = preview[:avail-3] + "..."
		} else if len(preview) > avail {
			preview = preview[:max(0, avail)]
		}
		logBody = baseStyle.Foreground(colorFg).Render(preview)
	} else {
		logBody = baseStyle.Foreground(colorMuted).Italic(true).Render("(new live milestone marker at end of stream)")
	}
	logLine := logLabel + logBody
	if curW := lipgloss.Width(logLine); curW < w {
		logLine += baseStyle.Render(strings.Repeat(" ", w-curW))
	} else if curW > w {
		logLine = baseStyle.MaxWidth(w).Render(logLine)
	}

	// 3. Note input line:  Note: [ power cycle #1 █                                               ]
	noteLabel := baseStyle.Foreground(colorAccent).Bold(true).Render("  Note: ")
	bracketOpen := baseStyle.Foreground(colorMuted).Render("[ ")
	bracketClose := baseStyle.Foreground(colorMuted).Render(" ]")

	promptW := lipgloss.Width(noteLabel) + lipgloss.Width(bracketOpen)
	bracketCloseW := lipgloss.Width(bracketClose)
	availInputW := w - promptW - bracketCloseW - 2
	if availInputW < 10 {
		availInputW = 10
	}
	inputStyle := baseStyle.Foreground(colorFg)
	input := m.markerInput.RenderWindow(availInputW, inputStyle)
	inputLine := noteLabel + bracketOpen + input + bracketClose
	if curW := lipgloss.Width(inputLine); curW < w {
		inputLine += baseStyle.Render(strings.Repeat(" ", w-curW))
	} else if curW > w {
		inputLine = baseStyle.MaxWidth(w).Render(inputLine)
	}

	// 4. Bottom divider line
	bottomLine := baseStyle.Foreground(colorAccent).Render(strings.Repeat("─", w))
	if curW := lipgloss.Width(bottomLine); curW > w {
		bottomLine = baseStyle.MaxWidth(w).Render(bottomLine)
	}

	return headerLine + "\n" + logLine + "\n" + inputLine + "\n" + bottomLine
}
