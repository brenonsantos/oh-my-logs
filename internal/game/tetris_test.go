package game

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestBufferStack_InitAndDimensions(t *testing.T) {
	g := NewBufferStack()
	bs, ok := g.(*BufferStack)
	if !ok {
		t.Fatalf("expected *BufferStack, got %T", g)
	}

	if bs.Title() != "Buffer Stack 🕹 [TETRIS]" {
		t.Errorf("unexpected title: %s", bs.Title())
	}
	if bs.IsGameOver() {
		t.Errorf("expected game not over initially")
	}
	if bs.Score() != 0 {
		t.Errorf("expected score 0, got %d", bs.Score())
	}
	if bs.level != 1 {
		t.Errorf("expected level 1, got %d", bs.level)
	}

	w, h := bs.Dimensions()
	if w != 54 || h != 25 {
		t.Errorf("expected dimensions (54, 25), got (%d, %d)", w, h)
	}
}

func TestBufferStack_MovementAndRotation(t *testing.T) {
	g := NewBufferStack()
	bs := g.(*BufferStack)

	// Set piece to T tetromino at (4, 5)
	bs.current = activePiece{
		matrix: copyMatrix(pieceTemplates[2]), // T shape (3x3)
		x:      4,
		y:      5,
	}

	// Move Left
	g.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if bs.current.x != 3 {
		t.Errorf("expected x to decrease to 3, got %d", bs.current.x)
	}

	// Move Right
	g.Update(tea.KeyMsg{Type: tea.KeyRight})
	if bs.current.x != 4 {
		t.Errorf("expected x to return to 4, got %d", bs.current.x)
	}

	// Rotate Clockwise
	origMatrix := copyMatrix(bs.current.matrix)
	g.Update(tea.KeyMsg{Type: tea.KeyUp})
	if bs.current.matrix[0][1] != origMatrix[1][0] {
		t.Errorf("expected matrix rotation")
	}
}

func TestBufferStack_HardDropAndLock(t *testing.T) {
	g := NewBufferStack()
	bs := g.(*BufferStack)

	// Clear grid and place O tetromino at (4, 0)
	bs.grid = [18][10]int{}
	bs.current = activePiece{
		matrix: copyMatrix(pieceTemplates[1]), // O shape (2x2)
		x:      4,
		y:      0,
	}

	// Hard drop with Space
	g.Update(tea.KeyMsg{Type: tea.KeySpace})

	// O piece should have locked at the bottom: row 16 and 17, cols 4 and 5
	if bs.grid[16][4] != 2 || bs.grid[16][5] != 2 || bs.grid[17][4] != 2 || bs.grid[17][5] != 2 {
		t.Errorf("expected O piece locked at rows 16-17 cols 4-5, grid row 17: %v", bs.grid[17])
	}
	if bs.score <= 0 {
		t.Errorf("expected score increase from hard drop, got %d", bs.score)
	}
}

func TestBufferStack_LineClearAndScoring(t *testing.T) {
	g := NewBufferStack()
	bs := g.(*BufferStack)

	// Fill row 17 except column 0
	for x := 1; x < 10; x++ {
		bs.grid[17][x] = 1
	}

	// Place I block vertically at col 0, row 14..17
	bs.current = activePiece{
		matrix: [][]int{
			{1},
			{1},
			{1},
			{1},
		},
		x: 0,
		y: 14,
	}

	// Lock piece
	bs.lockPiece()

	if bs.lines != 1 {
		t.Errorf("expected 1 line cleared, got %d", bs.lines)
	}
	if bs.score != 100 {
		t.Errorf("expected 100 points for single line clear, got %d", bs.score)
	}
	// Row 17 should now be empty (shifted down)
	if bs.grid[17][5] != 0 {
		t.Errorf("expected row 17 cleared, got cell: %d", bs.grid[17][5])
	}
}

func TestBufferStack_TopOutGameOver(t *testing.T) {
	g := NewBufferStack()
	bs := g.(*BufferStack)

	// Fill the top rows to block spawning
	for x := 0; x < 10; x++ {
		bs.grid[0][x] = 1
		bs.grid[1][x] = 1
	}

	bs.spawnPiece()

	if !bs.IsGameOver() {
		t.Errorf("expected game over from top-out")
	}
}

func TestBufferStack_RestartPreservesHighScore(t *testing.T) {
	g := NewBufferStack()
	bs := g.(*BufferStack)
	bs.score = 500
	bs.highScore = 500
	bs.gameOver = true

	// Press Space to restart
	g.Update(tea.KeyMsg{Type: tea.KeySpace})

	if bs.IsGameOver() {
		t.Errorf("expected game over cleared on restart")
	}
	if bs.score != 0 {
		t.Errorf("expected score reset to 0, got %d", bs.score)
	}
	if bs.highScore != 500 {
		t.Errorf("expected highScore 500 preserved, got %d", bs.highScore)
	}
}

func TestBufferStack_ViewRendering(t *testing.T) {
	g := NewBufferStack()
	viewNormal := g.View(50)
	if !strings.Contains(viewNormal, "BUFFER STACK") {
		t.Errorf("view missing title:\n%s", viewNormal)
	}
	if !strings.Contains(viewNormal, "NEXT FRAME:") {
		t.Errorf("view missing NEXT FRAME panel:\n%s", viewNormal)
	}
	if !strings.Contains(viewNormal, "LEVEL:") {
		t.Errorf("view missing LEVEL info:\n%s", viewNormal)
	}

	bs := g.(*BufferStack)
	bs.gameOver = true
	viewDead := g.View(50)
	if !strings.Contains(viewDead, "STACK OVERFLOW") {
		t.Errorf("view missing game over banner:\n%s", viewDead)
	}
}
