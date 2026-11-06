package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/brenoniehues/oh-my-logs/internal/config"
	"github.com/brenoniehues/oh-my-logs/internal/encoder"
	"github.com/brenoniehues/oh-my-logs/internal/record"
	"github.com/brenoniehues/oh-my-logs/internal/serial"
)

func TestSaveLogWithDecodedRecords(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "oml-save-decoded-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	appCfg := &config.AppConfig{
		ConfigDir: tmpDir,
	}

	buf := record.NewBuffer(100)
	m := New(serial.DefaultConfig(), nil, nil, buf, nil, appCfg)

	ts := time.Date(2026, 11, 4, 10, 30, 0, 123000000, time.UTC)

	// 1. Regular undecoded record
	r1 := record.NewRecord("10:30:00.123 [INF] Normal system boot")
	r1.Timestamp = ts
	r1.Fields["message"] = "Normal system boot"
	m.buffer.Add(r1)

	// 2. Decoded record from external protocol decoder
	r2 := record.NewRecord("10:30:00.123 [RX] 00 5c 01 02")
	r2.Timestamp = ts
	r2.Fields["_raw_message"] = "00 5c 01 02"
	r2.Fields["message"] = "TelemetryData: temp=23.5C, voltage=3.3V"
	m.buffer.Add(r2)

	// Save log to file
	targetFile := filepath.Join(tmpDir, "saved_run.log")
	saveCmd := m.cmdSaveLogToPath(targetFile)
	res := saveCmd()
	savedMsg, ok := res.(LogSavedMsg)
	if !ok {
		t.Fatalf("expected LogSavedMsg, got %T", res)
	}
	if savedMsg.Err != nil {
		t.Fatalf("failed to save log: %v", savedMsg.Err)
	}

	data, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatalf("failed to read saved file: %v", err)
	}
	content := string(data)
	lines := strings.Split(strings.TrimSpace(content), "\n")

	// Expect 3 lines total:
	// Line 1: [2026-11-04T10:30:00.123] 10:30:00.123 [INF] Normal system boot
	// Line 2: [2026-11-04T10:30:00.123] 10:30:00.123 [RX] 00 5c 01 02
	// Line 3: [2026-11-04T10:30:00.123]   ↳ TelemetryData: temp=23.5C, voltage=3.3V
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines in saved log, got %d:\n%s", len(lines), content)
	}

	if !strings.Contains(lines[0], "Normal system boot") {
		t.Errorf("line 0 missing normal message: %s", lines[0])
	}
	if !strings.Contains(lines[1], "00 5c 01 02") {
		t.Errorf("line 1 missing raw packet: %s", lines[1])
	}
	if !strings.Contains(lines[2], "  ↳ TelemetryData: temp=23.5C, voltage=3.3V") {
		t.Errorf("line 2 missing decoded continuation: %s", lines[2])
	}
}

func TestDiskLoggerDecodedStream(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "oml-disklogger-decoded-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	targetFile := filepath.Join(tmpDir, "stream.log")
	dl, err := NewDiskLogger(targetFile)
	if err != nil {
		t.Fatalf("failed to init disk logger: %v", err)
	}
	defer dl.Close()

	buf := record.NewBuffer(100)
	m := New(serial.DefaultConfig(), nil, nil, buf, nil, nil)
	m.diskLogger = dl

	ts := time.Date(2026, 11, 4, 11, 0, 0, 500000000, time.UTC)

	// Ingest a decoded record
	r := record.NewRecord("11:00:00.500 [RX] 01 02 03")
	r.Timestamp = ts
	r.Fields["_raw_message"] = "01 02 03"
	r.Fields["message"] = "ServiceStatus: active"

	m.ingestRecord(r)

	// Flush and close disk logger
	_ = dl.Close()

	data, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatalf("failed to read disk log: %v", err)
	}
	content := string(data)
	lines := strings.Split(strings.TrimSpace(content), "\n")

	if len(lines) != 2 {
		t.Fatalf("expected 2 lines in disk log, got %d:\n%s", len(lines), content)
	}
	if !strings.Contains(lines[0], "01 02 03") {
		t.Errorf("expected line 0 raw packet, got: %s", lines[0])
	}
	if !strings.Contains(lines[1], "  ↳ ServiceStatus: active") {
		t.Errorf("expected line 1 decoded continuation, got: %s", lines[1])
	}
}

func TestTXCommandContinuationWithLivePreview(t *testing.T) {
	buf := record.NewBuffer(100)
	mockSrc := newTXMockSource()
	m := New(serial.DefaultConfig(), nil, nil, buf, mockSrc, nil)

	mockEnc := &mockTestEncoder{
		name:    "DeviceControl",
		trigger: "app_cmd ",
		encodeFunc: func(text string) (encoder.EncodeResult, error) {
			return encoder.EncodeResult{
				Payload: "app_cmd publish 83 92 0 10 213",
				Display: "DeviceControl: setTemp=21.3C",
			}, nil
		},
	}
	m.encoders = encoder.NewPipelineWithEncoders([]encoder.Encoder{mockEnc})

	// Case 1: Matching trigger -> encodes and captures continuation
	m.mode = modeTXInput
	m.txInput.SetText("app_cmd publish 83 92 0 10 213")
	m.txPreview = "DeviceControl: setTemp=21.3C"

	// Press Enter
	resM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = resM.(Model)

	// Verify buffer received 2 records: the wire command and the decoded continuation
	records := m.buffer.All()
	if len(records) != 2 {
		t.Fatalf("expected 2 records in buffer after TX with matching trigger, got %d", len(records))
	}

	if !strings.Contains(records[0].Raw, "app_cmd publish 83 92 0 10 213") {
		t.Errorf("expected record 0 to contain wire command, got %q", records[0].Raw)
	}
	if !strings.Contains(records[1].Raw, "↳ DeviceControl: setTemp=21.3C") {
		t.Errorf("expected record 1 to contain continuation line, got %q", records[1].Raw)
	}

	// Case 2: Non-matching command (e.g. "help") -> strictly sent as RAW serial text with NO continuation
	m.mode = modeTXInput
	m.txInput.SetText("help")
	m.txPreview = ""

	resM, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = resM.(Model)

	records = m.buffer.All()
	if len(records) != 3 {
		t.Fatalf("expected 3 records in buffer total (2 from case 1 + 1 raw command), got %d", len(records))
	}
	if !strings.Contains(records[2].Raw, "help") {
		t.Errorf("expected record 2 to be raw 'help', got %q", records[2].Raw)
	}
	if strings.Contains(records[2].Raw, "↳") {
		t.Errorf("raw command should not contain decoded continuation line, got %q", records[2].Raw)
	}
	if !strings.Contains(m.message, m.txEnding.String()) || strings.Contains(m.message, "DeviceControl") {
		t.Errorf("expected sent message to show ending [%s] and NOT [DeviceControl], got %q", m.txEnding.String(), m.message)
	}
}

func TestClipboardRowCopyDecoded(t *testing.T) {
	r := record.NewRecord("10:00:00 [RX] 00 AA FF")
	r.Fields["_raw_message"] = "00 AA FF"
	r.Fields["message"] = "MotorState: running speed=1200rpm"

	clip := rowClipboardText(r)
	expected := "10:00:00 [RX] 00 AA FF\n  ↳ MotorState: running speed=1200rpm"
	if clip != expected {
		t.Fatalf("expected clipboard text:\n%q\ngot:\n%q", expected, clip)
	}

	// Normal record without decoding
	normal := record.NewRecord("10:00:00 [INF] System ok")
	normal.Fields["message"] = "System ok"
	if clipNorm := rowClipboardText(normal); clipNorm != "10:00:00 [INF] System ok" {
		t.Fatalf("expected normal clipboard text without continuation, got %q", clipNorm)
	}
}

func TestReplaySavedLogWithContinuationLines(t *testing.T) {
	// Line with bracket timestamp and continuation indicator
	line := "[2026-11-04T10:30:00.123]   ↳ TelemetryData: temp=23.5C, voltage=3.3V"

	rec, ts := parseLineWithTimestampFallback(nil, line)
	expectedTs := time.Date(2026, 11, 4, 10, 30, 0, 123000000, time.UTC)

	if !ts.Equal(expectedTs) {
		t.Fatalf("expected parsed timestamp %v, got %v", expectedTs, ts)
	}

	if rec.Raw != line {
		t.Fatalf("expected rec.Raw to preserve full line, got %q", rec.Raw)
	}
}
