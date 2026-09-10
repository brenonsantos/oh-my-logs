package tui

import (
	"fmt"
	"strings"

	"github.com/brenoniehues/oh-my-logs/internal/record"
	"github.com/charmbracelet/lipgloss"
)

// isInspectorActive returns true when Hex or Binary display mode is selected in single-pane view.
func (m Model) isInspectorActive() bool {
	return (m.displayFormat == FormatHex || m.displayFormat == FormatBinary) && m.splitMode == SplitNone
}

// activeInspectorRecord resolves the specific log record to inspect in the bottom drawer.
func (m Model) activeInspectorRecord() (record.Record, bool) {
	if len(m.visible) == 0 {
		return record.Record{}, false
	}
	if m.selectedRow >= 0 && m.selectedRow < len(m.visible) {
		return m.visible[m.selectedRow], true
	}
	if m.follow && len(m.visible) > 0 {
		return m.visible[len(m.visible)-1], true
	}
	if m.scrollOffset >= 0 && m.scrollOffset < len(m.visible) {
		return m.visible[m.scrollOffset], true
	}
	return m.visible[0], true
}

// viewInspectorDivider renders the divider and metadata header for the bottom inspector drawer.
func (m Model) viewInspectorDivider() string {
	w := m.width
	if w <= 0 {
		w = 80
	}

	r, ok := m.activeInspectorRecord()
	rawLen := 0
	recID := uint64(0)
	if ok {
		rawLen = len(r.Raw)
		recID = r.ID
	}

	formatLabel := "HEX DUMP (16B/line)"
	if m.displayFormat == FormatBinary {
		formatLabel = "BINARY BITS (8B/line)"
	}

	leftTitle := fmt.Sprintf("─── [REC #%d] %s · %s ", recID, FormatByteLen(rawLen), formatLabel)
	leftRendered := theme.Accent.Bold(true).Render(leftTitle)
	leftW := lipgloss.Width(leftRendered)

	rightHints := " [Alt+j/k scroll] ───"
	rightRendered := theme.Muted.Render(rightHints)
	rightW := lipgloss.Width(rightRendered)

	rem := w - leftW - rightW
	if rem < 0 {
		rem = 0
	}

	middle := theme.Divider.Render(strings.Repeat("─", rem))
	return leftRendered + middle + rightRendered
}

// viewInspectorDrawer renders the canonical multi-line hex or binary byte dump.
func (m Model) viewInspectorDrawer() string {
	h := m.inspectorHeight
	if h <= 0 {
		return ""
	}

	r, ok := m.activeInspectorRecord()
	if !ok || len(r.Raw) == 0 {
		var emptyLines []string
		for i := 0; i < h; i++ {
			if i == h/2 {
				emptyLines = append(emptyLines, theme.Muted.Render("  (no byte payload in selected record)"))
			} else {
				emptyLines = append(emptyLines, "")
			}
		}
		return strings.Join(emptyLines, "\n")
	}

	var formattedLines []string
	if m.displayFormat == FormatBinary {
		binLines := FormatCanonicalBinaryLines(r.Raw)
		for _, l := range binLines {
			lineStr := theme.Muted.Render("  "+l.Offset) +
				theme.ModalItem.Render(l.Binary) +
				theme.Divider.Render(" │ ") +
				theme.Secondary.Render(l.ASCII)
			formattedLines = append(formattedLines, lineStr)
		}
	} else {
		hexLines := FormatCanonicalHexLines(r.Raw)
		for _, l := range hexLines {
			lineStr := theme.Muted.Render("  "+l.Offset) +
				theme.ModalItem.Render(l.Hex) +
				theme.Divider.Render(" │ ") +
				theme.Secondary.Render(l.ASCII)
			formattedLines = append(formattedLines, lineStr)
		}
	}

	// Clamp inspector scroll
	maxScroll := len(formattedLines) - h
	if maxScroll < 0 {
		maxScroll = 0
	}
	scroll := m.inspectorScroll
	if scroll > maxScroll {
		scroll = maxScroll
	}
	if scroll < 0 {
		scroll = 0
	}

	end := scroll + h
	if end > len(formattedLines) {
		end = len(formattedLines)
	}

	visibleLines := formattedLines[scroll:end]

	// Pad with empty lines if record dump is shorter than inspector height
	for len(visibleLines) < h {
		visibleLines = append(visibleLines, "")
	}

	return strings.Join(visibleLines, "\n")
}
