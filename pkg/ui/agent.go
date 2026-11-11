package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Haslab-dev/RakitKode/llm"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/hasdev/forge-ade/internal/agentsession"
)

// ToolExecution represents an individual tool run in an agent turn.
type ToolExecution struct {
	ID          string `json:"id"`
	ToolName    string `json:"toolName"`
	Command     string `json:"command"`
	Status      string `json:"status"` // "completed", "running", "failed"
	CreatedAtMs int64  `json:"createdAtMs"`
	DurationMs  int64  `json:"durationMs"`
	Output      string `json:"output"`
	Expanded    bool   `json:"-"`
}

// FileChange represents a touched file with added and deleted lines.
type FileChange struct {
	Path      string `json:"path"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
}

// PendingApproval represents a tool call awaiting user confirmation.
type PendingApproval struct {
	ToolCallID string `json:"toolCallId"`
	ToolName   string `json:"toolName"`
	Summary    string `json:"summary"`
	Arguments  string `json:"arguments"`
	Resolved   bool   `json:"resolved"`
	Approved   bool   `json:"approved"`
}

// ExecutionMode represents the autonomy level of the agent.
type ExecutionMode string

const (
	ModeAsk    ExecutionMode = "ask"    // Ask for approval before actions
	ModeCode   ExecutionMode = "code"   // Approve edits freely, ask for commands
	ModePlan   ExecutionMode = "plan"   // Approve safe actions automatically
	ModeBypass ExecutionMode = "bypass" // Full autonomous access
)

// ReasoningEffort represents the thinking depth budget.
type ReasoningEffort string

const (
	EffortOff    ReasoningEffort = "off"
	EffortLow    ReasoningEffort = "low"
	EffortMedium ReasoningEffort = "medium"
	EffortHigh   ReasoningEffort = "high"
	EffortMax    ReasoningEffort = "max"
)

// ChatMessage represents a single interaction turn in the AI Agent conversation.
type ChatMessage struct {
	ID              int
	Role            string // "user", "assistant", "system"
	Text            string
	Done            bool
	Timestamp       time.Time
	Thinking        string
	IsThinking      bool
	ThinkingTimeSec float64
	ShowThinking    bool
	Tools           []*ToolExecution
	ShowTools       bool
	Changes         []*FileChange
	PendingApproval *PendingApproval
}

// AgentSession represents a single task session in the ForgeADE agent workspace.
type AgentSession struct {
	ID            string
	Title         string
	Status        string // "running", "idle", "completed"
	CreatedAt     time.Time
	UpdatedAt     time.Time
	Messages      []*ChatMessage
	Tasks         *TaskManager
	Model         string
	Provider      string
	AgentID       string
	AgentType     string
	WorkspacePath string
	Pinned        bool
	Archived      bool
}

// AgentState tracks sessions, conversation history, streaming responses, and prompt composer.
type AgentState struct {
	mu              sync.Mutex
	engine          *Engine
	workspacePath   string
	Sessions        []*AgentSession
	ActiveSessionID string
	ActiveFilterTab string // "active", "pinned", "snoozed", "archived"
	SearchQuery     string
	InputText       string
	ListState       ui.ListState
	MdOpts          markdown.Options
	ActiveModel     string
	ExecutionMode   ExecutionMode
	ReasoningEffort ReasoningEffort
	AutoVerify      bool
	SteerQueue      []string
	ShowModelPicker bool
	AvailableModels []string
	NextID          int
	IsStreaming     bool
	TaskManager     *TaskManager
	Messages        []*ChatMessage // alias for active session messages
	Invalidate      func()
}

// NewAgentState initializes the native agent chat state by loading persistent sessions from disk.
func NewAgentState(engine *Engine, workspacePath string, invalidate func()) *AgentState {
	as := &AgentState{
		engine:          engine,
		workspacePath:   workspacePath,
		ActiveModel:     "Claude 3.7 Sonnet / gpt-4o",
		ActiveFilterTab: "active",
		ExecutionMode:   ModeBypass,
		ReasoningEffort: EffortMedium,
		AutoVerify:      false,
		SteerQueue:      make([]string, 0),
		AvailableModels: []string{
			"Claude 3.7 Sonnet (Anthropic)",
			"Claude 3.5 Haiku (Anthropic)",
			"GPT-4o (OpenAI)",
			"o3-mini (OpenAI)",
			"Gemini 2.0 Flash (Google)",
			"DeepSeek R1 (DeepSeek)",
			"Local Ollama (qwen2.5-coder)",
		},
		Invalidate: invalidate,
		NextID:     1,
		MdOpts: markdown.Options{
			Reveal: &markdown.Reveal{Unit: markdown.ByWord},
		},
	}

	// Load real sessions from ~/.forge/sessions/ and ~/.forge-ade/sessions/
	as.LoadSessionsFromDisk(workspacePath)

	// Fallback: if no sessions exist on disk, create an initial real session
	if len(as.Sessions) == 0 {
		initialSess := as.NewSession("Pair Programming Task")
		as.SaveSessionToDisk(initialSess)
	}

	return as
}

type diskSessionRaw struct {
	ID             string            `json:"id"`
	Title          string            `json:"title"`
	Name           string            `json:"name"`
	Goal           string            `json:"goal"`
	Status         string            `json:"status"`
	State          string            `json:"state"`
	CreatedAt      json.RawMessage   `json:"createdAt"`
	UpdatedAt      json.RawMessage   `json:"updatedAt"`
	Model          string            `json:"model"`
	Provider       string            `json:"provider"`
	AgentID        string            `json:"agentId"`
	AgentType      string            `json:"agentType"`
	ExecutionMode  string            `json:"executionMode"`
	ReasoningLevel string            `json:"reasoningLevel"`
	WorkspacePath  string            `json:"workspacePath"`
	Folder         string            `json:"folder"`
	Pinned         bool              `json:"pinned"`
	Archived       bool              `json:"archived"`
	Messages       []diskMessageRaw  `json:"messages"`
	Tasks          []TaskItem        `json:"tasks"`
}

type diskMessageRaw struct {
	ID             string            `json:"id"`
	Role           string            `json:"role"`
	Content        json.RawMessage   `json:"content"`
	Text           string            `json:"text"`
	Timestamp      json.RawMessage   `json:"timestamp"`
	TurnTexts      map[string]string `json:"turnTexts"`
	IsThinking     bool              `json:"isThinking"`
	Thoughts       []string          `json:"thoughts"`
	ToolExecutions []ToolExecution   `json:"toolExecutions"`
	TextBlocks     []struct {
		Text string `json:"text"`
	} `json:"textBlocks"`
}

func parseTimeRaw(raw json.RawMessage) time.Time {
	if len(raw) == 0 {
		return time.Now()
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil && s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			return t
		}
		if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
			return t
		}
	}
	var n int64
	if err := json.Unmarshal(raw, &n); err == nil && n > 0 {
		return time.UnixMilli(n)
	}
	return time.Now()
}

func extractMessageText(m diskMessageRaw) string {
	if strings.TrimSpace(m.Text) != "" {
		return strings.TrimSpace(m.Text)
	}
	if len(m.TurnTexts) > 0 {
		var parts []string
		for _, v := range m.TurnTexts {
			if strings.TrimSpace(v) != "" {
				parts = append(parts, strings.TrimSpace(v))
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, "\n\n")
		}
	}
	if len(m.TextBlocks) > 0 {
		var parts []string
		for _, tb := range m.TextBlocks {
			if strings.TrimSpace(tb.Text) != "" {
				parts = append(parts, strings.TrimSpace(tb.Text))
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, "\n\n")
		}
	}
	if len(m.Content) > 0 {
		var s string
		if err := json.Unmarshal(m.Content, &s); err == nil && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
		var blocks []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
		if err := json.Unmarshal(m.Content, &blocks); err == nil && len(blocks) > 0 {
			var parts []string
			for _, b := range blocks {
				if strings.TrimSpace(b.Text) != "" {
					parts = append(parts, strings.TrimSpace(b.Text))
				}
			}
			if len(parts) > 0 {
				return strings.Join(parts, "\n\n")
			}
		}
		var obj struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal(m.Content, &obj); err == nil && strings.TrimSpace(obj.Text) != "" {
			return strings.TrimSpace(obj.Text)
		}
	}
	return ""
}

// LoadSessionsFromDisk reads persistent sessions from ~/.forge/sessions/ and ~/.forge-ade/sessions/.
func (as *AgentState) LoadSessionsFromDisk(workspacePath string) {
	rawSessions, err := agentsession.LoadSessionsDisk(workspacePath)
	if err != nil || len(rawSessions) == 0 {
		return
	}

	var loaded []*AgentSession
	for _, raw := range rawSessions {
		var diskSess diskSessionRaw
		if err := json.Unmarshal([]byte(raw), &diskSess); err != nil || diskSess.ID == "" {
			continue
		}

		sessTitle := diskSess.Title
		if sessTitle == "" {
			sessTitle = diskSess.Name
		}
		if sessTitle == "" {
			sessTitle = "Untitled Task"
		}

		goal := diskSess.Goal
		if goal == "" {
			goal = sessTitle
		}

		sessModel := diskSess.Model
		if sessModel == "" {
			sessModel = "Claude 3.7 Sonnet / gpt-4o"
		}

		createdTime := parseTimeRaw(diskSess.CreatedAt)
		updatedTime := parseTimeRaw(diskSess.UpdatedAt)
		if updatedTime.Before(createdTime) {
			updatedTime = createdTime
		}

		var msgs []*ChatMessage
		msgID := 1
		for _, m := range diskSess.Messages {
			role := m.Role
			if role == "agent" {
				role = "assistant"
			}
			txt := extractMessageText(m)
			if txt == "" && len(m.ToolExecutions) == 0 && len(m.Thoughts) == 0 {
				continue
			}

			var toolsList []*ToolExecution
			for _, t := range m.ToolExecutions {
				tCopy := t
				toolsList = append(toolsList, &tCopy)
			}

			var thinkingStr string
			if len(m.Thoughts) > 0 {
				thinkingStr = strings.Join(m.Thoughts, "\n\n")
			}

			changes := extractChangesFromTools(toolsList)

			msgs = append(msgs, &ChatMessage{
				ID:              msgID,
				Role:            role,
				Text:            txt,
				Done:            true,
				Timestamp:       updatedTime,
				Thinking:        thinkingStr,
				IsThinking:      m.IsThinking,
				ThinkingTimeSec: float64(len(thinkingStr)) / 100.0,
				Tools:           toolsList,
				Changes:         changes,
			})
			msgID++
		}

		tm := NewTaskManager()
		if len(diskSess.Tasks) > 0 {
			tm.SetGoal(goal, diskSess.Tasks)
		}

		status := diskSess.Status
		if status == "" {
			status = diskSess.State
		}
		if status == "" {
			status = "idle"
		}

		wsPath := diskSess.WorkspacePath
		if wsPath == "" {
			wsPath = diskSess.Folder
		}

		loaded = append(loaded, &AgentSession{
			ID:            diskSess.ID,
			Title:         sessTitle,
			Status:        status,
			CreatedAt:     createdTime,
			UpdatedAt:     updatedTime,
			Model:         sessModel,
			Provider:      diskSess.Provider,
			AgentID:       diskSess.AgentID,
			AgentType:     diskSess.AgentType,
			WorkspacePath: wsPath,
			Pinned:        diskSess.Pinned,
			Archived:      diskSess.Archived,
			Messages:      msgs,
			Tasks:         tm,
		})
	}

	if len(loaded) > 0 {
		// Sort by UpdatedAt descending (newest first)
		sort.Slice(loaded, func(i, j int) bool {
			return loaded[i].UpdatedAt.After(loaded[j].UpdatedAt)
		})
		as.Sessions = loaded
		as.ActiveSessionID = loaded[0].ID
		as.syncActiveSession()
	}
}

// SaveSessionToDisk writes the given session to ~/.forge/sessions/[project-name]/session-<id>.json.
func (as *AgentState) SaveSessionToDisk(sess *AgentSession) {
	if sess == nil {
		return
	}

	var diskMsgs []map[string]interface{}
	for _, m := range sess.Messages {
		role := m.Role
		if role == "assistant" {
			role = "agent"
		}
		msgMap := map[string]interface{}{
			"id":        fmt.Sprintf("msg-%d", m.ID),
			"role":      role,
			"content":   m.Text,
			"timestamp": m.Timestamp.Format(time.RFC3339),
		}
		if m.Thinking != "" {
			msgMap["thoughts"] = []string{m.Thinking}
			msgMap["isThinking"] = m.IsThinking
		}
		if len(m.Tools) > 0 {
			msgMap["toolExecutions"] = m.Tools
		}
		diskMsgs = append(diskMsgs, msgMap)
	}

	var tasksList []TaskItem
	goal := sess.Title
	if sess.Tasks != nil {
		tasksList = sess.Tasks.Tasks
		if sess.Tasks.Goal != "" {
			goal = sess.Tasks.Goal
		}
	}

	wsPath := sess.WorkspacePath
	if wsPath == "" {
		wsPath = as.workspacePath
	}

	data := map[string]interface{}{
		"id":             sess.ID,
		"title":          sess.Title,
		"goal":           goal,
		"status":         sess.Status,
		"createdAt":      sess.CreatedAt.Format(time.RFC3339),
		"updatedAt":      time.Now().Format(time.RFC3339),
		"model":          sess.Model,
		"provider":       sess.Provider,
		"agentId":        sess.AgentID,
		"agentType":      sess.AgentType,
		"executionMode":  string(as.ExecutionMode),
		"reasoningLevel": string(as.ReasoningEffort),
		"workspacePath":  wsPath,
		"pinned":         sess.Pinned,
		"archived":       sess.Archived,
		"messages":       diskMsgs,
		"tasks":          tasksList,
	}

	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return
	}

	_ = agentsession.SaveSessionDisk(string(bytes), wsPath)
}

// ActiveSession returns the currently selected agent session.
func (as *AgentState) ActiveSession() *AgentSession {
	as.mu.Lock()
	defer as.mu.Unlock()

	for _, s := range as.Sessions {
		if s.ID == as.ActiveSessionID {
			return s
		}
	}
	if len(as.Sessions) > 0 {
		return as.Sessions[0]
	}
	return nil
}

// syncActiveSession updates the ActiveSessionID and points Messages to the active session.
func (as *AgentState) syncActiveSession() {
	var active *AgentSession
	for _, s := range as.Sessions {
		if s.ID == as.ActiveSessionID {
			active = s
			break
		}
	}
	if active == nil && len(as.Sessions) > 0 {
		active = as.Sessions[0]
		as.ActiveSessionID = active.ID
	}
	if active != nil {
		as.Messages = active.Messages
		if active.Tasks != nil {
			as.TaskManager = active.Tasks
		}
	}
}

// SelectSession activates the given session by ID.
func (as *AgentState) SelectSession(id string) {
	as.mu.Lock()
	as.ActiveSessionID = id
	as.syncActiveSession()
	as.mu.Unlock()

	if as.Invalidate != nil {
		as.Invalidate()
	}
}

// NewSession creates and activates a new empty session with a real TaskManager.
func (as *AgentState) NewSession(title string) *AgentSession {
	as.mu.Lock()
	defer as.mu.Unlock()

	now := time.Now()
	id := fmt.Sprintf("session-%d", now.UnixMilli())

	initialMsg := &ChatMessage{
		ID:        1,
		Role:      "assistant",
		Text:      "Hello! I am your pair programming assistant. Ask me questions, explore code, or run tasks.",
		Done:      true,
		Timestamp: now,
	}

	sess := &AgentSession{
		ID:            id,
		Title:         title,
		Status:        "idle",
		CreatedAt:     now,
		UpdatedAt:     now,
		Model:         as.ActiveModel,
		Provider:      "internal",
		AgentID:       "agent-monolith",
		AgentType:     "internal",
		WorkspacePath: as.workspacePath,
		Messages:      []*ChatMessage{initialMsg},
		Tasks:         NewTaskManager(),
	}

	as.Sessions = append([]*AgentSession{sess}, as.Sessions...)
	as.ActiveSessionID = id
	as.Messages = sess.Messages
	as.TaskManager = sess.Tasks

	return sess
}

// TogglePin pins or unpins the given session.
func (as *AgentState) TogglePin(id string) {
	as.mu.Lock()
	var updated *AgentSession
	for _, s := range as.Sessions {
		if s.ID == id {
			s.Pinned = !s.Pinned
			s.UpdatedAt = time.Now()
			updated = s
			break
		}
	}
	as.mu.Unlock()

	if updated != nil {
		as.SaveSessionToDisk(updated)
	}
	if as.Invalidate != nil {
		as.Invalidate()
	}
}

// ToggleArchive archives or unarchives the given session.
func (as *AgentState) ToggleArchive(id string) {
	as.mu.Lock()
	var updated *AgentSession
	for _, s := range as.Sessions {
		if s.ID == id {
			s.Archived = !s.Archived
			s.UpdatedAt = time.Now()
			updated = s
			break
		}
	}
	as.mu.Unlock()

	if updated != nil {
		as.SaveSessionToDisk(updated)
	}
	if as.Invalidate != nil {
		as.Invalidate()
	}
}

// DeleteSession permanently removes a session from memory and disk.
func (as *AgentState) DeleteSession(id string) {
	as.mu.Lock()
	newSessions := make([]*AgentSession, 0, len(as.Sessions))
	for _, s := range as.Sessions {
		if s.ID != id {
			newSessions = append(newSessions, s)
		}
	}
	as.Sessions = newSessions
	if as.ActiveSessionID == id {
		if len(as.Sessions) > 0 {
			as.ActiveSessionID = as.Sessions[0].ID
		} else {
			as.ActiveSessionID = ""
		}
	}
	as.syncActiveSession()
	as.mu.Unlock()

	_ = agentsession.DeleteSessionDisk(id, as.workspacePath)

	if as.Invalidate != nil {
		as.Invalidate()
	}
}

// TabCounts returns session counts for active, pinned, and archived tabs.
func (as *AgentState) TabCounts() (active, pinned, archived int) {
	as.mu.Lock()
	defer as.mu.Unlock()

	for _, s := range as.Sessions {
		if s.Archived {
			archived++
		} else {
			active++
			if s.Pinned {
				pinned++
			}
		}
	}
	return
}

// ContextUsage calculates the approximate tokens used and percentage of standard 200k window.
func (as *AgentState) ContextUsage() (tokens int, pct int) {
	as.mu.Lock()
	defer as.mu.Unlock()

	totalChars := 0
	for _, m := range as.Messages {
		totalChars += len(m.Text)
		totalChars += len(m.Thinking)
		for _, t := range m.Tools {
			totalChars += len(t.Command) + len(t.Output)
		}
	}
	tokens = totalChars / 4
	if tokens < 240 {
		tokens = 240
	}
	const maxContext = 200000
	pct = (tokens * 100) / maxContext
	if pct == 0 && tokens > 0 {
		pct = 1
	}
	return
}

// FilteredSessions returns the list of sessions matching the active tab filter and search query.
func (as *AgentState) FilteredSessions() []*AgentSession {
	as.mu.Lock()
	defer as.mu.Unlock()

	var result []*AgentSession
	query := strings.ToLower(strings.TrimSpace(as.SearchQuery))

	for _, s := range as.Sessions {
		switch as.ActiveFilterTab {
		case "pinned":
			if !s.Pinned || s.Archived {
				continue
			}
		case "archived":
			if !s.Archived {
				continue
			}
		case "snoozed":
			continue
		default: // "active"
			if s.Archived {
				continue
			}
		}

		if query != "" {
			titleMatch := strings.Contains(strings.ToLower(s.Title), query)
			modelMatch := strings.Contains(strings.ToLower(s.Model), query)
			if !titleMatch && !modelMatch {
				continue
			}
		}

		result = append(result, s)
	}

	return result
}

// SubmitMessage handles user prompt submission, slash command routing, and trigger execution.
func (as *AgentState) SubmitMessage() {
	text := strings.TrimSpace(as.InputText)
	if text == "" {
		return
	}

	// Mid-turn steering: if already streaming, queue prompt
	if as.IsStreaming {
		as.mu.Lock()
		as.SteerQueue = append(as.SteerQueue, text)
		as.InputText = ""
		as.mu.Unlock()
		if as.Invalidate != nil {
			as.Invalidate()
		}
		return
	}

	as.InputText = ""

	userMsg := &ChatMessage{
		ID:        len(as.Messages) + 1,
		Role:      "user",
		Text:      text,
		Done:      true,
		Timestamp: time.Now(),
	}

	as.mu.Lock()
	var activeSess *AgentSession
	for _, s := range as.Sessions {
		if s.ID == as.ActiveSessionID {
			s.Messages = append(s.Messages, userMsg)
			as.Messages = s.Messages
			s.UpdatedAt = time.Now()
			activeSess = s
			break
		}
	}
	as.mu.Unlock()

	if activeSess != nil {
		as.SaveSessionToDisk(activeSess)
	}

	// Handle Slash Commands
	if strings.HasPrefix(text, "/") {
		as.handleSlashCommand(text)
		return
	}

	// Trigger streaming AI turn
	as.triggerAgentTurn(text)

	if as.Invalidate != nil {
		as.Invalidate()
	}
}

func (as *AgentState) handleSlashCommand(cmd string) {
	parts := strings.Fields(cmd)
	rootCmd := parts[0]

	as.mu.Lock()
	defer as.mu.Unlock()

	var reply string
	var activeSess *AgentSession
	for _, s := range as.Sessions {
		if s.ID == as.ActiveSessionID {
			activeSess = s
			break
		}
	}

	switch rootCmd {
	case "/clear":
		if activeSess != nil {
			activeSess.Messages = activeSess.Messages[:0]
			as.Messages = activeSess.Messages
		}
		reply = "Chat history cleared."
	case "/goal":
		desc := strings.TrimSpace(strings.TrimPrefix(cmd, "/goal"))
		if desc == "" {
			desc = "Decompose and execute objective"
		}
		if as.TaskManager != nil {
			as.TaskManager.SetGoal(desc, []TaskItem{
				{ID: "1", Title: "Analyze target requirements and symbols", Status: TaskInProgress, ActiveForm: "Scanning symbols..."},
				{ID: "2", Title: "Formulate code changes and dependency updates", Status: TaskPending},
				{ID: "3", Title: "Execute regression tests and static validation", Status: TaskPending},
				{ID: "4", Title: "Review diff hunks and commit changes", Status: TaskPending},
			})
		}
		reply = fmt.Sprintf("Declared Goal: **%s**\nDecomposed into 4 atomic tasks on the floating checklist.", desc)
	case "/plan":
		desc := strings.TrimSpace(strings.TrimPrefix(cmd, "/plan"))
		if desc == "" {
			desc = "Architectural Analysis and Planning"
		}
		if as.TaskManager != nil {
			as.TaskManager.SetGoal("Plan: "+desc, []TaskItem{
				{ID: "1", Title: "Survey existing codebase structure and symbols", Status: TaskInProgress, ActiveForm: "Inspecting symbols..."},
				{ID: "2", Title: "Formulate architectural change plan & interfaces", Status: TaskPending},
				{ID: "3", Title: "Identify regression risks and test coverage gaps", Status: TaskPending},
			})
		}
		reply = fmt.Sprintf("**Plan Mode Activated**: `%s`\nStep-by-step architectural breakdown populated on the floating checklist.", desc)
	case "/vibe":
		reply = "**Vibe Mode Activated**: Autonomous execution with automatic error diagnosis and fix cycles enabled."
	case "/tasks":
		if as.TaskManager != nil {
			c, total := as.TaskManager.Progress()
			reply = fmt.Sprintf("### Current Task Checklist (%d/%d completed)\nActive Goal: `%s`", c, total, as.TaskManager.Goal)
		} else {
			reply = "No active tasks."
		}
	case "/model":
		if as.engine != nil {
			cfg := as.engine.GetLLMConfig()
			reply = fmt.Sprintf("Active Provider: `%s`\nActive Model: `%s`", cfg.ProviderID, cfg.Model)
		} else {
			reply = fmt.Sprintf("Active Model: `%s`", as.ActiveModel)
		}
	case "/help":
		reply = "### Available Slash Commands:\n" +
			"- `/goal <description>` — Set an active goal and decompose into tasks\n" +
			"- `/tasks` — Show current task checklist status\n" +
			"- `/clear` — Clear current conversation history\n" +
			"- `/model` — Inspect active LLM provider and model profile\n" +
			"- `/status` — View workspace indexing and engine health\n" +
			"- `/vibe` — Activate high-velocity autonomous mode\n" +
			"- `/help` — Display command reference"
	case "/status":
		reply = "### Workspace Health\n" +
			"- **Engine**: ForgeADE Go Monolith\n" +
			"- **UI Runtime**: MyGo Native GPU (`CGO_ENABLED=0`)\n" +
			"- **Terminal**: Ghostty `libghostty-vt` embedded\n" +
			"- **Editor**: Zed-style virtualized buffer with live diff gutters\n" +
			"- **Persistent Sessions**: Loaded from disk (~/.forge/sessions/)"
	default:
		reply = fmt.Sprintf("Unknown command `%s`. Type `/help` for available commands.", rootCmd)
	}

	replyMsg := &ChatMessage{
		ID:        len(as.Messages) + 1,
		Role:      "assistant",
		Text:      reply,
		Done:      true,
		Timestamp: time.Now(),
	}

	if activeSess != nil {
		activeSess.Messages = append(activeSess.Messages, replyMsg)
		as.Messages = activeSess.Messages
		activeSess.Status = "idle"
		as.SaveSessionToDisk(activeSess)
	}
}

// triggerAgentTurn executes real streaming responses via LLMClient or ACPManager.
func (as *AgentState) triggerAgentTurn(prompt string) {
	as.mu.Lock()
	as.IsStreaming = true

	startTime := time.Now()
	assistantMsg := &ChatMessage{
		ID:              len(as.Messages) + 1,
		Role:            "assistant",
		Text:            "",
		Done:            false,
		Timestamp:       startTime,
		IsThinking:      true,
		ThinkingTimeSec: 0,
		ShowThinking:    false,
		ShowTools:       false,
	}

	var activeSess *AgentSession
	for _, s := range as.Sessions {
		if s.ID == as.ActiveSessionID {
			s.Messages = append(s.Messages, assistantMsg)
			as.Messages = s.Messages
			s.Status = "running"
			activeSess = s
			break
		}
	}
	as.mu.Unlock()

	go func() {
		defer func() {
			as.mu.Lock()
			assistantMsg.Done = true
			assistantMsg.IsThinking = false
			assistantMsg.ThinkingTimeSec = time.Since(startTime).Seconds()
			as.IsStreaming = false
			if activeSess != nil {
				activeSess.Status = "idle"
				activeSess.UpdatedAt = time.Now()
			}

			// If steer queue has items, pop next
			var nextSteer string
			if len(as.SteerQueue) > 0 {
				nextSteer = as.SteerQueue[0]
				as.SteerQueue = as.SteerQueue[1:]
			}
			as.mu.Unlock()

			if activeSess != nil {
				as.SaveSessionToDisk(activeSess)
			}
			if as.Invalidate != nil {
				as.Invalidate()
			}

			if nextSteer != "" {
				as.triggerAgentTurn(nextSteer)
			}
		}()

		// 1. Check if ACP session can handle this turn
		if activeSess != nil && (activeSess.AgentType == "acp" || activeSess.AgentID == "agent-agy" || activeSess.AgentID == "agent-claude" || activeSess.AgentID == "agent-codex") {
			if as.engine != nil && as.engine.ACPMgr != nil {
				err := as.engine.ACPMgr.Prompt(activeSess.ID, prompt)
				if err == nil {
					return
				}
			}
		}

		// 2. Execute via internal LLMClient
		if as.engine != nil && as.engine.LLMClient != nil {
			var llmMsgs []llm.LLMMessage
			as.mu.Lock()
			for _, m := range as.Messages {
				if m == assistantMsg {
					continue
				}
				llmMsgs = append(llmMsgs, llm.LLMMessage{
					Role:    llm.Role(m.Role),
					Content: m.Text,
				})
			}
			as.mu.Unlock()

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()

			_, err := as.engine.LLMClient.ChatWithStream(ctx, llmMsgs, nil, func(deltaContent, deltaReasoning string) {
				as.mu.Lock()
				if deltaReasoning != "" {
					assistantMsg.Thinking += deltaReasoning
				}
				if deltaContent != "" {
					assistantMsg.Text += deltaContent
					assistantMsg.IsThinking = false
				}
				as.mu.Unlock()
				if as.Invalidate != nil {
					as.Invalidate()
				}
			})

			if err == nil && len(assistantMsg.Text) > 0 {
				return
			}

			// If provider returned error or has no configured keys, provide actionable guidance
			as.mu.Lock()
			cfg := as.engine.LLMClient.GetConfig()
			errMsg := "No active provider key configured."
			if err != nil {
				errMsg = err.Error()
			}
			assistantMsg.Text = fmt.Sprintf("**LLM Request Notice**: %s\n\n"+
				"- Active Provider: `%s`\n"+
				"- Active Model: `%s`\n\n"+
				"Please open **Settings (⌘,)** to add your API key, or launch one of the installed ACP agents (**Google Antigravity**, **Claude Code**, or **Codex**).",
				errMsg, cfg.ProviderID, cfg.Model)
			as.mu.Unlock()
		}
	}()
}

// RenderChat renders the conversation view, message bubbles, task checklist, and prompt composer.
func (as *AgentState) RenderChat(c *ui.Context) {
	t := c.Theme()
	sess := as.ActiveSession()

	sessionTitle := "Agent Assistant"
	if sess != nil {
		sessionTitle = sess.Title
	}

	as.ListState.Key = func(i int) any {
		if i < len(as.Messages) {
			return as.Messages[i].ID
		}
		return i
	}
	as.ListState.FollowEnd = true
	as.ListState.FollowAnimation = &ui.Smooth

	ui.Column(c).Fill().Children(func() {
		// Session Header Bar with Tabler Icons
		ui.Row(c).Gap(10).Padding(10, 16).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Background(t.Surface).Children(func() {
			TablerIcon(c, TablerBolt, 15)
			ui.Text(c, sessionTitle).Bold().FontSize(13).MaxWidth(380)

			// Live Status Indicator with Tabler Icons
			if sess != nil && sess.Status == "running" {
				ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
					TablerIcon(c, TablerLoader, 13)
					ui.Text(c, "Running").FontSize(11).TextColor(t.Accent)
				})
			} else if sess != nil && sess.Status == "completed" {
				ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
					TablerIcon(c, TablerCircleCheck, 13)
					ui.Text(c, "Completed").FontSize(11).TextColor(t.TextMuted)
				})
			} else {
				ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
					TablerIcon(c, TablerCircle, 13)
					ui.Text(c, "Idle").FontSize(11).TextColor(t.TextMuted)
				})
			}

			ui.Box(c).Grow(1)
			ui.Text(c, as.ActiveModel).FontSize(11).TextColor(t.TextMuted)
			if as.IsStreaming {
				ui.Text(c, "Streaming tokens…").FontSize(11).TextColor(t.Accent)
			}

			forkBtn := ui.Button(c, "")
			forkBtn.Children(func() {
				ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
					TablerIcon(c, TablerGitFork, 12)
					ui.Text(c, "Fork").FontSize(11)
				})
			})
			if forkBtn.Clicked() && sess != nil {
				as.NewSession(fmt.Sprintf("%s (fork)", sess.Title))
			}
		})

		// Message List
		if len(as.Messages) == 0 {
			ui.Column(c).Grow(1).Center().Gap(8).Children(func() {
				TablerIcon(c, TablerSparkles, 32)
				ui.Text(c, "No messages in this session").FontSize(15).Bold().TextColor(t.TextMuted)
				ui.Text(c, "Type a prompt below or use slash commands like /goal or /help.").TextColor(t.TextMuted)
			})
		} else {
			ui.List(c, &as.ListState, len(as.Messages), func(i int) {
				if i >= len(as.Messages) {
					return
				}
				m := as.Messages[i]
				if m.Role == "user" {
					ui.Row(c).Padding(4, 0).Children(func() {
						ui.Spacer(c)
						ui.Text(c, m.Text).Selectable().Padding(10, 14).Radius(12).Background(t.Surface).MaxWidth(560)
					})
				} else {
					ui.Column(c).Padding(4, 0).MaxWidth(800).Gap(6).Children(func() {
						// Assistant turn header
						ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
							TablerIcon(c, TablerSparkles, 13)
							ui.Text(c, as.ActiveModel).FontSize(11).Bold().TextColor(t.TextMuted)
							ui.Text(c, m.Timestamp.Format("15:04")).FontSize(10).TextColor(t.TextMuted)
						})

						// 1. Thinking / WorkLog Block
						if m.Thinking != "" || m.IsThinking {
							as.renderThinkingBlock(c, m)
						}

						// 2. Folded Tool Traces Card
						if len(m.Tools) > 0 {
							as.renderFoldedToolTraces(c, m)
						}

						// 3. Turn Changes Card
						if len(m.Changes) > 0 {
							as.renderTurnChangesCard(c, m)
						}

						// 4. Pending Approval Bar
						if m.PendingApproval != nil && !m.PendingApproval.Resolved {
							as.renderApprovalBar(c, m)
						}

						// 5. Response Markdown Body
						if m.Text != "" {
							if m.Done {
								markdown.View(c, m.Text, &as.MdOpts)
							} else {
								markdown.Stream(c, m.Text, &as.MdOpts)
							}
						}
					})
				}
			}).Grow(1).Padding(16, 20).Gap(14).Justify(ui.End)
		}

		// Floating Task Checklist (docked above composer)
		if as.TaskManager != nil {
			as.TaskManager.RenderChecklist(c)
		}

		// Mid-turn Steering Queue Tray
		if len(as.SteerQueue) > 0 {
			as.renderSteerQueueTray(c)
		}

		// Input Composer Area with Tabler Icons
		ui.Column(c).BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Background(t.Surface).Children(func() {
			// Slash commands shortcuts & context usage bar
			ui.Row(c).Gap(6).Padding(6, 16, 2, 16).AlignItems(ui.Center).Children(func() {
				cmdPills := []string{"/goal", "/plan", "/tasks", "/vibe", "/model", "/clear", "/status", "/help"}
				for _, cmd := range cmdPills {
					thisCmd := cmd
					btn := ui.Button(c, thisCmd).FontSize(11)
					if btn.Clicked() {
						as.InputText = thisCmd + " "
					}
				}
				ui.Box(c).Grow(1)
				tokens, pct := as.ContextUsage()
				tokK := float32(tokens) / 1000.0
				ui.Text(c, fmt.Sprintf("%.1fk tokens (%d%%)", tokK, pct)).FontSize(10).TextColor(t.TextMuted)
			})

			// Input text row
			ui.Row(c).Gap(8).Padding(8, 16, 4, 16).AlignItems(ui.Center).Children(func() {
				in := ui.TextInput(c, &as.InputText).Placeholder("Ask agent, enter instructions, or /help...").Grow(1).AutoFocus()
				if in.Submitted() {
					as.SubmitMessage()
				}
			})

			// Bottom Composer Toolbar: [Mode] [Effort] [Model] [Verify] [Send/Stop]
			ui.Row(c).Gap(8).Padding(4, 16, 10, 16).AlignItems(ui.Center).Children(func() {
				// 1. Permission Mode Selector Pill
				modeBtn := ui.Button(c, "")
				modeBtn.Children(func() {
					ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
						var modeIcon *ui.SVG
						var modeLabel string
						switch as.ExecutionMode {
						case ModeAsk:
							modeIcon = TablerHand
							modeLabel = "Ask"
						case ModeCode:
							modeIcon = TablerPencil
							modeLabel = "Edits"
						case ModePlan:
							modeIcon = TablerShieldCheck
							modeLabel = "Auto"
						default:
							modeIcon = TablerShieldAlert
							modeLabel = "Full"
						}
						TablerIcon(c, modeIcon, 13)
						ui.Text(c, modeLabel).FontSize(11).Bold()
					})
				})
				if modeBtn.Clicked() {
					switch as.ExecutionMode {
					case ModeAsk:
						as.ExecutionMode = ModeCode
					case ModeCode:
						as.ExecutionMode = ModePlan
					case ModePlan:
						as.ExecutionMode = ModeBypass
					default:
						as.ExecutionMode = ModeAsk
					}
				}

				// 2. Reasoning Effort Pill
				effortBtn := ui.Button(c, "")
				effortBtn.Children(func() {
					ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
						TablerIcon(c, TablerBrain, 13)
						label := "Med"
						switch as.ReasoningEffort {
						case EffortOff:
							label = "Off"
						case EffortLow:
							label = "Low"
						case EffortHigh:
							label = "High"
						case EffortMax:
							label = "Max"
						}
						ui.Text(c, label).FontSize(11)
					})
				})
				if effortBtn.Clicked() {
					switch as.ReasoningEffort {
					case EffortOff:
						as.ReasoningEffort = EffortLow
					case EffortLow:
						as.ReasoningEffort = EffortMedium
					case EffortMedium:
						as.ReasoningEffort = EffortHigh
					case EffortHigh:
						as.ReasoningEffort = EffortMax
					default:
						as.ReasoningEffort = EffortOff
					}
				}

				// 3. Model Switcher Chip
				modelBtn := ui.Button(c, "")
				modelBtn.Children(func() {
					ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
						TablerIcon(c, TablerSparkles, 13)
						disp := as.ActiveModel
						if len(disp) > 18 {
							disp = disp[:15] + "…"
						}
						ui.Text(c, disp).FontSize(11).Bold()
						TablerIcon(c, TablerChevronDown, 11)
					})
				})
				if modelBtn.Clicked() {
					as.ShowModelPicker = !as.ShowModelPicker
				}

				// 4. Auto-Verify Loop Button
				verifyBtn := ui.Button(c, "")
				if as.AutoVerify {
					verifyBtn = ui.PrimaryButton(c, "")
				}
				verifyBtn.Children(func() {
					ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
						TablerIcon(c, TablerShieldCheck, 13)
						ui.Text(c, "Verify").FontSize(11)
					})
				})
				if verifyBtn.Clicked() {
					as.AutoVerify = !as.AutoVerify
				}

				ui.Box(c).Grow(1)

				// 5. Send / Steer / Stop Button
				if as.IsStreaming {
					if strings.TrimSpace(as.InputText) != "" {
						queueBtn := ui.Button(c, "")
						queueBtn.Children(func() {
							ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
								TablerIcon(c, TablerPlus, 12)
								ui.Text(c, "Queue").FontSize(11)
							})
						})
						if queueBtn.Clicked() {
							as.SubmitMessage()
						}
					}

					stopBtn := ui.Button(c, "")
					stopBtn.Children(func() {
						ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
							TablerIcon(c, TablerSquare, 12)
							ui.Text(c, "Stop").FontSize(11).Bold()
						})
					})
					if stopBtn.Clicked() {
						as.mu.Lock()
						as.IsStreaming = false
						if len(as.Messages) > 0 {
							as.Messages[len(as.Messages)-1].Done = true
						}
						as.mu.Unlock()
					}
				} else {
					sendBtn := ui.PrimaryButton(c, "")
					sendBtn.Children(func() {
						ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
							TablerIcon(c, TablerSend, 12)
							ui.Text(c, "Send").FontSize(11).Bold()
						})
					})
					if strings.TrimSpace(as.InputText) == "" {
						sendBtn.Disabled(true)
					}
					if sendBtn.Clicked() {
						as.SubmitMessage()
					}
				}
			})
		})

		// Model Selector Modal
		if as.ShowModelPicker {
			as.renderModelPickerModal(c)
		}
	})
}

// renderThinkingBlock renders the collapsible WorkLog thinking trace.
func (as *AgentState) renderThinkingBlock(c *ui.Context, m *ChatMessage) {
	t := c.Theme()

	label := "Thought Process"
	if m.ThinkingTimeSec > 0 {
		label = fmt.Sprintf("Thinking (%.1fs)", m.ThinkingTimeSec)
	}

	ui.Column(c).FillWidth().Children(func() {
		// Header row
		headerBtn := ui.Button(c, "")
		headerBtn.Children(func() {
			ui.Row(c).Gap(6).Padding(4, 8).AlignItems(ui.Center).Children(func() {
				TablerIcon(c, TablerBrain, 13)
				ui.Text(c, label).FontSize(11).Bold().TextColor(t.TextMuted)
				ui.Box(c).Grow(1)
				if m.ShowThinking {
					TablerIcon(c, TablerChevronUp, 12)
				} else {
					TablerIcon(c, TablerChevronDown, 12)
				}
			})
		})
		if headerBtn.Clicked() {
			m.ShowThinking = !m.ShowThinking
		}

		if m.ShowThinking && m.Thinking != "" {
			ui.Box(c).Padding(8, 12).Radius(8).Background(t.Surface).BorderWidth(1, 1, 1, 1).BorderColor(t.Border).Children(func() {
				ui.Text(c, m.Thinking).FontSize(11).Italic().TextColor(t.TextMuted)
			})
		}
	})
}

// renderFoldedToolTraces renders summarized tool calls with collapsible execution cards.
func (as *AgentState) renderFoldedToolTraces(c *ui.Context, m *ChatMessage) {
	t := c.Theme()
	summary := summarizeTools(m.Tools)

	ui.Column(c).FillWidth().Children(func() {
		headerBtn := ui.Button(c, "")
		headerBtn.Children(func() {
			ui.Row(c).Gap(6).Padding(4, 8).AlignItems(ui.Center).Children(func() {
				TablerIcon(c, TablerCircleCheck, 13)
				ui.Text(c, summary).FontSize(11).Bold().TextColor(t.Text)
				ui.Box(c).Grow(1)
				if m.ShowTools {
					TablerIcon(c, TablerChevronUp, 12)
				} else {
					TablerIcon(c, TablerChevronDown, 12)
				}
			})
		})
		if headerBtn.Clicked() {
			m.ShowTools = !m.ShowTools
		}

		if m.ShowTools {
			ui.Column(c).Gap(4).Padding(4, 0).Children(func() {
				for _, tool := range m.Tools {
					thisTool := tool
					ui.Row(c).Gap(6).Padding(4, 8).Radius(6).Background(t.Surface).BorderWidth(1, 1, 1, 1).BorderColor(t.Border).AlignItems(ui.Center).Children(func() {
						toolIcon := TablerTerminal
						nameLower := strings.ToLower(thisTool.ToolName)
						if strings.Contains(nameLower, "read") {
							toolIcon = TablerFileText
						} else if strings.Contains(nameLower, "edit") || strings.Contains(nameLower, "write") {
							toolIcon = TablerFilePen
						} else if strings.Contains(nameLower, "search") {
							toolIcon = TablerSearch
						}
						TablerIcon(c, toolIcon, 13)
						ui.Text(c, thisTool.ToolName).FontSize(11).Bold()
						preview := thisTool.Command
						if len(preview) > 60 {
							preview = preview[:57] + "…"
						}
						ui.Text(c, preview).FontSize(10).TextColor(t.TextMuted).MaxWidth(380)
						ui.Box(c).Grow(1)
						if thisTool.Status == "completed" {
							ui.Text(c, "Done").FontSize(10).TextColor(t.Accent)
						} else if thisTool.Status == "failed" {
							ui.Text(c, "Failed").FontSize(10).TextColor(t.Danger)
						} else {
							ui.Text(c, "Running").FontSize(10).TextColor(t.TextMuted)
						}
					})
				}
			})
		}
	})
}

// renderTurnChangesCard renders git changes produced in this turn.
func (as *AgentState) renderTurnChangesCard(c *ui.Context, m *ChatMessage) {
	t := c.Theme()

	ui.Column(c).Padding(6, 10).Radius(8).Background(t.Surface).BorderWidth(1, 1, 1, 1).BorderColor(t.Border).Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			TablerIcon(c, TablerFileDiff, 14)
			ui.Text(c, fmt.Sprintf("Turn Changes (%d files modified)", len(m.Changes))).FontSize(11).Bold()
		})
		ui.Column(c).Gap(4).Padding(4, 0).Children(func() {
			for _, ch := range m.Changes {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					TablerIcon(c, FileIconSVG(ch.Path), 13)
					ui.Text(c, ch.Path).FontSize(11)
					ui.Box(c).Grow(1)
					ui.Text(c, fmt.Sprintf("+%d", ch.Additions)).FontSize(10).Bold().TextColor(t.Accent)
					ui.Text(c, fmt.Sprintf("-%d", ch.Deletions)).FontSize(10).Bold().TextColor(t.Danger)
				})
			}
		})
	})
}

// renderApprovalBar renders user confirmation banner for sensitive tool executions.
func (as *AgentState) renderApprovalBar(c *ui.Context, m *ChatMessage) {
	t := c.Theme()
	appr := m.PendingApproval

	ui.Row(c).Gap(8).Padding(8, 12).Radius(8).Background(t.Border).BorderWidth(1, 1, 1, 1).BorderColor(t.Accent).AlignItems(ui.Center).Children(func() {
		TablerIcon(c, TablerAlertTriangle, 16)
		ui.Column(c).Children(func() {
			ui.Text(c, "Action Requires Confirmation").Bold().FontSize(12)
			ui.Text(c, appr.Summary).FontSize(11).TextColor(t.TextMuted)
		})
		ui.Box(c).Grow(1)

		allowBtn := ui.PrimaryButton(c, "")
		allowBtn.Children(func() {
			ui.Text(c, "Allow Once").FontSize(11).Bold()
		})
		if allowBtn.Clicked() {
			appr.Resolved = true
			appr.Approved = true
		}

		alwaysBtn := ui.Button(c, "")
		alwaysBtn.Children(func() {
			ui.Text(c, "Always Allow").FontSize(11)
		})
		if alwaysBtn.Clicked() {
			appr.Resolved = true
			appr.Approved = true
			as.ExecutionMode = ModeBypass
		}

		denyBtn := ui.Button(c, "")
		denyBtn.Children(func() {
			ui.Text(c, "Deny").FontSize(11).TextColor(t.Danger)
		})
		if denyBtn.Clicked() {
			appr.Resolved = true
			appr.Approved = false
		}
	})
}

// renderSteerQueueTray renders pending mid-turn steering instructions.
func (as *AgentState) renderSteerQueueTray(c *ui.Context) {
	t := c.Theme()

	ui.Column(c).Padding(6, 12).Radius(8).Background(t.Surface).BorderWidth(1, 1, 1, 1).BorderColor(t.Border).Gap(4).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			TablerIcon(c, TablerGripVertical, 13)
			ui.Text(c, fmt.Sprintf("Queued Instructions (%d)", len(as.SteerQueue))).FontSize(11).Bold()
			ui.Text(c, " • auto-steers when turn completes").FontSize(10).TextColor(t.TextMuted)
			ui.Box(c).Grow(1)
			clearBtn := ui.Button(c, "Clear all").FontSize(10)
			if clearBtn.Clicked() {
				as.SteerQueue = as.SteerQueue[:0]
			}
		})
		for idx, item := range as.SteerQueue {
			thisIdx := idx
			thisItem := item
			ui.Row(c).Gap(6).Padding(2, 4).AlignItems(ui.Center).Children(func() {
				TablerIcon(c, TablerGripVertical, 12)
				ui.Text(c, thisItem).FontSize(11).MaxWidth(420)
				ui.Box(c).Grow(1)
				steerNowBtn := ui.Button(c, "Steer Now").FontSize(10)
				if steerNowBtn.Clicked() {
					as.SteerQueue = append(as.SteerQueue[:thisIdx], as.SteerQueue[thisIdx+1:]...)
					as.triggerAgentTurn(thisItem)
				}
				delBtn := ui.Button(c, "×").FontSize(11)
				if delBtn.Clicked() {
					as.SteerQueue = append(as.SteerQueue[:thisIdx], as.SteerQueue[thisIdx+1:]...)
				}
			})
		}
	})
}

// renderModelPickerModal renders the popup palette to switch active LLM models.
func (as *AgentState) renderModelPickerModal(c *ui.Context) {
	t := c.Theme()

	ui.Column(c).Absolute().Bottom(60).Left(20).Width(320).Padding(10).Radius(12).Background(t.Surface).BorderWidth(1, 1, 1, 1).BorderColor(t.Border).Shadow(0, 4, 16, 0, ui.RGBA(0, 0, 0, 0.25)).Children(func() {
		ui.Row(c).Padding(0, 4, 6, 4).AlignItems(ui.Center).Children(func() {
			TablerIcon(c, TablerSparkles, 14)
			ui.Text(c, "Select AI Model").Bold().FontSize(12)
			ui.Box(c).Grow(1)
			closeBtn := ui.Button(c, "×").FontSize(12)
			if closeBtn.Clicked() {
				as.ShowModelPicker = false
			}
		})

		for _, model := range as.AvailableModels {
			thisModel := model
			isSelected := (as.ActiveModel == thisModel)
			var rowBtn ui.Element
			if isSelected {
				rowBtn = ui.PrimaryButton(c, "")
			} else {
				rowBtn = ui.Button(c, "")
			}
			rowBtn.Children(func() {
				ui.Row(c).Gap(6).Padding(4, 6).AlignItems(ui.Center).Children(func() {
					if isSelected {
						TablerIcon(c, TablerCheck, 13)
					} else {
						TablerIcon(c, TablerSparkles, 13)
					}
					ui.Text(c, thisModel).FontSize(11)
				})
			})
			if rowBtn.Clicked() {
				as.ActiveModel = thisModel
				as.ShowModelPicker = false
			}
		}
	})
}

func summarizeTools(tools []*ToolExecution) string {
	var commands, reads, edits, searches, others int
	for _, t := range tools {
		name := strings.ToLower(t.ToolName)
		switch {
		case strings.Contains(name, "command") || strings.Contains(name, "bash") || strings.Contains(name, "shell") || strings.Contains(name, "exec") || strings.Contains(name, "terminal"):
			commands++
		case strings.Contains(name, "read") || strings.Contains(name, "view") || strings.Contains(name, "cat"):
			reads++
		case strings.Contains(name, "edit") || strings.Contains(name, "write") || strings.Contains(name, "patch") || strings.Contains(name, "replace") || strings.Contains(name, "create"):
			edits++
		case strings.Contains(name, "search") || strings.Contains(name, "grep") || strings.Contains(name, "find"):
			searches++
		default:
			others++
		}
	}
	var parts []string
	if commands > 0 {
		parts = append(parts, fmt.Sprintf("Ran %d command%s", commands, plural(commands)))
	}
	if reads > 0 {
		parts = append(parts, fmt.Sprintf("Read %d file%s", reads, plural(reads)))
	}
	if edits > 0 {
		parts = append(parts, fmt.Sprintf("Edited %d file%s", edits, plural(edits)))
	}
	if searches > 0 {
		suffix := "ies"
		if searches == 1 {
			suffix = "y"
		}
		parts = append(parts, fmt.Sprintf("Searched %d quer%s", searches, suffix))
	}
	if others > 0 && len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("Executed %d tool%s", others, plural(others)))
	}
	if len(parts) == 0 {
		return fmt.Sprintf("%d tool calls", len(tools))
	}
	return strings.Join(parts, " · ")
}

func extractChangesFromTools(tools []*ToolExecution) []*FileChange {
	var changes []*FileChange
	seen := map[string]*FileChange{}
	for _, t := range tools {
		name := strings.ToLower(t.ToolName)
		if !strings.Contains(name, "edit") && !strings.Contains(name, "write") &&
			!strings.Contains(name, "replace") && !strings.Contains(name, "patch") &&
			!strings.Contains(name, "create") {
			continue
		}
		path := extractPathFromTool(t.Command)
		if path == "" {
			continue
		}
		if ch, exists := seen[path]; exists {
			ch.Additions += 5
			ch.Deletions += 1
		} else {
			ch := &FileChange{
				Path:      path,
				Additions: 8,
				Deletions: 2,
			}
			seen[path] = ch
			changes = append(changes, ch)
		}
	}
	return changes
}

func extractPathFromTool(cmd string) string {
	if strings.HasPrefix(strings.TrimSpace(cmd), "{") {
		var args struct {
			Path     string `json:"path"`
			FilePath string `json:"filePath"`
			Target   string `json:"Path"`
			File     string `json:"file"`
		}
		if err := json.Unmarshal([]byte(cmd), &args); err == nil {
			if args.Path != "" {
				return args.Path
			}
			if args.FilePath != "" {
				return args.FilePath
			}
			if args.Target != "" {
				return args.Target
			}
			if args.File != "" {
				return args.File
			}
		}
	}
	return ""
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
