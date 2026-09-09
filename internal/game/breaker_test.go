package game

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestBitBreaker_InitAndDimensions(t *testing.T) {
	g := NewBitBreaker()
	bb, ok := g.(*BitBreaker)
	if !ok {
		t.Fatalf("expected *BitBreaker, got %T", g)
	}

	if bb.Title() != "Bit Breaker 🧱 ⚡" {
		t.Errorf("unexpected title: %s", bb.Title())
	}
	if bb.IsGameOver() {
		t.Errorf("expected new game not to be over")
	}
	if bb.Score() != 0 {
		t.Errorf("expected initial score 0, got %d", bb.Score())
	}
	if bb.lives != 3 {
		t.Errorf("expected 3 lives, got %d", bb.lives)
	}
	if len(bb.bricks) != 32 {
		t.Fatalf("expected 32 bricks, got %d", len(bb.bricks))
	}

	w, h := bb.Dimensions()
	if w != 60 || h != 25 {
		t.Errorf("expected dimensions (60, 25), got (%d, %d)", w, h)
	}
}

func TestBitBreaker_PaddleMovementAndLaunch(t *testing.T) {
	g := NewBitBreaker()
	bb := g.(*BitBreaker)
	startPaddleX := bb.paddleX

	// Move Left
	g.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if bb.paddleX != startPaddleX-3 {
		t.Errorf("expected paddleX to decrease by 3, got %d vs %d", bb.paddleX, startPaddleX-3)
	}

	// Move Right
	g.Update(tea.KeyMsg{Type: tea.KeyRight})
	if bb.paddleX != startPaddleX {
		t.Errorf("expected paddleX to return to start, got %d vs %d", bb.paddleX, startPaddleX)
	}

	// Before launch, ball stays with paddle
	if bb.launched {
		t.Errorf("expected ball not launched yet")
	}

	// Launch with Space
	g.Update(tea.KeyMsg{Type: tea.KeySpace})
	if !bb.launched {
		t.Errorf("expected ball to be launched after Space")
	}
}

func TestBitBreaker_BallWallBounces(t *testing.T) {
	g := NewBitBreaker()
	bb := g.(*BitBreaker)
	bb.launched = true

	// Put ball near right wall moving right
	bb.ballX = float64(bb.width - 1)
	bb.ballY = 5
	bb.ballDX = 1.0
	bb.ballDY = -0.5

	g.Update(TickMsg{})

	if bb.ballDX >= 0 {
		t.Errorf("expected ballDX to reflect negative after right wall, got %f", bb.ballDX)
	}

	// Put ball near ceiling moving up
	bb.ballY = 0
	bb.ballDY = -0.5

	g.Update(TickMsg{})

	if bb.ballDY <= 0 {
		t.Errorf("expected ballDY to reflect positive after ceiling, got %f", bb.ballDY)
	}
}

func TestBitBreaker_BrickCollision(t *testing.T) {
	g := NewBitBreaker()
	bb := g.(*BitBreaker)
	bb.launched = true

	// Position ball directly hitting brick 0 (which is at Y = 1)
	target := &bb.bricks[0]
	bb.ballX = float64(target.X + 1)
	bb.ballY = 1.5
	bb.ballDX = 0.0
	bb.ballDY = -0.5
	initialScore := bb.score

	g.Update(TickMsg{})

	if target.Alive {
		t.Errorf("expected brick to be destroyed upon collision")
	}
	if bb.score <= initialScore {
		t.Errorf("expected score to increase, got %d vs %d", bb.score, initialScore)
	}
	if bb.ballDY <= 0 {
		t.Errorf("expected ballDY to bounce downward, got %f", bb.ballDY)
	}
}

func TestBitBreaker_BallDropAndGameOver(t *testing.T) {
	g := NewBitBreaker()
	bb := g.(*BitBreaker)
	bb.launched = true

	// Ball drops below floor
	bb.ballY = float64(bb.height - 1)
	bb.ballDY = 1.0
	g.Update(TickMsg{})

	if bb.lives != 2 {
		t.Errorf("expected 2 lives after 1 drop, got %d", bb.lives)
	}
	if bb.launched {
		t.Errorf("expected ball reset to paddle (unlaunched)")
	}

	// Drop two more times
	bb.launched = true
	bb.ballY = float64(bb.height - 1)
	bb.ballDY = 1.0
	g.Update(TickMsg{})

	bb.launched = true
	bb.ballY = float64(bb.height - 1)
	bb.ballDY = 1.0
	g.Update(TickMsg{})

	if !bb.IsGameOver() {
		t.Errorf("expected game over after losing all 3 lives")
	}
}

func TestBitBreaker_RestartPreservesHighScore(t *testing.T) {
	g := NewBitBreaker()
	bb := g.(*BitBreaker)
	bb.score = 240
	bb.highScore = 240
	bb.gameOver = true

	// Press Space to restart
	g.Update(tea.KeyMsg{Type: tea.KeySpace})

	if bb.IsGameOver() {
		t.Errorf("expected game over cleared on restart")
	}
	if bb.score != 0 {
		t.Errorf("expected score reset to 0, got %d", bb.score)
	}
	if bb.highScore != 240 {
		t.Errorf("expected highScore 240 preserved, got %d", bb.highScore)
	}
	if bb.lives != 3 {
		t.Errorf("expected 3 lives on restart, got %d", bb.lives)
	}
}

func TestBitBreaker_ViewRendering(t *testing.T) {
	g := NewBitBreaker()
	viewNormal := g.View(52)
	if !strings.Contains(viewNormal, "SCORE: 00000") {
		t.Errorf("view missing score header:\n%s", viewNormal)
	}
	if !strings.Contains(viewNormal, "LIVES:") {
		t.Errorf("view missing lives in header:\n%s", viewNormal)
	}
	if !strings.Contains(viewNormal, "[══════]") {
		t.Errorf("view missing paddle in view:\n%s", viewNormal)
	}

	bb := g.(*BitBreaker)
	bb.gameOver = true
	viewDead := g.View(52)
	if !strings.Contains(viewDead, "ALL PACKETS DROPPED") {
		t.Errorf("view missing game over banner:\n%s", viewDead)
	}
}
