package tui

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/brenoniehues/oh-my-logs/internal/config"
	"github.com/brenoniehues/oh-my-logs/internal/filter"
	"github.com/brenoniehues/oh-my-logs/internal/game"
	"github.com/brenoniehues/oh-my-logs/internal/parser"
	"github.com/brenoniehues/oh-my-logs/internal/record"
	"github.com/brenoniehues/oh-my-logs/internal/serial"
	"github.com/brenoniehues/oh-my-logs/internal/timing"
	"github.com/charmbracelet/bubbles/filepicker"
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
	modeFilterPresets
	modeSavePresetPrompt
	modeHelp
	modeGame
	modeTXInput
	modeSettings
	modeFilePicker
	modeRowDetail
)

type filePickerPurpose int

const (
	fpPurposeDirectToDisk filePickerPurpose = iota
	fpPurposeSaveLog
)

type filePickerSubMode int

const (
	fpModeBrowse filePickerSubMode = iota
	fpModeTypeDir
	fpModeTypePrefix
	fpModeNewFolder
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
type ConnStateMsg struct {
	State  ConnState
	Detail string
}

// LogSavedMsg signals the outcome of saving log records to a file.
type LogSavedMsg struct {
	Path  string
	Count int
	Err   error
}

// ViewportState encapsulates the scroll, search, selection, and viewport rendering
// state that is isolated per tab or pane.
type ViewportState struct {
	Visible        []record.Record
	ScrollOffset   int
	Follow         bool
	SearchInput    string
	SearchMatches  []int
	SearchCursor   int
	SelectedRow    int // selected row index into Visible (-1 if none)
	BookmarkedOnly bool
	DisplayFormat  DisplayFormat
	ScrollX        int // horizontal window scroll offset
	CursorCol      int // character cursor column (-1 if none)
	CharSelStart   int // character selection start (-1 if none)
	CharSelEnd     int // character selection end (-1 if none)
}

// Tab represents an independent virtual tab with its own filter, visible records,
// scroll position, follow state, and search state.
type Tab struct {
	Name      string
	FilterRaw string
	Filter    *filter.Filter
	ViewportState
}

// DisplayName returns a user-friendly label for the tab.
func (t Tab) DisplayName(defaultIndex int) string {
	if t.Name != "" {
		return t.Name
	}
	if t.FilterRaw != "" {
		return t.FilterRaw
	}
	return fmt.Sprintf("Tab %d", defaultIndex)
}

// Model is the top-level Bubble Tea model.
type Model struct {
	// Configuration
	keys   keyMap
	width  int
	height int

	// Serial / source
	source           serial.Source
	serialCfg        serial.Config
	connState        ConnState
	connDetail       string
	reconnecting     bool
	manualDisconnect bool
	isFileSource     bool
	isProcessSource  bool
	isPipeSource     bool
	processCmd       string

	// Profile & parser
	profile *parser.Profile
	parser  parser.Parser
	columns []record.Column

	// Data
	buffer  *record.Buffer
	visible []record.Record // filtered view

	// Filter
	activeFilter *filter.Filter
	filterInput  TextInput

	// Search
	searchInput   TextInput
	searchMatches []int // indices into visible
	searchCursor  int   // match navigation index into searchMatches

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

	// Filter presets
	filtersCfg          *config.FiltersConfig
	presetCursor        int
	savePresetNameInput TextInput

	// Serial TX transmission prompt
	txInput  TextInput
	txEnding serial.LineEnding

	// Easter egg mini-game
	activeGame     game.MiniGame
	logsDuringGame int

	// Timestamp & Delta-Time (Δt) display & settings (cycle mode with 't')
	showTimestamp  bool            // whether timestamp column is displayed (true if tsMode != TSModeOff)
	tsMode         TimestampMode   // clock, delta, both, or off
	tsField        string          // field name for the arrival timestamp (e.g. "_ts" or "time")
	tsFormat       string          // Go time layout for the timestamp
	deltaTracker   *timing.Tracker // dynamic EMA latency tracker
	lastRecordTime time.Time       // arrival time of previous stream record

	// Viewport dimensions (computed on resize)
	tableHeight  int
	sidebarWidth int

	// Virtual tabs
	tabs      []Tab
	activeTab int

	// Row selection & multi-row drag selection for copying
	selectedRow    int // index into visible (-1 if none)
	selectionStart int // multi-row drag start (-1 if none)
	selectionEnd   int // multi-row drag end (-1 if none)
	lastClickTime  time.Time
	lastClickRow   int

	// Bookmarks / pinning
	bookmarks      map[uint64]struct{} // set of bookmarked record IDs
	bookmarkedOnly bool                // when true, filter to show only bookmarked rows
	nextRecordID   uint64

	// Settings modal
	settingsCursor int

	// Dual-pane split view & chronological sync
	splitMode     SplitMode
	splitLeftTab  int  // index into m.tabs for pane 0 (left / top)
	splitRightTab int  // index into m.tabs for pane 1 (right / bottom)
	activePane    int  // 0 for pane 0 (splitLeftTab), 1 for pane 1 (splitRightTab)
	syncScroll    bool // when true, scrolling one pane time-locks the other

	// Multi-format representation (FormatParsed, FormatRaw, FormatHex, FormatBinary)
	displayFormat   DisplayFormat
	inspectorHeight int

	// 2D Window scroll & character cursor
	scrollX      int // horizontal window scroll offset
	cursorCol    int // character cursor column (-1 if none)
	charSelStart int // character selection start (-1 if none)
	charSelEnd   int // character selection end (-1 if none)

	// Direct-to-disk continuous logging & file picker
	diskLogger          *DiskLogger
	directToDiskPath    string
	directToDiskPrefix  string
	saveLogPrefix       string
	filePicker          filepicker.Model
	fpPurpose           filePickerPurpose
	fpSubMode           filePickerSubMode
	fpDirInput          TextInput
	fpPrefixInput       TextInput
	fpNewFolderInput    TextInput
	fpMatches           []string
	fpMatchPrefix       string
	fpMatchIndex        int

	// Row detail inspector modal
	detailScrollOffset int
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
	isProcess := false
	var procCmd string
	if ps, ok := src.(*serial.ProcessSource); ok {
		isProcess = true
		procCmd = ps.Command()
	}
	isPipe := false
	if _, ok := src.(*serial.PipeSource); ok {
		isPipe = true
	}

	initState := ConnDisconnected
	if src != nil {
		initState = ConnConnected
	}
	manualDisc := false
	if src == nil && !isFile && !isPipe {
		manualDisc = true
	}

	txEnd := serial.EndingCRLF
	var txHist []string
	if savedSettings != nil {
		if savedSettings.TXEnding != "" {
			txEnd = serial.ParseLineEnding(savedSettings.TXEnding)
		}
		if len(savedSettings.TXHistory) > 0 {
			txHist = append(txHist, savedSettings.TXHistory...)
		}
	}

	tsMode := TSModeClock
	if savedSettings != nil {
		if savedSettings.TimestampMode != "" {
			tsMode = ParseTimestampMode(savedSettings.TimestampMode)
		} else if !savedSettings.ShowTimestamp {
			tsMode = TSModeOff
		}
	} else if !initTS {
		tsMode = TSModeOff
	}

	var timingCfg timing.Config
	if profile != nil {
		timingCfg = profile.TimingParameters()
	} else {
		timingCfg = timing.DefaultConfig()
	}
	tracker := timing.NewTracker(timingCfg)

	initFollow := true
	if savedSettings != nil {
		if !savedSettings.DefaultFollow && savedSettings.BufferCapacity > 0 {
			initFollow = false
		}
		if savedSettings.BufferCapacity > 0 && buf != nil && buf.Cap() != savedSettings.BufferCapacity {
			buf.Resize(savedSettings.BufferCapacity)
		}
		if savedSettings.Theme != "" {
			SetCurrentTheme(savedSettings.Theme)
		}
	}

	m := Model{
		keys:                defaultKeyMap(),
		serialCfg:           cfg,
		source:              src,
		connState:           initState,
		manualDisconnect:    manualDisc,
		reconnecting:        false,
		isFileSource:        isFile,
		isProcessSource:     isProcess,
		isPipeSource:        isPipe,
		processCmd:          procCmd,
		profile:             profile,
		parser:              p,
		columns:             cols,
		buffer:              buf,
		follow:              initFollow,
		sidebarWidth:        20,
		showTimestamp:       tsMode != TSModeOff,
		tsMode:              tsMode,
		deltaTracker:        tracker,
		tsField:             tsField,
		tsFormat:            tsFormat,
		appConfig:           appCfg,
		settings:            savedSettings,
		baudList:            bauds,
		baudCursor:          baudIdx,
		bookmarks:           make(map[uint64]struct{}),
		filterInput:         NewTextInput(true),
		searchInput:         NewTextInput(false),
		savePresetNameInput: NewTextInput(false),
		txInput:             NewTextInput(true),
		txEnding:            txEnd,
		splitMode:           SplitNone,
		splitLeftTab:        0,
		splitRightTab:       1,
		activePane:          0,
		syncScroll:          true,
	}

	initPrefix := "oml"
	if savedSettings != nil && savedSettings.LogPrefix != "" {
		initPrefix = savedSettings.LogPrefix
	}
	m.directToDiskPrefix = initPrefix
	m.fpDirInput = NewTextInput(false)
	m.fpPrefixInput = NewTextInput(false)
	m.fpNewFolderInput = NewTextInput(false)

	m.txInput.History = txHist
	m.loadFilters()

	// Start with an empty permissive filter.
	initFilter, _ := filter.New("")
	m.activeFilter = initFilter

	initFormat := FormatParsed
	if savedSettings != nil && savedSettings.DisplayFormat != "" {
		initFormat = ParseDisplayFormat(savedSettings.DisplayFormat)
	}

	initTab := Tab{
		Name:      "All",
		FilterRaw: "",
		Filter:    initFilter,
		ViewportState: ViewportState{
			Follow:        true,
			SelectedRow:   -1,
			DisplayFormat: initFormat,
			CursorCol:     -1,
			CharSelStart:  -1,
			CharSelEnd:    -1,
		},
	}
	m.tabs = []Tab{initTab}
	m.activeTab = 0
	m.displayFormat = initFormat
	m.selectedRow = -1
	m.selectionStart = -1
	m.selectionEnd = -1
	m.lastClickRow = -1
	m.cursorCol = -1
	m.charSelStart = -1
	m.charSelEnd = -1

	if savedSettings != nil && savedSettings.DirectToDisk {
		targetPath := ""
		if savedSettings.LogDir != "" {
			targetPath = GenerateTimestampLogPathWithPrefix(savedSettings.LogDir, initPrefix)
		} else if appCfg != nil && appCfg.LogsDir != "" {
			targetPath = GenerateTimestampLogPathWithPrefix(appCfg.LogsDir, initPrefix)
		}
		_ = m.StartDiskLogger(targetPath)
	}

	return m
}

// StartDiskLogger begins direct-to-disk streaming to targetPath.
// If targetPath is empty, a timestamped file in the logs directory is used.
func (m *Model) StartDiskLogger(targetPath string) error {
	if m.diskLogger != nil {
		m.diskLogger.Close()
		m.diskLogger = nil
	}
	if targetPath == "" {
		logsDir := ""
		if m.appConfig != nil && m.appConfig.LogsDir != "" {
			logsDir = m.appConfig.LogsDir
		} else if m.settings != nil && m.settings.LogDir != "" {
			logsDir = m.settings.LogDir
		}
		if logsDir == "" {
			logsDir = "."
		}
		prefix := m.directToDiskPrefix
		if prefix == "" && m.settings != nil && m.settings.LogPrefix != "" {
			prefix = m.settings.LogPrefix
		}
		if prefix == "" {
			prefix = "oml"
		}
		targetPath = GenerateTimestampLogPathWithPrefix(logsDir, prefix)
	}
	dl, err := NewDiskLogger(targetPath)
	if err != nil {
		return err
	}
	m.diskLogger = dl
	m.directToDiskPath = targetPath
	if m.settings != nil {
		m.settings.DirectToDisk = true
		m.settings.LogDir = filepath.Dir(targetPath)
		if m.directToDiskPrefix != "" {
			m.settings.LogPrefix = m.directToDiskPrefix
		}
	}
	return nil
}

// StopDiskLogger closes the active disk logger if running.
func (m *Model) StopDiskLogger() {
	if m.diskLogger != nil {
		m.diskLogger.Close()
		m.diskLogger = nil
	}
	if m.settings != nil {
		m.settings.DirectToDisk = false
	}
}

// DiskLogger returns the active disk logger (if any).
func (m *Model) DiskLogger() *DiskLogger {
	return m.diskLogger
}

// DirectToDiskPath returns the configured or active log file path.
func (m *Model) DirectToDiskPath() string {
	return m.directToDiskPath
}

// DirectToDiskPrefix returns the configured log filename prefix.
func (m *Model) DirectToDiskPrefix() string {
	if m.directToDiskPrefix != "" {
		return m.directToDiskPrefix
	}
	if m.settings != nil && m.settings.LogPrefix != "" {
		return m.settings.LogPrefix
	}
	return "oml"
}

// SetDirectToDiskPrefix updates the log filename prefix.
func (m *Model) SetDirectToDiskPrefix(prefix string) {
	if prefix == "" {
		prefix = "oml"
	}
	m.directToDiskPrefix = prefix
	if m.settings != nil {
		m.settings.LogPrefix = prefix
	}
}

// currentTab returns a pointer to the currently active/focused tab.
func (m *Model) currentTab() *Tab {
	if len(m.tabs) == 0 {
		initFilter, _ := filter.New("")
		m.tabs = []Tab{{
			Name:      "All",
			FilterRaw: "",
			Filter:    initFilter,
			ViewportState: ViewportState{
				Follow:       true,
				SelectedRow:  -1,
				CursorCol:    -1,
				CharSelStart: -1,
				CharSelEnd:   -1,
			},
		}}
		m.activeTab = 0
		m.splitLeftTab = 0
		m.splitRightTab = 0
	}
	targetIdx := m.activeTabIdx()
	if targetIdx < 0 {
		targetIdx = 0
	}
	if targetIdx >= len(m.tabs) {
		targetIdx = len(m.tabs) - 1
	}
	return &m.tabs[targetIdx]
}

// activeTabIdx returns the index of the currently focused tab in m.tabs.
func (m *Model) activeTabIdx() int {
	if m.splitMode != SplitNone {
		if m.activePane == 1 {
			if m.splitRightTab >= 0 && m.splitRightTab < len(m.tabs) {
				return m.splitRightTab
			}
		} else {
			if m.splitLeftTab >= 0 && m.splitLeftTab < len(m.tabs) {
				return m.splitLeftTab
			}
		}
	}
	if m.activeTab < 0 {
		return 0
	}
	if m.activeTab >= len(m.tabs) {
		return len(m.tabs) - 1
	}
	return m.activeTab
}

// exportViewport captures the active tab/viewport state from Model.
func (m *Model) exportViewport() ViewportState {
	return ViewportState{
		Visible:        m.visible,
		ScrollOffset:   m.scrollOffset,
		Follow:         m.follow,
		SearchInput:    m.searchInput.Value,
		SearchMatches:  m.searchMatches,
		SearchCursor:   m.searchCursor,
		SelectedRow:    m.selectedRow,
		BookmarkedOnly: m.bookmarkedOnly,
		DisplayFormat:  m.displayFormat,
		ScrollX:        m.scrollX,
		CursorCol:      m.cursorCol,
		CharSelStart:   m.charSelStart,
		CharSelEnd:     m.charSelEnd,
	}
}

// importViewport restores active viewport state into Model from vs.
func (m *Model) importViewport(vs ViewportState) {
	m.visible = vs.Visible
	m.scrollOffset = vs.ScrollOffset
	m.follow = vs.Follow
	m.searchInput.SetText(vs.SearchInput)
	m.searchMatches = vs.SearchMatches
	m.searchCursor = vs.SearchCursor
	m.selectedRow = vs.SelectedRow
	m.bookmarkedOnly = vs.BookmarkedOnly
	m.displayFormat = vs.DisplayFormat
	m.scrollX = vs.ScrollX
	m.cursorCol = vs.CursorCol
	m.charSelStart = vs.CharSelStart
	m.charSelEnd = vs.CharSelEnd
	m.selectionStart = -1
	m.selectionEnd = -1
}

// syncActiveTabToModel saves the active model's interactive state back to the active tab struct.
func (m *Model) syncActiveTabToModel() {
	if len(m.tabs) == 0 {
		return
	}
	cur := m.currentTab()
	cur.Filter = m.activeFilter
	cur.FilterRaw = m.filterInput.Value
	cur.ViewportState = m.exportViewport()
}

// syncModelToActiveTab updates the model's active view state from the current tab.
func (m *Model) syncModelToActiveTab() {
	cur := m.currentTab()
	m.activeFilter = cur.Filter
	m.filterInput.SetText(cur.FilterRaw)
	m.importViewport(cur.ViewportState)
}

// switchTab changes the active tab and synchronizes state.
func (m *Model) switchTab(newIdx int) {
	if len(m.tabs) <= 1 || newIdx < 0 || newIdx >= len(m.tabs) {
		return
	}
	oldInsp := m.isInspectorActive()
	m.syncActiveTabToModel()
	if m.splitMode != SplitNone {
		if m.activePane == 0 {
			if newIdx == m.splitRightTab {
				// Tab is already displayed on the right pane: shift focus to right pane without changing positions
				m.activePane = 1
			} else {
				m.splitLeftTab = newIdx
			}
		} else {
			if newIdx == m.splitLeftTab {
				// Tab is already displayed on the left pane: shift focus to left pane without changing positions
				m.activePane = 0
			} else {
				m.splitRightTab = newIdx
			}
		}
	} else {
		m.activeTab = newIdx
	}
	m.syncModelToActiveTab()
	if m.isInspectorActive() != oldInsp {
		m.recalcLayout()
	}
	m.clampScroll()
}

// Init starts the source reader goroutines if connected on launch.
// When disconnected, it waits cleanly for user action (e.g. pressing 'r' to connect).
func (m Model) Init() tea.Cmd {
	if m.source != nil {
		return tea.Batch(listenToSource(m.source), listenToSourceErrors(m.source))
	}
	return nil
}

// SetProcessCommand sets or updates the shell command associated with a process source.
func (m *Model) SetProcessCommand(command string) {
	m.isProcessSource = true
	m.processCmd = command
}
