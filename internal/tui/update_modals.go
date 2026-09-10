package tui

import (
	"fmt"
	"strings"

	"github.com/brenoniehues/oh-my-logs/internal/config"
	"github.com/brenoniehues/oh-my-logs/internal/filter"
	"github.com/brenoniehues/oh-my-logs/internal/parser"
	"github.com/brenoniehues/oh-my-logs/internal/record"
	"github.com/brenoniehues/oh-my-logs/internal/timing"
	tea "github.com/charmbracelet/bubbletea"
)

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
			if old.Fields["level"] == "TX" {
				return old
			}
			newRec, _ := m.parser.Parse(old.Raw)
			newRec.ID = old.ID
			newRec.Timestamp = old.Timestamp
			newRec.Delta = old.Delta
			if newRec.Fields == nil {
				newRec.Fields = make(map[string]string)
			}
			if old.Fields["_delta"] != "" {
				newRec.Fields["_delta"] = old.Fields["_delta"]
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

		if m.profile != nil {
			m.deltaTracker = timing.NewTracker(m.profile.TimingParameters())
		} else {
			m.deltaTracker = timing.NewTracker(timing.DefaultConfig())
		}

		m.rebuildAllTabs()
		m.mode = modeNormal
		m.saveSettings()
		m.message = fmt.Sprintf("Profile: %s", selected.Name)
		return m, nil
	}
	return m, nil
}

// handleFilterPresetsKey handles keyboard navigation and actions in the filter presets modal.
func (m Model) handleFilterPresetsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.loadFilters()
	presets := m.filtersCfg.Presets

	switch {
	case keyMatches(msg, m.keys.Cancel) || msg.String() == "q":
		m.mode = modeNormal
		return m, nil

	case keyMatches(msg, m.keys.ScrollUp) || msg.String() == "k" || msg.Type == tea.KeyUp:
		if len(presets) > 0 {
			if m.presetCursor > 0 {
				m.presetCursor--
			} else {
				m.presetCursor = len(presets) - 1
			}
		}
		return m, nil

	case keyMatches(msg, m.keys.ScrollDown) || msg.String() == "j" || msg.Type == tea.KeyDown:
		if len(presets) > 0 {
			if m.presetCursor < len(presets)-1 {
				m.presetCursor++
			} else {
				m.presetCursor = 0
			}
		}
		return m, nil

	case msg.String() == "s" || msg.String() == "a" || msg.String() == "+":
		cur := m.currentTab()
		if cur == nil || strings.TrimSpace(cur.FilterRaw) == "" {
			m.message = "No active filter on current tab to save"
			return m, nil
		}
		m.savePresetNameInput = cur.Name
		if m.savePresetNameInput == "" || strings.HasPrefix(m.savePresetNameInput, "Tab ") {
			m.savePresetNameInput = cur.FilterRaw
		}
		m.mode = modeSavePresetPrompt
		return m, nil

	case msg.String() == "d" || msg.String() == "x" || msg.Type == tea.KeyDelete:
		if len(presets) == 0 || m.presetCursor < 0 || m.presetCursor >= len(presets) {
			return m, nil
		}
		delName := presets[m.presetCursor].Name
		m.filtersCfg.Presets = append(presets[:m.presetCursor], presets[m.presetCursor+1:]...)
		if m.presetCursor >= len(m.filtersCfg.Presets) && m.presetCursor > 0 {
			m.presetCursor = len(m.filtersCfg.Presets) - 1
		}
		if m.appConfig != nil {
			_ = m.appConfig.SaveFilters(m.filtersCfg)
		}
		m.message = fmt.Sprintf("Deleted preset %q", delName)
		return m, nil

	case keyMatches(msg, m.keys.Confirm):
		if len(presets) == 0 || m.presetCursor < 0 || m.presetCursor >= len(presets) {
			m.mode = modeNormal
			return m, nil
		}
		selected := presets[m.presetCursor]
		f, err := filter.New(selected.Filter)
		if err != nil {
			m.message = fmt.Sprintf("invalid filter preset: %v", err)
			m.mode = modeNormal
			return m, nil
		}

		m.activeFilter = f
		cur := m.currentTab()
		cur.Filter = f
		cur.FilterRaw = selected.Filter
		cur.Name = selected.Name
		m.rebuildVisible()
		cur.Visible = m.visible
		if m.follow {
			m.scrollToBottom()
			cur.ScrollOffset = m.scrollOffset
		}

		// Add to filter history if new
		if len(m.filterHistory) == 0 || m.filterHistory[len(m.filterHistory)-1] != selected.Filter {
			m.filterHistory = append(m.filterHistory, selected.Filter)
			if m.filtersCfg != nil {
				m.filtersCfg.History = m.filterHistory
				if m.appConfig != nil {
					_ = m.appConfig.SaveFilters(m.filtersCfg)
				}
			}
		}

		m.mode = modeNormal
		m.message = fmt.Sprintf("✓ Applied preset: %s", selected.Name)
		return m, nil
	}

	return m, nil
}

// handleSavePresetPromptKey handles keyboard input for naming a new filter preset.
func (m Model) handleSavePresetPromptKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case keyMatches(msg, m.keys.Cancel):
		m.mode = modeFilterPresets
		return m, nil

	case keyMatches(msg, m.keys.Confirm):
		name := strings.TrimSpace(m.savePresetNameInput)
		if name == "" {
			name = "Custom Preset"
		}
		cur := m.currentTab()
		curFilter := ""
		if cur != nil {
			curFilter = cur.FilterRaw
		}
		if curFilter == "" {
			m.mode = modeFilterPresets
			m.message = "No filter expression to save"
			return m, nil
		}

		newPreset := config.FilterPreset{
			Name:   name,
			Filter: curFilter,
		}
		m.filtersCfg.Presets = append(m.filtersCfg.Presets, newPreset)
		m.presetCursor = len(m.filtersCfg.Presets) - 1
		if m.appConfig != nil {
			_ = m.appConfig.SaveFilters(m.filtersCfg)
		}

		m.mode = modeFilterPresets
		m.message = fmt.Sprintf("✓ Saved preset %q", name)
		return m, nil

	default:
		m.savePresetNameInput = handleTextInput(m.savePresetNameInput, msg)
		return m, nil
	}
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
