package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/brenoniehues/oh-my-logs/internal/clipboard"
	"github.com/brenoniehues/oh-my-logs/internal/record"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func TestApplyRowCursorWidthPreservation(t *testing.T) {
	row := "2026-09-10 12:00:00 [INFO] test: System initialized successfully"
	w := lipgloss.Width(row)

	// 1. Cursor at column 0
	withCursor0 := applyRowCursor(row, 0, -1, -1)
	if lipgloss.Width(withCursor0) != w {
		t.Errorf("cursor at 0: expected width %d, got %d", w, lipgloss.Width(withCursor0))
	}

	// 2. Cursor in middle (col 15)
	withCursor15 := applyRowCursor(row, 15, -1, -1)
	if lipgloss.Width(withCursor15) != w {
		t.Errorf("cursor at 15: expected width %d, got %d", w, lipgloss.Width(withCursor15))
	}

	// 3. Character selection [10:25]
	withSel := applyRowCursor(row, 25, 10, 25)
	if lipgloss.Width(withSel) != w {
		t.Errorf("selection [10:25]: expected width %d, got %d", w, lipgloss.Width(withSel))
	}

	// 4. Character selection reverse [25:10]
	withSelRev := applyRowCursor(row, 10, 25, 10)
	if lipgloss.Width(withSelRev) != w {
		t.Errorf("reverse selection: expected width %d, got %d", w, lipgloss.Width(withSelRev))
	}
}

func TestHorizontalPanningBraces(t *testing.T) {
	records := []record.Record{
		{ID: 1, Raw: "Line 1: A very long log message that goes well beyond the edge of a standard screen", Timestamp: time.Now()},
	}
	m := newTestModelWithRecords(records)
	m.width = 60
	m.height = 20

	if m.scrollX != 0 {
		t.Fatalf("expected initial scrollX 0, got %d", m.scrollX)
	}

	// Press '}' -> pan right by 8
	res, _ := m.handleNormalKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'}'}})
	m = res.(Model)
	if m.scrollX != 8 {
		t.Errorf("expected scrollX 8 after '}', got %d", m.scrollX)
	}

	// Press '}' again -> pan right to 16
	res, _ = m.handleNormalKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'}'}})
	m = res.(Model)
	if m.scrollX != 16 {
		t.Errorf("expected scrollX 16 after '}', got %d", m.scrollX)
	}

	// Press '{' -> pan left by 8 to 8
	res, _ = m.handleNormalKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'{'}})
	m = res.(Model)
	if m.scrollX != 8 {
		t.Errorf("expected scrollX 8 after '{', got %d", m.scrollX)
	}

	// Press '{' twice -> clamp at 0
	res, _ = m.handleNormalKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'{'}})
	m = res.(Model)
	res, _ = m.handleNormalKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'{'}})
	m = res.(Model)
	if m.scrollX != 0 {
		t.Errorf("expected scrollX clamped to 0, got %d", m.scrollX)
	}
}

func TestCharacterCursorNavigationAndAutoPan(t *testing.T) {
	records := []record.Record{
		{ID: 1, Raw: "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ", Timestamp: time.Now()},
	}
	m := newTestModelWithRecords(records)
	m.width = 40
	m.height = 20

	// Navigate right with 'right' arrow key
	res, _ := m.handleNormalKey(tea.KeyMsg{Type: tea.KeyRight})
	m = res.(Model)
	if m.selectedRow != 0 {
		t.Errorf("expected row 0 selected on right arrow, got %d", m.selectedRow)
	}
	if m.cursorCol < 0 {
		t.Errorf("expected cursorCol >= 0, got %d", m.cursorCol)
	}

	// Move cursor 50 characters to the right
	for i := 0; i < 50; i++ {
		res, _ = m.handleNormalKey(tea.KeyMsg{Type: tea.KeyRight})
		m = res.(Model)
	}
	if m.cursorCol != 50 {
		t.Errorf("expected cursorCol 50, got %d", m.cursorCol)
	}
	// Viewport must have auto-panned horizontally to keep cursor visible
	if m.scrollX <= 0 {
		t.Errorf("expected scrollX to have auto-panned > 0, got %d", m.scrollX)
	}
	availW := m.tableWidth() - 3
	if m.cursorCol < m.scrollX || m.cursorCol >= m.scrollX+availW {
		t.Errorf("cursor %d should be visible within [%d, %d)", m.cursorCol, m.scrollX, m.scrollX+availW)
	}

	// Move cursor back left
	for i := 0; i < 40; i++ {
		res, _ = m.handleNormalKey(tea.KeyMsg{Type: tea.KeyLeft})
		m = res.(Model)
	}
	if m.cursorCol != 10 {
		t.Errorf("expected cursorCol 10, got %d", m.cursorCol)
	}
	// Viewport must follow left
	if m.scrollX > m.cursorCol {
		t.Errorf("scrollX %d should be <= cursorCol %d", m.scrollX, m.cursorCol)
	}

	// Test 'l' and 'h' as cursor right and left
	res, _ = m.handleNormalKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	m = res.(Model)
	if m.cursorCol != 11 {
		t.Errorf("expected cursorCol 11 after 'l', got %d", m.cursorCol)
	}

	res, _ = m.handleNormalKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	m = res.(Model)
	if m.cursorCol != 10 {
		t.Errorf("expected cursorCol 10 after 'h', got %d", m.cursorCol)
	}
}

func TestCharacterSelectionAndClipboardCopy(t *testing.T) {
	records := []record.Record{
		{ID: 1, Raw: "1234567890abcdefghijklmnopqrstuvwxyz", Timestamp: time.Now(), Fields: map[string]string{
			"message": "1234567890abcdefghijklmnopqrstuvwxyz",
		}},
	}
	m := newTestModelWithRecords(records)
	m.displayFormat = FormatRaw
	m.tabs[0].DisplayFormat = FormatRaw
	m.width = 80
	m.height = 20

	// Select row 0 and position cursor at 5
	m.selectedRow = 0
	m.cursorCol = 5

	m.charSelStart = 5
	m.charSelEnd = 10
	m.cursorCol = 10

	if m.charSelStart != 5 || m.charSelEnd != 10 {
		t.Fatalf("expected charSel [5:10], got [%d:%d]", m.charSelStart, m.charSelEnd)
	}

	// Press 'y' to copy character substring
	res, _ := m.handleNormalKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = res.(Model)

	clip, err := clipboard.Read()
	if err != nil {
		t.Fatalf("failed reading clipboard: %v", err)
	}
	t.Logf("copied substring: %q", clip)
	if len(clip) != 5 {
		t.Errorf("expected 5 characters copied, got %d (%q)", len(clip), clip)
	}

	// Press Esc to cancel character selection
	res, _ = m.handleNormalKey(tea.KeyMsg{Type: tea.KeyEscape})
	m = res.(Model)
	if m.charSelStart != -1 || m.charSelEnd != -1 {
		t.Errorf("expected char selection cleared on Esc, got [%d:%d]", m.charSelStart, m.charSelEnd)
	}
	// Row selection should still be preserved after first Esc
	if m.selectedRow != 0 {
		t.Errorf("expected selectedRow 0 preserved on first Esc, got %d", m.selectedRow)
	}

	// Second Esc unselects row
	res, _ = m.handleNormalKey(tea.KeyMsg{Type: tea.KeyEscape})
	m = res.(Model)
	if m.selectedRow != -1 {
		t.Errorf("expected selectedRow -1 on second Esc, got %d", m.selectedRow)
	}
}

func TestHorizontalMouseWheelPanning(t *testing.T) {
	records := []record.Record{
		{ID: 1, Raw: "Testing mouse wheel horizontal pan across a very long log line that exceeds the width", Timestamp: time.Now()},
	}
	m := newTestModelWithRecords(records)
	m.width = 60
	m.height = 20

	// Tilt wheel right
	res, _ := m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelRight})
	m = res.(Model)
	if m.scrollX != 6 {
		t.Errorf("expected scrollX 6 after WheelRight, got %d", m.scrollX)
	}

	// Tilt wheel right again
	res, _ = m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelRight})
	m = res.(Model)
	if m.scrollX != 12 {
		t.Errorf("expected scrollX 12 after WheelRight, got %d", m.scrollX)
	}

	// Tilt wheel left
	res, _ = m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelLeft})
	m = res.(Model)
	if m.scrollX != 6 {
		t.Errorf("expected scrollX 6 after WheelLeft, got %d", m.scrollX)
	}
}

func TestSplitPaneIndependentHorizontalScroll(t *testing.T) {
	records := []record.Record{
		{ID: 1, Raw: "Dual pane horizontal scrolling isolation test across a very long log line that exceeds pane width", Timestamp: time.Now()},
	}
	m := newTestModelWithRecords(records)
	m.width = 100
	m.height = 30
	m.handleCreateNewTab()
	m.toggleSplit(SplitVertical)

	// In pane 0 (Tab 0), press '}' twice -> scrollX = 16
	res, _ := m.handleNormalKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'}'}})
	m = res.(Model)
	res, _ = m.handleNormalKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'}'}})
	m = res.(Model)
	if m.scrollX != 16 {
		t.Fatalf("expected pane 0 scrollX 16, got %d", m.scrollX)
	}

	// Switch to pane 1 ('w')
	res, _ = m.handleNormalKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}})
	m = res.(Model)
	if m.scrollX != 0 {
		t.Errorf("expected initial pane 1 scrollX 0, got %d", m.scrollX)
	}

	// Pan pane 1 right by 24 ('}' x 3)
	for i := 0; i < 3; i++ {
		res, _ = m.handleNormalKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'}'}})
		m = res.(Model)
	}
	if m.scrollX != 24 {
		t.Errorf("expected pane 1 scrollX 24, got %d", m.scrollX)
	}

	// Switch back to pane 0 ('w') -> scrollX should restore to 16
	res, _ = m.handleNormalKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}})
	m = res.(Model)
	if m.scrollX != 16 {
		t.Errorf("expected pane 0 scrollX restored to 16, got %d", m.scrollX)
	}

	// Switch to pane 1 ('w') -> scrollX should restore to 24
	res, _ = m.handleNormalKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}})
	m = res.(Model)
	if m.scrollX != 24 {
		t.Errorf("expected pane 1 scrollX restored to 24, got %d", m.scrollX)
	}
}

func TestAnsiSafeRowSlicing(t *testing.T) {
	row := theme.Header.Render("TIMESTAMP     ") + "  " + theme.Accent.Render("INFO ") + "  " + theme.Muted.Render("network.server: connected from 192.168.1.100")
	totalW := lipgloss.Width(row)

	// Slicing offset 0 to 40
	cut0 := ansiCut(row, 0, 40)
	if lipgloss.Width(cut0) != 40 {
		t.Errorf("expected width 40, got %d", lipgloss.Width(cut0))
	}

	// Slicing offset 15 to 40
	cut15 := ansiCut(row, 15, 40)
	if lipgloss.Width(cut15) != 40 {
		t.Errorf("expected width 40, got %d", lipgloss.Width(cut15))
	}

	// ANSI codes should not be broken
	stripped := ansi.Strip(cut15)
	if !strings.Contains(stripped, "INFO") && !strings.Contains(stripped, "network") {
		t.Errorf("expected text content in slice, got %q", stripped)
	}

	_ = totalW
}

func TestShiftMouseWheelHorizontalScroll(t *testing.T) {
	records := []record.Record{
		{ID: 1, Raw: "Testing shift+wheel horizontal panning across a very long log line that exceeds width", Timestamp: time.Now()},
	}
	m := newTestModelWithRecords(records)
	m.width = 60
	m.height = 20

	// Shift + WheelDown -> pan right by 6
	res, _ := m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelDown, Shift: true})
	m = res.(Model)
	if m.scrollX != 6 {
		t.Errorf("expected scrollX 6 after Shift+WheelDown, got %d", m.scrollX)
	}

	// Shift + WheelDown again -> pan right to 12
	res, _ = m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelDown, Shift: true})
	m = res.(Model)
	if m.scrollX != 12 {
		t.Errorf("expected scrollX 12 after Shift+WheelDown, got %d", m.scrollX)
	}

	// Shift + WheelUp -> pan left to 6
	res, _ = m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelUp, Shift: true})
	m = res.(Model)
	if m.scrollX != 6 {
		t.Errorf("expected scrollX 6 after Shift+WheelUp, got %d", m.scrollX)
	}
}

func TestInteractiveVerticalScrollbarClickAndDrag(t *testing.T) {
	var records []record.Record
	for i := 0; i < 100; i++ {
		records = append(records, record.Record{ID: uint64(i + 1), Raw: "Record log line", Timestamp: time.Now()})
	}
	m := newTestModelWithRecords(records)
	m.width = 80
	m.height = 25
	m.tableHeight = 20
	m.follow = true

	renderedTable := m.viewTable()
	lines := strings.Split(renderedTable, "\n")
	if len(lines) != m.tableHeight {
		t.Fatalf("expected %d table lines, got %d", m.tableHeight, len(lines))
	}
	// Rightmost character of first line should contain track or thumb
	firstLine := lines[0]
	if lipgloss.Width(firstLine) != m.width {
		t.Errorf("expected row width %d, got %d", m.width, lipgloss.Width(firstLine))
	}

	// Click vertical scrollbar on row 10
	tableStartY := 4
	clickY := tableStartY + 10
	clickX := m.width - 1
	res, _ := m.Update(tea.MouseMsg{
		X:      clickX,
		Y:      clickY,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	})
	m = res.(Model)
	t.Logf("after click: scrollOffset=%d follow=%v tableWidth=%d tableHeight=%d", m.scrollOffset, m.follow, m.tableWidth(), m.tableHeight)
	if m.follow {
		t.Errorf("expected follow false after vertical scrollbar click")
	}
	if m.scrollOffset == 0 {
		t.Errorf("expected scrollOffset > 0 after clicking middle of vertical scrollbar, got %d", m.scrollOffset)
	}

	// Drag vertical scrollbar to row 18 (near bottom)
	dragY := tableStartY + 18
	res, _ = m.Update(tea.MouseMsg{
		X:      clickX,
		Y:      dragY,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionMotion,
	})
	m = res.(Model)
	t.Logf("after drag: scrollOffset=%d", m.scrollOffset)
	if m.scrollOffset < 50 {
		t.Errorf("expected scrollOffset >= 50 near bottom of scrollbar, got %d", m.scrollOffset)
	}
}

func TestInteractiveHorizontalScrollbarClick(t *testing.T) {
	track := renderHScrollTrack(20, 200, 60)
	if lipgloss.Width(track) != 60 {
		t.Errorf("expected horizontal track width 60, got %d", lipgloss.Width(track))
	}
	if !strings.Contains(track, "▀") {
		t.Errorf("expected thumb marker ▀ in track, got %s", track)
	}
	if strings.Contains(track, "◀") || strings.Contains(track, "▶") {
		t.Errorf("expected no arrow markers in track, got %s", track)
	}

	longMsg := "Testing horizontal scrollbar click across a very long line that exceeds the standard screen width easily"
	records := []record.Record{
		{ID: 1, Raw: longMsg, Timestamp: time.Now()},
	}
	m := newTestModelWithRecords(records)
	m.width = 60
	m.height = 25

	// Click on horizontal scrollbar divider row (m.height - 3) at X = 40
	res, _ := m.Update(tea.MouseMsg{
		X:      40,
		Y:      m.height - 3,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	})
	m = res.(Model)
	if m.scrollX <= 0 {
		t.Errorf("expected scrollX > 0 after clicking horizontal scrollbar, got %d", m.scrollX)
	}
}

func TestNoHorizontalScrollbarWhenNotNeeded(t *testing.T) {
	// Short line that fits well inside width 100
	records := []record.Record{
		{ID: 1, Raw: "Short message", Timestamp: time.Now()},
	}
	m := newTestModelWithRecords(records)
	m.width = 100
	m.height = 20

	maxW := m.maxContentWidth()
	if maxW > m.tableWidth() {
		t.Errorf("expected maxContentWidth %d <= tableWidth %d", maxW, m.tableWidth())
	}

	divider := m.viewHorizontalScrollbarDivider()
	if strings.Contains(divider, "▀") {
		t.Errorf("scrollbar thumb should NOT be rendered when all content fits on screen, got %s", divider)
	}

	// Pressing '}' should not scroll past maxScroll (0)
	res, _ := m.handleNormalKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'}'}})
	m = res.(Model)
	if m.scrollX != 0 {
		t.Errorf("expected scrollX to remain 0 when content fits, got %d", m.scrollX)
	}
}

func TestBoundedHorizontalScrollUntilLastCharacter(t *testing.T) {
	// Line length: 80 chars. Table width: 50.
	longRaw := "12345678901234567890123456789012345678901234567890123456789012345678901234567890"
	records := []record.Record{
		{ID: 1, Raw: longRaw, Timestamp: time.Now()},
	}
	m := newTestModelWithRecords(records)
	m.width = 50
	m.height = 20

	maxW := m.maxContentWidth()
	if maxW <= m.tableWidth() {
		t.Fatalf("expected maxContentWidth %d > tableWidth %d", maxW, m.tableWidth())
	}
	maxScroll := maxW - m.tableWidth()

	// Pan right many times (100 times)
	for i := 0; i < 100; i++ {
		res, _ := m.handleNormalKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'}'}})
		m = res.(Model)
	}

	if m.scrollX > maxScroll {
		t.Errorf("scrollX %d exceeded maxScroll %d (infinite scroll bug)", m.scrollX, maxScroll)
	}
	if m.scrollX != maxScroll {
		t.Errorf("expected scrollX to reach exact maxScroll %d, got %d", maxScroll, m.scrollX)
	}
}

func TestCursorColBoundedToRowLength(t *testing.T) {
	records := []record.Record{
		{ID: 1, Raw: "short row", Timestamp: time.Now()},
	}
	m := newTestModelWithRecords(records)
	m.width = 60
	m.height = 20
	m.selectedRow = 0

	plainText := m.selectedRowPlainText(0)
	rowLen := len([]rune(plainText))
	if rowLen == 0 {
		t.Fatalf("expected non-empty row plain text")
	}

	// Move cursor right 100 times
	for i := 0; i < 100; i++ {
		res, _ := m.handleNormalKey(tea.KeyMsg{Type: tea.KeyRight})
		m = res.(Model)
	}

	if m.cursorCol >= rowLen {
		t.Errorf("cursorCol %d exceeded rowLen-1 %d", m.cursorCol, rowLen-1)
	}
	if m.cursorCol != rowLen-1 {
		t.Errorf("expected cursorCol to clamp at last character %d, got %d", rowLen-1, m.cursorCol)
	}
}

