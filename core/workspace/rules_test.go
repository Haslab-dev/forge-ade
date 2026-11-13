package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkspaceRules_GetAndSync(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "rules_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Create AGENTS.md
	agentsContent := "# Project Guidelines\n- Always run tests\n- Use Go 1.26"
	err = os.WriteFile(filepath.Join(tempDir, "AGENTS.md"), []byte(agentsContent), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Read rules
	res, err := GetWorkspaceRules(tempDir)
	if err != nil {
		t.Fatalf("GetWorkspaceRules failed: %v", err)
	}
	if len(res.ActiveRules) == 0 {
		t.Fatalf("expected active rules, got none")
	}

	var foundAgents bool
	for _, r := range res.ActiveRules {
		if r.Path == "AGENTS.md" && r.Exists {
			foundAgents = true
			if !strings.Contains(r.Content, "Always run tests") {
				t.Errorf("content mismatch: %s", r.Content)
			}
		}
	}
	if !foundAgents {
		t.Errorf("AGENTS.md not detected as existing")
	}

	// Test Sync to .cursorrules
	err = SyncWorkspaceRule(tempDir, "AGENTS.md", ".cursorrules")
	if err != nil {
		t.Fatalf("SyncWorkspaceRule failed: %v", err)
	}

	cursorBytes, err := os.ReadFile(filepath.Join(tempDir, ".cursorrules"))
	if err != nil {
		t.Fatalf("failed reading synced .cursorrules: %v", err)
	}
	if !strings.Contains(string(cursorBytes), "Always run tests") {
		t.Errorf("synced .cursorrules missing expected content: %s", string(cursorBytes))
	}
}
