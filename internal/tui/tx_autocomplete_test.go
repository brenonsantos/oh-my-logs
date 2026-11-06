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

func TestTXSuggestionsScrollWindow(t *testing.T) {
	m := newTestModel()
	m.mode = modeTXInput
	m.width = 80
	m.height = 24

	// Populate 20 suggestions
	suggs := make([]encoder.Suggestion, 20)
	for i := 0; i < 20; i++ {
		suggs[i] = encoder.Suggestion{
			Value:       fmt.Sprintf("cmd_%02d", i),
			Description: fmt.Sprintf("Command %d description", i),
		}
	}
	m.txSuggestions = suggs
	m.txSuggestionIdx = 0
	m.txSuggestionOffset = 0

	// Initial view: top 5 items visible (0..4), item 0 selected
	v0 := m.viewTXSuggestionsDrawer()
	if !strings.Contains(v0, "cmd_00") || !strings.Contains(v0, "cmd_04") {
		t.Errorf("expected cmd_00 and cmd_04 in initial view, got:\n%s", v0)
	}
	if strings.Contains(v0, "cmd_05") {
		t.Errorf("cmd_05 should not be visible initially, got:\n%s", v0)
	}
	if !strings.Contains(v0, "(1/20)") {
		t.Errorf("expected position indicator '(1/20)' in header, got:\n%s", v0)
	}
	if !strings.Contains(v0, "█") {
		t.Errorf("expected scrollbar thumb in scrollable drawer, got:\n%s", v0)
	}

	// Press Down 5 times to reach item 5 -> offset should become 1
	for i := 0; i < 5; i++ {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = updated.(Model)
	}
	if m.txSuggestionIdx != 5 {
		t.Fatalf("expected txSuggestionIdx 5, got %d", m.txSuggestionIdx)
	}
	if m.txSuggestionOffset != 1 {
		t.Fatalf("expected txSuggestionOffset 1, got %d", m.txSuggestionOffset)
	}

	v1 := m.viewTXSuggestionsDrawer()
	if strings.Contains(v1, "cmd_00") {
		t.Errorf("cmd_00 should have scrolled out of view, got:\n%s", v1)
	}
	if !strings.Contains(v1, "cmd_05") {
		t.Errorf("cmd_05 should now be visible, got:\n%s", v1)
	}
	if !strings.Contains(v1, "(6/20)") {
		t.Errorf("expected position indicator '(6/20)' in header, got:\n%s", v1)
	}

	// Navigate to the last item (idx 19)
	for m.txSuggestionIdx < 19 {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = updated.(Model)
	}
	if m.txSuggestionOffset != 15 {
		t.Fatalf("expected offset 15 at bottom of 20 items, got %d", m.txSuggestionOffset)
	}

	vLast := m.viewTXSuggestionsDrawer()
	if !strings.Contains(vLast, "cmd_19") || !strings.Contains(vLast, "cmd_15") {
		t.Errorf("expected cmd_15 through cmd_19 visible at bottom, got:\n%s", vLast)
	}
	if strings.Contains(vLast, "cmd_14") {
		t.Errorf("cmd_14 should not be visible at bottom offset, got:\n%s", vLast)
	}
	if !strings.Contains(vLast, "(20/20)") {
		t.Errorf("expected '(20/20)' at bottom, got:\n%s", vLast)
	}

	// Wrap around from bottom to top on Down
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(Model)
	if m.txSuggestionIdx != 0 || m.txSuggestionOffset != 0 {
		t.Fatalf("expected wrap around to idx 0, offset 0, got idx %d, offset %d", m.txSuggestionIdx, m.txSuggestionOffset)
	}

	// Wrap around from top to bottom on Up
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = updated.(Model)
	if m.txSuggestionIdx != 19 || m.txSuggestionOffset != 15 {
		t.Fatalf("expected wrap around to idx 19, offset 15, got idx %d, offset %d", m.txSuggestionIdx, m.txSuggestionOffset)
	}

	// PgUp jumps back by 5 (from 19 to 14, setting window offset to 14)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	m = updated.(Model)
	if m.txSuggestionIdx != 14 || m.txSuggestionOffset != 14 {
		t.Fatalf("expected pgup jump to idx 14, offset 14, got idx %d, offset %d", m.txSuggestionIdx, m.txSuggestionOffset)
	}

	// Tab completes the selected item (cmd_14)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	if !strings.Contains(m.txInput.Value, "cmd_14") {
		t.Errorf("expected completion of cmd_14, got %q", m.txInput.Value)
	}
	if m.txSuggestionOffset != 0 || len(m.txSuggestions) != 0 {
		t.Errorf("expected offset reset and suggestions cleared on apply")
	}
}

func TestTXSuggestionsMouseWheelScroll(t *testing.T) {
	m := newTestModel()
	m.mode = modeTXInput
	m.width = 80
	m.height = 24

	suggs := make([]encoder.Suggestion, 10)
	for i := 0; i < 10; i++ {
		suggs[i] = encoder.Suggestion{Value: fmt.Sprintf("val_%02d", i)}
	}
	m.txSuggestions = suggs
	m.txSuggestionIdx = 0
	m.txSuggestionOffset = 0

	// Wheel Down scrolls down
	updated, _ := m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelDown})
	m = updated.(Model)
	if m.txSuggestionIdx != 1 {
		t.Errorf("expected idx 1 on wheel down, got %d", m.txSuggestionIdx)
	}

	// Wheel Up scrolls back up
	updated, _ = m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelUp})
	m = updated.(Model)
	if m.txSuggestionIdx != 0 {
		t.Errorf("expected idx 0 on wheel up, got %d", m.txSuggestionIdx)
	}

	// Terminal mouse sequence "65" (down)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("\x1b[<65;20;20M")})
	m = updated.(Model)
	if m.txSuggestionIdx != 1 {
		t.Errorf("expected idx 1 on mouse seq 65, got %d", m.txSuggestionIdx)
	}

	// Terminal mouse sequence "64" (up)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("\x1b[<64;20;20M")})
	m = updated.(Model)
	if m.txSuggestionIdx != 0 {
		t.Errorf("expected idx 0 on mouse seq 64, got %d", m.txSuggestionIdx)
	}
}

func TestTXSuggestionsMouseClickApply(t *testing.T) {
	m := newTestModel()
	m.mode = modeTXInput
	m.width = 80
	m.height = 30
	m.txInput.SetTextAndCursor("test_", 5)

	suggs := make([]encoder.Suggestion, 15)
	for i := 0; i < 15; i++ {
		suggs[i] = encoder.Suggestion{Value: fmt.Sprintf("target_%02d", i)}
	}
	m.txSuggestions = suggs
	m.txSuggestionIdx = 0
	m.txSuggestionOffset = 5 // Visible: target_05 .. target_09

	// dH = 6. topY = (30 - 3) - 6 = 21.
	// Header is at Y = 21.
	// Row 0 (target_05) is at Y = 22.
	// Row 2 (target_07) is at Y = 24.
	updated, _ := m.Update(tea.MouseMsg{
		X:      15,
		Y:      24,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	})
	m = updated.(Model)

	if !strings.Contains(m.txInput.Value, "target_07") {
		t.Errorf("expected clicked suggestion 'target_07' to be applied, got %q", m.txInput.Value)
	}
	if len(m.txSuggestions) != 0 || m.txSuggestionOffset != 0 {
		t.Errorf("expected suggestions cleared and offset reset after click")
	}
}
