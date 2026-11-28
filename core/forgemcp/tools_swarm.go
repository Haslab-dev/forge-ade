package forgemcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// SwarmAgentInfo describes an available or unavailable agent engine for swarm execution.
type SwarmAgentInfo struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"` // "internal" or "acp"
	Eligible  bool   `json:"eligible"`
	Reason    string `json:"reason,omitempty"`
	IsDefault bool   `json:"isDefault,omitempty"`
}

// SwarmPersonaInfo describes a reusable persona/bot role that can be assigned to a worker.
type SwarmPersonaInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Role        string `json:"role"`
	Limitations string `json:"limitations,omitempty"`
	Source      string `json:"source"` // "builtin" or "bot"
}

// SwarmSpawnRequest parameterizes spawning a parallel/swarm worker via MCP.
type SwarmSpawnRequest struct {
	ParentSessionID string `json:"parentSessionId,omitempty"`
	Name            string `json:"name"`
	Task            string `json:"task"`
	Persona         string `json:"persona,omitempty"`
	AgentID         string `json:"agentId,omitempty"`
	Model           string `json:"model,omitempty"`
	WorkspaceMode   string `json:"workspaceMode,omitempty"` // "shared", "worktree", or "" (auto by persona)
	WorktreeBranch  string `json:"worktreeBranch,omitempty"`
	SystemPrompt    string `json:"systemPrompt,omitempty"`
	Limitations     string `json:"limitations,omitempty"`
}

// SwarmWorkerInfo describes a spawned worker's live state.
type SwarmWorkerInfo struct {
	SessionID     string `json:"sessionId"`
	Name          string `json:"name"`
	AgentID       string `json:"agentId"`
	AgentType     string `json:"agentType"`
	Model         string `json:"model,omitempty"`
	WorkspacePath string `json:"workspacePath"`
	Branch        string `json:"branch,omitempty"`
	Status        string `json:"status"`
}

// SwarmWorkerResult captures the collected output of a worker.
type SwarmWorkerResult struct {
	SessionID    string   `json:"sessionId"`
	Name         string   `json:"name"`
	AgentLabel   string   `json:"agentLabel"`
	Status       string   `json:"status"`
	Summary      string   `json:"summary"`
	ChangedFiles []string `json:"changedFiles,omitempty"`
	DurationMs   int64    `json:"durationMs"`
}

// SwarmHandoffRequest parameterizes a cross-agent or worktree handoff via MCP.
type SwarmHandoffRequest struct {
	SourceSessionID string `json:"sourceSessionId,omitempty"`
	TargetAgentID   string `json:"targetAgentId,omitempty"`
	TargetModel     string `json:"targetModel,omitempty"`
	Goal            string `json:"goal"`
	NextSteps       string `json:"nextSteps,omitempty"`
	CreateWorktree  bool   `json:"createWorktree,omitempty"`
	WorktreeBranch  string `json:"worktreeBranch,omitempty"`
	Background      bool   `json:"background,omitempty"`
}

// SwarmBridge connects the UI-agnostic ForgeMCP server to the live ForgeADE orchestrator.
type SwarmBridge interface {
	ListAgents() ([]SwarmAgentInfo, []SwarmPersonaInfo)
	SpawnWorker(req SwarmSpawnRequest) (*SwarmWorkerInfo, error)
	ListWorkers(parentSessionID string) []SwarmWorkerInfo
	MessageWorker(workerSessionID, instruction string) error
	CollectWorker(workerSessionID string) (*SwarmWorkerResult, error)
	HandoffSession(req SwarmHandoffRequest) (*SwarmWorkerInfo, error)
}

// RegisterSwarmTools registers swarm orchestration, persona, and handoff tools on the MCP server.
func (s *Server) RegisterSwarmTools(bridge SwarmBridge) {
	if bridge == nil {
		return
	}

	s.RegisterTool(Tool{
		Name:        "swarm_list_agents",
		Description: "List eligible agent engines (e.g. agent-agy, agent-internal, agent-claude, agent-codex) with auth/quota eligibility and available swarm personas (research, plan, review, coder, or user bots).",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
	}, func(ctx context.Context, args map[string]interface{}) (*ToolCallResult, error) {
		agents, personas := bridge.ListAgents()
		return jsonTextResult(map[string]interface{}{
			"agents":   agents,
			"personas": personas,
		})
	})

	s.RegisterTool(Tool{
		Name:        "swarm_spawn_worker",
		Description: "Spawn a parallel swarm worker session (optionally in an isolated git worktree) with a persona role such as research, plan, review, coder, or a local Bot name. If agent_id is omitted, inherits the active/parent session's engine (e.g. agent-agy).",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name":              map[string]interface{}{"type": "string", "description": "Short worker name (e.g. research-auth, coder-a, reviewer)"},
				"task":              map[string]interface{}{"type": "string", "description": "Scoped task prompt and acceptance criteria for this worker"},
				"persona":           map[string]interface{}{"type": "string", "description": "Optional persona/role (research, plan, review, coder, or Local Bot name)"},
				"agent_id":          map[string]interface{}{"type": "string", "description": "Optional engine ID or alias (agy, internal, claude, codex). Omit to inherit coordinator engine."},
				"model":             map[string]interface{}{"type": "string", "description": "Optional model override"},
				"workspace_mode":    map[string]interface{}{"type": "string", "description": "shared or worktree (omit for auto: shared for research/plan/review, worktree for coding)", "enum": []string{"shared", "worktree"}},
				"worktree_branch":   map[string]interface{}{"type": "string", "description": "Optional branch name when workspace_mode is worktree"},
				"parent_session_id": map[string]interface{}{"type": "string", "description": "Optional coordinator session ID (defaults to active session)"},
				"system_prompt":     map[string]interface{}{"type": "string", "description": "Optional extra persona instructions"},
				"limitations":       map[string]interface{}{"type": "string", "description": "Optional guardrails/limitations for the worker"},
			},
			"required": []string{"name", "task"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (*ToolCallResult, error) {
		req := SwarmSpawnRequest{
			ParentSessionID: stringArg(args, "parent_session_id"),
			Name:            stringArg(args, "name"),
			Task:            stringArg(args, "task"),
			Persona:         stringArg(args, "persona"),
			AgentID:         stringArg(args, "agent_id"),
			Model:           stringArg(args, "model"),
			WorkspaceMode:   stringArg(args, "workspace_mode"),
			WorktreeBranch:  stringArg(args, "worktree_branch"),
			SystemPrompt:    stringArg(args, "system_prompt"),
			Limitations:     stringArg(args, "limitations"),
		}
		info, err := bridge.SpawnWorker(req)
		if err != nil {
			return swarmErrResult(err), nil
		}
		return jsonTextResult(info)
	})

	s.RegisterTool(Tool{
		Name:        "swarm_status",
		Description: "List active and completed swarm workers for the coordinator session.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"parent_session_id": map[string]interface{}{"type": "string", "description": "Optional coordinator session ID (defaults to active session)"},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (*ToolCallResult, error) {
		workers := bridge.ListWorkers(stringArg(args, "parent_session_id"))
		return jsonTextResult(map[string]interface{}{"workers": workers})
	})

	s.RegisterTool(Tool{
		Name:        "swarm_message_worker",
		Description: "Send a follow-up instruction or peer message to an existing swarm worker session.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"session_id":  map[string]interface{}{"type": "string", "description": "Target worker session ID"},
				"instruction": map[string]interface{}{"type": "string", "description": "Message or follow-up instruction"},
			},
			"required": []string{"session_id", "instruction"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (*ToolCallResult, error) {
		if err := bridge.MessageWorker(stringArg(args, "session_id"), stringArg(args, "instruction")); err != nil {
			return swarmErrResult(err), nil
		}
		return jsonTextResult(map[string]interface{}{"ok": true})
	})

	s.RegisterTool(Tool{
		Name:        "swarm_collect_worker",
		Description: "Collect the latest status, output summary, and changed files from a swarm worker.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"session_id": map[string]interface{}{"type": "string", "description": "Worker session ID"},
			},
			"required": []string{"session_id"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (*ToolCallResult, error) {
		res, err := bridge.CollectWorker(stringArg(args, "session_id"))
		if err != nil {
			return swarmErrResult(err), nil
		}
		return jsonTextResult(res)
	})

	s.RegisterTool(Tool{
		Name:        "swarm_handoff",
		Description: "Hand off the current task to another agent engine (e.g. agy, internal, claude, codex) or a fresh git worktree with a structured Goal, Summary, and NextSteps.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"source_session_id": map[string]interface{}{"type": "string", "description": "Source session ID (defaults to active session)"},
				"target_agent_id":   map[string]interface{}{"type": "string", "description": "Target agent ID or alias (agy, internal, claude, codex)"},
				"target_model":      map[string]interface{}{"type": "string", "description": "Optional target model"},
				"goal":              map[string]interface{}{"type": "string", "description": "Clear goal for the receiving agent"},
				"next_steps":        map[string]interface{}{"type": "string", "description": "Concrete next steps for the receiving agent"},
				"create_worktree":   map[string]interface{}{"type": "boolean", "description": "If true, isolate the handed-off session in a new git worktree"},
				"worktree_branch":   map[string]interface{}{"type": "string", "description": "Optional branch name when create_worktree is true"},
				"background":        map[string]interface{}{"type": "boolean", "description": "If true, keep current session focused instead of switching UI"},
			},
			"required": []string{"goal"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (*ToolCallResult, error) {
		bg, _ := args["background"].(bool)
		wt, _ := args["create_worktree"].(bool)
		req := SwarmHandoffRequest{
			SourceSessionID: stringArg(args, "source_session_id"),
			TargetAgentID:   stringArg(args, "target_agent_id"),
			TargetModel:     stringArg(args, "target_model"),
			Goal:            stringArg(args, "goal"),
			NextSteps:       stringArg(args, "next_steps"),
			CreateWorktree:  wt,
			WorktreeBranch:  stringArg(args, "worktree_branch"),
			Background:      bg,
		}
		info, err := bridge.HandoffSession(req)
		if err != nil {
			return swarmErrResult(err), nil
		}
		return jsonTextResult(info)
	})
}

func stringArg(args map[string]interface{}, key string) string {
	if args == nil {
		return ""
	}
	v, _ := args[key].(string)
	return strings.TrimSpace(v)
}

func swarmErrResult(err error) *ToolCallResult {
	return &ToolCallResult{
		Content: []ContentItem{{Type: "text", Text: err.Error()}},
		IsError: true,
	}
}

func jsonTextResult(v interface{}) (*ToolCallResult, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal tool result: %w", err)
	}
	return &ToolCallResult{
		Content: []ContentItem{{Type: "text", Text: string(b)}},
	}, nil
}
