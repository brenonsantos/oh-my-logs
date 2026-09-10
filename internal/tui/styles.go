package tui

import (
	"strings"

	"github.com/brenoniehues/oh-my-logs/internal/record"
	"github.com/brenoniehues/oh-my-logs/internal/timing"
	"github.com/charmbracelet/lipgloss"
)

// Palette defines the raw color values for a cohesive TUI theme.
type Palette struct {
	Name        string
	Description string

	Bg         lipgloss.Color
	TitleBg    lipgloss.Color
	Selected   lipgloss.Color
	SearchBg   lipgloss.Color
	BookmarkBg lipgloss.Color

	Fg      lipgloss.Color
	Header  lipgloss.Color
	Muted   lipgloss.Color
	Divider lipgloss.Color

	Accent lipgloss.Color
	Cyan   lipgloss.Color
	Purple lipgloss.Color
	Green  lipgloss.Color
	Yellow lipgloss.Color
	Red    lipgloss.Color
	TxBg   lipgloss.Color
	Maple  lipgloss.Color

	PillBg        lipgloss.Color
	PillFg        lipgloss.Color
	TabActiveBg   lipgloss.Color
	TabInactiveBg lipgloss.Color
}

// Pre-defined curated theme palettes.
var (
	PaletteDarkSlate = Palette{
		Name:          "Dark Slate",
		Description:   "Modern slate and royal blue (Default)",
		Bg:            lipgloss.Color("#12131a"),
		TitleBg:       lipgloss.Color("#181f2f"),
		Selected:      lipgloss.Color("#1e293b"),
		SearchBg:      lipgloss.Color("#1e2238"),
		BookmarkBg:    lipgloss.Color("#282012"),
		Fg:            lipgloss.Color("#e2e8f0"),
		Header:        lipgloss.Color("#f8fafc"),
		Muted:         lipgloss.Color("#64748b"),
		Divider:       lipgloss.Color("#334155"),
		Accent:        lipgloss.Color("#60a5fa"),
		Cyan:          lipgloss.Color("#38bdf8"),
		Purple:        lipgloss.Color("#c084fc"),
		Green:         lipgloss.Color("#4ade80"),
		Yellow:        lipgloss.Color("#facc15"),
		Red:           lipgloss.Color("#f87171"),
		TxBg:          lipgloss.Color("#2a1215"),
		Maple:         lipgloss.Color("#ea580c"),
		PillBg:        lipgloss.Color("#3b82f6"),
		PillFg:        lipgloss.Color("#ffffff"),
		TabActiveBg:   lipgloss.Color("#2563eb"),
		TabInactiveBg: lipgloss.Color("#1e293b"),
	}

	PaletteMonokai = Palette{
		Name:          "Monokai",
		Description:   "Retro code-editor palette with vibrant magenta and lime",
		Bg:            lipgloss.Color("#272822"),
		TitleBg:       lipgloss.Color("#1e1f1c"),
		Selected:      lipgloss.Color("#3e3d32"),
		SearchBg:      lipgloss.Color("#49483e"),
		BookmarkBg:    lipgloss.Color("#3e3520"),
		Fg:            lipgloss.Color("#f8f8f2"),
		Header:        lipgloss.Color("#ffffff"),
		Muted:         lipgloss.Color("#75715e"),
		Divider:       lipgloss.Color("#49483e"),
		Accent:        lipgloss.Color("#f92672"),
		Cyan:          lipgloss.Color("#66d9ef"),
		Purple:        lipgloss.Color("#ae81ff"),
		Green:         lipgloss.Color("#a6e22e"),
		Yellow:        lipgloss.Color("#e6db74"),
		Red:           lipgloss.Color("#f92672"),
		TxBg:          lipgloss.Color("#382020"),
		Maple:         lipgloss.Color("#fd971f"),
		PillBg:        lipgloss.Color("#f92672"),
		PillFg:        lipgloss.Color("#272822"),
		TabActiveBg:   lipgloss.Color("#f92672"),
		TabInactiveBg: lipgloss.Color("#3e3d32"),
	}

	PaletteNord = Palette{
		Name:          "Nord",
		Description:   "Cool arctic blue, frost slate, and aurora tones",
		Bg:            lipgloss.Color("#2e3440"),
		TitleBg:       lipgloss.Color("#242933"),
		Selected:      lipgloss.Color("#3b4252"),
		SearchBg:      lipgloss.Color("#434c5e"),
		BookmarkBg:    lipgloss.Color("#3d372e"),
		Fg:            lipgloss.Color("#eceff4"),
		Header:        lipgloss.Color("#ffffff"),
		Muted:         lipgloss.Color("#7b88a1"),
		Divider:       lipgloss.Color("#4c566a"),
		Accent:        lipgloss.Color("#88c0d0"),
		Cyan:          lipgloss.Color("#8fbcbb"),
		Purple:        lipgloss.Color("#b48ead"),
		Green:         lipgloss.Color("#a3be8c"),
		Yellow:        lipgloss.Color("#ebcb8b"),
		Red:           lipgloss.Color("#bf616a"),
		TxBg:          lipgloss.Color("#3b292e"),
		Maple:         lipgloss.Color("#d08770"),
		PillBg:        lipgloss.Color("#88c0d0"),
		PillFg:        lipgloss.Color("#242933"),
		TabActiveBg:   lipgloss.Color("#5e81ac"),
		TabInactiveBg: lipgloss.Color("#3b4252"),
	}

	PaletteGruvbox = Palette{
		Name:          "Gruvbox",
		Description:   "Warm retro groove with vintage amber and earth tones",
		Bg:            lipgloss.Color("#282828"),
		TitleBg:       lipgloss.Color("#1d2021"),
		Selected:      lipgloss.Color("#3c3836"),
		SearchBg:      lipgloss.Color("#504945"),
		BookmarkBg:    lipgloss.Color("#4a3820"),
		Fg:            lipgloss.Color("#ebdbb2"),
		Header:        lipgloss.Color("#fbf1c7"),
		Muted:         lipgloss.Color("#928374"),
		Divider:       lipgloss.Color("#504945"),
		Accent:        lipgloss.Color("#fe8019"),
		Cyan:          lipgloss.Color("#8ec07c"),
		Purple:        lipgloss.Color("#d3869b"),
		Green:         lipgloss.Color("#b8bb26"),
		Yellow:        lipgloss.Color("#fabd2f"),
		Red:           lipgloss.Color("#fb4934"),
		TxBg:          lipgloss.Color("#3c241c"),
		Maple:         lipgloss.Color("#fe8019"),
		PillBg:        lipgloss.Color("#fe8019"),
		PillFg:        lipgloss.Color("#1d2021"),
		TabActiveBg:   lipgloss.Color("#d65d0e"),
		TabInactiveBg: lipgloss.Color("#3c3836"),
	}

	PaletteTokyoNight = Palette{
		Name:          "Tokyo Night",
		Description:   "Deep neon cyber aesthetic inspired by Tokyo evening lights",
		Bg:            lipgloss.Color("#1a1b26"),
		TitleBg:       lipgloss.Color("#16161e"),
		Selected:      lipgloss.Color("#283457"),
		SearchBg:      lipgloss.Color("#2f354d"),
		BookmarkBg:    lipgloss.Color("#383020"),
		Fg:            lipgloss.Color("#c0caf5"),
		Header:        lipgloss.Color("#ffffff"),
		Muted:         lipgloss.Color("#565f89"),
		Divider:       lipgloss.Color("#3b4261"),
		Accent:        lipgloss.Color("#7aa2f7"),
		Cyan:          lipgloss.Color("#7dcfff"),
		Purple:        lipgloss.Color("#bb9af7"),
		Green:         lipgloss.Color("#9ece6a"),
		Yellow:        lipgloss.Color("#e0af68"),
		Red:           lipgloss.Color("#f7768e"),
		TxBg:          lipgloss.Color("#341f2a"),
		Maple:         lipgloss.Color("#ff9e64"),
		PillBg:        lipgloss.Color("#7aa2f7"),
		PillFg:        lipgloss.Color("#16161e"),
		TabActiveBg:   lipgloss.Color("#3d59a1"),
		TabInactiveBg: lipgloss.Color("#24283b"),
	}

	PaletteHighContrast = Palette{
		Name:          "High Contrast",
		Description:   "Pure black background with ultra-vivid colors",
		Bg:            lipgloss.Color("#000000"),
		TitleBg:       lipgloss.Color("#121212"),
		Selected:      lipgloss.Color("#242424"),
		SearchBg:      lipgloss.Color("#1a2634"),
		BookmarkBg:    lipgloss.Color("#332800"),
		Fg:            lipgloss.Color("#ffffff"),
		Header:        lipgloss.Color("#ffffff"),
		Muted:         lipgloss.Color("#a0a0a0"),
		Divider:       lipgloss.Color("#555555"),
		Accent:        lipgloss.Color("#00d8ff"),
		Cyan:          lipgloss.Color("#00ffff"),
		Purple:        lipgloss.Color("#d888ff"),
		Green:         lipgloss.Color("#00ff66"),
		Yellow:        lipgloss.Color("#ffff00"),
		Red:           lipgloss.Color("#ff4444"),
		TxBg:          lipgloss.Color("#2b1414"),
		Maple:         lipgloss.Color("#ff6600"),
		PillBg:        lipgloss.Color("#ffffff"),
		PillFg:        lipgloss.Color("#000000"),
		TabActiveBg:   lipgloss.Color("#00d8ff"),
		TabInactiveBg: lipgloss.Color("#333333"),
	}
)

var curatedPalettes = []Palette{
	PaletteDarkSlate,
	PaletteMonokai,
	PaletteNord,
	PaletteGruvbox,
	PaletteTokyoNight,
	PaletteHighContrast,
}

// Current active palette color variables (for backwards compatibility and direct cell styling)
var (
	colorBg         = PaletteDarkSlate.Bg
	colorSelected   = PaletteDarkSlate.Selected
	colorSearchBg   = PaletteDarkSlate.SearchBg
	colorBookmarkBg = PaletteDarkSlate.BookmarkBg

	colorFg      = PaletteDarkSlate.Fg
	colorHeader  = PaletteDarkSlate.Header
	colorMuted   = PaletteDarkSlate.Muted
	colorDivider = PaletteDarkSlate.Divider

	colorAccent = PaletteDarkSlate.Accent
	colorCyan   = PaletteDarkSlate.Cyan
	colorPurple = PaletteDarkSlate.Purple
	colorGreen  = PaletteDarkSlate.Green
	colorYellow = PaletteDarkSlate.Yellow
	colorRed    = PaletteDarkSlate.Red
	colorTxBg   = PaletteDarkSlate.TxBg
	colorMaple  = PaletteDarkSlate.Maple
)

var currentThemeName = PaletteDarkSlate.Name
var theme = BuildTheme(PaletteDarkSlate)

// Theme encapsulates all semantic visual styles.
type Theme struct {
	Name    string
	Palette Palette

	// Hierarchy
	Primary   lipgloss.Style // application name / active state (bold accent)
	Secondary lipgloss.Style // metadata values (clean soft white)
	Content   lipgloss.Style // actual log text
	Muted     lipgloss.Style // shortcuts / statistics / separators (gray)

	// Feedback / Status
	Accent  lipgloss.Style // accent
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

// BuildTheme constructs all visual lipgloss styles from a Palette.
func BuildTheme(p Palette) Theme {
	return Theme{
		Name:    p.Name,
		Palette: p,

		Primary:   lipgloss.NewStyle().Foreground(p.Accent).Bold(true),
		Secondary: lipgloss.NewStyle().Foreground(p.Fg),
		Content:   lipgloss.NewStyle().Foreground(p.Fg),
		Muted:     lipgloss.NewStyle().Foreground(p.Muted),

		Accent:  lipgloss.NewStyle().Foreground(p.Accent),
		Success: lipgloss.NewStyle().Foreground(p.Green).Bold(true),
		Warning: lipgloss.NewStyle().Foreground(p.Yellow).Bold(true),
		Error:   lipgloss.NewStyle().Foreground(p.Red).Bold(true),

		Header:  lipgloss.NewStyle().Foreground(p.Header).Bold(true),
		Divider: lipgloss.NewStyle().Foreground(p.Divider),

		Selected:    lipgloss.NewStyle().Background(p.Selected).Foreground(p.Fg),
		SearchMatch: lipgloss.NewStyle().Background(p.SearchBg).Foreground(p.Fg),
		SearchFocus: lipgloss.NewStyle().Background(p.Selected).Foreground(p.Yellow).Bold(true),
		Highlight:   lipgloss.NewStyle().Background(p.Yellow).Foreground(lipgloss.Color("#000000")).Bold(true),
		BookmarkRow: lipgloss.NewStyle().Background(p.BookmarkBg).Foreground(p.Fg),
		RowNormal:   lipgloss.NewStyle().Foreground(p.Fg),

		ModalBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(p.Accent).
			Padding(1, 2),
		ModalTitle: lipgloss.NewStyle().
			Foreground(p.Header).
			Bold(true),
		ModalSection: lipgloss.NewStyle().
			Foreground(p.Accent).
			Bold(true),
		ModalItem: lipgloss.NewStyle().
			Foreground(p.Fg),
		ModalSelected: lipgloss.NewStyle().
			Foreground(p.Accent).
			Bold(true),
		ModalFooter: lipgloss.NewStyle().
			Foreground(p.Muted),

		TitleBar: lipgloss.NewStyle().
			Background(p.TitleBg).
			Foreground(p.Fg),
		TitleBg: p.TitleBg,
		TitleAppBadge: lipgloss.NewStyle().
			Background(p.PillBg).
			Foreground(p.PillFg).
			Bold(true).
			Padding(0, 1),
		TitleApp: lipgloss.NewStyle().
			Background(p.TitleBg).
			Foreground(p.Accent).
			Bold(true),
		TitleLabel: lipgloss.NewStyle().
			Background(p.TitleBg).
			Foreground(p.Muted),
		TitleValue: lipgloss.NewStyle().
			Background(p.TitleBg).
			Foreground(p.Fg),
		TitleSep: lipgloss.NewStyle().
			Background(p.TitleBg).
			Foreground(p.Divider),
		TitleConnOn: lipgloss.NewStyle().
			Background(p.TitleBg).
			Foreground(p.Green).
			Bold(true),
		TitleConnOff: lipgloss.NewStyle().
			Background(p.TitleBg).
			Foreground(p.Muted),
		TitleConnErr: lipgloss.NewStyle().
			Background(p.TitleBg).
			Foreground(p.Red).
			Bold(true),

		StatusBar: lipgloss.NewStyle().
			Foreground(p.Muted),
		KeyBar: lipgloss.NewStyle().
			Foreground(p.Muted),
		KeyName: lipgloss.NewStyle().
			Foreground(p.Cyan).
			Bold(true),

		FollowOn:  lipgloss.NewStyle().Foreground(p.Green).Bold(true),
		FollowOff: lipgloss.NewStyle().Foreground(p.Yellow).Bold(true),
		MsgInfo:   lipgloss.NewStyle().Foreground(p.Muted),
		MsgErr:    lipgloss.NewStyle().Foreground(p.Red).Bold(true),

		TabActive: lipgloss.NewStyle().
			Foreground(p.Header).
			Background(p.TabActiveBg).
			Bold(true).
			Padding(0, 1),
		TabInactive: lipgloss.NewStyle().
			Foreground(p.Muted).
			Background(p.TabInactiveBg).
			Padding(0, 1),
	}
}

func normalizeThemeName(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, "_", "")
	s = strings.ReplaceAll(s, " ", "")
	return s
}

// AvailableThemes returns the display names of all registered themes.
func AvailableThemes() []string {
	names := make([]string, len(curatedPalettes))
	for i, p := range curatedPalettes {
		names[i] = p.Name
	}
	return names
}

// FindPalette retrieves a palette by case-insensitive name or slug.
func FindPalette(name string) (Palette, bool) {
	norm := normalizeThemeName(name)
	for _, p := range curatedPalettes {
		if normalizeThemeName(p.Name) == norm {
			return p, true
		}
	}
	return PaletteDarkSlate, false
}

// SetCurrentTheme switches the active global theme by name and returns the matched name.
func SetCurrentTheme(name string) string {
	p, ok := FindPalette(name)
	if !ok {
		p = PaletteDarkSlate
	}

	colorBg = p.Bg
	colorSelected = p.Selected
	colorSearchBg = p.SearchBg
	colorBookmarkBg = p.BookmarkBg
	colorFg = p.Fg
	colorHeader = p.Header
	colorMuted = p.Muted
	colorDivider = p.Divider
	colorAccent = p.Accent
	colorCyan = p.Cyan
	colorPurple = p.Purple
	colorGreen = p.Green
	colorYellow = p.Yellow
	colorRed = p.Red
	colorTxBg = p.TxBg
	colorMaple = p.Maple

	currentThemeName = p.Name
	theme = BuildTheme(p)
	return p.Name
}

// CurrentThemeName returns the name of the currently active theme.
func CurrentThemeName() string {
	if currentThemeName == "" {
		return PaletteDarkSlate.Name
	}
	return currentThemeName
}

// DefaultTheme returns the refined technical developer theme.
func DefaultTheme() Theme {
	return BuildTheme(PaletteDarkSlate)
}

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
		purpleColor := t.Palette.Purple
		if purpleColor == "" {
			purpleColor = colorPurple
		}
		return lipgloss.NewStyle().Foreground(purpleColor)
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
			purpleColor := t.Palette.Purple
			if purpleColor == "" {
				purpleColor = colorPurple
			}
			return lipgloss.NewStyle().Foreground(purpleColor)
		}
		return t.Content
	}
}

// LevelStyle automatically colors standard log levels (ERROR, WARN, INFO, DEBUG, TRACE).
func (t Theme) LevelStyle(val string) lipgloss.Style {
	cyanColor := t.Palette.Cyan
	if cyanColor == "" {
		cyanColor = colorCyan
	}
	mapleColor := t.Palette.Maple
	if mapleColor == "" {
		mapleColor = colorMaple
	}

	u := strings.ToUpper(strings.TrimSpace(val))
	switch {
	case strings.Contains(u, "ERR") || strings.Contains(u, "FATAL") || strings.Contains(u, "CRIT"):
		return t.Error
	case strings.Contains(u, "WARN") || strings.Contains(u, "WRN"):
		return t.Warning
	case strings.Contains(u, "INFO") || strings.Contains(u, "INF"):
		return lipgloss.NewStyle().Foreground(cyanColor)
	case strings.Contains(u, "DEBUG") || strings.Contains(u, "DBG") || strings.Contains(u, "TRACE"):
		return t.Muted
	case u == "TX" || u == "OUT" || strings.HasPrefix(u, "TX"):
		return lipgloss.NewStyle().Foreground(mapleColor).Bold(true)
	default:
		return t.Content
	}
}

// DeltaStyle returns the visual style for an inter-log latency interval.
func (t Theme) DeltaStyle(level timing.Level) lipgloss.Style {
	cyanColor := t.Palette.Cyan
	if cyanColor == "" {
		cyanColor = colorCyan
	}
	mapleColor := t.Palette.Maple
	if mapleColor == "" {
		mapleColor = colorMaple
	}

	switch level {
	case timing.LevelBurst:
		return lipgloss.NewStyle().Foreground(cyanColor)
	case timing.LevelHiccup:
		return t.Warning
	case timing.LevelAlert:
		return lipgloss.NewStyle().Foreground(mapleColor).Bold(true)
	default:
		return t.Muted
	}
}

func (t Theme) colorByName(name string) lipgloss.Style {
	cyanColor := t.Palette.Cyan
	if cyanColor == "" {
		cyanColor = colorCyan
	}
	purpleColor := t.Palette.Purple
	if purpleColor == "" {
		purpleColor = colorPurple
	}
	mapleColor := t.Palette.Maple
	if mapleColor == "" {
		mapleColor = colorMaple
	}

	switch strings.ToLower(name) {
	case "maple", "orange", "tx":
		return lipgloss.NewStyle().Foreground(mapleColor)
	case "red", "error":
		return t.Error
	case "yellow", "warn", "warning":
		return t.Warning
	case "green", "success":
		return t.Success
	case "blue", "cyan", "accent":
		return lipgloss.NewStyle().Foreground(cyanColor)
	case "purple", "magenta", "identifier":
		return lipgloss.NewStyle().Foreground(purpleColor)
	case "gray", "grey", "muted":
		return t.Muted
	case "white", "normal", "content", "primary":
		return t.Content
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color(name))
	}
}
