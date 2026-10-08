package mcp

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/brenoniehues/oh-my-logs/internal/config"
	"github.com/brenoniehues/oh-my-logs/internal/record"
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

