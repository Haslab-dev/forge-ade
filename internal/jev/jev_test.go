package jev

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/Haslab-dev/RakitKode/llm"
)

func sampleTools() []llm.ToolDefinition {
	names := []string{
		"find", "glob", "search", "read_directory_files",
		"read", "read_multiple", "write", "edit",
		"bash", "task_output", "git_status", "todo", "ask",
		"run_code", "mcp_weather",
	}
	var list []llm.ToolDefinition
	for _, n := range names {
		list = append(list, llm.ToolDefinition{
			Type: "function",
			Function: llm.FunctionSpec{
				Name:        n,
				Description: "Tool " + n,
			},
		})
	}
	return list
}

func TestFilterToolsByCategory(t *testing.T) {
	tools := sampleTools()

	// Explore category should only include exploration and read tools
	explore := filterToolsByCategory("explore", "", tools)
	for _, td := range explore {
		name := td.Function.Name
		if name == "edit" || name == "write" || name == "run_code" {
			t.Errorf("explore category should not contain mutating tool %q", name)
		}
	}

	// Modify category should include write, edit, read, etc.
	modify := filterToolsByCategory("modify", "", tools)
	hasEdit := false
	for _, td := range modify {
		if td.Function.Name == "edit" {
			hasEdit = true
		}
	}
	if !hasEdit {
		t.Errorf("modify category must contain edit tool")
	}

	// Prompt explicitly mentioning a tool should include it even if normally outside category
	withExplicit := filterToolsByCategory("explore", "please run run_code for me", tools)
	hasRunCode := false
	for _, td := range withExplicit {
		if td.Function.Name == "run_code" {
			hasRunCode = true
		}
	}
	if !hasRunCode {
		t.Errorf("explicitly mentioned tool run_code should be included in tools")
	}

	// Unknown category should fall back to all tools
	all := filterToolsByCategory("unknown", "", tools)
	if len(all) != len(tools) {
		t.Errorf("unknown category should return all tools, got %d vs %d", len(all), len(tools))
	}
}

func TestResolveAPIKeyFromEnv(t *testing.T) {
	orig := os.Getenv("TYPESAFE_API_KEY")
	defer func() {
		_ = os.Setenv("TYPESAFE_API_KEY", orig)
	}()

	_ = os.Setenv("TYPESAFE_API_KEY", "test-key-123")
	key := resolveAPIKey()
	if key != "test-key-123" {
		t.Errorf("expected 'test-key-123', got %q", key)
	}
}

func TestJevLiveRoutingIfKeyAvailable(t *testing.T) {
	key := resolveAPIKey()
	if key == "" {
		t.Skip("skipping live Jev test: no TYPESAFE_API_KEY found")
	}

	router := NewRouter(key)
	tools := sampleTools()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 1. Explore prompt
	routed, decision := router.Route(ctx, "list my codebase and show all files", tools)
	if decision == nil {
		t.Skip("skipping live Jev test: live API unreachable or timed out")
	}
	if decision.Category != "explore" {
		t.Errorf("expected category 'explore', got %q", decision.Category)
	}
	if len(routed) == 0 || len(routed) >= len(tools) {
		t.Errorf("expected filtered tools for explore, got %d tools out of %d", len(routed), len(tools))
	}

	// 2. Casual chat prompt
	chatRouted, chatDecision := router.Route(ctx, "hello! how is the weather today?", tools)
	if chatDecision == nil {
		t.Skip("skipping live Jev test: live API unreachable or timed out for chat")
	}
	if chatDecision.NeedsTools >= 0.5 {
		t.Errorf("expected needs_tools < 0.5 for chat, got %.2f", chatDecision.NeedsTools)
	}
	// Since the zero-tool change, casual chat keeps the universal set
	// (coordination + computer-use tools) so the model never hallucinates a
	// toolset — but nothing beyond it.
	universal := UniversalToolNames()
	uniNames := map[string]bool{}
	for _, n := range universal {
		uniNames[n] = true
	}
	if len(chatRouted) == 0 {
		t.Error("casual chat should keep the universal toolset, got 0 tools")
	}
	for _, td := range chatRouted {
		if !uniNames[td.Function.Name] {
			t.Errorf("casual chat leaked non-universal tool %q", td.Function.Name)
		}
	}
}

func TestHeuristicRoleDecision(t *testing.T) {
	tests := []struct {
		task     string
		roles    []string
		expected string
	}{
		{
			task:     "Find where RouteDecision is defined and trace its usage across internal/",
			expected: "research",
		},
		{
			task:     "Audit codebase for potential security vulnerabilities and race conditions",
			expected: "research",
		},
		{
			task:     "Design the architectural roadmap and decompose milestones for v2 migration",
			expected: "planning",
		},
		{
			task:     "Break down the steps required to implement OAuth2 authentication",
			expected: "planning",
		},
		{
			task:     "Implement JWT auth middleware and write unit tests in internal/auth",
			expected: "coding",
		},
		{
			task:     "Fix memory leak in websocket listener and verify with go test",
			expected: "coding",
		},
		{
			task:     "Please ask the database-specialist to optimize slow queries",
			roles:    []string{"database-specialist", "research", "coding"},
			expected: "database-specialist",
		},
		{
			task:     "general question about software engineering",
			expected: "coding",
		},
	}

	for _, tt := range tests {
		dec := HeuristicRoleDecision(tt.task, tt.roles)
		if dec.Role != tt.expected {
			t.Errorf("task %q: expected role %q, got %q (reason: %s)", tt.task, tt.expected, dec.Role, dec.Reason)
		}
		if dec.Confidence <= 0 {
			t.Errorf("task %q: expected confidence > 0, got %f", tt.task, dec.Confidence)
		}
	}
}

func TestHeuristicSubagentDecision(t *testing.T) {
	candidates := []SubagentCandidate{
		{ID: "coding", Name: "Coding Agent", Role: "coding", Description: "Write code, fix bugs, run builds"},
		{ID: "research", Name: "Research Agent", Role: "research", Description: "Codebase analysis, symbol search, file audits"},
		{ID: "planning", Name: "Planning Agent", Role: "planning", Description: "Architecture breakdown, plans, roadmaps"},
		{ID: "db-specialist", Name: "Database Specialist", Role: "research", Description: "PostgreSQL query optimization, slow queries, and database index audits"},
	}

	// 1. Specialist custom subagent match
	dec1 := HeuristicSubagentDecision("Analyze slow queries and optimize database index for orders table", candidates)
	if dec1.SelectedID != "db-specialist" {
		t.Errorf("expected 'db-specialist', got %q (%s)", dec1.SelectedID, dec1.Reason)
	}

	// 2. Research match
	dec2 := HeuristicSubagentDecision("Find where RouteDecision is used in internal/", candidates)
	if dec2.SelectedID != "research" {
		t.Errorf("expected 'research', got %q (%s)", dec2.SelectedID, dec2.Reason)
	}

	// 3. Planning match
	dec3 := HeuristicSubagentDecision("Draft architecture plan and decompose milestones", candidates)
	if dec3.SelectedID != "planning" {
		t.Errorf("expected 'planning', got %q (%s)", dec3.SelectedID, dec3.Reason)
	}

	// 4. Coding match
	dec4 := HeuristicSubagentDecision("Fix bug in JWT handler and run tests", candidates)
	if dec4.SelectedID != "coding" {
		t.Errorf("expected 'coding', got %q (%s)", dec4.SelectedID, dec4.Reason)
	}
}

func TestFilterToolsByCategoryIncludesComputerTools(t *testing.T) {
	all := []llm.ToolDefinition{
		{Function: llm.FunctionSpec{Name: "computer_screenshot"}},
		{Function: llm.FunctionSpec{Name: "computer_click"}},
		{Function: llm.FunctionSpec{Name: "bash"}},
		{Function: llm.FunctionSpec{Name: "read"}},
	}
	for _, category := range []string{"explore", "read", "modify", "execute", "git", "web", "skills", "mcp"} {
		got := filterToolsByCategory(category, "", all)
		names := map[string]bool{}
		for _, td := range got {
			names[td.Function.Name] = true
		}
		if !names["computer_screenshot"] || !names["computer_click"] {
			t.Errorf("category %q dropped computer tools (got %v)", category, names)
		}
	}
	// Unknown categories keep everything.
	if got := filterToolsByCategory("something-new", "", all); len(got) != len(all) {
		t.Errorf("unknown category should return all tools, got %d", len(got))
	}
}

func TestExplicitToolMentions(t *testing.T) {
	all := []llm.ToolDefinition{
		{Function: llm.FunctionSpec{Name: "computer_screenshot"}},
		{Function: llm.FunctionSpec{Name: "computer_click"}},
		{Function: llm.FunctionSpec{Name: "bash"}},
	}
	got := explicitToolMentions("Call the computer_screenshot tool now.", all)
	if len(got) != 1 || got[0].Function.Name != "computer_screenshot" {
		t.Fatalf("expected only computer_screenshot, got %v", got)
	}
	if got := explicitToolMentions("just chat with me", all); len(got) != 0 {
		t.Fatalf("no mentions expected, got %v", got)
	}
	if got := explicitToolMentions("", all); got != nil {
		t.Fatalf("empty prompt should return nil, got %v", got)
	}
}

func TestNoneCategoryKeepsUniversalTools(t *testing.T) {
	all := []llm.ToolDefinition{
		{Function: llm.FunctionSpec{Name: "computer_screenshot"}},
		{Function: llm.FunctionSpec{Name: "computer_click"}},
		{Function: llm.FunctionSpec{Name: "computer_key"}},
		{Function: llm.FunctionSpec{Name: "read"}},
		{Function: llm.FunctionSpec{Name: "bash"}},
		{Function: llm.FunctionSpec{Name: "web_fetch"}},
	}
	// Regression: a "none" classification used to return nil, leaving the
	// model with zero tools — it then hallucinated a toolset from training
	// data and claimed computer_* "isn't available in this session".
	got := filterToolsByCategory("none", "", all)
	names := map[string]bool{}
	for _, td := range got {
		names[td.Function.Name] = true
	}
	for _, want := range []string{"computer_screenshot", "computer_click", "computer_key"} {
		if !names[want] {
			t.Errorf("none-category dropped universal tool %q (got %v)", want, names)
		}
	}
	for _, unwanted := range []string{"read", "bash", "web_fetch"} {
		if names[unwanted] {
			t.Errorf("none-category leaked non-universal tool %q", unwanted)
		}
	}
}

func TestHeuristicRoute(t *testing.T) {
	all := []llm.ToolDefinition{
		{Function: llm.FunctionSpec{Name: "computer_screenshot"}},
		{Function: llm.FunctionSpec{Name: "computer_click"}},
		{Function: llm.FunctionSpec{Name: "bash"}},
		{Function: llm.FunctionSpec{Name: "read"}},
		{Function: llm.FunctionSpec{Name: "browser_navigate"}},
		{Function: llm.FunctionSpec{Name: "git_status"}},
	}

	// Explicit tool citation wins over everything.
	got, cat := HeuristicRoute("Call the computer_screenshot tool now.", all)
	if cat != "explicit-mention" || len(got) != 1 {
		t.Fatalf("explicit mention: cat=%q got=%v", cat, got)
	}

	// Screen intent keeps everything (computer + browser tools).
	got, cat = HeuristicRoute("take a screenshot of my screen", all)
	if cat != "all" || len(got) != len(all) {
		t.Fatalf("screen intent: cat=%q got=%d tools", cat, len(got))
	}

	// Keyword routing, English and Indonesian.
	if _, cat = HeuristicRoute("jalankan bun test untuk package ini", all); cat != "execute" {
		t.Errorf("execute intent: got %q", cat)
	}
	if _, cat = HeuristicRoute("check git status and recent commits", all); cat != "git" {
		t.Errorf("git intent: got %q", cat)
	}
	if _, cat = HeuristicRoute("open example.com in the browser", all); cat != "web" {
		t.Errorf("web intent: got %q", cat)
	}

	// No keyword match → "none" → universal set only (never nil, never all).
	got, cat = HeuristicRoute("halo apa kabar?", all)
	if cat != "none" {
		t.Fatalf("chat intent: got %q", cat)
	}
	names := map[string]bool{}
	for _, td := range got {
		names[td.Function.Name] = true
	}
	if !names["computer_screenshot"] {
		t.Error("none-category must keep universal computer tools")
	}
	if names["bash"] || names["web_fetch"] {
		t.Error("none-category leaked non-universal tools")
	}
}
