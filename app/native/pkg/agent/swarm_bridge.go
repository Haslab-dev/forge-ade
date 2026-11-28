package agent

import (
	"fmt"
	"strings"

	"github.com/hasdev/forge-ade/core/forgemcp"
)

// NewSwarmBridge returns a forgemcp.SwarmBridge backed by AgentState and BotStore.
func (as *AgentState) NewSwarmBridge(bots *BotStore) forgemcp.SwarmBridge {
	return &agentSwarmBridge{as: as, bots: bots}
}

type agentSwarmBridge struct {
	as   *AgentState
	bots *BotStore
}

func (b *agentSwarmBridge) ListAgents() ([]forgemcp.SwarmAgentInfo, []forgemcp.SwarmPersonaInfo) {
	if b.as == nil {
		return nil, nil
	}
	activeID := ""
	if s := b.as.ActiveSession(); s != nil {
		activeID = s.AgentID
	} else {
		b.as.mu.Lock()
		activeID = b.as.ActiveAgentID
		b.as.mu.Unlock()
	}

	statuses := b.as.SwarmEngineStatuses()
	agents := make([]forgemcp.SwarmAgentInfo, 0, len(statuses))
	for _, st := range statuses {
		agents = append(agents, forgemcp.SwarmAgentInfo{
			ID:        st.ID,
			Name:      st.Name,
			Type:      st.Type,
			Eligible:  st.Eligible,
			Reason:    st.Reason,
			IsDefault: st.ID == activeID,
		})
	}

	var personas []forgemcp.SwarmPersonaInfo
	if b.as.engine != nil && b.as.engine.AgentMgr != nil {
		for _, def := range b.as.engine.AgentMgr.ListAgentDefinitions() {
			lim := def.Rules
			if def.ReadOnly {
				if lim != "" {
					lim += "; "
				}
				lim += "read-only"
			}
			personas = append(personas, forgemcp.SwarmPersonaInfo{
				ID:          def.ID,
				Name:        def.Name,
				Role:        string(def.Role),
				Limitations: lim,
				Source:      "orchestration",
			})
		}
	}
	if len(personas) == 0 {
		for _, p := range builtinPersonaPresets {
			personas = append(personas, forgemcp.SwarmPersonaInfo{
				ID:          p.ID,
				Name:        p.Name,
				Role:        p.Role,
				Limitations: p.Limitations,
				Source:      "builtin",
			})
		}
	}
	botStore := b.bots
	if botStore == nil {
		botStore = b.as.Bots
	}
	if botStore != nil {
		for _, bot := range botStore.List() {
			if bot == nil {
				continue
			}
			personas = append(personas, forgemcp.SwarmPersonaInfo{
				ID:          bot.ID,
				Name:        bot.Name,
				Role:        bot.Role,
				Limitations: bot.Limitations,
				Source:      "bot",
			})
		}
	}
	return agents, personas
}

func (b *agentSwarmBridge) SpawnWorker(req forgemcp.SwarmSpawnRequest) (*forgemcp.SwarmWorkerInfo, error) {
	if b.as == nil {
		return nil, fmt.Errorf("agent state unavailable")
	}
	parentID := strings.TrimSpace(req.ParentSessionID)
	if parentID == "" {
		parentID = b.as.SnapshotActiveSessionID()
	}
	rec, err := b.as.SpawnWorker(WorkerSpec{
		ParentSessionID: parentID,
		Name:            req.Name,
		Persona:         req.Persona,
		SystemPrompt:    req.SystemPrompt,
		Limitations:     req.Limitations,
		AgentID:         req.AgentID,
		Model:           req.Model,
		Task:            req.Task,
		WorkspaceMode:   req.WorkspaceMode,
		WorktreeBranch:  req.WorktreeBranch,
		AutoStart:       true,
	})
	if err != nil {
		return nil, err
	}
	return &forgemcp.SwarmWorkerInfo{
		SessionID:     rec.SessionID,
		Name:          rec.Name,
		AgentID:       rec.AgentID,
		AgentType:     rec.AgentType,
		Model:         rec.Model,
		WorkspacePath: rec.WorkspacePath,
		Branch:        rec.Branch,
		Status:        rec.Status,
	}, nil
}

func (b *agentSwarmBridge) ListWorkers(parentSessionID string) []forgemcp.SwarmWorkerInfo {
	if b.as == nil {
		return nil
	}
	pid := strings.TrimSpace(parentSessionID)
	if pid == "" {
		pid = b.as.SnapshotActiveSessionID()
	}
	sessions := b.as.WorkersForParent(pid)
	out := make([]forgemcp.SwarmWorkerInfo, 0, len(sessions))
	for _, s := range sessions {
		name := s.WorkerName
		if name == "" {
			name = s.Title
		}
		out = append(out, forgemcp.SwarmWorkerInfo{
			SessionID:     s.ID,
			Name:          name,
			AgentID:       s.AgentID,
			AgentType:     s.AgentType,
			Model:         s.Model,
			WorkspacePath: s.WorkspacePath,
			Status:        s.Status,
		})
	}
	return out
}

func (b *agentSwarmBridge) MessageWorker(workerSessionID, instruction string) error {
	if b.as == nil {
		return fmt.Errorf("agent state unavailable")
	}
	return b.as.MessageWorker(workerSessionID, instruction)
}

func (b *agentSwarmBridge) CollectWorker(workerSessionID string) (*forgemcp.SwarmWorkerResult, error) {
	if b.as == nil {
		return nil, fmt.Errorf("agent state unavailable")
	}
	res, err := b.as.CollectWorker(workerSessionID)
	if err != nil {
		return nil, err
	}
	return &forgemcp.SwarmWorkerResult{
		SessionID:    res.SessionID,
		Name:         res.Name,
		AgentLabel:   res.AgentLabel,
		Status:       res.Status,
		Summary:      res.Summary,
		ChangedFiles: res.ChangedFiles,
		DurationMs:   res.Duration.Milliseconds(),
	}, nil
}

func (b *agentSwarmBridge) HandoffSession(req forgemcp.SwarmHandoffRequest) (*forgemcp.SwarmWorkerInfo, error) {
	if b.as == nil {
		return nil, fmt.Errorf("agent state unavailable")
	}
	srcID := strings.TrimSpace(req.SourceSessionID)
	if srcID == "" {
		srcID = b.as.SnapshotActiveSessionID()
	}
	targetID, targetType := NormalizeAgentAlias(req.TargetAgentID)
	if targetID == "" {
		targetID, targetType, _ = b.as.resolveWorkerEngine(srcID, "", "", req.TargetModel)
	}
	if err := b.as.validateSwarmEngine(targetID); err != nil {
		return nil, err
	}
	sess, err := b.as.ExecuteHandoff(HandoffRequest{
		SourceSessionID: srcID,
		TargetAgentID:   targetID,
		TargetAgentType: targetType,
		TargetModel:     req.TargetModel,
		Goal:            req.Goal,
		NextSteps:       req.NextSteps,
		CreateWorktree:  req.CreateWorktree,
		WorktreeBranch:  req.WorktreeBranch,
		AutoStart:       true,
		Background:      req.Background,
	})
	if err != nil {
		return nil, err
	}
	return &forgemcp.SwarmWorkerInfo{
		SessionID:     sess.ID,
		Name:          sess.Title,
		AgentID:       sess.AgentID,
		AgentType:     sess.AgentType,
		Model:         sess.Model,
		WorkspacePath: sess.WorkspacePath,
		Status:        sess.Status,
	}, nil
}
