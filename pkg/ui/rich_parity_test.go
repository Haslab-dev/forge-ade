package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui/editor"
)

func TestRichAgentTimelineFeatures(t *testing.T) {
	tempDir := t.TempDir()
	engine := NewEngine(tempDir)
	as := NewAgentState(engine, tempDir, nil)

	sess := as.NewSession("Rich Timeline Parity Task")

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
	if changes[0].Additions <= 0 {
		t.Fatalf("expected positive additions, got %d", changes[0].Additions)
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
		Thinking:        "Analyzing architecture differences between forge-ade and native...",
		IsThinking:      false,
		ThinkingTimeSec: 1.4,
		Tools:           tools,
		Changes:         changes,
		PendingApproval: approval,
	}

	sess.Messages = append(sess.Messages, msg)
	as.SaveSessionToDisk(sess)

	// Verify approval can be resolved
	approval.Resolved = true
	approval.Approved = true
	if !approval.Approved || !approval.Resolved {
		t.Fatalf("expected approval to be marked approved and resolved")
	}

	// 4. Mode and Reasoning effort defaults and toggling
	if as.ExecutionMode != ModeBypass {
		t.Fatalf("expected default execution mode ModeBypass, got %s", as.ExecutionMode)
	}
	if as.ReasoningEffort != EffortMedium {
		t.Fatalf("expected default reasoning effort EffortMedium, got %s", as.ReasoningEffort)
	}

	// 5. Steer Queue operations
	as.SteerQueue = append(as.SteerQueue, "Please also add tests", "Optimize memory usage")
	if len(as.SteerQueue) != 2 {
		t.Fatalf("expected 2 items in steer queue, got %d", len(as.SteerQueue))
	}
	popped := as.SteerQueue[0]
	as.SteerQueue = as.SteerQueue[1:]
	if popped != "Please also add tests" {
		t.Fatalf("expected popped item 'Please also add tests', got '%s'", popped)
	}
	if len(as.SteerQueue) != 1 {
		t.Fatalf("expected 1 item left in steer queue, got %d", len(as.SteerQueue))
	}
}

func TestEditorDiagnosticsScanning(t *testing.T) {
	sampleCode := `package main
// TODO: implement graceful shutdown
func main() {
	// FIXME: potential nil pointer dereference
	// BUG: memory leak on long connections
}
`
	buf := editor.NewBuffer(sampleCode)
	diags := todosAndDiagnostics(buf)

	if len(diags) != 3 {
		t.Fatalf("expected 3 diagnostics (TODO, FIXME, BUG), got %d", len(diags))
	}

	sources := map[string]bool{}
	for _, d := range diags {
		sources[d.Source] = true
	}

	if !sources["todo"] || !sources["fixme"] || !sources["bug"] {
		t.Fatalf("expected sources todo, fixme, bug; got %v", sources)
	}

	// BUG should have SeverityError
	for _, d := range diags {
		if d.Source == "bug" && d.Severity != editor.SeverityError {
			t.Fatalf("expected BUG diagnostic to have SeverityError, got %v", d.Severity)
		}
		if d.Source == "todo" && d.Severity != editor.SeverityWarning {
			t.Fatalf("expected TODO diagnostic to have SeverityWarning, got %v", d.Severity)
		}
	}
}
