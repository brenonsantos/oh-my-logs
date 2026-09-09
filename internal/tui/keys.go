package tui

import "github.com/charmbracelet/bubbles/key"

// keyMap holds all keyboard bindings for the application.
type keyMap struct {
	Search          key.Binding
	Filter          key.Binding
	Clear           key.Binding
	Pause           key.Binding
	ScrollUp        key.Binding
	ScrollDown      key.Binding
	PageUp          key.Binding
	PageDown        key.Binding
	GoToBottom      key.Binding
	GoToTop         key.Binding
	Port            key.Binding
	Reconnect       key.Binding
	SaveLog         key.Binding
	NextMatch       key.Binding
	PrevMatch       key.Binding
	Confirm         key.Binding
	Cancel          key.Binding
	ToggleTimestamp key.Binding
	ProfileSwitch   key.Binding
	NextTab         key.Binding
	PrevTab         key.Binding
	NewTab          key.Binding
	CloseTab        key.Binding
	ToggleBookmark  key.Binding
	NextBookmark    key.Binding
	PrevBookmark    key.Binding
	BookmarksOnly   key.Binding
	SplitVertical   key.Binding
	SplitHorizontal key.Binding
	SwitchPane      key.Binding
	ToggleSyncScroll key.Binding
	CopyRow         key.Binding
	CopyRaw         key.Binding
	SelectUp        key.Binding
	SelectDown      key.Binding
	Help            key.Binding
	Game            key.Binding
	Quit            key.Binding
}


// defaultKeyMap returns the standard key bindings.
func defaultKeyMap() keyMap {
	return keyMap{
		CopyRow: key.NewBinding(
			key.WithKeys("y"),
			key.WithHelp("y", "copy row"),
		),
		CopyRaw: key.NewBinding(
			key.WithKeys("Y"),
			key.WithHelp("Y", "copy raw"),
		),
		SelectUp: key.NewBinding(
			key.WithKeys("shift+up", "K"),
			key.WithHelp("Shift+↑/K", "expand selection up"),
		),
		SelectDown: key.NewBinding(
			key.WithKeys("shift+down", "J"),
			key.WithHelp("Shift+↓/J", "expand selection down"),
		),
		Search: key.NewBinding(
			key.WithKeys("ctrl+f"),
			key.WithHelp("Ctrl+F", "search"),
		),
		Filter: key.NewBinding(
			key.WithKeys("f"),
			key.WithHelp("f", "filter"),
		),
		Clear: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "clear"),
		),
		Pause: key.NewBinding(
			key.WithKeys(" "),
			key.WithHelp("Space", "pause/resume"),
		),
		ScrollUp: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "scroll up"),
		),
		ScrollDown: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "scroll down"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup", "ctrl+u"),
			key.WithHelp("PgUp/Ctrl+U", "page up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("pgdown", "ctrl+d"),
			key.WithHelp("PgDn/Ctrl+D", "page down"),
		),
		GoToTop: key.NewBinding(
			key.WithKeys("g", "home"),
			key.WithHelp("g/Home", "go to top"),
		),
		GoToBottom: key.NewBinding(
			key.WithKeys("G", "end"),
			key.WithHelp("G/End", "go to bottom (follow)"),
		),

		Port: key.NewBinding(
			key.WithKeys("p"),
			key.WithHelp("p", "serial port"),
		),
		Reconnect: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "reconnect"),
		),
		SaveLog: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "save log"),
		),
		NextMatch: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "next match"),
		),
		PrevMatch: key.NewBinding(
			key.WithKeys("N"),
			key.WithHelp("N", "prev match"),
		),
		Confirm: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("Enter", "confirm"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("Esc", "cancel"),
		),
		ToggleTimestamp: key.NewBinding(
			key.WithKeys("t"),
			key.WithHelp("t", "toggle timestamp"),
		),
		ProfileSwitch: key.NewBinding(
			key.WithKeys("P"),
			key.WithHelp("P", "profile"),
		),
		NextTab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("Tab", "next tab"),
		),
		PrevTab: key.NewBinding(
			key.WithKeys("shift+tab", "backtab"),
			key.WithHelp("Shift+Tab", "prev tab"),
		),
		NewTab: key.NewBinding(
			key.WithKeys("ctrl+t"),
			key.WithHelp("Ctrl+T", "new tab"),
		),
		CloseTab: key.NewBinding(
			key.WithKeys("ctrl+w"),
			key.WithHelp("Ctrl+W", "close tab"),
		),
		ToggleBookmark: key.NewBinding(
			key.WithKeys("b", "m"),
			key.WithHelp("b/m", "bookmark row"),
		),
		NextBookmark: key.NewBinding(
			key.WithKeys("]"),
			key.WithHelp("]", "next bookmark"),
		),
		PrevBookmark: key.NewBinding(
			key.WithKeys("["),
			key.WithHelp("[", "prev bookmark"),
		),
		BookmarksOnly: key.NewBinding(
			key.WithKeys("B"),
			key.WithHelp("B", "show bookmarked only"),
		),
		SplitVertical: key.NewBinding(
			key.WithKeys("|"),
			key.WithHelp("|", "split vertical"),
		),
		SplitHorizontal: key.NewBinding(
			key.WithKeys("_"),
			key.WithHelp("_", "split horizontal"),
		),
		SwitchPane: key.NewBinding(
			key.WithKeys("w"),
			key.WithHelp("w", "switch pane"),
		),
		ToggleSyncScroll: key.NewBinding(
			key.WithKeys("S"),
			key.WithHelp("S", "toggle sync scroll"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Game: key.NewBinding(
			key.WithKeys("ctrl+g"),
			key.WithHelp("^G", "mini-game"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
	}
}

