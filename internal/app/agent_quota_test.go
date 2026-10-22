package app

import (
	"testing"
)

func TestGetAgentQuotaAntigravity(t *testing.T) {
	app := &App{}
	quota, err := app.GetAgentQuota("agent-agy")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if quota["agentId"] != "agent-agy" {
		t.Errorf("expected agentId agent-agy, got %v", quota["agentId"])
	}
	t.Logf("Antigravity account: %v", quota["account"])
	t.Logf("Antigravity weekly: %+v", quota["weekly"])
	t.Logf("Antigravity fiveHour: %+v", quota["fiveHour"])
	t.Logf("Antigravity extraGroups: %+v", quota["extraGroups"])

	weekly, ok := quota["weekly"].(map[string]interface{})
	if !ok || weekly["remainingPercent"] == nil {
		t.Errorf("expected weekly quota info, got %v", quota["weekly"])
	}
}

func TestGetAgentQuotaCommandCode(t *testing.T) {
	app := &App{}
	quota, err := app.GetAgentQuota("agent-commandcode")
	if err != nil {
		t.Skipf("skipping live command code API test if unauthenticated: %v", err)
	}
	if quota["agentId"] != "agent-commandcode" {
		t.Errorf("expected agentId agent-commandcode, got %v", quota["agentId"])
	}
	if quota["plan"] == nil {
		t.Errorf("expected plan details in quota")
	}
}
