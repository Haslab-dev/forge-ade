package agent

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	internalgit "github.com/hasdev/forge-ade/core/git"
)

// HandoffPacket is the structured context envelope transferred across agents
// (Internal -> ACP, ACP -> ACP, ACP -> Internal, or across git worktrees)
// without depending on provider-private conversation memory.
type HandoffPacket struct {
	Goal            string    `json:"goal"`
	Summary         string    `json:"summary"`
	Branch          string    `json:"branch,omitempty"`
	WorktreePath    string    `json:"worktreePath,omitempty"`
	ChangedFiles    []string  `json:"changedFiles,omitempty"`
	NextSteps       string    `json:"nextSteps,omitempty"`
	SourceSessionID string    `json:"sourceSessionId"`
	SourceTitle     string    `json:"sourceTitle,omitempty"`
	SourceAgent     string    `json:"sourceAgent"`
	TargetAgent     string    `json:"targetAgent"`
	TargetModel     string    `json:"targetModel,omitempty"`
	CreatedAt       time.Time `json:"createdAt"`
}

// HandoffRequest parameterizes an ExecuteHandoff operation.
type HandoffRequest struct {
	SourceSessionID string
	TargetAgentID   string // e.g. "agent-internal", "agent-claude", "agent-codex", "agent-agy"
	TargetAgentType string // "internal" or "acp"
	TargetModel     string // optional override
	Goal            string
	NextSteps       string
	CreateWorktree  bool
	WorktreeBranch  string
	AutoStart       bool // if true, immediately dispatches the opening turn
	Background      bool // if true, does not switch ActiveSessionID
}

// BuildHandoffPacket inspects sourceSessionID and constructs a concise,
// editable HandoffPacket capturing recent progress and changed files.
func (as *AgentState) BuildHandoffPacket(sourceSessionID, goal, nextSteps string) *HandoffPacket {
	pkt := &HandoffPacket{
		Goal:            strings.TrimSpace(goal),
		NextSteps:       strings.TrimSpace(nextSteps),
		SourceSessionID: sourceSessionID,
		CreatedAt:       time.Now(),
	}
	if as == nil {
		return pkt
	}

	as.mu.Lock()
	var src *AgentSession
	for _, s := range as.Sessions {
		if s != nil && s.ID == sourceSessionID {
			src = s
			break
		}
	}
	var msgs []*ChatMessage
	if src != nil {
		pkt.SourceTitle = src.Title
		pkt.WorktreePath = src.WorkspacePath
		pkt.SourceAgent = displayAgentLabel(src.AgentID, src.AgentType, src.Model)
		if pkt.Goal == "" {
			pkt.Goal = src.Title
		}
		msgs = append(msgs, src.Messages...)
	} else {
		pkt.WorktreePath = as.workspacePath
		pkt.SourceAgent = displayAgentLabel(as.ActiveAgentID, as.ActiveAgentType, as.ActiveModel)
	}
	as.mu.Unlock()

	// Extract concise summary from the last few turns & touched files.
	seenFiles := make(map[string]bool)
	var summaryLines []string
	start := 0
	if len(msgs) > 6 {
		start = len(msgs) - 6
	}
	for _, m := range msgs[start:] {
		if m == nil {
			continue
		}
		for _, fc := range m.Changes {
			if fc.Path != "" && !seenFiles[fc.Path] {
				seenFiles[fc.Path] = true
				pkt.ChangedFiles = append(pkt.ChangedFiles, fc.Path)
			}
		}
		txt := strings.TrimSpace(m.Text)
		if txt == "" {
			continue
		}
		if len(txt) > 220 {
			txt = strings.TrimSpace(txt[:220]) + "…"
		}
		role := "User"
		if m.Role == "assistant" {
			role = "Agent"
		}
		summaryLines = append(summaryLines, fmt.Sprintf("- **%s**: %s", role, strings.ReplaceAll(txt, "\n", " ")))
	}
	if len(summaryLines) > 0 {
		pkt.Summary = strings.Join(summaryLines, "\n")
	}

	// Best-effort git branch & working-tree detection for local workspaces.
	if pkt.WorktreePath != "" && !strings.HasPrefix(pkt.WorktreePath, "ssh://") {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		status, err := internalgit.NewEngine().GetStatus(ctx, pkt.WorktreePath)
		cancel()
		if err == nil && status != nil {
			pkt.Branch = status.Branch
			for _, list := range [][]internalgit.FileStatus{status.Staged, status.Unstaged, status.Untracked, status.Conflicts} {
				for _, f := range list {
					if f.Path != "" && !seenFiles[f.Path] {
						seenFiles[f.Path] = true
						pkt.ChangedFiles = append(pkt.ChangedFiles, f.Path)
					}
				}
			}
		}
	}

	return pkt
}

// FormatHandoffPrompt renders a HandoffPacket into a clear structured prompt
// for the receiving agent (Internal or ACP).
func FormatHandoffPrompt(pkt *HandoffPacket) string {
	if pkt == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("## Session Handoff\n")
	if pkt.SourceAgent != "" {
		fmt.Fprintf(&b, "- **From:** %s", pkt.SourceAgent)
		if pkt.SourceTitle != "" {
			fmt.Fprintf(&b, " (%s)", pkt.SourceTitle)
		}
		b.WriteString("\n")
	}
	if pkt.Branch != "" {
		fmt.Fprintf(&b, "- **Branch:** `%s`\n", pkt.Branch)
	}
	if pkt.WorktreePath != "" {
		fmt.Fprintf(&b, "- **Workspace:** `%s`\n", pkt.WorktreePath)
	}
	if pkt.Goal != "" {
		b.WriteString("\n### Goal\n")
		b.WriteString(pkt.Goal)
		b.WriteString("\n")
	}
	if pkt.Summary != "" {
		b.WriteString("\n### Context & Progress So Far\n")
		b.WriteString(pkt.Summary)
		b.WriteString("\n")
	}
	if len(pkt.ChangedFiles) > 0 {
		b.WriteString("\n### Modified / Relevant Files\n")
		limit := len(pkt.ChangedFiles)
		if limit > 15 {
			limit = 15
		}
		for _, f := range pkt.ChangedFiles[:limit] {
			fmt.Fprintf(&b, "- `%s`\n", f)
		}
		if len(pkt.ChangedFiles) > limit {
			fmt.Fprintf(&b, "- … and %d more files\n", len(pkt.ChangedFiles)-limit)
		}
	}
	if pkt.NextSteps != "" {
		b.WriteString("\n### Next Instructions\n")
		b.WriteString(pkt.NextSteps)
		b.WriteString("\n")
	} else {
		b.WriteString("\n### Next Instructions\nContinue from the current workspace state and complete the goal above.\n")
	}
	return strings.TrimSpace(b.String())
}

// ExecuteHandoff creates a linked target session (Internal or ACP, optionally
// in a fresh git worktree), records bidirectional handoff lineage, and
// optionally starts the opening turn with the formatted HandoffPacket.
func (as *AgentState) ExecuteHandoff(req HandoffRequest) (*AgentSession, error) {
	if as == nil {
		return nil, fmt.Errorf("agent state is nil")
	}
	pkt := as.BuildHandoffPacket(req.SourceSessionID, req.Goal, req.NextSteps)

	targetAgentID := strings.TrimSpace(req.TargetAgentID)
	targetAgentType := strings.TrimSpace(req.TargetAgentType)
	if targetAgentID == "" {
		targetAgentID = "agent-internal"
	}
	if targetAgentType == "" {
		if targetAgentID == "agent-internal" {
			targetAgentType = "internal"
		} else {
			targetAgentType = "acp"
		}
	}

	targetModel := strings.TrimSpace(req.TargetModel)
	if targetModel == "" {
		if isACPAgentID(targetAgentID, targetAgentType) {
			targetModel = as.ResolveModelForAgent(targetAgentID, "")
		} else {
			as.mu.Lock()
			targetModel = as.ActiveModel
			as.mu.Unlock()
		}
	}

	wsPath := pkt.WorktreePath
	if wsPath == "" {
		as.mu.Lock()
		wsPath = as.workspacePath
		as.mu.Unlock()
	}

	// Optional isolated git worktree creation.
	if req.CreateWorktree && wsPath != "" && !strings.HasPrefix(wsPath, "ssh://") {
		branch := sanitizeWorktreeBranch(req.WorktreeBranch)
		if branch == "" {
			branch = fmt.Sprintf("handoff-%d", time.Now().Unix()%100000)
		}
		wtDir := filepath.Join(filepath.Dir(wsPath), filepath.Base(wsPath)+"-"+strings.ReplaceAll(branch, "/", "-"))
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		err := internalgit.CreateWorktree(ctx, wsPath, wtDir, branch, true)
		cancel()
		if err != nil {
			return nil, fmt.Errorf("create handoff worktree: %w", err)
		}
		wsPath = wtDir
		pkt.WorktreePath = wtDir
		pkt.Branch = branch
	}

	pkt.TargetAgent = displayAgentLabel(targetAgentID, targetAgentType, targetModel)
	pkt.TargetModel = targetModel
	prompt := FormatHandoffPrompt(pkt)

	now := time.Now()
	title := pkt.Goal
	if title == "" {
		title = "Handoff from " + pkt.SourceAgent
	}
	if len(title) > 46 {
		title = strings.TrimSpace(title[:46]) + "…"
	}

	userMsg := &ChatMessage{
		ID:        1,
		Role:      "user",
		Text:      prompt,
		Done:      true,
		Timestamp: now,
	}

	as.mu.Lock()
	id := as.nextSessionIDLocked()
	prov := as.ActiveProvider
	if prov == "" && as.engine != nil && as.engine.LLMClient != nil {
		prov = as.engine.LLMClient.GetConfig().ProviderID
	}

	newSess := &AgentSession{
		ID:               id,
		Title:            title,
		Status:           "idle",
		CreatedAt:        now,
		UpdatedAt:        now,
		Model:            targetModel,
		Provider:         prov,
		AgentID:          targetAgentID,
		AgentType:        targetAgentType,
		WorkspacePath:    wsPath,
		HandoffFromID:    req.SourceSessionID,
		HandoffFromAgent: pkt.SourceAgent,
		Messages:         []*ChatMessage{userMsg},
		Tasks:            NewTaskManager(),
		nextMsgID:        2,
	}

	// Link source session lineage & drop a concise handoff marker in source transcript.
	var srcSess *AgentSession
	for _, s := range as.Sessions {
		if s != nil && s.ID == req.SourceSessionID {
			srcSess = s
			s.HandoffToIDs = append(s.HandoffToIDs, id)
			s.UpdatedAt = now
			noteID := nextMessageIDLocked(s)
			s.Messages = append(s.Messages, &ChatMessage{
				ID:        noteID,
				Role:      "assistant",
				Text:      fmt.Sprintf("> **Handed off ↗** to **%s** (`%s`)\n> Goal: %s", pkt.TargetAgent, title, pkt.Goal),
				Done:      true,
				Timestamp: now,
			})
			break
		}
	}

	as.Sessions = append([]*AgentSession{newSess}, as.Sessions...)
	if !req.Background {
		as.ActiveSessionID = id
		as.ActiveAgentID = targetAgentID
		as.ActiveAgentType = targetAgentType
		if targetModel != "" {
			as.ActiveModel = targetModel
		}
		as.Messages = newSess.Messages
		as.TaskManager = newSess.Tasks
		as.SteerQueue = nil
		as.IsStreaming = false
	}
	as.mu.Unlock()

	as.SaveSessionToDisk(newSess)
	if srcSess != nil {
		as.SaveSessionToDisk(srcSess)
	}
	if req.AutoStart {
		as.TriggerTurnForSession(id, prompt)
	}
	if as.Invalidate != nil {
		as.Invalidate()
	}
	return newSess, nil
}

// HandoffsFromParent returns all non-archived sessions created via handoff
// from sourceSessionID, ordered oldest-first for stable sidebar nesting.
func (as *AgentState) HandoffsFromParent(sourceSessionID string) []*AgentSession {
	if as == nil || sourceSessionID == "" {
		return nil
	}
	as.mu.Lock()
	defer as.mu.Unlock()

	var out []*AgentSession
	for i := len(as.Sessions) - 1; i >= 0; i-- {
		s := as.Sessions[i]
		if s != nil && !s.Archived && s.HandoffFromID == sourceSessionID {
			out = append(out, s)
		}
	}
	return out
}

// HasSessionID reports whether a non-archived session with id exists.
func (as *AgentState) HasSessionID(id string) bool {
	if as == nil || id == "" {
		return false
	}
	as.mu.Lock()
	defer as.mu.Unlock()
	for _, s := range as.Sessions {
		if s != nil && !s.Archived && s.ID == id {
			return true
		}
	}
	return false
}

func displayAgentLabel(agentID, agentType, model string) string {
	if agentType == "internal" || strings.Contains(strings.ToLower(agentID), "internal") {
		if model != "" {
			return "ForgeADE Internal (" + model + ")"
		}
		return "ForgeADE Internal"
	}
	name := strings.TrimPrefix(agentID, "agent-")
	if name == "" {
		name = "ACP"
	}
	name = strings.ToUpper(name[:1]) + name[1:]
	if model != "" {
		return name + " (" + model + ")"
	}
	return name
}

func sanitizeWorktreeBranch(raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.Trim(s, "/-")
	return s
}
