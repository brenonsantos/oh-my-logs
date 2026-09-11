package serial

import (
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestProcessSource_EmptyCommand(t *testing.T) {
	_, err := NewProcessSource("")
	if err == nil {
		t.Fatalf("expected error for empty command, got nil")
	}
}

func TestProcessSource_ExecutionAndOutput(t *testing.T) {
	var cmd string
	if runtime.GOOS == "windows" {
		cmd = "echo line one & echo line two"
	} else {
		cmd = "echo line one; echo line two"
	}

	src, err := NewProcessSource(cmd)
	if err != nil {
		t.Fatalf("failed to start process source: %v", err)
	}
	defer src.Stop()

	if src.Command() != cmd {
		t.Errorf("expected Command() %q, got %q", cmd, src.Command())
	}

	var received []string
	timeout := time.After(3 * time.Second)

	done := false
	for !done {
		select {
		case line, ok := <-src.Lines():
			if !ok {
				done = true
				break
			}
			received = append(received, strings.TrimSpace(line))
		case err, ok := <-src.Errors():
			if ok && err != nil {
				t.Fatalf("unexpected error from source: %v", err)
			}
		case <-timeout:
			t.Fatalf("timed out waiting for process output, received: %v", received)
		}
	}

	if len(received) < 2 {
		t.Fatalf("expected at least 2 lines, got %d: %v", len(received), received)
	}
	if received[0] != "line one" || received[1] != "line two" {
		t.Errorf("unexpected output lines: %v", received)
	}
}

func TestProcessSource_StdinWrite(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping cat test on windows")
	}

	src, err := NewProcessSource("cat")
	if err != nil {
		t.Fatalf("failed to start process source: %v", err)
	}
	defer src.Stop()

	testLine := "interactive command test\n"
	n, err := src.Write([]byte(testLine))
	if err != nil {
		t.Fatalf("failed to write to process stdin: %v", err)
	}
	if n != len(testLine) {
		t.Errorf("expected to write %d bytes, wrote %d", len(testLine), n)
	}

	select {
	case line, ok := <-src.Lines():
		if !ok {
			t.Fatalf("lines channel closed prematurely")
		}
		if line != "interactive command test" {
			t.Errorf("expected echo 'interactive command test', got %q", line)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for cat echo")
	}
}

func TestProcessSource_StopGraceful(t *testing.T) {
	var cmd string
	if runtime.GOOS == "windows" {
		cmd = "ping 127.0.0.1 -n 5"
	} else {
		cmd = "sleep 10"
	}

	src, err := NewProcessSource(cmd)
	if err != nil {
		t.Fatalf("failed to start process source: %v", err)
	}

	stopped := make(chan struct{})
	go func() {
		src.Stop()
		close(stopped)
	}()

	select {
	case <-stopped:
		// success
	case <-time.After(2 * time.Second):
		t.Fatalf("Stop() did not terminate process within 2 seconds")
	}

	// After stop, write should return error
	_, err = src.Write([]byte("test\n"))
	if err == nil {
		t.Errorf("expected error writing to stopped process, got nil")
	}
}
