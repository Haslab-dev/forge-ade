package agent

import (
	"strings"
	"testing"
	"time"
)

func TestBuildAndExecuteHandoff(t *testing.T) {
	as := &AgentState{
		workspacePath:   t.TempDir(),
		ActiveAgentID:   "agent-internal",
		ActiveAgentType: "internal",
		ActiveModel:     "gemini-2.5-pro",
		Sessions: []*AgentSession{
			{
				ID:            "sess-src",
				Title:         "Implement checkout VAT rules",
				Status:        "completed",
				CreatedAt:     time.Now().Add(-10 * time.Minute),
				UpdatedAt:     time.Now(),
				AgentID:       "agent-internal",
				AgentType:     "internal",
				Model:         "gemini-2.5-pro",
				WorkspacePath: "/tmp/demo-ws",
				Messages: []*ChatMessage{
					{ID: 1, Role: "user", Text: "Add EU VAT discount ordering", Done: true},
					{
						ID:   2,
						Role: "assistant",
						Text: "Updated tax calculator and verified unit tests.",
						Done: true,
						Changes: []*FileChange{
							{Path: "pkg/tax/vat.go", Additions: 24, Deletions: 4},
						},
					},
				},
			},
		},
		ActiveSessionID: "sess-src",
	}

	pkt := as.BuildHandoffPacket("sess-src", "", "Add regression tests for DE and FR.")
	if pkt.Goal != "Implement checkout VAT rules" {
		t.Fatalf("expected goal from source title, got %q", pkt.Goal)
	}
	if len(pkt.ChangedFiles) != 1 || pkt.ChangedFiles[0] != "pkg/tax/vat.go" {
		t.Fatalf("expected changed file pkg/tax/vat.go, got %v", pkt.ChangedFiles)
	}
	prompt := FormatHandoffPrompt(pkt)
	if !strings.Contains(prompt, "pkg/tax/vat.go") || !strings.Contains(prompt, "DE and FR") {
		t.Fatalf("formatted prompt missing expected fields:\n%s", prompt)
	}

	// Execute handoff from Internal -> ACP Codex (AutoStart=false for unit test).
	target, err := as.ExecuteHandoff(HandoffRequest{
		SourceSessionID: "sess-src",
		TargetAgentID:   "agent-codex",
		TargetAgentType: "acp",
		TargetModel:     "gpt-5-codex",
		Goal:            "Finish EU VAT regression tests",
		NextSteps:       "Run go test ./... and open PR.",
		AutoStart:       false,
	})
	if err != nil {
		t.Fatalf("ExecuteHandoff failed: %v", err)
	}
	if target.AgentID != "agent-codex" || target.AgentType != "acp" {
		t.Fatalf("unexpected target agent: %s/%s", target.AgentID, target.AgentType)
	}
	if target.HandoffFromID != "sess-src" {
		t.Fatalf("expected HandoffFromID=sess-src, got %q", target.HandoffFromID)
	}
	if as.ActiveSessionID != target.ID {
		t.Fatalf("expected ActiveSessionID to switch to %s, got %s", target.ID, as.ActiveSessionID)
	}

	// Verify reverse lineage & marker appended to source session.
	var src *AgentSession
	for _, s := range as.Sessions {
		if s.ID == "sess-src" {
			src = s
			break
		}
	}
	if src == nil || len(src.HandoffToIDs) != 1 || src.HandoffToIDs[0] != target.ID {
		t.Fatalf("source session missing HandoffToIDs link: %+v", src)
	}
	lastMsg := src.Messages[len(src.Messages)-1]
	if !strings.Contains(lastMsg.Text, "Handed off") {
		t.Fatalf("expected handoff marker in source session, got %q", lastMsg.Text)
	}
}
