package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestTextInput_BasicEditing(t *testing.T) {
	ti := NewTextInput(false)
	if ti.Text() != "" || ti.Cursor != 0 {
		t.Fatalf("expected empty input with cursor 0, got %q pos %d", ti.Text(), ti.Cursor)
	}

	ti.SetText("hello")
	if ti.Text() != "hello" || ti.Cursor != 5 {
		t.Fatalf("expected 'hello' pos 5, got %q pos %d", ti.Text(), ti.Cursor)
	}

	// Insert at end
	ti.Insert(" world")
	if ti.Text() != "hello world" || ti.Cursor != 11 {
		t.Fatalf("expected 'hello world' pos 11, got %q pos %d", ti.Text(), ti.Cursor)
	}

	// Set text and cursor explicitly
	ti.SetTextAndCursor("test string", 4)
	if ti.Text() != "test string" || ti.Cursor != 4 {
		t.Fatalf("expected 'test string' pos 4, got %q pos %d", ti.Text(), ti.Cursor)
	}

	// Clear
	ti.Clear()
	if ti.Text() != "" || ti.Cursor != 0 {
		t.Fatalf("expected empty input pos 0 after Clear, got %q pos %d", ti.Text(), ti.Cursor)
	}
}

func TestTextInput_HandleKey_Editing(t *testing.T) {
	ti := NewTextInput(false)

	// Type runes
	for _, r := range "hello" {
		ti.HandleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	if ti.Text() != "hello" || ti.Cursor != 5 {
		t.Fatalf("expected 'hello' pos 5, got %q pos %d", ti.Text(), ti.Cursor)
	}

	// Left key
	ti.HandleKey(tea.KeyMsg{Type: tea.KeyLeft})
	if ti.Cursor != 4 {
		t.Fatalf("expected cursor 4, got %d", ti.Cursor)
	}

	// Backspace at 4 -> deletes 'l' before 'o' -> "helo"
	ti.HandleKey(tea.KeyMsg{Type: tea.KeyBackspace})
	if ti.Text() != "helo" || ti.Cursor != 3 {
		t.Fatalf("expected 'helo' pos 3, got %q pos %d", ti.Text(), ti.Cursor)
	}

	// Delete at 3 -> deletes 'o' under cursor -> "hel"
	ti.HandleKey(tea.KeyMsg{Type: tea.KeyDelete})
	if ti.Text() != "hel" || ti.Cursor != 3 {
		t.Fatalf("expected 'hel' pos 3, got %q pos %d", ti.Text(), ti.Cursor)
	}

	// Home
	ti.HandleKey(tea.KeyMsg{Type: tea.KeyHome})
	if ti.Cursor != 0 {
		t.Fatalf("expected cursor 0, got %d", ti.Cursor)
	}

	// End
	ti.HandleKey(tea.KeyMsg{Type: tea.KeyEnd})
	if ti.Cursor != 3 {
		t.Fatalf("expected cursor 3, got %d", ti.Cursor)
	}

	// Space
	ti.HandleKey(tea.KeyMsg{Type: tea.KeySpace})
	if ti.Text() != "hel " || ti.Cursor != 4 {
		t.Fatalf("expected 'hel ' pos 4, got %q pos %d", ti.Text(), ti.Cursor)
	}

	// Word deletion (Ctrl+W)
	ti.SetText("foo bar baz")
	ti.HandleKey(tea.KeyMsg{Type: tea.KeyCtrlW})
	if ti.Text() != "foo bar " || ti.Cursor != 8 {
		t.Fatalf("expected 'foo bar ' pos 8 after Ctrl+W, got %q pos %d", ti.Text(), ti.Cursor)
	}

	// Kill to end of line (Ctrl+K)
	ti.SetTextAndCursor("foo bar baz", 4)
	ti.HandleKey(tea.KeyMsg{Type: tea.KeyCtrlK})
	if ti.Text() != "foo " || ti.Cursor != 4 {
		t.Fatalf("expected 'foo ' pos 4 after Ctrl+K, got %q pos %d", ti.Text(), ti.Cursor)
	}

	// Clear line (Ctrl+U)
	ti.HandleKey(tea.KeyMsg{Type: tea.KeyCtrlU})
	if ti.Text() != "" || ti.Cursor != 0 {
		t.Fatalf("expected empty pos 0 after Ctrl+U, got %q pos %d", ti.Text(), ti.Cursor)
	}
}

func TestTextInput_History(t *testing.T) {
	ti := NewTextInput(true) // WithHistory enabled

	// Add entries
	ti.AddHistory("first")
	ti.AddHistory("second")
	ti.AddHistory("second") // duplicate ignored
	ti.AddHistory("   ")    // whitespace ignored
	ti.AddHistory("third")

	if len(ti.History) != 3 {
		t.Fatalf("expected 3 history items, got %d", len(ti.History))
	}

	// User types draft: "my draft"
	ti.SetText("my draft")

	// Up -> recall "third"
	changed := ti.HandleKey(tea.KeyMsg{Type: tea.KeyUp})
	if !changed || ti.Text() != "third" {
		t.Fatalf("expected 'third', got %q", ti.Text())
	}
	if ti.Draft != "my draft" {
		t.Fatalf("expected draft preserved as 'my draft', got %q", ti.Draft)
	}

	// Up -> recall "second"
	ti.HandleKey(tea.KeyMsg{Type: tea.KeyUp})
	if ti.Text() != "second" {
		t.Fatalf("expected 'second', got %q", ti.Text())
	}

	// Up -> recall "first"
	ti.HandleKey(tea.KeyMsg{Type: tea.KeyUp})
	if ti.Text() != "first" {
		t.Fatalf("expected 'first', got %q", ti.Text())
	}

	// Up at boundary -> stays "first"
	ti.HandleKey(tea.KeyMsg{Type: tea.KeyUp})
	if ti.Text() != "first" {
		t.Fatalf("expected 'first' at boundary, got %q", ti.Text())
	}

	// Down -> "second"
	ti.HandleKey(tea.KeyMsg{Type: tea.KeyDown})
	if ti.Text() != "second" {
		t.Fatalf("expected 'second', got %q", ti.Text())
	}

	// Down -> "third"
	ti.HandleKey(tea.KeyMsg{Type: tea.KeyDown})
	if ti.Text() != "third" {
		t.Fatalf("expected 'third', got %q", ti.Text())
	}

	// Down past end -> restore draft
	ti.HandleKey(tea.KeyMsg{Type: tea.KeyDown})
	if ti.Text() != "my draft" {
		t.Fatalf("expected draft 'my draft' restored, got %q", ti.Text())
	}
	if ti.HistoryCursor != -1 {
		t.Fatalf("expected HistoryCursor -1, got %d", ti.HistoryCursor)
	}

	// Reset clears draft and history cursor
	ti.HistoryPrev()
	ti.Reset()
	if ti.Text() != "" || ti.Draft != "" || ti.HistoryCursor != -1 {
		t.Fatalf("expected reset state, got val=%q draft=%q cur=%d", ti.Text(), ti.Draft, ti.HistoryCursor)
	}
}

func TestTextInput_RenderAndWindow(t *testing.T) {
	ti := NewTextInput(false)
	ti.SetText("hello")
	style := lipgloss.NewStyle()

	r := ti.Render(style)
	if !strings.Contains(r, "hello") || !strings.Contains(r, "█") {
		t.Fatalf("expected render to contain 'hello' and '█', got %q", r)
	}

	rw := ti.RenderWindow(10, style)
	if !strings.Contains(rw, "hello") {
		t.Fatalf("expected window render to contain 'hello', got %q", rw)
	}
}
