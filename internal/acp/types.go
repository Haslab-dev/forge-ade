package acp

import "encoding/json"

// JSON-RPC 2.0 primitives
type RPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"` // string, number, or nil for notifications
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type RPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

type RPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// ACP Protocol constants
const (
	ProtocolVersion = 2

	MethodInitialize          = "initialize"
	MethodSessionNew          = "session/new"
	MethodSessionPrompt       = "session/prompt"
	MethodSessionCancel       = "session/cancel"
	MethodSessionUpdate       = "session/update"
	MethodSessionRequestPerm  = "session/request_permission"
	MethodElicitationCreate   = "elicitation/create"
	MethodElicitationComplete = "elicitation/complete"
)

// Initialize
type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type ClientCapabilities struct {
	Streaming   bool `json:"streaming"`
	Permissions bool `json:"permissions"`
	Tools       bool `json:"tools"`
}

type InitializeParams struct {
	ProtocolVersion int                `json:"protocolVersion"`
	ClientInfo      ClientInfo         `json:"clientInfo"`
	Capabilities    ClientCapabilities `json:"capabilities"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type ServerCapabilities struct {
	Streaming   bool     `json:"streaming"`
	Models      []string `json:"models,omitempty"`
	Permissions bool     `json:"permissions"`
}

type InitializeResult struct {
	ProtocolVersion int                `json:"protocolVersion"`
	ServerInfo      ServerInfo         `json:"serverInfo"`
	Capabilities    ServerCapabilities `json:"capabilities"`
}

// Session Creation
type SessionNewParams struct {
	SessionID        string   `json:"sessionId"`
	WorkingDirectory string   `json:"workingDirectory"`
	Models           []string `json:"models,omitempty"`
}

type SessionNewResult struct {
	SessionID string   `json:"sessionId"`
	Models    []string `json:"models,omitempty"`
}

// Prompt Content
type PromptContentBlock struct {
	Type     string `json:"type"` // "text", "image", "resource"
	Text     string `json:"text,omitempty"`
	Data     string `json:"data,omitempty"` // base64
	MimeType string `json:"mimeType,omitempty"`
	URI      string `json:"uri,omitempty"`
}

type SessionPromptParams struct {
	SessionID string               `json:"sessionId"`
	Prompt    []PromptContentBlock `json:"prompt"`
}

type SessionPromptResult struct {
	SessionID string `json:"sessionId"`
	Status    string `json:"status"` // "done", "cancelled", "error"
}

// Session Update Notification (streaming output)
type ToolCallUpdate struct {
	ID        string                 `json:"id"`
	Title     string                 `json:"title,omitempty"`
	Name      string                 `json:"name"`
	Status    string                 `json:"status"` // "pending", "running", "success", "error"
	Input     map[string]interface{} `json:"input,omitempty"`
	Output    string                 `json:"output,omitempty"`
	Diff      string                 `json:"diff,omitempty"`
	Additions int                    `json:"additions,omitempty"`
	Deletions int                    `json:"deletions,omitempty"`
}

type ContextUsage struct {
	InputTokens  int     `json:"inputTokens,omitempty"`
	OutputTokens int     `json:"outputTokens,omitempty"`
	TotalTokens  int     `json:"totalTokens,omitempty"`
	CostUSD      float64 `json:"costUsd,omitempty"`
}

type SessionUpdateParams struct {
	SessionID  string          `json:"sessionId"`
	UpdateType string          `json:"updateType"` // "content", "thinking", "tool_call", "usage", "done"
	Content    string          `json:"content,omitempty"`
	Thinking   string          `json:"thinking,omitempty"`
	ToolCall   *ToolCallUpdate `json:"toolCall,omitempty"`
	Usage      *ContextUsage   `json:"usage,omitempty"`
}

// Request Permission
type RequestPermissionParams struct {
	RequestID   string                 `json:"requestId"`
	SessionID   string                 `json:"sessionId"`
	Title       string                 `json:"title"`
	Description string                 `json:"description,omitempty"`
	Tool        string                 `json:"tool,omitempty"`
	Arguments   map[string]interface{} `json:"arguments,omitempty"`
}

type RequestPermissionResult struct {
	Decision string `json:"decision"` // "allow", "deny", "allow_once"
}

// Session Configuration and Options
type SessionOpts struct {
	AgentID        string            `json:"agentId"`        // "agy", "claude", "codex", or local command name
	Command        string            `json:"command"`        // Executable path or name
	Args           []string          `json:"args,omitempty"` // Extra arguments
	Cwd            string            `json:"cwd"`            // Working directory
	Env            map[string]string `json:"env,omitempty"`  // Extra environment variables
	Model          string            `json:"model,omitempty"`
	PermissionMode string            `json:"permissionMode"` // "supervised", "auto", "auto_edits", "full"
}

// Session State Model
type Session struct {
	ID             string         `json:"id"`
	AgentID        string         `json:"agentId"`
	Cwd            string         `json:"cwd"`
	Model          string         `json:"model"`
	Protocol       string         `json:"protocol,omitempty"` // "jsonrpc", "stream-json", "claude-stream", "codex-appserver"
	ThreadID       string         `json:"threadId,omitempty"`
	PermissionMode string         `json:"permissionMode"`
	Status         string         `json:"status"` // "idle", "running", "waiting_permission", "error", "closed"
	CreatedAt      int64          `json:"createdAt"`
	UpdatedAt      int64          `json:"updatedAt"`
	Usage          *ContextUsage  `json:"usage,omitempty"`
	PendingPerms   []*PendingPerm `json:"pendingPerms,omitempty"`
}

type PendingPerm struct {
	RequestID   string                 `json:"requestId"`
	Title       string                 `json:"title"`
	Description string                 `json:"description,omitempty"`
	Tool        string                 `json:"tool,omitempty"`
	Arguments   map[string]interface{} `json:"arguments,omitempty"`
	CreatedAt   int64                  `json:"createdAt"`
}
