package tui

import (
	"strings"
	"time"

	"github.com/brenoniehues/oh-my-logs/internal/record"
	"github.com/brenoniehues/oh-my-logs/internal/timing"
	"github.com/charmbracelet/lipgloss"
)

// viewTableHeader renders the column header row with horizontal window panning.
func (m Model) viewTableHeader() string {
	availW := m.tableWidth() - 3
	if availW < 0 {
		availW = 0
	}
	renderedHeaders := m.renderRow(func(col record.Column, w int) string {
		return theme.Header.Render(padOrTrunc(col.Title, w))
	})
	slicedHeaders := ansiCut(renderedHeaders, m.scrollX, availW)
	headerLine := "   " + slicedHeaders
	tw := m.tableWidth()
	if curW := lipgloss.Width(headerLine); curW < tw {
		headerLine += strings.Repeat(" ", tw-curW)
	} else if curW > tw {
		headerLine = lipgloss.NewStyle().MaxWidth(tw).Render(headerLine)
	}
	return headerLine
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

	totalRows := len(m.visible)
	hasVScroll := totalRows > m.tableHeight && m.tableHeight > 1
	thumbH := 1
	thumbTop := 0
	if hasVScroll {
		thumbH = m.tableHeight * m.tableHeight / totalRows
		if thumbH < 1 {
			thumbH = 1
		}
		maxOffset := totalRows - m.tableHeight
		if maxOffset > 0 {
			thumbTop = m.scrollOffset * (m.tableHeight - thumbH) / maxOffset
		}
		if thumbTop < 0 {
			thumbTop = 0
		}
		if thumbTop+thumbH > m.tableHeight {
			thumbTop = m.tableHeight - thumbH
		}
	}

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
		case r.Fields["level"] == "TX":
			hasBg = true
			rowBg = colorTxBg
			renderedPrefix = lipgloss.NewStyle().Background(rowBg).Render("   ")
		default:
			renderedPrefix = "   "
		}

		var cellParts []string
		for colIdx, col := range cols {
			val := r.Fields[col.Field]
			switch col.Field {
			case "raw":
				val = r.Raw
			case "message":
				if val == "" {
					val = r.Raw
				}
			case "_len":
				val = FormatByteLen(len(r.Raw))
			case "_hex":
				val = FormatHexBytes(r.Raw)
			case "_bin":
				val = FormatBinaryBits(r.Raw)
			case "_ascii":
				val = FormatASCII(r.Raw)
			}
			if (col.Field == "uptime" || strings.EqualFold(col.Style, "uptime")) && val == "" {
				val = r.Fields["_ts"]
			}
			var rowDelta time.Duration
			if col.Field == "_delta" || col.Style == "delta" {
				if i > 0 && !rows[i-1].Timestamp.IsZero() && !r.Timestamp.IsZero() {
					rowDelta = r.Timestamp.Sub(rows[i-1].Timestamp)
					val = timing.FormatDelta(rowDelta)
				} else if r.Delta > 0 {
					rowDelta = r.Delta
					val = timing.FormatDelta(rowDelta)
				} else if r.Fields["_delta"] != "" {
					val = r.Fields["_delta"]
				} else {
					val = "---"
				}
			}
			w := 0
			if colIdx < len(colWidths) {
				w = colWidths[colIdx]
			}
			cellText := padOrFlex(val, w, col.Width == 0)

			cellStyle := theme.ResolveCellStyle(col, val)
			if col.Field == "_delta" || col.Style == "delta" {
				if m.deltaTracker != nil && rowDelta > 0 {
					cellStyle = theme.DeltaStyle(m.deltaTracker.Classify(rowDelta))
				} else {
					cellStyle = theme.Muted
				}
			}
			if r.Fields["level"] == "TX" && (col.Field == "message" || col.Style == "primary" || col.Field == "raw") {
				cellStyle = cellStyle.Foreground(colorMaple)
			}
			if hasBg {
				cellStyle = cellStyle.Background(rowBg)
			}

			var renderedCell string
			if isMatch && m.searchInput.Value != "" {
				renderedCell = highlightSubstring(cellText, m.searchInput.Value, cellStyle)
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
		if isSelectedRow && m.cursorCol >= 0 {
			rowBody = applyRowCursor(rowBody, m.cursorCol, m.charSelStart, m.charSelEnd)
		}

		contentW := tableWidth
		vScrollChar := ""
		if hasVScroll {
			contentW = tableWidth - 1
			if i >= thumbTop && i < thumbTop+thumbH {
				vScrollChar = theme.Accent.Bold(true).Render("█")
			} else {
				vScrollChar = theme.Divider.Render("│")
			}
		}

		availW := contentW - 3
		if availW < 0 {
			availW = 0
		}
		slicedBody := ansiCut(rowBody, m.scrollX, availW)
		fullRow := renderedPrefix + slicedBody

		curW := lipgloss.Width(fullRow)
		if curW < contentW {
			rem := contentW - curW
			if hasBg {
				fullRow += lipgloss.NewStyle().Background(rowBg).Render(strings.Repeat(" ", rem))
			} else {
				fullRow += strings.Repeat(" ", rem)
			}
		} else if curW > contentW {
			fullRow = lipgloss.NewStyle().MaxWidth(contentW).Render(fullRow)
		}
		fullRow += vScrollChar

		lines = append(lines, fullRow)
	}

	// Pad remaining vertical space to keep layout stable
	for len(lines) < m.tableHeight {
		i := len(lines)
		contentW := tableWidth
		vScrollChar := ""
		if hasVScroll {
			contentW = tableWidth - 1
			if i >= thumbTop && i < thumbTop+thumbH {
				vScrollChar = theme.Accent.Bold(true).Render("█")
			} else {
				vScrollChar = theme.Divider.Render("│")
			}
		}
		lines = append(lines, strings.Repeat(" ", contentW)+vScrollChar)
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
