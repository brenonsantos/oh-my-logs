package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/brenoniehues/oh-my-logs/internal/config"
	"github.com/brenoniehues/oh-my-logs/internal/filter"
	"github.com/brenoniehues/oh-my-logs/internal/parser"
	"github.com/brenoniehues/oh-my-logs/internal/record"
	"github.com/brenoniehues/oh-my-logs/internal/serial"
	tea "github.com/charmbracelet/bubbletea"
)


// listenToSource returns a command that waits for the next line from the source.
func listenToSource(src serial.Source) tea.Cmd {
	return func() tea.Msg {
		line, ok := <-src.Lines()
		if !ok {
			return ConnStateMsg{State: ConnDisconnected, Detail: "source closed"}
		}
		return lineMsg(line)
	}
}

// listenToSourceErrors returns a command that reads the next error from the source.
func listenToSourceErrors(src serial.Source) tea.Cmd {
	return func() tea.Msg {
		err, ok := <-src.Errors()
		if !ok {
			return nil
		}
		return ErrorMsg{Err: err}
	}
}

// lineMsg is an internal message carrying a raw line from the source.
type lineMsg string

// sourceReadyMsg is sent when a new serial connection has been established.
type sourceReadyMsg struct {
	source serial.Source
	port   string
}

const reconnectInterval = 750 * time.Millisecond

type reconnectTickMsg struct{}
type reconnectFailedMsg struct {
	reason string
}

func scheduleReconnectTick() tea.Cmd {
	return tea.Tick(reconnectInterval, func(t time.Time) tea.Msg {
		return reconnectTickMsg{}
	})
}

func tryReconnectCmd(cfg serial.Config) tea.Cmd {
	return func() tea.Msg {
		ports, err := serial.ListPorts()
		if err != nil {
			return reconnectFailedMsg{reason: fmt.Sprintf("Error scanning serial ports: %v", err)}
		}
		candidate := serial.MatchCandidatePort(cfg.Port, ports)
		if candidate == "" {
			return reconnectFailedMsg{
				reason: fmt.Sprintf("Device disconnected — auto-reconnecting (waiting for %s)", cfg.Port),
			}
		}

		candidateCfg := cfg
		candidateCfg.Port = candidate
		src, err := serial.NewSerialSource(candidateCfg)
		if err != nil {
			return reconnectFailedMsg{
				reason: fmt.Sprintf("Detected %s — waiting for device to become ready…", candidate),
			}
		}
		return sourceReadyMsg{source: src, port: candidate}
	}
}

// Update is the Bubble Tea update function.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	// ── Terminal resize ──────────────────────────────────────────────────────
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.recalcLayout()
		m.clampScroll()
		return m, nil

	// ── New line from source ─────────────────────────────────────────────────
	case lineMsg:
		r, _ := m.parser.Parse(string(msg))
		if r.Fields == nil {
			r.Fields = make(map[string]string)
		}
		// Always record arrival timestamp on ingest so past records have it when toggled on.
		nowStr := time.Now().Format(m.tsFormat)
		if r.Fields[m.tsField] == "" {
			r.Fields[m.tsField] = nowStr
		}
		if r.Fields["_ts"] == "" {
			r.Fields["_ts"] = nowStr
		}
		m.buffer.Add(r)

		// Dispatch to all tabs
		if len(m.tabs) == 0 {
			_ = m.currentTab()
		}
		for i := range m.tabs {
			if m.tabs[i].Filter == nil || m.tabs[i].Filter.Empty() || m.tabs[i].Filter.Matches(r) {
				m.tabs[i].Visible = append(m.tabs[i].Visible, r)
				if m.tabs[i].Follow && !m.paused {
					if len(m.tabs[i].Visible) > m.tableHeight {
						m.tabs[i].ScrollOffset = len(m.tabs[i].Visible) - m.tableHeight
					} else {
						m.tabs[i].ScrollOffset = 0
					}
				}
			}
		}

		cur := m.currentTab()
		m.visible = cur.Visible
		m.scrollOffset = cur.ScrollOffset
		m.follow = cur.Follow

		// Re-arm the listener.
		return m, listenToSource(m.source)

	// ── Source error ─────────────────────────────────────────────────────────
	case ErrorMsg:
		m.message = msg.Err.Error()
		if m.connState == ConnConnected {
			// A fatal read error occurred on active connection (e.g. cable unplugged)
			m.connState = ConnDisconnected
			m.connDetail = msg.Err.Error()
			if m.source != nil {
				m.source.Stop()
				m.source = nil
			}
			if m.serialCfg.Port != "" && !m.isFileSource {
				m.reconnecting = true
				m.message = "Device disconnected — auto-reconnecting…"
				return m, scheduleReconnectTick()
			}
			return m, nil
		}
		if m.source != nil {
			return m, listenToSourceErrors(m.source)
		}
		return m, nil

	// ── Connection state ─────────────────────────────────────────────────────
	case ConnStateMsg:
		m.connState = msg.State
		m.connDetail = msg.Detail
		if msg.State == ConnDisconnected {
			if m.source != nil {
				m.source.Stop()
				m.source = nil
			}
			if m.serialCfg.Port != "" && !m.isFileSource {
				if !m.reconnecting {
					m.reconnecting = true
					m.message = "Device disconnected — auto-reconnecting…"
					return m, scheduleReconnectTick()
				}
			}
		}
		return m, nil

	// ── Auto-reconnect polling ───────────────────────────────────────────────
	case reconnectTickMsg:
		if m.connState == ConnConnected || m.serialCfg.Port == "" || m.isFileSource {
			m.reconnecting = false
			return m, nil
		}
		m.reconnecting = true
		return m, tryReconnectCmd(m.serialCfg)

	case reconnectFailedMsg:
		if msg.reason != "" {
			m.message = msg.reason
		}
		if m.connState != ConnConnected && m.serialCfg.Port != "" && !m.isFileSource {
			m.reconnecting = true
			return m, scheduleReconnectTick()
		}
		m.reconnecting = false
		return m, nil

	// ── New source connected ─────────────────────────────────────────────────
	case sourceReadyMsg:
		if m.source != nil && m.source != msg.source {
			m.source.Stop()
		}
		m.source = msg.source
		m.serialCfg.Port = msg.port
		m.connState = ConnConnected
		m.connDetail = ""
		m.reconnecting = false
		m.mode = modeNormal
		m.message = fmt.Sprintf("Connected to %s", msg.port)
		m.saveSettings()
		return m, tea.Batch(listenToSource(m.source), listenToSourceErrors(m.source))

	// ── Mouse events ─────────────────────────────────────────────────────────
	case tea.MouseMsg:
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			switch m.mode {
			case modePortPicker:
				if m.portPickerSection == 0 {
					if m.portCursor > 0 {
						m.portCursor--
					}
				} else {
					if m.baudCursor > 0 {
						m.baudCursor--
					}
				}
			case modeProfilePicker:
				if m.profileCursor > 0 {
					m.profileCursor--
				}
			case modeHelp:
				// ignore scrolling while help modal is active
			default:
				m.follow = false
				m.scrollOffset -= 3
				m.clampScroll()
			}
			return m, nil

		case tea.MouseButtonWheelDown:
			switch m.mode {
			case modePortPicker:
				if m.portPickerSection == 0 {
					if m.portCursor < len(m.portList)-1 {
						m.portCursor++
					}
				} else {
					if m.baudCursor < len(m.baudList)-1 {
						m.baudCursor++
					}
				}
			case modeProfilePicker:
				if m.profileCursor < len(m.profileList)-1 {
					m.profileCursor++
				}
			case modeHelp:
				// ignore scrolling while help modal is active
			default:
				m.scrollOffset += 3
				if m.scrollOffset >= len(m.visible)-m.tableHeight {
					m.follow = true
				}
				m.clampScroll()
			}
			return m, nil
		}

	// ── Key events ───────────────────────────────────────────────────────────
	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}


// handleKey dispatches key events based on the current input mode.
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.mode {
	case modeSearch:
		return m.handleSearchKey(msg)
	case modeFilter:
		return m.handleFilterKey(msg)
	case modePortPicker:
		return m.handlePortPickerKey(msg)
	case modeProfilePicker:
		return m.handleProfilePickerKey(msg)
	case modeHelp:
		return m.handleHelpKey(msg)
	default:
		return m.handleNormalKey(msg)
	}
}


func (m Model) handleNormalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case keyMatches(msg, m.keys.Quit):
		m.saveSettings()
		if m.source != nil {
			m.source.Stop()
		}
		return m, tea.Quit

	case keyMatches(msg, m.keys.Search):
		m.mode = modeSearch
		m.searchInput = ""
		m.searchMatches = nil
		m.searchCursor = 0
		return m, nil

	case keyMatches(msg, m.keys.Filter):
		m.mode = modeFilter
		m.filterInput = m.currentTab().FilterRaw
		return m, nil

	case keyMatches(msg, m.keys.Clear):
		m.buffer.Clear()
		for i := range m.tabs {
			m.tabs[i].Visible = nil
			m.tabs[i].ScrollOffset = 0
			m.tabs[i].SearchMatches = nil
		}
		m.visible = nil
		m.searchMatches = nil
		m.scrollOffset = 0
		m.message = "Cleared"
		return m, nil

	case keyMatches(msg, m.keys.Pause):
		m.paused = !m.paused
		if !m.paused {
			m.rebuildVisible()
			if m.follow {
				m.scrollToBottom()
			}
		}
		return m, nil

	case keyMatches(msg, m.keys.ScrollUp):
		m.follow = false
		m.scrollOffset--
		m.clampScroll()
		return m, nil

	case keyMatches(msg, m.keys.ScrollDown):
		m.scrollOffset++
		if m.scrollOffset >= len(m.visible)-m.tableHeight {
			m.follow = true
		}
		m.clampScroll()
		return m, nil

	case keyMatches(msg, m.keys.PageUp):
		m.follow = false
		m.scrollOffset -= m.tableHeight
		m.clampScroll()
		return m, nil

	case keyMatches(msg, m.keys.PageDown):
		m.scrollOffset += m.tableHeight
		if m.scrollOffset >= len(m.visible)-m.tableHeight {
			m.follow = true
		}
		m.clampScroll()
		return m, nil

	case keyMatches(msg, m.keys.GoToBottom):
		m.follow = true
		m.scrollToBottom()
		return m, nil

	case keyMatches(msg, m.keys.GoToTop):
		m.follow = false
		m.scrollOffset = 0
		return m, nil

	case keyMatches(msg, m.keys.SaveLog):
		return m, m.cmdSaveLog()

	case keyMatches(msg, m.keys.Port):
		ports, _ := serial.ListPorts()
		m.portList = ports
		m.portCursor = 0
		for i, p := range ports {
			if p == m.serialCfg.Port {
				m.portCursor = i
				break
			}
		}
		for i, b := range m.baudList {
			if b == m.serialCfg.Baud {
				m.baudCursor = i
				break
			}
		}
		m.portPickerSection = 0
		m.mode = modePortPicker
		return m, nil


	case keyMatches(msg, m.keys.ProfileSwitch):
		paths := config.FindAllProfilePaths(m.appConfig)
		var items []ProfileItem
		items = append(items, ProfileItem{Name: "Raw (Default)", Path: ""})
		for _, p := range paths {
			prof, err := parser.LoadProfile(p)
			if err != nil {
				continue
			}
			items = append(items, ProfileItem{Name: prof.Name, Path: p})
		}
		m.profileList = items
		m.profileCursor = 0
		for i, item := range items {
			if m.profile != nil && item.Name == m.profile.Name {
				m.profileCursor = i
				break
			}
		}
		m.mode = modeProfilePicker
		return m, nil


	case keyMatches(msg, m.keys.Reconnect):
		if m.serialCfg.Port == "" {
			m.message = "No port configured — press p to pick one"
			return m, nil
		}
		if m.source != nil {
			m.source.Stop()
			m.source = nil
		}
		m.connState = ConnDisconnected
		return m, connectCmd(m.serialCfg)

	case keyMatches(msg, m.keys.ToggleTimestamp):
		m.showTimestamp = !m.showTimestamp
		m.saveSettings()
		if m.showTimestamp {
			m.message = fmt.Sprintf("⏱ Timestamp ON (%s)", m.tsField)
		} else {
			m.message = "⏱ Timestamp OFF"
		}
		return m, nil

	case keyMatches(msg, m.keys.Help):
		m.mode = modeHelp
		return m, nil

	case keyMatches(msg, m.keys.NextMatch):
		m.nextSearchMatch()
		return m, nil

	case keyMatches(msg, m.keys.PrevMatch):
		m.prevSearchMatch()
		return m, nil

	case keyMatches(msg, m.keys.NextTab):
		if len(m.tabs) > 1 {
			m.switchTab((m.activeTab + 1) % len(m.tabs))
		}
		return m, nil

	case keyMatches(msg, m.keys.PrevTab):
		if len(m.tabs) > 1 {
			m.switchTab((m.activeTab - 1 + len(m.tabs)) % len(m.tabs))
		}
		return m, nil

	case keyMatches(msg, m.keys.NewTab):
		return m.handleCreateNewTab()

	case keyMatches(msg, m.keys.CloseTab):
		return m.handleCloseActiveTab()

	default:
		s := msg.String()
		if len(s) == 1 && s >= "1" && s <= "9" {
			idx := int(s[0] - '1')
			if idx < len(m.tabs) {
				m.switchTab(idx)
				return m, nil
			}
		}
	}

	return m, nil
}

func (m Model) handleSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case keyMatches(msg, m.keys.Cancel):
		m.mode = modeNormal
		return m, nil

	case keyMatches(msg, m.keys.Confirm):
		if len(m.searchMatches) > 0 {
			m.nextSearchMatch()
		} else {
			m.mode = modeNormal
		}
		return m, nil

	case msg.String() == "down":
		m.nextSearchMatch()
		return m, nil

	case msg.String() == "up":
		m.prevSearchMatch()
		return m, nil

	default:
		m.searchInput = handleTextInput(m.searchInput, msg)
		m.runSearch()
	}
	return m, nil
}

func (m Model) handleFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case keyMatches(msg, m.keys.Cancel):
		m.mode = modeNormal
		m.filterInput = m.currentTab().FilterRaw
		return m, nil

	case keyMatches(msg, m.keys.Confirm):
		f, err := filter.New(m.filterInput)
		if err != nil {
			m.message = fmt.Sprintf("filter error: %v", err)
		} else {
			m.activeFilter = f
			cur := m.currentTab()
			cur.Filter = f
			cur.FilterRaw = m.filterInput
			if cur.Name == "" || strings.HasPrefix(cur.Name, "Tab ") {
				if m.filterInput != "" {
					cur.Name = m.filterInput
				}
			}
			m.rebuildVisible()
			cur.Visible = m.visible
			if m.follow {
				m.scrollToBottom()
				cur.ScrollOffset = m.scrollOffset
			}
		}
		m.mode = modeNormal
		return m, nil

	default:
		m.filterInput = handleTextInput(m.filterInput, msg)
	}
	return m, nil
}

// handlePortPickerKey handles navigation and selection inside the port & baud modal.
func (m Model) handlePortPickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case keyMatches(msg, m.keys.Cancel):
		m.mode = modeNormal
		return m, nil

	case msg.String() == "tab" || msg.String() == "shift+tab":
		if m.portPickerSection == 0 {
			m.portPickerSection = 1
		} else {
			m.portPickerSection = 0
		}
		return m, nil

	case msg.String() == "left" || msg.String() == "h":
		if m.baudCursor > 0 {
			m.baudCursor--
		}
		return m, nil

	case msg.String() == "right" || msg.String() == "l":
		if m.baudCursor < len(m.baudList)-1 {
			m.baudCursor++
		}
		return m, nil

	case keyMatches(msg, m.keys.ScrollUp):
		if m.portPickerSection == 0 {
			if m.portCursor > 0 {
				m.portCursor--
			}
		} else {
			// In baud section: moving up switches back to ports
			m.portPickerSection = 0
		}
		return m, nil

	case keyMatches(msg, m.keys.ScrollDown):
		if m.portPickerSection == 0 {
			if m.portCursor < len(m.portList)-1 {
				m.portCursor++
			} else {
				// At bottom of ports: moving down switches to baud
				m.portPickerSection = 1
			}
		} else {
			if m.baudCursor < len(m.baudList)-1 {
				m.baudCursor++
			}
		}
		return m, nil


	case keyMatches(msg, m.keys.Confirm):
		var port string
		if len(m.portList) > 0 && m.portCursor < len(m.portList) {
			port = m.portList[m.portCursor]
		} else {
			port = m.serialCfg.Port
		}
		if port == "" {
			m.message = "No port selected"
			m.mode = modeNormal
			return m, nil
		}

		baud := m.serialCfg.Baud
		if len(m.baudList) > 0 && m.baudCursor < len(m.baudList) {
			baud = m.baudList[m.baudCursor]
		}

		if m.source != nil {
			m.source.Stop()
			m.source = nil
		}
		m.connState = ConnDisconnected
		cfg := m.serialCfg
		cfg.Port = port
		cfg.Baud = baud
		m.serialCfg = cfg
		m.mode = modeNormal
		m.saveSettings()
		return m, connectCmd(cfg)
	}
	return m, nil
}


// handleProfilePickerKey handles navigation and selection inside the profile switcher.
func (m Model) handleProfilePickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case keyMatches(msg, m.keys.Cancel):
		m.mode = modeNormal
		return m, nil

	case keyMatches(msg, m.keys.ScrollUp):
		if m.profileCursor > 0 {
			m.profileCursor--
		}
		return m, nil

	case keyMatches(msg, m.keys.ScrollDown):
		if m.profileCursor < len(m.profileList)-1 {
			m.profileCursor++
		}
		return m, nil

	case keyMatches(msg, m.keys.Confirm):
		if len(m.profileList) == 0 {
			m.mode = modeNormal
			return m, nil
		}
		selected := m.profileList[m.profileCursor]
		if selected.Path == "" {
			// Built-in Raw parser
			m.profile = nil
			m.parser = parser.NewRawParser()
			m.columns = []record.Column{{Field: "message", Title: "Message", Width: 0}}
		} else {
			prof, err := parser.LoadProfile(selected.Path)
			if err != nil {
				m.message = fmt.Sprintf("profile error: %v", err)
				m.mode = modeNormal
				return m, nil
			}
			newParser, err := prof.BuildParser()
			if err != nil {
				m.message = fmt.Sprintf("parser error: %v", err)
				m.mode = modeNormal
				return m, nil
			}
			m.profile = prof
			m.parser = newParser
			m.columns = prof.ToColumns()

			// Update timestamp settings if configured
			if prof.Ingest.Timestamp.Enabled {
				m.tsField = prof.Ingest.Timestamp.TimestampField()
				m.tsFormat = prof.Ingest.Timestamp.TimestampFormat()
				m.showTimestamp = true
			} else {
				for _, col := range prof.Columns {
					if col.Style == "timestamp" || col.Field == "time" || col.Field == "timestamp" {
						m.tsField = col.Field
						m.showTimestamp = true
						break
					}
				}
			}
		}

		// Re-parse all existing records in the buffer using the new parser!
		m.buffer.Transform(func(old record.Record) record.Record {
			newRec, _ := m.parser.Parse(old.Raw)
			if newRec.Fields == nil {
				newRec.Fields = make(map[string]string)
			}
			// Preserve earlier recorded arrival timestamp
			prevTS := old.Fields[m.tsField]
			if prevTS == "" {
				prevTS = old.Fields["_ts"]
			}
			if prevTS == "" {
				prevTS = old.Fields["time"]
			}
			if prevTS != "" {
				if newRec.Fields[m.tsField] == "" {
					newRec.Fields[m.tsField] = prevTS
				}
				if newRec.Fields["_ts"] == "" {
					newRec.Fields["_ts"] = prevTS
				}
			}
			return newRec
		})

		m.rebuildAllTabs()
		m.mode = modeNormal
		m.saveSettings()
		m.message = fmt.Sprintf("Profile: %s", selected.Name)
		return m, nil
	}
	return m, nil
}

// handleHelpKey handles input while the help modal is displayed.
func (m Model) handleHelpKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case keyMatches(msg, m.keys.Help),
		keyMatches(msg, m.keys.Cancel),
		keyMatches(msg, m.keys.Confirm),
		msg.String() == "q":
		m.mode = modeNormal
		return m, nil
	}
	return m, nil
}



// connectCmd opens a new SerialSource asynchronously and returns a
// sourceReadyMsg on success or an ErrorMsg on failure.
func connectCmd(cfg serial.Config) tea.Cmd {
	return func() tea.Msg {
		src, err := serial.NewSerialSource(cfg)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("connect %s: %w", cfg.Port, err)}
		}
		return sourceReadyMsg{source: src, port: cfg.Port}
	}
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func keyMatches(msg tea.KeyMsg, b interface{ Keys() []string }) bool {
	for _, k := range b.Keys() {
		if msg.String() == k {
			return true
		}
	}
	return false
}

// handleTextInput processes printable key presses and backspace for text fields.
func handleTextInput(current string, msg tea.KeyMsg) string {
	switch msg.Type {
	case tea.KeyBackspace, tea.KeyDelete:
		if len(current) > 0 {
			runes := []rune(current)
			return string(runes[:len(runes)-1])
		}
	case tea.KeyRunes:
		for _, r := range msg.Runes {
			if unicode.IsPrint(r) {
				current += string(r)
			}
		}
	case tea.KeySpace:
		current += " "
	}
	return current
}

// rebuildVisible refilters the buffer and updates m.visible and the active tab.
func (m *Model) rebuildVisible() {
	all := m.buffer.All()
	if m.activeFilter == nil || m.activeFilter.Empty() {
		m.visible = all
	} else {
		out := make([]record.Record, 0, len(all))
		for _, r := range all {
			if m.activeFilter.Matches(r) {
				out = append(out, r)
			}
		}
		m.visible = out
	}
	if len(m.tabs) > 0 {
		cur := m.currentTab()
		cur.Visible = m.visible
	}
}

// rebuildAllTabs refilters the buffer across all tabs (e.g. on profile reload).
func (m *Model) rebuildAllTabs() {
	all := m.buffer.All()
	for i := range m.tabs {
		t := &m.tabs[i]
		if t.Filter == nil || t.Filter.Empty() {
			t.Visible = all
		} else {
			out := make([]record.Record, 0, len(all))
			for _, r := range all {
				if t.Filter.Matches(r) {
					out = append(out, r)
				}
			}
			t.Visible = out
		}
	}
	m.syncModelToActiveTab()
	m.clampScroll()
}

func (m Model) handleCreateNewTab() (tea.Model, tea.Cmd) {
	m.syncActiveTabToModel()
	newIdx := len(m.tabs)
	name := fmt.Sprintf("Tab %d", newIdx+1)
	newTab := Tab{
		Name:    name,
		Follow:  true,
		Visible: m.buffer.All(),
	}
	if len(newTab.Visible) > m.tableHeight {
		newTab.ScrollOffset = len(newTab.Visible) - m.tableHeight
	}
	m.tabs = append(m.tabs, newTab)
	m.activeTab = newIdx
	m.syncModelToActiveTab()
	m.recalcLayout()
	m.mode = modeFilter
	m.filterInput = ""
	m.message = fmt.Sprintf("Created %s — enter filter (or Enter/Esc for all)", name)
	return m, nil
}

func (m Model) handleCloseActiveTab() (tea.Model, tea.Cmd) {
	if len(m.tabs) <= 1 {
		m.message = "Cannot close the only tab"
		return m, nil
	}
	closedName := m.tabs[m.activeTab].DisplayName(m.activeTab + 1)
	m.tabs = append(m.tabs[:m.activeTab], m.tabs[m.activeTab+1:]...)
	if m.activeTab >= len(m.tabs) {
		m.activeTab = len(m.tabs) - 1
	}
	m.syncModelToActiveTab()
	m.recalcLayout()
	m.message = fmt.Sprintf("Closed %s", closedName)
	return m, nil
}

func (m *Model) scrollToMatch(idx int) {
	m.follow = false
	// Center the match in the viewport
	half := m.tableHeight / 2
	m.scrollOffset = idx - half
	m.clampScroll()
}

// runSearch finds all visible rows matching the search string.
func (m *Model) runSearch() {
	m.searchMatches = nil
	q := strings.ToLower(m.searchInput)
	if q == "" {
		return
	}
	for i, r := range m.visible {
		for _, v := range r.Fields {
			if strings.Contains(strings.ToLower(v), q) {
				m.searchMatches = append(m.searchMatches, i)
				break
			}
		}
	}
	m.searchCursor = 0
	if len(m.searchMatches) > 0 {
		m.scrollToMatch(m.searchMatches[0])
	}
}

func (m *Model) nextSearchMatch() {
	if len(m.searchMatches) == 0 {
		return
	}
	m.searchCursor = (m.searchCursor + 1) % len(m.searchMatches)
	m.scrollToMatch(m.searchMatches[m.searchCursor])
}

func (m *Model) prevSearchMatch() {
	if len(m.searchMatches) == 0 {
		return
	}
	m.searchCursor = (m.searchCursor - 1 + len(m.searchMatches)) % len(m.searchMatches)
	m.scrollToMatch(m.searchMatches[m.searchCursor])
}

func (m *Model) scrollToBottom() {
	if len(m.visible) > m.tableHeight {
		m.scrollOffset = len(m.visible) - m.tableHeight
	} else {
		m.scrollOffset = 0
	}
}

func (m *Model) clampScroll() {
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
	maxOffset := len(m.visible) - m.tableHeight
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.scrollOffset > maxOffset {
		m.scrollOffset = maxOffset
	}
	if len(m.tabs) > 0 {
		cur := m.currentTab()
		cur.ScrollOffset = m.scrollOffset
		cur.Follow = m.follow
	}
}

func (m *Model) recalcLayout() {
	// Fixed rows: 1 title + 1 divider + 1 header + 1 divider + 1 divider + 1 status + 1 keys = 7 fixed rows.
	// If more than 1 tab is present, tab bar adds 2 rows (1 row tab bar + 1 row divider).
	fixed := 7
	if len(m.tabs) > 1 {
		fixed += 2
	}
	m.tableHeight = m.height - fixed
	if m.tableHeight < 1 {
		m.tableHeight = 1
	}
}



// cmdSaveLog saves all raw lines in the buffer to a timestamped file.
func (m *Model) cmdSaveLog() tea.Cmd {
	return func() tea.Msg {
		all := m.buffer.All()
		name := fmt.Sprintf("oml-%s.log", time.Now().Format("2006-01-02T15-04-05"))
		path := filepath.Join(".", name)
		f, err := os.Create(path)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("save log: %w", err)}
		}
		defer f.Close()
		for _, r := range all {
			fmt.Fprintln(f, r.Raw)
		}
		return ErrorMsg{Err: fmt.Errorf("saved %d records to %s", len(all), path)}
	}
}
