package tui

import (
	"strings"

	"github.com/brenoniehues/oh-my-logs/internal/record"
	"github.com/charmbracelet/lipgloss"
)

// Palette defines the core color values used across the theme.
var (
	colorBg       = lipgloss.Color("#1a1b26") // background / dark slate
	colorHeaderBg = lipgloss.Color("#1f2335") // slightly lighter dark slate
	colorSelected = lipgloss.Color("#283457") // focused selection / row highlight
	colorSearchBg = lipgloss.Color("#252c48") // non-focused search match row

	colorFg     = lipgloss.Color("#c0caf5") // normal foreground / content
	colorMuted  = lipgloss.Color("#565f89") // muted / gray
	colorAccent = lipgloss.Color("#7aa2f7") // blue (Primary / Accent)
	colorCyan   = lipgloss.Color("#7dcfff") // bright cyan
	colorPurple = lipgloss.Color("#bb9af7") // purple / identifier
	colorGreen  = lipgloss.Color("#9ece6a") // success / connected
	colorYellow = lipgloss.Color("#e0af68") // warning / search focus
	colorRed    = lipgloss.Color("#f7768e") // error / alert
)

// Theme encapsulates all semantic visual styles.
type Theme struct {
	// Hierarchy
	Primary   lipgloss.Style // application / important state (bold accent)
	Secondary lipgloss.Style // headers / metadata (subtle accent / soft blue)
	Content   lipgloss.Style // actual log text
	Muted     lipgloss.Style // shortcuts / statistics / separators (gray)

	// Feedback / Status
	Accent  lipgloss.Style // cyan/blue
	Success lipgloss.Style // green
	Warning lipgloss.Style // yellow
	Error   lipgloss.Style // red

	// Table & Selection
	Header      lipgloss.Style // table header text
	HeaderBar   lipgloss.Style // table header background
	Divider     lipgloss.Style // horizontal divider lines
	Selected    lipgloss.Style // cursor / active selection
	SearchMatch lipgloss.Style // search match row
	SearchFocus lipgloss.Style // active focused search match
	Highlight   lipgloss.Style // matching substring highlight
	RowNormal   lipgloss.Style // normal table row

	// Modals & Panels
	ModalBox      lipgloss.Style // rounded border modal container
	ModalTitle    lipgloss.Style // title inside modal
	ModalSection  lipgloss.Style // section header (e.g. Baud rate, Serial Port)
	ModalItem     lipgloss.Style // normal item in modal
	ModalSelected lipgloss.Style // selected item in modal (with ›)
	ModalFooter   lipgloss.Style // help text at bottom of modal

	// Status & Footer
	TitleBar  lipgloss.Style
	StatusBar lipgloss.Style
	KeyBar    lipgloss.Style
	KeyName   lipgloss.Style
	FollowOn  lipgloss.Style
	FollowOff lipgloss.Style
	MsgInfo   lipgloss.Style
	MsgErr    lipgloss.Style
}

// DefaultTheme returns the default technical developer theme.
func DefaultTheme() Theme {
	return Theme{
		Primary:   lipgloss.NewStyle().Foreground(colorAccent).Bold(true),
		Secondary: lipgloss.NewStyle().Foreground(colorCyan),
		Content:   lipgloss.NewStyle().Foreground(colorFg),
		Muted:     lipgloss.NewStyle().Foreground(colorMuted),

		Accent:  lipgloss.NewStyle().Foreground(colorAccent),
		Success: lipgloss.NewStyle().Foreground(colorGreen).Bold(true),
		Warning: lipgloss.NewStyle().Foreground(colorYellow).Bold(true),
		Error:   lipgloss.NewStyle().Foreground(colorRed).Bold(true),

		Header:    lipgloss.NewStyle().Foreground(colorCyan).Bold(true),
		HeaderBar: lipgloss.NewStyle().Background(colorHeaderBg).Padding(0, 1),
		Divider:   lipgloss.NewStyle().Foreground(colorMuted),

		Selected:    lipgloss.NewStyle().Background(colorSelected).Foreground(colorFg).Padding(0, 1),
		SearchMatch: lipgloss.NewStyle().Background(colorSearchBg).Foreground(colorFg).Padding(0, 1),
		SearchFocus: lipgloss.NewStyle().Background(colorSelected).Foreground(colorYellow).Bold(true).Padding(0, 1),
		Highlight:   lipgloss.NewStyle().Background(colorYellow).Foreground(colorBg).Bold(true),
		RowNormal:   lipgloss.NewStyle().Foreground(colorFg).Padding(0, 1),

		ModalBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorAccent).
			Background(colorHeaderBg).
			Padding(1, 2),
		ModalTitle: lipgloss.NewStyle().
			Foreground(colorAccent).
			Bold(true).
			MarginBottom(1),
		ModalSection: lipgloss.NewStyle().
			Foreground(colorCyan).
			Bold(true),
		ModalItem: lipgloss.NewStyle().
			Foreground(colorFg),
		ModalSelected: lipgloss.NewStyle().
			Foreground(colorAccent).
			Bold(true),
		ModalFooter: lipgloss.NewStyle().
			Foreground(colorMuted).
			MarginTop(1),

		TitleBar: lipgloss.NewStyle().
			Background(colorHeaderBg).
			Foreground(colorFg).
			Padding(0, 1),
		StatusBar: lipgloss.NewStyle().
			Background(colorHeaderBg).
			Foreground(colorMuted).
			Padding(0, 1),
		KeyBar: lipgloss.NewStyle().
			Background(colorHeaderBg).
			Foreground(colorMuted).
			Padding(0, 1),
		KeyName: lipgloss.NewStyle().
			Foreground(colorAccent).
			Bold(true),

		FollowOn:  lipgloss.NewStyle().Foreground(colorGreen).Bold(true),
		FollowOff: lipgloss.NewStyle().Foreground(colorYellow).Bold(true),

		MsgInfo: lipgloss.NewStyle().Foreground(colorCyan),
		MsgErr:  lipgloss.NewStyle().Foreground(colorRed).Bold(true),
	}
}

var theme = DefaultTheme()

// ResolveCellStyle returns a lipgloss.Style for a cell value based on
// the column's semantic style or its custom color map.
func (t Theme) ResolveCellStyle(col record.Column, val string) lipgloss.Style {
	trimmed := strings.TrimSpace(val)

	// 1. Custom colors map in profile (e.g. colors: { ERROR: red, WARN: yellow })
	if len(col.Colors) > 0 {
		upperVal := strings.ToUpper(trimmed)
		if colorName, ok := col.Colors[upperVal]; ok {
			return t.colorByName(colorName)
		}
		if colorName, ok := col.Colors[trimmed]; ok {
			return t.colorByName(colorName)
		}
	}

	// 2. Built-in semantic styles
	switch strings.ToLower(col.Style) {
	case "timestamp", "time":
		return t.Muted
	case "level":
		return t.LevelStyle(val)
	case "identifier", "module", "task", "cpu":
		return lipgloss.NewStyle().Foreground(colorPurple)
	case "primary", "message", "text":
		return t.Content
	case "error":
		return t.Error
	case "warning", "warn":
		return t.Warning
	case "success":
		return t.Success
	case "muted":
		return t.Muted
	default:
		// Fallback heuristics: check column name
		if strings.EqualFold(col.Field, "level") {
			return t.LevelStyle(val)
		}
		if strings.EqualFold(col.Field, "time") || strings.EqualFold(col.Field, "timestamp") || strings.EqualFold(col.Field, "_ts") {
			return t.Muted
		}
		if strings.EqualFold(col.Field, "module") || strings.EqualFold(col.Field, "task") || strings.EqualFold(col.Field, "cpu") {
			return lipgloss.NewStyle().Foreground(colorPurple)
		}
		return t.Content
	}
}

// LevelStyle automatically colors standard log levels (ERROR, WARN, INFO, DEBUG, TRACE).
func (t Theme) LevelStyle(val string) lipgloss.Style {
	u := strings.ToUpper(strings.TrimSpace(val))
	switch {
	case strings.Contains(u, "ERR") || strings.Contains(u, "FATAL") || strings.Contains(u, "CRIT"):
		return t.Error
	case strings.Contains(u, "WARN"):
		return t.Warning
	case strings.Contains(u, "INFO"):
		return t.Accent
	case strings.Contains(u, "DEBUG") || strings.Contains(u, "TRACE"):
		return t.Muted
	default:
		return t.Content
	}
}

func (t Theme) colorByName(name string) lipgloss.Style {
	switch strings.ToLower(name) {
	case "red", "error":
		return t.Error
	case "yellow", "warn", "warning":
		return t.Warning
	case "green", "success":
		return t.Success
	case "blue", "cyan", "accent":
		return t.Accent
	case "purple", "magenta", "identifier":
		return lipgloss.NewStyle().Foreground(colorPurple)
	case "gray", "grey", "muted":
		return t.Muted
	case "white", "normal", "content", "primary":
		return t.Content
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color(name))
	}
}
