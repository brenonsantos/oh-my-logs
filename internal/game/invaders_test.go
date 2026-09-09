package game

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestPacketDefender_InitAndDimensions(t *testing.T) {
	g := NewPacketDefender()
	pd, ok := g.(*PacketDefender)
	if !ok {
		t.Fatalf("expected *PacketDefender, got %T", g)
	}

	if pd.Title() != "Packet Defender 👾 ◄▲►" {
		t.Errorf("unexpected title: %s", pd.Title())
	}
	if pd.IsGameOver() {
		t.Errorf("expected new game not to be over")
	}
	if pd.Score() != 0 {
		t.Errorf("expected score 0, got %d", pd.Score())
	}
	if pd.shields != 3 {
		t.Errorf("expected 3 shields, got %d", pd.shields)
	}
	if len(pd.invaders) != 18 {
		t.Fatalf("expected 18 invaders, got %d", len(pd.invaders))
	}

	w, h := pd.Dimensions()
	if w != 64 || h != 25 {
		t.Errorf("expected dimensions (64, 25), got (%d, %d)", w, h)
	}
}

func TestPacketDefender_MovementAndFiring(t *testing.T) {
	g := NewPacketDefender()
	pd := g.(*PacketDefender)
	startX := pd.playerX

	// Move Left
	g.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if pd.playerX != startX-2 {
		t.Errorf("expected playerX to decrease by 2, got %d vs %d", pd.playerX, startX)
	}

	// Move Right
	g.Update(tea.KeyMsg{Type: tea.KeyRight})
	if pd.playerX != startX {
		t.Errorf("expected playerX to return to startX, got %d vs %d", pd.playerX, startX)
	}

	// Fire laser with Space
	g.Update(tea.KeyMsg{Type: tea.KeySpace})
	if len(pd.lasers) != 1 {
		t.Fatalf("expected 1 active laser, got %d", len(pd.lasers))
	}
	laser := pd.lasers[0]
	if laser.X != pd.playerX+1 || laser.Y != pd.playerY-1 {
		t.Errorf("laser spawned at unexpected pos (%d, %d)", laser.X, laser.Y)
	}

	// Tick should advance laser upward
	g.Update(TickMsg{})
	if pd.lasers[0].Y != laser.Y-1 {
		t.Errorf("expected laser Y to decrease to %d, got %d", laser.Y-1, pd.lasers[0].Y)
	}
}

func TestPacketDefender_LaserHitsInvader(t *testing.T) {
	g := NewPacketDefender()
	pd := g.(*PacketDefender)

	// Target first invader
	target := &pd.invaders[0]
	pd.lasers = []projectile{
		{X: target.X, Y: target.Y + 1},
	}
	initialScore := pd.score

	// Tick should move laser into target and destroy it
	g.Update(TickMsg{})

	if target.Alive {
		t.Errorf("expected invader to be destroyed")
	}
	if pd.score <= initialScore {
		t.Errorf("expected score to increase, got %d vs %d", pd.score, initialScore)
	}
	if len(pd.lasers) != 0 {
		t.Errorf("expected laser to be consumed upon hit")
	}
}

func TestPacketDefender_BombHitsPlayerShields(t *testing.T) {
	g := NewPacketDefender()
	pd := g.(*PacketDefender)

	// Spawn bomb 1 cell above player
	pd.bombs = []projectile{
		{X: pd.playerX + 1, Y: pd.playerY - 1},
	}

	// Tick moves bomb into player
	g.Update(TickMsg{})

	if pd.shields != 2 {
		t.Errorf("expected shields to decrease to 2, got %d", pd.shields)
	}
	if pd.IsGameOver() {
		t.Errorf("expected player not to be game over with 2 shields left")
	}

	// Two more hits should trigger game over
	pd.bombs = []projectile{
		{X: pd.playerX + 1, Y: pd.playerY - 1},
	}
	g.Update(TickMsg{})
	pd.bombs = []projectile{
		{X: pd.playerX + 1, Y: pd.playerY - 1},
	}
	g.Update(TickMsg{})

	if !pd.IsGameOver() {
		t.Errorf("expected game over after 3 shield hits")
	}
}

func TestPacketDefender_GroundBreach(t *testing.T) {
	g := NewPacketDefender()
	pd := g.(*PacketDefender)

	// Move invader right next to right wall and close to ground
	for i := range pd.invaders {
		pd.invaders[i].Alive = false
	}
	pd.invaders[0].Alive = true
	pd.invaders[0].X = pd.width - 3
	pd.invaders[0].Y = pd.height - 2
	pd.invaderDir = 1
	pd.marchTimer = 1

	// Tick causes invader to hit wall, drop, and breach ground
	g.Update(TickMsg{})

	if !pd.IsGameOver() {
		t.Errorf("expected game over from invader ground breach")
	}
}

func TestPacketDefender_RestartPreservesHighScore(t *testing.T) {
	g := NewPacketDefender()
	pd := g.(*PacketDefender)
	pd.score = 250
	pd.highScore = 250
	pd.gameOver = true

	// Press Space to restart
	g.Update(tea.KeyMsg{Type: tea.KeySpace})

	if pd.IsGameOver() {
		t.Errorf("expected game over cleared on restart")
	}
	if pd.score != 0 {
		t.Errorf("expected score reset to 0, got %d", pd.score)
	}
	if pd.highScore != 250 {
		t.Errorf("expected highScore 250 preserved, got %d", pd.highScore)
	}
	if pd.shields != 3 {
		t.Errorf("expected shields reset to 3, got %d", pd.shields)
	}
}

func TestPacketDefender_ViewRendering(t *testing.T) {
	g := NewPacketDefender()
	viewNormal := g.View(60)
	if !strings.Contains(viewNormal, "SCORE: 00000") {
		t.Errorf("view missing score header:\n%s", viewNormal)
	}
	if !strings.Contains(viewNormal, "SHIELDS:") {
		t.Errorf("view missing shields in header:\n%s", viewNormal)
	}
	if !strings.Contains(viewNormal, "◄▲►") {
		t.Errorf("view missing player ship sprite:\n%s", viewNormal)
	}

	pd := g.(*PacketDefender)
	pd.gameOver = true
	pd.gameOverDesc = "💥 DEFENDER DESTROYED"
	viewDead := g.View(60)
	if !strings.Contains(viewDead, "💥 DEFENDER DESTROYED") {
		t.Errorf("view missing game over banner:\n%s", viewDead)
	}
}
