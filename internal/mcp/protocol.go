package mcp

import (
	"encoding/json"
)

// JSON-RPC 2.0 protocol structures conforming to the Model Context Protocol (MCP) specification.

// JSONRPCRequest represents an incoming JSON-RPC 2.0 request or notification.
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"` // null or omitted for notifications
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSONRPCResponse represents an outgoing JSON-RPC 2.0 response.
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  interface{}     `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

// JSONRPCError represents a JSON-RPC 2.0 error object.
type JSONRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Standard JSON-RPC 2.0 and MCP error codes.
const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternalError  = -32603
)

// InitializeResult represents the response to the "initialize" handshake.
type InitializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	ServerInfo      Implementation     `json:"serverInfo"`
	Instructions    string             `json:"instructions,omitempty"`
}

// ServerCapabilities declares what features this MCP server provides.
type ServerCapabilities struct {
	Tools *ToolsCapability `json:"tools,omitempty"`
}

// ToolsCapability indicates support for tool calls.
type ToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// Implementation contains name and version of the server.
type Implementation struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Tool describes a callable tool available on the server.
type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema InputSchema `json:"inputSchema"`
}

// InputSchema defines the JSON Schema for tool arguments.
type InputSchema struct {
	Type       string                 `json:"type"`
	Properties map[string]PropertyDef `json:"properties,omitempty"`
	Required   []string               `json:"required,omitempty"`
}

// PropertyDef describes a single property in an InputSchema.
type PropertyDef struct {
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Enum        []string `json:"enum,omitempty"`
	Default     any      `json:"default,omitempty"`
}

// ToolsListResult is returned for "tools/list".
type ToolsListResult struct {
	Tools []Tool `json:"tools"`
}

// CallToolParams is the parameter payload for "tools/call".
type CallToolParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

// CallToolResult is returned for "tools/call".
type CallToolResult struct {
	Content []ContentItem `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

// ContentItem represents a block of content returned by a tool.
type ContentItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}
