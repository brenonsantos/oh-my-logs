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
	Quit            key.Binding
}


// defaultKeyMap returns the standard key bindings.
func defaultKeyMap() keyMap {
	return keyMap{
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
			key.WithKeys("pgup", "ctrl+u", "b"),
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
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
	}
}

