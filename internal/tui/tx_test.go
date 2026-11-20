package tui

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/brenoniehues/oh-my-logs/internal/filter"
	"github.com/brenoniehues/oh-my-logs/internal/parser"
	"github.com/brenoniehues/oh-my-logs/internal/record"
	"github.com/brenoniehues/oh-my-logs/internal/serial"
	tea "github.com/charmbracelet/bubbletea"
)

type txMockSource struct {
	lines   chan string
	errors  chan error
	written [][]byte
	fail    bool
}

func newTXMockSource() *txMockSource {
	return &txMockSource{
		lines:  make(chan string),
		errors: make(chan error),
	}
}

func (s *txMockSource) Lines() <-chan string { return s.lines }
func (s *txMockSource) Errors() <-chan error { return s.errors }
func (s *txMockSource) Stop()                 {}
func (s *txMockSource) Write(p []byte) (int, error) {
	if s.fail {
		return 0, errors.New("simulated port write failure")
	}
	cp := make([]byte, len(p))
	copy(cp, p)
	s.written = append(s.written, cp)
	return len(p), nil
}

func TestTXModeActivation(t *testing.T) {
	m := newTestModel()
	if m.mode != modeNormal {
		t.Fatalf("expected initial mode normal, got %v", m.mode)
	}

	// Press 'i'
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	m = updated.(Model)
	if m.mode != modeTXInput {
		t.Errorf("expected modeTXInput after pressing 'i', got %v", m.mode)
	}

	// Cancel with Esc
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.mode != modeNormal {
		t.Errorf("expected modeNormal after Esc, got %v", m.mode)
	}

	// Press ':'
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{':'}})
	m = updated.(Model)
	if m.mode != modeTXInput {
		t.Errorf("expected modeTXInput after pressing ':', got %v", m.mode)
	}
}

func TestTXLineEndingCycling(t *testing.T) {
	m := newTestModel()
	m.mode = modeTXInput
	m.txEnding = serial.EndingCRLF

	// Tab -> LF
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	if m.txEnding != serial.EndingLF {
		t.Errorf("expected EndingLF, got %v", m.txEnding)
	}

	// Tab -> CR
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	if m.txEnding != serial.EndingCR {
		t.Errorf("expected EndingCR, got %v", m.txEnding)
	}

	// Tab -> None
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	if m.txEnding != serial.EndingNone {
		t.Errorf("expected EndingNone, got %v", m.txEnding)
	}

	// Ctrl+E -> CRLF
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlE})
	m = updated.(Model)
	if m.txEnding != serial.EndingCRLF {
		t.Errorf("expected EndingCRLF, got %v", m.txEnding)
	}
}

func TestTXHistoryNavigationAndDraftPreservation(t *testing.T) {
	m := newTestModel()
	m.mode = modeTXInput
	m.txInput.History = []string{"AT", "AT+VERSION", "AT+RST"}
	m.txInput.ResetHistoryCursor()

	// User types draft: "AT+TEST"
	for _, r := range "AT+TEST" {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(Model)
	}
	if m.txInput.Value != "AT+TEST" {
		t.Fatalf("expected txInput 'AT+TEST', got %q", m.txInput.Value)
	}

	// Press Up -> should load last history entry "AT+RST"
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = updated.(Model)
	if m.txInput.Value != "AT+RST" {
		t.Errorf("expected 'AT+RST', got %q", m.txInput.Value)
	}
	if m.txInput.Draft != "AT+TEST" {
		t.Errorf("expected draft preserved as 'AT+TEST', got %q", m.txInput.Draft)
	}

	// Press Up -> should load "AT+VERSION"
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = updated.(Model)
	if m.txInput.Value != "AT+VERSION" {
		t.Errorf("expected 'AT+VERSION', got %q", m.txInput.Value)
	}

	// Press Up -> should load "AT"
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = updated.(Model)
	if m.txInput.Value != "AT" {
		t.Errorf("expected 'AT', got %q", m.txInput.Value)
	}

	// Press Up at boundary -> remains "AT"
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = updated.(Model)
	if m.txInput.Value != "AT" {
		t.Errorf("expected boundary 'AT', got %q", m.txInput.Value)
	}

	// Press Down -> moves back to "AT+VERSION"
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(Model)
	if m.txInput.Value != "AT+VERSION" {
		t.Errorf("expected 'AT+VERSION', got %q", m.txInput.Value)
	}

	// Press Down -> moves to "AT+RST"
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(Model)
	if m.txInput.Value != "AT+RST" {
		t.Errorf("expected 'AT+RST', got %q", m.txInput.Value)
	}

	// Press Down past history -> draft "AT+TEST" is restored
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(Model)
	if m.txInput.Value != "AT+TEST" {
		t.Errorf("expected draft 'AT+TEST' restored, got %q", m.txInput.Value)
	}
	if m.txInput.HistoryCursor != -1 {
		t.Errorf("expected txHistoryCursor reset to -1, got %d", m.txInput.HistoryCursor)
	}
}

func TestTXSendExecution(t *testing.T) {
	m := newTestModel()
	mock := newTXMockSource()
	m.source = mock
	m.mode = modeTXInput
	m.txEnding = serial.EndingCRLF
	m.txInput.SetText(`STATUS\x02\r`)

	// Press Enter to transmit
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	if m.mode != modeNormal {
		t.Errorf("expected modeNormal after sending, got %v", m.mode)
	}
	if len(mock.written) != 1 {
		t.Fatalf("expected 1 write call, got %d", len(mock.written))
	}

	// Expected payload: STATUS (6 bytes) + 0x02 (1 byte) + \r (1 byte) + \r\n (2 bytes CRLF ending) = 10 bytes
	expected := append([]byte("STATUS\x02\r"), []byte("\r\n")...)
	if !bytes.Equal(mock.written[0], expected) {
		t.Errorf("expected written payload %v, got %v", expected, mock.written[0])
	}

	if !strings.Contains(m.message, "✓ Sent 10 bytes [CRLF]") {
		t.Errorf("expected success message with byte count, got %q", m.message)
	}

	if len(m.txInput.History) != 1 || m.txInput.History[0] != `STATUS\x02\r` {
		t.Errorf("expected command recorded in txHistory, got %v", m.txInput.History)
	}

	// Verify local echo record was added to buffer and visible rows
	if m.buffer.Len() != 1 {
		t.Fatalf("expected 1 record in buffer, got %d", m.buffer.Len())
	}
	allRecs := m.buffer.All()
	rec := allRecs[0]
	if rec.Raw != `TX [CRLF] › STATUS\x02\r` {
		t.Errorf("expected Raw 'TX [CRLF] › STATUS\\x02\\r', got %q", rec.Raw)
	}
	if rec.Fields["level"] != "TX" {
		t.Errorf("expected level 'TX', got %q", rec.Fields["level"])
	}
	if rec.Fields["ending"] != "CRLF" {
		t.Errorf("expected ending 'CRLF', got %q", rec.Fields["ending"])
	}
	if rec.Fields["message"] != `TX [CRLF] › STATUS\x02\r` {
		t.Errorf("expected message 'TX [CRLF] › STATUS\\x02\\r', got %q", rec.Fields["message"])
	}
	if len(m.visible) != 1 {
		t.Errorf("expected 1 visible record, got %d", len(m.visible))
	}
}

func TestTXSendEmptyPingEcho(t *testing.T) {
	m := newTestModel()
	mock := newTXMockSource()
	m.source = mock
	m.mode = modeTXInput
	m.txEnding = serial.EndingCRLF
	m.txInput.Reset()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	if m.buffer.Len() != 1 {
		t.Fatalf("expected 1 record in buffer, got %d", m.buffer.Len())
	}
	rec := m.buffer.All()[0]
	if rec.Raw != "TX [CRLF] ›" {
		t.Errorf("expected Raw 'TX [CRLF] ›', got %q", rec.Raw)
	}
	if rec.Fields["level"] != "TX" {
		t.Errorf("expected level 'TX', got %q", rec.Fields["level"])
	}
	if rec.Fields["ending"] != "CRLF" {
		t.Errorf("expected ending 'CRLF', got %q", rec.Fields["ending"])
	}
}

func TestTXSendFailureStates(t *testing.T) {
	// 1. Disconnected source
	m := newTestModel()
	m.source = nil
	m.mode = modeTXInput
	m.txInput.SetText("AT")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.mode != modeNormal {
		t.Errorf("expected modeNormal after error")
	}
	if m.message != "TX failed: disconnected" {
		t.Errorf("expected disconnected message, got %q", m.message)
	}

	// 2. Format error with invalid hex escape
	mock := newTXMockSource()
	m = newTestModel()
	m.source = mock
	m.mode = modeTXInput
	m.txInput.SetText(`PING\xZZ`)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if !strings.Contains(m.message, "TX format error") {
		t.Errorf("expected format error message, got %q", m.message)
	}
	if len(mock.written) != 0 {
		t.Errorf("expected no writes on format error")
	}

	// 3. Port write failure
	mock.fail = true
	m = newTestModel()
	m.source = mock
	m.mode = modeTXInput
	m.txInput.SetText("PING")
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if !strings.Contains(m.message, "TX error") {
		t.Errorf("expected TX error on port failure, got %q", m.message)
	}
}

func TestTXEmptyInputWithEndingNone(t *testing.T) {
	mock := newTXMockSource()
	m := newTestModel()
	m.source = mock
	m.mode = modeTXInput
	m.txEnding = serial.EndingNone
	m.txInput.Reset()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.mode != modeNormal {
		t.Errorf("expected modeNormal, got %v", m.mode)
	}
	if len(mock.written) != 0 {
		t.Errorf("expected 0 writes for empty input with EndingNone, got %d", len(mock.written))
	}
}

func TestTXViewKeyBarAndHelpModal(t *testing.T) {
	m := newTestModel()
	m.mode = modeTXInput
	m.txEnding = serial.EndingCRLF
	m.txInput.SetText("HELLO")

	bar := m.viewKeyBar()
	if !strings.Contains(bar, "TX [CRLF] ›") {
		t.Errorf("expected bar to contain prompt, got: %s", bar)
	}
	if !strings.Contains(bar, "HELLO") {
		t.Errorf("expected bar to display txInput, got: %s", bar)
	}
	if !strings.Contains(bar, "Enter: send") || !strings.Contains(bar, "Tab: line ending") {
		t.Errorf("expected bar to display key hints, got: %s", bar)
	}

	// Help modal should document Send serial cmd (TX)
	m.mode = modeHelp
	help := m.viewHelpModal()
	if !strings.Contains(help, "Send serial cmd (TX)") {
		t.Errorf("expected help modal to include Send serial cmd (TX), got: %s", help)
	}
}

func TestTXEchoProfileTransformAndFilter(t *testing.T) {
	m := newTestModel()
	mock := newTXMockSource()
	m.source = mock
	m.mode = modeTXInput
	m.txEnding = serial.EndingCRLF
	m.txInput.SetText("AT+TEST")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	if m.buffer.Len() != 1 {
		t.Fatalf("expected 1 record in buffer, got %d", m.buffer.Len())
	}

	// 1. Check filter matches TX record
	f, err := filter.New("TX")
	if err != nil {
		t.Fatalf("unexpected filter error: %v", err)
	}
	if !f.Matches(m.buffer.All()[0]) {
		t.Errorf("expected filter 'TX' to match TX record")
	}

	// 2. Check profile transform preserves TX record
	m.buffer.Transform(func(old record.Record) record.Record {
		if old.Fields["level"] == "TX" {
			return old
		}
		newRec, _ := m.parser.Parse(old.Raw)
		return newRec
	})

	recAfter := m.buffer.All()[0]
	if recAfter.Fields["level"] != "TX" {
		t.Errorf("expected level 'TX' preserved after transform, got %q", recAfter.Fields["level"])
	}
	if recAfter.Fields["ending"] != "CRLF" {
		t.Errorf("expected ending 'CRLF' preserved after transform, got %q", recAfter.Fields["ending"])
	}
	if recAfter.Raw != "TX [CRLF] › AT+TEST" {
		t.Errorf("expected Raw preserved after transform, got %q", recAfter.Raw)
	}
}

func TestShellInteractiveMode_ActivationAndExit(t *testing.T) {
	m := newTestModel()
	mock := newTXMockSource()
	m.source = mock
	m.connState = ConnConnected

	// 1. Press 'I' to enter shell passthrough mode
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'I'}})
	m = updated.(Model)
	if m.mode != modeShellInteractive {
		t.Fatalf("expected modeShellInteractive after pressing 'I', got %v", m.mode)
	}

	// 2. Key bar should render shell indicator
	keyBar := m.viewKeyBar()
	if !strings.Contains(keyBar, "SHELL") || !strings.Contains(keyBar, "MCU completion") {
		t.Errorf("expected key bar to display shell passthrough hints, got:\n%s", keyBar)
	}

	// 3. Status bar should render SHELL ACTIVE badge
	statusBar := m.viewStatusBar()
	if !strings.Contains(statusBar, "SHELL ACTIVE") {
		t.Errorf("expected status bar to display 'SHELL ACTIVE', got:\n%s", statusBar)
	}

	// 4. Press Esc to exit back to log view
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.mode != modeNormal {
		t.Fatalf("expected modeNormal after Esc, got %v", m.mode)
	}
}

func TestShellInteractiveMode_TabForwardingAndCharacterStreaming(t *testing.T) {
	m := newTestModel()
	mock := newTXMockSource()
	m.source = mock
	m.connState = ConnConnected
	m.mode = modeShellInteractive

	// 1. Type characters 'h', 'e', 'l', 'p'
	for _, r := range []rune{'h', 'e', 'l', 'p'} {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(Model)
	}

	if len(mock.written) != 4 {
		t.Fatalf("expected 4 character writes to serial, got %d", len(mock.written))
	}
	if string(mock.written[0]) != "h" || string(mock.written[3]) != "p" {
		t.Errorf("unexpected written characters: %v", mock.written)
	}
	if m.shellInput.Value != "help" {
		t.Errorf("expected shellInput value 'help', got %q", m.shellInput.Value)
	}

	// 2. Press Tab: should forward ASCII 0x09 directly to device!
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)

	lastWritten := mock.written[len(mock.written)-1]
	if !bytes.Equal(lastWritten, []byte{'\t'}) {
		t.Errorf("expected Tab (0x09) sent to serial, got %v", lastWritten)
	}

	// 3. Press Ctrl+C: should forward ASCII 0x03 interrupt signal!
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m = updated.(Model)

	lastWritten = mock.written[len(mock.written)-1]
	if !bytes.Equal(lastWritten, []byte{0x03}) {
		t.Errorf("expected Ctrl+C (0x03) sent to serial, got %v", lastWritten)
	}
	if m.shellInput.Value != "" {
		t.Errorf("expected shellInput to reset on Ctrl+C, got %q", m.shellInput.Value)
	}

	// 4. Arrow keys: Up should send ANSI \x1b[A
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = updated.(Model)
	lastWritten = mock.written[len(mock.written)-1]
	if string(lastWritten) != "\x1b[A" {
		t.Errorf("expected up arrow ANSI sequence, got %q", string(lastWritten))
	}

	// 5. Enter should send configured line ending only (preventing command duplication) and record TX badge
	m.shellInput.SetText("version")
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	lastWritten = mock.written[len(mock.written)-1]
	if string(lastWritten) != "\r\n" {
		t.Errorf("expected only newline ending '\\r\\n' sent on Enter in interactive mode, got %q", string(lastWritten))
	}

	if m.buffer.Len() != 1 {
		t.Fatalf("expected 1 record in buffer for TX badge, got %d", m.buffer.Len())
	}
	if !strings.Contains(m.buffer.All()[0].Raw, "version") {
		t.Errorf("expected TX badge record with 'version', got %q", m.buffer.All()[0].Raw)
	}
}

func TestShellInteractiveMode_PromptCompletionSync(t *testing.T) {
	m := newTestModel()
	mock := newTXMockSource()
	m.source = mock
	m.connState = ConnConnected
	m.mode = modeShellInteractive

	// Engineer typed 'os' and pressed Tab to request autocompletion
	m.shellInput.SetText("os")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)

	if !m.shellAwaitingTab {
		t.Errorf("expected shellAwaitingTab to be true after pressing Tab")
	}

	// Device responds with prompt and completed token 'os_'
	updated, _ = m.Update(lineMsg("uart:~$ os_"))
	m = updated.(Model)

	if m.shellAwaitingTab {
		t.Errorf("expected shellAwaitingTab to be reset to false after completion sync")
	}
	if m.shellPrompt != "uart:~$ " {
		t.Errorf("expected shellPrompt 'uart:~$ ', got %q", m.shellPrompt)
	}
	if m.shellInput.Value != "os_" {
		t.Errorf("expected shellInput to be updated to 'os_', got %q", m.shellInput.Value)
	}
	// Verify prompt wasn't dumped into the log buffer as a fake record
	if m.buffer.Len() != 0 {
		t.Errorf("expected buffer to remain empty of prompt artifacts, got %d records", m.buffer.Len())
	}
}

func TestShellInteractiveMode_DuplicateEchoSuppression(t *testing.T) {
	m := newTestModel()
	mock := newTXMockSource()
	m.source = mock
	m.connState = ConnConnected
	m.mode = modeShellInteractive

	// Send command
	m.shellInput.SetText("hi")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	// Buffer has TX record
	if m.buffer.Len() != 1 {
		t.Fatalf("expected 1 record (TX message), got %d", m.buffer.Len())
	}

	// Device echoes 'hi'
	updated, _ = m.Update(lineMsg("hi"))
	m = updated.(Model)

	// Duplicate echo must be suppressed, buffer still at 1
	if m.buffer.Len() != 1 {
		t.Errorf("expected duplicate echo 'hi' to be suppressed, buffer len = %d", m.buffer.Len())
	}

	// Device sends command output
	updated, _ = m.Update(lineMsg("hi: command not found"))
	m = updated.(Model)

	// Output is recorded!
	if m.buffer.Len() != 2 {
		t.Fatalf("expected 2 records, got %d", m.buffer.Len())
	}
	if !strings.Contains(m.buffer.All()[1].Raw, "command not found") {
		t.Errorf("expected command not found output, got %q", m.buffer.All()[1].Raw)
	}
}

func TestUnparsedRow_FullWidthSpanningAndGutter(t *testing.T) {
	m := newTestModel()
	cols := []record.Column{
		{Field: "uptime", Title: "Uptime", Width: 10, Style: "uptime"},
		{Field: "level", Title: "Level", Width: 6, Style: "level"},
		{Field: "module", Title: "Module", Width: 12, Style: "identifier"},
		{Field: "message", Title: "Message", Width: 0, Style: "primary"},
	}
	m.columns = cols
	m.width = 100
	m.height = 20
	m.recalcLayout()

	// Ingest a standard log record (parsed with uptime, level, module)
	m.ingestRecord(record.Record{
		Raw: "[   10.839] <inf> os_msg: test log",
		Fields: map[string]string{
			"uptime":  "10.839",
			"level":   "inf",
			"module":  "os_msg",
			"message": "test log",
		},
	})

	// Ingest an unparsed non-log subcommand help record
	unparsedHelp := "  set_conv_group                : Set Conversation Group ID"
	m.ingestRecord(record.Record{
		Raw: unparsedHelp,
		Fields: map[string]string{
			"_raw":    unparsedHelp,
			"message": unparsedHelp,
		},
	})

	view := m.viewTable()

	// Verify the unparsed row renders with gutter separator '│'
	if !strings.Contains(view, "│") {
		t.Errorf("expected viewTable to render unparsed row gutter indicator '│', got:\n%s", view)
	}

	// Verify the unparsed subcommand text appears in view
	if !strings.Contains(view, "set_conv_group") {
		t.Errorf("expected viewTable to contain unparsed help text 'set_conv_group', got:\n%s", view)
	}
}

func TestShellInteractiveMode_KeystrokeEchoFiltered(t *testing.T) {
	m := newTestModel()
	mock := newTXMockSource()
	m.source = mock
	m.connState = ConnConnected
	m.mode = modeShellInteractive

	// The serial line discipline reports echoed keystrokes as growing prompt
	// snapshots (never as committed rows).
	for _, snap := range []string{"uart:~$ o", "uart:~$ os", "uart:~$ os_", "uart:~$ os_m", "uart:~$ os_ms", "uart:~$ os_msg"} {
		updated, _ := m.Update(lineMsg(serial.MarkPromptSnapshot(snap)))
		m = updated.(Model)
	}

	// None of the single-character keystroke echoes should have been added to the buffer
	if m.buffer.Len() != 0 {
		t.Errorf("expected prompt snapshots to stay out of the table, got %d records in buffer", m.buffer.Len())
	}

	// Real multiline output from MCU should still be ingested normally
	updated, _ := m.Update(lineMsg("os_msg: command not found"))
	m = updated.(Model)
	if m.buffer.Len() != 1 {
		t.Errorf("expected real command output to be ingested, got %d records", m.buffer.Len())
	}

	// Short legitimate lines must no longer be dropped by a length heuristic.
	updated, _ = m.Update(lineMsg("OK"))
	m = updated.(Model)
	if m.buffer.Len() != 2 {
		t.Errorf("expected short line %q to be ingested, got %d records", "OK", m.buffer.Len())
	}
}

func TestShellInteractiveMode_NoTabCompletionWithoutTabPress(t *testing.T) {
	m := newTestModel()
	mock := newTXMockSource()
	m.source = mock
	m.connState = ConnConnected
	m.mode = modeShellInteractive

	// User types a command but DOES NOT press Tab
	m.shellInput.SetText("os_msg")

	// Device prints multiline subcommand help text where some lines might match prompt-like tokens
	helpLines := []string{
		"os_msg - OS Message module APIs test commands",
		"Subcommands:",
		"  publish                       : Publish a message",
		"  )<key0>=<value0>,<key1>=<value1>,....",
		"  uart:~$ help",
	}

	for _, l := range helpLines {
		updated, _ := m.Update(lineMsg(l))
		m = updated.(Model)
	}

	// shellInput must REMAIN strictly "os_msg" and NOT be polluted or overwritten by incoming text
	if m.shellInput.Value != "os_msg" {
		t.Errorf("expected shellInput to remain %q, but got corrupted value: %q", "os_msg", m.shellInput.Value)
	}
}

func TestShellInteractiveMode_AsyncLogsWithActiveInputNotPrefixed(t *testing.T) {
	m := newTestModel()
	zephyrParser, err := parser.NewRegexParser(`^\[\s*(?P<uptime>[^\]]+?)\s*\]\s+<(?P<level>[a-zA-Z]+)>\s+(?:(?P<module>[a-zA-Z0-9_.-]+):\s+)?(?P<message>.*)$`)
	if err != nil {
		t.Fatalf("failed to create regex parser: %v", err)
	}
	m.parser = zephyrParser
	mock := newTXMockSource()
	m.source = mock
	m.connState = ConnConnected
	m.mode = modeShellInteractive

	// User types "os_msg enable_logging" into interactive shell input
	m.shellInput.SetText("os_msg enable_logging")

	// Embedded shell receives async logs while prompt is active, erasing line with \r\x1b[K
	incoming := []string{
		"os_msg enable_logging\r\x1b[K[    368.331] <wrn> nxp_wifi: WLAN: Unknown Event: 5",
		"os_msg enable_logging\r\x1b[K[    368.331] <inf> wpa_supp: wifi_nxp: BG scan stopped",
	}

	for _, raw := range incoming {
		updated, _ := m.Update(lineMsg(raw))
		m = updated.(Model)
	}

	if m.buffer.Len() != 2 {
		t.Fatalf("expected 2 records in buffer, got %d", m.buffer.Len())
	}

	all := m.buffer.All()
	rec0 := all[0]
	if strings.Contains(rec0.Raw, "os_msg enable_logging") {
		t.Errorf("expected rec0 to not contain active input prefix, got: %q", rec0.Raw)
	}
	if rec0.Fields["module"] != "nxp_wifi" {
		t.Errorf("expected rec0 module to be nxp_wifi, got: %q", rec0.Fields["module"])
	}

	rec1 := all[1]
	if strings.Contains(rec1.Raw, "os_msg enable_logging") {
		t.Errorf("expected rec1 to not contain active input prefix, got: %q", rec1.Raw)
	}
	if rec1.Fields["module"] != "wpa_supp" {
		t.Errorf("expected rec1 module to be wpa_supp, got: %q", rec1.Fields["module"])
	}
}




