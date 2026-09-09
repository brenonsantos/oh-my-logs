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
	styleFlappyTitle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#38bdf8")).Bold(true) // Cyan
	styleFlappyPacket = lipgloss.NewStyle().Foreground(lipgloss.Color("#facc15")).Bold(true) // Yellow
	styleFlappyPipe   = lipgloss.NewStyle().Foreground(lipgloss.Color("#ef4444")).Bold(true) // Red
	styleFlappyWall   = lipgloss.NewStyle().Foreground(lipgloss.Color("#475569"))            // Slate
	styleFlappyScore  = lipgloss.NewStyle().Foreground(lipgloss.Color("#94a3b8"))            // Muted
	styleFlappyDead   = lipgloss.NewStyle().Foreground(lipgloss.Color("#f87171")).Bold(true) // Light red
)

type flappyPipe struct {
	x      float64
	gapY   int // row index where gap starts
	gapH   int // height of the gap
	passed bool
}

// FlappyPacket is a high-speed obstacle evasion mini-game.
type FlappyPacket struct {
	width     int
	height    int
	packetY   float64
	velY      float64
	gravity   float64
	jumpVel   float64
	pipes     []flappyPipe
	score     int
	highScore int
	gameOver  bool
	started   bool
	tickMs    time.Duration
}

const (
	flappyPacketX = 8
	flappyPipeW   = 4
)

// NewFlappyPacket creates a new Flappy Packet mini-game.
func NewFlappyPacket() MiniGame {
	g := &FlappyPacket{
		width:   44,
		height:  14,
		gravity: 0.28,
		jumpVel: -0.95,
		tickMs:  50 * time.Millisecond,
	}
	g.reset()
	return g
}

func (f *FlappyPacket) reset() {
	f.packetY = float64(f.height) / 2.0
	f.velY = 0
	f.score = 0
	f.gameOver = false
	f.started = false
	f.pipes = nil

	// Seed first 2 pipes
	f.spawnPipe(float64(f.width - 10))
	f.spawnPipe(float64(f.width + 12))
}

func (f *FlappyPacket) spawnPipe(x float64) {
	gapH := 5
	// Leave at least 2 rows top and bottom
	minGapY := 2
	maxGapY := f.height - gapH - 2
	gapY := minGapY
	if maxGapY > minGapY {
		gapY = minGapY + rand.Intn(maxGapY-minGapY+1)
	}

	f.pipes = append(f.pipes, flappyPipe{
		x:      x,
		gapY:   gapY,
		gapH:   gapH,
		passed: false,
	})
}

func (f *FlappyPacket) Title() string {
	return "Flappy Packet 🪽 [FLAP]"
}

func (f *FlappyPacket) Init() tea.Cmd {
	return Tick(f.tickMs)
}

func (f *FlappyPacket) IsGameOver() bool {
	return f.gameOver
}

func (f *FlappyPacket) Score() int {
	return f.score
}

func (f *FlappyPacket) Dimensions() (int, int) {
	return 58, 20
}

func (f *FlappyPacket) flap() {
	f.velY = f.jumpVel
	f.started = true
}

func (f *FlappyPacket) checkCollisions() bool {
	py := int(f.packetY)

	// Ground collision
	if py >= f.height {
		return true
	}

	// Ceiling clamp
	if py < 0 {
		f.packetY = 0
		f.velY = 0
		py = 0
	}

	// Pipe collision
	// Packet covers columns [flappyPacketX, flappyPacketX + 2]
	packetLeft := flappyPacketX
	packetRight := flappyPacketX + 2

	for _, p := range f.pipes {
		px1 := int(p.x)
		px2 := px1 + flappyPipeW - 1

		// Check horizontal overlap
		if packetRight >= px1 && packetLeft <= px2 {
			// In pipe column range: check if vertically inside gap
			if py < p.gapY || py >= p.gapY+p.gapH {
				return true
			}
		}
	}

	return false
}

func (f *FlappyPacket) Update(msg tea.Msg) (MiniGame, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if f.gameOver {
			switch msg.String() {
			case " ", "enter":
				best := f.highScore
				f.reset()
				f.highScore = best
				return f, Tick(f.tickMs)
			}
			return f, nil
		}

		switch msg.String() {
		case " ", "up", "w", "k":
			f.flap()
			return f, nil
		}

	case TickMsg:
		if f.gameOver {
			return f, nil
		}

		if f.started {
			// Apply gravity and update position
			f.velY += f.gravity
			f.packetY += f.velY

			// Move pipes left
			pipeSpeed := 0.45
			for i := range f.pipes {
				f.pipes[i].x -= pipeSpeed

				// Score when pipe clears packet
				if !f.pipes[i].passed && f.pipes[i].x+float64(flappyPipeW) < float64(flappyPacketX) {
					f.pipes[i].passed = true
					f.score++
					if f.score > f.highScore {
						f.highScore = f.score
					}
				}
			}

			// Prune off-screen pipes
			var activePipes []flappyPipe
			for _, p := range f.pipes {
				if p.x+float64(flappyPipeW) > 0 {
					activePipes = append(activePipes, p)
				}
			}
			f.pipes = activePipes

			// Spawn new pipe if needed
			if len(f.pipes) > 0 {
				lastPipe := f.pipes[len(f.pipes)-1]
				if float64(f.width)-lastPipe.x >= 22 {
					f.spawnPipe(float64(f.width + 2))
				}
			} else {
				f.spawnPipe(float64(f.width + 2))
			}

			// Collision check
			if f.checkCollisions() {
				f.gameOver = true
				return f, nil
			}
		}

		return f, Tick(f.tickMs)
	}

	return f, nil
}

func (f *FlappyPacket) View(targetWidth int) string {
	var sb strings.Builder

	boxW := f.width + 2 // 44 + 2 border chars = 46
	padN := (targetWidth - boxW) / 2
	if padN < 2 {
		padN = 2
	}
	pad := strings.Repeat(" ", padN)

	// Header
	header := fmt.Sprintf("SCORE: %03d   BEST: %03d", f.score, f.highScore)
	sb.WriteString(pad + styleFlappyTitle.Render("FLAPPY PACKET") + "\n")
	sb.WriteString(pad + styleFlappyScore.Render(header) + "\n\n")

	// Top border
	sb.WriteString(pad + styleFlappyWall.Render("┌"+strings.Repeat("─", f.width)+"┐") + "\n")

	// Grid rendering
	py := int(f.packetY)
	if py < 0 {
		py = 0
	}
	if py >= f.height {
		py = f.height - 1
	}

	for r := 0; r < f.height; r++ {
		rowRunes := make([]rune, f.width)
		for c := 0; c < f.width; c++ {
			rowRunes[c] = ' '
		}

		// Draw pipes
		for _, p := range f.pipes {
			px1 := int(p.x)
			for dx := 0; dx < flappyPipeW; dx++ {
				col := px1 + dx
				if col >= 0 && col < f.width {
					if r < p.gapY || r >= p.gapY+p.gapH {
						rowRunes[col] = '█'
					}
				}
			}
		}

		// Draw packet: [●] or [×] if game over
		isPacketRow := (r == py)
		sb.WriteString(pad + styleFlappyWall.Render("│"))

		// Construct line with exact width
		c := 0
		for c < f.width {
			if isPacketRow && c == flappyPacketX {
				if f.gameOver {
					sb.WriteString(styleFlappyDead.Render("[×]"))
				} else {
					sb.WriteString(styleFlappyPacket.Render("[●]"))
				}
				c += 3
				continue
			}

			ch := rowRunes[c]
			if ch == '█' {
				sb.WriteString(styleFlappyPipe.Render("█"))
			} else {
				sb.WriteByte(' ')
			}
			c++
		}

		sb.WriteString(styleFlappyWall.Render("│") + "\n")
	}

	// Bottom border (conduit floor)
	sb.WriteString(pad + styleFlappyWall.Render("└"+strings.Repeat("─", f.width)+"┘") + "\n\n")

	var status string
	if f.gameOver {
		status = "💥 PACKET DROPPED! Space to Restart · Esc to Exit"
	} else if !f.started {
		status = "Press Space / Up / W to Flap · Esc to Exit"
	} else {
		status = "Space / Up / W: Flap · Avoid Firewall Pipes!"
	}
	statusPadN := (targetWidth - lipgloss.Width(status)) / 2
	if statusPadN < 1 {
		statusPadN = 1
	}
	sb.WriteString(strings.Repeat(" ", statusPadN) + styleFlappyWall.Render(status))

	return sb.String()
}
