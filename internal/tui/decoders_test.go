package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brenoniehues/oh-my-logs/internal/config"
	"github.com/brenoniehues/oh-my-logs/internal/decoder"
	"github.com/brenoniehues/oh-my-logs/internal/filter"
	"github.com/brenoniehues/oh-my-logs/internal/parser"
	"github.com/brenoniehues/oh-my-logs/internal/record"
	"github.com/brenoniehues/oh-my-logs/internal/serial"
)

func TestTUIDecoders_DeclarativeIntegration(t *testing.T) {
	buf := record.NewBuffer(100)
	rawParser := parser.NewRawParser()
	m := New(serial.DefaultConfig(), nil, rawParser, buf, nil, nil)

	// Configure a declarative decoder
	m.SetProjectDecoders([]decoder.Config{
		{
			Match:  `^instruments,\s*(?P<cpu>\d+),\s*(?P<usage>\d+)`,
			Format: "CPU: {cpu}, Usage: {usage}%",
		},
	})

	// Ingest a raw line matching the decoder
	rec := record.NewRecord("instruments, 2, 85")
	rec.Fields["message"] = "instruments, 2, 85"
	m.ingestRecord(rec)

	if len(m.visible) != 1 {
		t.Fatalf("expected 1 visible record, got %d", len(m.visible))
	}

	ingested := m.visible[0]
	if ingested.Fields["message"] != "CPU: 2, Usage: 85%" {
		t.Errorf("expected formatted message, got %q", ingested.Fields["message"])
	}
	if ingested.Fields["cpu"] != "2" {
		t.Errorf("expected cpu=2, got %q", ingested.Fields["cpu"])
	}
	if ingested.Fields["usage"] != "85" {
		t.Errorf("expected usage=85, got %q", ingested.Fields["usage"])
	}
	if ingested.Fields["_raw_message"] != "instruments, 2, 85" {
		t.Errorf("expected _raw_message preserved, got %q", ingested.Fields["_raw_message"])
	}

	// Verify filtering by extracted field works
	flt1, _ := filter.New("cpu:2")
	m.tabs[0].Filter = flt1
	m.rebuildAllTabs()
	if len(m.visible) != 1 {
		t.Errorf("expected filter cpu:2 to match, got %d records", len(m.visible))
	}

	flt2, _ := filter.New("cpu:99")
	m.tabs[0].Filter = flt2
	m.rebuildAllTabs()
	if len(m.visible) != 0 {
		t.Errorf("expected filter cpu:99 to not match, got %d records", len(m.visible))
	}
}

func TestTUIDecoders_InspectorModalShowsRawAndDecoded(t *testing.T) {
	buf := record.NewBuffer(100)
	rawParser := parser.NewRawParser()
	m := New(serial.DefaultConfig(), nil, rawParser, buf, nil, nil)
	m.width = 120
	m.height = 40
	m.tableHeight = 30

	m.SetProjectDecoders([]decoder.Config{
		{
			Match:  `^telemetry:\s*(?P<temp>\d+C)`,
			Format: "Temperature: {temp}",
		},
	})

	rec := record.NewRecord("[00:01:00] telemetry: 42C")
	rec.Fields["message"] = "telemetry: 42C"
	m.ingestRecord(rec)

	m.selectedRow = 0
	m.mode = modeRowDetail

	view := m.viewRowDetailModal()

	// Should contain the decoded message
	if !strings.Contains(view, "Temperature: 42C") {
		t.Errorf("expected inspector view to contain decoded summary, got: %s", view)
	}

	// Should contain the ORIGINAL MESSAGE section with the raw candidate
	if !strings.Contains(view, "ORIGINAL MESSAGE") || !strings.Contains(view, "telemetry: 42C") {
		t.Errorf("expected inspector view to display ORIGINAL MESSAGE section, got: %s", view)
	}

	// Should display extracted temp field
	if !strings.Contains(view, "temp:") || !strings.Contains(view, "42C") {
		t.Errorf("expected inspector view to list extracted field temp: 42C, got: %s", view)
	}

	// Should NOT display _raw_message in the PARSED FIELDS list
	if strings.Contains(view, "_raw_message:") {
		t.Errorf("expected _raw_message to be hidden from PARSED FIELDS list, got: %s", view)
	}

	// Test FormattedRecordDetail helper used for clipboard copying
	formatted := FormattedRecordDetail(m.visible[0], "_ts", false)
	if !strings.Contains(formatted, "Original Message:") || !strings.Contains(formatted, "telemetry: 42C") {
		t.Errorf("expected FormattedRecordDetail to contain Original Message, got: %s", formatted)
	}
	if strings.Contains(formatted, "_raw_message:") {
		t.Errorf("expected FormattedRecordDetail to omit _raw_message from fields list, got: %s", formatted)
	}
}

func TestTUIDecoders_NestedDecodersDirDiscovery(t *testing.T) {
	tempDir := t.TempDir()
	nestedDecDir := filepath.Join(tempDir, "bundle", "nested")
	if err := os.MkdirAll(nestedDecDir, 0o755); err != nil {
		t.Fatalf("failed to create nested dir: %v", err)
	}

	appCfg := &config.AppConfig{
		DecodersDir: tempDir,
	}

	buf := record.NewBuffer(100)
	rawParser := parser.NewRawParser()
	m := New(serial.DefaultConfig(), nil, rawParser, buf, nil, appCfg)

	decoders := []decoder.Config{
		{
			Match: `^nested:\s*(.*)`,
			Format: "Handled: {0}",
		},
	}
	m.rebuildDecodersPipeline(decoders)

	if m.decoders == nil {
		t.Fatalf("expected pipeline to be created")
	}
}

func TestTUIDecoders_IngestLineWithHostTimestampAndDecoder(t *testing.T) {
	buf := record.NewBuffer(100)
	p, err := parser.NewRegexParser(`^\[\s*(?P<uptime>[^\]]+?)\s*\]\s+<(?P<level>[a-zA-Z]+)>\s+(?P<module>[a-zA-Z0-9_.-]+):\s*(?P<message>.*)$`)
	if err != nil {
		t.Fatalf("failed to create regex parser: %v", err)
	}

	m := New(serial.DefaultConfig(), nil, p, buf, nil, nil)
	m.SetProjectDecoders([]decoder.Config{
		{
			Match:  `^CMD:(?P<cmd>[A-Za-z0-9]+)`,
			Format: "Processed command: {cmd}",
		},
	})

	// Ingest line with host timestamp prefix as written by disk logger
	rawLine := "[2026-09-30T15:52:40.788] [  15863.410] <inf> sensor_node: CMD:RebootRequest"
	updatedModel, _ := m.Update(lineMsg(rawLine))
	m = updatedModel.(Model)

	if len(m.visible) != 1 {
		t.Fatalf("expected 1 record visible, got %d", len(m.visible))
	}

	rec := m.visible[0]
	if rec.Fields["level"] != "inf" {
		t.Errorf("expected level=inf, got %q", rec.Fields["level"])
	}
	if rec.Fields["uptime"] != "15863.410" {
		t.Errorf("expected uptime=15863.410, got %q", rec.Fields["uptime"])
	}
	if rec.Fields["module"] != "sensor_node" {
		t.Errorf("expected module=sensor_node, got %q", rec.Fields["module"])
	}
	if rec.Fields["message"] != "Processed command: RebootRequest" {
		t.Errorf("expected decoded message 'Processed command: RebootRequest', got %q", rec.Fields["message"])
	}
	if rec.Fields["_raw_message"] != "CMD:RebootRequest" {
		t.Errorf("expected _raw_message='CMD:RebootRequest', got %q", rec.Fields["_raw_message"])
	}
	if rec.Raw != rawLine {
		t.Errorf("expected full raw preserved, got %q", rec.Raw)
	}
}
