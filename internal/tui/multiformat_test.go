package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/brenoniehues/oh-my-logs/internal/record"
	tea "github.com/charmbracelet/bubbletea"
)

func TestFormatCyclingWithKeyX(t *testing.T) {
	m := newTestModel()
	m.width = 100
	m.height = 30

	if m.displayFormat != FormatParsed {
		t.Fatalf("expected initial format FormatParsed, got %v", m.displayFormat)
	}

	// Press x -> FormatRaw
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	m = newModel.(Model)
	if m.displayFormat != FormatRaw || m.currentTab().DisplayFormat != FormatRaw {
		t.Fatalf("expected FormatRaw, got %v", m.displayFormat)
	}
	if !strings.Contains(m.message, "Raw Text") {
		t.Errorf("expected flash message containing 'Raw Text', got %q", m.message)
	}

	// Press x -> FormatHex
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	m = newModel.(Model)
	if m.displayFormat != FormatHex || m.currentTab().DisplayFormat != FormatHex {
		t.Fatalf("expected FormatHex, got %v", m.displayFormat)
	}
	if !strings.Contains(m.message, "Hex Dump") {
		t.Errorf("expected flash message containing 'Hex Dump', got %q", m.message)
	}

	// Press x -> FormatBinary
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	m = newModel.(Model)
	if m.displayFormat != FormatBinary || m.currentTab().DisplayFormat != FormatBinary {
		t.Fatalf("expected FormatBinary, got %v", m.displayFormat)
	}
	if !strings.Contains(m.message, "Binary Bits") {
		t.Errorf("expected flash message containing 'Binary Bits', got %q", m.message)
	}

	// Press x -> FormatParsed
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	m = newModel.(Model)
	if m.displayFormat != FormatParsed || m.currentTab().DisplayFormat != FormatParsed {
		t.Fatalf("expected FormatParsed, got %v", m.displayFormat)
	}
}

func TestSplitDualFormatCorrelation(t *testing.T) {
	now := time.Now()
	records := []record.Record{
		{ID: 1, Raw: "Hello World!", Timestamp: now, Fields: map[string]string{"message": "Hello World!"}},
		{ID: 2, Raw: "Booting MCU...", Timestamp: now.Add(10 * time.Millisecond), Fields: map[string]string{"message": "Booting MCU..."}},
	}
	m := newTestModelWithRecords(records)

	// Split vertically: pane 0 (tab 0), pane 1 (tab 1)
	m.toggleSplit(SplitVertical)
	if m.splitMode != SplitVertical {
		t.Fatalf("expected split mode vertical, got %v", m.splitMode)
	}

	// Tab 0 is Parsed
	m.tabs[0].DisplayFormat = FormatParsed
	// Tab 1 is Hex Dump
	m.tabs[1].DisplayFormat = FormatHex

	cols0 := m.effectiveColumnsForTab(&m.tabs[0])
	cols1 := m.effectiveColumnsForTab(&m.tabs[1])

	// Check Tab 0 columns
	foundHex0 := false
	for _, c := range cols0 {
		if c.Field == "_hex" {
			foundHex0 = true
		}
	}
	if foundHex0 {
		t.Errorf("expected Tab 0 not to have _hex column, got %v", cols0)
	}

	// Check Tab 1 columns
	foundHex1 := false
	for _, c := range cols1 {
		if c.Field == "_hex" {
			foundHex1 = true
		}
	}
	if !foundHex1 {
		t.Errorf("expected Tab 1 to have _hex column, got %v", cols1)
	}

	// Render panes
	lines0 := m.renderPaneView(&m.tabs[0], 0, 50, 10, true)
	lines1 := m.renderPaneView(&m.tabs[1], 1, 50, 10, false)

	joined0 := strings.Join(lines0, "\n")
	joined1 := strings.Join(lines1, "\n")

	// Tab 0 should show "Hello World!" message
	if !strings.Contains(joined0, "Hello World!") {
		t.Errorf("expected Tab 0 to show text message, got:\n%s", joined0)
	}

	// Tab 1 should show Hex "48 65 6c 6c 6f"
	if !strings.Contains(joined1, "48 65 6c 6c 6f") {
		t.Errorf("expected Tab 1 to show hex bytes, got:\n%s", joined1)
	}
}

func TestTableRenderingHexAndBinary(t *testing.T) {
	now := time.Now()
	r := record.Record{
		ID:        1,
		Raw:       "ABC",
		Timestamp: now,
		Fields:    map[string]string{"message": "ABC"},
	}
	m := newTestModelWithRecords([]record.Record{r})
	m.tsMode = TSModeOff // simplify column width test

	// 1. Hex Mode
	m.currentTab().DisplayFormat = FormatHex
	m.displayFormat = FormatHex
	viewHex := m.viewTable()

	// A=41, B=42, C=43
	if !strings.Contains(viewHex, "41 42 43") {
		t.Errorf("expected viewTable to contain hex bytes '41 42 43', got:\n%s", viewHex)
	}
	if !strings.Contains(viewHex, "3B") { // length 3 bytes
		t.Errorf("expected viewTable to contain length '3B', got:\n%s", viewHex)
	}

	// 2. Binary Mode
	m.currentTab().DisplayFormat = FormatBinary
	m.displayFormat = FormatBinary
	viewBin := m.viewTable()

	// A=01000001
	if !strings.Contains(viewBin, "01000001") {
		t.Errorf("expected viewTable to contain binary bits '01000001', got:\n%s", viewBin)
	}

	// 3. Raw Mode
	m.currentTab().DisplayFormat = FormatRaw
	m.displayFormat = FormatRaw
	viewRaw := m.viewTable()
	if !strings.Contains(viewRaw, "ABC") {
		t.Errorf("expected viewTable to contain raw text 'ABC', got:\n%s", viewRaw)
	}
}

func TestSettingsModalDisplayFormat(t *testing.T) {
	m := newTestModel()
	m.mode = modeSettings
	m.settingsCursor = int(settingRowFormat)

	if m.settingRowName(settingRowFormat) != "Display Format" {
		t.Errorf("unexpected name: %s", m.settingRowName(settingRowFormat))
	}

	// Press Right -> cycles to FormatRaw
	m = m.adjustSetting(settingRowFormat, 1)
	if m.displayFormat != FormatRaw || m.settings.DisplayFormat != "raw" {
		t.Errorf("expected FormatRaw, got %v", m.displayFormat)
	}

	// Press Right -> cycles to FormatHex
	m = m.adjustSetting(settingRowFormat, 1)
	if m.displayFormat != FormatHex || m.settings.DisplayFormat != "hex" {
		t.Errorf("expected FormatHex, got %v", m.displayFormat)
	}
	if m.settingValueLabel(settingRowFormat) != "Hex Dump" {
		t.Errorf("expected label 'Hex Dump', got %q", m.settingValueLabel(settingRowFormat))
	}
}

func TestTabBarAndStatusBarBadge(t *testing.T) {
	m := newTestModel()
	m.width = 120

	// Set Hex format
	m.currentTab().DisplayFormat = FormatHex
	m.displayFormat = FormatHex

	tabBar := m.viewTabBar()
	if !strings.Contains(tabBar, "[HEX]") {
		t.Errorf("expected tab bar to show [HEX] badge, got:\n%s", tabBar)
	}

	statusBar := m.viewStatusBar()
	if !strings.Contains(statusBar, "HEX") {
		t.Errorf("expected status bar to show HEX indicator, got:\n%s", statusBar)
	}
}
