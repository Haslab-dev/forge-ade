package agent

import (
	"strings"
	"testing"
	"time"
)

func TestHybridOrchestratorMixedInternalAndACPWorkers(t *testing.T) {
	as := &AgentState{
		workspacePath:   t.TempDir(),
		ActiveAgentID:   "agent-internal",
		ActiveAgentType: "internal",
		ActiveModel:     "gemini-2.5-pro",
		Sessions: []*AgentSession{
			{
				ID:            "coord-1",
				Title:         "Fix EU promo checkout",
				Status:        "running",
				CreatedAt:     time.Now(),
				UpdatedAt:     time.Now(),
				AgentID:       "agent-internal",
				AgentType:     "internal",
				WorkspacePath: "/tmp/acme",
				Tasks:         NewTaskManager(),
			},
		},
		ActiveSessionID: "coord-1",
	}

	// Spawn 1 ACP worker (Codex) and 1 Internal worker under the same coordinator.
	w1, err := as.SpawnWorker(WorkerSpec{
		ParentSessionID: "coord-1",
		Name:            "tax-rules",
		AgentID:         "agent-codex",
		AgentType:       "acp",
		Model:           "gpt-5-codex",
		Task:            "Apply promo discounts after regional VAT",
		WorkspaceMode:   "shared",
		AutoStart:       false,
	})
	if err != nil {
		t.Fatalf("SpawnWorker w1 failed: %v", err)
	}
	w2, err := as.SpawnWorker(WorkerSpec{
		ParentSessionID: "coord-1",
		Name:            "promo-tests",
		AgentID:         "agent-internal",
		AgentType:       "internal",
		Model:           "gemini-2.5-flash",
		Task:            "Add 8 regression tests for DE, FR, NL",
		WorkspaceMode:   "shared",
		AutoStart:       false,
	})
	if err != nil {
		t.Fatalf("SpawnWorker w2 failed: %v", err)
	}

	workers := as.WorkersForParent("coord-1")
	if len(workers) != 2 {
		t.Fatalf("expected 2 workers for coord-1, got %d", len(workers))
	}
	if workers[0].WorkerName != "tax-rules" || workers[1].WorkerName != "promo-tests" {
		t.Fatalf("unexpected worker ordering: %s, %s", workers[0].WorkerName, workers[1].WorkerName)
	}

	// Simulate worker completion & verify CollectWorker.
	as.mu.Lock()
	for _, s := range as.Sessions {
		if s.ID == w1.SessionID {
			s.Status = "completed"
			s.Messages = append(s.Messages, &ChatMessage{
				ID:   2,
				Role: "assistant",
				Text: "Fixed VAT ordering; 11 tax tests pass.",
				Done: true,
				Changes: []*FileChange{
					{Path: "pkg/tax/promo.go", Additions: 18, Deletions: 2},
				},
			})
		}
	}
	as.mu.Unlock()

	res, err := as.CollectWorker(w1.SessionID)
	if err != nil {
		t.Fatalf("CollectWorker failed: %v", err)
	}
	if res.Status != "completed" || res.Summary != "Fixed VAT ordering; 11 tax tests pass." {
		t.Fatalf("unexpected collect result: %+v", res)
	}
	if len(res.ChangedFiles) != 1 || res.ChangedFiles[0] != "pkg/tax/promo.go" {
		t.Fatalf("unexpected changed files: %v", res.ChangedFiles)
	}

	// Stop second worker cleanly.
	as.StopWorker(w2.SessionID)
}

func TestSwarmAliasInheritPersonaAndEligibility(t *testing.T) {
	dir := t.TempDir()
	acpMgr := &ACPManager{
		Agents: []ACPAgentDefinition{
			{ID: "agent-agy", Name: "Antigravity", Type: "acp", Command: "agy", Available: true},
			{ID: "agent-codex", Name: "Codex", Type: "acp", Command: "codex", Available: false},
		},
	}
	as := &AgentState{
		workspacePath: dir,
		ACPMgr:        acpMgr,
		Sessions: []*AgentSession{
			{
				ID:            "coord-agy",
				Title:         "Coordinator",
				Status:        "idle",
				AgentID:       "agent-agy",
				AgentType:     "acp",
				Model:         "gemini-4-hydrogen",
				WorkspacePath: dir,
				Tasks:         NewTaskManager(),
				CreatedAt:     time.Now(),
				UpdatedAt:     time.Now(),
			},
		},
		ActiveSessionID: "coord-agy",
	}

	// 1. Alias normalization
	if id, typ := NormalizeAgentAlias("agy"); id != "agent-agy" || typ != "acp" {
		t.Fatalf("expected (agent-agy, acp), got (%s, %s)", id, typ)
	}

	// 2. Inherit parent harness when AgentID is omitted + research persona
	w, err := as.SpawnWorker(WorkerSpec{
		ParentSessionID: "coord-agy",
		Name:            "research-auth",
		Persona:         "research",
		Task:            "Inspect auth handlers",
		AutoStart:       false,
	})
	if err != nil {
		t.Fatalf("SpawnWorker inherit failed: %v", err)
	}
	if w.AgentID != "agent-agy" || w.AgentType != "acp" {
		t.Fatalf("expected worker to inherit agent-agy/acp, got %s/%s", w.AgentID, w.AgentType)
	}

	// Verify persona instructions were injected into opening prompt
	sess := as.FindSession(w.SessionID)
	if sess == nil || len(sess.Messages) == 0 || !strings.Contains(sess.Messages[0].Text, "Read-only: do NOT edit") {
		t.Fatalf("expected research persona limitations in opening prompt")
	}

	// 3. Reject unavailable ACP engine with clear error
	if _, err := as.SpawnWorker(WorkerSpec{
		ParentSessionID: "coord-agy",
		Name:            "bad-worker",
		AgentID:         "codex",
		Task:            "Try unavailable engine",
		AutoStart:       false,
	}); err == nil || !strings.Contains(err.Error(), "agent-codex unavailable") || !strings.Contains(err.Error(), "agent-agy") {
		t.Fatalf("expected unavailability error listing agent-agy, got: %v", err)
	}
}
