package tui

import (
	"fmt"
	"strings"

	"github.com/brenoniehues/oh-my-logs/internal/timing"
	"github.com/charmbracelet/lipgloss"
)

// viewDivider renders a clean horizontal divider across the full terminal width.
func (m Model) viewDivider() string {
	w := m.tableWidth()
	if w <= 0 {
		return ""
	}
	return theme.Divider.Render(strings.Repeat("─", w))
}

// viewTabBar renders the pill-based virtual tab switcher bar.
func (m Model) viewTabBar() string {
	w := m.tableWidth()
	if w <= 0 {
		return ""
	}
	activeIdx := m.activeTabIdx()
	var tabPills []string
	for i := range m.tabs {
		t := &m.tabs[i]
		displayName := t.DisplayName(i + 1)
		countStr := fmt.Sprintf("%d", len(t.Visible))
		tag := ""
		if m.splitMode != SplitNone {
			if i == m.paneTabIdx(0) {
				tag = " [P1]"
			} else if i == m.paneTabIdx(1) {
				tag = " [P2]"
			}
		}
		label := fmt.Sprintf("%d: %s (%s)%s", i+1, displayName, countStr, tag)
		if i == activeIdx {
			tabPills = append(tabPills, theme.TabActive.Render(label))
		} else {
			tabPills = append(tabPills, theme.TabInactive.Render(label))
		}
	}
	left := " " + strings.Join(tabPills, " ")
	hintsText := "Tab: cycle · ^T: new · ^W: close  "
	if m.splitMode != SplitNone {
		hintsText = "w: pane · S: sync · |/_: split · Tab: cycle  "
	}
	hints := theme.Muted.Render(hintsText)
	leftW := lipgloss.Width(left)
	hintsW := lipgloss.Width(hints)
	gap := w - leftW - hintsW
	if gap > 0 {
		return left + strings.Repeat(" ", gap) + hints
	}
	if leftW > w {
		return lipgloss.NewStyle().MaxWidth(w).Render(left)
	}
	return left
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

	var tsStr string
	switch m.tsMode {
	case TSModeClock:
		tsStr = theme.Success.Render("⏱ CLOCK")
	case TSModeDelta:
		tsStr = theme.Success.Render("⏱ Δt")
	case TSModeBoth:
		tsStr = theme.Success.Render("⏱ BOTH")
	default:
		tsStr = theme.Muted.Render("⏱ OFF")
	}

	var parts []string
	if m.splitMode != SplitNone {
		splitType := "SPLIT [VERT]"
		if m.splitMode == SplitHorizontal {
			splitType = "SPLIT [HORIZ]"
		}
		parts = append(parts, theme.Primary.Bold(true).Render(splitType))

		if m.syncScroll {
			parts = append(parts, theme.Success.Bold(true).Render("⟷ SYNC ON"))
		} else {
			parts = append(parts, theme.Muted.Render("⟷ SYNC OFF"))
		}

		paneName := "Left"
		if m.splitMode == SplitHorizontal {
			paneName = "Top"
			if m.activePane == 1 {
				paneName = "Bottom"
			}
		} else if m.activePane == 1 {
			paneName = "Right"
		}
		cur := m.currentTab()
		parts = append(parts, theme.Accent.Render(fmt.Sprintf("focus: %s [%s]", paneName, cur.DisplayName(m.activeTabIdx()+1))))
	} else if len(m.tabs) > 1 {
		tabName := m.tabs[m.activeTab].DisplayName(m.activeTab + 1)
		parts = append(parts, theme.Primary.Render(fmt.Sprintf("tab [%d/%d: %s]", m.activeTab+1, len(m.tabs), tabName)))
	}
	parts = append(parts, theme.Muted.Render(fmt.Sprintf("%d records", total)))
	if shown != total {
		parts = append(parts, theme.Secondary.Render(fmt.Sprintf("%d shown", shown)))
	}
	if start, end := m.selectionRange(); start >= 0 && end >= 0 {
		count := end - start + 1
		if d, ok := m.selectionDelta(); ok {
			parts = append(parts, theme.Secondary.Bold(true).Render(fmt.Sprintf("%d selected (Δt: %s)", count, timing.FormatDelta(d))))
		} else {
			parts = append(parts, theme.Secondary.Bold(true).Render(fmt.Sprintf("%d selected", count)))
		}
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

	if m.bookmarkedOnly {
		parts = append(parts, theme.Warning.Render(fmt.Sprintf("★ %d pinned (only)", len(m.bookmarks))))
	} else if len(m.bookmarks) > 0 {
		parts = append(parts, theme.Warning.Render(fmt.Sprintf("★ %d pinned", len(m.bookmarks))))
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
		help := theme.Muted.Render("  [Enter: next · ↑/↓: navigate · ^V: paste · Esc: cancel]")
		return "  " + prompt + text + help

	case modeFilter:
		prompt := theme.Primary.Render("Filter: ")
		text := theme.Content.Render(m.filterInput + "█")
		help := theme.Muted.Render("  [Enter: apply · ↑/↓: history · ^P: presets · ^V: paste · Esc: cancel]")
		return "  " + prompt + text + help

	case modeFilterPresets:
		return "  " + theme.Muted.Render("Filter Presets: [Enter: apply · ↑/↓: navigate · s: save active · d: delete · Esc: close]")

	case modeSavePresetPrompt:
		return "  " + theme.Muted.Render("Save Filter Preset: [Enter: save · Esc: cancel]")

	case modeTXInput:
		prompt := lipgloss.NewStyle().Foreground(colorMaple).Bold(true).Render(fmt.Sprintf("TX [%s] › ", m.txEnding.String()))
		text := theme.Content.Render(m.txInput + "█")
		help := theme.Muted.Render("  [Enter: send · Tab: line ending · ↑/↓: history · ^V: paste · Esc: cancel]")
		return "  " + prompt + text + help

	case modeHelp:
		return "  " + theme.Muted.Render("Press ") + theme.KeyName.Render("?") + theme.Muted.Render(", ") + theme.KeyName.Render("Esc") + theme.Muted.Render(", or ") + theme.KeyName.Render("q") + theme.Muted.Render(" to close help")

	default:
		hints := []string{
			theme.KeyName.Render("^F") + " " + theme.Muted.Render("search"),
			theme.KeyName.Render("f") + " " + theme.Muted.Render("filter"),
			theme.KeyName.Render("i") + " " + theme.Muted.Render("send"),
			theme.KeyName.Render("F") + " " + theme.Muted.Render("presets"),
			theme.KeyName.Render("b") + " " + theme.Muted.Render("pin"),
			theme.KeyName.Render("y") + " " + theme.Muted.Render("copy"),
		}
		if m.splitMode != SplitNone {
			hints = append(hints,
				theme.KeyName.Render("w")+" "+theme.Muted.Render("pane"),
				theme.KeyName.Render("S")+" "+theme.Muted.Render("sync"),
				theme.KeyName.Render("|")+" "+theme.Muted.Render("close"),
			)
		} else {
			hints = append(hints,
				theme.KeyName.Render("|")+" "+theme.Muted.Render("split"),
			)
		}
		if len(m.tabs) > 1 {
			hints = append(hints, theme.KeyName.Render("Tab")+" "+theme.Muted.Render("tab"))
			hints = append(hints, theme.KeyName.Render("^T")+" "+theme.Muted.Render("new tab"))
		} else {
			hints = append(hints, theme.KeyName.Render("^T")+" "+theme.Muted.Render("new tab"))
		}
		hints = append(hints,
			theme.KeyName.Render("Space")+" "+theme.Muted.Render("pause"),
			theme.KeyName.Render("c")+" "+theme.Muted.Render("clear"),
			theme.KeyName.Render("t")+" "+theme.Muted.Render("⏱ ts"),
			theme.KeyName.Render("p")+" "+theme.Muted.Render("port"),
			theme.KeyName.Render("P")+" "+theme.Muted.Render("profile"),
			theme.KeyName.Render("?")+" "+theme.Muted.Render("help"),
			theme.KeyName.Render("q")+" "+theme.Muted.Render("quit"),
		)
		return "  " + strings.Join(hints, "   ")
	}
}
