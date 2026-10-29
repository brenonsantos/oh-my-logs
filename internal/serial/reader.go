package serial

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
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

	// Set a small read timeout so the reader can emit partial interactive prompts
	// (e.g. "uart:~$ os_") when UART goes idle waiting for engineer input, rather
	// than blocking indefinitely waiting for a trailing newline.
	_ = s.port.SetReadTimeout(25 * time.Millisecond)

	buf := make([]byte, 1024)
	var lineBuf []byte

	emitLine := func(l string) bool {
		cleanLine := strings.TrimRight(l, "\r\n")
		if cleanLine == "" {
			return true
		}
		select {
		case <-s.stop:
			return false
		case s.lines <- cleanLine:
			return true
		}
	}

	for {
		select {
		case <-s.stop:
			return
		default:
		}

		n, err := s.port.Read(buf)
		if err != nil {
			select {
			case <-s.stop:
				return
			default:
			}
			select {
			case s.errors <- fmt.Errorf("serial read: %w", err):
			default:
			}
			return
		}

		if n > 0 {
			chunk := buf[:n]
			for len(chunk) > 0 {
				idx := -1
				for i, b := range chunk {
					if b == '\n' {
						idx = i
						break
					}
				}

				if idx >= 0 {
					lineBuf = append(lineBuf, chunk[:idx]...)
					lineStr := string(lineBuf)
					lineBuf = lineBuf[:0]
					if !emitLine(lineStr) {
						return
					}
					chunk = chunk[idx+1:]
				} else {
					lineBuf = append(lineBuf, chunk...)
					break
				}
			}
		} else {
			// n == 0: Read timed out (25ms idle UART). If lineBuf contains a genuine idle interactive shell prompt
			// (e.g. "device:~$ " or "device:~$ os_") waiting for user input without a trailing newline,
			// flush it immediately so the TUI can update prompt & completion.
			// Never flush in-flight log messages (which may pause across USB or MCU workqueue delays).
			if len(lineBuf) > 0 {
				lineStr := string(lineBuf)
				if IsIdleShellPrompt(lineStr) {
					lineBuf = lineBuf[:0]
					if !emitLine(lineStr) {
						return
					}
				} else if len(lineBuf) > 65536 {
					// Safety failsafe against unbounded memory growth on corrupt streams without newlines.
					lineBuf = lineBuf[:0]
					if !emitLine(lineStr) {
						return
					}
				}
			}
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

	reader := bufio.NewReader(f)
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			cleanLine := strings.TrimRight(line, "\r\n")
			select {
			case <-s.stop:
				return
			case s.lines <- cleanLine:
			}
		}
		if err != nil {
			select {
			case <-s.stop:
				return
			default:
			}
			if err != io.EOF && err != os.ErrClosed {
				select {
				case s.errors <- fmt.Errorf("file read: %w", err):
				default:
				}
			}
			return
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

