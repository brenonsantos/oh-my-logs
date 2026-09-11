package tui

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/brenoniehues/oh-my-logs/internal/game"
	"github.com/brenoniehues/oh-my-logs/internal/record"
	"github.com/brenoniehues/oh-my-logs/internal/serial"
	"github.com/brenoniehues/oh-my-logs/internal/timing"
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

// disconnect explicitly closes the active serial port and halts auto-reconnect,
// disconnect explicitly closes the active serial port and halts auto-reconnect,
// freeing the hardware port for external flashing tools without quitting the app.
func (m Model) disconnect() (Model, tea.Cmd) {
	if m.isFileSource {
		m.message = "Replay of offline log file — cannot disconnect"
		return m, nil
	}
	if m.connState == ConnDisconnected && m.source == nil && !m.reconnecting {
		m.manualDisconnect = true
		m.message = "Already disconnected"
		return m, nil
	}
	m.manualDisconnect = true
	m.reconnecting = false
	if m.source != nil {
		m.source.Stop()
		m.source = nil
	}
	m.connState = ConnDisconnected
	m.connDetail = "Disconnected manually"
	if m.serialCfg.Port != "" {
		m.message = fmt.Sprintf("Disconnected from %s — port released (r to reconnect)", m.serialCfg.Port)
	} else {
		m.message = "Disconnected — port released (r to reconnect)"
	}
	return m, nil
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
		m.clampScrollX()
		if m.mode == modeFilePicker {
			m.filePicker.SetHeight(m.filePickerHeight())
		}
		return m, nil

	// ── New line from source ─────────────────────────────────────────────────
	case lineMsg:
		r, _ := m.parser.Parse(string(msg))
		m.ingestRecord(r)
		return m, listenToSource(m.source)

	// ── Log save completed ───────────────────────────────────────────────────
	case LogSavedMsg:
		if msg.Err != nil {
			m.message = fmt.Sprintf("Save log failed: %v", msg.Err)
		} else {
			m.message = fmt.Sprintf("✓ Saved %d records to %s", msg.Count, filepath.Base(msg.Path))
		}
		return m, nil

	// ── Source error ─────────────────────────────────────────────────────────
	case ErrorMsg:
		if !m.manualDisconnect {
			m.message = msg.Err.Error()
		}
		if m.connState == ConnConnected {
			// A fatal read error occurred on active connection (e.g. cable unplugged)
			m.connState = ConnDisconnected
			m.connDetail = msg.Err.Error()
			if m.source != nil {
				m.source.Stop()
				m.source = nil
			}
			if !m.manualDisconnect && m.serialCfg.Port != "" && !m.isFileSource {
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
		if !m.manualDisconnect {
			m.connState = msg.State
			m.connDetail = msg.Detail
		}
		if msg.State == ConnDisconnected {
			if m.source != nil {
				m.source.Stop()
				m.source = nil
			}
			if !m.manualDisconnect && m.serialCfg.Port != "" && !m.isFileSource {
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
		if m.manualDisconnect || m.connState == ConnConnected || m.serialCfg.Port == "" || m.isFileSource {
			m.reconnecting = false
			return m, nil
		}
		m.reconnecting = true
		return m, tryReconnectCmd(m.serialCfg)

	case reconnectFailedMsg:
		if m.manualDisconnect {
			m.reconnecting = false
			return m, nil
		}
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
		m.manualDisconnect = false
		m.mode = modeNormal
		m.message = fmt.Sprintf("Connected to %s", msg.port)
		m.saveSettings()
		return m, tea.Batch(listenToSource(m.source), listenToSourceErrors(m.source))

	// ── Mini-game physics tick ───────────────────────────────────────────────
	case game.TickMsg:
		if m.mode == modeGame && m.activeGame != nil {
			var cmd tea.Cmd
			m.activeGame, cmd = m.activeGame.Update(msg)
			return m, cmd
		}
		return m, nil

	// ── Mouse events ─────────────────────────────────────────────────────────
	case tea.MouseMsg:
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			if msg.Shift {
				if m.scrollX > 0 {
					m.scrollX -= 6
					m.clampScrollX()
				}
				if m.splitMode != SplitNone {
					m.syncActiveTabToModel()
				}
				return m, nil
			}
			switch m.mode {
			case modePortPicker:
				if m.portCursor > 0 {
					m.portCursor--
				}
			case modeProfilePicker:
				if m.profileCursor > 0 {
					m.profileCursor--
				}
			case modeFilterPresets:
				if m.presetCursor > 0 {
					m.presetCursor--
				}
			case modeRowDetail:
				if m.detailScrollOffset > 0 {
					m.detailScrollOffset -= 2
					if m.detailScrollOffset < 0 {
						m.detailScrollOffset = 0
					}
				}
			case modeHelp, modeGame, modeSavePresetPrompt:
				// ignore scrolling while modal or game is active
			default:
				m.follow = false
				m.scrollOffset -= 3
				m.clampScroll()
				if m.splitMode != SplitNone && m.syncScroll {
					m.syncOtherPaneChronologically()
				}
			}
			return m, nil

		case tea.MouseButtonWheelDown:
			if msg.Shift {
				m.scrollX += 6
				m.clampScrollX()
				if m.splitMode != SplitNone {
					m.syncActiveTabToModel()
				}
				return m, nil
			}
			switch m.mode {
			case modePortPicker:
				if m.portCursor < len(m.portList)-1 {
					m.portCursor++
				}
			case modeProfilePicker:
				if m.profileCursor < len(m.profileList)-1 {
					m.profileCursor++
				}
			case modeFilterPresets:
				if m.filtersCfg != nil && m.presetCursor < len(m.filtersCfg.Presets)-1 {
					m.presetCursor++
				}
			case modeRowDetail:
				m.detailScrollOffset += 2
			case modeHelp, modeGame, modeSavePresetPrompt:
				// ignore scrolling while modal or game is active
			default:
				m.scrollOffset += 3
				m.clampScroll()
				h := m.activeDataHeight()
				if m.scrollOffset >= len(m.visible)-h {
					m.follow = true
				} else {
					m.follow = false
				}
				if m.splitMode != SplitNone && m.syncScroll {
					m.syncOtherPaneChronologically()
				}
			}
			return m, nil

		case tea.MouseButtonWheelLeft:
			if m.scrollX > 0 {
				m.scrollX -= 6
				m.clampScrollX()
			}
			if m.splitMode != SplitNone {
				m.syncActiveTabToModel()
			}
			return m, nil

		case tea.MouseButtonWheelRight:
			m.scrollX += 6
			m.clampScrollX()
			if m.splitMode != SplitNone {
				m.syncActiveTabToModel()
			}
			return m, nil

		default:
			switch msg.Action {
			case tea.MouseActionPress:
				if msg.Button == tea.MouseButtonLeft {
					return m.handleMousePress(msg)
				}
			case tea.MouseActionMotion:
				return m.handleMouseMotion(msg)
			case tea.MouseActionRelease:
				return m.handleMouseRelease(msg)
			}
		}

	// ── Key events ───────────────────────────────────────────────────────────
	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	// ── FilePicker background messages (e.g. readDirMsg) ────────────────────
	if m.mode == modeFilePicker {
		var cmd tea.Cmd
		m.filePicker, cmd = m.filePicker.Update(msg)
		return m, cmd
	}

	return m, nil
}

var timestampLayouts = []string{
	"15:04:05.000000000",
	"15:04:05.000000",
	"15:04:05.000",
	"15:04:05",
	"2006-01-02 15:04:05.000000",
	"2006-01-02 15:04:05.000",
	"2006-01-02 15:04:05",
	time.RFC3339Nano,
	time.RFC3339,
}

func parseTimeString(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	// Try parsing as float seconds (e.g. uptime "5.182" or "123.456789")
	if sec, err := strconv.ParseFloat(s, 64); err == nil && sec >= 0 {
		return time.Unix(0, int64(sec*float64(time.Second)))
	}

	// Normalize comma-separated milliseconds/microseconds (e.g. "00:00:03.165,977" -> "00:00:03.165977")
	normalized := strings.ReplaceAll(s, ",", ".")

	for _, layout := range timestampLayouts {
		if t, err := time.Parse(layout, normalized); err == nil {
			return t
		}
	}
	return time.Time{}
}

func tryParseRecordTimestamp(r record.Record, configuredField string) time.Time {
	candidates := []string{configuredField, "time", "timestamp", "ts", "_ts", "uptime"}
	for _, k := range candidates {
		if k == "" {
			continue
		}
		if v, ok := r.Fields[k]; ok && v != "" {
			if t := parseTimeString(v); !t.IsZero() {
				return t
			}
		}
	}

	// Also check if r.Raw starts with a bracketed timestamp: e.g. [12:00:01.234]
	raw := strings.TrimSpace(r.Raw)
	if strings.HasPrefix(raw, "[") {
		if idx := strings.Index(raw, "]"); idx > 1 {
			if t := parseTimeString(raw[1:idx]); !t.IsZero() {
				return t
			}
		}
	}
	return time.Time{}
}

// ingestRecord processes a newly parsed or generated record, stamps arrival metadata,
// pushes it into the ring buffer, and dispatches it to all active virtual tabs.
func (m *Model) ingestRecord(r record.Record) {
	if r.Fields == nil {
		r.Fields = make(map[string]string)
	}
	ts := r.Timestamp
	if ts.IsZero() {
		ts = tryParseRecordTimestamp(r, m.tsField)
		if ts.IsZero() {
			ts = time.Now()
		}
		r.Timestamp = ts
	}
	nowStr := ts.Format(m.tsFormat)
	if !m.lastRecordTime.IsZero() {
		sub := ts.Sub(m.lastRecordTime)
		if sub < 0 && sub > -24*time.Hour && m.lastRecordTime.Year() == 0 {
			sub += 24 * time.Hour
		}
		r.Delta = sub
		r.Fields["_delta"] = timing.FormatDelta(r.Delta)
	} else {
		r.Fields["_delta"] = "---"
	}
	m.lastRecordTime = ts
	if m.deltaTracker != nil && r.Delta > 0 {
		m.deltaTracker.Update(r.Delta)
	}

	if r.Fields[m.tsField] == "" {
		r.Fields[m.tsField] = nowStr
	}
	if r.Fields["_ts"] == "" {
		r.Fields["_ts"] = nowStr
	}

	m.nextRecordID++
	r.ID = m.nextRecordID
	m.buffer.Add(r)
	if m.mode == modeGame {
		m.logsDuringGame++
	}

	if m.diskLogger != nil && m.diskLogger.IsActive() {
		raw := r.Raw
		if raw == "" && r.Fields != nil {
			raw = r.Fields["message"]
		}
		if raw != "" {
			m.diskLogger.WriteLine(raw)
		}
	}

	// Dispatch to all tabs
	if len(m.tabs) == 0 {
		_ = m.currentTab()
	}
	for i := range m.tabs {
		if m.tabs[i].BookmarkedOnly {
			continue
		}
		if m.tabs[i].Filter == nil || m.tabs[i].Filter.Empty() || m.tabs[i].Filter.Matches(r) {
			m.tabs[i].Visible = append(m.tabs[i].Visible, r)
			if m.tabs[i].Follow && !m.paused {
				h := m.tableHeight
				if m.splitMode == SplitHorizontal {
					if i == m.paneTabIdx(0) {
						h = m.paneDataHeight(0)
					} else if i == m.paneTabIdx(1) {
						h = m.paneDataHeight(1)
					}
				}
				if len(m.tabs[i].Visible) > h {
					m.tabs[i].ScrollOffset = len(m.tabs[i].Visible) - h
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
}

// recordTXMessage adds an outbound transmission record to the log buffer for chronological correlation.
func (m *Model) recordTXMessage(cmdText string) {
	endingStr := m.txEnding.String()
	raw := fmt.Sprintf("TX [%s] › %s", endingStr, cmdText)
	if cmdText == "" {
		raw = fmt.Sprintf("TX [%s] ›", endingStr)
	}
	r := record.NewRecord(raw)
	r.Fields["message"] = raw
	r.Fields["level"] = "TX"
	r.Fields["ending"] = endingStr
	r.Fields["raw"] = raw
	r.Fields["cmd"] = cmdText
	m.ingestRecord(r)
}
