package tui

import (
	"strings"

	"github.com/brenoniehues/oh-my-logs/internal/record"
)

// tableWidth is the usable width for the table area.
func (m Model) tableWidth() int {
	return m.width
}

// renderRow calls cellFn for each column and joins them.
func (m Model) renderRow(cellFn func(col record.Column, width int) string) string {
	cols := m.effectiveColumns()
	colWidths := m.computeColWidths(cols)
	var parts []string
	for i, col := range cols {
		parts = append(parts, cellFn(col, colWidths[i]))
	}
	return strings.Join(parts, "  ")
}

// computeColWidths distributes available width across columns.
func (m Model) computeColWidths(cols []record.Column) []int {
	return m.computeColWidthsForWidth(cols, m.tableWidth())
}

// computeColWidthsForWidth distributes available width across columns for a given container width.
func (m Model) computeColWidthsForWidth(cols []record.Column, tableW int) []int {
	if len(cols) == 0 {
		return nil
	}
	widths := make([]int, len(cols))
	flexIdx := -1
	used := 3 // prefix takes 3 chars: cursor (1) + bookmark (1) + gap (1)

	// Each gap between columns takes 2 spaces: "  "
	if len(cols) > 1 {
		used += (len(cols) - 1) * 2
	}

	for i, col := range cols {
		if col.Width == 0 {
			flexIdx = i
		} else {
			widths[i] = col.Width
			used += col.Width
		}
	}

	// In narrow viewports, shrink hex or bin column if present to ensure flex column (e.g. ascii) has room
	if flexIdx >= 0 && (tableW-used) < 15 {
		for i, col := range cols {
			if (col.Field == "_hex" || col.Field == "_bin") && widths[i] > 18 {
				deficit := 15 - (tableW - used)
				shrink := widths[i] - 18
				if shrink > deficit {
					shrink = deficit
				}
				widths[i] -= shrink
				used -= shrink
				break
			}
		}
	}

	if flexIdx >= 0 {
		flex := tableW - used
		if flex < 8 {
			flex = 8
		}
		widths[flexIdx] = flex
	}

	return widths
}

// maxContentWidthForTab computes the maximum visual width across all visible rows for a given tab.
// If all rows fit within containerW, it returns containerW so no scrollbar is needed.
func (m Model) maxContentWidthForTab(tab *Tab, containerW int) int {
	if tab == nil || len(tab.Visible) == 0 || containerW <= 0 {
		return containerW
	}
	cols := m.effectiveColumnsForTab(tab)
	if len(cols) == 0 {
		return containerW
	}
	colWidths := m.computeColWidthsForWidth(cols, containerW)

	fixedW := 3
	if len(cols) > 1 {
		fixedW += (len(cols) - 1) * 2
	}

	flexIdx := -1
	for i, col := range cols {
		if col.Width == 0 {
			flexIdx = i
		} else if i < len(colWidths) {
			fixedW += colWidths[i]
		}
	}

	if flexIdx < 0 {
		if fixedW < containerW {
			return containerW
		}
		return fixedW
	}

	flexBaseW := 0
	if flexIdx < len(colWidths) {
		flexBaseW = colWidths[flexIdx]
	}

	flexCol := cols[flexIdx]
	maxFlex := flexBaseW

	for _, r := range tab.Visible {
		var val string
		switch flexCol.Field {
		case "raw":
			val = r.Raw
		case "_ascii":
			val = r.Raw
		case "message":
			val = r.Fields["message"]
			if val == "" {
				val = r.Raw
			}
		default:
			val = r.Fields[flexCol.Field]
		}
		l := len([]rune(val))
		if l > maxFlex {
			maxFlex = l
		}
	}

	totalW := fixedW + maxFlex
	if totalW < containerW {
		return containerW
	}
	return totalW
}

// maxContentWidth computes the maximum visual width for the active tab across tableWidth.
func (m Model) maxContentWidth() int {
	cur := m.currentTab()
	return m.maxContentWidthForTab(cur, m.tableWidth())
}

func isTimestampCol(col record.Column, tsField string) bool {
	// Uptime is device uptime, never a toggleable arrival timestamp column
	if col.Field == "uptime" || strings.EqualFold(col.Style, "uptime") {
		return false
	}
	if col.Field == "_delta" || col.Style == "delta" {
		return false
	}
	return col.Field == tsField || col.Field == "_ts" || col.Style == "timestamp"
}

func isDeltaCol(col record.Column) bool {
	return col.Field == "_delta" || col.Style == "delta"
}

// effectiveColumns returns the columns to render, respecting timestamp and delta mode for the current tab.
func (m Model) effectiveColumns() []record.Column {
	return m.effectiveColumnsForTab(m.currentTab())
}

// effectiveColumnsForTab returns the columns to render for a specific tab based on its DisplayFormat and timestamp mode.
func (m Model) effectiveColumnsForTab(tab *Tab) []record.Column {
	fmtMode := m.displayFormat
	if tab != nil {
		fmtMode = tab.DisplayFormat
	}

	var baseCols []record.Column
	switch fmtMode {
	case FormatRaw:
		baseCols = []record.Column{
			{Field: "raw", Title: "RAW LOG", Width: 0, Style: "primary"},
		}
	case FormatHex:
		baseCols = []record.Column{
			{Field: "_len", Title: "LEN", Width: 6, Style: "identifier"},
			{Field: "_hex", Title: "HEX DUMP", Width: 48, Style: "muted"},
			{Field: "_ascii", Title: "ASCII", Width: 0, Style: "primary"},
		}
	case FormatBinary:
		baseCols = []record.Column{
			{Field: "_len", Title: "LEN", Width: 6, Style: "identifier"},
			{Field: "_bin", Title: "BINARY BITS", Width: 36, Style: "muted"},
			{Field: "_ascii", Title: "ASCII", Width: 0, Style: "primary"},
		}
	default: // FormatParsed
		for _, col := range m.columns {
			if !isTimestampCol(col, m.tsField) && !isDeltaCol(col) {
				baseCols = append(baseCols, col)
			}
		}
	}

	if m.tsMode == TSModeOff {
		return baseCols
	}

	tsCol := record.Column{
		Field: m.tsField,
		Title: "Time",
		Width: 14,
		Style: "timestamp",
	}
	deltaCol := record.Column{
		Field: "_delta",
		Title: "Δt",
		Width: 10,
		Style: "delta",
	}

	switch m.tsMode {
	case TSModeDelta:
		result := make([]record.Column, 0, len(baseCols)+1)
		result = append(result, deltaCol)
		result = append(result, baseCols...)
		return result

	case TSModeBoth:
		result := make([]record.Column, 0, len(baseCols)+2)
		result = append(result, tsCol, deltaCol)
		result = append(result, baseCols...)
		return result

	default: // TSModeClock
		result := make([]record.Column, 0, len(baseCols)+1)
		result = append(result, tsCol)
		result = append(result, baseCols...)
		return result
	}
}
