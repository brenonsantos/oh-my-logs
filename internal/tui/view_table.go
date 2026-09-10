package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/brenoniehues/oh-my-logs/internal/record"
	"github.com/brenoniehues/oh-my-logs/internal/timing"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// ansiCut extracts visual substring [offset : offset+width] from an ANSI-styled string.
func ansiCut(s string, offset int, width int) string {
	if width <= 0 {
		return ""
	}
	if offset <= 0 {
		return ansi.Truncate(s, width, "")
	}
	leftTrimmed := ansi.TruncateLeft(s, offset, "")
	return ansi.Truncate(leftTrimmed, width, "")
}

// padOrFlex pads s to width, or if isFlex is true and len(runes) >= width, returns s without truncation.
func padOrFlex(s string, width int, isFlex bool) string {
	if width <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) > width {
		if isFlex {
			return s
		}
		if width > 1 {
			return string(runes[:width-1]) + "…"
		}
		return "…"
	}
	return s + strings.Repeat(" ", width-len(runes))
}

// applyRowCursor overlays a character-level cursor and selection highlight on a rendered rowBody.
func applyRowCursor(rowBody string, cursorCol int, selStart int, selEnd int) string {
	if cursorCol < 0 {
		return rowBody
	}
	w := lipgloss.Width(rowBody)
	if w == 0 {
		cursorStyle := lipgloss.NewStyle().Background(colorCyan).Foreground(lipgloss.Color("#000000")).Bold(true)
		return cursorStyle.Render(" ")
	}

	if cursorCol >= w {
		rowBody += strings.Repeat(" ", cursorCol-w+1)
		w = cursorCol + 1
	}

	cursorStyle := lipgloss.NewStyle().Background(colorCyan).Foreground(lipgloss.Color("#000000")).Bold(true)
	selStyle := lipgloss.NewStyle().Background(colorAccent).Foreground(lipgloss.Color("#000000"))

	hasSel := selStart >= 0 && selEnd >= 0 && selStart != selEnd
	if !hasSel {
		before := ansiCut(rowBody, 0, cursorCol)
		curChar := ansiCut(rowBody, cursorCol, 1)
		if curChar == "" {
			curChar = " "
		}
		after := ansiCut(rowBody, cursorCol+1, w-(cursorCol+1))
		return before + cursorStyle.Render(ansi.Strip(curChar)) + after
	}

	minSel, maxSel := selStart, selEnd
	if minSel > maxSel {
		minSel, maxSel = maxSel, minSel
	}
	if minSel < 0 {
		minSel = 0
	}
	if maxSel > w {
		maxSel = w
	}

	left := ansiCut(rowBody, 0, minSel)
	right := ansiCut(rowBody, maxSel, w-maxSel)

	var mid string
	if cursorCol == minSel {
		curChar := ansiCut(rowBody, cursorCol, 1)
		if curChar == "" {
			curChar = " "
		}
		selRest := ansiCut(rowBody, minSel+1, maxSel-(minSel+1))
		mid = cursorStyle.Render(ansi.Strip(curChar)) + selStyle.Render(ansi.Strip(selRest))
	} else if cursorCol >= maxSel {
		selPart := ansiCut(rowBody, minSel, maxSel-minSel)
		curChar := ansiCut(rowBody, cursorCol, 1)
		if curChar == "" {
			curChar = " "
		}
		afterCursor := ansiCut(rowBody, cursorCol+1, w-(cursorCol+1))
		return left + selStyle.Render(ansi.Strip(selPart)) + cursorStyle.Render(ansi.Strip(curChar)) + afterCursor
	} else {
		selBefore := ansiCut(rowBody, minSel, cursorCol-minSel)
		curChar := ansiCut(rowBody, cursorCol, 1)
		if curChar == "" {
			curChar = " "
		}
		selAfter := ansiCut(rowBody, cursorCol+1, maxSel-(cursorCol+1))
		mid = selStyle.Render(ansi.Strip(selBefore)) + cursorStyle.Render(ansi.Strip(curChar)) + selStyle.Render(ansi.Strip(selAfter))
	}

	return left + mid + right
}

// selectedRowPlainText returns the plain unstyled text of the row at absIdx.
func (m Model) selectedRowPlainText(absIdx int) string {
	if absIdx < 0 || absIdx >= len(m.visible) {
		return ""
	}
	r := m.visible[absIdx]
	cols := m.effectiveColumns()
	colWidths := m.computeColWidths(cols)

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
		if col.Field == "_delta" || col.Style == "delta" {
			if absIdx > 0 && !m.visible[absIdx-1].Timestamp.IsZero() && !r.Timestamp.IsZero() {
				val = timing.FormatDelta(r.Timestamp.Sub(m.visible[absIdx-1].Timestamp))
			} else if r.Delta > 0 {
				val = timing.FormatDelta(r.Delta)
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
		cellParts = append(cellParts, padOrFlex(val, w, col.Width == 0))
	}
	return strings.Join(cellParts, "  ")
}

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
	return m.computeColWidthsForWidth(cols, m.tableWidth())
}

// computeColWidthsForWidth distributes available width across columns for a given container width.
func (m Model) computeColWidthsForWidth(cols []record.Column, tableW int) []int {
	if len(cols) == 0 {
		return nil
	}
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

	// In narrow viewports, shrink hex or bin column if present to ensure flex column (e.g. ascii) has room
	if flexIdx >= 0 && (tableW-used) < 15 {
		for i, col := range cols {
			if (col.Field == "_hex" || col.Field == "_bin") && widths[i] > 18 {
				deficit := 15 - (tableW - used)
				shrink := widths[i] - 18
				if shrink > deficit {
					shrink = deficit
				}
				widths[i] -= shrink
				used -= shrink
				break
			}
		}
	}

	if flexIdx >= 0 {
		flex := tableW - used
		if flex < 8 {
			flex = 8
		}
		widths[flexIdx] = flex
	}

	return widths
}

// maxContentWidthForTab computes the maximum visual width across all visible rows for a given tab.
// If all rows fit within containerW, it returns containerW so no scrollbar is needed.
func (m Model) maxContentWidthForTab(tab *Tab, containerW int) int {
	if tab == nil || len(tab.Visible) == 0 || containerW <= 0 {
		return containerW
	}
	cols := m.effectiveColumnsForTab(tab)
	if len(cols) == 0 {
		return containerW
	}
	colWidths := m.computeColWidthsForWidth(cols, containerW)

	fixedW := 3
	if len(cols) > 1 {
		fixedW += (len(cols) - 1) * 2
	}

	flexIdx := -1
	for i, col := range cols {
		if col.Width == 0 {
			flexIdx = i
		} else if i < len(colWidths) {
			fixedW += colWidths[i]
		}
	}

	if flexIdx < 0 {
		if fixedW < containerW {
			return containerW
		}
		return fixedW
	}

	flexBaseW := 0
	if flexIdx < len(colWidths) {
		flexBaseW = colWidths[flexIdx]
	}

	flexCol := cols[flexIdx]
	maxFlex := flexBaseW

	for _, r := range tab.Visible {
		var val string
		switch flexCol.Field {
		case "raw":
			val = r.Raw
		case "_ascii":
			val = r.Raw
		case "message":
			val = r.Fields["message"]
			if val == "" {
				val = r.Raw
			}
		default:
			val = r.Fields[flexCol.Field]
		}
		l := len([]rune(val))
		if l > maxFlex {
			maxFlex = l
		}
	}

	totalW := fixedW + maxFlex
	if totalW < containerW {
		return containerW
	}
	return totalW
}

// maxContentWidth computes the maximum visual width for the active tab across tableWidth.
func (m Model) maxContentWidth() int {
	cur := m.currentTab()
	return m.maxContentWidthForTab(cur, m.tableWidth())
}

// renderPaneView renders an isolated pane view for a virtual tab, returning exact paneH lines,
// each formatted and padded to paneW characters.
func (m Model) renderPaneView(tab *Tab, tabIdx int, paneW int, paneH int, isFocused bool) []string {
	if paneH <= 0 || paneW <= 0 {
		return nil
	}

	var lines []string

	// If tab is nil, fill with blank lines
	if tab == nil {
		for len(lines) < paneH {
			lines = append(lines, strings.Repeat(" ", paneW))
		}
		return lines
	}

	cols := m.effectiveColumnsForTab(tab)
	colWidths := m.computeColWidthsForWidth(cols, paneW)

	// Line 0: Header
	var prefix string
	if isFocused {
		prefix = theme.Accent.Bold(true).Render(fmt.Sprintf("▶%d ", tabIdx+1))
	} else {
		prefix = theme.Muted.Render(fmt.Sprintf(" %d ", tabIdx+1))
	}

	var colHeaderParts []string
	for i, col := range cols {
		w := 0
		if i < len(colWidths) {
			w = colWidths[i]
		}
		colHeaderParts = append(colHeaderParts, theme.Header.Render(padOrTrunc(col.Title, w)))
	}
	headerCols := strings.Join(colHeaderParts, "  ")
	availW := paneW - 3
	if availW < 0 {
		availW = 0
	}
	slicedHeaders := ansiCut(headerCols, tab.ScrollX, availW)
	headerLine := prefix + slicedHeaders

	displayName := tab.DisplayName(tabIdx + 1)
	tag := fmt.Sprintf("[%d: %s]", tabIdx+1, displayName)
	if tab.Filter != nil && !tab.Filter.Empty() {
		tag = fmt.Sprintf("[%d: %s · %s]", tabIdx+1, displayName, tab.Filter.Raw)
	}

	headerW := lipgloss.Width(headerLine)
	tagW := lipgloss.Width(tag)
	if paneW-headerW >= tagW+2 {
		gap := paneW - headerW - tagW
		var styledTag string
		if isFocused {
			styledTag = theme.Accent.Bold(true).Render(tag)
		} else {
			styledTag = theme.Muted.Render(tag)
		}
		headerLine = headerLine + strings.Repeat(" ", gap) + styledTag
	} else if headerW < paneW {
		headerLine = headerLine + strings.Repeat(" ", paneW-headerW)
	} else if headerW > paneW {
		headerLine = lipgloss.NewStyle().MaxWidth(paneW).Render(headerLine)
	}
	lines = append(lines, headerLine)

	if len(lines) >= paneH {
		return lines[:paneH]
	}

	// Line 1: Header divider
	var dividerLine string
	if isFocused {
		dividerLine = theme.Accent.Render(strings.Repeat("━", paneW))
	} else {
		dividerLine = theme.Divider.Render(strings.Repeat("─", paneW))
	}
	if lipgloss.Width(dividerLine) > paneW {
		dividerLine = lipgloss.NewStyle().MaxWidth(paneW).Render(dividerLine)
	}
	lines = append(lines, dividerLine)

	if len(lines) >= paneH {
		return lines[:paneH]
	}

	// Lines 2 to paneH - 1: Data rows
	dataH := paneH - 2
	visible := tab.Visible
	offset := tab.ScrollOffset

	if len(visible) == 0 {
		// Empty state for this pane
		emptyMsg := "No logs match filter"
		if tab.Filter == nil || tab.Filter.Empty() {
			emptyMsg = "No logs in tab"
		}
		midY := dataH / 2
		for i := 0; i < dataH; i++ {
			if i == midY {
				styledMsg := theme.Muted.Render(emptyMsg)
				msgW := lipgloss.Width(styledMsg)
				leftPad := (paneW - msgW) / 2
				if leftPad < 0 {
					leftPad = 0
				}
				row := strings.Repeat(" ", leftPad) + styledMsg
				curW := lipgloss.Width(row)
				if curW < paneW {
					row += strings.Repeat(" ", paneW-curW)
				}
				lines = append(lines, row)
			} else {
				lines = append(lines, strings.Repeat(" ", paneW))
			}
		}
		return lines
	}

	if offset < 0 {
		offset = 0
	}
	if offset > len(visible)-dataH {
		offset = len(visible) - dataH
	}
	if offset < 0 {
		offset = 0
	}
	end := offset + dataH
	if end > len(visible) {
		end = len(visible)
	}

	// Build search match set for tab
	matchSet := make(map[int]bool)
	for _, idx := range tab.SearchMatches {
		matchSet[idx] = true
	}
	focusedSearchAbsIdx := -1
	if len(tab.SearchMatches) > 0 && tab.SearchCursor >= 0 && tab.SearchCursor < len(tab.SearchMatches) {
		focusedSearchAbsIdx = tab.SearchMatches[tab.SearchCursor]
	}
	totalRows := len(visible)
	hasVScroll := totalRows > dataH && dataH > 1
	thumbH := 1
	thumbTop := 0
	if hasVScroll {
		thumbH = dataH * dataH / totalRows
		if thumbH < 1 {
			thumbH = 1
		}
		maxOffset := totalRows - dataH
		if maxOffset > 0 {
			thumbTop = offset * (dataH - thumbH) / maxOffset
		}
		if thumbTop < 0 {
			thumbTop = 0
		}
		if thumbTop+thumbH > dataH {
			thumbTop = dataH - thumbH
		}
	}

	paneRows := visible[offset:end]
	for i, r := range paneRows {
		absIdx := offset + i
		isMatch := matchSet[absIdx]
		isSearchFocused := (absIdx == focusedSearchAbsIdx)

		isMultiSelected := false
		if isFocused && m.selectionStart >= 0 && m.selectionEnd >= 0 && m.selectionStart != m.selectionEnd {
			minSel, maxSel := m.selectionStart, m.selectionEnd
			if minSel > maxSel {
				minSel, maxSel = maxSel, minSel
			}
			if absIdx >= minSel && absIdx <= maxSel {
				isMultiSelected = true
			}
		}
		isSelectedRow := (absIdx == tab.SelectedRow)
		_, isBookmarked := m.bookmarks[r.ID]

		var rowBg lipgloss.TerminalColor
		hasBg := false
		var renderedPrefix string

		switch {
		case isSearchFocused:
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
			fgColor := colorAccent
			if !isFocused {
				fgColor = colorMuted
			}
			cStyle := lipgloss.NewStyle().Background(rowBg).Foreground(fgColor).Bold(true)
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
			var rowDelta time.Duration
			if col.Field == "_delta" || col.Style == "delta" {
				if i > 0 && !paneRows[i-1].Timestamp.IsZero() && !r.Timestamp.IsZero() {
					rowDelta = r.Timestamp.Sub(paneRows[i-1].Timestamp)
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
			if isMatch && tab.SearchInput != "" {
				renderedCell = highlightSubstring(cellText, tab.SearchInput, cellStyle)
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
		if isSelectedRow && isFocused && tab.CursorCol >= 0 {
			rowBody = applyRowCursor(rowBody, tab.CursorCol, tab.CharSelStart, tab.CharSelEnd)
		}

		contentW := paneW
		vScrollChar := ""
		if hasVScroll {
			contentW = paneW - 1
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
		slicedRow := ansiCut(rowBody, tab.ScrollX, availW)
		fullRow := renderedPrefix + slicedRow

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

	// Pad remaining vertical lines if needed
	for len(lines) < paneH {
		i := len(lines) - 2
		contentW := paneW
		vScrollChar := ""
		if hasVScroll && i >= 0 && i < dataH {
			contentW = paneW - 1
			if i >= thumbTop && i < thumbTop+thumbH {
				vScrollChar = theme.Accent.Bold(true).Render("█")
			} else {
				vScrollChar = theme.Divider.Render("│")
			}
		}
		lines = append(lines, strings.Repeat(" ", contentW)+vScrollChar)
	}

	return lines
}

// viewSplitTable renders dual virtual tabs side-by-side (vertical) or stacked (horizontal).
func (m Model) viewSplitTable() string {
	m.syncActiveTabToModel()
	totalH := m.tableHeight + 2

	t0 := m.currentTabForPane(0)
	t0Idx := m.paneTabIdx(0)
	t1 := m.currentTabForPane(1)
	t1Idx := m.paneTabIdx(1)

	if m.splitMode == SplitVertical {
		leftW := (m.width - 1) / 2
		rightW := m.width - 1 - leftW

		leftLines := m.renderPaneView(t0, t0Idx, leftW, totalH, m.activePane == 0)
		rightLines := m.renderPaneView(t1, t1Idx, rightW, totalH, m.activePane == 1)

		var joined []string
		for i := 0; i < totalH; i++ {
			l := ""
			if i < len(leftLines) {
				l = leftLines[i]
			}
			r := ""
			if i < len(rightLines) {
				r = rightLines[i]
			}
			sep := "│"
			if i == 1 {
				sep = "┼"
			}
			sepStyle := theme.Divider
			if i == 1 && (m.activePane == 0 || m.activePane == 1) {
				sepStyle = theme.Accent
			}
			row := l + sepStyle.Render(sep) + r
			if lipgloss.Width(row) > m.width {
				row = lipgloss.NewStyle().MaxWidth(m.width).Render(row)
			}
			joined = append(joined, row)
		}
		return strings.Join(joined, "\n")
	}

	// SplitHorizontal: stacked top and bottom
	availH := totalH - 1
	topH := availH / 2
	bottomH := availH - topH
	if topH < 3 {
		topH = 3
	}
	if bottomH < 3 {
		bottomH = 3
	}

	topLines := m.renderPaneView(t0, t0Idx, m.width, topH, m.activePane == 0)
	bottomLines := m.renderPaneView(t1, t1Idx, m.width, bottomH, m.activePane == 1)

	divLine := theme.Divider.Render(strings.Repeat("─", m.width))
	if m.activePane == 0 {
		divLine = theme.Accent.Render(strings.Repeat("━", m.width))
	}
	if lipgloss.Width(divLine) > m.width {
		divLine = lipgloss.NewStyle().MaxWidth(m.width).Render(divLine)
	}

	var all []string
	all = append(all, topLines...)
	all = append(all, divLine)
	all = append(all, bottomLines...)

	// Guarantee exact totalH lines
	if len(all) > totalH {
		all = all[:totalH]
	}
	for len(all) < totalH {
		all = append(all, strings.Repeat(" ", m.width))
	}
	return strings.Join(all, "\n")
}

func isTimestampCol(col record.Column, tsField string) bool {
	// Uptime is device uptime, never a toggleable arrival timestamp column
	if col.Field == "uptime" || strings.EqualFold(col.Style, "uptime") {
		return false
	}
	if col.Field == "_delta" || col.Style == "delta" {
		return false
	}
	return col.Field == tsField || col.Field == "_ts" || col.Style == "timestamp"
}

func isDeltaCol(col record.Column) bool {
	return col.Field == "_delta" || col.Style == "delta"
}

// effectiveColumns returns the columns to render, respecting timestamp and delta mode for the current tab.
func (m Model) effectiveColumns() []record.Column {
	return m.effectiveColumnsForTab(m.currentTab())
}

// effectiveColumnsForTab returns the columns to render for a specific tab based on its DisplayFormat and timestamp mode.
func (m Model) effectiveColumnsForTab(tab *Tab) []record.Column {
	fmtMode := m.displayFormat
	if tab != nil {
		fmtMode = tab.DisplayFormat
	}

	var baseCols []record.Column
	switch fmtMode {
	case FormatRaw:
		baseCols = []record.Column{
			{Field: "raw", Title: "RAW LOG", Width: 0, Style: "primary"},
		}
	case FormatHex:
		baseCols = []record.Column{
			{Field: "_len", Title: "LEN", Width: 6, Style: "identifier"},
			{Field: "_hex", Title: "HEX DUMP", Width: 48, Style: "muted"},
			{Field: "_ascii", Title: "ASCII", Width: 0, Style: "primary"},
		}
	case FormatBinary:
		baseCols = []record.Column{
			{Field: "_len", Title: "LEN", Width: 6, Style: "identifier"},
			{Field: "_bin", Title: "BINARY BITS", Width: 36, Style: "muted"},
			{Field: "_ascii", Title: "ASCII", Width: 0, Style: "primary"},
		}
	default: // FormatParsed
		for _, col := range m.columns {
			if !isTimestampCol(col, m.tsField) && !isDeltaCol(col) {
				baseCols = append(baseCols, col)
			}
		}
	}

	if m.tsMode == TSModeOff {
		return baseCols
	}

	tsCol := record.Column{
		Field: m.tsField,
		Title: "Time",
		Width: 14,
		Style: "timestamp",
	}
	deltaCol := record.Column{
		Field: "_delta",
		Title: "Δt",
		Width: 10,
		Style: "delta",
	}

	switch m.tsMode {
	case TSModeDelta:
		result := make([]record.Column, 0, len(baseCols)+1)
		result = append(result, deltaCol)
		result = append(result, baseCols...)
		return result

	case TSModeBoth:
		result := make([]record.Column, 0, len(baseCols)+2)
		result = append(result, tsCol, deltaCol)
		result = append(result, baseCols...)
		return result

	default: // TSModeClock
		result := make([]record.Column, 0, len(baseCols)+1)
		result = append(result, tsCol)
		result = append(result, baseCols...)
		return result
	}
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
