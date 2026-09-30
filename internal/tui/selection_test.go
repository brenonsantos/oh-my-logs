package tui

import (
	"fmt"
	"testing"

	"github.com/brenoniehues/oh-my-logs/internal/record"
	tea "github.com/charmbracelet/bubbletea"
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
