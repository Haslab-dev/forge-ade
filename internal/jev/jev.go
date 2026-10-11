package jev

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/hasdev/forge-ade/internal/llm"
)

const (
	defaultEndpoint = "https://api.typesafe.ai/v1/systemone"
	defaultModel    = "jev-latest"
)

// RouteDecision captures Jev's classification and confidence metrics for an agent turn.
type RouteDecision struct {
	Category   string  `json:"category"`
	Confidence float64 `json:"confidence"`
	NeedsTools float64 `json:"needs_tools"`
	Model      string  `json:"model"`
}

// Router uses TypeSafe AI's Jev model as a high-speed System One classifier
// to select the exact tools an LLM needs for a task, preventing token bloat,
// tool hallucinations, and inappropriate script execution.
type Router struct {
	mu             sync.RWMutex
	apiKey         string
	endpoint       string
	client         *http.Client
	totalRoutes    int
	categoryCounts map[string]int
	tokensSaved    int64
	totalLatencyMs int64
}

// NewRouter creates a new Jev tool router.
// If apiKey is empty, it attempts to load from environment or .env files.
func NewRouter(apiKey string) *Router {
	if apiKey == "" {
		apiKey = resolveAPIKey()
	}
	return &Router{
		apiKey:   apiKey,
		endpoint: defaultEndpoint,
		client: &http.Client{
			Timeout: 2500 * time.Millisecond,
		},
		categoryCounts: make(map[string]int),
	}
}

// IsEnabled returns true if a valid TypeSafe API key is configured.
func (r *Router) IsEnabled() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.apiKey != ""
}

// SetAPIKey dynamically updates the router's API key.
func (r *Router) SetAPIKey(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.apiKey = strings.TrimSpace(key)
}

// GetAPIKey returns the currently configured API key (masked or plain).
func (r *Router) GetAPIKey() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.apiKey
}

// GetStats returns usage and performance metrics for the Jev tool router.
func (r *Router) GetStats() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()
	counts := make(map[string]int)
	for k, v := range r.categoryCounts {
		counts[k] = v
	}
	avgLatency := 0.0
	if r.totalRoutes > 0 {
		avgLatency = float64(r.totalLatencyMs) / float64(r.totalRoutes)
	}
	return map[string]interface{}{
		"enabled":         r.apiKey != "",
		"has_key":         r.apiKey != "",
		"endpoint":        r.endpoint,
		"model":           defaultModel,
		"total_routes":    r.totalRoutes,
		"category_counts": counts,
		"tokens_saved":    r.tokensSaved,
		"avg_latency_ms":  avgLatency,
	}
}

func (r *Router) recordRoute(category string, totalTools, filteredTools int, dur time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.totalRoutes++
	if category == "" {
		category = "general"
	}
	r.categoryCounts[category]++
	if totalTools > filteredTools {
		// Pruned tool definitions save roughly ~80 tokens per schema
		r.tokensSaved += int64(totalTools-filteredTools) * 80
	}
	r.totalLatencyMs += dur.Milliseconds()
}

type jevQuestion struct {
	Type         string `json:"type"`
	Instructions string `json:"instructions"`
	Criteria     any    `json:"criteria,omitempty"`
}

type jevRequest struct {
	State     string                 `json:"state"`
	Model     string                 `json:"model"`
	Questions map[string]jevQuestion `json:"questions"`
}

type jevAnswer struct {
	Type          string             `json:"type"`
	Choice        string             `json:"choice,omitempty"`
	Confidence    float64            `json:"confidence,omitempty"`
	Noul          float64            `json:"noul,omitempty"`
	Score         float64            `json:"score,omitempty"`
	Probabilities map[string]float64 `json:"probabilities,omitempty"`
}

type jevResponse struct {
	Model   string               `json:"model"`
	Answers map[string]jevAnswer `json:"answers"`
	Error   string               `json:"error,omitempty"`
}

// Route asks Jev to classify the goal/prompt and filters the given tool definitions
// to only the relevant tools. If Jev is not enabled, times out, or fails,
// it gracefully returns the original tools with no error so the agent continues normally.
func (r *Router) Route(ctx context.Context, prompt string, allTools []llm.ToolDefinition) ([]llm.ToolDefinition, *RouteDecision) {
	if !r.IsEnabled() || strings.TrimSpace(prompt) == "" || len(allTools) == 0 {
		return allTools, nil
	}
	start := time.Now()

	r.mu.RLock()
	apiKey := r.apiKey
	endpoint := r.endpoint
	r.mu.RUnlock()

	reqBody := jevRequest{
		State: prompt,
		Model: defaultModel,
		Questions: map[string]jevQuestion{
			"needs_tools": {
				Type:         "noul",
				Instructions: "Does this request require searching, reading, modifying, or executing tools on the codebase?",
			},
			"category": {
				Type:         "choice",
				Instructions: "What tool category is needed for this request? Consider file operations, terminal execution, version control, web browsing, skills, MCP servers, or all available tools.",
				Criteria: map[string]string{
					"explore": "Listing directory contents, exploring repository files, searching code, finding files by name or pattern",
					"read":    "Reading contents of specific code or configuration files",
					"modify":  "Creating, editing, or refactoring files or code",
					"execute": "Running terminal shell commands, scripts, builds, or tests",
					"git":     "Checking git status, diffs, branches, or version control",
					"web":     "Web browsing, visiting URLs, scraping websites, or interacting with web pages",
					"skills":  "Creating, loading, or managing agent skills, custom workflows, or specialized routines",
					"mcp":     "Using external Model Context Protocol (MCP) server tools, external APIs, or custom servers",
					"all":     "Complex, multi-step, or ambiguous tasks requiring all available tools and capabilities",
					"none":    "General conversational chat, answering questions, explaining code, without needing to run tools",
				},
			},
		},
	}

	jsonBytes, err := json.Marshal(reqBody)
	if err != nil {
		return allTools, nil
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return allTools, nil
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := r.client.Do(httpReq)
	if err != nil {
		log.Printf("[jev-router] request error (falling back to all tools): %v", err)
		return allTools, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBytes, _ := io.ReadAll(resp.Body)
		log.Printf("[jev-router] API returned status %d: %s (falling back to all tools)", resp.StatusCode, string(respBytes))
		return allTools, nil
	}

	var jevResp jevResponse
	if err := json.NewDecoder(resp.Body).Decode(&jevResp); err != nil {
		log.Printf("[jev-router] decode error: %v", err)
		return allTools, nil
	}

	needsTools := jevResp.Answers["needs_tools"].Noul
	catAnswer := jevResp.Answers["category"]
	category := strings.ToLower(strings.TrimSpace(catAnswer.Choice))
	confidence := catAnswer.Confidence

	decision := &RouteDecision{
		Category:   category,
		Confidence: confidence,
		NeedsTools: needsTools,
		Model:      jevResp.Model,
	}

	// 1. If Jev indicates with high confidence that no tools are needed, return empty list
	// so conversational replies don't consume tool schemas or trigger tool hallucinations.
	if needsTools < 0.25 || category == "none" {
		r.recordRoute("none", len(allTools), 0, time.Since(start))
		return nil, decision
	}

	// 2. Filter tools according to Jev's chosen category
	filtered := filterToolsByCategory(category, prompt, allTools)
	if len(filtered) == 0 {
		r.recordRoute(category, len(allTools), len(allTools), time.Since(start))
		return allTools, decision
	}

	r.recordRoute(category, len(allTools), len(filtered), time.Since(start))
	return filtered, decision
}

func filterToolsByCategory(category string, prompt string, allTools []llm.ToolDefinition) []llm.ToolDefinition {
	if category == "all" {
		return allTools
	}

	// Base tool sets by category
	allowed := make(map[string]bool)
	lowerPrompt := strings.ToLower(prompt)

	// Universal workflow and task tools always active across all categories:
	for _, name := range []string{"todo", "update_tasks", "invoke_subagent", "get_subagent_output", "list_subagents", "ask"} {
		allowed[name] = true
	}

	switch category {
	case "explore":
		// Tools for exploring, searching, listing repo contents
		for _, name := range []string{"find", "glob", "search", "read_directory_files", "read", "load_skill", "skill", "call_skill", "run_skill", "invoke_skill", "list_skills", "python_eval", "python", "todo", "ask"} {
			allowed[name] = true
		}

	case "read":
		// Tools for reading specific files
		for _, name := range []string{"read", "read_multiple", "read_directory_files", "search", "find", "load_skill", "skill", "call_skill", "run_skill", "invoke_skill", "todo", "ask"} {
			allowed[name] = true
		}

	case "modify":
		// Tools for editing, writing, creating files
		for _, name := range []string{"edit", "write", "read", "read_multiple", "search", "find", "git_status", "bash", "load_skill", "skill", "call_skill", "run_skill", "invoke_skill", "python_eval", "python", "todo", "ask"} {
			allowed[name] = true
		}

	case "execute":
		// Tools for running commands, builds, tests
		for _, name := range []string{"bash", "task_output", "task_stop", "task_list", "read", "python_eval", "python", "todo", "ask", "run_code"} {
			allowed[name] = true
		}

	case "git":
		// Tools for version control
		for _, name := range []string{"git_status", "bash", "read", "todo", "ask"} {
			allowed[name] = true
		}

	case "web":
		// Tools for browser interactions
		for _, name := range []string{"browser_navigate", "browser_snapshot", "browser_click", "browser_type", "browser_key", "browser_scroll", "browser_screenshot", "browser_tabs", "browser_close", "read", "todo", "ask"} {
			allowed[name] = true
		}

	case "skills":
		// Tools for managing or creating agent skills
		for _, name := range []string{"create_skill", "load_skill", "skill", "call_skill", "run_skill", "invoke_skill", "list_skills", "read", "write", "todo", "ask"} {
			allowed[name] = true
		}

	case "mcp":
		// External MCP tools plus basics
		for _, name := range []string{"read", "todo", "ask"} {
			allowed[name] = true
		}
		for _, td := range allTools {
			if strings.HasPrefix(td.Function.Name, "mcp_") || strings.HasPrefix(td.Function.Name, "cordis_") {
				allowed[td.Function.Name] = true
			}
		}

	default:
		// Unknown or general: return all tools
		return allTools
	}

	var result []llm.ToolDefinition
	for _, td := range allTools {
		name := td.Function.Name
		// Built-in matching category, or explicitly mentioned in prompt
		if allowed[name] || (lowerPrompt != "" && strings.Contains(lowerPrompt, strings.ToLower(name))) {
			result = append(result, td)
		} else if (category == "modify" || category == "execute") && (strings.HasPrefix(name, "mcp_") || strings.HasPrefix(name, "cordis_")) {
			result = append(result, td)
		}
	}
	return result
}

// resolveAPIKey searches for TYPESAFE_API_KEY across env vars and .env files.
func resolveAPIKey() string {
	if k := os.Getenv("TYPESAFE_API_KEY"); k != "" {
		return strings.TrimSpace(k)
	}

	// Try reading from .env in current directory and parent paths
	candidates := []string{
		".env",
		"../.env",
		"../../.env",
		filepath.Join(os.Getenv("HOME"), ".forge-ade", ".env"),
	}

	for _, path := range candidates {
		if data, err := os.ReadFile(path); err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "TYPESAFE_API_KEY=") {
					val := strings.TrimPrefix(line, "TYPESAFE_API_KEY=")
					val = strings.TrimSpace(val)
					val = strings.Trim(val, `"'`)
					if val != "" {
						_ = os.Setenv("TYPESAFE_API_KEY", val)
						return val
					}
				}
			}
		}
	}

	return ""
}
