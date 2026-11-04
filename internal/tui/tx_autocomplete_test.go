package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/brenoniehues/oh-my-logs/internal/encoder"
	"github.com/brenoniehues/oh-my-logs/internal/serial"
	tea "github.com/charmbracelet/bubbletea"
)

type mockTestEncoder struct {
	name        string
	trigger     string
	desc        string
	suggestFunc func(query string, cursor int) ([]encoder.Suggestion, error)
	encodeFunc  func(text string) (encoder.EncodeResult, error)
}

func (m *mockTestEncoder) Name() string        { return m.name }
func (m *mockTestEncoder) Trigger() string     { return m.trigger }
func (m *mockTestEncoder) Description() string { return m.desc }

func (m *mockTestEncoder) Suggest(query string, cursor int) ([]encoder.Suggestion, error) {
	if m.suggestFunc != nil {
		return m.suggestFunc(query, cursor)
	}
	return nil, nil
}

func (m *mockTestEncoder) Encode(text string) (encoder.EncodeResult, error) {
	if m.encodeFunc != nil {
		return m.encodeFunc(text)
	}
	return encoder.EncodeResult{Payload: text, Display: text}, nil
}

func (m *mockTestEncoder) Close() error { return nil }

func TestApplySuggestion(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		cursor      int
		replacement string
		expectedStr string
		expectedCur int
	}{
		{
			name:        "empty input",
			input:       "",
			cursor:      0,
			replacement: "command",
			expectedStr: "command ",
			expectedCur: 8,
		},
		{
			name:        "single token complete",
			input:       "comm",
			cursor:      4,
			replacement: "command",
			expectedStr: "command ",
			expectedCur: 8,
		},
		{
			name:        "middle token replacement",
			input:       "set tem unit",
			cursor:      7,
			replacement: "temperature",
			expectedStr: "set temperature unit",
			expectedCur: 15,
		},
		{
			name:        "last token appended",
			input:       "get status",
			cursor:      10,
			replacement: "status_all",
			expectedStr: "get status_all ",
			expectedCur: 15,
		},
		{
			name:        "protocol topic delimiter replacement",
			input:       "os_msg publish 13:req_cook",
			cursor:      26,
			replacement: "03/",
			expectedStr: "os_msg publish 13:03/",
			expectedCur: 21,
		},
		{
			name:        "protocol message delimiter replacement",
			input:       "os_msg publish 13:03/ala",
			cursor:      24,
			replacement: "D3dK(",
			expectedStr: "os_msg publish 13:03/D3dK(",
			expectedCur: 26,
		},
		{
			name:        "protocol key delimiter replacement",
			input:       "os_msg publish 13:03/D3dK(432)boost",
			cursor:      35,
			replacement: "AFfub=",
			expectedStr: "os_msg publish 13:03/D3dK(432)AFfub=",
			expectedCur: 36,
		},
		{
			name:        "protocol value delimiter replacement",
			input:       "os_msg publish 13:03/D3dK(432)AFfub=123",
			cursor:      39,
			replacement: "AAAAB7,",
			expectedStr: "os_msg publish 13:03/D3dK(432)AFfub=AAAAB7,",
			expectedCur: 43,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotStr, gotCur := applySuggestion(tc.input, tc.cursor, tc.replacement)
			if gotStr != tc.expectedStr {
				t.Errorf("expected string %q, got %q", tc.expectedStr, gotStr)
			}
			if gotCur != tc.expectedCur {
				t.Errorf("expected cursor %d, got %d", tc.expectedCur, gotCur)
			}
		})
	}
}

func TestTXSuggestionsDrawerHeight(t *testing.T) {
	if h := txSuggestionsDrawerHeight(0); h != 0 {
		t.Errorf("expected 0 for 0 suggestions, got %d", h)
	}
	if h := txSuggestionsDrawerHeight(3); h != 4 {
		t.Errorf("expected 4 for 3 suggestions (1 header + 3 rows), got %d", h)
	}
	if h := txSuggestionsDrawerHeight(10); h != 6 {
		t.Errorf("expected 6 for 10 suggestions (capped at 5 rows + 1 header), got %d", h)
	}
}

func TestTXSuggestionsDrawerView(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.txSuggestions = []encoder.Suggestion{
		{Value: "get_status", Description: "Query system status"},
		{Value: "get_version", Description: "Query firmware version"},
	}
	m.txSuggestionIdx = 0

	view := m.viewTXSuggestionsDrawer()
	if !strings.Contains(view, "SUGGESTIONS") {
		t.Errorf("expected view to contain SUGGESTIONS, got %s", view)
	}
	if !strings.Contains(view, "get_status") || !strings.Contains(view, "get_version") {
		t.Errorf("expected view to contain suggestion values, got %s", view)
	}
	if !strings.Contains(view, "Query system status") {
		t.Errorf("expected view to contain description, got %s", view)
	}
	if !strings.Contains(view, "▸") {
		t.Errorf("expected active suggestion indicator ▸, got %s", view)
	}
}

func TestTXEncoderKeyToggling(t *testing.T) {
	m := newTestModel()
	enc1 := &mockTestEncoder{name: "CAN"}
	enc2 := &mockTestEncoder{name: "ProtoBuf"}
	m.encoders = encoder.NewPipelineWithEncoders([]encoder.Encoder{enc1, enc2})
	m.mode = modeTXInput
	m.activeEncoderIdx = -1 // RAW initially

	// Initial prompt shows RAW
	bar := m.viewKeyBar()
	if !strings.Contains(bar, "TX [RAW | CRLF] ›") {
		t.Fatalf("expected RAW prompt initially, got: %s", bar)
	}

	// Alt+E -> toggles to enc1 (CAN)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}, Alt: true})
	m = updated.(Model)
	if m.activeEncoderIdx != 0 {
		t.Errorf("expected activeEncoderIdx 0, got %d", m.activeEncoderIdx)
	}
	bar = m.viewKeyBar()
	if !strings.Contains(bar, "TX [CAN | CRLF] ›") {
		t.Errorf("expected CAN prompt, got: %s", bar)
	}

	// Alt+E -> toggles to enc2 (ProtoBuf)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}, Alt: true})
	m = updated.(Model)
	if m.activeEncoderIdx != 1 {
		t.Errorf("expected activeEncoderIdx 1, got %d", m.activeEncoderIdx)
	}
	bar = m.viewKeyBar()
	if !strings.Contains(bar, "TX [ProtoBuf | CRLF] ›") {
		t.Errorf("expected ProtoBuf prompt, got: %s", bar)
	}

	// Alt+E -> cycles back to RAW (-1)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}, Alt: true})
	m = updated.(Model)
	if m.activeEncoderIdx != -1 {
		t.Errorf("expected activeEncoderIdx -1 (RAW), got %d", m.activeEncoderIdx)
	}
	bar = m.viewKeyBar()
	if !strings.Contains(bar, "TX [RAW | CRLF] ›") {
		t.Errorf("expected RAW prompt after cycle, got: %s", bar)
	}

	// Ctrl+T -> toggles to enc1 (CAN) as macOS-friendly shortcut
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlT})
	m = updated.(Model)
	if m.activeEncoderIdx != 0 {
		t.Errorf("expected activeEncoderIdx 0 on Ctrl+T, got %d", m.activeEncoderIdx)
	}
}

func TestTXSuggestionsNavigationAndApply(t *testing.T) {
	m := newTestModel()
	m.mode = modeTXInput
	m.txInput.SetTextAndCursor("get_", 4)
	m.txSuggestions = []encoder.Suggestion{
		{Value: "get_state", Description: "Query device state"},
		{Value: "get_telemetry", Description: "Query telemetry data"},
	}
	m.txSuggestionIdx = 0

	// Down arrow navigates to second item
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(Model)
	if m.txSuggestionIdx != 1 {
		t.Errorf("expected txSuggestionIdx 1, got %d", m.txSuggestionIdx)
	}

	// Up arrow navigates back to first item
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = updated.(Model)
	if m.txSuggestionIdx != 0 {
		t.Errorf("expected txSuggestionIdx 0, got %d", m.txSuggestionIdx)
	}

	// Tab applies the suggestion
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	if m.txInput.Value != "get_state " {
		t.Errorf("expected 'get_state ', got %q", m.txInput.Value)
	}
	if len(m.txSuggestions) != 0 {
		t.Errorf("expected suggestions cleared after completion")
	}

	// Re-populate suggestions and verify Escape closes them without exiting modeTXInput
	m.txSuggestions = []encoder.Suggestion{{Value: "get_state"}}
	m.txSuggestionIdx = 0
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if len(m.txSuggestions) != 0 {
		t.Errorf("expected suggestions cleared on Esc")
	}
	if m.mode != modeTXInput {
		t.Errorf("expected to remain in modeTXInput, got %v", m.mode)
	}

	// Next Escape exits modeTXInput
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.mode != modeNormal {
		t.Errorf("expected to exit to modeNormal, got %v", m.mode)
	}
}

func TestTXSendWithEncoder(t *testing.T) {
	m := newTestModel()
	mockSrc := newTXMockSource()
	m.source = mockSrc
	m.mode = modeTXInput
	m.txEnding = serial.EndingCRLF

	enc := &mockTestEncoder{
		name: "PacketCodec",
		encodeFunc: func(text string) (encoder.EncodeResult, error) {
			return encoder.EncodeResult{
				Payload: "BASE64:AABBCCDDEE",
				Display: "PacketCodec: ping(0x01)",
			}, nil
		},
	}
	m.encoders = encoder.NewPipelineWithEncoders([]encoder.Encoder{enc})
	m.activeEncoderIdx = 0
	m.txInput.SetTextAndCursor("ping", 4)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	// Verify wire payload was transmitted
	expectedWire := "BASE64:AABBCCDDEE\r\n"
	if len(mockSrc.written) != 1 {
		t.Fatalf("expected 1 write call, got %d", len(mockSrc.written))
	}
	if string(mockSrc.written[0]) != expectedWire {
		t.Errorf("expected wire write %q, got %q", expectedWire, string(mockSrc.written[0]))
	}

	// Verify dual history records: 1 wire payload + 1 decoded representation
	if m.buffer.Len() != 2 {
		t.Fatalf("expected 2 records in buffer (wire + decoded continuation), got %d", m.buffer.Len())
	}
	recs := m.buffer.All()
	if !strings.Contains(recs[0].Fields["message"], "BASE64:AABBCCDDEE") {
		t.Errorf("expected wire message in first record, got: %s", recs[0].Fields["message"])
	}
	if !strings.Contains(recs[1].Fields["message"], "↳ PacketCodec: ping(0x01)") {
		t.Errorf("expected display continuation message in second record, got: %s", recs[1].Fields["message"])
	}

	// Verify success message
	if !strings.Contains(m.message, "PacketCodec | CRLF") {
		t.Errorf("expected message to mention PacketCodec | CRLF, got: %s", m.message)
	}
}

func TestTXSuggestionsDrawerLayoutVisibility(t *testing.T) {
	// Verify that TX input bar is never cut off or occluded when suggestions drawer is active,
	// both in empty state (disconnected/no logs) and with active logs.
	m := newTestModel()
	m.width = 120
	m.height = 35
	m.recalcLayout()

	m.mode = modeTXInput
	m.txInput.SetTextAndCursor("device_cmd 123", 14)
	m.txSuggestions = []encoder.Suggestion{
		{Value: "device_cmd 123:01/", Description: "Option 1"},
		{Value: "device_cmd 123:02/", Description: "Option 2"},
		{Value: "device_cmd 123:03/", Description: "Option 3"},
	}
	m.txSuggestionIdx = 0

	// Case 1: Empty state (0 logs)
	viewEmpty := m.View()
	linesEmpty := strings.Split(viewEmpty, "\n")
	if len(linesEmpty) != m.height {
		t.Fatalf("expected exactly %d lines in empty state, got %d", m.height, len(linesEmpty))
	}
	lastLineEmpty := linesEmpty[len(linesEmpty)-1]
	if !strings.Contains(lastLineEmpty, "TX") || !strings.Contains(lastLineEmpty, "device_cmd 123") {
		t.Errorf("expected last line in empty state to be TX input bar, got: %q", lastLineEmpty)
	}
	if !strings.Contains(viewEmpty, "SUGGESTIONS") {
		t.Errorf("expected suggestions drawer to be visible in empty state")
	}

	// Case 2: Populated logs
	for i := 0; i < 50; i++ {
		updated, _ := m.Update(lineMsg(fmt.Sprintf("log line %d", i)))
		m = updated.(Model)
	}

	viewPop := m.View()
	linesPop := strings.Split(viewPop, "\n")
	if len(linesPop) != m.height {
		t.Fatalf("expected exactly %d lines with logs, got %d", m.height, len(linesPop))
	}
	lastLinePop := linesPop[len(linesPop)-1]
	if !strings.Contains(lastLinePop, "TX") || !strings.Contains(lastLinePop, "device_cmd 123") {
		t.Errorf("expected last line with logs to be TX input bar, got: %q", lastLinePop)
	}

	// Case 3: Small terminal height (stress test overflow cut)
	m.height = 12
	m.recalcLayout()
	viewSmall := m.View()
	linesSmall := strings.Split(viewSmall, "\n")
	if len(linesSmall) != m.height {
		t.Fatalf("expected exactly %d lines in small viewport, got %d", m.height, len(linesSmall))
	}
	lastLineSmall := linesSmall[len(linesSmall)-1]
	if !strings.Contains(lastLineSmall, "TX") || !strings.Contains(lastLineSmall, "device_cmd 123") {
		t.Errorf("expected last line in small viewport to strictly preserve TX input bar, got: %q", lastLineSmall)
	}
}
