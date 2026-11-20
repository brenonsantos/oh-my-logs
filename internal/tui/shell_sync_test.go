package tui

import (
	"bytes"
	"testing"

	"github.com/brenoniehues/oh-my-logs/internal/serial"

	tea "github.com/charmbracelet/bubbletea"
)

func shellModel() (Model, *txMockSource) {
	m := newTestModel()
	mock := newTXMockSource()
	m.source = mock
	m.connState = ConnConnected
	m.mode = modeShellInteractive
	return m, mock
}

func feedLine(m Model, s string) Model {
	u, _ := m.Update(lineMsg(serial.MarkPromptSnapshot(s)))
	return u.(Model)
}

func pressKey(m Model, k tea.KeyMsg) Model {
	u, cmd := m.Update(k)
	m = u.(Model)
	if cmd != nil {
		if msg := cmd(); msg != nil {
			u, _ = m.Update(msg)
			m = u.(Model)
		}
	}
	return m
}

func TestShell_InputBarSyncsFromDeviceRow(t *testing.T) {
	m, _ := shellModel()
	// MCU history recall (Up) or Tab completion redraws the row: bar must follow the device.
	m = feedLine(m, "uart:~$ kernel version")
	if m.shellInput.Value != "kernel version" {
		t.Fatalf("bar = %q, want device row", m.shellInput.Value)
	}
	if m.buffer.Len() != 0 {
		t.Fatalf("snapshot leaked into table")
	}
	m = feedLine(m, "uart:~$ ")
	if m.shellInput.Value != "" {
		t.Fatalf("bar = %q, want empty", m.shellInput.Value)
	}
}

func TestShell_EnterHoldIgnoresStaleEchoUntilPromptRedraw(t *testing.T) {
	m, _ := shellModel()
	m.shellInput.SetText("help")
	m = pressKey(m, tea.KeyMsg{Type: tea.KeyEnter})
	m = feedLine(m, "uart:~$ hel") // late echo of the typed chars
	if m.shellInput.Value != "" {
		t.Fatalf("stale echo resurrected bar text: %q", m.shellInput.Value)
	}
	m = feedLine(m, "uart:~$ ") // prompt redrawn after command ran
	m = feedLine(m, "uart:~$ x")
	if m.shellInput.Value != "x" {
		t.Fatalf("sync did not resume, bar = %q", m.shellInput.Value)
	}
}

func TestShell_ReenterShowsDeviceBufferedText(t *testing.T) {
	m, _ := shellModel()
	m = feedLine(m, "uart:~$ os_ms")
	m.mode = modeNormal
	m = feedLine(m, "uart:~$ os_msg") // snapshot while outside shell mode
	m.mode = modeNormal
	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(m.keys.ShellPassthrough.Keys()[0])})
	m = u.(Model)
	if m.mode == modeShellInteractive && m.shellInput.Value != "os_msg" {
		t.Fatalf("bar = %q, want device buffer", m.shellInput.Value)
	}
}

func TestShell_EditKeysForwarded(t *testing.T) {
	cases := []struct {
		key  tea.KeyMsg
		want string
	}{
		{tea.KeyMsg{Type: tea.KeyHome}, "\x1b[H"},
		{tea.KeyMsg{Type: tea.KeyEnd}, "\x1b[F"},
		{tea.KeyMsg{Type: tea.KeyDelete}, "\x1b[3~"},
		{tea.KeyMsg{Type: tea.KeyCtrlA}, "\x01"},
		{tea.KeyMsg{Type: tea.KeyCtrlE}, "\x05"},
		{tea.KeyMsg{Type: tea.KeyCtrlK}, "\x0b"},
		{tea.KeyMsg{Type: tea.KeyCtrlU}, "\x15"},
		{tea.KeyMsg{Type: tea.KeyCtrlW}, "\x17"},
		{tea.KeyMsg{Type: tea.KeyCtrlL}, "\x0c"},
		{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b"), Alt: true}, "\x1bb"},
	}
	for _, tc := range cases {
		m, mock := shellModel()
		m = pressKey(m, tc.key)
		if len(mock.written) != 1 || !bytes.Equal(mock.written[0], []byte(tc.want)) {
			t.Errorf("%v wrote %q, want %q", tc.key, mock.written, tc.want)
		}
	}
}

func TestShell_PasteNewlinesFlattenedAndWriteFailureReported(t *testing.T) {
	m, mock := shellModel()
	m = pressKey(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a\nb"), Paste: true})
	if len(mock.written) != 1 || string(mock.written[0]) != "a b" {
		t.Fatalf("paste wrote %q", mock.written)
	}
	mock.fail = true
	m = pressKey(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	if m.message == "" {
		t.Fatalf("write failure not surfaced")
	}
}
