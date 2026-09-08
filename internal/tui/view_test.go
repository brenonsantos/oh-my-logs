package tui

import (
	"strings"
	"testing"

	"github.com/brenoniehues/oh-my-logs/internal/config"
	"github.com/brenoniehues/oh-my-logs/internal/parser"
	"github.com/brenoniehues/oh-my-logs/internal/record"
	"github.com/brenoniehues/oh-my-logs/internal/serial"
	tea "github.com/charmbracelet/bubbletea"
)

func newTestModel() Model {
	cfg := serial.Config{Port: "COM1", Baud: 115200}
	p := parser.NewRawParser()
	buf := record.NewBuffer(100)
	appCfg := &config.AppConfig{}
	m := New(cfg, nil, p, buf, nil, appCfg)
	m.width = 100
	m.height = 30
	m.tableHeight = 23
	return m
}

func TestHelpModalToggleAndDismiss(t *testing.T) {
	m := newTestModel()

	if m.mode != modeNormal {
		t.Fatalf("expected initial mode to be modeNormal, got %v", m.mode)
	}

	// Press '?' to open help modal
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = updated.(Model)
	if m.mode != modeHelp {
		t.Fatalf("expected modeHelp after '?', got %v", m.mode)
	}

	// View should render help modal
	viewOutput := m.View()
	if !strings.Contains(viewOutput, "Help — Keyboard Shortcuts") {
		t.Errorf("expected view to contain 'Help — Keyboard Shortcuts'")
	}
	if !strings.Contains(viewOutput, "NAVIGATION") || !strings.Contains(viewOutput, "ACTIONS & CONTROLS") {
		t.Errorf("expected view to contain help categories")
	}

	// Press '?' again to close
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = updated.(Model)
	if m.mode != modeNormal {
		t.Fatalf("expected modeNormal after second '?', got %v", m.mode)
	}

	// Press '?' to open, then Esc to close
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.mode != modeNormal {
		t.Fatalf("expected modeNormal after Esc, got %v", m.mode)
	}

	// Press '?' to open, then 'q' to close
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	m = updated.(Model)
	if m.mode != modeNormal {
		t.Fatalf("expected modeNormal after 'q', got %v", m.mode)
	}
}

func TestEmptyStateWithoutHeader(t *testing.T) {
	m := newTestModel()

	// Initially disconnected with empty buffer
	m.connState = ConnDisconnected
	v := m.View()

	// Should show empty state message
	if !strings.Contains(v, "No serial device connected") {
		t.Errorf("expected view to contain 'No serial device connected', got:\n%s", v)
	}
	if !strings.Contains(v, "Press p to select a port") {
		t.Errorf("expected view to contain 'Press p to select a port', got:\n%s", v)
	}
	if !strings.Contains(v, "Press ? for shortcuts") {
		t.Errorf("expected view to contain 'Press ? for shortcuts', got:\n%s", v)
	}

	// Table header (e.g. "Message") should NOT be present in empty state
	if strings.Contains(v, "  Message") {
		t.Errorf("table header should be omitted in empty state, but found in view")
	}

	// Switch to connected with empty buffer
	m.connState = ConnConnected
	vConn := m.View()
	if !strings.Contains(vConn, "Waiting for serial data…") {
		t.Errorf("expected view to contain 'Waiting for serial data…', got:\n%s", vConn)
	}
	if strings.Contains(vConn, "  Message") {
		t.Errorf("table header should be omitted in connected empty state")
	}

	// Add record and rebuild visible -> header should now appear
	rec := record.Record{Fields: map[string]string{"message": "hello world"}, Raw: "hello world"}
	m.buffer.Add(rec)
	m.visible = append(m.visible, rec)
	vWithLogs := m.View()
	if !strings.Contains(vWithLogs, "Message") {
		t.Errorf("table header 'Message' should be present when logs exist")
	}
	if !strings.Contains(vWithLogs, "hello world") {
		t.Errorf("log line should be rendered")
	}
}
