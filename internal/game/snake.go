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
	styleSnakeHead = lipgloss.NewStyle().Foreground(lipgloss.Color("#4ade80")).Bold(true) // Neon Green
	styleSnakeBody = lipgloss.NewStyle().Foreground(lipgloss.Color("#22c55e"))             // Green
	styleSnakeDead = lipgloss.NewStyle().Foreground(lipgloss.Color("#f87171")).Bold(true) // Red
	styleFood      = lipgloss.NewStyle().Foreground(lipgloss.Color("#facc15")).Bold(true) // Yellow
	styleBorder    = lipgloss.NewStyle().Foreground(lipgloss.Color("#38bdf8"))             // Sky Blue
	styleHexBadge  = lipgloss.NewStyle().Foreground(lipgloss.Color("#c084fc")).Bold(true) // Purple
)

type dir int

const (
	dirUp dir = iota
	dirDown
	dirLeft
	dirRight
)

// Point2D represents a coordinate on the snake grid.
type Point2D struct {
	X int
	Y int
}

var bytePackets = []string{
	"0x42", "0xFF", "0xAA", "0x00", "0x55", "0xC0", "CRC", "ACK", "SYN", "DATA",
}

// ByteSnake is a retro snake mini-game with embedded serial packet aesthetics.
type ByteSnake struct {
	gridW      int
	gridH      int
	snake      []Point2D // snake[0] is head
	currentDir dir
	nextDir    dir
	food       Point2D
	foodByte   string
	score      int
	highScore  int
	foodsEaten int
	gameOver   bool
	tickSpeed  time.Duration
}

// NewByteSnake creates a fresh Byte Snake mini-game instance.
func NewByteSnake() MiniGame {
	g := &ByteSnake{
		gridW:     20,
		gridH:     11,
		tickSpeed: 105 * time.Millisecond,
	}
	g.reset()
	return g
}

func (s *ByteSnake) reset() {
	midX := s.gridW / 2
	midY := s.gridH / 2

	// Initial snake of length 3 moving Right
	s.snake = []Point2D{
		{X: midX, Y: midY},
		{X: midX - 1, Y: midY},
		{X: midX - 2, Y: midY},
	}
	s.currentDir = dirRight
	s.nextDir = dirRight
	s.score = 0
	s.foodsEaten = 0
	s.gameOver = false
	s.tickSpeed = 105 * time.Millisecond
	s.spawnFood()
}

func (s *ByteSnake) spawnFood() {
	occupied := make(map[Point2D]bool, len(s.snake))
	for _, p := range s.snake {
		occupied[p] = true
	}

	var free []Point2D
	for y := 0; y < s.gridH; y++ {
		for x := 0; x < s.gridW; x++ {
			pt := Point2D{X: x, Y: y}
			if !occupied[pt] {
				free = append(free, pt)
			}
		}
	}

	if len(free) == 0 {
		// Board full, win condition
		s.gameOver = true
		return
	}

	s.food = free[rand.Intn(len(free))]
	s.foodByte = bytePackets[rand.Intn(len(bytePackets))]
}

func (s *ByteSnake) Title() string {
	return "Byte Snake 🐍 0x42"
}

func (s *ByteSnake) Init() tea.Cmd {
	return Tick(s.tickSpeed)
}

func (s *ByteSnake) IsGameOver() bool {
	return s.gameOver
}

func (s *ByteSnake) Score() int {
	return s.score
}

func (s *ByteSnake) Update(msg tea.Msg) (MiniGame, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "w", "k":
			if s.currentDir != dirDown {
				s.nextDir = dirUp
			}
		case "down", "s", "j":
			if s.currentDir != dirUp {
				s.nextDir = dirDown
			}
		case "left", "a", "h":
			if s.currentDir != dirRight {
				s.nextDir = dirLeft
			}
		case "right", "d", "l":
			if s.currentDir != dirLeft {
				s.nextDir = dirRight
			}
		case " ", "enter":
			if s.gameOver {
				best := s.highScore
				s.reset()
				s.highScore = best
				return s, s.Init()
			}
		}

	case TickMsg:
		if s.gameOver {
			return s, nil
		}

		s.currentDir = s.nextDir

		// Compute next head location
		head := s.snake[0]
		nextHead := head
		switch s.currentDir {
		case dirUp:
			nextHead.Y--
		case dirDown:
			nextHead.Y++
		case dirLeft:
			nextHead.X--
		case dirRight:
			nextHead.X++
		}

		// Wall collision check
		if nextHead.X < 0 || nextHead.X >= s.gridW || nextHead.Y < 0 || nextHead.Y >= s.gridH {
			s.gameOver = true
			return s, nil
		}

		// Self collision check (ignoring tail tip which will move away unless eating)
		willEat := (nextHead == s.food)
		checkLen := len(s.snake)
		if !willEat {
			checkLen--
		}
		for i := 0; i < checkLen; i++ {
			if s.snake[i] == nextHead {
				s.gameOver = true
				return s, nil
			}
		}

		// Move snake
		newSnake := make([]Point2D, 0, len(s.snake)+1)
		newSnake = append(newSnake, nextHead)

		if willEat {
			// Grow: keep entire existing snake
			newSnake = append(newSnake, s.snake...)
			s.score += 10
			s.foodsEaten++
			if s.score > s.highScore {
				s.highScore = s.score
			}
			// Speed up slightly every 3 eaten bytes down to 60ms
			if s.foodsEaten%3 == 0 && s.tickSpeed > 60*time.Millisecond {
				s.tickSpeed -= 5 * time.Millisecond
			}
			s.spawnFood()
		} else {
			// Normal move: drop tail
			newSnake = append(newSnake, s.snake[:len(s.snake)-1]...)
		}

		s.snake = newSnake
		return s, Tick(s.tickSpeed)
	}

	return s, nil
}

func (s *ByteSnake) View(targetWidth int) string {
	var sb strings.Builder

	// Header: Score, High Score, Current Target Packet
	scoreLine := fmt.Sprintf("  SCORE: %05d   BEST: %05d   PACKET: %s", s.score, s.highScore, styleHexBadge.Render(s.foodByte))
	sb.WriteString(styleScore.Render(scoreLine))
	sb.WriteString("\n\n")

	// Pre-map coordinates for efficient grid rendering
	type cellType int
	const (
		cellEmpty cellType = iota
		cellHead
		cellBody
		cellFood
	)

	grid := make([][]cellType, s.gridH)
	for y := 0; y < s.gridH; y++ {
		grid[y] = make([]cellType, s.gridW)
	}

	for i, pt := range s.snake {
		if pt.Y >= 0 && pt.Y < s.gridH && pt.X >= 0 && pt.X < s.gridW {
			if i == 0 {
				grid[pt.Y][pt.X] = cellHead
			} else {
				grid[pt.Y][pt.X] = cellBody
			}
		}
	}

	if !s.gameOver && s.food.Y >= 0 && s.food.Y < s.gridH && s.food.X >= 0 && s.food.X < s.gridW {
		grid[s.food.Y][s.food.X] = cellFood
	}

	// Board top border (each cell is 2 chars wide: 2 * gridW chars)
	topBorder := "  " + styleBorder.Render("┌"+strings.Repeat("─", s.gridW*2)+"┐")
	sb.WriteString(topBorder)
	sb.WriteByte('\n')

	for y := 0; y < s.gridH; y++ {
		sb.WriteString("  ")
		sb.WriteString(styleBorder.Render("│"))
		for x := 0; x < s.gridW; x++ {
			switch grid[y][x] {
			case cellHead:
				if s.gameOver {
					sb.WriteString(styleSnakeDead.Render("💥"))
				} else {
					sb.WriteString(styleSnakeHead.Render("◈ "))
				}
			case cellBody:
				if s.gameOver {
					sb.WriteString(styleSnakeDead.Render("▪ "))
				} else {
					sb.WriteString(styleSnakeBody.Render("■ "))
				}
			case cellFood:
				sb.WriteString(styleFood.Render("★ "))
			default:
				sb.WriteString("  ")
			}
		}
		sb.WriteString(styleBorder.Render("│"))
		sb.WriteByte('\n')
	}

	// Board bottom border
	bottomBorder := "  " + styleBorder.Render("└"+strings.Repeat("─", s.gridW*2)+"┘")
	sb.WriteString(bottomBorder)
	sb.WriteString("\n\n")

	// Status / Controls
	if s.gameOver {
		sb.WriteString(styleCrash.Render("  💥 BUFFER OVERFLOW! Press Space to Restart · Esc to Exit"))
	} else {
		sb.WriteString(styleHelp.Render("  Arrows / WASD / hjkl: Steer · Esc: Back to logs"))
	}

	return sb.String()
}

func (s *ByteSnake) Dimensions() (int, int) {
	return 54, 21
}
