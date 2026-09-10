package tui

import (
	"fmt"
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

func TestColumnWidthsHexAndBinaryFlex(t *testing.T) {
	m := newTestModel()
	m.tsMode = TSModeOff

	// 1. Hex Mode at width 140
	m.displayFormat = FormatHex
	m.tabs[0].DisplayFormat = FormatHex
	colsHex := m.effectiveColumnsForTab(&m.tabs[0])
	widthsHex140 := m.computeColWidthsForWidth(colsHex, 140)

	// cols: LEN (6), HEX DUMP (flex), ASCII (24)
	// used = 3 (prefix) + 4 (2 gaps of 2) + 6 (LEN) + 24 (ASCII) = 37
	// flex = 140 - 37 = 103 columns!
	if widthsHex140[1] != 103 {
		t.Errorf("expected hex dump column width 103 at tableW=140, got %d", widthsHex140[1])
	}
	if widthsHex140[2] != 24 {
		t.Errorf("expected ascii column width 24, got %d", widthsHex140[2])
	}

	// 2. Binary Mode at width 140
	m.displayFormat = FormatBinary
	m.tabs[0].DisplayFormat = FormatBinary
	colsBin := m.effectiveColumnsForTab(&m.tabs[0])
	widthsBin140 := m.computeColWidthsForWidth(colsBin, 140)

	// cols: LEN (6), BINARY BITS (flex), ASCII (16)
	// used = 3 + 4 + 6 + 16 = 29
	// flex = 140 - 29 = 111 columns!
	if widthsBin140[1] != 111 {
		t.Errorf("expected binary bits column width 111 at tableW=140, got %d", widthsBin140[1])
	}
	if widthsBin140[2] != 16 {
		t.Errorf("expected ascii column width 16, got %d", widthsBin140[2])
	}

	// 3. Narrow viewport at width 45: ASCII scales down to prevent flex starvation
	widthsNarrow := m.computeColWidthsForWidth(colsHex, 45)
	if widthsNarrow[1] < 20 {
		t.Errorf("expected flex column to have at least 20 cols in narrow viewport, got %d", widthsNarrow[1])
	}
}

func TestWiresharkInspectorDrawer(t *testing.T) {
	records := []record.Record{
		{ID: 1, Raw: "Hello World! This is a long serial log message that will exceed normal line limits.", Timestamp: time.Now(), Fields: map[string]string{"message": "Hello World!"}},
		{ID: 2, Raw: "Booting MCU with 126 bytes of payload testing...", Timestamp: time.Now(), Fields: map[string]string{"message": "Booting MCU..."}},
	}
	m := newTestModelWithRecords(records)
	m.width = 120
	m.height = 30
	m.selectedRow = 0

	// Initially FormatParsed: inspector is inactive
	m.recalcLayout()
	if m.isInspectorActive() {
		t.Errorf("expected inspector to be inactive in FormatParsed")
	}
	if m.inspectorHeight != 0 {
		t.Errorf("expected inspectorHeight 0 in FormatParsed, got %d", m.inspectorHeight)
	}

	// 1. Switch to FormatHex: inspector automatically activates
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}}) // -> Raw
	newM, _ = newM.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}}) // -> Hex
	m = newM.(Model)

	if !m.isInspectorActive() {
		t.Fatalf("expected inspector to be active in FormatHex")
	}
	if m.inspectorHeight < 4 {
		t.Errorf("expected inspectorHeight >= 4, got %d", m.inspectorHeight)
	}

	view := m.View()
	if !strings.Contains(view, "HEX DUMP (16B/line)") {
		t.Errorf("expected view to contain inspector divider 'HEX DUMP (16B/line)', got:\n%s", view)
	}
	if !strings.Contains(view, "0000: ") {
		t.Errorf("expected view to contain offset '0000: ', got:\n%s", view)
	}
	if !strings.Contains(view, " │ ") {
		t.Errorf("expected view to contain canonical ASCII gutter ' │ ', got:\n%s", view)
	}

	// 2. Alt+j scrolls inspector
	m.inspectorScroll = 0
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}, Alt: true})
	m = newM.(Model)
	if m.inspectorScroll != 1 {
		t.Errorf("expected inspectorScroll 1 after Alt+j, got %d", m.inspectorScroll)
	}

	// 3. Moving selected row (j) resets inspectorScroll to 0
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = newM.(Model)
	if m.inspectorScroll != 0 {
		t.Errorf("expected inspectorScroll reset to 0 after moving rows, got %d", m.inspectorScroll)
	}

	// 4. Switch to Binary: inspector displays binary bits
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}}) // -> Binary
	m = newM.(Model)
	if !m.isInspectorActive() {
		t.Fatalf("expected inspector active in FormatBinary")
	}
	viewBin := m.View()
	if !strings.Contains(viewBin, "BINARY BITS (8B/line)") {
		t.Errorf("expected view to contain 'BINARY BITS (8B/line)', got:\n%s", viewBin)
	}

	// 5. Switch back to Parsed: inspector auto closes
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}}) // -> Parsed
	m = newM.(Model)
	if m.isInspectorActive() {
		t.Errorf("expected inspector inactive in FormatParsed")
	}
	if m.inspectorHeight != 0 {
		t.Errorf("expected inspectorHeight 0 in FormatParsed, got %d", m.inspectorHeight)
	}
}

func TestViewHeightConsistency(t *testing.T) {
	records := make([]record.Record, 100)
	for i := 0; i < 100; i++ {
		records[i] = record.Record{
			ID:        uint64(i + 1),
			Raw:       fmt.Sprintf("[%d] Test log message payload with 52 bytes of raw data!!", i+1),
			Timestamp: time.Now(),
			Fields:    map[string]string{"message": fmt.Sprintf("msg %d", i+1)},
		}
	}
	m := newTestModelWithRecords(records)
	m.width = 241
	m.height = 59
	m.recalcLayout()

	checkLines := func(desc string) {
		t.Helper()
		v := m.View()
		lines := strings.Split(v, "\n")
		t.Logf("%s: view lines = %d (expected %d), tableH = %d, inspH = %d, split = %d", desc, len(lines), m.height, m.tableHeight, m.inspectorHeight, m.splitMode)
		if len(lines) != m.height {
			t.Errorf("%s: expected exactly %d lines in View(), got %d", desc, m.height, len(lines))
		}
	}

	checkLines("1 tab, parsed")

	// Switch to Hex
	m.displayFormat = FormatHex
	m.currentTab().DisplayFormat = FormatHex
	m.recalcLayout()
	checkLines("1 tab, hex")

	// Add second tab
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlT})
	m = newM.(Model)
	m.recalcLayout()
	checkLines("2 tabs, hex")

	// Split vertical (|)
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'|'}})
	m = newM.(Model)
	checkLines("split vertical")

	// Focus pane 0 (left tab, which has FormatHex!)
	m.activePane = 0
	m.splitLeftTab = 0
	m.activeTab = 0
	m.syncModelToActiveTab()
	checkLines("split vertical with left hex")
}

func TestUserBugSequence(t *testing.T) {
	records := make([]record.Record, 400)
	for i := 0; i < 400; i++ {
		records[i] = record.Record{
			ID:        uint64(i + 1),
			Raw:       fmt.Sprintf("[%d] Test log message payload with 52 bytes of raw data!!", i+1),
			Timestamp: time.Now(),
			Fields:    map[string]string{"message": fmt.Sprintf("msg %d", i+1)},
		}
	}
	m := newTestModelWithRecords(records)
	m.width = 241
	m.height = 59
	m.recalcLayout()

	// 1. Switch to FormatHex (press x twice: Parsed -> Raw -> Hex)
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	newM, _ = newM.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	m = newM.(Model)

	v1 := m.View()
	lines1 := strings.Split(v1, "\n")
	t.Logf("State 1 (Hex, 1 tab): lines=%d (height=%d), tableH=%d, inspH=%d", len(lines1), m.height, m.tableHeight, m.inspectorHeight)

	// 2. Press | to enter split view (toggleSplit)
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'|'}})
	m = newM.(Model)

	v2 := m.View()
	lines2 := strings.Split(v2, "\n")
	t.Logf("State 2 (Split vertical): lines=%d (height=%d), tableH=%d, inspH=%d", len(lines2), m.height, m.tableHeight, m.inspectorHeight)
	if len(lines2) != m.height {
		t.Fatalf("State 2 expected exactly %d lines, got %d", m.height, len(lines2))
	}
	if !strings.Contains(v2, "HEX DUMP (16B/line)") {
		t.Errorf("State 2 expected view to contain inspector divider in split mode")
	}
	if !strings.Contains(v2, "0000: ") {
		t.Errorf("State 2 expected view to contain hex dump line in split mode")
	}

	// 4. Test tab switching between Hex and Parsed
	// Tab 1 is Hex, Tab 2 is Parsed
	m.tabs[0].DisplayFormat = FormatHex
	m.tabs[1].DisplayFormat = FormatParsed
	m.activeTab = 0
	m.syncModelToActiveTab()
	// Switch to Tab 2 (Parsed) with Tab key
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = newM.(Model)
	vTab2 := m.View()
	linesTab2 := strings.Split(vTab2, "\n")
	t.Logf("State 4 (Switch to Tab 2 Parsed): lines=%d (height=%d), tableH=%d, inspH=%d", len(linesTab2), m.height, m.tableHeight, m.inspectorHeight)
	if len(linesTab2) != m.height {
		t.Fatalf("State 4 expected exactly %d lines, got %d", m.height, len(linesTab2))
	}

	// Switch back to Tab 1 (Hex) with Tab key
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = newM.(Model)
	vTab1 := m.View()
	linesTab1 := strings.Split(vTab1, "\n")
	t.Logf("State 5 (Switch back to Tab 1 Hex): lines=%d (height=%d), tableH=%d, inspH=%d", len(linesTab1), m.height, m.tableHeight, m.inspectorHeight)
	if len(linesTab1) != m.height {
		t.Fatalf("State 5 expected exactly %d lines, got %d", m.height, len(linesTab1))
	}

	// Now unsplit with |
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'|'}})
	m = newM.(Model)
	vUnsplit := m.View()
	linesUnsplit := strings.Split(vUnsplit, "\n")
	t.Logf("State 6 (Unsplit): lines=%d (height=%d), tableH=%d, inspH=%d", len(linesUnsplit), m.height, m.tableHeight, m.inspectorHeight)
	if len(linesUnsplit) != m.height {
		t.Fatalf("State 6 expected exactly %d lines, got %d", m.height, len(linesUnsplit))
	}

	// Now press Tab to switch to Tab 2 (Parsed) in single pane
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = newM.(Model)
	vTab2Single := m.View()
	linesTab2Single := strings.Split(vTab2Single, "\n")
	t.Logf("State 7 (Tab 2 Parsed in single pane): lines=%d (height=%d), tableH=%d, inspH=%d", len(linesTab2Single), m.height, m.tableHeight, m.inspectorHeight)
	if len(linesTab2Single) != m.height {
		t.Fatalf("State 7 expected exactly %d lines, got %d", m.height, len(linesTab2Single))
	}
	if strings.Contains(vTab2Single, "HEX DUMP (16B/line)") {
		t.Errorf("State 7 should NOT contain inspector drawer in Parsed format")
	}
}



