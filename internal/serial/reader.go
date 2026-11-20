package serial

import (
	"fmt"
	"os"
	"sync"
	"time"

	goserial "go.bug.st/serial"
)

const chanBufSize = 256

// Source is the abstraction over any line-oriented data source.
// SerialSource and FileSource both implement this interface, allowing the
// rest of the application to be source-agnostic.
type Source interface {
	// Lines returns a channel that emits raw lines as they arrive.
	// The channel is closed when the source is exhausted or Stop is called.
	Lines() <-chan string
	// Errors returns a channel for non-fatal read errors.
	Errors() <-chan error
	// Stop signals the source to stop and waits for its goroutine to exit.
	Stop()
	// Write transmits raw bytes to the underlying destination.
	Write(p []byte) (n int, err error)
}

// ── SerialSource ─────────────────────────────────────────────────────────────

// SerialSource reads from a physical serial port.
type SerialSource struct {
	port      goserial.Port
	lines     chan string
	errors    chan error
	stop      chan struct{}
	closeOnce sync.Once
	wg        sync.WaitGroup
	writeMu   sync.Mutex
}

// NewSerialSource opens the serial port described by cfg and starts reading.
func NewSerialSource(cfg Config) (*SerialSource, error) {
	mode := toSerialMode(cfg)
	port, err := goserial.Open(cfg.Port, &mode)
	if err != nil {
		return nil, fmt.Errorf("serial: cannot open %q: %w", cfg.Port, err)
	}

	s := &SerialSource{
		port:   port,
		lines:  make(chan string, chanBufSize),
		errors: make(chan error, chanBufSize),
		stop:   make(chan struct{}),
	}

	s.wg.Add(1)
	go s.readLoop()
	return s, nil
}

func (s *SerialSource) readLoop() {
	defer s.wg.Done()
	defer close(s.lines)
	defer close(s.errors)

	// The read timeout only bounds how long a blocked Read can delay noticing
	// Stop(); it carries no framing semantics. Line boundaries are decided
	// exclusively by the VT line discipline, so a pause mid-line never splits it.
	_ = s.port.SetReadTimeout(100 * time.Millisecond)

	buf := make([]byte, 4096)
	ld := NewLineDiscipline()
	stopped := false
	emit := func(e Event) {
		if stopped {
			return
		}
		text := e.Text
		if e.Kind == EventPrompt {
			text = MarkPromptSnapshotWithCol(text, e.Col)
		}
		select {
		case <-s.stop:
			stopped = true
		case s.lines <- text:
		}
	}

	for {
		select {
		case <-s.stop:
			return
		default:
		}

		n, err := s.port.Read(buf)
		if n > 0 {
			ld.Feed(buf[:n], emit)
			if stopped {
				return
			}
		}
		if err != nil {
			select {
			case <-s.stop:
				return
			default:
			}
			// Deliver any complete-but-unterminated row before reporting the error.
			ld.Flush(emit)
			select {
			case s.errors <- fmt.Errorf("serial read: %w", err):
			default:
			}
			return
		}
	}
}

// Lines implements Source.
func (s *SerialSource) Lines() <-chan string { return s.lines }

// Errors implements Source.
func (s *SerialSource) Errors() <-chan error { return s.errors }

// Stop closes the serial port and waits for the reader goroutine to finish.
func (s *SerialSource) Stop() {
	s.closeOnce.Do(func() {
		close(s.stop)
		_ = s.port.Close()
	})
	s.wg.Wait()
}

// Write transmits raw bytes to the physical serial port.
func (s *SerialSource) Write(p []byte) (int, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	select {
	case <-s.stop:
		return 0, fmt.Errorf("serial: port is closed")
	default:
	}

	if s.port == nil {
		return 0, fmt.Errorf("serial: port is nil")
	}

	return s.port.Write(p)
}

// ── FileSource ────────────────────────────────────────────────────────────────

// FileSource reads lines from a file, enabling log replay without hardware.
type FileSource struct {
	lines     chan string
	errors    chan error
	stop      chan struct{}
	closeOnce sync.Once
	wg        sync.WaitGroup
}

// NewFileSource opens the file at path and starts emitting lines.
func NewFileSource(path string) (*FileSource, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("file source: cannot open %q: %w", path, err)
	}

	s := &FileSource{
		lines:  make(chan string, chanBufSize),
		errors: make(chan error, chanBufSize),
		stop:   make(chan struct{}),
	}

	s.wg.Add(1)
	go s.readLoop(f)
	return s, nil
}

func (s *FileSource) readLoop(f *os.File) {
	defer s.wg.Done()
	defer close(s.lines)
	defer close(s.errors)
	defer f.Close()

	if err := streamReaderToLines(f, s.stop, s.lines); err != nil && err != os.ErrClosed {
		select {
		case <-s.stop:
			return
		default:
		}
		select {
		case s.errors <- fmt.Errorf("file read: %w", err):
		default:
		}
	}
}

// Lines implements Source.
func (s *FileSource) Lines() <-chan string { return s.lines }

// Errors implements Source.
func (s *FileSource) Errors() <-chan error { return s.errors }

// Stop signals the goroutine to stop and waits for it to exit.
func (s *FileSource) Stop() {
	s.closeOnce.Do(func() {
		close(s.stop)
	})
	s.wg.Wait()
}

// ErrReadOnlySource indicates that the source is read-only and cannot transmit data.
var ErrReadOnlySource = fmt.Errorf("source is read-only")

// Write implements Source for FileSource, returning ErrReadOnlySource.
func (s *FileSource) Write(p []byte) (int, error) {
	return 0, fmt.Errorf("file source: %w", ErrReadOnlySource)
}

