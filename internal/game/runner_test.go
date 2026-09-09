package game

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestRoverRunner_InitAndTick(t *testing.T) {
	game := NewRoverRunner().(*RoverRunner)
	if game.Title() == "" {
		t.Errorf("expected non-empty game title")
	}
	if game.IsGameOver() {
		t.Errorf("expected game not to be over initially")
	}
	if game.Score() != 0 {
		t.Errorf("expected initial score to be 0, got %d", game.Score())
	}
	if game.playerY != 0 {
		t.Errorf("expected player on ground, got Y=%d", game.playerY)
	}

	// First tick advances score
	updated, cmd := game.Update(TickMsg{})
	game = updated.(*RoverRunner)
	if game.Score() != 1 {
		t.Errorf("expected score 1 after tick, got %d", game.Score())
	}
	if cmd == nil {
		t.Errorf("expected next tick cmd")
	}
}

func TestRoverRunner_Jump(t *testing.T) {
	game := NewRoverRunner().(*RoverRunner)

	// Press Space to jump
	updated, _ := game.Update(tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})
	game = updated.(*RoverRunner)
	if game.playerY == 0 {
		t.Fatalf("expected playerY > 0 after jump key, got %d", game.playerY)
	}

	// Advance ticks through the jump arc until landing
	for i := 0; i < len(jumpArc)+2; i++ {
		updated, _ = game.Update(TickMsg{})
		game = updated.(*RoverRunner)
	}

	// Player should have landed back on ground
	if game.playerY != 0 {
		t.Errorf("expected player to land on ground (Y=0), got %d", game.playerY)
	}
}

func TestRoverRunner_CollisionAndGameOver(t *testing.T) {
	game := NewRoverRunner().(*RoverRunner)
	game.playerY = 0
	// Place obstacle directly at player X
	game.obstacles = []Obstacle{{X: game.playerX, Sprite: "🌵 "}}

	updated, _ := game.Update(TickMsg{})
	game = updated.(*RoverRunner)

	if !game.IsGameOver() {
		t.Fatalf("expected game over after collision with obstacle")
	}

	scoreAtCrash := game.Score()

	// Subsequent ticks should not advance score while game over
	updated, _ = game.Update(TickMsg{})
	game = updated.(*RoverRunner)
	if game.Score() != scoreAtCrash {
		t.Errorf("expected score to remain %d, got %d", scoreAtCrash, game.Score())
	}

	// Press Space to restart
	updated, cmd := game.Update(tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})
	game = updated.(*RoverRunner)
	if game.IsGameOver() {
		t.Errorf("expected restart to reset game over")
	}
	if cmd == nil {
		t.Errorf("expected game tick command on restart")
	}
}
