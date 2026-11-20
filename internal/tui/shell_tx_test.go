package tui

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type blockingTXSource struct {
	lines   chan string
	errors  chan error
	gate    chan struct{}
	mu      sync.Mutex
	written []string
}

func (s *blockingTXSource) Lines() <-chan string { return s.lines }
func (s *blockingTXSource) Errors() <-chan error { return s.errors }
func (s *blockingTXSource) Stop()                {}
func (s *blockingTXSource) Write(p []byte) (int, error) {
	<-s.gate
	s.mu.Lock()
	s.written = append(s.written, string(p))
	s.mu.Unlock()
	return len(p), nil
}

func TestListenToSource_BatchesBufferedLines(t *testing.T) {
	src := &txMockSource{
		lines:  make(chan string, 256),
		errors: make(chan error, 1),
	}
	for i := 0; i < 200; i++ {
		src.lines <- fmt.Sprintf("line-%d", i)
	}

	cmd := listenToSource(src)
	msg := cmd()
	batch, ok := msg.(lineBatchMsg)
	if !ok {
		t.Fatalf("expected lineBatchMsg, got %T", msg)
	}
	if len(batch) != maxLineBatchSize {
		t.Fatalf("expected batch of %d lines, got %d", maxLineBatchSize, len(batch))
	}

	// Second drain gets the remaining 72 lines.
	msg2 := listenToSource(src)()
	batch2, ok := msg2.(lineBatchMsg)
	if !ok {
		t.Fatalf("expected second lineBatchMsg, got %T", msg2)
	}
	if len(batch2) != 200-maxLineBatchSize {
		t.Fatalf("expected remaining %d lines, got %d", 200-maxLineBatchSize, len(batch2))
	}
}

func TestLineBatchMsg_IngestsAllLinesInOneUpdate(t *testing.T) {
	m := newTestModel()
	batch := make(lineBatchMsg, 50)
	for i := range batch {
		batch[i] = fmt.Sprintf("burst message %d", i)
	}
	u, _ := m.Update(batch)
	m = u.(Model)
	if m.buffer.Len() != 50 {
		t.Fatalf("expected 50 ingested records, got %d", m.buffer.Len())
	}
}

func TestShellTXQueue_NonBlockingAndFIFO(t *testing.T) {
	src := &blockingTXSource{
		lines:  make(chan string),
		errors: make(chan error),
		gate:   make(chan struct{}),
	}
	m := newTestModel()
	m.source = src
	m.connState = ConnConnected
	m.mode = modeShellInteractive

	// Enqueue "help" while src.Write is completely stalled; Update must return immediately.
	var cmds []tea.Cmd
	start := time.Now()
	for _, r := range []rune("help") {
		u, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = u.(Model)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Fatalf("Update blocked on stalled serial port (%v)", elapsed)
	}
	// Only the first keystroke spawns a drain cmd; subsequent ones coalesce onto the FIFO queue.
	if len(cmds) != 1 {
		t.Fatalf("expected 1 coalesced drain cmd, got %d", len(cmds))
	}

	// Start the drain worker, then unblock the serial port.
	done := make(chan tea.Msg, 1)
	go func() { done <- cmds[0]() }()
	close(src.gate)

	select {
	case msg := <-done:
		if msg != nil {
			t.Fatalf("unexpected error msg: %v", msg)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for shell TX drain")
	}

	src.mu.Lock()
	got := strings.Join(src.written, "")
	src.mu.Unlock()
	if got != "help" {
		t.Fatalf("FIFO order broken: got %q, want %q", got, "help")
	}
}
