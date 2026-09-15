package tui

import (
	"strings"
	"testing"

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
