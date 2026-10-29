package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/brenoniehues/oh-my-logs/internal/clipboard"
	"github.com/brenoniehues/oh-my-logs/internal/record"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func newTestSelectionModel(t *testing.T, bufferCap int) Model {
	t.Helper()
	m := newTestModel()
	m.width = 120
	m.height = 25
	m.tableHeight = 10
	if bufferCap > 0 {
		m.buffer = record.NewBuffer(bufferCap)
	}
	return m
}

func TestPreserveSelectionOnIngest_HighThroughput(t *testing.T) {
	m := newTestSelectionModel(t, 1000)

	// Ingest 10 initial records
	for i := 1; i <= 10; i++ {
		updated, _ := m.Update(lineMsg(fmt.Sprintf("initial log %d", i)))
		m = updated.(Model)
	}

	if len(m.visible) != 10 {
		t.Fatalf("expected 10 visible records, got %d", len(m.visible))
	}

	// Select row 3 ("initial log 4")
	selectedIdx := 3
	selectedRecordID := m.visible[selectedIdx].ID
	if selectedRecordID == 0 {
		t.Fatalf("expected non-zero record ID at index %d", selectedIdx)
	}

	// Navigate to row 3 using Up key from bottom
	// Alternatively, directly set selectedRow and clamp
	m.selectedRow = selectedIdx
	m.clampScroll()

	if m.follow {
		t.Fatalf("expected follow to be false once a row is selected")
	}

	// Simulate high log rate: ingest 100 new log lines in rapid succession
	for i := 1; i <= 100; i++ {
		updated, _ := m.Update(lineMsg(fmt.Sprintf("high rate stream burst %d", i)))
		m = updated.(Model)
	}

	// Verify selection was NOT lost
	if m.selectedRow != selectedIdx {
		t.Fatalf("expected selectedRow to stay %d, got %d", selectedIdx, m.selectedRow)
	}
	if m.follow {
		t.Fatalf("expected follow to remain false while row is selected")
	}
	if m.visible[m.selectedRow].ID != selectedRecordID {
		t.Fatalf("expected selected record ID to remain %d, got %d", selectedRecordID, m.visible[m.selectedRow].ID)
	}

	// Press 'b' to toggle bookmark / pin on the selected row
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	m = updated.(Model)

	// Verify the bookmarked record is the SELECTED record, not the newest incoming record
	if _, ok := m.bookmarks[selectedRecordID]; !ok {
		t.Fatalf("expected selected record ID %d to be bookmarked, bookmarks: %+v", selectedRecordID, m.bookmarks)
	}
}

func TestPreserveSelectionOnIngest_KeyboardNavigation(t *testing.T) {
	m := newTestSelectionModel(t, 1000)

	for i := 1; i <= 20; i++ {
		updated, _ := m.Update(lineMsg(fmt.Sprintf("log entry %d", i)))
		m = updated.(Model)
	}

	// Initially in follow mode
	if !m.follow {
		t.Fatalf("expected follow to be true initially")
	}

	// Press Up arrow ('k' or up key) to navigate into table
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = updated.(Model)

	if m.follow {
		t.Fatalf("expected follow to become false on Up arrow navigation")
	}
	if m.selectedRow < 0 {
		t.Fatalf("expected selectedRow >= 0 on Up arrow navigation, got %d", m.selectedRow)
	}

	targetRow := m.selectedRow
	targetID := m.visible[targetRow].ID

	// Ingest 5 new records
	for i := 1; i <= 5; i++ {
		updated, _ := m.Update(lineMsg(fmt.Sprintf("streaming log %d", i)))
		m = updated.(Model)
	}

	// Verify target row and ID are intact
	if m.selectedRow != targetRow {
		t.Fatalf("expected selectedRow %d, got %d", targetRow, m.selectedRow)
	}
	if m.visible[m.selectedRow].ID != targetID {
		t.Fatalf("expected record ID %d, got %d", targetID, m.visible[m.selectedRow].ID)
	}

	// Navigate down to the bottom visible record using Down arrow
	for i := 0; i < len(m.visible); i++ {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = updated.(Model)
	}

	// Even on the bottom line, if selectedRow >= 0, follow must remain FALSE!
	if m.selectedRow != len(m.visible)-1 {
		t.Fatalf("expected selectedRow at bottom line %d, got %d", len(m.visible)-1, m.selectedRow)
	}
	if m.follow {
		t.Fatalf("expected follow to stay false when a row is explicitly selected, even at bottom")
	}

	// Ingest more records while at bottom selected row
	lastRowID := m.visible[m.selectedRow].ID
	for i := 1; i <= 5; i++ {
		updated, _ = m.Update(lineMsg(fmt.Sprintf("appended log %d", i)))
		m = updated.(Model)
	}

	// Selection must stay anchored to that exact record ID
	if m.visible[m.selectedRow].ID != lastRowID {
		t.Fatalf("expected selection to stay on ID %d, got %d", lastRowID, m.visible[m.selectedRow].ID)
	}

	// Press 'G' (GoToBottom) to explicitly resume follow
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	m = updated.(Model)

	if !m.follow {
		t.Fatalf("expected follow to be true after GoToBottom ('G')")
	}
	if m.selectedRow != -1 {
		t.Fatalf("expected selectedRow to be cleared (-1) after GoToBottom ('G'), got %d", m.selectedRow)
	}
}

func TestPreserveSelectionOnIngest_RingBufferEviction(t *testing.T) {
	// Ring buffer with small capacity of 10 records
	m := newTestSelectionModel(t, 10)

	for i := 1; i <= 10; i++ {
		updated, _ := m.Update(lineMsg(fmt.Sprintf("line %02d", i)))
		m = updated.(Model)
	}

	// Select row 5 (record ID 6)
	m.selectedRow = 5
	m.clampScroll()
	targetID := m.visible[5].ID

	// Ingest 2 new records -> causes eviction of 2 oldest records
	for i := 11; i <= 12; i++ {
		updated, _ := m.Update(lineMsg(fmt.Sprintf("line %02d", i)))
		m = updated.(Model)
	}

	// Selection should have shifted from index 5 to 5 - 2 = 3
	if m.selectedRow != 3 {
		t.Fatalf("expected selectedRow to shift to 3 after 2 evictions, got %d", m.selectedRow)
	}
	if m.visible[m.selectedRow].ID != targetID {
		t.Fatalf("expected selected record ID to remain %d, got %d", targetID, m.visible[m.selectedRow].ID)
	}

	// Now ingest 10 new records -> completely evicts record targetID past the buffer top
	for i := 13; i <= 22; i++ {
		updated, _ := m.Update(lineMsg(fmt.Sprintf("line %02d", i)))
		m = updated.(Model)
	}

	// Once the record is evicted from the ring buffer, selection must cleanly clear to -1
	if m.selectedRow != -1 {
		t.Fatalf("expected selectedRow to clear to -1 after being evicted, got %d", m.selectedRow)
	}
}

func TestFindWordAt(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		col       int
		wantStart int
		wantEnd   int
		wantWord  string
	}{
		{
			name:      "empty text",
			text:      "",
			col:       0,
			wantStart: -1,
			wantEnd:   -1,
			wantWord:  "",
		},
		{
			name:      "single word middle",
			text:      "hello world",
			col:       2,
			wantStart: 0,
			wantEnd:   5,
			wantWord:  "hello",
		},
		{
			name:      "word with underscore and dash",
			text:      "status: wifi-nxp_drv connected",
			col:       12,
			wantStart: 8,
			wantEnd:   20,
			wantWord:  "wifi-nxp_drv",
		},
		{
			name:      "path or url",
			text:      "see http://example.com/api/test for details",
			col:       10,
			wantStart: 4,
			wantEnd:   31,
			wantWord:  "http://example.com/api/test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, e, w := findWordAt(tt.text, tt.col)
			if s != tt.wantStart || e != tt.wantEnd || w != tt.wantWord {
				t.Errorf("findWordAt(%q, %d) = (%d, %d, %q); want (%d, %d, %q)",
					tt.text, tt.col, s, e, w, tt.wantStart, tt.wantEnd, tt.wantWord)
			}
		})
	}
}

func TestCtrlA_SelectAll(t *testing.T) {
	m := newTestSelectionModel(t, 100)
	for i := 1; i <= 15; i++ {
		updated, _ := m.Update(lineMsg(fmt.Sprintf("log record %d", i)))
		m = updated.(Model)
	}

	if len(m.visible) != 15 {
		t.Fatalf("expected 15 visible records, got %d", len(m.visible))
	}

	// Press Ctrl+A
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlA})
	m = updated.(Model)

	if m.selectionStart != 0 || m.selectionEnd != 14 {
		t.Fatalf("expected selection range 0..14, got %d..%d", m.selectionStart, m.selectionEnd)
	}
	if m.selectedRow != 14 {
		t.Fatalf("expected selectedRow 14, got %d", m.selectedRow)
	}
	if m.follow {
		t.Fatalf("expected follow to be false after SelectAll")
	}

	// Press Ctrl+C while SelectAll active: should copy and NOT quit
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m = updated.(Model)
	if cmd != nil {
		t.Fatalf("expected cmd to be nil (not tea.Quit)")
	}
	if m.message == "" || !containsSub(m.message, "Copied 15 rows") {
		t.Fatalf("expected message mentioning copied 15 rows, got %q", m.message)
	}
}

func TestCtrlC_ContextualQuitOrCopy(t *testing.T) {
	m := newTestSelectionModel(t, 100)
	for i := 1; i <= 5; i++ {
		updated, _ := m.Update(lineMsg(fmt.Sprintf("entry %d", i)))
		m = updated.(Model)
	}

	// Case 1: No selection active -> Ctrl+C triggers Quit
	m.charSelStart = -1
	m.charSelEnd = -1
	m.selectionStart = -1
	m.selectionEnd = -1
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatalf("expected Ctrl+C to return tea.Quit when no selection is active")
	}

	// Case 2: Character selection active -> Ctrl+C copies substring, does NOT quit
	m.charSelStart = 1
	m.charSelEnd = 4
	m.selectedRow = 0
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m = updated.(Model)
	if cmd != nil {
		t.Fatalf("expected Ctrl+C to NOT quit when character selection is active")
	}
	if m.message == "" || !containsSub(m.message, "Copied") {
		t.Fatalf("expected message confirming copy, got %q", m.message)
	}

	// Case 3: Multi-row selection active -> Ctrl+C copies rows, does NOT quit
	m.charSelStart = -1
	m.charSelEnd = -1
	m.selectionStart = 0
	m.selectionEnd = 2
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m = updated.(Model)
	if cmd != nil {
		t.Fatalf("expected Ctrl+C to NOT quit when multi-row selection is active")
	}
	if m.message == "" || !containsSub(m.message, "Copied 3 rows") {
		t.Fatalf("expected message confirming 3 rows copied, got %q", m.message)
	}
}

func containsSub(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestMouseDragSelection_CharacterSelection(t *testing.T) {
	m := newTestModel()
	m.width = 100
	m.height = 30
	m.recalcLayout()

	updated, _ := m.Update(lineMsg("os_msg enable_logging wifi_nxp"))
	m = updated.(Model)

	tableStartY := m.tableDataStartY()
	// Press at row 0, col prefixWidth + 7 ('e' of enable_logging)
	pressX := prefixWidth + 7
	updated, _ = m.Update(tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
		X:      pressX,
		Y:      tableStartY,
	})
	m = updated.(Model)

	if m.selectedRow != 0 {
		t.Fatalf("expected selectedRow 0, got %d", m.selectedRow)
	}

	// Motion drag to col prefixWidth + 21 (end of enable_logging) on same row
	dragX := prefixWidth + 21
	updated, _ = m.Update(tea.MouseMsg{
		Action: tea.MouseActionMotion,
		Button: tea.MouseButtonLeft,
		X:      dragX,
		Y:      tableStartY,
	})
	m = updated.(Model)

	if m.charSelStart != 7 || m.charSelEnd != 21 {
		t.Fatalf("expected char selection 7..21, got %d..%d", m.charSelStart, m.charSelEnd)
	}

	// Release mouse button -> should auto-copy the selected substring
	updated, _ = m.Update(tea.MouseMsg{
		Action: tea.MouseActionRelease,
		Button: tea.MouseButtonLeft,
		X:      dragX,
		Y:      tableStartY,
	})
	m = updated.(Model)

	if !containsSub(m.message, "Copied 'enable_logging' to clipboard") {
		t.Fatalf("expected message to confirm copied 'enable_logging', got %q", m.message)
	}
}

func TestMouseDragSelection_MultiLineNormalRows(t *testing.T) {
	m := newTestModel()
	m.width = 120
	m.height = 30
	m.recalcLayout()

	lines := []string{
		"first line of log records",
		"second intermediate log line",
		"third concluding line here",
	}
	for _, l := range lines {
		updated, _ := m.Update(lineMsg(l))
		m = updated.(Model)
	}

	tableStartY := m.tableDataStartY()

	// 1. Mouse press on line 0 without modifier
	pressX := prefixWidth + 6
	updated, _ := m.Update(tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
		X:      pressX,
		Y:      tableStartY,
	})
	m = updated.(Model)

	// 2. Normal drag down to line 2 without modifier -> selects entire rows
	dragX := prefixWidth + 16
	updated, _ = m.Update(tea.MouseMsg{
		Action: tea.MouseActionMotion,
		Button: tea.MouseButtonLeft,
		X:      dragX,
		Y:      tableStartY + 2,
	})
	m = updated.(Model)

	if m.selectionStart != 0 || m.selectionEnd != 2 {
		t.Fatalf("expected selection range 0..2, got %d..%d", m.selectionStart, m.selectionEnd)
	}
	// Normal row drag must NOT set charSel bounds
	if m.charSelStart != -1 || m.charSelEnd != -1 {
		t.Fatalf("expected char bounds -1..-1 for normal row selection, got %d..%d", m.charSelStart, m.charSelEnd)
	}
	if !containsSub(m.message, "3 rows selected") {
		t.Fatalf("expected message to show 3 rows selected, got %q", m.message)
	}

	// 3. Mouse release keeps rows selected without auto-copying
	updated, _ = m.Update(tea.MouseMsg{
		Action: tea.MouseActionRelease,
		Button: tea.MouseButtonLeft,
		X:      dragX,
		Y:      tableStartY + 2,
	})
	m = updated.(Model)

	if m.selectionStart != 0 || m.selectionEnd != 2 {
		t.Fatalf("expected selection range to persist after release, got %d..%d", m.selectionStart, m.selectionEnd)
	}

	// 4. Pressing Ctrl+C copies the full 3 rows
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m = updated.(Model)

	clipText, err := clipboard.Read()
	if err != nil {
		t.Fatalf("clipboard read error: %v", err)
	}

	expected := "first line of log records\nsecond intermediate log line\nthird concluding line here"
	if clipText != expected {
		t.Fatalf("expected clipboard:\n%q\ngot:\n%q", expected, clipText)
	}
}

func TestMouseDragSelection_MultiLineRespectingRows(t *testing.T) {
	m := newTestModel()
	m.width = 120
	m.height = 30
	m.recalcLayout()

	lines := []string{
		"first line of log records",
		"second intermediate log line",
		"third concluding line here",
	}
	for _, l := range lines {
		updated, _ := m.Update(lineMsg(l))
		m = updated.(Model)
	}

	tableStartY := m.tableDataStartY()

	// 1. Mouse press on line 0 at col 6 with Alt: true (Option on Mac)
	pressX := prefixWidth + 6
	updated, _ := m.Update(tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
		Alt:    true,
		X:      pressX,
		Y:      tableStartY,
	})
	m = updated.(Model)

	// 2. Drag down to line 2 at col 16 with Alt: true -> rectangular block selection
	dragX := prefixWidth + 16
	updated, _ = m.Update(tea.MouseMsg{
		Action: tea.MouseActionMotion,
		Button: tea.MouseButtonLeft,
		Alt:    true,
		X:      dragX,
		Y:      tableStartY + 2,
	})
	m = updated.(Model)

	if m.selectionStart != 0 || m.selectionEnd != 2 {
		t.Fatalf("expected selection range 0..2, got %d..%d", m.selectionStart, m.selectionEnd)
	}
	if m.charSelStart != 6 || m.charSelEnd != 16 {
		t.Fatalf("expected char bounds 6..16, got %d..%d", m.charSelStart, m.charSelEnd)
	}

	// 3. Mouse release copies the rectangular block section
	updated, _ = m.Update(tea.MouseMsg{
		Action: tea.MouseActionRelease,
		Button: tea.MouseButtonLeft,
		Alt:    true,
		X:      dragX,
		Y:      tableStartY + 2,
	})
	m = updated.(Model)

	if !containsSub(m.message, "Copied 3 rows block") {
		t.Fatalf("expected message to show Copied 3 rows block, got %q", m.message)
	}

	clipText, err := clipboard.Read()
	if err != nil {
		t.Fatalf("clipboard read error: %v", err)
	}

	// Columns 6..16 on each of the 3 lines (a square block section!)
	expected := "line of lo\n intermedi\nconcluding"
	if clipText != expected {
		t.Fatalf("expected clipboard:\n%q\ngot:\n%q", expected, clipText)
	}
}

func TestMouseDragSelection_MultiLineUpward(t *testing.T) {
	m := newTestModel()
	m.width = 120
	m.height = 30
	m.recalcLayout()

	lines := []string{
		"alpha beginning line",
		"beta middle line",
		"gamma ending line",
	}
	for _, l := range lines {
		updated, _ := m.Update(lineMsg(l))
		m = updated.(Model)
	}

	tableStartY := m.tableDataStartY()

	// 1. Mouse press on line 2 at col 12 with Option/Alt ('e' of "ending line")
	pressX := prefixWidth + 12
	updated, _ := m.Update(tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
		Alt:    true,
		X:      pressX,
		Y:      tableStartY + 2,
	})
	m = updated.(Model)

	// 2. Drag UP to line 0 at col 6 with Option/Alt ('b' of "beginning line")
	dragX := prefixWidth + 6
	updated, _ = m.Update(tea.MouseMsg{
		Action: tea.MouseActionMotion,
		Button: tea.MouseButtonLeft,
		Alt:    true,
		X:      dragX,
		Y:      tableStartY,
	})
	m = updated.(Model)

	if m.selectionStart != 2 || m.selectionEnd != 0 {
		t.Fatalf("expected selection range 2..0, got %d..%d", m.selectionStart, m.selectionEnd)
	}

	// 3. Mouse release copies the rectangular block section
	updated, _ = m.Update(tea.MouseMsg{
		Action: tea.MouseActionRelease,
		Button: tea.MouseButtonLeft,
		Alt:    true,
		X:      dragX,
		Y:      tableStartY,
	})
	m = updated.(Model)

	clipText, err := clipboard.Read()
	if err != nil {
		t.Fatalf("clipboard read error: %v", err)
	}

	// Columns 6..12 on each of the 3 lines from line 0 to line 2 (square section!)
	expected := "beginn\niddle \nending"
	if clipText != expected {
		t.Fatalf("expected clipboard:\n%q\ngot:\n%q", expected, clipText)
	}
}

func TestMouseDragSelection_SquareSectionExample(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	m := newTestModel()
	m.width = 120
	m.height = 30
	m.recalcLayout()

	// Ingest 8 lines
	for i := 0; i < 8; i++ {
		l := fmt.Sprintf("[%d] 012345678901234567890123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789", i)
		updated, _ := m.Update(lineMsg(l))
		m = updated.(Model)
	}

	tableStartY := m.tableDataStartY()

	// Click at row 1, col 30 with Alt (Option on macOS)
	pressX := prefixWidth + 30
	updated, _ := m.Update(tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
		Alt:    true,
		X:      pressX,
		Y:      tableStartY + 1,
	})
	m = updated.(Model)

	// Drag to row 6, col 60 with Alt
	dragX := prefixWidth + 60
	updated, _ = m.Update(tea.MouseMsg{
		Action: tea.MouseActionMotion,
		Button: tea.MouseButtonLeft,
		Alt:    true,
		X:      dragX,
		Y:      tableStartY + 6,
	})
	m = updated.(Model)

	// Verify all rows from 1 to 6 have square bounds 30..60
	for r := 1; r <= 6; r++ {
		sCol, eCol, ok := m.rowCharSelection(r)
		if !ok || sCol != 30 || eCol != 60 {
			t.Fatalf("row %d expected square selection [30..60], got [%d..%d], ok=%v", r, sCol, eCol, ok)
		}
	}
	// Row 0 and row 7 must NOT have selection
	if _, _, ok := m.rowCharSelection(0); ok {
		t.Fatalf("row 0 should not be in selection")
	}
	if _, _, ok := m.rowCharSelection(7); ok {
		t.Fatalf("row 7 should not be in selection")
	}

	tableOut := m.viewTable()
	renderedLines := strings.Split(tableOut, "\n")
	for r := 1; r <= 6; r++ {
		// Verify every row in the square section has the selection background styling
		if !strings.Contains(renderedLines[r], "6789ABCDEFGHIJKLMNOPQRSTUVWXYZ") {
			t.Errorf("row %d expected highlighted block text", r)
		}
	}

	// Release mouse -> auto-copies the square section
	updated, _ = m.Update(tea.MouseMsg{
		Action: tea.MouseActionRelease,
		Button: tea.MouseButtonLeft,
		Alt:    true,
		X:      dragX,
		Y:      tableStartY + 6,
	})
	m = updated.(Model)

	clipText, err := clipboard.Read()
	if err != nil {
		t.Fatalf("clipboard read error: %v", err)
	}

	lines := strings.Split(clipText, "\n")
	if len(lines) != 6 {
		t.Fatalf("expected 6 lines in square section, got %d", len(lines))
	}
	for i, l := range lines {
		if len([]rune(l)) != 30 {
			t.Fatalf("line %d expected 30 chars (col 30..60), got %d (%q)", i, len([]rune(l)), l)
		}
	}
}

func TestSpaceKey_ToggleFollow_PauseAndResume(t *testing.T) {
	m := newTestSelectionModel(t, 100)

	// Ingest 20 records so we have enough lines
	for i := 1; i <= 20; i++ {
		updated, _ := m.Update(lineMsg(fmt.Sprintf("log line %d", i)))
		m = updated.(Model)
	}

	// Initially, follow should be true and scrolled to bottom
	if !m.follow {
		t.Fatalf("expected follow=true initially")
	}

	// 1. Press Space -> pauses follow
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = updated.(Model)

	if m.follow {
		t.Errorf("expected follow=false after pressing Space once")
	}
	if curTab := m.currentTab(); curTab != nil && curTab.Follow {
		t.Errorf("expected curTab.Follow=false after pressing Space")
	}

	// Ingest new lines while paused -> viewport should NOT auto-scroll
	oldOffset := m.scrollOffset
	for i := 21; i <= 25; i++ {
		updated, _ = m.Update(lineMsg(fmt.Sprintf("new log line %d", i)))
		m = updated.(Model)
	}
	if m.scrollOffset != oldOffset {
		t.Errorf("expected scrollOffset=%d while paused, got %d", oldOffset, m.scrollOffset)
	}

	// 2. Press Space again -> resumes follow and scrolls to bottom
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = updated.(Model)

	if !m.follow {
		t.Errorf("expected follow=true after pressing Space a second time")
	}
	if curTab := m.currentTab(); curTab != nil && !curTab.Follow {
		t.Errorf("expected curTab.Follow=true after resuming")
	}

	// Verify scroll offset moved to bottom
	expectedOffset := len(m.visible) - m.tableHeight
	if m.scrollOffset != expectedOffset {
		t.Errorf("expected scrollOffset to be at bottom (%d), got %d", expectedOffset, m.scrollOffset)
	}
}

func TestSpaceKey_ResumeFollow_ClearsRowSelection(t *testing.T) {
	m := newTestSelectionModel(t, 100)

	for i := 1; i <= 20; i++ {
		updated, _ := m.Update(lineMsg(fmt.Sprintf("log line %d", i)))
		m = updated.(Model)
	}

	// Select row 5 (which pauses follow)
	m.selectedRow = 5
	m.selectionStart = 5
	m.selectionEnd = 5
	m.follow = false
	if curTab := m.currentTab(); curTab != nil {
		curTab.Follow = false
		curTab.SelectedRow = 5
	}

	// Press Space to resume follow
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = updated.(Model)

	if !m.follow {
		t.Fatalf("expected follow=true after Space")
	}
	if m.selectedRow != -1 {
		t.Errorf("expected selectedRow to be cleared to -1 upon follow resume, got %d", m.selectedRow)
	}
	if m.selectionStart != -1 || m.selectionEnd != -1 {
		t.Errorf("expected range selection to be cleared, got [%d..%d]", m.selectionStart, m.selectionEnd)
	}

	// Ingest new line -> follow must remain active
	updated, _ = m.Update(lineMsg("line after space"))
	m = updated.(Model)

	if !m.follow {
		t.Errorf("expected follow to remain true when new logs arrive after pressing Space")
	}
}

func TestSpaceKey_EmptyBuffer_TogglesFollow(t *testing.T) {
	m := newTestSelectionModel(t, 100)

	if m.buffer.Len() != 0 {
		t.Fatalf("expected buffer to be empty")
	}
	if !m.follow {
		t.Fatalf("expected follow=true initially")
	}

	// Press Space on empty buffer -> should pause follow without opening mini-game
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = updated.(Model)

	if m.mode == modeGame {
		t.Fatalf("Space on empty buffer should NOT open mini-game, got modeGame")
	}
	if m.follow {
		t.Errorf("expected follow=false after Space on empty buffer")
	}

	// Press Space again -> should resume follow
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = updated.(Model)

	if !m.follow {
		t.Errorf("expected follow=true after second Space on empty buffer")
	}
}

