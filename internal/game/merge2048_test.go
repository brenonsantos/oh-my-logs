package game

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestMerge2048_InitAndDimensions(t *testing.T) {
	g := NewMerge2048()
	m, ok := g.(*Merge2048)
	if !ok {
		t.Fatalf("expected *Merge2048, got %T", g)
	}

	if m.Title() != "2048 Bytes 🔢 [MERGE]" {
		t.Errorf("unexpected title: %s", m.Title())
	}
	if m.IsGameOver() {
		t.Errorf("expected game not over initially")
	}
	if m.Score() != 0 {
		t.Errorf("expected score 0, got %d", m.Score())
	}

	w, h := m.Dimensions()
	if w != 54 || h != 20 {
		t.Errorf("expected dimensions (54, 20), got (%d, %d)", w, h)
	}

	// Should start with exactly 2 non-zero tiles
	count := 0
	for r := 0; r < 4; r++ {
		for c := 0; c < 4; c++ {
			if m.board[r][c] != 0 {
				count++
			}
		}
	}
	if count != 2 {
		t.Errorf("expected 2 initial tiles, found %d", count)
	}
}

func TestMerge2048_SlideAndMergeLine(t *testing.T) {
	tests := []struct {
		input       [4]int
		expected    [4]int
		expectedPts int
		changed     bool
	}{
		{
			input:       [4]int{2, 2, 0, 0},
			expected:    [4]int{4, 0, 0, 0},
			expectedPts: 4,
			changed:     true,
		},
		{
			input:       [4]int{2, 2, 2, 2},
			expected:    [4]int{4, 4, 0, 0},
			expectedPts: 8,
			changed:     true,
		},
		{
			input:       [4]int{2, 0, 2, 4},
			expected:    [4]int{4, 4, 0, 0},
			expectedPts: 4,
			changed:     true,
		},
		{
			input:       [4]int{4, 2, 2, 0},
			expected:    [4]int{4, 4, 0, 0},
			expectedPts: 4,
			changed:     true,
		},
		{
			input:       [4]int{2, 4, 8, 16},
			expected:    [4]int{2, 4, 8, 16},
			expectedPts: 0,
			changed:     false,
		},
		{
			input:       [4]int{0, 0, 0, 0},
			expected:    [4]int{0, 0, 0, 0},
			expectedPts: 0,
			changed:     false,
		},
	}

	for _, tc := range tests {
		got, pts, ch := slideAndMergeLine(tc.input)
		if got != tc.expected || pts != tc.expectedPts || ch != tc.changed {
			t.Errorf("slideAndMergeLine(%v) = (%v, %d, %v); expected (%v, %d, %v)",
				tc.input, got, pts, ch, tc.expected, tc.expectedPts, tc.changed)
		}
	}
}

func TestMerge2048_Movement(t *testing.T) {
	g := NewMerge2048()
	m := g.(*Merge2048)

	// Preset custom board
	m.board = [4][4]int{
		{2, 2, 0, 0},
		{0, 0, 0, 0},
		{4, 0, 4, 0},
		{0, 0, 0, 0},
	}
	m.score = 0

	// Move Left (via 'a')
	g.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if m.board[0][0] != 4 {
		t.Errorf("expected merged 4 at (0,0), got %d", m.board[0][0])
	}
	if m.board[2][0] != 8 {
		t.Errorf("expected merged 8 at (2,0), got %d", m.board[2][0])
	}
	if m.score != 12 {
		t.Errorf("expected score 12, got %d", m.score)
	}

	// Move Right
	m.board = [4][4]int{
		{0, 0, 2, 2},
		{0, 0, 0, 0},
		{0, 0, 0, 0},
		{0, 0, 0, 0},
	}
	g.Update(tea.KeyMsg{Type: tea.KeyRight})
	if m.board[0][3] != 4 {
		t.Errorf("expected 4 at (0,3), got %d", m.board[0][3])
	}

	// Move Up
	m.board = [4][4]int{
		{2, 0, 0, 0},
		{2, 0, 0, 0},
		{0, 0, 0, 0},
		{0, 0, 0, 0},
	}
	g.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.board[0][0] != 4 {
		t.Errorf("expected 4 at (0,0) after moveUp, got %d", m.board[0][0])
	}

	// Move Down
	m.board = [4][4]int{
		{0, 0, 0, 2},
		{0, 0, 0, 2},
		{0, 0, 0, 0},
		{0, 0, 0, 0},
	}
	g.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.board[3][3] != 4 {
		t.Errorf("expected 4 at (3,3) after moveDown, got %d", m.board[3][3])
	}
}

func TestMerge2048_GameOverAndRestart(t *testing.T) {
	g := NewMerge2048()
	m := g.(*Merge2048)

	// Board with no possible moves
	m.board = [4][4]int{
		{2, 4, 2, 4},
		{4, 2, 4, 2},
		{2, 4, 2, 4},
		{4, 2, 4, 2},
	}

	if m.canMove() {
		t.Errorf("expected canMove to be false for packed board")
	}

	// A move attempt on packed board should trigger game over
	m.gameOver = true

	v := m.View(54)
	if !strings.Contains(v, "NO MORE MOVES") {
		t.Errorf("expected game over banner in view")
	}

	// Press Space to restart
	g.Update(tea.KeyMsg{Type: tea.KeySpace})
	if m.IsGameOver() {
		t.Errorf("expected game to reset and not be over")
	}
	if m.Score() != 0 {
		t.Errorf("expected score reset to 0, got %d", m.Score())
	}
}

func TestMerge2048_View(t *testing.T) {
	g := NewMerge2048()
	m := g.(*Merge2048)

	v := m.View(54)
	if !strings.Contains(v, "2048 BYTES") {
		t.Errorf("expected title in view")
	}

	m.won = true
	vWon := m.View(54)
	if !strings.Contains(vWon, "2048 REACHED") {
		t.Errorf("expected win banner in view")
	}
}
