package tui

import (
	"fmt"
	"time"

	"github.com/brenoniehues/oh-my-logs/internal/game"
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
		m.nextRecordID++
		r.ID = m.nextRecordID
		m.buffer.Add(r)
		if m.mode == modeGame {
			m.logsDuringGame++
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
			switch m.mode {
			case modePortPicker:
				if m.portCursor > 0 {
					m.portCursor--
				}
			case modeProfilePicker:
				if m.profileCursor > 0 {
					m.profileCursor--
				}
			case modeHelp, modeGame:
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
			switch m.mode {
			case modePortPicker:
				if m.portCursor < len(m.portList)-1 {
					m.portCursor++
				}
			case modeProfilePicker:
				if m.profileCursor < len(m.profileList)-1 {
					m.profileCursor++
				}
			case modeHelp, modeGame:
				// ignore scrolling while modal or game is active
			default:
				m.scrollOffset += 3
				h := m.activeDataHeight()
				if m.scrollOffset >= len(m.visible)-h {
					m.follow = true
				}
				m.clampScroll()
				if m.splitMode != SplitNone && m.syncScroll {
					m.syncOtherPaneChronologically()
				}
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

	return m, nil
}
