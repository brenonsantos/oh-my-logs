package tui

import (
	"fmt"
	"os"
	"path/filepath"
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

	case msg.String() == "d" || msg.String() == "D" || msg.String() == "x":
		m.mode = modeNormal
		return m.disconnect()

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
		m.manualDisconnect = false
		m.reconnecting = false
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
		defaultName := cur.Name
		if defaultName == "" || strings.HasPrefix(defaultName, "Tab ") {
			defaultName = cur.FilterRaw
		}
		m.savePresetNameInput.SetText(defaultName)
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
		if m.filterInput.AddHistory(selected.Filter) {
			if m.filtersCfg != nil {
				m.filtersCfg.History = m.filterInput.History
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
		name := strings.TrimSpace(m.savePresetNameInput.Value)
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
		m.savePresetNameInput.HandleKey(msg)
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

// expandHomePath expands a leading ~ or ~/ to the user's home directory.
func expandHomePath(p string) string {
	if strings.HasPrefix(p, "~/") || p == "~" {
		home, err := os.UserHomeDir()
		if err == nil {
			if p == "~" {
				return home
			}
			return filepath.Join(home, p[2:])
		}
	}
	return p
}

// handleFilePickerKey handles keyboard navigation, path typing (:), prefix editing (p), and file selection.
func (m Model) handleFilePickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.fpSubMode {
	case fpModeTypeDir:
		switch {
		case msg.String() == "esc":
			m.fpSubMode = fpModeBrowse
			m.fpMatches = nil
			m.fpMatchIndex = -1
			return m, nil

		case msg.String() == "tab" || msg.Type == tea.KeyTab:
			if len(m.fpMatches) > 0 {
				m.fpMatchIndex = (m.fpMatchIndex + 1) % len(m.fpMatches)
				completed := m.fpMatchPrefix + m.fpMatches[m.fpMatchIndex]
				m.fpDirInput.SetText(completed)
			} else {
				res := CompletePath(m.fpDirInput.Value, m.filePicker.CurrentDirectory)
				if len(res.Matches) == 1 {
					m.fpDirInput.SetText(res.Completed)
					m.fpMatches = nil
					m.fpMatchIndex = -1
				} else if len(res.Matches) > 1 {
					m.fpMatches = res.Matches
					m.fpMatchPrefix = res.InputPrefix
					if res.Completed != m.fpDirInput.Value {
						m.fpDirInput.SetText(res.Completed)
						m.fpMatchIndex = -1
					} else {
						m.fpMatchIndex = 0
						completed := m.fpMatchPrefix + m.fpMatches[0]
						m.fpDirInput.SetText(completed)
					}
				}
			}

			val := strings.TrimSpace(m.fpDirInput.Value)
			expanded := expandHomePath(val)
			if fi, err := os.Stat(expanded); err == nil && fi.IsDir() {
				cleaned := filepath.Clean(expanded)
				if cleaned != m.filePicker.CurrentDirectory {
					m.filePicker.CurrentDirectory = cleaned
					return m, m.filePicker.Init()
				}
			}
			return m, nil

		case msg.String() == "enter" || msg.Type == tea.KeyEnter:
			m.fpMatches = nil
			m.fpMatchIndex = -1
			rawPath := strings.TrimSpace(m.fpDirInput.Value)
			if rawPath != "" {
				targetDir := expandHomePath(rawPath)
				targetDir = filepath.Clean(targetDir)
				wasNew := false
				if _, err := os.Stat(targetDir); os.IsNotExist(err) {
					wasNew = true
				}
				_ = os.MkdirAll(targetDir, 0o755)
				m.filePicker.CurrentDirectory = targetDir
				if m.fpPurpose == fpPurposeDirectToDisk && m.settings != nil {
					m.settings.LogDir = targetDir
				}
				if wasNew {
					m.message = fmt.Sprintf("✓ Created and opened %s", filepath.Base(targetDir))
				}
				m.fpSubMode = fpModeBrowse
				return m, m.filePicker.Init()
			}
			m.fpSubMode = fpModeBrowse
			return m, nil

		default:
			m.fpMatches = nil
			m.fpMatchIndex = -1
			m.fpDirInput.HandleKey(msg)
			val := strings.TrimSpace(m.fpDirInput.Value)
			if val != "" {
				expanded := expandHomePath(val)
				if fi, err := os.Stat(expanded); err == nil && fi.IsDir() {
					cleaned := filepath.Clean(expanded)
					if cleaned != m.filePicker.CurrentDirectory {
						m.filePicker.CurrentDirectory = cleaned
						return m, m.filePicker.Init()
					}
				}
			}
			return m, nil
		}

	case fpModeNewFolder:
		switch {
		case msg.String() == "esc":
			m.fpSubMode = fpModeBrowse
			return m, nil

		case msg.String() == "enter" || msg.Type == tea.KeyEnter:
			folderName := strings.TrimSpace(m.fpNewFolderInput.Value)
			if folderName != "" {
				newPath := filepath.Join(m.filePicker.CurrentDirectory, folderName)
				if err := os.MkdirAll(newPath, 0o755); err != nil {
					m.message = fmt.Sprintf("Failed to create folder: %v", err)
				} else {
					m.filePicker.CurrentDirectory = newPath
					if m.fpPurpose == fpPurposeDirectToDisk && m.settings != nil {
						m.settings.LogDir = newPath
					}
					m.message = fmt.Sprintf("✓ Created and opened %s", folderName)
					m.fpSubMode = fpModeBrowse
					return m, m.filePicker.Init()
				}
			}
			m.fpSubMode = fpModeBrowse
			return m, nil

		default:
			m.fpNewFolderInput.HandleKey(msg)
			return m, nil
		}

	case fpModeTypePrefix:
		switch {
		case msg.String() == "esc":
			m.fpSubMode = fpModeBrowse
			return m, nil

		case msg.String() == "enter" || msg.Type == tea.KeyEnter:
			pref := strings.TrimSpace(m.fpPrefixInput.Value)
			if pref == "" {
				pref = "oml"
			}
			if m.fpPurpose == fpPurposeSaveLog {
				m.saveLogPrefix = pref
			} else {
				m.directToDiskPrefix = pref
				if m.settings != nil {
					m.settings.LogPrefix = pref
				}
				m.saveSettings()
			}
			m.fpSubMode = fpModeBrowse
			return m, nil

		default:
			m.fpPrefixInput.HandleKey(msg)
			return m, nil
		}

	default: // fpModeBrowse
		switch {
		case msg.String() == "esc" || msg.String() == "q":
			if m.fpPurpose == fpPurposeSaveLog {
				m.mode = modeNormal
				m.message = "Save log canceled"
			} else {
				m.mode = modeSettings
			}
			return m, nil

		case msg.String() == ":":
			m.fpSubMode = fpModeTypeDir
			m.fpMatches = nil
			m.fpMatchIndex = -1
			cur := m.filePicker.CurrentDirectory
			if cur != "" && !strings.HasSuffix(cur, "/") {
				cur += "/"
			}
			m.fpDirInput.SetText(cur)
			return m, nil

		case msg.String() == "+" || msg.String() == "n" || msg.String() == "N":
			m.fpSubMode = fpModeNewFolder
			m.fpNewFolderInput.SetText("")
			return m, nil

		case msg.String() == "p" || msg.String() == "P":
			m.fpSubMode = fpModeTypePrefix
			m.fpPrefixInput.SetText(m.currentFilePickerPrefix())
			return m, nil

		case msg.String() == " " || msg.String() == "s" || msg.String() == "S":
			// Select current directory as target log folder
			return m.handleFilePickerSelected(m.filePicker.CurrentDirectory)

		default:
			var cmd tea.Cmd
			m.filePicker, cmd = m.filePicker.Update(msg)
			if didSelect, path := m.filePicker.DidSelectFile(msg); didSelect {
				return m.handleFilePickerSelected(path)
			}
			if didSelectDisabled, path := m.filePicker.DidSelectDisabledFile(msg); didSelectDisabled {
				m.message = fmt.Sprintf("Selected item %s cannot be used", filepath.Base(path))
			}
			return m, cmd
		}
	}
}

// handleFilePickerSelected processes a selected file or directory path.
func (m Model) handleFilePickerSelected(path string) (Model, tea.Cmd) {
	if path == "" {
		if m.fpPurpose == fpPurposeSaveLog {
			m.mode = modeNormal
		} else {
			m.mode = modeSettings
		}
		return m, nil
	}

	prefix := m.currentFilePickerPrefix()

	fi, err := os.Stat(path)
	var targetPath string
	if err == nil && fi.IsDir() {
		targetPath = GenerateTimestampLogPathWithPrefix(path, prefix)
	} else {
		targetPath = path
	}

	if m.fpPurpose == fpPurposeSaveLog {
		// Decoupled: One-off log save does NOT mutate direct-to-disk settings
		m.mode = modeNormal
		cmd := m.cmdSaveLogToPath(targetPath)
		return m, cmd
	}

	// Direct-to-disk continuous logging updates persistent settings
	if err == nil && fi.IsDir() {
		if m.settings != nil {
			m.settings.LogDir = path
			m.settings.LogPrefix = prefix
		}
	} else {
		if m.settings != nil {
			m.settings.LogDir = filepath.Dir(path)
		}
	}
	m.saveSettings()

	m.directToDiskPath = targetPath
	if m.settings != nil && m.settings.DirectToDisk {
		if err := m.StartDiskLogger(m.directToDiskPath); err != nil {
			m.message = fmt.Sprintf("Failed to redirect logger: %v", err)
		} else {
			m.message = fmt.Sprintf("Logging to %s", m.diskLogger.Filename())
		}
	} else {
		m.message = fmt.Sprintf("Log destination set: %s", filepath.Base(m.directToDiskPath))
	}

	m.mode = modeSettings
	return m, nil
}
