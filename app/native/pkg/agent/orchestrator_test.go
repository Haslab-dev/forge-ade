package agent

import (
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
