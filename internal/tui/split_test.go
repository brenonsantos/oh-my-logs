package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/brenoniehues/oh-my-logs/internal/filter"
	"github.com/brenoniehues/oh-my-logs/internal/record"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func newTestModelWithRecords(records []record.Record) Model {
	m := newTestModel()
	m.width = 120
	m.height = 30
	m.tableHeight = 23
	for _, r := range records {
		m.buffer.Add(r)
	}
	m.rebuildVisible()
	return m
}

func TestFindClosestRecordIndex(t *testing.T) {
	// Empty slice
	if idx := findLastRecordBeforeOrAtID(nil, 100); idx != -1 {
		t.Errorf("expected -1 for empty slice, got %d", idx)
	}

	records := []record.Record{
		{ID: 10, Raw: "rec1"},
		{ID: 25, Raw: "rec2"},
		{ID: 50, Raw: "rec3"},
		{ID: 80, Raw: "rec4"},
		{ID: 100, Raw: "rec5"},
	}

	// Target before first
	if idx := findLastRecordBeforeOrAtID(records, 5); idx != 0 {
		t.Errorf("expected 0 for target 5, got %d", idx)
	}

	// Target after last
	if idx := findLastRecordBeforeOrAtID(records, 200); idx != 4 {
		t.Errorf("expected 4 for target 200, got %d", idx)
	}

	// Exact matches
	if idx := findLastRecordBeforeOrAtID(records, 25); idx != 1 {
		t.Errorf("expected 1 for target 25, got %d", idx)
	}
	if idx := findLastRecordBeforeOrAtID(records, 80); idx != 3 {
		t.Errorf("expected 3 for target 80, got %d", idx)
	}

	// In-between: returns last record <= targetID (stays still until passing next ID)
	// Between 25 and 50: target 30 -> last record <= 30 is 25 (idx 1)
	if idx := findLastRecordBeforeOrAtID(records, 30); idx != 1 {
		t.Errorf("expected 1 for target 30, got %d", idx)
	}
	// Target 49 -> still 25 (idx 1), does not jump to 50 until target reaches 50
	if idx := findLastRecordBeforeOrAtID(records, 49); idx != 1 {
		t.Errorf("expected 1 for target 49, got %d", idx)
	}
	if idx := findLastRecordBeforeOrAtID(records, 50); idx != 2 {
		t.Errorf("expected 2 for target 50, got %d", idx)
	}

	// Between 50 and 80: target 70 -> last record <= 70 is 50 (idx 2)
	if idx := findLastRecordBeforeOrAtID(records, 70); idx != 2 {
		t.Errorf("expected 2 for target 70, got %d", idx)
	}
}

func TestDivergentStreamChronologicalSync(t *testing.T) {
	// Stream A (sparse, e.g. BLE events every 100 ID units)
	// IDs: 0, 100, 200, 300, 400
	var streamA []record.Record
	for i := 0; i <= 4; i++ {
		streamA = append(streamA, record.Record{
			ID:     uint64(i * 100),
			Raw:    fmt.Sprintf("BLE event %d", i),
			Fields: map[string]string{"message": fmt.Sprintf("BLE event %d", i)},
		})
	}

	// Stream B (dense, e.g. Sensor telemetry every 5 ID units)
	// IDs: 0, 5, 10, 15, ..., 450
	var streamB []record.Record
	for i := 0; i <= 90; i++ {
		streamB = append(streamB, record.Record{
			ID:     uint64(i * 5),
			Raw:    fmt.Sprintf("Sensor reading %d", i),
			Fields: map[string]string{"message": fmt.Sprintf("Sensor reading %d", i)},
		})
	}

	// When navigating in Stream A to event 2 (ID 200):
	targetID := streamA[2].ID // 200
	closestBIdx := findLastRecordBeforeOrAtID(streamB, targetID)
	if closestBIdx < 0 || streamB[closestBIdx].ID != 200 {
		t.Fatalf("expected stream B to find record with ID 200, got index %d with ID %d", closestBIdx, streamB[closestBIdx].ID)
	}

	// When navigating in Stream A to an event with arbitrary ID 213:
	closestBIdx = findLastRecordBeforeOrAtID(streamB, 213)
	// Last record in Stream B <= 213 is ID 210
	if streamB[closestBIdx].ID != 210 {
		t.Fatalf("expected stream B to find ID 210 for target 213, got ID %d", streamB[closestBIdx].ID)
	}
}

func TestSplitViewToggleAndAutoTabCreation(t *testing.T) {
	var records []record.Record
	for i := 0; i < 50; i++ {
		records = append(records, record.Record{
			ID:     uint64(i),
			Raw:    fmt.Sprintf("log entry %d", i),
			Fields: map[string]string{"message": fmt.Sprintf("log entry %d", i)},
		})
	}
	m := newTestModelWithRecords(records)

	if m.splitMode != SplitNone {
		t.Errorf("expected initial splitMode to be SplitNone, got %v", m.splitMode)
	}
	if len(m.tabs) != 1 {
		t.Errorf("expected 1 initial tab, got %d", len(m.tabs))
	}

	// Press "|" to toggle vertical split
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'|'}})
	m = updated.(Model)

	if m.splitMode != SplitVertical {
		t.Errorf("expected splitMode to be SplitVertical, got %v", m.splitMode)
	}
	// Verify Tab 2 was automatically created
	if len(m.tabs) != 2 {
		t.Fatalf("expected 2 tabs after splitting, got %d", len(m.tabs))
	}
	if m.activePane != 0 {
		t.Errorf("expected activePane to be 0, got %d", m.activePane)
	}
	if m.splitRightTab != 1 {
		t.Errorf("expected splitRightTab to be 1, got %d", m.splitRightTab)
	}
	if !m.syncScroll {
		t.Errorf("expected syncScroll to be true by default on split")
	}

	// Press "|" again to close split
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'|'}})
	m = updated.(Model)

	if m.splitMode != SplitNone {
		t.Errorf("expected splitMode to revert to SplitNone, got %v", m.splitMode)
	}

	// Press "_" to toggle horizontal split
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'_'}})
	m = updated.(Model)

	if m.splitMode != SplitHorizontal {
		t.Errorf("expected splitMode to be SplitHorizontal, got %v", m.splitMode)
	}
}

func TestSwitchPaneFocusAndIndependentFilters(t *testing.T) {
	var records []record.Record
	for i := 0; i < 30; i++ {
		tag := "ble"
		if i%2 == 0 {
			tag = "sensor"
		}
		records = append(records, record.Record{
			ID:     uint64(i),
			Raw:    fmt.Sprintf("[%s] event %d", tag, i),
			Fields: map[string]string{"tag": tag, "message": fmt.Sprintf("event %d", i)},
		})
	}
	m := newTestModelWithRecords(records)

	// Split vertical
	m.toggleSplit(SplitVertical)
	if len(m.tabs) != 2 {
		t.Fatalf("expected 2 tabs, got %d", len(m.tabs))
	}

	// Tab 1 filter: "ble"
	f1, _ := filter.New("ble")
	m.tabs[0].Filter = f1
	m.tabs[0].FilterRaw = "ble"
	m.tabs[0].Visible = nil
	for _, r := range records {
		if f1.Matches(r) {
			m.tabs[0].Visible = append(m.tabs[0].Visible, r)
		}
	}

	// Tab 2 filter: "sensor"
	f2, _ := filter.New("sensor")
	m.tabs[1].Filter = f2
	m.tabs[1].FilterRaw = "sensor"
	m.tabs[1].Visible = nil
	for _, r := range records {
		if f2.Matches(r) {
			m.tabs[1].Visible = append(m.tabs[1].Visible, r)
		}
	}

	m.syncModelToActiveTab()

	// Initial active pane: 0 (Tab 1: ble)
	if m.activePane != 0 {
		t.Errorf("expected activePane to be 0, got %d", m.activePane)
	}
	if len(m.visible) != 15 {
		t.Errorf("expected 15 visible in Tab 1, got %d", len(m.visible))
	}

	// Switch focus with 'w'
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}})
	m = updated.(Model)

	if m.activePane != 1 {
		t.Fatalf("expected activePane to be 1 after 'w', got %d", m.activePane)
	}
	if m.activeTabIdx() != 1 {
		t.Fatalf("expected activeTabIdx to be 1, got %d", m.activeTabIdx())
	}
	// Active visible should now be Tab 2's sensor records
	if len(m.visible) != 15 {
		t.Errorf("expected 15 visible in Tab 2, got %d", len(m.visible))
	}
	if m.visible[0].Fields["tag"] != "sensor" {
		t.Errorf("expected Tab 2 first record tag to be 'sensor', got '%s'", m.visible[0].Fields["tag"])
	}

	// Switch focus back with 'w'
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}})
	m = updated.(Model)

	if m.activePane != 0 {
		t.Fatalf("expected activePane to return to 0, got %d", m.activePane)
	}
	if m.visible[0].Fields["tag"] != "ble" {
		t.Errorf("expected Tab 1 first record tag to be 'ble', got '%s'", m.visible[0].Fields["tag"])
	}
}

func TestChronologicalSyncScroll(t *testing.T) {
	var records []record.Record
	for i := 0; i < 100; i++ {
		records = append(records, record.Record{
			ID:     uint64(i),
			Raw:    fmt.Sprintf("record %d", i),
			Fields: map[string]string{"message": fmt.Sprintf("record %d", i)},
		})
	}
	m := newTestModelWithRecords(records)

	// Split vertical
	m.toggleSplit(SplitVertical)
	m.tableHeight = 10

	// Set Tab 0 to display even IDs (0, 2, 4, ..., 98) = 50 records
	var evens []record.Record
	for _, r := range records {
		if r.ID%2 == 0 {
			evens = append(evens, r)
		}
	}
	m.tabs[0].Visible = evens
	m.tabs[0].ScrollOffset = 0
	m.tabs[0].SelectedRow = 0
	m.tabs[0].Follow = false

	// Set Tab 1 to display odd IDs (1, 3, 5, ..., 99) = 50 records
	var odds []record.Record
	for _, r := range records {
		if r.ID%2 == 1 {
			odds = append(odds, r)
		}
	}
	m.tabs[1].Visible = odds
	m.tabs[1].ScrollOffset = 0
	m.tabs[1].SelectedRow = 0
	m.tabs[1].Follow = false

	m.syncModelToActiveTab()

	// Select row 20 in Tab 0 (ID 40)
	m.selectedRow = 20
	m.scrollOffset = 15
	m.syncOtherPaneChronologically()

	// In Tab 1, last visible record <= ID 40 is ID 39 (index 19 in odds)
	targetIdx := m.tabs[1].SelectedRow
	if targetIdx < 0 || targetIdx >= len(odds) {
		t.Fatalf("expected Tab 1 selected row to be aligned, got %d", targetIdx)
	}
	targetID := odds[targetIdx].ID
	if targetID != 39 {
		t.Fatalf("expected Tab 1 target ID to be 39 (last record <= 40), got %d", targetID)
	}

	// Toggle sync scroll off
	m.toggleSyncScroll()
	if m.syncScroll {
		t.Errorf("expected syncScroll to be false")
	}

	// Move row in Tab 0, Tab 1 should not change
	oldTab1Row := m.tabs[1].SelectedRow
	m.selectedRow = 45
	m.syncOtherPaneChronologically()
	if m.tabs[1].SelectedRow != oldTab1Row {
		t.Errorf("expected Tab 1 selected row to remain %d when sync is off, got %d", oldTab1Row, m.tabs[1].SelectedRow)
	}
}

func TestSyncScrollHoldsStillAcrossHiddenRecords(t *testing.T) {
	// Tab A has 50 records (IDs 0 .. 49)
	var allRecs []record.Record
	for i := 0; i < 50; i++ {
		allRecs = append(allRecs, record.Record{
			ID:     uint64(i),
			Raw:    fmt.Sprintf("entry %d", i),
			Fields: map[string]string{"message": fmt.Sprintf("entry %d", i)},
		})
	}
	m := newTestModelWithRecords(allRecs)
	m.toggleSplit(SplitVertical)
	m.tableHeight = 10

	// Tab B has only 3 sparse records: ID 10, ID 25, ID 40
	tabBRecs := []record.Record{
		{ID: 10, Raw: "sparse 10", Fields: map[string]string{"message": "sparse 10"}},
		{ID: 25, Raw: "sparse 25", Fields: map[string]string{"message": "sparse 25"}},
		{ID: 40, Raw: "sparse 40", Fields: map[string]string{"message": "sparse 40"}},
	}
	m.tabs[1].Visible = tabBRecs
	m.tabs[1].ScrollOffset = 0
	m.tabs[1].SelectedRow = 0

	m.syncModelToActiveTab()

	// 1. Move Tab A across IDs 10 to 24 (which are hidden in Tab B)
	// Tab B must stay completely still at index 0 (ID 10)
	for id := 10; id <= 24; id++ {
		m.selectedRow = id
		m.scrollOffset = 0
		m.syncOtherPaneChronologically()

		if m.tabs[1].SelectedRow != 0 {
			t.Fatalf("at Tab A ID %d, expected Tab B SelectedRow to stay at 0 (ID 10), got %d", id, m.tabs[1].SelectedRow)
		}
		if m.tabs[1].ScrollOffset != 0 {
			t.Fatalf("at Tab A ID %d, expected Tab B ScrollOffset to stay at 0, got %d", id, m.tabs[1].ScrollOffset)
		}
	}

	// 2. Tab A reaches ID 25: Tab A goes past Tab B's ID, so Tab B advances to index 1 (ID 25)
	m.selectedRow = 25
	m.syncOtherPaneChronologically()
	if m.tabs[1].SelectedRow != 1 {
		t.Fatalf("at Tab A ID 25, expected Tab B SelectedRow to advance to 1 (ID 25), got %d", m.tabs[1].SelectedRow)
	}

	// 3. Move Tab A across IDs 26 to 39: Tab B must stay completely still at index 1 (ID 25)
	for id := 26; id <= 39; id++ {
		m.selectedRow = id
		m.syncOtherPaneChronologically()

		if m.tabs[1].SelectedRow != 1 {
			t.Fatalf("at Tab A ID %d, expected Tab B SelectedRow to stay at 1 (ID 25), got %d", id, m.tabs[1].SelectedRow)
		}
	}

	// 4. Tab A reaches ID 40: Tab B advances to index 2 (ID 40)
	m.selectedRow = 40
	m.syncOtherPaneChronologically()
	if m.tabs[1].SelectedRow != 2 {
		t.Fatalf("at Tab A ID 40, expected Tab B SelectedRow to advance to 2 (ID 40), got %d", m.tabs[1].SelectedRow)
	}
}

func TestSplitTabsPositionsFrozen(t *testing.T) {
	m := newTestModel()
	m.tabs = append(m.tabs, Tab{Name: "Tab 2", Visible: m.buffer.All()})
	m.tabs = append(m.tabs, Tab{Name: "Tab 3", Visible: m.buffer.All()})

	// Split vertical: pane 0 (left) is Tab 1 (idx 0), pane 1 (right) is Tab 2 (idx 1)
	m.toggleSplit(SplitVertical)
	if m.paneTabIdx(0) != 0 || m.paneTabIdx(1) != 1 {
		t.Fatalf("expected panes (0, 1), got (%d, %d)", m.paneTabIdx(0), m.paneTabIdx(1))
	}

	// 1. Switching focus between panes: left and right tab positions MUST stay frozen
	m.switchPaneFocus()
	if m.activePane != 1 {
		t.Fatalf("expected activePane 1, got %d", m.activePane)
	}
	if m.paneTabIdx(0) != 0 || m.paneTabIdx(1) != 1 {
		t.Errorf("expected pane positions to stay frozen at (0, 1) after focus switch, got (%d, %d)", m.paneTabIdx(0), m.paneTabIdx(1))
	}

	m.switchPaneFocus()
	if m.activePane != 0 {
		t.Fatalf("expected activePane 0, got %d", m.activePane)
	}
	if m.paneTabIdx(0) != 0 || m.paneTabIdx(1) != 1 {
		t.Errorf("expected pane positions to stay frozen at (0, 1) after focus switch, got (%d, %d)", m.paneTabIdx(0), m.paneTabIdx(1))
	}

	// 2. While focused on pane 0, switching to Tab 2 (which is open on pane 1):
	// Simply switches focus to pane 1 without shifting either tab's position!
	m.switchTab(1)
	if m.activePane != 1 {
		t.Errorf("expected selecting Tab 2 to move focus to pane 1, got %d", m.activePane)
	}
	if m.paneTabIdx(0) != 0 || m.paneTabIdx(1) != 1 {
		t.Errorf("expected positions to remain frozen at (0, 1), got (%d, %d)", m.paneTabIdx(0), m.paneTabIdx(1))
	}

	// 3. While focused on pane 1, switching to Tab 3 (which is not open):
	// Loads Tab 3 in pane 1 while pane 0 remains frozen at Tab 1
	m.switchTab(2)
	if m.paneTabIdx(0) != 0 {
		t.Errorf("expected pane 0 to remain frozen at Tab 1 (0), got %d", m.paneTabIdx(0))
	}
	if m.paneTabIdx(1) != 2 {
		t.Errorf("expected pane 1 to display Tab 3 (2), got %d", m.paneTabIdx(1))
	}
}

func TestSplitViewRenderingDimensions(t *testing.T) {
	var records []record.Record
	for i := 0; i < 40; i++ {
		records = append(records, record.Record{
			ID:     uint64(i),
			Raw:    fmt.Sprintf("log line %d", i),
			Fields: map[string]string{"message": fmt.Sprintf("log line %d", i)},
		})
	}
	m := newTestModelWithRecords(records)
	m.width = 100
	m.tableHeight = 15

	// 1. Vertical split
	m.toggleSplit(SplitVertical)
	viewV := m.viewSplitTable()
	linesV := strings.Split(viewV, "\n")

	// Total height must be m.tableHeight + 2
	expectedH := m.tableHeight + 2
	if len(linesV) != expectedH {
		t.Errorf("expected %d vertical split lines, got %d", expectedH, len(linesV))
	}
	for i, line := range linesV {
		w := lipgloss.Width(line)
		if w != m.width {
			t.Errorf("line %d has width %d, expected %d", i, w, m.width)
		}
		if !strings.Contains(line, "│") && !strings.Contains(line, "┼") {
			t.Errorf("line %d missing vertical divider: %s", i, line)
		}
	}

	// 2. Horizontal split
	m.toggleSplit(SplitHorizontal)
	viewH := m.viewSplitTable()
	linesH := strings.Split(viewH, "\n")

	if len(linesH) != expectedH {
		t.Errorf("expected %d horizontal split lines, got %d", expectedH, len(linesH))
	}
	for i, line := range linesH {
		w := lipgloss.Width(line)
		if w != m.width {
			t.Errorf("line %d has width %d, expected %d", i, w, m.width)
		}
	}
}

func TestSplitMouseClickPaneFocus(t *testing.T) {
	var records []record.Record
	for i := 0; i < 20; i++ {
		records = append(records, record.Record{
			ID:     uint64(i),
			Raw:    fmt.Sprintf("row %d", i),
			Fields: map[string]string{"message": fmt.Sprintf("row %d", i)},
		})
	}
	m := newTestModelWithRecords(records)
	m.width = 100
	m.toggleSplit(SplitVertical)

	if m.activePane != 0 {
		t.Fatalf("expected initial activePane 0, got %d", m.activePane)
	}

	// Click in right pane (msg.X = 75 > splitX = 49, msg.Y = 8 in data rows)
	updated, _ := m.Update(tea.MouseMsg{
		X:      75,
		Y:      8,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	})
	m = updated.(Model)

	if m.activePane != 1 {
		t.Errorf("expected click in right pane to focus pane 1, got %d", m.activePane)
	}

	// Click in left pane (msg.X = 20 < splitX = 49)
	updated, _ = m.Update(tea.MouseMsg{
		X:      20,
		Y:      8,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	})
	m = updated.(Model)

	if m.activePane != 0 {
		t.Errorf("expected click in left pane to focus pane 0, got %d", m.activePane)
	}
}

func TestSplitLiveStreamingDualPanes(t *testing.T) {
	m := newTestModel()
	m.width = 120
	m.height = 30
	m.tableHeight = 20

	// Split vertical (creates Tab 2)
	m.toggleSplit(SplitVertical)
	if len(m.tabs) != 2 {
		t.Fatalf("expected 2 tabs, got %d", len(m.tabs))
	}

	// Filter Tab 1 for "ble"
	f1, _ := filter.New("ble")
	m.tabs[0].Filter = f1
	m.tabs[0].FilterRaw = "ble"
	m.tabs[0].Name = "BLE Logs"

	// Filter Tab 2 for "wifi"
	f2, _ := filter.New("wifi")
	m.tabs[1].Filter = f2
	m.tabs[1].FilterRaw = "wifi"
	m.tabs[1].Name = "WiFi Logs"

	m.syncModelToActiveTab()

	// Stream 6 messages:
	// - 3 matching ble
	// - 2 matching wifi
	// - 1 matching neither
	messages := []string{
		"ble: device connected",
		"wifi: scanning channels",
		"ble: received payload [0x01, 0x02]",
		"sensor: accelerometer idle",
		"wifi: joined network SSID_TEST",
		"ble: disconnect event",
	}

	for _, msg := range messages {
		updated, _ := m.Update(lineMsg(msg))
		m = updated.(Model)
	}

	// Check Tab 1: should have exactly 3 ble logs
	if len(m.tabs[0].Visible) != 3 {
		t.Errorf("expected Tab 1 to have 3 records, got %d", len(m.tabs[0].Visible))
	}
	for _, r := range m.tabs[0].Visible {
		if !strings.Contains(r.Raw, "ble") {
			t.Errorf("unexpected record in Tab 1: %s", r.Raw)
		}
	}

	// Check Tab 2: should have exactly 2 wifi logs
	if len(m.tabs[1].Visible) != 2 {
		t.Errorf("expected Tab 2 to have 2 records, got %d", len(m.tabs[1].Visible))
	}
	for _, r := range m.tabs[1].Visible {
		if !strings.Contains(r.Raw, "wifi") {
			t.Errorf("unexpected record in Tab 2: %s", r.Raw)
		}
	}

	// Verify both tabs maintained follow
	if !m.tabs[0].Follow {
		t.Errorf("expected Tab 1 Follow to remain true")
	}
	if !m.tabs[1].Follow {
		t.Errorf("expected Tab 2 Follow to remain true")
	}

	// Verify full View() renders both panes
	viewOutput := m.View()
	if !strings.Contains(viewOutput, "BLE Logs") {
		t.Errorf("expected View output to contain Tab 1 title 'BLE Logs'")
	}
	if !strings.Contains(viewOutput, "WiFi Logs") {
		t.Errorf("expected View output to contain Tab 2 title 'WiFi Logs'")
	}
	if !strings.Contains(viewOutput, "SPLIT [VERT]") {
		t.Errorf("expected View status bar to contain 'SPLIT [VERT]'")
	}
}

func TestHorizontalSplitScrollVisibleHeight(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.height = 30
	m.recalcLayout()
	// tableHeight = 30 - 7 = 23 (with 1 tab)

	// Populate buffer with 100 records
	for i := 1; i <= 100; i++ {
		m.buffer.Add(record.Record{
			ID:     uint64(i),
			Fields: map[string]string{"message": fmt.Sprintf("Line %03d", i)},
			Raw:    fmt.Sprintf("Line %03d", i),
		})
	}
	m.visible = m.buffer.All()
	m.tabs[0].Visible = m.visible

	// Toggle horizontal split
	m.toggleSplit(SplitHorizontal)
	if m.splitMode != SplitHorizontal {
		t.Fatalf("expected SplitHorizontal, got %v", m.splitMode)
	}

	topH := m.paneDataHeight(0)
	bottomH := m.paneDataHeight(1)

	if topH <= 0 || bottomH <= 0 {
		t.Fatalf("expected positive data heights, got top=%d, bottom=%d", topH, bottomH)
	}
	if topH >= m.tableHeight {
		t.Errorf("top pane height (%d) should be significantly smaller than single-view tableHeight (%d)", topH, m.tableHeight)
	}

	// 1. Top pane scrolling down: verify cursor stays in visible viewport at every step
	m.activePane = 0
	m.syncModelToActiveTab()
	m.follow = false
	m.scrollOffset = 0
	m.selectedRow = 0

	downKey := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}

	for step := 1; step <= 50; step++ {
		updated, _ := m.Update(downKey)
		m = updated.(Model)

		curH := m.activeDataHeight()
		if m.selectedRow < m.scrollOffset {
			t.Fatalf("step %d: selectedRow %d is above scrollOffset %d", step, m.selectedRow, m.scrollOffset)
		}
		if m.selectedRow >= m.scrollOffset+curH {
			t.Fatalf("step %d: cursor invisible! selectedRow %d exceeds visible height (offset %d + height %d = %d)",
				step, m.selectedRow, m.scrollOffset, curH, m.scrollOffset+curH)
		}
	}

	// 2. Switch focus to bottom pane: verify scrolling down in bottom pane also keeps cursor visible
	m.switchPaneFocus()
	if m.activePane != 1 {
		t.Fatalf("expected activePane 1, got %d", m.activePane)
	}
	m.follow = false
	m.scrollOffset = 0
	m.selectedRow = 0

	for step := 1; step <= 50; step++ {
		updated, _ := m.Update(downKey)
		m = updated.(Model)

		curH := m.activeDataHeight()
		if m.selectedRow < m.scrollOffset {
			t.Fatalf("bottom pane step %d: selectedRow %d is above scrollOffset %d", step, m.selectedRow, m.scrollOffset)
		}
		if m.selectedRow >= m.scrollOffset+curH {
			t.Fatalf("bottom pane step %d: cursor invisible! selectedRow %d exceeds visible height (offset %d + height %d = %d)",
				step, m.selectedRow, m.scrollOffset, curH, m.scrollOffset+curH)
		}
	}

	// 3. Scroll up in bottom pane: verify cursor stays visible
	upKey := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}
	for step := 1; step <= 30; step++ {
		updated, _ := m.Update(upKey)
		m = updated.(Model)

		curH := m.activeDataHeight()
		if m.selectedRow < m.scrollOffset {
			t.Fatalf("bottom pane scroll up step %d: selectedRow %d is above scrollOffset %d", step, m.selectedRow, m.scrollOffset)
		}
		if m.selectedRow >= m.scrollOffset+curH {
			t.Fatalf("bottom pane scroll up step %d: cursor invisible! selectedRow %d exceeds visible height (offset %d + height %d = %d)",
				step, m.selectedRow, m.scrollOffset, curH, m.scrollOffset+curH)
		}
	}
}

func TestHorizontalSplitViewShowsCursorWhileScrolling(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.height = 20
	m.recalcLayout()

	for i := 1; i <= 50; i++ {
		m.buffer.Add(record.Record{
			ID:     uint64(i),
			Fields: map[string]string{"message": fmt.Sprintf("Event #%02d", i)},
			Raw:    fmt.Sprintf("Event #%02d", i),
		})
	}
	m.visible = m.buffer.All()
	m.tabs[0].Visible = m.visible

	m.toggleSplit(SplitHorizontal)

	downKey := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}

	// Scroll down 25 times
	for step := 1; step <= 25; step++ {
		updated, _ := m.Update(downKey)
		m = updated.(Model)

		view := m.View()
		if !strings.Contains(view, "▶") {
			t.Fatalf("step %d: cursor ▶ missing from View() output during horizontal split scrolling (selectedRow=%d, scrollOffset=%d)",
				step, m.selectedRow, m.scrollOffset)
		}
	}
}

