package serial

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"sync"
)

// PipeSource reads lines from an arbitrary io.Reader (such as standard input).
type PipeSource struct {
	lines     chan string
	errors    chan error
	stop      chan struct{}
	closeOnce sync.Once
	wg        sync.WaitGroup
}

// NewPipeSource creates a new Source that reads lines from r.
func NewPipeSource(r io.Reader) *PipeSource {
	ps := &PipeSource{
		lines:  make(chan string, chanBufSize),
		errors: make(chan error, chanBufSize),
		stop:   make(chan struct{}),
	}

	ps.wg.Add(1)
	go ps.readLoop(r)
	return ps
}

func (p *PipeSource) readLoop(r io.Reader) {
	defer p.wg.Done()
	defer close(p.lines)
	defer close(p.errors)

	reader := bufio.NewReader(r)
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			cleanLine := strings.TrimRight(line, "\r\n")
			select {
			case <-p.stop:
				return
			case p.lines <- cleanLine:
			}
		}
		if err != nil {
			select {
			case <-p.stop:
				return
			default:
			}
			if err != io.EOF {
				select {
				case p.errors <- fmt.Errorf("pipe read: %w", err):
				default:
				}
			}
			return
		}
	}
}

// Lines implements Source.
func (p *PipeSource) Lines() <-chan string {
	return p.lines
}

// Errors implements Source.
func (p *PipeSource) Errors() <-chan error {
	return p.errors
}

// Stop signals the reader to stop and waits for the goroutine to exit.
func (p *PipeSource) Stop() {
	p.closeOnce.Do(func() {
		close(p.stop)
	})
	p.wg.Wait()
}

// Write implements Source for PipeSource, returning ErrReadOnlySource.
func (p *PipeSource) Write(b []byte) (int, error) {
	return 0, fmt.Errorf("pipe source: %w", ErrReadOnlySource)
}
