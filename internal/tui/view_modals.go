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
	sb.WriteString(theme.ModalFooter.Render("Enter select · ↑/↓ port · ←/→ baud · Esc cancel"))

	modalBox := theme.ModalBox.Width(modalWidth).Render(sb.String())
	return centerBox(m.width, m.tableHeight+2, modalBox)
}

// viewProfilePickerModal renders the centered rounded modal for Profile switching.
func (m Model) viewProfilePickerModal() string {
	modalWidth := 52
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
		renderItem("↑, k", "Scroll / select row", colWidth),
		renderItem("↓, j", "Scroll / select row", colWidth),
		renderItem("PgUp, ^U", "Page up", colWidth),
		renderItem("PgDn, ^D", "Page down", colWidth),
		renderItem("g, Home", "Jump to top", colWidth),
		renderItem("G, End", "Jump to bottom", colWidth),
		renderItem("[, ]", "Prev / next pin", colWidth),
		renderItem("Wheel", "Smooth scroll", colWidth),
		renderHeader("VIRTUAL TABS", colWidth),
		renderItem("Click tab", "Switch to tab", colWidth),
		renderItem("Tab", "Next tab", colWidth),
		renderItem("S-Tab", "Previous tab", colWidth),
		renderItem("1 .. 9", "Jump to tab N", colWidth),
		renderItem("Ctrl+T", "Create new tab", colWidth),
		renderItem("Ctrl+W", "Close active tab", colWidth),
	}

	right := []string{
		renderHeader("SEARCH, FILTER & PINS", colWidth),
		renderItem("Ctrl+F", "Search logs", colWidth),
		renderItem("f", "Filter active tab", colWidth),
		renderItem("b, m", "Pin / unpin row", colWidth),
		renderItem("B", "Toggle pinned only", colWidth),
		renderItem("Ctrl+V", "Paste in input", colWidth),
		renderItem("y / Y", "Copy row / raw text", colWidth),
		renderItem("Click row", "Select row & pause", colWidth),
		renderItem("Shift+↑/↓", "Extend line selection", colWidth),
		renderItem("Drag mouse", "Select multiple rows", colWidth),
		renderItem("2x Click", "Copy row to clipboard", colWidth),
		renderHeader("ACTIONS & CONTROLS", colWidth),
		renderItem("Space", "Pause / resume follow", colWidth),
		renderItem("c", "Clear buffer", colWidth),
		renderItem("t", "Toggle timestamp", colWidth),
		renderItem("s", "Save log to file", colWidth),
		renderItem("p, P, r", "Port / Profile / Reconn", colWidth),
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
	sb.WriteString(theme.ModalFooter.Render("Press ? or Esc to close · Hold Opt (Mac) / Shift (Linux) for native select"))

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
