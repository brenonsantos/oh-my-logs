package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/brenoniehues/oh-my-logs/internal/clipboard"
	"github.com/brenoniehues/oh-my-logs/internal/config"
	"github.com/brenoniehues/oh-my-logs/internal/filter"
	"github.com/brenoniehues/oh-my-logs/internal/game"
	"github.com/brenoniehues/oh-my-logs/internal/parser"
	"github.com/brenoniehues/oh-my-logs/internal/record"
	"github.com/brenoniehues/oh-my-logs/internal/serial"
	tea "github.com/charmbracelet/bubbletea"
)

// handleKey dispatches key events based on the current input mode.
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.mode {
	case modeSearch:
		return m.handleSearchKey(msg)
	case modeFilter:
		return m.handleFilterKey(msg)
	case modePortPicker:
		return m.handlePortPickerKey(msg)
	case modeProfilePicker:
		return m.handleProfilePickerKey(msg)
	case modeHelp:
		return m.handleHelpKey(msg)
	case modeGame:
		return m.handleGameKey(msg)
	default:
		return m.handleNormalKey(msg)
	}
}

func (m Model) handleNormalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case keyMatches(msg, m.keys.Quit):
		m.saveSettings()
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
		m.filterInput = m.currentTab().FilterRaw
		return m, nil

	case keyMatches(msg, m.keys.Clear):
		m.buffer.Clear()
		for i := range m.tabs {
			m.tabs[i].Visible = nil
			m.tabs[i].ScrollOffset = 0
			m.tabs[i].SearchMatches = nil
		}
		m.visible = nil
		m.searchMatches = nil
		m.scrollOffset = 0
		m.bookmarks = make(map[uint64]struct{})
		m.message = "Cleared"
		return m, nil

	case keyMatches(msg, m.keys.Pause):
		if m.buffer.Len() == 0 {
			m.activeGame = game.RandomMiniGame()
			m.logsDuringGame = 0
			m.mode = modeGame
			return m, m.activeGame.Init()
		}
		m.paused = !m.paused
		if !m.paused {
			m.rebuildVisible()
			if m.follow {
				m.scrollToBottom()
			}
		}
		return m, nil

	case keyMatches(msg, m.keys.ScrollUp):
		m.selectionStart = -1
		m.selectionEnd = -1
		wasFollow := m.follow
		m.follow = false
		h := m.activeDataHeight()
		if len(m.visible) > 0 {
			if m.selectedRow < 0 {
				if wasFollow {
					m.selectedRow = len(m.visible) - 2
					if m.selectedRow < 0 {
						m.selectedRow = 0
					}
				} else {
					m.selectedRow = m.scrollOffset + h - 1
					if m.selectedRow >= len(m.visible) {
						m.selectedRow = len(m.visible) - 1
					}
					if m.selectedRow > 0 {
						m.selectedRow--
					}
				}
			} else if m.selectedRow > 0 {
				m.selectedRow--
			}
			if m.selectedRow < m.scrollOffset {
				m.scrollOffset = m.selectedRow
			}
		} else {
			m.scrollOffset--
		}
		m.clampScroll()
		if m.splitMode != SplitNone && m.syncScroll {
			m.syncOtherPaneChronologically()
		}
		return m, nil

	case keyMatches(msg, m.keys.ScrollDown):
		m.selectionStart = -1
		m.selectionEnd = -1
		h := m.activeDataHeight()
		if len(m.visible) > 0 {
			if m.selectedRow < 0 {
				m.selectedRow = m.scrollOffset
				if m.selectedRow < len(m.visible)-1 {
					m.selectedRow++
				}
			} else if m.selectedRow < len(m.visible)-1 {
				m.selectedRow++
			}
			if m.selectedRow >= m.scrollOffset+h {
				m.scrollOffset = m.selectedRow - h + 1
			}
			if m.selectedRow >= len(m.visible)-1 {
				m.follow = true
			} else {
				m.follow = false
			}
		} else {
			m.scrollOffset++
			if m.scrollOffset >= len(m.visible)-h {
				m.follow = true
			}
		}
		m.clampScroll()
		if m.splitMode != SplitNone && m.syncScroll {
			m.syncOtherPaneChronologically()
		}
		return m, nil

	case keyMatches(msg, m.keys.PageUp):
		h := m.activeDataHeight()
		m.follow = false
		m.scrollOffset -= h
		if m.selectedRow >= 0 {
			m.selectedRow -= h
			if m.selectedRow < 0 {
				m.selectedRow = 0
			}
		}
		m.clampScroll()
		if m.splitMode != SplitNone && m.syncScroll {
			m.syncOtherPaneChronologically()
		}
		return m, nil

	case keyMatches(msg, m.keys.PageDown):
		h := m.activeDataHeight()
		m.scrollOffset += h
		if m.selectedRow >= 0 {
			m.selectedRow += h
			if m.selectedRow >= len(m.visible) {
				m.selectedRow = len(m.visible) - 1
			}
		}
		if m.scrollOffset >= len(m.visible)-h {
			m.follow = true
		}
		m.clampScroll()
		if m.splitMode != SplitNone && m.syncScroll {
			m.syncOtherPaneChronologically()
		}
		return m, nil

	case keyMatches(msg, m.keys.GoToBottom):
		m.follow = true
		m.scrollToBottom()
		m.selectedRow = -1
		m.selectionStart = -1
		m.selectionEnd = -1
		if m.splitMode != SplitNone && m.syncScroll {
			m.syncOtherPaneChronologically()
		}
		return m, nil

	case keyMatches(msg, m.keys.GoToTop):
		if m.buffer.Len() == 0 {
			m.activeGame = game.RandomMiniGame()
			m.logsDuringGame = 0
			m.mode = modeGame
			return m, m.activeGame.Init()
		}
		m.follow = false
		m.scrollOffset = 0
		if len(m.visible) > 0 {
			m.selectedRow = 0
		}
		if m.splitMode != SplitNone && m.syncScroll {
			m.syncOtherPaneChronologically()
		}
		return m, nil

	case keyMatches(msg, m.keys.Game):
		m.activeGame = game.RandomMiniGame()
		m.logsDuringGame = 0
		m.mode = modeGame
		return m, m.activeGame.Init()

	case keyMatches(msg, m.keys.SaveLog):
		return m, m.cmdSaveLog()

	case keyMatches(msg, m.keys.Port):
		ports, _ := serial.ListPorts()
		m.portList = ports
		m.portCursor = 0
		for i, p := range ports {
			if p == m.serialCfg.Port {
				m.portCursor = i
				break
			}
		}
		for i, b := range m.baudList {
			if b == m.serialCfg.Baud {
				m.baudCursor = i
				break
			}
		}
		m.portPickerSection = 0
		m.mode = modePortPicker
		return m, nil

	case keyMatches(msg, m.keys.ProfileSwitch):
		paths := config.FindAllProfilePaths(m.appConfig)
		seen := make(map[string]bool)
		var items []ProfileItem
		items = append(items, ProfileItem{Name: "Raw", Path: ""})
		seen["raw"] = true
		for _, p := range paths {
			prof, err := parser.LoadProfile(p)
			if err != nil {
				continue
			}
			normName := strings.ToLower(strings.TrimSpace(prof.Name))
			if normName == "" {
				normName = strings.ToLower(strings.TrimSuffix(filepath.Base(p), filepath.Ext(p)))
			}
			if seen[normName] {
				continue
			}
			seen[normName] = true
			items = append(items, ProfileItem{Name: prof.Name, Path: p})
		}
		m.profileList = items
		m.profileCursor = 0
		for i, item := range items {
			if m.profile != nil && strings.EqualFold(item.Name, m.profile.Name) {
				m.profileCursor = i
				break
			} else if m.profile == nil && strings.EqualFold(item.Name, "raw") {
				m.profileCursor = i
				break
			}
		}
		m.mode = modeProfilePicker
		return m, nil

	case keyMatches(msg, m.keys.Reconnect):
		if m.serialCfg.Port == "" {
			m.message = "No port configured — press p to pick one"
			return m, nil
		}
		if m.source != nil {
			m.source.Stop()
			m.source = nil
		}
		m.connState = ConnDisconnected
		return m, connectCmd(m.serialCfg)

	case keyMatches(msg, m.keys.ToggleTimestamp):
		m.showTimestamp = !m.showTimestamp
		m.saveSettings()
		if m.showTimestamp {
			m.message = fmt.Sprintf("⏱ Timestamp ON (%s)", m.tsField)
		} else {
			m.message = "⏱ Timestamp OFF"
		}
		return m, nil

	case keyMatches(msg, m.keys.Help):
		m.mode = modeHelp
		return m, nil

	case keyMatches(msg, m.keys.NextMatch):
		m.nextSearchMatch()
		return m, nil

	case keyMatches(msg, m.keys.PrevMatch):
		m.prevSearchMatch()
		return m, nil

	case keyMatches(msg, m.keys.NextTab):
		if len(m.tabs) > 1 {
			m.switchTab((m.activeTabIdx() + 1) % len(m.tabs))
		}
		return m, nil

	case keyMatches(msg, m.keys.PrevTab):
		if len(m.tabs) > 1 {
			m.switchTab((m.activeTabIdx() - 1 + len(m.tabs)) % len(m.tabs))
		}
		return m, nil

	case keyMatches(msg, m.keys.SplitVertical):
		m.toggleSplit(SplitVertical)
		return m, nil

	case keyMatches(msg, m.keys.SplitHorizontal):
		m.toggleSplit(SplitHorizontal)
		return m, nil

	case keyMatches(msg, m.keys.SwitchPane):
		m.switchPaneFocus()
		return m, nil

	case keyMatches(msg, m.keys.ToggleSyncScroll):
		m.toggleSyncScroll()
		return m, nil

	case keyMatches(msg, m.keys.NewTab):
		return m.handleCreateNewTab()

	case keyMatches(msg, m.keys.CloseTab):
		return m.handleCloseActiveTab()

	case keyMatches(msg, m.keys.ToggleBookmark):
		return m.handleToggleBookmark()

	case keyMatches(msg, m.keys.NextBookmark):
		return m.handleNextBookmark()

	case keyMatches(msg, m.keys.PrevBookmark):
		return m.handlePrevBookmark()

	case keyMatches(msg, m.keys.BookmarksOnly):
		return m.handleToggleBookmarksOnly()

	case keyMatches(msg, m.keys.CopyRow), keyMatches(msg, m.keys.CopyRaw):
		return m.handleCopyKey()

	case keyMatches(msg, m.keys.SelectUp), msg.String() == "shift+up":
		return m.handleSelectUp()

	case keyMatches(msg, m.keys.SelectDown), msg.String() == "shift+down":
		return m.handleSelectDown()

	case keyMatches(msg, m.keys.Cancel):
		m.selectedRow = -1
		m.selectionStart = -1
		m.selectionEnd = -1
		m.searchInput = ""
		m.searchMatches = nil
		return m, nil

	default:
		s := msg.String()
		if len(s) == 1 && s >= "1" && s <= "9" {
			idx := int(s[0] - '1')
			if idx < len(m.tabs) {
				m.switchTab(idx)
				return m, nil
			}
		}
	}

	return m, nil
}

func (m Model) handleSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case keyMatches(msg, m.keys.Cancel):
		m.mode = modeNormal
		return m, nil

	case keyMatches(msg, m.keys.Confirm):
		if len(m.searchMatches) > 0 {
			m.nextSearchMatch()
		} else {
			m.mode = modeNormal
		}
		return m, nil

	case msg.Type == tea.KeyCtrlV || msg.String() == "ctrl+v":
		clipText, err := clipboard.Read()
		if err == nil && clipText != "" {
			clean := strings.ReplaceAll(strings.ReplaceAll(clipText, "\r", ""), "\n", " ")
			m.searchInput += strings.TrimSpace(clean)
			m.runSearch()
		}
		return m, nil

	case msg.String() == "down":
		m.nextSearchMatch()
		return m, nil

	case msg.String() == "up":
		m.prevSearchMatch()
		return m, nil

	default:
		m.searchInput = handleTextInput(m.searchInput, msg)
		m.runSearch()
	}
	return m, nil
}

func (m Model) handleFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case keyMatches(msg, m.keys.Cancel):
		m.mode = modeNormal
		m.filterInput = m.currentTab().FilterRaw
		return m, nil

	case keyMatches(msg, m.keys.Confirm):
		f, err := filter.New(m.filterInput)
		if err != nil {
			m.message = fmt.Sprintf("filter error: %v", err)
		} else {
			m.activeFilter = f
			cur := m.currentTab()
			cur.Filter = f
			cur.FilterRaw = m.filterInput
			if cur.Name == "" || strings.HasPrefix(cur.Name, "Tab ") {
				if m.filterInput != "" {
					cur.Name = m.filterInput
				}
			}
			m.rebuildVisible()
			cur.Visible = m.visible
			if m.follow {
				m.scrollToBottom()
				cur.ScrollOffset = m.scrollOffset
			}
		}
		m.mode = modeNormal
		return m, nil

	case msg.Type == tea.KeyCtrlV || msg.String() == "ctrl+v":
		clipText, err := clipboard.Read()
		if err == nil && clipText != "" {
			clean := strings.ReplaceAll(strings.ReplaceAll(clipText, "\r", ""), "\n", " ")
			m.filterInput += strings.TrimSpace(clean)
		}
		return m, nil

	default:
		m.filterInput = handleTextInput(m.filterInput, msg)
	}
	return m, nil
}

// handlePortPickerKey handles navigation and selection inside the port & baud modal.
func (m Model) handlePortPickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case keyMatches(msg, m.keys.Cancel):
		m.mode = modeNormal
		return m, nil

	case msg.String() == "left" || msg.String() == "h":
		if len(m.baudList) > 0 {
			if m.baudCursor > 0 {
				m.baudCursor--
			} else {
				m.baudCursor = len(m.baudList) - 1
			}
		}
		return m, nil

	case msg.String() == "right" || msg.String() == "l":
		if len(m.baudList) > 0 {
			if m.baudCursor < len(m.baudList)-1 {
				m.baudCursor++
			} else {
				m.baudCursor = 0
			}
		}
		return m, nil

	case keyMatches(msg, m.keys.ScrollUp):
		if m.portCursor > 0 {
			m.portCursor--
		}
		return m, nil

	case keyMatches(msg, m.keys.ScrollDown):
		if m.portCursor < len(m.portList)-1 {
			m.portCursor++
		}
		return m, nil

	case keyMatches(msg, m.keys.Confirm):
		var port string
		if len(m.portList) > 0 && m.portCursor < len(m.portList) {
			port = m.portList[m.portCursor]
		} else {
			port = m.serialCfg.Port
		}
		if port == "" {
			m.message = "No port selected"
			m.mode = modeNormal
			return m, nil
		}

		baud := m.serialCfg.Baud
		if len(m.baudList) > 0 && m.baudCursor < len(m.baudList) {
			baud = m.baudList[m.baudCursor]
		}

		if m.source != nil {
			m.source.Stop()
			m.source = nil
		}
		m.connState = ConnDisconnected
		cfg := m.serialCfg
		cfg.Port = port
		cfg.Baud = baud
		m.serialCfg = cfg
		m.mode = modeNormal
		m.saveSettings()
		return m, connectCmd(cfg)
	}
	return m, nil
}

// handleProfilePickerKey handles navigation and selection inside the profile switcher.
func (m Model) handleProfilePickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case keyMatches(msg, m.keys.Cancel):
		m.mode = modeNormal
		return m, nil

	case keyMatches(msg, m.keys.ScrollUp):
		if m.profileCursor > 0 {
			m.profileCursor--
		}
		return m, nil

	case keyMatches(msg, m.keys.ScrollDown):
		if m.profileCursor < len(m.profileList)-1 {
			m.profileCursor++
		}
		return m, nil

	case keyMatches(msg, m.keys.Confirm):
		if len(m.profileList) == 0 {
			m.mode = modeNormal
			return m, nil
		}
		selected := m.profileList[m.profileCursor]
		if selected.Path == "" {
			// Built-in Raw parser
			m.profile = nil
			m.parser = parser.NewRawParser()
			m.columns = []record.Column{{Field: "message", Title: "Message", Width: 0}}
		} else {
			prof, err := parser.LoadProfile(selected.Path)
			if err != nil {
				m.message = fmt.Sprintf("profile error: %v", err)
				m.mode = modeNormal
				return m, nil
			}
			newParser, err := prof.BuildParser()
			if err != nil {
				m.message = fmt.Sprintf("parser error: %v", err)
				m.mode = modeNormal
				return m, nil
			}
			m.profile = prof
			m.parser = newParser
			m.columns = prof.ToColumns()

			// Update timestamp settings if configured
			if prof.Ingest.Timestamp.Enabled {
				m.tsField = prof.Ingest.Timestamp.TimestampField()
				m.tsFormat = prof.Ingest.Timestamp.TimestampFormat()
				m.showTimestamp = true
			} else {
				for _, col := range prof.Columns {
					if col.Style == "timestamp" || col.Field == "time" || col.Field == "timestamp" {
						m.tsField = col.Field
						m.showTimestamp = true
						break
					}
				}
			}
		}

		// Re-parse all existing records in the buffer using the new parser!
		m.buffer.Transform(func(old record.Record) record.Record {
			newRec, _ := m.parser.Parse(old.Raw)
			newRec.ID = old.ID
			if newRec.Fields == nil {
				newRec.Fields = make(map[string]string)
			}
			// Preserve earlier recorded arrival timestamp
			prevTS := old.Fields[m.tsField]
			if prevTS == "" {
				prevTS = old.Fields["_ts"]
			}
			if prevTS == "" {
				prevTS = old.Fields["time"]
			}
			if prevTS != "" {
				if newRec.Fields[m.tsField] == "" {
					newRec.Fields[m.tsField] = prevTS
				}
				if newRec.Fields["_ts"] == "" {
					newRec.Fields["_ts"] = prevTS
				}
			}
			return newRec
		})

		m.rebuildAllTabs()
		m.mode = modeNormal
		m.saveSettings()
		m.message = fmt.Sprintf("Profile: %s", selected.Name)
		return m, nil
	}
	return m, nil
}

// handleHelpKey handles input while the help modal is displayed.
func (m Model) handleHelpKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case keyMatches(msg, m.keys.Help),
		keyMatches(msg, m.keys.Cancel),
		keyMatches(msg, m.keys.Confirm),
		msg.String() == "q":
		m.mode = modeNormal
		return m, nil
	}
	return m, nil
}

// handleGameKey handles input while an easter egg mini-game is active.
func (m Model) handleGameKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case keyMatches(msg, m.keys.Cancel), msg.String() == "q", msg.String() == "esc":
		m.mode = modeNormal
		m.activeGame = nil
		if m.logsDuringGame > 0 {
			m.message = fmt.Sprintf("Returned to logs (%d new records received)", m.logsDuringGame)
		}
		m.logsDuringGame = 0
		return m, nil
	default:
		if m.activeGame != nil {
			var cmd tea.Cmd
			m.activeGame, cmd = m.activeGame.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

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

func (m Model) handleToggleBookmark() (tea.Model, tea.Cmd) {
	if len(m.visible) == 0 {
		return m, nil
	}
	if m.bookmarks == nil {
		m.bookmarks = make(map[uint64]struct{})
	}

	// 1. Multi-row selection active (via mouse drag or Shift+Up/Down)
	if m.selectionStart >= 0 && m.selectionEnd >= 0 && m.selectionStart != m.selectionEnd {
		start := m.selectionStart
		end := m.selectionEnd
		if start > end {
			start, end = end, start
		}
		if start < 0 {
			start = 0
		}
		if end >= len(m.visible) {
			end = len(m.visible) - 1
		}
		if start <= end {
			allBookmarked := true
			for i := start; i <= end; i++ {
				rec := m.visible[i]
				if rec.ID == 0 {
					continue
				}
				if _, ok := m.bookmarks[rec.ID]; !ok {
					allBookmarked = false
					break
				}
			}

			count := end - start + 1
			if allBookmarked {
				for i := start; i <= end; i++ {
					delete(m.bookmarks, m.visible[i].ID)
				}
				m.message = fmt.Sprintf("Unpinned %d rows (%d-%d)", count, start+1, end+1)
			} else {
				for i := start; i <= end; i++ {
					rec := m.visible[i]
					if rec.ID == 0 {
						m.nextRecordID++
						rec.ID = m.nextRecordID
						m.visible[i] = rec
					}
					m.bookmarks[rec.ID] = struct{}{}
				}
				m.message = fmt.Sprintf("★ Pinned %d rows (%d-%d)", count, start+1, end+1)
			}

			if m.bookmarkedOnly {
				m.rebuildVisible()
				m.selectionStart = -1
				m.selectionEnd = -1
				m.selectedRow = -1
				m.clampScroll()
			}
			return m, nil
		}
	}

	// 2. Single row toggle
	rowIdx := m.selectedRow
	if rowIdx < 0 || rowIdx >= len(m.visible) {
		rowIdx = m.scrollOffset
		if rowIdx >= len(m.visible) {
			rowIdx = len(m.visible) - 1
		}
		m.selectedRow = rowIdx
	}
	if rowIdx >= 0 && rowIdx < len(m.visible) {
		rec := m.visible[rowIdx]
		if rec.ID == 0 {
			m.nextRecordID++
			rec.ID = m.nextRecordID
			m.visible[rowIdx] = rec
		}
		if _, exists := m.bookmarks[rec.ID]; exists {
			delete(m.bookmarks, rec.ID)
			m.message = fmt.Sprintf("Unpinned row %d", rowIdx+1)
		} else {
			m.bookmarks[rec.ID] = struct{}{}
			m.message = fmt.Sprintf("★ Pinned row %d", rowIdx+1)
		}
		if m.bookmarkedOnly {
			m.rebuildVisible()
			if m.selectedRow >= len(m.visible) {
				m.selectedRow = len(m.visible) - 1
			}
			m.clampScroll()
		}
	}
	return m, nil
}

func (m Model) handleNextBookmark() (tea.Model, tea.Cmd) {
	if len(m.bookmarks) == 0 || len(m.visible) == 0 {
		m.message = "No bookmarks"
		return m, nil
	}
	start := m.selectedRow
	if start < 0 {
		start = m.scrollOffset - 1
	}
	found := -1
	for i := start + 1; i < len(m.visible); i++ {
		if _, ok := m.bookmarks[m.visible[i].ID]; ok {
			found = i
			break
		}
	}
	if found < 0 {
		for i := 0; i <= start && i < len(m.visible); i++ {
			if _, ok := m.bookmarks[m.visible[i].ID]; ok {
				found = i
				break
			}
		}
	}
	if found >= 0 {
		m.selectedRow = found
		m.follow = false
		h := m.activeDataHeight()
		if m.selectedRow < m.scrollOffset || m.selectedRow >= m.scrollOffset+h {
			m.scrollOffset = m.selectedRow - h/2
		}
		m.clampScroll()
		if m.splitMode != SplitNone && m.syncScroll {
			m.syncOtherPaneChronologically()
		}
		m.message = fmt.Sprintf("★ Bookmark at row %d", m.selectedRow+1)
	} else {
		m.message = "No bookmarks in visible view"
	}
	return m, nil
}

func (m Model) handlePrevBookmark() (tea.Model, tea.Cmd) {
	if len(m.bookmarks) == 0 || len(m.visible) == 0 {
		m.message = "No bookmarks"
		return m, nil
	}
	start := m.selectedRow
	if start < 0 {
		start = m.scrollOffset
	}
	found := -1
	for i := start - 1; i >= 0; i-- {
		if _, ok := m.bookmarks[m.visible[i].ID]; ok {
			found = i
			break
		}
	}
	if found < 0 {
		for i := len(m.visible) - 1; i >= start && i >= 0; i-- {
			if _, ok := m.bookmarks[m.visible[i].ID]; ok {
				found = i
				break
			}
		}
	}
	if found >= 0 {
		m.selectedRow = found
		m.follow = false
		h := m.activeDataHeight()
		if m.selectedRow < m.scrollOffset || m.selectedRow >= m.scrollOffset+h {
			m.scrollOffset = m.selectedRow - h/2
		}
		m.clampScroll()
		if m.splitMode != SplitNone && m.syncScroll {
			m.syncOtherPaneChronologically()
		}
		m.message = fmt.Sprintf("★ Bookmark at row %d", m.selectedRow+1)
	} else {
		m.message = "No bookmarks in visible view"
	}
	return m, nil
}

func (m Model) handleToggleBookmarksOnly() (tea.Model, tea.Cmd) {
	m.bookmarkedOnly = !m.bookmarkedOnly
	m.rebuildVisible()
	m.selectedRow = -1
	m.selectionStart = -1
	m.selectionEnd = -1
	m.scrollOffset = 0
	m.clampScroll()
	if m.bookmarkedOnly {
		m.message = fmt.Sprintf("★ Showing %d bookmarked row(s)", len(m.visible))
	} else {
		m.message = "Showing all rows"
	}
	return m, nil
}

