package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/brenoniehues/oh-my-logs/internal/filter"
	"github.com/brenoniehues/oh-my-logs/internal/record"
	"github.com/brenoniehues/oh-my-logs/internal/serial"
	tea "github.com/charmbracelet/bubbletea"
)

// listenToSource returns a command that waits for the next line from the source.
func listenToSource(src serial.Source) tea.Cmd {
	return func() tea.Msg {
		line, ok := <-src.Lines()
		if !ok {
			return ConnStateMsg{State: ConnDisconnected, Detail: "source closed"}
		}
		return lineMsg(line)
	}
}

// listenToSourceErrors returns a command that reads the next error from the source.
func listenToSourceErrors(src serial.Source) tea.Cmd {
	return func() tea.Msg {
		err, ok := <-src.Errors()
		if !ok {
			return nil
		}
		return ErrorMsg{Err: err}
	}
}

// lineMsg is an internal message carrying a raw line from the source.
type lineMsg string

// Update is the Bubble Tea update function.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	// ── Terminal resize ──────────────────────────────────────────────────────
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.recalcLayout()
		m.clampScroll()
		return m, nil

	// ── New line from source ─────────────────────────────────────────────────
	case lineMsg:
		r, _ := m.parser.Parse(string(msg))
		m.buffer.Add(r)
		m.rebuildVisible()
		if m.follow && !m.paused {
			m.scrollToBottom()
		}
		// Re-arm the listener.
		return m, listenToSource(m.source)

	// ── Source error ─────────────────────────────────────────────────────────
	case ErrorMsg:
		m.message = msg.Err.Error()
		return m, listenToSourceErrors(m.source)

	// ── Connection state ─────────────────────────────────────────────────────
	case ConnStateMsg:
		m.connState = msg.State
		m.connDetail = msg.Detail
		return m, nil

	// ── Key events ───────────────────────────────────────────────────────────
	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

// handleKey dispatches key events based on the current input mode.
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.mode {
	case modeSearch:
		return m.handleSearchKey(msg)
	case modeFilter:
		return m.handleFilterKey(msg)
	default:
		return m.handleNormalKey(msg)
	}
}

func (m Model) handleNormalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case keyMatches(msg, m.keys.Quit):
		if m.source != nil {
			m.source.Stop()
		}
		return m, tea.Quit

	case keyMatches(msg, m.keys.Search):
		m.mode = modeSearch
		m.searchInput = ""
		m.searchMatches = nil
		m.searchCursor = 0
		return m, nil

	case keyMatches(msg, m.keys.Filter):
		m.mode = modeFilter
		return m, nil

	case keyMatches(msg, m.keys.Clear):
		m.buffer.Clear()
		m.visible = nil
		m.searchMatches = nil
		m.scrollOffset = 0
		m.message = "Cleared"
		return m, nil

	case keyMatches(msg, m.keys.Pause):
		m.paused = !m.paused
		if !m.paused {
			m.rebuildVisible()
			if m.follow {
				m.scrollToBottom()
			}
		}
		return m, nil

	case keyMatches(msg, m.keys.ScrollUp):
		m.follow = false
		m.scrollOffset--
		m.clampScroll()
		return m, nil

	case keyMatches(msg, m.keys.ScrollDown):
		m.scrollOffset++
		if m.scrollOffset >= len(m.visible)-m.tableHeight {
			m.follow = true
		}
		m.clampScroll()
		return m, nil

	case keyMatches(msg, m.keys.PageUp):
		m.follow = false
		m.scrollOffset -= m.tableHeight
		m.clampScroll()
		return m, nil

	case keyMatches(msg, m.keys.PageDown):
		m.scrollOffset += m.tableHeight
		if m.scrollOffset >= len(m.visible)-m.tableHeight {
			m.follow = true
		}
		m.clampScroll()
		return m, nil

	case keyMatches(msg, m.keys.GoToBottom):
		m.follow = true
		m.scrollToBottom()
		return m, nil

	case keyMatches(msg, m.keys.GoToTop):
		m.follow = false
		m.scrollOffset = 0
		return m, nil

	case keyMatches(msg, m.keys.SaveLog):
		return m, m.cmdSaveLog()

	case keyMatches(msg, m.keys.Reconnect):
		// Reconnect is handled externally; just emit a message.
		m.message = "Reconnect not yet implemented in this mode."
		return m, nil

	case keyMatches(msg, m.keys.NextMatch):
		m.nextSearchMatch()
		return m, nil

	case keyMatches(msg, m.keys.PrevMatch):
		m.prevSearchMatch()
		return m, nil
	}

	return m, nil
}

func (m Model) handleSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case keyMatches(msg, m.keys.Cancel):
		m.mode = modeNormal
		m.searchInput = ""
		m.searchMatches = nil
		return m, nil

	case keyMatches(msg, m.keys.Confirm):
		m.runSearch()
		m.mode = modeNormal
		return m, nil

	default:
		m.searchInput = handleTextInput(m.searchInput, msg)
	}
	return m, nil
}

func (m Model) handleFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case keyMatches(msg, m.keys.Cancel):
		m.mode = modeNormal
		return m, nil

	case keyMatches(msg, m.keys.Confirm):
		f, err := filter.New(m.filterInput)
		if err != nil {
			m.message = fmt.Sprintf("filter error: %v", err)
		} else {
			m.activeFilter = f
			m.rebuildVisible()
			if m.follow {
				m.scrollToBottom()
			}
		}
		m.mode = modeNormal
		return m, nil

	default:
		m.filterInput = handleTextInput(m.filterInput, msg)
	}
	return m, nil
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func keyMatches(msg tea.KeyMsg, b interface{ Keys() []string }) bool {
	for _, k := range b.Keys() {
		if msg.String() == k {
			return true
		}
	}
	return false
}

// handleTextInput processes printable key presses and backspace for text fields.
func handleTextInput(current string, msg tea.KeyMsg) string {
	switch msg.Type {
	case tea.KeyBackspace, tea.KeyDelete:
		if len(current) > 0 {
			runes := []rune(current)
			return string(runes[:len(runes)-1])
		}
	case tea.KeyRunes:
		for _, r := range msg.Runes {
			if unicode.IsPrint(r) {
				current += string(r)
			}
		}
	case tea.KeySpace:
		current += " "
	}
	return current
}

// rebuildVisible refilters the buffer and updates m.visible.
func (m *Model) rebuildVisible() {
	all := m.buffer.All()
	if m.activeFilter == nil || m.activeFilter.Empty() {
		m.visible = all
		return
	}
	out := make([]record.Record, 0, len(all))
	for _, r := range all {
		if m.activeFilter.Matches(r) {
			out = append(out, r)
		}
	}
	m.visible = out
}

// runSearch finds all visible rows matching the search string.
func (m *Model) runSearch() {
	m.searchMatches = nil
	q := strings.ToLower(m.searchInput)
	if q == "" {
		return
	}
	for i, r := range m.visible {
		for _, v := range r.Fields {
			if strings.Contains(strings.ToLower(v), q) {
				m.searchMatches = append(m.searchMatches, i)
				break
			}
		}
	}
	m.searchCursor = 0
	if len(m.searchMatches) > 0 {
		m.scrollOffset = m.searchMatches[0]
		m.clampScroll()
	}
}

func (m *Model) nextSearchMatch() {
	if len(m.searchMatches) == 0 {
		return
	}
	m.searchCursor = (m.searchCursor + 1) % len(m.searchMatches)
	m.scrollOffset = m.searchMatches[m.searchCursor]
	m.clampScroll()
}

func (m *Model) prevSearchMatch() {
	if len(m.searchMatches) == 0 {
		return
	}
	m.searchCursor = (m.searchCursor - 1 + len(m.searchMatches)) % len(m.searchMatches)
	m.scrollOffset = m.searchMatches[m.searchCursor]
	m.clampScroll()
}

func (m *Model) scrollToBottom() {
	if len(m.visible) > m.tableHeight {
		m.scrollOffset = len(m.visible) - m.tableHeight
	} else {
		m.scrollOffset = 0
	}
}

func (m *Model) clampScroll() {
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
	maxOffset := len(m.visible) - m.tableHeight
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.scrollOffset > maxOffset {
		m.scrollOffset = maxOffset
	}
}

func (m *Model) recalcLayout() {
	// Reserve: 1 title + 1 header + 1 status + 1 keys = 4 fixed rows.
	m.tableHeight = m.height - 4
	if m.tableHeight < 1 {
		m.tableHeight = 1
	}
}

// cmdSaveLog saves all raw lines in the buffer to a timestamped file.
func (m *Model) cmdSaveLog() tea.Cmd {
	return func() tea.Msg {
		all := m.buffer.All()
		name := fmt.Sprintf("oml-%s.log", time.Now().Format("2006-01-02T15-04-05"))
		path := filepath.Join(".", name)
		f, err := os.Create(path)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("save log: %w", err)}
		}
		defer f.Close()
		for _, r := range all {
			fmt.Fprintln(f, r.Raw)
		}
		return ErrorMsg{Err: fmt.Errorf("saved %d records to %s", len(all), path)}
	}
}
