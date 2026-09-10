package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/brenoniehues/oh-my-logs/internal/clipboard"
	"github.com/brenoniehues/oh-my-logs/internal/config"
	"github.com/brenoniehues/oh-my-logs/internal/parser"
	"github.com/brenoniehues/oh-my-logs/internal/record"
	"github.com/brenoniehues/oh-my-logs/internal/serial"
)

func TestMouseClickTabBar_SwitchTab(t *testing.T) {
	m := newTestModel()
	m.width = 100
	m.height = 30
	m.recalcLayout()

	// Initially 1 tab
	if len(m.tabs) != 1 {
		t.Fatalf("expected 1 tab, got %d", len(m.tabs))
	}

	// Create a second tab
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlT})
	m = updated.(Model)
	m.mode = modeNormal // dismiss filter prompt
	if len(m.tabs) != 2 {
		t.Fatalf("expected 2 tabs, got %d", len(m.tabs))
	}
	if m.activeTab != 1 {
		t.Fatalf("expected activeTab to be 1 after create, got %d", m.activeTab)
	}

	// Click on Tab 1 (around X=3, Y=2)
	updated, _ = m.Update(tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
		X:      3,
		Y:      2,
	})
	m = updated.(Model)
	if m.activeTab != 0 {
		t.Errorf("expected activeTab to be 0 after clicking tab 1, got %d", m.activeTab)
	}

	// Click on Tab 2
	// Tab 1 width: "1: All (0)" -> 10 chars + 2 padding = 12 width. Tab 2 starts around X=14
	updated, _ = m.Update(tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
		X:      16,
		Y:      2,
	})
	m = updated.(Model)
	if m.activeTab != 1 {
		t.Errorf("expected activeTab to be 1 after clicking tab 2, got %d", m.activeTab)
	}
}

func TestMouseClickTabBar_NewAndClose(t *testing.T) {
	m := newTestModel()
	m.width = 100
	m.height = 30
	m.recalcLayout()

	// Add a second tab to display tab bar
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlT})
	m = updated.(Model)
	m.mode = modeNormal

	// Right-side hints: "Tab: cycle · ^T: new · ^W: close  "
	// hints is 34 chars. hintsStartX = 100 - 34 = 66
	// Click ^T: new at relX ~ 18 -> X = 66 + 18 = 84
	updated, _ = m.Update(tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
		X:      84,
		Y:      2,
	})
	m = updated.(Model)
	if len(m.tabs) != 3 {
		t.Errorf("expected 3 tabs after clicking ^T in tab bar, got %d", len(m.tabs))
	}
	m.mode = modeNormal

	// Click ^W: close at relX ~ 28 -> X = 66 + 28 = 94
	updated, _ = m.Update(tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
		X:      94,
		Y:      2,
	})
	m = updated.(Model)
	if len(m.tabs) != 2 {
		t.Errorf("expected 2 tabs after clicking ^W in tab bar, got %d", len(m.tabs))
	}
}

func TestMouseClickTableRow_SelectAndPause(t *testing.T) {
	m := newTestModel()
	m.width = 100
	m.height = 30
	m.recalcLayout()

	// Ingest 5 records
	for i := 1; i <= 5; i++ {
		updated, _ := m.Update(lineMsg(strings.Repeat("a", i)))
		m = updated.(Model)
	}

	if !m.follow {
		t.Fatalf("expected follow to be true initially")
	}

	// For 1 tab, tableStartY = 4
	// Click row 2 (Y = 4 + 2 = 6)
	updated, _ := m.Update(tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
		X:      10,
		Y:      6,
	})
	m = updated.(Model)

	if m.follow {
		t.Errorf("expected follow to be paused (false) after clicking row")
	}
	if m.selectedRow != 2 {
		t.Errorf("expected selectedRow to be 2, got %d", m.selectedRow)
	}

	// Press Esc to clear selection
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.selectedRow != -1 {
		t.Errorf("expected selectedRow to be -1 after Esc, got %d", m.selectedRow)
	}
}

func TestMouseDoubleClickTableRow_Copy(t *testing.T) {
	m := newTestModel()
	m.width = 100
	m.height = 30
	m.recalcLayout()

	updated, _ := m.Update(lineMsg("line to copy"))
	m = updated.(Model)

	// First click at Y=4
	updated, _ = m.Update(tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
		X:      5,
		Y:      4,
	})
	m = updated.(Model)

	// Second click within 400ms on same row
	updated, _ = m.Update(tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
		X:      5,
		Y:      4,
	})
	m = updated.(Model)

	if !strings.Contains(m.message, "Copied row to clipboard") {
		t.Errorf("expected message to contain 'Copied row to clipboard', got %q", m.message)
	}

	clipText, err := clipboard.Read()
	if err == nil {
		if clipText != "line to copy" {
			t.Errorf("expected clipboard to have 'line to copy', got %q", clipText)
		}
	}
}

func TestMouseDragSelection_MultiRowCopy(t *testing.T) {
	m := newTestModel()
	m.width = 100
	m.height = 30
	m.recalcLayout()

	// Ingest 5 lines
	lines := []string{"row0", "row1", "row2", "row3", "row4"}
	for _, l := range lines {
		updated, _ := m.Update(lineMsg(l))
		m = updated.(Model)
	}

	// 1. Mouse press at row 1 (Y = 4 + 1 = 5)
	updated, _ := m.Update(tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
		X:      10,
		Y:      5,
	})
	m = updated.(Model)
	if m.selectionStart != 1 || m.selectionEnd != 1 {
		t.Fatalf("expected selectionStart=1, selectionEnd=1, got %d, %d", m.selectionStart, m.selectionEnd)
	}

	// 2. Mouse motion drag down to row 3 (Y = 4 + 3 = 7)
	updated, _ = m.Update(tea.MouseMsg{
		Action: tea.MouseActionMotion,
		Button: tea.MouseButtonLeft,
		X:      10,
		Y:      7,
	})
	m = updated.(Model)
	if m.selectionStart != 1 || m.selectionEnd != 3 {
		t.Fatalf("expected selectionStart=1, selectionEnd=3 after drag, got %d, %d", m.selectionStart, m.selectionEnd)
	}

	// 3. Mouse release -> should keep rows 1..3 selected WITHOUT auto-copying
	updated, _ = m.Update(tea.MouseMsg{
		Action: tea.MouseActionRelease,
		Button: tea.MouseButtonLeft,
		X:      10,
		Y:      7,
	})
	m = updated.(Model)

	if !strings.Contains(m.message, "3 rows selected") {
		t.Errorf("expected message to contain '3 rows selected', got %q", m.message)
	}
	if strings.Contains(m.message, "Copied") {
		t.Errorf("mouse release should not auto-copy without pressing y, got %q", m.message)
	}

	// 4. Press 'y' -> now it should copy the 3 selected rows
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = updated.(Model)

	if !strings.Contains(m.message, "Copied 3 rows") || !strings.Contains(m.message, "to clipboard") {
		t.Errorf("expected message to contain 'Copied 3 rows' and 'to clipboard', got %q", m.message)
	}

	clipText, err := clipboard.Read()
	if err == nil {
		expected := "row1\nrow2\nrow3"
		if clipText != expected {
			t.Errorf("expected clipboard to be %q, got %q", expected, clipText)
		}
	}
}

func TestShiftUpDownMultiRowSelection(t *testing.T) {
	m := newTestModel()
	m.width = 100
	m.height = 30
	m.recalcLayout()

	// Ingest 6 lines
	lines := []string{"zero", "one", "two", "three", "four", "five"}
	for _, l := range lines {
		updated, _ := m.Update(lineMsg(l))
		m = updated.(Model)
	}

	// 1. Focus row 2 by clicking it (Y = 4 + 2 = 6)
	updated, _ := m.Update(tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
		X:      10,
		Y:      6,
	})
	m = updated.(Model)
	if m.selectedRow != 2 {
		t.Fatalf("expected selectedRow=2, got %d", m.selectedRow)
	}

	// 2. Press Shift+Down to expand selection to row 3 (2 rows: two, three)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'J'}})
	m = updated.(Model)
	if m.selectionStart != 2 || m.selectionEnd != 3 {
		t.Fatalf("expected selection range 2..3, got %d..%d", m.selectionStart, m.selectionEnd)
	}
	if !strings.Contains(m.message, "2 rows selected") {
		t.Errorf("expected message '2 rows selected', got %q", m.message)
	}

	// 3. Press Shift+Down again (3 rows: two, three, four)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'J'}})
	m = updated.(Model)
	if m.selectionStart != 2 || m.selectionEnd != 4 {
		t.Fatalf("expected selection range 2..4, got %d..%d", m.selectionStart, m.selectionEnd)
	}
	if !strings.Contains(m.message, "3 rows selected") {
		t.Errorf("expected message '3 rows selected', got %q", m.message)
	}

	// 4. Press Shift+Up to contract selection back to row 3 (2 rows: two, three)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'K'}})
	m = updated.(Model)
	if m.selectionStart != 2 || m.selectionEnd != 3 {
		t.Fatalf("expected selection range 2..3, got %d..%d", m.selectionStart, m.selectionEnd)
	}

	// 5. Press 'y' to copy the 2 selected rows ("two", "three")
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = updated.(Model)
	if !strings.Contains(m.message, "Copied 2 rows") || !strings.Contains(m.message, "to clipboard") {
		t.Errorf("expected 'Copied 2 rows' and 'to clipboard', got %q", m.message)
	}

	clipText, err := clipboard.Read()
	if err == nil {
		expected := "two\nthree"
		if clipText != expected {
			t.Errorf("expected clipboard to have %q, got %q", expected, clipText)
		}
	}

	// 6. Pressing normal 'k' collapses the multi-row selection and moves up to row 2
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m = updated.(Model)
	if m.selectionStart != -1 || m.selectionEnd != -1 {
		t.Errorf("expected selection range to collapse after pressing normal 'k', got start=%d, end=%d", m.selectionStart, m.selectionEnd)
	}
	if m.selectedRow != 2 {
		t.Errorf("expected selectedRow to move to 2, got %d", m.selectedRow)
	}
}

func TestKeyboardRowNavigationAndCopy(t *testing.T) {
	m := newTestModel()
	m.width = 100
	m.height = 30
	m.recalcLayout()

	// Ingest 4 lines
	for _, l := range []string{"alpha", "bravo", "charlie", "delta"} {
		updated, _ := m.Update(lineMsg(l))
		m = updated.(Model)
	}

	// Press 'k' to move selection up and pause
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m = updated.(Model)

	if m.follow {
		t.Errorf("expected follow to be false after 'k'")
	}
	if m.selectedRow != 2 {
		t.Errorf("expected selectedRow to be 2 after first 'k' from bottom, got %d", m.selectedRow)
	}

	// Press 'k' again
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m = updated.(Model)
	if m.selectedRow != 1 {
		t.Errorf("expected selectedRow to be 1 after second 'k', got %d", m.selectedRow)
	}

	// Press 'y' to copy selected row ("bravo")
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = updated.(Model)
	if !strings.Contains(m.message, "Copied row to clipboard") {
		t.Errorf("expected message to contain 'Copied row to clipboard', got %q", m.message)
	}

	clipText, err := clipboard.Read()
	if err == nil {
		if clipText != "bravo" {
			t.Errorf("expected clipboard to have 'bravo', got %q", clipText)
		}
	}

	// Press 'j' to move down to row 2
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = updated.(Model)
	if m.selectedRow != 2 {
		t.Errorf("expected selectedRow to be 2 after 'j', got %d", m.selectedRow)
	}

	// Press 'Y' to copy raw
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'Y'}})
	m = updated.(Model)
	if !strings.Contains(m.message, "Copied row to clipboard") {
		t.Errorf("expected copy message after 'Y', got %q", m.message)
	}
}

func TestCtrlVPasteInSearchAndFilter(t *testing.T) {
	m := newTestModel()
	m.width = 100
	m.height = 30
	m.recalcLayout()

	// 1. Copy search term to clipboard
	_ = clipboard.Copy("sensor_timeout")

	// Enter search mode (Ctrl+F)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlF})
	m = updated.(Model)
	if m.mode != modeSearch {
		t.Fatalf("expected modeSearch, got %v", m.mode)
	}

	// Send Ctrl+V
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlV})
	m = updated.(Model)
	if m.searchInput != "sensor_timeout" {
		t.Errorf("expected searchInput to be 'sensor_timeout', got %q", m.searchInput)
	}

	// Cancel search
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.mode != modeNormal {
		t.Fatalf("expected modeNormal after Esc, got %v", m.mode)
	}

	// 2. Copy filter term to clipboard
	_ = clipboard.Copy("level:err -timeout")

	// Enter filter mode ('f')
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	m = updated.(Model)
	if m.mode != modeFilter {
		t.Fatalf("expected modeFilter, got %v", m.mode)
	}

	// Send Ctrl+V
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlV})
	m = updated.(Model)
	if m.filterInput != "level:err -timeout" {
		t.Errorf("expected filterInput to be 'level:err -timeout', got %q", m.filterInput)
	}
}

func TestStatusBarClickToggleFollow(t *testing.T) {
	m := newTestModel()
	m.width = 100
	m.height = 30
	m.recalcLayout()

	// Add rows and pause
	for i := 0; i < 3; i++ {
		updated, _ := m.Update(lineMsg("data"))
		m = updated.(Model)
	}
	m.follow = false

	// Click status bar at Y = height - 2 = 28
	updated, _ := m.Update(tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
		X:      10,
		Y:      28,
	})
	m = updated.(Model)
	if !m.follow {
		t.Errorf("expected follow to be true after clicking status bar")
	}

	// Click again to pause
	updated, _ = m.Update(tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
		X:      10,
		Y:      28,
	})
	m = updated.(Model)
	if m.follow {
		t.Errorf("expected follow to be false after second click on status bar")
	}
}

func TestSelectionRenderingContiguousBackground(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	m := newTestModel()
	m.width = 120
	m.height = 20
	m.recalcLayout()

	// Ingest a line
	updated, _ := m.Update(lineMsg("hello world log message"))
	m = updated.(Model)

	// Select row 0
	m.selectedRow = 0
	tableOut := m.viewTable()

	lines := strings.Split(tableOut, "\n")
	if len(lines) == 0 {
		t.Fatalf("expected rendered table lines")
	}

	selectedLine := lines[0]
	// Verify selectedLine has selection indicator
	if !strings.Contains(selectedLine, "▶") {
		t.Errorf("expected selected row to contain '▶' cursor")
	}
	// Verify selectedLine contains the message
	if !strings.Contains(selectedLine, "hello world log message") {
		t.Errorf("expected selected row to contain message")
	}

	// Verify the row width matches m.tableWidth()
	lineWidth := lipgloss.Width(selectedLine)
	if lineWidth != m.width {
		t.Errorf("expected rendered line width %d, got %d", m.width, lineWidth)
	}

	t.Logf("selectedLine: %q", selectedLine)
	// Verify the background ANSI sequence is present
	if !strings.Contains(selectedLine, "30;40;59") && !strings.Contains(selectedLine, "48;2;") {
		t.Errorf("expected selected row to contain background color ANSI escape sequence, got %q", selectedLine)
	}
}

func TestSelectionRenderingZephyrColumns(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	prof := &parser.Profile{
		Name: "Zephyr",
		Columns: []parser.ColumnConfig{
			{Field: "uptime", Title: "Uptime (s)", Width: 12, Style: "uptime"},
			{Field: "level", Title: "Level", Width: 7, Style: "level", Colors: map[string]string{
				"wrn": "yellow",
				"inf": "cyan",
				"err": "red",
				"dbg": "gray",
			}},
			{Field: "message", Title: "Message", Width: 0, Style: "primary"},
		},
	}
	p, err := parser.NewRegexParser(`^\[\s*(?P<uptime>[0-9.]+)\]\s+<(?P<level>[a-zA-Z]+)>\s+(?P<message>.*)$`)
	if err != nil {
		t.Fatalf("BuildParser error: %v", err)
	}

	buf := record.NewBuffer(100)
	m := New(serial.Config{}, prof, p, buf, nil, &config.AppConfig{})
	m.width = 187
	m.height = 30
	m.recalcLayout()

	// Ingest a Zephyr line with wrn level
	line := "[      6.211] <wrn> net_core: Network interface initialization timed out"
	updated, _ := m.Update(lineMsg(line))
	m = updated.(Model)

	// Simulate drag selection on row 0
	m.selectionStart = 0
	m.selectionEnd = 1
	m.follow = false

	tableOut := m.viewTable()
	lines := strings.Split(tableOut, "\n")
	if len(lines) == 0 {
		t.Fatalf("expected rendered lines")
	}

	selLine := lines[0]
	t.Logf("Zephyr selected line: %q", selLine)

	// 1. Must contain multi-row indicator ▌
	if !strings.Contains(selLine, "▌") {
		t.Errorf("expected multi-row indicator '▌'")
	}

	// 2. Must contain all 3 fields: uptime, wrn, and message
	if !strings.Contains(selLine, "6.211") {
		t.Errorf("expected uptime 6.211 in rendered line")
	}
	if !strings.Contains(selLine, "wrn") {
		t.Errorf("expected level 'wrn' in rendered line")
	}
	if !strings.Contains(selLine, "net_core: Network interface initialization timed out") {
		t.Errorf("expected message in rendered line")
	}

	// 3. Width must match table width exactly (187)
	if lipgloss.Width(selLine) != 187 {
		t.Errorf("expected rendered line width 187, got %d", lipgloss.Width(selLine))
	}

	// 4. Background color 48;2;30;40;59 (or 48;2;30;41;59) must appear multiple times
	// (for prefix, uptime, separator, level, separator, message, and padding)
	bgCount := strings.Count(selLine, "48;2;30;4")
	if bgCount < 3 {
		t.Errorf("expected background color to be applied across all columns and separators, got count %d", bgCount)
	}
}

func TestMouseClickInSplitMode(t *testing.T) {
	records := make([]record.Record, 10)
	for i := 0; i < 10; i++ {
		records[i] = record.Record{
			ID:        uint64(i + 1),
			Raw:       fmt.Sprintf("log line %d", i),
			Timestamp: time.Now(),
			Fields:    map[string]string{"message": fmt.Sprintf("msg %d", i)},
		}
	}

	m := newTestModelWithRecords(records)
	m.width = 100
	m.height = 30
	m.recalcLayout()

	// 1. Add a second tab (so len(m.tabs) == 2)
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlT})
	m = newM.(Model)
	if len(m.tabs) != 2 {
		t.Fatalf("expected 2 tabs, got %d", len(m.tabs))
	}

	// 2. Toggle vertical split
	m.toggleSplit(SplitVertical)
	if m.splitMode != SplitVertical {
		t.Fatalf("expected SplitVertical")
	}

	// With 2 tabs in SplitVertical:
	// Row 0: Title bar
	// Row 1: Title divider
	// Row 2: Tab bar
	// Row 3: Tab divider
	// Row 4: Pane header
	// Row 5: Pane divider
	// Row 6: Data row 0
	// Row 7: Data row 1
	// Row 8: Data row 2
	updated, _ := m.Update(tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
		X:      10, // left pane
		Y:      6,  // data row 0
	})
	m = updated.(Model)

	if m.selectedRow != 0 {
		t.Errorf("clicking Y=6 with 2 tabs in split view: expected selectedRow=0, got %d", m.selectedRow)
	}

	// Click row 2 at Y=8
	updated, _ = m.Update(tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
		X:      10, // left pane
		Y:      8,  // data row 2
	})
	m = updated.(Model)

	if m.selectedRow != 2 {
		t.Errorf("clicking Y=8 with 2 tabs in split view: expected selectedRow=2, got %d", m.selectedRow)
	}
}

