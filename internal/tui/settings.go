package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/brenoniehues/oh-my-logs/internal/config"
	"github.com/brenoniehues/oh-my-logs/internal/serial"
	"github.com/charmbracelet/bubbles/filepicker"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

type settingRow int

const (
	settingRowBufferCap settingRow = iota
	settingRowBaud
	settingRowTXEnding
	settingRowTimestamp
	settingRowFormat
	settingRowFollow
	settingRowDirectToDisk
	settingRowLogPath
	settingRowTheme
	settingRowCount
)

var (
	bufferCapOptions = []int{10_000, 25_000, 50_000, 100_000, 250_000}
	baudOptions      = []int{9600, 19200, 38400, 57600, 115200, 230400, 460800, 921600}
	txEndingOptions  = []serial.LineEnding{serial.EndingCRLF, serial.EndingLF, serial.EndingCR, serial.EndingNone}
	timestampOptions = []TimestampMode{TSModeClock, TSModeDelta, TSModeBoth, TSModeOff}
	formatOptions    = []DisplayFormat{FormatParsed, FormatRaw, FormatHex, FormatBinary}
)

func (m Model) ensureSettings() *config.Settings {
	if m.settings == nil {
		return &config.Settings{
			Baud:           115200,
			BufferCapacity: 50000,
			DefaultFollow:  true,
			Theme:          PaletteDarkSlate.Name,
		}
	}
	if m.settings.BufferCapacity <= 0 {
		m.settings.BufferCapacity = 50000
	}
	if m.settings.Theme == "" {
		m.settings.Theme = PaletteDarkSlate.Name
	}
	return m.settings
}

func (m Model) settingRowName(row settingRow) string {
	switch row {
	case settingRowBufferCap:
		return "Buffer Capacity"
	case settingRowBaud:
		return "Default Baud Rate"
	case settingRowTXEnding:
		return "TX Line Ending"
	case settingRowTimestamp:
		return "Timestamp Mode"
	case settingRowFormat:
		return "Display Format"
	case settingRowFollow:
		return "Auto-Follow on Launch"
	case settingRowDirectToDisk:
		return "Direct-to-Disk Stream"
	case settingRowLogPath:
		return "Logs Destination"
	case settingRowTheme:
		return "Theme Palette"
	default:
		return "Unknown"
	}
}

func (m Model) settingRowDescription(row settingRow) string {
	switch row {
	case settingRowBufferCap:
		return "Maximum log records retained in memory ring buffer"
	case settingRowBaud:
		return "Default communication baud rate for serial connection"
	case settingRowTXEnding:
		return "Line terminator appended when transmitting commands"
	case settingRowTimestamp:
		return "Display format for arrival timing and inter-log latency (Δt)"
	case settingRowFormat:
		return "Log data representation: parsed columns, raw text, hex, or binary"
	case settingRowFollow:
		return "Automatically follow newest incoming logs upon startup"
	case settingRowDirectToDisk:
		return "Continuous unbuffered disk tee for streaming and soak logging"
	case settingRowLogPath:
		return "Default folder and destination for log exports and direct-to-disk logging"
	case settingRowTheme:
		return "Active color scheme across all tables, bars, modals, and tabs"
	default:
		return ""
	}
}

func txEndingLabel(e serial.LineEnding) string {
	switch e {
	case serial.EndingCRLF:
		return "CRLF (\\r\\n)"
	case serial.EndingLF:
		return "LF (\\n)"
	case serial.EndingCR:
		return "CR (\\r)"
	case serial.EndingNone:
		return "None"
	default:
		return e.String()
	}
}

func (m Model) settingValueLabel(row settingRow) string {
	s := m.ensureSettings()
	switch row {
	case settingRowBufferCap:
		capVal := 50000
		if m.buffer != nil {
			capVal = m.buffer.Cap()
		} else if s.BufferCapacity > 0 {
			capVal = s.BufferCapacity
		}
		if capVal%1000 == 0 {
			return fmt.Sprintf("%dk records", capVal/1000)
		}
		return fmt.Sprintf("%d records", capVal)

	case settingRowBaud:
		baudVal := m.serialCfg.Baud
		if baudVal <= 0 {
			baudVal = s.Baud
		}
		if baudVal <= 0 {
			baudVal = 115200
		}
		return fmt.Sprintf("%d", baudVal)

	case settingRowTXEnding:
		return txEndingLabel(m.txEnding)

	case settingRowTimestamp:
		switch m.tsMode {
		case TSModeClock:
			return "Clock Time"
		case TSModeDelta:
			return "Delta-t (Δt)"
		case TSModeBoth:
			return "Clock + Δt"
		case TSModeOff:
			return "Off"
		default:
			return "Clock Time"
		}

	case settingRowFormat:
		return m.displayFormat.Label()

	case settingRowFollow:
		if s.DefaultFollow {
			return "Enabled"
		}
		return "Disabled"

	case settingRowDirectToDisk:
		if s.DirectToDisk {
			return "Enabled"
		}
		return "Disabled"

	case settingRowLogPath:
		if m.diskLogger != nil && m.diskLogger.IsActive() {
			return m.diskLogger.Filename()
		}
		if m.directToDiskPath != "" {
			return filepath.Base(m.directToDiskPath)
		}
		if s.LogDir != "" {
			return filepath.Base(s.LogDir)
		}
		return "(auto in logs/)"

	case settingRowTheme:
		if s.Theme != "" {
			return s.Theme
		}
		return CurrentThemeName()

	default:
		return ""
	}
}

func (m Model) adjustSetting(row settingRow, delta int) Model {
	m.settings = m.ensureSettings()

	switch row {
	case settingRowBufferCap:
		curCap := 50000
		if m.buffer != nil {
			curCap = m.buffer.Cap()
		} else if m.settings.BufferCapacity > 0 {
			curCap = m.settings.BufferCapacity
		}
		curIdx := 2
		for i, c := range bufferCapOptions {
			if c == curCap {
				curIdx = i
				break
			}
		}
		nextIdx := (curIdx + delta + len(bufferCapOptions)) % len(bufferCapOptions)
		newCap := bufferCapOptions[nextIdx]
		if m.buffer != nil {
			m.buffer.Resize(newCap)
		}
		m.settings.BufferCapacity = newCap
		m.message = fmt.Sprintf("Buffer capacity set to %dk records", newCap/1000)

	case settingRowBaud:
		curBaud := m.serialCfg.Baud
		if curBaud <= 0 {
			curBaud = m.settings.Baud
		}
		curIdx := 4
		for i, b := range baudOptions {
			if b == curBaud {
				curIdx = i
				break
			}
		}
		nextIdx := (curIdx + delta + len(baudOptions)) % len(baudOptions)
		newBaud := baudOptions[nextIdx]
		m.serialCfg.Baud = newBaud
		m.settings.Baud = newBaud
		m.message = fmt.Sprintf("Default baud rate set to %d", newBaud)

	case settingRowTXEnding:
		curIdx := 0
		for i, e := range txEndingOptions {
			if e == m.txEnding {
				curIdx = i
				break
			}
		}
		nextIdx := (curIdx + delta + len(txEndingOptions)) % len(txEndingOptions)
		newEnding := txEndingOptions[nextIdx]
		m.txEnding = newEnding
		m.settings.TXEnding = newEnding.String()
		m.message = fmt.Sprintf("TX line ending set to %s", txEndingLabel(newEnding))

	case settingRowTimestamp:
		curIdx := 0
		for i, t := range timestampOptions {
			if t == m.tsMode {
				curIdx = i
				break
			}
		}
		nextIdx := (curIdx + delta + len(timestampOptions)) % len(timestampOptions)
		newTS := timestampOptions[nextIdx]
		m.tsMode = newTS
		m.showTimestamp = (newTS != TSModeOff)
		m.settings.TimestampMode = newTS.String()
		m.settings.ShowTimestamp = m.showTimestamp
		m.message = fmt.Sprintf("Timestamp mode set to %s", m.settingValueLabel(settingRowTimestamp))

	case settingRowFormat:
		curIdx := 0
		for i, f := range formatOptions {
			if f == m.displayFormat {
				curIdx = i
				break
			}
		}
		nextIdx := (curIdx + delta + len(formatOptions)) % len(formatOptions)
		newFmt := formatOptions[nextIdx]
		m.displayFormat = newFmt
		if cur := m.currentTab(); cur != nil {
			cur.DisplayFormat = newFmt
		}
		m.settings.DisplayFormat = newFmt.String()
		m.message = fmt.Sprintf("Display format set to %s", newFmt.Label())

	case settingRowFollow:
		m.settings.DefaultFollow = !m.settings.DefaultFollow
		if m.settings.DefaultFollow {
			m.message = "Auto-follow on launch enabled"
		} else {
			m.message = "Auto-follow on launch disabled"
		}

	case settingRowDirectToDisk:
		m.settings.DirectToDisk = !m.settings.DirectToDisk
		if m.settings.DirectToDisk {
			if err := m.StartDiskLogger(m.directToDiskPath); err != nil {
				m.message = fmt.Sprintf("Failed to start disk logger: %v", err)
				m.settings.DirectToDisk = false
			} else {
				m.message = fmt.Sprintf("Direct-to-disk stream started: %s", m.diskLogger.Filename())
			}
		} else {
			m.StopDiskLogger()
			m.message = "Direct-to-disk stream disabled"
		}

	case settingRowTheme:
		opts := AvailableThemes()
		curTheme := m.settings.Theme
		if curTheme == "" {
			curTheme = CurrentThemeName()
		}
		curIdx := 0
		for i, name := range opts {
			if strings.EqualFold(normalizeThemeName(name), normalizeThemeName(curTheme)) {
				curIdx = i
				break
			}
		}
		nextIdx := (curIdx + delta + len(opts)) % len(opts)
		newTheme := opts[nextIdx]
		m.settings.Theme = newTheme
		SetCurrentTheme(newTheme)
		m.message = fmt.Sprintf("Theme set to %s", newTheme)
	}

	m.saveSettings()
	return m
}

// currentFilePickerPrefix returns the active prefix based on modal purpose.
func (m Model) currentFilePickerPrefix() string {
	if m.fpPurpose == fpPurposeSaveLog {
		if m.saveLogPrefix != "" {
			return m.saveLogPrefix
		}
		return "oml"
	}
	if m.directToDiskPrefix != "" {
		return m.directToDiskPrefix
	}
	return "oml"
}

// filePickerModalWidth computes a responsive width for the file picker modal based on terminal width.
func (m Model) filePickerModalWidth() int {
	target := int(float64(m.width) * 0.72)
	if target < 86 {
		target = 86
	}
	if target > 116 {
		target = 116
	}
	if target > m.width-4 {
		target = m.width - 4
	}
	if target < 40 {
		target = 40
	}
	return target
}

// filePickerHeight computes a responsive height for the file list based on available vertical space.
func (m Model) filePickerHeight() int {
	avail := m.tableHeight + 2
	if m.height > 0 && m.height-14 > avail {
		avail = m.height - 14
	}
	fpH := avail - 11
	if fpH < 8 {
		fpH = 8
	}
	if fpH > 26 {
		fpH = 26
	}
	return fpH
}

// openFilePickerWithPurpose initializes and displays the file/folder browser modal for the specified purpose.
func (m Model) openFilePickerWithPurpose(purpose filePickerPurpose) (Model, tea.Cmd) {
	fp := filepicker.New()
	dir := ""
	if purpose == fpPurposeSaveLog {
		// Log export snapshot always starts at the current launch / working directory
		dir, _ = os.Getwd()
	} else {
		// Direct-to-disk continuous logging starts at stable configured settings
		if m.directToDiskPath != "" {
			dir = filepath.Dir(m.directToDiskPath)
		} else if m.settings != nil && m.settings.LogDir != "" {
			dir = m.settings.LogDir
		} else if m.appConfig != nil && m.appConfig.LogsDir != "" {
			dir = m.appConfig.LogsDir
		}
	}
	if dir == "" {
		dir, _ = os.Getwd()
	}
	if dir == "" {
		dir = "."
	}
	_ = os.MkdirAll(dir, 0o755)
	fp.CurrentDirectory = dir
	fp.DirAllowed = false
	fp.FileAllowed = true
	fp.ShowHidden = false
	fp.AutoHeight = false
	fp.SetHeight(m.filePickerHeight())
	fp.KeyMap.Back = key.NewBinding(key.WithKeys("h", "backspace", "left"), key.WithHelp("h/←", "back"))
	m.filePicker = fp
	m.fpPurpose = purpose
	m.fpSubMode = fpModeBrowse
	m.fpMatches = nil
	m.fpMatchIndex = -1
	m.mode = modeFilePicker
	return m, m.filePicker.Init()
}

// openFilePicker initializes and displays the file/folder browser modal for direct-to-disk logging.
func (m Model) openFilePicker() (Model, tea.Cmd) {
	return m.openFilePickerWithPurpose(fpPurposeDirectToDisk)
}

// openSaveLogPicker initializes and displays the file/folder browser modal for exporting/saving logs.
func (m Model) openSaveLogPicker() (Model, tea.Cmd) {
	return m.openFilePickerWithPurpose(fpPurposeSaveLog)
}

// handleSettingsKey handles keyboard navigation and option selection in the Settings modal.
func (m Model) handleSettingsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.String() == "esc" || msg.String() == "q" || msg.String() == "," || msg.String() == "C" || msg.Type == tea.KeyEscape:
		m.mode = modeNormal
		return m, nil

	case msg.Type == tea.KeyUp || msg.String() == "k":
		m.settingsCursor--
		if m.settingsCursor < 0 {
			m.settingsCursor = int(settingRowCount) - 1
		}
		return m, nil

	case msg.Type == tea.KeyDown || msg.String() == "j":
		m.settingsCursor++
		if m.settingsCursor >= int(settingRowCount) {
			m.settingsCursor = 0
		}
		return m, nil

	case msg.Type == tea.KeyLeft || msg.String() == "h":
		if settingRow(m.settingsCursor) == settingRowLogPath {
			return m.openFilePicker()
		}
		m = m.adjustSetting(settingRow(m.settingsCursor), -1)
		return m, nil

	case msg.Type == tea.KeyRight || msg.String() == "l" || msg.String() == " ":
		if settingRow(m.settingsCursor) == settingRowLogPath {
			return m.openFilePicker()
		}
		m = m.adjustSetting(settingRow(m.settingsCursor), 1)
		return m, nil

	case msg.Type == tea.KeyEnter || msg.String() == "enter":
		if settingRow(m.settingsCursor) == settingRowLogPath {
			return m.openFilePicker()
		}
		m.mode = modeNormal
		return m, nil
	}
	return m, nil
}

// viewSettingsModal renders the interactive Settings & Preferences modal.
func (m Model) viewSettingsModal() string {
	modalWidth := 66
	if modalWidth > m.width-4 {
		modalWidth = m.width - 4
	}

	var sb strings.Builder

	sb.WriteString(theme.ModalTitle.Render("⚙  Settings & Preferences"))
	sb.WriteString("\n\n")

	for r := 0; r < int(settingRowCount); r++ {
		row := settingRow(r)
		name := m.settingRowName(row)
		val := m.settingValueLabel(row)

		isSelected := (r == m.settingsCursor)

		var line string
		if isSelected {
			valFormatted := fmt.Sprintf("◀  %s  ▶", val)
			// Calculate padding for two-column alignment
			col1Width := 28
			pad := col1Width - len(name)
			if pad < 2 {
				pad = 2
			}
			line = fmt.Sprintf("  › %s%s%s",
				theme.ModalSelected.Render(name),
				strings.Repeat(" ", pad),
				theme.Accent.Bold(true).Render(valFormatted),
			)
		} else {
			valFormatted := fmt.Sprintf("   %s   ", val)
			col1Width := 28
			pad := col1Width - len(name)
			if pad < 2 {
				pad = 2
			}
			line = fmt.Sprintf("    %s%s%s",
				theme.ModalItem.Render(name),
				strings.Repeat(" ", pad),
				theme.Muted.Render(valFormatted),
			)
		}

		sb.WriteString(line)
		sb.WriteString("\n")
	}

	sb.WriteString("\n")

	// Description of active setting
	activeDesc := m.settingRowDescription(settingRow(m.settingsCursor))
	if activeDesc != "" {
		sb.WriteString("  " + theme.Muted.Render(activeDesc))
		sb.WriteString("\n\n")
	}

	// Footer hints
	sb.WriteString(theme.ModalFooter.Render("Enter/Esc close · ↑/↓ select · ←/→ adjust · Space toggle"))

	modalBox := theme.ModalBox.Width(modalWidth).Render(sb.String())
	return centerBox(m.width, m.tableHeight+2, modalBox)
}

// saveSettings persists current port, baud, profile, and timestamp display state to disk.
func (m Model) saveSettings() {
	if m.appConfig == nil {
		return
	}
	s := m.settings
	if s == nil {
		s = &config.Settings{}
	}
	s.Port = m.serialCfg.Port
	s.Baud = m.serialCfg.Baud
	if m.profile != nil {
		s.Profile = m.profile.Name
	} else {
		s.Profile = ""
	}
	s.ShowTimestamp = (m.tsMode != TSModeOff)
	s.TimestampMode = m.tsMode.String()
	s.TXEnding = m.txEnding.String()
	s.TXHistory = m.txInput.History
	if m.buffer != nil {
		s.BufferCapacity = m.buffer.Cap()
	}
	if m.settings != nil {
		s.DefaultFollow = m.settings.DefaultFollow
		s.DirectToDisk = m.settings.DirectToDisk
		s.LogDir = m.settings.LogDir
		s.Theme = m.settings.Theme
	}
	_ = m.appConfig.SaveSettings(s)
}

// loadFilters loads filter presets and history from config, or populates defaults.
func (m *Model) loadFilters() {
	if m.filtersCfg != nil {
		return
	}
	if m.appConfig != nil {
		if fc, err := m.appConfig.LoadFilters(); err == nil && fc != nil {
			m.filtersCfg = fc
			if len(m.filterInput.History) == 0 {
				m.filterInput.History = fc.History
			}
			return
		}
	}
	m.filtersCfg = &config.FiltersConfig{
		Presets: config.DefaultFilterPresets(),
		History: []string{},
	}
}
