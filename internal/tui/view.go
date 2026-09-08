package tui

import (
	"fmt"
	"strings"

	"github.com/brenoniehues/oh-my-logs/internal/record"
	"github.com/charmbracelet/lipgloss"
)

// View renders the full terminal UI.
func (m Model) View() string {
	if m.width == 0 {
		return "Initializing…"
	}

	var sb strings.Builder

	// 1. Title bar
	sb.WriteString(m.viewTitleBar())
	sb.WriteByte('\n')

	// 2. Table header
	sb.WriteString(m.viewTableHeader())
	sb.WriteByte('\n')

	// 3. Table rows
	sb.WriteString(m.viewTable())
	sb.WriteByte('\n')

	// 4. Status bar
	sb.WriteString(m.viewStatusBar())
	sb.WriteByte('\n')

	// 5. Key hints / input bar
	sb.WriteString(m.viewKeyBar())

	return sb.String()
}

// viewTitleBar renders the top bar with port, baud, connection state, profile.
func (m Model) viewTitleBar() string {
	port := m.serialCfg.Port
	if port == "" {
		port = "(no port)"
	}
	baud := fmt.Sprintf("%d", m.serialCfg.Baud)

	var connStr string
	switch m.connState {
	case ConnConnected:
		connStr = styleConnected.Render("● Connected")
	case ConnError:
		connStr = styleError.Render("⚠ " + m.connDetail)
	default:
		connStr = styleDisconnected.Render("○ Disconnected")
	}

	profileName := "(no profile)"
	if m.profile != nil {
		profileName = m.profile.Name
	}

	title := fmt.Sprintf(" Oh My Logs  │  Port: %s  Baud: %s  %s  Profile: %s",
		port, baud, connStr, profileName)

	return styleTitleBar.Width(m.width).Render(title)
}

// viewTableHeader renders the column header row.
func (m Model) viewTableHeader() string {
	tableWidth := m.tableWidth()
	return styleTableHeader.Width(tableWidth).Render(m.renderRow(func(col record.Column, w int) string {
		return padOrTrunc(col.Title, w)
	}))
}

// viewTable renders the scrollable table body.
func (m Model) viewTable() string {
	tableWidth := m.tableWidth()
	rows := m.visibleRows()
	matchSet := m.searchMatchSet()

	var lines []string
	for i, r := range rows {
		absIdx := m.scrollOffset + i
		isMatch := matchSet[absIdx]

		rendered := m.renderRow(func(col record.Column, w int) string {
			val := r.Fields[col.Field]
			cell := padOrTrunc(val, w)
			if isMatch && m.searchInput != "" {
				cell = highlightSubstring(cell, m.searchInput)
			}
			return cell
		})

		if isMatch {
			lines = append(lines, styleTableRowSelected.Width(tableWidth).Render(rendered))
		} else {
			lines = append(lines, styleTableRow.Width(tableWidth).Render(rendered))
		}
	}

	// Pad to fill the table area.
	for len(lines) < m.tableHeight {
		lines = append(lines, styleTableRow.Width(tableWidth).Render(""))
	}

	return strings.Join(lines, "\n")
}

// viewStatusBar renders the footer status line.
func (m Model) viewStatusBar() string {
	total := m.buffer.Len()
	shown := len(m.visible)

	followStr := styleFollowOn.Render("FOLLOW")
	if !m.follow {
		followStr = styleFollowOff.Render("PAUSED")
	}

	var parts []string
	parts = append(parts, fmt.Sprintf("%d records", total))
	if shown != total {
		parts = append(parts, fmt.Sprintf("%d shown", shown))
	}
	if m.activeFilter != nil && !m.activeFilter.Empty() {
		parts = append(parts, fmt.Sprintf("filter: %s", m.activeFilter.Raw))
	}
	if len(m.searchMatches) > 0 {
		parts = append(parts, fmt.Sprintf("match %d/%d", m.searchCursor+1, len(m.searchMatches)))
	}
	if m.message != "" {
		parts = append(parts, styleMsg.Render(m.message))
	}
	parts = append(parts, followStr)

	return styleStatusBar.Width(m.width).Render(strings.Join(parts, "  │  "))
}

// viewKeyBar renders the key hints at the bottom, or an active input field.
func (m Model) viewKeyBar() string {
	switch m.mode {
	case modeSearch:
		prompt := styleInputPrompt.Render("Search: ")
		text := styleInputText.Render(m.searchInput + "█")
		return styleKeyHint.Width(m.width).Render(prompt + text)
	case modeFilter:
		prompt := styleInputPrompt.Render("Filter: ")
		text := styleInputText.Render(m.filterInput + "█")
		return styleKeyHint.Width(m.width).Render(prompt + text)
	default:
		hints := []string{
			styleKeyName.Render("Ctrl+F") + " search",
			styleKeyName.Render("f") + " filter",
			styleKeyName.Render("c") + " clear",
			styleKeyName.Render("Space") + " pause",
			styleKeyName.Render("s") + " save",
			styleKeyName.Render("r") + " reconnect",
			styleKeyName.Render("q") + " quit",
		}
		return styleKeyHint.Width(m.width).Render(strings.Join(hints, "   "))
	}
}

// ── Internal helpers ──────────────────────────────────────────────────────────

// visibleRows returns the slice of records currently in the viewport.
func (m Model) visibleRows() []record.Record {
	if len(m.visible) == 0 {
		return nil
	}
	end := m.scrollOffset + m.tableHeight
	if end > len(m.visible) {
		end = len(m.visible)
	}
	if m.scrollOffset >= end {
		return nil
	}
	return m.visible[m.scrollOffset:end]
}

// tableWidth is the usable width for the table area.
func (m Model) tableWidth() int {
	return m.width
}

// renderRow calls cellFn for each column and joins them.
func (m Model) renderRow(cellFn func(col record.Column, width int) string) string {
	colWidths := m.computeColWidths()
	var parts []string
	for i, col := range m.columns {
		parts = append(parts, cellFn(col, colWidths[i]))
	}
	return strings.Join(parts, " ")
}

// computeColWidths distributes available width across columns.
// Columns with Width > 0 get their fixed width; Width == 0 fills the rest.
func (m Model) computeColWidths() []int {
	total := m.tableWidth()
	widths := make([]int, len(m.columns))
	flexIdx := -1
	used := 0

	for i, col := range m.columns {
		if col.Width == 0 {
			flexIdx = i
		} else {
			widths[i] = col.Width
			used += col.Width + 1 // +1 for separator space
		}
	}

	if flexIdx >= 0 {
		flex := total - used - 1
		if flex < 0 {
			flex = 0
		}
		widths[flexIdx] = flex
	}

	return widths
}

// searchMatchSet returns a set of visible-row indices that are search matches.
func (m Model) searchMatchSet() map[int]bool {
	if len(m.searchMatches) == 0 {
		return nil
	}
	set := make(map[int]bool, len(m.searchMatches))
	for _, idx := range m.searchMatches {
		set[idx] = true
	}
	return set
}

// padOrTrunc pads s to width or truncates it (with "…") if too long.
func padOrTrunc(s string, width int) string {
	if width <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) > width {
		if width > 1 {
			return string(runes[:width-1]) + "…"
		}
		return "…"
	}
	return s + strings.Repeat(" ", width-len(runes))
}

// highlightSubstring wraps occurrences of q (case-insensitive) in the cell
// with the highlight style.
func highlightSubstring(cell, q string) string {
	lower := strings.ToLower(cell)
	lowerQ := strings.ToLower(q)
	idx := strings.Index(lower, lowerQ)
	if idx < 0 {
		return cell
	}
	before := cell[:idx]
	match := cell[idx : idx+len(q)]
	after := cell[idx+len(q):]
	return before + styleHighlight.Render(match) + after
}

// Ensure lipgloss is used (it's referenced via styles.go but let's be safe).
var _ = lipgloss.NewStyle
