package game

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestMemorySweeper_InitAndDimensions(t *testing.T) {
	g := NewMemorySweeper()
	m, ok := g.(*MemorySweeper)
	if !ok {
		t.Fatalf("expected *MemorySweeper, got %T", g)
	}

	if m.Title() != "Memory Sweeper 💣 [SWEEP]" {
		t.Errorf("unexpected title: %s", m.Title())
	}
	if m.IsGameOver() {
		t.Errorf("expected game not over initially")
	}
	if m.Score() != 0 {
		t.Errorf("expected score 0, got %d", m.Score())
	}

	w, h := m.Dimensions()
	if w != 52 || h != 20 {
		t.Errorf("expected dimensions (52, 20), got (%d, %d)", w, h)
	}

	if m.cursorR != sweepRows/2 || m.cursorC != sweepCols/2 {
		t.Errorf("expected cursor centered at (%d, %d), got (%d, %d)",
			sweepRows/2, sweepCols/2, m.cursorR, m.cursorC)
	}
}

func TestMemorySweeper_CursorMovement(t *testing.T) {
	g := NewMemorySweeper()
	m := g.(*MemorySweeper)

	// Move Up
	g.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.cursorR != sweepRows/2-1 {
		t.Errorf("expected cursorR to decrement, got %d", m.cursorR)
	}

	// Move Left
	g.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if m.cursorC != sweepCols/2-1 {
		t.Errorf("expected cursorC to decrement, got %d", m.cursorC)
	}

	// Move Down
	g.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.cursorR != sweepRows/2 {
		t.Errorf("expected cursorR to increment, got %d", m.cursorR)
	}

	// Move Right
	g.Update(tea.KeyMsg{Type: tea.KeyRight})
	if m.cursorC != sweepCols/2 {
		t.Errorf("expected cursorC to increment, got %d", m.cursorC)
	}
}

func TestMemorySweeper_Flagging(t *testing.T) {
	g := NewMemorySweeper()
	m := g.(*MemorySweeper)

	// Flag current cell with 'f'
	g.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	if !m.grid[m.cursorR][m.cursorC].flagged {
		t.Errorf("expected cell to be flagged")
	}
	if m.flagsCount != 1 {
		t.Errorf("expected flagsCount 1, got %d", m.flagsCount)
	}

	// Unflag with 'm'
	g.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	if m.grid[m.cursorR][m.cursorC].flagged {
		t.Errorf("expected cell to be unflagged")
	}
	if m.flagsCount != 0 {
		t.Errorf("expected flagsCount 0, got %d", m.flagsCount)
	}
}

func TestMemorySweeper_FirstMoveSafe(t *testing.T) {
	g := NewMemorySweeper()
	m := g.(*MemorySweeper)

	// Reveal first cell
	g.Update(tea.KeyMsg{Type: tea.KeySpace})

	if !m.generated {
		t.Errorf("expected mines to be generated")
	}
	if m.grid[m.cursorR][m.cursorC].isMine {
		t.Errorf("first move clicked cell should never be a mine")
	}
	if !m.grid[m.cursorR][m.cursorC].revealed {
		t.Errorf("expected clicked cell to be revealed")
	}
	if m.IsGameOver() {
		t.Errorf("first move should not cause game over")
	}
}

func TestMemorySweeper_MineDetonation(t *testing.T) {
	g := NewMemorySweeper()
	m := g.(*MemorySweeper)

	// Force generation
	m.generateMines(0, 0)

	// Find a mine and reveal it
	mineR, mineC := -1, -1
	for r := 0; r < sweepRows; r++ {
		for c := 0; c < sweepCols; c++ {
			if m.grid[r][c].isMine {
				mineR, mineC = r, c
				break
			}
		}
		if mineR != -1 {
			break
		}
	}

	if mineR == -1 {
		t.Fatalf("no mine found on board")
	}

	m.cursorR = mineR
	m.cursorC = mineC
	g.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if !m.IsGameOver() {
		t.Errorf("expected game over after revealing mine")
	}

	v := m.View(52)
	if !strings.Contains(v, "SEGFAULT") {
		t.Errorf("expected segfault message in view")
	}

	// Restart
	g.Update(tea.KeyMsg{Type: tea.KeySpace})
	if m.IsGameOver() {
		t.Errorf("expected game not over after restart")
	}
}

func TestMemorySweeper_WinCondition(t *testing.T) {
	g := NewMemorySweeper()
	m := g.(*MemorySweeper)

	// Create custom grid with 1 mine at (0,0) and safe everywhere else
	m.grid = [sweepRows][sweepCols]sweeperCell{}
	m.grid[0][0].isMine = true
	m.grid[0][1].adjacent = 1 // adjacent to (0,0) mine, will not flood-fill
	m.minesCount = 1
	m.safeTotal = sweepRows*sweepCols - 1
	m.safeLeft = 1
	m.generated = true

	// Reveal the last safe cell
	m.reveal(0, 1)

	if !m.won {
		t.Errorf("expected won to be true when all safe cells cleared")
	}

	v := m.View(52)
	if !strings.Contains(v, "ALL SAFE SECTORS REVEALED") {
		t.Errorf("expected win message in view")
	}
}
