package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/brenoniehues/oh-my-logs/internal/parser"
	"github.com/brenoniehues/oh-my-logs/internal/record"
	tea "github.com/charmbracelet/bubbletea"
)

func TestBookmarkToggle(t *testing.T) {
	m := newTestModel()

	// Feed 5 log lines
	for i := 1; i <= 5; i++ {
		updated, _ := m.Update(lineMsg(fmt.Sprintf("log line %d", i)))
		m = updated.(Model)
	}

	if len(m.visible) != 5 {
		t.Fatalf("expected 5 visible records, got %d", len(m.visible))
	}

	// Initial bookmarks set should be empty
	if len(m.bookmarks) != 0 {
		t.Fatalf("expected 0 bookmarks initially, got %d", len(m.bookmarks))
	}

	// Select row index 1 ("log line 2")
	m.selectedRow = 1
	recID := m.visible[1].ID

	// Toggle bookmark with 'b'
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	m = updated.(Model)

	if len(m.bookmarks) != 1 {
		t.Fatalf("expected 1 bookmark, got %d", len(m.bookmarks))
	}
	if _, ok := m.bookmarks[recID]; !ok {
		t.Fatalf("expected record ID %d to be bookmarked", recID)
	}
	if !strings.Contains(m.message, "Pinned row 2") {
		t.Errorf("expected message to mention Pinned row 2, got %q", m.message)
	}

	// Untoggle bookmark with 'm'
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	m = updated.(Model)

	if len(m.bookmarks) != 0 {
		t.Fatalf("expected 0 bookmarks after unpinning, got %d", len(m.bookmarks))
	}
	if !strings.Contains(m.message, "Unpinned row 2") {
		t.Errorf("expected message to mention Unpinned row 2, got %q", m.message)
	}
}

func TestBookmarkNavigation(t *testing.T) {
	m := newTestModel()

	// Feed 6 lines
	for i := 1; i <= 6; i++ {
		updated, _ := m.Update(lineMsg(fmt.Sprintf("log line %d", i)))
		m = updated.(Model)
	}

	// Bookmark row 1 and row 4 (indices 1 and 4)
	m.selectedRow = 1
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	m = updated.(Model)

	m.selectedRow = 4
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	m = updated.(Model)

	if len(m.bookmarks) != 2 {
		t.Fatalf("expected 2 bookmarks, got %d", len(m.bookmarks))
	}

	// Start at row 0, navigate next with ']'
	m.selectedRow = 0
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}})
	m = updated.(Model)

	if m.selectedRow != 1 {
		t.Errorf("expected selectedRow to jump to 1 on ']', got %d", m.selectedRow)
	}

	// Jump next again -> row 4
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}})
	m = updated.(Model)

	if m.selectedRow != 4 {
		t.Errorf("expected selectedRow to jump to 4 on ']', got %d", m.selectedRow)
	}

	// Jump next again -> wraps around to row 1
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}})
	m = updated.(Model)

	if m.selectedRow != 1 {
		t.Errorf("expected selectedRow to wrap around to 1 on ']', got %d", m.selectedRow)
	}

	// Navigate prev with '[' -> wraps backwards to row 4
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'['}})
	m = updated.(Model)

	if m.selectedRow != 4 {
		t.Errorf("expected selectedRow to jump back to 4 on '[', got %d", m.selectedRow)
	}

	// Navigate prev again -> row 1
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'['}})
	m = updated.(Model)

	if m.selectedRow != 1 {
		t.Errorf("expected selectedRow to jump back to 1 on '[', got %d", m.selectedRow)
	}
}

func TestBookmarksOnlyFilterToggle(t *testing.T) {
	m := newTestModel()

	// Feed 5 lines
	for i := 1; i <= 5; i++ {
		updated, _ := m.Update(lineMsg(fmt.Sprintf("log line %d", i)))
		m = updated.(Model)
	}

	// Bookmark rows 0 and 2
	m.selectedRow = 0
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	m = updated.(Model)

	m.selectedRow = 2
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	m = updated.(Model)

	// Press 'B' to toggle bookmarked-only view
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'B'}})
	m = updated.(Model)

	if !m.bookmarkedOnly {
		t.Fatalf("expected bookmarkedOnly to be true after 'B'")
	}
	if len(m.visible) != 2 {
		t.Fatalf("expected 2 visible rows in bookmarked-only mode, got %d", len(m.visible))
	}
	if m.visible[0].Raw != "log line 1" || m.visible[1].Raw != "log line 3" {
		t.Errorf("unexpected rows in bookmarked-only view: %+v", m.visible)
	}

	// New incoming log while bookmarked-only is active should NOT be added to visible
	updated, _ = m.Update(lineMsg("new unpinned line 6"))
	m = updated.(Model)

	if len(m.visible) != 2 {
		t.Errorf("expected visible count to stay 2 for unpinned incoming line, got %d", len(m.visible))
	}

	// Toggle 'B' again -> returns to all rows
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'B'}})
	m = updated.(Model)

	if m.bookmarkedOnly {
		t.Fatalf("expected bookmarkedOnly to be false after second 'B'")
	}
	if len(m.visible) != 6 {
		t.Fatalf("expected all 6 rows visible after disabling bookmarked-only, got %d", len(m.visible))
	}
}

func TestBookmarksVisualRenderingAndClear(t *testing.T) {
	m := newTestModel()

	for i := 1; i <= 3; i++ {
		updated, _ := m.Update(lineMsg(fmt.Sprintf("log line %d", i)))
		m = updated.(Model)
	}

	// Bookmark row 1
	m.selectedRow = 1
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	m = updated.(Model)

	// Verify table view renders star '★'
	tableView := m.viewTable()
	if !strings.Contains(tableView, "★") {
		t.Errorf("expected viewTable to contain star glyph '★'")
	}

	// Verify status bar shows pinned count
	statusBar := m.viewStatusBar()
	if !strings.Contains(statusBar, "★ 1 pinned") {
		t.Errorf("expected statusBar to show '★ 1 pinned', got: %s", statusBar)
	}

	// Verify key bar shows pin hint
	keyBar := m.viewKeyBar()
	if !strings.Contains(keyBar, "pin") {
		t.Errorf("expected keyBar to include 'pin', got: %s", keyBar)
	}

	// Press 'c' to clear buffer -> bookmarks should be cleared
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m = updated.(Model)

	if len(m.bookmarks) != 0 {
		t.Fatalf("expected bookmarks to be empty after clear, got %d", len(m.bookmarks))
	}
	if len(m.visible) != 0 {
		t.Fatalf("expected visible to be empty after clear, got %d", len(m.visible))
	}
}

func TestBookmarksMultiSelection(t *testing.T) {
	m := newTestModel()

	for i := 1; i <= 10; i++ {
		updated, _ := m.Update(lineMsg(fmt.Sprintf("log line %d", i)))
		m = updated.(Model)
	}

	// Select rows 2 to 6 (5 rows)
	m.selectionStart = 2
	m.selectionEnd = 6

	// Press 'b' to pin all selected rows
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	m = updated.(Model)

	if len(m.bookmarks) != 5 {
		t.Fatalf("expected 5 bookmarks, got %d", len(m.bookmarks))
	}

	for i := 2; i <= 6; i++ {
		recID := m.visible[i].ID
		if _, ok := m.bookmarks[recID]; !ok {
			t.Errorf("expected visible row %d (ID %d) to be bookmarked", i, recID)
		}
	}

	if _, ok := m.bookmarks[m.visible[1].ID]; ok {
		t.Errorf("row 1 should not be bookmarked")
	}
	if _, ok := m.bookmarks[m.visible[7].ID]; ok {
		t.Errorf("row 7 should not be bookmarked")
	}

	if !strings.Contains(m.message, "Pinned 5 rows (3-7)") {
		t.Errorf("expected message to report 5 rows pinned, got: %s", m.message)
	}

	// Press 'b' again with same selection -> unpins all
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	m = updated.(Model)

	if len(m.bookmarks) != 0 {
		t.Fatalf("expected 0 bookmarks after unpinning selection, got %d", len(m.bookmarks))
	}
	if !strings.Contains(m.message, "Unpinned 5 rows (3-7)") {
		t.Errorf("expected message to report 5 rows unpinned, got: %s", m.message)
	}
}

func TestBookmarksProfileSwitchTransformPreservesID(t *testing.T) {
	m := newTestModel()

	for i := 1; i <= 15; i++ {
		updated, _ := m.Update(lineMsg(fmt.Sprintf("log line %d", i)))
		m = updated.(Model)
	}

	// Simulate profile switch re-parse
	m.buffer.Transform(func(old record.Record) record.Record {
		newRec, _ := m.parser.Parse(old.Raw)
		newRec.ID = old.ID
		return newRec
	})
	m.rebuildAllTabs()

	// Verify all records have distinct non-zero IDs
	seen := make(map[uint64]bool)
	for i, r := range m.visible {
		if r.ID == 0 {
			t.Fatalf("row %d has zero ID after transform", i)
		}
		if seen[r.ID] {
			t.Fatalf("row %d has duplicate ID %d", i, r.ID)
		}
		seen[r.ID] = true
	}

	// Bookmark single row 4
	m.selectedRow = 4
	m.selectionStart = -1
	m.selectionEnd = -1
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	m = updated.(Model)

	if len(m.bookmarks) != 1 {
		t.Fatalf("expected exactly 1 bookmark after pinning row 4, got %d", len(m.bookmarks))
	}

	// Status bar should report 1 pinned, NOT all lines
	statusBar := m.viewStatusBar()
	if !strings.Contains(statusBar, "★ 1 pinned") {
		t.Errorf("expected status bar to show '★ 1 pinned', got: %s", statusBar)
	}
}

func TestBookmarksZephyrLogFile(t *testing.T) {
	// Sample lines from zephyr device logs
	sample := []string{
		"88:E7:12:6D:0E:D4 board_type: 2, board_type mapping:",
		"0----QFN",
		"1----CSP",
		"2----BGA",
		"[      5.182] <inf> fs_nvs: 16 Sectors of 4096 bytes",
		"[      5.182] <inf> fs_nvs: alloc wra: 0, fa0",
		"[      5.182] <inf> fs_nvs: data wra: 0, 1a",
		"[      5.182] <dbg> os_pm: pm_subscribe: os_pm: pm_subscribe",
		"[      5.182] <dbg> os_pm: pm_subscribe: os_pm: pm_subscribe succeeded 0x1009be50",
		"[      5.182] <inf> littlefs: LittleFS version 2.11, disk version 2.1",
	}

	m := newTestModel()
	zephyrParser, err := parser.NewRegexParser(`^\[\s*(?P<uptime>[^\]]+?)\s*\]\s+<(?P<level>[a-zA-Z]+)>\s+(?:(?P<module>[a-zA-Z0-9_.-]+):\s+)?(?P<message>.*)$`)
	if err != nil {
		t.Fatalf("failed to create zephyr parser: %v", err)
	}
	m.parser = zephyrParser

	for _, line := range sample {
		updated, _ := m.Update(lineMsg(line))
		m = updated.(Model)
	}

	if len(m.visible) != len(sample) {
		t.Fatalf("expected %d rows visible, got %d", len(sample), len(m.visible))
	}

	// 1. Single row bookmark: row 4
	m.selectedRow = 4
	m.selectionStart = -1
	m.selectionEnd = -1
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	m = updated.(Model)

	if len(m.bookmarks) != 1 {
		t.Fatalf("expected exactly 1 bookmark, got %d", len(m.bookmarks))
	}
	if _, ok := m.bookmarks[m.visible[4].ID]; !ok {
		t.Errorf("row 4 should be bookmarked")
	}
	if _, ok := m.bookmarks[m.visible[0].ID]; ok {
		t.Errorf("row 0 should NOT be bookmarked")
	}

	// 2. Multi-selection bookmark: rows 6 to 8 (3 rows)
	m.selectionStart = 6
	m.selectionEnd = 8
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	m = updated.(Model)

	// Total bookmarks should now be 1 + 3 = 4
	if len(m.bookmarks) != 4 {
		t.Fatalf("expected 4 bookmarks after multi-selection pin, got %d", len(m.bookmarks))
	}
	for i := 6; i <= 8; i++ {
		if _, ok := m.bookmarks[m.visible[i].ID]; !ok {
			t.Errorf("expected row %d to be bookmarked", i)
		}
	}

	// Filter to pinned only with 'B'
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'B'}})
	m = updated.(Model)

	if len(m.visible) != 4 {
		t.Fatalf("expected 4 rows in bookmarked-only view, got %d", len(m.visible))
	}
}


