package game

import (
	"fmt"
	"math/rand"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	style2048Title = lipgloss.NewStyle().Foreground(lipgloss.Color("#facc15")).Bold(true) // Yellow
	styleBorder2048 = lipgloss.NewStyle().Foreground(lipgloss.Color("#475569"))            // Slate
)

var tileColors = map[int]lipgloss.Style{
	0:    lipgloss.NewStyle().Background(lipgloss.Color("#1e293b")).Foreground(lipgloss.Color("#475569")),                          // Empty dark tile
	2:    lipgloss.NewStyle().Background(lipgloss.Color("#334155")).Foreground(lipgloss.Color("#ffffff")).Bold(true),              // Slate 2
	4:    lipgloss.NewStyle().Background(lipgloss.Color("#0284c7")).Foreground(lipgloss.Color("#ffffff")).Bold(true),              // Sky Blue 4
	8:    lipgloss.NewStyle().Background(lipgloss.Color("#2563eb")).Foreground(lipgloss.Color("#ffffff")).Bold(true),              // Royal Blue 8
	16:   lipgloss.NewStyle().Background(lipgloss.Color("#4f46e5")).Foreground(lipgloss.Color("#ffffff")).Bold(true),              // Indigo 16
	32:   lipgloss.NewStyle().Background(lipgloss.Color("#7c3aed")).Foreground(lipgloss.Color("#ffffff")).Bold(true),              // Violet 32
	64:   lipgloss.NewStyle().Background(lipgloss.Color("#9333ea")).Foreground(lipgloss.Color("#ffffff")).Bold(true),              // Purple 64
	128:  lipgloss.NewStyle().Background(lipgloss.Color("#c026d3")).Foreground(lipgloss.Color("#ffffff")).Bold(true),              // Fuchsia 128
	256:  lipgloss.NewStyle().Background(lipgloss.Color("#db2777")).Foreground(lipgloss.Color("#ffffff")).Bold(true),              // Pink 256
	512:  lipgloss.NewStyle().Background(lipgloss.Color("#e11d48")).Foreground(lipgloss.Color("#ffffff")).Bold(true),              // Rose 512
	1024: lipgloss.NewStyle().Background(lipgloss.Color("#ea580c")).Foreground(lipgloss.Color("#ffffff")).Bold(true),              // Amber/Orange 1024
	2048: lipgloss.NewStyle().Background(lipgloss.Color("#16a34a")).Foreground(lipgloss.Color("#ffffff")).Bold(true),              // Emerald 2048
}

// Merge2048 is a classic 4x4 tile merging puzzle game.
type Merge2048 struct {
	board     [4][4]int
	score     int
	highScore int
	won       bool
	gameOver  bool
}

// NewMerge2048 creates a fresh 2048 Bytes game instance.
func NewMerge2048() MiniGame {
	g := &Merge2048{}
	g.reset()
	return g
}

func (m *Merge2048) reset() {
	m.board = [4][4]int{}
	m.score = 0
	m.won = false
	m.gameOver = false
	m.spawnTile()
	m.spawnTile()
}

func (m *Merge2048) spawnTile() {
	type pt struct{ r, c int }
	var empty []pt
	for r := 0; r < 4; r++ {
		for c := 0; c < 4; c++ {
			if m.board[r][c] == 0 {
				empty = append(empty, pt{r, c})
			}
		}
	}
	if len(empty) == 0 {
		return
	}
	target := empty[rand.Intn(len(empty))]
	val := 2
	if rand.Float64() < 0.10 {
		val = 4
	}
	m.board[target.r][target.c] = val
}

func (m *Merge2048) Title() string {
	return "2048 Bytes 🔢 [MERGE]"
}

func (m *Merge2048) Init() tea.Cmd {
	return nil
}

func (m *Merge2048) IsGameOver() bool {
	return m.gameOver
}

func (m *Merge2048) Score() int {
	return m.score
}

func (m *Merge2048) Dimensions() (int, int) {
	return 54, 20
}

func slideAndMergeLine(line [4]int) ([4]int, int, bool) {
	// 1. Filter non-zero
	var filtered []int
	for _, v := range line {
		if v != 0 {
			filtered = append(filtered, v)
		}
	}

	// 2. Merge adjacent equals
	var merged []int
	pts := 0
	for i := 0; i < len(filtered); i++ {
		if i+1 < len(filtered) && filtered[i] == filtered[i+1] {
			sum := filtered[i] * 2
			merged = append(merged, sum)
			pts += sum
			i++ // skip next
		} else {
			merged = append(merged, filtered[i])
		}
	}

	// 3. Pad with zeros to 4
	var res [4]int
	for i := 0; i < 4; i++ {
		if i < len(merged) {
			res[i] = merged[i]
		} else {
			res[i] = 0
		}
	}

	changed := (res != line)
	return res, pts, changed
}

func (m *Merge2048) canMove() bool {
	for r := 0; r < 4; r++ {
		for c := 0; c < 4; c++ {
			if m.board[r][c] == 0 {
				return true
			}
			if c+1 < 4 && m.board[r][c] == m.board[r][c+1] {
				return true
			}
			if r+1 < 4 && m.board[r][c] == m.board[r+1][c] {
				return true
			}
		}
	}
	return false
}

func (m *Merge2048) moveLeft() bool {
	changed := false
	for r := 0; r < 4; r++ {
		newLine, pts, ch := slideAndMergeLine(m.board[r])
		if ch {
			changed = true
			m.board[r] = newLine
			m.score += pts
		}
	}
	return changed
}

func (m *Merge2048) moveRight() bool {
	changed := false
	for r := 0; r < 4; r++ {
		rev := [4]int{m.board[r][3], m.board[r][2], m.board[r][1], m.board[r][0]}
		newLine, pts, ch := slideAndMergeLine(rev)
		if ch {
			changed = true
			m.board[r] = [4]int{newLine[3], newLine[2], newLine[1], newLine[0]}
			m.score += pts
		}
	}
	return changed
}

func (m *Merge2048) moveUp() bool {
	changed := false
	for c := 0; c < 4; c++ {
		col := [4]int{m.board[0][c], m.board[1][c], m.board[2][c], m.board[3][c]}
		newLine, pts, ch := slideAndMergeLine(col)
		if ch {
			changed = true
			for r := 0; r < 4; r++ {
				m.board[r][c] = newLine[r]
			}
			m.score += pts
		}
	}
	return changed
}

func (m *Merge2048) moveDown() bool {
	changed := false
	for c := 0; c < 4; c++ {
		col := [4]int{m.board[3][c], m.board[2][c], m.board[1][c], m.board[0][c]}
		newLine, pts, ch := slideAndMergeLine(col)
		if ch {
			changed = true
			for r := 0; r < 4; r++ {
				m.board[r][c] = newLine[3-r]
			}
			m.score += pts
		}
	}
	return changed
}

func (m *Merge2048) Update(msg tea.Msg) (MiniGame, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.gameOver {
			switch msg.String() {
			case " ", "enter":
				best := m.highScore
				m.reset()
				m.highScore = best
				return m, nil
			}
			return m, nil
		}

		changed := false
		switch msg.String() {
		case "left", "a", "h":
			changed = m.moveLeft()
		case "right", "d", "l":
			changed = m.moveRight()
		case "up", "w", "k":
			changed = m.moveUp()
		case "down", "s", "j":
			changed = m.moveDown()
		}

		if changed {
			if m.score > m.highScore {
				m.highScore = m.score
			}
			m.spawnTile()

			// Check 2048 win tile
			for r := 0; r < 4; r++ {
				for c := 0; c < 4; c++ {
					if m.board[r][c] >= 2048 {
						m.won = true
					}
				}
			}

			// Check game over
			if !m.canMove() {
				m.gameOver = true
			}
		}
	}

	return m, nil
}

func formatTile(val int) string {
	st, ok := tileColors[val]
	if !ok {
		st = tileColors[2048]
	}
	if val == 0 {
		return st.Render("  ·   ")
	}
	s := fmt.Sprintf("%d", val)
	switch len(s) {
	case 1:
		return st.Render(fmt.Sprintf("  %s   ", s))
	case 2:
		return st.Render(fmt.Sprintf("  %s  ", s))
	case 3:
		return st.Render(fmt.Sprintf(" %s  ", s))
	default:
		return st.Render(fmt.Sprintf(" %s ", s))
	}
}

func (m *Merge2048) View(targetWidth int) string {
	var sb strings.Builder

	gridW := 29
	padN := (targetWidth - gridW) / 2
	if padN < 2 {
		padN = 2
	}
	pad := strings.Repeat(" ", padN)

	// Header
	header := fmt.Sprintf("SCORE: %06d   BEST: %06d", m.score, m.highScore)
	if m.won {
		header += "  " + style2048Title.Render("★ 2048 REACHED! ★")
	}
	sb.WriteString(pad + style2048Title.Render("2048 BYTES") + "\n")
	sb.WriteString(pad + header + "\n\n")

	// 4x4 Grid Board
	topBorder := pad + styleBorder2048.Render("┌"+strings.Repeat("──────┬", 3)+"──────┐") + "\n"
	midBorder := pad + styleBorder2048.Render("├"+strings.Repeat("──────┼", 3)+"──────┤") + "\n"
	botBorder := pad + styleBorder2048.Render("└"+strings.Repeat("──────┴", 3)+"──────┘") + "\n"

	sb.WriteString(topBorder)

	for r := 0; r < 4; r++ {
		sb.WriteString(pad)
		sb.WriteString(styleBorder2048.Render("│"))
		for c := 0; c < 4; c++ {
			sb.WriteString(formatTile(m.board[r][c]))
			sb.WriteString(styleBorder2048.Render("│"))
		}
		sb.WriteByte('\n')

		if r < 3 {
			sb.WriteString(midBorder)
		}
	}

	sb.WriteString(botBorder)
	sb.WriteString("\n")

	var status string
	if m.gameOver {
		status = "💥 NO MORE MOVES! Space to Restart · Esc to Exit"
	} else {
		status = "Arrows / WASD / HJKL: Slide & Merge · Esc: Exit"
	}
	statusPadN := (targetWidth - lipgloss.Width(status)) / 2
	if statusPadN < 1 {
		statusPadN = 1
	}
	sb.WriteString(strings.Repeat(" ", statusPadN) + styleBorder2048.Render(status))

	return sb.String()
}
