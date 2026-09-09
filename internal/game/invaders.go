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
	styleInvaderTitle = lipgloss.NewStyle().Foreground(lipgloss.Color("#a78bfa")).Bold(true) // Violet
	styleShip         = lipgloss.NewStyle().Foreground(lipgloss.Color("#38bdf8")).Bold(true) // Cyan
	styleLaser        = lipgloss.NewStyle().Foreground(lipgloss.Color("#facc15")).Bold(true) // Yellow
	styleBomb         = lipgloss.NewStyle().Foreground(lipgloss.Color("#f87171")).Bold(true) // Red
	styleShield       = lipgloss.NewStyle().Foreground(lipgloss.Color("#4ade80")).Bold(true) // Green
	stylePerimeter    = lipgloss.NewStyle().Foreground(lipgloss.Color("#38bdf8"))             // Cyan
)

type invaderUnit struct {
	X      int
	Y      int
	Sprite string
	Points int
	Alive  bool
}

type projectile struct {
	X int
	Y int
}

// PacketDefender is a Space-Invaders style arcade defender game.
type PacketDefender struct {
	width        int
	height       int
	playerX      int
	playerY      int
	shields      int
	score        int
	highScore    int
	wave         int
	invaders     []invaderUnit
	invaderDir   int // +1 (right) or -1 (left)
	marchTimer   int
	marchSpeed   int
	lasers       []projectile
	bombs        []projectile
	bombTimer    int
	gameOver     bool
	gameOverDesc string
	tickCount    int
}

// NewPacketDefender creates a fresh Packet Defender game instance.
func NewPacketDefender() MiniGame {
	g := &PacketDefender{
		width:   56,
		height:  17,
		shields: 3,
		wave:    1,
	}
	g.reset()
	return g
}

func (p *PacketDefender) reset() {
	p.playerX = p.width/2 - 1
	p.playerY = p.height - 1
	p.shields = 3
	p.score = 0
	p.wave = 1
	p.lasers = nil
	p.bombs = nil
	p.gameOver = false
	p.gameOverDesc = ""
	p.spawnFormation()
}

func (p *PacketDefender) spawnFormation() {
	p.invaders = nil
	p.invaderDir = 1
	p.marchSpeed = 7 - p.wave
	if p.marchSpeed < 3 {
		p.marchSpeed = 3
	}
	p.marchTimer = p.marchSpeed
	p.bombTimer = 15

	// 3 rows x 6 invaders = 18 invaders
	rows := []struct {
		sprite string
		points int
	}{
		{"👾", 30},
		{"🐛", 20},
		{"⚡", 10},
	}

	startX := 4
	spacingX := 8
	for rIdx, r := range rows {
		for c := 0; c < 6; c++ {
			p.invaders = append(p.invaders, invaderUnit{
				X:      startX + c*spacingX,
				Y:      rIdx + 1,
				Sprite: r.sprite,
				Points: r.points,
				Alive:  true,
			})
		}
	}
}

func (p *PacketDefender) Title() string {
	return "Packet Defender 👾 ◄▲►"
}

func (p *PacketDefender) Init() tea.Cmd {
	return Tick(50 * time.Millisecond)
}

func (p *PacketDefender) IsGameOver() bool {
	return p.gameOver
}

func (p *PacketDefender) Score() int {
	return p.score
}

func (p *PacketDefender) Dimensions() (int, int) {
	return 64, 25
}

func (p *PacketDefender) Update(msg tea.Msg) (MiniGame, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "left", "a", "h":
			if !p.gameOver && p.playerX > 1 {
				p.playerX -= 2
			}
		case "right", "d", "l":
			if !p.gameOver && p.playerX < p.width-4 {
				p.playerX += 2
			}
		case " ", "up", "w", "k":
			if p.gameOver {
				best := p.highScore
				p.reset()
				p.highScore = best
				return p, p.Init()
			}
			// Fire laser (up to 3 concurrent lasers)
			if len(p.lasers) < 3 {
				p.lasers = append(p.lasers, projectile{
					X: p.playerX + 1,
					Y: p.playerY - 1,
				})
			}
		}

	case TickMsg:
		if p.gameOver {
			return p, nil
		}

		p.tickCount++

		// 1. Advance player lasers upward
		var activeLasers []projectile
		for _, l := range p.lasers {
			l.Y--
			if l.Y >= 0 {
				activeLasers = append(activeLasers, l)
			}
		}
		p.lasers = activeLasers

		// 2. Advance invader bombs downward
		var activeBombs []projectile
		for _, b := range p.bombs {
			b.Y++
			if b.Y < p.height {
				// Hit player check (ship is 3 columns wide: playerX .. playerX+2)
				if b.Y == p.playerY && b.X >= p.playerX && b.X <= p.playerX+2 {
					p.shields--
					if p.shields <= 0 {
						p.gameOver = true
						p.gameOverDesc = "💥 DEFENDER DESTROYED"
						return p, nil
					}
					continue // bomb consumed
				}
				activeBombs = append(activeBombs, b)
			}
		}
		p.bombs = activeBombs

		// 3. Laser hits invader check
		aliveCount := 0
		for i := range p.invaders {
			if !p.invaders[i].Alive {
				continue
			}
			aliveCount++

			inv := &p.invaders[i]
			for lIdx, l := range p.lasers {
				// Invader sprites are 2 columns wide
				if l.Y == inv.Y && (l.X == inv.X || l.X == inv.X+1) {
					inv.Alive = false
					p.score += inv.Points
					if p.score > p.highScore {
						p.highScore = p.score
					}
					// Remove laser
					p.lasers = append(p.lasers[:lIdx], p.lasers[lIdx+1:]...)
					aliveCount--
					break
				}
			}
		}

		// Wave cleared check
		if aliveCount == 0 {
			p.score += 200
			if p.score > p.highScore {
				p.highScore = p.score
			}
			p.wave++
			p.spawnFormation()
			return p, Tick(50 * time.Millisecond)
		}

		// 4. Invader march movement
		p.marchTimer--
		// Dynamic march speed: the fewer invaders remain, the faster they move
		speedBonus := (18 - aliveCount) / 4
		targetMarchSpeed := p.marchSpeed - speedBonus
		if targetMarchSpeed < 1 {
			targetMarchSpeed = 1
		}

		if p.marchTimer <= 0 {
			p.marchTimer = targetMarchSpeed

			// Check if formation hit left or right wall
			shouldDrop := false
			for _, inv := range p.invaders {
				if !inv.Alive {
					continue
				}
				if (p.invaderDir > 0 && inv.X >= p.width-3) || (p.invaderDir < 0 && inv.X <= 1) {
					shouldDrop = true
					break
				}
			}

			if shouldDrop {
				p.invaderDir = -p.invaderDir
				for i := range p.invaders {
					if p.invaders[i].Alive {
						p.invaders[i].Y++
						// Ground breach check
						if p.invaders[i].Y >= p.height-1 {
							p.gameOver = true
							p.gameOverDesc = "💥 SYSTEM OVERRUN! PACKETS BREACHED GROUND"
							return p, nil
						}
					}
				}
			} else {
				for i := range p.invaders {
					if p.invaders[i].Alive {
						p.invaders[i].X += p.invaderDir
					}
				}
			}
		}

		// 5. Invaders drop bombs
		p.bombTimer--
		if p.bombTimer <= 0 {
			p.bombTimer = 18 - (p.wave * 2)
			if p.bombTimer < 8 {
				p.bombTimer = 8
			}

			// Pick a random alive invader to drop a spark bomb
			var shooters []invaderUnit
			for _, inv := range p.invaders {
				if inv.Alive {
					shooters = append(shooters, inv)
				}
			}
			if len(shooters) > 0 {
				shooter := shooters[rand.Intn(len(shooters))]
				p.bombs = append(p.bombs, projectile{
					X: shooter.X + 1,
					Y: shooter.Y + 1,
				})
			}
		}

		return p, Tick(50 * time.Millisecond)
	}

	return p, nil
}

// renderDefenderRow renders a single horizontal line of the airspace with exact visual column tracking.
func (p *PacketDefender) renderDefenderRow(w int, row int) string {
	var sb strings.Builder
	col := 0

	for col < w {
		// 1. Check player ship on this row
		if row == p.playerY && col == p.playerX {
			var shipStr string
			if p.gameOver {
				shipStr = "💥 "
			} else {
				shipStr = styleShip.Render("◄▲►")
			}
			sb.WriteString(shipStr)
			sw := lipgloss.Width(shipStr)
			if sw <= 0 {
				sw = 3
			}
			col += sw
			continue
		}

		// 2. Check laser on this row
		laserFound := false
		for _, l := range p.lasers {
			if l.Y == row && l.X == col {
				sb.WriteString(styleLaser.Render("│"))
				col++
				laserFound = true
				break
			}
		}
		if laserFound {
			continue
		}

		// 3. Check bomb on this row
		bombFound := false
		for _, b := range p.bombs {
			if b.Y == row && b.X == col {
				sb.WriteString(styleBomb.Render("!"))
				col++
				bombFound = true
				break
			}
		}
		if bombFound {
			continue
		}

		// 4. Check invader on this row
		invFound := false
		for _, inv := range p.invaders {
			if inv.Alive && inv.Y == row && inv.X == col {
				sb.WriteString(inv.Sprite)
				iw := lipgloss.Width(inv.Sprite)
				if iw <= 0 {
					iw = 2
				}
				col += iw
				invFound = true
				break
			}
		}
		if invFound {
			continue
		}

		// Empty space
		sb.WriteByte(' ')
		col++
	}

	return sb.String()
}

func (p *PacketDefender) View(targetWidth int) string {
	w := p.width
	if targetWidth > 20 && targetWidth-4 < w {
		w = targetWidth - 4
	}

	var sb strings.Builder

	// Header: Score, High Score, Wave, Shields
	shieldsStr := strings.Repeat("■ ", p.shields) + strings.Repeat("□ ", 3-p.shields)
	scoreLine := fmt.Sprintf("  SCORE: %05d   BEST: %05d   WAVE: %d   SHIELDS: %s",
		p.score, p.highScore, p.wave, styleShield.Render(shieldsStr))
	sb.WriteString(styleInvaderTitle.Render(scoreLine))
	sb.WriteString("\n\n")

	// Airspace canvas rows (height = 12)
	for row := 0; row < p.height; row++ {
		sb.WriteString("  ")
		sb.WriteString(p.renderDefenderRow(w, row))
		sb.WriteByte('\n')
	}

	// Ground defense perimeter line
	sb.WriteString("  ")
	sb.WriteString(stylePerimeter.Render(strings.Repeat("═", w)))
	sb.WriteString("\n\n")

	// Status / Controls
	if p.gameOver {
		banner := p.gameOverDesc
		if banner == "" {
			banner = "💥 PORT BREACHED!"
		}
		sb.WriteString(styleBomb.Render("  " + banner + "  Press Space to Restart · Esc to Exit"))
	} else {
		sb.WriteString(styleHelp.Render("  Arrows / A / D: Move · Space: Fire Cannon · Esc: Back to logs"))
	}

	return sb.String()
}
