package tui

import (
	"strings"

	"github.com/brenoniehues/oh-my-logs/internal/record"
	"github.com/charmbracelet/lipgloss"
)

// Palette defines the refined color values across the theme.
var (
	colorBg         = lipgloss.Color("#12131a") // dark slate for modals
	colorSelected   = lipgloss.Color("#1e293b") // focused selection / row highlight
	colorSearchBg   = lipgloss.Color("#1e2238") // non-focused search match row
	colorBookmarkBg = lipgloss.Color("#282012") // faded golden row background for bookmarked rows

	colorFg      = lipgloss.Color("#e2e8f0") // crisp white/slate (clean log text)
	colorHeader  = lipgloss.Color("#f8fafc") // bright white for column headers
	colorMuted   = lipgloss.Color("#64748b") // slate gray for secondary/separators
	colorDivider = lipgloss.Color("#334155") // subtle divider lines

	colorAccent = lipgloss.Color("#60a5fa") // clean royal blue / accent
	colorCyan   = lipgloss.Color("#38bdf8") // sky blue / info
	colorPurple = lipgloss.Color("#c084fc") // light purple / identifier / module
	colorGreen  = lipgloss.Color("#4ade80") // success / connected
	colorYellow = lipgloss.Color("#facc15") // warning / search highlight
	colorRed    = lipgloss.Color("#f87171") // coral red / error
	colorTxBg   = lipgloss.Color("#2a1215") // subtle dark red / maple row background for TX commands
	colorMaple  = lipgloss.Color("#ea580c") // warm vibrant maple red-orange for TX badge and text
)

// Theme encapsulates all semantic visual styles.
type Theme struct {
	// Hierarchy
	Primary   lipgloss.Style // application name / active state (bold accent)
	Secondary lipgloss.Style // metadata values (clean soft white)
	Content   lipgloss.Style // actual log text
	Muted     lipgloss.Style // shortcuts / statistics / separators (gray)

	// Feedback / Status
	Accent  lipgloss.Style // blue/cyan
	Success lipgloss.Style // green
	Warning lipgloss.Style // yellow
	Error   lipgloss.Style // red

	// Table & Selection
	Header      lipgloss.Style // table header column text
	Divider     lipgloss.Style // horizontal divider lines
	Selected    lipgloss.Style // cursor / active selection
	SearchMatch lipgloss.Style // search match row
	SearchFocus lipgloss.Style // active focused search match
	Highlight   lipgloss.Style // matching substring highlight
	BookmarkRow lipgloss.Style // bookmarked row background highlight
	RowNormal   lipgloss.Style // normal table row

	// Modals & Panels
	ModalBox      lipgloss.Style // rounded border modal container
	ModalTitle    lipgloss.Style // title inside modal
	ModalSection  lipgloss.Style // section header (e.g. Baud rate, Serial Port)
	ModalItem     lipgloss.Style // normal item in modal
	ModalSelected lipgloss.Style // selected item in modal (with ›)
	ModalFooter   lipgloss.Style // help text at bottom of modal

	// Status & Footer
	TitleBar      lipgloss.Style
	TitleBg       lipgloss.Color
	TitleAppBadge lipgloss.Style // vibrant inverted pill " OH MY LOGS "
	TitleApp      lipgloss.Style // bold accent "Oh My Logs"
	TitleLabel    lipgloss.Style // muted gray "Port: "
	TitleValue    lipgloss.Style // soft white "/dev/..."
	TitleSep      lipgloss.Style // subtle separator "│"
	TitleConnOn   lipgloss.Style // green "● Connected"
	TitleConnOff  lipgloss.Style // muted "○ Disconnected"
	TitleConnErr  lipgloss.Style // red "⚠ ..."

	StatusBar   lipgloss.Style
	KeyBar      lipgloss.Style
	KeyName     lipgloss.Style
	FollowOn    lipgloss.Style
	FollowOff   lipgloss.Style
	MsgInfo     lipgloss.Style
	MsgErr      lipgloss.Style
	TabActive   lipgloss.Style
	TabInactive lipgloss.Style
}

// DefaultTheme returns the refined technical developer theme.
func DefaultTheme() Theme {
	colorTitleBg := lipgloss.Color("#181f2f") // deep slate navy background
	colorPillBg := lipgloss.Color("#3b82f6")  // vibrant royal blue badge
	colorPillFg := lipgloss.Color("#ffffff")  // bright white text

	return Theme{
		Primary:   lipgloss.NewStyle().Foreground(colorAccent).Bold(true),
		Secondary: lipgloss.NewStyle().Foreground(colorFg),
		Content:   lipgloss.NewStyle().Foreground(colorFg),
		Muted:     lipgloss.NewStyle().Foreground(colorMuted),

		Accent:  lipgloss.NewStyle().Foreground(colorAccent),
		Success: lipgloss.NewStyle().Foreground(colorGreen).Bold(true),
		Warning: lipgloss.NewStyle().Foreground(colorYellow).Bold(true),
		Error:   lipgloss.NewStyle().Foreground(colorRed).Bold(true),

		Header:  lipgloss.NewStyle().Foreground(colorHeader).Bold(true),
		Divider: lipgloss.NewStyle().Foreground(colorDivider),

		Selected:    lipgloss.NewStyle().Background(colorSelected).Foreground(colorFg),
		SearchMatch: lipgloss.NewStyle().Background(colorSearchBg).Foreground(colorFg),
		SearchFocus: lipgloss.NewStyle().Background(colorSelected).Foreground(colorYellow).Bold(true),
		Highlight:   lipgloss.NewStyle().Background(colorYellow).Foreground(lipgloss.Color("#000000")).Bold(true),
		BookmarkRow: lipgloss.NewStyle().Background(colorBookmarkBg).Foreground(colorFg),
		RowNormal:   lipgloss.NewStyle().Foreground(colorFg),

		ModalBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorAccent).
			Padding(1, 2),
		ModalTitle: lipgloss.NewStyle().
			Foreground(colorHeader).
			Bold(true),
		ModalSection: lipgloss.NewStyle().
			Foreground(colorAccent).
			Bold(true),
		ModalItem: lipgloss.NewStyle().
			Foreground(colorFg),
		ModalSelected: lipgloss.NewStyle().
			Foreground(colorAccent).
			Bold(true),
		ModalFooter: lipgloss.NewStyle().
			Foreground(colorMuted),

		TitleBar: lipgloss.NewStyle().
			Background(colorTitleBg).
			Foreground(colorFg),
		TitleBg: colorTitleBg,
		TitleAppBadge: lipgloss.NewStyle().
			Background(colorPillBg).
			Foreground(colorPillFg).
			Bold(true).
			Padding(0, 1),
		TitleApp: lipgloss.NewStyle().
			Background(colorTitleBg).
			Foreground(colorAccent).
			Bold(true),
		TitleLabel: lipgloss.NewStyle().
			Background(colorTitleBg).
			Foreground(colorMuted),
		TitleValue: lipgloss.NewStyle().
			Background(colorTitleBg).
			Foreground(colorFg),
		TitleSep: lipgloss.NewStyle().
			Background(colorTitleBg).
			Foreground(colorDivider),
		TitleConnOn: lipgloss.NewStyle().
			Background(colorTitleBg).
			Foreground(colorGreen).
			Bold(true),
		TitleConnOff: lipgloss.NewStyle().
			Background(colorTitleBg).
			Foreground(colorMuted),
		TitleConnErr: lipgloss.NewStyle().
			Background(colorTitleBg).
			Foreground(colorRed).
			Bold(true),

		StatusBar: lipgloss.NewStyle().
			Foreground(colorMuted),
		KeyBar: lipgloss.NewStyle().
			Foreground(colorMuted),
		KeyName: lipgloss.NewStyle().
			Foreground(colorCyan).
			Bold(true),

		FollowOn:  lipgloss.NewStyle().Foreground(colorGreen).Bold(true),
		FollowOff: lipgloss.NewStyle().Foreground(colorYellow).Bold(true),
		MsgInfo:   lipgloss.NewStyle().Foreground(colorMuted),
		MsgErr:    lipgloss.NewStyle().Foreground(colorRed).Bold(true),

		TabActive: lipgloss.NewStyle().
			Foreground(colorHeader).
			Background(lipgloss.Color("#2563eb")).
			Bold(true).
			Padding(0, 1),
		TabInactive: lipgloss.NewStyle().
			Foreground(colorMuted).
			Background(lipgloss.Color("#1e293b")).
			Padding(0, 1),
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
	case "timestamp", "time", "uptime":
		return t.Muted
	case "level":
		return t.LevelStyle(val)
	case "identifier", "module", "task", "cpu", "code":
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
		if strings.EqualFold(col.Field, "time") || strings.EqualFold(col.Field, "timestamp") || strings.EqualFold(col.Field, "_ts") || strings.EqualFold(col.Field, "uptime") {
			return t.Muted
		}
		if strings.EqualFold(col.Field, "module") || strings.EqualFold(col.Field, "task") || strings.EqualFold(col.Field, "cpu") || strings.EqualFold(col.Field, "code") {
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
	case strings.Contains(u, "WARN") || strings.Contains(u, "WRN"):
		return t.Warning
	case strings.Contains(u, "INFO") || strings.Contains(u, "INF"):
		return lipgloss.NewStyle().Foreground(colorCyan)
	case strings.Contains(u, "DEBUG") || strings.Contains(u, "DBG") || strings.Contains(u, "TRACE"):
		return t.Muted
	case u == "TX" || u == "OUT" || strings.HasPrefix(u, "TX"):
		return lipgloss.NewStyle().Foreground(colorMaple).Bold(true)
	default:
		return t.Content
	}
}

func (t Theme) colorByName(name string) lipgloss.Style {
	switch strings.ToLower(name) {
	case "maple", "orange", "tx":
		return lipgloss.NewStyle().Foreground(colorMaple)
	case "red", "error":
		return t.Error
	case "yellow", "warn", "warning":
		return t.Warning
	case "green", "success":
		return t.Success
	case "blue", "cyan", "accent":
		return lipgloss.NewStyle().Foreground(colorCyan)
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
