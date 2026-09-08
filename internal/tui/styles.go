package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Colour palette
	colorBg       = lipgloss.Color("#1a1b26") // dark navy
	colorFg       = lipgloss.Color("#c0caf5") // soft white
	colorSubtle   = lipgloss.Color("#565f89") // muted
	colorAccent   = lipgloss.Color("#7aa2f7") // blue
	colorGreen    = lipgloss.Color("#9ece6a") // connected
	colorYellow   = lipgloss.Color("#e0af68") // warning
	colorRed      = lipgloss.Color("#f7768e") // error / disconnected
	colorHeader   = lipgloss.Color("#24283b") // table header bg
	colorSelected = lipgloss.Color("#2d3149") // selected row bg

	// Base text
	styleFg = lipgloss.NewStyle().Foreground(colorFg)

	// Title bar
	styleTitleBar = lipgloss.NewStyle().
			Background(colorHeader).
			Foreground(colorAccent).
			Bold(true).
			Padding(0, 1)

	// Status indicators
	styleConnected    = lipgloss.NewStyle().Foreground(colorGreen).Bold(true)
	styleDisconnected = lipgloss.NewStyle().Foreground(colorRed).Bold(true)
	styleError        = lipgloss.NewStyle().Foreground(colorYellow).Bold(true)

	// Table header
	styleTableHeader = lipgloss.NewStyle().
				Background(colorHeader).
				Foreground(colorAccent).
				Bold(true).
				Padding(0, 1)

	// Normal table row
	styleTableRow = lipgloss.NewStyle().
			Foreground(colorFg).
			Padding(0, 1)

	// Selected / highlighted row
	styleTableRowSelected = lipgloss.NewStyle().
				Background(colorSelected).
				Foreground(colorFg).
				Padding(0, 1)

	// Sidebar
	styleSidebar = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(colorSubtle).
			Padding(0, 1)

	styleSidebarTitle = lipgloss.NewStyle().
				Foreground(colorAccent).
				Bold(true)

	// Input / search bar
	styleInputPrompt = lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
	styleInputText   = lipgloss.NewStyle().Foreground(colorFg)

	// Status bar (footer)
	styleStatusBar = lipgloss.NewStyle().
			Background(colorHeader).
			Foreground(colorSubtle).
			Padding(0, 1)

	styleFollowOn  = lipgloss.NewStyle().Foreground(colorGreen).Bold(true)
	styleFollowOff = lipgloss.NewStyle().Foreground(colorYellow).Bold(true)

	// Key hints
	styleKeyHint = lipgloss.NewStyle().
			Background(colorHeader).
			Foreground(colorSubtle).
			Padding(0, 1)

	styleKeyName = lipgloss.NewStyle().Foreground(colorAccent)

	// Search highlight
	styleHighlight = lipgloss.NewStyle().
			Background(colorYellow).
			Foreground(colorBg)

	// Error / info message overlay
	styleMsg = lipgloss.NewStyle().
			Foreground(colorRed).
			Bold(true)
)
