package tui

import (
	"path/filepath"
	"testing"

	"github.com/brenoniehues/oh-my-logs/internal/config"
	"github.com/brenoniehues/oh-my-logs/internal/parser"
	"github.com/brenoniehues/oh-my-logs/internal/record"
	tea "github.com/charmbracelet/bubbletea"
)

func setupTestModelWithColumns(t *testing.T) Model {
	t.Helper()
	tempDir := t.TempDir()
	appCfg := &config.AppConfig{
		ConfigDir:   tempDir,
		ProfilesDir: filepath.Join(tempDir, "profiles"),
		LogsDir:     filepath.Join(tempDir, "logs"),
	}

	cols := []record.Column{
		{Field: "uptime", Title: "Uptime", Width: 19, Style: "uptime"},
		{Field: "level", Title: "Level", Width: 7, Style: "level"},
		{Field: "module", Title: "Module", Width: 22, Style: "identifier"},
		{Field: "message", Title: "Message", Width: 0, Style: "primary"},
	}

	prof := &parser.Profile{
		Name: "TestProfile",
		Columns: []parser.ColumnConfig{
			{Field: "uptime", Title: "Uptime", Width: 19, Style: "uptime"},
			{Field: "level", Title: "Level", Width: 7, Style: "level"},
			{Field: "module", Title: "Module", Width: 22, Style: "identifier"},
			{Field: "message", Title: "Message", Width: 0, Style: "primary"},
		},
	}

	m := Model{
		keys:              defaultKeyMap(),
		width:             120,
		height:            30,
		tableHeight:       20,
		columns:           cols,
		profile:           prof,
		appConfig:         appCfg,
		settings:          &config.Settings{Baud: 115200, BufferCapacity: 1000},
		colVisibility:     make(map[string]bool),
		colWidthOverrides: make(map[string]int),
		tsMode:            TSModeOff,
		tabs: []Tab{
			{Name: "All", ViewportState: ViewportState{Follow: true, SelectedRow: -1}},
		},
	}
	m.loadColumnCustomization()
	return m
}

func TestColumnCustomization_BasicGetSet(t *testing.T) {
	m := setupTestModelWithColumns(t)

	// All should be visible by default
	tab := m.currentTab()
	if !m.isColVisibleForTab(tab, "uptime") || !m.isColVisibleForTab(tab, "module") {
		t.Errorf("expected all columns visible by default")
	}

	// Hide module
	m.setColVisibility(tab, "module", false)
	if m.isColVisibleForTab(tab, "module") {
		t.Errorf("expected module to be hidden")
	}
	if !m.isColVisibleForTab(tab, "uptime") {
		t.Errorf("expected uptime to still be visible")
	}

	// Override width
	uptimeCol := m.columns[0]
	if m.getColWidthForTab(tab, uptimeCol) != 19 {
		t.Errorf("expected default uptime width 19, got %d", m.getColWidthForTab(tab, uptimeCol))
	}

	m.setColWidth(tab, "uptime", 25)
	if m.getColWidthForTab(tab, uptimeCol) != 25 {
		t.Errorf("expected custom uptime width 25, got %d", m.getColWidthForTab(tab, uptimeCol))
	}

	// Reset
	m.resetColumnCustomization(tab)
	if !m.isColVisibleForTab(tab, "module") {
		t.Errorf("expected module visible after reset")
	}
	if m.getColWidthForTab(tab, uptimeCol) != 19 {
		t.Errorf("expected uptime width restored to 19, got %d", m.getColWidthForTab(tab, uptimeCol))
	}
}

func TestColumnCustomization_EffectiveColumnsAndWidths(t *testing.T) {
	m := setupTestModelWithColumns(t)
	tab := m.currentTab()

	initialCols := m.effectiveColumns()
	if len(initialCols) != 4 {
		t.Fatalf("expected 4 effective columns, got %d", len(initialCols))
	}

	initialWidths := m.computeColWidths(initialCols)
	initialMessageW := initialWidths[3] // flex column

	// Hide module (22 width + 2 colGap freed)
	m.setColVisibility(tab, "module", false)
	colsAfterHide := m.effectiveColumns()
	if len(colsAfterHide) != 3 {
		t.Fatalf("expected 3 effective columns after hiding module, got %d", len(colsAfterHide))
	}
	for _, c := range colsAfterHide {
		if c.Field == "module" {
			t.Errorf("module column should not be present in effective columns")
		}
	}

	widthsAfterHide := m.computeColWidths(colsAfterHide)
	messageWAfterHide := widthsAfterHide[2]
	if messageWAfterHide <= initialMessageW {
		t.Errorf("expected flex message column to expand when module is hidden; got %d, was %d", messageWAfterHide, initialMessageW)
	}

	// Fallback safety: hide all remaining columns
	m.setColVisibility(tab, "uptime", false)
	m.setColVisibility(tab, "level", false)
	m.setColVisibility(tab, "message", false)

	fallbackCols := m.effectiveColumns()
	if len(fallbackCols) == 0 {
		t.Errorf("expected safety fallback to prevent 0 columns")
	}
}

func TestColumnCustomization_ModalKeys(t *testing.T) {
	m := setupTestModelWithColumns(t)

	// Open modal
	m.openColumnModal()
	if m.mode != modeColumnModal {
		t.Fatalf("expected modeColumnModal, got %v", m.mode)
	}
	if m.colModalCursor != 0 {
		t.Errorf("expected cursor at 0, got %d", m.colModalCursor)
	}

	// Navigate down
	mModel, _ := m.handleColumnModalKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = mModel.(Model)
	if m.colModalCursor != 1 {
		t.Errorf("expected cursor at 1, got %d", m.colModalCursor)
	}

	// Navigate to module (index 2)
	mModel, _ = m.handleColumnModalKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = mModel.(Model)
	if m.colModalCursor != 2 {
		t.Errorf("expected cursor at 2, got %d", m.colModalCursor)
	}

	// Toggle module off with Space
	tab := m.currentTab()
	mModel, _ = m.handleColumnModalKey(tea.KeyMsg{Type: tea.KeySpace})
	m = mModel.(Model)
	if m.isColVisibleForTab(tab, "module") {
		t.Errorf("expected module to be toggled off")
	}

	// Toggle module back on
	mModel, _ = m.handleColumnModalKey(tea.KeyMsg{Type: tea.KeySpace})
	m = mModel.(Model)
	if !m.isColVisibleForTab(tab, "module") {
		t.Errorf("expected module to be toggled on")
	}

	// Expand width with '+'
	moduleCol := m.columns[2]
	originalW := m.getColWidthForTab(tab, moduleCol)
	mModel, _ = m.handleColumnModalKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
	m = mModel.(Model)
	expandedW := m.getColWidthForTab(tab, moduleCol)
	if expandedW != originalW+2 {
		t.Errorf("expected width %d, got %d", originalW+2, expandedW)
	}

	// Shrink width with '-'
	mModel, _ = m.handleColumnModalKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'-'}})
	m = mModel.(Model)
	shrunkW := m.getColWidthForTab(tab, moduleCol)
	if shrunkW != originalW {
		t.Errorf("expected width %d, got %d", originalW, shrunkW)
	}

	// Reset with 'r'
	mModel, _ = m.handleColumnModalKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = mModel.(Model)
	if m.message == "" {
		t.Errorf("expected confirmation message after reset")
	}

	// Close modal with 'Esc'
	mModel, _ = m.handleColumnModalKey(tea.KeyMsg{Type: tea.KeyEscape})
	m = mModel.(Model)
	if m.mode != modeNormal {
		t.Errorf("expected modeNormal after Esc, got %v", m.mode)
	}
}

func TestColumnCustomization_LastColumnVisibilityGuard(t *testing.T) {
	m := setupTestModelWithColumns(t)
	tab := m.currentTab()
	m.openColumnModal()

	// Hide uptime (0), level (1), module (2)
	m.setColVisibility(tab, "uptime", false)
	m.setColVisibility(tab, "level", false)
	m.setColVisibility(tab, "module", false)

	// Cursor at message (3)
	m.colModalCursor = 3

	// Attempt to toggle message off -> should be prevented
	mModel, _ := m.handleColumnModalKey(tea.KeyMsg{Type: tea.KeySpace})
	m = mModel.(Model)

	if !m.isColVisibleForTab(tab, "message") {
		t.Errorf("guard failed: message column was hidden despite being the only visible column")
	}
	if m.message != "At least one column must remain visible" {
		t.Errorf("unexpected message: %q", m.message)
	}
}

func TestColumnCustomization_Persistence(t *testing.T) {
	m := setupTestModelWithColumns(t)
	tab := m.currentTab()

	// Customize columns
	m.setColVisibility(tab, "module", false)
	m.setColWidth(tab, "uptime", 28)
	m.saveColumnCustomization()

	// Verify in settings struct
	cust := m.settings.GetColumnCustomization("TestProfile")
	if len(cust.HiddenColumns) != 1 || cust.HiddenColumns[0] != "module" {
		t.Errorf("expected HiddenColumns [module], got %v", cust.HiddenColumns)
	}
	if cust.ColumnWidths["uptime"] != 28 {
		t.Errorf("expected ColumnWidths[uptime] 28, got %d", cust.ColumnWidths["uptime"])
	}

	// Create fresh model loading from same settings
	freshModel := Model{
		keys:              defaultKeyMap(),
		columns:           m.columns,
		profile:           m.profile,
		appConfig:         m.appConfig,
		settings:          m.settings,
		colVisibility:     make(map[string]bool),
		colWidthOverrides: make(map[string]int),
	}
	freshModel.loadColumnCustomization()

	if freshModel.isColVisibleForTab(nil, "module") {
		t.Errorf("expected module to be hidden in freshly initialized model")
	}
	uptimeCol := freshModel.columns[0]
	if freshModel.getColWidthForTab(nil, uptimeCol) != 28 {
		t.Errorf("expected uptime width 28 in freshly initialized model, got %d", freshModel.getColWidthForTab(nil, uptimeCol))
	}
}
