package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/brenoniehues/oh-my-logs/internal/config"
	"github.com/brenoniehues/oh-my-logs/internal/filter"
	"github.com/brenoniehues/oh-my-logs/internal/game"
	"github.com/brenoniehues/oh-my-logs/internal/parser"
	"github.com/brenoniehues/oh-my-logs/internal/record"
	"github.com/brenoniehues/oh-my-logs/internal/serial"
	"github.com/brenoniehues/oh-my-logs/internal/timing"
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
)

// TimestampMode defines whether to render arrival clock time, relative delta (Δt), or both.
type TimestampMode int

const (
	TSModeClock TimestampMode = iota // clock arrival time (e.g. 15:04:05.000)
	TSModeDelta                      // relative elapsed time since previous log (e.g. +14.2ms)
	TSModeBoth                       // both clock and delta columns
	TSModeOff                        // hidden
)

func (m TimestampMode) String() string {
	switch m {
	case TSModeClock:
		return "clock"
	case TSModeDelta:
		return "delta"
	case TSModeBoth:
		return "both"
	case TSModeOff:
		return "off"
	default:
		return "clock"
	}
}

func (m TimestampMode) Next() TimestampMode {
	switch m {
	case TSModeClock:
		return TSModeDelta
	case TSModeDelta:
		return TSModeBoth
	case TSModeBoth:
		return TSModeOff
	case TSModeOff:
		return TSModeClock
	default:
		return TSModeClock
	}
}

func ParseTimestampMode(s string) TimestampMode {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "delta", "dt":
		return TSModeDelta
	case "both":
		return TSModeBoth
	case "off", "none", "false":
		return TSModeOff
	default:
		return TSModeClock
	}
}

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

// SplitMode defines the dual-pane view state.
type SplitMode int

const (
	SplitNone       SplitMode = iota // Standard single-pane view
	SplitVertical                    // Side-by-side split (left & right)
	SplitHorizontal                  // Stacked split (top & bottom)
)

// Tab represents an independent virtual tab with its own filter, visible records,
// scroll position, follow state, and search state.
type Tab struct {
	Name          string
	FilterRaw     string
	Filter        *filter.Filter
	Visible       []record.Record
	ScrollOffset  int
	Follow        bool
	SearchInput   string
	SearchMatches []int
	SearchCursor  int
	SelectedRow    int // selected row index into Visible (-1 if none)
	BookmarkedOnly bool
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

	// Filter history & presets
	filterHistory       []string
	filterHistoryCursor int
	filterDraft         string
	filtersCfg          *config.FiltersConfig
	presetCursor        int
	savePresetNameInput string

	// Serial TX transmission prompt
	txInput         string
	txHistory       []string
	txHistoryCursor int
	txDraft         string
	txEnding        serial.LineEnding

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

	// Dual-pane split view & chronological sync
	splitMode     SplitMode
	splitLeftTab  int  // index into m.tabs for pane 0 (left / top)
	splitRightTab int  // index into m.tabs for pane 1 (right / bottom)
	activePane    int  // 0 for pane 0 (splitLeftTab), 1 for pane 1 (splitRightTab)
	syncScroll    bool // when true, scrolling one pane time-locks the other
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
		showTimestamp: tsMode != TSModeOff,
		tsMode:        tsMode,
		deltaTracker:  tracker,
		tsField:       tsField,
		tsFormat:      tsFormat,
		appConfig:     appCfg,
		settings:      savedSettings,
		baudList:      bauds,
		baudCursor:          baudIdx,
		bookmarks:           make(map[uint64]struct{}),
		filterHistoryCursor: -1,
		txEnding:            txEnd,
		txHistory:           txHist,
		txHistoryCursor:     -1,
		splitMode:           SplitNone,
		splitLeftTab:        0,
		splitRightTab:       1,
		activePane:          0,
		syncScroll:          true,
	}

	m.loadFilters()

	// Start with an empty permissive filter.
	initFilter, _ := filter.New("")
	m.activeFilter = initFilter

	initTab := Tab{
		Name:        "All",
		FilterRaw:   "",
		Filter:      initFilter,
		Follow:      true,
		SelectedRow: -1,
	}
	m.tabs = []Tab{initTab}
	m.activeTab = 0
	m.selectedRow = -1
	m.selectionStart = -1
	m.selectionEnd = -1
	m.lastClickRow = -1
	return m
}

// currentTab returns a pointer to the currently active/focused tab.
func (m *Model) currentTab() *Tab {
	if len(m.tabs) == 0 {
		initFilter, _ := filter.New("")
		m.tabs = []Tab{{
			Name:        "All",
			FilterRaw:   "",
			Filter:      initFilter,
			Follow:      true,
			SelectedRow: -1,
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

// toggleSplit switches between SplitNone and the requested split mode (SplitVertical or SplitHorizontal).
func (m *Model) toggleSplit(mode SplitMode) {
	if m.splitMode == mode {
		// Close split mode — resume single tab with the currently focused tab
		m.syncActiveTabToModel()
		m.activeTab = m.activeTabIdx()
		m.splitMode = SplitNone
		m.activePane = 0
		m.syncModelToActiveTab()
		m.clampScroll()
		m.message = "Split view closed (single tab)"
		return
	}

	m.syncActiveTabToModel()
	if len(m.tabs) == 1 {
		// Automatically create a second tab if only 1 exists
		initFilter, _ := filter.New("")
		m.tabs = append(m.tabs, Tab{
			Name:        "Tab 2",
			FilterRaw:   "",
			Filter:      initFilter,
			Visible:     m.buffer.All(),
			Follow:      true,
			SelectedRow: -1,
		})
		m.splitLeftTab = 0
		m.splitRightTab = 1
	} else {
		m.splitLeftTab = m.activeTab
		if m.splitRightTab == m.splitLeftTab || m.splitRightTab < 0 || m.splitRightTab >= len(m.tabs) {
			m.splitRightTab = (m.splitLeftTab + 1) % len(m.tabs)
		}
	}

	m.splitMode = mode
	m.activePane = 0
	m.syncScroll = true // default to chronological sync on split

	// Ensure viewports and follow offsets match the split pane heights
	for p := 0; p < 2; p++ {
		t := m.currentTabForPane(p)
		if t != nil {
			h := m.paneDataHeight(p)
			if t.Follow && len(t.Visible) > h {
				t.ScrollOffset = len(t.Visible) - h
			} else {
				maxO := len(t.Visible) - h
				if maxO < 0 {
					maxO = 0
				}
				if t.ScrollOffset > maxO {
					t.ScrollOffset = maxO
				}
			}
		}
	}

	m.syncModelToActiveTab()
	m.syncOtherPaneChronologically()
	m.clampScroll()
	if mode == SplitVertical {
		m.message = "Split view: Vertical (side-by-side) · [S] Sync ON · [w] Switch pane"
	} else {
		m.message = "Split view: Horizontal (stacked) · [S] Sync ON · [w] Switch pane"
	}
}

// switchPaneFocus toggles focus between primary pane (0) and secondary pane (1) without moving tabs.
func (m *Model) switchPaneFocus() {
	if m.splitMode == SplitNone {
		return
	}
	m.syncActiveTabToModel()
	if m.activePane == 0 {
		m.activePane = 1
	} else {
		m.activePane = 0
	}
	m.syncModelToActiveTab()
	m.clampScroll()

	paneName := "Left"
	if m.splitMode == SplitHorizontal {
		paneName = "Top"
		if m.activePane == 1 {
			paneName = "Bottom"
		}
	} else if m.activePane == 1 {
		paneName = "Right"
	}
	cur := m.currentTab()
	m.message = fmt.Sprintf("Focus: %s Pane [%s]", paneName, cur.DisplayName(m.activeTabIdx()+1))
}

// toggleSyncScroll toggles chronological time-locked scrolling on/off.
func (m *Model) toggleSyncScroll() {
	m.syncScroll = !m.syncScroll
	if m.syncScroll {
		m.message = "Chronological Sync: ON"
		m.syncOtherPaneChronologically()
	} else {
		m.message = "Chronological Sync: OFF (Independent scrolling)"
	}
}

// currentTabForPane returns the tab assigned to the given pane index (0 = left/top, 1 = right/bottom).
func (m *Model) currentTabForPane(pane int) *Tab {
	idx := m.paneTabIdx(pane)
	if idx < 0 || idx >= len(m.tabs) {
		return m.currentTab()
	}
	return &m.tabs[idx]
}

// paneTabIdx returns the tab index in m.tabs assigned to pane 0 or 1.
// Positions are frozen: pane 0 is always splitLeftTab, pane 1 is always splitRightTab.
func (m *Model) paneTabIdx(pane int) int {
	if pane == 1 {
		if m.splitRightTab >= 0 && m.splitRightTab < len(m.tabs) {
			return m.splitRightTab
		}
		if len(m.tabs) > 1 {
			return 1
		}
		return 0
	}
	if m.splitLeftTab >= 0 && m.splitLeftTab < len(m.tabs) {
		return m.splitLeftTab
	}
	return 0
}

// paneDataHeight returns the number of visible log data rows for the given pane (0 or 1).
func (m *Model) paneDataHeight(pane int) int {
	if m.splitMode != SplitHorizontal {
		h := m.tableHeight
		if h < 1 {
			return 1
		}
		return h
	}

	totalH := m.tableHeight + 2
	availH := totalH - 1
	topH := availH / 2
	if topH < 3 {
		topH = 3
	}
	bottomH := availH - topH
	if bottomH < 3 {
		bottomH = 3
	}

	if pane == 1 {
		h := bottomH - 2
		if h < 1 {
			return 1
		}
		return h
	}
	h := topH - 2
	if h < 1 {
		return 1
	}
	return h
}

// activeDataHeight returns the number of visible log data rows in the currently focused pane.
func (m *Model) activeDataHeight() int {
	return m.paneDataHeight(m.activePane)
}

// syncActiveTabToModel saves the active model's interactive state back to the active tab struct.
func (m *Model) syncActiveTabToModel() {
	if len(m.tabs) == 0 {
		return
	}
	cur := m.currentTab()
	cur.Filter = m.activeFilter
	cur.FilterRaw = m.filterInput
	cur.Visible = m.visible
	cur.ScrollOffset = m.scrollOffset
	cur.Follow = m.follow
	cur.SearchInput = m.searchInput
	cur.SearchMatches = m.searchMatches
	cur.SearchCursor = m.searchCursor
	cur.SelectedRow = m.selectedRow
	cur.BookmarkedOnly = m.bookmarkedOnly
}

// syncModelToActiveTab updates the model's active view state from the current tab.
func (m *Model) syncModelToActiveTab() {
	cur := m.currentTab()
	m.activeFilter = cur.Filter
	m.filterInput = cur.FilterRaw
	m.visible = cur.Visible
	m.scrollOffset = cur.ScrollOffset
	m.follow = cur.Follow
	m.searchInput = cur.SearchInput
	m.searchMatches = cur.SearchMatches
	m.searchCursor = cur.SearchCursor
	m.selectedRow = cur.SelectedRow
	m.bookmarkedOnly = cur.BookmarkedOnly
	m.selectionStart = -1
	m.selectionEnd = -1
}

// switchTab changes the active tab and synchronizes state.
func (m *Model) switchTab(newIdx int) {
	if len(m.tabs) <= 1 || newIdx < 0 || newIdx >= len(m.tabs) {
		return
	}
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
	m.clampScroll()
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
	s.ShowTimestamp = (m.tsMode != TSModeOff)
	s.TimestampMode = m.tsMode.String()
	s.TXEnding = m.txEnding.String()
	s.TXHistory = m.txHistory
	_ = m.appConfig.SaveSettings(s)
}

// loadFilters loads filter presets and history from config, or populates defaults.
func (m *Model) loadFilters() {
	if m.filtersCfg != nil {
		return
	}
	if m.appConfig != nil {
		if fc, err := m.appConfig.LoadFilters(); err == nil && fc != nil {
			m.filtersCfg = fc
			if len(m.filterHistory) == 0 {
				m.filterHistory = fc.History
			}
			return
		}
	}
	m.filtersCfg = &config.FiltersConfig{
		Presets: config.DefaultFilterPresets(),
		History: []string{},
	}
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

// selectionRange returns the normalized [start, end] (inclusive) bounds of the
// active multi-row selection, or (-1, -1) if no multi-row selection is active.
func (m Model) selectionRange() (int, int) {
	if m.selectionStart < 0 || m.selectionEnd < 0 || m.selectionStart == m.selectionEnd {
		return -1, -1
	}
	start, end := m.selectionStart, m.selectionEnd
	if start > end {
		start, end = end, start
	}
	if start < 0 {
		start = 0
	}
	if end >= len(m.visible) {
		end = len(m.visible) - 1
	}
	if start >= end || len(m.visible) == 0 {
		return -1, -1
	}
	return start, end
}

// selectionDelta calculates the elapsed time between the first and last
// record in the currently active multi-row selection, if timestamps/deltas are available.
func (m Model) selectionDelta() (time.Duration, bool) {
	start, end := m.selectionRange()
	if start < 0 || end < 0 {
		return 0, false
	}

	firstRec := m.visible[start]
	lastRec := m.visible[end]

	t0 := firstRec.Timestamp
	if t0.IsZero() {
		t0 = tryParseRecordTimestamp(firstRec, m.tsField)
	}
	t1 := lastRec.Timestamp
	if t1.IsZero() {
		t1 = tryParseRecordTimestamp(lastRec, m.tsField)
	}

	// 1. Both endpoints have valid timestamps
	if !t0.IsZero() && !t1.IsZero() {
		d := t1.Sub(t0)
		if d < 0 && d > -24*time.Hour && t0.Year() == 0 {
			d += 24 * time.Hour
		} else if d < 0 {
			d = -d
		}
		return d, true
	}

	// 2. Fallback: sum inter-record deltas across the selected range
	var sum time.Duration
	hasDelta := false
	for i := start + 1; i <= end; i++ {
		r := m.visible[i]
		if r.Delta > 0 {
			sum += r.Delta
			hasDelta = true
		}
	}
	if hasDelta {
		return sum, true
	}

	return 0, false
}

// selectionMessage formats a user-friendly selection summary with elapsed delta if available.
func (m Model) selectionMessage(count int, baseMsg string) string {
	d, ok := m.selectionDelta()
	suffix := ""
	if baseMsg != "" {
		suffix = fmt.Sprintf(" (%s)", baseMsg)
	}
	if ok {
		return fmt.Sprintf("%d rows selected · Δt: %s%s", count, timing.FormatDelta(d), suffix)
	}
	return fmt.Sprintf("%d rows selected%s", count, suffix)
}
