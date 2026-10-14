package jev

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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

// RoleDecision captures Jev's sub-agent role recommendation and rationale.
type RoleDecision struct {
	Role       string  `json:"role"`
	Reason     string  `json:"reason"`
	Confidence float64 `json:"confidence"`
	Source     string  `json:"source"`
}

// SubagentCandidate represents a sub-agent definition available for Jev delegation.
type SubagentCandidate struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Role        string `json:"role"`
	Description string `json:"description"`
}

// SubagentDecision captures Jev's sub-agent recommendation.
type SubagentDecision struct {
	SelectedID string  `json:"selected_id"`
	Name       string  `json:"name"`
	Role       string  `json:"role"`
	Reason     string  `json:"reason"`
	Confidence float64 `json:"confidence"`
	Source     string  `json:"source"`
}

// RouterStats tracks cumulative usage metrics for Jev routing decisions.
type RouterStats struct {
	TotalRequests    int            `json:"total_requests"`
	TotalRoutes      int            `json:"total_routes"`
	CategoryCounts   map[string]int `json:"category_counts"`
	RoleCounts       map[string]int `json:"role_counts,omitempty"`
	PromptTokens     int64          `json:"prompt_tokens"`
	CompletionTokens int64          `json:"completion_tokens"`
	TotalTokens      int64          `json:"total_tokens"`
	TokensSaved      int64          `json:"tokens_saved"`
	TotalLatencyMs   int64          `json:"total_latency_ms"`
}

// Router uses TypeSafe AI's Jev model as a high-speed System One classifier
// to select the exact tools an LLM needs for a task, preventing token bloat,
// tool hallucinations, and inappropriate script execution.
type Router struct {
	mu            sync.RWMutex
	apiKey        string
	endpoint      string
	storePath     string
	client        *http.Client
	stats         RouterStats
	disabledUntil time.Time
}

// NewRouter creates a new Jev tool router.
// If apiKey is empty, it attempts to load from environment or .env files.
// Optional dataDir specifies where to persist jev_stats.json.
func NewRouter(apiKey string, dataDir ...string) *Router {
	if apiKey == "" {
		apiKey = resolveAPIKey()
	}
	var storePath string
	if len(dataDir) > 0 && dataDir[0] != "" {
		storePath = filepath.Join(dataDir[0], "jev_stats.json")
	}
	r := &Router{
		apiKey:    apiKey,
		endpoint:  defaultEndpoint,
		storePath: storePath,
		client: &http.Client{
			Timeout: 1200 * time.Millisecond,
		},
		stats: RouterStats{
			CategoryCounts: make(map[string]int),
			RoleCounts:     make(map[string]int),
		},
	}
	r.loadStats()
	return r
}

func (r *Router) loadStats() {
	if r.storePath == "" {
		return
	}
	data, err := os.ReadFile(r.storePath)
	if err != nil {
		return
	}
	var s RouterStats
	if err := json.Unmarshal(data, &s); err == nil {
		if s.CategoryCounts == nil {
			s.CategoryCounts = make(map[string]int)
		}
		if s.RoleCounts == nil {
			s.RoleCounts = make(map[string]int)
		}
		r.stats = s
	}
}

// IsEnabled returns true if a valid TypeSafe API key is configured and not in cooldown.
func (r *Router) IsEnabled() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.apiKey != "" && time.Now().After(r.disabledUntil)
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
	for k, v := range r.stats.CategoryCounts {
		counts[k] = v
	}
	roles := make(map[string]int)
	for k, v := range r.stats.RoleCounts {
		roles[k] = v
	}
	avgLatency := 0.0
	if r.stats.TotalRoutes > 0 {
		avgLatency = float64(r.stats.TotalLatencyMs) / float64(r.stats.TotalRoutes)
	}
	return map[string]interface{}{
		"enabled":           r.apiKey != "",
		"has_key":           r.apiKey != "",
		"endpoint":          r.endpoint,
		"model":             defaultModel,
		"total_requests":    r.stats.TotalRequests,
		"total_routes":      r.stats.TotalRoutes,
		"category_counts":   counts,
		"role_counts":       roles,
		"prompt_tokens":     r.stats.PromptTokens,
		"completion_tokens": r.stats.CompletionTokens,
		"total_tokens":      r.stats.TotalTokens,
		"tokens_saved":      r.stats.TokensSaved,
		"avg_latency_ms":    avgLatency,
	}
}

func (r *Router) recordRoute(category string, totalTools, filteredTools int, promptLen int, dur time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stats.TotalRequests++
	r.stats.TotalRoutes++
	if category == "" {
		category = "general"
	}
	if r.stats.CategoryCounts == nil {
		r.stats.CategoryCounts = make(map[string]int)
	}
	r.stats.CategoryCounts[category]++
	if totalTools > filteredTools {
		// Pruned tool definitions save roughly ~80 tokens per schema
		r.stats.TokensSaved += int64(totalTools-filteredTools) * 80
	}
	pTok := int64(promptLen/4 + 180)
	cTok := int64(45)
	r.stats.PromptTokens += pTok
	r.stats.CompletionTokens += cTok
	r.stats.TotalTokens += pTok + cTok
	r.stats.TotalLatencyMs += dur.Milliseconds()

	if r.storePath != "" {
		if data, err := json.MarshalIndent(r.stats, "", "  "); err == nil {
			_ = os.WriteFile(r.storePath, data, 0644)
		}
	}
}

func (r *Router) recordRoleDecision(role string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stats.TotalRequests++
	if role == "" {
		role = "coding"
	}
	if r.stats.RoleCounts == nil {
		r.stats.RoleCounts = make(map[string]int)
	}
	r.stats.RoleCounts[role]++
	if r.storePath != "" {
		if data, err := json.MarshalIndent(r.stats, "", "  "); err == nil {
			_ = os.WriteFile(r.storePath, data, 0644)
		}
	}
}

// DecideAgentRole queries Jev to classify a task into the optimal sub-agent role
// ('research', 'planning', 'coding', or a custom role). If Jev is not configured,
// times out, or fails, it falls back to fast heuristic role decision.
func (r *Router) DecideAgentRole(ctx context.Context, task string, availableRoles []string) *RoleDecision {
	task = strings.TrimSpace(task)
	if task == "" {
		return &RoleDecision{
			Role:       "coding",
			Reason:     "Empty task provided; default coding agent assigned.",
			Confidence: 1.0,
			Source:     "default",
		}
	}

	if !r.IsEnabled() {
		return HeuristicRoleDecision(task, availableRoles)
	}

	r.mu.RLock()
	apiKey := r.apiKey
	endpoint := r.endpoint
	r.mu.RUnlock()

	reqBody := jevRequest{
		State: task,
		Model: defaultModel,
		Questions: map[string]jevQuestion{
			"role": {
				Type:         "choice",
				Instructions: "Which sub-agent role is best suited to execute this task?",
				Criteria: map[string]string{
					"research": "Deep codebase exploration, finding references, reading documentation, analyzing code structure or security audits without making file modifications",
					"planning": "High-level architecture planning, decomposing complex goals into structured roadmaps or execution steps",
					"coding":   "Writing or modifying code, refactoring, fixing bugs, creating files, running scripts, builds or test suites",
				},
			},
		},
	}

	jsonBytes, err := json.Marshal(reqBody)
	if err != nil {
		return HeuristicRoleDecision(task, availableRoles)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return HeuristicRoleDecision(task, availableRoles)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := r.client.Do(httpReq)
	if err != nil {
		log.Printf("[jev-agent-router] request error (falling back to heuristic): %v", err)
		return HeuristicRoleDecision(task, availableRoles)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[jev-agent-router] API returned status %d (falling back to heuristic)", resp.StatusCode)
		return HeuristicRoleDecision(task, availableRoles)
	}

	var jevResp jevResponse
	if err := json.NewDecoder(resp.Body).Decode(&jevResp); err != nil {
		return HeuristicRoleDecision(task, availableRoles)
	}

	roleAns, ok := jevResp.Answers["role"]
	if !ok || roleAns.Choice == "" {
		return HeuristicRoleDecision(task, availableRoles)
	}

	chosenRole := strings.ToLower(strings.TrimSpace(roleAns.Choice))
	confidence := roleAns.Confidence
	if confidence == 0 {
		confidence = 0.95
	}

	reason := ""
	switch chosenRole {
	case "research":
		reason = "Jev classified this task as deep codebase exploration, symbol investigation, or audit without mutating files."
	case "planning":
		reason = "Jev classified this task as strategic architectural planning, goal decomposition, or roadmap synthesis."
	case "coding":
		reason = "Jev classified this task as active code implementation, editing, refactoring, or terminal verification."
	default:
		reason = fmt.Sprintf("Jev assigned role %q based on task requirements.", chosenRole)
	}

	r.recordRoleDecision(chosenRole)

	return &RoleDecision{
		Role:       chosenRole,
		Reason:     reason,
		Confidence: confidence,
		Source:     "jev",
	}
}

// HeuristicRoleDecision determines the optimal sub-agent role using fast keyword and intent heuristics.
func HeuristicRoleDecision(task string, availableRoles []string) *RoleDecision {
	lower := strings.ToLower(task)

	// Check if a specific custom role name from availableRoles is explicitly mentioned in the task
	for _, role := range availableRoles {
		normalized := strings.ToLower(strings.TrimSpace(role))
		if normalized != "" && normalized != "auto" && normalized != "coding" && normalized != "research" && normalized != "planning" {
			if strings.Contains(lower, normalized) {
				return &RoleDecision{
					Role:       normalized,
					Reason:     fmt.Sprintf("Task explicitly matches configured sub-agent role %q.", role),
					Confidence: 0.95,
					Source:     "heuristic",
				}
			}
		}
	}

	planningWords := []string{
		"plan", "roadmap", "architecture", "design", "spec", "specification",
		"break down", "breakdown", "decompose", "strategy", "phases", "milestone",
		"blueprint", "proposal", "step-by-step plan",
	}
	researchWords := []string{
		"research", "investigate", "find where", "where is", "trace", "explore",
		"audit", "analyze", "inspect", "read", "understand", "explain", "locate",
		"how does", "why does", "search for", "look into", "check how", "review codebase",
		"find symbol", "find usage", "list files", "overview",
	}
	codingWords := []string{
		"implement", "fix", "bug", "refactor", "code", "write", "edit",
		"update", "patch", "create", "add", "delete", "remove", "build",
		"test", "run", "install", "script", "benchmark", "optimize code",
		"format", "lint", "compile", "wire", "integrate",
	}

	planningScore := 0
	for _, w := range planningWords {
		if strings.Contains(lower, w) {
			planningScore += 2
		}
	}

	researchScore := 0
	for _, w := range researchWords {
		if strings.Contains(lower, w) {
			researchScore += 2
		}
	}

	codingScore := 0
	for _, w := range codingWords {
		if strings.Contains(lower, w) {
			codingScore += 2
		}
	}

	if planningScore > 0 && planningScore >= researchScore && planningScore >= codingScore {
		return &RoleDecision{
			Role:       "planning",
			Reason:     "Task involves high-level architectural planning, goal decomposition, or roadmap synthesis without file mutations.",
			Confidence: 0.90,
			Source:     "heuristic",
		}
	}

	if researchScore > 0 && researchScore >= codingScore {
		return &RoleDecision{
			Role:       "research",
			Reason:     "Task involves deep codebase exploration, symbol discovery, or literature audit without needing file mutations.",
			Confidence: 0.92,
			Source:     "heuristic",
		}
	}

	if codingScore > 0 {
		return &RoleDecision{
			Role:       "coding",
			Reason:     "Task requires hands-on implementation, bug fixing, file editing, or terminal verification.",
			Confidence: 0.95,
			Source:     "heuristic",
		}
	}

	return &RoleDecision{
		Role:       "coding",
		Reason:     "Default implementation role with full tool and terminal execution permissions.",
		Confidence: 0.80,
		Source:     "heuristic",
	}
}

// DecideSubagent selects the best-suited sub-agent candidate for a task to be delegated.
// It matches task semantics against candidates' names, roles, and descriptions.
func (r *Router) DecideSubagent(ctx context.Context, task string, candidates []SubagentCandidate) *SubagentDecision {
	task = strings.TrimSpace(task)
	if len(candidates) == 0 {
		return &SubagentDecision{
			SelectedID: "coding",
			Name:       "Coding Agent",
			Role:       "coding",
			Reason:     "No custom subagents configured; defaulting to coding agent.",
			Confidence: 0.8,
			Source:     "default",
		}
	}

	if !r.IsEnabled() {
		return HeuristicSubagentDecision(task, candidates)
	}

	r.mu.RLock()
	apiKey := r.apiKey
	endpoint := r.endpoint
	r.mu.RUnlock()

	criteria := make(map[string]string)
	for _, c := range candidates {
		desc := c.Description
		if desc == "" {
			desc = fmt.Sprintf("%s agent with role %s", c.Name, c.Role)
		}
		criteria[c.ID] = fmt.Sprintf("%s: %s (role: %s)", c.Name, desc, c.Role)
	}

	reqBody := jevRequest{
		State: task,
		Model: defaultModel,
		Questions: map[string]jevQuestion{
			"selected_agent": {
				Type:         "choice",
				Instructions: "Which sub-agent is best suited to execute this delegated task?",
				Criteria:     criteria,
			},
		},
	}

	jsonBytes, err := json.Marshal(reqBody)
	if err != nil {
		return HeuristicSubagentDecision(task, candidates)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return HeuristicSubagentDecision(task, candidates)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := r.client.Do(httpReq)
	if err != nil {
		log.Printf("[jev-subagent-router] request error (falling back to heuristic): %v", err)
		return HeuristicSubagentDecision(task, candidates)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[jev-subagent-router] API returned status %d (falling back to heuristic)", resp.StatusCode)
		return HeuristicSubagentDecision(task, candidates)
	}

	var jevResp jevResponse
	if err := json.NewDecoder(resp.Body).Decode(&jevResp); err != nil {
		return HeuristicSubagentDecision(task, candidates)
	}

	ans, ok := jevResp.Answers["selected_agent"]
	if !ok || ans.Choice == "" {
		return HeuristicSubagentDecision(task, candidates)
	}

	chosenID := strings.TrimSpace(ans.Choice)
	var matched *SubagentCandidate
	for _, c := range candidates {
		if strings.EqualFold(c.ID, chosenID) || strings.EqualFold(c.Name, chosenID) {
			matched = &c
			break
		}
	}

	if matched == nil {
		return HeuristicSubagentDecision(task, candidates)
	}

	confidence := ans.Confidence
	if confidence == 0 {
		confidence = 0.95
	}

	reason := fmt.Sprintf("Jev selected %q (%s) as the best match for the delegated task.", matched.Name, matched.Role)
	r.recordRoleDecision(matched.Role)

	return &SubagentDecision{
		SelectedID: matched.ID,
		Name:       matched.Name,
		Role:       matched.Role,
		Reason:     reason,
		Confidence: confidence,
		Source:     "jev",
	}
}

// HeuristicSubagentDecision matches the task against candidate sub-agents using intent and keyword scoring.
func HeuristicSubagentDecision(task string, candidates []SubagentCandidate) *SubagentDecision {
	lowerTask := strings.ToLower(task)

	// First, check for direct name or custom description matches
	bestScore := -1
	var bestCandidate *SubagentCandidate
	var matchReason string

	for i := range candidates {
		c := &candidates[i]
		score := 0
		lowerName := strings.ToLower(c.Name)
		lowerDesc := strings.ToLower(c.Description)

		// Name keyword match
		nameWords := strings.Fields(lowerName)
		for _, w := range nameWords {
			if len(w) > 3 && strings.Contains(lowerTask, w) {
				score += 5
			}
		}

		// Description keyword match
		descWords := strings.Fields(lowerDesc)
		for _, w := range descWords {
			w = strings.Trim(w, ",.;:()\"'")
			if len(w) > 3 && strings.Contains(lowerTask, w) {
				score += 2
			}
		}

		if score > bestScore && score > 2 {
			bestScore = score
			bestCandidate = c
			matchReason = fmt.Sprintf("Task keywords matched the specialization of %q (%s).", c.Name, c.Description)
		}
	}

	if bestCandidate != nil {
		return &SubagentDecision{
			SelectedID: bestCandidate.ID,
			Name:       bestCandidate.Name,
			Role:       bestCandidate.Role,
			Reason:     matchReason,
			Confidence: 0.94,
			Source:     "heuristic",
		}
	}

	// Fallback to role-based heuristic
	roleDec := HeuristicRoleDecision(task, nil)
	// Find candidate matching this role
	for _, c := range candidates {
		if strings.EqualFold(c.Role, roleDec.Role) || strings.EqualFold(c.ID, roleDec.Role) {
			return &SubagentDecision{
				SelectedID: c.ID,
				Name:       c.Name,
				Role:       c.Role,
				Reason:     roleDec.Reason,
				Confidence: roleDec.Confidence,
				Source:     "heuristic",
			}
		}
	}

	// If no candidate directly matched role, return first candidate
	first := candidates[0]
	return &SubagentDecision{
		SelectedID: first.ID,
		Name:       first.Name,
		Role:       first.Role,
		Reason:     roleDec.Reason,
		Confidence: 0.85,
		Source:     "heuristic",
	}
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
		r.mu.Lock()
		r.disabledUntil = time.Now().Add(60 * time.Second)
		r.mu.Unlock()
		log.Printf("[jev-router] request error (falling back to all tools, backing off 60s): %v", err)
		return allTools, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		r.mu.Lock()
		r.disabledUntil = time.Now().Add(60 * time.Second)
		r.mu.Unlock()
		respBytes, _ := io.ReadAll(resp.Body)
		log.Printf("[jev-router] API returned status %d: %s (falling back to all tools, backing off 60s)", resp.StatusCode, string(respBytes))
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
		r.recordRoute("none", len(allTools), 0, len(prompt), time.Since(start))
		return nil, decision
	}

	// 2. Filter tools according to Jev's chosen category
	filtered := filterToolsByCategory(category, prompt, allTools)
	if len(filtered) == 0 {
		r.recordRoute(category, len(allTools), len(allTools), len(prompt), time.Since(start))
		return allTools, decision
	}

	r.recordRoute(category, len(allTools), len(filtered), len(prompt), time.Since(start))
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
	for _, name := range []string{"todo", "update_tasks", "invoke_subagent", "task", "select_subagent", "get_subagent_output", "list_subagents", "ask", "schedule", "cancel_schedule", "list_schedules"} {
		allowed[name] = true
	}

	switch category {
	case "explore":
		// Tools for exploring, searching, listing repo contents
		for _, name := range []string{"index", "find", "glob", "search", "read_directory_files", "read", "load_skill", "skill", "call_skill", "run_skill", "invoke_skill", "list_skills", "python_eval", "python", "todo", "ask"} {
			allowed[name] = true
		}

	case "read":
		// Tools for reading specific files
		for _, name := range []string{"index", "read", "read_multiple", "read_directory_files", "search", "find", "load_skill", "skill", "call_skill", "run_skill", "invoke_skill", "todo", "ask"} {
			allowed[name] = true
		}

	case "modify":
		// Tools for editing, writing, creating files
		for _, name := range []string{"index", "edit", "multiedit", "write", "read", "read_multiple", "search", "find", "git_status", "bash", "load_skill", "skill", "call_skill", "run_skill", "invoke_skill", "eval", "run_code", "python_eval", "python", "todo", "ask"} {
			allowed[name] = true
		}

	case "execute":
		// Tools for running commands, builds, tests
		for _, name := range []string{"bash", "background_shell", "task_output", "task_stop", "task_list", "read", "eval", "run_code", "python_eval", "python", "todo", "ask"} {
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
