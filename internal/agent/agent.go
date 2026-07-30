package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
	Messages     []AgentMessage `json:"messages"`
	Tasks        []TaskItem     `json:"tasks"`
	TokenUsage   llm.TokenStats `json:"token_usage"`
	AutoApprove  bool           `json:"auto_approve"`
	PendingTool  *llm.ToolCall  `json:"pending_tool,omitempty"`
	SystemPrompt string         `json:"system_prompt,omitempty"`
}

type Manager struct {
	mu          sync.RWMutex
	sessions    map[string]*Session
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
		llmClient:   llmClient,
		toolReg:     toolReg,
		skillMgr:    skillMgr,
		mcpMgr:      mcpMgr,
		bus:         bus,
		cancelFuncs: make(map[string]context.CancelFunc),
		storePath:   storePath,
	}
	m.loadSessions()
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

	sysPrompt := getSystemPrompt(role)

	sess := &Session{
		ID:           id,
		Name:         name,
		RoleFilter:   role,
		State:        StateIdle,
		Folder:       folder,
		Messages:     make([]AgentMessage, 0),
		Tasks:        make([]TaskItem, 0),
		SystemPrompt: sysPrompt,
	}

	m.sessions[id] = sess
	m.saveSessionsLocked()
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
		fullContent += "\n\nReferenced context files/folders:\n" + stringsJoin(mentionedPaths, "\n")
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
	m.mu.RLock()
	sess, ok := m.sessions[sessionID]
	if !ok {
		m.mu.RUnlock()
		return
	}

	// Build LLM messages payload
	messages := []llm.LLMMessage{
		{Role: llm.RoleSystem, Content: sess.SystemPrompt},
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

func getSystemPrompt(role RoleFilter) string {
	switch role {
	case RoleCoding:
		return `You are ForgeADE Coding Agent. Your goal is to write clean, modular, production-ready code.
Use tools to read files, write code, run tests, and check git status.`
	case RolePlanning:
		return `You are ForgeADE Planning Agent. Break down tasks into structured steps without directly mutating code unless requested.`
	case RoleResearch:
		return `You are ForgeADE Research Agent. Explore codebase symbols, dependencies, documentation, and synthesize architectural insights.`
	default:
		return `You are ForgeADE Assistant. Execute requested tasks efficiently with tools.`
	}
}

func stringsJoin(elems []string, sep string) string {
	res := ""
	for i, e := range elems {
		if i > 0 {
			res += sep
		}
		res += e
	}
	return res
}
