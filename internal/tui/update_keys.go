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
		m.filterDraft = m.filterInput
		m.filterHistoryCursor = -1
		return m, nil

	case keyMatches(msg, m.keys.SendTX):
		m.mode = modeTXInput
		m.txInput = ""
		m.txDraft = ""
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

	case keyMatches(msg, m.keys.ScrollUp) || msg.Type == tea.KeyUp:
		if len(m.filterHistory) > 0 {
			if m.filterHistoryCursor == -1 {
				m.filterDraft = m.filterInput
				m.filterHistoryCursor = len(m.filterHistory) - 1
			} else if m.filterHistoryCursor > 0 {
				m.filterHistoryCursor--
			}
			if m.filterHistoryCursor >= 0 && m.filterHistoryCursor < len(m.filterHistory) {
				m.filterInput = m.filterHistory[m.filterHistoryCursor]
			}
		}
		return m, nil

	case keyMatches(msg, m.keys.ScrollDown) || msg.Type == tea.KeyDown:
		if m.filterHistoryCursor != -1 {
			if m.filterHistoryCursor < len(m.filterHistory)-1 {
				m.filterHistoryCursor++
				m.filterInput = m.filterHistory[m.filterHistoryCursor]
			} else {
				m.filterHistoryCursor = -1
				m.filterInput = m.filterDraft
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
			m.filterInput += strings.TrimSpace(clean)
		}
		return m, nil

	default:
		m.filterInput = handleTextInput(m.filterInput, msg)
	}
	return m, nil
}

func (m Model) handleTXKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case keyMatches(msg, m.keys.Cancel):
		m.mode = modeNormal
		m.txInput = ""
		m.txDraft = ""
		m.txHistoryCursor = -1
		return m, nil

	case keyMatches(msg, m.keys.Confirm) || msg.Type == tea.KeyEnter:
		if len(m.txInput) == 0 && m.txEnding == serial.EndingNone {
			m.mode = modeNormal
			m.txHistoryCursor = -1
			m.txDraft = ""
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
		m.mode = modeNormal
		return m, nil

	case msg.Type == tea.KeyTab || msg.String() == "tab" || msg.Type == tea.KeyCtrlE || msg.String() == "ctrl+e":
		m.txEnding = m.txEnding.Next()
		m.saveSettings()
		return m, nil

	case keyMatches(msg, m.keys.ScrollUp) || msg.Type == tea.KeyUp:
		if len(m.txHistory) > 0 {
			if m.txHistoryCursor == -1 {
				m.txDraft = m.txInput
				m.txHistoryCursor = len(m.txHistory) - 1
			} else if m.txHistoryCursor > 0 {
				m.txHistoryCursor--
			}
			if m.txHistoryCursor >= 0 && m.txHistoryCursor < len(m.txHistory) {
				m.txInput = m.txHistory[m.txHistoryCursor]
			}
		}
		return m, nil

	case keyMatches(msg, m.keys.ScrollDown) || msg.Type == tea.KeyDown:
		if m.txHistoryCursor != -1 {
			if m.txHistoryCursor < len(m.txHistory)-1 {
				m.txHistoryCursor++
				m.txInput = m.txHistory[m.txHistoryCursor]
			} else {
				m.txHistoryCursor = -1
				m.txInput = m.txDraft
			}
		}
		return m, nil

	case msg.Type == tea.KeyCtrlV || msg.String() == "ctrl+v":
		clipText, err := clipboard.Read()
		if err == nil && clipText != "" {
			clean := strings.ReplaceAll(strings.ReplaceAll(clipText, "\r", ""), "\n", " ")
			m.txInput += strings.TrimSpace(clean)
		}
		return m, nil

	case msg.Type == tea.KeyCtrlU || msg.String() == "ctrl+u":
		m.txInput = ""
		return m, nil

	default:
		m.txInput = handleTextInput(m.txInput, msg)
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

