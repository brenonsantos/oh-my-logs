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
		if t.DisplayFormat != FormatParsed {
			tag += fmt.Sprintf(" [%s]", t.DisplayFormat.Tag())
		}
		if m.splitMode != SplitNone {
			if i == m.paneTabIdx(0) {
				tag += " [P1]"
			} else if i == m.paneTabIdx(1) {
				tag += " [P2]"
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
	} else if contentWidth > m.width {
		content = lipgloss.NewStyle().MaxWidth(m.width).Render(content)
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
		if m.reconnecting && m.serialCfg.Port != "" && !m.isFileSource {
			subtitle = theme.Muted.Render("Auto-reconnecting to ") + theme.Accent.Render(m.serialCfg.Port) +
				theme.Muted.Render("   ·   Press ") + theme.KeyName.Render("p") +
				theme.Muted.Render(" to select a port   ·   Press ") +
				theme.KeyName.Render("?") + theme.Muted.Render(" for shortcuts")
		} else if m.serialCfg.Port != "" && !m.isFileSource {
			subtitle = theme.Muted.Render("Port released (") + theme.Accent.Render(m.serialCfg.Port) +
				theme.Muted.Render(")   ·   Press ") + theme.KeyName.Render("r") +
				theme.Muted.Render(" to reconnect   ·   Press ") +
				theme.KeyName.Render("p") + theme.Muted.Render(" to select port")
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

	curFormat := m.displayFormat
	if cur := m.currentTab(); cur != nil {
		curFormat = cur.DisplayFormat
	}
	if curFormat != FormatParsed {
		parts = append(parts, theme.Accent.Bold(true).Render(fmt.Sprintf("▤ %s", curFormat.Tag())))
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
	res := "  " + strings.Join(parts, sep)
	if m.width > 0 && lipgloss.Width(res) > m.width {
		return lipgloss.NewStyle().MaxWidth(m.width).Render(res)
	}
	return res
}

// viewKeyBar renders context-sensitive key hints or input prompt.
func (m Model) viewKeyBar() string {
	res := m.renderKeyBarContent()
	if m.width > 0 && lipgloss.Width(res) > m.width {
		return lipgloss.NewStyle().MaxWidth(m.width).Render(res)
	}
	return res
}

func (m Model) renderKeyBarContent() string {
	switch m.mode {
	case modeSearch:
		prompt := theme.Primary.Render("Search: ")
		text := renderInputWithCursor(m.searchInput, m.searchPos, theme.Content)
		help := theme.Muted.Render("  [Enter: next · ↑/↓: matches · ←/→: cursor · ^V: paste · Esc: cancel]")
		return "  " + prompt + text + help

	case modeFilter:
		prompt := theme.Primary.Render("Filter: ")
		text := renderInputWithCursor(m.filterInput, m.filterCursor, theme.Content)
		help := theme.Muted.Render("  [Enter: apply · ↑/↓: history · ←/→: cursor · ^P: presets · ^V: paste · Esc: cancel]")
		return "  " + prompt + text + help

	case modeFilterPresets:
		return "  " + theme.Muted.Render("Filter Presets: [Enter: apply · ↑/↓: navigate · s: save active · d: delete · Esc: close]")

	case modeSavePresetPrompt:
		return "  " + theme.Muted.Render("Save Filter Preset: [Enter: save · ←/→: cursor · Esc: cancel]")

	case modeTXInput:
		prompt := lipgloss.NewStyle().Foreground(colorMaple).Bold(true).Render(fmt.Sprintf("TX [%s] › ", m.txEnding.String()))
		text := renderInputWithCursor(m.txInput, m.txCursor, theme.Content)
		help := theme.Muted.Render("  [Enter: send · Tab: line ending · ↑/↓: history · ←/→: cursor · ^V: paste · Esc: cancel]")
		return "  " + prompt + text + help

	case modeSettings:
		return "  " + theme.Muted.Render("Settings: [↑/↓: select · ←/→: adjust · Space: toggle · Enter/Esc: close]")

	case modeHelp:
		return "  " + theme.Muted.Render("Press ") + theme.KeyName.Render("?") + theme.Muted.Render(", ") + theme.KeyName.Render("Esc") + theme.Muted.Render(", or ") + theme.KeyName.Render("q") + theme.Muted.Render(" to close help")

	default:
		hint := func(k, action string) string {
			return theme.KeyName.Render(k) + " " + theme.Muted.Render(action)
		}

		var candidates []string

		// Context 1: Row Selection Active
		if m.selectedRow >= 0 {
			if m.selectionStart >= 0 && m.selectionEnd >= 0 && m.selectionStart != m.selectionEnd {
				start := m.selectionStart
				end := m.selectionEnd
				if start > end {
					start, end = end, start
				}
				count := end - start + 1
				candidates = append(candidates, hint("y", fmt.Sprintf("copy (%d)", count)))
			} else {
				candidates = append(candidates, hint("y", "copy"))
			}
			candidates = append(candidates,
				hint("Y", "raw"),
				hint("b", "pin"),
				hint("Esc", "unselect"),
				hint("f", "filter"),
				hint("^F", "search"),
				hint("Space", "resume"),
				hint("c", "clear"),
				hint(",", "⚙ cfg"),
				hint("x", "hex"),
			)
			candidates = append(candidates,
				hint("i", "send"),
			)
			if m.connState == ConnConnected && !m.isFileSource {
				candidates = append(candidates, hint("D", "disconnect"))
			} else if m.serialCfg.Port != "" && !m.isFileSource {
				candidates = append(candidates, hint("r", "reconnect"))
			}
		} else if m.splitMode != SplitNone {
			// Context 2: Split View Active
			splitCloseKey := "|"
			if m.splitMode == SplitHorizontal {
				splitCloseKey = "_"
			}
			candidates = append(candidates,
				hint("f", "filter"),
				hint("^F", "search"),
				hint("w", "pane"),
				hint("S", "sync"),
				hint(splitCloseKey, "unsplit"),
				hint("Space", "pause"),
				hint("c", "clear"),
				hint(",", "⚙ cfg"),
				hint("x", "hex"),
				hint("i", "send"),
			)
			if m.connState == ConnConnected && !m.isFileSource {
				candidates = append(candidates, hint("D", "disconnect"))
			} else if m.serialCfg.Port != "" && !m.isFileSource {
				candidates = append(candidates, hint("r", "reconnect"))
			}
			candidates = append(candidates,
				hint("t", "⏱ ts"),
			)
		} else {
			// Context 3: Normal Follow / Stream Mode
			pauseLabel := "pause"
			if !m.follow {
				pauseLabel = "resume"
			}
			candidates = append(candidates,
				hint("f", "filter"),
				hint("^F", "search"),
			)
			// Connection control (Disconnect / Reconnect / Port)
			if m.connState == ConnConnected && !m.isFileSource {
				candidates = append(candidates, hint("D", "disconnect"))
			} else if m.serialCfg.Port != "" && !m.isFileSource {
				candidates = append(candidates, hint("r", "reconnect"))
			} else if !m.isFileSource {
				candidates = append(candidates, hint("p", "port"))
			}
			if len(m.tabs) > 1 {
				candidates = append(candidates, hint("Tab", "tab"), hint("^T", "new tab"))
			} else {
				candidates = append(candidates, hint("^T", "new tab"))
			}
			candidates = append(candidates,
				hint("t", "⏱ ts"),
				hint("x", "hex"),
			)
			candidates = append(candidates,
				hint("P", "profile"),
				hint("F", "presets"),
				hint(",", "⚙ cfg"),
				hint("Space", pauseLabel),
				hint("c", "clear"),
				hint("i", "send"),
				hint("|", "split"),
			)
			if (m.connState == ConnConnected || m.serialCfg.Port != "") && !m.isFileSource {
				candidates = append(candidates, hint("p", "port"))
			}
		}

		// Adaptive width budget:
		// Always anchor `? help · q quit` at the right end of the bar.
		sep := theme.Muted.Render(" · ")
		sepWidth := lipgloss.Width(sep)
		anchor := hint("?", "help") + sep + hint("q", "quit")
		anchorWidth := lipgloss.Width(anchor)

		targetWidth := m.width - 4
		if targetWidth < anchorWidth+10 {
			return "  " + anchor
		}

		var shown []string
		usedWidth := 2 + anchorWidth // prefix "  " + anchor

		for _, c := range candidates {
			cWidth := lipgloss.Width(c)
			needed := cWidth + sepWidth
			if usedWidth+needed <= targetWidth {
				shown = append(shown, c)
				usedWidth += needed
			}
		}

		shown = append(shown, anchor)
		return "  " + strings.Join(shown, sep)
	}
}

// renderInputWithCursor renders input text with an accurate visual block cursor.
// If the cursor is at the end (pos >= len(runes)), a "█" block is appended.
// If the cursor is over a character, that character is styled with highlighted/inverted colors.
func renderInputWithCursor(text string, pos int, baseStyle lipgloss.Style) string {
	runes := []rune(text)
	if pos < 0 {
		pos = 0
	}
	if pos > len(runes) {
		pos = len(runes)
	}

	cursorCharStyle := lipgloss.NewStyle().
		Background(colorCyan).
		Foreground(lipgloss.Color("#000000")).
		Bold(true)
	cursorEndBlock := lipgloss.NewStyle().
		Foreground(colorCyan).
		Bold(true).
		Render("█")

	if pos >= len(runes) {
		return baseStyle.Render(text) + cursorEndBlock
	}

	before := baseStyle.Render(string(runes[:pos]))
	underCursor := cursorCharStyle.Render(string(runes[pos]))
	after := baseStyle.Render(string(runes[pos+1:]))

	return before + underCursor + after
}

