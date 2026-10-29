package app

import (
	"testing"

	"github.com/Haslab-dev/RakitKode/events"
	"github.com/hasdev/forge-ade/internal/agentsession"
)

func TestCheckAgentsAvailability(t *testing.T) {
	bus := events.NewBus()
	tmp := t.TempDir()
	store := agentsession.NewConfigStore(tmp)
	app := &App{
		bus:             bus,
		dataDir:         tmp,
		agentCLIConfigs: store,
	}

	queries := []AgentAvailabilityQuery{
		{ID: "agent-internal", Kind: "internal", Executable: ""},
		{ID: "agent-fake-xyz-123", Kind: "acp", Executable: "non_existent_binary_for_test_xyz"},
	}

	results := app.CheckAgentsAvailability(queries)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// Internal must always be available
	if !results[0].Available || results[0].Path != "builtin" {
		t.Errorf("expected internal agent to be available builtin, got %+v", results[0])
	}

	// Fake binary must be unavailable
	if results[1].Available {
		t.Errorf("expected fake agent to be unavailable, got %+v", results[1])
	}
}

func TestListAvailableAgentCLIConfigs(t *testing.T) {
	tmp := t.TempDir()
	store := agentsession.NewConfigStore(tmp)
	app := &App{
		dataDir:         tmp,
		agentCLIConfigs: store,
	}

	configs := app.ListAvailableAgentCLIConfigs()
	// All returned configs must have resolvable executables
	for _, c := range configs {
		if !agentsession.DetectExecutable(c.Executable) {
			t.Errorf("expected available config executable %q to resolve", c.Executable)
		}
	}
}
