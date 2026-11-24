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
// ACP agent such as Claude Code, Codex, Gemini, or OpenCode) spawned by a
// coordinator session.
type WorkerSpec struct {
	ParentSessionID string
	Name            string
	AgentID         string // e.g. "agent-internal", "agent-claude", "agent-codex", "agent-agy"
	AgentType       string // "internal" or "acp"
	Model           string
	Task            string
	WorkspaceMode   string // "shared" (default) or "worktree"
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

	agentID := strings.TrimSpace(spec.AgentID)
	agentType := strings.TrimSpace(spec.AgentType)
	if agentID == "" {
		agentID = "agent-internal"
	}
	if agentType == "" {
		if agentID == "agent-internal" {
			agentType = "internal"
		} else {
			agentType = "acp"
		}
	}

	model := strings.TrimSpace(spec.Model)
	if model == "" {
		if isACPAgentID(agentID, agentType) {
			model = as.ResolveModelForAgent(agentID, "")
		} else {
			as.mu.Lock()
			model = as.ActiveModel
			as.mu.Unlock()
		}
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
	if strings.EqualFold(spec.WorkspaceMode, "worktree") && wsPath != "" && !strings.HasPrefix(wsPath, "ssh://") {
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
	workerPrompt := fmt.Sprintf("## Task from Coordinator\n**Worker:** `%s`\n\n%s", name, task)
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
