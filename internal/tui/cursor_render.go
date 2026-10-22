package tui

import (
	"fmt"
	"strings"
	"unicode"

	"regexp"
	"github.com/brenoniehues/oh-my-logs/internal/serial"
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
	hasSel := selStart >= 0 && selEnd >= 0 && selStart != selEnd
	if !hasSel && cursorCol < 0 {
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

	if cursorCol < 0 {
		selPart := ansiCut(rowBody, minSel, maxSel-minSel)
		return left + selStyle.Render(ansi.Strip(selPart)) + right
	}

	var mid string
	if cursorCol < minSel {
		selPart := ansiCut(rowBody, minSel, maxSel-minSel)
		mid = selStyle.Render(ansi.Strip(selPart))
	} else if cursorCol == minSel {
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
	if r.IsMarker {
		ts := ""
		if !r.Timestamp.IsZero() {
			ts = r.Timestamp.Format("15:04:05.000")
		} else if r.Fields["time"] != "" {
			ts = r.Fields["time"]
		}
		if r.MarkerNote != "" {
			if ts != "" {
				return fmt.Sprintf("── 📌 MARKER [%s]: %s", ts, r.MarkerNote)
			}
			return fmt.Sprintf("── 📌 MARKER: %s", r.MarkerNote)
		}
		if ts != "" {
			return fmt.Sprintf("── 📌 MARKER [%s]", ts)
		}
		return "── 📌 MARKER"
	}

	cols := m.effectiveColumns()
	if isUnparsedRow(r, cols, m.displayFormat) {
		val := r.Raw
		if val == "" {
			val = r.Fields["message"]
		}
		return sanitizeCellValue(serial.ExpandTabs(val, 8))
	}

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
func highlightSubstring(cell, q string, matchCase, wholeWord bool, baseStyle lipgloss.Style) string {
	if q == "" {
		return baseStyle.Render(cell)
	}

	pattern := regexp.QuoteMeta(q)
	if wholeWord {
		pattern = `\b` + pattern + `\b`
	}
	if !matchCase {
		pattern = `(?i)` + pattern
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return baseStyle.Render(cell)
	}

	matches := re.FindAllStringIndex(cell, -1)
	if len(matches) == 0 {
		return baseStyle.Render(cell)
	}

	var sb strings.Builder
	start := 0
	for _, match := range matches {
		matchStart := match[0]
		matchEnd := match[1]

		if matchStart > start {
			sb.WriteString(baseStyle.Render(cell[start:matchStart]))
		}
		sb.WriteString(theme.Highlight.Render(cell[matchStart:matchEnd]))
		start = matchEnd
	}
	
	if start < len(cell) {
		sb.WriteString(baseStyle.Render(cell[start:]))
	}

	return sb.String()
}

// isWordRune returns whether r should be considered part of a selectable word / token.
func isWordRune(r rune) bool {
	if unicode.IsLetter(r) || unicode.IsDigit(r) {
		return true
	}
	switch r {
	case '_', '.', '-', ':', '/', '@', '#', '$', '+', '%', '=', '~':
		return true
	default:
		return false
	}
}

// findWordAt returns the start and end column indices (runes) and the word text
// containing the specified column in text.
func findWordAt(text string, col int) (int, int, string) {
	runes := []rune(text)
	if len(runes) == 0 {
		return -1, -1, ""
	}
	if col < 0 {
		col = 0
	}
	if col >= len(runes) {
		col = len(runes) - 1
	}

	// If the clicked character is whitespace or non-word, look for adjacent word rune
	if !isWordRune(runes[col]) {
		if col > 0 && isWordRune(runes[col-1]) {
			col--
		} else if col+1 < len(runes) && isWordRune(runes[col+1]) {
			col++
		} else {
			// Single punctuation or space character
			return col, col + 1, string(runes[col : col+1])
		}
	}

	start := col
	for start > 0 && isWordRune(runes[start-1]) {
		start--
	}

	end := col + 1
	for end < len(runes) && isWordRune(runes[end]) {
		end++
	}

	return start, end, string(runes[start:end])
}

// rowCharSelection calculates the start and end column character selection on row absIdx.
func (m Model) rowCharSelection(absIdx int) (int, int, bool) {
	if absIdx < 0 || absIdx >= len(m.visible) {
		return -1, -1, false
	}

	// 1. Single row drag or char selection on selectedRow
	if m.selectionStart < 0 || m.selectionEnd < 0 || m.selectionStart == m.selectionEnd {
		targetRow := m.selectedRow
		if targetRow < 0 {
			targetRow = m.selectionStart
		}
		if absIdx == targetRow && m.charSelStart >= 0 && m.charSelEnd >= 0 && m.charSelStart != m.charSelEnd {
			minCol, maxCol := m.charSelStart, m.charSelEnd
			if minCol > maxCol {
				minCol, maxCol = maxCol, minCol
			}
			return minCol, maxCol, true
		}
		return -1, -1, false
	}

	// 2. Multi-row rectangular block selection: [minCol : maxCol] for every row in range
	if m.selectionStart >= 0 && m.selectionEnd >= 0 && m.selectionStart != m.selectionEnd {
		if m.charSelStart < 0 || m.charSelEnd < 0 || m.charSelStart == m.charSelEnd {
			return -1, -1, false
		}

		topRow := m.selectionStart
		bottomRow := m.selectionEnd
		if topRow > bottomRow {
			topRow, bottomRow = bottomRow, topRow
		}

		if absIdx < topRow || absIdx > bottomRow {
			return -1, -1, false
		}

		minCol := m.charSelStart
		maxCol := m.charSelEnd
		if minCol > maxCol {
			minCol, maxCol = maxCol, minCol
		}
		return minCol, maxCol, true
	}

	return -1, -1, false
}
