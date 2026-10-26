package tui

import (
	"strings"
	"testing"

	"github.com/brenoniehues/oh-my-logs/internal/game"
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

func TestModel_AimTrainerTargetClick(t *testing.T) {
	m := newTestModel()
	m.width = 120
	m.height = 30
	m.recalcLayout()

	at := game.NewAimTrainer()
	m.mode = modeGame
	m.activeGame = at

	view := m.View()
	lines := strings.Split(view, "\n")
	for i, l := range lines {
		t.Logf("Line %2d: %s", i, l)
	}

	originX, originY := m.gameModalBoardOrigin()
	t.Logf("Calculated origin: originX=%d, originY=%d", originX, originY)

	// Access aim trainer
	aimTrainer, ok := m.activeGame.(*game.AimTrainer)
	if !ok {
		t.Fatalf("expected *game.AimTrainer")
	}

	targets := aimTrainer.Targets()
	if len(targets) != 3 {
		t.Fatalf("expected 3 targets, got %d", len(targets))
	}

	tg := targets[0]
	t.Logf("Target 0 at relX=%d, relY=%d", tg.X, tg.Y)

	// Click directly at the bullseye of Target 0 (tg.X + 2, tg.Y + 1)
	clickX := originX + tg.X + 2
	clickY := originY + tg.Y + 1

	clickMsg := tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
		X:      clickX,
		Y:      clickY,
	}

	updated, _ := m.Update(clickMsg)
	m = updated.(Model)

	if m.activeGame.Score() == 0 {
		t.Fatalf("expected score > 0 after clicking target 0, got 0")
	}
	t.Logf("Success! Score after hit: %d", m.activeGame.Score())

	// Test 2: Multiple tabs active (topChrome = 4 lines)
	m2 := newTestModel()
	m2.width = 120
	m2.height = 30
	m2.tabs = append(m2.tabs, Tab{Name: "Tab 2", ViewportState: ViewportState{Visible: m2.buffer.All()}})
	m2.recalcLayout()

	at2 := game.NewAimTrainer()
	m2.mode = modeGame
	m2.activeGame = at2

	origin2X, origin2Y := m2.gameModalBoardOrigin()
	aimTrainer2 := m2.activeGame.(*game.AimTrainer)
	tg2 := aimTrainer2.Targets()[0]

	click2Msg := tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
		X:      origin2X + tg2.X + 2,
		Y:      origin2Y + tg2.Y + 1,
	}
	updated2, _ := m2.Update(click2Msg)
	m2 = updated2.(Model)

	if m2.activeGame.Score() == 0 {
		t.Fatalf("expected score > 0 with multiple tabs, got 0")
	}
	t.Logf("Success with multiple tabs! Score: %d", m2.activeGame.Score())
}

