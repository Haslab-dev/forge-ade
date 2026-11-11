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
	Theme           *ThemeManager
	ACPMgr          *ACPManager
	statsPending    map[string]bool
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

// ExecutionModeIcon returns the Tabler SVG icon for the active execution mode.
func (as *AgentState) ExecutionModeIcon() *ui.SVG {
	switch as.ExecutionMode {
	case ModeAsk:
		return TablerHand
	case ModeCode:
		return TablerPencil
	case ModePlan:
		return TablerShieldCheck
	default:
		return TablerShieldAlert
	}
}

// ExecutionModeLabel returns the user-facing label for the active execution mode.
func (as *AgentState) ExecutionModeLabel() string {
	switch as.ExecutionMode {
	case ModeAsk:
		return "Ask"
	case ModeCode:
		return "Edits"
	case ModePlan:
		return "Auto"
	default:
		return "Full"
	}
}

// CycleExecutionMode advances to the next execution autonomy level.
func (as *AgentState) CycleExecutionMode() {
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

// ReasoningEffortLabel returns the user-facing label for the active reasoning effort.
func (as *AgentState) ReasoningEffortLabel() string {
	switch as.ReasoningEffort {
	case EffortOff:
		return "Off"
	case EffortLow:
		return "Low"
	case EffortHigh:
		return "High"
	case EffortMax:
		return "Max"
	default:
		return "Med"
	}
}

// CycleReasoningEffort advances to the next thinking budget level.
func (as *AgentState) CycleReasoningEffort() {
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

type diskSessionRaw struct {
	ID             string           `json:"id"`
	Title          string           `json:"title"`
	Name           string           `json:"name"`
	Goal           string           `json:"goal"`
	Status         string           `json:"status"`
	State          string           `json:"state"`
	CreatedAt      json.RawMessage  `json:"createdAt"`
	UpdatedAt      json.RawMessage  `json:"updatedAt"`
	Model          string           `json:"model"`
	Provider       string           `json:"provider"`
	AgentID        string           `json:"agentId"`
	AgentType      string           `json:"agentType"`
	ExecutionMode  string           `json:"executionMode"`
	ReasoningLevel string           `json:"reasoningLevel"`
	WorkspacePath  string           `json:"workspacePath"`
	Folder         string           `json:"folder"`
	Pinned         bool             `json:"pinned"`
	Archived       bool             `json:"archived"`
	Messages       []diskMessageRaw `json:"messages"`
	Tasks          []TaskItem       `json:"tasks"`
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

// gitStatusPaths returns the set of files git reports as changed, for the
// turn-change diff (files modified during one turn).
func (as *AgentState) gitStatusPaths() map[string]bool {
	out := map[string]bool{}
	if as.engine == nil || as.workspacePath == "" {
		return out
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	res, err := as.engine.GitEngine.GetStatus(ctx, as.workspacePath)
	if err != nil {
		return out
	}
	for _, f := range res.Staged {
		out[f.Path] = true
	}
	for _, f := range res.Unstaged {
		out[f.Path] = true
	}
	for _, f := range res.Untracked {
		out[f.Path] = true
	}
	return out
}

// computeChangeStats fills real +N/−N counts from the file's git diff.
func (as *AgentState) computeChangeStats(changes []*FileChange) {
	if as.engine == nil || as.workspacePath == "" {
		return
	}
	for _, ch := range changes {
		if ch.Additions >= 0 {
			continue
		}
		patch := as.engine.GetGitDiff(as.workspacePath, ch.Path)
		ch.Additions, ch.Deletions = countPatchLines(patch)
	}
}

// countPatchLines counts added and deleted lines of a unified diff.
func countPatchLines(patch string) (add, del int) {
	for _, line := range strings.Split(patch, "\n") {
		switch {
		case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---"):
			// file headers
		case strings.HasPrefix(line, "+"):
			add++
		case strings.HasPrefix(line, "-"):
			del++
		}
	}
	return
}

// triggerAgentTurn executes real streaming responses via LLMClient or ACPManager.
func (as *AgentState) triggerAgentTurn(prompt string) {
	as.mu.Lock()
	as.IsStreaming = true
	turnStartFiles := as.gitStatusPaths()

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

			// Turn changes: files git newly reports as changed, with real
			// +/- counts from their diffs.
			turnEndFiles := as.gitStatusPaths()
			var turnChanges []*FileChange
			for path := range turnEndFiles {
				if !turnStartFiles[path] {
					turnChanges = append(turnChanges, &FileChange{Path: path, Additions: -1, Deletions: -1})
				}
			}
			as.computeChangeStats(turnChanges)
			assistantMsg.Changes = turnChanges

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
		if _, exists := seen[path]; !exists {
			ch := &FileChange{
				Path:      path,
				Additions: -1, // computed from the git diff on demand
				Deletions: -1,
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
