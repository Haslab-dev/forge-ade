package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestSettingsManager(t *testing.T) {
	var savedProvider, savedModel string
	sm := NewSettingsManager(nil, func(provider, model string) {
		savedProvider = provider
		savedModel = model
	})

	if sm.Provider != "anthropic" {
		t.Fatalf("expected initial provider 'anthropic', got '%s'", sm.Provider)
	}

	sm.providerIdx = 0 // "openai"
	sm.Model = "gpt-4o"
	sm.APIKey = "sk-test"

	if err := sm.Save(); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	if savedProvider != "openai" || savedModel != "gpt-4o" {
		t.Fatalf("expected saved ('openai', 'gpt-4o'), got ('%s', '%s')", savedProvider, savedModel)
	}

	// Toggle
	if sm.IsOpen {
		t.Fatalf("expected initial IsOpen false")
	}
	sm.Toggle()
	if !sm.IsOpen {
		t.Fatalf("expected IsOpen true after Toggle")
	}

	// Test multi-tab navigation
	for _, tab := range []int{SettingsTabModels, SettingsTabAppearance, SettingsTabAgents, SettingsTabShortcuts} {
		sm.ActiveTab = tab
		if sm.ActiveTab != tab {
			t.Fatalf("expected active tab %d, got %d", tab, sm.ActiveTab)
		}
	}
}

func TestGitManagerEnhancedParity(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "git-parity-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	engine := NewEngine(tempDir)
	gm := NewGitManager(engine, tempDir, nil)

	if gm.ActiveTab != GitTabChanges {
		t.Fatalf("expected default active tab GitTabChanges, got %d", gm.ActiveTab)
	}

	gm.ActiveTab = GitTabHistory
	if gm.ActiveTab != GitTabHistory {
		t.Fatalf("expected GitTabHistory")
	}

	gm.ActiveTab = GitTabBranches
	if gm.ActiveTab != GitTabBranches {
		t.Fatalf("expected GitTabBranches")
	}

	// Test AI Commit invocation trigger
	gm.GenerateAICommit("test instruction")
	if !gm.IsGeneratingAI && gm.CommitMsg == "" && gm.StatusMsg == "" {
		t.Fatalf("expected AI commit generation to be triggered or status updated")
	}
}

func TestAgentStateTabCountsAndContextUsage(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "agent-counts-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	engine := NewEngine(tempDir)
	as := NewAgentState(engine, tempDir, nil)

	// Ensure there is at least 1 session
	if len(as.Sessions) == 0 {
		as.NewSession("Initial Task")
	}

	active, pinned, archived := as.TabCounts()
	if active == 0 {
		t.Fatalf("expected at least 1 active session, got %d", active)
	}
	if pinned != 0 && pinned != 1 {
		t.Fatalf("unexpected pinned count: %d", pinned)
	}
	if archived != 0 {
		t.Fatalf("expected 0 archived initially, got %d", archived)
	}

	// Test context usage estimation
	tokens, pct := as.ContextUsage()
	if tokens < 240 {
		t.Fatalf("expected min tokens >= 240, got %d", tokens)
	}
	if pct < 0 || pct > 100 {
		t.Fatalf("expected pct between 0 and 100, got %d", pct)
	}

	// Test slash command /plan and /vibe
	as.handleSlashCommand("/plan Port Architecture")
	if as.TaskManager == nil || as.TaskManager.Goal != "Plan: Port Architecture" {
		t.Fatalf("expected goal to be 'Plan: Port Architecture', got %v", as.TaskManager.Goal)
	}

	as.handleSlashCommand("/vibe")
	lastMsg := as.Messages[len(as.Messages)-1]
	if lastMsg.Role != "assistant" || len(lastMsg.Text) == 0 {
		t.Fatalf("expected assistant response to /vibe")
	}
}

func TestFileIconSVG(t *testing.T) {
	tests := map[string]*ui.SVG{
		"main.go":       TablerBrandGolang,
		"app.tsx":       TablerBrandTypescript,
		"index.js":      TablerBrandJavascript,
		"package.json":  TablerJson,
		"README.md":     TablerMarkdown,
		"index.html":    TablerBrandHtml5,
		"styles.css":    TablerBrandCss3,
		"data.sqlite":   TablerDatabase,
		"script.sh":     TablerTerminal,
		"config.yaml":   TablerSettings,
		"unknown.xyz":   TablerFile,
	}

	for path, expectedIcon := range tests {
		got := FileIconSVG(filepath.Join("some/dir", path))
		if got == nil {
			t.Errorf("FileIconSVG(%s) is nil", path)
		}
		if got != expectedIcon {
			t.Errorf("FileIconSVG(%s) mismatch", path)
		}
	}
}
