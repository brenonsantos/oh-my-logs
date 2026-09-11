package tui

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

// DiskLogger provides asynchronous, buffered, non-blocking writing of incoming
// raw log lines directly to a file on disk.
type DiskLogger struct {
	path         string
	file         *os.File
	writer       *bufio.Writer
	ch           chan string
	done         chan struct{}
	linesWritten int64
	bytesWritten int64
	startTime    time.Time
	mu           sync.Mutex
	closed       bool
}

const (
	diskLoggerQueueSize   = 16384
	diskLoggerFlushPeriod = 500 * time.Millisecond
	diskLoggerBufferSize  = 64 * 1024 // 64 KB
)

// NewDiskLogger creates and starts a new asynchronous disk logger appending to targetPath.
func NewDiskLogger(targetPath string) (*DiskLogger, error) {
	if targetPath == "" {
		return nil, fmt.Errorf("target log path cannot be empty")
	}

	dir := filepath.Dir(targetPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create log directory %q: %w", dir, err)
	}

	f, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open log file %q: %w", targetPath, err)
	}

	dl := &DiskLogger{
		path:      targetPath,
		file:      f,
		writer:    bufio.NewWriterSize(f, diskLoggerBufferSize),
		ch:        make(chan string, diskLoggerQueueSize),
		done:      make(chan struct{}),
		startTime: time.Now(),
	}

	go dl.worker()
	return dl, nil
}

func (dl *DiskLogger) worker() {
	defer close(dl.done)
	ticker := time.NewTicker(diskLoggerFlushPeriod)
	defer ticker.Stop()

	for {
		select {
		case line, ok := <-dl.ch:
			if !ok {
				// Drain any remaining lines in channel
				for remaining := range dl.ch {
					dl.writeLineToBuffer(remaining)
				}
				dl.mu.Lock()
				_ = dl.writer.Flush()
				_ = dl.file.Sync()
				dl.mu.Unlock()
				return
			}
			dl.writeLineToBuffer(line)

		case <-ticker.C:
			dl.mu.Lock()
			_ = dl.writer.Flush()
			dl.mu.Unlock()
		}
	}
}

func (dl *DiskLogger) writeLineToBuffer(line string) {
	dl.mu.Lock()
	defer dl.mu.Unlock()

	n, _ := dl.writer.WriteString(line)
	nl, _ := dl.writer.WriteString("\n")
	atomic.AddInt64(&dl.linesWritten, 1)
	atomic.AddInt64(&dl.bytesWritten, int64(n+nl))
}

// WriteLine enqueues a log line for background persistence to disk.
func (dl *DiskLogger) WriteLine(line string) bool {
	if dl == nil {
		return false
	}
	dl.mu.Lock()
	if dl.closed {
		dl.mu.Unlock()
		return false
	}
	dl.mu.Unlock()

	dl.ch <- line
	return true
}

// Close flushes all queued log lines, stops the background worker, and closes the underlying file.
func (dl *DiskLogger) Close() error {
	if dl == nil {
		return nil
	}
	dl.mu.Lock()
	if dl.closed {
		dl.mu.Unlock()
		return nil
	}
	dl.closed = true
	close(dl.ch)
	dl.mu.Unlock()

	<-dl.done
	return dl.file.Close()
}

// Path returns the destination file path.
func (dl *DiskLogger) Path() string {
	if dl == nil {
		return ""
	}
	return dl.path
}

// Filename returns the basename of the log destination file.
func (dl *DiskLogger) Filename() string {
	if dl == nil {
		return ""
	}
	return filepath.Base(dl.path)
}

// LinesWritten returns the total number of lines appended to disk.
func (dl *DiskLogger) LinesWritten() int64 {
	if dl == nil {
		return 0
	}
	return atomic.LoadInt64(&dl.linesWritten)
}

// BytesWritten returns the total number of bytes written to disk.
func (dl *DiskLogger) BytesWritten() int64 {
	if dl == nil {
		return 0
	}
	return atomic.LoadInt64(&dl.bytesWritten)
}

// IsActive returns true if the logger is running and not closed.
func (dl *DiskLogger) IsActive() bool {
	if dl == nil {
		return false
	}
	dl.mu.Lock()
	defer dl.mu.Unlock()
	return !dl.closed
}

// GenerateTimestampLogPath generates a standardized log filename in targetDir with prefix "oml".
func GenerateTimestampLogPath(targetDir string) string {
	return GenerateTimestampLogPathWithPrefix(targetDir, "oml")
}

// GenerateTimestampLogPathWithPrefix generates a timestamped log path in targetDir using a custom prefix.
func GenerateTimestampLogPathWithPrefix(targetDir, prefix string) string {
	if targetDir == "" {
		targetDir = "."
	}
	if prefix == "" {
		prefix = "oml"
	}
	filename := fmt.Sprintf("%s-%s.log", prefix, time.Now().Format("2006-01-02T15-04-05"))
	return filepath.Join(targetDir, filename)
}
