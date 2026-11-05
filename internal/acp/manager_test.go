package acp

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Haslab-dev/RakitKode/events"
)

func TestRPCSerialization(t *testing.T) {
	req := RPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  MethodInitialize,
		Params:  json.RawMessage(`{"protocolVersion":2}`),
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var parsed RPCRequest
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if parsed.Method != MethodInitialize {
		t.Errorf("expected method %s, got %s", MethodInitialize, parsed.Method)
	}
}

func TestManagerSessionLifecycle(t *testing.T) {
	bus := events.NewBus()
	mgr := NewManager(bus)

	// List sessions initially empty
	sessions := mgr.ListSessions()
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}

	// Verify lookups fail for unknown session
	err := mgr.Prompt("non-existent", "hello")
	if err == nil {
		t.Error("expected error for non-existent session prompt")
	}

	err = mgr.Cancel("non-existent")
	if err == nil {
		t.Error("expected error for non-existent session cancel")
	}
}

func TestManagerPermissionHandling(t *testing.T) {
	bus := events.NewBus()
	mgr := NewManager(bus)

	sessionID := "test-session"
	mgr.mu.Lock()
	mgr.sessions[sessionID] = &Session{
		ID:             sessionID,
		AgentID:        "agy",
		PermissionMode: "supervised",
		Status:         "running",
	}
	mgr.mu.Unlock()

	// Handle a simulated incoming request permission
	reqID := "req-123"

	// Manually invoke handleRequest with a dummy channel
	decisionCh := make(chan string, 1)
	mgr.mu.Lock()
	mgr.pendingDecisions[reqID] = decisionCh
	mgr.sessions[sessionID].Status = "waiting_permission"
	mgr.sessions[sessionID].PendingPerms = append(mgr.sessions[sessionID].PendingPerms, &PendingPerm{
		RequestID: reqID,
		Title:     "Execute bash command",
		Tool:      "run_command",
	})
	mgr.mu.Unlock()

	// Verify session is waiting permission
	s := mgr.sessions[sessionID]
	if s.Status != "waiting_permission" {
		t.Errorf("expected status waiting_permission, got %s", s.Status)
	}
	if len(s.PendingPerms) != 1 {
		t.Fatalf("expected 1 pending perm, got %d", len(s.PendingPerms))
	}

	// Respond with permission allow
	err := mgr.RespondPermission(sessionID, reqID, true)
	if err != nil {
		t.Fatalf("unexpected error responding permission: %v", err)
	}

	select {
	case decision := <-decisionCh:
		if decision != "allow" {
			t.Errorf("expected decision allow, got %s", decision)
		}
	case <-time.After(1 * time.Second):
		t.Error("timeout waiting for permission decision")
	}

	if s.Status != "running" {
		t.Errorf("expected status running after resolving perms, got %s", s.Status)
	}
	if len(s.PendingPerms) != 0 {
		t.Errorf("expected 0 pending perms remaining, got %d", len(s.PendingPerms))
	}
}

func TestNotificationRouting(t *testing.T) {
	bus := events.NewBus()
	mgr := NewManager(bus)

	sessionID := "notif-session"
	mgr.mu.Lock()
	mgr.sessions[sessionID] = &Session{
		ID:      sessionID,
		AgentID: "agy",
		Status:  "running",
	}
	mgr.mu.Unlock()

	var receivedDelta string
	bus.Subscribe("agent:message_delta", func(e events.Event) {
		if d, ok := e.Data["delta"].(string); ok {
			receivedDelta = d
		}
	})

	updateParams, _ := json.Marshal(SessionUpdateParams{
		SessionID:  sessionID,
		UpdateType: "content",
		Content:    "Hello ACP stream!",
	})

	mgr.handleNotification(sessionID, MethodSessionUpdate, updateParams)

	// Allow event propagation
	time.Sleep(50 * time.Millisecond)

	if receivedDelta != "Hello ACP stream!" {
		t.Errorf("expected 'Hello ACP stream!', got '%s'", receivedDelta)
	}
}

func TestCommandCodeAgentConfig(t *testing.T) {
	bus := events.NewBus()
	mgr := NewManager(bus)

	sessionID := "cmdcode-session"
	mgr.mu.Lock()
	mgr.sessions[sessionID] = &Session{
		ID:             sessionID,
		AgentID:        "agent-commandcode",
		Protocol:       "acp",
		PermissionMode: "supervised",
		Status:         "running",
	}
	mgr.mu.Unlock()

	// Verify update notifications route properly
	var receivedDelta string
	bus.Subscribe("agent:message_delta", func(e events.Event) {
		if d, ok := e.Data["delta"].(string); ok {
			receivedDelta = d
		}
	})

	updateParams, _ := json.Marshal(map[string]interface{}{
		"sessionId": sessionID,
		"update": map[string]interface{}{
			"sessionUpdate": "agent_message_delta",
			"delta": map[string]interface{}{
				"content": "Command Code response via ACP",
			},
		},
	})

	mgr.handleNotification(sessionID, MethodSessionUpdate, updateParams)
	time.Sleep(50 * time.Millisecond)

	if receivedDelta != "Command Code response via ACP" {
		t.Errorf("expected 'Command Code response via ACP', got '%s'", receivedDelta)
	}

	// Verify resolveExecutablePath handles empty string
	if _, err := resolveExecutablePath(""); err == nil {
		t.Error("expected error for empty command name")
	}
}

func TestAvailableCommandsUpdate(t *testing.T) {
	bus := events.NewBus()
	mgr := NewManager(bus)

	sessionID := "cmdcode-commands-session"
	mgr.mu.Lock()
	mgr.sessions[sessionID] = &Session{
		ID:             sessionID,
		AgentID:        "agent-commandcode",
		Protocol:       "acp",
		PermissionMode: "supervised",
		Status:         "running",
	}
	mgr.mu.Unlock()

	var receivedCmds []AvailableCommand
	bus.Subscribe("agent:available_commands", func(e events.Event) {
		if raw, ok := e.Data["commands"].([]AvailableCommand); ok {
			receivedCmds = raw
		}
	})

	updateParams, _ := json.Marshal(map[string]interface{}{
		"sessionId": sessionID,
		"update": map[string]interface{}{
			"sessionUpdate": "available_commands_update",
			"availableCommands": []map[string]interface{}{
				{
					"name":        "usage",
					"description": "Show token usage and context breakdown",
					"input": map[string]interface{}{
						"hint": "",
					},
				},
				{
					"name":        "compact",
					"description": "Compact conversation history",
				},
			},
		},
	})

	mgr.handleNotification(sessionID, MethodSessionUpdate, updateParams)
	time.Sleep(50 * time.Millisecond)

	if len(receivedCmds) != 2 {
		t.Fatalf("expected 2 received commands, got %d", len(receivedCmds))
	}
	if receivedCmds[0].Name != "usage" {
		t.Errorf("expected command name 'usage', got '%s'", receivedCmds[0].Name)
	}

	stored := mgr.GetAvailableCommands(sessionID)
	if len(stored) != 2 || stored[0].Name != "usage" {
		t.Errorf("expected stored commands to contain 'usage', got %+v", stored)
	}
}

func TestSessionResumeOpts(t *testing.T) {
	bus := events.NewBus()
	mgr := NewManager(bus)

	// Verify that if a session with SessionID is already running, StartSession reuses it
	existingID := "existing-acp-session"
	mgr.mu.Lock()
	mgr.sessions[existingID] = &Session{
		ID:       existingID,
		AgentID:  "agent-opencode",
		Protocol: "acp",
		Status:   "idle",
	}
	mgr.clients[existingID] = &Client{}
	mgr.mu.Unlock()

	opts := SessionOpts{
		SessionID:       existingID,
		ResumeSessionID: "remote-123",
		AgentID:         "agent-opencode",
		Command:         "opencode",
	}
	reused, err := mgr.StartSession(opts)
	if err != nil {
		t.Fatalf("expected StartSession to reuse running session, got err: %v", err)
	}
	if reused.ID != existingID {
		t.Errorf("expected reused session ID %s, got %s", existingID, reused.ID)
	}

	// Verify Prompt checks closed status
	closedID := "closed-session"
	mgr.mu.Lock()
	mgr.sessions[closedID] = &Session{
		ID:     closedID,
		Status: "closed",
	}
	mgr.mu.Unlock()

	err = mgr.Prompt(closedID, "hello")
	if err == nil || !strings.Contains(err.Error(), "session not found") {
		t.Errorf("expected 'session not found' error for closed session, got %v", err)
	}
}

func TestCommandCodeToolLifecycle(t *testing.T) {
	bus := events.NewBus()
	mgr := NewManager(bus)

	sessionID := "cmd-tool-test"
	mgr.mu.Lock()
	mgr.sessions[sessionID] = &Session{
		ID:             sessionID,
		AgentID:        "commandcode",
		Protocol:       "acp",
		PermissionMode: "supervised",
		Status:         "running",
	}
	mgr.mu.Unlock()

	var startName string
	var startArgs interface{}
	bus.Subscribe("agent:tool_start", func(e events.Event) {
		startName, _ = e.Data["name"].(string)
		startArgs = e.Data["args"]
	})

	var endName string
	var endResult string
	var endArgs interface{}
	var endErr bool
	bus.Subscribe("agent:tool_end", func(e events.Event) {
		endName, _ = e.Data["name"].(string)
		endResult, _ = e.Data["result"].(string)
		endArgs = e.Data["args"]
		endErr, _ = e.Data["is_error"].(bool)
	})

	// 1. Tool start notification from Command Code
	startPayload, _ := json.Marshal(map[string]interface{}{
		"sessionId": sessionID,
		"update": map[string]interface{}{
			"sessionUpdate": "tool_call",
			"toolCallId":    "call_abc",
			"title":         "Read: .forge/board_instructions.md",
			"kind":          "read",
			"status":        "pending",
			"rawInput": map[string]interface{}{
				"file_path": ".forge/board_instructions.md",
			},
		},
	})
	mgr.handleNotification(sessionID, MethodSessionUpdate, startPayload)
	time.Sleep(20 * time.Millisecond)

	if startName != "read_file" {
		t.Errorf("expected startName to be 'read_file', got '%s'", startName)
	}
	if m, ok := startArgs.(map[string]interface{}); !ok || m["file_path"] != ".forge/board_instructions.md" {
		t.Errorf("expected file_path in startArgs, got %v", startArgs)
	}

	// 2. Tool completed notification from Command Code (omits title and rawInput per spec)
	endPayload, _ := json.Marshal(map[string]interface{}{
		"sessionId": sessionID,
		"update": map[string]interface{}{
			"sessionUpdate": "tool_call_update",
			"toolCallId":    "call_abc",
			"status":        "completed",
			"content": []interface{}{
				map[string]interface{}{
					"type": "content",
					"content": map[string]interface{}{
						"type": "text",
						"text": "# Forge ADE Workflow Kanban Board\nIntro content",
					},
				},
			},
		},
	})
	mgr.handleNotification(sessionID, MethodSessionUpdate, endPayload)
	time.Sleep(20 * time.Millisecond)

	if endName != "read_file" {
		t.Errorf("expected endName to be retained as 'read_file', got '%s'", endName)
	}
	if endErr {
		t.Errorf("expected endErr to be false, got true")
	}
	if !strings.Contains(endResult, "Forge ADE Workflow Kanban Board") {
		t.Errorf("expected endResult to contain Kanban Board text, got '%s'", endResult)
	}
	if m, ok := endArgs.(map[string]interface{}); !ok || m["file_path"] != ".forge/board_instructions.md" {
		t.Errorf("expected retained file_path in endArgs, got %v", endArgs)
	}
}

func TestACPUsageTracking(t *testing.T) {
	bus := events.NewBus()
	tempDir, err := os.MkdirTemp("", "acp_usage_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	mgr := NewManager(bus)
	mgr.SetDataDir(tempDir)

	sessionID := "acp-usage-sess-1"
	mgr.mu.Lock()
	mgr.sessions[sessionID] = &Session{
		ID:          sessionID,
		Title:       "Test ACP Task",
		AgentID:     "agy",
		Model:       "gemini-2.5-pro",
		Cwd:         "/work/project-a",
		Status:      "idle",
		PromptCount: 2,
		CreatedAt:   time.Now().UnixMilli(),
		UpdatedAt:   time.Now().UnixMilli(),
	}
	mgr.recordUsageLocked(sessionID, &ContextUsage{
		InputTokens:  1500,
		OutputTokens: 300,
		CachedTokens: 200,
		TotalTokens:  1800,
		CostUSD:      0.005,
	})
	mgr.mu.Unlock()

	summary := mgr.GetUsageSummary()
	totals, ok := summary["totals"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected totals in summary, got %v", summary["totals"])
	}

	if totals["total_tokens"] != 1800 {
		t.Errorf("expected 1800 total tokens, got %v", totals["total_tokens"])
	}
	if totals["prompt_tokens"] != 1500 {
		t.Errorf("expected 1500 prompt tokens, got %v", totals["prompt_tokens"])
	}
	if totals["completion_tokens"] != 300 {
		t.Errorf("expected 300 completion tokens, got %v", totals["completion_tokens"])
	}
	if totals["cached_tokens"] != 200 {
		t.Errorf("expected 200 cached tokens, got %v", totals["cached_tokens"])
	}
	if totals["turn_count"] != 2 {
		t.Errorf("expected 2 turns, got %v", totals["turn_count"])
	}

	// Verify persistence: create a new manager on the same directory
	mgr2 := NewManager(bus)
	mgr2.SetDataDir(tempDir)

	summary2 := mgr2.GetUsageSummary()
	totals2, ok := summary2["totals"].(map[string]interface{})
	if !ok || totals2["total_tokens"] != 1800 {
		t.Errorf("expected restored total tokens 1800, got %v", totals2["total_tokens"])
	}
}

func TestACPResponseTextVariants(t *testing.T) {
	bus := events.NewBus()
	mgr := NewManager(bus)

	sessionID := "acp-variants-session"
	mgr.mu.Lock()
	mgr.sessions[sessionID] = &Session{
		ID:       sessionID,
		AgentID:  "agent-pi",
		Protocol: "acp",
		Status:   "running",
	}
	mgr.mu.Unlock()

	var receivedDeltas []string
	bus.Subscribe("agent:message_delta", func(e events.Event) {
		if d, ok := e.Data["delta"].(string); ok {
			receivedDeltas = append(receivedDeltas, d)
		}
	})

	// 1. Array content in agent_message_chunk
	arrayContentParams, _ := json.Marshal(map[string]interface{}{
		"sessionId": sessionID,
		"update": map[string]interface{}{
			"sessionUpdate": "agent_message_chunk",
			"content": []interface{}{
				map[string]interface{}{
					"type": "text",
					"text": "Chunk from array block",
				},
			},
		},
	})
	mgr.handleNotification(sessionID, "session/update", arrayContentParams)

	// 2. CamelCase agentMessageChunk
	camelCaseParams, _ := json.Marshal(map[string]interface{}{
		"sessionId": sessionID,
		"update": map[string]interface{}{
			"sessionUpdate": "agentMessageChunk",
			"content": map[string]interface{}{
				"type": "text",
				"text": "Chunk from camelCase",
			},
		},
	})
	mgr.handleNotification(sessionID, "session/update", camelCaseParams)

	// 3. agent_message with text
	agentMsgParams, _ := json.Marshal(map[string]interface{}{
		"sessionId": sessionID,
		"update": map[string]interface{}{
			"sessionUpdate": "agent_message",
			"content":       "Full agent message content",
		},
	})
	mgr.handleNotification(sessionID, "session/update", agentMsgParams)

	// 4. Flat notification (no nested "update" key)
	flatParams, _ := json.Marshal(map[string]interface{}{
		"sessionId":     sessionID,
		"sessionUpdate": "agent_message_chunk",
		"content":       "Flat params chunk",
	})
	mgr.handleNotification(sessionID, "session/update", flatParams)

	time.Sleep(50 * time.Millisecond)

	expected := []string{
		"Chunk from array block",
		"Chunk from camelCase",
		"Full agent message content",
		"Flat params chunk",
	}

	if len(receivedDeltas) != len(expected) {
		t.Fatalf("expected %d deltas, got %d: %v", len(expected), len(receivedDeltas), receivedDeltas)
	}

	for i, exp := range expected {
		if receivedDeltas[i] != exp {
			t.Errorf("delta[%d]: expected '%s', got '%s'", i, exp, receivedDeltas[i])
		}
	}
}

func TestManagerCancelStopsProcessAndSuppressesEvents(t *testing.T) {
	bus := events.NewBus()
	mgr := NewManager(bus)

	sessionID := "test-cancel-session"
	mgr.mu.Lock()
	mgr.sessions[sessionID] = &Session{
		ID:       sessionID,
		AgentID:  "agent-agy",
		Protocol: "agy-stream",
		Status:   "running",
	}
	mgr.clients[sessionID] = &Client{}
	mgr.mu.Unlock()

	var stoppedEmitted bool
	var turnEndEmitted bool
	var deltaEmitted bool

	bus.Subscribe("agent:stopped", func(e events.Event) {
		stoppedEmitted = true
	})
	bus.Subscribe("agent:turn_end", func(e events.Event) {
		turnEndEmitted = true
	})
	bus.Subscribe("agent:message_delta", func(e events.Event) {
		deltaEmitted = true
	})

	err := mgr.Cancel(sessionID)
	if err != nil {
		t.Fatalf("unexpected cancel error: %v", err)
	}

	mgr.mu.RLock()
	sess := mgr.sessions[sessionID]
	_, hasCli := mgr.clients[sessionID]
	mgr.mu.RUnlock()

	if sess.Status != "idle" {
		t.Errorf("expected session status 'idle', got '%s'", sess.Status)
	}
	if hasCli {
		t.Errorf("expected agy client to be removed from mgr.clients after cancel")
	}
	if !stoppedEmitted {
		t.Errorf("expected agent:stopped event to be emitted")
	}
	if !turnEndEmitted {
		t.Errorf("expected agent:turn_end event to be emitted")
	}

	// Verify subsequent stream events are suppressed when session is idle
	stepRaw := []byte(`{"event":"step_update","step_update":{"step_index":1,"step_type":"text","text_delta":"leaked text"}}`)
	mgr.handleStreamEvent(sessionID, "step_update", stepRaw, nil)

	if deltaEmitted {
		t.Errorf("expected stream event delta to be suppressed after cancellation")
	}
}

func TestStreamResultExtractsNestedConversationID(t *testing.T) {
	bus := events.NewBus()
	mgr := NewManager(bus)

	sessionID := "test-agy-result-sess"
	mgr.mu.Lock()
	mgr.sessions[sessionID] = &Session{
		ID:       sessionID,
		AgentID:  "agent-agy",
		Protocol: "agy-stream",
		Status:   "running",
	}
	mgr.mu.Unlock()

	var turnEndData map[string]interface{}
	bus.Subscribe(events.EventType("agent:turn_end"), func(e events.Event) {
		turnEndData = e.Data
	})

	rawResult := []byte(`{
		"event": "result",
		"result": {
			"conversation_id": "conv-nested-uuid-1234",
			"status": "SUCCESS",
			"response": "Done!",
			"duration_seconds": 1.25,
			"usage": {
				"input_tokens": 500,
				"output_tokens": 20
			}
		}
	}`)

	mgr.handleStreamEvent(sessionID, "result", rawResult, nil)

	mgr.mu.RLock()
	sess := mgr.sessions[sessionID]
	mgr.mu.RUnlock()

	if sess.RemoteSessionID != "conv-nested-uuid-1234" {
		t.Errorf("expected RemoteSessionID 'conv-nested-uuid-1234', got '%s'", sess.RemoteSessionID)
	}
	if sess.Status != "idle" {
		t.Errorf("expected session status 'idle', got '%s'", sess.Status)
	}
	if turnEndData == nil {
		t.Fatalf("expected agent:turn_end event to be emitted")
	}
	if turnEndData["remoteSessionId"] != "conv-nested-uuid-1234" {
		t.Errorf("expected turnEndData remoteSessionId 'conv-nested-uuid-1234', got '%v'", turnEndData["remoteSessionId"])
	}
}

func TestClientOnCloseEmitsTurnEndWhenRunning(t *testing.T) {
	bus := events.NewBus()
	mgr := NewManager(bus)

	sessionID := "test-close-running-sess"
	sess := &Session{
		ID:              sessionID,
		AgentID:         "agent-claude",
		Protocol:        "claude-stream",
		RemoteSessionID: "claude-uuid-789",
		Status:          "running",
	}
	client := &Client{}

	mgr.mu.Lock()
	mgr.sessions[sessionID] = sess
	mgr.clients[sessionID] = client
	mgr.mu.Unlock()

	var turnEndEmitted bool
	var turnEndStatus string
	var stoppedEmitted bool
	bus.Subscribe(events.EventType("agent:turn_end"), func(e events.Event) {
		turnEndEmitted = true
		if s, ok := e.Data["status"].(string); ok {
			turnEndStatus = s
		}
	})
	bus.Subscribe(events.EventType("agent:stopped"), func(e events.Event) {
		stoppedEmitted = true
	})

	// Simulate client closing while running
	client.OnClose = func(err error) {
		mgr.mu.Lock()
		s, ok := mgr.sessions[sessionID]
		var wasRunning bool
		var remoteSessID string
		if ok {
			wasRunning = (s.Status == "running")
			s.Status = "closed"
			s.UpdatedAt = time.Now().UnixMilli()
			remoteSessID = s.RemoteSessionID
		}
		mgr.mu.Unlock()

		if wasRunning {
			mgr.emitEvent("agent:turn_end", map[string]interface{}{
				"sessionId":       sessionID,
				"session_id":      sessionID,
				"remoteSessionId": remoteSessID,
				"status":          "closed",
			})
		}
		mgr.emitEvent("agent:stopped", map[string]interface{}{
			"sessionId":  sessionID,
			"session_id": sessionID,
		})
	}

	client.OnClose(errors.New("process exited with status 1"))

	if !turnEndEmitted {
		t.Errorf("expected agent:turn_end to be emitted on close during running turn")
	}
	if turnEndStatus != "closed" {
		t.Errorf("expected turn_end status 'closed', got '%s'", turnEndStatus)
	}
	if !stoppedEmitted {
		t.Errorf("expected agent:stopped to be emitted")
	}
}
