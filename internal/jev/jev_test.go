package jev

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/hasdev/forge-ade/internal/llm"
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
	if len(chatRouted) != 0 {
		t.Errorf("expected 0 tools for casual chat, got %d", len(chatRouted))
	}
}
