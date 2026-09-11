package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// viewPortPickerModal renders the centered rounded modal for Port & Baud Rate.
// Up/Down selects the port directly, Left/Right adjusts baud rate directly.
func (m Model) viewPortPickerModal() string {
	modalWidth := modalWidthPortPicker
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
			if i == m.portCursor {
				sb.WriteString(theme.ModalSelected.Render(fmt.Sprintf("  › %s", port)))
			} else {
				sb.WriteString(theme.ModalItem.Render(fmt.Sprintf("    %s", port)))
			}
			sb.WriteString("\n")
		}
		if len(m.portList) > maxVisible {
			sb.WriteString(theme.Muted.Render(fmt.Sprintf("    ... (%d more)", len(m.portList)-endIdx)))
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

	sb.WriteString(fmt.Sprintf("    %s  %s  %s\n\n", theme.Muted.Render("←"), theme.ModalSelected.Render(baudStr), theme.Muted.Render("→")))

	// Footer
	sb.WriteString(theme.ModalFooter.Render("Enter select · d disconnect · ↑/↓ port · ←/→ baud · Esc cancel"))

	modalBox := theme.ModalBox.Width(modalWidth).Render(sb.String())
	return centerBox(m.width, m.tableHeight+splitPaneHeaderOverhead, modalBox)
}

// viewProfilePickerModal renders the centered rounded modal for Profile switching.
func (m Model) viewProfilePickerModal() string {
	modalWidth := modalWidthSettings
	for _, p := range m.profileList {
		labelLen := len(p.Name) + 14
		if labelLen > modalWidth {
			modalWidth = labelLen
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
			tag := ""
			if prof.Path != "" {
				isGlobal := false
				if m.appConfig != nil && m.appConfig.ProfilesDir != "" {
					absP, _ := filepath.Abs(prof.Path)
					absGlobal, _ := filepath.Abs(m.appConfig.ProfilesDir)
					if strings.HasPrefix(absP, absGlobal) {
						isGlobal = true
					}
				}
				if !isGlobal {
					tag = " [local]"
				}
			}
			if i == m.profileCursor {
				sb.WriteString(theme.ModalSelected.Render(fmt.Sprintf("  › %s%s", prof.Name, tag)))
			} else {
				sb.WriteString(theme.ModalItem.Render(fmt.Sprintf("    %s", prof.Name)) + theme.Muted.Render(tag))
			}
			sb.WriteString("\n")
		}
	}

	sb.WriteString("\n")
	sb.WriteString(theme.ModalFooter.Render("Enter select · ↑/↓ navigate · Esc cancel"))

	modalBox := theme.ModalBox.Width(modalWidth).Render(sb.String())
	return centerBox(m.width, m.tableHeight+2, modalBox)
}

// viewFilterPresetsModal renders the centered rounded modal for Filter Presets.
func (m Model) viewFilterPresetsModal() string {
	presets := m.filtersCfg.Presets

	// Calculate maximum lengths for dynamic sizing
	maxNameW := 0
	maxFilterW := 0
	for _, p := range presets {
		if len(p.Name) > maxNameW {
			maxNameW = len(p.Name)
		}
		if len(p.Filter) > maxFilterW {
			maxFilterW = len(p.Filter)
		}
	}

	nameW := maxNameW
	if nameW < 20 {
		nameW = 20
	}
	if nameW > 28 {
		nameW = 28
	}

	// Calculate desired inner content width
	footerFull := "Enter apply · ↑/↓ navigate · s save active · d delete · Esc cancel"
	footerW := lipgloss.Width(footerFull) // 66

	curFilter := ""
	if cur := m.currentTab(); cur != nil && cur.FilterRaw != "" {
		curFilter = cur.FilterRaw
	}

	// Content needs: prefix(4) + name(nameW) + sep(2) + filter(maxFilterW) + breathing margin(2)
	rowNeededW := 4 + nameW + 2 + maxFilterW + 2
	desiredInnerW := footerW
	if rowNeededW > desiredInnerW {
		desiredInnerW = rowNeededW
	}
	if curFilter != "" {
		actW := 15 + len(curFilter) + 2
		if actW > 90 {
			actW = 90
		}
		if actW > desiredInnerW {
			desiredInnerW = actW
		}
	}

	// ModalBox padding(2+2=4) and rounded border(1+1=2) require 6 columns overhead.
	modalWidth := desiredInnerW + 6
	if modalWidth < 74 {
		modalWidth = 74
	}

	// Clamp to terminal boundaries
	maxModalW := m.width - 6
	if maxModalW < 30 {
		maxModalW = m.width
	}
	if modalWidth > maxModalW {
		modalWidth = maxModalW
	}
	if modalWidth < 40 && maxModalW >= 40 {
		modalWidth = 40
	}

	// The actual usable inner width inside ModalBox (accounting for borders and horizontal padding)
	innerW := modalWidth - 6
	if innerW < 20 {
		innerW = 20
	}

	// If inner width is constrained on narrow screens, adjust name column
	if nameW > innerW/2 {
		nameW = innerW / 2
	}
	if nameW < 12 && innerW >= 24 {
		nameW = 12
	}

	var sb strings.Builder
	sb.WriteString(theme.ModalTitle.Render("Filter Presets"))
	sb.WriteString("\n\n")

	if len(presets) == 0 {
		sb.WriteString(theme.Muted.Render("  (no presets available)"))
		sb.WriteString("\n")
	} else {
		maxVisible := 6
		startIdx := 0
		if m.presetCursor >= maxVisible {
			startIdx = m.presetCursor - maxVisible + 1
		}
		endIdx := startIdx + maxVisible
		if endIdx > len(presets) {
			endIdx = len(presets)
		}

		availFilterW := innerW - 4 - nameW - 2
		if availFilterW < 0 {
			availFilterW = 0
		}

		for i := startIdx; i < endIdx; i++ {
			p := presets[i]
			nameStr := padOrTrunc(p.Name, nameW)
			filterStr := p.Filter
			if availFilterW > 3 && len(filterStr) > availFilterW {
				filterStr = filterStr[:availFilterW-3] + "..."
			} else if len(filterStr) > availFilterW {
				filterStr = filterStr[:availFilterW]
			}

			if i == m.presetCursor {
				line := fmt.Sprintf("  › %s  %s", theme.ModalSelected.Render(nameStr), theme.Accent.Render(filterStr))
				sb.WriteString(line)
			} else {
				line := fmt.Sprintf("    %s  %s", theme.ModalItem.Render(nameStr), theme.Muted.Render(filterStr))
				sb.WriteString(line)
			}
			sb.WriteString("\n")
		}
		if len(presets) > maxVisible {
			sb.WriteString(theme.Muted.Render(fmt.Sprintf("    ... (%d more)", len(presets)-endIdx)))
			sb.WriteString("\n")
		}
	}

	sb.WriteString("\n")
	if curFilter != "" {
		availActiveW := innerW - 15
		dispFilter := curFilter
		if availActiveW > 3 && len(dispFilter) > availActiveW {
			dispFilter = dispFilter[:availActiveW-3] + "..."
		} else if len(dispFilter) > availActiveW {
			dispFilter = dispFilter[:max(0, availActiveW)]
		}
		sb.WriteString(theme.Muted.Render("Active filter: ") + theme.Accent.Render(dispFilter) + "\n\n")
	}

	footer := footerFull
	if innerW < lipgloss.Width(footer) {
		footer = "Enter apply · ↑/↓ nav · s save · d del · Esc exit"
		if innerW < lipgloss.Width(footer) {
			footer = "Enter apply · Esc exit"
		}
	}
	sb.WriteString(theme.ModalFooter.Render(footer))

	modalBox := theme.ModalBox.Width(modalWidth).Render(sb.String())
	return centerBox(m.width, m.tableHeight+2, modalBox)
}

// viewSavePresetModal renders the centered prompt to save the active filter as a new preset.
func (m Model) viewSavePresetModal() string {
	cur := m.currentTab()
	curFilter := ""
	if cur != nil {
		curFilter = cur.FilterRaw
	}

	reqW := len("Filter: ") + len(curFilter) + 8
	modalWidth := modalWidthFilters
	if reqW > modalWidth {
		modalWidth = reqW
	}
	if modalWidth > 90 {
		modalWidth = 90
	}
	maxModalW := m.width - 6
	if maxModalW < 30 {
		maxModalW = m.width
	}
	if modalWidth > maxModalW {
		modalWidth = maxModalW
	}
	if modalWidth < 46 && maxModalW >= 46 {
		modalWidth = 46
	}

	innerW := modalWidth - 6
	if innerW < 20 {
		innerW = 20
	}

	var sb strings.Builder
	sb.WriteString(theme.ModalTitle.Render("Save Filter Preset"))
	sb.WriteString("\n\n")

	availFilterW := innerW - 8
	dispFilter := curFilter
	if availFilterW > 3 && len(dispFilter) > availFilterW {
		dispFilter = dispFilter[:availFilterW-3] + "..."
	} else if len(dispFilter) > availFilterW {
		dispFilter = dispFilter[:max(0, availFilterW)]
	}
	sb.WriteString(theme.Muted.Render("Filter: ") + theme.Accent.Render(dispFilter))
	sb.WriteString("\n\n")

	prompt := theme.Secondary.Render("Preset Name: ")
	availInputW := innerW - savePresetPromptLabelWidth
	if availInputW < minSavePresetInputWidth {
		availInputW = minSavePresetInputWidth
	}
	input := m.savePresetNameInput.RenderWindow(availInputW, theme.ModalSelected)
	sb.WriteString("  " + prompt + input)
	sb.WriteString("\n\n")

	sb.WriteString(theme.ModalFooter.Render("Enter save · ←/→ cursor · Esc cancel"))

	modalBox := theme.ModalBox.Width(modalWidth).Render(sb.String())
	return centerBox(m.width, m.tableHeight+splitPaneHeaderOverhead, modalBox)
}



// viewGameModal renders the active mini-game in a centered floating container
// sized according to the game's requested dimensions.
func (m Model) viewGameModal() string {
	if m.activeGame == nil {
		return ""
	}
	reqW, _ := m.activeGame.Dimensions()
	if reqW <= 0 {
		reqW = 54
	}
	modalWidth := reqW
	if modalWidth > m.width-4 {
		modalWidth = m.width - 4
	}
	if modalWidth < 30 {
		modalWidth = 30
	}

	var sb strings.Builder

	// Game Title & incoming log notification
	sb.WriteString(theme.ModalTitle.Render(m.activeGame.Title()))
	sb.WriteString("\n")

	if m.logsDuringGame > 0 {
		badge := fmt.Sprintf("  ● %d new logs arrived! Press Esc to view", m.logsDuringGame)
		sb.WriteString(theme.Accent.Bold(true).Render(badge))
		sb.WriteString("\n\n")
	} else {
		sb.WriteString(theme.Muted.Render("  Waiting for serial logs…"))
		sb.WriteString("\n\n")
	}

	innerW := modalWidth - 6
	if innerW < 10 {
		innerW = 10
	}
	sb.WriteString(m.activeGame.View(innerW))

	modalBox := theme.ModalBox.Width(modalWidth).Render(sb.String())
	return centerBox(m.width, m.tableHeight+splitPaneHeaderOverhead, modalBox)
}

// viewHelpModal renders a clean floating modal with categorized keyboard shortcuts.
func (m Model) viewHelpModal() string {
	colWidth := 40
	// 2 columns of colWidth + 3 for separator " │ " + 6 for borders and padding
	modalWidth := modalWidthHelp
	if modalWidth > m.width-4 {
		modalWidth = m.width - 4
		colWidth = (modalWidth - 9) / 2
	}
	if colWidth < 16 {
		colWidth = 16
	}
	if modalWidth < 40 {
		modalWidth = 40
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
		renderItem("↑, k / ↓, j", "Scroll / select row", colWidth),
		renderItem("←, h / →, l", "Char cursor / auto-pan", colWidth),
		renderItem("{, }", "Pan viewport left / right", colWidth),
		renderItem("Shift+←/→", "Select characters", colWidth),
		renderItem("PgUp / PgDn", "Page up / down", colWidth),
		renderItem("g / G", "Jump top / bottom", colWidth),
		renderItem("[, ]", "Prev / next pin", colWidth),
		renderItem("Wheel", "Smooth scroll (vert/horiz)", colWidth),
		renderHeader("VIRTUAL TABS", colWidth),
		renderItem("Click tab", "Switch to tab", colWidth),
		renderItem("Tab", "Next tab", colWidth),
		renderItem("S-Tab", "Previous tab", colWidth),
		renderItem("1 .. 9", "Jump to tab N", colWidth),
		renderItem("Ctrl+T", "Create new tab", colWidth),
		renderItem("Ctrl+W", "Close active tab", colWidth),
		renderHeader("SPLIT DUAL VIEW", colWidth),
		renderItem("|", "Toggle vertical split", colWidth),
		renderItem("_", "Toggle horiz split", colWidth),
		renderItem("w", "Switch pane focus", colWidth),
		renderItem("S", "Toggle sync scroll", colWidth),
	}

	right := []string{
		renderHeader("SEARCH, FILTER & PINS", colWidth),
		renderItem("Ctrl+F", "Search logs", colWidth),
		renderItem("f", "Filter (↑/↓ hist)", colWidth),
		renderItem("F, ^P", "Filter presets", colWidth),
		renderItem("b, m", "Pin / unpin row", colWidth),
		renderItem("B", "Toggle pinned only", colWidth),
		renderItem("Ctrl+V", "Paste in input", colWidth),
		renderItem("y / Y", "Copy row / raw text", colWidth),
		renderItem("Click row", "Select row & pause", colWidth),
		renderItem("Shift+↑/↓", "Extend line selection", colWidth),
		renderItem("Drag mouse", "Select multiple rows", colWidth),
		renderItem("2x Click", "Copy row to clipboard", colWidth),
		renderHeader("ACTIONS & CONTROLS", colWidth),
		renderItem("i, :", "Send serial cmd (TX)", colWidth),
		renderItem("Space", "Pause / resume follow", colWidth),
		renderItem("c", "Clear buffer", colWidth),
		renderItem("t", "Toggle timestamp / Δt", colWidth),
		renderItem("x, X", "Format (Hex/Bin/Raw)", colWidth),
		renderItem(", , C", "Settings & preferences", colWidth),
		renderItem("s", "Save log to file", colWidth),
		renderItem("D, r", "Disconnect / Reconnect", colWidth),
		renderItem("p, P", "Port / Profile select", colWidth),
		renderItem("?, q", "Toggle help / Quit", colWidth),
	}

	sep := theme.Divider.Render(" │ ")
	var sb strings.Builder

	sb.WriteString(theme.ModalTitle.Render("Help — Keyboard & Mouse Shortcuts"))
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
	innerW := modalWidth - 6
	footerText := "Press ? or Esc to close · Hold Opt (Mac) / Shift (Linux) for native select"
	if innerW < lipgloss.Width(footerText) {
		footerText = "Press ? or Esc to close · Opt/Shift for native select"
		if innerW < lipgloss.Width(footerText) {
			footerText = "Press ? or Esc to close"
		}
	}
	sb.WriteString(theme.ModalFooter.Render(footerText))

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
