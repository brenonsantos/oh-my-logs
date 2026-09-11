package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/brenoniehues/oh-my-logs/internal/record"
	tea "github.com/charmbracelet/bubbletea"
)

func TestRowDetailModal_OpenAndClose(t *testing.T) {
	// 1. Empty buffer
	m := newTestModelWithRecords(nil)
	m, _ = m.openRowDetail()
	if m.mode == modeRowDetail {
		t.Fatal("expected mode != modeRowDetail when buffer is empty")
	}

	// 2. Buffer with records
	records := []record.Record{
		{
			ID:        1,
			Raw:       "first log line",
			Timestamp: time.Now(),
		},
		{
			ID:        2,
			Raw:       `{"event":"heartbeat","seq":1}`,
			Timestamp: time.Now(),
		},
	}
	m = newTestModelWithRecords(records)
	m.selectedRow = -1
	m.follow = true

	m, _ = m.openRowDetail()
	if m.mode != modeRowDetail {
		t.Fatalf("expected mode=modeRowDetail, got %v", m.mode)
	}
	if m.selectedRow != 1 {
		t.Errorf("expected selectedRow=1 (latest follow row), got %d", m.selectedRow)
	}

	// 3. Close with Escape
	res, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = res.(Model)
	if m.mode != modeNormal {
		t.Errorf("expected mode=modeNormal after Escape, got %v", m.mode)
	}

	// 4. Open with 'v' and close with 'v'
	res, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	m = res.(Model)
	if m.mode != modeRowDetail {
		t.Fatalf("expected mode=modeRowDetail after 'v', got %v", m.mode)
	}

	res, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	m = res.(Model)
	if m.mode != modeNormal {
		t.Errorf("expected mode=modeNormal after second 'v', got %v", m.mode)
	}

	// 5. Open with Enter and close with Enter
	res, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = res.(Model)
	if m.mode != modeRowDetail {
		t.Fatalf("expected mode=modeRowDetail after Enter, got %v", m.mode)
	}

	res, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = res.(Model)
	if m.mode != modeNormal {
		t.Errorf("expected mode=modeNormal after second Enter, got %v", m.mode)
	}
}

func TestRowDetailModal_NavigationAndScrolling(t *testing.T) {
	records := []record.Record{
		{ID: 10, Raw: "record zero", Timestamp: time.Now()},
		{ID: 11, Raw: "record one", Timestamp: time.Now()},
		{ID: 12, Raw: "record two", Timestamp: time.Now()},
	}
	m := newTestModelWithRecords(records)
	m.selectedRow = 0
	m.mode = modeRowDetail

	// Navigate next record with 'l' or ']'
	res, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	m = res.(Model)
	if m.selectedRow != 1 {
		t.Errorf("expected selectedRow=1 after 'l', got %d", m.selectedRow)
	}

	res, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}})
	m = res.(Model)
	if m.selectedRow != 2 {
		t.Errorf("expected selectedRow=2 after ']', got %d", m.selectedRow)
	}

	// Navigate prev record with 'h' or '['
	res, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	m = res.(Model)
	if m.selectedRow != 1 {
		t.Errorf("expected selectedRow=1 after 'h', got %d", m.selectedRow)
	}

	// Vertical scrolling with 'j' and 'k'
	res, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = res.(Model)
	if m.detailScrollOffset != 1 {
		t.Errorf("expected detailScrollOffset=1 after 'j', got %d", m.detailScrollOffset)
	}

	res, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m = res.(Model)
	if m.detailScrollOffset != 0 {
		t.Errorf("expected detailScrollOffset=0 after 'k', got %d", m.detailScrollOffset)
	}

	// PageDown and Home
	res, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	m = res.(Model)
	if m.detailScrollOffset != 8 {
		t.Errorf("expected detailScrollOffset=8 after PgDn, got %d", m.detailScrollOffset)
	}

	res, _ = m.Update(tea.KeyMsg{Type: tea.KeyHome})
	m = res.(Model)
	if m.detailScrollOffset != 0 {
		t.Errorf("expected detailScrollOffset=0 after Home, got %d", m.detailScrollOffset)
	}
}

func TestRowDetailModal_CopyAndBookmark(t *testing.T) {
	records := []record.Record{
		{
			ID:        99,
			Raw:       "raw test log line",
			Fields:    map[string]string{"level": "ERROR", "module": "radio"},
			Timestamp: time.Now(),
		},
	}
	m := newTestModelWithRecords(records)
	m.selectedRow = 0
	m.mode = modeRowDetail

	// Bookmark toggle with 'b'
	res, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	m = res.(Model)
	if _, ok := m.bookmarks[99]; !ok {
		t.Error("expected record #99 to be bookmarked after 'b'")
	}

	// Copy detail with 'y'
	res, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = res.(Model)
	if !strings.Contains(m.message, "Copied record #99 details") {
		t.Errorf("unexpected message after 'y': %q", m.message)
	}

	// Copy raw with 'Y'
	res, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'Y'}})
	m = res.(Model)
	if !strings.Contains(m.message, "Copied raw log #99") {
		t.Errorf("unexpected message after 'Y': %q", m.message)
	}
}

func TestRowDetailModal_PinMovePinNext(t *testing.T) {
	records := []record.Record{
		{ID: 1, Raw: "log 1", Timestamp: time.Now()},
		{ID: 2, Raw: "log 2", Timestamp: time.Now()},
		{ID: 3, Raw: "log 3", Timestamp: time.Now()},
	}
	m := newTestModelWithRecords(records)
	m.selectedRow = 0
	m.mode = modeRowDetail

	// 1. Pin first record (#1)
	res, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	m = res.(Model)
	if _, ok := m.bookmarks[1]; !ok {
		t.Fatalf("expected record #1 to be bookmarked")
	}

	// 2. Move to next record (#2)
	res, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	m = res.(Model)
	if m.selectedRow != 1 {
		t.Fatalf("expected selectedRow=1, got %d", m.selectedRow)
	}

	// 3. Pin second record (#2)
	res, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	m = res.(Model)

	t.Logf("Bookmarks after pinning second record: %+v", m.bookmarks)
	t.Logf("Message: %s", m.message)
	if _, ok := m.bookmarks[1]; !ok {
		t.Errorf("expected record #1 to STILL be bookmarked")
	}
	if _, ok := m.bookmarks[2]; !ok {
		t.Errorf("expected record #2 to BE bookmarked")
	}
}

func TestRowDetailModal_PinStreamingIngest(t *testing.T) {
	m := newTestModel()
	m.width = 120
	m.height = 30
	m.tableHeight = 20

	// Ingest 3 records
	for i := 1; i <= 3; i++ {
		r := record.NewRecord(fmt.Sprintf("log %d", i))
		m.ingestRecord(r)
	}

	// User opens detail modal
	res, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = res.(Model)
	rec1, _ := m.activeInspectorRecord()
	t.Logf("Record 1 ID: %d, selectedRow: %d", rec1.ID, m.selectedRow)

	// User pins
	res, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	m = res.(Model)

	// Now a new record is ingested while modal is open!
	rNew := record.NewRecord("new log while modal open")
	m.ingestRecord(rNew)

	// User moves to next
	res, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = res.(Model)
	rec2, _ := m.activeInspectorRecord()
	t.Logf("Record 2 ID: %d, selectedRow: %d", rec2.ID, m.selectedRow)

	// User pins next
	res, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	m = res.(Model)

	t.Logf("Bookmarks: %+v", m.bookmarks)
	if len(m.bookmarks) != 2 {
		t.Errorf("expected 2 bookmarks, got %d", len(m.bookmarks))
	}
}

func TestRowDetailModal_SplitViewPinMovePinNext(t *testing.T) {
	records := []record.Record{
		{ID: 1, Raw: "log 1", Timestamp: time.Now()},
		{ID: 2, Raw: "log 2", Timestamp: time.Now()},
		{ID: 3, Raw: "log 3", Timestamp: time.Now()},
	}
	m := newTestModelWithRecords(records)
	m.splitMode = SplitVertical
	m.activePane = 1 // Right pane!
	t1 := m.currentTabForPane(1)
	t1.Visible = records
	t1.SelectedRow = 0
	m.mode = modeRowDetail

	// 1. Pin first record (#1)
	res, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	m = res.(Model)

	// 2. Move to next record (#2)
	res, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = res.(Model)

	// 3. Pin second record (#2)
	res, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	m = res.(Model)

	t.Logf("Split view bookmarks: %+v", m.bookmarks)
	if len(m.bookmarks) != 2 {
		t.Errorf("expected 2 bookmarks in split view, got %d", len(m.bookmarks))
	}
}

func TestRowDetailModal_ViewRendering(t *testing.T) {
	records := []record.Record{
		{
			ID:        105,
			Raw:       `2026-09-11 10:00:00 [ERR] net: {"status":"disconnected","attempts":5}`,
			Fields:    map[string]string{"level": "ERROR", "module": "net", "message": `{"status":"disconnected","attempts":5}`},
			Timestamp: time.Now(),
			Delta:     25 * time.Millisecond,
		},
	}
	m := newTestModelWithRecords(records)
	m.selectedRow = 0
	m.mode = modeRowDetail

	viewStr := m.View()
	if !strings.Contains(viewStr, "Log Row Inspector") {
		t.Errorf("expected 'Log Row Inspector' in View, got:\n%s", viewStr)
	}
	if !strings.Contains(viewStr, "#105") {
		t.Errorf("expected '#105' in View, got:\n%s", viewStr)
	}
	if !strings.Contains(viewStr, "PARSED FIELDS") {
		t.Errorf("expected 'PARSED FIELDS' in View, got:\n%s", viewStr)
	}
	if !strings.Contains(viewStr, "JSON FORMATTED") {
		t.Errorf("expected '[JSON FORMATTED]' in View, got:\n%s", viewStr)
	}
	if !strings.Contains(viewStr, "HEX PREVIEW") {
		t.Errorf("expected 'HEX PREVIEW' in View, got:\n%s", viewStr)
	}
}

func TestRowDetailModal_MultiFormatRendering(t *testing.T) {
	// XML record
	xmlRec := record.Record{
		ID:     201,
		Fields: map[string]string{"message": `<response status="ok"><data value="true"/></response>`},
		Raw:    `<response status="ok"><data value="true"/></response>`,
	}
	m := newTestModelWithRecords([]record.Record{xmlRec})
	m.selectedRow = 0
	m.mode = modeRowDetail
	viewXML := m.View()
	if !strings.Contains(viewXML, "XML FORMATTED") {
		t.Errorf("expected '[XML FORMATTED]' in View for XML, got:\n%s", viewXML)
	}

	// YAML record
	yamlRec := record.Record{
		ID:     202,
		Fields: map[string]string{"message": "server:\n  port: 8080\n  host: 127.0.0.1"},
		Raw:    "server:\n  port: 8080\n  host: 127.0.0.1",
	}
	m = newTestModelWithRecords([]record.Record{yamlRec})
	m.selectedRow = 0
	m.mode = modeRowDetail
	viewYAML := m.View()
	if !strings.Contains(viewYAML, "YAML FORMATTED") {
		t.Errorf("expected '[YAML FORMATTED]' in View for YAML, got:\n%s", viewYAML)
	}

	// Logfmt record
	logfmtRec := record.Record{
		ID:     203,
		Fields: map[string]string{"message": `level=info msg="sync complete" bytes=1024 success=true`},
		Raw:    `level=info msg="sync complete" bytes=1024 success=true`,
	}
	m = newTestModelWithRecords([]record.Record{logfmtRec})
	m.selectedRow = 0
	m.mode = modeRowDetail
	viewLogfmt := m.View()
	if !strings.Contains(viewLogfmt, "LOGFMT FORMATTED") {
		t.Errorf("expected '[LOGFMT FORMATTED]' in View for Logfmt, got:\n%s", viewLogfmt)
	}

	// Prefixed YAML record (from sample_inspect.log row 4)
	prefixedYamlRec := record.Record{
		ID:  204,
		Raw: `2026-09-11 11:30:02.180 [INF] config: manifest:\n  version: "1.2.0"\n  environment: production\n  services:\n    telemetry:\n      enabled: true\n      port: 9090\n      protocol: udp\n    ingest:\n      buffer_size: 50000\n      flush_ms: 250\n  tags:\n    - embedded\n    - serial\n    - zephyr`,
	}
	m = newTestModelWithRecords([]record.Record{prefixedYamlRec})
	m.selectedRow = 0
	m.mode = modeRowDetail
	viewPrefixedYAML := m.View()
	if !strings.Contains(viewPrefixedYAML, "YAML FORMATTED") {
		t.Errorf("expected '[YAML FORMATTED]' in View for prefixed YAML, got:\n%s", viewPrefixedYAML)
	}
}

