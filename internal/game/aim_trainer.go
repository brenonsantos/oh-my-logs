package game

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	targetWidth  = 6
	targetHeight = 3
	defaultPlayW = 54
	defaultPlayH = 12
	blitzTime    = 10 * time.Second
	tickInterval = 50 * time.Millisecond
)

var (
	styleAimBorder   = lipgloss.NewStyle().Foreground(lipgloss.Color("#475569"))
	styleAimTarget   = lipgloss.NewStyle().Foreground(lipgloss.Color("#38bdf8")).Bold(true) // Neon Cyan
	styleAimHit      = lipgloss.NewStyle().Foreground(lipgloss.Color("#4ade80")).Bold(true) // Green
	styleAimMiss     = lipgloss.NewStyle().Foreground(lipgloss.Color("#f87171")).Bold(true) // Red
	styleAimCombo    = lipgloss.NewStyle().Foreground(lipgloss.Color("#f59e0b")).Bold(true) // Orange/Amber
	styleAimTimer    = lipgloss.NewStyle().Foreground(lipgloss.Color("#e2e8f0")).Bold(true)
	styleAimMuted    = lipgloss.NewStyle().Foreground(lipgloss.Color("#64748b"))
	styleAimStatVal  = lipgloss.NewStyle().Foreground(lipgloss.Color("#38bdf8")).Bold(true)
	styleAimStatLbl  = lipgloss.NewStyle().Foreground(lipgloss.Color("#94a3b8"))
	styleAimRankS    = lipgloss.NewStyle().Foreground(lipgloss.Color("#facc15")).Bold(true) // Gold
	styleAimRankA    = lipgloss.NewStyle().Foreground(lipgloss.Color("#a855f7")).Bold(true) // Purple
	styleAimRankB    = lipgloss.NewStyle().Foreground(lipgloss.Color("#38bdf8")).Bold(true) // Cyan
	styleAimRankC    = lipgloss.NewStyle().Foreground(lipgloss.Color("#64748b")).Bold(true) // Slate
)

// AimTarget represents one active target on the playfield.
type AimTarget struct {
	ID        int
	X         int
	Y         int
	SpawnTime time.Time
}

// AimFlash represents a temporary hit/miss pop effect on the playfield.
type AimFlash struct {
	X       int
	Y       int
	Text    string
	IsMiss  bool
	Expires time.Time
}

// AimClickMsg carries relative playfield coordinates for a mouse click.
type AimClickMsg struct {
	X int
	Y int
}

// AimTrainer is a 10-second mouse-driven blitz reflex trainer.
type AimTrainer struct {
	playWidth     int
	playHeight    int
	targets       []AimTarget
	flashes       []AimFlash
	hits          int
	misses        int
	score         int
	combo         int
	maxCombo      int
	reactionTimes []time.Duration
	fastestTime   time.Duration
	timeLeft      time.Duration
	gameOver      bool
	nextTargetID  int
	rng           *rand.Rand
}

// NewAimTrainer constructs a new 10-second aim trainer game instance.
func NewAimTrainer() MiniGame {
	at := &AimTrainer{
		playWidth:  defaultPlayW,
		playHeight: defaultPlayH,
		combo:      1,
		maxCombo:   1,
		timeLeft:   blitzTime,
		rng:        rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	at.reset()
	return at
}

func (at *AimTrainer) Title() string {
	return "🎯 Aim Trainer (Blitz)"
}

func (at *AimTrainer) Dimensions() (width, height int) {
	return at.playWidth + 8, at.playHeight + 12
}

func (at *AimTrainer) Score() int {
	return at.score
}

// Targets returns a copy of the active targets for testing and inspection.
func (at *AimTrainer) Targets() []AimTarget {
	out := make([]AimTarget, len(at.targets))
	copy(out, at.targets)
	return out
}

func (at *AimTrainer) IsGameOver() bool {
	return at.gameOver
}

func (at *AimTrainer) Init() tea.Cmd {
	return Tick(tickInterval)
}

func targetsOverlap(x1, y1, x2, y2 int) bool {
	return (x1 < x2+targetWidth+2 && x1+targetWidth+2 > x2) &&
		(y1 < y2+targetHeight+1 && y1+targetHeight+1 > y2)
}

func (at *AimTrainer) spawnTarget() AimTarget {
	maxX := at.playWidth - targetWidth
	maxY := at.playHeight - targetHeight
	if maxX <= 0 {
		maxX = 1
	}
	if maxY <= 0 {
		maxY = 1
	}

	bestX, bestY := 0, 0
	attempts := 0
	for attempts < 50 {
		candidateX := at.rng.Intn(maxX)
		candidateY := at.rng.Intn(maxY)
		overlaps := false
		for _, existing := range at.targets {
			if targetsOverlap(candidateX, candidateY, existing.X, existing.Y) {
				overlaps = true
				break
			}
		}
		if !overlaps || len(at.targets) == 0 {
			bestX = candidateX
			bestY = candidateY
			break
		}
		attempts++
	}

	at.nextTargetID++
	return AimTarget{
		ID:        at.nextTargetID,
		X:         bestX,
		Y:         bestY,
		SpawnTime: time.Now(),
	}
}

func (at *AimTrainer) reset() {
	at.targets = nil
	at.flashes = nil
	at.hits = 0
	at.misses = 0
	at.score = 0
	at.combo = 1
	at.maxCombo = 1
	at.reactionTimes = nil
	at.fastestTime = 0
	at.timeLeft = blitzTime
	at.gameOver = false

	for len(at.targets) < 3 {
		at.targets = append(at.targets, at.spawnTarget())
	}
}

// HandleClick processes a click at board coordinate (x, y). Returns true if a target was hit.
func (at *AimTrainer) HandleClick(x, y int) bool {
	if at.gameOver {
		at.reset()
		return false
	}

	now := time.Now()
	for i, tg := range at.targets {
		if x >= tg.X && x < tg.X+targetWidth && y >= tg.Y && y < tg.Y+targetHeight {
			// HIT!
			reaction := now.Sub(tg.SpawnTime)
			at.hits++
			at.reactionTimes = append(at.reactionTimes, reaction)
			if at.fastestTime == 0 || reaction < at.fastestTime {
				at.fastestTime = reaction
			}

			// Points: 100 base + speed bonus up to 100 for sub-300ms reaction
			speedBonus := 0
			if reaction < 400*time.Millisecond {
				speedBonus = int((400*time.Millisecond - reaction).Milliseconds()) / 3
			}
			points := (100 + speedBonus) * at.combo
			at.score += points

			at.flashes = append(at.flashes, AimFlash{
				X:       tg.X,
				Y:       tg.Y,
				Text:    fmt.Sprintf("+%d (%dms)", points, reaction.Milliseconds()),
				IsMiss:  false,
				Expires: now.Add(350 * time.Millisecond),
			})

			at.combo++
			if at.combo > at.maxCombo {
				at.maxCombo = at.combo
			}

			// Remove hit target and spawn replacement
			at.targets = append(at.targets[:i], at.targets[i+1:]...)
			at.targets = append(at.targets, at.spawnTarget())
			return true
		}
	}

	// MISS
	at.misses++
	at.combo = 1
	flashX := x - 2
	if flashX < 0 {
		flashX = 0
	}
	at.flashes = append(at.flashes, AimFlash{
		X:       flashX,
		Y:       y,
		Text:    "MISS",
		IsMiss:  true,
		Expires: now.Add(250 * time.Millisecond),
	})
	return false
}

func (at *AimTrainer) Update(msg tea.Msg) (MiniGame, tea.Cmd) {
	now := time.Now()

	switch msg := msg.(type) {
	case TickMsg:
		if !at.gameOver {
			at.timeLeft -= tickInterval
			if at.timeLeft <= 0 {
				at.timeLeft = 0
				at.gameOver = true
			}
		}

		// Prune expired flashes
		var activeFlashes []AimFlash
		for _, f := range at.flashes {
			if now.Before(f.Expires) {
				activeFlashes = append(activeFlashes, f)
			}
		}
		at.flashes = activeFlashes

		if !at.gameOver {
			return at, Tick(tickInterval)
		}
		return at, nil

	case AimClickMsg:
		at.HandleClick(msg.X, msg.Y)
		return at, nil

	case tea.MouseMsg:
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			at.HandleClick(msg.X, msg.Y)
		}
		return at, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "r", "R", " ":
			if at.gameOver {
				at.reset()
				return at, Tick(tickInterval)
			}
		}
	}

	return at, nil
}

func (at *AimTrainer) averageReactionTime() time.Duration {
	if len(at.reactionTimes) == 0 {
		return 0
	}
	var total time.Duration
	for _, d := range at.reactionTimes {
		total += d
	}
	return total / time.Duration(len(at.reactionTimes))
}

func (at *AimTrainer) accuracyPercent() int {
	totalClicks := at.hits + at.misses
	if totalClicks == 0 {
		return 0
	}
	return int((float64(at.hits) / float64(totalClicks)) * 100)
}

func (at *AimTrainer) rankBadge() (string, lipgloss.Style) {
	acc := at.accuracyPercent()
	if at.hits >= 22 && acc >= 90 {
		return "RANK S (GODLIKE)", styleAimRankS
	}
	if at.hits >= 16 && acc >= 80 {
		return "RANK A (SHARPSHOOTER)", styleAimRankA
	}
	if at.hits >= 10 && acc >= 65 {
		return "RANK B (DECENT)", styleAimRankB
	}
	return "RANK C (ROOKIE)", styleAimRankC
}

func (at *AimTrainer) View(width int) string {
	var sb strings.Builder

	// 1. Header Bar
	sec := float64(at.timeLeft.Milliseconds()) / 1000.0
	timeStr := fmt.Sprintf("⏱ %04.1fs", sec)
	timePart := styleAimTimer.Render(timeStr)
	scorePart := fmt.Sprintf("SCORE: %s", styleAimStatVal.Render(fmt.Sprintf("%d", at.score)))
	comboStr := fmt.Sprintf("🔥 x%d", at.combo)
	if at.combo > 1 {
		comboStr = styleAimCombo.Render(comboStr)
	} else {
		comboStr = styleAimMuted.Render("x1")
	}

	accVal := at.accuracyPercent()
	totalClicks := at.hits + at.misses
	accPart := fmt.Sprintf("ACC: %s%% (%d/%d)", styleAimStatVal.Render(fmt.Sprintf("%d", accVal)), at.hits, totalClicks)

	header := fmt.Sprintf("  %s  │  %s  │  %s  │  %s", timePart, scorePart, comboStr, accPart)
	sb.WriteString(header)
	sb.WriteString("\n")

	// 2. Playfield Divider
	topBorder := "┌" + strings.Repeat("─", at.playWidth) + "┐"
	sb.WriteString(styleAimBorder.Render(topBorder))
	sb.WriteString("\n")

	// 3. Grid Playfield
	if at.gameOver {
		// End of Round Summary Screen
		rankStr, rankStyle := at.rankBadge()
		cps := float64(at.hits) / 10.0
		avgMs := at.averageReactionTime().Milliseconds()
		fastestMs := at.fastestTime.Milliseconds()

		card := []string{
			"                  ★ GAME OVER ★                       ",
			rankStyle.Render(fmt.Sprintf("   ★ ★ ★  %s  ★ ★ ★   ", rankStr)),
			"",
			fmt.Sprintf("       FINAL SCORE: %s", styleAimStatVal.Render(fmt.Sprintf("%d", at.score))),
			fmt.Sprintf("       TARGETS HIT: %s / %d clicks", styleAimStatVal.Render(fmt.Sprintf("%d", at.hits)), totalClicks),
			fmt.Sprintf("          ACCURACY: %s%%", styleAimStatVal.Render(fmt.Sprintf("%d", accVal))),
			fmt.Sprintf("       AVG REACTION: %s ms", styleAimStatVal.Render(fmt.Sprintf("%d", avgMs))),
			fmt.Sprintf("      FASTEST FLICK: %s ms", styleAimStatVal.Render(fmt.Sprintf("%d", fastestMs))),
			fmt.Sprintf("             SPEED: %s targets/sec", styleAimStatVal.Render(fmt.Sprintf("%.1f", cps))),
			fmt.Sprintf("         MAX COMBO: %s", styleAimCombo.Render(fmt.Sprintf("🔥 x%d", at.maxCombo))),
			"",
			styleAimMuted.Render("       [R / Space] Play Again  ·  [Esc] Back to Logs"),
		}

		for _, line := range card {
			contentW := lipgloss.Width(line)
			pad := at.playWidth - contentW
			if pad < 0 {
				pad = 0
			}
			row := "│" + line + strings.Repeat(" ", pad) + "│"
			sb.WriteString(styleAimBorder.Render(row))
			sb.WriteString("\n")
		}
	} else {
		// Active Playfield: Render 3 centered targets and flashes by visual columns
		type visualSpan struct {
			x     int
			text  string
			width int
		}

		for r := 0; r < at.playHeight; r++ {
			var spans []visualSpan

			// Targets on row r
			for _, tg := range at.targets {
				if r == tg.Y {
					spans = append(spans, visualSpan{x: tg.X, text: styleAimTarget.Render("╭────╮"), width: targetWidth})
				} else if r == tg.Y+1 {
					spans = append(spans, visualSpan{x: tg.X, text: styleAimTarget.Render("│ 🎯 │"), width: targetWidth})
				} else if r == tg.Y+2 {
					spans = append(spans, visualSpan{x: tg.X, text: styleAimTarget.Render("╰────╯"), width: targetWidth})
				}
			}

			// Flashes on row r
			for _, fl := range at.flashes {
				if fl.Y == r {
					flStyle := styleAimHit
					if fl.IsMiss {
						flStyle = styleAimMiss
					}
					flTxt := flStyle.Render(fl.Text)
					wTxt := lipgloss.Width(fl.Text)
					spans = append(spans, visualSpan{x: fl.X, text: flTxt, width: wTxt})
				}
			}

			// Sort spans by ascending X coordinate
			sort.Slice(spans, func(i, j int) bool {
				return spans[i].x < spans[j].x
			})

			// Build row by visual column
			var rowBuf strings.Builder
			curCol := 0
			for _, sp := range spans {
				if sp.x > curCol {
					rowBuf.WriteString(strings.Repeat(" ", sp.x-curCol))
					curCol = sp.x
				}
				if sp.x == curCol && curCol+sp.width <= at.playWidth {
					rowBuf.WriteString(sp.text)
					curCol += sp.width
				}
			}
			if curCol < at.playWidth {
				rowBuf.WriteString(strings.Repeat(" ", at.playWidth-curCol))
			}

			sb.WriteString(styleAimBorder.Render("│") + rowBuf.String() + styleAimBorder.Render("│"))
			sb.WriteString("\n")
		}
	}

	// 4. Playfield Footer
	bottomBorder := "└" + strings.Repeat("─", at.playWidth) + "┘"
	sb.WriteString(styleAimBorder.Render(bottomBorder))
	sb.WriteString("\n")
	sb.WriteString(styleAimMuted.Render("  Click targets to shoot · Misses break combo · [R] Reset · [Esc] Log"))

	return sb.String()
}
