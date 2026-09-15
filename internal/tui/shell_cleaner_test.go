package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brenoniehues/oh-my-logs/internal/parser"
	"github.com/brenoniehues/oh-my-logs/internal/record"
	"github.com/brenoniehues/oh-my-logs/internal/serial"
)

func TestTUI_UniversalShellPromptCleaning(t *testing.T) {
	// A profile WITHOUT any strip_prefix configuration.
	const yamlProfile = `name: CustomRTOS
parser:
  type: regex
  pattern: '^\[\s*(?P<uptime>[^\]]+?)\s*\]\s+<(?P<level>[a-zA-Z]+)>\s+(?P<module>[a-zA-Z0-9_.-]+):\s*(?P<message>.*)$'
columns:
  - field: uptime
    title: Uptime
    width: 10
  - field: level
    title: Level
    width: 5
  - field: module
    title: Module
    width: 10
  - field: message
    title: Message
    width: 0
`
	tmp := filepath.Join(t.TempDir(), "custom.yaml")
	if err := os.WriteFile(tmp, []byte(yamlProfile), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	prof, err := parser.LoadProfile(tmp)
	if err != nil {
		t.Fatalf("LoadProfile: %v", err)
	}
	p, err := prof.BuildParser()
	if err != nil {
		t.Fatalf("BuildParser: %v", err)
	}

	buf := record.NewBuffer(100)
	m := New(serial.DefaultConfig(), prof, p, buf, nil, nil)

	// Feed line 1: normal log line without prompt
	updated, _ := m.Update(lineMsg("[   5206.361] <inf> os_msg: CS:Vd/BzS6(JJ)CGvXa=MQAAAA,CFph3=MgUAAA"))
	m = updated.(Model)

	// Feed line 2: shell prompt with ANSI cursor-back + erase sequence
	updated, _ = m.Update(lineMsg("board:~$ \x1b[9D\x1b[J[   5208.360] <inf> os_msg: CS:Vd/BzS6(JL)CGvXa=MQAAAA,CFph3=LQUAAA"))
	m = updated.(Model)

	// Feed line 3: standalone prompt (should be ignored and not ingested)
	updated, _ = m.Update(lineMsg("board:~$ "))
	m = updated.(Model)

	if len(m.visible) != 2 {
		t.Fatalf("expected 2 visible records, got %d", len(m.visible))
	}

	r0 := m.visible[0]
	if r0.Fields["uptime"] != "5206.361" || r0.Fields["level"] != "inf" {
		t.Errorf("r0 unexpected fields: %+v", r0.Fields)
	}

	r1 := m.visible[1]
	if r1.Fields["uptime"] != "5208.360" {
		t.Errorf("r1 expected uptime=5208.360, got %q", r1.Fields["uptime"])
	}
	if r1.Fields["level"] != "inf" {
		t.Errorf("r1 expected level=inf, got %q", r1.Fields["level"])
	}
	if r1.Fields["module"] != "os_msg" {
		t.Errorf("r1 expected module=os_msg, got %q", r1.Fields["module"])
	}
	if strings.Contains(r1.Raw, "board:~$") {
		t.Errorf("r1.Raw contains shell prompt: %q", r1.Raw)
	}
	if strings.Contains(r1.Raw, "\x1b[9D") {
		t.Errorf("r1.Raw contains ANSI escape code: %q", r1.Raw)
	}
	expectedRaw := "[   5208.360] <inf> os_msg: CS:Vd/BzS6(JL)CGvXa=MQAAAA,CFph3=LQUAAA"
	if r1.Raw != expectedRaw {
		t.Errorf("r1.Raw expected %q, got %q", expectedRaw, r1.Raw)
	}
}

func TestTUI_SaveLog_CleansShellNoise(t *testing.T) {
	buf := record.NewBuffer(100)
	rawParser := parser.NewRawParser()
	m := New(serial.DefaultConfig(), nil, rawParser, buf, nil, nil)

	// Ingest line through lineMsg
	updated, _ := m.Update(lineMsg("board:~$ \x1b[9D\x1b[J[   5208.360] <inf> app: boot ok"))
	m = updated.(Model)

	outPath := filepath.Join(t.TempDir(), "saved.log")
	cmd := m.cmdSaveLogToPath(outPath)
	msg := cmd()
	savedMsg, ok := msg.(LogSavedMsg)
	if !ok || savedMsg.Err != nil {
		t.Fatalf("cmdSaveLogToPath failed: %+v", msg)
	}

	savedBytes, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	savedStr := string(savedBytes)
	if strings.Contains(savedStr, "board:~$") || strings.Contains(savedStr, "\x1b[9D") {
		t.Fatalf("saved file still contains shell prompt/escape sequences: %q", savedStr)
	}
	if !strings.Contains(savedStr, "[   5208.360] <inf> app: boot ok") {
		t.Fatalf("saved file missing clean log content: %q", savedStr)
	}
}
