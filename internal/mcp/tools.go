package mcp

import (
	"encoding/json"
	"fmt"
	"math"
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
			Description: "Retrieve recent log records from the in-memory ring buffer (including pre-existing logs from active oml TUI session), optionally filtered by minimum log level or module.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertyDef{
					"count": {
						Type:        "integer",
						Description: "Number of most recent log lines to retrieve (default: 50, max: 10000).",
						Default:     50,
					},
					"all": {
						Type:        "boolean",
						Description: "Retrieve all available logs from memory history regardless of count limit (default: false).",
						Default:     false,
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
					"until_id": {
						Type:        "integer",
						Description: "Only return logs with ID less than or equal to this upper bound ID.",
					},
					"start_time": {
						Type:        "string",
						Description: "Optional start time boundary (uptime float like '250.0', clock time '14:15:00', or ISO-8601).",
					},
					"end_time": {
						Type:        "string",
						Description: "Optional end time boundary (uptime float like '255.0', clock time '14:15:30', or ISO-8601).",
					},
				},
			},
		},
		{
			Name:        "search_logs",
			Description: "Search the log history buffer using plain text query or regular expression, optionally filtered within a time range.",
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
					"start_time": {
						Type:        "string",
						Description: "Optional start time boundary (uptime float like '250.0', clock time '14:15:00', or ISO-8601).",
					},
					"end_time": {
						Type:        "string",
						Description: "Optional end time boundary (uptime float like '255.0', clock time '14:15:30', or ISO-8601).",
					},
				},
				Required: []string{"query"},
			},
		},
		{
			Name:        "get_logs_in_timerange",
			Description: "Retrieve log records within a specific time range or device uptime window (e.g. from uptime 250.0 to 255.0, or between timestamps 14:15:00 and 14:15:30), optionally filtered by query, min_level, or module.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertyDef{
					"from_time": {
						Type:        "string",
						Description: "Start timestamp, clock time (e.g. '14:15:00', '14:15:00.000'), uptime seconds (e.g. '250.0', '250', '250.'), or ISO-8601 string.",
					},
					"to_time": {
						Type:        "string",
						Description: "End timestamp, clock time, uptime seconds, or ISO-8601 string.",
					},
					"query": {
						Type:        "string",
						Description: "Optional text or regex pattern to filter matching lines within the time range.",
					},
					"is_regex": {
						Type:        "boolean",
						Description: "Treat query as regular expression (default: false).",
						Default:     false,
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
					"max_results": {
						Type:        "integer",
						Description: "Maximum lines to return (default: 100, max: 2000).",
						Default:     100,
					},
				},
				Required: []string{"from_time", "to_time"},
			},
		},
		{
			Name:        "get_logs_around",
			Description: "Inspect the context window of logs surrounding a specific event by record ID, timestamp/uptime, or search query. Perfect for seeing what happened immediately before and after an assert, reboot, state change, or uptime milestone.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertyDef{
					"target_id": {
						Type:        "integer",
						Description: "Log record ID to center around (e.g. 452).",
					},
					"target_time": {
						Type:        "string",
						Description: "Timestamp, clock time, or device uptime to center around (e.g. '250.', '250.125', '14:15:30.123'). Finds the closest matching record.",
					},
					"target_query": {
						Type:        "string",
						Description: "Search query string or regex pattern to locate the pivot record (e.g. 'HardFault' or 'assert failed').",
					},
					"is_regex": {
						Type:        "boolean",
						Description: "Treat target_query as regular expression (default: false).",
						Default:     false,
					},
					"before": {
						Type:        "integer",
						Description: "Number of lines to show before the target event (default: 20, max: 200).",
						Default:     20,
					},
					"after": {
						Type:        "integer",
						Description: "Number of lines to show after the target event (default: 20, max: 200).",
						Default:     20,
					},
					"window": {
						Type:        "integer",
						Description: "Total context window size. If before/after are omitted, splits window evenly around target.",
					},
					"min_level": {
						Type:        "string",
						Description: "Optional minimum severity level filter.",
						Enum:        []string{"DEBUG", "INFO", "WARN", "WARNING", "ERROR", "FATAL"},
					},
					"module": {
						Type:        "string",
						Description: "Optional module or tag name substring filter.",
					},
				},
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
	case "get_logs_in_timerange":
		return h.handleGetLogsInTimerange(rawArgs)
	case "get_logs_around":
		return h.handleGetLogsAround(rawArgs)
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
	} else if status.Reconnecting {
		sb.WriteString(fmt.Sprintf("Status: Auto-Reconnecting (waiting for %s)\n", status.Port))
		if status.Detail != "" {
			sb.WriteString(fmt.Sprintf("Detail: %s\n", status.Detail))
		}
		sb.WriteString(fmt.Sprintf("Profile: %s\n", status.Profile))
		sb.WriteString(fmt.Sprintf("Buffer Records Preserved: %d / %d\n", status.BufferCount, status.BufferCap))
		ports, _ := serial.ListPorts()
		if len(ports) > 0 {
			sb.WriteString(fmt.Sprintf("Currently Detected Ports: %s\n", strings.Join(ports, ", ")))
		}
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
	Count     int    `json:"count"`
	All       bool   `json:"all"`
	MinLevel  string `json:"min_level"`
	Module    string `json:"module"`
	SinceID   uint64 `json:"since_id"`
	UntilID   uint64 `json:"until_id"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
}

func filterByTimeBounds(records []record.Record, startStr, endStr string) ([]record.Record, error) {
	if startStr == "" && endStr == "" {
		return records, nil
	}
	var startPt, endPt ParsedTime
	var err error
	if startStr != "" {
		startPt, err = ParseTimeQuery(startStr)
		if err != nil {
			return nil, fmt.Errorf("invalid start_time %q: %w", startStr, err)
		}
	}
	if endStr != "" {
		endPt, err = ParseTimeQuery(endStr)
		if err != nil {
			return nil, fmt.Errorf("invalid end_time %q: %w", endStr, err)
		}
	}

	// If only start provided, set end to maximum
	if startStr != "" && endStr == "" {
		endPt = startPt
		if startPt.Kind == TimeKindUptimeSeconds {
			endPt.Seconds = math.MaxFloat64
		} else if startPt.Kind == TimeKindDateTime {
			endPt.DateTime = time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC)
		} else if startPt.Kind == TimeKindTimeOfDay {
			endPt.Nanoseconds = math.MaxInt64
		}
	} else if startStr == "" && endStr != "" {
		startPt = endPt
		if endPt.Kind == TimeKindUptimeSeconds {
			startPt.Seconds = 0
		} else if endPt.Kind == TimeKindDateTime {
			startPt.DateTime = time.Time{}
		} else if endPt.Kind == TimeKindTimeOfDay {
			startPt.Nanoseconds = 0
		}
	}

	var res []record.Record
	for _, r := range records {
		if IsRecordInTimeRange(r, startPt, endPt) {
			res = append(res, r)
		}
	}
	return res, nil
}

func (h *ToolHandler) handleGetRecentLogs(rawArgs json.RawMessage) CallToolResult {
	params := getLogsParams{Count: 50}
	if len(rawArgs) > 0 {
		_ = json.Unmarshal(rawArgs, &params)
	}

	fetchCount := params.Count
	if params.All {
		fetchCount = 0 // 0 indicates retrieve full buffer
	} else {
		if fetchCount <= 0 {
			fetchCount = 50
		} else if fetchCount > 10000 {
			fetchCount = 10000
		}
	}

	if h.session.IsIPCAvailable() {
		// When time bounds or ID bounds are specified, retrieve full buffer from IPC to filter locally
		ipcFetchCount := fetchCount
		if params.StartTime != "" || params.EndTime != "" || params.UntilID > 0 {
			ipcFetchCount = 0
		}

		logs, err := h.session.IPCClient().GetRecentLogs(ipcFetchCount, params.MinLevel, params.Module)
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
			if params.UntilID > 0 {
				var filteredUntil []record.Record
				for _, r := range logs {
					if r.ID <= params.UntilID {
						filteredUntil = append(filteredUntil, r)
					}
				}
				logs = filteredUntil
			}
			if params.StartTime != "" || params.EndTime != "" {
				bounded, err := filterByTimeBounds(logs, params.StartTime, params.EndTime)
				if err != nil {
					return textError(err.Error())
				}
				logs = bounded
			}

			if fetchCount > 0 && len(logs) > fetchCount {
				logs = logs[len(logs)-fetchCount:]
			}

			if len(logs) == 0 {
				return textSuccess("No new logs available matching filter in active TUI session.")
			}
			var sb strings.Builder
			totalInMem := len(logs)
			if stat, err := h.session.IPCClient().GetStatus(); err == nil {
				if cnt, ok := stat["buffer_count"].(float64); ok && int(cnt) > 0 {
					totalInMem = int(cnt)
				}
			}
			sb.WriteString(fmt.Sprintf("--- Showing %d log record(s) (total in memory: %d) ---\n", len(logs), totalInMem))
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
			return textSuccess("No logs available (device is currently disconnected and buffer is empty).")
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

	if params.UntilID > 0 {
		var beforeUntil []record.Record
		for _, r := range all {
			if r.ID <= params.UntilID {
				beforeUntil = append(beforeUntil, r)
			}
		}
		all = beforeUntil
	}

	if params.StartTime != "" || params.EndTime != "" {
		bounded, err := filterByTimeBounds(all, params.StartTime, params.EndTime)
		if err != nil {
			return textError(err.Error())
		}
		all = bounded
	}

	filtered := filterRecords(all, params.MinLevel, params.Module)
	if fetchCount > 0 && len(filtered) > fetchCount {
		filtered = filtered[len(filtered)-fetchCount:]
	}

	if len(filtered) == 0 {
		return textSuccess("No logs matched the specified filters.")
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("--- Showing %d log record(s) (total in memory: %d) ---\n", len(filtered), h.session.Buffer().Len()))
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
	StartTime  string `json:"start_time"`
	EndTime    string `json:"end_time"`
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
		// If time bounds are present, retrieve full search results or full buffer from IPC
		matches, err := h.session.IPCClient().SearchLogs(params.Query, params.IsRegex, 0)
		if err == nil {
			if params.StartTime != "" || params.EndTime != "" {
				bounded, err := filterByTimeBounds(matches, params.StartTime, params.EndTime)
				if err != nil {
					return textError(err.Error())
				}
				matches = bounded
			}

			if len(matches) == 0 {
				return textSuccess(fmt.Sprintf("No logs matched search query: %q in active TUI session", params.Query))
			}

			totalMatches := len(matches)
			if len(matches) > params.MaxResults {
				matches = matches[len(matches)-params.MaxResults:]
			}

			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("Found %d match(es) for %q in active TUI session:\n", totalMatches, params.Query))
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

	if params.StartTime != "" || params.EndTime != "" {
		bounded, err := filterByTimeBounds(all, params.StartTime, params.EndTime)
		if err != nil {
			return textError(err.Error())
		}
		all = bounded
	}

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

type getLogsInTimerangeParams struct {
	FromTime   string `json:"from_time"`
	ToTime     string `json:"to_time"`
	Query      string `json:"query"`
	IsRegex    bool   `json:"is_regex"`
	MinLevel   string `json:"min_level"`
	Module     string `json:"module"`
	MaxResults int    `json:"max_results"`
}

func (h *ToolHandler) handleGetLogsInTimerange(rawArgs json.RawMessage) CallToolResult {
	var params getLogsInTimerangeParams
	if len(rawArgs) > 0 {
		if err := json.Unmarshal(rawArgs, &params); err != nil {
			return textError(fmt.Sprintf("invalid arguments: %v", err))
		}
	}

	if params.FromTime == "" || params.ToTime == "" {
		return textError("parameters 'from_time' and 'to_time' are required")
	}

	if params.MaxResults <= 0 {
		params.MaxResults = 100
	} else if params.MaxResults > 2000 {
		params.MaxResults = 2000
	}

	var all []record.Record
	if h.session.IsIPCAvailable() {
		logs, err := h.session.IPCClient().GetRecentLogs(0, "", "")
		if err == nil {
			all = logs
		}
	}
	if len(all) == 0 {
		all = h.session.Buffer().All()
	}

	if len(all) == 0 {
		status := h.session.GetStatus()
		if !status.Connected {
			return textSuccess("No logs available (device is disconnected and buffer is empty).")
		}
		return textSuccess("No logs recorded yet.")
	}

	inRange, err := filterByTimeBounds(all, params.FromTime, params.ToTime)
	if err != nil {
		return textError(err.Error())
	}

	filtered := filterRecords(inRange, params.MinLevel, params.Module)

	if params.Query != "" {
		var reg *regexp.Regexp
		if params.IsRegex {
			var err error
			reg, err = regexp.Compile("(?i)" + params.Query)
			if err != nil {
				return textError(fmt.Sprintf("invalid regular expression: %v", err))
			}
		}
		queryLower := strings.ToLower(params.Query)
		var queryMatches []record.Record
		for _, r := range filtered {
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
				queryMatches = append(queryMatches, r)
			}
		}
		filtered = queryMatches
	}

	if len(filtered) == 0 {
		return textSuccess(fmt.Sprintf("No logs found between %s and %s matching criteria.", params.FromTime, params.ToTime))
	}

	total := len(filtered)
	display := filtered
	if len(display) > params.MaxResults {
		display = display[:params.MaxResults]
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d log record(s) between %s and %s (showing %d):\n", total, params.FromTime, params.ToTime, len(display)))
	for _, r := range display {
		sb.WriteString(formatRecord(r))
		sb.WriteString("\n")
	}

	return textSuccess(sb.String())
}

type getLogsAroundParams struct {
	TargetID    uint64 `json:"target_id"`
	TargetTime  string `json:"target_time"`
	TargetQuery string `json:"target_query"`
	IsRegex     bool   `json:"is_regex"`
	Before      int    `json:"before"`
	After       int    `json:"after"`
	Window      int    `json:"window"`
	MinLevel    string `json:"min_level"`
	Module      string `json:"module"`
}

func (h *ToolHandler) handleGetLogsAround(rawArgs json.RawMessage) CallToolResult {
	var params getLogsAroundParams
	if len(rawArgs) > 0 {
		if err := json.Unmarshal(rawArgs, &params); err != nil {
			return textError(fmt.Sprintf("invalid arguments: %v", err))
		}
	}

	if params.TargetID == 0 && params.TargetTime == "" && params.TargetQuery == "" {
		return textError("at least one of 'target_id', 'target_time', or 'target_query' must be provided")
	}

	before := params.Before
	after := params.After
	if before <= 0 && after <= 0 {
		w := params.Window
		if w <= 0 {
			w = 40
		}
		before = w / 2
		after = w - before
	}
	if before < 0 {
		before = 0
	} else if before > 200 {
		before = 200
	}
	if after < 0 {
		after = 0
	} else if after > 200 {
		after = 200
	}

	var all []record.Record
	if h.session.IsIPCAvailable() {
		logs, err := h.session.IPCClient().GetRecentLogs(0, "", "")
		if err == nil {
			all = logs
		}
	}
	if len(all) == 0 {
		all = h.session.Buffer().All()
	}

	if len(all) == 0 {
		return textSuccess("No logs recorded yet (buffer is empty).")
	}

	targetIdx := -1

	// 1. Locate by TargetID
	if params.TargetID > 0 {
		for i, r := range all {
			if r.ID == params.TargetID {
				targetIdx = i
				break
			}
		}
		if targetIdx == -1 {
			return textError(fmt.Sprintf("record with ID %d not found in buffer", params.TargetID))
		}
	} else if params.TargetTime != "" {
		pt, err := ParseTimeQuery(params.TargetTime)
		if err != nil {
			return textError(fmt.Sprintf("invalid target_time: %v", err))
		}
		bestDist := math.MaxFloat64
		for i, r := range all {
			dist, ok := RecordDistanceToTime(r, pt)
			if ok && dist < bestDist {
				bestDist = dist
				targetIdx = i
			}
		}
		if targetIdx == -1 {
			return textError(fmt.Sprintf("no records with comparable timestamp or uptime found for %q", params.TargetTime))
		}
	} else if params.TargetQuery != "" {
		var reg *regexp.Regexp
		if params.IsRegex {
			var err error
			reg, err = regexp.Compile("(?i)" + params.TargetQuery)
			if err != nil {
				return textError(fmt.Sprintf("invalid regular expression: %v", err))
			}
		}
		queryLower := strings.ToLower(params.TargetQuery)
		// Find the last match in the buffer (most recent)
		for i := len(all) - 1; i >= 0; i-- {
			r := all[i]
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
				targetIdx = i
				break
			}
		}
		if targetIdx == -1 {
			return textSuccess(fmt.Sprintf("No logs matched target_query: %q", params.TargetQuery))
		}
	}

	startIdx := targetIdx - before
	if startIdx < 0 {
		startIdx = 0
	}
	endIdx := targetIdx + after + 1
	if endIdx > len(all) {
		endIdx = len(all)
	}

	window := all[startIdx:endIdx]
	targetRecord := all[targetIdx]

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("--- Showing %d log record(s) centered around target (ID: %d, index %d of %d) ---\n",
		len(window), targetRecord.ID, targetIdx+1, len(all)))

	for _, r := range window {
		if r.ID == targetRecord.ID {
			sb.WriteString(fmt.Sprintf(">>> TARGET (ID:%d) >>> %s\n", r.ID, formatRecord(r)))
		} else {
			sb.WriteString(formatRecord(r))
			sb.WriteString("\n")
		}
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

	// Baseline checkpoint ID: only inspect records arriving after wait_for_log starts
	var lastID uint64 = 0
	if h.session.IsIPCAvailable() {
		recent, _ := h.session.IPCClient().GetRecentLogs(1, "", "")
		if len(recent) > 0 {
			lastID = recent[len(recent)-1].ID
		}
	} else {
		all := h.session.Buffer().All()
		if len(all) > 0 {
			lastID = all[len(all)-1].ID
		}
	}

	// Poll loop
	for time.Since(startTime) < timeout {
		var candidateLogs []record.Record

		if h.session.IsIPCAvailable() {
			candidateLogs, _ = h.session.IPCClient().GetRecentLogs(100, "", "")
		} else {
			candidateLogs = h.session.Buffer().All()
		}

		for _, r := range candidateLogs {
			if r.ID <= lastID {
				continue
			}
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

		if len(candidateLogs) > 0 {
			newestID := candidateLogs[len(candidateLogs)-1].ID
			if newestID > lastID {
				lastID = newestID
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
	content := r.Raw
	if content == "" {
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
			content = strings.Join(parts, " ")
		} else {
			content = fmt.Sprintf("%v", r.Fields)
		}
	}

	var metaParts []string
	if r.ID > 0 {
		metaParts = append(metaParts, fmt.Sprintf("ID:%d", r.ID))
	}
	if !r.Timestamp.IsZero() {
		metaParts = append(metaParts, r.Timestamp.Format("15:04:05.000"))
	}

	if len(metaParts) > 0 {
		return fmt.Sprintf("[%s] %s", strings.Join(metaParts, " | "), content)
	}
	return content
}
