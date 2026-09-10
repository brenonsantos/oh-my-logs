package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/brenoniehues/oh-my-logs/internal/record"
	"github.com/brenoniehues/oh-my-logs/internal/timing"
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
			cellText := padOrTrunc(val, w)

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

	if flexIdx >= 0 {
		flex := tableW - used
		if flex < 0 {
			flex = 0
		}
		widths[flexIdx] = flex
	}

	return widths
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
	headerLine := prefix + headerCols

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
			cellText := padOrTrunc(val, w)

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
		fullRow := renderedPrefix + rowBody

		curW := lipgloss.Width(fullRow)
		if curW < paneW {
			rem := paneW - curW
			if hasBg {
				fullRow += lipgloss.NewStyle().Background(rowBg).Render(strings.Repeat(" ", rem))
			} else {
				fullRow += strings.Repeat(" ", rem)
			}
		} else if curW > paneW {
			fullRow = lipgloss.NewStyle().MaxWidth(paneW).Render(fullRow)
		}

		lines = append(lines, fullRow)
	}

	// Pad remaining vertical lines if needed
	for len(lines) < paneH {
		lines = append(lines, strings.Repeat(" ", paneW))
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
			joined = append(joined, l+sepStyle.Render(sep)+r)
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
			{Field: "_hex", Title: "HEX DUMP", Width: 48, Style: "primary"},
			{Field: "_ascii", Title: "ASCII", Width: 0, Style: "muted"},
		}
	case FormatBinary:
		baseCols = []record.Column{
			{Field: "_len", Title: "LEN", Width: 6, Style: "identifier"},
			{Field: "_bin", Title: "BINARY BITS", Width: 72, Style: "primary"},
			{Field: "_ascii", Title: "ASCII", Width: 0, Style: "muted"},
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
