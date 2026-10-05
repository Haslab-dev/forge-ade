package acp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveBinary(t *testing.T) {
	// Test standard path resolution
	piPath := resolveBinary("pi")
	if piPath == "" {
		t.Errorf("expected non-empty resolveBinary for pi")
	}

	ompPath := resolveBinary("omp")
	if ompPath == "" {
		t.Errorf("expected non-empty resolveBinary for omp")
	}

	opencodePath := resolveBinary("opencode")
	if opencodePath == "" {
		t.Errorf("expected non-empty resolveBinary for opencode")
	}
}

func TestCheckBinary(t *testing.T) {
	mgr := NewManager(t.TempDir(), nil)

	found, path := mgr.CheckBinary("opencode")
	if !found || path == "" {
		t.Errorf("expected opencode to be found, got found=%v, path=%q", found, path)
	}

	found, path = mgr.CheckBinary("omp")
	if !found || path == "" {
		t.Errorf("expected omp to be found, got found=%v, path=%q", found, path)
	}

	found, path = mgr.CheckBinary("pi")
	if !found || path == "" {
		t.Errorf("expected pi to be found, got found=%v, path=%q", found, path)
	}
}

func TestSeedDefaultsAndMigrations(t *testing.T) {
	tempDir := t.TempDir()
	mgr := NewManager(tempDir, nil)

	agents := mgr.ListAgents()
	var opencodeCfg, piCfg, ompCfg *AgentConfig
	for i := range agents {
		switch agents[i].ID {
		case "agent-opencode":
			opencodeCfg = &agents[i]
		case "agent-pi":
			piCfg = &agents[i]
		case "agent-ohmypi":
			ompCfg = &agents[i]
		}
	}

	// Native drivers ignore stored args (each driver builds its own command
	// line), but the seeds no longer carry ACP flags.
	if opencodeCfg == nil || len(opencodeCfg.Args) != 0 {
		t.Fatalf("agent-opencode args should be empty (native driver), got: %v", opencodeCfg)
	}

	if ompCfg == nil || len(ompCfg.Args) != 0 {
		t.Fatalf("agent-ohmypi args should be empty (native driver), got: %v", ompCfg)
	}

	if piCfg == nil || (piCfg.Command != "pi-acp" && piCfg.Command != "pi") || (len(piCfg.Args) > 0 && piCfg.Args[0] == "--mode") {
		t.Fatalf("agent-pi command/args should be pi-acp/empty, got: %v", piCfg)
	}

	// Now simulate outdated args saved on disk and verify migration clears them
	outdatedJSON := `[
		{"id": "agent-opencode", "name": "OpenCode", "command": "opencode", "args": ["serve", "--port", "0"], "enabled": true},
		{"id": "agent-ohmypi", "name": "OhMyPi", "command": "omp", "args": ["--mode", "rpc"], "enabled": true},
		{"id": "agent-pi", "name": "Pi Agent", "command": "pi", "args": ["--mode", "rpc"], "enabled": true}
	]`
	_ = os.WriteFile(filepath.Join(tempDir, "acp_agents.json"), []byte(outdatedJSON), 0644)

	mgr2 := NewManager(tempDir, nil)
	agents2 := mgr2.ListAgents()
	for _, a := range agents2 {
		if a.ID == "agent-opencode" && len(a.Args) != 0 {
			t.Errorf("migration failed for agent-opencode: %v", a.Args)
		}
		if a.ID == "agent-ohmypi" && len(a.Args) != 0 {
			t.Errorf("migration failed for agent-ohmypi: %v", a.Args)
		}
		if a.ID == "agent-pi" && len(a.Args) > 0 && a.Args[0] == "--mode" {
			t.Errorf("migration failed for agent-pi: %v", a.Args)
		}
	}
}

func TestSessionLookupAndAutoApprove(t *testing.T) {
	mgr := NewManager(t.TempDir(), nil)
	s := &ACPSession{
		ID:          "local-sess-1",
		ProviderID:  "acp-remote-xyz",
		AutoApprove: true,
	}
	mgr.sessions[s.ID] = s

	found := mgr.findSessionLocked("local-sess-1")
	if found == nil || found.ID != "local-sess-1" {
		t.Errorf("expected to find session by local ID")
	}

	foundRemote := mgr.findSessionLocked("acp-remote-xyz")
	if foundRemote == nil || foundRemote.ID != "local-sess-1" {
		t.Errorf("expected to find session by remote acpSessionID")
	}
}

func TestDriverResolution(t *testing.T) {
	mgr := NewManager(t.TempDir(), nil)

	claudeDrv := mgr.drivers.DriverFor(&AgentConfig{ID: "agent-claude", Command: "claude"})
	if claudeDrv == nil || claudeDrv.ID() != "claude" {
		t.Errorf("expected claude driver, got %v", claudeDrv)
	}

	codexDrv := mgr.drivers.DriverFor(&AgentConfig{ID: "agent-codex", Command: "codex"})
	if codexDrv == nil || codexDrv.ID() != "codex" {
		t.Errorf("expected codex driver, got %v", codexDrv)
	}

	opencodeDrv := mgr.drivers.DriverFor(&AgentConfig{ID: "agent-opencode", Command: "opencode"})
	if opencodeDrv == nil || opencodeDrv.ID() != "opencode" {
		t.Errorf("expected opencode driver, got %v", opencodeDrv)
	}

	piDrv := mgr.drivers.DriverFor(&AgentConfig{ID: "agent-pi", Command: "pi-acp"})
	if piDrv == nil || piDrv.ID() != "pi" {
		t.Errorf("expected pi driver, got %v", piDrv)
	}

	ompDrv := mgr.drivers.DriverFor(&AgentConfig{ID: "agent-ohmypi", Command: "omp"})
	if ompDrv == nil || ompDrv.ID() != "omp" {
		t.Errorf("expected omp driver, got %v", ompDrv)
	}
}
