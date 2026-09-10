package tui

import (
	"strings"
	"testing"

	"github.com/brenoniehues/oh-my-logs/internal/config"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestHandleTextInputWithCursor_Basic(t *testing.T) {
	str := "hello"
	pos := 5

	// Type a character at the end
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'!'}}
	str, pos = handleTextInputWithCursor(str, pos, msg)
	if str != "hello!" || pos != 6 {
		t.Fatalf("expected 'hello!' pos 6, got %q pos %d", str, pos)
	}

	// Move Left twice
	msgLeft := tea.KeyMsg{Type: tea.KeyLeft}
	str, pos = handleTextInputWithCursor(str, pos, msgLeft)
	str, pos = handleTextInputWithCursor(str, pos, msgLeft)
	if str != "hello!" || pos != 4 {
		t.Fatalf("expected 'hello!' pos 4 (before 'o'), got %q pos %d", str, pos)
	}

	// Insert 'X' in the middle
	msgX := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'X'}}
	str, pos = handleTextInputWithCursor(str, pos, msgX)
	if str != "hellXo!" || pos != 5 {
		t.Fatalf("expected 'hellXo!' pos 5, got %q pos %d", str, pos)
	}

	// Backspace deletes 'X'
	msgBS := tea.KeyMsg{Type: tea.KeyBackspace}
	str, pos = handleTextInputWithCursor(str, pos, msgBS)
	if str != "hello!" || pos != 4 {
		t.Fatalf("expected 'hello!' pos 4 after backspace, got %q pos %d", str, pos)
	}

	// Delete deletes 'o' under cursor
	msgDel := tea.KeyMsg{Type: tea.KeyDelete}
	str, pos = handleTextInputWithCursor(str, pos, msgDel)
	if str != "hell!" || pos != 4 {
		t.Fatalf("expected 'hell!' pos 4 after delete, got %q pos %d", str, pos)
	}

	// Home jumps to start
	msgHome := tea.KeyMsg{Type: tea.KeyHome}
	str, pos = handleTextInputWithCursor(str, pos, msgHome)
	if pos != 0 {
		t.Fatalf("expected pos 0 after Home, got %d", pos)
	}

	// Backspace at 0 does nothing
	str, pos = handleTextInputWithCursor(str, pos, msgBS)
	if str != "hell!" || pos != 0 {
		t.Fatalf("expected backspace at 0 to be no-op, got %q pos %d", str, pos)
	}

	// End jumps to end
	msgEnd := tea.KeyMsg{Type: tea.KeyEnd}
	str, pos = handleTextInputWithCursor(str, pos, msgEnd)
	if pos != 5 {
		t.Fatalf("expected pos 5 after End, got %d", pos)
	}

	// Delete at end does nothing
	str, pos = handleTextInputWithCursor(str, pos, msgDel)
	if str != "hell!" || pos != 5 {
		t.Fatalf("expected delete at end to be no-op, got %q pos %d", str, pos)
	}
}

func TestHandleTextInputWithCursor_ReadlineAndWordJumps(t *testing.T) {
	str := "apple banana cherry"
	pos := len([]rune(str)) // 19

	// Ctrl+A -> Home
	msgCtrlA := tea.KeyMsg{Type: tea.KeyCtrlA}
	str, pos = handleTextInputWithCursor(str, pos, msgCtrlA)
	if pos != 0 {
		t.Fatalf("expected pos 0 after Ctrl+A, got %d", pos)
	}

	// Alt+Right -> jump forward by word using custom string key
	str, pos = handleTextInputWithCursor(str, pos, tea.KeyMsg{Type: tea.KeyRunes, Runes: nil})
	// Simulate "alt+right"
	str, pos = handleTextInputWithCursor(str, pos, fakeKeyMsg("alt+right"))
	if pos != 6 { // after "apple "
		t.Fatalf("expected pos 6 after alt+right, got %d", pos)
	}

	str, pos = handleTextInputWithCursor(str, pos, fakeKeyMsg("alt+right"))
	if pos != 13 { // after "banana "
		t.Fatalf("expected pos 13 after second alt+right, got %d", pos)
	}

	// Alt+Left -> jump backward by word
	str, pos = handleTextInputWithCursor(str, pos, fakeKeyMsg("alt+left"))
	if pos != 6 { // before "banana"
		t.Fatalf("expected pos 6 after alt+left, got %d", pos)
	}

	// Ctrl+K -> kill to end of line
	str, pos = handleTextInputWithCursor(str, pos, tea.KeyMsg{Type: tea.KeyCtrlK})
	if str != "apple " || pos != 6 {
		t.Fatalf("expected 'apple ' pos 6 after Ctrl+K, got %q pos %d", str, pos)
	}

	// Ctrl+U -> clear line
	str, pos = handleTextInputWithCursor(str, pos, tea.KeyMsg{Type: tea.KeyCtrlU})
	if str != "" || pos != 0 {
		t.Fatalf("expected empty string pos 0 after Ctrl+U, got %q pos %d", str, pos)
	}
}

func TestHandleTextInputWithCursor_UTF8(t *testing.T) {
	str := "こんにちは" // 5 runes
	pos := 5

	// Move left twice -> pos 3
	str, pos = handleTextInputWithCursor(str, pos, tea.KeyMsg{Type: tea.KeyLeft})
	str, pos = handleTextInputWithCursor(str, pos, tea.KeyMsg{Type: tea.KeyLeft})
	if pos != 3 {
		t.Fatalf("expected pos 3, got %d", pos)
	}

	// Insert "世界"
	str, pos = handleTextInputWithCursor(str, pos, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'世'}})
	str, pos = handleTextInputWithCursor(str, pos, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'界'}})
	if str != "こんに世界ちは" || pos != 5 {
		t.Fatalf("expected 'こんに世界ちは' pos 5, got %q pos %d", str, pos)
	}

	// Backspace removes '界'
	str, pos = handleTextInputWithCursor(str, pos, tea.KeyMsg{Type: tea.KeyBackspace})
	if str != "こんに世ちは" || pos != 4 {
		t.Fatalf("expected 'こんに世ちは' pos 4, got %q pos %d", str, pos)
	}
}

func TestInsertStringAtCursor(t *testing.T) {
	orig := "ac"
	res, newPos := insertStringAtCursor(orig, 1, "b")
	if res != "abc" || newPos != 2 {
		t.Fatalf("expected 'abc' pos 2, got %q pos %d", res, newPos)
	}

	// Insert multi-character at start
	res, newPos = insertStringAtCursor("world", 0, "hello ")
	if res != "hello world" || newPos != 6 {
		t.Fatalf("expected 'hello world' pos 6, got %q pos %d", res, newPos)
	}

	// Insert at end
	res, newPos = insertStringAtCursor("foo", 3, "bar")
	if res != "foobar" || newPos != 6 {
		t.Fatalf("expected 'foobar' pos 6, got %q pos %d", res, newPos)
	}
}

func TestRenderInputWithCursor(t *testing.T) {
	style := lipgloss.NewStyle()

	// End of text: should include █
	renderedEnd := renderInputWithCursor("test", 4, style)
	if !strings.Contains(renderedEnd, "█") || !strings.Contains(renderedEnd, "test") {
		t.Fatalf("expected renderedEnd to contain 'test' and '█', got %q", renderedEnd)
	}

	// Middle of text: should highlight character at cursor
	renderedMid := renderInputWithCursor("test", 1, style)
	if strings.Contains(renderedMid, "█") {
		t.Fatalf("renderedMid should not contain end-block '█', got %q", renderedMid)
	}
	if !strings.Contains(renderedMid, "t") || !strings.Contains(renderedMid, "e") || !strings.Contains(renderedMid, "st") {
		t.Fatalf("renderedMid missing characters, got %q", renderedMid)
	}

	// Empty string: should render █
	renderedEmpty := renderInputWithCursor("", 0, style)
	if !strings.Contains(renderedEmpty, "█") {
		t.Fatalf("expected empty input to render cursor block █, got %q", renderedEmpty)
	}
}

func TestSliceInputForWindow(t *testing.T) {
	// String shorter than maxW
	text, cur := sliceInputForWindow("hello", 3, 10)
	if text != "hello" || cur != 3 {
		t.Fatalf("expected 'hello' cur 3, got %q cur %d", text, cur)
	}

	// Long string, cursor at start
	longText := "0123456789abcdefghij"
	text, cur = sliceInputForWindow(longText, 0, 10)
	if len([]rune(text)) != 10 || cur != 0 {
		t.Fatalf("expected window length 10 cur 0, got %q (%d) cur %d", text, len([]rune(text)), cur)
	}

	// Long string, cursor at end
	text, cur = sliceInputForWindow(longText, len(longText), 10)
	if len([]rune(text)) != 10 || cur != 10 {
		t.Fatalf("expected window length 10 cur 10, got %q (%d) cur %d", text, len([]rune(text)), cur)
	}

	// Long string, cursor in middle
	text, cur = sliceInputForWindow(longText, 10, 10)
	if len([]rune(text)) != 10 || cur < 0 || cur > 10 {
		t.Fatalf("expected window length 10 cur within bounds, got %q (%d) cur %d", text, len([]rune(text)), cur)
	}
}

func TestFilterMode_CursorNavigationAndTypingK(t *testing.T) {
	m := newTestModel()
	m.mode = modeFilter
	m.filterInput = "task:worker"
	m.filterCursor = len([]rune(m.filterInput)) // 11

	// Ensure typing 'k' or 'j' does NOT trigger history Up/Down!
	mMod, _ := m.handleFilterKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m = mMod.(Model)
	if m.filterInput != "task:workerk" || m.filterCursor != 12 {
		t.Fatalf("expected typing 'k' to append 'k', got %q cursor %d", m.filterInput, m.filterCursor)
	}

	mMod, _ = m.handleFilterKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = mMod.(Model)
	if m.filterInput != "task:workerkj" || m.filterCursor != 13 {
		t.Fatalf("expected typing 'j' to append 'j', got %q cursor %d", m.filterInput, m.filterCursor)
	}

	// Move Left 4 times
	for i := 0; i < 4; i++ {
		mMod, _ = m.handleFilterKey(tea.KeyMsg{Type: tea.KeyLeft})
		m = mMod.(Model)
	}
	if m.filterCursor != 9 {
		t.Fatalf("expected filterCursor 9, got %d", m.filterCursor)
	}

	// Insert 'X'
	mMod, _ = m.handleFilterKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'X'}})
	m = mMod.(Model)
	if m.filterInput != "task:workXerkj" || m.filterCursor != 10 {
		t.Fatalf("expected 'task:workXerkj' cursor 10, got %q cursor %d", m.filterInput, m.filterCursor)
	}

	// Backspace deletes 'X'
	mMod, _ = m.handleFilterKey(tea.KeyMsg{Type: tea.KeyBackspace})
	m = mMod.(Model)
	if m.filterInput != "task:workerkj" || m.filterCursor != 9 {
		t.Fatalf("expected 'task:workerkj' cursor 9 after backspace, got %q cursor %d", m.filterInput, m.filterCursor)
	}

	// Delete deletes 'e'
	mMod, _ = m.handleFilterKey(tea.KeyMsg{Type: tea.KeyDelete})
	m = mMod.(Model)
	if m.filterInput != "task:workrkj" || m.filterCursor != 9 {
		t.Fatalf("expected 'task:workrkj' cursor 9 after delete, got %q cursor %d", m.filterInput, m.filterCursor)
	}
}

func TestSearchMode_CursorNavigation(t *testing.T) {
	m := newTestModel()
	m.mode = modeSearch
	m.searchInput = "sensor"
	m.searchPos = 6

	// Move left 3 times
	for i := 0; i < 3; i++ {
		mMod, _ := m.handleSearchKey(tea.KeyMsg{Type: tea.KeyLeft})
		m = mMod.(Model)
	}
	if m.searchPos != 3 {
		t.Fatalf("expected searchPos 3, got %d", m.searchPos)
	}

	// Insert "12"
	mMod, _ := m.handleSearchKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1', '2'}})
	m = mMod.(Model)
	if m.searchInput != "sen12sor" || m.searchPos != 5 {
		t.Fatalf("expected 'sen12sor' pos 5, got %q pos %d", m.searchInput, m.searchPos)
	}
}

func TestTXMode_CursorNavigationAndTypingK(t *testing.T) {
	m := newTestModel()
	m.mode = modeTXInput
	m.txInput = "pkg:status"
	m.txCursor = len([]rune(m.txInput)) // 10

	// Typing 'k' or 'j' should NOT trigger history!
	mMod, _ := m.handleTXKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m = mMod.(Model)
	if m.txInput != "pkg:statusk" || m.txCursor != 11 {
		t.Fatalf("expected 'pkg:statusk' txCursor 11, got %q txCursor %d", m.txInput, m.txCursor)
	}

	// Home key
	mMod, _ = m.handleTXKey(tea.KeyMsg{Type: tea.KeyHome})
	m = mMod.(Model)
	if m.txCursor != 0 {
		t.Fatalf("expected txCursor 0 after Home, got %d", m.txCursor)
	}

	// Insert "AT+"
	mMod, _ = m.handleTXKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A', 'T', '+'}})
	m = mMod.(Model)
	if m.txInput != "AT+pkg:statusk" || m.txCursor != 3 {
		t.Fatalf("expected 'AT+pkg:statusk' txCursor 3, got %q txCursor %d", m.txInput, m.txCursor)
	}
}

func TestSavePresetPrompt_CursorNavigation(t *testing.T) {
	m := newTestModel()
	m.filtersCfg = &config.FiltersConfig{}
	m.mode = modeSavePresetPrompt
	m.savePresetNameInput = "My Preset"
	m.savePresetNameCursor = 9

	// Move Left 3 times (before "set")
	for i := 0; i < 3; i++ {
		mMod, _ := m.handleSavePresetPromptKey(tea.KeyMsg{Type: tea.KeyLeft})
		m = mMod.(Model)
	}
	if m.savePresetNameCursor != 6 {
		t.Fatalf("expected savePresetNameCursor 6, got %d", m.savePresetNameCursor)
	}

	// Delete deletes 's'
	mMod, _ := m.handleSavePresetPromptKey(tea.KeyMsg{Type: tea.KeyDelete})
	m = mMod.(Model)
	if m.savePresetNameInput != "My Preet" || m.savePresetNameCursor != 6 {
		t.Fatalf("expected 'My Preet' cursor 6, got %q cursor %d", m.savePresetNameInput, m.savePresetNameCursor)
	}

	// Ctrl+U clears
	mMod, _ = m.handleSavePresetPromptKey(tea.KeyMsg{Type: tea.KeyCtrlU})
	m = mMod.(Model)
	if m.savePresetNameInput != "" || m.savePresetNameCursor != 0 {
		t.Fatalf("expected empty string cursor 0, got %q cursor %d", m.savePresetNameInput, m.savePresetNameCursor)
	}
}

// fakeKeyMsg creates a tea.KeyMsg matching BubbleTea's string representation.
func fakeKeyMsg(s string) tea.KeyMsg {
	switch s {
	case "alt+left":
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}, Alt: true}
	case "alt+right":
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}, Alt: true}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

var _ = key.NewBinding

