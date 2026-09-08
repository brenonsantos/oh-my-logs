package serial

import (
	"os"
	"path/filepath"
	"testing"
	"time"
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
