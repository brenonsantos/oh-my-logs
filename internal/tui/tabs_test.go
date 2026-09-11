package tui

import (
	"strings"
	"testing"

	"github.com/brenoniehues/oh-my-logs/internal/config"
	"github.com/brenoniehues/oh-my-logs/internal/filter"
	"github.com/brenoniehues/oh-my-logs/internal/parser"
	"github.com/brenoniehues/oh-my-logs/internal/record"
	"github.com/brenoniehues/oh-my-logs/internal/serial"
	tea "github.com/charmbracelet/bubbletea"
)

func newTestTabModel() Model {
	cfg := serial.Config{Port: "COM1", Baud: 115200}
	p := parser.NewRawParser()
	buf := record.NewBuffer(100)
	appCfg := &config.AppConfig{}
	m := New(cfg, nil, p, buf, nil, appCfg)
	m.width = 100
	m.height = 30
	m.recalcLayout()
	return m
}

func TestVirtualTabsLifecycleAndNavigation(t *testing.T) {
	m := newTestTabModel()

	// Initially 1 tab
	if len(m.tabs) != 1 {
		t.Fatalf("expected 1 initial tab, got %d", len(m.tabs))
	}
	if m.tabs[0].DisplayName(1) != "All" {
		t.Errorf("expected default tab name 'All', got %q", m.tabs[0].DisplayName(1))
	}
	if m.activeTab != 0 {
		t.Errorf("expected activeTab to be 0, got %d", m.activeTab)
	}

	// Layout with 1 tab should allocate height - 7
	initialTableHeight := m.tableHeight
	if initialTableHeight != 30-7 {
		t.Errorf("expected tableHeight with 1 tab to be %d, got %d", 30-7, initialTableHeight)
	}

	// 1. Create a new tab via Ctrl+T
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlT})
	m = updated.(Model)

	if len(m.tabs) != 2 {
		t.Fatalf("expected 2 tabs after Ctrl+T, got %d", len(m.tabs))
	}
	if m.activeTab != 1 {
		t.Errorf("expected activeTab to be 1, got %d", m.activeTab)
	}
	if m.mode != modeFilter {
		t.Errorf("expected modeFilter after creating new tab, got %v", m.mode)
	}
	// Layout with 2 tabs should dynamically allocate 2 rows for tab bar + divider (height - 9)
	if m.tableHeight != 30-9 {
		t.Errorf("expected tableHeight with 2 tabs to be %d, got %d", 30-9, m.tableHeight)
	}

	// Set filter for Tab 2: "error"
	for _, r := range "error" {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(Model)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	if m.mode != modeNormal {
		t.Errorf("expected modeNormal after confirming filter, got %v", m.mode)
	}
	if m.tabs[1].Name != "error" {
		t.Errorf("expected tab name to be auto-labeled 'error', got %q", m.tabs[1].Name)
	}

	// 2. Navigation with Tab / Shift+Tab
	// Press Tab to cycle to Tab 1 (0)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	if m.activeTab != 0 {
		t.Errorf("expected activeTab to cycle back to 0, got %d", m.activeTab)
	}

	// Press Shift+Tab to cycle back to Tab 2 (1)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	m = updated.(Model)
	if m.activeTab != 1 {
		t.Errorf("expected activeTab to cycle to 1 on Shift+Tab, got %d", m.activeTab)
	}


	// Direct number key jump '1' -> tab 0
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	m = updated.(Model)
	if m.activeTab != 0 {
		t.Errorf("expected activeTab to be 0 on key '1', got %d", m.activeTab)
	}

	// Direct number key jump '2' -> tab 1
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	m = updated.(Model)
	if m.activeTab != 1 {
		t.Errorf("expected activeTab to be 1 on key '2', got %d", m.activeTab)
	}

	// 3. Close active tab via Ctrl+W
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlW})
	m = updated.(Model)
	if len(m.tabs) != 1 {
		t.Fatalf("expected 1 tab after closing Tab 2, got %d", len(m.tabs))
	}
	if m.activeTab != 0 {
		t.Errorf("expected activeTab to be 0, got %d", m.activeTab)
	}
	if m.tableHeight != 30-7 {
		t.Errorf("expected tableHeight to revert to %d after closing down to 1 tab, got %d", 30-7, m.tableHeight)
	}

	// 4. Try closing the last tab -> should be prevented
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlW})
	m = updated.(Model)
	if len(m.tabs) != 1 {
		t.Errorf("closing the last tab should be rejected, but tab count is %d", len(m.tabs))
	}
	if !strings.Contains(m.message, "Cannot close the only tab") {
		t.Errorf("expected warning message when closing only tab, got %q", m.message)
	}
}

func TestVirtualTabsIndependentFilteringAndStreaming(t *testing.T) {
	m := newTestTabModel()

	// Tab 1: All
	// Create Tab 2 with filter "error"
	errFilter, _ := filter.New("error")
	m.tabs = append(m.tabs, Tab{
		Name:      "Errors",
		FilterRaw: "error",
		Filter:    errFilter,
		ViewportState: ViewportState{
			Follow: true,
		},
	})
	m.recalcLayout()

	// Stream 3 lines into the app
	lines := []string{
		"system booting ok",
		"disk error: block read timeout",
		"network client connected",
	}

	for _, l := range lines {
		updated, _ := m.Update(lineMsg(l))
		m = updated.(Model)
	}

	// Verify Tab 1 (All) received all 3 lines
	if len(m.tabs[0].Visible) != 3 {
		t.Errorf("expected Tab 1 to have 3 visible records, got %d", len(m.tabs[0].Visible))
	}

	// Verify Tab 2 (Errors) received only the 1 error line
	if len(m.tabs[1].Visible) != 1 {
		t.Errorf("expected Tab 2 to have 1 visible record, got %d", len(m.tabs[1].Visible))
	}
	if !strings.Contains(m.tabs[1].Visible[0].Raw, "disk error") {
		t.Errorf("expected Tab 2 visible record to be the error line, got %q", m.tabs[1].Visible[0].Raw)
	}

	// Active tab is 0: m.visible should be 3
	if len(m.visible) != 3 {
		t.Errorf("expected m.visible to be 3 when activeTab is 0, got %d", len(m.visible))
	}

	// Switch to Tab 2
	m.switchTab(1)
	if len(m.visible) != 1 {
		t.Errorf("expected m.visible to be 1 after switching to Tab 2, got %d", len(m.visible))
	}

	// Switch back to Tab 1
	m.switchTab(0)
	if len(m.visible) != 3 {
		t.Errorf("expected m.visible to be 3 after switching back to Tab 1, got %d", len(m.visible))
	}
}

func TestVirtualTabsIndependentScrollAndFollow(t *testing.T) {
	m := newTestTabModel()
	m.tableHeight = 5

	// Add second tab
	f, _ := filter.New("")
	m.tabs = append(m.tabs, Tab{
		Name:   "Tab 2",
		Filter: f,
		ViewportState: ViewportState{
			Follow: true,
		},
	})

	// Add 10 lines
	for i := 0; i < 10; i++ {
		updated, _ := m.Update(lineMsg("test log line"))
		m = updated.(Model)
	}

	// Tab 1 is currently active and following at bottom (offset = 10 - 5 = 5)
	if m.scrollOffset != 5 {
		t.Errorf("expected scrollOffset 5, got %d", m.scrollOffset)
	}
	if !m.follow {
		t.Errorf("expected follow to be true")
	}

	// In Tab 1, scroll up to top
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	m = updated.(Model)
	if m.scrollOffset != 0 {
		t.Errorf("expected Tab 1 scrollOffset 0 after 'g', got %d", m.scrollOffset)
	}
	if m.follow {
		t.Errorf("expected Tab 1 follow to be false after scrolling up")
	}

	// Switch to Tab 2
	m.switchTab(1)
	if m.scrollOffset != 5 {
		t.Errorf("expected Tab 2 scrollOffset to be preserved at 5, got %d", m.scrollOffset)
	}
	if !m.follow {
		t.Errorf("expected Tab 2 follow to still be true")
	}

	// Switch back to Tab 1
	m.switchTab(0)
	if m.scrollOffset != 0 {
		t.Errorf("expected Tab 1 scrollOffset to still be 0, got %d", m.scrollOffset)
	}
	if m.follow {
		t.Errorf("expected Tab 1 follow to still be false")
	}
}

func TestVirtualTabsViewRendering(t *testing.T) {
	m := newTestTabModel()

	// 1 Tab: View() should not show tab bar
	vSingle := m.View()
	if strings.Contains(vSingle, "Tab: cycle") {
		t.Errorf("tab bar hints should NOT appear when only 1 tab exists")
	}

	// 2 Tabs: View() should show tab bar and hints
	m.tabs = append(m.tabs, Tab{
		Name: "Errors",
		ViewportState: ViewportState{
			Follow: true,
		},
	})
	m.recalcLayout()

	vMulti := m.View()
	if !strings.Contains(vMulti, "Tab: cycle") {
		t.Errorf("tab bar hints should appear when multiple tabs exist")
	}
	if !strings.Contains(vMulti, "1: All") || !strings.Contains(vMulti, "2: Errors") {
		t.Errorf("tab pills should be rendered with their indices and names")
	}
	if !strings.Contains(vMulti, "tab [1/2: All]") {
		t.Errorf("status bar should display active tab info badge")
	}
}
