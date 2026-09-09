package game

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	stylePongTitle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#38bdf8")).Bold(true) // Cyan
	stylePongPlayer = lipgloss.NewStyle().Foreground(lipgloss.Color("#4ade80")).Bold(true) // Green (TX)
	stylePongCPU    = lipgloss.NewStyle().Foreground(lipgloss.Color("#f43f5e")).Bold(true) // Rose (RX)
	stylePongBall   = lipgloss.NewStyle().Foreground(lipgloss.Color("#facc15")).Bold(true) // Yellow
	stylePongNet    = lipgloss.NewStyle().Foreground(lipgloss.Color("#334155"))            // Dark slate
	stylePongWall   = lipgloss.NewStyle().Foreground(lipgloss.Color("#475569"))            // Slate
	stylePongScore  = lipgloss.NewStyle().Foreground(lipgloss.Color("#e2e8f0")).Bold(true)
)

const (
	pongWidth     = 46
	pongHeight    = 14
	pongPaddleH   = 4
	pongLeftCol   = 2
	pongRightCol  = 43
	pongMaxScore  = 5
)

// Pong is a high-speed TX vs RX Ping-Pong Buffer duel.
type Pong struct {
	width      int
	height     int
	paddleH    int
	playerY    float64
	cpuY       float64
	ballX      float64
	ballY      float64
	ballVx     float64
	ballVy     float64
	playerPts  int
	cpuPts     int
	rally      int
	bestRally  int
	inServe    bool
	gameOver   bool
	winner     string // "TX" or "RX"
	tickMs     time.Duration
}

// NewPong creates a fresh Ping-Pong Buffer mini-game.
func NewPong() MiniGame {
	p := &Pong{
		width:   pongWidth,
		height:  pongHeight,
		paddleH: pongPaddleH,
		tickMs:  45 * time.Millisecond,
	}
	p.resetMatch()
	return p
}

func (p *Pong) resetMatch() {
	p.playerPts = 0
	p.cpuPts = 0
	p.rally = 0
	p.gameOver = false
	p.winner = ""
	p.resetServe(1.0)
}

func (p *Pong) resetServe(dirX float64) {
	p.playerY = float64(p.height-p.paddleH) / 2.0
	p.cpuY = float64(p.height-p.paddleH) / 2.0
	p.ballX = float64(p.width) / 2.0
	p.ballY = float64(p.height) / 2.0
	p.ballVx = 0.7 * dirX
	// Slight random vertical trajectory
	p.ballVy = (rand.Float64()*0.6 - 0.3)
	p.inServe = true
}

func (p *Pong) Title() string {
	return "Ping-Pong Buffer 🏓 [PONG]"
}

func (p *Pong) Init() tea.Cmd {
	return Tick(p.tickMs)
}

func (p *Pong) IsGameOver() bool {
	return p.gameOver
}

func (p *Pong) Score() int {
	return p.playerPts*100 + p.bestRally*10
}

func (p *Pong) Dimensions() (int, int) {
	return 60, 20
}

func (p *Pong) movePlayer(delta float64) {
	p.playerY += delta
	if p.playerY < 0 {
		p.playerY = 0
	}
	max := float64(p.height - p.paddleH)
	if p.playerY > max {
		p.playerY = max
	}
}

func (p *Pong) Update(msg tea.Msg) (MiniGame, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if p.gameOver {
			switch msg.String() {
			case " ", "enter":
				p.resetMatch()
				return p, Tick(p.tickMs)
			}
			return p, nil
		}

		if p.inServe {
			switch msg.String() {
			case " ", "enter":
				p.inServe = false
				return p, nil
			}
		}

		switch msg.String() {
		case "up", "w", "k":
			p.movePlayer(-1.5)
			return p, nil
		case "down", "s", "j":
			p.movePlayer(1.5)
			return p, nil
		}

	case TickMsg:
		if p.gameOver {
			return p, nil
		}

		if !p.inServe {
			// Update ball
			p.ballX += p.ballVx
			p.ballY += p.ballVy

			// Wall bounce (top/bottom)
			if p.ballY <= 0 {
				p.ballY = 0
				p.ballVy = math.Abs(p.ballVy)
			} else if p.ballY >= float64(p.height-1) {
				p.ballY = float64(p.height - 1)
				p.ballVy = -math.Abs(p.ballVy)
			}

			// Left paddle (Player) collision
			if p.ballX <= float64(pongLeftCol+1) && p.ballX >= float64(pongLeftCol-1) {
				if p.ballY >= p.playerY && p.ballY <= p.playerY+float64(p.paddleH) {
					p.ballX = float64(pongLeftCol + 1)
					p.ballVx = math.Abs(p.ballVx) * 1.03 // slight speedup
					// Deflection angle depending on impact relative to paddle center
					offset := (p.ballY - (p.playerY + float64(p.paddleH)/2.0))
					p.ballVy = offset * 0.35
					p.rally++
					if p.rally > p.bestRally {
						p.bestRally = p.rally
					}
				}
			}

			// Right paddle (CPU) collision
			if p.ballX >= float64(pongRightCol-1) && p.ballX <= float64(pongRightCol+1) {
				if p.ballY >= p.cpuY && p.ballY <= p.cpuY+float64(p.paddleH) {
					p.ballX = float64(pongRightCol - 1)
					p.ballVx = -math.Abs(p.ballVx) * 1.03
					offset := (p.ballY - (p.cpuY + float64(p.paddleH)/2.0))
					p.ballVy = offset * 0.35
					p.rally++
					if p.rally > p.bestRally {
						p.bestRally = p.rally
					}
				}
			}

			// CPU AI tracking
			cpuCenter := p.cpuY + float64(p.paddleH)/2.0
			cpuSpeed := 0.45
			if p.ballVx > 0 {
				// Ball coming towards CPU: track ball
				if p.ballY > cpuCenter+0.5 {
					p.cpuY += cpuSpeed
				} else if p.ballY < cpuCenter-0.5 {
					p.cpuY -= cpuSpeed
				}
			} else {
				// Ball moving away: return towards center
				fieldCenter := float64(p.height) / 2.0
				if cpuCenter < fieldCenter-1.0 {
					p.cpuY += cpuSpeed * 0.5
				} else if cpuCenter > fieldCenter+1.0 {
					p.cpuY -= cpuSpeed * 0.5
				}
			}
			// Clamp CPU paddle
			if p.cpuY < 0 {
				p.cpuY = 0
			}
			maxCpu := float64(p.height - p.paddleH)
			if p.cpuY > maxCpu {
				p.cpuY = maxCpu
			}

			// Scoring check
			if p.ballX < 0 {
				// CPU scores
				p.cpuPts++
				p.rally = 0
				if p.cpuPts >= pongMaxScore {
					p.gameOver = true
					p.winner = "RX"
				} else {
					p.resetServe(1.0) // serve towards player
				}
			} else if p.ballX >= float64(p.width) {
				// Player scores
				p.playerPts++
				p.rally = 0
				if p.playerPts >= pongMaxScore {
					p.gameOver = true
					p.winner = "TX"
				} else {
					p.resetServe(-1.0) // serve towards CPU
				}
			}
		}

		return p, Tick(p.tickMs)
	}

	return p, nil
}

func (p *Pong) View(targetWidth int) string {
	var sb strings.Builder

	boxW := p.width + 2 // 46 + 2 = 48
	padN := (targetWidth - boxW) / 2
	if padN < 2 {
		padN = 2
	}
	pad := strings.Repeat(" ", padN)

	// Header
	header := fmt.Sprintf("SCORE: %04d   TX: %d   RX: %d   RALLY: %d", p.Score(), p.playerPts, p.cpuPts, p.rally)
	sb.WriteString(pad + stylePongTitle.Render("PING-PONG BUFFER") + "\n")
	sb.WriteString(pad + stylePongScore.Render(header) + "\n\n")

	// Top border
	sb.WriteString(pad + stylePongWall.Render("┌"+strings.Repeat("─", p.width)+"┐") + "\n")

	// Field rows
	bx := int(math.Round(p.ballX))
	by := int(math.Round(p.ballY))
	midCol := p.width / 2

	for r := 0; r < p.height; r++ {
		sb.WriteString(pad + stylePongWall.Render("│"))

		for c := 0; c < p.width; c++ {
			// Ball
			if c == bx && r == by {
				sb.WriteString(stylePongBall.Render("●"))
				continue
			}

			// Left paddle (TX)
			if c == pongLeftCol && r >= int(p.playerY) && r < int(p.playerY)+p.paddleH {
				sb.WriteString(stylePongPlayer.Render("█"))
				continue
			}

			// Right paddle (RX)
			if c == pongRightCol && r >= int(p.cpuY) && r < int(p.cpuY)+p.paddleH {
				sb.WriteString(stylePongCPU.Render("█"))
				continue
			}

			// Center net
			if c == midCol {
				if r%2 == 0 {
					sb.WriteString(stylePongNet.Render("┆"))
				} else {
					sb.WriteByte(' ')
				}
				continue
			}

			sb.WriteByte(' ')
		}

		sb.WriteString(stylePongWall.Render("│") + "\n")
	}

	// Bottom border
	sb.WriteString(pad + stylePongWall.Render("└"+strings.Repeat("─", p.width)+"┘") + "\n\n")

	// Footer messages
	var status string
	if p.gameOver {
		if p.winner == "TX" {
			status = "🎉 TX BUFFER DOMINANCE! You Won! Press Space to Rematch"
		} else {
			status = "💥 RX OVERFLOW! CPU Won! Press Space to Rematch"
		}
	} else if p.inServe {
		status = "Press Space to Serve · Up / Down / W / S: Move Paddle"
	} else {
		status = "Up / Down / W / S: Move · Esc: Back to logs"
	}
	statusPadN := (targetWidth - lipgloss.Width(status)) / 2
	if statusPadN < 1 {
		statusPadN = 1
	}
	sb.WriteString(strings.Repeat(" ", statusPadN) + stylePongWall.Render(status))

	return sb.String()
}
