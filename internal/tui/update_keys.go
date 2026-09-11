package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/brenoniehues/oh-my-logs/internal/config"
	"github.com/brenoniehues/oh-my-logs/internal/filter"
	"github.com/brenoniehues/oh-my-logs/internal/game"
	"github.com/brenoniehues/oh-my-logs/internal/parser"
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
	case modeFilterPresets:
		return m.handleFilterPresetsKey(msg)
	case modeSavePresetPrompt:
		return m.handleSavePresetPromptKey(msg)
	case modePortPicker:
		return m.handlePortPickerKey(msg)
	case modeProfilePicker:
		return m.handleProfilePickerKey(msg)
	case modeHelp:
		return m.handleHelpKey(msg)
	case modeGame:
		return m.handleGameKey(msg)
	case modeTXInput:
		return m.handleTXKey(msg)
	case modeSettings:
		return m.handleSettingsKey(msg)
	case modeFilePicker:
		return m.handleFilePickerKey(msg)
	case modeRowDetail:
		return m.handleRowDetailKey(msg)
	default:
		return m.handleNormalKey(msg)
	}
}

func (m Model) handleNormalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case keyMatches(msg, m.keys.Quit):
		m.saveSettings()
		m.StopDiskLogger()
		if m.source != nil {
			m.source.Stop()
		}
		return m, tea.Quit

	case keyMatches(msg, m.keys.Search):
		m.mode = modeSearch
		m.searchInput.Clear()
		m.searchMatches = nil
		m.searchCursor = 0
		return m, nil

	case keyMatches(msg, m.keys.Filter):
		m.mode = modeFilter
		m.filterInput.SetText(m.currentTab().FilterRaw)
		m.filterInput.ResetHistoryCursor()
		return m, nil

	case keyMatches(msg, m.keys.SendTX):
		m.mode = modeTXInput
		m.txInput.Reset()
		return m, nil

	case keyMatches(msg, m.keys.FilterPresets):
		m.loadFilters()
		m.presetCursor = 0
		m.mode = modeFilterPresets
		return m, nil

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

	case keyMatches(msg, m.keys.Settings):
		m.mode = modeSettings
		return m, nil

	case keyMatches(msg, m.keys.ViewDetail):
		return m.openRowDetail()

	case keyMatches(msg, m.keys.Help):
		m.mode = modeHelp
		return m, nil

	case keyMatches(msg, m.keys.Game):
		m.activeGame = game.RandomMiniGame()
		m.logsDuringGame = 0
		m.mode = modeGame
		return m, m.activeGame.Init()

	case keyMatches(msg, m.keys.Cancel):
		if m.charSelStart >= 0 && m.charSelEnd >= 0 && m.charSelStart != m.charSelEnd {
			m.charSelStart = -1
			m.charSelEnd = -1
			return m, nil
		}
		m.selectedRow = -1
		m.selectionStart = -1
		m.selectionEnd = -1
		m.cursorCol = -1
		m.charSelStart = -1
		m.charSelEnd = -1
		m.searchInput.Clear()
		m.searchMatches = nil
		return m, nil
	}

	if newM, cmd, ok := m.handleNavigationKey(msg); ok {
		return newM, cmd
	}
	if newM, cmd, ok := m.handleTabKey(msg); ok {
		return newM, cmd
	}
	if newM, cmd, ok := m.handleSplitKey(msg); ok {
		return newM, cmd
	}
	if newM, cmd, ok := m.handleActionKey(msg); ok {
		return newM, cmd
	}

	return m, nil
}

func (m Model) openRowDetail() (Model, tea.Cmd) {
	totalRows := len(m.visible)
	if m.splitMode != SplitNone {
		t := m.currentTabForPane(m.activePane)
		if t != nil {
			totalRows = len(t.Visible)
		}
	}
	if totalRows == 0 {
		m.message = "No log entry to inspect"
		return m, nil
	}

	if m.splitMode != SplitNone {
		t := m.currentTabForPane(m.activePane)
		if t != nil && t.SelectedRow < 0 {
			if t.Follow && len(t.Visible) > 0 {
				t.SelectedRow = len(t.Visible) - 1
			} else if t.ScrollOffset >= 0 && t.ScrollOffset < len(t.Visible) {
				t.SelectedRow = t.ScrollOffset
			} else {
				t.SelectedRow = 0
			}
		}
	} else {
		if m.selectedRow < 0 {
			if m.follow && len(m.visible) > 0 {
				m.selectedRow = len(m.visible) - 1
			} else if m.scrollOffset >= 0 && m.scrollOffset < len(m.visible) {
				m.selectedRow = m.scrollOffset
			} else {
				m.selectedRow = 0
			}
		}
	}

	m.detailScrollOffset = 0
	m.mode = modeRowDetail
	return m, nil
}

func (m Model) handleNavigationKey(msg tea.KeyMsg) (Model, tea.Cmd, bool) {
	switch {
	case keyMatches(msg, m.keys.ScrollUp):
		m.selectionStart = -1
		m.selectionEnd = -1
		m.charSelStart = -1
		m.charSelEnd = -1
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
			if m.selectedRow >= 0 && m.selectedRow < len(m.visible) && m.cursorCol >= 0 {
				rowLen := len([]rune(m.selectedRowPlainText(m.selectedRow)))
				if rowLen > 0 && m.cursorCol >= rowLen {
					m.cursorCol = rowLen - 1
				}
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
		return m, nil, true

	case keyMatches(msg, m.keys.ScrollDown):
		m.selectionStart = -1
		m.selectionEnd = -1
		m.charSelStart = -1
		m.charSelEnd = -1
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
			if m.selectedRow >= 0 && m.selectedRow < len(m.visible) && m.cursorCol >= 0 {
				rowLen := len([]rune(m.selectedRowPlainText(m.selectedRow)))
				if rowLen > 0 && m.cursorCol >= rowLen {
					m.cursorCol = rowLen - 1
				}
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
		return m, nil, true

	case keyMatches(msg, m.keys.ScrollLeft):
		if m.scrollX > 0 {
			m.scrollX -= 8
			m.clampScrollX()
		}
		return m, nil, true

	case keyMatches(msg, m.keys.ScrollRight):
		m.scrollX += 8
		m.clampScrollX()
		return m, nil, true

	case keyMatches(msg, m.keys.CursorLeft):
		if len(m.visible) == 0 {
			return m, nil, true
		}
		if m.selectedRow < 0 {
			m.selectedRow = m.scrollOffset
			if m.selectedRow >= len(m.visible) {
				m.selectedRow = len(m.visible) - 1
			}
			m.cursorCol = 0
		} else if m.cursorCol > 0 {
			m.cursorCol--
		} else if m.cursorCol < 0 {
			m.cursorCol = 0
		}
		m.charSelStart = -1
		m.charSelEnd = -1
		if m.cursorCol < m.scrollX+2 {
			m.scrollX = m.cursorCol - 2
			m.clampScrollX()
		}
		return m, nil, true

	case keyMatches(msg, m.keys.CursorRight):
		if len(m.visible) == 0 {
			return m, nil, true
		}
		if m.selectedRow < 0 {
			m.selectedRow = m.scrollOffset
			if m.selectedRow >= len(m.visible) {
				m.selectedRow = len(m.visible) - 1
			}
			m.cursorCol = 0
		} else {
			rowLen := len([]rune(m.selectedRowPlainText(m.selectedRow)))
			if rowLen > 0 && m.cursorCol >= rowLen-1 {
				m.cursorCol = rowLen - 1
			} else if m.cursorCol < 0 {
				m.cursorCol = 1
			} else {
				m.cursorCol++
			}
		}
		m.charSelStart = -1
		m.charSelEnd = -1
		availW := m.tableWidth() - 3
		if m.splitMode == SplitVertical {
			availW = (m.tableWidth() - 1) / 2 - 3
		}
		if availW > 4 && m.cursorCol >= m.scrollX+availW-2 {
			m.scrollX = m.cursorCol - availW + 3
			m.clampScrollX()
		}
		return m, nil, true

	case keyMatches(msg, m.keys.CharSelectLeft), msg.String() == "shift+left":
		if len(m.visible) == 0 {
			return m, nil, true
		}
		if m.selectedRow < 0 {
			m.selectedRow = m.scrollOffset
			if m.selectedRow >= len(m.visible) {
				m.selectedRow = len(m.visible) - 1
			}
			m.cursorCol = 0
		}
		if m.cursorCol < 0 {
			m.cursorCol = 0
		}
		if m.charSelStart < 0 {
			m.charSelStart = m.cursorCol
		}
		if m.cursorCol > 0 {
			m.cursorCol--
		}
		m.charSelEnd = m.cursorCol
		if m.cursorCol < m.scrollX+2 {
			m.scrollX = m.cursorCol - 2
			m.clampScrollX()
		}
		return m, nil, true

	case keyMatches(msg, m.keys.CharSelectRight), msg.String() == "shift+right":
		if len(m.visible) == 0 {
			return m, nil, true
		}
		if m.selectedRow < 0 {
			m.selectedRow = m.scrollOffset
			if m.selectedRow >= len(m.visible) {
				m.selectedRow = len(m.visible) - 1
			}
			m.cursorCol = 0
		}
		if m.cursorCol < 0 {
			m.cursorCol = 0
		}
		if m.charSelStart < 0 {
			m.charSelStart = m.cursorCol
		}
		rowLen := len([]rune(m.selectedRowPlainText(m.selectedRow)))
		if rowLen > 0 && m.cursorCol >= rowLen {
			m.cursorCol = rowLen
		} else {
			m.cursorCol++
		}
		m.charSelEnd = m.cursorCol
		availW := m.tableWidth() - 3
		if m.splitMode == SplitVertical {
			availW = (m.tableWidth() - 1) / 2 - 3
		}
		if availW > 4 && m.cursorCol >= m.scrollX+availW-2 {
			m.scrollX = m.cursorCol - availW + 3
			m.clampScrollX()
		}
		return m, nil, true

	case keyMatches(msg, m.keys.PageUp):
		m.charSelStart = -1
		m.charSelEnd = -1
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
		return m, nil, true

	case keyMatches(msg, m.keys.PageDown):
		m.charSelStart = -1
		m.charSelEnd = -1
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
		return m, nil, true

	case keyMatches(msg, m.keys.GoToBottom):
		m.follow = true
		m.scrollToBottom()
		m.selectedRow = -1
		m.selectionStart = -1
		m.selectionEnd = -1
		m.cursorCol = -1
		m.charSelStart = -1
		m.charSelEnd = -1
		m.scrollX = 0
		if m.splitMode != SplitNone && m.syncScroll {
			m.syncOtherPaneChronologically()
		}
		return m, nil, true

	case keyMatches(msg, m.keys.GoToTop):
		m.cursorCol = -1
		m.charSelStart = -1
		m.charSelEnd = -1
		m.scrollX = 0
		if m.buffer.Len() == 0 {
			m.activeGame = game.RandomMiniGame()
			m.logsDuringGame = 0
			m.mode = modeGame
			return m, m.activeGame.Init(), true
		}
		m.follow = false
		m.scrollOffset = 0
		if len(m.visible) > 0 {
			m.selectedRow = 0
		}
		if m.splitMode != SplitNone && m.syncScroll {
			m.syncOtherPaneChronologically()
		}
		return m, nil, true

	case keyMatches(msg, m.keys.NextMatch):
		m.nextSearchMatch()
		return m, nil, true

	case keyMatches(msg, m.keys.PrevMatch):
		m.prevSearchMatch()
		return m, nil, true

	case keyMatches(msg, m.keys.SelectUp), msg.String() == "shift+up":
		resM, cmd := m.handleSelectUp()
		return resM.(Model), cmd, true

	case keyMatches(msg, m.keys.SelectDown), msg.String() == "shift+down":
		resM, cmd := m.handleSelectDown()
		return resM.(Model), cmd, true
	}

	return m, nil, false
}

func (m Model) handleTabKey(msg tea.KeyMsg) (Model, tea.Cmd, bool) {
	switch {
	case keyMatches(msg, m.keys.NextTab):
		if len(m.tabs) > 1 {
			m.switchTab((m.activeTabIdx() + 1) % len(m.tabs))
		}
		return m, nil, true

	case keyMatches(msg, m.keys.PrevTab):
		if len(m.tabs) > 1 {
			m.switchTab((m.activeTabIdx() - 1 + len(m.tabs)) % len(m.tabs))
		}
		return m, nil, true

	case keyMatches(msg, m.keys.NewTab):
		resM, cmd := m.handleCreateNewTab()
		return resM.(Model), cmd, true

	case keyMatches(msg, m.keys.CloseTab):
		resM, cmd := m.handleCloseActiveTab()
		return resM.(Model), cmd, true

	default:
		s := msg.String()
		if len(s) == 1 && s >= "1" && s <= "9" {
			idx := int(s[0] - '1')
			if idx < len(m.tabs) {
				m.switchTab(idx)
				return m, nil, true
			}
		}
	}
	return m, nil, false
}

func (m Model) handleSplitKey(msg tea.KeyMsg) (Model, tea.Cmd, bool) {
	switch {
	case keyMatches(msg, m.keys.SplitVertical):
		m.toggleSplit(SplitVertical)
		return m, nil, true

	case keyMatches(msg, m.keys.SplitHorizontal):
		m.toggleSplit(SplitHorizontal)
		return m, nil, true

	case keyMatches(msg, m.keys.SwitchPane):
		m.switchPaneFocus()
		return m, nil, true

	case keyMatches(msg, m.keys.ToggleSyncScroll):
		m.toggleSyncScroll()
		return m, nil, true
	}
	return m, nil, false
}

func (m Model) handleActionKey(msg tea.KeyMsg) (Model, tea.Cmd, bool) {
	switch {
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
		return m, nil, true

	case keyMatches(msg, m.keys.Pause):
		if m.buffer.Len() == 0 {
			m.activeGame = game.RandomMiniGame()
			m.logsDuringGame = 0
			m.mode = modeGame
			return m, m.activeGame.Init(), true
		}
		m.paused = !m.paused
		if !m.paused {
			m.rebuildVisible()
			if m.follow {
				m.scrollToBottom()
			}
		}
		return m, nil, true

	case keyMatches(msg, m.keys.SaveLog):
		resM, cmd := m.openSaveLogPicker()
		return resM, cmd, true

	case keyMatches(msg, m.keys.Disconnect):
		resM, cmd := m.disconnect()
		return resM, cmd, true

	case keyMatches(msg, m.keys.Reconnect):
		if m.isFileSource {
			m.message = "Replay of offline log file — cannot reconnect"
			return m, nil, true
		}
		if m.isPipeSource {
			m.message = "Standard input stream — cannot reconnect"
			return m, nil, true
		}
		if m.isProcessSource || m.processCmd != "" {
			if m.source != nil {
				m.source.Stop()
				m.source = nil
			}
			m.manualDisconnect = false
			m.connState = ConnDisconnected
			m.connDetail = ""
			m.reconnecting = true
			m.message = fmt.Sprintf("Restarting %s…", m.processCmd)
			return m, tryRestartProcessCmd(m.processCmd), true
		}
		if m.serialCfg.Port == "" {
			m.message = "No port configured — press p to pick one"
			return m, nil, true
		}
		if m.source != nil {
			m.source.Stop()
			m.source = nil
		}
		m.manualDisconnect = false
		m.connState = ConnDisconnected
		m.connDetail = ""
		m.reconnecting = true
		m.message = fmt.Sprintf("Reconnecting to %s…", m.serialCfg.Port)
		return m, tryReconnectCmd(m.serialCfg), true

	case keyMatches(msg, m.keys.ToggleTimestamp):
		m.tsMode = m.tsMode.Next()
		m.showTimestamp = (m.tsMode != TSModeOff)
		m.saveSettings()
		switch m.tsMode {
		case TSModeClock:
			m.message = fmt.Sprintf("⏱ Timestamp: Clock (%s)", m.tsField)
		case TSModeDelta:
			m.message = "⏱ Timestamp: Delta-Time (Δt)"
		case TSModeBoth:
			m.message = "⏱ Timestamp: Both (Clock + Δt)"
		case TSModeOff:
			m.message = "⏱ Timestamp: OFF"
		}
		return m, nil, true

	case keyMatches(msg, m.keys.ToggleFormat):
		curTab := m.currentTab()
		curTab.DisplayFormat = curTab.DisplayFormat.Next()
		m.displayFormat = curTab.DisplayFormat
		m.recalcLayout()
		m.clampScroll()
		m.message = fmt.Sprintf("Display format: %s", curTab.DisplayFormat.Label())
		return m, nil, true

	case keyMatches(msg, m.keys.ToggleBookmark):
		resM, cmd := m.handleToggleBookmark()
		return resM.(Model), cmd, true

	case keyMatches(msg, m.keys.NextBookmark):
		resM, cmd := m.handleNextBookmark()
		return resM.(Model), cmd, true

	case keyMatches(msg, m.keys.PrevBookmark):
		resM, cmd := m.handlePrevBookmark()
		return resM.(Model), cmd, true

	case keyMatches(msg, m.keys.BookmarksOnly):
		resM, cmd := m.handleToggleBookmarksOnly()
		return resM.(Model), cmd, true

	case keyMatches(msg, m.keys.CopyRow), keyMatches(msg, m.keys.CopyRaw):
		resM, cmd := m.handleCopyKey()
		return resM.(Model), cmd, true
	}
	return m, nil, false
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

	case msg.String() == "down" || msg.Type == tea.KeyDown:
		m.nextSearchMatch()
		return m, nil

	case msg.String() == "up" || msg.Type == tea.KeyUp:
		m.prevSearchMatch()
		return m, nil

	default:
		prev := m.searchInput.Value
		m.searchInput.HandleKey(msg)
		if m.searchInput.Value != prev {
			m.runSearch()
		}
	}
	return m, nil
}

func (m Model) handleFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case keyMatches(msg, m.keys.Cancel):
		m.mode = modeNormal
		m.filterInput.SetText(m.currentTab().FilterRaw)
		m.filterInput.ResetHistoryCursor()
		return m, nil

	case keyMatches(msg, m.keys.Confirm):
		val := m.filterInput.Value
		f, err := filter.New(val)
		if err != nil {
			m.message = fmt.Sprintf("filter error: %v", err)
		} else {
			m.activeFilter = f
			cur := m.currentTab()
			cur.Filter = f
			cur.FilterRaw = val
			if cur.Name == "" || strings.HasPrefix(cur.Name, "Tab ") {
				if val != "" {
					cur.Name = val
				}
			}
			m.rebuildVisible()
			cur.Visible = m.visible
			if m.follow {
				m.scrollToBottom()
				cur.ScrollOffset = m.scrollOffset
			}

			if m.filterInput.AddHistory(val) {
				if m.filtersCfg != nil {
					m.filtersCfg.History = m.filterInput.History
					if m.appConfig != nil {
						_ = m.appConfig.SaveFilters(m.filtersCfg)
					}
				}
			}
		}
		m.filterInput.ResetHistoryCursor()
		m.mode = modeNormal
		return m, nil

	case msg.Type == tea.KeyCtrlP || msg.String() == "ctrl+p":
		m.loadFilters()
		m.presetCursor = 0
		m.mode = modeFilterPresets
		return m, nil

	default:
		m.filterInput.HandleKey(msg)
	}
	return m, nil
}

func (m Model) handleTXKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case keyMatches(msg, m.keys.Cancel):
		m.mode = modeNormal
		m.txInput.Reset()
		return m, nil

	case keyMatches(msg, m.keys.Confirm) || msg.Type == tea.KeyEnter:
		val := m.txInput.Value
		if len(val) == 0 && m.txEnding == serial.EndingNone {
			m.mode = modeNormal
			m.txInput.Reset()
			return m, nil
		}

		payload, err := serial.FormatTXPayload(val, m.txEnding)
		if err != nil {
			m.message = fmt.Sprintf("TX format error: %v", err)
			return m, nil
		}

		if m.source == nil {
			m.message = "TX failed: disconnected"
			m.mode = modeNormal
			m.txInput.Reset()
			return m, nil
		}

		n, err := m.source.Write(payload)
		if err != nil {
			m.message = fmt.Sprintf("TX error: %v", err)
		} else {
			m.message = fmt.Sprintf("✓ Sent %d bytes [%s]", n, m.txEnding.String())
			m.recordTXMessage(val)
		}

		if m.txInput.AddHistory(val) {
			m.saveSettings()
		}

		m.txInput.Reset()
		m.mode = modeNormal
		return m, nil

	case msg.Type == tea.KeyTab || msg.String() == "tab" || msg.Type == tea.KeyCtrlE || msg.String() == "ctrl+e":
		m.txEnding = m.txEnding.Next()
		m.saveSettings()
		return m, nil

	default:
		m.txInput.HandleKey(msg)
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


