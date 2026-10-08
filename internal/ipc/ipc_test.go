package ipc

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/brenoniehues/oh-my-logs/internal/record"
)

type mockHandler struct {
	records []record.Record
}

func (m *mockHandler) GetStatus() map[string]interface{} {
	return map[string]interface{}{
		"connected": true,
		"port":      "/dev/mock0",
		"baud":      115200,
	}
}

func (m *mockHandler) GetRecentLogs(count int, minLevel, module string) []record.Record {
	return m.records
}

func (m *mockHandler) SearchLogs(query string, isRegex bool, maxResults int) []record.Record {
	var out []record.Record
	for _, r := range m.records {
		if strings.Contains(r.Raw, query) {
			out = append(out, r)
		}
	}
	return out
}

func (m *mockHandler) SendSerialCommand(command string, waitMS int, ending string) (int, []record.Record, error) {
	return len(command), []record.Record{{Raw: "response to " + command}}, nil
}

func (m *mockHandler) HandoverPort() error {
	return nil
}

func TestIPCServerAndClient(t *testing.T) {
	tmpDir := t.TempDir()
	sockAddr := filepath.Join(tmpDir, "test.sock")

	handler := &mockHandler{
		records: []record.Record{
			{Raw: "[INFO] boot ok", Timestamp: time.Now()},
			{Raw: "[WARN] low battery", Timestamp: time.Now()},
		},
	}

	srv, err := NewServer(sockAddr, handler)
	if err != nil {
		t.Fatalf("failed starting IPC server: %v", err)
	}
	defer srv.Close()

	client := NewClient(sockAddr)
	if !client.IsAvailable() {
		t.Fatalf("client.IsAvailable returned false")
	}

	if err := client.Ping(); err != nil {
		t.Fatalf("ping failed: %v", err)
	}

	stat, err := client.GetStatus()
	if err != nil {
		t.Fatalf("getStatus failed: %v", err)
	}
	if stat["port"] != "/dev/mock0" {
		t.Errorf("expected port /dev/mock0, got: %v", stat["port"])
	}

	logs, err := client.GetRecentLogs(10, "", "")
	if err != nil {
		t.Fatalf("getRecentLogs failed: %v", err)
	}
	if len(logs) != 2 {
		t.Fatalf("expected 2 logs, got %d", len(logs))
	}

	matches, err := client.SearchLogs("battery", false, 10)
	if err != nil {
		t.Fatalf("searchLogs failed: %v", err)
	}
	if len(matches) != 1 || !strings.Contains(matches[0].Raw, "low battery") {
		t.Errorf("search failed to find battery record: %v", matches)
	}

	n, resps, err := client.SendSerialCommand("uptime", 50, "crlf")
	if err != nil {
		t.Fatalf("sendSerialCommand failed: %v", err)
	}
	if n != 6 || len(resps) != 1 {
		t.Errorf("unexpected command result: n=%d, resps=%v", n, resps)
	}

	if err := client.RequestHandover(); err != nil {
		t.Fatalf("requestHandover failed: %v", err)
	}
}
