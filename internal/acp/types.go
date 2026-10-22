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
	Cwd        string        `json:"cwd"`
	McpServers []interface{} `json:"mcpServers"`
}

type SessionNewResult struct {
	SessionID     string                 `json:"sessionId"`
	Models        interface{}            `json:"models,omitempty"`
	ConfigOptions []interface{}          `json:"configOptions,omitempty"`
	Modes         interface{}            `json:"modes,omitempty"`
	Meta          map[string]interface{} `json:"_meta,omitempty"`
}

// Prompt Content
type PromptImageSource struct {
	Type      string `json:"type,omitempty"`       // "base64"
	MediaType string `json:"media_type,omitempty"` // "image/png"
	Data      string `json:"data,omitempty"`       // base64
}

type PromptContentBlock struct {
	Type     string             `json:"type"` // "text", "image", "resource"
	Text     string             `json:"text,omitempty"`
	Data     string             `json:"data,omitempty"` // base64
	MimeType string             `json:"mimeType,omitempty"`
	URI      string             `json:"uri,omitempty"`
	Source   *PromptImageSource `json:"source,omitempty"`
}

type SessionPromptParams struct {
	SessionID string               `json:"sessionId"`
	Prompt    []PromptContentBlock `json:"prompt"`
}

type SessionPromptResult struct {
	SessionID  string        `json:"sessionId,omitempty"`
	Status     string        `json:"status,omitempty"`     // "done", "cancelled", "error"
	StopReason string        `json:"stopReason,omitempty"` // "end_turn", "cancelled", etc.
	Usage      *ContextUsage `json:"usage,omitempty"`
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
	CachedTokens int     `json:"cachedTokens,omitempty"`
	TotalTokens  int     `json:"totalTokens,omitempty"`
	CostUSD      float64 `json:"costUsd,omitempty"`
}

// UnmarshalJSON supports both camelCase (ACP SDK) and snake_case (Claude, Antigravity, OpenAI).
func (u *ContextUsage) UnmarshalJSON(data []byte) error {
	type Alias ContextUsage
	var aux struct {
		Alias
		InputTokensAlt     int     `json:"input_tokens"`
		OutputTokensAlt    int     `json:"output_tokens"`
		CachedTokensAlt    int     `json:"cached_tokens"`
		CacheReadTokensAlt int     `json:"cache_read_input_tokens"`
		TotalTokensAlt     int     `json:"total_tokens"`
		CostUSDAlt         float64 `json:"cost_usd"`
		TotalCostUSDAlt    float64 `json:"total_cost_usd"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	*u = ContextUsage(aux.Alias)
	if u.InputTokens == 0 && aux.InputTokensAlt > 0 {
		u.InputTokens = aux.InputTokensAlt
	}
	if u.OutputTokens == 0 && aux.OutputTokensAlt > 0 {
		u.OutputTokens = aux.OutputTokensAlt
	}
	if u.CachedTokens == 0 {
		if aux.CachedTokensAlt > 0 {
			u.CachedTokens = aux.CachedTokensAlt
		} else if aux.CacheReadTokensAlt > 0 {
			u.CachedTokens = aux.CacheReadTokensAlt
		}
	}
	if u.TotalTokens == 0 {
		if aux.TotalTokensAlt > 0 {
			u.TotalTokens = aux.TotalTokensAlt
		} else if u.InputTokens > 0 || u.OutputTokens > 0 {
			u.TotalTokens = u.InputTokens + u.OutputTokens
		}
	}
	if u.CostUSD == 0 {
		if aux.CostUSDAlt > 0 {
			u.CostUSD = aux.CostUSDAlt
		} else if aux.TotalCostUSDAlt > 0 {
			u.CostUSD = aux.TotalCostUSDAlt
		}
	}
	return nil
}

type SessionUpdateParams struct {
	SessionID  string          `json:"sessionId"`
	UpdateType string          `json:"updateType"` // "content", "thinking", "tool_call", "usage", "done"
	Content    string          `json:"content,omitempty"`
	Thinking   string          `json:"thinking,omitempty"`
	ToolCall   *ToolCallUpdate `json:"toolCall,omitempty"`
	Usage      *ContextUsage   `json:"usage,omitempty"`
}

// ACP SDK Permission Types
type ACPPermissionOption struct {
	OptionID string `json:"optionId"`
	Name     string `json:"name,omitempty"`
	Kind     string `json:"kind,omitempty"`
}

type ACPToolCallUpdate struct {
	ToolCallID string      `json:"toolCallId,omitempty"`
	Title      string      `json:"title,omitempty"`
	Kind       string      `json:"kind,omitempty"`
	RawInput   interface{} `json:"rawInput,omitempty"`
}

// Request Permission
type RequestPermissionParams struct {
	RequestID   string                 `json:"requestId,omitempty"`
	SessionID   string                 `json:"sessionId,omitempty"`
	Title       string                 `json:"title,omitempty"`
	Description string                 `json:"description,omitempty"`
	Tool        string                 `json:"tool,omitempty"`
	Arguments   map[string]interface{} `json:"arguments,omitempty"`

	ToolCall *ACPToolCallUpdate    `json:"toolCall,omitempty"`
	Options  []ACPPermissionOption `json:"options,omitempty"`
}

type RequestPermissionResult struct {
	Decision string                 `json:"decision,omitempty"` // "allow", "deny", "allow_once"
	Outcome  map[string]interface{} `json:"outcome,omitempty"`
}

// Session Configuration and Options
type SessionOpts struct {
	SessionID       string            `json:"sessionId,omitempty"`       // Local session ID to reuse/restore
	ResumeSessionID string            `json:"resumeSessionId,omitempty"` // Previous remote or agent session ID to resume
	Title           string            `json:"title,omitempty"`           // User-friendly session title
	AgentID         string            `json:"agentId"`                   // "agy", "claude", "codex", or local command name
	Command         string            `json:"command"`                   // Executable path or name
	Args            []string          `json:"args,omitempty"`            // Extra arguments
	Cwd             string            `json:"cwd"`                       // Working directory
	Env             map[string]string `json:"env,omitempty"`             // Extra environment variables
	Model           string            `json:"model,omitempty"`
	PermissionMode  string            `json:"permissionMode"` // "supervised", "auto", "auto_edits", "full"
}

// Session State Model
type AvailableCommand struct {
	Name        string        `json:"name"`
	Description string        `json:"description,omitempty"`
	Input       *CommandInput `json:"input,omitempty"`
}

type CommandInput struct {
	Hint string `json:"hint,omitempty"`
}

type Session struct {
	ID                string             `json:"id"`
	Title             string             `json:"title,omitempty"`
	AgentID           string             `json:"agentId"`
	Cwd               string             `json:"cwd"`
	Model             string             `json:"model"`
	Protocol          string             `json:"protocol,omitempty"` // "jsonrpc", "stream-json", "claude-stream", "codex-appserver"
	ThreadID          string             `json:"threadId,omitempty"`
	RemoteSessionID   string             `json:"remoteSessionId,omitempty"`
	PermissionMode    string             `json:"permissionMode"`
	Status            string             `json:"status"` // "idle", "running", "waiting_permission", "error", "closed"
	CreatedAt         int64              `json:"createdAt"`
	UpdatedAt         int64              `json:"updatedAt"`
	PromptCount       int                `json:"promptCount,omitempty"`
	Usage             *ContextUsage      `json:"usage,omitempty"`
	PendingPerms      []*PendingPerm     `json:"pendingPerms,omitempty"`
	AvailableCommands []AvailableCommand `json:"availableCommands,omitempty"`
}

type PendingPerm struct {
	RequestID   string                 `json:"requestId"`
	Title       string                 `json:"title"`
	Description string                 `json:"description,omitempty"`
	Tool        string                 `json:"tool,omitempty"`
	Arguments   map[string]interface{} `json:"arguments,omitempty"`
	CreatedAt   int64                  `json:"createdAt"`
}
