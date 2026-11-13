package forgemcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeAgentID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"agent-claude", "claude"},
		{"claude", "claude"},
		{"agent-agy", "antigravity"},
		{"antigravity", "antigravity"},
		{"agent-codex", "codex"},
		{"codex", "codex"},
		{"agent-opencode", "opencode"},
		{"opencode", "opencode"},
		{"agent-ohmypi", "ohmypi"},
		{"omp", "ohmypi"},
		{"agent-pi", "pi"},
		{"pi-acp", "pi"},
		{"cursor", "cursor"},
		{"windsurf", "windsurf"},
		{"gemini", "gemini"},
		{"agent-commandcode", "commandcode"},
		{"cmd", "commandcode"},
	}

	for _, tt := range tests {
		got := NormalizeAgentID(tt.input)
		if got != tt.expected {
			t.Errorf("NormalizeAgentID(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestClaudeInstallAndUninstall(t *testing.T) {
	tmpDir := t.TempDir()
	claudeFile := filepath.Join(tmpDir, ".claude.json")

	// Initial file with existing server and custom setting
	initial := map[string]interface{}{
		"editorMode": "vim",
		"mcpServers": map[string]interface{}{
			"atlassian": map[string]interface{}{
				"command": "uvx",
				"args":    []string{"mcp-atlassian"},
			},
		},
	}
	data, _ := json.MarshalIndent(initial, "", "  ")
	_ = os.WriteFile(claudeFile, data, 0o644)

	// Verify not installed yet
	if checkForgeInstalled(claudeFile, "claude") {
		t.Fatal("expected forge not to be installed initially")
	}

	// Install
	if err := installToClaude(claudeFile, "forge-ade"); err != nil {
		t.Fatalf("installToClaude failed: %v", err)
	}

	// Check installed
	if !checkForgeInstalled(claudeFile, "claude") {
		t.Fatal("expected forge to be installed after installToClaude")
	}

	// Verify existing fields preserved
	readBytes, _ := os.ReadFile(claudeFile)
	var doc map[string]interface{}
	if err := json.Unmarshal(readBytes, &doc); err != nil {
		t.Fatalf("failed unmarshaling claude.json: %v", err)
	}
	if doc["editorMode"] != "vim" {
		t.Errorf("expected editorMode=vim preserved, got %v", doc["editorMode"])
	}
	servers := doc["mcpServers"].(map[string]interface{})
	if _, ok := servers["atlassian"]; !ok {
		t.Error("expected atlassian server preserved")
	}
	if _, ok := servers["forge"]; !ok {
		t.Error("expected forge server added")
	}

	// Uninstall
	if err := uninstallFromClaude(claudeFile); err != nil {
		t.Fatalf("uninstallFromClaude failed: %v", err)
	}
	if checkForgeInstalled(claudeFile, "claude") {
		t.Fatal("expected forge to be uninstalled")
	}
}

func TestOpencodeInstallAndUninstall(t *testing.T) {
	tmpDir := t.TempDir()
	opencodeFile := filepath.Join(tmpDir, "opencode.json")

	initial := `{
		"$schema": "https://opencode.ai/config.json",
		"mcp": {
			"exa": {
				"type": "remote",
				"url": "https://mcp.exa.ai"
			}
		}
	}`
	_ = os.WriteFile(opencodeFile, []byte(initial), 0o644)

	if checkForgeInstalled(opencodeFile, "opencode") {
		t.Fatal("expected forge not to be installed")
	}

	if err := installToOpencode(opencodeFile, "forge-ade"); err != nil {
		t.Fatalf("installToOpencode failed: %v", err)
	}
	if !checkForgeInstalled(opencodeFile, "opencode") {
		t.Fatal("expected forge to be installed in opencode")
	}

	readBytes, _ := os.ReadFile(opencodeFile)
	var doc struct {
		MCP map[string]interface{} `json:"mcp"`
	}
	_ = json.Unmarshal(readBytes, &doc)
	if _, ok := doc.MCP["exa"]; !ok {
		t.Error("expected existing exa tool preserved in opencode")
	}
	if _, ok := doc.MCP["forge"]; !ok {
		t.Error("expected forge tool present in opencode")
	}

	if err := uninstallFromOpencode(opencodeFile); err != nil {
		t.Fatalf("uninstallFromOpencode failed: %v", err)
	}
	if checkForgeInstalled(opencodeFile, "opencode") {
		t.Fatal("expected forge to be uninstalled from opencode")
	}
}

func TestCodexInstallAndUninstall(t *testing.T) {
	tmpDir := t.TempDir()
	codexFile := filepath.Join(tmpDir, "config.toml")

	initial := `model = "deepseek-v4"
model_provider = "kenari"

[tui]
screen_reader = true

[mcp_servers.atlassian]
command = "uvx"
args = ["mcp-atlassian"]
`
	_ = os.WriteFile(codexFile, []byte(initial), 0o644)

	if checkForgeInstalled(codexFile, "codex") {
		t.Fatal("expected forge not installed in codex initially")
	}

	if err := installToCodex(codexFile, "forge-ade"); err != nil {
		t.Fatalf("installToCodex failed: %v", err)
	}
	if !checkForgeInstalled(codexFile, "codex") {
		t.Fatal("expected forge installed in codex")
	}

	data, _ := os.ReadFile(codexFile)
	content := string(data)
	if !strings.Contains(content, "model = \"deepseek-v4\"") {
		t.Error("expected initial model setting preserved in codex")
	}
	if !strings.Contains(content, "[mcp_servers.atlassian]") {
		t.Error("expected atlassian server preserved in codex")
	}

	if err := uninstallFromCodex(codexFile); err != nil {
		t.Fatalf("uninstallFromCodex failed: %v", err)
	}
	if checkForgeInstalled(codexFile, "codex") {
		t.Fatal("expected forge uninstalled from codex")
	}
}

func TestGenericMCPServersInstall(t *testing.T) {
	tmpDir := t.TempDir()
	mcpFile := filepath.Join(tmpDir, "sub", "mcp_config.json")

	// File doesn't exist initially
	if err := installToGenericMCPServers(mcpFile, "forge-ade"); err != nil {
		t.Fatalf("installToGenericMCPServers failed on new file: %v", err)
	}
	if !checkForgeInstalled(mcpFile, "mcpServers") {
		t.Fatal("expected forge installed in generic mcpServers")
	}

	if err := uninstallFromGenericMCPServers(mcpFile); err != nil {
		t.Fatalf("uninstallFromGenericMCPServers failed: %v", err)
	}
	if checkForgeInstalled(mcpFile, "mcpServers") {
		t.Fatal("expected forge uninstalled from generic mcpServers")
	}
}
