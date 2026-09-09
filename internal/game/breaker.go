package game

import (
	"fmt"
	"math"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	styleBreakerTitle = lipgloss.NewStyle().Foreground(lipgloss.Color("#38bdf8")).Bold(true) // Cyan
	stylePaddle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#60a5fa")).Bold(true) // Royal Blue
	styleBall         = lipgloss.NewStyle().Foreground(lipgloss.Color("#facc15")).Bold(true) // Yellow
	styleBrickErr     = lipgloss.NewStyle().Foreground(lipgloss.Color("#f87171")).Bold(true) // Red
	styleBrickWrn     = lipgloss.NewStyle().Foreground(lipgloss.Color("#facc15")).Bold(true) // Yellow
	styleBrickSyn     = lipgloss.NewStyle().Foreground(lipgloss.Color("#4ade80")).Bold(true) // Green
	styleBrickDat     = lipgloss.NewStyle().Foreground(lipgloss.Color("#38bdf8")).Bold(true) // Cyan
	styleFloorTrack   = lipgloss.NewStyle().Foreground(lipgloss.Color("#475569"))             // Dark Slate
)

type brickUnit struct {
	X      int
	Y      int
	Width  int
	Label  string
	Points int
	Style  lipgloss.Style
	Alive  bool
}

// BitBreaker is an arcade breakout brick-breaking mini-game.
type BitBreaker struct {
	width     int
	height    int
	paddleX   int
	paddleY   int
	paddleW   int
	ballX     float64
	ballY     float64
	ballDX    float64
	ballDY    float64
	launched  bool
	lives     int
	score     int
	highScore   int
	wave        int
	bricks      []brickUnit
	bricksAlive int
	gameOver    bool
}

// NewBitBreaker creates a fresh Bit Breaker game instance.
func NewBitBreaker() MiniGame {
	g := &BitBreaker{
		width:   52,
		height:  17,
		paddleW: 8,
		lives:   3,
		wave:    1,
	}
	g.reset()
	return g
}

func (b *BitBreaker) reset() {
	b.paddleX = (b.width - b.paddleW) / 2
	b.paddleY = b.height - 1
	b.lives = 3
	b.score = 0
	b.wave = 1
	b.gameOver = false
	b.resetBall()
	b.spawnBricks()
}

func (b *BitBreaker) resetBall() {
	b.launched = false
	b.ballX = float64(b.paddleX + b.paddleW/2)
	b.ballY = float64(b.paddleY - 1)
	b.ballDX = 0.5
	b.ballDY = -0.6
}

func (b *BitBreaker) spawnBricks() {
	b.bricks = nil

	// 4 rows x 8 bricks = 32 bricks
	brickDefs := []struct {
		label  string
		points int
		style  lipgloss.Style
	}{
		{"[ERR]", 40, styleBrickErr},
		{"[WRN]", 30, styleBrickWrn},
		{"[SYN]", 20, styleBrickSyn},
		{"[DAT]", 10, styleBrickDat},
	}

	startX := 2
	brickW := 5
	spacing := 6 // 5 chars + 1 space

	for rIdx, def := range brickDefs {
		for c := 0; c < 8; c++ {
			b.bricks = append(b.bricks, brickUnit{
				X:      startX + c*spacing,
				Y:      rIdx + 1,
				Width:  brickW,
				Label:  def.label,
				Points: def.points,
				Style:  def.style,
				Alive:  true,
			})
		}
	}
	b.bricksAlive = len(b.bricks)
}

func (b *BitBreaker) Title() string {
	return "Bit Breaker 🧱 ⚡"
}

func (b *BitBreaker) Init() tea.Cmd {
	return Tick(35 * time.Millisecond)
}

func (b *BitBreaker) IsGameOver() bool {
	return b.gameOver
}

func (b *BitBreaker) Score() int {
	return b.score
}

func (b *BitBreaker) Dimensions() (int, int) {
	return 60, 25
}

func (b *BitBreaker) Update(msg tea.Msg) (MiniGame, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "left", "a", "h":
			if !b.gameOver {
				b.paddleX -= 3
				if b.paddleX < 0 {
					b.paddleX = 0
				}
				if !b.launched {
					b.ballX = float64(b.paddleX + b.paddleW/2)
				}
			}
		case "right", "d", "l":
			if !b.gameOver {
				b.paddleX += 3
				if b.paddleX > b.width-b.paddleW {
					b.paddleX = b.width - b.paddleW
				}
				if !b.launched {
					b.ballX = float64(b.paddleX + b.paddleW/2)
				}
			}
		case " ", "up", "w", "k", "enter":
			if b.gameOver {
				best := b.highScore
				b.reset()
				b.highScore = best
				return b, b.Init()
			}
			if !b.launched {
				b.launched = true
			}
		}

	case TickMsg:
		if b.gameOver {
			return b, nil
		}

		if !b.launched {
			// Keep ball stuck to paddle until launched
			b.ballX = float64(b.paddleX + b.paddleW/2)
			b.ballY = float64(b.paddleY - 1)
			return b, Tick(35 * time.Millisecond)
		}

		// Advance ball
		b.ballX += b.ballDX
		b.ballY += b.ballDY

		// Wall bounces
		if b.ballX <= 0 {
			b.ballX = 0
			b.ballDX = math.Abs(b.ballDX)
		} else if b.ballX >= float64(b.width-1) {
			b.ballX = float64(b.width - 1)
			b.ballDX = -math.Abs(b.ballDX)
		}

		if b.ballY <= 0 {
			b.ballY = 0
			b.ballDY = math.Abs(b.ballDY)
		}

		// Paddle collision check
		if int(b.ballY) == b.paddleY-1 && b.ballDY > 0 {
			bx := int(b.ballX)
			if bx >= b.paddleX && bx < b.paddleX+b.paddleW {
				// Paddle angle deflection
				center := float64(b.paddleX) + float64(b.paddleW)/2.0
				offset := (b.ballX - center) / (float64(b.paddleW) / 2.0)
				if offset < -1.0 {
					offset = -1.0
				} else if offset > 1.0 {
					offset = 1.0
				}
				b.ballDX = offset * 0.8
				b.ballDY = -0.6
			}
		}

		// Brick collision check
		bx := int(b.ballX)
		by := int(b.ballY)

		for i := range b.bricks {
			br := &b.bricks[i]
			if !br.Alive {
				continue
			}

			if by == br.Y && bx >= br.X && bx < br.X+br.Width {
				br.Alive = false
				b.bricksAlive--
				b.ballDY = -b.ballDY
				b.score += br.Points
				if b.score > b.highScore {
					b.highScore = b.score
				}
				break
			}
		}

		// Wave cleared check
		if b.bricksAlive == 0 {
			b.score += 300
			if b.score > b.highScore {
				b.highScore = b.score
			}
			b.wave++
			b.spawnBricks()
			b.resetBall()
			return b, Tick(35 * time.Millisecond)
		}

		// Ball fell below floor
		if b.ballY >= float64(b.height) {
			b.lives--
			if b.lives <= 0 {
				b.gameOver = true
				return b, nil
			}
			b.resetBall()
		}

		return b, Tick(35 * time.Millisecond)
	}

	return b, nil
}

// renderBreakerRow renders a single line of the breakout field with exact column tracking.
func (b *BitBreaker) renderBreakerRow(w int, row int) string {
	var sb strings.Builder
	col := 0

	for col < w {
		// 1. Paddle on bottom row
		if row == b.paddleY && col == b.paddleX {
			paddleStr := stylePaddle.Render("[══════]")
			sb.WriteString(paddleStr)
			col += b.paddleW
			continue
		}

		// 2. Ball
		if int(b.ballY) == row && int(b.ballX) == col {
			sb.WriteString(styleBall.Render("●"))
			col++
			continue
		}

		// 3. Bricks
		brickFound := false
		for _, br := range b.bricks {
			if br.Alive && br.Y == row && br.X == col {
				sb.WriteString(br.Style.Render(br.Label))
				col += br.Width
				brickFound = true
				break
			}
		}
		if brickFound {
			continue
		}

		// Empty space
		sb.WriteByte(' ')
		col++
	}

	return sb.String()
}

func (b *BitBreaker) View(targetWidth int) string {
	w := b.width
	if targetWidth > 20 && targetWidth-4 < w {
		w = targetWidth - 4
	}

	var sb strings.Builder

	// Header: Score, High Score, Wave, Lives
	livesStr := strings.Repeat("● ", b.lives) + strings.Repeat("○ ", 3-b.lives)
	scoreLine := fmt.Sprintf("  SCORE: %05d   BEST: %05d   WAVE: %d   LIVES: %s",
		b.score, b.highScore, b.wave, styleBall.Render(livesStr))
	sb.WriteString(styleBreakerTitle.Render(scoreLine))
	sb.WriteString("\n\n")

	// Field rows
	for row := 0; row < b.height; row++ {
		sb.WriteString("  ")
		sb.WriteString(b.renderBreakerRow(w, row))
		sb.WriteByte('\n')
	}

	// Floor baseline
	sb.WriteString("  ")
	sb.WriteString(styleFloorTrack.Render(strings.Repeat("═", w)))
	sb.WriteString("\n\n")

	// Status / Controls
	if b.gameOver {
		sb.WriteString(styleBrickErr.Render("  💥 ALL PACKETS DROPPED! Press Space to Restart · Esc to Exit"))
	} else if !b.launched {
		sb.WriteString(styleBrickWrn.Render("  Press Space or Up to Launch Ball · Left/Right to Move"))
	} else {
		sb.WriteString(styleHelp.Render("  Arrows / A / D: Move Paddle · Esc: Back to logs"))
	}

	return sb.String()
}
