package serial

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"strings"
	"sync"
)

// ProcessSource runs an external command and captures its standard output and
// standard error as log lines, while supporting standard input writes for interactive TX.
type ProcessSource struct {
	cmdStr    string
	cmd       *exec.Cmd
	stdinPipe io.WriteCloser
	lines     chan string
	errors    chan error
	stop      chan struct{}
	closeOnce sync.Once
	wg        sync.WaitGroup
	writeMu   sync.Mutex
}

// NewProcessSource launches command via the system shell and streams its output.
func NewProcessSource(command string) (*ProcessSource, error) {
	if strings.TrimSpace(command) == "" {
		return nil, fmt.Errorf("process source: command cannot be empty")
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd.exe", "/c", command)
	} else {
		cmd = exec.Command("/bin/sh", "-c", command)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("process source: stdout pipe error: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		_ = stdout.Close()
		return nil, fmt.Errorf("process source: stderr pipe error: %w", err)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		_ = stdout.Close()
		_ = stderr.Close()
		return nil, fmt.Errorf("process source: stdin pipe error: %w", err)
	}

	if err := cmd.Start(); err != nil {
		_ = stdout.Close()
		_ = stderr.Close()
		_ = stdin.Close()
		return nil, fmt.Errorf("process source: cannot start %q: %w", command, err)
	}

	ps := &ProcessSource{
		cmdStr:    command,
		cmd:       cmd,
		stdinPipe: stdin,
		lines:     make(chan string, chanBufSize),
		errors:    make(chan error, chanBufSize),
		stop:      make(chan struct{}),
	}

	var pipeWg sync.WaitGroup
	pipeWg.Add(2)

	ps.wg.Add(1)
	go func() {
		defer ps.wg.Done()
		defer close(ps.lines)
		defer close(ps.errors)

		// Wait for both pipes to EOF, then wait for child process to terminate
		pipeWg.Wait()
		_ = ps.cmd.Wait()
	}()

	readPipe := func(r io.Reader) {
		defer pipeWg.Done()
		reader := bufio.NewReader(r)
		for {
			line, err := reader.ReadString('\n')
			if len(line) > 0 {
				cleanLine := strings.TrimRight(line, "\r\n")
				select {
				case <-ps.stop:
					return
				case ps.lines <- cleanLine:
				}
			}
			if err != nil {
				return
			}
		}
	}

	go readPipe(stdout)
	go readPipe(stderr)

	return ps, nil
}

// Command returns the shell command line being executed.
func (p *ProcessSource) Command() string {
	return p.cmdStr
}

// Lines implements Source.
func (p *ProcessSource) Lines() <-chan string {
	return p.lines
}

// Errors implements Source.
func (p *ProcessSource) Errors() <-chan error {
	return p.errors
}

// Stop terminates the running process and waits for reader goroutines to finish.
func (p *ProcessSource) Stop() {
	p.closeOnce.Do(func() {
		close(p.stop)
		if p.stdinPipe != nil {
			_ = p.stdinPipe.Close()
		}
		if p.cmd != nil && p.cmd.Process != nil {
			_ = p.cmd.Process.Kill()
		}
	})
	p.wg.Wait()
}

// Write transmits raw bytes to the process's standard input.
func (p *ProcessSource) Write(b []byte) (int, error) {
	p.writeMu.Lock()
	defer p.writeMu.Unlock()

	select {
	case <-p.stop:
		return 0, fmt.Errorf("process source: process is stopped")
	default:
	}

	if p.stdinPipe == nil {
		return 0, fmt.Errorf("process source: stdin is nil")
	}

	return p.stdinPipe.Write(b)
}
