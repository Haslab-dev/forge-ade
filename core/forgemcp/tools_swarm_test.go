package forgemcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

type mockSwarmBridge struct {
	lastSpawn   SwarmSpawnRequest
	lastHandoff SwarmHandoffRequest
}

func (m *mockSwarmBridge) ListAgents() ([]SwarmAgentInfo, []SwarmPersonaInfo) {
	return []SwarmAgentInfo{
		{ID: "agent-agy", Name: "Antigravity", Type: "acp", Eligible: true, IsDefault: true},
		{ID: "agent-codex", Name: "Codex", Type: "acp", Eligible: false, Reason: "codex CLI not detected on PATH"},
	}, []SwarmPersonaInfo{
		{ID: "research", Name: "Researcher", Role: "Researcher", Source: "builtin"},
	}
}

func (m *mockSwarmBridge) SpawnWorker(req SwarmSpawnRequest) (*SwarmWorkerInfo, error) {
	m.lastSpawn = req
	return &SwarmWorkerInfo{
		SessionID:     "sess-worker-1",
		Name:          req.Name,
		AgentID:       "agent-agy",
		AgentType:     "acp",
		WorkspacePath: "/tmp/ws",
		Status:        "running",
	}, nil
}

func (m *mockSwarmBridge) ListWorkers(parentSessionID string) []SwarmWorkerInfo {
	return []SwarmWorkerInfo{{SessionID: "sess-worker-1", Name: "researcher", AgentID: "agent-agy", Status: "running"}}
}

func (m *mockSwarmBridge) MessageWorker(workerSessionID, instruction string) error {
	return nil
}

func (m *mockSwarmBridge) CollectWorker(workerSessionID string) (*SwarmWorkerResult, error) {
	return &SwarmWorkerResult{SessionID: workerSessionID, Name: "researcher", Status: "complete", Summary: "done"}, nil
}

func (m *mockSwarmBridge) HandoffSession(req SwarmHandoffRequest) (*SwarmWorkerInfo, error) {
	m.lastHandoff = req
	return &SwarmWorkerInfo{SessionID: "sess-handoff-1", Name: req.Goal, AgentID: "agent-agy", Status: "running"}, nil
}

func TestRegisterSwarmTools(t *testing.T) {
	srv := NewServer("/tmp/ws")
	bridge := &mockSwarmBridge{}
	srv.RegisterSwarmTools(bridge)

	tools := srv.ListTools()
	if len(tools) != 6 {
		t.Fatalf("expected 6 swarm tools, got %d", len(tools))
	}

	// Call swarm_list_agents
	resp := srv.HandleRequest(context.Background(), &JSONRPCMessage{
		JSONRPC: JSONRPCVersion,
		ID:      1,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"swarm_list_agents","arguments":{}}`),
	})
	if resp == nil || resp.Error != nil {
		t.Fatalf("swarm_list_agents failed: %+v", resp)
	}
	resBytes, _ := json.Marshal(resp.Result)
	if !strings.Contains(string(resBytes), "agent-agy") {
		t.Fatalf("expected agent-agy in swarm_list_agents output: %s", string(resBytes))
	}

	// Call swarm_spawn_worker
	resp2 := srv.HandleRequest(context.Background(), &JSONRPCMessage{
		JSONRPC: JSONRPCVersion,
		ID:      2,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"swarm_spawn_worker","arguments":{"name":"research-auth","persona":"research","agent_id":"agy","task":"Find auth handlers"}}`),
	})
	if resp2 == nil || resp2.Error != nil {
		t.Fatalf("swarm_spawn_worker failed: %+v", resp2)
	}
	if bridge.lastSpawn.Persona != "research" || bridge.lastSpawn.AgentID != "agy" {
		t.Fatalf("unexpected spawn request forwarded to bridge: %+v", bridge.lastSpawn)
	}
}
