package forgemcp

import "encoding/json"

// Standard JSON-RPC and MCP protocol constants
const (
	JSONRPCVersion     = "2.0"
	MCPProtocolVersion = "2024-11-05"
	ServerName         = "forge-mcp"
	ServerVersion      = "1.0.6"
)

// Standard JSON-RPC 2.0 Error Codes
const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternalError  = -32603
)

// JSONRPCMessage represents a standard JSON-RPC 2.0 request, notification, or response.
type JSONRPCMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  interface{}     `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

// JSONRPCError represents a standard JSON-RPC 2.0 error.
type JSONRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ServerInfo describes this MCP server.
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ServerCapabilities defines features supported by the server.
type ServerCapabilities struct {
	Tools     map[string]interface{} `json:"tools,omitempty"`
	Resources map[string]interface{} `json:"resources,omitempty"`
	Prompts   map[string]interface{} `json:"prompts,omitempty"`
}

// InitializeResult is sent back in response to "initialize".
type InitializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	ServerInfo      ServerInfo         `json:"serverInfo"`
}

// Tool describes an MCP tool available for external agents to call.
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// ToolCallResult represents the outcome of calling an MCP tool.
type ToolCallResult struct {
	Content []ContentItem `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

// ContentItem represents a text or media chunk in a tool result.
type ContentItem struct {
	Type     string `json:"type"` // "text", "image", "resource"
	Text     string `json:"text,omitempty"`
	Data     string `json:"data,omitempty"` // base64 payload for images
	MimeType string `json:"mimeType,omitempty"`
}
