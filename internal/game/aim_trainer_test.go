package game

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestAimTrainer_InitialState(t *testing.T) {
	at := NewAimTrainer().(*AimTrainer)
	if at.Title() != "🎯 Aim Trainer (Blitz)" {
		t.Fatalf("unexpected title: %q", at.Title())
	}
	w, h := at.Dimensions()
	if w <= 0 || h <= 0 {
		t.Fatalf("invalid dimensions: %d x %d", w, h)
	}
	if len(at.targets) != 3 {
		t.Fatalf("expected exactly 3 targets, got %d", len(at.targets))
	}
	if at.timeLeft != 10*time.Second {
		t.Fatalf("expected 10s timeLeft, got %v", at.timeLeft)
	}
	if at.Score() != 0 {
		t.Fatalf("expected initial score 0, got %d", at.Score())
	}
	if at.IsGameOver() {
		t.Fatalf("game should not be over initially")
	}

	// Verify all 3 targets have valid coordinates and do not overlap
	for i := 0; i < len(at.targets); i++ {
		t1 := at.targets[i]
		if t1.X < 0 || t1.X+targetWidth > at.playWidth || t1.Y < 0 || t1.Y+targetHeight > at.playHeight {
			t.Fatalf("target %d out of bounds: %+v", i, t1)
		}
		for j := i + 1; j < len(at.targets); j++ {
			t2 := at.targets[j]
			if targetsOverlap(t1.X, t1.Y, t2.X, t2.Y) {
				t.Fatalf("targets %d and %d overlap: %+v vs %+v", i, j, t1, t2)
			}
		}
	}
}

func TestAimTrainer_HitAndMiss(t *testing.T) {
	at := NewAimTrainer().(*AimTrainer)

	// Pick target 0
	target := at.targets[0]
	// Click inside target (center of target)
	clickX := target.X + 2
	clickY := target.Y + 1

	hit := at.HandleClick(clickX, clickY)
	if !hit {
		t.Fatalf("expected hit on target at (%d, %d)", clickX, clickY)
	}
	if at.hits != 1 {
		t.Fatalf("expected 1 hit, got %d", at.hits)
	}
	if at.score <= 0 {
		t.Fatalf("expected score > 0, got %d", at.score)
	}
	if at.combo != 2 {
		t.Fatalf("expected combo to become 2, got %d", at.combo)
	}
	if len(at.reactionTimes) != 1 {
		t.Fatalf("expected 1 recorded reaction time, got %d", len(at.reactionTimes))
	}
	if len(at.targets) != 3 {
		t.Fatalf("expected 3 targets after replacement, got %d", len(at.targets))
	}

	// Click empty space that definitely has no target
	emptyX := -1
	emptyY := -1
	for y := 0; y < at.playHeight; y++ {
		for x := 0; x < at.playWidth; x++ {
			occupied := false
			for _, tg := range at.targets {
				if x >= tg.X && x < tg.X+targetWidth && y >= tg.Y && y < tg.Y+targetHeight {
					occupied = true
					break
				}
			}
			if !occupied {
				emptyX = x
				emptyY = y
				break
			}
		}
		if emptyX >= 0 {
			break
		}
	}

	if emptyX < 0 {
		t.Fatalf("could not find empty spot on playfield")
	}

	miss := at.HandleClick(emptyX, emptyY)
	if miss {
		t.Fatalf("expected miss at empty coordinate (%d, %d)", emptyX, emptyY)
	}
	if at.misses != 1 {
		t.Fatalf("expected 1 miss, got %d", at.misses)
	}
	if at.combo != 1 {
		t.Fatalf("expected combo to reset to 1 after miss, got %d", at.combo)
	}
}

func TestAimTrainer_CountdownAndGameOver(t *testing.T) {
	at := NewAimTrainer().(*AimTrainer)

	// Fast forward 10 seconds through ticks
	for i := 0; i < 200; i++ {
		updated, _ := at.Update(TickMsg{})
		at = updated.(*AimTrainer)
	}

	if !at.IsGameOver() {
		t.Fatalf("expected game over after 10s of ticks")
	}

	view := at.View(60)
	if !strings.Contains(view, "GAME OVER") {
		t.Fatalf("expected view to contain GAME OVER, got:\n%s", view)
	}

	// Clicks during game over must NOT restart the game
	hit := at.HandleClick(10, 5)
	if hit {
		t.Fatalf("click during game over should not hit")
	}
	if !at.IsGameOver() {
		t.Fatalf("mouse click must not restart game when game over")
	}

	mouseUpdated, _ := at.Update(AimClickMsg{X: 10, Y: 5})
	at = mouseUpdated.(*AimTrainer)
	if !at.IsGameOver() {
		t.Fatalf("AimClickMsg must not restart game when game over")
	}

	// Restart via keyboard key (e.g. space or 'r')
	updated, _ := at.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	at = updated.(*AimTrainer)
	if at.IsGameOver() {
		t.Fatalf("game should be restarted after keyboard press")
	}
	if at.timeLeft != 10*time.Second {
		t.Fatalf("expected timeLeft 10s after restart, got %v", at.timeLeft)
	}
}

func TestAimTrainer_ComboAndRanks(t *testing.T) {
	at := NewAimTrainer().(*AimTrainer)

	// Hit 25 targets in a row
	for i := 0; i < 25; i++ {
		target := at.targets[0]
		updated, _ := at.Update(tea.MouseMsg{
			Action: tea.MouseActionPress,
			Button: tea.MouseButtonLeft,
			X:      target.X + 1,
			Y:      target.Y + 1,
		})
		at = updated.(*AimTrainer)
	}

	if at.hits != 25 {
		t.Fatalf("expected 25 hits, got %d", at.hits)
	}
	if at.misses != 0 {
		t.Fatalf("expected 0 misses, got %d", at.misses)
	}
	if at.accuracyPercent() != 100 {
		t.Fatalf("expected 100%% accuracy, got %d", at.accuracyPercent())
	}
	if at.combo != 26 {
		t.Fatalf("expected combo 26, got %d", at.combo)
	}

	rankStr, _ := at.rankBadge()
	if !strings.Contains(rankStr, "GODLIKE") {
		t.Fatalf("expected GODLIKE rank for 25 hits and 100%% acc, got %s", rankStr)
	}
}

