package encoder

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// WorkerEncoder runs an external persistent subprocess communicating via JSON lines over stdin/stdout.
type WorkerEncoder struct {
	name        string
	trigger     string
	description string
	command     string
	extraPaths  []string

	mu           sync.Mutex
	cmd          *exec.Cmd
	stdin        io.WriteCloser
	stdoutReader *bufio.Reader
	closed       bool
	timeout      time.Duration
	lastPreview  string
}

// workerRequest represents a request sent to the encoder worker.
type workerRequest struct {
	Action string `json:"action"`
	Query  string `json:"query,omitempty"`
	Cursor int    `json:"cursor,omitempty"`
	Text   string `json:"text,omitempty"`
}

// workerResponse represents a response received from the encoder worker.
type workerResponse struct {
	Suggestions []Suggestion `json:"suggestions,omitempty"`
	Payload     string       `json:"payload,omitempty"`
	Display     string       `json:"display,omitempty"`
	Preview     string       `json:"preview,omitempty"`
	Error       string       `json:"error,omitempty"`
}

// NewWorkerEncoder constructs a WorkerEncoder for the given configuration.
func NewWorkerEncoder(cfg Config) (*WorkerEncoder, error) {
	if cfg.Exec == "" {
		return nil, errors.New("encoder exec command cannot be empty")
	}

	name := cfg.Name
	if name == "" {
		name = cfg.Trigger
	}
	if name == "" && len(cfg.Triggers) > 0 {
		name = cfg.Triggers[0]
	}
	if name == "" {
		name = cfg.Exec
	}

	trigger := cfg.Trigger
	if len(cfg.Triggers) > 0 {
		if trigger == "" {
			trigger = strings.Join(cfg.Triggers, ",")
		} else {
			trigger = trigger + "," + strings.Join(cfg.Triggers, ",")
		}
	}

	return &WorkerEncoder{
		name:        name,
		trigger:     trigger,
		description: cfg.Description,
		command:     cfg.Exec,
		extraPaths:  cfg.ExtraPaths,
		timeout:     600 * time.Millisecond,
	}, nil
}

func (w *WorkerEncoder) Name() string        { return w.name }
func (w *WorkerEncoder) Trigger() string     { return w.trigger }
func (w *WorkerEncoder) Description() string { return w.description }

// Suggest queries the worker for autocomplete suggestions matching query and cursor offset.
func (w *WorkerEncoder) Suggest(query string, cursor int) ([]Suggestion, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return nil, errors.New("encoder is closed")
	}

	req := workerRequest{
		Action: "suggest",
		Query:  query,
		Cursor: cursor,
	}

	resp, err := w.sendRequestLocked(req)
	if err != nil {
		return nil, err
	}

	if resp.Error != "" {
		return nil, errors.New(resp.Error)
	}

	w.lastPreview = resp.Preview
	return resp.Suggestions, nil
}

// LastPreview returns the latest live decoded or wire preview received from the worker.
func (w *WorkerEncoder) LastPreview() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.lastPreview
}

// Encode transforms human-readable text into wire-level payload.
func (w *WorkerEncoder) Encode(text string) (EncodeResult, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return EncodeResult{}, errors.New("encoder is closed")
	}

	req := workerRequest{
		Action: "encode",
		Text:   text,
	}

	resp, err := w.sendRequestLocked(req)
	if err != nil {
		return EncodeResult{}, err
	}

	if resp.Error != "" {
		return EncodeResult{}, errors.New(resp.Error)
	}

	payload := resp.Payload
	display := resp.Display
	if display == "" {
		display = text
	}

	return EncodeResult{
		Payload: payload,
		Display: display,
	}, nil
}

func (w *WorkerEncoder) sendRequestLocked(req workerRequest) (*workerResponse, error) {
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	if w.cmd == nil || w.cmd.Process == nil {
		if err := w.startWorkerLocked(); err != nil {
			return nil, err
		}
	}

	if err := w.writeLineLocked(string(reqBytes)); err != nil {
		// Attempt one restart if worker terminated
		if err := w.restartWorkerLocked(); err != nil {
			return nil, err
		}
		if err := w.writeLineLocked(string(reqBytes)); err != nil {
			return nil, err
		}
	}

	respLine, err := w.readLineWithTimeoutLocked(w.timeout)
	if err != nil {
		_ = w.killWorkerLocked()
		return nil, err
	}

	var resp workerResponse
	trimmed := strings.TrimSpace(respLine)
	if strings.HasPrefix(trimmed, "{") {
		if err := json.Unmarshal([]byte(trimmed), &resp); err != nil {
			return nil, fmt.Errorf("invalid json from encoder worker: %w", err)
		}
		return &resp, nil
	}

	// Plain text fallback (used as raw payload)
	return &workerResponse{
		Payload: trimmed,
		Display: req.Text,
	}, nil
}

func (w *WorkerEncoder) writeLineLocked(line string) error {
	if w.stdin == nil {
		return errors.New("stdin is nil")
	}
	_, err := fmt.Fprintf(w.stdin, "%s\n", line)
	return err
}

func (w *WorkerEncoder) readLineWithTimeoutLocked(timeout time.Duration) (string, error) {
	type readResult struct {
		line string
		err  error
	}

	ch := make(chan readResult, 1)
	go func() {
		line, err := w.stdoutReader.ReadString('\n')
		ch <- readResult{line: line, err: err}
	}()

	select {
	case res := <-ch:
		return res.line, res.err
	case <-time.After(timeout):
		return "", errors.New("encoder worker read timeout")
	}
}

func (w *WorkerEncoder) startWorkerLocked() error {
	resolvedCmd := w.command
	workDir := ""
	var cmd *exec.Cmd
	parts := strings.Fields(w.command)
	if len(parts) >= 2 && (parts[0] == "python" || parts[0] == "python3" || parts[0] == "sh" || parts[0] == "bash" || parts[0] == "node") {
		script := parts[1]
		if _, err := os.Stat(script); os.IsNotExist(err) {
			for _, dir := range w.extraPaths {
				cand := filepath.Join(dir, script)
				if fi, err := os.Stat(cand); err == nil && !fi.IsDir() {
					parts[1] = cand
					workDir = dir
					break
				}
			}
		} else {
			if fi, err := os.Stat(script); err == nil && !fi.IsDir() {
				workDir = filepath.Dir(script)
			}
		}
		cmd = exec.Command(parts[0], parts[1:]...)
	} else if len(parts) == 1 {
		binName := parts[0]
		for _, dir := range w.extraPaths {
			cand := filepath.Join(dir, binName)
			if fi, err := os.Stat(cand); err == nil && !fi.IsDir() {
				parts[0] = cand
				workDir = dir
				break
			}
		}
		if workDir != "" {
			cmd = exec.Command(parts[0])
		} else if runtime.GOOS == "windows" {
			cmd = exec.Command("cmd.exe", "/c", resolvedCmd)
		} else {
			cmd = exec.Command("sh", "-c", resolvedCmd)
		}
	} else if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd.exe", "/c", resolvedCmd)
	} else {
		cmd = exec.Command("sh", "-c", resolvedCmd)
	}

	if workDir != "" {
		cmd.Dir = workDir
	}

	if len(w.extraPaths) > 0 {
		sep := string(os.PathListSeparator)
		extraJoined := strings.Join(w.extraPaths, sep)
		currentPath := os.Getenv("PATH")
		newPath := extraJoined
		if currentPath != "" {
			newPath = extraJoined + sep + currentPath
		}
		env := os.Environ()
		pathFound := false
		for i, v := range env {
			if strings.HasPrefix(strings.ToUpper(v), "PATH=") {
				env[i] = "PATH=" + newPath
				pathFound = true
				break
			}
		}
		if !pathFound {
			env = append(env, "PATH="+newPath)
		}
		cmd.Env = env
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

	w.cmd = cmd
	w.stdin = stdin
	w.stdoutReader = bufio.NewReader(stdout)
	return nil
}

func (w *WorkerEncoder) restartWorkerLocked() error {
	_ = w.killWorkerLocked()
	return w.startWorkerLocked()
}

func (w *WorkerEncoder) killWorkerLocked() error {
	if w.stdin != nil {
		_ = w.stdin.Close()
		w.stdin = nil
	}
	if w.cmd != nil && w.cmd.Process != nil {
		_ = w.cmd.Process.Kill()
		_ = w.cmd.Wait()
		w.cmd = nil
	}
	w.stdoutReader = nil
	return nil
}

// Close terminates the worker process.
func (w *WorkerEncoder) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.closed = true
	return w.killWorkerLocked()
}
