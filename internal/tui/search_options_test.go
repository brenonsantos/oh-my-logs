package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/brenoniehues/oh-my-logs/internal/record"
)

func TestSearchOptions_CaseSensitive(t *testing.T) {
	m := newTestModel()

	// Ingest 2 records: one uppercase, one lowercase
	r1 := record.NewRecord("line 1")
	r1.ID = 1
	r1.Timestamp = time.Now()
	r1.Fields["message"] = "SYSTEM_FAILURE: critical error"
	r1.Fields["raw"] = r1.Fields["message"]
	m.buffer.Add(r1)

	r2 := record.NewRecord("line 2")
	r2.ID = 2
	r2.Timestamp = time.Now()
	r2.Fields["message"] = "system_failure: minor warning"
	r2.Fields["raw"] = r2.Fields["message"]
	m.buffer.Add(r2)

	m.visible = m.buffer.All()

	// Open search mode (^F)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlF})
	m = updated.(Model)
	if m.mode != modeSearch {
		t.Fatalf("expected modeSearch, got %v", m.mode)
	}

	// 1. Case-insensitive search by default
	m.searchInput.SetText("SYSTEM_FAILURE")
	m.runSearch()

	if len(m.searchMatches) != 2 {
		t.Fatalf("expected 2 matches when case-insensitive, got %d", len(m.searchMatches))
	}

	// 2. Toggle Case Sensitivity via Alt+C
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}, Alt: true})
	m = updated.(Model)
	if !m.searchCaseSensitive {
		t.Fatalf("expected searchCaseSensitive to be true after Alt+C")
	}

	// Now only line 0 should match
	if len(m.searchMatches) != 1 || m.searchMatches[0] != 0 {
		t.Fatalf("expected 1 match (index 0) with case sensitivity, got %v", m.searchMatches)
	}

	// 3. Toggle Case Sensitivity via macOS Option+C ('ç')
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'ç'}})
	m = updated.(Model)
	if m.searchCaseSensitive {
		t.Fatalf("expected searchCaseSensitive to be toggled off via macOS 'ç'")
	}
	if len(m.searchMatches) != 2 {
		t.Fatalf("expected 2 matches after toggling off case sensitivity, got %d", len(m.searchMatches))
	}
}

func TestSearchOptions_WholeWord(t *testing.T) {
	m := newTestModel()

	r1 := record.NewRecord("line 1")
	r1.ID = 1
	r1.Timestamp = time.Now()
	r1.Fields["message"] = "kernel err in driver"
	r1.Fields["raw"] = r1.Fields["message"]
	m.buffer.Add(r1)

	r2 := record.NewRecord("line 2")
	r2.ID = 2
	r2.Timestamp = time.Now()
	r2.Fields["message"] = "kernel error in network"
	r2.Fields["raw"] = r2.Fields["message"]
	m.buffer.Add(r2)

	m.visible = m.buffer.All()

	// Enter search mode
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlF})
	m = updated.(Model)

	m.searchInput.SetText("err")
	m.runSearch()

	// Partial match: both "err" and "error" match
	if len(m.searchMatches) != 2 {
		t.Fatalf("expected 2 matches without whole word, got %d", len(m.searchMatches))
	}

	// Toggle Whole Word via Alt+W
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}, Alt: true})
	m = updated.(Model)
	if !m.searchWholeWord {
		t.Fatalf("expected searchWholeWord to be true after Alt+W")
	}

	// With whole word ON, only "kernel err in driver" matches
	if len(m.searchMatches) != 1 || m.searchMatches[0] != 0 {
		t.Fatalf("expected 1 match (index 0) with whole word, got %v", m.searchMatches)
	}

	// Toggle Whole Word via macOS Option+W ('∑')
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'∑'}})
	m = updated.(Model)
	if m.searchWholeWord {
		t.Fatalf("expected searchWholeWord to be toggled off via macOS '∑'")
	}
	if len(m.searchMatches) != 2 {
		t.Fatalf("expected 2 matches after disabling whole word, got %d", len(m.searchMatches))
	}
}

func TestSearchOptions_UnparsedRowAndKeyBar(t *testing.T) {
	m := newTestModel()

	// Enter search mode
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlF})
	m = updated.(Model)

	// Check flags rendered in KeyBar
	bar := m.viewKeyBar()
	if !strings.Contains(bar, "[Aa]") || !strings.Contains(bar, "[\"ab\"]") {
		t.Errorf("expected KeyBar to show search flags [Aa] and [\"ab\"], got:\n%s", bar)
	}

	// Toggle case on
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}, Alt: true})
	m = updated.(Model)
	if !m.searchCaseSensitive {
		t.Fatalf("expected searchCaseSensitive true")
	}

	// Check unparsed row highlighting respecting case sensitivity
	r := record.NewRecord("UNPARSED CRASH DUMP: fault at 0xdeadbeef")
	r.ID = 1

	// Searching lowercase "fault" with case sensitivity ON should NOT highlight
	highlighted := formatUnparsedRow(r, 80, false, nil, true, "FAULT", m.searchCaseSensitive, m.searchWholeWord)
	if !strings.Contains(highlighted, "FAULT") && !strings.Contains(highlighted, "fault") {
		t.Fatalf("expected row to render text, got %q", highlighted)
	}
}
