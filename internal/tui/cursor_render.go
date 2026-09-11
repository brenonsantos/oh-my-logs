package tui

import (
	"strings"

	"github.com/brenoniehues/oh-my-logs/internal/timing"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// ansiCut extracts visual substring [offset : offset+width] from an ANSI-styled string.
func ansiCut(s string, offset int, width int) string {
	if width <= 0 {
		return ""
	}
	if offset <= 0 {
		return ansi.Truncate(s, width, "")
	}
	leftTrimmed := ansi.TruncateLeft(s, offset, "")
	return ansi.Truncate(leftTrimmed, width, "")
}

// padOrFlex pads s to width, or if isFlex is true and len(runes) >= width, returns s without truncation.
func padOrFlex(s string, width int, isFlex bool) string {
	if width <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) > width {
		if isFlex {
			return s
		}
		if width > 1 {
			return string(runes[:width-1]) + "…"
		}
		return "…"
	}
	return s + strings.Repeat(" ", width-len(runes))
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

// applyRowCursor overlays a character-level cursor and selection highlight on a rendered rowBody.
func applyRowCursor(rowBody string, cursorCol int, selStart int, selEnd int) string {
	if cursorCol < 0 {
		return rowBody
	}
	w := lipgloss.Width(rowBody)
	if w == 0 {
		cursorStyle := lipgloss.NewStyle().Background(colorCyan).Foreground(lipgloss.Color("#000000")).Bold(true)
		return cursorStyle.Render(" ")
	}

	if cursorCol >= w {
		rowBody += strings.Repeat(" ", cursorCol-w+1)
		w = cursorCol + 1
	}

	cursorStyle := lipgloss.NewStyle().Background(colorCyan).Foreground(lipgloss.Color("#000000")).Bold(true)
	selStyle := lipgloss.NewStyle().Background(colorAccent).Foreground(lipgloss.Color("#000000"))

	hasSel := selStart >= 0 && selEnd >= 0 && selStart != selEnd
	if !hasSel {
		before := ansiCut(rowBody, 0, cursorCol)
		curChar := ansiCut(rowBody, cursorCol, 1)
		if curChar == "" {
			curChar = " "
		}
		after := ansiCut(rowBody, cursorCol+1, w-(cursorCol+1))
		return before + cursorStyle.Render(ansi.Strip(curChar)) + after
	}

	minSel, maxSel := selStart, selEnd
	if minSel > maxSel {
		minSel, maxSel = maxSel, minSel
	}
	if minSel < 0 {
		minSel = 0
	}
	if maxSel > w {
		maxSel = w
	}

	left := ansiCut(rowBody, 0, minSel)
	right := ansiCut(rowBody, maxSel, w-maxSel)

	var mid string
	if cursorCol == minSel {
		curChar := ansiCut(rowBody, cursorCol, 1)
		if curChar == "" {
			curChar = " "
		}
		selRest := ansiCut(rowBody, minSel+1, maxSel-(minSel+1))
		mid = cursorStyle.Render(ansi.Strip(curChar)) + selStyle.Render(ansi.Strip(selRest))
	} else if cursorCol >= maxSel {
		selPart := ansiCut(rowBody, minSel, maxSel-minSel)
		curChar := ansiCut(rowBody, cursorCol, 1)
		if curChar == "" {
			curChar = " "
		}
		afterCursor := ansiCut(rowBody, cursorCol+1, w-(cursorCol+1))
		return left + selStyle.Render(ansi.Strip(selPart)) + cursorStyle.Render(ansi.Strip(curChar)) + afterCursor
	} else {
		selBefore := ansiCut(rowBody, minSel, cursorCol-minSel)
		curChar := ansiCut(rowBody, cursorCol, 1)
		if curChar == "" {
			curChar = " "
		}
		selAfter := ansiCut(rowBody, cursorCol+1, maxSel-(cursorCol+1))
		mid = selStyle.Render(ansi.Strip(selBefore)) + cursorStyle.Render(ansi.Strip(curChar)) + selStyle.Render(ansi.Strip(selAfter))
	}

	return left + mid + right
}

// selectedRowPlainText returns the plain unstyled text of the row at absIdx.
func (m Model) selectedRowPlainText(absIdx int) string {
	if absIdx < 0 || absIdx >= len(m.visible) {
		return ""
	}
	r := m.visible[absIdx]
	cols := m.effectiveColumns()
	colWidths := m.computeColWidths(cols)

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
		if col.Field == "_delta" || col.Style == "delta" {
			if absIdx > 0 && !m.visible[absIdx-1].Timestamp.IsZero() && !r.Timestamp.IsZero() {
				val = timing.FormatDelta(r.Timestamp.Sub(m.visible[absIdx-1].Timestamp))
			} else if r.Delta > 0 {
				val = timing.FormatDelta(r.Delta)
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
		cellParts = append(cellParts, padOrFlex(val, w, col.Width == 0))
	}
	return strings.Join(cellParts, "  ")
}

// highlightSubstring wraps occurrences of q with theme.Highlight, using baseStyle for non-matching portions.
func highlightSubstring(cell, q string, baseStyle lipgloss.Style) string {
	lower := strings.ToLower(cell)
	lowerQ := strings.ToLower(q)
	if lowerQ == "" {
		return baseStyle.Render(cell)
	}
	var sb strings.Builder
	start := 0
	for {
		idx := strings.Index(lower[start:], lowerQ)
		if idx < 0 {
			sb.WriteString(baseStyle.Render(cell[start:]))
			break
		}
		matchStart := start + idx
		matchEnd := matchStart + len(q)
		if matchStart > start {
			sb.WriteString(baseStyle.Render(cell[start:matchStart]))
		}
		sb.WriteString(theme.Highlight.Render(cell[matchStart:matchEnd]))
		start = matchEnd
	}
	return sb.String()
}
