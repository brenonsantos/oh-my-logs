package serial

import (
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	goserial "go.bug.st/serial"
)

func TestFileSourceReadingAndStop(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")
	content := "line 1\nline 2\r\nline 3\n"
	if err := os.WriteFile(logFile, []byte(content), 0600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	src, err := NewFileSource(logFile)
	if err != nil {
		t.Fatalf("NewFileSource failed: %v", err)
	}

	var lines []string
	for l := range src.Lines() {
		lines = append(lines, l)
	}

	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %v", len(lines), lines)
	}
	if lines[0] != "line 1" || lines[1] != "line 2" || lines[2] != "line 3" {
		t.Fatalf("unexpected lines: %v", lines)
	}

	// Verify Stop is idempotent and does not panic
	src.Stop()
	src.Stop()

	// Verify errors channel is closed
	select {
	case _, ok := <-src.Errors():
		if ok {
			t.Errorf("expected errors channel to be closed")
		}
	case <-time.After(500 * time.Millisecond):
		t.Errorf("timed out waiting for errors channel to close")
	}
}

// mockSerialPort implements goserial.Port for unit testing readLoop behavior.
type mockSerialPort struct {
	reads [][]byte
	mu    sync.Mutex
}

func (m *mockSerialPort) SetMode(mode *goserial.Mode) error               { return nil }
func (m *mockSerialPort) Write(p []byte) (n int, err error)               { return len(p), nil }
func (m *mockSerialPort) Drain() error                                   { return nil }
func (m *mockSerialPort) ResetInputBuffer() error                         { return nil }
func (m *mockSerialPort) ResetOutputBuffer() error                        { return nil }
func (m *mockSerialPort) SetDTR(dtr bool) error                           { return nil }
func (m *mockSerialPort) SetRTS(rts bool) error                           { return nil }
func (m *mockSerialPort) GetModemStatusBits() (*goserial.ModemStatusBits, error) {
	return &goserial.ModemStatusBits{}, nil
}
func (m *mockSerialPort) SetReadTimeout(t time.Duration) error { return nil }
func (m *mockSerialPort) Close() error                         { return nil }
func (m *mockSerialPort) Break(d time.Duration) error          { return nil }

func (m *mockSerialPort) Read(p []byte) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.reads) == 0 {
		return 0, io.EOF
	}
	next := m.reads[0]
	m.reads = m.reads[1:]
	if len(next) == 0 {
		// Simulates timeout (n=0, err=nil)
		return 0, nil
	}
	copy(p, next)
	return len(next), nil
}

func TestSerialSource_InFlightLogNeverSplitOnTimeout(t *testing.T) {
	// Chunk 1: Starts with shell prompt erasure and begins a log line that gets cut mid-word.
	// Empty chunk: Simulates 25ms UART pause (e.g. workqueue task taking 25ms).
	// Chunk 2: Finishes the log line and sends \r\n.
	mock := &mockSerialPort{
		reads: [][]byte{
			[]byte("device:~$ \x1b[9D\x1b[J[    262.937] <inf> [lib-mqtt]: HandlePublishToClo"),
			[]byte(""), // timeout event!
			[]byte("udOsWork(): Published message successfully\r\n"),
		},
	}

	src := &SerialSource{
		port:   mock,
		lines:  make(chan string, chanBufSize),
		errors: make(chan error, chanBufSize),
		stop:   make(chan struct{}),
	}

	src.wg.Add(1)
	go src.readLoop()

	var lines []string
	done := make(chan struct{})
	go func() {
		for l := range src.Lines() {
			lines = append(lines, l)
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for readLoop to finish")
	}

	// Must be exactly ONE combined line without splitting "HandlePublishToCloudOsWork"
	if len(lines) != 1 {
		t.Fatalf("expected 1 combined log line, got %d: %q", len(lines), lines)
	}

	wantPrefix := "device:~$ \x1b[9D\x1b[J[    262.937] <inf> [lib-mqtt]: HandlePublishToCloudOsWork(): Published message successfully"
	if lines[0] != wantPrefix {
		t.Errorf("line got %q, want %q", lines[0], wantPrefix)
	}
}

func TestSerialSource_IdlePromptFlushedOnTimeout(t *testing.T) {
	// Genuine idle prompt without newline followed by timeout
	mock := &mockSerialPort{
		reads: [][]byte{
			[]byte("device:~$ "),
			[]byte(""), // timeout event flushes idle prompt!
		},
	}

	src := &SerialSource{
		port:   mock,
		lines:  make(chan string, chanBufSize),
		errors: make(chan error, chanBufSize),
		stop:   make(chan struct{}),
	}

	src.wg.Add(1)
	go src.readLoop()

	var lines []string
	done := make(chan struct{})
	go func() {
		for l := range src.Lines() {
			lines = append(lines, l)
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for readLoop to finish")
	}

	if len(lines) != 1 {
		t.Fatalf("expected 1 flushed prompt, got %d: %q", len(lines), lines)
	}
	if lines[0] != "device:~$ " {
		t.Errorf("flushed prompt got %q, want %q", lines[0], "device:~$ ")
	}
}

