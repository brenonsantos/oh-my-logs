package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/brenoniehues/oh-my-logs/internal/config"
)

// Server coordinates JSON-RPC 2.0 message handling over stdio.
type Server struct {
	session     *Session
	toolHandler *ToolHandler
	appCfg      *config.AppConfig
	version     string
}

// NewServer initializes an MCP Server with the provided session and application configuration.
func NewServer(session *Session, appCfg *config.AppConfig, version string) *Server {
	return &Server{
		session:     session,
		toolHandler: NewToolHandler(session, appCfg),
		appCfg:      appCfg,
		version:     version,
	}
}

// Run reads JSON-RPC requests line by line from r and writes responses to w.
func (s *Server) Run(r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(r)
	// Allow larger JSON-RPC frames (up to 4MB) for large log buffers
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 4*1024*1024)

	encoder := json.NewEncoder(w)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req JSONRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			resp := JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      nil,
				Error: &JSONRPCError{
					Code:    CodeParseError,
					Message: fmt.Sprintf("failed to parse JSON-RPC request: %v", err),
				},
			}
			if err := encoder.Encode(resp); err != nil {
				return err
			}
			continue
		}

		// Notifications have no ID and do not return a response
		isNotification := len(req.ID) == 0 || string(req.ID) == "null"

		resp := s.handleRequest(&req)
		if !isNotification && resp != nil {
			if err := encoder.Encode(resp); err != nil {
				return err
			}
		}
	}

	return scanner.Err()
}

// ServeStdio is the standard entrypoint that runs the server over os.Stdin and os.Stdout.
func (s *Server) ServeStdio() error {
	return s.Run(os.Stdin, os.Stdout)
}

func (s *Server) handleRequest(req *JSONRPCRequest) *JSONRPCResponse {
	resp := &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
	}

	switch req.Method {
	case "initialize":
		resp.Result = InitializeResult{
			ProtocolVersion: "2024-11-05",
			Capabilities: ServerCapabilities{
				Tools: &ToolsCapability{ListChanged: false},
			},
			ServerInfo: Implementation{
				Name:    "oh-my-logs",
				Version: s.version,
			},
			Instructions: "oh-my-logs MCP server provides access to live hardware serial logs, device shells, and port management.",
		}
		return resp

	case "notifications/initialized", "initialized":
		// Standard lifecycle notification: no response required
		return nil

	case "ping":
		resp.Result = map[string]interface{}{}
		return resp

	case "tools/list":
		resp.Result = ToolsListResult{
			Tools: getAvailableTools(),
		}
		return resp

	case "tools/call":
		var params CallToolParams
		if len(req.Params) > 0 {
			if err := json.Unmarshal(req.Params, &params); err != nil {
				resp.Error = &JSONRPCError{
					Code:    CodeInvalidParams,
					Message: fmt.Sprintf("invalid tools/call params: %v", err),
				}
				return resp
			}
		}

		result := s.toolHandler.Execute(params.Name, params.Arguments)
		resp.Result = result
		return resp

	default:
		resp.Error = &JSONRPCError{
			Code:    CodeMethodNotFound,
			Message: fmt.Sprintf("method not found: %s", req.Method),
		}
		return resp
	}
}
