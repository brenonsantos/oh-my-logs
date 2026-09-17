package tui

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/brenoniehues/oh-my-logs/internal/config"
	tea "github.com/charmbracelet/bubbletea"
)

func setupTestModelForThemes(t *testing.T) Model {
	t.Helper()
	tempDir := t.TempDir()
	appCfg := &config.AppConfig{
		ConfigDir:   tempDir,
		ProfilesDir: filepath.Join(tempDir, "profiles"),
		LogsDir:     filepath.Join(tempDir, "logs"),
	}

	return Model{
		keys:        defaultKeyMap(),
		width:       120,
		height:      30,
		tableHeight: 20,
		appConfig:   appCfg,
		settings:    &config.Settings{Theme: "Dark Slate"},
		mode:        modeNormal,
	}
}

func TestThemes_OpenModal(t *testing.T) {
	defer SetCurrentTheme("Dark Slate")
	SetCurrentTheme("Monokai")

	m := setupTestModelForThemes(t)
	m.settings.Theme = "Monokai"

	m.openThemeModal()

	if m.mode != modeThemeModal {
		t.Fatalf("expected mode %v, got %v", modeThemeModal, m.mode)
	}
	if m.themeModalInitial != "Monokai" {
		t.Errorf("expected initial theme 'Monokai', got %q", m.themeModalInitial)
	}

	// Verify cursor points to Monokai
	if m.themeModalCursor < 0 || m.themeModalCursor >= len(curatedPalettes) {
		t.Fatalf("cursor out of range: %d", m.themeModalCursor)
	}
	if curatedPalettes[m.themeModalCursor].Name != "Monokai" {
		t.Errorf("expected cursor at Monokai, got %q", curatedPalettes[m.themeModalCursor].Name)
	}
}

func TestThemes_NavigationAndLivePreview(t *testing.T) {
	defer SetCurrentTheme("Dark Slate")
	SetCurrentTheme("Dark Slate")

	m := setupTestModelForThemes(t)
	m.openThemeModal()

	// Initial cursor is 0 (Dark Slate)
	if m.themeModalCursor != 0 {
		t.Fatalf("expected initial cursor 0, got %d", m.themeModalCursor)
	}

	// Move down with 'j'
	res, _ := m.handleThemeModalKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = res.(Model)
	if m.themeModalCursor != 1 {
		t.Errorf("expected cursor 1 after 'j', got %d", m.themeModalCursor)
	}
	// Verify live preview switched current theme
	if CurrentThemeName() != curatedPalettes[1].Name {
		t.Errorf("expected live preview theme %q, got %q", curatedPalettes[1].Name, CurrentThemeName())
	}

	// Move down with Down key
	res, _ = m.handleThemeModalKey(tea.KeyMsg{Type: tea.KeyDown})
	m = res.(Model)
	if m.themeModalCursor != 2 {
		t.Errorf("expected cursor 2 after Down, got %d", m.themeModalCursor)
	}
	if CurrentThemeName() != curatedPalettes[2].Name {
		t.Errorf("expected live preview theme %q, got %q", curatedPalettes[2].Name, CurrentThemeName())
	}

	// Move up with 'k'
	res, _ = m.handleThemeModalKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m = res.(Model)
	if m.themeModalCursor != 1 {
		t.Errorf("expected cursor 1 after 'k', got %d", m.themeModalCursor)
	}

	// Move up with Up key back to 0
	res, _ = m.handleThemeModalKey(tea.KeyMsg{Type: tea.KeyUp})
	m = res.(Model)
	if m.themeModalCursor != 0 {
		t.Errorf("expected cursor 0 after Up, got %d", m.themeModalCursor)
	}

	// Wrap around up: should jump to last theme
	res, _ = m.handleThemeModalKey(tea.KeyMsg{Type: tea.KeyUp})
	m = res.(Model)
	expectedLast := len(curatedPalettes) - 1
	if m.themeModalCursor != expectedLast {
		t.Errorf("expected wrap-around to %d, got %d", expectedLast, m.themeModalCursor)
	}
	if CurrentThemeName() != curatedPalettes[expectedLast].Name {
		t.Errorf("expected live preview to wrap to %q, got %q", curatedPalettes[expectedLast].Name, CurrentThemeName())
	}
}

func TestThemes_ConfirmSelection(t *testing.T) {
	defer SetCurrentTheme("Dark Slate")
	SetCurrentTheme("Dark Slate")

	m := setupTestModelForThemes(t)
	m.openThemeModal()

	// Select Dracula (find its index)
	draculaIdx := -1
	for i, p := range curatedPalettes {
		if p.Name == "Dracula" {
			draculaIdx = i
			break
		}
	}
	if draculaIdx == -1 {
		t.Fatal("Dracula palette not found in curatedPalettes")
	}

	m.themeModalCursor = draculaIdx
	// Confirm with Enter
	res, _ := m.handleThemeModalKey(tea.KeyMsg{Type: tea.KeyEnter})
	m = res.(Model)

	if m.mode != modeSettings {
		t.Errorf("expected modeSettings after Enter, got %v", m.mode)
	}
	if m.settings.Theme != "Dracula" {
		t.Errorf("expected settings.Theme 'Dracula', got %q", m.settings.Theme)
	}
	if CurrentThemeName() != "Dracula" {
		t.Errorf("expected CurrentThemeName 'Dracula', got %q", CurrentThemeName())
	}
	if !strings.Contains(m.message, "Dracula") {
		t.Errorf("expected status message mentioning Dracula, got %q", m.message)
	}

	// Verify disk persistence using temp dir
	saved, err := m.appConfig.LoadSettings()
	if err != nil {
		t.Fatalf("failed to load settings: %v", err)
	}
	if saved.Theme != "Dracula" {
		t.Errorf("expected saved.Theme 'Dracula' in settings.json, got %q", saved.Theme)
	}
}

func TestThemes_CancelRevertsTheme(t *testing.T) {
	defer SetCurrentTheme("Dark Slate")
	SetCurrentTheme("Gruvbox")

	m := setupTestModelForThemes(t)
	m.settings.Theme = "Gruvbox"
	m.openThemeModal()

	// Navigate to something else (e.g. Cyberpunk)
	for i, p := range curatedPalettes {
		if p.Name == "Cyberpunk" {
			m.themeModalCursor = i
			SetCurrentTheme(p.Name)
			break
		}
	}

	if CurrentThemeName() != "Cyberpunk" {
		t.Fatalf("expected active theme to be Cyberpunk during preview, got %q", CurrentThemeName())
	}

	// Cancel with Esc
	res, _ := m.handleThemeModalKey(tea.KeyMsg{Type: tea.KeyEsc})
	m = res.(Model)

	if m.mode != modeSettings {
		t.Errorf("expected modeSettings after Esc, got %v", m.mode)
	}
	// Reverted to Gruvbox
	if CurrentThemeName() != "Gruvbox" {
		t.Errorf("expected reverted theme 'Gruvbox', got %q", CurrentThemeName())
	}
}

func TestSettings_EnterOnThemeRowOpensThemeModal(t *testing.T) {
	defer SetCurrentTheme("Dark Slate")
	SetCurrentTheme("Dark Slate")

	m := setupTestModelForThemes(t)
	m.mode = modeSettings
	m.settingsCursor = int(settingRowTheme)

	res, _ := m.handleSettingsKey(tea.KeyMsg{Type: tea.KeyEnter})
	m = res.(Model)

	if m.mode != modeThemeModal {
		t.Fatalf("expected mode %v after Enter on Theme row, got %v", modeThemeModal, m.mode)
	}
}

func TestThemes_ViewThemeModal(t *testing.T) {
	defer SetCurrentTheme("Dark Slate")
	SetCurrentTheme("Dark Slate")

	m := setupTestModelForThemes(t)
	m.openThemeModal()

	view := m.viewThemeModal()

	if !strings.Contains(view, "Theme Palette") {
		t.Errorf("view missing title 'Theme Palette'")
	}
	if !strings.Contains(view, "Enter apply") {
		t.Errorf("view missing footer instructions")
	}

	// Verify presence of sample palettes in the view
	expectedSamples := []string{"Dark Slate", "Catppuccin Mocha", "Dracula", "One Dark", "Matrix"}
	for _, s := range expectedSamples {
		if !strings.Contains(view, s) {
			t.Errorf("view missing expected palette %q", s)
		}
	}
}
