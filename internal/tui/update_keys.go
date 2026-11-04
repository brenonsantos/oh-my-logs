package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/brenoniehues/oh-my-logs/internal/config"
	"github.com/brenoniehues/oh-my-logs/internal/encoder"
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
	case modeTimeJump:
		return m.handleTimeJumpKey(msg)
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
	case modeShellInteractive:
		return m.handleShellInteractiveKey(msg)
	case modeSettings:
		return m.handleSettingsKey(msg)
	case modeFilePicker:
		return m.handleFilePickerKey(msg)
	case modeRowDetail:
		return m.handleRowDetailKey(msg)
	case modeColumnModal:
		return m.handleColumnModalKey(msg)
	case modeThemeModal:
		return m.handleThemeModalKey(msg)
	case modeMarkerPrompt:
		return m.handleMarkerPromptKey(msg)
	case modeClearConfirm:
		return m.handleClearConfirmKey(msg)
	default:
		return m.handleNormalKey(msg)
	}
}

func (m Model) handleNormalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case keyMatches(msg, m.keys.Quit):
		hasCharSel := m.charSelStart >= 0 && m.charSelEnd >= 0 && m.charSelStart != m.charSelEnd
		hasRowSel := m.selectionStart >= 0 && m.selectionEnd >= 0 &&
			(m.selectionStart != m.selectionEnd || (m.selectionStart == 0 && m.selectionEnd == len(m.visible)-1 && len(m.visible) > 0))
		if (msg.Type == tea.KeyCtrlC || msg.String() == "ctrl+c") && (hasCharSel || hasRowSel) {
			return m.handleCopyKey()
		}
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

	case keyMatches(msg, m.keys.TimeJump):
		m.mode = modeTimeJump
		m.timeJumpInput.Clear()
		m.timeJumpInput.ResetHistoryCursor()
		return m, nil

	case keyMatches(msg, m.keys.SendTX):
		m.mode = modeTXInput
		m.txInput.Reset()
		return m, nil

	case keyMatches(msg, m.keys.ShellPassthrough):
		m.mode = modeShellInteractive
		m.shellAwaitingTab = false
		m.shellInput.Reset()
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

	case keyMatches(msg, m.keys.Columns):
		m.openColumnModal()
		return m, nil

	case keyMatches(msg, m.keys.AddMarker):
		m.openMarkerPrompt()
		return m, nil

	case msg.String() == "d" || msg.String() == "delete" || msg.String() == "backspace":
		if rec, ok := m.activeSelectedRecord(); ok && rec.IsMarker {
			m.removeMarker(rec.ID)
			return m, nil
		}

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
		m.clampScroll()
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
		if t != nil {
			t.Follow = false
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
		m.follow = false
		m.clampScroll()
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
			if m.selectedRow >= 0 {
				m.follow = false
			} else if m.scrollOffset >= len(m.visible)-h {
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
		m.clampScroll()
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
		m.clampScroll()
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
		m.clampScroll()
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
		m.clampScroll()
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
			m.follow = false
		} else if m.scrollOffset >= len(m.visible)-h {
			m.follow = true
		}
		m.clampScroll()
		if m.splitMode != SplitNone && m.syncScroll {
			m.syncOtherPaneChronologically()
		}
		return m, nil, true

	case keyMatches(msg, m.keys.GoToBottom):
		m.follow = true
		m.paused = false
		m.scrollToBottom()
		m.selectedRow = -1
		m.selectionStart = -1
		m.selectionEnd = -1
		m.cursorCol = -1
		m.charSelStart = -1
		m.charSelEnd = -1
		m.scrollX = 0
		if curTab := m.currentTab(); curTab != nil {
			curTab.Follow = true
			curTab.SelectedRow = -1
			curTab.CursorCol = -1
			curTab.CharSelStart = -1
			curTab.CharSelEnd = -1
			curTab.ScrollX = 0
			curTab.ScrollOffset = m.scrollOffset
		}
		if m.splitMode != SplitNone {
			if t := m.currentTabForPane(m.activePane); t != nil {
				t.Follow = true
				t.SelectedRow = -1
			}
			if m.syncScroll {
				m.syncOtherPaneChronologically()
			}
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

	case keyMatches(msg, m.keys.SelectAll), msg.String() == "ctrl+a":
		resM, cmd := m.handleSelectAll()
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
		m.mode = modeClearConfirm
		return m, nil, true

	case keyMatches(msg, m.keys.Pause) || msg.Type == tea.KeySpace || msg.String() == " " || msg.String() == "space":
		m.toggleFollow()
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
	
	case isSearchToggleCase(msg):
		m.searchCaseSensitive = !m.searchCaseSensitive
		m.runSearch()
		return m, nil

	case isSearchToggleWord(msg):
		m.searchWholeWord = !m.searchWholeWord
		m.runSearch()
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

func isSearchToggleCase(msg tea.KeyMsg) bool {
	if msg.String() == "alt+c" || (msg.Alt && (msg.String() == "c" || msg.String() == "C")) {
		return true
	}
	// macOS Option+c without Meta key produces 'ç' or 'Ç'
	if msg.String() == "ç" || msg.String() == "Ç" || (len(msg.Runes) > 0 && (msg.Runes[0] == 'ç' || msg.Runes[0] == 'Ç')) {
		return true
	}
	return false
}

func isSearchToggleWord(msg tea.KeyMsg) bool {
	if msg.String() == "alt+w" || (msg.Alt && (msg.String() == "w" || msg.String() == "W")) {
		return true
	}
	// macOS Option+w without Meta key produces '∑'
	if msg.String() == "∑" || (len(msg.Runes) > 0 && msg.Runes[0] == '∑') {
		return true
	}
	return false
}

func (m Model) handleClearConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := msg.String()
	if s == "y" || s == "Y" || keyMatches(msg, m.keys.Confirm) || msg.Type == tea.KeyEnter {
		m.clearBuffer()
		m.mode = modeNormal
		m.message = "Buffer cleared"
		return m, nil
	} else if s == "n" || s == "N" || keyMatches(msg, m.keys.Cancel) || msg.Type == tea.KeyEsc {
		m.mode = modeNormal
		m.message = "Clear cancelled"
		return m, nil
	}
	return m, nil
}

func (m *Model) clearBuffer() {
	m.buffer.Clear()
	for i := range m.tabs {
		m.tabs[i].Visible = nil
		m.tabs[i].ScrollOffset = 0
		m.tabs[i].SearchMatches = nil
		m.tabs[i].SelectedRow = -1
		m.tabs[i].CursorCol = -1
		m.tabs[i].CharSelStart = -1
		m.tabs[i].CharSelEnd = -1
	}
	m.visible = nil
	m.searchMatches = nil
	m.searchCursor = -1
	m.scrollOffset = 0
	m.selectedRow = -1
	m.selectionStart = -1
	m.selectionEnd = -1
	m.cursorCol = -1
	m.charSelStart = -1
	m.charSelEnd = -1
	m.bookmarks = make(map[uint64]struct{})
	m.recalcLayout()
	m.clampScroll()
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
			if val != "" {
				cur.Name = val
			} else {
				if m.activeTab == 0 {
					cur.Name = "All"
				} else {
					cur.Name = fmt.Sprintf("Tab %d", m.activeTab+1)
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
		if len(m.txSuggestions) > 0 {
			m.txSuggestions = nil
			m.txSuggestionIdx = -1
			return m, nil
		}
		m.mode = modeNormal
		m.txInput.Reset()
		m.txPreview = ""
		return m, nil

	case msg.Type == tea.KeyCtrlT || msg.String() == "ctrl+t" || msg.Type == tea.KeyF2 || msg.String() == "f2" || msg.String() == "alt+e" || (msg.Alt && (msg.String() == "e" || (msg.Type == tea.KeyRunes && len(msg.Runes) > 0 && msg.Runes[0] == 'e'))):
		m.toggleActiveEncoder()
		return m, m.triggerSuggestCmd()

	case msg.Type == tea.KeyCtrlE || msg.String() == "ctrl+e":
		m.txEnding = m.txEnding.Next()
		m.saveSettings()
		return m, nil

	case msg.Type == tea.KeyUp || msg.String() == "up":
		if len(m.txSuggestions) > 0 {
			m.txSuggestionIdx--
			if m.txSuggestionIdx < 0 {
				m.txSuggestionIdx = len(m.txSuggestions) - 1
			}
			return m, nil
		}
		m.txInput.HandleKey(msg)
		return m, m.triggerSuggestCmd()

	case msg.Type == tea.KeyDown || msg.String() == "down":
		if len(m.txSuggestions) > 0 {
			m.txSuggestionIdx++
			if m.txSuggestionIdx >= len(m.txSuggestions) {
				m.txSuggestionIdx = 0
			}
			return m, nil
		}
		m.txInput.HandleKey(msg)
		return m, m.triggerSuggestCmd()

	case msg.Type == tea.KeyTab || msg.String() == "tab":
		if len(m.txSuggestions) > 0 && m.txSuggestionIdx >= 0 && m.txSuggestionIdx < len(m.txSuggestions) {
			sug := m.txSuggestions[m.txSuggestionIdx]
			newVal, newCursor := applySuggestion(m.txInput.Value, m.txInput.Cursor, sug.Value)
			m.txInput.SetTextAndCursor(newVal, newCursor)
			m.txSuggestions = nil
			m.txSuggestionIdx = -1
			return m, m.triggerSuggestCmd()
		}
		m.txEnding = m.txEnding.Next()
		m.saveSettings()
		return m, nil

	case keyMatches(msg, m.keys.Confirm) || msg.Type == tea.KeyEnter:
		if len(m.txSuggestions) > 0 && m.txSuggestionIdx >= 0 && m.txSuggestionIdx < len(m.txSuggestions) {
			sug := m.txSuggestions[m.txSuggestionIdx]
			newVal, newCursor := applySuggestion(m.txInput.Value, m.txInput.Cursor, sug.Value)
			m.txInput.SetTextAndCursor(newVal, newCursor)
			m.txSuggestions = nil
			m.txSuggestionIdx = -1
			return m, m.triggerSuggestCmd()
		}

		val := m.txInput.Value
		if len(val) == 0 && m.txEnding == serial.EndingNone {
			m.mode = modeNormal
			m.txInput.Reset()
			return m, nil
		}

		if m.source == nil {
			m.message = "TX failed: disconnected"
			m.mode = modeNormal
			m.txInput.Reset()
			return m, nil
		}

		var activeEnc encoder.Encoder
		if m.activeEncoderIdx >= 0 && m.encoders != nil && m.activeEncoderIdx < len(m.encoders.Encoders()) {
			activeEnc = m.encoders.Encoders()[m.activeEncoderIdx]
		} else if m.encoders != nil {
			activeEnc, _ = m.encoders.Match(val)
		}

		wireText := val
		displayText := val
		sentDesc := m.txEnding.String()

		if activeEnc != nil {
			res, err := activeEnc.Encode(val)
			if err != nil {
				m.message = fmt.Sprintf("Encoder [%s] error: %v", activeEnc.Name(), err)
				return m, nil
			}
			wireText = res.Payload
			if res.Display != "" {
				displayText = res.Display
			}
			sentDesc = fmt.Sprintf("%s | %s", activeEnc.Name(), m.txEnding.String())
		} else if m.encoders != nil {
			// If not matched by trigger prefix, try encoders in pipeline
			for _, enc := range m.encoders.Encoders() {
				if res, err := enc.Encode(val); err == nil && res.Payload != "" {
					wireText = res.Payload
					if res.Display != "" {
						displayText = res.Display
					}
					sentDesc = fmt.Sprintf("%s | %s", enc.Name(), m.txEnding.String())
					break
				}
			}
		}

		// Fallback to active live preview if display was not transformed
		if displayText == wireText && m.txPreview != "" {
			displayText = m.txPreview
		}

		payload, err := serial.FormatTXPayload(wireText, m.txEnding)
		if err != nil {
			m.message = fmt.Sprintf("TX format error: %v", err)
			return m, nil
		}

		n, err := m.source.Write(payload)
		if err != nil {
			m.message = fmt.Sprintf("TX error: %v", err)
		} else {
			m.message = fmt.Sprintf("✓ Sent %d bytes [%s]", n, sentDesc)
			if displayText != "" && displayText != wireText {
				m.recordTXMessage(wireText)
				m.recordTXContinuation(fmt.Sprintf("↳ %s", displayText))
			} else {
				m.recordTXMessage(wireText)
			}
		}

		if m.txInput.AddHistory(val) {
			m.saveSettings()
		}

		m.txInput.Reset()
		m.txSuggestions = nil
		m.txSuggestionIdx = -1
		m.txPreview = ""
		m.mode = modeNormal
		return m, nil

	default:
		m.txInput.HandleKey(msg)
		return m, m.triggerSuggestCmd()
	}
}

func (m Model) handleShellInteractiveKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// 1. Esc or Ctrl+] returns cleanly to normal log navigation
	if keyMatches(msg, m.keys.Cancel) || msg.Type == tea.KeyEsc || msg.String() == "ctrl+]" {
		m.mode = modeNormal
		m.shellAwaitingTab = false
		m.shellInput.Reset()
		return m, nil
	}

	// 2. Disconnected check
	if m.source == nil {
		m.message = "Shell TX failed: disconnected"
		m.mode = modeNormal
		m.shellAwaitingTab = false
		return m, nil
	}

	// 3. Tab: Forward ASCII 0x09 directly to MCU shell for native autocompletion!
	if msg.Type == tea.KeyTab || msg.String() == "tab" {
		m.shellAwaitingTab = true
		_, _ = m.source.Write([]byte{'\t'})
		return m, nil
	}

	// Any non-Tab key clears active Tab completion expectation
	m.shellAwaitingTab = false

	// 4. Ctrl+C: Send ASCII 0x03 interrupt signal to MCU shell
	if msg.Type == tea.KeyCtrlC || msg.String() == "ctrl+c" {
		_, _ = m.source.Write([]byte{0x03})
		m.shellInput.Reset()
		return m, nil
	}

	// 5. Ctrl+D: Send EOF (0x04)
	if msg.Type == tea.KeyCtrlD || msg.String() == "ctrl+d" {
		_, _ = m.source.Write([]byte{0x04})
		return m, nil
	}

	// 6. Up / Down: Send ANSI arrow sequences for MCU shell history navigation,
	// and update local input history
	if msg.Type == tea.KeyUp || msg.String() == "up" {
		_, _ = m.source.Write([]byte("\x1b[A"))
		m.shellInput.HistoryPrev()
		return m, nil
	}
	if msg.Type == tea.KeyDown || msg.String() == "down" {
		_, _ = m.source.Write([]byte("\x1b[B"))
		m.shellInput.HistoryNext()
		return m, nil
	}

	// 7. Left / Right: Send ANSI cursor movement to MCU shell
	if msg.Type == tea.KeyLeft || msg.String() == "left" {
		_, _ = m.source.Write([]byte("\x1b[D"))
		m.shellInput.HandleKey(msg)
		return m, nil
	}
	if msg.Type == tea.KeyRight || msg.String() == "right" {
		_, _ = m.source.Write([]byte("\x1b[C"))
		m.shellInput.HandleKey(msg)
		return m, nil
	}

	// 8. Backspace: Forward backspace (0x7F) to MCU shell
	if msg.Type == tea.KeyBackspace || msg.String() == "backspace" {
		_, _ = m.source.Write([]byte{0x7F})
		m.shellInput.HandleKey(msg)
		return m, nil
	}

	// 9. Enter: Transmit command line ending, save history, record TX badge.
	// NOTE: In interactive passthrough mode, character runes were ALREADY transmitted
	// character-by-character as the engineer typed them. Therefore, pressing Enter must
	// ONLY transmit the configured line ending suffix (e.g. \r or \r\n), NOT re-transmit
	// the entire string `val + ending`, which would result in command duplication (e.g. "os_msgos_msg").
	if keyMatches(msg, m.keys.Confirm) || msg.Type == tea.KeyEnter {
		val := m.shellInput.Value
		cleanVal := serial.CleanTerminalLine(val)
		if cleanVal != "" {
			val = cleanVal
		}
		ending := m.txEnding.Suffix()
		if ending == "" {
			ending = "\r"
		}
		_, _ = m.source.Write([]byte(ending))
		if val != "" {
			m.recordTXMessage(val)
			m.lastTXCommand = strings.TrimSpace(val)
			if m.shellInput.AddHistory(val) {
				m.saveSettings()
			}
		}
		m.shellInput.Reset()
		return m, nil
	}

	// 10. Normal character typing: send characters directly over serial TX
	runes := msg.Runes
	if len(runes) > 0 {
		rawBytes := []byte(string(runes))
		_, _ = m.source.Write(rawBytes)
		m.shellInput.Insert(string(runes))
		return m, nil
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


