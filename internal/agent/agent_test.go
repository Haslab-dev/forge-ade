package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func TestTruncateAndOffloadToolOutput(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "forge-offload-test-*")
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

	sess, err := mgr.CreateSession("Test Offload", RoleCoding, tempDir)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	// Short output should NOT be truncated
	shortContent := "hello world"
	res := mgr.truncateAndOffloadToolOutput(sess.ID, "call_1", "read", shortContent)
	if res != shortContent {
		t.Errorf("expected short content unchanged, got %q", res)
	}

	// Huge line output (> 2000 lines) should be offloaded to disk
	var sb strings.Builder
	for i := 0; i < 2100; i++ {
		sb.WriteString("line item content goes here for testing\n")
	}
	hugeContent := sb.String()
	truncated := mgr.truncateAndOffloadToolOutput(sess.ID, "call_2", "bash", hugeContent)
	if !strings.Contains(truncated, "saved to") || !strings.Contains(truncated, "lines omitted") {
		t.Errorf("expected truncation notice in %q", truncated)
	}

	// Verify file was written to disk
	outDir := filepath.Join(tempDir, "sessions", sess.ID, "tool_outputs")
	entries, err := os.ReadDir(outDir)
	if err != nil || len(entries) == 0 {
		t.Fatalf("expected tool_outputs directory with offloaded file, err: %v", err)
	}
}

func TestWindowTranscriptWithSummary(t *testing.T) {
	msgs := []AgentMessage{
		{
			ID:   "m1",
			Role: "user",
			Content: []ContentBlock{
				{Type: "text", Text: "Please refactor the database connector and check config.json"},
			},
		},
		{
			ID:   "m2",
			Role: "assistant",
			Content: []ContentBlock{
				{Type: "tool_use", Name: "read", Arguments: map[string]any{"path": "config.json"}},
			},
		},
		{
			ID:   "m3",
			Role: "tool",
			Content: []ContentBlock{
				{Type: "tool_result", Text: "{}"},
			},
		},
	}
	// Append 45 dummy turns
	for i := 0; i < 45; i++ {
		msgs = append(msgs, AgentMessage{
			ID:   string(rune('a' + (i % 26))),
			Role: "assistant",
			Content: []ContentBlock{
				{Type: "text", Text: "progressing step"},
			},
		})
	}

	windowed, summary := windowTranscriptWithSummary(msgs, 20)
	if len(windowed) >= len(msgs) {
		t.Errorf("expected windowed to be shorter than original messages")
	}
	if !strings.Contains(summary, "<CONTEXT_SUMMARY>") {
		t.Errorf("expected <CONTEXT_SUMMARY> in summary, got: %s", summary)
	}
	if !strings.Contains(summary, "Please refactor the database") {
		t.Errorf("expected earlier user request in summary, got: %s", summary)
	}
	if !strings.Contains(summary, "config.json") {
		t.Errorf("expected touched file config.json in summary, got: %s", summary)
	}
}

func TestSubagentNotification(t *testing.T) {
	mgr := &Manager{
		subagentNotes: make(map[string][]SubagentNotification),
	}
	mgr.subagentNotes["parent-1"] = []SubagentNotification{
		{
			ID:     "sub-1",
			Name:   "Audit Worker",
			Role:   "research",
			Output: "Finished analysis: 0 vulnerabilities found.",
		},
	}

	drained := mgr.drainSubagentNotifications("parent-1")
	if len(drained) != 1 {
		t.Fatalf("expected 1 drained notification, got %d", len(drained))
	}
	formatted := formatSubagentNotifications(drained)
	if !strings.Contains(formatted, "<subagent-notification>") || !strings.Contains(formatted, "Audit Worker") {
		t.Errorf("expected formatted subagent notification, got: %s", formatted)
	}

	// Second drain should be empty
	drainedAgain := mgr.drainSubagentNotifications("parent-1")
	if len(drainedAgain) != 0 {
		t.Errorf("expected 0 drained notifications on second call, got %d", len(drainedAgain))
	}
}

func TestGetUsageSummary(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "forge-usage-test-*")
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

	sess, _ := mgr.CreateSession("Usage Session", RoleCoding, tempDir)
	sess.Model = "claude-3-7-sonnet"
	sess.TokenUsage.PromptTokens = 500
	sess.TokenUsage.CompletionTokens = 200
	sess.TokenUsage.CachedTokens = 150
	sess.TokenUsage.TotalTokens = 700

	usage := mgr.GetUsageSummary()
	totals, ok := usage["totals"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing totals in usage summary")
	}
	if totals["total_tokens"] != 700 {
		t.Errorf("expected total_tokens 700, got %v", totals["total_tokens"])
	}

	byModel, ok := usage["by_model"].([]map[string]interface{})
	if !ok || len(byModel) == 0 {
		t.Fatalf("missing by_model in usage summary")
	}
	if byModel[0]["model"] != "claude-3-7-sonnet" {
		t.Errorf("expected model claude-3-7-sonnet, got %v", byModel[0]["model"])
	}
	if byModel[0]["provider"] != "anthropic" {
		t.Errorf("expected inferred provider anthropic, got %v", byModel[0]["provider"])
	}
}

func TestRoleGatingReadonly(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "forge-rolegating-test-*")
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

	sess, err := mgr.CreateSession("Research Agent", RoleResearch, tempDir)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	block := ContentBlock{
		Type:       "tool_use",
		ToolCallID: "call_write_1",
		Name:       "write",
		Arguments: map[string]any{
			"path":    "exploit.txt",
			"content": "payload",
		},
	}

	contentStr, _, err := mgr.executeToolCallDetailed(context.Background(), sess.ID, block, 1)
	if err == nil {
		t.Fatalf("expected write tool to be blocked for research agent, but got no error")
	}
	if !strings.Contains(contentStr, "blocked for research agents") {
		t.Errorf("expected tool_use_error containing blocked notice, got: %s", contentStr)
	}
}

func TestContextOverflowDetection(t *testing.T) {
	if !isContextOverflowError(fmt.Errorf("API error: prompt is too long for model context window")) {
		t.Errorf("expected true for prompt is too long")
	}
	if !isContextOverflowError(fmt.Errorf("HTTP 413 Request Entity Too Large")) {
		t.Errorf("expected true for 413 error")
	}
	if !isContextOverflowError(fmt.Errorf("maximum context length is 128000 tokens")) {
		t.Errorf("expected true for maximum context length")
	}
	if isContextOverflowError(fmt.Errorf("network connection reset by peer")) {
		t.Errorf("expected false for network reset error")
	}
}

func TestSubagentRecursionGuard(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "forge-subagent-guard-*")
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

	rootSess, err := mgr.CreateSession("Root Session", RoleCoding, tempDir)
	if err != nil {
		t.Fatalf("create root: %v", err)
	}

	bridgeRoot := &sessionBridge{m: mgr, sessionID: rootSess.ID}
	childID, err := bridgeRoot.SpawnSubagent("Task 1", "coding")
	if err != nil {
		t.Fatalf("spawn level 1 child: %v", err)
	}

	bridgeChild := &sessionBridge{m: mgr, sessionID: childID}
	grandChildID, err := bridgeChild.SpawnSubagent("Task 2", "coding")
	if err != nil {
		t.Fatalf("spawn level 2 grandchild: %v", err)
	}

	bridgeGrandChild := &sessionBridge{m: mgr, sessionID: grandChildID}
	_, err = bridgeGrandChild.SpawnSubagent("Task 3", "coding")
	if err == nil {
		t.Fatalf("expected recursion depth limit error at level 3, got nil")
	}
	if !strings.Contains(err.Error(), "maximum sub-agent recursion depth") {
		t.Errorf("expected recursion depth error message, got: %v", err)
	}
}

func TestSubagentRoleModelAndPrompt(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "forge-subagent-def-test-*")
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

	// Save custom subagent definition
	_, err = mgr.SaveAgentDefinition(AgentDefinition{
		ID:        "deep-researcher",
		Name:      "Deep Researcher",
		Role:      RoleResearch,
		Model:     "gpt-6-luna",
		Prompt:    "You are a specialized deep research agent.",
		Rules:     "Always cite sources and inspect files.",
		AllowBash: true,
		ReadOnly:  true,
	})
	if err != nil {
		t.Fatalf("failed to save agent definition: %v", err)
	}

	rootSess, err := mgr.CreateSession("Parent Session", RoleCoding, tempDir)
	if err != nil {
		t.Fatalf("create root session: %v", err)
	}

	bridge := &sessionBridge{m: mgr, sessionID: rootSess.ID}
	subID, err := bridge.SpawnSubagent("Investigate architecture", "deep-researcher")
	if err != nil {
		t.Fatalf("spawn subagent with deep-researcher role: %v", err)
	}

	subSess, ok := mgr.GetSession(subID)
	if !ok {
		t.Fatalf("subagent session %s not found", subID)
	}

	if subSess.Model != "gpt-6-luna" {
		t.Errorf("expected subagent model 'gpt-6-luna', got %q", subSess.Model)
	}
	if !strings.Contains(subSess.SystemPrompt, "You are a specialized deep research agent.") {
		t.Errorf("expected custom prompt in subagent system prompt, got: %s", subSess.SystemPrompt)
	}
	if !subSess.AllowBash {
		t.Errorf("expected subagent AllowBash to be true")
	}
	if !subSess.ReadOnly {
		t.Errorf("expected subagent ReadOnly to be true")
	}

	// Verify tool permissions
	if !mgr.isToolAllowedForSession(subSess, "bash") {
		t.Errorf("expected bash to be allowed for subagent with AllowBash=true")
	}
	if mgr.isToolAllowedForSession(subSess, "write") {
		t.Errorf("expected write to be blocked for subagent with ReadOnly=true")
	}
	if mgr.isToolAllowedForSession(subSess, "edit") {
		t.Errorf("expected edit to be blocked for subagent with ReadOnly=true")
	}
	if !mgr.isToolAllowedForSession(subSess, "read") {
		t.Errorf("expected read to be allowed")
	}
}

func TestSubagentAutoWakeParent(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "forge-subagent-wake-test-*")
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

	parentSess, err := mgr.CreateSession("Parent Session", RoleCoding, tempDir)
	if err != nil {
		t.Fatalf("create parent: %v", err)
	}

	subSess, err := mgr.CreateSession("Research Sub-agent", RoleResearch, tempDir)
	if err != nil {
		t.Fatalf("create subagent: %v", err)
	}
	subSess.ParentSessionID = parentSess.ID
	subSess.State = StateExecuting
	subSess.Messages = append(subSess.Messages, AgentMessage{
		ID:   "sub-msg-1",
		Role: "assistant",
		Content: []ContentBlock{
			{Type: "text", Text: "Completed deep audit. Found 3 items."},
		},
		Timestamp: time.Now(),
	})

	// Subagent finishes turn with max reasoning steps message
	mgr.finishTurn(subSess.ID, "Agent reached maximum reasoning steps and provided summary of accomplishments and remaining work.")

	// Wait briefly for goroutine to invoke SendMessage on parent
	time.Sleep(100 * time.Millisecond)

	updatedParent, ok := mgr.GetSession(parentSess.ID)
	if !ok {
		t.Fatalf("parent session not found")
	}

	if len(updatedParent.Messages) == 0 {
		t.Fatalf("expected parent session to receive auto-wake notification message, but messages is empty")
	}

	var notifMsg *AgentMessage
	for _, m := range updatedParent.Messages {
		if strings.Contains(m.Text(), "[Sub-agent Notification]") {
			notifMsg = &m
			break
		}
	}

	if notifMsg == nil {
		t.Fatalf("expected parent session to receive '[Sub-agent Notification]' message, but none found")
	}

	notifText := notifMsg.Text()
	if !strings.Contains(notifText, "Completed deep audit. Found 3 items.") {
		t.Errorf("expected subagent output in parent message, got: %s", notifText)
	}
	if !strings.Contains(notifText, "maximum reasoning steps") {
		t.Errorf("expected status notice in parent message, got: %s", notifText)
	}
}
