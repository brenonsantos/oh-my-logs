package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestModel_GameLifecycleAndBackgroundIngest(t *testing.T) {
	m := newTestModel()
	m.connState = ConnConnected

	// 1. Initial empty state is clean and does NOT leak the easter egg
	emptyView := m.View()
	if strings.Contains(emptyView, "play while you wait") {
		t.Errorf("expected empty state not to leak easter egg hint, got:\n%s", emptyView)
	}
	if !strings.Contains(emptyView, "Waiting for serial data") {
		t.Errorf("expected clean waiting state, got:\n%s", emptyView)
	}

	// 2. Press 'g' when buffer is empty -> launches game
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	m = updated.(Model)
	if m.mode != modeGame {
		t.Fatalf("expected modeGame, got %v", m.mode)
	}
	if m.activeGame == nil {
		t.Fatalf("expected activeGame to be non-nil")
	}
	if m.activeGame.Init() != nil && cmd == nil {
		t.Errorf("expected game init command")
	}

	// 3. View renders game canvas
	gameView := m.View()
	if !strings.Contains(gameView, m.activeGame.Title()) {
		t.Errorf("expected game modal title in view, got:\n%s", gameView)
	}
	if !strings.Contains(gameView, "SCORE:") {
		t.Errorf("expected score in game view, got:\n%s", gameView)
	}

	// 4. Background log arrives while playing!
	// It should NOT interrupt gameplay or be lost.
	updated, _ = m.Update(lineMsg("sensor_init: ok"))
	m = updated.(Model)

	if m.buffer.Len() != 1 {
		t.Fatalf("expected log to be added to ring buffer, got %d", m.buffer.Len())
	}
	if m.logsDuringGame != 1 {
		t.Fatalf("expected logsDuringGame counter to be 1, got %d", m.logsDuringGame)
	}

	// Game view now shows notification badge about arrived logs
	gameWithLogView := m.View()
	if !strings.Contains(gameWithLogView, "1 new logs arrived") {
		t.Errorf("expected live notification badge in game view, got:\n%s", gameWithLogView)
	}

	// 5. Press Esc to exit game back to logs
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.mode != modeNormal {
		t.Fatalf("expected modeNormal after Esc, got %v", m.mode)
	}
	if m.activeGame != nil {
		t.Errorf("expected activeGame to be cleared")
	}

	normalView := m.View()
	if !strings.Contains(normalView, "sensor_init: ok") {
		t.Errorf("expected buffered log to be visible in normal view, got:\n%s", normalView)
	}
}
