package mcp

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/brenoniehues/oh-my-logs/internal/config"
	"github.com/brenoniehues/oh-my-logs/internal/record"
	"github.com/brenoniehues/oh-my-logs/internal/serial"
)

// Define available tools conforming to MCP Tools specification.
func getAvailableTools() []Tool {
	return []Tool{
		{
			Name:        "list_ports",
			Description: "List all available hardware serial ports detected on the system.",
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]PropertyDef{},
			},
		},
		{
			Name:        "connect_port",
			Description: "Connect or dynamically switch to a specified serial port, baud rate, and parsing profile.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertyDef{
					"port": {
						Type:        "string",
						Description: "The serial port to connect to (e.g. '/dev/ttyACM0', '/dev/ttyUSB0', or 'COM3').",
					},
					"baud": {
						Type:        "integer",
						Description: "Baud rate (default: 115200). Common values: 9600, 115200, 230400, 921600, 1000000.",
						Default:     115200,
					},
					"profile": {
						Type:        "string",
						Description: "Name or path of the log parsing profile (e.g. 'zephyr', 'logcat', 'raw'). Default is 'raw'.",
						Default:     "raw",
					},
				},
				Required: []string{"port"},
			},
		},
		{
			Name:        "disconnect_port",
			Description: "Disconnect from the active serial port and release the device handle (useful before flashing firmware via external tools like west or pyocd).",
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]PropertyDef{},
			},
		},
		{
			Name:        "get_device_status",
			Description: "Get the current serial connection status, active port, baud rate, buffer stats, and throughput.",
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]PropertyDef{},
			},
		},
		{
			Name:        "get_recent_logs",
			Description: "Retrieve recent log records from the in-memory ring buffer, optionally filtered by minimum log level or module.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertyDef{
					"count": {
						Type:        "integer",
						Description: "Number of most recent log lines to retrieve (default: 50, max: 1000).",
						Default:     50,
					},
					"min_level": {
						Type:        "string",
						Description: "Optional minimum severity level filter (e.g. 'DEBUG', 'INFO', 'WARN', 'ERROR').",
						Enum:        []string{"DEBUG", "INFO", "WARN", "WARNING", "ERROR", "FATAL"},
					},
					"module": {
						Type:        "string",
						Description: "Optional module or tag name substring filter (e.g. 'kernel', 'ble', 'wifi').",
					},
					"since_id": {
						Type:        "integer",
						Description: "Only return logs with ID greater than this checkpoint ID (useful for watching new logs after a milestone).",
					},
				},
			},
		},
		{
			Name:        "search_logs",
			Description: "Search the log history buffer using plain text query or regular expression.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertyDef{
					"query": {
						Type:        "string",
						Description: "Search string or regular expression.",
					},
					"is_regex": {
						Type:        "boolean",
						Description: "Treat query as a regular expression (default: false).",
						Default:     false,
					},
					"max_results": {
						Type:        "integer",
						Description: "Maximum matching lines to return (default: 50, max: 500).",
						Default:     50,
					},
				},
				Required: []string{"query"},
			},
		},
		{
			Name:        "send_serial_command",
			Description: "Send a command line string over serial TX to the connected device shell and wait for its immediate response.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertyDef{
					"command": {
						Type:        "string",
						Description: "The command string to transmit (e.g. 'help', 'kernel uptime', 'version').",
					},
					"wait_ms": {
						Type:        "integer",
						Description: "Milliseconds to wait for the device response before returning (default: 500, max: 5000).",
						Default:     500,
					},
					"ending": {
						Type:        "string",
						Description: "Line ending to append: 'crlf', 'lf', 'cr', or 'none' (default: 'crlf').",
						Enum:        []string{"crlf", "lf", "cr", "none"},
						Default:     "crlf",
					},
				},
				Required: []string{"command"},
			},
		},
		{
			Name:        "clear_log_buffer",
			Description: "Clear the in-memory log buffer history for a fresh recording session.",
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]PropertyDef{},
			},
		},
		{
			Name:        "wait_for_log",
			Description: "Wait and monitor incoming serial logs until a specific pattern (substring or regex) appears, or until timeout. Perfect for waiting for boot events, button presses, calibration, or state machine transitions.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertyDef{
					"pattern": {
						Type:        "string",
						Description: "Substring or regex pattern to wait for (e.g. 'Booting', 'State -> CALIBRATING', 'assert failed').",
					},
					"is_regex": {
						Type:        "boolean",
						Description: "Treat pattern as regular expression (default: false).",
						Default:     false,
					},
					"timeout_sec": {
						Type:        "integer",
						Description: "Maximum seconds to wait before timing out (default: 15, max: 120).",
						Default:     15,
					},
				},
				Required: []string{"pattern"},
			},
		},
		{
			Name:        "watch_session",
			Description: "Monitor incoming serial logs continuously for a specified duration and stream or return all newly arrived lines. Essential when waiting for the user to perform physical interactions (button presses, hardware reset, sensor stimulus, cable plugging).",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertyDef{
					"duration_sec": {
						Type:        "integer",
						Description: "Duration in seconds to watch and accumulate incoming logs (default: 10, max: 60).",
						Default:     10,
					},
					"min_level": {
						Type:        "string",
						Description: "Optional minimum severity level filter (DEBUG, INFO, WARN, ERROR, FATAL).",
						Enum:        []string{"DEBUG", "INFO", "WARN", "WARNING", "ERROR", "FATAL"},
					},
					"module": {
						Type:        "string",
						Description: "Optional module or tag name substring filter.",
					},
					"stop_on_pattern": {
						Type:        "string",
						Description: "Optional pattern that stops the watch session immediately if encountered before duration expires.",
					},
				},
			},
		},
	}
}

// ToolHandler handles execution of a single MCP tool.
type ToolHandler struct {
	session *Session
	appCfg  *config.AppConfig
}

// NewToolHandler creates a new handler attached to the given session.
func NewToolHandler(session *Session, appCfg *config.AppConfig) *ToolHandler {
	return &ToolHandler{
		session: session,
		appCfg:  appCfg,
	}
}

// Execute runs the requested tool by name with unparsed raw JSON arguments.
func (h *ToolHandler) Execute(name string, rawArgs json.RawMessage) CallToolResult {
	switch name {
	case "list_ports":
		return h.handleListPorts()
	case "connect_port":
		return h.handleConnectPort(rawArgs)
	case "disconnect_port":
		return h.handleDisconnectPort()
	case "get_device_status":
		return h.handleGetDeviceStatus()
	case "get_recent_logs":
		return h.handleGetRecentLogs(rawArgs)
	case "search_logs":
		return h.handleSearchLogs(rawArgs)
	case "send_serial_command":
		return h.handleSendSerialCommand(rawArgs)
	case "clear_log_buffer":
		return h.handleClearLogBuffer()
	case "wait_for_log":
		return h.handleWaitForLog(rawArgs)
	case "watch_session":
		return h.handleWatchSession(rawArgs)
	default:
		return textError(fmt.Sprintf("unknown tool: %q", name))
	}
}

func (h *ToolHandler) handleListPorts() CallToolResult {
	ports, err := serial.ListPorts()
	if err != nil && len(ports) == 0 {
		return textError(fmt.Sprintf("error scanning serial ports: %v", err))
	}

	if len(ports) == 0 {
		return textSuccess("No serial ports detected on the system.")
	}

	status := h.session.GetStatus()
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d serial port(s):\n", len(ports)))
	for i, p := range ports {
		activeMarker := ""
		if status.Connected && p == status.Port {
			activeMarker = " [CURRENTLY CONNECTED]"
		}
		sb.WriteString(fmt.Sprintf("%d. %s%s\n", i+1, p, activeMarker))
	}

	return textSuccess(sb.String())
}

type connectParams struct {
	Port    string `json:"port"`
	Baud    int    `json:"baud"`
	Profile string `json:"profile"`
}

func (h *ToolHandler) handleConnectPort(rawArgs json.RawMessage) CallToolResult {
	var params connectParams
	if len(rawArgs) > 0 {
		if err := json.Unmarshal(rawArgs, &params); err != nil {
			return textError(fmt.Sprintf("invalid arguments: %v", err))
		}
	}

	if params.Port == "" {
		return textError("parameter 'port' is required")
	}
	if params.Baud <= 0 {
		params.Baud = 115200
	}

	err := h.session.Connect(params.Port, params.Baud, params.Profile)
	if err != nil {
		return textError(fmt.Sprintf("failed to connect to %s: %v", params.Port, err))
	}

	profDisplay := params.Profile
	if profDisplay == "" {
		profDisplay = "raw"
	}

	return textSuccess(fmt.Sprintf("✓ Successfully connected to %s at %d baud using %s profile.", params.Port, params.Baud, profDisplay))
}

func (h *ToolHandler) handleDisconnectPort() CallToolResult {
	status := h.session.GetStatus()
	if !status.Connected {
		return textSuccess("No serial port was connected.")
	}

	port := status.Port
	err := h.session.Disconnect()
	if err != nil {
		return textError(fmt.Sprintf("failed to disconnect: %v", err))
	}

	return textSuccess(fmt.Sprintf("✓ Successfully disconnected from %s. Port released.", port))
}

func (h *ToolHandler) handleGetDeviceStatus() CallToolResult {
	if h.session.IsIPCAvailable() {
		stat, err := h.session.IPCClient().GetStatus()
		if err == nil {
			var sb strings.Builder
			sb.WriteString("Status: Connected (via active oml TUI session)\n")
			if p, ok := stat["port"].(string); ok && p != "" {
				sb.WriteString(fmt.Sprintf("Port: %s\n", p))
			}
			if b, ok := stat["baud"].(float64); ok && b > 0 {
				sb.WriteString(fmt.Sprintf("Baud: %d\n", int(b)))
			}
			if prof, ok := stat["profile"].(string); ok && prof != "" {
				sb.WriteString(fmt.Sprintf("Profile: %s\n", prof))
			}
			if cnt, ok := stat["buffer_count"].(float64); ok {
				capVal, _ := stat["buffer_cap"].(float64)
				sb.WriteString(fmt.Sprintf("Buffer Records: %d / %d\n", int(cnt), int(capVal)))
			}
			return textSuccess(sb.String())
		}
	}

	status := h.session.GetStatus()
	var sb strings.Builder

	if status.Connected {
		sb.WriteString(fmt.Sprintf("Status: Connected\n"))
		sb.WriteString(fmt.Sprintf("Port: %s\n", status.Port))
		sb.WriteString(fmt.Sprintf("Baud: %d\n", status.Baud))
		sb.WriteString(fmt.Sprintf("Profile: %s\n", status.Profile))
		sb.WriteString(fmt.Sprintf("Uptime: %.1fs\n", status.UptimeSec))
		sb.WriteString(fmt.Sprintf("Buffer Records: %d / %d\n", status.BufferCount, status.BufferCap))
		sb.WriteString(fmt.Sprintf("Total Lines Received: %d (%d bytes)\n", status.TotalLines, status.TotalBytes))
	} else {
		sb.WriteString("Status: Disconnected\n")
		ports, _ := serial.ListPorts()
		if len(ports) > 0 {
			sb.WriteString(fmt.Sprintf("Available Ports: %s\n", strings.Join(ports, ", ")))
			sb.WriteString("Tip: Call 'connect_port' to open a connection.\n")
		} else {
			sb.WriteString("Available Ports: None detected\n")
		}
	}

	return textSuccess(sb.String())
}

type getLogsParams struct {
	Count    int    `json:"count"`
	MinLevel string `json:"min_level"`
	Module   string `json:"module"`
	SinceID  uint64 `json:"since_id"`
}

func (h *ToolHandler) handleGetRecentLogs(rawArgs json.RawMessage) CallToolResult {
	params := getLogsParams{Count: 50}
	if len(rawArgs) > 0 {
		_ = json.Unmarshal(rawArgs, &params)
	}

	if params.Count <= 0 {
		params.Count = 50
	} else if params.Count > 1000 {
		params.Count = 1000
	}

	if h.session.IsIPCAvailable() {
		logs, err := h.session.IPCClient().GetRecentLogs(params.Count, params.MinLevel, params.Module)
		if err == nil {
			if params.SinceID > 0 {
				var filteredSince []record.Record
				for _, r := range logs {
					if r.ID > params.SinceID {
						filteredSince = append(filteredSince, r)
					}
				}
				logs = filteredSince
			}
			if len(logs) == 0 {
				return textSuccess("No new logs available matching filter in active TUI session.")
			}
			var sb strings.Builder
			for _, r := range logs {
				sb.WriteString(formatRecord(r))
				sb.WriteString("\n")
			}
			return textSuccess(sb.String())
		}
	}

	all := h.session.Buffer().All()
	if len(all) == 0 {
		status := h.session.GetStatus()
		if !status.Connected {
			return textSuccess("No logs available (device is currently disconnected).")
		}
		return textSuccess("No logs recorded yet.")
	}

	if params.SinceID > 0 {
		var afterSince []record.Record
		for _, r := range all {
			if r.ID > params.SinceID {
				afterSince = append(afterSince, r)
			}
		}
		all = afterSince
	}

	filtered := filterRecords(all, params.MinLevel, params.Module)
	if len(filtered) > params.Count {
		filtered = filtered[len(filtered)-params.Count:]
	}

	if len(filtered) == 0 {
		return textSuccess("No logs matched the specified filters.")
	}

	var sb strings.Builder
	for _, r := range filtered {
		sb.WriteString(formatRecord(r))
		sb.WriteString("\n")
	}

	return textSuccess(sb.String())
}

type searchLogsParams struct {
	Query      string `json:"query"`
	IsRegex    bool   `json:"is_regex"`
	MaxResults int    `json:"max_results"`
}

func (h *ToolHandler) handleSearchLogs(rawArgs json.RawMessage) CallToolResult {
	var params searchLogsParams
	if len(rawArgs) > 0 {
		if err := json.Unmarshal(rawArgs, &params); err != nil {
			return textError(fmt.Sprintf("invalid arguments: %v", err))
		}
	}

	if params.Query == "" {
		return textError("parameter 'query' is required")
	}

	if params.MaxResults <= 0 {
		params.MaxResults = 50
	} else if params.MaxResults > 500 {
		params.MaxResults = 500
	}

	if h.session.IsIPCAvailable() {
		matches, err := h.session.IPCClient().SearchLogs(params.Query, params.IsRegex, params.MaxResults)
		if err == nil {
			if len(matches) == 0 {
				return textSuccess(fmt.Sprintf("No logs matched search query: %q in active TUI session", params.Query))
			}
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("Found %d match(es) for %q in active TUI session:\n", len(matches), params.Query))
			for _, r := range matches {
				sb.WriteString(formatRecord(r))
				sb.WriteString("\n")
			}
			return textSuccess(sb.String())
		}
	}

	var reg *regexp.Regexp
	var err error
	if params.IsRegex {
		reg, err = regexp.Compile("(?i)" + params.Query)
		if err != nil {
			return textError(fmt.Sprintf("invalid regular expression: %v", err))
		}
	}

	queryLower := strings.ToLower(params.Query)
	all := h.session.Buffer().All()

	var matches []record.Record
	for _, r := range all {
		matched := false
		if reg != nil {
			matched = reg.MatchString(r.Raw)
			if !matched {
				for _, v := range r.Fields {
					if reg.MatchString(v) {
						matched = true
						break
					}
				}
			}
		} else {
			matched = strings.Contains(strings.ToLower(r.Raw), queryLower)
			if !matched {
				for _, v := range r.Fields {
					if strings.Contains(strings.ToLower(v), queryLower) {
						matched = true
						break
					}
				}
			}
		}

		if matched {
			matches = append(matches, r)
		}
	}

	if len(matches) == 0 {
		return textSuccess(fmt.Sprintf("No logs matched search query: %q", params.Query))
	}

	totalMatches := len(matches)
	if len(matches) > params.MaxResults {
		matches = matches[len(matches)-params.MaxResults:]
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d match(es) for %q (showing last %d):\n", totalMatches, params.Query, len(matches)))
	for _, r := range matches {
		sb.WriteString(formatRecord(r))
		sb.WriteString("\n")
	}

	return textSuccess(sb.String())
}

type sendCmdParams struct {
	Command string `json:"command"`
	WaitMS  int    `json:"wait_ms"`
	Ending  string `json:"ending"`
}

func (h *ToolHandler) handleSendSerialCommand(rawArgs json.RawMessage) CallToolResult {
	params := sendCmdParams{WaitMS: 500, Ending: "crlf"}
	if len(rawArgs) > 0 {
		if err := json.Unmarshal(rawArgs, &params); err != nil {
			return textError(fmt.Sprintf("invalid arguments: %v", err))
		}
	}

	if params.Command == "" {
		return textError("parameter 'command' is required")
	}

	if params.WaitMS <= 0 {
		params.WaitMS = 500
	} else if params.WaitMS > 5000 {
		params.WaitMS = 5000
	}

	if h.session.IsIPCAvailable() {
		n, resps, err := h.session.IPCClient().SendSerialCommand(params.Command, params.WaitMS, params.Ending)
		if err != nil {
			return textError(fmt.Sprintf("failed transmitting command via active TUI session: %v", err))
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("✓ Transmitted %d bytes via active TUI session: %q\n", n, params.Command))
		if len(resps) > 0 {
			sb.WriteString(fmt.Sprintf("--- Response (%d lines) ---\n", len(resps)))
			for _, r := range resps {
				sb.WriteString(formatRecord(r))
				sb.WriteString("\n")
			}
		} else {
			sb.WriteString("(No new lines received within response window)\n")
		}
		return textSuccess(sb.String())
	}

	var ending serial.LineEnding
	switch strings.ToLower(params.Ending) {
	case "lf":
		ending = serial.EndingLF
	case "cr":
		ending = serial.EndingCR
	case "none":
		ending = serial.EndingNone
	default:
		ending = serial.EndingCRLF
	}

	payload, err := serial.FormatTXPayload(params.Command, ending)
	if err != nil {
		return textError(fmt.Sprintf("failed formatting command: %v", err))
	}

	// Capture record count before transmitting
	countBefore := h.session.Buffer().Len()

	n, err := h.session.Write(payload)
	if err != nil {
		return textError(fmt.Sprintf("failed transmitting command over serial: %v", err))
	}

	// Wait for response lines to arrive in buffer
	time.Sleep(time.Duration(params.WaitMS) * time.Millisecond)

	all := h.session.Buffer().All()
	newLines := []record.Record{}
	if len(all) > countBefore {
		newLines = all[countBefore:]
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("✓ Transmitted %d bytes: %q\n", n, params.Command))
	if len(newLines) > 0 {
		sb.WriteString(fmt.Sprintf("--- Response (%d lines) ---\n", len(newLines)))
		for _, r := range newLines {
			sb.WriteString(formatRecord(r))
			sb.WriteString("\n")
		}
	} else {
		sb.WriteString("(No new lines received within response window)\n")
	}

	return textSuccess(sb.String())
}

func (h *ToolHandler) handleClearLogBuffer() CallToolResult {
	h.session.ClearBuffer()
	return textSuccess("✓ Log buffer cleared.")
}

type waitForLogParams struct {
	Pattern    string `json:"pattern"`
	IsRegex    bool   `json:"is_regex"`
	TimeoutSec int    `json:"timeout_sec"`
}

func (h *ToolHandler) handleWaitForLog(rawArgs json.RawMessage) CallToolResult {
	params := waitForLogParams{TimeoutSec: 15}
	if len(rawArgs) > 0 {
		if err := json.Unmarshal(rawArgs, &params); err != nil {
			return textError(fmt.Sprintf("invalid arguments: %v", err))
		}
	}

	if params.Pattern == "" {
		return textError("parameter 'pattern' is required")
	}

	if params.TimeoutSec <= 0 {
		params.TimeoutSec = 15
	} else if params.TimeoutSec > 120 {
		params.TimeoutSec = 120
	}

	var reg *regexp.Regexp
	var err error
	if params.IsRegex {
		reg, err = regexp.Compile("(?i)" + params.Pattern)
		if err != nil {
			return textError(fmt.Sprintf("invalid regular expression: %v", err))
		}
	}
	patternLower := strings.ToLower(params.Pattern)

	startTime := time.Now()
	timeout := time.Duration(params.TimeoutSec) * time.Second

	// Record buffer index before polling
	startCount := 0
	if h.session.IsIPCAvailable() {
		status, _ := h.session.IPCClient().GetStatus()
		if cnt, ok := status["buffer_count"].(float64); ok {
			startCount = int(cnt)
		}
	} else {
		startCount = h.session.Buffer().Len()
	}

	// Poll loop
	for time.Since(startTime) < timeout {
		var candidateLogs []record.Record

		if h.session.IsIPCAvailable() {
			logs, _ := h.session.IPCClient().GetRecentLogs(100, "", "")
			candidateLogs = logs
		} else {
			candidateLogs = h.session.Buffer().All()
		}

		// Check new lines since startCount
		if len(candidateLogs) > startCount {
			for _, r := range candidateLogs[startCount:] {
				matched := false
				if reg != nil {
					matched = reg.MatchString(r.Raw)
				} else {
					matched = strings.Contains(strings.ToLower(r.Raw), patternLower)
				}

				if matched {
					elapsed := time.Since(startTime).Seconds()
					return textSuccess(fmt.Sprintf("✓ Pattern matched in %.2fs: %s", elapsed, formatRecord(r)))
				}
			}
		}

		time.Sleep(100 * time.Millisecond)
	}

	return textError(fmt.Sprintf("timed out after %ds waiting for pattern: %q", params.TimeoutSec, params.Pattern))
}

type watchSessionParams struct {
	DurationSec   int    `json:"duration_sec"`
	MinLevel      string `json:"min_level"`
	Module        string `json:"module"`
	StopOnPattern string `json:"stop_on_pattern"`
}

func (h *ToolHandler) handleWatchSession(rawArgs json.RawMessage) CallToolResult {
	params := watchSessionParams{DurationSec: 10}
	if len(rawArgs) > 0 {
		if err := json.Unmarshal(rawArgs, &params); err != nil {
			return textError(fmt.Sprintf("invalid arguments: %v", err))
		}
	}

	if params.DurationSec <= 0 {
		params.DurationSec = 10
	} else if params.DurationSec > 60 {
		params.DurationSec = 60
	}

	startTime := time.Now()
	deadline := startTime.Add(time.Duration(params.DurationSec) * time.Second)

	// Baseline checkpoint ID
	var lastID uint64 = 0
	if h.session.IsIPCAvailable() {
		logs, err := h.session.IPCClient().GetRecentLogs(1, "", "")
		if err == nil && len(logs) > 0 {
			lastID = logs[len(logs)-1].ID
		}
	} else {
		all := h.session.Buffer().All()
		if len(all) > 0 {
			lastID = all[len(all)-1].ID
		}
	}

	stopPatternLower := strings.ToLower(params.StopOnPattern)
	var captured []record.Record
	stoppedEarly := false
	var triggerLine string

	for time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)

		var newBatch []record.Record
		if h.session.IsIPCAvailable() {
			logs, err := h.session.IPCClient().GetRecentLogs(100, "", "")
			if err == nil {
				for _, r := range logs {
					if r.ID > lastID {
						newBatch = append(newBatch, r)
						lastID = r.ID
					}
				}
			}
		} else {
			all := h.session.Buffer().All()
			for _, r := range all {
				if r.ID > lastID {
					newBatch = append(newBatch, r)
					lastID = r.ID
				}
			}
		}

		for _, r := range newBatch {
			captured = append(captured, r)
			if stopPatternLower != "" && strings.Contains(strings.ToLower(r.Raw), stopPatternLower) {
				stoppedEarly = true
				triggerLine = r.Raw
				break
			}
		}

		if stoppedEarly {
			break
		}
	}

	elapsed := time.Since(startTime).Seconds()
	filtered := filterRecords(captured, params.MinLevel, params.Module)

	var sb strings.Builder
	if stoppedEarly {
		sb.WriteString(fmt.Sprintf("Watch session terminated early at %.2fs (trigger pattern matched: %q):\n", elapsed, triggerLine))
	} else {
		sb.WriteString(fmt.Sprintf("Watch session completed (%.1fs duration):\n", elapsed))
	}

	if len(filtered) == 0 {
		sb.WriteString("No new logs received during this observation window.")
	} else {
		sb.WriteString(fmt.Sprintf("Captured %d new log event(s):\n", len(filtered)))
		for _, r := range filtered {
			sb.WriteString(formatRecord(r))
			sb.WriteString("\n")
		}
	}

	return textSuccess(sb.String())
}

func textSuccess(text string) CallToolResult {
	return CallToolResult{
		Content: []ContentItem{
			{Type: "text", Text: text},
		},
		IsError: false,
	}
}

func textError(msg string) CallToolResult {
	return CallToolResult{
		Content: []ContentItem{
			{Type: "text", Text: msg},
		},
		IsError: true,
	}
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

func formatRecord(r record.Record) string {
	if r.Raw != "" {
		return r.Raw
	}
	// Fallback to formatted fields
	parts := make([]string, 0, len(r.Fields))
	if ts := r.Get("time"); ts != "" {
		parts = append(parts, ts)
	}
	if lvl := r.Get("level"); lvl != "" {
		parts = append(parts, "["+lvl+"]")
	}
	if mod := r.Get("module"); mod != "" {
		parts = append(parts, "["+mod+"]")
	}
	if msg := r.Get("message"); msg != "" {
		parts = append(parts, msg)
	}
	if len(parts) > 0 {
		return strings.Join(parts, " ")
	}
	return fmt.Sprintf("%v", r.Fields)
}
