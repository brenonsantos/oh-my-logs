package game

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestByteSnake_InitAndMovement(t *testing.T) {
	g := NewByteSnake()
	snake, ok := g.(*ByteSnake)
	if !ok {
		t.Fatalf("expected *ByteSnake, got %T", g)
	}

	if snake.Title() != "Byte Snake 🐍 0x42" {
		t.Errorf("unexpected title: %s", snake.Title())
	}
	if snake.IsGameOver() {
		t.Errorf("expected new game not to be over")
	}
	if snake.Score() != 0 {
		t.Errorf("expected score 0, got %d", snake.Score())
	}
	if len(snake.snake) != 3 {
		t.Fatalf("expected initial snake length 3, got %d", len(snake.snake))
	}

	initialHead := snake.snake[0]

	// Tick forward once (default dir: Right)
	g, cmd := g.Update(TickMsg{})
	if cmd == nil {
		t.Errorf("expected non-nil cmd on tick")
	}
	snake = g.(*ByteSnake)

	newHead := snake.snake[0]
	if newHead.X != initialHead.X+1 || newHead.Y != initialHead.Y {
		t.Errorf("expected head to advance right, got (%d,%d) vs initial (%d,%d)",
			newHead.X, newHead.Y, initialHead.X, initialHead.Y)
	}
}

func TestByteSnake_SteeringAndReverseProtection(t *testing.T) {
	g := NewByteSnake().(*ByteSnake)

	// Default moving Right. Attempt to move Left should be ignored to prevent 180° instant suicide.
	g.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	if g.nextDir != dirRight {
		t.Errorf("expected nextDir to remain dirRight, got %v", g.nextDir)
	}

	// Turn Up with 'w'
	g.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("w")})
	if g.nextDir != dirUp {
		t.Errorf("expected nextDir to be dirUp, got %v", g.nextDir)
	}

	// Tick to apply direction
	g.Update(TickMsg{})
	if g.currentDir != dirUp {
		t.Errorf("expected currentDir to be dirUp, got %v", g.currentDir)
	}

	// Moving Up, attempt to move Down with 's' should be ignored
	g.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	if g.nextDir != dirUp {
		t.Errorf("expected nextDir to remain dirUp, got %v", g.nextDir)
	}

	// Turn Left with Arrow Left
	g.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if g.nextDir != dirLeft {
		t.Errorf("expected nextDir to be dirLeft, got %v", g.nextDir)
	}
}

func TestByteSnake_EatFoodAndGrow(t *testing.T) {
	g := NewByteSnake().(*ByteSnake)

	// Place food directly 1 cell to the right of the head
	head := g.snake[0]
	g.food = Point2D{X: head.X + 1, Y: head.Y}

	initialLen := len(g.snake)
	initialScore := g.score

	// Tick should eat the food
	g.Update(TickMsg{})

	if g.score != initialScore+10 {
		t.Errorf("expected score to increase by 10, got %d", g.score)
	}
	if len(g.snake) != initialLen+1 {
		t.Errorf("expected snake length %d, got %d", initialLen+1, len(g.snake))
	}
	if g.foodsEaten != 1 {
		t.Errorf("expected foodsEaten 1, got %d", g.foodsEaten)
	}
}

func TestByteSnake_WallCollision(t *testing.T) {
	g := NewByteSnake().(*ByteSnake)

	// Move snake right near border
	g.snake[0] = Point2D{X: g.gridW - 1, Y: 5}
	g.snake[1] = Point2D{X: g.gridW - 2, Y: 5}
	g.snake[2] = Point2D{X: g.gridW - 3, Y: 5}
	g.currentDir = dirRight
	g.nextDir = dirRight

	// Tick into wall
	g.Update(TickMsg{})

	if !g.IsGameOver() {
		t.Errorf("expected game over after hitting right wall")
	}
}

func TestByteSnake_SelfCollision(t *testing.T) {
	g := NewByteSnake().(*ByteSnake)

	// Form a snake that loops into itself:
	// head at (5,5), body at (6,5), (6,6), (5,6), (5,5)
	g.snake = []Point2D{
		{X: 5, Y: 5},
		{X: 6, Y: 5},
		{X: 6, Y: 6},
		{X: 5, Y: 6},
		{X: 4, Y: 6},
	}
	g.currentDir = dirRight
	g.nextDir = dirRight
	// Head will move to (6,5) which is snake[1]
	g.food = Point2D{X: 0, Y: 0} // food elsewhere

	g.Update(TickMsg{})

	if !g.IsGameOver() {
		t.Errorf("expected game over after self-collision")
	}
}

func TestByteSnake_RestartPreservesHighScore(t *testing.T) {
	g := NewByteSnake().(*ByteSnake)
	g.score = 50
	g.highScore = 50
	g.gameOver = true

	// Press Space to restart
	g.Update(tea.KeyMsg{Type: tea.KeySpace})

	if g.IsGameOver() {
		t.Errorf("expected game over to be cleared on restart")
	}
	if g.score != 0 {
		t.Errorf("expected score reset to 0, got %d", g.score)
	}
	if g.highScore != 50 {
		t.Errorf("expected highScore 50 preserved, got %d", g.highScore)
	}
}

func TestByteSnake_ViewRendering(t *testing.T) {
	g := NewByteSnake()
	viewNormal := g.View(46)
	if !strings.Contains(viewNormal, "SCORE: 00000") {
		t.Errorf("view missing score header:\n%s", viewNormal)
	}
	if !strings.Contains(viewNormal, "┌") || !strings.Contains(viewNormal, "┘") {
		t.Errorf("view missing border box:\n%s", viewNormal)
	}

	snake := g.(*ByteSnake)
	snake.gameOver = true
	viewDead := g.View(46)
	if !strings.Contains(viewDead, "BUFFER OVERFLOW") {
		t.Errorf("view missing game over banner:\n%s", viewDead)
	}
}
