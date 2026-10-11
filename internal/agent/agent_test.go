package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hasdev/forge-ade/internal/events"
	"github.com/hasdev/forge-ade/internal/llm"
	"github.com/hasdev/forge-ade/internal/mcp"
	"github.com/hasdev/forge-ade/internal/plugins"
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
	pluginMgr := plugins.NewManager(tempDir, bus)
	mcpMgr := mcp.NewManager(tempDir)

	mgr := NewManager(llmClient, toolReg, skillMgr, pluginMgr, mcpMgr, nil, bus, tempDir)

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
	pluginMgr := plugins.NewManager(tempDir, bus)
	mcpMgr := mcp.NewManager(tempDir)

	// 1. Create a session, verify it lands in sessions/<id>.json and the
	// legacy store file is gone.
	mgr := NewManager(llmClient, toolReg, skillMgr, pluginMgr, mcpMgr, nil, bus, tempDir)
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
	mgr2 := NewManager(llmClient, toolReg, skillMgr, pluginMgr, mcpMgr, nil, bus, tempDir)
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
	mgr3 := NewManager(llmClient, toolReg, skillMgr, pluginMgr, mcpMgr, nil, bus, tempDir)
	if _, ok := mgr3.GetSession(sess.ID); !ok {
		t.Fatalf("session not migrated from legacy store")
	}
	if _, err := os.Stat(filepath.Join(dir, sess.ID+".json")); err != nil {
		t.Fatalf("expected migrated per-session file: %v", err)
	}
}

func TestFormatSmartSessionTitle(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"fix navbar styling bug on mobile", "fix navbar styling bug on mobile"},
		{"/skill:refactor improve performance", "improve performance"},
		{"@src/App.tsx please optimize this component and make it fast", "please optimize this component and make"},
		{"", "New Task"},
		{"\n\n   \n", "New Task"},
		{"### Task: Fix the issue with login\nDetails below...", "Task Fix the issue with login"},
	}

	for _, c := range cases {
		got := formatSmartSessionTitle(c.input)
		if got != c.expected {
			t.Errorf("formatSmartSessionTitle(%q) = %q; want %q", c.input, got, c.expected)
		}
	}
}

func TestMessagesToLLMFilteringAndCorrelation(t *testing.T) {
	mgr := &Manager{}

	msgs := []AgentMessage{
		{
			Role: "user",
			Content: []ContentBlock{
				{Type: "text", Text: "Please do the task"},
			},
		},
		{
			// Empty assistant message (should be skipped)
			Role:    "assistant",
			Content: []ContentBlock{},
		},
		{
			// Assistant message with ask tool call
			Role: "assistant",
			Content: []ContentBlock{
				{Type: "tool_call", ToolCallID: "call_ask_123", Name: "ask", Arguments: map[string]any{"question": "which one?"}},
			},
		},
		{
			// Tool result matching ask
			Role: "tool",
			Content: []ContentBlock{
				{Type: "tool_result", ToolCallID: "call_ask_123", Name: "ask", Text: "user chose A"},
			},
		},
		{
			// Orphan tool result (no preceding tool call with this ID)
			Role: "tool",
			Content: []ContentBlock{
				{Type: "tool_result", ToolCallID: "call_unknown", Name: "custom_tool", Text: "orphan result"},
			},
		},
	}

	llmMsgs := mgr.messagesToLLM(msgs)

	// Expected:
	// 1: User message "Please do the task"
	// 2: Assistant message with tool_call "call_ask_123"
	// 3: Tool message with ToolCallID "call_ask_123"
	// 4: Orphan result converted to RoleUser (not RoleTool)
	if len(llmMsgs) != 4 {
		t.Fatalf("expected 4 LLM messages, got %d: %+v", len(llmMsgs), llmMsgs)
	}

	if llmMsgs[0].Role != llm.RoleUser || llmMsgs[0].Content != "Please do the task" {
		t.Errorf("unexpected msg 0: %+v", llmMsgs[0])
	}
	if llmMsgs[1].Role != llm.RoleAssistant || len(llmMsgs[1].ToolCalls) != 1 || llmMsgs[1].ToolCalls[0].ID != "call_ask_123" {
		t.Errorf("unexpected msg 1: %+v", llmMsgs[1])
	}
	if llmMsgs[2].Role != llm.RoleTool || llmMsgs[2].ToolCallID != "call_ask_123" {
		t.Errorf("unexpected msg 2: %+v", llmMsgs[2])
	}
	// Orphan tool result safely rendered as RoleUser
	if llmMsgs[3].Role != llm.RoleUser {
		t.Errorf("expected orphan tool result converted to RoleUser, got %+v", llmMsgs[3])
	}
}

func TestRespondAskUpdatesPendingResult(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "forge-respond-ask-*")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	bus := events.NewBus()
	llmClient := llm.NewLLMClient(tempDir)
	searchMgr := search.NewSearchManager()
	toolReg := tools.NewRegistry(searchMgr)
	skillMgr := skills.NewManager()
	pluginMgr := plugins.NewManager(tempDir, bus)
	mcpMgr := mcp.NewManager(tempDir)

	mgr := NewManager(llmClient, toolReg, skillMgr, pluginMgr, mcpMgr, nil, bus, tempDir)
	sess, err := mgr.CreateSession("Ask Session", RoleCoding, tempDir)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	// Setup pending ask state
	mgr.mu.Lock()
	sess.State = StateAwaitingInput
	sess.PendingQuestions = []tools.AskQuestion{
		{ID: "choice", Question: "Pick one", Options: []string{"A", "B"}},
	}
	sess.Messages = append(sess.Messages,
		AgentMessage{
			Role: "assistant",
			Content: []ContentBlock{
				{Type: "tool_call", ToolCallID: "call_ask_999", Name: "ask"},
			},
		},
		AgentMessage{
			Role: "tool",
			Content: []ContentBlock{
				{Type: "tool_result", ToolCallID: "call_ask_999", Name: "ask", Text: `{"status":"awaiting_input"}`},
			},
		},
	)
	mgr.mu.Unlock()

	// Call RespondAsk
	answers := map[string]any{"choice": "A"}
	if err := mgr.RespondAsk(sess.ID, answers); err != nil {
		t.Fatalf("RespondAsk failed: %v", err)
	}

	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	// Verify questions cleared and state updated
	if len(sess.PendingQuestions) != 0 {
		t.Errorf("expected pending questions cleared, got %v", sess.PendingQuestions)
	}
	// Verify tool result was updated with answers and preserved ToolCallID
	var askResult *ContentBlock
	for _, m := range sess.Messages {
		if m.Role == "tool" {
			for _, cb := range m.Content {
				if cb.Name == "ask" {
					askResult = &cb
					break
				}
			}
		}
	}
	if askResult == nil {
		t.Fatalf("ask tool result not found in messages")
	}
	if askResult.ToolCallID != "call_ask_999" {
		t.Errorf("expected ToolCallID 'call_ask_999', got %q", askResult.ToolCallID)
	}
	if !strings.Contains(askResult.Text, `"choice": "A"`) {
		t.Errorf("expected answers in tool result text, got %q", askResult.Text)
	}
}
