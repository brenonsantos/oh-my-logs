package tui

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestDiskLogger_BasicWriteAndClose(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test_soak.log")

	dl, err := NewDiskLogger(logPath)
	if err != nil {
		t.Fatalf("unexpected error creating logger: %v", err)
	}

	if !dl.IsActive() {
		t.Fatalf("expected logger to be active")
	}
	if dl.Path() != logPath {
		t.Errorf("expected path %s, got %s", logPath, dl.Path())
	}
	if dl.Filename() != "test_soak.log" {
		t.Errorf("expected filename 'test_soak.log', got %s", dl.Filename())
	}

	lines := []string{
		"[10:00:01.000] system boot ok",
		"[10:00:01.120] sensor initialized",
		"[10:00:02.500] warning: battery low 3.2V",
	}

	for _, l := range lines {
		if !dl.WriteLine(l) {
			t.Fatalf("failed to write line: %s", l)
		}
	}

	if err := dl.Close(); err != nil {
		t.Fatalf("failed to close logger: %v", err)
	}

	if dl.IsActive() {
		t.Fatalf("expected logger to be inactive after close")
	}

	// Verify post-close WriteLine returns false
	if dl.WriteLine("should fail") {
		t.Errorf("expected WriteLine to return false after close")
	}

	// Verify file content on disk
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	expected := strings.Join(lines, "\n") + "\n"
	if string(content) != expected {
		t.Errorf("expected content:\n%q\ngot:\n%q", expected, string(content))
	}

	if dl.LinesWritten() != int64(len(lines)) {
		t.Errorf("expected %d lines written, got %d", len(lines), dl.LinesWritten())
	}
	if dl.BytesWritten() != int64(len(expected)) {
		t.Errorf("expected %d bytes written, got %d", len(expected), dl.BytesWritten())
	}
}

func TestDiskLogger_ConcurrentWrites(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "concurrent.log")

	dl, err := NewDiskLogger(logPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	const goroutines = 10
	const linesPerGoroutine = 100
	var wg sync.WaitGroup

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < linesPerGoroutine; i++ {
				dl.WriteLine("log message from worker")
			}
		}(g)
	}

	wg.Wait()
	_ = dl.Close()

	file, err := os.Open(logPath)
	if err != nil {
		t.Fatalf("failed to open file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	count := 0
	for scanner.Scan() {
		count++
	}

	expectedTotal := goroutines * linesPerGoroutine
	if count != expectedTotal {
		t.Errorf("expected %d lines on disk, got %d", expectedTotal, count)
	}
	if dl.LinesWritten() != int64(expectedTotal) {
		t.Errorf("expected %d LinesWritten stats, got %d", expectedTotal, dl.LinesWritten())
	}
}

func TestDiskLogger_NilAndEmptyPathSafety(t *testing.T) {
	var nilDL *DiskLogger
	if nilDL.IsActive() {
		t.Errorf("nil logger should not be active")
	}
	if nilDL.WriteLine("test") {
		t.Errorf("nil logger WriteLine should return false")
	}
	if err := nilDL.Close(); err != nil {
		t.Errorf("nil logger Close should return nil, got %v", err)
	}
	if nilDL.LinesWritten() != 0 || nilDL.BytesWritten() != 0 || nilDL.Path() != "" || nilDL.Filename() != "" {
		t.Errorf("nil logger getters should return zero values")
	}

	_, err := NewDiskLogger("")
	if err == nil {
		t.Errorf("expected error for empty target path")
	}
}

func TestDiskLogger_GenerateTimestampLogPath(t *testing.T) {
	path := GenerateTimestampLogPath("/var/log/oml")
	if !strings.HasPrefix(path, "/var/log/oml/oml-") || !strings.HasSuffix(path, ".log") {
		t.Errorf("unexpected generated log path: %s", path)
	}

	curDirPath := GenerateTimestampLogPath("")
	if !strings.HasPrefix(curDirPath, "oml-") && !strings.HasPrefix(curDirPath, "./oml-") {
		t.Errorf("unexpected generated log path for empty dir: %s", curDirPath)
	}
}
