package tui

import (
	"strings"
	"testing"

	"github.com/brenoniehues/oh-my-logs/internal/config"
	"github.com/brenoniehues/oh-my-logs/internal/record"
	"github.com/brenoniehues/oh-my-logs/internal/serial"
	tea "github.com/charmbracelet/bubbletea"
)

func newFilterTestModel(t *testing.T) Model {
	tmpDir := t.TempDir()
	appCfg := &config.AppConfig{
		ConfigDir:   tmpDir,
		ProfilesDir: tmpDir,
		LogsDir:     tmpDir,
	}

	buf := record.NewBuffer(100)
	buf.Add(record.Record{ID: 1, Fields: map[string]string{"level": "INFO", "message": "Boot ok"}, Raw: "INFO Boot ok"})
	buf.Add(record.Record{ID: 2, Fields: map[string]string{"level": "WARN", "message": "Low battery"}, Raw: "WARN Low battery"})
	buf.Add(record.Record{ID: 3, Fields: map[string]string{"level": "ERROR", "message": "Sensor fail"}, Raw: "ERROR Sensor fail"})
	buf.Add(record.Record{ID: 4, Fields: map[string]string{"level": "DEBUG", "message": "chatter"}, Raw: "DEBUG chatter"})

	m := New(
		serial.Config{Port: "COM1", Baud: 115200},
		nil,
		nil,
		buf,
		nil,
		appCfg,
	)
	m.width = 80
	m.height = 24
	m.recalcLayout()
	m.visible = buf.All()
	m.tabs[0].Visible = m.visible
	return m
}

func TestFilterHistoryNavigation(t *testing.T) {
	m := newFilterTestModel(t)

	// 1. Enter first filter: "level:ERROR"
	m, _ = updateKey(m, 'f')
	if m.mode != modeFilter {
		t.Fatalf("expected modeFilter, got %v", m.mode)
	}
	m.filterInput = "level:ERROR"
	m, _ = updateSpecialKey(m, tea.KeyEnter)
	if m.mode != modeNormal {
		t.Fatalf("expected modeNormal after submit, got %v", m.mode)
	}
	if len(m.visible) != 1 || m.visible[0].Fields["level"] != "ERROR" {
		t.Fatalf("expected 1 ERROR record, got %d", len(m.visible))
	}

	// 2. Enter second filter: "level:WARN"
	m, _ = updateKey(m, 'f')
	m.filterInput = "level:WARN"
	m, _ = updateSpecialKey(m, tea.KeyEnter)
	if len(m.visible) != 1 || m.visible[0].Fields["level"] != "WARN" {
		t.Fatalf("expected 1 WARN record, got %d", len(m.visible))
	}

	// 3. Open filter again and test history navigation with Up/Down
	m, _ = updateKey(m, 'f')
	m.filterInput = "my-draft"

	// Press Up: should recall "level:WARN" and save "my-draft" as draft
	m, _ = updateSpecialKey(m, tea.KeyUp)
	if m.filterInput != "level:WARN" {
		t.Errorf("expected level:WARN on first Up, got %q", m.filterInput)
	}

	// Press Up again: should recall "level:ERROR"
	m, _ = updateSpecialKey(m, tea.KeyUp)
	if m.filterInput != "level:ERROR" {
		t.Errorf("expected level:ERROR on second Up, got %q", m.filterInput)
	}

	// Press Up at top of history: stays on oldest ("level:ERROR")
	m, _ = updateSpecialKey(m, tea.KeyUp)
	if m.filterInput != "level:ERROR" {
		t.Errorf("expected level:ERROR at top of history, got %q", m.filterInput)
	}

	// Press Down: should return to "level:WARN"
	m, _ = updateSpecialKey(m, tea.KeyDown)
	if m.filterInput != "level:WARN" {
		t.Errorf("expected level:WARN on Down, got %q", m.filterInput)
	}

	// Press Down again: should restore original draft "my-draft"
	m, _ = updateSpecialKey(m, tea.KeyDown)
	if m.filterInput != "my-draft" {
		t.Errorf("expected my-draft on second Down, got %q", m.filterInput)
	}
}

func TestFilterPresetsModalOpenAndNavigate(t *testing.T) {
	m := newFilterTestModel(t)

	// Press 'F' to open presets modal
	m, _ = updateKey(m, 'F')
	if m.mode != modeFilterPresets {
		t.Fatalf("expected modeFilterPresets, got %v", m.mode)
	}

	view := m.View()
	if !strings.Contains(view, "Filter Presets") {
		t.Errorf("expected View() to render Filter Presets modal title, got:\n%s", view)
	}

	// Test navigation j/k
	initialCursor := m.presetCursor
	m, _ = updateKey(m, 'j')
	if m.presetCursor != initialCursor+1 {
		t.Errorf("expected presetCursor to advance to %d, got %d", initialCursor+1, m.presetCursor)
	}

	m, _ = updateKey(m, 'k')
	if m.presetCursor != initialCursor {
		t.Errorf("expected presetCursor to return to %d, got %d", initialCursor, m.presetCursor)
	}

	// Press Enter to apply first preset ("Errors & Critical")
	m, _ = updateSpecialKey(m, tea.KeyEnter)
	if m.mode != modeNormal {
		t.Fatalf("expected modeNormal after applying preset, got %v", m.mode)
	}

	cur := m.currentTab()
	if !strings.Contains(cur.FilterRaw, "level:") {
		t.Errorf("expected tab filter to be applied from preset, got %q", cur.FilterRaw)
	}
	if len(m.visible) != 1 || m.visible[0].Fields["level"] != "ERROR" {
		t.Errorf("expected visible to be filtered to 1 ERROR record, got %d", len(m.visible))
	}
}

func TestFilterPresetsSaveCurrentFilter(t *testing.T) {
	m := newFilterTestModel(t)

	// Set a filter on active tab
	m, _ = updateKey(m, 'f')
	m.filterInput = "level:WARN Low"
	m, _ = updateSpecialKey(m, tea.KeyEnter)

	// Open presets modal
	m, _ = updateKey(m, 'F')
	if m.mode != modeFilterPresets {
		t.Fatalf("expected modeFilterPresets, got %v", m.mode)
	}

	// Press 's' to trigger save preset prompt
	m, _ = updateKey(m, 's')
	if m.mode != modeSavePresetPrompt {
		t.Fatalf("expected modeSavePresetPrompt, got %v", m.mode)
	}

	promptView := m.View()
	if !strings.Contains(promptView, "Save Filter Preset") {
		t.Errorf("expected View() to show Save Filter Preset prompt, got:\n%s", promptView)
	}

	// Set custom preset name and confirm
	m.savePresetNameInput = "Battery Alert"
	m, _ = updateSpecialKey(m, tea.KeyEnter)
	if m.mode != modeFilterPresets {
		t.Fatalf("expected return to modeFilterPresets after save, got %v", m.mode)
	}

	// Verify preset is in list
	presets := m.filtersCfg.Presets
	found := false
	for _, p := range presets {
		if p.Name == "Battery Alert" && p.Filter == "level:WARN Low" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected 'Battery Alert' in presets list, got: %+v", presets)
	}

	// Verify persistence: load from disk directly
	loaded, err := m.appConfig.LoadFilters()
	if err != nil {
		t.Fatalf("failed to reload filters from disk: %v", err)
	}
	diskFound := false
	for _, p := range loaded.Presets {
		if p.Name == "Battery Alert" && p.Filter == "level:WARN Low" {
			diskFound = true
			break
		}
	}
	if !diskFound {
		t.Errorf("expected 'Battery Alert' persisted on disk, but not found in %+v", loaded.Presets)
	}
}

func TestFilterPresetsDelete(t *testing.T) {
	m := newFilterTestModel(t)

	m, _ = updateKey(m, 'F')
	initialCount := len(m.filtersCfg.Presets)
	if initialCount == 0 {
		t.Fatalf("expected presets in initial list")
	}

	// Delete current preset with 'd'
	toDelete := m.filtersCfg.Presets[0].Name
	m, _ = updateKey(m, 'd')

	if len(m.filtersCfg.Presets) != initialCount-1 {
		t.Errorf("expected preset count %d, got %d", initialCount-1, len(m.filtersCfg.Presets))
	}
	if strings.Contains(m.message, toDelete) == false {
		t.Errorf("expected status message mentioning deleted preset %q, got %q", toDelete, m.message)
	}
}

func TestFilterPresetsCtrlPShortcut(t *testing.T) {
	m := newFilterTestModel(t)

	// 1. From normal mode: Ctrl+P opens presets modal
	ctrlP := tea.KeyMsg{Type: tea.KeyCtrlP}
	updated, _ := m.Update(ctrlP)
	m = updated.(Model)
	if m.mode != modeFilterPresets {
		t.Errorf("expected Ctrl+P to open modeFilterPresets, got %v", m.mode)
	}

	// Close with Esc
	m, _ = updateSpecialKey(m, tea.KeyEsc)
	if m.mode != modeNormal {
		t.Fatalf("expected modeNormal after Esc, got %v", m.mode)
	}

	// 2. From filter input mode: Ctrl+P opens presets modal
	m, _ = updateKey(m, 'f')
	if m.mode != modeFilter {
		t.Fatalf("expected modeFilter, got %v", m.mode)
	}
	updated, _ = m.Update(ctrlP)
	m = updated.(Model)
	if m.mode != modeFilterPresets {
		t.Errorf("expected Ctrl+P inside filter mode to open modeFilterPresets, got %v", m.mode)
	}
}

func updateKey(m Model, r rune) (Model, tea.Cmd) {
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	return updated.(Model), cmd
}

func updateSpecialKey(m Model, keyType tea.KeyType) (Model, tea.Cmd) {
	updated, cmd := m.Update(tea.KeyMsg{Type: keyType})
	return updated.(Model), cmd
}

func TestFilterPresetsModalSizingAndNoWrapping(t *testing.T) {
	m := newFilterTestModel(t)
	m.width = 241
	m.height = 59
	m.tableHeight = 55

	m, _ = updateKey(m, 'F')
	if m.mode != modeFilterPresets {
		t.Fatalf("expected modeFilterPresets, got %v", m.mode)
	}

	view := m.viewFilterPresetsModal()
	// Modal must render without wrapping the "Warnings & Above" filter onto an orphan line
	lines := strings.Split(view, "\n")
	for _, l := range lines {
		// If wrapping occurred, "wa..." or "level:warn..." would start flush at the border
		if strings.HasPrefix(strings.TrimSpace(l), "level:warn") {
			t.Errorf("detected wrapped filter line in wide terminal: %q", l)
		}
	}

	// Now test narrow terminal (60 cols)
	m.width = 60
	viewNarrow := m.viewFilterPresetsModal()
	linesNarrow := strings.Split(viewNarrow, "\n")
	for _, l := range linesNarrow {
		if strings.HasPrefix(strings.TrimSpace(l), "level:warn") {
			t.Errorf("detected wrapped filter line in narrow terminal: %q", l)
		}
	}
}

func TestHelpModalWidthAndNoTruncation(t *testing.T) {
	m := newFilterTestModel(t)
	m.width = 241
	m.height = 59
	m.tableHeight = 55

	m, _ = updateKey(m, '?')
	if m.mode != modeHelp {
		t.Fatalf("expected modeHelp, got %v", m.mode)
	}

	view := m.viewHelpModal()
	if !strings.Contains(view, "Port / Profile / Reconnect") {
		t.Errorf("expected full description 'Port / Profile / Reconnect' in help modal, got:\n%s", view)
	}
	if strings.Contains(view, "select\n") || strings.Contains(view, "\nselect") {
		t.Errorf("detected wrapped footer in help modal")
	}
}
