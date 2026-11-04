package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/brenoniehues/oh-my-logs/internal/record"
)

func TestClearConfirmModal_Flow(t *testing.T) {
	m := newTestModel()

	// Ingest some log lines and a bookmark
	for i := 1; i <= 5; i++ {
		updated, _ := m.Update(lineMsg("boot line"))
		m = updated.(Model)
	}
	m.bookmarks[1] = struct{}{}

	if m.buffer.Len() != 5 {
		t.Fatalf("expected 5 records in buffer, got %d", m.buffer.Len())
	}

	// 1. Shift+C must open Settings, NOT clear confirm!
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}})
	m = updated.(Model)
	if m.mode != modeSettings {
		t.Fatalf("expected modeSettings on 'C', got %v", m.mode)
	}

	// Dismiss settings modal with Esc
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.mode != modeNormal {
		t.Fatalf("expected modeNormal after Esc, got %v", m.mode)
	}

	// 2. Press lowercase 'c' -> should transition to modeClearConfirm
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m = updated.(Model)
	if m.mode != modeClearConfirm {
		t.Fatalf("expected modeClearConfirm after 'c', got %v", m.mode)
	}

	// 3. View() must render the clear confirmation modal dialog
	view := m.View()
	if !strings.Contains(view, "Clear Screen Buffer") {
		t.Errorf("expected View() to contain modal title 'Clear Screen Buffer', got:\n%s", view)
	}
	if !strings.Contains(view, "Are you sure you want to clear all log") {
		t.Errorf("expected View() to contain confirmation prompt text, got:\n%s", view)
	}
	if !strings.Contains(view, "y / Enter confirm · n / Esc cancel") {
		t.Errorf("expected View() to contain modal footer, got:\n%s", view)
	}

	// Key bar hint must also display confirmation instructions
	keyBar := m.viewKeyBar()
	if !strings.Contains(keyBar, "Clear screen buffer?") {
		t.Errorf("expected keybar to contain 'Clear screen buffer?', got: %s", keyBar)
	}

	// 4. Test Cancel with 'n'
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m = updated.(Model)
	if m.mode != modeNormal {
		t.Fatalf("expected return to modeNormal on 'n', got %v", m.mode)
	}
	if m.message != "Clear cancelled" {
		t.Errorf("expected message 'Clear cancelled', got %q", m.message)
	}
	if m.buffer.Len() != 5 {
		t.Errorf("expected buffer to remain intact after cancel, got len %d", m.buffer.Len())
	}
	if len(m.bookmarks) != 1 {
		t.Errorf("expected bookmarks to remain intact after cancel, got %d", len(m.bookmarks))
	}

	// 5. Test Cancel with Esc
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m = updated.(Model)
	if m.mode != modeClearConfirm {
		t.Fatalf("expected modeClearConfirm on 'c', got %v", m.mode)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.mode != modeNormal || m.message != "Clear cancelled" {
		t.Errorf("expected cancel on Esc, got mode %v, message %q", m.mode, m.message)
	}

	// 6. Test Confirm with 'y'
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m = updated.(Model)
	if m.mode != modeClearConfirm {
		t.Fatalf("expected modeClearConfirm on 'c', got %v", m.mode)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = updated.(Model)
	if m.mode != modeNormal {
		t.Fatalf("expected modeNormal after confirmed clear, got %v", m.mode)
	}
	if m.message != "Buffer cleared" {
		t.Errorf("expected message 'Buffer cleared', got %q", m.message)
	}
	if m.buffer.Len() != 0 {
		t.Errorf("expected empty buffer after clear, got %d", m.buffer.Len())
	}
	if len(m.bookmarks) != 0 {
		t.Errorf("expected empty bookmarks after clear, got %d", len(m.bookmarks))
	}
	if len(m.visible) != 0 {
		t.Errorf("expected empty visible slice after clear, got %d", len(m.visible))
	}

	// 7. Test Confirm with Enter
	r := record.NewRecord("fresh line")
	r.Timestamp = time.Now()
	m.buffer.Add(r)
	m.recalcLayout()
	if m.buffer.Len() != 1 {
		t.Fatalf("expected 1 record in buffer, got %d", m.buffer.Len())
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.buffer.Len() != 0 || m.message != "Buffer cleared" {
		t.Errorf("expected clear on Enter, got len %d, message %q", m.buffer.Len(), m.message)
	}
}
