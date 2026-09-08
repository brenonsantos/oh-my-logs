package tui

import (
	"github.com/brenoniehues/oh-my-logs/internal/config"
	"github.com/brenoniehues/oh-my-logs/internal/filter"
	"github.com/brenoniehues/oh-my-logs/internal/parser"
	"github.com/brenoniehues/oh-my-logs/internal/record"
	"github.com/brenoniehues/oh-my-logs/internal/serial"
	tea "github.com/charmbracelet/bubbletea"
)

// inputMode tracks which text input is currently active.
type inputMode int

const (
	modeNormal inputMode = iota
	modeSearch
	modeFilter
	modePortPicker
	modeProfilePicker
	modeHelp
)

// ProfileItem represents an entry in the profile switcher list.
type ProfileItem struct {
	Name string
	Path string // empty if built-in raw
}

// ConnState represents the serial connection status.
type ConnState int

const (
	ConnDisconnected ConnState = iota
	ConnConnected
	ConnError
)

// RecordMsg is a Bubble Tea message carrying a newly parsed record.
type RecordMsg record.Record

// ErrorMsg carries a non-fatal error to display in the TUI.
type ErrorMsg struct{ Err error }

// ConnStateMsg signals a connection state change.
type ConnStateMsg struct{ State ConnState; Detail string }

// Model is the top-level Bubble Tea model.
type Model struct {
	// Configuration
	keys    keyMap
	width   int
	height  int

	// Serial / source
	source       serial.Source
	serialCfg    serial.Config
	connState    ConnState
	connDetail   string
	reconnecting bool
	isFileSource bool

	// Profile & parser
	profile  *parser.Profile
	parser   parser.Parser
	columns  []record.Column

	// Data
	buffer  *record.Buffer
	visible []record.Record // filtered view

	// Filter
	activeFilter *filter.Filter
	filterInput  string

	// Search
	searchInput   string
	searchMatches []int // indices into visible
	searchCursor  int

	// Scroll / follow
	scrollOffset int // index of the top visible row
	follow       bool
	paused       bool

	// UI state
	mode    inputMode
	message string // ephemeral status/error message

	// Port & Baud picker
	portList          []string
	portCursor        int
	baudList          []int
	baudCursor        int
	portPickerSection int // 0 = port, 1 = baud

	// Profile picker
	profileList   []ProfileItem
	profileCursor int
	appConfig     *config.AppConfig
	settings      *config.Settings

	// Timestamp display & settings (toggle visibility at runtime with 't')
	showTimestamp bool   // whether the timestamp column is displayed in the UI
	tsField       string // field name for the arrival timestamp (e.g. "_ts" or "time")
	tsFormat      string // Go time layout for the timestamp

	// Viewport dimensions (computed on resize)
	tableHeight  int
	sidebarWidth int
}

// New creates a new Model with sensible defaults.
func New(
	cfg serial.Config,
	profile *parser.Profile,
	p parser.Parser,
	buf *record.Buffer,
	src serial.Source,
	appCfg *config.AppConfig,
) Model {
	cols := []record.Column{{Field: "message", Title: "Message", Width: 0}}
	if profile != nil {
		cols = profile.ToColumns()
	}

	var savedSettings *config.Settings
	if appCfg != nil {
		savedSettings, _ = appCfg.LoadSettings()
	}

	// Resolve timestamp settings.
	tsField := "_ts"
	tsFormat := "15:04:05.000"
	initTS := false

	if profile != nil {
		if profile.Ingest.Timestamp.Enabled {
			initTS = true
			tsField = profile.Ingest.Timestamp.TimestampField()
			tsFormat = profile.Ingest.Timestamp.TimestampFormat()
		} else {
			for _, col := range profile.Columns {
				if col.Style == "timestamp" || col.Field == "time" || col.Field == "timestamp" {
					initTS = true
					tsField = col.Field
					break
				}
			}
		}
	} else if savedSettings != nil && savedSettings.ShowTimestamp {
		initTS = true
	}

	bauds := serial.CommonBaudRates()
	baudIdx := 4 // default to 115200 if found
	for i, b := range bauds {
		if b == cfg.Baud {
			baudIdx = i
			break
		}
	}

	isFile := false
	if _, ok := src.(*serial.FileSource); ok {
		isFile = true
	}

	initState := ConnDisconnected
	if src != nil {
		initState = ConnConnected
	}
	reconn := false
	if src == nil && cfg.Port != "" && !isFile {
		reconn = true
	}

	m := Model{
		keys:          defaultKeyMap(),
		serialCfg:     cfg,
		source:        src,
		connState:     initState,
		reconnecting:  reconn,
		isFileSource:  isFile,
		profile:       profile,
		parser:        p,
		columns:       cols,
		buffer:        buf,
		follow:        true,
		sidebarWidth:  20,
		showTimestamp: initTS,
		tsField:       tsField,
		tsFormat:      tsFormat,
		appConfig:     appCfg,
		settings:      savedSettings,
		baudList:      bauds,
		baudCursor:    baudIdx,
	}

	// Start with an empty permissive filter.
	m.activeFilter, _ = filter.New("")
	return m
}

// saveSettings persists current port, baud, profile, and timestamp display state to disk.
func (m Model) saveSettings() {
	if m.appConfig == nil {
		return
	}
	s := m.settings
	if s == nil {
		s = &config.Settings{}
	}
	s.Port = m.serialCfg.Port
	s.Baud = m.serialCfg.Baud
	if m.profile != nil {
		s.Profile = m.profile.Name
	} else {
		s.Profile = ""
	}
	s.ShowTimestamp = m.showTimestamp
	_ = m.appConfig.SaveSettings(s)
}

// Init starts the source reader goroutines or triggers auto-reconnect if disconnected.
func (m Model) Init() tea.Cmd {
	if m.source != nil {
		return tea.Batch(listenToSource(m.source), listenToSourceErrors(m.source))
	}
	if m.serialCfg.Port != "" && !m.isFileSource {
		return scheduleReconnectTick()
	}
	return nil
}
