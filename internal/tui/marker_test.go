package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/brenoniehues/oh-my-logs/internal/config"
	"github.com/brenoniehues/oh-my-logs/internal/record"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func setupTestModelForMarkers(t *testing.T) Model {
	t.Helper()
	tempDir := t.TempDir()
	appCfg := &config.AppConfig{
		ConfigDir:   tempDir,
		ProfilesDir: filepath.Join(tempDir, "profiles"),
		LogsDir:     filepath.Join(tempDir, "logs"),
	}

	buf := record.NewBuffer(100)
	cols := []record.Column{
		{Field: "level", Title: "Level", Width: 7},
		{Field: "message", Title: "Message", Width: 0},
	}

	tab := Tab{
		Name: "Main",
		ViewportState: ViewportState{
			Follow: true,
		},
	}

	return Model{
		keys:         defaultKeyMap(),
		width:        100,
		height:       30,
		tableHeight:  20,
		buffer:       buf,
		columns:      cols,
		appConfig:    appCfg,
		settings:     &config.Settings{Theme: "Dark Slate"},
		bookmarks:    make(map[uint64]struct{}),
		tabs:         []Tab{tab},
		activeTab:    0,
		mode:         modeNormal,
		markerInput:  NewTextInput(true),
		tsFormat:     "15:04:05.000",
		tsField:      "time",
	}
}

func TestMarker_NewMarkerRecord(t *testing.T) {
	ts := time.Date(2026, 9, 18, 14, 30, 45, 123000000, time.UTC)

	// 1. With note
	r1 := record.NewMarkerRecord("unplugged sensor", ts)
	if !r1.IsMarker {
		t.Errorf("expected IsMarker true")
	}
	if r1.MarkerNote != "unplugged sensor" {
		t.Errorf("expected MarkerNote 'unplugged sensor', got %q", r1.MarkerNote)
	}
	if !strings.Contains(r1.Raw, "📌 MARKER [14:30:45.123]: unplugged sensor") {
		t.Errorf("unexpected raw marker text: %q", r1.Raw)
	}
	if r1.Fields["level"] != "MARK" {
		t.Errorf("expected level MARK, got %q", r1.Fields["level"])
	}

	// 2. Empty note
	r2 := record.NewMarkerRecord("", ts)
	if !r2.IsMarker {
		t.Errorf("expected IsMarker true")
	}
	if r2.MarkerNote != "" {
		t.Errorf("expected empty MarkerNote, got %q", r2.MarkerNote)
	}
	if !strings.Contains(r2.Raw, "📌 MARKER [14:30:45.123]") {
		t.Errorf("unexpected raw marker text: %q", r2.Raw)
	}
}

func TestMarker_OpenAndCancelPrompt(t *testing.T) {
	m := setupTestModelForMarkers(t)

	// Press 'm' to open prompt
	m.openMarkerPrompt()
	if m.mode != modeMarkerPrompt {
		t.Fatalf("expected modeMarkerPrompt, got %v", m.mode)
	}

	// Type some draft text
	m.markerInput.SetText("draft note")

	// Press Esc to cancel
	res, _ := m.handleMarkerPromptKey(tea.KeyMsg{Type: tea.KeyEsc})
	m = res.(Model)

	if m.mode != modeNormal {
		t.Errorf("expected modeNormal after Esc, got %v", m.mode)
	}
	if m.markerInput.Value != "" {
		t.Errorf("expected markerInput to be reset, got %q", m.markerInput.Value)
	}
	if m.buffer.Len() != 0 {
		t.Errorf("expected buffer to be empty, got %d", m.buffer.Len())
	}
}

func TestMarker_DropMarkerWithNote(t *testing.T) {
	m := setupTestModelForMarkers(t)
	m.openMarkerPrompt()

	m.markerInput.SetText("power cycle #1")

	// Press Enter to confirm
	res, _ := m.handleMarkerPromptKey(tea.KeyMsg{Type: tea.KeyEnter})
	m = res.(Model)

	if m.mode != modeNormal {
		t.Errorf("expected modeNormal after Enter, got %v", m.mode)
	}
	if m.buffer.Len() != 1 {
		t.Fatalf("expected 1 record in buffer, got %d", m.buffer.Len())
	}

	all := m.buffer.All()
	rec := all[0]
	if !rec.IsMarker {
		t.Errorf("expected record.IsMarker to be true")
	}
	if rec.MarkerNote != "power cycle #1" {
		t.Errorf("expected MarkerNote 'power cycle #1', got %q", rec.MarkerNote)
	}

	// Check auto-bookmarking
	if _, ok := m.bookmarks[rec.ID]; !ok {
		t.Errorf("expected marker record ID %d to be auto-bookmarked", rec.ID)
	}

	// Check visibility in active tab
	cur := m.currentTab()
	if len(cur.Visible) != 1 {
		t.Fatalf("expected 1 visible record in tab, got %d", len(cur.Visible))
	}
	if !cur.Visible[0].IsMarker {
		t.Errorf("expected visible record to be marker")
	}

	// Check status message
	if !strings.Contains(m.message, "power cycle #1") {
		t.Errorf("expected status message mentioning note, got %q", m.message)
	}
}

func TestMarker_VisibleInBookmarkedOnlyTab(t *testing.T) {
	m := setupTestModelForMarkers(t)

	// Set tab to BookmarkedOnly
	m.tabs[0].BookmarkedOnly = true

	// Add regular log (should not show)
	m.ingestRecord(record.NewRecord("regular log line"))
	if len(m.tabs[0].Visible) != 0 {
		t.Errorf("expected 0 visible for regular line in BookmarkedOnly tab, got %d", len(m.tabs[0].Visible))
	}

	// Drop marker
	m.recordMarker("bench milestone")
	if len(m.tabs[0].Visible) != 1 {
		t.Fatalf("expected marker to appear in BookmarkedOnly tab, got %d", len(m.tabs[0].Visible))
	}
	if !m.tabs[0].Visible[0].IsMarker {
		t.Errorf("expected visible row in BookmarkedOnly tab to be marker")
	}
}

func TestMarker_FormatMarkerRowRendering(t *testing.T) {
	ts := time.Date(2026, 9, 18, 14, 30, 45, 0, time.UTC)
	rec := record.NewMarkerRecord("test note", ts)

	rendered := formatMarkerRow(rec, 80, false, lipgloss.NoColor{})
	if !strings.Contains(rendered, "📌 MARKER") {
		t.Errorf("expected rendered banner to contain '📌 MARKER', got %q", rendered)
	}
	if !strings.Contains(rendered, "test note") {
		t.Errorf("expected rendered banner to contain 'test note', got %q", rendered)
	}
	if !strings.Contains(rendered, "14:30:45") {
		t.Errorf("expected rendered banner to contain timestamp, got %q", rendered)
	}
}

func TestMarker_DirectToDiskWriting(t *testing.T) {
	m := setupTestModelForMarkers(t)
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test-stream.log")

	dl, err := NewDiskLogger(logPath)
	if err != nil {
		t.Fatalf("failed to start DiskLogger: %v", err)
	}
	m.diskLogger = dl
	defer dl.Close()

	// Drop marker
	m.recordMarker("flash erased")

	// Close logger to flush and terminate worker
	_ = dl.Close()

	// Verify file on disk contains marker line
	contentBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read disk log: %v", err)
	}
	content := string(contentBytes)
	if !strings.Contains(content, "📌 MARKER") || !strings.Contains(content, "flash erased") {
		t.Errorf("expected disk log to contain marker line, got:\n%s", content)
	}
}

func TestMarker_InsertAtSelectedLine(t *testing.T) {
	m := setupTestModelForMarkers(t)

	// Ingest 3 records
	m.ingestRecord(record.NewRecord("line 1"))
	m.ingestRecord(record.NewRecord("line 2"))
	m.ingestRecord(record.NewRecord("line 3"))

	if len(m.visible) != 3 {
		t.Fatalf("expected 3 records, got %d", len(m.visible))
	}

	targetRecID := m.visible[1].ID

	// Select row 1 ("line 2")
	m.selectedRow = 1
	m.follow = false

	// Open marker prompt with 'm'
	m.openMarkerPrompt()

	if m.mode != modeMarkerPrompt {
		t.Fatalf("expected modeMarkerPrompt, got %v", m.mode)
	}
	if m.markerTargetRow != 1 {
		t.Fatalf("expected markerTargetRow 1, got %d", m.markerTargetRow)
	}
	if m.markerTargetID != targetRecID {
		t.Fatalf("expected markerTargetID %d, got %d", targetRecID, m.markerTargetID)
	}

	// Verify marker drawer renders target line information
	drawerView := m.viewMarkerDrawer()
	if !strings.Contains(drawerView, "📌 ADD STREAM MARKER") {
		t.Errorf("expected drawer to contain title, got:\n%s", drawerView)
	}
	if !strings.Contains(drawerView, "Line #2") {
		t.Errorf("expected drawer to mention Line #2, got:\n%s", drawerView)
	}

	// Type note and press Enter
	m.markerInput.SetText("glitch occurred here")
	res, _ := m.handleMarkerPromptKey(tea.KeyMsg{Type: tea.KeyEnter})
	m = res.(Model)

	if m.mode != modeNormal {
		t.Errorf("expected modeNormal after submit, got %v", m.mode)
	}
	if len(m.visible) != 4 {
		t.Fatalf("expected 4 visible records after insertion, got %d", len(m.visible))
	}

	// Verify marker is at index 1 (the selected row)
	if !m.visible[1].IsMarker {
		t.Errorf("expected row at index 1 to be marker, got %+v", m.visible[1])
	}
	if m.visible[1].MarkerNote != "glitch occurred here" {
		t.Errorf("expected note 'glitch occurred here', got %q", m.visible[1].MarkerNote)
	}

	// Verify the original line 2 was shifted to index 2
	if m.visible[2].ID != targetRecID {
		t.Errorf("expected original record ID %d at index 2, got %d", targetRecID, m.visible[2].ID)
	}

	// Verify selection highlights the newly inserted marker
	if m.selectedRow != 1 {
		t.Errorf("expected selectedRow to point at marker (index 1), got %d", m.selectedRow)
	}

	// Verify buffer also contains the marker before the target record
	allBuf := m.buffer.All()
	if len(allBuf) != 4 {
		t.Fatalf("expected 4 records in buffer, got %d", len(allBuf))
	}
	if allBuf[1].MarkerNote != "glitch occurred here" {
		t.Errorf("expected buffer index 1 to be marker, got %q", allBuf[1].MarkerNote)
	}
	if allBuf[2].ID != targetRecID {
		t.Errorf("expected buffer index 2 to be target record, got %d", allBuf[2].ID)
	}

	// Verify status message
	if !strings.Contains(m.message, "Inserted marker at line 2") {
		t.Errorf("expected status message mentioning line 2, got %q", m.message)
	}
}

func TestMarker_RowDetailModal_PressM(t *testing.T) {
	m := setupTestModelForMarkers(t)
	m.ingestRecord(record.NewRecord("inspected line"))
	m.selectedRow = 0

	// Open inspector
	m.mode = modeRowDetail

	// Press 'm' in row detail modal
	res, _ := m.handleRowDetailKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	m = res.(Model)

	if m.mode != modeMarkerPrompt {
		t.Fatalf("expected pressing 'm' in row detail to enter modeMarkerPrompt, got %v", m.mode)
	}
	if m.markerTargetRow != 0 {
		t.Errorf("expected markerTargetRow 0, got %d", m.markerTargetRow)
	}
}

func TestMarker_DeleteInNormalMode(t *testing.T) {
	m := setupTestModelForMarkers(t)
	m.ingestRecord(record.NewRecord("line 1"))
	m.recordMarker("marker to delete")
	m.ingestRecord(record.NewRecord("line 2"))

	if len(m.visible) != 3 {
		t.Fatalf("expected 3 records, got %d", len(m.visible))
	}

	markerID := m.visible[1].ID
	if !m.visible[1].IsMarker {
		t.Fatalf("expected row 1 to be marker")
	}

	// Focus marker row
	m.selectedRow = 1

	// Press 'd' to delete marker
	res, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = res.(Model)

	if len(m.visible) != 2 {
		t.Fatalf("expected 2 visible records after deleting marker, got %d", len(m.visible))
	}
	if m.visible[0].Raw != "line 1" || m.visible[1].Raw != "line 2" {
		t.Fatalf("unexpected records remaining: %v", m.visible)
	}
	if m.buffer.Len() != 2 {
		t.Fatalf("expected 2 records in buffer, got %d", m.buffer.Len())
	}
	if _, ok := m.bookmarks[markerID]; ok {
		t.Errorf("expected marker ID %d to be removed from bookmarks", markerID)
	}
	if !strings.Contains(m.message, "Removed marker") {
		t.Errorf("expected status message mentioning 'Removed marker', got %q", m.message)
	}

	// Try deleting with 'delete' and 'backspace' keys on another marker
	m.recordMarker("second marker")
	if len(m.visible) != 3 {
		t.Fatalf("expected 3 records after adding second marker, got %d", len(m.visible))
	}
	m.selectedRow = 2

	res, _ = m.Update(tea.KeyMsg{Type: tea.KeyDelete})
	m = res.(Model)
	if len(m.visible) != 2 {
		t.Fatalf("expected 2 visible records after pressing Delete, got %d", len(m.visible))
	}
}

func TestMarker_NormalRecordNotDeletedByD(t *testing.T) {
	m := setupTestModelForMarkers(t)
	m.ingestRecord(record.NewRecord("important log line"))
	m.selectedRow = 0

	// Press 'd' on regular log row (must NOT delete)
	res, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = res.(Model)

	if len(m.visible) != 1 {
		t.Fatalf("expected regular log not to be deleted by 'd', got len %d", len(m.visible))
	}
	if m.buffer.Len() != 1 {
		t.Fatalf("expected buffer to still have 1 record, got %d", m.buffer.Len())
	}
}

func TestMarker_EditMarkerNote(t *testing.T) {
	m := setupTestModelForMarkers(t)
	m.recordMarker("initial milestone")

	if len(m.visible) != 1 || m.visible[0].MarkerNote != "initial milestone" {
		t.Fatalf("expected marker with note 'initial milestone'")
	}
	markerID := m.visible[0].ID

	// Select the marker row
	m.selectedRow = 0

	// Press 'm' to edit
	m.openMarkerPrompt()

	if !m.markerIsEditing {
		t.Errorf("expected markerIsEditing to be true")
	}
	if m.markerEditID != markerID {
		t.Errorf("expected markerEditID %d, got %d", markerID, m.markerEditID)
	}
	if m.markerInput.Value != "initial milestone" {
		t.Errorf("expected markerInput to be pre-filled with 'initial milestone', got %q", m.markerInput.Value)
	}

	// Change text to 'updated milestone'
	m.markerInput.SetText("updated milestone")
	res, _ := m.handleMarkerPromptKey(tea.KeyMsg{Type: tea.KeyEnter})
	m = res.(Model)

	if m.mode != modeNormal {
		t.Errorf("expected return to modeNormal after save")
	}
	if len(m.visible) != 1 {
		t.Fatalf("expected still 1 marker (no duplicate inserted), got %d", len(m.visible))
	}
	if m.visible[0].MarkerNote != "updated milestone" {
		t.Errorf("expected updated note 'updated milestone', got %q", m.visible[0].MarkerNote)
	}
	if m.visible[0].ID != markerID {
		t.Errorf("expected marker ID to be preserved (%d), got %d", markerID, m.visible[0].ID)
	}
	if !strings.Contains(m.message, "Updated marker") {
		t.Errorf("expected status message mentioning 'Updated marker', got %q", m.message)
	}
}

func TestMarker_DeleteFromDrawerCtrlD(t *testing.T) {
	m := setupTestModelForMarkers(t)
	m.recordMarker("marker to delete via drawer")
	m.selectedRow = 0

	// Open prompt on marker
	m.openMarkerPrompt()
	if !m.markerIsEditing {
		t.Fatalf("expected markerIsEditing true")
	}

	// Press Ctrl+D
	res, _ := m.handleMarkerPromptKey(tea.KeyMsg{Type: tea.KeyCtrlD})
	m = res.(Model)

	if m.mode != modeNormal {
		t.Errorf("expected modeNormal after Ctrl+D")
	}
	if len(m.visible) != 0 {
		t.Fatalf("expected marker to be removed, got %d visible", len(m.visible))
	}
	if m.buffer.Len() != 0 {
		t.Fatalf("expected buffer to be empty, got %d", m.buffer.Len())
	}
}

func TestMarker_DeleteFromRowDetail(t *testing.T) {
	m := setupTestModelForMarkers(t)
	m.recordMarker("marker in detail view")
	m.selectedRow = 0

	// Open detail view
	m.mode = modeRowDetail

	// Press 'd' in detail view
	res, _ := m.handleRowDetailKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = res.(Model)

	if m.mode != modeNormal {
		t.Errorf("expected detail modal to close after deleting marker, got mode %v", m.mode)
	}
	if len(m.visible) != 0 {
		t.Fatalf("expected marker to be deleted from row detail modal, got %d", len(m.visible))
	}
}

func TestMarker_KeyBarHintsForMarkerRow(t *testing.T) {
	m := setupTestModelForMarkers(t)
	m.recordMarker("hint marker")
	m.selectedRow = 0

	keyBar := m.viewKeyBar()
	if !strings.Contains(keyBar, "delete marker") {
		t.Errorf("expected key bar to contain 'delete marker' when marker is selected, got:\n%s", keyBar)
	}
	if !strings.Contains(keyBar, "edit marker") {
		t.Errorf("expected key bar to contain 'edit marker' when marker is selected, got:\n%s", keyBar)
	}
}


