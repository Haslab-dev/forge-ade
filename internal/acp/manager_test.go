package acp

import (
	"encoding/json"
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
