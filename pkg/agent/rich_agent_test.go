package agent

import (
	"strings"
	"testing"
	"time"

	"github.com/hasdev/forge-ade/forge-ade-native/pkg/engine"
	"github.com/hasdev/forge-ade/forge-ade-native/pkg/icons"
)

func TestRichAgentTimelineFeatures(t *testing.T) {
	tempDir := t.TempDir()
	eng := engine.NewEngine(tempDir)
	as := NewAgentState(eng, tempDir, nil)

	_ = as.NewSession("Rich Timeline Parity Task")

	// 1. Tool execution and summarization
	tools := []*ToolExecution{
		{ID: "t1", ToolName: "run_command", Command: "go test ./...", Status: "completed", DurationMs: 340},
		{ID: "t2", ToolName: "read_file", Command: `{"path": "pkg/ui/app.go"}`, Status: "completed", DurationMs: 45},
		{ID: "t3", ToolName: "replace_file_content", Command: `{"path": "pkg/ui/agent.go"}`, Status: "completed", DurationMs: 120},
		{ID: "t4", ToolName: "search_web", Command: `{"query": "mygo api"}`, Status: "completed", DurationMs: 820},
	}

	summary := summarizeTools(tools)
	if !strings.Contains(summary, "1 command") || !strings.Contains(summary, "1 file") || !strings.Contains(summary, "1 query") {
		t.Fatalf("unexpected summary format: %s", summary)
	}

	// 2. Turn changes extraction
	changes := extractChangesFromTools(tools)
	if len(changes) == 0 {
		t.Fatalf("expected at least 1 turn change for replace_file_content tool")
	}
	if changes[0].Path != "pkg/ui/agent.go" {
		t.Fatalf("expected path 'pkg/ui/agent.go', got '%s'", changes[0].Path)
	}
	if changes[0].Additions != -1 || changes[0].Deletions != -1 {
		t.Fatalf("expected pending diff counts (-1), got +%d/-%d", changes[0].Additions, changes[0].Deletions)
	}

	// 3. Pending approval resolution
	approval := &PendingApproval{
		ToolCallID: "call-123",
		ToolName:   "run_command",
		Summary:    "Run bash command 'git push origin main'",
		Arguments:  "git push origin main",
		Resolved:   false,
		Approved:   false,
	}

	msg := &ChatMessage{
		ID:              2,
		Role:            "assistant",
		Text:            "I have updated the codebase.",
		Done:            true,
		Timestamp:       time.Now(),
		Tools:           tools,
		Changes:         changes,
		PendingApproval: approval,
	}

	if msg.PendingApproval.Resolved {
		t.Fatalf("expected approval to be unresolved initially")
	}

	approval.Resolved = true
	approval.Approved = true
	if !approval.Resolved || !approval.Approved {
		t.Fatalf("expected approval to be marked resolved and approved")
	}
}

func TestAgentStateExecutionModeAndEffortCycles(t *testing.T) {
	tempDir := t.TempDir()
	as := NewAgentState(nil, tempDir, nil)

	as.ExecutionMode = ModeAsk
	if as.ExecutionModeLabel() != "Ask" {
		t.Errorf("Expected 'Ask', got '%s'", as.ExecutionModeLabel())
	}
	if as.ExecutionModeIcon() != icons.TablerHand {
		t.Errorf("Expected TablerHand icon for ModeAsk")
	}

	as.CycleExecutionMode()
	if as.ExecutionMode != ModeCode {
		t.Errorf("Expected ModeCode after cycle from ModeAsk, got %v", as.ExecutionMode)
	}

	as.CycleExecutionMode()
	if as.ExecutionMode != ModePlan {
		t.Errorf("Expected ModePlan, got %v", as.ExecutionMode)
	}

	as.CycleExecutionMode()
	if as.ExecutionMode != ModeBypass {
		t.Errorf("Expected ModeBypass, got %v", as.ExecutionMode)
	}

	as.CycleExecutionMode()
	if as.ExecutionMode != ModeAsk {
		t.Errorf("Expected wrap to ModeAsk, got %v", as.ExecutionMode)
	}

	// Test ReasoningEffort cycle
	as.ReasoningEffort = EffortOff
	as.CycleReasoningEffort()
	if as.ReasoningEffort != EffortLow {
		t.Errorf("Expected EffortLow, got %v", as.ReasoningEffort)
	}
}
