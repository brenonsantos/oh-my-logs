package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// openThemeModal initializes the theme selection modal state and switches mode.
func (m *Model) openThemeModal() {
	m.themeModalInitial = CurrentThemeName()
	cur := CurrentThemeName()
	m.themeModalCursor = 0
	for i, p := range curatedPalettes {
		if strings.EqualFold(normalizeThemeName(p.Name), normalizeThemeName(cur)) {
			m.themeModalCursor = i
			break
		}
	}
	m.mode = modeThemeModal
}

// handleThemeModalKey processes keyboard events in the theme selection modal.
func (m Model) handleThemeModalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case keyMatches(msg, m.keys.Cancel) || msg.String() == "q" || msg.Type == tea.KeyEscape:
		// Revert to initial theme if canceled
		SetCurrentTheme(m.themeModalInitial)
		m.mode = modeSettings
		return m, nil

	case keyMatches(msg, m.keys.Confirm) || msg.String() == " ":
		// Confirm selection and persist to settings
		selected := curatedPalettes[m.themeModalCursor]
		newTheme := SetCurrentTheme(selected.Name)
		m.settings.Theme = newTheme
		m.saveSettings()
		m.message = fmt.Sprintf("Theme set to %s", newTheme)
		m.mode = modeSettings
		return m, nil

	case msg.Type == tea.KeyUp || msg.String() == "k":
		n := len(curatedPalettes)
		if n > 0 {
			m.themeModalCursor = (m.themeModalCursor - 1 + n) % n
			// Live-preview active cursor theme immediately
			SetCurrentTheme(curatedPalettes[m.themeModalCursor].Name)
		}
		return m, nil

	case msg.Type == tea.KeyDown || msg.String() == "j":
		n := len(curatedPalettes)
		if n > 0 {
			m.themeModalCursor = (m.themeModalCursor + 1) % n
			// Live-preview active cursor theme immediately
			SetCurrentTheme(curatedPalettes[m.themeModalCursor].Name)
		}
		return m, nil
	}

	return m, nil
}

// viewThemeModal renders the centered floating theme picker modal dialog with swatches and live previews.
func (m Model) viewThemeModal() string {
	modalWidth := modalWidthTheme
	if modalWidth > m.width-6 {
		modalWidth = m.width - 6
	}

	var sb strings.Builder

	sb.WriteString(theme.ModalTitle.Render("Theme Palette"))
	sb.WriteString("\n\n")

	savedTheme := m.settings.Theme
	if savedTheme == "" {
		savedTheme = m.themeModalInitial
	}

	// Calculate scrolling window if table height is constrained
	totalThemes := len(curatedPalettes)
	maxVisible := m.tableHeight - 2
	if maxVisible < 6 {
		maxVisible = 6
	}
	if maxVisible > totalThemes {
		maxVisible = totalThemes
	}

	startIdx := 0
	if m.themeModalCursor >= maxVisible {
		startIdx = m.themeModalCursor - maxVisible + 1
	}
	endIdx := startIdx + maxVisible
	if endIdx > totalThemes {
		endIdx = totalThemes
	}

	for i := startIdx; i < endIdx; i++ {
		p := curatedPalettes[i]

		cursor := "  "
		if i == m.themeModalCursor {
			cursor = "› "
		}

		statusDot := "  "
		if strings.EqualFold(normalizeThemeName(p.Name), normalizeThemeName(savedTheme)) {
			statusDot = "● "
		}

		// Color swatches (Accent, Green, Yellow, Red, Cyan, Purple)
		swatches := lipgloss.NewStyle().Foreground(p.Accent).Render("■") +
			lipgloss.NewStyle().Foreground(p.Green).Render("■") +
			lipgloss.NewStyle().Foreground(p.Yellow).Render("■") +
			lipgloss.NewStyle().Foreground(p.Red).Render("■") +
			lipgloss.NewStyle().Foreground(p.Cyan).Render("■") +
			lipgloss.NewStyle().Foreground(p.Purple).Render("■")

		nameStr := fmt.Sprintf("%-18s", p.Name)
		descStr := p.Description

		// Adjust description length to fit modalWidth
		// prefix (2) + status (2) + name (18) + gap (2) + swatches (6) + gap (2) = 32 overhead
		availDesc := modalWidth - 32 - 4
		if availDesc > 0 && len(descStr) > availDesc {
			if availDesc > 3 {
				descStr = descStr[:availDesc-3] + "..."
			} else {
				descStr = ""
			}
		}

		if i == m.themeModalCursor {
			line := fmt.Sprintf("%s%s%s  %s  %s",
				cursor,
				theme.Success.Render(statusDot),
				theme.ModalSelected.Render(nameStr),
				swatches,
				theme.ModalItem.Render(descStr),
			)
			sb.WriteString(line)
		} else {
			line := fmt.Sprintf("%s%s%s  %s  %s",
				cursor,
				theme.Muted.Render(statusDot),
				theme.ModalItem.Render(nameStr),
				swatches,
				theme.Muted.Render(descStr),
			)
			sb.WriteString(line)
		}
		sb.WriteString("\n")
	}

	if totalThemes > maxVisible {
		more := totalThemes - endIdx
		if more > 0 {
			sb.WriteString(theme.Muted.Render(fmt.Sprintf("    ... (%d more below)", more)))
			sb.WriteString("\n")
		} else if startIdx > 0 {
			sb.WriteString(theme.Muted.Render(fmt.Sprintf("    ... (%d more above)", startIdx)))
			sb.WriteString("\n")
		}
	}

	sb.WriteString("\n")
	sb.WriteString(theme.ModalFooter.Render("Enter apply · ↑/↓ navigate & live preview · Esc cancel"))

	modalBox := theme.ModalBox.Width(modalWidth).Render(sb.String())
	return centerBox(m.width, m.tableHeight+2, modalBox)
}
