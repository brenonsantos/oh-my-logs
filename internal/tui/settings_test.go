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
