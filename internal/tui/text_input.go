package tui

import (
	"strings"
	"unicode"

	"github.com/brenoniehues/oh-my-logs/internal/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TextInput manages single-line text input with cursor positioning,
// editing shortcuts, optional history recall, and clipboard support.
type TextInput struct {
	Value         string
	Cursor        int
	History       []string
	HistoryCursor int
	Draft         string
	WithHistory   bool
}

// NewTextInput creates an empty TextInput. If withHistory is true,
// up/down navigation traverses history.
func NewTextInput(withHistory bool) TextInput {
	return TextInput{
		HistoryCursor: -1,
		WithHistory:   withHistory,
	}
}

// Text returns the current input string.
func (t *TextInput) Text() string {
	return t.Value
}

// String implements fmt.Stringer, returning the current input string.
func (t *TextInput) String() string {
	return t.Value
}

// SetText replaces the current text and places the cursor at the end.
func (t *TextInput) SetText(s string) {
	t.Value = s
	t.Cursor = len([]rune(s))
}

// SetTextAndCursor sets both text and cursor position (clamped to bounds).
func (t *TextInput) SetTextAndCursor(s string, cursor int) {
	t.Value = s
	runes := []rune(s)
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(runes) {
		cursor = len(runes)
	}
	t.Cursor = cursor
}

// Clear clears the input value and resets cursor to 0.
func (t *TextInput) Clear() {
	t.Value = ""
	t.Cursor = 0
}

// Reset clears value, cursor, draft, and resets history cursor to -1.
func (t *TextInput) Reset() {
	t.Clear()
	t.Draft = ""
	t.HistoryCursor = -1
}

// ResetHistoryCursor resets the history navigation pointer to -1.
func (t *TextInput) ResetHistoryCursor() {
	t.Draft = ""
	t.HistoryCursor = -1
}

// Insert inserts a string at the current cursor position and moves the cursor.
func (t *TextInput) Insert(s string) {
	t.Value, t.Cursor = insertStringAtCursor(t.Value, t.Cursor, s)
}

// Paste reads from clipboard, strips newlines, and inserts at cursor.
func (t *TextInput) Paste() bool {
	clipText, err := clipboard.Read()
	if err == nil && clipText != "" {
		clean := strings.ReplaceAll(strings.ReplaceAll(clipText, "\r", ""), "\n", " ")
		t.Insert(strings.TrimSpace(clean))
		return true
	}
	return false
}

// AddHistory appends entry to history if trimmed entry is non-empty and different from the last entry.
func (t *TextInput) AddHistory(entry string) bool {
	trimmed := strings.TrimSpace(entry)
	if trimmed == "" {
		return false
	}
	if len(t.History) == 0 || t.History[len(t.History)-1] != trimmed {
		t.History = append(t.History, trimmed)
		return true
	}
	return false
}

// HistoryPrev moves to the previous history item (Up arrow).
func (t *TextInput) HistoryPrev() bool {
	if len(t.History) == 0 {
		return false
	}
	if t.HistoryCursor == -1 {
		t.Draft = t.Value
		t.HistoryCursor = len(t.History) - 1
	} else if t.HistoryCursor > 0 {
		t.HistoryCursor--
	}
	if t.HistoryCursor >= 0 && t.HistoryCursor < len(t.History) {
		t.SetText(t.History[t.HistoryCursor])
		return true
	}
	return false
}

// HistoryNext moves to the next history item (Down arrow).
func (t *TextInput) HistoryNext() bool {
	if t.HistoryCursor == -1 {
		return false
	}
	if t.HistoryCursor < len(t.History)-1 {
		t.HistoryCursor++
		t.SetText(t.History[t.HistoryCursor])
	} else {
		t.HistoryCursor = -1
		t.SetText(t.Draft)
	}
	return true
}

// HandleKey processes key events for editing, cursor movement, clipboard paste,
// and (if WithHistory is enabled) history navigation.
// Returns true if the value or cursor changed.
func (t *TextInput) HandleKey(msg tea.KeyMsg) bool {
	switch {
	case msg.Type == tea.KeyCtrlV || msg.String() == "ctrl+v":
		return t.Paste()

	case t.WithHistory && (msg.Type == tea.KeyUp || msg.String() == "up"):
		return t.HistoryPrev()

	case t.WithHistory && (msg.Type == tea.KeyDown || msg.String() == "down"):
		return t.HistoryNext()

	default:
		oldVal, oldCur := t.Value, t.Cursor
		t.Value, t.Cursor = handleTextInputWithCursor(t.Value, t.Cursor, msg)
		return t.Value != oldVal || t.Cursor != oldCur
	}
}

// Render renders the text input with visual block cursor using baseStyle.
func (t *TextInput) Render(baseStyle lipgloss.Style) string {
	return renderInputWithCursor(t.Value, t.Cursor, baseStyle)
}

// RenderWindow renders the text input within a sliding window of maxW runes.
func (t *TextInput) RenderWindow(maxW int, baseStyle lipgloss.Style) string {
	disp, relCur := sliceInputForWindow(t.Value, t.Cursor, maxW)
	return renderInputWithCursor(disp, relCur, baseStyle)
}

// insertStringAtCursor inserts toInsert into current at the 0-indexed rune position pos,
// returning the resulting string and the new cursor position after the inserted text.
func insertStringAtCursor(current string, pos int, toInsert string) (string, int) {
	runes := []rune(current)
	if pos < 0 {
		pos = 0
	}
	if pos > len(runes) {
		pos = len(runes)
	}
	ins := []rune(toInsert)
	result := make([]rune, 0, len(runes)+len(ins))
	result = append(result, runes[:pos]...)
	result = append(result, ins...)
	result = append(result, runes[pos:]...)
	return string(result), pos + len(ins)
}

// handleTextInput is a backward-compatible wrapper appending/deleting at the end.
func handleTextInput(current string, msg tea.KeyMsg) string {
	res, _ := handleTextInputWithCursor(current, len([]rune(current)), msg)
	return res
}

// handleTextInputWithCursor handles printable character insertion, deletion, and horizontal navigation.
// It accepts the current string and the 0-indexed rune cursor position pos, returning the updated string and cursor.
func handleTextInputWithCursor(current string, pos int, msg tea.KeyMsg) (string, int) {
	runes := []rune(current)
	if pos < 0 {
		pos = 0
	}
	if pos > len(runes) {
		pos = len(runes)
	}

	switch msg.Type {
	case tea.KeyLeft:
		if pos > 0 {
			pos--
		}
		return current, pos

	case tea.KeyRight:
		if pos < len(runes) {
			pos++
		}
		return current, pos

	case tea.KeyHome, tea.KeyCtrlA:
		return current, 0

	case tea.KeyEnd, tea.KeyCtrlE:
		return current, len(runes)

	case tea.KeyBackspace:
		if pos > 0 {
			runes = append(runes[:pos-1], runes[pos:]...)
			pos--
			return string(runes), pos
		}
		return current, pos

	case tea.KeyDelete, tea.KeyCtrlD:
		if pos < len(runes) {
			runes = append(runes[:pos], runes[pos+1:]...)
			return string(runes), pos
		}
		return current, pos

	case tea.KeyCtrlW:
		end := pos
		for pos > 0 && unicode.IsSpace(runes[pos-1]) {
			pos--
		}
		for pos > 0 && !unicode.IsSpace(runes[pos-1]) {
			pos--
		}
		runes = append(runes[:pos], runes[end:]...)
		return string(runes), pos

	case tea.KeyCtrlK:
		runes = runes[:pos]
		return string(runes), pos

	case tea.KeyCtrlU:
		return "", 0

	case tea.KeyRunes:
		if !msg.Alt {
			for _, r := range msg.Runes {
				if unicode.IsPrint(r) {
					runes = append(runes[:pos], append([]rune{r}, runes[pos:]...)...)
					pos++
				}
			}
			return string(runes), pos
		}

	case tea.KeySpace:
		runes = append(runes[:pos], append([]rune{' '}, runes[pos:]...)...)
		pos++
		return string(runes), pos
	}

	// String fallback for terminals reporting specific escape sequence names
	switch msg.String() {
	case "left", "ctrl+b":
		if pos > 0 {
			pos--
		}
	case "right", "ctrl+f":
		if pos < len(runes) {
			pos++
		}
	case "home", "ctrl+a":
		pos = 0
	case "end", "ctrl+e":
		pos = len(runes)
	case "delete", "ctrl+d":
		if pos < len(runes) {
			runes = append(runes[:pos], runes[pos+1:]...)
			return string(runes), pos
		}
	case "ctrl+w":
		end := pos
		for pos > 0 && unicode.IsSpace(runes[pos-1]) {
			pos--
		}
		for pos > 0 && !unicode.IsSpace(runes[pos-1]) {
			pos--
		}
		runes = append(runes[:pos], runes[end:]...)
		return string(runes), pos
	case "ctrl+k":
		runes = runes[:pos]
		return string(runes), pos
	case "ctrl+u":
		return "", 0
	case "alt+left", "alt+b", "ctrl+left":
		for pos > 0 && unicode.IsSpace(runes[pos-1]) {
			pos--
		}
		for pos > 0 && !unicode.IsSpace(runes[pos-1]) {
			pos--
		}
	case "alt+right", "alt+f", "ctrl+right":
		for pos < len(runes) && !unicode.IsSpace(runes[pos]) {
			pos++
		}
		for pos < len(runes) && unicode.IsSpace(runes[pos]) {
			pos++
		}
	}

	return string(runes), pos
}

// renderInputWithCursor renders input text with an accurate visual block cursor.
// If the cursor is at the end (pos >= len(runes)), a "█" block is appended.
// If the cursor is over a character, that character is styled with highlighted/inverted colors.
func renderInputWithCursor(text string, pos int, baseStyle lipgloss.Style) string {
	runes := []rune(text)
	if pos < 0 {
		pos = 0
	}
	if pos > len(runes) {
		pos = len(runes)
	}

	cursorCharStyle := lipgloss.NewStyle().
		Background(colorCyan).
		Foreground(lipgloss.Color("#000000")).
		Bold(true)
	cursorEndBlock := lipgloss.NewStyle().
		Foreground(colorCyan).
		Bold(true).
		Render("█")

	if pos >= len(runes) {
		return baseStyle.Render(text) + cursorEndBlock
	}

	before := baseStyle.Render(string(runes[:pos]))
	underCursor := cursorCharStyle.Render(string(runes[pos]))
	after := baseStyle.Render(string(runes[pos+1:]))

	return before + underCursor + after
}

// sliceInputForWindow calculates a sliding window of maxW runes containing the cursor.
// It returns the visible slice of text and the relative cursor position inside that slice.
func sliceInputForWindow(text string, cursor int, maxW int) (string, int) {
	runes := []rune(text)
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(runes) {
		cursor = len(runes)
	}
	if maxW <= 0 || len(runes) <= maxW {
		return text, cursor
	}

	start := cursor - maxW/2
	if start < 0 {
		start = 0
	}
	end := start + maxW
	if end > len(runes) {
		end = len(runes)
		start = end - maxW
		if start < 0 {
			start = 0
		}
	}

	return string(runes[start:end]), cursor - start
}
