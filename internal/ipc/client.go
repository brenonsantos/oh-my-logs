package ipc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/brenoniehues/oh-my-logs/internal/record"
)

// Client communicates with a running oml IPC server.
type Client struct {
	addr string
	mu   sync.Mutex
}

// NewClient creates a client targeting the given socket address.
func NewClient(addr string) *Client {
	return &Client{addr: addr}
}

// IsAvailable checks if an active IPC server is listening.
func (c *Client) IsAvailable() bool {
	conn, err := Dial(c.addr)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func (c *Client) call(action string, paramData interface{}, resultDest interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	conn, err := Dial(c.addr)
	if err != nil {
		return fmt.Errorf("cannot connect to oml instance: %w", err)
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))

	var rawParams json.RawMessage
	if paramData != nil {
		rawParams, _ = json.Marshal(paramData)
	}

	req := Request{Action: action, Data: rawParams}
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return fmt.Errorf("failed sending IPC request: %w", err)
	}

	var resp Response
	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		return fmt.Errorf("empty IPC response from oml server")
	}

	if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
		return fmt.Errorf("failed decoding IPC response: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("IPC error: %s", resp.Error)
	}

	if resultDest != nil && len(resp.Data) > 0 {
		if err := json.Unmarshal(resp.Data, resultDest); err != nil {
			return fmt.Errorf("failed unmarshaling IPC data: %w", err)
		}
	}

	return nil
}

// Ping checks if the server is responding.
func (c *Client) Ping() error {
	var res string
	return c.call("ping", nil, &res)
}

// GetStatus returns the remote instance's status.
func (c *Client) GetStatus() (map[string]interface{}, error) {
	var res map[string]interface{}
	err := c.call("status", nil, &res)
	return res, err
}

// GetRecentLogs retrieves recent records from the remote instance.
func (c *Client) GetRecentLogs(count int, minLevel, module string) ([]record.Record, error) {
	p := map[string]interface{}{
		"count":     count,
		"min_level": minLevel,
		"module":    module,
	}
	var res []record.Record
	err := c.call("logs", p, &res)
	return res, err
}

// SearchLogs executes a search on the remote instance.
func (c *Client) SearchLogs(query string, isRegex bool, maxResults int) ([]record.Record, error) {
	p := map[string]interface{}{
		"query":       query,
		"is_regex":    isRegex,
		"max_results": maxResults,
	}
	var res []record.Record
	err := c.call("search", p, &res)
	return res, err
}

// SendSerialCommand sends a command through the remote instance's serial port.
func (c *Client) SendSerialCommand(command string, waitMS int, ending string) (int, []record.Record, error) {
	p := map[string]interface{}{
		"command": command,
		"wait_ms": waitMS,
		"ending":  ending,
	}
	var res struct {
		Bytes     int             `json:"bytes"`
		Responses []record.Record `json:"responses"`
	}
	err := c.call("send_cmd", p, &res)
	return res.Bytes, res.Responses, err
}

// RequestHandover asks the remote instance to release its serial port.
func (c *Client) RequestHandover() error {
	var res string
	return c.call("handover", nil, &res)
}
