package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/brenoniehues/oh-my-logs/internal/config"
	"github.com/brenoniehues/oh-my-logs/internal/ipc"
	"github.com/brenoniehues/oh-my-logs/internal/parser"
	"github.com/brenoniehues/oh-my-logs/internal/record"
	"github.com/brenoniehues/oh-my-logs/internal/serial"
)

func TestMCPServer_HandshakeAndToolsList(t *testing.T) {
	appCfg := &config.AppConfig{}
	session := NewSession(appCfg, 100)
	server := NewServer(session, appCfg, "1.4.1")

	// 1. Send "initialize" request
	initReq := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n"
	// 2. Send "notifications/initialized" notification
	initializedNotif := `{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n"
	// 3. Send "ping" request
	pingReq := `{"jsonrpc":"2.0","id":2,"method":"ping"}` + "\n"
	// 4. Send "tools/list" request
	toolsListReq := `{"jsonrpc":"2.0","id":3,"method":"tools/list"}` + "\n"

	input := initReq + initializedNotif + pingReq + toolsListReq
	inBuf := strings.NewReader(input)
	var outBuf bytes.Buffer

	err := server.Run(inBuf, &outBuf)
	if err != nil {
		t.Fatalf("server.Run failed: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(outBuf.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 responses (init, ping, tools/list), got %d:\n%s", len(lines), outBuf.String())
	}

	// Verify initialize response
	var initResp JSONRPCResponse
	if err := json.Unmarshal([]byte(lines[0]), &initResp); err != nil {
		t.Fatalf("failed decoding initialize response: %v", err)
	}
	if initResp.Error != nil {
		t.Fatalf("initialize returned error: %v", initResp.Error)
	}

	// Verify ping response
	var pingResp JSONRPCResponse
	if err := json.Unmarshal([]byte(lines[1]), &pingResp); err != nil {
		t.Fatalf("failed decoding ping response: %v", err)
	}
	if pingResp.Error != nil {
		t.Fatalf("ping returned error: %v", pingResp.Error)
	}

	// Verify tools/list response
	var toolsResp JSONRPCResponse
	if err := json.Unmarshal([]byte(lines[2]), &toolsResp); err != nil {
		t.Fatalf("failed decoding tools/list response: %v", err)
	}
	if toolsResp.Error != nil {
		t.Fatalf("tools/list returned error: %v", toolsResp.Error)
	}

	toolsMap, ok := toolsResp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("tools/list result not a map: %T", toolsResp.Result)
	}
	toolsArr, ok := toolsMap["tools"].([]interface{})
	if !ok || len(toolsArr) == 0 {
		t.Fatalf("expected non-empty tools array, got: %v", toolsMap)
	}

	// Ensure essential tools are present
	expectedTools := map[string]bool{
		"list_ports":          false,
		"connect_port":        false,
		"disconnect_port":     false,
		"get_device_status":   false,
		"get_recent_logs":     false,
		"search_logs":         false,
		"send_serial_command": false,
		"clear_log_buffer":    false,
		"wait_for_log":        false,
		"watch_session":       false,
	}

	for _, toolItem := range toolsArr {
		tm, ok := toolItem.(map[string]interface{})
		if ok {
			name, _ := tm["name"].(string)
			if _, exists := expectedTools[name]; exists {
				expectedTools[name] = true
			}
		}
	}

	for toolName, found := range expectedTools {
		if !found {
			t.Errorf("expected tool %q not found in tools/list", toolName)
		}
	}
}

func TestMCPServer_ToolCalls(t *testing.T) {
	appCfg := &config.AppConfig{}
	session := NewSession(appCfg, 100)

	// Pre-populate buffer with test records
	session.Buffer().Add(record.Record{
		Fields:    map[string]string{"level": "INFO", "module": "kernel", "message": "Booting system..."},
		Raw:       "[INFO] [kernel] Booting system...",
		Timestamp: time.Now(),
	})
	session.Buffer().Add(record.Record{
		Fields:    map[string]string{"level": "WARN", "module": "wifi", "message": "Low RSSI: -82 dBm"},
		Raw:       "[WARN] [wifi] Low RSSI: -82 dBm",
		Timestamp: time.Now(),
	})
	session.Buffer().Add(record.Record{
		Fields:    map[string]string{"level": "ERROR", "module": "sensor", "message": "I2C ACK failure on 0x68"},
		Raw:       "[ERROR] [sensor] I2C ACK failure on 0x68",
		Timestamp: time.Now(),
	})

	server := NewServer(session, appCfg, "1.4.1")

	// Call get_device_status
	statusReq := `{"jsonrpc":"2.0","id":10,"method":"tools/call","params":{"name":"get_device_status","arguments":{}}}` + "\n"

	// Call get_recent_logs with min_level=ERROR
	logsReq := `{"jsonrpc":"2.0","id":11,"method":"tools/call","params":{"name":"get_recent_logs","arguments":{"min_level":"ERROR"}}}` + "\n"

	// Call search_logs with query="ACK"
	searchReq := `{"jsonrpc":"2.0","id":12,"method":"tools/call","params":{"name":"search_logs","arguments":{"query":"ACK"}}}` + "\n"

	// Call clear_log_buffer
	clearReq := `{"jsonrpc":"2.0","id":13,"method":"tools/call","params":{"name":"clear_log_buffer","arguments":{}}}` + "\n"

	input := statusReq + logsReq + searchReq + clearReq
	inBuf := strings.NewReader(input)
	var outBuf bytes.Buffer

	err := server.Run(inBuf, &outBuf)
	if err != nil {
		t.Fatalf("server.Run failed: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(outBuf.String()), "\n")
	if len(lines) != 4 {
		t.Fatalf("expected 4 tool call responses, got %d:\n%s", len(lines), outBuf.String())
	}

	// 1. Verify get_device_status
	var statusResp JSONRPCResponse
	_ = json.Unmarshal([]byte(lines[0]), &statusResp)
	if statusResp.Error != nil {
		t.Fatalf("get_device_status error: %v", statusResp.Error)
	}

	// 2. Verify get_recent_logs
	var logsResp JSONRPCResponse
	_ = json.Unmarshal([]byte(lines[1]), &logsResp)
	resultMap, _ := logsResp.Result.(map[string]interface{})
	contentArr, _ := resultMap["content"].([]interface{})
	if len(contentArr) == 0 {
		t.Fatalf("expected content in get_recent_logs result")
	}
	textItem, _ := contentArr[0].(map[string]interface{})
	logText, _ := textItem["text"].(string)
	if !strings.Contains(logText, "I2C ACK failure") {
		t.Errorf("expected filtered logs to contain ERROR line, got: %s", logText)
	}
	if strings.Contains(logText, "Booting system") {
		t.Errorf("expected filtered logs to exclude INFO line, got: %s", logText)
	}

	// 3. Verify search_logs
	var searchResp JSONRPCResponse
	_ = json.Unmarshal([]byte(lines[2]), &searchResp)
	searchMap, _ := searchResp.Result.(map[string]interface{})
	searchContent, _ := searchMap["content"].([]interface{})
	searchTextItem, _ := searchContent[0].(map[string]interface{})
	searchText, _ := searchTextItem["text"].(string)
	if !strings.Contains(searchText, "Found 1 match") || !strings.Contains(searchText, "I2C ACK failure") {
		t.Errorf("expected search result to find ACK match, got: %s", searchText)
	}

	// 4. Verify clear_log_buffer
	if session.Buffer().Len() != 0 {
		t.Errorf("expected buffer to be empty after clear_log_buffer, got %d", session.Buffer().Len())
	}
}

func TestMCPServer_UnknownMethodAndTool(t *testing.T) {
	appCfg := &config.AppConfig{}
	session := NewSession(appCfg, 100)
	server := NewServer(session, appCfg, "1.4.1")

	// Unknown method
	req1 := `{"jsonrpc":"2.0","id":99,"method":"unknown_rpc_method"}` + "\n"
	// Unknown tool
	req2 := `{"jsonrpc":"2.0","id":100,"method":"tools/call","params":{"name":"nonexistent_tool"}}` + "\n"

	input := req1 + req2
	var outBuf bytes.Buffer

	err := server.Run(strings.NewReader(input), &outBuf)
	if err != nil {
		t.Fatalf("server.Run failed: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(outBuf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 error responses, got %d", len(lines))
	}

	var resp1 JSONRPCResponse
	_ = json.Unmarshal([]byte(lines[0]), &resp1)
	if resp1.Error == nil || resp1.Error.Code != CodeMethodNotFound {
		t.Errorf("expected method not found error, got: %v", resp1.Error)
	}

	var resp2 JSONRPCResponse
	_ = json.Unmarshal([]byte(lines[1]), &resp2)
	toolRes, _ := resp2.Result.(map[string]interface{})
	if isErr, ok := toolRes["isError"].(bool); !ok || !isErr {
		t.Errorf("expected tool result to have isError=true for unknown tool")
	}
}

func TestMCPServer_WaitForLogAndSinceID(t *testing.T) {
	appCfg := &config.AppConfig{}
	session := NewSession(appCfg, 100)

	session.Buffer().Add(record.Record{
		Fields:    map[string]string{"message": "line 1"},
		Raw:       "line 1",
		Timestamp: time.Now(),
	})

	server := NewServer(session, appCfg, "1.4.1")

	// 1. Test since_id filtering
	req1 := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_recent_logs","arguments":{"since_id":1}}}` + "\n"

	// 2. Add line 2 asynchronously to test wait_for_log matching
	go func() {
		time.Sleep(100 * time.Millisecond)
		session.Buffer().Add(record.Record{
			Fields:    map[string]string{"message": "State -> CALIBRATING"},
			Raw:       "[sm] State -> CALIBRATING",
			Timestamp: time.Now(),
		})
	}()

	// 3. Test wait_for_log
	req2 := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"wait_for_log","arguments":{"pattern":"CALIBRATING","timeout_sec":3}}}` + "\n"

	input := req1 + req2
	var outBuf bytes.Buffer

	err := server.Run(strings.NewReader(input), &outBuf)
	if err != nil {
		t.Fatalf("server.Run failed: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(outBuf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 responses, got %d:\n%s", len(lines), outBuf.String())
	}

	// Verify wait_for_log matched successfully
	var waitResp JSONRPCResponse
	_ = json.Unmarshal([]byte(lines[1]), &waitResp)
	if waitResp.Error != nil {
		t.Fatalf("wait_for_log returned error: %v", waitResp.Error)
	}
	waitMap, _ := waitResp.Result.(map[string]interface{})
	contentArr, _ := waitMap["content"].([]interface{})
	textItem, _ := contentArr[0].(map[string]interface{})
	waitText, _ := textItem["text"].(string)
	if !strings.Contains(waitText, "Pattern matched") || !strings.Contains(waitText, "CALIBRATING") {
		t.Errorf("expected wait_for_log to match pattern, got: %s", waitText)
	}
}

func TestMCPServer_WatchSession(t *testing.T) {
	appCfg := &config.AppConfig{}
	session := NewSession(appCfg, 100)

	session.Buffer().Add(record.Record{
		Fields:    map[string]string{"message": "booting line"},
		Raw:       "booting line",
		Timestamp: time.Now(),
	})

	server := NewServer(session, appCfg, "1.4.1")

	// Inject logs during watch session
	go func() {
		time.Sleep(100 * time.Millisecond)
		session.Buffer().Add(record.Record{
			Fields:    map[string]string{"message": "button pressed: BTN1"},
			Raw:       "button pressed: BTN1",
			Timestamp: time.Now(),
		})
	}()

	req := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"watch_session","arguments":{"duration_sec":2,"stop_on_pattern":"BTN1"}}}` + "\n"

	var outBuf bytes.Buffer
	err := server.Run(strings.NewReader(req), &outBuf)
	if err != nil {
		t.Fatalf("server.Run failed: %v", err)
	}

	var resp JSONRPCResponse
	if err := json.Unmarshal(outBuf.Bytes(), &resp); err != nil {
		t.Fatalf("failed decoding watch_session response: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("watch_session returned error: %v", resp.Error)
	}

	resMap, _ := resp.Result.(map[string]interface{})
	contentArr, _ := resMap["content"].([]interface{})
	textItem, _ := contentArr[0].(map[string]interface{})
	watchText, _ := textItem["text"].(string)

	if !strings.Contains(watchText, "Watch session terminated early") || !strings.Contains(watchText, "BTN1") {
		t.Errorf("expected watch_session to trigger early on BTN1, got: %s", watchText)
	}
}

type mockSerialSource struct {
	lines  chan string
	errors chan error
	stop   chan struct{}
}

func newMockSerialSource() *mockSerialSource {
	return &mockSerialSource{
		lines:  make(chan string, 10),
		errors: make(chan error, 10),
		stop:   make(chan struct{}),
	}
}

func (m *mockSerialSource) Lines() <-chan string { return m.lines }
func (m *mockSerialSource) Errors() <-chan error { return m.errors }
func (m *mockSerialSource) Stop()                 { close(m.stop) }
func (m *mockSerialSource) Write(p []byte) (int, error) {
	return len(p), nil
}

func TestMCPSession_AutoReconnectOnDisconnect(t *testing.T) {
	appCfg := &config.AppConfig{}
	session := NewSession(appCfg, 100)

	mockSrc := newMockSerialSource()
	session.source = mockSrc
	session.serialCfg = serial.Config{Port: "/dev/ttyTestPort", Baud: 115200}
	session.running = true
	session.stopChan = make(chan struct{})

	go session.readLoop(mockSrc, parser.NewRawParser(), session.stopChan)

	// Verify initial status is connected
	status := session.GetStatus()
	if !status.Connected {
		t.Fatalf("expected session to be connected initially")
	}

	// Emit an error simulating hardware reset (e.g. device unplugged / bus reset)
	mockSrc.errors <- fmt.Errorf("read: device not configured")

	// Wait for session to transition to reconnecting
	deadline := time.Now().Add(2 * time.Second)
	transitioned := false
	for time.Now().Before(deadline) {
		st := session.GetStatus()
		if !st.Connected && st.Reconnecting {
			transitioned = true
			if !strings.Contains(st.Detail, "device not configured") {
				t.Errorf("expected detail to contain error, got: %s", st.Detail)
			}
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if !transitioned {
		t.Fatalf("session failed to transition to reconnecting state upon hardware error")
	}

	// Verify get_device_status reflects auto-reconnecting
	handler := NewToolHandler(session, appCfg)
	res := handler.handleGetDeviceStatus()
	content := res.Content[0].Text
	if !strings.Contains(content, "Auto-Reconnecting") {
		t.Errorf("expected handleGetDeviceStatus to show Auto-Reconnecting, got: %s", content)
	}

	// Explicit Disconnect must cleanly terminate reconnect loop
	if err := session.Disconnect(); err != nil {
		t.Fatalf("Disconnect failed: %v", err)
	}

	finalStatus := session.GetStatus()
	if finalStatus.Connected || finalStatus.Reconnecting {
		t.Errorf("expected session to be fully disconnected, got connected=%v, reconnecting=%v",
			finalStatus.Connected, finalStatus.Reconnecting)
	}
}

type mockTUIIPCHandler struct {
	records []record.Record
}

func (m *mockTUIIPCHandler) GetStatus() map[string]interface{} {
	return map[string]interface{}{
		"connected":    true,
		"port":         "/dev/ttyACM0",
		"baud":         115200,
		"profile":      "Zephyr",
		"buffer_count": len(m.records),
		"buffer_cap":   100000,
		"source_type":  "tui",
	}
}

func (m *mockTUIIPCHandler) GetRecentLogs(count int, minLevel, module string) []record.Record {
	if count <= 0 || count >= len(m.records) {
		return m.records
	}
	return m.records[len(m.records)-count:]
}

func (m *mockTUIIPCHandler) SearchLogs(query string, isRegex bool, maxResults int) []record.Record {
	var matches []record.Record
	for _, r := range m.records {
		if strings.Contains(r.Raw, query) {
			matches = append(matches, r)
		}
	}
	if maxResults > 0 && len(matches) > maxResults {
		return matches[len(matches)-maxResults:]
	}
	return matches
}

func (m *mockTUIIPCHandler) SendSerialCommand(command string, waitMS int, ending string) (int, []record.Record, error) {
	return len(command), []record.Record{{Raw: "ok: " + command}}, nil
}

func (m *mockTUIIPCHandler) HandoverPort() error {
	return nil
}

func TestMCPSession_SyncPreExistingLogsFromActiveTUI(t *testing.T) {
	tmpConfigDir := filepath.Join(os.TempDir(), fmt.Sprintf("oml_cfg_%d", time.Now().UnixNano()))
	_ = os.MkdirAll(tmpConfigDir, 0o755)
	defer os.RemoveAll(tmpConfigDir)

	sockAddr := ipc.SocketPath(tmpConfigDir)
	defer os.Remove(sockAddr)

	// Simulate 1,500 logs existent in memory from 15 minutes of prior usage
	totalRecords := 1500
	records := make([]record.Record, totalRecords)
	for i := 0; i < totalRecords; i++ {
		records[i] = record.Record{
			ID:        uint64(i + 1),
			Raw:       fmt.Sprintf("[12:00:%02d.%03d] <inf> sys: sensor_%d packet=%d status=OK", i/60, i%1000, i%8, i),
			Timestamp: time.Now(),
			Fields: map[string]string{
				"level":   "INFO",
				"module":  "sys",
				"message": fmt.Sprintf("sensor_%d packet=%d status=OK", i%8, i),
			},
		}
	}

	mockTUI := &mockTUIIPCHandler{records: records}
	srv, err := ipc.NewServer(sockAddr, mockTUI)
	if err != nil {
		t.Fatalf("failed starting mock TUI IPC server: %v", err)
	}
	defer srv.Close()

	appCfg := &config.AppConfig{ConfigDir: tmpConfigDir}

	// Start MCP session (as Gemini would when opening 15 minutes later)
	session := NewSession(appCfg, 50_000)

	// 1. Verify session immediately hydrated its in-memory buffer with all 1,500 pre-existing logs
	if session.Buffer().Len() != totalRecords {
		t.Fatalf("expected session buffer to have %d pre-existing records, got %d", totalRecords, session.Buffer().Len())
	}

	handler := NewToolHandler(session, appCfg)

	// 2. Default get_recent_logs returns recent batch with total memory metadata
	resDefault := handler.handleGetRecentLogs(nil)
	if resDefault.IsError {
		t.Fatalf("handleGetRecentLogs failed: %s", resDefault.Content[0].Text)
	}
	textDefault := resDefault.Content[0].Text
	if !strings.Contains(textDefault, "total in memory: 1500") {
		t.Errorf("expected memory metadata header in logs output, got: %s", textDefault)
	}
	if !strings.Contains(textDefault, "Showing 50 log record(s)") {
		t.Errorf("expected default 50 records, got: %s", textDefault)
	}

	// 3. get_recent_logs with all: true retrieves all 1,500 logs from memory history
	resAll := handler.handleGetRecentLogs([]byte(`{"all": true}`))
	if resAll.IsError {
		t.Fatalf("handleGetRecentLogs with all:true failed: %s", resAll.Content[0].Text)
	}
	textAll := resAll.Content[0].Text
	if !strings.Contains(textAll, "Showing 1500 log record(s)") {
		t.Errorf("expected all 1500 records, got: %s", textAll)
	}

	// 4. get_recent_logs with count: 500 retrieves 500 logs
	res500 := handler.handleGetRecentLogs([]byte(`{"count": 500}`))
	if res500.IsError {
		t.Fatalf("handleGetRecentLogs with count:500 failed: %s", res500.Content[0].Text)
	}
	text500 := res500.Content[0].Text
	if !strings.Contains(text500, "Showing 500 log record(s)") {
		t.Errorf("expected 500 records, got: %s", text500)
	}

	// 5. search_logs searches across the entire history
	resSearch := handler.handleSearchLogs([]byte(`{"query": "sensor_3", "max_results": 10}`))
	if resSearch.IsError {
		t.Fatalf("handleSearchLogs failed: %s", resSearch.Content[0].Text)
	}
	if !strings.Contains(resSearch.Content[0].Text, "sensor_3") {
		t.Errorf("expected search to find sensor_3 in historical memory, got: %s", resSearch.Content[0].Text)
	}
}

func TestMCPSession_SeedFromRecentDiskLog(t *testing.T) {
	tmpLogsDir := t.TempDir()
	logFile := filepath.Join(tmpLogsDir, "oml-2026-10-15T12-00-00.log")

	var sb strings.Builder
	for i := 0; i < 40; i++ {
		sb.WriteString(fmt.Sprintf("[12:00:%02d] <inf> boot: subsystem %d initialized\n", i, i))
	}
	if err := os.WriteFile(logFile, []byte(sb.String()), 0o644); err != nil {
		t.Fatalf("failed writing test log: %v", err)
	}

	appCfg := &config.AppConfig{
		ConfigDir: t.TempDir(),
		LogsDir:   tmpLogsDir,
	}

	// Create MCP session with no IPC server running
	session := NewSession(appCfg, 100)

	// Verify buffer was seeded from the recent disk log
	if session.Buffer().Len() != 40 {
		t.Fatalf("expected buffer to have 40 seeded records, got %d", session.Buffer().Len())
	}

	handler := NewToolHandler(session, appCfg)
	res := handler.handleGetRecentLogs(nil)
	if res.IsError {
		t.Fatalf("handleGetRecentLogs failed: %s", res.Content[0].Text)
	}
	if !strings.Contains(res.Content[0].Text, "Showing 40 log record(s)") {
		t.Errorf("expected 40 records in output, got: %s", res.Content[0].Text)
	}
}

func TestFormatRecord_WithIDAndTimestamp(t *testing.T) {
	ts := time.Date(2026, 10, 29, 14, 15, 30, 123000000, time.UTC)
	rec := record.Record{
		ID:        105,
		Timestamp: ts,
		Raw:       "[  250.125] <inf> sys: ready",
	}

	formatted := formatRecord(rec)
	expectedPrefix := "[ID:105 | 14:15:30.123]"
	if !strings.HasPrefix(formatted, expectedPrefix) {
		t.Errorf("expected prefix %q, got: %q", expectedPrefix, formatted)
	}
	if !strings.Contains(formatted, "[  250.125] <inf> sys: ready") {
		t.Errorf("expected original content preserved, got: %q", formatted)
	}

	// Record with ID only (zero timestamp)
	recIDOnly := record.Record{
		ID:  42,
		Raw: "plain message",
	}
	if got := formatRecord(recIDOnly); got != "[ID:42] plain message" {
		t.Errorf("expected [ID:42] plain message, got %q", got)
	}

	// Record with no ID and no timestamp
	recNone := record.Record{Raw: "raw message"}
	if got := formatRecord(recNone); got != "raw message" {
		t.Errorf("expected raw message, got %q", got)
	}
}

func TestToolHandler_GetLogsInTimerange(t *testing.T) {
	appCfg := &config.AppConfig{}
	session := NewSession(appCfg, 200)

	baseTime := time.Date(2026, 10, 29, 10, 0, 0, 0, time.UTC)
	for i := 1; i <= 100; i++ {
		uptimeSec := float64(200 + i) // 201.0 .. 300.0
		session.Buffer().Add(record.Record{
			ID:        uint64(i),
			Timestamp: baseTime.Add(time.Duration(i) * time.Second),
			Fields: map[string]string{
				"uptime":  fmt.Sprintf("%.3f", uptimeSec),
				"level":   "INFO",
				"message": fmt.Sprintf("event %d at uptime %.1f", i, uptimeSec),
			},
			Raw: fmt.Sprintf("[%8.3f] <inf> sys: event %d at uptime %.1f", uptimeSec, i, uptimeSec),
		})
	}

	handler := NewToolHandler(session, appCfg)

	// 1. Query uptime range 240.0 to 260.0
	res := handler.handleGetLogsInTimerange([]byte(`{"from_time": "240.0", "to_time": "260.0"}`))
	if res.IsError {
		t.Fatalf("handleGetLogsInTimerange failed: %s", res.Content[0].Text)
	}
	txt := res.Content[0].Text
	if !strings.Contains(txt, "Found 21 log record(s)") {
		t.Errorf("expected 21 records between 240.0 and 260.0 (inclusive), got: %s", txt)
	}
	if !strings.Contains(txt, "240.000") || !strings.Contains(txt, "260.000") {
		t.Errorf("expected range boundaries in output, got: %s", txt)
	}

	// 2. Query with query filter inside time range
	resFiltered := handler.handleGetLogsInTimerange([]byte(`{
		"from_time": "240.0",
		"to_time": "260.0",
		"query": "event 45"
	}`))
	if resFiltered.IsError {
		t.Fatalf("handleGetLogsInTimerange with query failed: %s", resFiltered.Content[0].Text)
	}
	if !strings.Contains(resFiltered.Content[0].Text, "event 45") {
		t.Errorf("expected event 45 in result, got: %s", resFiltered.Content[0].Text)
	}

	// 3. Error on missing parameters
	resErr := handler.handleGetLogsInTimerange([]byte(`{"from_time": "240.0"}`))
	if !resErr.IsError {
		t.Errorf("expected error when to_time is missing")
	}
}

func TestToolHandler_GetLogsAround(t *testing.T) {
	appCfg := &config.AppConfig{}
	session := NewSession(appCfg, 200)

	baseTime := time.Date(2026, 10, 29, 10, 0, 0, 0, time.UTC)
	for i := 1; i <= 100; i++ {
		uptimeSec := float64(200 + i)
		msg := fmt.Sprintf("normal activity %d", i)
		if i == 50 {
			msg = "CRITICAL HARDFAULT DETECTED"
		}
		session.Buffer().Add(record.Record{
			ID:        uint64(i),
			Timestamp: baseTime.Add(time.Duration(i) * time.Second),
			Fields: map[string]string{
				"uptime":  fmt.Sprintf("%.3f", uptimeSec),
				"message": msg,
			},
			Raw: fmt.Sprintf("[%8.3f] %s", uptimeSec, msg),
		})
	}

	handler := NewToolHandler(session, appCfg)

	// 1. Center around target_id: 50 with before: 5, after: 5
	resID := handler.handleGetLogsAround([]byte(`{"target_id": 50, "before": 5, "after": 5}`))
	if resID.IsError {
		t.Fatalf("handleGetLogsAround by ID failed: %s", resID.Content[0].Text)
	}
	txtID := resID.Content[0].Text
	if !strings.Contains(txtID, "Showing 11 log record(s) centered around target") {
		t.Errorf("expected 11 records (5 before + 1 target + 5 after), got: %s", txtID)
	}
	if !strings.Contains(txtID, ">>> TARGET (ID:50)") {
		t.Errorf("expected target line marker >>> TARGET (ID:50) >>>, got: %s", txtID)
	}
	if !strings.Contains(txtID, "CRITICAL HARDFAULT DETECTED") {
		t.Errorf("expected target line text, got: %s", txtID)
	}

	// 2. Center around target_time: "250." (or "250.0")
	resTime := handler.handleGetLogsAround([]byte(`{"target_time": "250.", "before": 3, "after": 3}`))
	if resTime.IsError {
		t.Fatalf("handleGetLogsAround by time failed: %s", resTime.Content[0].Text)
	}
	txtTime := resTime.Content[0].Text
	// Record 50 corresponds to uptime 250.000
	if !strings.Contains(txtTime, "TARGET (ID:50)") {
		t.Errorf("expected closest record to 250. to be ID 50, got: %s", txtTime)
	}

	// 3. Center around target_query: "HARDFAULT"
	resQuery := handler.handleGetLogsAround([]byte(`{"target_query": "HARDFAULT", "window": 10}`))
	if resQuery.IsError {
		t.Fatalf("handleGetLogsAround by query failed: %s", resQuery.Content[0].Text)
	}
	txtQuery := resQuery.Content[0].Text
	if !strings.Contains(txtQuery, "TARGET (ID:50)") {
		t.Errorf("expected query match on ID 50, got: %s", txtQuery)
	}
}

func TestToolHandler_SearchAndRecentLogsWithTimeBounds(t *testing.T) {
	appCfg := &config.AppConfig{}
	session := NewSession(appCfg, 200)

	baseTime := time.Date(2026, 10, 29, 10, 0, 0, 0, time.UTC)
	for i := 1; i <= 50; i++ {
		uptimeSec := float64(100 + i)
		session.Buffer().Add(record.Record{
			ID:        uint64(i),
			Timestamp: baseTime.Add(time.Duration(i) * time.Second),
			Fields: map[string]string{
				"uptime":  fmt.Sprintf("%.3f", uptimeSec),
				"message": fmt.Sprintf("status beacon packet %d", i),
			},
			Raw: fmt.Sprintf("[%8.3f] status beacon packet %d", uptimeSec, i),
		})
	}

	handler := NewToolHandler(session, appCfg)

	// search_logs with start_time & end_time
	resSearch := handler.handleSearchLogs([]byte(`{
		"query": "beacon",
		"start_time": "120.0",
		"end_time": "125.0"
	}`))
	if resSearch.IsError {
		t.Fatalf("search_logs with time bounds failed: %s", resSearch.Content[0].Text)
	}
	txtSearch := resSearch.Content[0].Text
	if !strings.Contains(txtSearch, "Found 6 match(es)") { // 120.0 to 125.0 inclusive = 6 items (indices 20..25)
		t.Errorf("expected 6 matches between 120.0 and 125.0, got: %s", txtSearch)
	}

	// get_recent_logs with until_id
	resRecent := handler.handleGetRecentLogs([]byte(`{
		"since_id": 10,
		"until_id": 20
	}`))
	if resRecent.IsError {
		t.Fatalf("get_recent_logs with until_id failed: %s", resRecent.Content[0].Text)
	}
	txtRecent := resRecent.Content[0].Text
	if !strings.Contains(txtRecent, "Showing 10 log record(s)") {
		t.Errorf("expected 10 records between ID 10 and 20, got: %s", txtRecent)
	}
}

func TestMCPSession_IPCProxy_RangeAndAround(t *testing.T) {
	tmpConfigDir := filepath.Join(os.TempDir(), fmt.Sprintf("oml_ipc_range_%d", time.Now().UnixNano()))
	_ = os.MkdirAll(tmpConfigDir, 0o755)
	defer os.RemoveAll(tmpConfigDir)

	sockAddr := ipc.SocketPath(tmpConfigDir)
	defer os.Remove(sockAddr)

	records := make([]record.Record, 100)
	baseTime := time.Date(2026, 10, 29, 10, 0, 0, 0, time.UTC)
	for i := 0; i < 100; i++ {
		uptimeSec := float64(100 + i)
		records[i] = record.Record{
			ID:        uint64(i + 1),
			Timestamp: baseTime.Add(time.Duration(i) * time.Second),
			Raw:       fmt.Sprintf("[%8.3f] <inf> sys: device step %d", uptimeSec, i+1),
			Fields: map[string]string{
				"uptime":  fmt.Sprintf("%.3f", uptimeSec),
				"message": fmt.Sprintf("device step %d", i+1),
			},
		}
	}

	mockTUI := &mockTUIIPCHandler{records: records}
	srv, err := ipc.NewServer(sockAddr, mockTUI)
	if err != nil {
		t.Fatalf("failed starting mock TUI IPC server: %v", err)
	}
	defer srv.Close()

	appCfg := &config.AppConfig{ConfigDir: tmpConfigDir}
	session := NewSession(appCfg, 50)
	handler := NewToolHandler(session, appCfg)

	// 1. get_logs_in_timerange proxied through IPC
	resRange := handler.handleGetLogsInTimerange([]byte(`{"from_time": "120.0", "to_time": "130.0"}`))
	if resRange.IsError {
		t.Fatalf("handleGetLogsInTimerange over IPC failed: %s", resRange.Content[0].Text)
	}
	if !strings.Contains(resRange.Content[0].Text, "Found 11 log record(s)") {
		t.Errorf("expected 11 records in range over IPC, got: %s", resRange.Content[0].Text)
	}

	// 2. get_logs_around proxied through IPC
	resAround := handler.handleGetLogsAround([]byte(`{"target_id": 25, "before": 2, "after": 2}`))
	if resAround.IsError {
		t.Fatalf("handleGetLogsAround over IPC failed: %s", resAround.Content[0].Text)
	}
	if !strings.Contains(resAround.Content[0].Text, "TARGET (ID:25)") {
		t.Errorf("expected target ID:25 in output over IPC, got: %s", resAround.Content[0].Text)
	}
}

func TestToolHandler_SearchLogsRegex(t *testing.T) {
	appCfg := &config.AppConfig{}
	session := NewSession(appCfg, 100)

	baseTime := time.Date(2026, 10, 29, 10, 0, 0, 0, time.UTC)
	logs := []string{
		"[  100.100] <inf> sys: normal operation started",
		"[  100.250] <wrn> bus: I2C ACK retry 1 on addr 0x68",
		"[  100.500] <err> bus: I2C ACK retry 2 on addr 0x68 failed",
		"[  101.000] <err> kernel: HardFault exception detected at 0x08004120",
		"[  101.200] <inf> sys: rebooting subsystem",
	}

	for i, raw := range logs {
		uptimeSec := 100.0 + float64(i)*0.25
		session.Buffer().Add(record.Record{
			ID:        uint64(i + 1),
			Timestamp: baseTime.Add(time.Duration(i) * time.Second),
			Raw:       raw,
			Fields: map[string]string{
				"uptime":  fmt.Sprintf("%.3f", uptimeSec),
				"message": raw,
			},
		})
	}

	handler := NewToolHandler(session, appCfg)

	// 1. Regular expression search for disjunction: (HardFault|failed)
	resDisjunction := handler.Execute("search_logs_regex", []byte(`{"pattern": "(HardFault|failed)"}`))
	if resDisjunction.IsError {
		t.Fatalf("search_logs_regex failed: %s", resDisjunction.Content[0].Text)
	}
	txt := resDisjunction.Content[0].Text
	if !strings.Contains(txt, "Found 2 match(es)") {
		t.Errorf("expected 2 matches for (HardFault|failed), got: %s", txt)
	}
	if !strings.Contains(txt, "HardFault exception") || !strings.Contains(txt, "retry 2 on addr 0x68 failed") {
		t.Errorf("expected both matching lines in output, got: %s", txt)
	}

	// 2. Case-insensitivity verification (pattern "hardfault" matches "HardFault")
	resCase := handler.Execute("search_logs_regex", []byte(`{"pattern": "hardfault"}`))
	if resCase.IsError {
		t.Fatalf("search_logs_regex case-insensitive failed: %s", resCase.Content[0].Text)
	}
	if !strings.Contains(resCase.Content[0].Text, "HardFault exception") {
		t.Errorf("expected case-insensitive match on HardFault, got: %s", resCase.Content[0].Text)
	}

	// 3. Regex matching with time bounds
	resBounded := handler.Execute("search_logs_regex", []byte(`{
		"pattern": "addr 0x[0-9a-fA-F]+",
		"start_time": "100.0",
		"end_time": "100.3"
	}`))
	if resBounded.IsError {
		t.Fatalf("search_logs_regex with time bounds failed: %s", resBounded.Content[0].Text)
	}
	// Only line 2 (100.250) falls in 100.0..100.3
	if !strings.Contains(resBounded.Content[0].Text, "Found 1 match(es)") {
		t.Errorf("expected 1 match within time window, got: %s", resBounded.Content[0].Text)
	}

	// 4. Invalid regex pattern returns descriptive error
	resInvalid := handler.Execute("search_logs_regex", []byte(`{"pattern": "([unclosed"}`))
	if !resInvalid.IsError {
		t.Errorf("expected error on invalid regex syntax, got success: %s", resInvalid.Content[0].Text)
	}
	if !strings.Contains(resInvalid.Content[0].Text, "invalid regular expression") {
		t.Errorf("expected error message to cite invalid regular expression, got: %s", resInvalid.Content[0].Text)
	}

	// 5. Missing pattern parameter error
	resMissing := handler.Execute("search_logs_regex", []byte(`{}`))
	if !resMissing.IsError || !strings.Contains(resMissing.Content[0].Text, "'pattern' is required") {
		t.Errorf("expected error for missing pattern, got: %s", resMissing.Content[0].Text)
	}
}

func TestTools_DescriptionsHaveHintsAndExamples(t *testing.T) {
	tools := getAvailableTools()
	toolMap := make(map[string]Tool)
	for _, tool := range tools {
		toolMap[tool.Name] = tool
	}

	// Verify search_logs_regex is present
	if _, ok := toolMap["search_logs_regex"]; !ok {
		t.Fatalf("expected search_logs_regex tool to be registered")
	}

	// Verify get_recent_logs mentions ID / timestamp hint
	recentDesc := toolMap["get_recent_logs"].Description
	if !strings.Contains(recentDesc, "ID") || !strings.Contains(recentDesc, "get_logs_around") {
		t.Errorf("expected get_recent_logs to hint at get_logs_around, got: %s", recentDesc)
	}

	// Verify get_logs_around describes target_id and target_time with uptime examples
	aroundDesc := toolMap["get_logs_around"].Description
	if !strings.Contains(aroundDesc, "ID") || !strings.Contains(aroundDesc, "uptime") {
		t.Errorf("expected get_logs_around description to mention ID and uptime, got: %s", aroundDesc)
	}

	targetTimeProp := toolMap["get_logs_around"].InputSchema.Properties["target_time"]
	if !strings.Contains(targetTimeProp.Description, "250.") && !strings.Contains(targetTimeProp.Description, "250.0") {
		t.Errorf("expected target_time property to give concrete uptime example like 250., got: %s", targetTimeProp.Description)
	}

	// Verify search_logs_regex has regex pattern examples
	regexProp := toolMap["search_logs_regex"].InputSchema.Properties["pattern"]
	if !strings.Contains(regexProp.Description, "(") || !strings.Contains(regexProp.Description, "|") {
		t.Errorf("expected search_logs_regex pattern to include example regex like (assert|fault), got: %s", regexProp.Description)
	}
}




