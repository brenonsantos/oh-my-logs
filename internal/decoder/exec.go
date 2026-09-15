package decoder

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/brenoniehues/oh-my-logs/internal/record"
)

// ExecDecoder executes a persistent external process and streams payloads over stdin/stdout.
type ExecDecoder struct {
	re      *regexp.Regexp
	command string

	mu           sync.Mutex
	cmd          *exec.Cmd
	stdin        io.WriteCloser
	stdoutReader *bufio.Reader
	closed       bool
	timeout      time.Duration
}

type execResponse struct {
	Summary string                 `json:"summary"`
	Fields  map[string]interface{} `json:"fields"`
}

// NewExecDecoder creates an ExecDecoder for the given configuration.
func NewExecDecoder(cfg Config) (*ExecDecoder, error) {
	re, err := regexp.Compile(cfg.Match)
	if err != nil {
		return nil, err
	}
	return &ExecDecoder{
		re:      re,
		command: cfg.Exec,
		timeout: 800 * time.Millisecond,
	}, nil
}

// Decode checks if the candidate payload matches and sends it to the worker process.
func (e *ExecDecoder) Decode(r record.Record) (record.Record, bool) {
	if e.re == nil || e.command == "" {
		return r, false
	}

	candidate := r.Get("message")
	if candidate == "" {
		candidate = r.Get("msg")
	}
	if candidate == "" {
		candidate = r.Raw
	}

	if !e.re.MatchString(candidate) {
		return r, false
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return r, false
	}

	if e.cmd == nil || e.cmd.Process == nil {
		if err := e.startWorkerLocked(); err != nil {
			return r, false
		}
	}

	// Write candidate to worker process
	if err := e.writeLineLocked(candidate); err != nil {
		// Attempt one restart if worker terminated
		if err := e.restartWorkerLocked(); err != nil {
			return r, false
		}
		if err := e.writeLineLocked(candidate); err != nil {
			return r, false
		}
	}

	// Read response line with timeout
	respLine, err := e.readLineWithTimeoutLocked(e.timeout)
	if err != nil {
		// If read timed out or failed, terminate unresponsive worker
		_ = e.killWorkerLocked()
		return r, false
	}

	if r.Fields == nil {
		r.Fields = make(map[string]string)
	}

	if r.Fields["_raw_message"] == "" {
		r.Fields["_raw_message"] = candidate
	}

	// Parse JSON or plain text
	var resp execResponse
	trimmed := strings.TrimSpace(respLine)
	if strings.HasPrefix(trimmed, "{") && json.Unmarshal([]byte(trimmed), &resp) == nil {
		if resp.Summary != "" {
			r.Fields["message"] = resp.Summary
		}
		for k, v := range resp.Fields {
			r.Fields[k] = formatFieldValue(v)
		}
	} else if trimmed != "" {
		r.Fields["message"] = trimmed
	}

	return r, true
}

func (e *ExecDecoder) writeLineLocked(line string) error {
	if e.stdin == nil {
		return errors.New("stdin is nil")
	}
	_, err := fmt.Fprintf(e.stdin, "%s\n", line)
	return err
}

func (e *ExecDecoder) readLineWithTimeoutLocked(timeout time.Duration) (string, error) {
	type readResult struct {
		line string
		err  error
	}

	ch := make(chan readResult, 1)
	go func() {
		line, err := e.stdoutReader.ReadString('\n')
		ch <- readResult{line: line, err: err}
	}()

	select {
	case res := <-ch:
		return res.line, res.err
	case <-time.After(timeout):
		return "", errors.New("worker read timeout")
	}
}

func (e *ExecDecoder) startWorkerLocked() error {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd.exe", "/c", e.command)
	} else {
		cmd = exec.Command("sh", "-c", e.command)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return err
	}

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return err
	}

	e.cmd = cmd
	e.stdin = stdin
	e.stdoutReader = bufio.NewReader(stdout)
	return nil
}

func (e *ExecDecoder) restartWorkerLocked() error {
	_ = e.killWorkerLocked()
	return e.startWorkerLocked()
}

func (e *ExecDecoder) killWorkerLocked() error {
	if e.stdin != nil {
		_ = e.stdin.Close()
		e.stdin = nil
	}
	if e.cmd != nil && e.cmd.Process != nil {
		_ = e.cmd.Process.Kill()
		_ = e.cmd.Wait()
		e.cmd = nil
	}
	return nil
}

// Close terminates the worker process.
func (e *ExecDecoder) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.closed = true
	return e.killWorkerLocked()
}

func formatFieldValue(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case float64:
		if val == float64(int64(val)) {
			return strconv.FormatInt(int64(val), 10)
		}
		return strconv.FormatFloat(val, 'f', -1, 64)
	case bool:
		if val {
			return "true"
		}
		return "false"
	default:
		b, err := json.Marshal(v)
		if err == nil {
			return string(b)
		}
		return fmt.Sprintf("%v", v)
	}
}
