package game

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	styleScore = lipgloss.NewStyle().Foreground(lipgloss.Color("#60a5fa")).Bold(true)
	styleTrack = lipgloss.NewStyle().Foreground(lipgloss.Color("#38bdf8"))
	styleCrash = lipgloss.NewStyle().Foreground(lipgloss.Color("#facc15")).Bold(true)
	styleHelp  = lipgloss.NewStyle().Foreground(lipgloss.Color("#64748b"))
)

// Obstacle represents a hazard on the ground track.
type Obstacle struct {
	X      int
	Sprite string
}

// RoverRunner is a 1-button side-scrolling jump runner.
type RoverRunner struct {
	score      int
	highScore  int
	playerX    int
	playerY    int // 0 = ground, 1 = low, 2 = mid, 3 = high
	jumpIdx    int // -1 when on ground
	obstacles  []Obstacle
	spawnTimer int
	gameOver   bool
	tickCount  int
	width      int
}

// NewRoverRunner creates a fresh Rover Runner game instance.
func NewRoverRunner() MiniGame {
	return &RoverRunner{
		playerX:    5,
		playerY:    0,
		jumpIdx:    -1,
		spawnTimer: 18,
		width:      44,
	}
}

func (r *RoverRunner) Title() string {
	return "Rover Runner 🏎 💨"
}

func (r *RoverRunner) Init() tea.Cmd {
	return Tick(70 * time.Millisecond)
}

func (r *RoverRunner) IsGameOver() bool {
	return r.gameOver
}

func (r *RoverRunner) Score() int {
	return r.score
}

var jumpArc = []int{1, 2, 2, 3, 3, 3, 3, 2, 2, 1, 0}

// Obstacles have a uniform visual width of 3 columns to avoid terminal jitter.
var obstacleSprites = []string{"🌵 ", "👾 ", "⚡ ", "🐛 ", "▲  "}

func (r *RoverRunner) Update(msg tea.Msg) (MiniGame, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case " ", "up", "w", "k":
			if r.gameOver {
				// Restart game preserving high score
				best := r.highScore
				fresh := NewRoverRunner().(*RoverRunner)
				fresh.highScore = best
				return fresh, fresh.Init()
			}
			if r.jumpIdx == -1 {
				r.jumpIdx = 0
				r.playerY = jumpArc[0]
			}
		}

	case TickMsg:
		if r.gameOver {
			return r, nil
		}

		r.tickCount++
		r.score++
		if r.score > r.highScore {
			r.highScore = r.score
		}

		// Progress jump
		if r.jumpIdx >= 0 {
			r.jumpIdx++
			if r.jumpIdx < len(jumpArc) {
				r.playerY = jumpArc[r.jumpIdx]
			} else {
				r.jumpIdx = -1
				r.playerY = 0
			}
		}

		// Advance obstacles leftward smoothly by 1 column
		var remaining []Obstacle
		for _, obs := range r.obstacles {
			obs.X--
			if obs.X >= 0 {
				remaining = append(remaining, obs)
			}
		}
		r.obstacles = remaining

		// Spawn new obstacle
		r.spawnTimer--
		if r.spawnTimer <= 0 {
			s := obstacleSprites[rand.Intn(len(obstacleSprites))]
			r.obstacles = append(r.obstacles, Obstacle{
				X:      r.width - 3,
				Sprite: s,
			})
			// Generous interval between 22 and 42 ticks for enjoyable gameplay
			r.spawnTimer = 22 + rand.Intn(20)
		}

		// Collision detection
		if r.playerY == 0 {
			for _, obs := range r.obstacles {
				if obs.X >= r.playerX-1 && obs.X <= r.playerX+1 {
					r.gameOver = true
					return r, nil
				}
			}
		}

		return r, Tick(70 * time.Millisecond)
	}

	return r, nil
}

// renderGameRow renders a single horizontal line of the canvas with exact visual column tracking.
func renderGameRow(w int, playerCol int, playerSprite string, obstacles []Obstacle) string {
	var sb strings.Builder
	col := 0
	for col < w {
		if playerSprite != "" && col == playerCol {
			sb.WriteString(playerSprite)
			pw := lipgloss.Width(playerSprite)
			if pw <= 0 {
				pw = 1
			}
			col += pw
			continue
		}

		obsFound := false
		for _, obs := range obstacles {
			if obs.X == col {
				sb.WriteString(obs.Sprite)
				ow := lipgloss.Width(obs.Sprite)
				if ow <= 0 {
					ow = 1
				}
				col += ow
				obsFound = true
				break
			}
		}
		if obsFound {
			continue
		}

		sb.WriteByte(' ')
		col++
	}
	return sb.String()
}

func (r *RoverRunner) View(targetWidth int) string {
	w := r.width
	if targetWidth > 15 && targetWidth < w {
		w = targetWidth
	}

	// Constant visual width for all player states: exact width 3 columns.
	var playerSprite string
	if r.gameOver {
		playerSprite = "💥 "
	} else if r.playerY > 0 {
		playerSprite = "🚀 "
	} else {
		playerSprite = "🏎  "
	}

	var sb strings.Builder

	// Header / Score
	scoreText := fmt.Sprintf("  SCORE: %05d   BEST: %05d", r.score, r.highScore)
	sb.WriteString(styleScore.Render(scoreText))
	sb.WriteString("\n\n")

	// 4 vertical canvas lines:
	// row 0: sky (playerY = 3)
	// row 1: upper air (playerY = 2)
	// row 2: lower air (playerY = 1)
	// row 3: ground (playerY = 0)
	for row := 0; row < 4; row++ {
		sb.WriteString("  ")
		curPlayerSprite := ""
		pRow := 3 - r.playerY
		if pRow == row {
			curPlayerSprite = playerSprite
		}

		var rowObstacles []Obstacle
		if row == 3 {
			rowObstacles = r.obstacles
		}

		lineStr := renderGameRow(w, r.playerX, curPlayerSprite, rowObstacles)
		sb.WriteString(lineStr)
		sb.WriteByte('\n')
	}

	// Ground surface track
	sb.WriteString("  ")
	sb.WriteString(styleTrack.Render(strings.Repeat("═", w)))
	sb.WriteString("\n\n")

	// Controls / Status
	if r.gameOver {
		sb.WriteString(styleCrash.Render("  💥 CRASH!  Press Space to Restart · Esc to Exit"))
	} else {
		sb.WriteString(styleHelp.Render("  Space: Jump · Esc: Back to logs"))
	}

	return sb.String()
}
