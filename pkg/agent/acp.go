package agent

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/Haslab-dev/RakitKode/events"
	"github.com/hasdev/forge-ade/forge-ade-native/pkg/engine"
	"github.com/hasdev/forge-ade/internal/acp"
)

// ACPAgentDefinition defines metadata and CLI execution parameters for an ACP coding agent.
type ACPAgentDefinition struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Type             string   `json:"type"` // "internal", "acp", "claude", "codex", "opencode", "commandcode", "pi", "ohmypi", "gemini"
	Description      string   `json:"description"`
	Command          string   `json:"command"`
	Args             []string `json:"args"`
	Model            string   `json:"model"`
	SupportedModels  []string `json:"supportedModels"`
	DefaultEffort    string   `json:"defaultEffort"`
	SupportedEfforts []string `json:"supportedEfforts"`
	Available        bool     `json:"available"` // Detected on PATH
	IsDefault        bool     `json:"isDefault"`
}

// PermissionOption is one choice of an ACP permission request.
type PermissionOption struct {
	OptionID string `json:"optionId"`
	Name     string `json:"name"`
	Kind     string `json:"kind"` // "allow_once", "allow_always", "reject_once", ...
}

// PermissionRequest represents a pending permission prompt from an ACP agent,
// as emitted by forge-ade's internal manager on the "agent:ask" event.
type PermissionRequest struct {
	SessionID   string             `json:"sessionId"`
	RequestID   string             `json:"requestId"`
	ToolName    string             `json:"toolName"`
	Title       string             `json:"title"`
	Description string             `json:"description"`
	Arguments   string             `json:"arguments,omitempty"`
	Options     []PermissionOption `json:"options,omitempty"`
}

// ToolEvent is one ACP tool call update (forge-ade emits agent:tool_start /
// agent:tool_end with these payload fields).
type ToolEvent struct {
	SessionID  string
	ToolCallID string
	Name       string
	Args       string
	Status     string // "running", "completed", "failed"
	Output     string
	IsError    bool
}

// TurnEndInfo carries the agent:turn_end payload fields.
type TurnEndInfo struct {
	Status          string // "completed", "stopped", "closed", "error", ...
	Error           string
	TotalSteps      int
	StepCount       int
	RemoteSessionID string
}

// UsageInfo carries the token usage of one turn (agent:usage / turn_end).
type UsageInfo struct {
	InputTokens  int
	OutputTokens int
	CachedTokens int
	TotalTokens  int
	CostUSD      float64
}

// ACPManager integrates ForgeADE's internal ACP JSON-RPC manager with the native UI.
type ACPManager struct {
	mu             sync.RWMutex
	manager        *acp.Manager
	bus            *events.Bus
	Agents         []ACPAgentDefinition
	ActiveReq      *PermissionRequest
	OnTokenChunk   func(sessionID, text string)
	OnThoughtChunk func(sessionID, text string)
	OnToolEvent    func(ToolEvent)
	OnTurnStart    func(sessionID, prompt string)
	OnTurnDone     func(sessionID string, info TurnEndInfo)
	OnAsk          func(*PermissionRequest)
	OnUsage        func(sessionID string, usage UsageInfo)
	OnCommands     func(sessionID string, commands []string)
	OnNeedRedraw   func()
}

// NewACPManager creates and initializes the native ACP integration.
func NewACPManager(engine *engine.Engine, onRedraw func()) *ACPManager {
	mgr := &ACPManager{
		manager:      engine.ACPMgr,
		bus:          engine.Bus,
		OnNeedRedraw: onRedraw,
		Agents:       defaultACPAgentDefinitions(),
	}

	mgr.RefreshAvailability()
	mgr.subscribeToBusEvents()

	return mgr
}

// defaultACPAgentDefinitions registers the official ForgeADE ACP agent ecosystem.
func defaultACPAgentDefinitions() []ACPAgentDefinition {
	return []ACPAgentDefinition{
		{
			ID:               "agent-internal",
			Name:             "ForgeADE Internal",
			Type:             "internal",
			Description:      "High-throughput autonomous engine with multi-turn planning, AST parsing, and direct LLM provider reasoning.",
			Command:          "",
			Model:            "",
			SupportedModels:  []string{},
			DefaultEffort:    "high",
			SupportedEfforts: []string{"low", "medium", "high", "max"},
			Available:        true,
			IsDefault:        true,
		},
		{
			ID:          "agent-agy",
			Name:        "Google Antigravity (AGY)",
			Type:        "acp",
			Description: "Official Google Antigravity autonomous coding agent harness connected via ACP JSON-RPC protocol.",
			Command:     "agy",
			Args:        []string{"--input-format", "stream-json", "--output-format", "stream-json"},
			Model:       "gemini-3.8-flash-high",
			SupportedModels: []string{
				"gemini-3.8-flash-high",
				"gemini-3.8-flash-medium",
				"gemini-3.8-flash-low",
				"claude-sonnet-4-6",
				"claude-opus-4-6-thinking",
				"gpt-oss-120b-medium",
			},
			DefaultEffort:    "high",
			SupportedEfforts: []string{"low", "medium", "high", "max"},
		},
		{
			ID:          "agent-claude",
			Name:        "Anthropic Claude Code",
			Type:        "claude",
			Description: "Anthropic Claude Code CLI harness driving local shell, filesystem tools, and reasoning over stream-json.",
			Command:     "claude",
			Args:        []string{"-p", "--input-format", "stream-json", "--output-format", "stream-json", "--verbose", "--include-partial-messages"},
			Model:       "sonnet",
			SupportedModels: []string{
				"sonnet",
				"glm-5.3-flash",
				"opus",
				"haiku",
				"claude-opus-5-5",
				"claude-fable-5-1[1m]",
			},
			DefaultEffort:    "high",
			SupportedEfforts: []string{"low", "medium", "high", "xhigh"},
		},
		{
			ID:          "agent-codex",
			Name:        "OpenAI Codex",
			Type:        "codex",
			Description: "OpenAI Codex agent harness driven through JSON-RPC 2.0 app-server over stdio.",
			Command:     "codex",
			Args:        []string{"app-server", "--listen", "stdio://"},
			Model:       "deepseek-v4-1-flash",
			SupportedModels: []string{
				"deepseek-v4-1-flash",
				"gpt-6.1-sol",
				"gpt-6-luna",
				"gpt-5.6-terra",
				"gpt-5.6-luna",
				"gpt-5.5",
			},
			DefaultEffort:    "medium",
			SupportedEfforts: []string{"low", "medium", "high"},
		},
		{
			ID:          "agent-opencode",
			Name:        "OpenCode",
			Type:        "opencode",
			Description: "OpenCode autonomous coding assistant connected over Agent Client Protocol (ACP).",
			Command:     "opencode",
			Args:        []string{"acp"},
			Model:       "Kenari/glm-5-3-flash",
			SupportedModels: []string{
				"Kenari/glm-5-3-flash",
				"Kenari/deepseek-v4-1-flash",
				"Kenari/claude-haiku-5-5",
				"Kenari/gpt-6-luna",
				"inadigital/inadigital-flash",
				"inadigital/inadigital-pro",
				"deepseek/deepseek-flash",
				"deepseek/deepseek-v4-pro",
				"opencode/claude-sonnet-5-5",
				"opencode/deepseek-v4.1-flash",
				"opencode/gpt-6-astra",
			},
			DefaultEffort:    "medium",
			SupportedEfforts: []string{"low", "medium", "high"},
		},
		{
			ID:          "agent-commandcode",
			Name:        "Command Code",
			Type:        "commandcode",
			Description: "Command Code autonomous agent with taste learning connected via Agent Client Protocol (ACP).",
			Command:     "cmd",
			Args:        []string{"acp"},
			Model:       "deepseek/deepseek-v4.1-flash",
			SupportedModels: []string{
				"deepseek/deepseek-v4.1-flash",
				"deepseek/deepseek-v4-flash",
				"claude-sonnet-5-5",
				"claude-opus-5-5",
				"gpt-6-astra",
				"gpt-6.1-sol",
				"google/gemini-3.8-flash",
				"qwen/qwen3.8-max",
			},
			DefaultEffort:    "medium",
			SupportedEfforts: []string{"low", "medium", "high", "max"},
		},
		{
			ID:          "agent-pi",
			Name:        "Pi Agent",
			Type:        "pi",
			Description: "Autonomous core agent implementing the Pi protocol with codebase research and live terminal orchestration.",
			Command:     "pi-acp",
			Args:        []string{},
			Model:       "Kenari/glm-5-3-flash",
			SupportedModels: []string{
				"Kenari/glm-5-3-flash",
				"Kenari/claude-haiku-5-5",
				"Kenari/mimo-v2-5",
				"Kenari/deepseek-v4-1-flash",
				"Kenari/gpt-6-luna",
			},
			DefaultEffort:    "medium",
			SupportedEfforts: []string{"minimal", "low", "medium", "high"},
		},
		{
			ID:          "agent-ohmypi",
			Name:        "Oh My Pi (omp)",
			Type:        "ohmypi",
			Description: "High-throughput agent fork with multi-turn reasoning and parallel tool execution.",
			Command:     "omp",
			Args:        []string{"acp"},
			Model:       "kenari/gpt-6-luna",
			SupportedModels: []string{
				"kenari/gpt-6-luna",
				"kenari/claude-haiku-5-5",
				"kenari/mimo-v2.6-flash",
				"kenari/glm-5.3-flash",
			},
			DefaultEffort:    "medium",
			SupportedEfforts: []string{"low", "medium", "high"},
		},
		{
			ID:          "agent-gemini",
			Name:        "Google Gemini CLI",
			Type:        "gemini",
			Description: "Official Google Gemini coding CLI with direct multimodal reasoning.",
			Command:     "gemini",
			Args:        []string{"--acp"},
			Model:       "gemini-2.5-pro",
			SupportedModels: []string{
				"gemini-2.5-pro",
				"gemini-2.5-flash",
				"gemini-3.8-flash-high",
			},
			DefaultEffort:    "high",
			SupportedEfforts: []string{"low", "medium", "high"},
		},
	}
}

// RefreshAvailability detects which agent CLIs are installed and on the user's PATH.
func (mgr *ACPManager) RefreshAvailability() {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	for i := range mgr.Agents {
		if mgr.Agents[i].Type == "internal" {
			mgr.Agents[i].Available = true
			continue
		}
		if mgr.Agents[i].Command != "" {
			_, err := exec.LookPath(mgr.Agents[i].Command)
			mgr.Agents[i].Available = (err == nil)
		}
	}
}

// StartSession launches an ACP agent session.
func (mgr *ACPManager) StartSession(agentID, cwd string, model string) (*acp.Session, error) {
	return mgr.StartSessionWithID("", agentID, cwd, model, "supervised")
}

// StartSessionWithID launches or attaches an ACP agent session with a specific
// session ID. permissionMode is one of forge-ade's SessionOpts values:
// "supervised" | "auto" | "auto_edits" | "full".
func (mgr *ACPManager) StartSessionWithID(sessionID, agentID, cwd, model, permissionMode string) (*acp.Session, error) {
	mgr.mu.Lock()
	var targetDef *ACPAgentDefinition
	for i := range mgr.Agents {
		if mgr.Agents[i].ID == agentID {
			targetDef = &mgr.Agents[i]
			break
		}
	}
	// Fallback to first available ACP agent if unknown or legacy monolith
	if targetDef == nil || targetDef.ID == "agent-monolith" {
		for i := range mgr.Agents {
			if mgr.Agents[i].Available && mgr.Agents[i].Type != "internal" {
				targetDef = &mgr.Agents[i]
				break
			}
		}
	}
	mgr.mu.Unlock()

	if targetDef == nil {
		return nil, fmt.Errorf("no available ACP agent found for ID: %s", agentID)
	}

	if sessionID == "" {
		sessionID = fmt.Sprintf("sess-acp-%d", time.Now().UnixNano()%100000)
	}

	if targetDef.Type == "internal" {
		return &acp.Session{
			ID:        sessionID,
			Title:     targetDef.Name,
			AgentID:   targetDef.ID,
			Cwd:       cwd,
			Model:     model,
			Status:    "idle",
			CreatedAt: time.Now().Unix(),
		}, nil
	}

	// Normalize model against agent's supported models
	if len(targetDef.SupportedModels) > 0 {
		isSupported := false
		for _, sm := range targetDef.SupportedModels {
			if strings.EqualFold(sm, model) {
				isSupported = true
				model = sm
				break
			}
		}
		if !isSupported {
			model = targetDef.Model
		}
	} else if model == "" {
		model = targetDef.Model
	}

	if permissionMode == "" {
		permissionMode = "supervised"
	}

	opts := acp.SessionOpts{
		SessionID:      sessionID,
		AgentID:        targetDef.ID,
		Command:        targetDef.Command,
		Args:           targetDef.Args,
		Cwd:            cwd,
		Model:          model,
		PermissionMode: permissionMode,
	}

	if mgr.manager != nil {
		return mgr.manager.StartSession(opts)
	}
	return nil, fmt.Errorf("acp manager not initialized")
}

// Prompt sends a user turn to an active ACP session.
func (mgr *ACPManager) Prompt(sessionID, text string) error {
	if mgr.manager != nil {
		return mgr.manager.Prompt(sessionID, text)
	}
	return fmt.Errorf("acp manager not initialized")
}

// Cancel stops the currently running turn in the ACP session (interrupts the
// CLI process group for stream agents, protocol cancel for standard ACP).
func (mgr *ACPManager) Cancel(sessionID string) error {
	if mgr.manager != nil {
		return mgr.manager.Cancel(sessionID)
	}
	return nil
}

// RespondPermission answers a permission request with allow/deny; allow picks
// the best allow option of the request, as forge-ade's UI does.
func (mgr *ACPManager) RespondPermission(sessionID, requestID string, allow bool) error {
	mgr.mu.Lock()
	mgr.ActiveReq = nil
	mgr.mu.Unlock()

	if mgr.manager == nil {
		return nil
	}
	if !allow {
		return mgr.manager.RespondPermissionOption(sessionID, requestID, false, "deny")
	}
	// Pick the first allow-kind option; the manager falls back sanely when
	// given an empty option id.
	mgr.mu.RLock()
	req := mgr.ActiveReq
	optionID := ""
	if req != nil && req.RequestID == requestID {
		for _, opt := range req.Options {
			if strings.Contains(opt.Kind, "allow") || opt.OptionID != "" {
				optionID = opt.OptionID
				if strings.Contains(opt.Kind, "allow") {
					break
				}
			}
		}
	}
	mgr.mu.RUnlock()
	if optionID != "" {
		return mgr.manager.RespondPermissionOption(sessionID, requestID, true, optionID)
	}
	return mgr.manager.RespondPermission(sessionID, requestID, true)
}

// RespondPermissionOption answers a permission request with an explicit
// option id ("allow"/"deny"/a specific ACP option).
func (mgr *ACPManager) RespondPermissionOption(sessionID, requestID, optionID string) error {
	mgr.mu.Lock()
	mgr.ActiveReq = nil
	mgr.mu.Unlock()
	if mgr.manager != nil {
		return mgr.manager.RespondPermissionOption(sessionID, requestID, true, optionID)
	}
	return nil
}

// GetUsageSummary returns forge-ade's aggregated ACP usage report for the
// quota/usage modal.
func (mgr *ACPManager) GetUsageSummary() map[string]interface{} {
	if mgr.manager != nil {
		return mgr.manager.GetUsageSummary()
	}
	return map[string]interface{}{}
}

// ListSessions returns the live ACP sessions known to the internal manager.
func (mgr *ACPManager) ListSessions() []*acp.Session {
	if mgr.manager != nil {
		return mgr.manager.ListSessions()
	}
	return nil
}

// subscribeToBusEvents listens to the agent:* events forge-ade's internal ACP
// manager emits on the event bus. Payload keys mirror internal/acp/manager.go.
func (mgr *ACPManager) subscribeToBusEvents() {
	if mgr.bus == nil {
		return
	}

	sessionKey := func(data map[string]interface{}) string {
		id, _ := data["sessionId"].(string)
		if id == "" {
			id, _ = data["session_id"].(string)
		}
		return id
	}
	str := func(data map[string]interface{}, keys ...string) string {
		for _, k := range keys {
			if v, ok := data[k].(string); ok && v != "" {
				return v
			}
		}
		return ""
	}
	num := func(data map[string]interface{}, keys ...string) int {
		for _, k := range keys {
			if v, ok := data[k].(float64); ok {
				return int(v)
			}
		}
		return 0
	}
	stringify := func(v interface{}) string {
		switch t := v.(type) {
		case string:
			return t
		case nil:
			return ""
		default:
			if b, err := json.Marshal(t); err == nil {
				return string(b)
			}
			return fmt.Sprintf("%v", t)
		}
	}

	// Text streaming: {sessionId, delta, kind:"text"}
	mgr.bus.Subscribe(events.AgentMessageDelta, func(e events.Event) {
		if e.Data == nil {
			return
		}
		sessionID := sessionKey(e.Data)
		if kind := str(e.Data, "kind"); kind != "" && kind != "text" && kind != "message" {
			return // thinking and other kinds stream via their own events
		}
		text := str(e.Data, "delta", "text", "content")
		if sessionID != "" && text != "" && mgr.OnTokenChunk != nil {
			mgr.OnTokenChunk(sessionID, text)
		}
	})

	// Thinking: {sessionId, delta}
	mgr.bus.Subscribe("agent:thinking_delta", func(e events.Event) {
		if e.Data == nil {
			return
		}
		sessionID := sessionKey(e.Data)
		text := str(e.Data, "delta", "text", "content", "thinking")
		if sessionID != "" && text != "" && mgr.OnThoughtChunk != nil {
			mgr.OnThoughtChunk(sessionID, text)
		}
	})

	// Tool calls: start {tool_call_id, name, title?, args}; end {tool_call_id,
	// name?, title?, args?, result, is_error}
	handleTool := func(e events.Event, status string) {
		if e.Data == nil {
			return
		}
		ev := ToolEvent{
			SessionID:  sessionKey(e.Data),
			ToolCallID: str(e.Data, "tool_call_id", "toolCallId", "id"),
			Name:       str(e.Data, "name", "toolName", "title"),
			Args:       stringify(e.Data["args"]),
			Status:     status,
			Output:     str(e.Data, "result", "output"),
			IsError:    boolOf(e.Data["is_error"]) || boolOf(e.Data["isError"]),
		}
		if ev.Status == "completed" && ev.Output == "" {
			ev.Output = stringify(e.Data["result"])
		}
		if mgr.OnToolEvent != nil && ev.SessionID != "" {
			mgr.OnToolEvent(ev)
		}
	}
	mgr.bus.Subscribe("agent:tool_start", func(e events.Event) { handleTool(e, "running") })
	mgr.bus.Subscribe("agent:tool_end", func(e events.Event) {
		if e.Data == nil {
			return
		}
		status := "completed"
		if boolOf(e.Data["is_error"]) || boolOf(e.Data["isError"]) {
			status = "failed"
		}
		handleTool(e, status)
	})

	// Turn lifecycle
	mgr.bus.Subscribe("agent:turn_start", func(e events.Event) {
		if e.Data == nil {
			return
		}
		sessionID := sessionKey(e.Data)
		if sessionID != "" && mgr.OnTurnStart != nil {
			mgr.OnTurnStart(sessionID, str(e.Data, "prompt"))
		}
	})
	mgr.bus.Subscribe("agent:turn_end", func(e events.Event) {
		if e.Data == nil {
			return
		}
		sessionID := sessionKey(e.Data)
		if sessionID == "" || mgr.OnTurnDone == nil {
			return
		}
		info := TurnEndInfo{
			Status:          str(e.Data, "status"),
			Error:           str(e.Data, "error"),
			TotalSteps:      num(e.Data, "total_steps", "totalSteps"),
			StepCount:       num(e.Data, "step_count", "stepCount"),
			RemoteSessionID: str(e.Data, "remoteSessionId", "remote_session_id", "conversation_id"),
		}
		if u, ok := e.Data["usage"].(map[string]interface{}); ok {
			mgr.deliverUsage(sessionID, u, num, stringify)
		}
		mgr.OnTurnDone(sessionID, info)
	})

	// Permission prompts: forge-ade emits "agent:ask" in supervised mode with
	// {requestId, title, desc, tool, arguments, options}.
	askHandler := func(e events.Event) {
		if e.Data == nil {
			return
		}
		reqID := str(e.Data, "requestId", "request_id")
		if reqID == "" {
			return
		}
		req := &PermissionRequest{
			SessionID:   sessionKey(e.Data),
			RequestID:   reqID,
			ToolName:    str(e.Data, "tool", "toolName"),
			Title:       str(e.Data, "title"),
			Description: str(e.Data, "desc", "description"),
			Arguments:   stringify(e.Data["arguments"]),
		}
		if req.Title == "" {
			req.Title = req.Description
		}
		if opts, ok := e.Data["options"].([]interface{}); ok {
			for _, o := range opts {
				om, ok := o.(map[string]interface{})
				if !ok {
					continue
				}
				req.Options = append(req.Options, PermissionOption{
					OptionID: str(om, "optionId", "option_id", "id"),
					Name:     str(om, "name", "label"),
					Kind:     str(om, "kind"),
				})
			}
		}
		mgr.mu.Lock()
		mgr.ActiveReq = req
		mgr.mu.Unlock()
		if mgr.OnAsk != nil {
			mgr.OnAsk(req)
		}
		if mgr.OnNeedRedraw != nil {
			mgr.OnNeedRedraw()
		}
	}
	mgr.bus.Subscribe("agent:ask", askHandler)
	// Kept for forward-compatibility with a plain permission event.
	mgr.bus.Subscribe(events.AgentPermission, askHandler)

	// Slash commands advertised by the agent: {commands: [{name, description}]}
	mgr.bus.Subscribe("agent:available_commands", func(e events.Event) {
		if e.Data == nil {
			return
		}
		sessionID := sessionKey(e.Data)
		cmds, _ := e.Data["commands"].([]interface{})
		var names []string
		for _, cmd := range cmds {
			cm, ok := cmd.(map[string]interface{})
			if !ok {
				continue
			}
			if name := str(cm, "name", "command"); name != "" {
				names = append(names, name)
			}
		}
		if sessionID != "" && len(names) > 0 && mgr.OnCommands != nil {
			mgr.OnCommands(sessionID, names)
		}
	})

	// Usage: {usage: {inputTokens, outputTokens, ...}}
	mgr.bus.Subscribe("agent:usage", func(e events.Event) {
		if e.Data == nil {
			return
		}
		sessionID := sessionKey(e.Data)
		if u, ok := e.Data["usage"].(map[string]interface{}); ok && sessionID != "" {
			mgr.deliverUsage(sessionID, u, num, stringify)
		}
	})
}

func boolOf(v interface{}) bool {
	b, _ := v.(bool)
	return b
}

// deliverUsage parses a usage map into UsageInfo and dispatches it.
func (mgr *ACPManager) deliverUsage(sessionID string, u map[string]interface{}, num func(map[string]interface{}, ...string) int, stringify func(interface{}) string) {
	usage := UsageInfo{
		InputTokens:  num(u, "inputTokens", "input_tokens"),
		OutputTokens: num(u, "outputTokens", "output_tokens"),
		CachedTokens: num(u, "cachedTokens", "cached_read_input_tokens", "cache_read_input_tokens", "cached_input_tokens"),
		TotalTokens:  num(u, "totalTokens", "total_tokens"),
	}
	if cost := stringify(u["costUsd"]); cost != "" {
		fmt.Sscanf(cost, "%f", &usage.CostUSD)
	}
	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.InputTokens + usage.OutputTokens
	}
	if mgr.OnUsage != nil {
		mgr.OnUsage(sessionID, usage)
	}
}
