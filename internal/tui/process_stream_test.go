package tui

import (
	"runtime"
	"strings"
	"testing"

	"github.com/brenoniehues/oh-my-logs/internal/record"
	"github.com/brenoniehues/oh-my-logs/internal/serial"
	tea "github.com/charmbracelet/bubbletea"
)

func TestProcessSource_TUI_Initialization(t *testing.T) {
	cmdStr := "echo hello"
	if runtime.GOOS == "windows" {
		cmdStr = "cmd /c echo hello"
	}

	src, err := serial.NewProcessSource(cmdStr)
	if err != nil {
		t.Fatalf("failed to create process source: %v", err)
	}
	defer src.Stop()

	buf := record.NewBuffer(100)
	cfg := serial.DefaultConfig()
	m := New(cfg, nil, nil, buf, src, nil)
	m.width = 120
	m.height = 30
	m.tableHeight = 23

	if !m.isProcessSource {
		t.Errorf("expected isProcessSource to be true")
	}
	if m.processCmd != cmdStr {
		t.Errorf("expected processCmd %q, got %q", cmdStr, m.processCmd)
	}
	if m.connState != ConnConnected {
		t.Errorf("expected connState ConnConnected, got %v", m.connState)
	}

	// 1. Verify title bar
	tb := m.viewTitleBar()
	if !strings.Contains(tb, "Cmd:") || !strings.Contains(tb, "Running") {
		t.Errorf("expected title bar to show Cmd and Running, got:\n%s", tb)
	}
	if strings.Contains(tb, "Baud:") {
		t.Errorf("title bar should not display Baud for process source, got:\n%s", tb)
	}

	// 2. Verify empty state while waiting for output
	empty := m.viewEmptyState()
	if !strings.Contains(empty, "Waiting for process output…") {
		t.Errorf("expected empty state to indicate waiting for process output, got:\n%s", empty)
	}
}

func TestProcessSource_TUI_ExitAndRestart(t *testing.T) {
	cmdStr := "echo restart_test"
	src, err := serial.NewProcessSource(cmdStr)
	if err != nil {
		t.Fatalf("failed to create process source: %v", err)
	}
	defer src.Stop()

	buf := record.NewBuffer(100)
	cfg := serial.DefaultConfig()
	m := New(cfg, nil, nil, buf, src, nil)
	m.width = 120
	m.height = 30
	m.tableHeight = 23

	// 1. Simulate process termination
	mMod, _ := m.Update(ConnStateMsg{State: ConnDisconnected, Detail: "source closed"})
	m = mMod.(Model)

	if m.connState != ConnDisconnected {
		t.Errorf("expected ConnDisconnected after process exit, got %v", m.connState)
	}
	if m.reconnecting {
		t.Errorf("expected reconnecting to be false after process exit (manual action needed)")
	}

	tb := m.viewTitleBar()
	if !strings.Contains(tb, "Stopped") {
		t.Errorf("expected title bar to show Stopped, got:\n%s", tb)
	}

	empty := m.viewEmptyState()
	if !strings.Contains(empty, "Process stopped") || !strings.Contains(empty, "restart") {
		t.Errorf("expected empty state to offer restart, got:\n%s", empty)
	}

	// 2. Press 'r' to restart
	mMod, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = mMod.(Model)

	if !m.reconnecting {
		t.Errorf("expected reconnecting to be true after pressing 'r'")
	}
	if cmd == nil {
		t.Fatalf("expected tryRestartProcessCmd after pressing 'r'")
	}

	// 3. Simulate sourceReadyMsg returning new process
	newSrc, err := serial.NewProcessSource(cmdStr)
	if err != nil {
		t.Fatalf("failed to create second process source: %v", err)
	}
	defer newSrc.Stop()

	mMod, _ = m.Update(sourceReadyMsg{source: newSrc, port: cmdStr})
	m = mMod.(Model)

	if m.connState != ConnConnected {
		t.Errorf("expected ConnConnected after sourceReadyMsg, got %v", m.connState)
	}
	if m.reconnecting {
		t.Errorf("expected reconnecting to be false after sourceReadyMsg")
	}
	if !m.isProcessSource {
		t.Errorf("expected isProcessSource to remain true")
	}
}

func TestPipeSource_TUI_Lifecycle(t *testing.T) {
	reader := strings.NewReader("log line one\nlog line two\n")
	src := serial.NewPipeSource(reader)
	defer src.Stop()

	buf := record.NewBuffer(100)
	cfg := serial.DefaultConfig()
	m := New(cfg, nil, nil, buf, src, nil)
	m.width = 120
	m.height = 30
	m.tableHeight = 23

	if !m.isPipeSource {
		t.Fatalf("expected isPipeSource to be true")
	}

	// 1. Verify title bar
	tb := m.viewTitleBar()
	if !strings.Contains(tb, "Stream: stdin") || !strings.Contains(tb, "Streaming") {
		t.Errorf("expected title bar to show Stream: stdin and Streaming, got:\n%s", tb)
	}
	if strings.Contains(tb, "Baud:") {
		t.Errorf("title bar should not display Baud for pipe source, got:\n%s", tb)
	}

	// 2. Simulate EOF on pipe
	mMod, _ := m.Update(ConnStateMsg{State: ConnDisconnected, Detail: "source closed"})
	m = mMod.(Model)

	empty := m.viewEmptyState()
	if !strings.Contains(empty, "Standard input stream ended") {
		t.Errorf("expected empty state to show stream ended, got:\n%s", empty)
	}

	// 3. Pressing 'r' should notify that piped input cannot reconnect
	mMod, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = mMod.(Model)
	if cmd != nil {
		t.Errorf("expected nil cmd when trying to reconnect a piped stream")
	}
	if !strings.Contains(m.message, "Standard input stream — cannot reconnect") {
		t.Errorf("unexpected message on pipe reconnect attempt: %q", m.message)
	}
}
