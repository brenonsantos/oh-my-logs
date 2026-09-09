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
	styleTetrisTitle = lipgloss.NewStyle().Foreground(lipgloss.Color("#c084fc")).Bold(true) // Purple
	styleWellBorder  = lipgloss.NewStyle().Foreground(lipgloss.Color("#475569"))             // Slate
	styleGhost       = lipgloss.NewStyle().Foreground(lipgloss.Color("#334155"))             // Dimmed
	stylePanelLabel  = lipgloss.NewStyle().Foreground(lipgloss.Color("#94a3b8"))             // Muted Slate
	stylePanelValue  = lipgloss.NewStyle().Foreground(lipgloss.Color("#f1f5f9")).Bold(true) // Crisp White
	styleHighlight   = lipgloss.NewStyle().Foreground(lipgloss.Color("#facc15")).Bold(true) // Yellow
)

var pieceTemplates = [][][]int{
	// 1: I (Cyan)
	{
		{0, 0, 0, 0},
		{1, 1, 1, 1},
		{0, 0, 0, 0},
		{0, 0, 0, 0},
	},
	// 2: O (Yellow)
	{
		{2, 2},
		{2, 2},
	},
	// 3: T (Purple)
	{
		{0, 3, 0},
		{3, 3, 3},
		{0, 0, 0},
	},
	// 4: S (Green)
	{
		{0, 4, 4},
		{4, 4, 0},
		{0, 0, 0},
	},
	// 5: Z (Red)
	{
		{5, 5, 0},
		{0, 5, 5},
		{0, 0, 0},
	},
	// 6: J (Blue)
	{
		{6, 0, 0},
		{6, 6, 6},
		{0, 0, 0},
	},
	// 7: L (Orange)
	{
		{0, 0, 7},
		{7, 7, 7},
		{0, 0, 0},
	},
}

var pieceStyles = []lipgloss.Style{
	lipgloss.NewStyle(),                                      // 0: empty
	lipgloss.NewStyle().Foreground(lipgloss.Color("#38bdf8")), // 1: I (Cyan)
	lipgloss.NewStyle().Foreground(lipgloss.Color("#facc15")), // 2: O (Yellow)
	lipgloss.NewStyle().Foreground(lipgloss.Color("#c084fc")), // 3: T (Purple)
	lipgloss.NewStyle().Foreground(lipgloss.Color("#4ade80")), // 4: S (Green)
	lipgloss.NewStyle().Foreground(lipgloss.Color("#f87171")), // 5: Z (Red)
	lipgloss.NewStyle().Foreground(lipgloss.Color("#60a5fa")), // 6: J (Blue)
	lipgloss.NewStyle().Foreground(lipgloss.Color("#fb923c")), // 7: L (Orange)
}

// activePiece tracks the position and rotation of the falling tetromino.
type activePiece struct {
	matrix [][]int
	x      int
	y      int
}

// BufferStack is a classic Tetris game themed around buffer overflow stack frames.
type BufferStack struct {
	wellW       int
	wellH       int
	grid        [18][10]int
	current     activePiece
	nextPiece   int // 0 to 6
	score       int
	highScore   int
	lines       int
	level       int
	gameOver    bool
	dropSpeed   time.Duration
	lastClear   int
	lockDelay   int
}

// NewBufferStack creates a fresh Buffer Stack (Tetris) game instance.
func NewBufferStack() MiniGame {
	g := &BufferStack{
		wellW:     10,
		wellH:     18,
		dropSpeed: 750 * time.Millisecond,
		level:     1,
	}
	g.reset()
	return g
}

func (b *BufferStack) reset() {
	b.grid = [18][10]int{}
	b.score = 0
	b.lines = 0
	b.level = 1
	b.gameOver = false
	b.dropSpeed = 750 * time.Millisecond
	b.lockDelay = 0
	b.nextPiece = rand.Intn(len(pieceTemplates))
	b.spawnPiece()
}

func copyMatrix(m [][]int) [][]int {
	out := make([][]int, len(m))
	for i := range m {
		out[i] = make([]int, len(m[i]))
		copy(out[i], m[i])
	}
	return out
}

func (b *BufferStack) spawnPiece() {
	b.lockDelay = 0
	idx := b.nextPiece
	b.nextPiece = rand.Intn(len(pieceTemplates))

	m := copyMatrix(pieceTemplates[idx])
	p := activePiece{
		matrix: m,
		x:      b.wellW/2 - len(m)/2,
		y:      0,
	}

	// If initial spawn doesn't fit, game over (top-out)
	if !b.canFit(p.matrix, p.x, p.y) {
		b.gameOver = true
		b.current = p
		return
	}

	b.current = p
}

func (b *BufferStack) canFit(m [][]int, px, py int) bool {
	for r := 0; r < len(m); r++ {
		for c := 0; c < len(m[r]); c++ {
			if m[r][c] == 0 {
				continue
			}
			wx := px + c
			wy := py + r

			if wx < 0 || wx >= b.wellW || wy >= b.wellH {
				return false
			}
			if wy >= 0 && b.grid[wy][wx] != 0 {
				return false
			}
		}
	}
	return true
}

func rotateMatrix(m [][]int) [][]int {
	n := len(m)
	out := make([][]int, n)
	for i := range out {
		out[i] = make([]int, n)
		for j := range out[i] {
			out[i][j] = m[n-1-j][i]
		}
	}
	return out
}

func (b *BufferStack) rotateCurrent() {
	rotated := rotateMatrix(b.current.matrix)
	// Try standard position and wall-kicks (-1, +1, -2, +2)
	kicks := []int{0, -1, 1, -2, 2}
	for _, kx := range kicks {
		if b.canFit(rotated, b.current.x+kx, b.current.y) {
			b.current.matrix = rotated
			b.current.x += kx
			return
		}
	}
}

func (b *BufferStack) getGhostY() int {
	gy := b.current.y
	for b.canFit(b.current.matrix, b.current.x, gy+1) {
		gy++
	}
	return gy
}

func (b *BufferStack) lockPiece() {
	for r := 0; r < len(b.current.matrix); r++ {
		for c := 0; c < len(b.current.matrix[r]); c++ {
			val := b.current.matrix[r][c]
			if val == 0 {
				continue
			}
			wy := b.current.y + r
			wx := b.current.x + c
			if wy >= 0 && wy < b.wellH && wx >= 0 && wx < b.wellW {
				b.grid[wy][wx] = val
			}
		}
	}

	b.clearLines()
	b.spawnPiece()
}

func (b *BufferStack) clearLines() {
	cleared := 0
	for y := b.wellH - 1; y >= 0; y-- {
		full := true
		for x := 0; x < b.wellW; x++ {
			if b.grid[y][x] == 0 {
				full = false
				break
			}
		}

		if full {
			cleared++
			// Shift rows down
			for sy := y; sy > 0; sy-- {
				b.grid[sy] = b.grid[sy-1]
			}
			b.grid[0] = [10]int{}
			y++ // recheck same row
		}
	}

	if cleared > 0 {
		b.lastClear = cleared
		b.lines += cleared
		// Classic scoring multipliers
		points := 0
		switch cleared {
		case 1:
			points = 100 * b.level
		case 2:
			points = 300 * b.level
		case 3:
			points = 500 * b.level
		case 4:
			points = 800 * b.level
		default:
			points = 1000 * b.level
		}
		b.score += points
		if b.score > b.highScore {
			b.highScore = b.score
		}

		// Level up every 10 lines
		b.level = 1 + b.lines/10
		newSpeed := 750*time.Millisecond - time.Duration((b.level-1)*30)*time.Millisecond
		if newSpeed < 200*time.Millisecond {
			newSpeed = 200 * time.Millisecond
		}
		b.dropSpeed = newSpeed
	}
}

func (b *BufferStack) Title() string {
	return "Buffer Stack 🕹 [TETRIS]"
}

func (b *BufferStack) Init() tea.Cmd {
	return Tick(b.dropSpeed)
}

func (b *BufferStack) IsGameOver() bool {
	return b.gameOver
}

func (b *BufferStack) Score() int {
	return b.score
}

func (b *BufferStack) Dimensions() (int, int) {
	return 54, 25
}

func (b *BufferStack) Update(msg tea.Msg) (MiniGame, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "left", "a", "h":
			if !b.gameOver && b.canFit(b.current.matrix, b.current.x-1, b.current.y) {
				b.current.x--
				b.lockDelay = 0
			}
		case "right", "d", "l":
			if !b.gameOver && b.canFit(b.current.matrix, b.current.x+1, b.current.y) {
				b.current.x++
				b.lockDelay = 0
			}
		case "up", "w", "k":
			if !b.gameOver {
				b.rotateCurrent()
				b.lockDelay = 0
			}
		case "down", "s", "j":
			if !b.gameOver {
				if b.canFit(b.current.matrix, b.current.x, b.current.y+1) {
					b.current.y++
					b.score++
					b.lockDelay = 0
					if b.score > b.highScore {
						b.highScore = b.score
					}
				}
			}
		case " ":
			// Space = Hard drop
			if !b.gameOver {
				ghostY := b.getGhostY()
				dropDist := ghostY - b.current.y
				b.current.y = ghostY
				b.score += dropDist * 2
				if b.score > b.highScore {
					b.highScore = b.score
				}
				b.lockPiece()
				return b, Tick(b.dropSpeed)
			} else {
				// Restart on Space when game over
				best := b.highScore
				b.reset()
				b.highScore = best
				return b, b.Init()
			}
		case "enter":
			if b.gameOver {
				best := b.highScore
				b.reset()
				b.highScore = best
				return b, b.Init()
			}
		}

	case TickMsg:
		if b.gameOver {
			return b, nil
		}

		// Gravity step down
		if b.canFit(b.current.matrix, b.current.x, b.current.y+1) {
			b.current.y++
			b.lockDelay = 0
		} else {
			// Piece has reached bottom / stacked blocks: grant lock delay grace period
			b.lockDelay++
			if b.lockDelay >= 2 {
				b.lockPiece()
			}
		}

		return b, Tick(b.dropSpeed)
	}

	return b, nil
}

func (b *BufferStack) View(targetWidth int) string {
	var sb strings.Builder

	// Header
	header := fmt.Sprintf("  %s   SCORE: %06d   BEST: %06d",
		styleTetrisTitle.Render("BUFFER STACK"), b.score, b.highScore)
	sb.WriteString(header)
	sb.WriteString("\n\n")

	ghostY := b.getGhostY()

	// Side panel strings prepared per row
	panelRows := make([]string, b.wellH)
	panelRows[0] = stylePanelLabel.Render("NEXT FRAME:")
	// Render 4x2 preview of next piece
	nextM := pieceTemplates[b.nextPiece]
	for pr := 0; pr < 2; pr++ {
		var pLine strings.Builder
		for pc := 0; pc < 4; pc++ {
			val := 0
			if pr < len(nextM) && pc < len(nextM[pr]) {
				val = nextM[pr][pc]
			}
			if val > 0 {
				pLine.WriteString(pieceStyles[val].Render("■ "))
			} else {
				pLine.WriteString("  ")
			}
		}
		panelRows[1+pr] = "  " + pLine.String()
	}
	panelRows[3] = ""
	panelRows[4] = stylePanelLabel.Render("LEVEL:") + " " + stylePanelValue.Render(fmt.Sprintf("%d", b.level))
	panelRows[5] = stylePanelLabel.Render("LINES:") + " " + stylePanelValue.Render(fmt.Sprintf("%d", b.lines))
	panelRows[6] = ""
	if b.lastClear == 4 {
		panelRows[7] = styleHighlight.Render("★ TETRIS! ★")
	} else if b.lastClear > 0 {
		panelRows[7] = stylePanelValue.Render(fmt.Sprintf("+%d Lines!", b.lastClear))
	} else {
		panelRows[7] = ""
	}
	panelRows[8] = ""
	panelRows[9] = stylePanelLabel.Render("CONTROLS:")
	panelRows[10] = stylePanelLabel.Render("←/→/A/D: Move")
	panelRows[11] = stylePanelLabel.Render("↑/W/K:   Rotate")
	panelRows[12] = stylePanelLabel.Render("↓/S/J:   Soft Drop")
	panelRows[13] = stylePanelLabel.Render("Space:   Hard Drop")
	panelRows[14] = stylePanelLabel.Render("Esc:     Exit")

	// Render well + side panel row by row
	topBorder := "  " + styleWellBorder.Render("┌"+strings.Repeat("─", b.wellW*2)+"┐")
	sb.WriteString(topBorder)
	sb.WriteByte('\n')

	for y := 0; y < b.wellH; y++ {
		var rowSb strings.Builder
		rowSb.WriteString("  ")
		rowSb.WriteString(styleWellBorder.Render("│"))

		for x := 0; x < b.wellW; x++ {
			// 1. Check active piece
			activeVal := 0
			pr := y - b.current.y
			pc := x - b.current.x
			if pr >= 0 && pr < len(b.current.matrix) && pc >= 0 && pc < len(b.current.matrix[pr]) {
				activeVal = b.current.matrix[pr][pc]
			}

			// 2. Check ghost piece
			ghostVal := 0
			gr := y - ghostY
			gc := x - b.current.x
			if gr >= 0 && gr < len(b.current.matrix) && gc >= 0 && gc < len(b.current.matrix[gr]) {
				ghostVal = b.current.matrix[gr][gc]
			}

			// 3. Grid cell
			gridVal := b.grid[y][x]

			switch {
			case activeVal > 0:
				if b.gameOver {
					rowSb.WriteString(pieceStyles[5].Render("■ ")) // Red on game over
				} else {
					rowSb.WriteString(pieceStyles[activeVal].Render("■ "))
				}
			case gridVal > 0:
				if b.gameOver {
					rowSb.WriteString(pieceStyles[5].Render("■ "))
				} else {
					rowSb.WriteString(pieceStyles[gridVal].Render("■ "))
				}
			case ghostVal > 0 && !b.gameOver:
				rowSb.WriteString(styleGhost.Render("::"))
			default:
				rowSb.WriteString("  ")
			}
		}

		rowSb.WriteString(styleWellBorder.Render("│"))
		rowSb.WriteString("   ")
		if y < len(panelRows) {
			rowSb.WriteString(panelRows[y])
		}

		sb.WriteString(rowSb.String())
		sb.WriteByte('\n')
	}

	bottomBorder := "  " + styleWellBorder.Render("└"+strings.Repeat("─", b.wellW*2)+"┘")
	sb.WriteString(bottomBorder)
	sb.WriteString("\n\n")

	if b.gameOver {
		sb.WriteString(pieceStyles[5].Render("  💥 STACK OVERFLOW! Press Space to Restart · Esc to Exit"))
	} else {
		sb.WriteString(stylePanelLabel.Render("  Rotate: Up/W · Hard Drop: Space · Esc: Back to logs"))
	}

	return sb.String()
}
