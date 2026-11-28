package agent

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	internalgit "github.com/hasdev/forge-ade/core/git"
)

// WorkerSpec defines a hybrid worker (either Internal RakitKode or an external
// ACP agent such as Antigravity/agy, Claude Code, Codex, Gemini, or OpenCode)
// spawned by a coordinator session.
type WorkerSpec struct {
	ParentSessionID string
	Name            string
	Persona         string // e.g. "research", "plan", "review", "coder", or a Local Bot name/ID
	SystemPrompt    string
	Limitations     string
	AgentID         string // e.g. "agent-agy", "agent-internal", "agent-claude", "agent-codex", or alias ("agy", "auto", "")
	AgentType       string // "internal" or "acp"
	Model           string
	Task            string
	WorkspaceMode   string // "shared", "worktree", or "" (auto by persona)
	WorktreeBranch  string
	AutoStart       bool
}

// WorkerRecord captures the identity and runtime metadata of a spawned worker.
type WorkerRecord struct {
	WorkerID        string
	SessionID       string
	ParentSessionID string
	Name            string
	AgentID         string
	AgentType       string
	Model           string
	WorkspacePath   string
	Branch          string
	Status          string
	StartedAt       time.Time
}

// WorkerResult is returned by CollectWorker for coordinator synthesis.
type WorkerResult struct {
	SessionID    string
	Name         string
	AgentLabel   string
	Status       string
	Summary      string
	ChangedFiles []string
	Duration     time.Duration
}

type personaPreset struct {
	ID            string
	Name          string
	Role          string
	SystemPrompt  string
	Limitations   string
	WorkspaceMode string
}

var builtinPersonaPresets = []personaPreset{
	{
		ID:            "research",
		Name:          "Researcher",
		Role:          "Researcher",
		SystemPrompt:  "Explore the codebase, trace symbols and call sites, and report concise findings with exact file paths.",
		Limitations:   "Read-only: do NOT edit, create, or delete project files.",
		WorkspaceMode: "shared",
	},
	{
		ID:            "plan",
		Name:          "Planner",
		Role:          "Planner",
		SystemPrompt:  "Design the architecture, explicit contracts/interfaces, and disjoint subtasks for parallel workers.",
		Limitations:   "Read-only: do NOT edit, create, or delete project files.",
		WorkspaceMode: "shared",
	},
	{
		ID:            "review",
		Name:          "Reviewer",
		Role:          "Reviewer",
		SystemPrompt:  "Inspect diffs, run relevant tests or linters, and verify correctness, edge cases, and regressions.",
		Limitations:   "Do NOT introduce unrelated feature edits; focus on verification and concise review findings.",
		WorkspaceMode: "shared",
	},
	{
		ID:            "coder",
		Name:          "Coder",
		Role:          "Implementer",
		SystemPrompt:  "Implement the assigned module cleanly, add focused unit tests, and verify they pass.",
		Limitations:   "Stay strictly within your assigned files/module and isolated worktree.",
		WorkspaceMode: "worktree",
	},
}

// NormalizeAgentAlias maps user/LLM shorthand (e.g. "agy", "antigravity", "claude",
// "codex", "internal") to canonical ForgeADE AgentID and AgentType values.
func NormalizeAgentAlias(raw string) (string, string) {
	key := strings.ToLower(strings.TrimSpace(raw))
	switch key {
	case "", "auto", "inherit":
		return "", ""
	case "internal", "rakitkode", "forge", "forge-internal", "agent-internal":
		return "agent-internal", "internal"
	case "agy", "antigravity", "agent-agy":
		return "agent-agy", "acp"
	case "claude", "claude-code", "agent-claude":
		return "agent-claude", "acp"
	case "codex", "openai-codex", "agent-codex":
		return "agent-codex", "acp"
	case "gemini", "gemini-cli", "agent-gemini":
		return "agent-gemini", "acp"
	case "opencode", "agent-opencode":
		return "agent-opencode", "acp"
	}
	if strings.HasPrefix(key, "agent-") {
		return key, "acp"
	}
	return "agent-" + key, "acp"
}

func (as *AgentState) resolveWorkerEngine(parentSessionID, rawAgentID, rawAgentType, rawModel string) (agentID, agentType, model string) {
	agentID, agentType = NormalizeAgentAlias(rawAgentID)
	if agentType == "" && strings.TrimSpace(rawAgentType) != "" {
		agentType = strings.TrimSpace(rawAgentType)
	}

	// Inherit from parent session (or active session/state) when omitted or "auto".
	if agentID == "" {
		as.mu.Lock()
		for _, s := range as.Sessions {
			if s != nil && s.ID == parentSessionID && s.AgentID != "" {
				agentID = s.AgentID
				agentType = s.AgentType
				if rawModel == "" {
					rawModel = s.Model
				}
				break
			}
		}
		if agentID == "" && as.ActiveSessionID != "" {
			for _, s := range as.Sessions {
				if s != nil && s.ID == as.ActiveSessionID && s.AgentID != "" {
					agentID = s.AgentID
					agentType = s.AgentType
					if rawModel == "" {
						rawModel = s.Model
					}
					break
				}
			}
		}
		if agentID == "" && as.ActiveAgentID != "" {
			agentID = as.ActiveAgentID
			agentType = as.ActiveAgentType
		}
		as.mu.Unlock()
	}

	if agentID == "" {
		agentID = "agent-internal"
		agentType = "internal"
	}
	if agentType == "" {
		if agentID == "agent-internal" {
			agentType = "internal"
		} else {
			agentType = "acp"
		}
	}

	model = strings.TrimSpace(rawModel)
	if model == "" {
		if isACPAgentID(agentID, agentType) {
			model = as.ResolveModelForAgent(agentID, "")
		} else {
			as.mu.Lock()
			model = as.ActiveModel
			as.mu.Unlock()
		}
	}
	return agentID, agentType, model
}

// SwarmEngineStatus reports whether an engine is eligible for spawning workers.
type SwarmEngineStatus struct {
	ID       string
	Name     string
	Type     string
	Eligible bool
	Reason   string
}

// SwarmEngineStatuses returns eligibility of Internal and registered ACP engines.
func (as *AgentState) SwarmEngineStatuses() []SwarmEngineStatus {
	var out []SwarmEngineStatus
	internalOK := false
	internalReason := "no internal LLM provider configured"
	if as != nil && as.engine != nil && as.engine.LLMClient != nil {
		cfg := as.engine.LLMClient.GetConfig()
		if strings.TrimSpace(cfg.APIKey) != "" || strings.EqualFold(cfg.ProviderID, "ollama") || strings.TrimSpace(cfg.BaseURL) != "" {
			internalOK = true
			internalReason = ""
		}
	}
	out = append(out, SwarmEngineStatus{
		ID:       "agent-internal",
		Name:     "ForgeADE Internal",
		Type:     "internal",
		Eligible: internalOK,
		Reason:   internalReason,
	})

	if as != nil && as.ACPMgr != nil {
		for _, def := range as.ACPMgr.Agents {
			if def.Type == "internal" || def.ID == "agent-internal" {
				continue
			}
			reason := ""
			if !def.Available {
				reason = fmt.Sprintf("%s CLI not detected on PATH", def.Command)
			}
			out = append(out, SwarmEngineStatus{
				ID:       def.ID,
				Name:     def.Name,
				Type:     "acp",
				Eligible: def.Available,
				Reason:   reason,
			})
		}
	}
	return out
}

func (as *AgentState) validateSwarmEngine(agentID string) error {
	if as == nil || as.ACPMgr == nil {
		return nil
	}
	if agentID == "agent-internal" {
		return nil
	}
	for _, st := range as.SwarmEngineStatuses() {
		if st.ID == agentID {
			if st.Eligible {
				return nil
			}
			var eligible []string
			for _, cand := range as.SwarmEngineStatuses() {
				if cand.Eligible {
					eligible = append(eligible, cand.ID)
				}
			}
			if len(eligible) > 0 {
				return fmt.Errorf("%s unavailable (%s); eligible engines: %s", agentID, st.Reason, strings.Join(eligible, ", "))
			}
			return fmt.Errorf("%s unavailable (%s)", agentID, st.Reason)
		}
	}
	return nil
}

func (as *AgentState) resolveWorkerPersona(spec WorkerSpec) (role, sysPrompt, limitations, defaultWsMode, botAgentID, botAgentType, defModel string) {
	personaKey := strings.ToLower(strings.TrimSpace(spec.Persona))
	if personaKey != "" {
		if as != nil && as.Bots != nil {
			for _, b := range as.Bots.List() {
				if b == nil {
					continue
				}
				if strings.EqualFold(b.ID, personaKey) || strings.EqualFold(b.Name, personaKey) {
					role = b.Role
					if role == "" {
						role = b.Name
					}
					sysPrompt = b.SystemPrompt
					limitations = b.Limitations
					botAgentID = b.AgentID
					botAgentType = b.AgentType
					defModel = b.Model
					break
				}
			}
		}
		if role == "" && as != nil && as.engine != nil && as.engine.AgentMgr != nil {
			for _, def := range as.engine.AgentMgr.ListAgentDefinitions() {
				if strings.EqualFold(def.ID, personaKey) || strings.EqualFold(def.Name, personaKey) || strings.EqualFold(string(def.Role), personaKey) {
					role = def.Name
					sysPrompt = def.Prompt
					limitations = def.Rules
					defModel = def.Model
					if def.ReadOnly {
						if limitations != "" {
							limitations += "\n"
						}
						limitations += "Read-only: do NOT edit, create, or delete project files."
						defaultWsMode = "shared"
					} else if def.Role == "coding" || def.Role == "testing" {
						defaultWsMode = "worktree"
					}
					break
				}
			}
		}
		if role == "" {
			for _, p := range builtinPersonaPresets {
				if personaKey == p.ID || strings.EqualFold(personaKey, p.Name) || strings.EqualFold(personaKey, p.Role) {
					role = p.Role
					sysPrompt = p.SystemPrompt
					limitations = p.Limitations
					defaultWsMode = p.WorkspaceMode
					break
				}
			}
		}
	}
	if strings.TrimSpace(spec.SystemPrompt) != "" {
		if sysPrompt != "" {
			sysPrompt += "\n\n" + strings.TrimSpace(spec.SystemPrompt)
		} else {
			sysPrompt = strings.TrimSpace(spec.SystemPrompt)
		}
	}
	if strings.TrimSpace(spec.Limitations) != "" {
		if limitations != "" {
			limitations += "\n" + strings.TrimSpace(spec.Limitations)
		} else {
			limitations = strings.TrimSpace(spec.Limitations)
		}
	}
	if defaultWsMode == "" {
		lowerName := strings.ToLower(spec.Name + " " + spec.Persona)
		switch {
		case strings.Contains(lowerName, "research"), strings.Contains(lowerName, "plan"), strings.Contains(lowerName, "review"), strings.Contains(lowerName, "explore"):
			defaultWsMode = "shared"
		}
	}
	return role, sysPrompt, limitations, defaultWsMode, botAgentID, botAgentType, defModel
}

// SpawnWorker launches a hybrid worker session (Internal or ACP) linked to
// spec.ParentSessionID. When WorkspaceMode is "worktree", the worker receives
// an isolated git worktree so parallel editing workers never conflict.
func (as *AgentState) SpawnWorker(spec WorkerSpec) (*WorkerRecord, error) {
	if as == nil {
		return nil, fmt.Errorf("agent state is nil")
	}
	task := strings.TrimSpace(spec.Task)
	if task == "" {
		return nil, fmt.Errorf("worker task is required")
	}

	name := strings.TrimSpace(spec.Name)
	if name == "" {
		name = "worker"
	}

	role, sysPrompt, limitations, defaultWsMode, botAgentID, botAgentType, defModel := as.resolveWorkerPersona(spec)
	rawAgentID := spec.AgentID
	rawAgentType := spec.AgentType
	rawModel := spec.Model
	if strings.TrimSpace(rawAgentID) == "" && botAgentID != "" {
		rawAgentID = botAgentID
		rawAgentType = botAgentType
	}
	if strings.TrimSpace(rawModel) == "" && defModel != "" {
		rawModel = defModel
	}
	agentID, agentType, model := as.resolveWorkerEngine(spec.ParentSessionID, rawAgentID, rawAgentType, rawModel)
	if err := as.validateSwarmEngine(agentID); err != nil {
		return nil, err
	}

	wsMode := strings.TrimSpace(spec.WorkspaceMode)
	if wsMode == "" {
		wsMode = defaultWsMode
	}
	if wsMode == "" {
		wsMode = "shared"
	}

	// Resolve parent workspace path.
	as.mu.Lock()
	wsPath := as.workspacePath
	for _, s := range as.Sessions {
		if s != nil && s.ID == spec.ParentSessionID && s.WorkspacePath != "" {
			wsPath = s.WorkspacePath
			break
		}
	}
	as.mu.Unlock()

	branch := ""
	if strings.EqualFold(wsMode, "worktree") && wsPath != "" && !strings.HasPrefix(wsPath, "ssh://") {
		branch = sanitizeWorktreeBranch(spec.WorktreeBranch)
		if branch == "" {
			branch = fmt.Sprintf("worker/%s-%d", sanitizeWorktreeBranch(name), time.Now().Unix()%10000)
		}
		wtDir := filepath.Join(filepath.Dir(wsPath), filepath.Base(wsPath)+"-"+strings.ReplaceAll(branch, "/", "-"))
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		err := internalgit.CreateWorktree(ctx, wsPath, wtDir, branch, true)
		cancel()
		if err != nil {
			return nil, fmt.Errorf("create worker worktree: %w", err)
		}
		wsPath = wtDir
	}

	now := time.Now()
	var promptParts []string
	header := fmt.Sprintf("## Task from Coordinator\n**Worker:** `%s`", name)
	if role != "" {
		header += fmt.Sprintf(" (%s)", role)
	}
	promptParts = append(promptParts, header)
	if sysPrompt != "" {
		promptParts = append(promptParts, "### Persona Instructions\n"+sysPrompt)
	}
	if limitations != "" {
		promptParts = append(promptParts, "### Limitations & Guardrails\n"+limitations)
	}
	promptParts = append(promptParts, "### Task\n"+task)
	workerPrompt := strings.Join(promptParts, "\n\n")
	userMsg := &ChatMessage{
		ID:        1,
		Role:      "user",
		Text:      workerPrompt,
		Done:      true,
		Timestamp: now,
	}

	as.mu.Lock()
	id := as.nextSessionIDLocked()
	prov := as.ActiveProvider
	if prov == "" && as.engine != nil && as.engine.LLMClient != nil {
		prov = as.engine.LLMClient.GetConfig().ProviderID
	}

	workerSess := &AgentSession{
		ID:              id,
		Title:           name,
		Status:          "running",
		CreatedAt:       now,
		UpdatedAt:       now,
		Model:           model,
		Provider:        prov,
		AgentID:         agentID,
		AgentType:       agentType,
		WorkspacePath:   wsPath,
		ParentSessionID: spec.ParentSessionID,
		WorkerName:      name,
		Messages:        []*ChatMessage{userMsg},
		Tasks:           NewTaskManager(),
		nextMsgID:       2,
	}
	if !spec.AutoStart {
		workerSess.Status = "idle"
	}

	// Record a concise "Started worker" card note in the parent coordinator session.
	var parentSess *AgentSession
	for _, s := range as.Sessions {
		if s != nil && s.ID == spec.ParentSessionID {
			parentSess = s
			noteID := nextMessageIDLocked(s)
			s.Messages = append(s.Messages, &ChatMessage{
				ID:        noteID,
				Role:      "assistant",
				Text:      fmt.Sprintf("> **Started worker** `%s` (%s)", name, displayAgentLabel(agentID, agentType, model)),
				Done:      true,
				Timestamp: now,
			})
			s.UpdatedAt = now
			break
		}
	}

	as.Sessions = append([]*AgentSession{workerSess}, as.Sessions...)
	as.mu.Unlock()

	as.SaveSessionToDisk(workerSess)
	if parentSess != nil {
		as.SaveSessionToDisk(parentSess)
	}
	if spec.AutoStart {
		as.TriggerTurnForSession(id, workerPrompt)
	}
	if as.Invalidate != nil {
		as.Invalidate()
	}

	return &WorkerRecord{
		WorkerID:        id,
		SessionID:       id,
		ParentSessionID: spec.ParentSessionID,
		Name:            name,
		AgentID:         agentID,
		AgentType:       agentType,
		Model:           model,
		WorkspacePath:   wsPath,
		Branch:          branch,
		Status:          workerSess.Status,
		StartedAt:       now,
	}, nil
}

// MessageWorker sends a follow-up instruction to an existing hybrid worker session.
func (as *AgentState) MessageWorker(workerSessionID, instruction string) error {
	if as == nil {
		return fmt.Errorf("agent state is nil")
	}
	instruction = strings.TrimSpace(instruction)
	if instruction == "" {
		return fmt.Errorf("instruction is empty")
	}

	as.mu.Lock()
	var target *AgentSession
	for _, s := range as.Sessions {
		if s != nil && s.ID == workerSessionID {
			target = s
			break
		}
	}
	if target == nil {
		as.mu.Unlock()
		return fmt.Errorf("worker session %q not found", workerSessionID)
	}
	now := time.Now()
	msgID := nextMessageIDLocked(target)
	target.Messages = append(target.Messages, &ChatMessage{
		ID:        msgID,
		Role:      "user",
		Text:      instruction,
		Done:      true,
		Timestamp: now,
	})
	target.UpdatedAt = now
	as.mu.Unlock()

	as.SaveSessionToDisk(target)
	as.TriggerTurnForSession(workerSessionID, instruction)
	if as.Invalidate != nil {
		as.Invalidate()
	}
	return nil
}

// StopWorker cancels a running hybrid worker (both Internal and ACP).
func (as *AgentState) StopWorker(workerSessionID string) {
	if as == nil || workerSessionID == "" {
		return
	}
	as.mu.Lock()
	var target *AgentSession
	for _, s := range as.Sessions {
		if s != nil && s.ID == workerSessionID {
			target = s
			break
		}
	}
	remoteID := ""
	agentType := ""
	if target != nil {
		remoteID = target.RemoteSessionID
		agentType = target.AgentType
		if target.Status == "running" {
			target.Status = "stopped"
			target.UpdatedAt = time.Now()
		}
	}
	as.mu.Unlock()

	if agentType == "internal" && as.engine != nil && as.engine.AgentMgr != nil && remoteID != "" {
		as.engine.AgentMgr.StopTurn(remoteID)
	} else if as.ACPMgr != nil && as.ACPMgr.manager != nil {
		_ = as.ACPMgr.manager.Cancel(workerSessionID)
	}
	if as.Invalidate != nil {
		as.Invalidate()
	}
}

// CollectWorker gathers the latest output summary and changed files from a worker.
func (as *AgentState) CollectWorker(workerSessionID string) (*WorkerResult, error) {
	if as == nil {
		return nil, fmt.Errorf("agent state is nil")
	}
	as.mu.Lock()
	defer as.mu.Unlock()

	var target *AgentSession
	for _, s := range as.Sessions {
		if s != nil && s.ID == workerSessionID {
			target = s
			break
		}
	}
	if target == nil {
		return nil, fmt.Errorf("worker session %q not found", workerSessionID)
	}

	res := &WorkerResult{
		SessionID:  target.ID,
		Name:       target.WorkerName,
		AgentLabel: displayAgentLabel(target.AgentID, target.AgentType, target.Model),
		Status:     target.Status,
		Duration:   target.UpdatedAt.Sub(target.CreatedAt),
	}
	if res.Name == "" {
		res.Name = target.Title
	}
	seen := make(map[string]bool)
	for _, m := range target.Messages {
		if m == nil {
			continue
		}
		for _, fc := range m.Changes {
			if fc.Path != "" && !seen[fc.Path] {
				seen[fc.Path] = true
				res.ChangedFiles = append(res.ChangedFiles, fc.Path)
			}
		}
		if m.Role == "assistant" && strings.TrimSpace(m.Text) != "" {
			res.Summary = strings.TrimSpace(m.Text)
		}
	}
	return res, nil
}

// WorkersForParent returns all non-archived hybrid worker sessions spawned
// under parentSessionID, ordered oldest-first so sidebar rows stay stable.
func (as *AgentState) WorkersForParent(parentSessionID string) []*AgentSession {
	if as == nil || parentSessionID == "" {
		return nil
	}
	as.mu.Lock()
	defer as.mu.Unlock()

	var out []*AgentSession
	for i := len(as.Sessions) - 1; i >= 0; i-- {
		s := as.Sessions[i]
		if s != nil && !s.Archived && s.ParentSessionID == parentSessionID {
			out = append(out, s)
		}
	}
	return out
}
