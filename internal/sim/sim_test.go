package sim_test

import (
	"bytes"
	"context"
	"io"
	"net"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/brenoniehues/oh-my-logs/internal/parser"
	"github.com/brenoniehues/oh-my-logs/internal/serial"
	"github.com/brenoniehues/oh-my-logs/internal/sim"
)

// startPipeDevice wires a simulated Device to a bidirectional net.Pipe so
// tests can send keystrokes and read raw UART bytes deterministically.
func startPipeDevice(t *testing.T, cfg sim.Config) (client net.Conn, stop func()) {
	t.Helper()
	devConn, cliConn := net.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	dev := sim.NewDevice(devConn, cfg)

	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = dev.Run(ctx)
	}()

	return cliConn, func() {
		cancel()
		_ = devConn.Close()
		_ = cliConn.Close()
		<-done
	}
}

func readUntil(t *testing.T, conn net.Conn, substr string, timeout time.Duration) string {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(timeout))
	defer func() { _ = conn.SetReadDeadline(time.Time{}) }()

	var out bytes.Buffer
	buf := make([]byte, 4096)
	for !strings.Contains(out.String(), substr) {
		n, err := conn.Read(buf)
		if n > 0 {
			out.Write(buf[:n])
		}
		if err != nil {
			t.Fatalf("readUntil(%q) failed after %q: %v", substr, out.String(), err)
		}
	}
	return out.String()
}

func TestParseProfileAndScenario(t *testing.T) {
	for _, name := range []string{"", "zephyr", "esp-idf", "linux"} {
		p, err := sim.ParseProfile(name)
		if err != nil {
			t.Fatalf("ParseProfile(%q): %v", name, err)
		}
		if p.Prompt == "" {
			t.Errorf("ParseProfile(%q) returned empty prompt", name)
		}
		line := p.FormatLog(1234*time.Millisecond, sim.LevelInfo, "net", "ready")
		if !strings.Contains(line, "ready") {
			t.Errorf("FormatLog(%q) = %q, missing message", name, line)
		}
	}
	if _, err := sim.ParseProfile("unknown-rtos"); err == nil {
		t.Error("expected error for unknown profile")
	}

	for _, sc := range []string{"", "normal", "vt-stress", "flood", "payloads", "all"} {
		if _, err := sim.ParseScenario(sc); err != nil {
			t.Fatalf("ParseScenario(%q): %v", sc, err)
		}
	}
	if _, err := sim.ParseScenario("invalid"); err == nil {
		t.Error("expected error for unknown scenario")
	}
}

func TestZephyrFormatMatchesBuiltinProfile(t *testing.T) {
	prof, err := parser.LoadProfile(filepath.Join("..", "..", "examples", "profiles", "zephyr.yaml"))
	if err != nil {
		t.Fatalf("LoadProfile(zephyr.yaml): %v", err)
	}
	p, err := prof.BuildParser()
	if err != nil {
		t.Fatalf("BuildParser: %v", err)
	}

	simProf, _ := sim.ParseProfile("zephyr")
	simProf.UseColor = false
	line := simProf.FormatLog(3165977*time.Microsecond, sim.LevelWarn, "wifi", "RSSI low (-82 dBm)")
	rec, err := p.Parse(line)
	if err != nil {
		t.Fatalf("Parse(%q): %v", line, err)
	}
	if rec.Fields["level"] != "wrn" || rec.Fields["module"] != "wifi" {
		t.Fatalf("unexpected parsed fields: %+v", rec.Fields)
	}
}

func TestShell_CommandsAndTabCompletion(t *testing.T) {
	cfg := sim.DefaultConfig()
	cfg.RateHz = 0
	cfg.MidLinePause = 0
	cfg.Seed = 42

	conn, stop := startPipeDevice(t, cfg)
	defer stop()

	// Wait for initial boot banner and prompt.
	boot := readUntil(t, conn, "uart:~$ ", 2*time.Second)
	if !strings.Contains(boot, "Booting Zephyr OS") {
		t.Fatalf("missing boot banner: %q", boot)
	}

	// Unique Tab completion: "st" is ambiguous (status, stress), "sta" -> "status".
	_, _ = io.WriteString(conn, "st\t")
	out := readUntil(t, conn, "uart:~$ st", 2*time.Second)
	if !strings.Contains(out, "status") || !strings.Contains(out, "stress") {
		t.Fatalf("ambiguous tab should list status & stress, got %q", out)
	}

	// Complete "a" + Tab -> "status", then Enter.
	_, _ = io.WriteString(conn, "a\t\r")
	out = readUntil(t, conn, "uart:~$ ", 2*time.Second)
	if !strings.Contains(out, "OK\r\nACK\r\n") {
		t.Fatalf("status command output = %q, want OK/ACK", out)
	}

	// Subcommand Tab completion: "ker" + Tab -> "kernel ", "ver" + Tab -> "version".
	_, _ = io.WriteString(conn, "ker\tver\t\r")
	out = readUntil(t, conn, "uart:~$ ", 2*time.Second)
	if !strings.Contains(out, "ARM Cortex-M4") {
		t.Fatalf("kernel version output = %q", out)
	}

	// Payload command emits JSON, hex, and telemetry.
	_, _ = io.WriteString(conn, "payload all\r")
	out = readUntil(t, conn, "uart:~$ ", 2*time.Second)
	if !strings.Contains(out, `"device":"node-01"`) || !strings.Contains(out, "RX PDU") || !strings.Contains(out, "current_mA=") {
		t.Fatalf("payload all missing expected formats: %q", out)
	}

	// VT stress command emits progress, backspace, UTF-8, and OK/ACK.
	_, _ = io.WriteString(conn, "stress vt\r")
	out = readUntil(t, conn, "uart:~$ ", 2*time.Second)
	if !strings.Contains(out, "100%") || !strings.Contains(out, "café") || !strings.Contains(out, "OK\r\nACK\r\n") {
		t.Fatalf("stress vt output incomplete: %q", out)
	}
}

func TestShell_LineEditingAndHistory(t *testing.T) {
	cfg := sim.DefaultConfig()
	cfg.RateHz = 0
	cfg.MidLinePause = 0

	conn, stop := startPipeDevice(t, cfg)
	defer stop()
	readUntil(t, conn, "uart:~$ ", 2*time.Second)

	// Type "statuz", Backspace, "s", Enter -> runs "status".
	_, _ = io.WriteString(conn, "statuz\x7fs\r")
	out := readUntil(t, conn, "uart:~$ ", 2*time.Second)
	if !strings.Contains(out, "OK\r\nACK\r\n") {
		t.Fatalf("backspace edit failed: %q", out)
	}

	// Press Up arrow to recall "status", then Enter.
	_, _ = io.WriteString(conn, "\x1b[A\r")
	out = readUntil(t, conn, "OK\r\nACK\r\n", 2*time.Second)
	if !strings.Contains(out, "uart:~$ status") {
		t.Fatalf("history recall did not redraw prompt: %q", out)
	}
	readUntil(t, conn, "uart:~$ ", 2*time.Second)

	// Mid-line cursor edit: type "hlp", Left Left, insert "e", Enter -> runs "help".
	_, _ = io.WriteString(conn, "hlp\x1b[D\x1b[De\r")
	readUntil(t, conn, "Available commands:", 2*time.Second)
	readUntil(t, conn, "uart:~$ ", 2*time.Second)

	// Ctrl+U kills line, then type "status" + Enter.
	_, _ = io.WriteString(conn, "garbage\x15status\r")
	readUntil(t, conn, "OK\r\nACK\r\n", 2*time.Second)
	readUntil(t, conn, "uart:~$ ", 2*time.Second)
}

func TestBurstThroughput(t *testing.T) {
	cfg := sim.DefaultConfig()
	cfg.RateHz = 0
	cfg.MidLinePause = 0
	cfg.Profile.UseColor = false

	conn, stop := startPipeDevice(t, cfg)
	defer stop()
	readUntil(t, conn, "uart:~$ ", 2*time.Second)

	_, _ = io.WriteString(conn, "burst 2000\r")
	out := readUntil(t, conn, "completed 2000 lines", 5*time.Second)
	if !strings.Contains(out, "flood line 1/2000") || !strings.Contains(out, "flood line 2000/2000") {
		t.Fatalf("burst missing boundary lines")
	}
}

func TestAsyncLogErasesAndRedrawsActiveInput(t *testing.T) {
	cfg := sim.DefaultConfig()
	cfg.RateHz = 25 // fast async tick for quick test
	cfg.Scenario = sim.ScenarioNormal
	cfg.MidLinePause = 10 * time.Millisecond
	cfg.Seed = 7

	conn, stop := startPipeDevice(t, cfg)
	defer stop()
	readUntil(t, conn, "uart:~$ ", 2*time.Second)

	// Type partial input without pressing Enter; async log must erase "uart:~$ par"
	// (11 chars -> ESC[11D ESC[J) and redraw "uart:~$ par" after the log line.
	_, _ = io.WriteString(conn, "par")
	out := readUntil(t, conn, "\x1b[11D\x1b[J", 2*time.Second)
	if !strings.Contains(out, "\x1b[11D\x1b[J") {
		t.Fatalf("expected prompt erase sequence, got %q", out)
	}
	out2 := readUntil(t, conn, "uart:~$ par", 2*time.Second)
	if !strings.Contains(out2, "uart:~$ par") {
		t.Fatalf("expected redrawn prompt with partial input, got %q", out2)
	}
}

func TestPTY_EndToEndWithSerialSource(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("PTY test requires macOS or Linux")
	}

	symlink := filepath.Join(t.TempDir(), "oml-sim-test")
	pty, err := sim.OpenPTY(symlink)
	if err != nil {
		t.Fatalf("OpenPTY: %v", err)
	}
	defer func() { _ = pty.Close() }()

	cfg := sim.DefaultConfig()
	cfg.RateHz = 0
	cfg.MidLinePause = 0

	ctx, cancel := context.WithCancel(context.Background())
	dev := sim.NewDevice(pty, cfg)
	devDone := make(chan struct{})
	go func() {
		defer close(devDone)
		_ = dev.Run(ctx)
	}()
	defer func() {
		cancel()
		_ = pty.Close()
		<-devDone
	}()

	src, err := serial.NewSerialSource(serial.Config{Port: symlink, Baud: 115200})
	if err != nil {
		t.Fatalf("NewSerialSource(%q): %v", symlink, err)
	}
	defer src.Stop()

	if _, err := src.Write([]byte("status\r")); err != nil {
		t.Fatalf("src.Write: %v", err)
	}

	seenOK := false
	seenACK := false
	deadline := time.After(3 * time.Second)
	for !(seenOK && seenACK) {
		select {
		case line, ok := <-src.Lines():
			if !ok {
				t.Fatal("serial source closed unexpectedly")
			}
			clean := serial.CleanTerminalLine(line)
			if clean == "OK" {
				seenOK = true
			}
			if clean == "ACK" {
				seenACK = true
			}
		case <-deadline:
			t.Fatalf("timed out waiting for OK/ACK over PTY (seenOK=%v, seenACK=%v)", seenOK, seenACK)
		}
	}
}
