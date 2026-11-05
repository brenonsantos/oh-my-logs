package mcp

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
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

	reconnecting bool
	connDetail   string

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

	s := &Session{
		appCfg:    appCfg,
		buffer:    record.NewBuffer(bufCap),
		parser:    parser.NewRawParser(),
		ipcClient: client,
	}

	// If an active TUI session is running, hydrate the in-memory buffer
	// with all pre-existing records so Gemini / MCP has immediate access to history!
	if client.IsAvailable() {
		s.SyncFromIPC()
	} else if appCfg != nil && appCfg.LogsDir != "" {
		s.SeedFromRecentDiskLog(2 * time.Hour)
	}

	return s
}

// SyncFromIPC synchronizes the local session state and in-memory buffer
// with an active running TUI instance.
func (s *Session) SyncFromIPC() {
	if s.ipcClient == nil || !s.ipcClient.IsAvailable() {
		return
	}

	// 1. Fetch remote status
	stat, err := s.ipcClient.GetStatus()
	if err == nil {
		s.mu.Lock()
		if prof, ok := stat["profile"].(string); ok && prof != "" {
			s.profileName = prof
		}
		if p, ok := stat["port"].(string); ok && p != "" {
			s.serialCfg.Port = p
		}
		if b, ok := stat["baud"].(float64); ok && b > 0 {
			s.serialCfg.Baud = int(b)
		}
		s.mu.Unlock()
	}

	// 2. Fetch existing in-memory logs up to buffer capacity
	bufCap := s.buffer.Cap()
	if bufCap <= 0 {
		bufCap = record.DefaultCapacity
	}
	logs, err := s.ipcClient.GetRecentLogs(bufCap, "", "")
	if err == nil && len(logs) > 0 {
		s.mu.Lock()
		s.buffer.Clear()
		for _, r := range logs {
			s.buffer.Add(r)
		}
		s.lineCount = int64(len(logs))
		s.mu.Unlock()
	}
}

// SeedFromRecentDiskLog attempts to load records from the most recently modified log file
// in the logs directory into the in-memory buffer.
func (s *Session) SeedFromRecentDiskLog(maxAge time.Duration) (string, int) {
	if s.appCfg == nil || s.appCfg.LogsDir == "" {
		return "", 0
	}

	entries, err := os.ReadDir(s.appCfg.LogsDir)
	if err != nil {
		return "", 0
	}

	var newestPath string
	var newestMod time.Time
	for _, entry := range entries {
		if entry.IsDir() || (!strings.HasSuffix(entry.Name(), ".txt") && !strings.HasSuffix(entry.Name(), ".log")) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if maxAge > 0 && time.Since(info.ModTime()) > maxAge {
			continue
		}
		if info.ModTime().After(newestMod) {
			newestMod = info.ModTime()
			newestPath = filepath.Join(s.appCfg.LogsDir, entry.Name())
		}
	}

	if newestPath == "" {
		return "", 0
	}

	f, err := os.Open(newestPath)
	if err != nil {
		return "", 0
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	count := 0
	s.mu.Lock()
	defer s.mu.Unlock()
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		r := record.Record{
			ID:        uint64(count + 1),
			Raw:       line,
			Timestamp: newestMod,
		}
		s.buffer.Add(r)
		count++
	}
	s.lineCount = int64(count)
	return newestPath, count
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
	Connected    bool      `json:"connected"`
	Reconnecting bool      `json:"reconnecting,omitempty"`
	Detail       string    `json:"detail,omitempty"`
	Port         string    `json:"port,omitempty"`
	Baud         int       `json:"baud,omitempty"`
	Profile      string    `json:"profile,omitempty"`
	ConnectedAt  time.Time `json:"connected_at,omitempty"`
	UptimeSec    float64   `json:"uptime_sec,omitempty"`
	BufferCount  int       `json:"buffer_count"`
	BufferCap    int       `json:"buffer_cap"`
	TotalLines   int64     `json:"total_lines"`
	TotalBytes   int64     `json:"total_bytes"`
}

// GetStatus returns the current device status.
func (s *Session) GetStatus() DeviceStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stat := DeviceStatus{
		Connected:    s.running && s.source != nil,
		Reconnecting: s.reconnecting,
		Detail:       s.connDetail,
		Port:         s.serialCfg.Port,
		Baud:         s.serialCfg.Baud,
		Profile:      s.profileName,
		BufferCount:  s.buffer.Len(),
		BufferCap:    s.buffer.Cap(),
		TotalLines:   s.lineCount,
		TotalBytes:   s.byteCount,
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
	s.reconnecting = false
	s.connDetail = ""

	go s.readLoop(src, p, s.stopChan)

	return nil
}

// Disconnect cleanly stops the background reader and closes the active serial port.
func (s *Session) Disconnect() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return fmt.Errorf("no serial device is currently connected")
	}

	s.stopSourceLocked()
	s.reconnecting = false
	s.connDetail = ""
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
	errs := src.Errors()

	for {
		select {
		case <-stop:
			return
		case err, ok := <-errs:
			if ok && err != nil {
				s.handleDisconnect(src, err)
				return
			}
		case line, ok := <-lines:
			if !ok {
				s.handleDisconnect(src, nil)
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

func (s *Session) handleDisconnect(faultySrc serial.Source, err error) {
	s.mu.Lock()
	if !s.running || s.source != faultySrc {
		s.mu.Unlock()
		return
	}

	// Close the faulted source
	if s.source != nil {
		s.source.Stop()
		s.source = nil
	}

	detail := "device reset or disconnected"
	if err != nil {
		detail = err.Error()
	}
	s.connDetail = detail
	s.reconnecting = true

	cfg := s.serialCfg
	stop := s.stopChan
	p := s.parser
	s.mu.Unlock()

	go s.reconnectLoop(cfg, p, stop)
}

func (s *Session) reconnectLoop(cfg serial.Config, p parser.Parser, stop <-chan struct{}) {
	ticker := time.NewTicker(750 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			s.mu.RLock()
			if !s.running {
				s.mu.RUnlock()
				return
			}
			s.mu.RUnlock()

			ports, err := serial.ListPorts()
			if err != nil || len(ports) == 0 {
				continue
			}

			candidate := serial.MatchCandidatePort(cfg.Port, ports)
			if candidate == "" {
				continue
			}

			candidateCfg := cfg
			candidateCfg.Port = candidate
			newSrc, err := serial.NewSerialSource(candidateCfg)
			if err != nil {
				continue
			}

			s.mu.Lock()
			if !s.running {
				newSrc.Stop()
				s.mu.Unlock()
				return
			}
			s.source = newSrc
			s.serialCfg.Port = candidate
			s.connectedAt = time.Now()
			s.reconnecting = false
			s.connDetail = ""
			s.mu.Unlock()

			go s.readLoop(newSrc, p, stop)
			return
		}
	}
}
