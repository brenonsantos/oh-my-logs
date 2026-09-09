package game

import (
	"fmt"
	"math/rand"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	styleSweeperTitle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#facc15")).Bold(true) // Yellow
	styleSweeperBorder = lipgloss.NewStyle().Foreground(lipgloss.Color("#475569"))            // Slate
	styleSweeperCursor = lipgloss.NewStyle().Foreground(lipgloss.Color("#38bdf8")).Bold(true) // Cyan
	styleSweeperFlag   = lipgloss.NewStyle().Foreground(lipgloss.Color("#f59e0b")).Bold(true) // Amber
	styleSweeperMine   = lipgloss.NewStyle().Foreground(lipgloss.Color("#ef4444")).Bold(true) // Red
	styleSweeperSafe   = lipgloss.NewStyle().Foreground(lipgloss.Color("#4ade80")).Bold(true) // Green
	styleSweeperDead   = lipgloss.NewStyle().Foreground(lipgloss.Color("#f87171")).Bold(true)
	styleSweeperMuted  = lipgloss.NewStyle().Foreground(lipgloss.Color("#64748b"))
)

var sweeperNumStyles = map[int]lipgloss.Style{
	1: lipgloss.NewStyle().Foreground(lipgloss.Color("#60a5fa")).Bold(true), // Blue
	2: lipgloss.NewStyle().Foreground(lipgloss.Color("#4ade80")).Bold(true), // Green
	3: lipgloss.NewStyle().Foreground(lipgloss.Color("#f87171")).Bold(true), // Red
	4: lipgloss.NewStyle().Foreground(lipgloss.Color("#c084fc")).Bold(true), // Purple
	5: lipgloss.NewStyle().Foreground(lipgloss.Color("#fb923c")).Bold(true), // Orange
	6: lipgloss.NewStyle().Foreground(lipgloss.Color("#2dd4bf")).Bold(true), // Teal
	7: lipgloss.NewStyle().Foreground(lipgloss.Color("#f472b6")).Bold(true), // Pink
	8: lipgloss.NewStyle().Foreground(lipgloss.Color("#e2e8f0")).Bold(true), // White
}

const (
	sweepRows  = 9
	sweepCols  = 12
	sweepMines = 12
)

type sweeperCell struct {
	isMine   bool
	revealed bool
	flagged  bool
	adjacent int
}

// MemorySweeper is a memory-inspection minesweeper mini-game.
type MemorySweeper struct {
	grid       [sweepRows][sweepCols]sweeperCell
	cursorR    int
	cursorC    int
	minesCount int
	flagsCount int
	safeTotal  int
	safeLeft   int
	won        bool
	gameOver   bool
	generated  bool
	score      int
}

// NewMemorySweeper creates a fresh Memory Sweeper instance.
func NewMemorySweeper() MiniGame {
	m := &MemorySweeper{
		minesCount: sweepMines,
		safeTotal:  sweepRows*sweepCols - sweepMines,
	}
	m.reset()
	return m
}

func (m *MemorySweeper) reset() {
	m.grid = [sweepRows][sweepCols]sweeperCell{}
	m.cursorR = sweepRows / 2
	m.cursorC = sweepCols / 2
	m.flagsCount = 0
	m.safeLeft = m.safeTotal
	m.won = false
	m.gameOver = false
	m.generated = false
	m.score = 0
}

func (m *MemorySweeper) Title() string {
	return "Memory Sweeper 💣 [SWEEP]"
}

func (m *MemorySweeper) Init() tea.Cmd {
	return nil
}

func (m *MemorySweeper) IsGameOver() bool {
	return m.gameOver
}

func (m *MemorySweeper) Score() int {
	return m.score
}

func (m *MemorySweeper) Dimensions() (int, int) {
	return 52, 20
}

func (m *MemorySweeper) generateMines(firstR, firstC int) {
	placed := 0
	for placed < m.minesCount {
		r := rand.Intn(sweepRows)
		c := rand.Intn(sweepCols)
		// Don't place mine on first clicked cell or its immediate neighbors
		if (abs(r-firstR) <= 1 && abs(c-firstC) <= 1) || m.grid[r][c].isMine {
			continue
		}
		m.grid[r][c].isMine = true
		placed++
	}

	// Calculate adjacency numbers
	for r := 0; r < sweepRows; r++ {
		for c := 0; c < sweepCols; c++ {
			if m.grid[r][c].isMine {
				continue
			}
			count := 0
			for dr := -1; dr <= 1; dr++ {
				for dc := -1; dc <= 1; dc++ {
					nr, nc := r+dr, c+dc
					if nr >= 0 && nr < sweepRows && nc >= 0 && nc < sweepCols {
						if m.grid[nr][nc].isMine {
							count++
						}
					}
				}
			}
			m.grid[r][c].adjacent = count
		}
	}
	m.generated = true
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func (m *MemorySweeper) reveal(r, c int) {
	if r < 0 || r >= sweepRows || c < 0 || c >= sweepCols {
		return
	}
	cell := &m.grid[r][c]
	if cell.revealed || cell.flagged {
		return
	}

	// First reveal safety
	if !m.generated {
		m.generateMines(r, c)
	}

	cell.revealed = true

	if cell.isMine {
		m.gameOver = true
		// Reveal all mines
		for row := 0; row < sweepRows; row++ {
			for col := 0; col < sweepCols; col++ {
				if m.grid[row][col].isMine {
					m.grid[row][col].revealed = true
				}
			}
		}
		return
	}

	m.safeLeft--
	m.score += 10

	// Flood-fill opening for 0-neighbor sectors
	if cell.adjacent == 0 {
		for dr := -1; dr <= 1; dr++ {
			for dc := -1; dc <= 1; dc++ {
				m.reveal(r+dr, c+dc)
			}
		}
	}

	// Win check
	if m.safeLeft == 0 {
		m.won = true
		m.gameOver = true
		m.score += 500
	}
}

func (m *MemorySweeper) toggleFlag(r, c int) {
	if r < 0 || r >= sweepRows || c < 0 || c >= sweepCols {
		return
	}
	cell := &m.grid[r][c]
	if cell.revealed {
		return
	}
	if cell.flagged {
		cell.flagged = false
		m.flagsCount--
	} else {
		cell.flagged = true
		m.flagsCount++
	}
}

func (m *MemorySweeper) Update(msg tea.Msg) (MiniGame, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.gameOver {
			switch msg.String() {
			case " ", "enter", "r":
				m.reset()
				return m, nil
			}
			return m, nil
		}

		switch msg.String() {
		case "up", "w", "k":
			if m.cursorR > 0 {
				m.cursorR--
			}
		case "down", "s", "j":
			if m.cursorR < sweepRows-1 {
				m.cursorR++
			}
		case "left", "a", "h":
			if m.cursorC > 0 {
				m.cursorC--
			}
		case "right", "d", "l":
			if m.cursorC < sweepCols-1 {
				m.cursorC++
			}
		case "enter", " ":
			m.reveal(m.cursorR, m.cursorC)
		case "f", "m":
			m.toggleFlag(m.cursorR, m.cursorC)
		}
	}

	return m, nil
}

func (m *MemorySweeper) View(targetWidth int) string {
	var sb strings.Builder

	gridWidth := sweepCols * 3 // 12 * 3 = 36
	boxW := gridWidth + 2      // 36 + 2 = 38
	padN := (targetWidth - boxW) / 2
	if padN < 2 {
		padN = 2
	}
	pad := strings.Repeat(" ", padN)

	// Header
	header := fmt.Sprintf("SCORE: %04d   SECTORS: %02d/%02d   FLAGS: %02d",
		m.score, m.safeTotal-m.safeLeft, m.safeTotal, m.flagsCount)
	sb.WriteString(pad + styleSweeperTitle.Render("MEMORY SWEEPER") + "\n")
	sb.WriteString(pad + styleSweeperMuted.Render(header) + "\n\n")

	// Top border
	sb.WriteString(pad + styleSweeperBorder.Render("┌"+strings.Repeat("─", gridWidth)+"┐") + "\n")

	for r := 0; r < sweepRows; r++ {
		sb.WriteString(pad + styleSweeperBorder.Render("│"))

		for c := 0; c < sweepCols; c++ {
			cell := m.grid[r][c]
			isCursor := (r == m.cursorR && c == m.cursorC && !m.gameOver)

			var cellStr string
			if isCursor {
				if cell.flagged {
					cellStr = styleSweeperCursor.Render("[⚑]")
				} else if !cell.revealed {
					cellStr = styleSweeperCursor.Render("[·]")
				} else if cell.isMine {
					cellStr = styleSweeperCursor.Render("[💣]")
				} else if cell.adjacent == 0 {
					cellStr = styleSweeperCursor.Render("[ ]")
				} else {
					cellStr = styleSweeperCursor.Render(fmt.Sprintf("[%d]", cell.adjacent))
				}
			} else {
				if cell.flagged {
					cellStr = " " + styleSweeperFlag.Render("⚑") + " "
				} else if !cell.revealed {
					cellStr = " " + styleSweeperMuted.Render("·") + " "
				} else if cell.isMine {
					cellStr = " " + styleSweeperMine.Render("💣")
				} else if cell.adjacent == 0 {
					cellStr = "   "
				} else {
					st := sweeperNumStyles[cell.adjacent]
					cellStr = " " + st.Render(fmt.Sprintf("%d", cell.adjacent)) + " "
				}
			}

			sb.WriteString(cellStr)
		}

		sb.WriteString(styleSweeperBorder.Render("│") + "\n")
	}

	// Bottom border
	sb.WriteString(pad + styleSweeperBorder.Render("└"+strings.Repeat("─", gridWidth)+"┘") + "\n\n")

	var status string
	if m.won {
		status = "🎉 ALL SAFE SECTORS REVEALED! Space to Restart · Esc: Exit"
	} else if m.gameOver {
		status = "💥 SEGFAULT! Bad Sector Detonated! Space to Restart · Esc: Exit"
	} else {
		status = "Arrows: Move · Space: Reveal · F: Flag · Esc: Exit"
	}
	statusPadN := (targetWidth - lipgloss.Width(status)) / 2
	if statusPadN < 1 {
		statusPadN = 1
	}
	sb.WriteString(strings.Repeat(" ", statusPadN) + styleSweeperBorder.Render(status))

	return sb.String()
}
