package tui

import (
	"strings"
	"time"

	"github.com/brenoniehues/oh-my-logs/internal/ipc"
	"github.com/brenoniehues/oh-my-logs/internal/record"
	"github.com/brenoniehues/oh-my-logs/internal/serial"
)

// IPCHandler implements ipc.Handler for an active TUI Model.
type IPCHandler struct {
	m *Model
}

var _ ipc.Handler = (*IPCHandler)(nil)

// NewIPCHandler wraps the TUI model for IPC service.
func NewIPCHandler(m *Model) *IPCHandler {
	return &IPCHandler{m: m}
}

// GetStatus returns metadata regarding active port and buffer stats.
func (h *IPCHandler) GetStatus() map[string]interface{} {
	connected := h.m.connState == ConnConnected
	port := h.m.serialCfg.Port
	baud := h.m.serialCfg.Baud
	profileName := "raw"
	if h.m.profile != nil && h.m.profile.Name != "" {
		profileName = h.m.profile.Name
	}

	bufLen := 0
	bufCap := 0
	if h.m.buffer != nil {
		bufLen = h.m.buffer.Len()
		bufCap = h.m.buffer.Cap()
	}

	return map[string]interface{}{
		"connected":    connected,
		"port":         port,
		"baud":         baud,
		"profile":      profileName,
		"buffer_count": bufLen,
		"buffer_cap":   bufCap,
		"source_type":  "tui",
	}
}

// GetRecentLogs returns recent records from the TUI's buffer, matching filters.
func (h *IPCHandler) GetRecentLogs(count int, minLevel, module string) []record.Record {
	if h.m.buffer == nil {
		return nil
	}

	all := h.m.buffer.All()
	if len(all) == 0 {
		return nil
	}

	filtered := filterRecords(all, minLevel, module)
	if count > 0 && len(filtered) > count {
		filtered = filtered[len(filtered)-count:]
	}
	return filtered
}

// SearchLogs searches the TUI's buffer for matching records.
func (h *IPCHandler) SearchLogs(query string, isRegex bool, maxResults int) []record.Record {
	if h.m.buffer == nil || query == "" {
		return nil
	}

	all := h.m.buffer.All()
	queryLower := strings.ToLower(query)

	var matches []record.Record
	for _, r := range all {
		matched := strings.Contains(strings.ToLower(r.Raw), queryLower)
		if !matched {
			for _, v := range r.Fields {
				if strings.Contains(strings.ToLower(v), queryLower) {
					matched = true
					break
				}
			}
		}
		if matched {
			matches = append(matches, r)
		}
	}

	if maxResults > 0 && len(matches) > maxResults {
		matches = matches[len(matches)-maxResults:]
	}
	return matches
}

// SendSerialCommand writes bytes over the TUI's active serial TX channel and captures response lines.
func (h *IPCHandler) SendSerialCommand(command string, waitMS int, ending string) (int, []record.Record, error) {
	if h.m.source == nil {
		return 0, nil, nil
	}

	var lineEnding serial.LineEnding
	switch strings.ToLower(ending) {
	case "lf":
		lineEnding = serial.EndingLF
	case "cr":
		lineEnding = serial.EndingCR
	case "none":
		lineEnding = serial.EndingNone
	default:
		lineEnding = serial.EndingCRLF
	}

	payload, err := serial.FormatTXPayload(command, lineEnding)
	if err != nil {
		return 0, nil, err
	}

	countBefore := 0
	if h.m.buffer != nil {
		countBefore = h.m.buffer.Len()
	}

	n, err := h.m.source.Write(payload)
	if err != nil {
		return 0, nil, err
	}

	if waitMS <= 0 {
		waitMS = 500
	}
	time.Sleep(time.Duration(waitMS) * time.Millisecond)

	var newLines []record.Record
	if h.m.buffer != nil {
		all := h.m.buffer.All()
		if len(all) > countBefore {
			newLines = all[countBefore:]
		}
	}

	return n, newLines, nil
}

// HandoverPort is called when another primary instance requests the hardware port.
// Since TUI has priority, TUI keeps ownership.
func (h *IPCHandler) HandoverPort() error {
	return nil
}

func filterRecords(records []record.Record, minLevel, module string) []record.Record {
	minLevel = strings.ToUpper(strings.TrimSpace(minLevel))
	module = strings.ToLower(strings.TrimSpace(module))

	levelOrder := map[string]int{
		"DEBUG":   10,
		"INFO":    20,
		"WARN":    30,
		"WARNING": 30,
		"ERROR":   40,
		"FATAL":   50,
	}

	targetSeverity := 0
	if minLevel != "" {
		targetSeverity = levelOrder[minLevel]
	}

	var res []record.Record
	for _, r := range records {
		if targetSeverity > 0 {
			recLevel := strings.ToUpper(r.Get("level"))
			if sev, ok := levelOrder[recLevel]; ok {
				if sev < targetSeverity {
					continue
				}
			}
		}

		if module != "" {
			tag := strings.ToLower(r.Get("module"))
			if tag == "" {
				tag = strings.ToLower(r.Get("tag"))
			}
			if !strings.Contains(tag, module) && !strings.Contains(strings.ToLower(r.Raw), module) {
				continue
			}
		}

		res = append(res, r)
	}

	return res
}
