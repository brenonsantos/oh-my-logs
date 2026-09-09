package game

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestPong_InitAndDimensions(t *testing.T) {
	g := NewPong()
	p, ok := g.(*Pong)
	if !ok {
		t.Fatalf("expected *Pong, got %T", g)
	}

	if p.Title() != "Ping-Pong Buffer 🏓 [PONG]" {
		t.Errorf("unexpected title: %s", p.Title())
	}
	if p.IsGameOver() {
		t.Errorf("expected game not over initially")
	}
	if p.Score() != 0 {
		t.Errorf("expected score 0, got %d", p.Score())
	}

	w, h := p.Dimensions()
	if w != 60 || h != 20 {
		t.Errorf("expected dimensions (60, 20), got (%d, %d)", w, h)
	}

	if !p.inServe {
		t.Errorf("expected to start in serve mode")
	}
}

func TestPong_PlayerMovement(t *testing.T) {
	g := NewPong()
	p := g.(*Pong)

	initialY := p.playerY

	// Move Up
	g.Update(tea.KeyMsg{Type: tea.KeyUp})
	if p.playerY >= initialY {
		t.Errorf("expected playerY to decrease on Up")
	}

	// Move Down
	g.Update(tea.KeyMsg{Type: tea.KeyDown})
	g.Update(tea.KeyMsg{Type: tea.KeyDown})
	if p.playerY <= initialY {
		t.Errorf("expected playerY to increase on Down")
	}

	// Clamp test at bottom
	for i := 0; i < 20; i++ {
		g.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	max := float64(p.height - p.paddleH)
	if p.playerY > max {
		t.Errorf("expected playerY <= %f, got %f", max, p.playerY)
	}
}

func TestPong_ServeAndBallMovement(t *testing.T) {
	g := NewPong()
	p := g.(*Pong)

	// Press Space to serve
	g.Update(tea.KeyMsg{Type: tea.KeySpace})
	if p.inServe {
		t.Errorf("expected inServe false after space")
	}

	origX := p.ballX
	origY := p.ballY

	// Tick
	g.Update(TickMsg{})
	if p.ballX == origX && p.ballY == origY {
		t.Errorf("expected ball to move on tick")
	}
}

func TestPong_WallBounce(t *testing.T) {
	g := NewPong()
	p := g.(*Pong)
	p.inServe = false

	// Position at ceiling moving upwards
	p.ballY = 0
	p.ballVy = -1.0

	g.Update(TickMsg{})
	if p.ballVy <= 0 {
		t.Errorf("expected ballVy to bounce positive, got %f", p.ballVy)
	}
}

func TestPong_PaddleBounce(t *testing.T) {
	g := NewPong()
	p := g.(*Pong)
	p.inServe = false

	// Position ball right at player paddle
	p.playerY = 5.0
	p.ballX = float64(pongLeftCol + 1)
	p.ballY = 6.0
	p.ballVx = -1.0

	g.Update(TickMsg{})
	if p.ballVx <= 0 {
		t.Errorf("expected ballVx to bounce positive on paddle hit, got %f", p.ballVx)
	}
	if p.rally != 1 {
		t.Errorf("expected rally count 1, got %d", p.rally)
	}
}

func TestPong_ScoringAndGameOver(t *testing.T) {
	g := NewPong()
	p := g.(*Pong)
	p.inServe = false

	// Ball passes player on left -> CPU scores
	p.ballX = -1.0
	g.Update(TickMsg{})
	if p.cpuPts != 1 {
		t.Errorf("expected CPU to score, cpuPts = %d", p.cpuPts)
	}

	// Fast forward to game over
	p.cpuPts = pongMaxScore - 1
	p.inServe = false
	p.ballX = -1.0
	g.Update(TickMsg{})

	if !p.IsGameOver() {
		t.Errorf("expected game over when CPU reaches max score")
	}
	if p.winner != "RX" {
		t.Errorf("expected RX to be winner, got %s", p.winner)
	}

	// Rematch
	g.Update(tea.KeyMsg{Type: tea.KeySpace})
	if p.IsGameOver() {
		t.Errorf("expected game over to be cleared on rematch")
	}
	if p.cpuPts != 0 || p.playerPts != 0 {
		t.Errorf("expected scores reset to 0")
	}
}

func TestPong_View(t *testing.T) {
	g := NewPong()
	p := g.(*Pong)

	v := p.View(60)
	if !strings.Contains(v, "PING-PONG BUFFER") {
		t.Errorf("expected title in view")
	}
	if !strings.Contains(v, "TX:") || !strings.Contains(v, "RX:") {
		t.Errorf("expected score labels in view")
	}

	p.gameOver = true
	p.winner = "TX"
	vWin := p.View(60)
	if !strings.Contains(vWin, "TX BUFFER DOMINANCE") {
		t.Errorf("expected win message in view")
	}
}
