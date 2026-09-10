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
		m.searchPos = 0
		m.searchMatches = nil
		m.searchCursor = 0
		return m, nil

	case keyMatches(msg, m.keys.Filter):
		m.mode = modeFilter
		m.filterInput = m.currentTab().FilterRaw
		m.filterDraft = m.filterInput
		m.filterCursor = len([]rune(m.filterInput))
		m.filterHistoryCursor = -1
		return m, nil

	case keyMatches(msg, m.keys.SendTX):
		m.mode = modeTXInput
		m.txInput = ""
		m.txDraft = ""
		m.txCursor = 0
		m.txHistoryCursor = -1
		return m, nil

	case keyMatches(msg, m.keys.FilterPresets):
		m.loadFilters()
		m.presetCursor = 0
		m.mode = modeFilterPresets
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
	return m, nil

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
		return m, nil

	case keyMatches(msg, m.keys.ScrollLeft):
		if m.scrollX > 0 {
			m.scrollX -= 8
			m.clampScrollX()
		}
		return m, nil

	case keyMatches(msg, m.keys.ScrollRight):
		m.scrollX += 8
		m.clampScrollX()
		return m, nil

	case keyMatches(msg, m.keys.CursorLeft):
		if len(m.visible) == 0 {
			return m, nil
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
		return m, nil

	case keyMatches(msg, m.keys.CursorRight):
		if len(m.visible) == 0 {
			return m, nil
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
		return m, nil

	case keyMatches(msg, m.keys.CharSelectLeft), msg.String() == "shift+left":
		if len(m.visible) == 0 {
			return m, nil
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
		return m, nil

	case keyMatches(msg, m.keys.CharSelectRight), msg.String() == "shift+right":
		if len(m.visible) == 0 {
			return m, nil
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
		return m, nil

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
		return m, nil

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
		return m, nil

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
		return m, nil

	case keyMatches(msg, m.keys.GoToTop):
		m.cursorCol = -1
		m.charSelStart = -1
		m.charSelEnd = -1
		m.scrollX = 0
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

	case keyMatches(msg, m.keys.Settings):
		m.mode = modeSettings
		return m, nil

	case keyMatches(msg, m.keys.Disconnect):
		return m.disconnect()

	case keyMatches(msg, m.keys.Reconnect):
		if m.serialCfg.Port == "" {
			m.message = "No port configured — press p to pick one"
			return m, nil
		}
		if m.isFileSource {
			m.message = "Replay of offline log file — cannot reconnect"
			return m, nil
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
		return m, tryReconnectCmd(m.serialCfg)

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
		return m, nil

	case keyMatches(msg, m.keys.ToggleFormat):
		curTab := m.currentTab()
		curTab.DisplayFormat = curTab.DisplayFormat.Next()
		m.displayFormat = curTab.DisplayFormat
		m.recalcLayout()
		m.clampScroll()
		m.message = fmt.Sprintf("Display format: %s", curTab.DisplayFormat.Label())
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
		m.searchInput = ""
		m.searchPos = 0
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
			m.searchInput, m.searchPos = insertStringAtCursor(m.searchInput, m.searchPos, strings.TrimSpace(clean))
			m.runSearch()
		}
		return m, nil

	case msg.String() == "down" || msg.Type == tea.KeyDown:
		m.nextSearchMatch()
		return m, nil

	case msg.String() == "up" || msg.Type == tea.KeyUp:
		m.prevSearchMatch()
		return m, nil

	default:
		prev := m.searchInput
		m.searchInput, m.searchPos = handleTextInputWithCursor(m.searchInput, m.searchPos, msg)
		if m.searchInput != prev {
			m.runSearch()
		}
	}
	return m, nil
}

func (m Model) handleFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case keyMatches(msg, m.keys.Cancel):
		m.mode = modeNormal
		m.filterInput = m.currentTab().FilterRaw
		m.filterCursor = len([]rune(m.filterInput))
		m.filterHistoryCursor = -1
		m.filterDraft = ""
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

			// Record in filter history if non-empty and unique from last entry
			trimmed := strings.TrimSpace(m.filterInput)
			if trimmed != "" {
				if len(m.filterHistory) == 0 || m.filterHistory[len(m.filterHistory)-1] != trimmed {
					m.filterHistory = append(m.filterHistory, trimmed)
					if m.filtersCfg != nil {
						m.filtersCfg.History = m.filterHistory
						if m.appConfig != nil {
							_ = m.appConfig.SaveFilters(m.filtersCfg)
						}
					}
				}
			}
		}
		m.filterHistoryCursor = -1
		m.filterDraft = ""
		m.mode = modeNormal
		return m, nil

	case msg.Type == tea.KeyUp || msg.String() == "up":
		if len(m.filterHistory) > 0 {
			if m.filterHistoryCursor == -1 {
				m.filterDraft = m.filterInput
				m.filterHistoryCursor = len(m.filterHistory) - 1
			} else if m.filterHistoryCursor > 0 {
				m.filterHistoryCursor--
			}
			if m.filterHistoryCursor >= 0 && m.filterHistoryCursor < len(m.filterHistory) {
				m.filterInput = m.filterHistory[m.filterHistoryCursor]
				m.filterCursor = len([]rune(m.filterInput))
			}
		}
		return m, nil

	case msg.Type == tea.KeyDown || msg.String() == "down":
		if m.filterHistoryCursor != -1 {
			if m.filterHistoryCursor < len(m.filterHistory)-1 {
				m.filterHistoryCursor++
				m.filterInput = m.filterHistory[m.filterHistoryCursor]
				m.filterCursor = len([]rune(m.filterInput))
			} else {
				m.filterHistoryCursor = -1
				m.filterInput = m.filterDraft
				m.filterCursor = len([]rune(m.filterInput))
			}
		}
		return m, nil

	case msg.Type == tea.KeyCtrlP || msg.String() == "ctrl+p":
		m.loadFilters()
		m.presetCursor = 0
		m.mode = modeFilterPresets
		return m, nil

	case msg.Type == tea.KeyCtrlV || msg.String() == "ctrl+v":
		clipText, err := clipboard.Read()
		if err == nil && clipText != "" {
			clean := strings.ReplaceAll(strings.ReplaceAll(clipText, "\r", ""), "\n", " ")
			m.filterInput, m.filterCursor = insertStringAtCursor(m.filterInput, m.filterCursor, strings.TrimSpace(clean))
		}
		return m, nil

	default:
		m.filterInput, m.filterCursor = handleTextInputWithCursor(m.filterInput, m.filterCursor, msg)
	}
	return m, nil
}

func (m Model) handleTXKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case keyMatches(msg, m.keys.Cancel):
		m.mode = modeNormal
		m.txInput = ""
		m.txCursor = 0
		m.txDraft = ""
		m.txHistoryCursor = -1
		return m, nil

	case keyMatches(msg, m.keys.Confirm) || msg.Type == tea.KeyEnter:
		if len(m.txInput) == 0 && m.txEnding == serial.EndingNone {
			m.mode = modeNormal
			m.txHistoryCursor = -1
			m.txDraft = ""
			m.txCursor = 0
			return m, nil
		}

		payload, err := serial.FormatTXPayload(m.txInput, m.txEnding)
		if err != nil {
			m.message = fmt.Sprintf("TX format error: %v", err)
			return m, nil
		}

		if m.source == nil {
			m.message = "TX failed: disconnected"
			m.mode = modeNormal
			m.txHistoryCursor = -1
			m.txDraft = ""
			m.txInput = ""
			m.txCursor = 0
			return m, nil
		}

		n, err := m.source.Write(payload)
		if err != nil {
			m.message = fmt.Sprintf("TX error: %v", err)
		} else {
			m.message = fmt.Sprintf("✓ Sent %d bytes [%s]", n, m.txEnding.String())
			m.recordTXMessage(m.txInput)
		}

		trimmed := strings.TrimSpace(m.txInput)
		if trimmed != "" {
			if len(m.txHistory) == 0 || m.txHistory[len(m.txHistory)-1] != trimmed {
				m.txHistory = append(m.txHistory, trimmed)
				m.saveSettings()
			}
		}

		m.txHistoryCursor = -1
		m.txDraft = ""
		m.txInput = ""
		m.txCursor = 0
		m.mode = modeNormal
		return m, nil

	case msg.Type == tea.KeyTab || msg.String() == "tab" || msg.Type == tea.KeyCtrlE || msg.String() == "ctrl+e":
		m.txEnding = m.txEnding.Next()
		m.saveSettings()
		return m, nil

	case msg.Type == tea.KeyUp || msg.String() == "up":
		if len(m.txHistory) > 0 {
			if m.txHistoryCursor == -1 {
				m.txDraft = m.txInput
				m.txHistoryCursor = len(m.txHistory) - 1
			} else if m.txHistoryCursor > 0 {
				m.txHistoryCursor--
			}
			if m.txHistoryCursor >= 0 && m.txHistoryCursor < len(m.txHistory) {
				m.txInput = m.txHistory[m.txHistoryCursor]
				m.txCursor = len([]rune(m.txInput))
			}
		}
		return m, nil

	case msg.Type == tea.KeyDown || msg.String() == "down":
		if m.txHistoryCursor != -1 {
			if m.txHistoryCursor < len(m.txHistory)-1 {
				m.txHistoryCursor++
				m.txInput = m.txHistory[m.txHistoryCursor]
				m.txCursor = len([]rune(m.txInput))
			} else {
				m.txHistoryCursor = -1
				m.txInput = m.txDraft
				m.txCursor = len([]rune(m.txInput))
			}
		}
		return m, nil

	case msg.Type == tea.KeyCtrlV || msg.String() == "ctrl+v":
		clipText, err := clipboard.Read()
		if err == nil && clipText != "" {
			clean := strings.ReplaceAll(strings.ReplaceAll(clipText, "\r", ""), "\n", " ")
			m.txInput, m.txCursor = insertStringAtCursor(m.txInput, m.txCursor, strings.TrimSpace(clean))
		}
		return m, nil

	case msg.Type == tea.KeyCtrlU || msg.String() == "ctrl+u":
		m.txInput = ""
		m.txCursor = 0
		return m, nil

	default:
		m.txInput, m.txCursor = handleTextInputWithCursor(m.txInput, m.txCursor, msg)
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

// handleTextInput is a backward-compatible wrapper appending/deleting at the end.
func handleTextInput(current string, msg tea.KeyMsg) string {
	res, _ := handleTextInputWithCursor(current, len([]rune(current)), msg)
	return res
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
