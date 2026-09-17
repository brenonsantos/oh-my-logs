package tui

import (
	"strings"

	"github.com/brenoniehues/oh-my-logs/internal/config"
	"github.com/brenoniehues/oh-my-logs/internal/record"
)

// activeProfileKey returns the normalized slug key for the active profile.
func (m *Model) activeProfileKey() string {
	if m.profile != nil && m.profile.Name != "" {
		return strings.ToLower(strings.TrimSpace(m.profile.Name))
	}
	return "raw"
}

// loadColumnCustomization initializes column visibility and width overrides from saved settings.
func (m *Model) loadColumnCustomization() {
	m.colVisibility = make(map[string]bool)
	m.colWidthOverrides = make(map[string]int)

	if m.settings == nil {
		return
	}

	profKey := m.activeProfileKey()
	custom := m.settings.GetColumnCustomization(profKey)
	for _, hidden := range custom.HiddenColumns {
		m.colVisibility[hidden] = false
	}
	for field, w := range custom.ColumnWidths {
		if w > 0 {
			m.colWidthOverrides[field] = w
		}
	}
}

// saveColumnCustomization persists column visibility and width overrides to settings.json.
func (m *Model) saveColumnCustomization() {
	if m.settings == nil || m.appConfig == nil {
		return
	}

	profKey := m.activeProfileKey()

	var hidden []string
	for field, vis := range m.colVisibility {
		if !vis {
			hidden = append(hidden, field)
		}
	}

	widths := make(map[string]int)
	for field, w := range m.colWidthOverrides {
		if w > 0 {
			widths[field] = w
		}
	}

	custom := config.ColumnCustomization{
		HiddenColumns: hidden,
		ColumnWidths:  widths,
	}
	m.settings.SetColumnCustomization(profKey, custom)
	_ = m.appConfig.SaveSettings(m.settings)
}

// isColVisibleForTab returns whether a column field is visible for the specified tab.
func (m Model) isColVisibleForTab(tab *Tab, field string) bool {
	if tab != nil && tab.ColVisibility != nil {
		if vis, ok := tab.ColVisibility[field]; ok {
			return vis
		}
	}
	if vis, ok := m.colVisibility[field]; ok {
		return vis
	}
	return true
}

// setColVisibility sets the visibility of a column field.
func (m *Model) setColVisibility(tab *Tab, field string, visible bool) {
	if m.colVisibility == nil {
		m.colVisibility = make(map[string]bool)
	}
	m.colVisibility[field] = visible

	if tab != nil {
		if tab.ColVisibility == nil {
			tab.ColVisibility = make(map[string]bool)
		}
		tab.ColVisibility[field] = visible
	}
}

// getColWidthForTab returns the effective width for a column, respecting custom width overrides.
func (m Model) getColWidthForTab(tab *Tab, col record.Column) int {
	if tab != nil && tab.ColWidthOverrides != nil {
		if w, ok := tab.ColWidthOverrides[col.Field]; ok && w > 0 {
			return w
		}
	}
	if w, ok := m.colWidthOverrides[col.Field]; ok && w > 0 {
		return w
	}
	return col.Width
}

// setColWidth sets a custom width override for a column field.
func (m *Model) setColWidth(tab *Tab, field string, width int) {
	if m.colWidthOverrides == nil {
		m.colWidthOverrides = make(map[string]int)
	}
	if width <= 0 {
		delete(m.colWidthOverrides, field)
	} else {
		m.colWidthOverrides[field] = width
	}

	if tab != nil {
		if tab.ColWidthOverrides == nil {
			tab.ColWidthOverrides = make(map[string]int)
		}
		if width <= 0 {
			delete(tab.ColWidthOverrides, field)
		} else {
			tab.ColWidthOverrides[field] = width
		}
	}
}

// resetColumnCustomization clears all column customizations for the active profile.
func (m *Model) resetColumnCustomization(tab *Tab) {
	m.colVisibility = make(map[string]bool)
	m.colWidthOverrides = make(map[string]int)
	if tab != nil {
		tab.ColVisibility = nil
		tab.ColWidthOverrides = nil
	}
	m.saveColumnCustomization()
}

// openColumnModal transitions the UI into the column configuration modal.
func (m *Model) openColumnModal() {
	if len(m.columns) == 0 {
		m.message = "No columns available to configure"
		return
	}
	if m.colModalCursor >= len(m.columns) {
		m.colModalCursor = 0
	}
	m.mode = modeColumnModal
}

// visibleColumnsCount returns the count of currently visible columns in the active profile.
func (m Model) visibleColumnsCount(tab *Tab) int {
	count := 0
	for _, col := range m.columns {
		if m.isColVisibleForTab(tab, col.Field) {
			count++
		}
	}
	return count
}
