package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/brenoniehues/oh-my-logs/internal/timing"
	"github.com/charmbracelet/lipgloss"
)

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
