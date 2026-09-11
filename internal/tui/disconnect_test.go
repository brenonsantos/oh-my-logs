package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/brenoniehues/oh-my-logs/internal/record"
	"github.com/brenoniehues/oh-my-logs/internal/serial"
	tea "github.com/charmbracelet/bubbletea"
)

type mockCloseSource struct {
	lines   chan string
	errors  chan error
	stopped bool
}

func newMockCloseSource() *mockCloseSource {
	return &mockCloseSource{
		lines:  make(chan string, 10),
		errors: make(chan error, 10),
	}
}

func (m *mockCloseSource) Lines() <-chan string { return m.lines }
func (m *mockCloseSource) Errors() <-chan error { return m.errors }
func (m *mockCloseSource) Stop()                 { m.stopped = true }
func (m *mockCloseSource) Write(p []byte) (int, error) {
	return len(p), nil
}

func TestDisconnect_KeyD_ReleasesSourceAndPreservesBuffer(t *testing.T) {
	m := newTestModel()
	mockSrc := newMockCloseSource()
	m.source = mockSrc
	m.connState = ConnConnected
	m.serialCfg = serial.Config{Port: "/dev/ttyUSB0", Baud: 115200}
	m.reconnecting = false

	// Ingest sample records
	m.ingestRecord(record.Record{Raw: "log 1", Fields: map[string]string{"message": "log 1"}})
	m.ingestRecord(record.Record{Raw: "log 2", Fields: map[string]string{"message": "log 2"}})
	if m.buffer.Len() != 2 {
		t.Fatalf("expected 2 records in buffer, got %d", m.buffer.Len())
	}

	// Press 'D' in normal mode
	mMod, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}})
	m = mMod.(Model)

	if cmd != nil {
		t.Fatalf("expected nil cmd from disconnect, got %v", cmd)
	}
	if !mockSrc.stopped {
		t.Fatalf("expected mock source Stop() to be called on disconnect")
	}
	if m.source != nil {
		t.Fatalf("expected m.source to be nil after disconnect")
	}
	if m.connState != ConnDisconnected {
		t.Fatalf("expected connState to be ConnDisconnected, got %v", m.connState)
	}
	if m.reconnecting {
		t.Fatalf("expected reconnecting to be false after explicit disconnect")
	}
	if !strings.Contains(m.message, "Disconnected from /dev/ttyUSB0") || !strings.Contains(m.message, "port released") {
		t.Fatalf("unexpected message after disconnect: %q", m.message)
	}

	// Verify buffer & records are completely preserved
	if m.buffer.Len() != 2 {
		t.Fatalf("expected buffer to still have 2 records after disconnect, got %d", m.buffer.Len())
	}
	if len(m.visible) != 2 {
		t.Fatalf("expected visible records to remain 2, got %d", len(m.visible))
	}
}

func TestDisconnect_FromPortPickerModal(t *testing.T) {
	m := newTestModel()
	mockSrc := newMockCloseSource()
	m.source = mockSrc
	m.connState = ConnConnected
	m.serialCfg = serial.Config{Port: "COM3", Baud: 115200}
	m.mode = modePortPicker

	// Press 'd' inside port picker modal
	mMod, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = mMod.(Model)

	if m.mode != modeNormal {
		t.Fatalf("expected mode to return to modeNormal, got %v", m.mode)
	}
	if !mockSrc.stopped {
		t.Fatalf("expected source to be stopped")
	}
	if m.source != nil {
		t.Fatalf("expected m.source to be nil")
	}
	if m.connState != ConnDisconnected {
		t.Fatalf("expected ConnDisconnected, got %v", m.connState)
	}
	if m.reconnecting {
		t.Fatalf("expected reconnecting to be false")
	}
	if !strings.Contains(m.message, "Disconnected from COM3") {
		t.Fatalf("unexpected message: %q", m.message)
	}
}

func TestDisconnect_AlreadyDisconnected(t *testing.T) {
	m := newTestModel()
	m.source = nil
	m.connState = ConnDisconnected
	m.reconnecting = false

	mMod, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}})
	m = mMod.(Model)

	if m.message != "Already disconnected" {
		t.Fatalf("expected 'Already disconnected' message, got %q", m.message)
	}
}

func TestDisconnect_FileSourceGuarded(t *testing.T) {
	m := newTestModel()
	mockSrc := newMockCloseSource()
	m.source = mockSrc
	m.isFileSource = true
	m.connState = ConnConnected

	mMod, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}})
	m = mMod.(Model)

	if mockSrc.stopped {
		t.Fatalf("file source should not be stopped on 'D'")
	}
	if m.source == nil {
		t.Fatalf("file source should not be cleared")
	}
	if !strings.Contains(m.message, "Replay of offline log file — cannot disconnect") {
		t.Fatalf("expected file source guard message, got %q", m.message)
	}
}

func TestReconnect_KeyR_InitiatesScanning(t *testing.T) {
	m := newTestModel()
	m.source = nil
	m.connState = ConnDisconnected
	m.serialCfg = serial.Config{Port: "/dev/ttyUSB0", Baud: 115200}
	m.reconnecting = false

	// Press 'r'
	mMod, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = mMod.(Model)

	if cmd == nil {
		t.Fatalf("expected non-nil cmd from Reconnect")
	}
	if !m.reconnecting {
		t.Fatalf("expected reconnecting to be true after 'r'")
	}
	if !strings.Contains(m.message, "Reconnecting to /dev/ttyUSB0") {
		t.Fatalf("unexpected reconnect message: %q", m.message)
	}
}

func TestReconnect_NoPortConfigured(t *testing.T) {
	m := newTestModel()
	m.source = nil
	m.connState = ConnDisconnected
	m.serialCfg = serial.Config{Port: "", Baud: 115200}

	mMod, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = mMod.(Model)

	if cmd != nil {
		t.Fatalf("expected nil cmd when no port configured")
	}
	if !strings.Contains(m.message, "No port configured") {
		t.Fatalf("unexpected message: %q", m.message)
	}
}

func TestViewBars_DisconnectedRendering(t *testing.T) {
	m := newTestModel()
	m.serialCfg = serial.Config{Port: "COM1", Baud: 115200}
	m.width = 100
	m.height = 30
	m.tableHeight = 23

	// 1. Connected state
	m.connState = ConnConnected
	keyBar := m.viewKeyBar()
	if !strings.Contains(keyBar, "D") || !strings.Contains(keyBar, "disconnect") {
		t.Errorf("expected keyBar to contain 'D disconnect' when connected, got:\n%s", keyBar)
	}

	titleBar := m.viewTitleBar()
	if !strings.Contains(titleBar, "Connected") {
		t.Errorf("expected titleBar to contain 'Connected', got:\n%s", titleBar)
	}

	// 2. Disconnected state
	m.connState = ConnDisconnected
	m.source = nil
	m.reconnecting = false

	keyBar = m.viewKeyBar()
	if !strings.Contains(keyBar, "r") || !strings.Contains(keyBar, "reconnect") {
		t.Errorf("expected keyBar to contain 'r reconnect' when disconnected, got:\n%s", keyBar)
	}

	titleBar = m.viewTitleBar()
	if !strings.Contains(titleBar, "Disconnected") {
		t.Errorf("expected titleBar to contain 'Disconnected', got:\n%s", titleBar)
	}

	emptyState := m.viewEmptyState()
	if !strings.Contains(emptyState, "Ready to connect") || !strings.Contains(emptyState, "COM1") {
		t.Errorf("expected emptyState to announce ready to connect for COM1, got:\n%s", emptyState)
	}

	// 3. Port released state (e.g. after pressing 'D')
	m.connDetail = "Port released"
	emptyStateReleased := m.viewEmptyState()
	if !strings.Contains(emptyStateReleased, "Port released") || !strings.Contains(emptyStateReleased, "COM1") {
		t.Errorf("expected emptyState to announce port released for COM1, got:\n%s", emptyStateReleased)
	}
}

func TestDisconnect_SuppressesSubsequentAutoReconnectMessages(t *testing.T) {
	m := newTestModel()
	mockSrc := newMockCloseSource()
	m.source = mockSrc
	m.connState = ConnConnected
	m.serialCfg = serial.Config{Port: "/dev/ttyUSB0", Baud: 115200}
	m.reconnecting = false

	// 1. User explicitly disconnects
	mMod, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}})
	m = mMod.(Model)

	if !m.manualDisconnect {
		t.Fatalf("expected manualDisconnect to be true")
	}
	if m.reconnecting {
		t.Fatalf("expected reconnecting to be false")
	}
	expectedMsg := "Disconnected from /dev/ttyUSB0 — port released (r to reconnect)"
	if m.message != expectedMsg {
		t.Fatalf("expected message %q, got %q", expectedMsg, m.message)
	}

	// 2. Background goroutine finishes and emits ConnStateMsg
	mMod, cmd := m.Update(ConnStateMsg{State: ConnDisconnected, Detail: "source closed"})
	m = mMod.(Model)
	if m.reconnecting {
		t.Fatalf("expected reconnecting to remain false after ConnStateMsg, but got true")
	}
	if cmd != nil {
		t.Fatalf("expected no command from ConnStateMsg when manually disconnected, got %v", cmd)
	}
	if m.message != expectedMsg {
		t.Fatalf("expected message not to be overwritten by ConnStateMsg, got %q", m.message)
	}

	// 3. Serial reader error message arrives from port closure
	mMod, cmd = m.Update(ErrorMsg{Err: errors.New("serial read: bad file descriptor")})
	m = mMod.(Model)
	if m.reconnecting {
		t.Fatalf("expected reconnecting to remain false after ErrorMsg, but got true")
	}
	if cmd != nil {
		t.Fatalf("expected no command from ErrorMsg when manually disconnected, got %v", cmd)
	}
	if m.message != expectedMsg {
		t.Fatalf("expected message not to be overwritten by ErrorMsg, got %q", m.message)
	}

	// 4. Any leftover tick or failed message should abort without starting reconnect
	mMod, cmd = m.Update(reconnectTickMsg{})
	m = mMod.(Model)
	if m.reconnecting {
		t.Fatalf("expected reconnectTickMsg to be suppressed, but reconnecting is true")
	}
	if cmd != nil {
		t.Fatalf("expected reconnectTickMsg to return nil cmd, got %v", cmd)
	}

	mMod, cmd = m.Update(reconnectFailedMsg{reason: "Device disconnected"})
	m = mMod.(Model)
	if m.reconnecting {
		t.Fatalf("expected reconnectFailedMsg to be suppressed, but reconnecting is true")
	}
	if cmd != nil {
		t.Fatalf("expected reconnectFailedMsg to return nil cmd, got %v", cmd)
	}

	// 5. User explicitly presses 'r' to reconnect
	mMod, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = mMod.(Model)
	if m.manualDisconnect {
		t.Fatalf("expected manualDisconnect to be reset to false after 'r'")
	}
	if !m.reconnecting {
		t.Fatalf("expected reconnecting to be true after 'r'")
	}
	if cmd == nil {
		t.Fatalf("expected tryReconnectCmd to be scheduled after 'r'")
	}
}

func TestDefaultDisconnectedOnLaunch(t *testing.T) {
	cfg := serial.Config{Port: "/dev/ttyUSB0", Baud: 115200}
	buf := record.NewBuffer(100)
	m := New(cfg, nil, nil, buf, nil, nil)
	m.width = 100
	m.height = 30
	m.tableHeight = 23

	// 1. Verify initial model state is disconnected and not auto-reconnecting
	if m.connState != ConnDisconnected {
		t.Errorf("expected connState to be ConnDisconnected, got %v", m.connState)
	}
	if m.reconnecting {
		t.Errorf("expected reconnecting to be false on launch, got true")
	}
	if !m.manualDisconnect {
		t.Errorf("expected manualDisconnect to be true on launch without active source")
	}

	// 2. Init() should NOT schedule any reconnect ticks
	initCmd := m.Init()
	if initCmd != nil {
		t.Errorf("expected Init() to return nil when launched disconnected, got %v", initCmd)
	}

	// 3. View should show ready to connect with the configured port
	emptyView := m.viewEmptyState()
	if !strings.Contains(emptyView, "Ready to connect (/dev/ttyUSB0)") {
		t.Errorf("expected empty state to show 'Ready to connect (/dev/ttyUSB0)', got:\n%s", emptyView)
	}
	if !strings.Contains(emptyView, "Press r to connect") {
		t.Errorf("expected empty state to show 'Press r to connect', got:\n%s", emptyView)
	}

	// 4. Pressing 'r' initiates connection
	mMod, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = mMod.(Model)
	if m.manualDisconnect {
		t.Errorf("expected manualDisconnect to be reset to false after 'r'")
	}
	if !m.reconnecting {
		t.Errorf("expected reconnecting to be true after pressing 'r'")
	}
	if cmd == nil {
		t.Errorf("expected tryReconnectCmd to be returned after pressing 'r'")
	}
}
