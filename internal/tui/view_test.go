package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brenoniehues/oh-my-logs/internal/config"
	"github.com/brenoniehues/oh-my-logs/internal/parser"
	"github.com/brenoniehues/oh-my-logs/internal/record"
	"github.com/brenoniehues/oh-my-logs/internal/serial"
	tea "github.com/charmbracelet/bubbletea"
)

func newTestModel() Model {
	cfg := serial.Config{Port: "COM1", Baud: 115200}
	p := parser.NewRawParser()
	buf := record.NewBuffer(100)
	appCfg := &config.AppConfig{}
	m := New(cfg, nil, p, buf, nil, appCfg)
	m.width = 100
	m.height = 30
	m.tableHeight = 23
	return m
}

func TestHelpModalToggleAndDismiss(t *testing.T) {
	m := newTestModel()

	if m.mode != modeNormal {
		t.Fatalf("expected initial mode to be modeNormal, got %v", m.mode)
	}

	// Press '?' to open help modal
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = updated.(Model)
	if m.mode != modeHelp {
		t.Fatalf("expected modeHelp after '?', got %v", m.mode)
	}

	// View should render help modal
	viewOutput := m.View()
	if !strings.Contains(viewOutput, "Help — Keyboard & Mouse Shortcuts") {
		t.Errorf("expected view to contain 'Help — Keyboard & Mouse Shortcuts'")
	}
	if !strings.Contains(viewOutput, "NAVIGATION") || !strings.Contains(viewOutput, "ACTIONS & CONTROLS") {
		t.Errorf("expected view to contain help categories")
	}

	// Press '?' again to close
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = updated.(Model)
	if m.mode != modeNormal {
		t.Fatalf("expected modeNormal after second '?', got %v", m.mode)
	}

	// Press '?' to open, then Esc to close
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.mode != modeNormal {
		t.Fatalf("expected modeNormal after Esc, got %v", m.mode)
	}

	// Press '?' to open, then 'q' to close
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	m = updated.(Model)
	if m.mode != modeNormal {
		t.Fatalf("expected modeNormal after 'q', got %v", m.mode)
	}
}

func TestEmptyStateWithoutHeader(t *testing.T) {
	m := newTestModel()

	// Initially disconnected with empty buffer
	m.connState = ConnDisconnected
	v := m.View()

	// Should show empty state message
	if !strings.Contains(v, "No serial device connected") {
		t.Errorf("expected view to contain 'No serial device connected', got:\n%s", v)
	}
	if !strings.Contains(v, "Press p to select a port") {
		t.Errorf("expected view to contain 'Press p to select a port', got:\n%s", v)
	}
	if !strings.Contains(v, "Press ? for shortcuts") {
		t.Errorf("expected view to contain 'Press ? for shortcuts', got:\n%s", v)
	}

	// Table header (e.g. "Message") should NOT be present in empty state
	if strings.Contains(v, "  Message") {
		t.Errorf("table header should be omitted in empty state, but found in view")
	}

	// Switch to connected with empty buffer
	m.connState = ConnConnected
	vConn := m.View()
	if !strings.Contains(vConn, "Waiting for serial data…") {
		t.Errorf("expected view to contain 'Waiting for serial data…', got:\n%s", vConn)
	}
	if strings.Contains(vConn, "  Message") {
		t.Errorf("table header should be omitted in connected empty state")
	}

	// Add record and rebuild visible -> header should now appear
	rec := record.Record{Fields: map[string]string{"message": "hello world"}, Raw: "hello world"}
	m.buffer.Add(rec)
	m.visible = append(m.visible, rec)
	vWithLogs := m.View()
	if !strings.Contains(vWithLogs, "Message") {
		t.Errorf("table header 'Message' should be present when logs exist")
	}
	if !strings.Contains(vWithLogs, "hello world") {
		t.Errorf("log line should be rendered")
	}
}

func TestTimestampPersistentAndToggle(t *testing.T) {
	m := newTestModel()
	m.connState = ConnConnected

	// 1. Initially timestamp display is OFF
	if m.showTimestamp {
		t.Fatalf("expected showTimestamp to be false by default")
	}

	// 2. Incoming log line arrives while timestamp is hidden
	line := "sensor init ok"
	updated, _ := m.Update(lineMsg(line))
	m = updated.(Model)

	if len(m.visible) != 1 {
		t.Fatalf("expected 1 visible record, got %d", len(m.visible))
	}

	// Verify timestamp was captured on the record despite being hidden
	rec := m.visible[0]
	if rec.Fields["_ts"] == "" {
		t.Fatalf("expected incoming record to have arrival timestamp recorded in _ts")
	}

	// View while timestamp is disabled should NOT contain the Time column header
	if strings.Contains(m.viewTableHeader(), "Time") {
		t.Errorf("Time column should be hidden when timestamp is OFF, got header:\n%s", m.viewTableHeader())
	}
	if len(m.effectiveColumns()) != 1 {
		t.Fatalf("expected 1 effective column when OFF, got %d", len(m.effectiveColumns()))
	}

	// 3. Press 't' to toggle timestamp ON
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	m = updated.(Model)
	if !m.showTimestamp {
		t.Fatalf("expected showTimestamp to be true after pressing 't'")
	}

	// View should now display the Time header and the recorded timestamp for the older record
	vOn := m.View()
	if !strings.Contains(m.viewTableHeader(), "Time") {
		t.Errorf("Time column should appear when timestamp is ON, got header:\n%s", m.viewTableHeader())
	}
	if len(m.effectiveColumns()) != 2 {
		t.Fatalf("expected 2 effective columns when ON, got %d", len(m.effectiveColumns()))
	}
	if !strings.Contains(vOn, rec.Fields["_ts"]) {
		t.Errorf("expected view to display the older record's timestamp %q, got:\n%s", rec.Fields["_ts"], vOn)
	}

	// 4. Press 't' again to toggle timestamp OFF
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	m = updated.(Model)
	if m.showTimestamp {
		t.Fatalf("expected showTimestamp to be false after second 't'")
	}

	if strings.Contains(m.viewTableHeader(), "Time") {
		t.Errorf("Time column should be hidden again when toggled OFF, got header:\n%s", m.viewTableHeader())
	}
	if len(m.effectiveColumns()) != 1 {
		t.Fatalf("expected 1 effective column when toggled back OFF, got %d", len(m.effectiveColumns()))
	}
}

func TestTUISettingsPersistence(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "oml-tui-settings-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	appCfg := &config.AppConfig{
		ConfigDir: tempDir,
	}

	cfg := serial.Config{Port: "COM1", Baud: 115200}
	p := parser.NewRawParser()
	buf := record.NewBuffer(100)
	m := New(cfg, nil, p, buf, nil, appCfg)

	// Toggle timestamp -> should save ShowTimestamp: true
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	m = updated.(Model)

	saved, err := appCfg.LoadSettings()
	if err != nil {
		t.Fatalf("LoadSettings failed: %v", err)
	}
	if !saved.ShowTimestamp {
		t.Errorf("expected saved.ShowTimestamp to be true")
	}
	if saved.Port != "COM1" {
		t.Errorf("expected saved.Port to be COM1, got %s", saved.Port)
	}
	if saved.Baud != 115200 {
		t.Errorf("expected saved.Baud to be 115200, got %d", saved.Baud)
	}
}

type dummySource struct {
	lines  chan string
	errors chan error
}

func (d *dummySource) Lines() <-chan string { return d.lines }
func (d *dummySource) Errors() <-chan error { return d.errors }
func (d *dummySource) Stop()                 {}

func TestAutoReconnectLifecycle(t *testing.T) {
	m := newTestModel()
	m.serialCfg.Port = "/dev/ttyUSB0"
	m.connState = ConnDisconnected
	m.reconnecting = true

	// 1. Check title bar shows Reconnecting
	v := m.viewTitleBar()
	if !strings.Contains(v, "Reconnecting") {
		t.Errorf("expected title bar to show 'Reconnecting', got: %s", v)
	}

	// 2. Check empty state displays auto-reconnecting target
	empty := m.viewEmptyState()
	if !strings.Contains(empty, "Auto-reconnecting to") || !strings.Contains(empty, "/dev/ttyUSB0") {
		t.Errorf("expected empty state to mention auto-reconnecting target, got:\n%s", empty)
	}

	// 3. Simulating reconnectFailedMsg returns another tick if still disconnected
	updated, cmd := m.Update(reconnectFailedMsg{})
	m = updated.(Model)
	if !m.reconnecting {
		t.Errorf("expected reconnecting to remain true after reconnectFailedMsg")
	}
	if cmd == nil {
		t.Errorf("expected scheduleReconnectTick cmd to be returned")
	}

	// 4. Simulating sourceReadyMsg connects and resets reconnecting
	lines := make(chan string)
	errs := make(chan error)
	dummySrc := &dummySource{lines: lines, errors: errs}
	updated, cmd = m.Update(sourceReadyMsg{source: dummySrc, port: "/dev/ttyUSB0"})
	m = updated.(Model)
	if m.connState != ConnConnected {
		t.Errorf("expected ConnConnected, got %v", m.connState)
	}
	if m.reconnecting {
		t.Errorf("expected reconnecting to be false after sourceReadyMsg")
	}
	if cmd == nil {
		t.Errorf("expected listen cmds after sourceReadyMsg")
	}

	// 5. Simulating cable unplug via ConnStateMsg(ConnDisconnected)
	updated, cmd = m.Update(ConnStateMsg{State: ConnDisconnected, Detail: "source closed"})
	m = updated.(Model)
	if m.connState != ConnDisconnected {
		t.Errorf("expected ConnDisconnected, got %v", m.connState)
	}
	if !m.reconnecting {
		t.Errorf("expected reconnecting to be true after cable unplug")
	}
	if cmd == nil {
		t.Errorf("expected scheduleReconnectTick cmd on disconnect")
	}

	// 6. Check reconnectFailedMsg with diagnostic reason updates status message
	updated, _ = m.Update(reconnectFailedMsg{reason: "Scanning ports… (target /dev/ttyUSB0 not found)"})
	m = updated.(Model)
	if m.message != "Scanning ports… (target /dev/ttyUSB0 not found)" {
		t.Errorf("expected updated message, got %q", m.message)
	}

	// 7. Simulating ErrorMsg while connected (e.g. fatal USB read error) triggers auto-reconnect
	m.connState = ConnConnected
	m.source = dummySrc
	m.reconnecting = false
	updated, cmd = m.Update(ErrorMsg{Err: fmt.Errorf("read: device not configured")})
	m = updated.(Model)
	if m.connState != ConnDisconnected {
		t.Errorf("expected ErrorMsg to trigger ConnDisconnected, got %v", m.connState)
	}
	if !m.reconnecting {
		t.Errorf("expected reconnecting to be true after ErrorMsg")
	}
	if cmd == nil {
		t.Errorf("expected scheduleReconnectTick cmd after ErrorMsg")
	}

	// 8. Ensure View() does not panic with error message
	_ = m.View()
	updated, _ = m.Update(ErrorMsg{Err: fmt.Errorf("connect /dev/cu.usbserial-1101: serial: cannot open /dev/cu.usbserial-1101: Invalid serial port: error setting term settings: invalid argument")})
	m = updated.(Model)
	_ = m.View()
}

func TestUptimeAndTimestampDistinct(t *testing.T) {
	// Setup a model with Zephyr profile columns (uptime, level, message)
	prof := &parser.Profile{
		Name: "Zephyr",
		Columns: []parser.ColumnConfig{
			{Field: "uptime", Title: "Uptime (s)", Width: 12, Style: "uptime"},
			{Field: "level", Title: "Level", Width: 7, Style: "level"},
			{Field: "message", Title: "Message", Width: 0, Style: "primary"},
		},
	}
	p, err := parser.NewRegexParser(`^\[\s*(?P<uptime>[0-9.]+)\]\s+<(?P<level>[a-zA-Z]+)>\s+(?P<message>.*)$`)
	if err != nil {
		t.Fatalf("failed to build parser: %v", err)
	}

	cfg := serial.Config{Port: "COM1", Baud: 115200}
	buf := record.NewBuffer(100)
	m := New(cfg, prof, p, buf, nil, &config.AppConfig{})
	m.width = 120
	m.height = 30
	m.recalcLayout()
	m.showTimestamp = false // initially timestamp visibility is OFF

	// 1. Initial state: Uptime (s) is visible; Time (arrival ts) is NOT
	colsOff := m.effectiveColumns()
	if len(colsOff) != 3 {
		t.Fatalf("expected 3 columns when timestamp is OFF, got %d", len(colsOff))
	}
	if colsOff[0].Field != "uptime" || colsOff[0].Title != "Uptime (s)" {
		t.Errorf("expected first column to be 'Uptime (s)', got %q (%q)", colsOff[0].Title, colsOff[0].Field)
	}

	// Ingest a Zephyr line
	line := "[      5.182] <inf> fs_nvs: 16 Sectors of 4096 bytes"
	updated, _ := m.Update(lineMsg(line))
	m = updated.(Model)

	if len(m.visible) != 1 {
		t.Fatalf("expected 1 visible record, got %d", len(m.visible))
	}
	rec := m.visible[0]
	if rec.Fields["uptime"] != "5.182" {
		t.Errorf("expected uptime '5.182', got %q", rec.Fields["uptime"])
	}
	if rec.Fields["_ts"] == "" {
		t.Errorf("expected host arrival timestamp to be recorded in '_ts'")
	}

	// View with timestamp OFF: contains uptime, does NOT contain Time header
	vOff := m.View()
	if !strings.Contains(vOff, "Uptime (s)") {
		t.Errorf("Uptime (s) column header should be present when timestamp is OFF")
	}
	if !strings.Contains(vOff, "5.182") {
		t.Errorf("Uptime value '5.182' should be present when timestamp is OFF")
	}
	if strings.Contains(m.viewTableHeader(), "Time") {
		t.Errorf("Time column header should NOT be present when timestamp is OFF")
	}

	// 2. Press 't' to toggle timestamp ON
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	m = updated.(Model)
	if !m.showTimestamp {
		t.Fatalf("expected showTimestamp to be true after pressing 't'")
	}

	// Columns when timestamp is ON: [Time, Uptime (s), Level, Message]
	colsOn := m.effectiveColumns()
	if len(colsOn) != 4 {
		t.Fatalf("expected 4 columns when timestamp is ON, got %d", len(colsOn))
	}
	if colsOn[0].Field != "_ts" || colsOn[0].Title != "Time" {
		t.Errorf("expected first column to be 'Time' (_ts), got %q (%q)", colsOn[0].Title, colsOn[0].Field)
	}
	if colsOn[1].Field != "uptime" || colsOn[1].Title != "Uptime (s)" {
		t.Errorf("expected second column to remain 'Uptime (s)', got %q (%q)", colsOn[1].Title, colsOn[1].Field)
	}

	vOn := m.View()
	if !strings.Contains(m.viewTableHeader(), "Time") {
		t.Errorf("Time column header should be present when timestamp is ON")
	}
	if !strings.Contains(vOn, "Uptime (s)") {
		t.Errorf("Uptime (s) column header should still be present when timestamp is ON")
	}
	if !strings.Contains(vOn, rec.Fields["_ts"]) {
		t.Errorf("expected view to render arrival timestamp %q", rec.Fields["_ts"])
	}
	if !strings.Contains(vOn, "5.182") {
		t.Errorf("expected view to render uptime 5.182 alongside arrival timestamp")
	}

	// 3. Press 't' again to toggle timestamp OFF
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	m = updated.(Model)
	if m.showTimestamp {
		t.Fatalf("expected showTimestamp to be false after second 't'")
	}
	if len(m.effectiveColumns()) != 3 {
		t.Fatalf("expected 3 columns when toggled back OFF, got %d", len(m.effectiveColumns()))
	}
	if !strings.Contains(m.View(), "5.182") {
		t.Errorf("uptime should remain visible when timestamp is toggled back OFF")
	}
}

func TestPortPickerModalRendering(t *testing.T) {
	m := newTestModel()
	m.mode = modePortPicker
	m.portList = []string{"/dev/ttyUSB0", "/dev/ttyUSB1", "/dev/ttyUSB2"}
	m.baudList = []int{9600, 57600, 115200}
	m.portCursor = 0
	m.baudCursor = 2 // 115200

	v := m.View()
	if !strings.Contains(v, "Serial Port") {
		t.Errorf("expected view to contain 'Serial Port', got:\n%s", v)
	}
	if !strings.Contains(v, "Baud rate") {
		t.Errorf("expected view to contain 'Baud rate', got:\n%s", v)
	}
	if !strings.Contains(v, "←") || !strings.Contains(v, "→") {
		t.Errorf("expected view to contain left/right arrows for baud rate, got:\n%s", v)
	}
	if !strings.Contains(v, "115200") {
		t.Errorf("expected view to contain 115200, got:\n%s", v)
	}
	if !strings.Contains(v, "/dev/ttyUSB0") {
		t.Errorf("expected view to contain '/dev/ttyUSB0', got:\n%s", v)
	}
	if !strings.Contains(v, "Enter select · ↑/↓ port · ←/→ baud · Esc cancel") {
		t.Errorf("expected footer hints for port and baud navigation, got:\n%s", v)
	}

	// Test Right arrow wraps baud cursor around
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = updated.(Model)
	if m.baudCursor != 0 {
		t.Errorf("expected baudCursor to wrap to 0, got %d", m.baudCursor)
	}

	// Test Left arrow wraps back
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m = updated.(Model)
	if m.baudCursor != 2 {
		t.Errorf("expected baudCursor to wrap to 2, got %d", m.baudCursor)
	}

	// Test Down arrow navigates port cursor
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(Model)
	if m.portCursor != 1 {
		t.Errorf("expected portCursor to be 1, got %d", m.portCursor)
	}

	// Test Up arrow navigates port cursor back
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = updated.(Model)
	if m.portCursor != 0 {
		t.Errorf("expected portCursor to be 0, got %d", m.portCursor)
	}

	// Test mouse wheel down scrolls port
	updated, _ = m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelDown})
	m = updated.(Model)
	if m.portCursor != 1 {
		t.Errorf("expected mouse wheel down to increment portCursor to 1, got %d", m.portCursor)
	}

	// Test mouse wheel up scrolls port back
	updated, _ = m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelUp})
	m = updated.(Model)
	if m.portCursor != 0 {
		t.Errorf("expected mouse wheel up to decrement portCursor to 0, got %d", m.portCursor)
	}
}

func TestProfilePickerModalRendering(t *testing.T) {
	m := newTestModel()
	globalDir := filepath.Join(os.TempDir(), "global_profiles")
	m.appConfig = &config.AppConfig{
		ProfilesDir: globalDir,
	}
	m.mode = modeProfilePicker
	m.profileList = []ProfileItem{
		{Name: "Raw", Path: ""},
		{Name: "Zephyr", Path: filepath.Join(globalDir, "zephyr.yaml")},
		{Name: "LocalDev", Path: filepath.Join(os.TempDir(), "workspace", "localdev.yaml")},
	}
	m.profileCursor = 0

	v := m.View()
	if !strings.Contains(v, "Profile") {
		t.Errorf("expected view to contain 'Profile', got:\n%s", v)
	}
	if !strings.Contains(v, "[local]") {
		t.Errorf("expected local profile to have '[local]' tag, got:\n%s", v)
	}
	if strings.Contains(v, "[global]") {
		t.Errorf("modal should not display '[global]', got:\n%s", v)
	}
	if strings.Contains(v, "Raw [local]") {
		t.Errorf("built-in Raw should not have '[local]' tag")
	}
	if strings.Contains(v, "Zephyr [local]") {
		t.Errorf("global profile Zephyr should not have '[local]' tag")
	}
	if !strings.Contains(v, "Enter select · ↑/↓ navigate · Esc cancel") {
		t.Errorf("expected footer hints for profile navigation, got:\n%s", v)
	}

	// Down arrow navigates profile cursor
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(Model)
	if m.profileCursor != 1 {
		t.Errorf("expected profileCursor 1, got %d", m.profileCursor)
	}

	// Mouse wheel down scrolls profile cursor
	updated, _ = m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelDown})
	m = updated.(Model)
	if m.profileCursor != 2 {
		t.Errorf("expected mouse wheel down to advance profileCursor to 2, got %d", m.profileCursor)
	}

	// Mouse wheel up scrolls profile cursor back
	updated, _ = m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelUp})
	m = updated.(Model)
	if m.profileCursor != 1 {
		t.Errorf("expected mouse wheel up to decrement profileCursor to 1, got %d", m.profileCursor)
	}
}



