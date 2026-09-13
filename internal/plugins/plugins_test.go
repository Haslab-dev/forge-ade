package plugins

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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
	if len(plugins) < 4 {
		t.Fatalf("expected at least 4 builtin plugins (git, code-mode, minimal, creator, deepseek-coder), got %d", len(plugins))
	}

	// 2. Create a custom plugin with a command tool
	req := CreatePluginRequest{
		ID:           "custom-test-plugin",
		Name:         "Test Plugin",
		Description:  "A test plugin for validation",
		Version:      "1.0.0",
		Scope:        "global",
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
		if strings.Contains(p, "Always be polite") {
			foundPrompt = true
			break
		}
	}
	if !foundPrompt {
		t.Fatalf("expected active system prompts from custom-test-plugin")
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

func TestDSHCodeModeExecution(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "forge-dsh-codemode-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	mgr := NewManager(tempDir, nil)
	ctx := context.Background()

	// Execute run_code with JavaScript (Node.js)
	jsCode := `console.log(JSON.stringify({ calculated: 40 + 2, status: 'ok' }));`
	res, err := mgr.ExecuteTool(ctx, "dsh-code-mode", "run_code", map[string]interface{}{
		"code":     jsCode,
		"language": "javascript",
	}, tempDir)
	if err != nil {
		t.Fatalf("failed to execute run_code (js): %v", err)
	}
	if !res.Success {
		t.Fatalf("run_code (js) failed: %s (stderr: %s)", res.Error, res.Stderr)
	}
	if !strings.Contains(res.Stdout, `"calculated": 42`) && !strings.Contains(res.Stdout, `"calculated":42`) {
		t.Fatalf("expected calculated 42 in stdout, got: %s", res.Stdout)
	}

	// Execute eval_expression with JavaScript
	evalRes, err := mgr.ExecuteTool(ctx, "dsh-code-mode", "eval_expression", map[string]interface{}{
		"expression": "100 * 5",
		"language":   "javascript",
	}, tempDir)
	if err != nil {
		t.Fatalf("failed to execute eval_expression: %v", err)
	}
	if !evalRes.Success || !strings.Contains(evalRes.Stdout, "500") {
		t.Fatalf("expected 500 from eval_expression, got: %q (err: %s)", evalRes.Stdout, evalRes.Error)
	}
}

func TestDSHCreatorModeInspect(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "forge-dsh-creator-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	mgr := NewManager(tempDir, nil)
	ctx := context.Background()

	// cordis_inspect
	res, err := mgr.ExecuteTool(ctx, "dsh-creator-mode", "cordis_inspect", map[string]interface{}{
		"target": "all",
	}, tempDir)
	if err != nil {
		t.Fatalf("failed to execute cordis_inspect: %v", err)
	}
	if !res.Success {
		t.Fatalf("cordis_inspect failed: %s", res.Error)
	}
	if !strings.Contains(res.Stdout, "dsh-code-mode") || !strings.Contains(res.Stdout, "active_tools") {
		t.Fatalf("expected dsh-code-mode and active_tools in cordis_inspect output, got: %s", res.Stdout)
	}

	// validate_plugin on self tempDir
	manifestPath := filepath.Join(tempDir, "plugin.json")
	_ = os.WriteFile(manifestPath, []byte(`{"id": "test-val", "name": "Validate Test", "tools": []}`), 0644)

	valRes, err := mgr.ExecuteTool(ctx, "dsh-creator-mode", "validate_plugin", map[string]interface{}{
		"path": tempDir,
	}, tempDir)
	if err != nil {
		t.Fatalf("failed to execute validate_plugin: %v", err)
	}
	if !valRes.Success || !strings.Contains(valRes.Stdout, `"valid": true`) {
		t.Fatalf("expected valid: true, got: %s", valRes.Stdout)
	}
}

func TestCordisPresetAndSkillScanning(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "forge-cordis-preset-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	wsDir := filepath.Join(tempDir, "workspace")
	pluginsDir := filepath.Join(wsDir, ".dsh", "plugins", "sample-preset")
	_ = os.MkdirAll(pluginsDir, 0755)

	// Write preset.yml
	presetYAML := `name: Sample Preset
description: A test preset loaded directly from YAML
order: 1
`
	_ = os.WriteFile(filepath.Join(pluginsDir, "preset.yml"), []byte(presetYAML), 0644)

	// Write agent.cordis.yml
	agentYAML := `- id: persona
  name: '@deepseek-ai/dsh-persona'
  config:
    prefix: You are a custom Cordis persona.
- id: tool-presentation
  config:
    mode: ptc
`
	_ = os.WriteFile(filepath.Join(pluginsDir, "agent.cordis.yml"), []byte(agentYAML), 0644)

	// Write bundled skill
	skillDir := filepath.Join(pluginsDir, "skills", "custom-guide")
	_ = os.MkdirAll(skillDir, 0755)
	skillContent := `---
name: custom-guide
description: Test skill bundled in preset
---
# Guide Content
Follow this guide carefully.
`
	_ = os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0644)

	// Initialize Manager and set workspace
	dataDir := filepath.Join(tempDir, "data")
	mgr := NewManager(dataDir, nil)
	mgr.SetWorkspace(wsDir)

	// Check that sample-preset was discovered
	p, ok := mgr.Get("sample-preset")
	if !ok {
		t.Fatalf("expected sample-preset to be discovered from .dsh/plugins")
	}
	if p.Name != "Sample Preset" {
		t.Fatalf("expected Name 'Sample Preset', got %q", p.Name)
	}
	if !strings.Contains(p.SystemPrompt, "You are a custom Cordis persona") {
		t.Fatalf("expected persona prefix in SystemPrompt, got: %q", p.SystemPrompt)
	}

	// Check that skills were auto-loaded
	foundSkill := false
	for _, s := range p.Skills {
		if s.Name == "custom-guide" && strings.Contains(s.Body, "Follow this guide carefully") {
			foundSkill = true
			break
		}
	}
	if !foundSkill {
		t.Fatalf("expected bundled skill 'custom-guide' to be parsed and loaded into plugin")
	}

	// Check that PTC tools were attached
	foundRunCode := false
	for _, tool := range p.Tools {
		if tool.Name == "run_code" {
			foundRunCode = true
			break
		}
	}
	if !foundRunCode {
		t.Fatalf("expected run_code tool to be attached to PTC preset")
	}
}
