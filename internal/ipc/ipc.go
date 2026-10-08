package ipc

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
)

// SocketPath returns the default filesystem path or address for the local IPC socket.
// On Unix/macOS: ~/.config/oh-my-logs/oml.sock (or inside configDir).
// On Windows: \\.\pipe\oml-ipc
func SocketPath(configDir string) string {
	if runtime.GOOS == "windows" {
		return `\\.\pipe\oml-ipc`
	}
	if configDir == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, ".config", "oh-my-logs", "oml.sock")
		}
		return "/tmp/oml.sock"
	}
	return filepath.Join(configDir, "oml.sock")
}

// Request defines an IPC command sent from client to server.
type Request struct {
	Action string          `json:"action"` // e.g. "status", "logs", "search", "send_cmd", "handover", "ping"
	Data   json.RawMessage `json:"data,omitempty"`
}

// Response defines an IPC result returned from server to client.
type Response struct {
	Success bool            `json:"success"`
	Error   string          `json:"error,omitempty"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// Listen creates a local network listener for IPC on the appropriate platform.
func Listen(addr string) (net.Listener, error) {
	if runtime.GOOS == "windows" {
		// On Windows fall back to localhost loopback port or named pipe
		// Using 127.0.0.1:49152 (dynamic private port) or Unix socket where supported
		return net.Listen("tcp", "127.0.0.1:48123")
	}

	// Remove stale unix socket if it exists and nobody is listening
	if fi, err := os.Stat(addr); err == nil && !fi.IsDir() {
		// Test if active
		conn, dialErr := net.Dial("unix", addr)
		if dialErr == nil {
			_ = conn.Close()
			return nil, fmt.Errorf("socket %s is already in use by an active oml instance", addr)
		}
		// Stale file: remove it
		_ = os.Remove(addr)
	}

	// Ensure parent directory exists
	dir := filepath.Dir(addr)
	_ = os.MkdirAll(dir, 0o755)

	return net.Listen("unix", addr)
}

// Dial connects to the local IPC server.
func Dial(addr string) (net.Conn, error) {
	if runtime.GOOS == "windows" {
		return net.Dial("tcp", "127.0.0.1:48123")
	}
	return net.Dial("unix", addr)
}
