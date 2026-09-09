package game

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestFlappyPacket_InitAndDimensions(t *testing.T) {
	g := NewFlappyPacket()
	f, ok := g.(*FlappyPacket)
	if !ok {
		t.Fatalf("expected *FlappyPacket, got %T", g)
	}

	if f.Title() != "Flappy Packet 🪽 [FLAP]" {
		t.Errorf("unexpected title: %s", f.Title())
	}
	if f.IsGameOver() {
		t.Errorf("expected game not over initially")
	}
	if f.Score() != 0 {
		t.Errorf("expected score 0, got %d", f.Score())
	}

	w, h := f.Dimensions()
	if w != 58 || h != 20 {
		t.Errorf("expected dimensions (58, 20), got (%d, %d)", w, h)
	}

	if len(f.pipes) == 0 {
		t.Errorf("expected initial pipes to be spawned")
	}
}

func TestFlappyPacket_Flap(t *testing.T) {
	g := NewFlappyPacket()
	f := g.(*FlappyPacket)

	if f.started {
		t.Errorf("expected not started initially")
	}

	// Jump with space
	g.Update(tea.KeyMsg{Type: tea.KeySpace})
	if !f.started {
		t.Errorf("expected started after flap")
	}
	if f.velY != f.jumpVel {
		t.Errorf("expected velY %f, got %f", f.jumpVel, f.velY)
	}
}

func TestFlappyPacket_TickAndMovement(t *testing.T) {
	g := NewFlappyPacket()
	f := g.(*FlappyPacket)

	// Start game
	g.Update(tea.KeyMsg{Type: tea.KeySpace})
	initialY := f.packetY
	initialPipeX := f.pipes[0].x

	// Tick
	g.Update(TickMsg{})

	if f.packetY == initialY {
		t.Errorf("expected packetY to change after tick")
	}
	if f.pipes[0].x >= initialPipeX {
		t.Errorf("expected pipes to scroll left")
	}
}

func TestFlappyPacket_GroundCollision(t *testing.T) {
	g := NewFlappyPacket()
	f := g.(*FlappyPacket)
	f.started = true
	f.packetY = float64(f.height)

	if !f.checkCollisions() {
		t.Errorf("expected collision at ground")
	}

	g.Update(TickMsg{})
	if !f.IsGameOver() {
		t.Errorf("expected game over on ground hit")
	}
}

func TestFlappyPacket_PipeCollision(t *testing.T) {
	g := NewFlappyPacket()
	f := g.(*FlappyPacket)
	f.started = true

	// Position pipe right on packet (flappyPacketX = 8), with gap far away
	f.pipes = []flappyPipe{
		{
			x:      float64(flappyPacketX),
			gapY:   10,
			gapH:   3,
			passed: false,
		},
	}
	// Packet at row 2 (above gap)
	f.packetY = 2.0

	if !f.checkCollisions() {
		t.Errorf("expected pipe collision when hitting obstacle")
	}
}

func TestFlappyPacket_Restart(t *testing.T) {
	g := NewFlappyPacket()
	f := g.(*FlappyPacket)

	f.gameOver = true
	f.score = 15

	// Space restarts
	g.Update(tea.KeyMsg{Type: tea.KeySpace})
	if f.IsGameOver() {
		t.Errorf("expected game over cleared on restart")
	}
	if f.Score() != 0 {
		t.Errorf("expected score reset to 0, got %d", f.Score())
	}
}

func TestFlappyPacket_View(t *testing.T) {
	g := NewFlappyPacket()
	f := g.(*FlappyPacket)

	v := f.View(58)
	if !strings.Contains(v, "FLAPPY PACKET") {
		t.Errorf("expected title in view")
	}

	f.gameOver = true
	vDead := f.View(58)
	if !strings.Contains(vDead, "PACKET DROPPED") {
		t.Errorf("expected dead message in view")
	}
}
