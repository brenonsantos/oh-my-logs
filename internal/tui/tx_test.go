package tui

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/brenoniehues/oh-my-logs/internal/filter"
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
	m.txHistory = []string{"AT", "AT+VERSION", "AT+RST"}
	m.txHistoryCursor = -1

	// User types draft: "AT+TEST"
	for _, r := range "AT+TEST" {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(Model)
	}
	if m.txInput != "AT+TEST" {
		t.Fatalf("expected txInput 'AT+TEST', got %q", m.txInput)
	}

	// Press Up -> should load last history entry "AT+RST"
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = updated.(Model)
	if m.txInput != "AT+RST" {
		t.Errorf("expected 'AT+RST', got %q", m.txInput)
	}
	if m.txDraft != "AT+TEST" {
		t.Errorf("expected draft preserved as 'AT+TEST', got %q", m.txDraft)
	}

	// Press Up -> should load "AT+VERSION"
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = updated.(Model)
	if m.txInput != "AT+VERSION" {
		t.Errorf("expected 'AT+VERSION', got %q", m.txInput)
	}

	// Press Up -> should load "AT"
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = updated.(Model)
	if m.txInput != "AT" {
		t.Errorf("expected 'AT', got %q", m.txInput)
	}

	// Press Up at boundary -> remains "AT"
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = updated.(Model)
	if m.txInput != "AT" {
		t.Errorf("expected boundary 'AT', got %q", m.txInput)
	}

	// Press Down -> moves back to "AT+VERSION"
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(Model)
	if m.txInput != "AT+VERSION" {
		t.Errorf("expected 'AT+VERSION', got %q", m.txInput)
	}

	// Press Down -> moves to "AT+RST"
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(Model)
	if m.txInput != "AT+RST" {
		t.Errorf("expected 'AT+RST', got %q", m.txInput)
	}

	// Press Down past history -> draft "AT+TEST" is restored
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(Model)
	if m.txInput != "AT+TEST" {
		t.Errorf("expected draft 'AT+TEST' restored, got %q", m.txInput)
	}
	if m.txHistoryCursor != -1 {
		t.Errorf("expected txHistoryCursor reset to -1, got %d", m.txHistoryCursor)
	}
}

func TestTXSendExecution(t *testing.T) {
	m := newTestModel()
	mock := newTXMockSource()
	m.source = mock
	m.mode = modeTXInput
	m.txEnding = serial.EndingCRLF
	m.txInput = `STATUS\x02\r`

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

	if len(m.txHistory) != 1 || m.txHistory[0] != `STATUS\x02\r` {
		t.Errorf("expected command recorded in txHistory, got %v", m.txHistory)
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
	m.txInput = ""

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
	m.txInput = "AT"
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
	m.txInput = `PING\xZZ`
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
	m.txInput = "PING"
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
	m.txInput = ""

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
	m.txInput = "HELLO"

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
	m.txInput = "AT+TEST"

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
