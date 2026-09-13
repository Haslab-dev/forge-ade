package plugins

import (
	"context"
	"os"
	"testing"
)

func TestPluginLifecycle(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "forge-plugins-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	mgr := NewManager(tempDir, nil)

	// 1. Check builtins
	plugins := mgr.List()
	if len(plugins) == 0 {
		t.Fatalf("expected at least 1 builtin plugin, got 0")
	}

	// 2. Create a custom plugin with a command tool
	req := CreatePluginRequest{
		ID:          "custom-test-plugin",
		Name:        "Test Plugin",
		Description: "A test plugin for validation",
		Version:     "1.0.0",
		Scope:       "global",
		SystemPrompt: "Always be polite when testing.",
		Tools: []PluginToolDef{
			{
				Name:        "echo_test",
				Description: "Echo test message",
				HandlerType: HandlerCommand,
				Command:     "echo 'hello from plugin'",
			},
		},
		Skills: []PluginSkillDef{
			{
				Name:        "test-skill",
				Description: "Testing skill guide",
				Body:        "Follow testing guidelines.",
			},
		},
	}

	created, err := mgr.CreatePlugin(req, "")
	if err != nil {
		t.Fatalf("failed to create plugin: %v", err)
	}
	if created.ID != "custom-test-plugin" {
		t.Fatalf("expected plugin id custom-test-plugin, got %s", created.ID)
	}

	// 3. Verify tool execution
	ctx := context.Background()
	res, err := mgr.ExecuteTool(ctx, "custom-test-plugin", "echo_test", nil, "")
	if err != nil {
		t.Fatalf("failed to execute plugin tool: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected success, got error: %s", res.Error)
	}
	if res.Stdout != "hello from plugin" {
		t.Fatalf("expected 'hello from plugin', got %q", res.Stdout)
	}

	// 4. Verify active system prompts
	prompts := mgr.ActiveSystemPrompts()
	foundPrompt := false
	for _, p := range prompts {
		if p != "" {
			foundPrompt = true
			break
		}
	}
	if !foundPrompt {
		t.Fatalf("expected active system prompts from enabled plugins")
	}

	// 5. Test toggle
	if err := mgr.TogglePlugin("custom-test-plugin", false); err != nil {
		t.Fatalf("failed to toggle plugin: %v", err)
	}
	p, ok := mgr.Get("custom-test-plugin")
	if !ok || p.Enabled {
		t.Fatalf("expected plugin to be disabled")
	}

	// Verify executing disabled tool fails
	_, err = mgr.ExecuteTool(ctx, "custom-test-plugin", "echo_test", nil, "")
	if err == nil {
		t.Fatalf("expected error executing disabled plugin tool")
	}

	// 6. Delete plugin
	if err := mgr.DeletePlugin("custom-test-plugin"); err != nil {
		t.Fatalf("failed to delete plugin: %v", err)
	}
	if _, ok := mgr.Get("custom-test-plugin"); ok {
		t.Fatalf("expected plugin to be deleted")
	}
}
