package tui

import (
	"strings"

	"github.com/brenoniehues/oh-my-logs/internal/record"
	"github.com/charmbracelet/lipgloss"
)

// viewTableHeader renders the column header row.
func (m Model) viewTableHeader() string {
	renderedHeaders := m.renderRow(func(col record.Column, w int) string {
		return theme.Header.Render(padOrTrunc(col.Title, w))
	})
	return "   " + renderedHeaders
}

// viewTable renders the scrollable table body with search match and focus highlights.
func (m Model) viewTable() string {
	tableWidth := m.tableWidth()
	rows := m.visibleRows()
	matchSet := m.searchMatchSet()

	focusedAbsIdx := -1
	if len(m.searchMatches) > 0 && m.searchCursor >= 0 && m.searchCursor < len(m.searchMatches) {
		focusedAbsIdx = m.searchMatches[m.searchCursor]
	}

	cols := m.effectiveColumns()
	colWidths := m.computeColWidths(cols)

	var lines []string
	for i, r := range rows {
		absIdx := m.scrollOffset + i
		isMatch := matchSet[absIdx]
		isFocused := absIdx == focusedAbsIdx

		isMultiSelected := false
		if m.selectionStart >= 0 && m.selectionEnd >= 0 && m.selectionStart != m.selectionEnd {
			minSel, maxSel := m.selectionStart, m.selectionEnd
			if minSel > maxSel {
				minSel, maxSel = maxSel, minSel
			}
			if absIdx >= minSel && absIdx <= maxSel {
				isMultiSelected = true
			}
		}
		isSelectedRow := absIdx == m.selectedRow
		_, isBookmarked := m.bookmarks[r.ID]

		var rowBg lipgloss.TerminalColor
		hasBg := false
		var renderedPrefix string

		switch {
		case isFocused:
			hasBg = true
			rowBg = colorSelected
			cStyle := lipgloss.NewStyle().Background(rowBg).Foreground(colorYellow).Bold(true)
			if isBookmarked {
				bStyle := lipgloss.NewStyle().Background(rowBg).Foreground(colorYellow).Bold(true)
				spStyle := lipgloss.NewStyle().Background(rowBg)
				renderedPrefix = cStyle.Render("▶") + bStyle.Render("★") + spStyle.Render(" ")
			} else {
				renderedPrefix = cStyle.Render("▶  ")
			}
		case isMultiSelected:
			hasBg = true
			rowBg = colorSelected
			cStyle := lipgloss.NewStyle().Background(rowBg).Foreground(colorAccent).Bold(true)
			if isBookmarked {
				bStyle := lipgloss.NewStyle().Background(rowBg).Foreground(colorYellow).Bold(true)
				spStyle := lipgloss.NewStyle().Background(rowBg)
				renderedPrefix = cStyle.Render("▌") + bStyle.Render("★") + spStyle.Render(" ")
			} else {
				renderedPrefix = cStyle.Render("▌  ")
			}
		case isSelectedRow:
			hasBg = true
			rowBg = colorSelected
			cStyle := lipgloss.NewStyle().Background(rowBg).Foreground(colorAccent).Bold(true)
			if isBookmarked {
				bStyle := lipgloss.NewStyle().Background(rowBg).Foreground(colorYellow).Bold(true)
				spStyle := lipgloss.NewStyle().Background(rowBg)
				renderedPrefix = cStyle.Render("▶") + bStyle.Render("★") + spStyle.Render(" ")
			} else {
				renderedPrefix = cStyle.Render("▶  ")
			}
		case isBookmarked:
			hasBg = true
			rowBg = colorBookmarkBg
			bStyle := lipgloss.NewStyle().Background(rowBg).Foreground(colorYellow).Bold(true)
			spStyle := lipgloss.NewStyle().Background(rowBg)
			renderedPrefix = spStyle.Render(" ") + bStyle.Render("★") + spStyle.Render(" ")
		case isMatch:
			hasBg = true
			rowBg = colorSearchBg
			renderedPrefix = lipgloss.NewStyle().Background(rowBg).Render("   ")
		default:
			renderedPrefix = "   "
		}

		var cellParts []string
		for colIdx, col := range cols {
			val := r.Fields[col.Field]
			w := 0
			if colIdx < len(colWidths) {
				w = colWidths[colIdx]
			}
			cellText := padOrTrunc(val, w)

			cellStyle := theme.ResolveCellStyle(col, val)
			if hasBg {
				cellStyle = cellStyle.Background(rowBg)
			}

			var renderedCell string
			if isMatch && m.searchInput != "" {
				renderedCell = highlightSubstring(cellText, m.searchInput, cellStyle)
			} else {
				renderedCell = cellStyle.Render(cellText)
			}
			cellParts = append(cellParts, renderedCell)
		}

		sep := "  "
		if hasBg {
			sep = lipgloss.NewStyle().Background(rowBg).Render("  ")
		}

		rowBody := strings.Join(cellParts, sep)
		fullRow := renderedPrefix + rowBody

		curW := lipgloss.Width(fullRow)
		if curW < tableWidth {
			rem := tableWidth - curW
			if hasBg {
				fullRow += lipgloss.NewStyle().Background(rowBg).Render(strings.Repeat(" ", rem))
			} else {
				fullRow += strings.Repeat(" ", rem)
			}
		}

		lines = append(lines, fullRow)
	}

	// Pad remaining vertical space to keep layout stable
	for len(lines) < m.tableHeight {
		lines = append(lines, theme.RowNormal.Width(tableWidth).Render(""))
	}

	return strings.Join(lines, "\n")
}

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
	cols := m.effectiveColumns()
	colWidths := m.computeColWidths(cols)
	var parts []string
	for i, col := range cols {
		parts = append(parts, cellFn(col, colWidths[i]))
	}
	return strings.Join(parts, "  ")
}

// computeColWidths distributes available width across columns.
func (m Model) computeColWidths(cols []record.Column) []int {
	if len(cols) == 0 {
		return nil
	}
	tableW := m.tableWidth()
	widths := make([]int, len(cols))
	flexIdx := -1
	used := 3 // prefix takes 3 chars: cursor (1) + bookmark (1) + gap (1)

	// Each gap between columns takes 2 spaces: "  "
	if len(cols) > 1 {
		used += (len(cols) - 1) * 2
	}

	for i, col := range cols {
		if col.Width == 0 {
			flexIdx = i
		} else {
			widths[i] = col.Width
			used += col.Width
		}
	}

	if flexIdx >= 0 {
		flex := tableW - used
		if flex < 0 {
			flex = 0
		}
		widths[flexIdx] = flex
	}

	return widths
}

func isTimestampCol(col record.Column, tsField string) bool {
	// Uptime is device uptime, never a toggleable arrival timestamp column
	if col.Field == "uptime" || strings.EqualFold(col.Style, "uptime") {
		return false
	}
	return col.Field == tsField || col.Field == "_ts" || col.Style == "timestamp"
}

// effectiveColumns returns the columns to render, respecting timestamp visibility.
func (m Model) effectiveColumns() []record.Column {
	if !m.showTimestamp {
		// Filter out any timestamp columns so they are hidden
		var cols []record.Column
		for _, col := range m.columns {
			if !isTimestampCol(col, m.tsField) {
				cols = append(cols, col)
			}
		}
		return cols
	}

	// When timestamp is enabled: if already present in columns, return m.columns
	for _, col := range m.columns {
		if isTimestampCol(col, m.tsField) {
			return m.columns
		}
	}

	// Otherwise, prepend the arrival timestamp column
	tsCol := record.Column{
		Field: m.tsField,
		Title: "Time",
		Width: 14,
		Style: "timestamp",
	}
	result := make([]record.Column, 0, len(m.columns)+1)
	result = append(result, tsCol)
	result = append(result, m.columns...)
	return result
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

// padOrTrunc pads s to width or truncates it with "…" if too long.
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

// highlightSubstring wraps occurrences of q with theme.Highlight, using baseStyle for non-matching portions.
func highlightSubstring(cell, q string, baseStyle lipgloss.Style) string {
	lower := strings.ToLower(cell)
	lowerQ := strings.ToLower(q)
	if lowerQ == "" {
		return baseStyle.Render(cell)
	}
	var sb strings.Builder
	start := 0
	for {
		idx := strings.Index(lower[start:], lowerQ)
		if idx < 0 {
			sb.WriteString(baseStyle.Render(cell[start:]))
			break
		}
		matchStart := start + idx
		matchEnd := matchStart + len(q)
		if matchStart > start {
			sb.WriteString(baseStyle.Render(cell[start:matchStart]))
		}
		sb.WriteString(theme.Highlight.Render(cell[matchStart:matchEnd]))
		start = matchEnd
	}
	return sb.String()
}
