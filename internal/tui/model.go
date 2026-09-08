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
	source    serial.Source
	serialCfg serial.Config
	connState ConnState
	connDetail string

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

	// Timestamp injection (toggleable at runtime with 't')
	injectTimestamp bool   // whether to stamp incoming records with receive time
	tsField         string // field name for the injected timestamp
	tsFormat        string // Go time layout for the injected timestamp

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

	// Resolve timestamp injection settings. The profile sets the initial state;
	// the user can toggle at runtime with 't'.
	tsField := "_ts"
	tsFormat := "15:04:05.000"
	initTS := false
	if profile != nil && profile.Ingest.Timestamp.Enabled {
		initTS = true
		tsField = profile.Ingest.Timestamp.TimestampField()
		tsFormat = profile.Ingest.Timestamp.TimestampFormat()
	}

	bauds := serial.CommonBaudRates()
	baudIdx := 4 // default to 115200 if found
	for i, b := range bauds {
		if b == cfg.Baud {
			baudIdx = i
			break
		}
	}

	m := Model{
		keys:            defaultKeyMap(),
		serialCfg:       cfg,
		source:          src,
		profile:         profile,
		parser:          p,
		columns:         cols,
		buffer:          buf,
		follow:          true,
		sidebarWidth:    20,
		injectTimestamp: initTS,
		tsField:         tsField,
		tsFormat:        tsFormat,
		appConfig:       appCfg,
		baudList:        bauds,
		baudCursor:      baudIdx,
	}


	// Start with an empty permissive filter.
	m.activeFilter, _ = filter.New("")
	return m
}


// Init starts the source reader goroutine (via a command) and returns the
// initial command set.
func (m Model) Init() tea.Cmd {
	if m.source != nil {
		return tea.Batch(listenToSource(m.source), listenToSourceErrors(m.source))
	}
	return nil
}
