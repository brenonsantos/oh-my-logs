package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brenoniehues/oh-my-logs/internal/config"
	"github.com/brenoniehues/oh-my-logs/internal/record"
	"github.com/brenoniehues/oh-my-logs/internal/serial"
	tea "github.com/charmbracelet/bubbletea"
)

func createTestModelWithConfig(t *testing.T) (Model, string) {
	t.Helper()
	tempDir, err := os.MkdirTemp("", "oml-settings-tui-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	appCfg := &config.AppConfig{
		ConfigDir:   tempDir,
		ProfilesDir: filepath.Join(tempDir, "profiles"),
		LogsDir:     filepath.Join(tempDir, "logs"),
	}

	m := newTestModel()
	m.appConfig = appCfg
	m.settings = &config.Settings{
		Baud:           115200,
		BufferCapacity: 50000,
		DefaultFollow:  true,
		Theme:          "dark-slate",
	}
	m.buffer = record.NewBuffer(50000)
	m.serialCfg = serial.Config{Port: "/dev/ttyUSB0", Baud: 115200}
	m.txEnding = serial.EndingCRLF
	m.tsMode = TSModeClock
	m.showTimestamp = true
	return m, tempDir
}

func TestSettingsModal_OpenAndDismiss(t *testing.T) {
	m, tempDir := createTestModelWithConfig(t)
	defer os.RemoveAll(tempDir)

	// 1. Open with ','
	mMod, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{','}})
	m = mMod.(Model)
	if m.mode != modeSettings {
		t.Fatalf("expected modeSettings after ',', got %v", m.mode)
	}

	// 2. Dismiss with 'esc'
	mMod, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = mMod.(Model)
	if m.mode != modeNormal {
		t.Fatalf("expected modeNormal after Esc, got %v", m.mode)
	}

	// 3. Open with 'C'
	mMod, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}})
	m = mMod.(Model)
	if m.mode != modeSettings {
		t.Fatalf("expected modeSettings after 'C', got %v", m.mode)
	}

	// 4. Dismiss with 'q'
	mMod, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	m = mMod.(Model)
	if m.mode != modeNormal {
		t.Fatalf("expected modeNormal after 'q', got %v", m.mode)
	}

	// 5. Open and dismiss with Enter
	mMod, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{','}})
	m = mMod.(Model)
	mMod, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mMod.(Model)
	if m.mode != modeNormal {
		t.Fatalf("expected modeNormal after Enter, got %v", m.mode)
	}
}

func TestSettingsModal_NavigationWrapAround(t *testing.T) {
	m, tempDir := createTestModelWithConfig(t)
	defer os.RemoveAll(tempDir)

	m.mode = modeSettings
	m.settingsCursor = 0

	// Move Up should wrap to bottom
	mMod, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = mMod.(Model)
	if m.settingsCursor != int(settingRowCount)-1 {
		t.Fatalf("expected cursor wrap to %d, got %d", int(settingRowCount)-1, m.settingsCursor)
	}

	// Move Down should wrap back to top (0)
	mMod, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = mMod.(Model)
	if m.settingsCursor != 0 {
		t.Fatalf("expected cursor wrap to 0, got %d", m.settingsCursor)
	}

	// Down with 'j'
	mMod, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = mMod.(Model)
	if m.settingsCursor != 1 {
		t.Fatalf("expected cursor 1 after 'j', got %d", m.settingsCursor)
	}

	// Up with 'k'
	mMod, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m = mMod.(Model)
	if m.settingsCursor != 0 {
		t.Fatalf("expected cursor 0 after 'k', got %d", m.settingsCursor)
	}
}

func TestSettingsModal_AdjustBufferCapacity(t *testing.T) {
	m, tempDir := createTestModelWithConfig(t)
	defer os.RemoveAll(tempDir)

	m.mode = modeSettings
	m.settingsCursor = int(settingRowBufferCap)

	// Ingest sample records to verify retention
	m.buffer.Add(record.Record{Raw: "log 1"})
	m.buffer.Add(record.Record{Raw: "log 2"})

	// Buffer starts at 50,000 (index 2 in bufferCapOptions: 10k, 25k, 50k, 100k, 250k)
	// Press 'l' (right) -> should advance to 100,000
	mMod, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	m = mMod.(Model)

	if m.buffer.Cap() != 100000 {
		t.Fatalf("expected buffer cap 100000, got %d", m.buffer.Cap())
	}
	if m.settings.BufferCapacity != 100000 {
		t.Fatalf("expected settings.BufferCapacity 100000, got %d", m.settings.BufferCapacity)
	}
	if m.buffer.Len() != 2 {
		t.Fatalf("expected records preserved, got len %d", m.buffer.Len())
	}

	// Verify persistence file updated on disk
	saved, err := m.appConfig.LoadSettings()
	if err != nil {
		t.Fatalf("failed to reload settings: %v", err)
	}
	if saved.BufferCapacity != 100000 {
		t.Fatalf("expected persisted BufferCapacity 100000, got %d", saved.BufferCapacity)
	}
}

func TestSettingsModal_AdjustBaudAndTXEnding(t *testing.T) {
	m, tempDir := createTestModelWithConfig(t)
	defer os.RemoveAll(tempDir)

	m.mode = modeSettings

	// 1. Adjust Baud Rate (row 1)
	m.settingsCursor = int(settingRowBaud)
	// Initial baud is 115200 (index 4). Press right -> 230400
	mMod, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = mMod.(Model)
	if m.serialCfg.Baud != 230400 {
		t.Fatalf("expected baud 230400, got %d", m.serialCfg.Baud)
	}
	if m.settings.Baud != 230400 {
		t.Fatalf("expected settings baud 230400, got %d", m.settings.Baud)
	}

	// 2. Adjust TX Line Ending (row 2)
	m.settingsCursor = int(settingRowTXEnding)
	// Initial ending is CRLF (index 0). Press right -> LF
	mMod, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = mMod.(Model)
	if m.txEnding != serial.EndingLF {
		t.Fatalf("expected EndingLF, got %v", m.txEnding)
	}
	if m.settings.TXEnding != "LF" {
		t.Fatalf("expected settings TXEnding 'LF', got %s", m.settings.TXEnding)
	}

	// Press left -> CRLF
	mMod, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m = mMod.(Model)
	if m.txEnding != serial.EndingCRLF {
		t.Fatalf("expected EndingCRLF, got %v", m.txEnding)
	}
}

func TestSettingsModal_ToggleFollowAndDirectToDisk(t *testing.T) {
	m, tempDir := createTestModelWithConfig(t)
	defer os.RemoveAll(tempDir)

	m.mode = modeSettings

	// 1. Follow row (row 4)
	m.settingsCursor = int(settingRowFollow)
	if !m.settings.DefaultFollow {
		t.Fatalf("expected DefaultFollow true initially")
	}
	// Press Space to toggle
	mMod, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	m = mMod.(Model)
	if m.settings.DefaultFollow {
		t.Fatalf("expected DefaultFollow false after toggle")
	}

	// 2. Direct-to-Disk row (row 5)
	m.settingsCursor = int(settingRowDirectToDisk)
	if m.settings.DirectToDisk {
		t.Fatalf("expected DirectToDisk false initially")
	}
	// Press Space to toggle
	mMod, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	m = mMod.(Model)
	if !m.settings.DirectToDisk {
		t.Fatalf("expected DirectToDisk true after toggle")
	}
}

func TestSettingsModal_Rendering(t *testing.T) {
	m, tempDir := createTestModelWithConfig(t)
	defer os.RemoveAll(tempDir)

	m.width = 90
	m.height = 30
	m.tableHeight = 23
	m.mode = modeSettings
	m.settingsCursor = 0

	view := m.viewSettingsModal()

	if !strings.Contains(view, "Settings & Preferences") {
		t.Errorf("expected view to contain modal title, got:\n%s", view)
	}
	if !strings.Contains(view, "Buffer Capacity") {
		t.Errorf("expected view to contain 'Buffer Capacity', got:\n%s", view)
	}
	if !strings.Contains(view, "Default Baud Rate") {
		t.Errorf("expected view to contain 'Default Baud Rate', got:\n%s", view)
	}
	if !strings.Contains(view, "TX Line Ending") {
		t.Errorf("expected view to contain 'TX Line Ending', got:\n%s", view)
	}
	if !strings.Contains(view, "Auto-Follow on Launch") {
		t.Errorf("expected view to contain 'Auto-Follow on Launch', got:\n%s", view)
	}
	if !strings.Contains(view, "Direct-to-Disk Stream") {
		t.Errorf("expected view to contain 'Direct-to-Disk Stream', got:\n%s", view)
	}
	if !strings.Contains(view, "Logs Destination") {
		t.Errorf("expected view to contain 'Logs Destination', got:\n%s", view)
	}
	if !strings.Contains(view, "Theme Palette") {
		t.Errorf("expected view to contain 'Theme Palette', got:\n%s", view)
	}
	if !strings.Contains(view, "Enter/Esc close") {
		t.Errorf("expected view to contain footer hints, got:\n%s", view)
	}
}

func TestSettingsModal_ThemeSelection(t *testing.T) {
	m, tempDir := createTestModelWithConfig(t)
	defer os.RemoveAll(tempDir)
	defer SetCurrentTheme("Dark Slate")

	m.mode = modeSettings
	m.settingsCursor = int(settingRowTheme)

	// Verify initial theme
	initialTheme := m.settingValueLabel(settingRowTheme)
	if initialTheme != "Dark Slate" && initialTheme != "dark-slate" {
		t.Fatalf("expected initial theme 'Dark Slate', got %s", initialTheme)
	}

	// Press right -> Monokai
	mMod, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = mMod.(Model)
	if m.settings.Theme != "Monokai" {
		t.Fatalf("expected theme 'Monokai', got %s", m.settings.Theme)
	}
	if CurrentThemeName() != "Monokai" {
		t.Fatalf("expected CurrentThemeName 'Monokai', got %s", CurrentThemeName())
	}

	// Press right -> Nord
	mMod, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = mMod.(Model)
	if m.settings.Theme != "Nord" {
		t.Fatalf("expected theme 'Nord', got %s", m.settings.Theme)
	}

	// Press left -> Monokai
	mMod, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m = mMod.(Model)
	if m.settings.Theme != "Monokai" {
		t.Fatalf("expected theme 'Monokai' after left arrow, got %s", m.settings.Theme)
	}
}

func TestSettingsModal_DirectToDisk_StreamingAndBadge(t *testing.T) {
	m, tempDir := createTestModelWithConfig(t)
	defer os.RemoveAll(tempDir)

	m.mode = modeSettings
	m.settingsCursor = int(settingRowDirectToDisk)

	// Toggle ON
	mMod, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	m = mMod.(Model)
	if !m.settings.DirectToDisk {
		t.Fatalf("expected DirectToDisk true after toggle")
	}
	if m.diskLogger == nil || !m.diskLogger.IsActive() {
		t.Fatalf("expected active diskLogger")
	}

	// Verify title bar contains REC badge
	m.width = 150
	titleBar := m.viewTitleBar()
	if !strings.Contains(titleBar, "🔴 REC:") {
		t.Fatalf("expected title bar to contain REC badge, got: %s", titleBar)
	}

	// Ingest record
	rec := record.NewRecord("TEST_RAW_LOG_ENTRY_STREAMING_001")
	m.ingestRecord(rec)

	// Toggle OFF
	mMod, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	m = mMod.(Model)
	if m.settings.DirectToDisk {
		t.Fatalf("expected DirectToDisk false after toggle off")
	}
	if m.diskLogger != nil {
		t.Fatalf("expected diskLogger nil after toggle off")
	}

	// Verify title bar no longer contains REC badge
	titleBarAfter := m.viewTitleBar()
	if strings.Contains(titleBarAfter, "🔴 REC:") {
		t.Fatalf("expected title bar to not contain REC badge, got: %s", titleBarAfter)
	}
}

func TestSettingsModal_FilePicker_NavigationAndSelect(t *testing.T) {
	m, tempDir := createTestModelWithConfig(t)
	defer os.RemoveAll(tempDir)

	m.mode = modeSettings
	m.settingsCursor = int(settingRowLogPath)

	// Press Enter to open file picker
	mMod, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mMod.(Model)
	if m.mode != modeFilePicker {
		t.Fatalf("expected modeFilePicker, got %v", m.mode)
	}
	if cmd == nil {
		t.Fatalf("expected non-nil Init cmd from filepicker")
	}

	// Verify file picker modal view renders without panic
	m.width = 80
	m.tableHeight = 20
	view := m.viewFilePickerModal()
	if !strings.Contains(view, "Select Log Destination") {
		t.Fatalf("expected file picker view to contain title, got: %s", view)
	}

	// Test Esc cancels back to modeSettings
	mMod, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = mMod.(Model)
	if m.mode != modeSettings {
		t.Fatalf("expected return to modeSettings after Esc, got %v", m.mode)
	}

	// Reopen file picker
	m.settingsCursor = int(settingRowLogPath)
	mMod, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mMod.(Model)

	// Test ':' to type directory
	mMod, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{':'}})
	m = mMod.(Model)
	if m.fpSubMode != fpModeTypeDir {
		t.Fatalf("expected fpModeTypeDir after ':', got %v", m.fpSubMode)
	}
	dirView := m.viewFilePickerModal()
	if !strings.Contains(dirView, "Go to Directory:") {
		t.Fatalf("expected dirView to contain 'Go to Directory:', got: %s", dirView)
	}

	newDir := filepath.Join(tempDir, "custom_logs")
	m.fpDirInput.SetText(newDir)
	mMod, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mMod.(Model)
	if m.fpSubMode != fpModeBrowse {
		t.Fatalf("expected return to fpModeBrowse after Enter, got %v", m.fpSubMode)
	}
	if m.filePicker.CurrentDirectory != newDir {
		t.Fatalf("expected filePicker.CurrentDirectory %s, got %s", newDir, m.filePicker.CurrentDirectory)
	}

	// Test 'p' to type prefix
	mMod, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	m = mMod.(Model)
	if m.fpSubMode != fpModeTypePrefix {
		t.Fatalf("expected fpModeTypePrefix after 'p', got %v", m.fpSubMode)
	}
	prefView := m.viewFilePickerModal()
	if !strings.Contains(prefView, "Filename Prefix:") {
		t.Fatalf("expected prefView to contain 'Filename Prefix:', got: %s", prefView)
	}

	m.fpPrefixInput.SetText("node_sensor")
	mMod, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mMod.(Model)
	if m.fpSubMode != fpModeBrowse {
		t.Fatalf("expected return to fpModeBrowse after Enter, got %v", m.fpSubMode)
	}
	if m.DirectToDiskPrefix() != "node_sensor" {
		t.Fatalf("expected prefix 'node_sensor', got %s", m.DirectToDiskPrefix())
	}

	// Test selecting directory with Space
	mMod, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	m = mMod.(Model)
	if m.mode != modeSettings {
		t.Fatalf("expected modeSettings after selecting folder, got %v", m.mode)
	}
	if !strings.Contains(m.directToDiskPath, "node_sensor-") {
		t.Fatalf("expected directToDiskPath to contain 'node_sensor-', got %s", m.directToDiskPath)
	}
	if !strings.Contains(m.directToDiskPath, "custom_logs") {
		t.Fatalf("expected directToDiskPath to be in 'custom_logs', got %s", m.directToDiskPath)
	}

	// Reopen file picker and test Tab autocomplete
	m.mode = modeSettings
	m.settingsCursor = int(settingRowLogPath)
	mMod, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mMod.(Model)

	mMod, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{':'}})
	m = mMod.(Model)

	subDir := filepath.Join(tempDir, "auto_complete_target")
	_ = os.MkdirAll(subDir, 0o755)
	partial := filepath.Join(tempDir, "auto_comp")
	m.fpDirInput.SetText(partial)

	mMod, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = mMod.(Model)
	if !strings.Contains(m.fpDirInput.Value, "auto_complete_target") {
		t.Fatalf("expected Tab autocomplete to complete to 'auto_complete_target', got %s", m.fpDirInput.Value)
	}

	// Verify that view shows file list contents while typing
	typeDirView := m.viewFilePickerModal()
	if !strings.Contains(typeDirView, "Contents of") {
		t.Fatalf("expected typeDirView to show directory contents, got: %s", typeDirView)
	}

	// Cancel out to browse mode
	mMod, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = mMod.(Model)
	if m.fpSubMode != fpModeBrowse {
		t.Fatalf("expected return to fpModeBrowse, got %v", m.fpSubMode)
	}

	// Test creating new folder with 'n'
	mMod, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m = mMod.(Model)
	if m.fpSubMode != fpModeNewFolder {
		t.Fatalf("expected fpModeNewFolder after 'n', got %v", m.fpSubMode)
	}
	newFolderView := m.viewFilePickerModal()
	if !strings.Contains(newFolderView, "New Folder Name:") {
		t.Fatalf("expected newFolderView to contain 'New Folder Name:', got: %s", newFolderView)
	}

	m.fpNewFolderInput.SetText("session_logs")
	mMod, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mMod.(Model)
	if m.fpSubMode != fpModeBrowse {
		t.Fatalf("expected return to fpModeBrowse after creating folder, got %v", m.fpSubMode)
	}
	createdDir := filepath.Join(tempDir, "auto_complete_target", "session_logs")
	if fi, err := os.Stat(createdDir); err != nil || !fi.IsDir() {
		t.Fatalf("expected session_logs folder to be created on disk at %s", createdDir)
	}
	if m.filePicker.CurrentDirectory != createdDir {
		t.Fatalf("expected filePicker.CurrentDirectory to be %s, got %s", createdDir, m.filePicker.CurrentDirectory)
	}

	// Test creating new directory by typing in ':' and pressing Enter
	mMod, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{':'}})
	m = mMod.(Model)
	directCreatedDir := filepath.Join(tempDir, "brand_new_dir")
	m.fpDirInput.SetText(directCreatedDir)
	mMod, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mMod.(Model)
	if fi, err := os.Stat(directCreatedDir); err != nil || !fi.IsDir() {
		t.Fatalf("expected brand_new_dir to be created on disk at %s", directCreatedDir)
	}
	if m.filePicker.CurrentDirectory != directCreatedDir {
		t.Fatalf("expected CurrentDirectory to be %s, got %s", directCreatedDir, m.filePicker.CurrentDirectory)
	}
}

func TestSaveLog_FilePicker_ModalAndSave(t *testing.T) {
	m, tempDir := createTestModelWithConfig(t)
	defer os.RemoveAll(tempDir)

	// Ingest sample records
	m.ingestRecord(record.Record{Raw: "SENSOR_INIT: OK"})
	m.ingestRecord(record.Record{Raw: "VOLTAGE_READING: 3.32V"})

	// Ensure normal mode
	m.mode = modeNormal
	m.width = 80
	m.tableHeight = 20

	// 1. Press 's' in normal mode to open save picker modal
	mMod, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = mMod.(Model)
	if m.mode != modeFilePicker {
		t.Fatalf("expected modeFilePicker on 's', got %v", m.mode)
	}
	if m.fpPurpose != fpPurposeSaveLog {
		t.Fatalf("expected fpPurposeSaveLog, got %v", m.fpPurpose)
	}
	if cmd == nil {
		t.Fatalf("expected non-nil Init command from filepicker")
	}

	// 2. Verify modal renders Save-specific title and hints
	view := m.viewFilePickerModal()
	if !strings.Contains(view, "Save Logs to File") {
		t.Fatalf("expected view to contain 'Save Logs to File', got: %s", view)
	}
	if !strings.Contains(view, "Save to:") {
		t.Fatalf("expected view to contain 'Save to:', got: %s", view)
	}
	if !strings.Contains(view, "Space save") {
		t.Fatalf("expected view to contain 'Space save', got: %s", view)
	}

	// Verify responsive width and height
	m.width = 140
	m.height = 40
	m.tableHeight = 30
	if w := m.filePickerModalWidth(); w < 90 || w > 116 {
		t.Fatalf("expected modalWidth between 90 and 116 for width=140, got %d", w)
	}
	if h := m.filePickerHeight(); h < 15 {
		t.Fatalf("expected filePickerHeight >= 15 for height=40, got %d", h)
	}

	// 3. Test Esc cancels back to modeNormal without saving
	mMod, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = mMod.(Model)
	if m.mode != modeNormal {
		t.Fatalf("expected return to modeNormal on Esc, got %v", m.mode)
	}
	if m.message != "Save log canceled" {
		t.Fatalf("expected 'Save log canceled', got %q", m.message)
	}

	// Configure settings.LogDir and settings.LogPrefix to verify decoupling
	m.settings.LogDir = "/stable/direct/dir"
	m.SetDirectToDiskPrefix("soak_stream")

	// 4. Reopen save picker: verify it starts at launch dir (os.Getwd), NOT /stable/direct/dir
	mMod, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = mMod.(Model)
	cwd, _ := os.Getwd()
	if m.filePicker.CurrentDirectory != cwd {
		t.Fatalf("expected save picker to start at launch dir %s, got %s", cwd, m.filePicker.CurrentDirectory)
	}

	// Set directory to tempDir via ':'
	mMod, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{':'}})
	m = mMod.(Model)
	m.fpDirInput.SetText(tempDir)
	mMod, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mMod.(Model)

	// Change prefix to "diag_dump" via 'p'
	mMod, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	m = mMod.(Model)
	m.fpPrefixInput.SetText("diag_dump")
	mMod, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mMod.(Model)

	prefView := m.viewFilePickerModal()
	if !strings.Contains(prefView, "diag_dump-") {
		t.Fatalf("expected preview to show diag_dump prefix, got: %s", prefView)
	}

	// 5. Press Space to save to tempDir
	mMod, saveCmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	m = mMod.(Model)
	if m.mode != modeNormal {
		t.Fatalf("expected return to modeNormal after Space confirm, got %v", m.mode)
	}
	if saveCmd == nil {
		t.Fatalf("expected non-nil save command")
	}

	// Verify decoupling: settings.LogDir and settings.LogPrefix must remain unchanged!
	if m.settings.LogDir != "/stable/direct/dir" {
		t.Fatalf("expected settings.LogDir to remain '/stable/direct/dir', got %s", m.settings.LogDir)
	}
	if m.settings.LogPrefix != "soak_stream" {
		t.Fatalf("expected settings.LogPrefix to remain 'soak_stream', got %s", m.settings.LogPrefix)
	}
	if m.DirectToDiskPrefix() != "soak_stream" {
		t.Fatalf("expected DirectToDiskPrefix to remain 'soak_stream', got %s", m.DirectToDiskPrefix())
	}

	// Execute save command
	msg := saveCmd()
	savedMsg, ok := msg.(LogSavedMsg)
	if !ok {
		t.Fatalf("expected LogSavedMsg, got %T", msg)
	}
	if savedMsg.Err != nil {
		t.Fatalf("unexpected save error: %v", savedMsg.Err)
	}
	if savedMsg.Count != 2 {
		t.Fatalf("expected 2 saved records, got %d", savedMsg.Count)
	}

	// Process LogSavedMsg in Update
	mMod, _ = m.Update(savedMsg)
	m = mMod.(Model)
	if !strings.Contains(m.message, "✓ Saved 2 records to diag_dump-") {
		t.Fatalf("expected success message with diag_dump-, got: %s", m.message)
	}

	// Verify file content on disk
	content, err := os.ReadFile(savedMsg.Path)
	if err != nil {
		t.Fatalf("failed to read saved file: %v", err)
	}
	contentStr := string(content)
	if !strings.Contains(contentStr, "SENSOR_INIT: OK") || !strings.Contains(contentStr, "VOLTAGE_READING: 3.32V") {
		t.Fatalf("unexpected saved log content: %s", contentStr)
	}

	// 6. Test saving directly to an explicit file (overwrite)
	explicitFile := filepath.Join(tempDir, "explicit_target.log")
	var fileCmd tea.Cmd
	m, fileCmd = m.handleFilePickerSelected(explicitFile)
	if m.mode != modeNormal {
		t.Fatalf("expected modeNormal after explicit file selection, got %v", m.mode)
	}
	fileMsg := fileCmd().(LogSavedMsg)
	if fileMsg.Err != nil {
		t.Fatalf("unexpected error saving to explicit file: %v", fileMsg.Err)
	}
	if fileMsg.Path != explicitFile {
		t.Fatalf("expected path %s, got %s", explicitFile, fileMsg.Path)
	}
	expContent, err := os.ReadFile(explicitFile)
	if err != nil {
		t.Fatalf("failed to read explicit file: %v", err)
	}
	if !strings.Contains(string(expContent), "SENSOR_INIT: OK") {
		t.Fatalf("unexpected explicit file content: %s", string(expContent))
	}
}
