package mcp

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/brenoniehues/oh-my-logs/internal/config"
	"github.com/brenoniehues/oh-my-logs/internal/ipc"
	"github.com/brenoniehues/oh-my-logs/internal/parser"
	"github.com/brenoniehues/oh-my-logs/internal/record"
	"github.com/brenoniehues/oh-my-logs/internal/serial"
)

// Session manages the background serial ingestion, active profile parsing,
// and the thread-safe record ring buffer, supporting transparent IPC proxying
// when an active TUI session is running.
type Session struct {
	mu sync.RWMutex

	appCfg    *config.AppConfig
	buffer    *record.Buffer
	source    serial.Source
	serialCfg serial.Config

	profileName string
	profile     *parser.Profile
	parser      parser.Parser

	stopChan chan struct{}
	running  bool

	ipcClient *ipc.Client

	connectedAt time.Time
	lineCount   int64
	byteCount   int64
}

// NewSession creates a session with the provided configuration.
// If an active TUI instance is listening on the local IPC socket,
// the session attaches to it automatically.
func NewSession(appCfg *config.AppConfig, bufCap int) *Session {
	if bufCap <= 0 {
		bufCap = record.DefaultCapacity
	}

	configDir := ""
	if appCfg != nil {
		configDir = appCfg.ConfigDir
	}
	sockAddr := ipc.SocketPath(configDir)
	client := ipc.NewClient(sockAddr)

	return &Session{
		appCfg:    appCfg,
		buffer:    record.NewBuffer(bufCap),
		parser:    parser.NewRawParser(),
		ipcClient: client,
	}
}

// IsIPCAvailable checks if an active TUI instance is servicing requests.
func (s *Session) IsIPCAvailable() bool {
	if s.ipcClient == nil {
		return false
	}
	return s.ipcClient.IsAvailable()
}

// IPCClient returns the IPC client instance.
func (s *Session) IPCClient() *ipc.Client {
	return s.ipcClient
}

// Buffer returns the underlying thread-safe record buffer.
func (s *Session) Buffer() *record.Buffer {
	return s.buffer
}

// IsConnected reports whether a serial source is currently active.
func (s *Session) IsConnected() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running && s.source != nil
}

// Status returns current connection metadata and statistics.
type DeviceStatus struct {
	Connected   bool      `json:"connected"`
	Port        string    `json:"port,omitempty"`
	Baud        int       `json:"baud,omitempty"`
	Profile     string    `json:"profile,omitempty"`
	ConnectedAt time.Time `json:"connected_at,omitempty"`
	UptimeSec   float64   `json:"uptime_sec,omitempty"`
	BufferCount int       `json:"buffer_count"`
	BufferCap   int       `json:"buffer_cap"`
	TotalLines  int64     `json:"total_lines"`
	TotalBytes  int64     `json:"total_bytes"`
}

// GetStatus returns the current device status.
func (s *Session) GetStatus() DeviceStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stat := DeviceStatus{
		Connected:   s.running && s.source != nil,
		Port:        s.serialCfg.Port,
		Baud:        s.serialCfg.Baud,
		Profile:     s.profileName,
		BufferCount: s.buffer.Len(),
		BufferCap:   s.buffer.Cap(),
		TotalLines:  s.lineCount,
		TotalBytes:  s.byteCount,
	}

	if stat.Connected && !s.connectedAt.IsZero() {
		stat.ConnectedAt = s.connectedAt
		stat.UptimeSec = time.Since(s.connectedAt).Seconds()
	}
	return stat
}

// Connect establishes a connection to a specific serial port, baud rate, and profile.
// If an existing connection is active, it is cleanly closed first.
func (s *Session) Connect(port string, baud int, profileName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Clean up existing connection if present
	s.stopSourceLocked()

	if baud <= 0 {
		baud = 115200
	}

	cfg := serial.Config{
		Port: port,
		Baud: baud,
	}

	// Resolve and build parser
	p, prof, resolvedName, err := s.resolveParserLocked(profileName)
	if err != nil {
		return fmt.Errorf("failed resolving profile %q: %w", profileName, err)
	}

	src, err := serial.NewSerialSource(cfg)
	if err != nil {
		return fmt.Errorf("cannot connect to %s: %w", port, err)
	}

	s.source = src
	s.serialCfg = cfg
	s.parser = p
	s.profile = prof
	s.profileName = resolvedName
	s.connectedAt = time.Now()
	s.stopChan = make(chan struct{})
	s.running = true

	go s.readLoop(src, p, s.stopChan)

	return nil
}

// Disconnect cleanly stops the background reader and closes the active serial port.
func (s *Session) Disconnect() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running || s.source == nil {
		return fmt.Errorf("no serial device is currently connected")
	}

	s.stopSourceLocked()
	return nil
}

// Write sends raw command bytes to the connected serial device.
func (s *Session) Write(data []byte) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.running || s.source == nil {
		return 0, fmt.Errorf("cannot write: device is disconnected")
	}
	return s.source.Write(data)
}

// ClearBuffer clears the in-memory record buffer.
func (s *Session) ClearBuffer() {
	s.buffer.Clear()
}

func (s *Session) stopSourceLocked() {
	if s.running {
		s.running = false
		if s.stopChan != nil {
			close(s.stopChan)
		}
	}
	if s.source != nil {
		s.source.Stop()
		s.source = nil
	}
}

func (s *Session) resolveParserLocked(name string) (parser.Parser, *parser.Profile, string, error) {
	if name == "" || strings.EqualFold(name, "raw") {
		return parser.NewRawParser(), nil, "raw", nil
	}

	if s.appCfg == nil {
		return parser.NewRawParser(), nil, "raw", nil
	}

	path := config.ResolveProfilePath(s.appCfg, name)
	if path == "" {
		return nil, nil, "", fmt.Errorf("profile %q not found in profiles directory", name)
	}

	prof, err := parser.LoadProfile(path)
	if err != nil {
		return nil, nil, "", fmt.Errorf("cannot load profile %q: %w", path, err)
	}

	p, err := prof.BuildParser()
	if err != nil {
		return nil, nil, "", fmt.Errorf("cannot build parser for %q: %w", name, err)
	}

	return p, prof, prof.Name, nil
}

func (s *Session) readLoop(src serial.Source, p parser.Parser, stop <-chan struct{}) {
	lines := src.Lines()

	for {
		select {
		case <-stop:
			return
		case line, ok := <-lines:
			if !ok {
				return
			}
			clean := serial.CleanTerminalLine(line)
			if clean == "" {
				continue
			}

			rec, err := p.Parse(clean)
			if err != nil || rec.Fields == nil {
				rec = record.NewRecord(clean)
				rec.Fields["message"] = clean
			}
			if rec.Timestamp.IsZero() {
				rec.Timestamp = time.Now()
			}
			if rec.Raw == "" {
				rec.Raw = clean
			}

			s.buffer.Add(rec)

			s.mu.Lock()
			s.lineCount++
			s.byteCount += int64(len(clean))
			s.mu.Unlock()
		}
	}
}
