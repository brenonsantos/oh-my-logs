package tui

import (
	"fmt"
	"strings"

	"github.com/brenoniehues/oh-my-logs/internal/record"
	"github.com/charmbracelet/lipgloss"
)

// isInspectorActive returns true when Hex or Binary display mode is selected in single-pane or split view.
func (m Model) isInspectorActive() bool {
	if m.splitMode != SplitNone {
		t0 := m.currentTabForPane(0)
		t1 := m.currentTabForPane(1)
		f0 := FormatParsed
		if t0 != nil {
			f0 = t0.DisplayFormat
		}
		f1 := FormatParsed
		if t1 != nil {
			f1 = t1.DisplayFormat
		}
		return f0 == FormatHex || f0 == FormatBinary || f1 == FormatHex || f1 == FormatBinary
	}
	return m.displayFormat == FormatHex || m.displayFormat == FormatBinary
}

// currentInspectorFormat resolves which format (Hex or Binary) should be rendered in the inspector drawer.
func (m Model) currentInspectorFormat() DisplayFormat {
	if m.splitMode != SplitNone {
		tActive := m.currentTabForPane(m.activePane)
		if tActive != nil && (tActive.DisplayFormat == FormatHex || tActive.DisplayFormat == FormatBinary) {
			return tActive.DisplayFormat
		}
		tOther := m.currentTabForPane(1 - m.activePane)
		if tOther != nil && (tOther.DisplayFormat == FormatHex || tOther.DisplayFormat == FormatBinary) {
			return tOther.DisplayFormat
		}
	}
	if m.displayFormat == FormatBinary {
		return FormatBinary
	}
	return FormatHex
}

// activeInspectorRecord resolves the specific log record to inspect in the bottom drawer.
func (m Model) activeInspectorRecord() (record.Record, bool) {
	if m.splitMode != SplitNone {
		t := m.currentTabForPane(m.activePane)
		if t != nil && len(t.Visible) > 0 {
			if t.SelectedRow >= 0 && t.SelectedRow < len(t.Visible) {
				return t.Visible[t.SelectedRow], true
			}
			if t.Follow && len(t.Visible) > 0 {
				return t.Visible[len(t.Visible)-1], true
			}
			if t.ScrollOffset >= 0 && t.ScrollOffset < len(t.Visible) {
				return t.Visible[t.ScrollOffset], true
			}
			return t.Visible[0], true
		}
	}

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

	fmtMode := m.currentInspectorFormat()
	formatLabel := "HEX DUMP (16B/line)"
	if fmtMode == FormatBinary {
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

	w := m.width
	if w <= 0 {
		w = 80
	}

	r, ok := m.activeInspectorRecord()
	if !ok || len(r.Raw) == 0 {
		var emptyLines []string
		for i := 0; i < h; i++ {
			if i == h/2 {
				msg := theme.Muted.Render("  (no byte payload in selected record)")
				curW := lipgloss.Width(msg)
				if curW < w {
					msg += strings.Repeat(" ", w-curW)
				}
				emptyLines = append(emptyLines, msg)
			} else {
				emptyLines = append(emptyLines, theme.RowNormal.Width(w).Render(""))
			}
		}
		return strings.Join(emptyLines, "\n")
	}

	fmtMode := m.currentInspectorFormat()
	var formattedLines []string
	if fmtMode == FormatBinary {
		binLines := FormatCanonicalBinaryLines(r.Raw)
		for _, l := range binLines {
			lineStr := theme.Muted.Render("  "+l.Offset) +
				theme.ModalItem.Render(l.Binary) +
				theme.Divider.Render(" │ ") +
				theme.Secondary.Render(l.ASCII)
			curW := lipgloss.Width(lineStr)
			if curW < w {
				lineStr += strings.Repeat(" ", w-curW)
			}
			formattedLines = append(formattedLines, lineStr)
		}
	} else {
		hexLines := FormatCanonicalHexLines(r.Raw)
		for _, l := range hexLines {
			lineStr := theme.Muted.Render("  "+l.Offset) +
				theme.ModalItem.Render(l.Hex) +
				theme.Divider.Render(" │ ") +
				theme.Secondary.Render(l.ASCII)
			curW := lipgloss.Width(lineStr)
			if curW < w {
				lineStr += strings.Repeat(" ", w-curW)
			}
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
		visibleLines = append(visibleLines, theme.RowNormal.Width(w).Render(""))
	}

	return strings.Join(visibleLines, "\n")
}
