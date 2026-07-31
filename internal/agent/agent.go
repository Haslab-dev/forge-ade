package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/hasdev/forge-ade/internal/events"
	"github.com/hasdev/forge-ade/internal/llm"
	"github.com/hasdev/forge-ade/internal/mcp"
	"github.com/hasdev/forge-ade/internal/skills"
	"github.com/hasdev/forge-ade/internal/tools"
)

type RoleFilter string

const (
	RoleCoding   RoleFilter = "coding"
	RolePlanning RoleFilter = "planning"
	RoleResearch RoleFilter = "research"
	RoleCustom   RoleFilter = "custom"
)

type TaskItem struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Completed bool   `json:"completed"`
}

type AgentMessage struct {
	ID        string         `json:"id"`
	Role      string         `json:"role"` // "user", "assistant", "system", "tool"
	Content   string         `json:"content"`
	Reasoning string         `json:"reasoning,omitempty"`
	ToolCalls []llm.ToolCall `json:"tool_calls,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
}

type SessionState string

const (
	StateIdle      SessionState = "idle"
	StateThinking  SessionState = "thinking"
	StateExecuting SessionState = "executing"
	StateAwaiting  SessionState = "awaiting_approval"
)

type Session struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	RoleFilter   RoleFilter     `json:"role_filter"`
	State        SessionState   `json:"state"`
	Folder       string         `json:"folder"`
	ProjectName  string         `json:"project_name,omitempty"`
	Messages     []AgentMessage `json:"messages"`
	Tasks        []TaskItem     `json:"tasks"`
	TokenUsage   llm.TokenStats `json:"token_usage"`
	AutoApprove  bool           `json:"auto_approve"`
	PendingTool  *llm.ToolCall  `json:"pending_tool,omitempty"`
	SystemPrompt string         `json:"system_prompt,omitempty"`
	CustomPrompt string         `json:"custom_prompt,omitempty"`
	CustomRules  string         `json:"custom_rules,omitempty"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

type Manager struct {
	mu          sync.RWMutex
	sessions    map[string]*Session
	definitions map[string]AgentDefinition
	llmClient   *llm.LLMClient
	toolReg     *tools.Registry
	skillMgr    *skills.Manager
	mcpMgr      *mcp.Manager
	bus         *events.Bus
	cancelFuncs map[string]context.CancelFunc
	storePath   string
}

func NewManager(llmClient *llm.LLMClient, toolReg *tools.Registry, skillMgr *skills.Manager, mcpMgr *mcp.Manager, bus *events.Bus, dataDir string) *Manager {
	storePath := filepath.Join(dataDir, "agent_sessions.json")
	m := &Manager{
		sessions:    make(map[string]*Session),
		definitions: make(map[string]AgentDefinition),
		llmClient:   llmClient,
		toolReg:     toolReg,
		skillMgr:    skillMgr,
		mcpMgr:      mcpMgr,
		bus:         bus,
		cancelFuncs: make(map[string]context.CancelFunc),
		storePath:   storePath,
	}
	m.loadSessions()
	m.loadDefinitions()
	return m
}

func (m *Manager) loadSessions() {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(m.storePath)
	if err == nil {
		var list []*Session
		if json.Unmarshal(data, &list) == nil {
			for _, s := range list {
				if s != nil && s.ID != "" {
					s.State = StateIdle
					// Rebuild the system prompt so sessions created before the
					// prompt overhaul pick up style guidance and custom content.
					if !strings.Contains(s.SystemPrompt, "Response style") {
						s.SystemPrompt = buildSystemPrompt(s.RoleFilter, s.CustomPrompt, s.CustomRules)
					}
					m.sessions[s.ID] = s
				}
			}
		}
	}
}

func (m *Manager) saveSessionsLocked() {
	if m.storePath == "" {
		return
	}
	list := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		list = append(list, s)
	}
	_ = os.MkdirAll(filepath.Dir(m.storePath), 0755)
	data, err := json.MarshalIndent(list, "", "  ")
	if err == nil {
		_ = os.WriteFile(m.storePath, data, 0644)
	}
}

func (m *Manager) CreateSession(name string, role RoleFilter, folder string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := uuid.New().String()
	if name == "" {
		name = fmt.Sprintf("%s Agent (%s)", getRoleTitle(role), time.Now().Format("15:04:05"))
	}

	// Normalize folder to an absolute path; when empty, fall back to the
	// process working directory so sessions are always linked to a project.
	projFolder := folder
	if projFolder == "" {
		if cwd, err := os.Getwd(); err == nil {
			projFolder = cwd
		}
	} else if abs, err := filepath.Abs(projFolder); err == nil {
		projFolder = abs
	}

	sysPrompt := buildSystemPrompt(role, "", "")

	sess := &Session{
		ID:           id,
		Name:         name,
		RoleFilter:   role,
		State:        StateIdle,
		Folder:       projFolder,
		ProjectName:  filepath.Base(projFolder),
		Messages:     make([]AgentMessage, 0),
		Tasks:        make([]TaskItem, 0),
		SystemPrompt: sysPrompt,
		UpdatedAt:    time.Now(),
	}

	m.sessions[id] = sess
	m.saveSessionsLocked()
	return sess, nil
}

// UpdateSession updates editable agent session fields (name, role, custom
// prompt, custom rules). The system prompt is rebuilt to include custom content.
func (m *Manager) UpdateSession(id string, name string, role RoleFilter, customPrompt string, customRules string) (*Session, error) {
	m.mu.Lock()

	sess, ok := m.sessions[id]
	if !ok {
		m.mu.Unlock()
		return nil, fmt.Errorf("session %s not found", id)
	}

	if name != "" {
		sess.Name = name
	}
	if role != "" {
		sess.RoleFilter = role
	}
	sess.CustomPrompt = customPrompt
	sess.CustomRules = customRules
	sess.SystemPrompt = buildSystemPrompt(sess.RoleFilter, sess.CustomPrompt, sess.CustomRules)
	sess.UpdatedAt = time.Now()
	m.saveSessionsLocked()
	m.mu.Unlock()

	m.emitSessionUpdate(id)
	return sess, nil
}

func (m *Manager) GetSession(id string) (*Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	sess, ok := m.sessions[id]
	return sess, ok
}

func (m *Manager) ListSessions() []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	list := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		list = append(list, s)
	}
	return list
}

func (m *Manager) DeleteSession(id string) {
	m.mu.Lock()
	if cancel, ok := m.cancelFuncs[id]; ok {
		cancel()
		delete(m.cancelFuncs, id)
	}
	delete(m.sessions, id)
	m.saveSessionsLocked()
	m.mu.Unlock()
	m.emitSessionUpdate(id)
}

func (m *Manager) SendMessage(ctx context.Context, sessionID string, userContent string, mentionedPaths []string) error {
	m.mu.Lock()
	sess, ok := m.sessions[sessionID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("session %s not found", sessionID)
	}

	// Prepare user text with mentioned paths context
	fullContent := userContent
	if len(mentionedPaths) > 0 {
		fullContent += "\n\nReferenced context files/folders:\n" + strings.Join(mentionedPaths, "\n")
	}

	userMsg := AgentMessage{
		ID:        uuid.New().String(),
		Role:      "user",
		Content:   fullContent,
		Timestamp: time.Now(),
	}
	sess.Messages = append(sess.Messages, userMsg)
	sess.State = StateThinking
	m.saveSessionsLocked()
	m.mu.Unlock()

	// EMIT IMMEDIATELY SO USER MESSAGE APPEARS INSTANTLY IN REAL-TIME
	m.emitSessionUpdate(sessionID)

	// Launch background execution loop for agent turn
	execCtx, cancel := context.WithCancel(context.Background())
	m.mu.Lock()
	m.cancelFuncs[sessionID] = cancel
	m.mu.Unlock()

	go m.runAgentTurn(execCtx, sessionID)

	return nil
}

func (m *Manager) runAgentTurn(ctx context.Context, sessionID string) {
	// Build project + skills context outside the lock (git snapshot can take
	// up to a few seconds and must not block session updates).
	skillsCtx := m.buildSkillsContext()

	m.mu.RLock()
	sess, ok := m.sessions[sessionID]
	if !ok {
		m.mu.RUnlock()
		return
	}

	// Build LLM messages payload
	sysContent := sess.SystemPrompt
	if sess.Folder != "" {
		sysContent += buildProjectContext(sess.Folder)
	}
	sysContent += skillsCtx
	messages := []llm.LLMMessage{
		{Role: llm.RoleSystem, Content: sysContent},
	}

	for _, msg := range sess.Messages {
		messages = append(messages, llm.LLMMessage{
			Role:      llm.Role(msg.Role),
			Content:   msg.Content,
			ToolCalls: msg.ToolCalls,
		})
	}
	m.mu.RUnlock()

	// Fetch tool definitions
	toolDefs := m.toolReg.Definitions()

	// Call LLM with streaming callback for real-time thinking and content output
	var lastEmit time.Time
	resp, err := m.llmClient.ChatWithStream(ctx, messages, toolDefs, func(deltaContent string, deltaReasoning string) {
		m.mu.Lock()
		n := len(sess.Messages)
		if n > 0 && sess.Messages[n-1].Role == "assistant" {
			sess.Messages[n-1].Content += deltaContent
			sess.Messages[n-1].Reasoning += deltaReasoning
		} else {
			sess.Messages = append(sess.Messages, AgentMessage{
				ID:        uuid.New().String(),
				Role:      "assistant",
				Content:   deltaContent,
				Reasoning: deltaReasoning,
				Timestamp: time.Now(),
			})
		}
		m.mu.Unlock()

		// Throttle streaming updates to 60ms for smooth UI rendering
		if time.Since(lastEmit) > 60*time.Millisecond {
			lastEmit = time.Now()
			m.emitSessionUpdate(sessionID)
		}
	})
	if err != nil {
		m.mu.Lock()
		sess.State = StateIdle
		sess.Messages = append(sess.Messages, AgentMessage{
			ID:        uuid.New().String(),
			Role:      "assistant",
			Content:   fmt.Sprintf("Error calling LLM: %v", err),
			Timestamp: time.Now(),
		})
		m.saveSessionsLocked()
		m.mu.Unlock()
		m.emitSessionUpdate(sessionID)
		return
	}

	m.mu.Lock()
	sess.State = StateIdle
	m.saveSessionsLocked()
	m.mu.Unlock()
	m.emitSessionUpdate(sessionID)

	m.mu.Lock()
	// Update token usage
	sess.TokenUsage.PromptTokens += resp.TokenUsage.PromptTokens
	sess.TokenUsage.CompletionTokens += resp.TokenUsage.CompletionTokens
	sess.TokenUsage.CachedTokens += resp.TokenUsage.CachedTokens
	sess.TokenUsage.TotalTokens += resp.TokenUsage.TotalTokens

	// Update existing streaming assistant message in-place instead of appending duplicate
	n := len(sess.Messages)
	if n > 0 && sess.Messages[n-1].Role == "assistant" {
		sess.Messages[n-1].Content = resp.Content
		sess.Messages[n-1].Reasoning = resp.Reasoning
		sess.Messages[n-1].ToolCalls = resp.ToolCalls
	} else {
		assistantMsg := AgentMessage{
			ID:        uuid.New().String(),
			Role:      "assistant",
			Content:   resp.Content,
			Reasoning: resp.Reasoning,
			ToolCalls: resp.ToolCalls,
			Timestamp: time.Now(),
		}
		sess.Messages = append(sess.Messages, assistantMsg)
	}

	// Process tool calls if present
	if len(resp.ToolCalls) > 0 {
		firstTool := resp.ToolCalls[0]
		if !sess.AutoApprove && isMutatingTool(firstTool.Function.Name) {
			sess.State = StateAwaiting
			sess.PendingTool = &firstTool
			m.mu.Unlock()
			m.emitSessionUpdate(sessionID)
			return
		}

		// Execute tool immediately
		sess.State = StateExecuting
		m.mu.Unlock()
		m.executeToolCall(ctx, sessionID, firstTool)
		return
	}

	sess.State = StateIdle
	m.mu.Unlock()
	m.emitSessionUpdate(sessionID)
}

func (m *Manager) RespondApproval(ctx context.Context, sessionID string, approve bool, autoApproveAll bool) error {
	m.mu.Lock()
	sess, ok := m.sessions[sessionID]
	if !ok || sess.PendingTool == nil {
		m.mu.Unlock()
		return fmt.Errorf("no pending tool approval for session %s", sessionID)
	}

	pendingTool := *sess.PendingTool
	sess.PendingTool = nil

	if autoApproveAll {
		sess.AutoApprove = true
	}

	if !approve {
		sess.State = StateIdle
		sess.Messages = append(sess.Messages, AgentMessage{
			ID:        uuid.New().String(),
			Role:      "tool",
			Content:   fmt.Sprintf("Tool call %s rejected by user.", pendingTool.Function.Name),
			Timestamp: time.Now(),
		})
		m.mu.Unlock()
		m.emitSessionUpdate(sessionID)
		return nil
	}

	sess.State = StateExecuting
	m.mu.Unlock()

	go m.executeToolCall(ctx, sessionID, pendingTool)
	return nil
}

func (m *Manager) executeToolCall(ctx context.Context, sessionID string, toolCall llm.ToolCall) {
	res, err := m.toolReg.Execute(ctx, toolCall.Function.Name, toolCall.Function.Arguments)
	var contentStr string
	if err != nil {
		contentStr = fmt.Sprintf("Tool Error: %v", err)
	} else {
		b, _ := json.MarshalIndent(res, "", "  ")
		contentStr = string(b)
	}

	m.mu.Lock()
	sess, ok := m.sessions[sessionID]
	if ok {
		sess.Messages = append(sess.Messages, AgentMessage{
			ID:        uuid.New().String(),
			Role:      "tool",
			Content:   contentStr,
			Timestamp: time.Now(),
		})
		sess.State = StateThinking
	}
	m.mu.Unlock()

	m.emitSessionUpdate(sessionID)

	// Continue agent loop for follow up turn
	go m.runAgentTurn(ctx, sessionID)
}

func (m *Manager) ToggleTask(sessionID string, taskID string, completed bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	sess, ok := m.sessions[sessionID]
	if !ok {
		return
	}
	for i := range sess.Tasks {
		if sess.Tasks[i].ID == taskID {
			sess.Tasks[i].Completed = completed
			break
		}
	}
}

func (m *Manager) emitSessionUpdate(sessionID string) {
	if m.bus != nil {
		m.bus.Publish(events.Event{
			Type: "agent:updated",
			Data: map[string]interface{}{"session_id": sessionID},
		})
	}
}

func isMutatingTool(name string) bool {
	return name == "write_file" || name == "run_shell" || name == "edit_file"
}

func getRoleTitle(role RoleFilter) string {
	switch role {
	case RoleCoding:
		return "Coding"
	case RolePlanning:
		return "Planning"
	case RoleResearch:
		return "Research"
	default:
		return "Custom"
	}
}

func buildSystemPrompt(role RoleFilter, customPrompt string, customRules string) string {
	var sb strings.Builder
	sb.WriteString(`You are ForgeADE, an AI coding agent operating inside the user's code editor.
You help with coding, planning, research, and general development tasks.

## Response style (MANDATORY)
- Always respond in clear, natural prose. Never reply with shell snippets, status dumps,
  ` + "`mode=...`" + ` lines, or terse fragment output unless the user explicitly asks for a script.
- Explain what you did, what you found, or what you recommend, using short structured prose.
- Before acting, think through the task step by step. Use tools to gather real context from the
  workspace (read files, search, list directories, check git status) instead of guessing.
- Keep answers focused and skimmable: use short paragraphs and bullets where helpful.
- When you make changes, summarize the changes and suggest how to verify them.

## Working style
- Prefer reading files and searching the workspace before writing code.
- Run tests or build commands with the shell tool when you need to verify changes.
- Respect the user's existing code style and project conventions.
- When a task is ambiguous, ask a clarifying question instead of guessing.

## Tool use
- You have access to tools: read_file, write_file, edit_file, run_shell, list_dir,
  search_workspace, git_status. Use them liberally to ground your answers in the codebase.
- Report tool results in your own words; do not dump raw tool JSON at the user.
`)
	sb.WriteString("\n## Role\n")
	switch role {
	case RoleCoding:
		sb.WriteString("You are operating as a Coding Agent. Focus on writing clean, modular, production-ready code and verifying it with tests/builds.")
	case RolePlanning:
		sb.WriteString("You are operating as a Planning Agent. Break tasks into structured steps and produce plans; avoid mutating code unless explicitly asked.")
	case RoleResearch:
		sb.WriteString("You are operating as a Research Agent. Explore the codebase, trace symbols and dependencies, and synthesize architectural insights.")
	default:
		sb.WriteString("You are operating as a general Assistant. Execute requested tasks efficiently with tools.")
	}
	sb.WriteString("\n")

	if strings.TrimSpace(customPrompt) != "" {
		sb.WriteString("\n## Additional instructions\n")
		sb.WriteString(strings.TrimSpace(customPrompt))
		sb.WriteString("\n")
	}
	if strings.TrimSpace(customRules) != "" {
		sb.WriteString("\n## Rules (always follow)\n")
		sb.WriteString(strings.TrimSpace(customRules))
		sb.WriteString("\n")
	}
	return sb.String()
}

// buildProjectContext attaches the workspace folder, git state, and AGENTS.md
// guidance to the system prompt so agent responses are grounded in the project.
func buildProjectContext(folder string) string {
	var sb strings.Builder
	sb.WriteString("\n## Project context\n")
	sb.WriteString("Workspace folder: ")
	sb.WriteString(folder)
	sb.WriteString("\n")

	// Read AGENTS.md (or .agents/AGENTS.md) conventions if present.
	agentsFile := filepath.Join(folder, "AGENTS.md")
	if _, err := os.Stat(agentsFile); err != nil {
		agentsFile = filepath.Join(folder, ".agents", "AGENTS.md")
	}
	if data, err := os.ReadFile(agentsFile); err == nil && len(data) > 0 {
		sb.WriteString("\nProject conventions (AGENTS.md):\n")
		if len(data) > 8000 {
			data = data[:8000]
		}
		sb.WriteString(string(data))
		sb.WriteString("\n")
	}

	// Lightweight git status snapshot (best effort, fast).
	if out, err := gitSnapshot(folder); err == nil && out != "" {
		sb.WriteString("\nGit status:\n")
		sb.WriteString(out)
		sb.WriteString("\n")
	}

	return sb.String()
}

func gitSnapshot(folder string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	branchCmd := exec.CommandContext(ctx, "git", "branch", "--show-current")
	branchCmd.Dir = folder
	branch, _ := branchCmd.Output()

	statusCmd := exec.CommandContext(ctx, "git", "status", "--porcelain")
	statusCmd.Dir = folder
	out, err := statusCmd.Output()
	if err != nil {
		return "", nil
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		lines = nil
	}
	if len(lines) > 60 {
		lines = lines[:60]
	}

	var sb strings.Builder
	if b := strings.TrimSpace(string(branch)); b != "" {
		sb.WriteString("branch: ")
		sb.WriteString(b)
		sb.WriteString("\n")
	}
	if len(lines) > 0 {
		sb.WriteString("changes:\n")
		for _, l := range lines {
			sb.WriteString("  ")
			sb.WriteString(l)
			sb.WriteString("\n")
		}
	}
	return sb.String(), nil
}

// buildSkillsContext appends loaded SKILL.md knowledge to the system prompt.
func (m *Manager) buildSkillsContext() string {
	if m.skillMgr == nil {
		return ""
	}
	skills := m.skillMgr.List()
	if len(skills) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n## Available skills (expert playbooks)\n")
	sb.WriteString("You can consult these when the task matches their domain. Apply their guidance when relevant.\n")
	for _, s := range skills {
		sb.WriteString("\n### Skill: ")
		sb.WriteString(s.Name)
		if s.Description != "" {
			sb.WriteString(" — ")
			sb.WriteString(s.Description)
		}
		sb.WriteString("\n")
		if s.Body != "" {
			body := s.Body
			if len(body) > 4000 {
				body = body[:4000]
			}
			sb.WriteString(body)
			sb.WriteString("\n")
		}
	}
	return sb.String()
}
