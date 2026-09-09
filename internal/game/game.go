package game

import (
	"math/rand"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// MiniGame represents a playable mini-game inside oh-my-logs.
type MiniGame interface {
	Title() string
	Init() tea.Cmd
	Update(msg tea.Msg) (MiniGame, tea.Cmd)
	View(width int) string
	IsGameOver() bool
	Score() int
}

// TickMsg is sent periodically to drive the mini-game physics and rendering loop.
type TickMsg struct{}

// Tick returns a tea.Cmd for the next game frame.
func Tick(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return TickMsg{}
	})
}

// Registry holds constructors for all available mini-games.
var Registry = []func() MiniGame{
	NewRoverRunner,
}

// RandomMiniGame selects and creates a new instance of a mini-game.
func RandomMiniGame() MiniGame {
	if len(Registry) == 0 {
		return NewRoverRunner()
	}
	idx := rand.Intn(len(Registry))
	return Registry[idx]()
}
