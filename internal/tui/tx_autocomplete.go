package tui

import (
	"fmt"
	"strings"

	"github.com/brenoniehues/oh-my-logs/internal/encoder"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// txSuggestMsg carries asynchronous autocomplete suggestions from an encoder worker.
type txSuggestMsg struct {
	suggestions []encoder.Suggestion
	preview     string
	err         error
}

// txMaxSuggestionsVisible is the maximum number of suggestion rows visible in the drawer.
const txMaxSuggestionsVisible = 5

// txSuggestionsDrawerHeight returns the vertical lines required to display suggestions.
func txSuggestionsDrawerHeight(count int) int {
	if count <= 0 {
		return 0
	}
	if count > txMaxSuggestionsVisible {
		count = txMaxSuggestionsVisible
	}
	return count + 1 // 1 header line + up to txMaxSuggestionsVisible items
}

// clampTXSuggestionOffset adjusts the visible window so txSuggestionIdx is always visible.
func (m *Model) clampTXSuggestionOffset() {
	total := len(m.txSuggestions)
	if total <= txMaxSuggestionsVisible {
		m.txSuggestionOffset = 0
		return
	}
	if m.txSuggestionIdx < 0 {
		m.txSuggestionIdx = 0
	}
	if m.txSuggestionIdx >= total {
		m.txSuggestionIdx = total - 1
	}

	if m.txSuggestionIdx < m.txSuggestionOffset {
		m.txSuggestionOffset = m.txSuggestionIdx
	} else if m.txSuggestionIdx >= m.txSuggestionOffset+txMaxSuggestionsVisible {
		m.txSuggestionOffset = m.txSuggestionIdx - txMaxSuggestionsVisible + 1
	}

	maxOffset := total - txMaxSuggestionsVisible
	if m.txSuggestionOffset > maxOffset {
		m.txSuggestionOffset = maxOffset
	}
	if m.txSuggestionOffset < 0 {
		m.txSuggestionOffset = 0
	}
}

// triggerSuggestCmd queries the active or matched encoder for autocomplete suggestions.
func (m Model) triggerSuggestCmd() tea.Cmd {
	var enc encoder.Encoder
	val := m.txInput.Value
	cursor := m.txInput.Cursor

	if m.activeEncoderIdx >= 0 && m.encoders != nil && m.activeEncoderIdx < len(m.encoders.Encoders()) {
		enc = m.encoders.Encoders()[m.activeEncoderIdx]
	} else if m.encoders != nil {
		enc, _ = m.encoders.Match(val)
	}

	if enc == nil {
		return nil
	}

	return func() tea.Msg {
		suggs, err := enc.Suggest(val, cursor)
		prev := ""
		if p, ok := enc.(encoder.Previewer); ok {
			prev = p.LastPreview()
		}
		return txSuggestMsg{
			suggestions: suggs,
			preview:     prev,
			err:         err,
		}
	}
}

func isTokenDelim(r rune) bool {
	return r == ' ' || r == '\t' || r == ':' || r == '/' || r == '(' || r == ')' || r == '=' || r == ','
}

// applySuggestion replaces the token under the cursor with replacement and repositions cursor.
func applySuggestion(text string, cursor int, replacement string) (string, int) {
	runes := []rune(text)
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(runes) {
		cursor = len(runes)
	}

	// Find the start of the token immediately preceding the cursor
	start := cursor
	for start > 0 && !isTokenDelim(runes[start-1]) {
		start--
	}

	// Find the end of the token after the cursor
	end := cursor
	for end < len(runes) && !isTokenDelim(runes[end]) {
		end++
	}

	var b strings.Builder
	b.WriteString(string(runes[:start]))
	b.WriteString(replacement)
	b.WriteString(string(runes[end:]))

	newStr := b.String()
	newRunes := []rune(newStr)
	newCursor := start + len([]rune(replacement))

	// Only append trailing space if replacement does not end with a syntax delimiter
	if newCursor == len(newRunes) && len(replacement) > 0 {
		repRunes := []rune(replacement)
		lastRune := repRunes[len(repRunes)-1]
		if !isTokenDelim(lastRune) {
			newStr += " "
			newCursor++
		}
	}

	return newStr, newCursor
}

// viewTXSuggestionsDrawer renders the docked autocomplete drawer above the TX input bar.
func (m Model) viewTXSuggestionsDrawer() string {
	w := m.width
	if w <= 0 {
		w = 80
	}

	bg := colorSelected
	baseStyle := lipgloss.NewStyle().Background(bg)

	// Determine active encoder title
	encName := "Encoder"
	if m.activeEncoderIdx >= 0 && m.encoders != nil && m.activeEncoderIdx < len(m.encoders.Encoders()) {
		encName = m.encoders.Encoders()[m.activeEncoderIdx].Name()
	} else if m.encoders != nil {
		if matched, _ := m.encoders.Match(m.txInput.Value); matched != nil {
			encName = matched.Name()
		}
	}

	total := len(m.txSuggestions)
	title := fmt.Sprintf("─── 💡 SUGGESTIONS [%s] ", encName)
	if total > txMaxSuggestionsVisible {
		title = fmt.Sprintf("─── 💡 SUGGESTIONS [%s] (%d/%d) ", encName, m.txSuggestionIdx+1, total)
	} else if total > 0 {
		title = fmt.Sprintf("─── 💡 SUGGESTIONS [%s] (%d) ", encName, total)
	}
	titleStyled := baseStyle.Foreground(colorYellow).Bold(true).Render(title)
	titleW := lipgloss.Width(titleStyled)

	help := " [Tab/Enter: complete · ↑/↓: scroll · Esc: close] ───"
	helpStyled := baseStyle.Foreground(colorMuted).Render(help)
	helpW := lipgloss.Width(helpStyled)

	dashCount := w - titleW - helpW
	if dashCount < 1 {
		dashCount = 1
	}
	middleDashes := baseStyle.Foreground(colorAccent).Render(strings.Repeat("─", dashCount))
	headerLine := titleStyled + middleDashes + helpStyled
	if curW := lipgloss.Width(headerLine); curW < w {
		headerLine += baseStyle.Foreground(colorAccent).Render(strings.Repeat("─", w-curW))
	} else if curW > w {
		headerLine = ansiCut(headerLine, 0, w)
	}

	lines := []string{headerLine}

	// Calculate visible slice
	offset := m.txSuggestionOffset
	if offset < 0 {
		offset = 0
	}
	if offset > total {
		offset = total
	}
	end := offset + txMaxSuggestionsVisible
	if end > total {
		end = total
	}
	items := m.txSuggestions[offset:end]

	hasVScroll := total > txMaxSuggestionsVisible
	trackH := txMaxSuggestionsVisible
	thumbH := 1
	if total > 0 {
		thumbH = (trackH * txMaxSuggestionsVisible) / total
		if thumbH < 1 {
			thumbH = 1
		}
		if thumbH > trackH {
			thumbH = trackH
		}
	}
	maxOffset := total - txMaxSuggestionsVisible
	thumbTop := 0
	if maxOffset > 0 {
		thumbTop = (offset * (trackH - thumbH)) / maxOffset
		if thumbTop+thumbH > trackH {
			thumbTop = trackH - thumbH
		}
		if thumbTop < 0 {
			thumbTop = 0
		}
	}

	for i, s := range items {
		actualIdx := offset + i
		isSelected := actualIdx == m.txSuggestionIdx
		rowStyle := baseStyle
		prefix := "   "
		if isSelected {
			rowStyle = baseStyle.Bold(true)
			prefix = baseStyle.Foreground(colorYellow).Render(" ▸ ")
		}

		valPart := rowStyle.Foreground(colorAccent).Render(s.Value)
		descPart := ""
		if s.Description != "" {
			descPart = rowStyle.Foreground(colorMuted).Render("  " + s.Description)
		}

		rowContent := prefix + valPart + descPart
		contentW := w
		vScrollChar := ""
		if hasVScroll {
			contentW = w - 1
			if i >= thumbTop && i < thumbTop+thumbH {
				vScrollChar = baseStyle.Foreground(colorYellow).Bold(true).Render("█")
			} else {
				vScrollChar = baseStyle.Foreground(colorAccent).Render("│")
			}
		}

		rowW := lipgloss.Width(rowContent)
		if rowW < contentW {
			rowContent += baseStyle.Render(strings.Repeat(" ", contentW-rowW))
		} else if rowW > contentW {
			rowContent = ansiCut(rowContent, 0, contentW)
		}
		lines = append(lines, rowContent+vScrollChar)
	}

	return strings.Join(lines, "\n")
}
