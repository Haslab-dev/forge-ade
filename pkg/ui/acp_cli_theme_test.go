package ui

import (
	"os"
	"testing"
)

func TestThemePalettes(t *testing.T) {
	tm := NewThemeManager()

	if tm.Appearance != "dark" {
		t.Fatalf("expected default appearance 'dark', got '%s'", tm.Appearance)
	}

	palettes := tm.AvailablePalettes()
	if len(palettes) != 6 {
		t.Fatalf("expected 6 built-in ForgeADE palettes, got %d", len(palettes))
	}

	expectedIDs := []string{
		ThemePaletteDefault,
		ThemePaletteBlossom,
		ThemePaletteGrove,
		ThemePaletteOcean,
		ThemePaletteEmber,
		ThemePaletteIris,
	}

	for _, id := range expectedIDs {
		found := false
		for _, p := range palettes {
			if p.ID == id {
				found = true
				dark := p.DarkTheme()
				light := p.LightTheme()
				if dark == nil || light == nil {
					t.Fatalf("palette %s returned nil theme", id)
				}
				if !dark.Dark || light.Dark {
					t.Fatalf("palette %s has incorrect Dark flag", id)
				}
				break
			}
		}
		if !found {
			t.Fatalf("missing expected palette %s", id)
		}
	}

	// Test ActiveTheme generation
	tm.PaletteID = ThemePaletteOcean
	tm.Appearance = "light"
	tm.FontSize = 14
	th := tm.ActiveTheme()
	if th == nil {
		t.Fatalf("expected active theme to be non-nil")
	}
	if th.Dark {
		t.Fatalf("expected active theme to be light")
	}
	if th.FontSize != 14 {
		t.Fatalf("expected active theme font size 14, got %f", th.FontSize)
	}
}

func TestACPAgentRegistry(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "acp-test-*")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	engine := NewEngine(tempDir)
	acpMgr := NewACPManager(engine, nil)

	if len(acpMgr.Agents) < 8 {
		t.Fatalf("expected at least 8 registered ACP agents, got %d", len(acpMgr.Agents))
	}

	// Verify official agents exist
	expectedAgents := map[string]string{
		"agent-internal":    "internal",
		"agent-agy":         "acp",
		"agent-claude":      "claude",
		"agent-codex":       "codex",
		"agent-opencode":    "opencode",
		"agent-commandcode": "commandcode",
		"agent-pi":          "pi",
		"agent-ohmypi":      "ohmypi",
		"agent-gemini":      "gemini",
	}

	for id, expectedType := range expectedAgents {
		found := false
		for _, a := range acpMgr.Agents {
			if a.ID == id {
				found = true
				if a.Type != expectedType {
					t.Fatalf("agent %s expected type %s, got %s", id, expectedType, a.Type)
				}
				break
			}
		}
		if !found {
			t.Fatalf("expected agent %s not found in registry", id)
		}
	}

	// Verify Internal Agent session launch
	sess, err := acpMgr.StartSession("agent-internal", tempDir, "Claude 3.7 Sonnet")
	if err != nil {
		t.Fatalf("failed to start internal ACP session: %v", err)
	}
	if sess == nil || sess.AgentID != "agent-internal" {
		t.Fatalf("expected valid internal session, got %v", sess)
	}

	// Test Permission request handling
	acpMgr.ActiveReq = &PermissionRequest{
		SessionID: sess.ID,
		RequestID: "req-1",
		ToolName:  "bash",
		Command:   "go build ./...",
	}

	_ = acpMgr.RespondPermission(sess.ID, "req-1", true)
	if acpMgr.ActiveReq != nil {
		t.Fatalf("expected active request to be cleared after response")
	}
}

func TestAgentCLIManager(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cli-test-*")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cliMgr := NewAgentCLIManager(tempDir, nil)

	// Verify configs discovery works without crash
	configs := cliMgr.Configs
	if len(configs) == 0 {
		t.Logf("no agent CLI configs found on system (normal in clean sandbox)")
	}

	// Create CLI task session
	sess := cliMgr.CreateSession("test-agent", "Test Agent", "sh", []string{"-c", "echo 'hello from agent cli'"}, tempDir)
	if sess == nil {
		t.Fatalf("expected non-nil CLI session")
	}
	if sess.AgentID != "test-agent" || sess.Status != "running" {
		t.Fatalf("expected running test-agent session, got %v", sess)
	}

	if sess.Terminal == nil {
		t.Fatalf("expected embedded terminal in CLI session")
	}

	// Verify ActiveSession lookup
	active := cliMgr.ActiveSession()
	if active == nil || active.ID != sess.ID {
		t.Fatalf("expected active session to match created session")
	}

	// Stop session
	cliMgr.StopSession(sess.ID)
	if sess.Status != "completed" {
		t.Fatalf("expected session status 'completed' after StopSession, got '%s'", sess.Status)
	}
}
