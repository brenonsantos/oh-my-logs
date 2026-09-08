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

	// 2. Divider line under title bar
	sb.WriteString(m.viewDivider())
	sb.WriteByte('\n')

	// 3. Middle area: Modals or Table
	if m.mode == modePortPicker {
		sb.WriteString(m.viewPortPickerModal())
	} else if m.mode == modeProfilePicker {
		sb.WriteString(m.viewProfilePickerModal())
	} else if m.mode == modeHelp {
		sb.WriteString(m.viewHelpModal())
	} else if len(m.visible) == 0 {
		sb.WriteString(m.viewEmptyState())
	} else {
		// Table Header
		sb.WriteString(m.viewTableHeader())
		sb.WriteByte('\n')

		// Header Divider
		sb.WriteString(m.viewDivider())
		sb.WriteByte('\n')

		// Table Rows
		sb.WriteString(m.viewTable())
	}

	sb.WriteByte('\n')

	// 4. Divider before status bar
	sb.WriteString(m.viewDivider())
	sb.WriteByte('\n')

	// 5. Status bar
	sb.WriteString(m.viewStatusBar())
	sb.WriteByte('\n')

	// 6. Key hints / input bar
	sb.WriteString(m.viewKeyBar())

	return sb.String()
}

// viewDivider renders a clean horizontal divider across the full terminal width.
func (m Model) viewDivider() string {
	w := m.tableWidth()
	if w <= 0 {
		return ""
	}
	return theme.Divider.Render(strings.Repeat("─", w))
}

// viewTitleBar renders the top bar with a solid background accent and no holes or clipping.
func (m Model) viewTitleBar() string {
	badge := theme.TitleAppBadge.Render("OH MY LOGS")

	portVal := m.serialCfg.Port
	if portVal == "" {
		portVal = "(no port)"
	}
	port := theme.TitleLabel.Render("Port: ") + theme.TitleValue.Render(portVal)
	baud := theme.TitleLabel.Render("Baud: ") + theme.TitleValue.Render(fmt.Sprintf("%d", m.serialCfg.Baud))

	var connStr string
	switch m.connState {
	case ConnConnected:
		connStr = theme.TitleConnOn.Render("● Connected")
	case ConnError:
		connStr = theme.TitleConnErr.Render("⚠ " + m.connDetail)
	default:
		if m.reconnecting && m.serialCfg.Port != "" && !m.isFileSource {
			connStr = theme.Accent.Background(theme.TitleBg).Bold(true).Render("⟳ Reconnecting…")
		} else {
			connStr = theme.TitleConnOff.Render("○ Disconnected")
		}
	}

	profileVal := "(no profile)"
	if m.profile != nil {
		profileVal = m.profile.Name
	}
	prof := theme.TitleLabel.Render("Profile: ") + theme.TitleValue.Render(profileVal)

	sep := theme.TitleSep.Render("  │  ")
	gap := lipgloss.NewStyle().Background(theme.TitleBg).Render("    ")
	padLeft := lipgloss.NewStyle().Background(theme.TitleBg).Render(" ")

	content := padLeft + badge + " " + port + gap + baud + gap + connStr + sep + prof

	// Fill the exact remainder of the terminal width with the background accent:
	contentWidth := lipgloss.Width(content)
	rem := m.width - contentWidth
	if rem > 0 {
		content += lipgloss.NewStyle().Background(theme.TitleBg).Render(strings.Repeat(" ", rem))
	}

	return content
}

// viewTableHeader renders the column header row.
func (m Model) viewTableHeader() string {
	renderedHeaders := m.renderRow(func(col record.Column, w int) string {
		return theme.Header.Render(padOrTrunc(col.Title, w))
	})
	return "  " + renderedHeaders
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

	var lines []string
	for i, r := range rows {
		absIdx := m.scrollOffset + i
		isMatch := matchSet[absIdx]
		isFocused := absIdx == focusedAbsIdx

		rendered := m.renderRow(func(col record.Column, w int) string {
			val := r.Fields[col.Field]
			cell := padOrTrunc(val, w)

			// If search active and matches, highlight the matching substring
			if isMatch && m.searchInput != "" {
				cell = highlightSubstring(cell, m.searchInput)
			} else {
				// Apply dynamic semantic or custom column style
				cellStyle := theme.ResolveCellStyle(col, val)
				cell = cellStyle.Render(cell)
			}
			return cell
		})

		switch {
		case isFocused:
			lines = append(lines, theme.SearchFocus.Width(tableWidth).Render("▶ "+rendered))
		case isMatch:
			lines = append(lines, theme.SearchMatch.Width(tableWidth).Render("  "+rendered))
		default:
			lines = append(lines, theme.RowNormal.Width(tableWidth).Render("  "+rendered))
		}
	}

	// Pad remaining vertical space to keep layout stable
	for len(lines) < m.tableHeight {
		lines = append(lines, theme.RowNormal.Width(tableWidth).Render(""))
	}

	return strings.Join(lines, "\n")
}

// viewEmptyState renders a centered, context-aware empty state message.
func (m Model) viewEmptyState() string {
	var title string
	var subtitle string

	switch {
	case m.connState != ConnConnected:
		title = theme.Secondary.Bold(true).Render("No serial device connected")
		if m.serialCfg.Port != "" && !m.isFileSource {
			subtitle = theme.Muted.Render("Auto-reconnecting to ") + theme.Accent.Render(m.serialCfg.Port) +
				theme.Muted.Render("   ·   Press ") + theme.KeyName.Render("p") +
				theme.Muted.Render(" to select a port   ·   Press ") +
				theme.KeyName.Render("?") + theme.Muted.Render(" for shortcuts")
		} else {
			subtitle = theme.Muted.Render("Press ") + theme.KeyName.Render("p") +
				theme.Muted.Render(" to select a port   ·   Press ") +
				theme.KeyName.Render("?") + theme.Muted.Render(" for shortcuts")
		}

	case m.buffer.Len() == 0:
		title = theme.Accent.Render("Waiting for serial data…")
		subtitle = theme.Muted.Render("Press ") + theme.KeyName.Render("?") +
			theme.Muted.Render(" for shortcuts")

	default:
		title = theme.Secondary.Bold(true).Render("No logs match current filter")
		subtitle = theme.Muted.Render("Press ") + theme.KeyName.Render("f") +
			theme.Muted.Render(" to edit filter   ·   Press ") +
			theme.KeyName.Render("?") + theme.Muted.Render(" for shortcuts")
	}

	contentLines := []string{
		title,
		"",
		subtitle,
	}

	tableWidth := m.tableWidth()
	totalHeight := m.tableHeight + 1 // replaces table header (1 line) + table rows (tableHeight lines)
	topPad := (totalHeight - len(contentLines)) / 2
	if topPad < 0 {
		topPad = 0
	}

	var lines []string
	for i := 0; i < topPad; i++ {
		lines = append(lines, theme.RowNormal.Width(tableWidth).Render(""))
	}

	for _, cl := range contentLines {
		if cl == "" {
			lines = append(lines, theme.RowNormal.Width(tableWidth).Render(""))
			continue
		}
		w := lipgloss.Width(cl)
		leftPad := (tableWidth - w) / 2
		if leftPad < 0 {
			leftPad = 0
		}
		padded := strings.Repeat(" ", leftPad) + cl
		lines = append(lines, theme.RowNormal.Width(tableWidth).Render(padded))
	}

	for len(lines) < totalHeight {
		lines = append(lines, theme.RowNormal.Width(tableWidth).Render(""))
	}

	return strings.Join(lines, "\n")
}

// viewStatusBar renders statistics, active filter/search info, and message.
func (m Model) viewStatusBar() string {
	total := m.buffer.Len()
	shown := len(m.visible)

	followStr := theme.FollowOn.Render("FOLLOW")
	if !m.follow {
		followStr = theme.FollowOff.Render("PAUSED")
	}

	tsStr := theme.Muted.Render("⏱ OFF")
	if m.showTimestamp {
		tsStr = theme.Success.Render("⏱ ON")
	}

	var parts []string
	parts = append(parts, theme.Muted.Render(fmt.Sprintf("%d records", total)))
	if shown != total {
		parts = append(parts, theme.Secondary.Render(fmt.Sprintf("%d shown", shown)))
	}
	if m.activeFilter != nil && !m.activeFilter.Empty() {
		parts = append(parts, theme.Accent.Render(fmt.Sprintf("filter: %s", m.activeFilter.Raw)))
	}
	if len(m.searchMatches) > 0 {
		matchInfo := fmt.Sprintf("match %d of %d", m.searchCursor+1, len(m.searchMatches))
		parts = append(parts, theme.Warning.Render(matchInfo))
	} else if m.searchInput != "" {
		parts = append(parts, theme.Muted.Render("no matches"))
	}

	parts = append(parts, tsStr)
	parts = append(parts, followStr)

	// Action message: styled gently in muted/info or error
	if m.message != "" {
		lowerMsg := strings.ToLower(m.message)
		if strings.Contains(lowerMsg, "err") || strings.Contains(lowerMsg, "fail") {
			parts = append(parts, theme.MsgErr.Render("• "+m.message))
		} else {
			parts = append(parts, theme.MsgInfo.Render("• "+m.message))
		}
	}

	sep := theme.Muted.Render("  │  ")
	return "  " + strings.Join(parts, sep)
}

// viewKeyBar renders context-sensitive key hints or input prompt.
func (m Model) viewKeyBar() string {
	switch m.mode {
	case modeSearch:
		prompt := theme.Primary.Render("Search: ")
		text := theme.Content.Render(m.searchInput + "█")
		help := theme.Muted.Render("  [Enter: next · ↑/↓: navigate · Esc: cancel]")
		return "  " + prompt + text + help

	case modeFilter:
		prompt := theme.Primary.Render("Filter: ")
		text := theme.Content.Render(m.filterInput + "█")
		help := theme.Muted.Render("  [Enter: apply · Esc: cancel]")
		return "  " + prompt + text + help

	case modeHelp:
		return "  " + theme.Muted.Render("Press ") + theme.KeyName.Render("?") + theme.Muted.Render(", ") + theme.KeyName.Render("Esc") + theme.Muted.Render(", or ") + theme.KeyName.Render("q") + theme.Muted.Render(" to close help")

	default:
		hints := []string{
			theme.KeyName.Render("Ctrl+F") + " " + theme.Muted.Render("search"),
			theme.KeyName.Render("f") + " " + theme.Muted.Render("filter"),
			theme.KeyName.Render("Space") + " " + theme.Muted.Render("pause"),
			theme.KeyName.Render("c") + " " + theme.Muted.Render("clear"),
			theme.KeyName.Render("t") + " " + theme.Muted.Render("⏱ ts"),
			theme.KeyName.Render("p") + " " + theme.Muted.Render("port"),
			theme.KeyName.Render("P") + " " + theme.Muted.Render("profile"),
			theme.KeyName.Render("?") + " " + theme.Muted.Render("help"),
			theme.KeyName.Render("q") + " " + theme.Muted.Render("quit"),
		}
		return "  " + strings.Join(hints, "   ")
	}
}

// viewPortPickerModal renders the centered rounded modal for Port & Baud Rate.
func (m Model) viewPortPickerModal() string {
	modalWidth := 54
	for _, p := range m.portList {
		if len(p)+10 > modalWidth {
			modalWidth = len(p) + 10
		}
	}
	if modalWidth > m.width-6 {
		modalWidth = m.width - 6
	}

	var sb strings.Builder

	// Section 1: Serial Port
	sb.WriteString(theme.ModalTitle.Render("Serial Port"))
	sb.WriteString("\n\n")

	if len(m.portList) == 0 {
		sb.WriteString(theme.Muted.Render("  (no ports detected)"))
		sb.WriteString("\n")
	} else {
		maxVisible := 6
		startIdx := 0
		if m.portCursor >= maxVisible {
			startIdx = m.portCursor - maxVisible + 1
		}
		endIdx := startIdx + maxVisible
		if endIdx > len(m.portList) {
			endIdx = len(m.portList)
		}

		for i := startIdx; i < endIdx; i++ {
			port := m.portList[i]
			if m.portPickerSection == 0 && i == m.portCursor {
				sb.WriteString(theme.ModalSelected.Render(fmt.Sprintf("  › %s", port)))
			} else if i == m.portCursor {
				sb.WriteString(theme.Accent.Render(fmt.Sprintf("  › %s", port)))
			} else {
				sb.WriteString(theme.ModalItem.Render(fmt.Sprintf("    %s", port)))
			}
			sb.WriteString("\n")
		}
		if len(m.portList) > maxVisible {
			sb.WriteString(theme.Muted.Render(fmt.Sprintf("    ... (%d more)", len(m.portList)-maxVisible)))
			sb.WriteString("\n")
		}
	}

	sb.WriteString("\n")

	// Section 2: Baud rate
	sb.WriteString(theme.ModalTitle.Render("Baud rate"))
	sb.WriteString("\n\n")

	baudVal := m.serialCfg.Baud
	if len(m.baudList) > 0 && m.baudCursor < len(m.baudList) {
		baudVal = m.baudList[m.baudCursor]
	}
	baudStr := fmt.Sprintf("%d", baudVal)

	if m.portPickerSection == 1 {
		sb.WriteString(theme.ModalSelected.Render(fmt.Sprintf("  › %s", baudStr)))
		sb.WriteString(theme.Muted.Render("  (←/→ to change)"))
	} else {
		sb.WriteString(theme.ModalItem.Render(fmt.Sprintf("    %s", baudStr)))
	}
	sb.WriteString("\n\n")

	// Footer
	sb.WriteString(theme.ModalFooter.Render("Enter select · Esc cancel"))

	modalBox := theme.ModalBox.Width(modalWidth).Render(sb.String())
	return centerBox(m.width, m.tableHeight+2, modalBox)
}

// viewProfilePickerModal renders the centered rounded modal for Profile switching.
func (m Model) viewProfilePickerModal() string {
	modalWidth := 48
	for _, p := range m.profileList {
		if len(p.Name)+10 > modalWidth {
			modalWidth = len(p.Name) + 10
		}
	}
	if modalWidth > m.width-6 {
		modalWidth = m.width - 6
	}

	var sb strings.Builder

	sb.WriteString(theme.ModalTitle.Render("Profile"))
	sb.WriteString("\n\n")

	if len(m.profileList) == 0 {
		sb.WriteString(theme.Muted.Render("  (no profiles found)"))
		sb.WriteString("\n")
	} else {
		for i, prof := range m.profileList {
			if i == m.profileCursor {
				sb.WriteString(theme.ModalSelected.Render(fmt.Sprintf("  › %s", prof.Name)))
			} else {
				sb.WriteString(theme.ModalItem.Render(fmt.Sprintf("    %s", prof.Name)))
			}
			sb.WriteString("\n")
		}
	}

	sb.WriteString("\n")
	sb.WriteString(theme.ModalFooter.Render("Enter select · Esc cancel"))

	modalBox := theme.ModalBox.Width(modalWidth).Render(sb.String())
	return centerBox(m.width, m.tableHeight+2, modalBox)
}

// viewHelpModal renders a clean floating modal with categorized keyboard shortcuts.
func (m Model) viewHelpModal() string {
	modalWidth := 74
	if m.width > 90 {
		modalWidth = 76
	}
	if modalWidth > m.width-4 {
		modalWidth = m.width - 4
	}
	if modalWidth < 40 {
		modalWidth = 40
	}

	colWidth := (modalWidth - 9) / 2
	if colWidth < 16 {
		colWidth = 16
	}

	renderItem := func(k, desc string, maxW int) string {
		keyWidth := 11
		if keyWidth > maxW-6 {
			keyWidth = maxW - 6
		}
		if keyWidth < 4 {
			keyWidth = 4
		}
		kStyled := theme.KeyName.Render(padOrTrunc(k, keyWidth))
		descMax := maxW - keyWidth - 1
		if descMax < 1 {
			descMax = 1
		}
		descStyled := theme.Content.Render(padOrTrunc(desc, descMax))
		return kStyled + " " + descStyled
	}

	renderHeader := func(title string, maxW int) string {
		h := theme.ModalSection.Render(title)
		w := lipgloss.Width(title)
		if w < maxW {
			return h + strings.Repeat(" ", maxW-w)
		}
		return h
	}

	left := []string{
		renderHeader("NAVIGATION", colWidth),
		renderItem("↑, k", "Scroll up 1 line", colWidth),
		renderItem("↓, j", "Scroll down 1 line", colWidth),
		renderItem("PgUp, ^U", "Page up", colWidth),
		renderItem("PgDn, ^D", "Page down", colWidth),
		renderItem("g, Home", "Jump to top", colWidth),
		renderItem("G, End", "Jump to bottom", colWidth),
		renderItem("Wheel", "Smooth scroll", colWidth),
		renderHeader("SEARCH & FILTER", colWidth),
		renderItem("Ctrl+F", "Search logs", colWidth),
		renderItem("Enter, ↓", "Next match", colWidth),
		renderItem("n / N", "Next / prev match", colWidth),
		renderItem("f", "Filter logs", colWidth),
		renderItem("Esc", "Cancel / clear", colWidth),
	}

	right := []string{
		renderHeader("ACTIONS & CONTROLS", colWidth),
		renderItem("Space", "Pause / resume", colWidth),
		renderItem("c", "Clear buffer", colWidth),
		renderItem("t", "Toggle timestamp", colWidth),
		renderItem("s", "Save log to file", colWidth),
		renderItem("p", "Serial port & baud", colWidth),
		renderItem("P", "Switch profile", colWidth),
		renderItem("r", "Reconnect port", colWidth),
		renderItem("?", "Toggle this help", colWidth),
		renderItem("q, ^C", "Quit application", colWidth),
		renderHeader("FILTER SYNTAX", colWidth),
		renderItem("term", "Include substring", colWidth),
		renderItem("-term", "Exclude substring", colWidth),
		renderItem("col:v1,v2", "Match field (OR)", colWidth),
	}

	sep := theme.Divider.Render(" │ ")
	var sb strings.Builder

	sb.WriteString(theme.ModalTitle.Render("Help — Keyboard Shortcuts"))
	sb.WriteString("\n\n")

	maxRows := len(left)
	if len(right) > maxRows {
		maxRows = len(right)
	}

	for i := 0; i < maxRows; i++ {
		lLine := ""
		if i < len(left) {
			lLine = left[i]
		}
		rLine := ""
		if i < len(right) {
			rLine = right[i]
		}
		lWidth := lipgloss.Width(lLine)
		if lWidth < colWidth {
			lLine += strings.Repeat(" ", colWidth-lWidth)
		}
		sb.WriteString(lLine)
		sb.WriteString(sep)
		sb.WriteString(rLine)
		sb.WriteByte('\n')
	}

	sb.WriteByte('\n')
	sb.WriteString(theme.ModalFooter.Render("Press ? or Esc or q to close"))

	modalBox := theme.ModalBox.Padding(0, 2).Width(modalWidth).Render(sb.String())
	return centerBox(m.width, m.tableHeight+2, modalBox)
}

// centerBox centers a multi-line box horizontally and vertically within target dimensions.
func centerBox(screenWidth, screenHeight int, box string) string {
	lines := strings.Split(box, "\n")
	boxHeight := len(lines)
	topPad := (screenHeight - boxHeight) / 2
	if topPad < 0 {
		topPad = 0
	}

	var res strings.Builder
	for i := 0; i < topPad; i++ {
		res.WriteString(strings.Repeat(" ", screenWidth))
		res.WriteByte('\n')
	}
	for _, l := range lines {
		w := lipgloss.Width(l)
		leftPad := (screenWidth - w) / 2
		if leftPad < 0 {
			leftPad = 0
		}
		res.WriteString(strings.Repeat(" ", leftPad))
		res.WriteString(l)
		res.WriteByte('\n')
	}
	totalRendered := topPad + boxHeight
	for i := totalRendered; i < screenHeight; i++ {
		res.WriteString(strings.Repeat(" ", screenWidth))
		res.WriteByte('\n')
	}
	return strings.TrimRight(res.String(), "\n")
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
	total := m.tableWidth() - 4 // reserve prefix spaces
	widths := make([]int, len(cols))
	flexIdx := -1
	used := 0

	for i, col := range cols {
		if col.Width == 0 {
			flexIdx = i
		} else {
			widths[i] = col.Width
			used += col.Width + 2 // +2 for column spacing
		}
	}

	if flexIdx >= 0 {
		flex := total - used - 2
		if flex < 0 {
			flex = 0
		}
		widths[flexIdx] = flex
	}

	return widths
}

func isTimestampCol(col record.Column, tsField string) bool {
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

// highlightSubstring wraps occurrences of q with theme.Highlight.
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
	return before + theme.Highlight.Render(match) + after
}
