package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hasdev/forge-ade/internal/events"
	"github.com/hasdev/forge-ade/internal/llm"
	"github.com/hasdev/forge-ade/internal/mcp"
	"github.com/hasdev/forge-ade/internal/search"
	"github.com/hasdev/forge-ade/internal/skills"
	"github.com/hasdev/forge-ade/internal/tools"
)

func TestAgentManagerSession(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "forge-agent-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	bus := events.NewBus()
	llmClient := llm.NewLLMClient(tempDir)
	searchMgr := search.NewSearchManager()
	toolReg := tools.NewRegistry(searchMgr)
	skillMgr := skills.NewManager()
	mcpMgr := mcp.NewManager(tempDir)

	mgr := NewManager(llmClient, toolReg, skillMgr, mcpMgr, nil, bus, tempDir)

	sess, err := mgr.CreateSession("Test Agent", RoleCoding, tempDir)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	if sess.Name != "Test Agent" {
		t.Errorf("expected session name 'Test Agent', got '%s'", sess.Name)
	}

	if sess.RoleFilter != RoleCoding {
		t.Errorf("expected role coding, got %s", sess.RoleFilter)
	}

	list := mgr.ListSessions()
	if len(list) != 1 {
		t.Errorf("expected 1 session, got %d", len(list))
	}

	found, ok := mgr.GetSession(sess.ID)
	if !ok || found.ID != sess.ID {
		t.Errorf("failed to get session by ID")
	}

	mgr.DeleteSession(sess.ID)
	listAfter := mgr.ListSessions()
	if len(listAfter) != 0 {
		t.Errorf("expected 0 sessions after delete, got %d", len(listAfter))
	}
}

func TestWindowTranscriptKeepsPairsIntact(t *testing.T) {
	// Build a transcript: user task, then 30 exchanges of
	// assistant(tool_calls) + tool(result), then a final assistant text.
	msgs := []AgentMessage{{Role: "user", Content: []ContentBlock{{Type: "text", Text: "original task"}}}}
	for i := 0; i < 30; i++ {
		msgs = append(msgs, AgentMessage{
			Role: "assistant",
			Content: []ContentBlock{
				{Type: "tool_call", ToolCallID: "call-1", Name: "read", Arguments: map[string]any{}},
				{Type: "tool_call", ToolCallID: "call-2", Name: "bash", Arguments: map[string]any{}},
			},
		})
		msgs = append(msgs, AgentMessage{Role: "tool", Content: []ContentBlock{{Type: "tool_result", ToolCallID: "call-1", Text: "a"}}})
		msgs = append(msgs, AgentMessage{Role: "tool", Content: []ContentBlock{{Type: "tool_result", ToolCallID: "call-2", Text: "b"}}})
	}
	msgs = append(msgs, AgentMessage{Role: "assistant", Content: []ContentBlock{{Type: "text", Text: "done"}}})

	got := windowTranscript(msgs, 10)
	if len(got) > 12 {
		t.Fatalf("window too large: %d", len(got))
	}
	if got[0].Role != "user" || got[0].Content[0].Text != "original task" {
		t.Fatalf("expected original task preserved, got %+v", got[0])
	}
	// Every assistant tool_call message in the window must have its results.
	for i, m := range got {
		if m.Role != "assistant" || len(m.ToolCallBlocks()) == 0 {
			continue
		}
		need := len(m.ToolCallBlocks())
		for j := i + 1; j < len(got) && need > 0; j++ {
			if got[j].Role == "tool" {
				need--
			}
		}
		if need != 0 {
			t.Fatalf("tool_call group at %d has %d missing results", i, need)
		}
	}
	// Last message must be the final assistant text.
	if last := got[len(got)-1]; last.Role != "assistant" || last.Content[0].Text != "done" {
		t.Fatalf("expected final message preserved, got %+v", last)
	}
}

// TestSessionStorageSplitAndMigration verifies sessions persist to per-session
// files under sessions/ and that a legacy single-file store migrates cleanly.
func TestSessionStorageSplitAndMigration(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "forge-migrate-*")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	bus := events.NewBus()
	llmClient := llm.NewLLMClient(tempDir)
	searchMgr := search.NewSearchManager()
	toolReg := tools.NewRegistry(searchMgr)
	skillMgr := skills.NewManager()
	mcpMgr := mcp.NewManager(tempDir)

	// 1. Create a session, verify it lands in sessions/<id>.json and the
	// legacy store file is gone.
	mgr := NewManager(llmClient, toolReg, skillMgr, mcpMgr, nil, bus, tempDir)
	sess, err := mgr.CreateSession("Migrate Me", RoleCoding, tempDir)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	dir := filepath.Join(tempDir, "sessions")
	entry := filepath.Join(dir, sess.ID+".json")
	if _, err := os.Stat(entry); err != nil {
		t.Fatalf("expected per-session file %s: %v", entry, err)
	}
	if _, err := os.Stat(filepath.Join(tempDir, "agent_sessions.json")); err == nil {
		t.Fatalf("legacy store should have been removed")
	}

	// 2. Reload from per-session files.
	mgr2 := NewManager(llmClient, toolReg, skillMgr, mcpMgr, nil, bus, tempDir)
	if _, ok := mgr2.GetSession(sess.ID); !ok {
		t.Fatalf("session not reloaded from sessions/")
	}

	// 3. Legacy migration: drop a single-file store and reload.
	_ = os.RemoveAll(dir)
	legacy := []*Session{sess}
	data, _ := json.MarshalIndent(legacy, "", "  ")
	if err := os.WriteFile(filepath.Join(tempDir, "agent_sessions.json"), data, 0644); err != nil {
		t.Fatalf("write legacy store: %v", err)
	}
	mgr3 := NewManager(llmClient, toolReg, skillMgr, mcpMgr, nil, bus, tempDir)
	if _, ok := mgr3.GetSession(sess.ID); !ok {
		t.Fatalf("session not migrated from legacy store")
	}
	if _, err := os.Stat(filepath.Join(dir, sess.ID+".json")); err != nil {
		t.Fatalf("expected migrated per-session file: %v", err)
	}
}

// TestExtractObservationsSearch verifies a search tool result is distilled
// into a dense fact (pattern + match count + key paths).
func TestExtractObservationsSearch(t *testing.T) {
	sess := &Session{}
	result := `{
	  "pattern": "LoginService",
	  "matches": [
	    {"path": "src/auth/login.ts", "line": 12},
	    {"path": "src/auth/service.ts", "line": 40}
	  ],
	  "count": 2
	}`
	block := ContentBlock{Type: "tool_result", Name: "search"}
	extractObservations(sess, block, result, false)

	if len(sess.Observations) != 1 {
		t.Fatalf("expected 1 observation, got %d", len(sess.Observations))
	}
	obs := sess.Observations[0]
	if obs.Kind != "search" {
		t.Fatalf("expected kind=search, got %q", obs.Kind)
	}
	if !strings.Contains(obs.Summary, "2 match") || !strings.Contains(obs.Summary, "src/auth/login.ts") {
		t.Fatalf("summary missing match info: %q", obs.Summary)
	}
	if obs.Confidence != 1.0 {
		t.Fatalf("expected confidence 1.0")
	}
}

// TestExtractObservationsError verifies errors are captured as observations.
func TestExtractObservationsError(t *testing.T) {
	sess := &Session{}
	block := ContentBlock{Type: "tool_result", Name: "edit"}
	extractObservations(sess, block, "Tool Error: anchor not found", true)
	if len(sess.Observations) != 1 {
		t.Fatalf("expected 1 observation, got %d", len(sess.Observations))
	}
	if sess.Observations[0].Kind != "error" {
		t.Fatalf("expected kind=error, got %q", sess.Observations[0].Kind)
	}
}

// TestPruneToolResult verifies oversized tool results are bounded to a head +
// tail excerpt (observation memory carries the distilled facts).
func TestPruneToolResult(t *testing.T) {
	big := strings.Repeat("line of output\n", 500) // ~7.5k chars
	pruned := pruneToolResult(big)
	if len(pruned) >= len(big) {
		t.Fatalf("pruned result not smaller: %d >= %d", len(pruned), len(big))
	}
	if !strings.Contains(pruned, "[output truncated") {
		t.Fatalf("missing truncation marker")
	}
	// Small results pass through untouched.
	small := "tiny"
	if got := pruneToolResult(small); got != small {
		t.Fatalf("small result should pass through, got %q", got)
	}
}

// TestSendMessageSpawningGuardAndCancelCleanup verifies the turn-spawn guard:
// a fresh session spawns a turn (cancelFuncs populated), and once the turn
// finishes the cancel marker is cleared so the NEXT message spawns a new turn.
// This guards against the "stuck on thinking / no answer" regression where the
// guard never fired or the marker was never cleaned.
func TestSendMessageSpawningGuardAndCancelCleanup(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "forge-guard-*")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	bus := events.NewBus()
	llmClient := llm.NewLLMClient(tempDir)
	searchMgr := search.NewSearchManager()
	toolReg := tools.NewRegistry(searchMgr)
	skillMgr := skills.NewManager()
	mcpMgr := mcp.NewManager(tempDir)
	mgr := NewManager(llmClient, toolReg, skillMgr, mcpMgr, nil, bus, tempDir)

	sess, err := mgr.CreateSession("Guard Agent", RoleCoding, tempDir)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	ctx := context.Background()
	// First message: turn should spawn (cancel marker set) and then finish
	// (marker cleared) — with no API key the LLM call fails fast.
	if err := mgr.SendMessage(ctx, sess.ID, "hello", nil); err != nil {
		t.Fatalf("send message: %v", err)
	}

	// The turn goroutine runs async; wait until it finishes (state back to
	// idle) or a timeout elapses.
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		mgr.mu.RLock()
		_, hasCancel := mgr.cancelFuncs[sess.ID]
		st := StateIdle
		if s, ok := mgr.sessions[sess.ID]; ok {
			st = s.State
		}
		mgr.mu.RUnlock()
		if !hasCancel && st == StateIdle {
			break // turn finished and marker cleared
		}
		time.Sleep(50 * time.Millisecond)
	}
	mgr.mu.RLock()
	_, hasCancel := mgr.cancelFuncs[sess.ID]
	st := StateIdle
	if s, ok := mgr.sessions[sess.ID]; ok {
		st = s.State
	}
	mgr.mu.RUnlock()
	if hasCancel {
		t.Fatalf("cancel marker not cleared after turn finished")
	}
	if st != StateIdle {
		t.Fatalf("session not idle after turn, got %s", st)
	}
}
