package tui

import (
	"sync"

	"github.com/brenoniehues/oh-my-logs/internal/serial"
	tea "github.com/charmbracelet/bubbletea"
)

// shellWriteErrMsg reports an asynchronous interactive shell TX write failure
// back to the Bubble Tea Update loop without blocking keystroke handling.
type shellWriteErrMsg struct {
	err error
}

// shellTXQueue serializes asynchronous shell writes in strict FIFO order using
// Bubble Tea commands, ensuring keystrokes never arrive out of order and a
// stalled UART/USB-CDC port never freezes the TUI.
type shellTXQueue struct {
	mu      sync.Mutex
	pending [][]byte
	running bool
}

func newShellTXQueue() *shellTXQueue {
	return &shellTXQueue{}
}

// enqueue appends a copy of p to the FIFO queue. If no drain worker is active,
// it returns a tea.Cmd that drains the queue until empty (or until an error
// occurs); otherwise it returns nil and the running worker will pick it up.
func (q *shellTXQueue) enqueue(src serial.Source, p []byte) tea.Cmd {
	if src == nil || len(p) == 0 {
		return nil
	}
	cp := make([]byte, len(p))
	copy(cp, p)

	q.mu.Lock()
	q.pending = append(q.pending, cp)
	if q.running {
		q.mu.Unlock()
		return nil
	}
	q.running = true
	q.mu.Unlock()

	return func() tea.Msg {
		for {
			q.mu.Lock()
			if len(q.pending) == 0 {
				q.running = false
				q.mu.Unlock()
				return nil
			}
			chunk := q.pending[0]
			q.pending[0] = nil
			q.pending = q.pending[1:]
			q.mu.Unlock()

			if _, err := src.Write(chunk); err != nil {
				q.mu.Lock()
				q.pending = nil
				q.running = false
				q.mu.Unlock()
				return shellWriteErrMsg{err: err}
			}
		}
	}
}
