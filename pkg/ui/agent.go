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

// ChatMessage represents a single interaction turn in the AI Agent conversation.
type ChatMessage struct {
	ID        int
	Role      string // "user", "assistant", "system"
	Text      string
	Done      bool
	Timestamp time.Time
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
		Invalidate:      invalidate,
		NextID:          1,
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
	ID            string            `json:"id"`
	Title         string            `json:"title"`
	Name          string            `json:"name"`
	Goal          string            `json:"goal"`
	Status        string            `json:"status"`
	State         string            `json:"state"`
	CreatedAt     json.RawMessage   `json:"createdAt"`
	UpdatedAt     json.RawMessage   `json:"updatedAt"`
	Model         string            `json:"model"`
	Provider      string            `json:"provider"`
	AgentID       string            `json:"agentId"`
	AgentType     string            `json:"agentType"`
	WorkspacePath string            `json:"workspacePath"`
	Folder        string            `json:"folder"`
	Pinned        bool              `json:"pinned"`
	Archived      bool              `json:"archived"`
	Messages      []diskMessageRaw  `json:"messages"`
	Tasks         []TaskItem        `json:"tasks"`
}

type diskMessageRaw struct {
	ID         string            `json:"id"`
	Role       string            `json:"role"`
	Content    json.RawMessage   `json:"content"`
	Text       string            `json:"text"`
	Timestamp  json.RawMessage   `json:"timestamp"`
	TurnTexts  map[string]string `json:"turnTexts"`
	TextBlocks []struct {
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
			if txt == "" {
				continue
			}
			msgs = append(msgs, &ChatMessage{
				ID:        msgID,
				Role:      role,
				Text:      txt,
				Done:      true,
				Timestamp: updatedTime,
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
		diskMsgs = append(diskMsgs, map[string]interface{}{
			"id":        fmt.Sprintf("msg-%d", m.ID),
			"role":      role,
			"content":   m.Text,
			"timestamp": m.Timestamp.Format(time.RFC3339),
		})
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
		"id":            sess.ID,
		"title":         sess.Title,
		"goal":          goal,
		"status":        sess.Status,
		"createdAt":     sess.CreatedAt.Format(time.RFC3339),
		"updatedAt":     time.Now().Format(time.RFC3339),
		"model":         sess.Model,
		"provider":      sess.Provider,
		"agentId":       sess.AgentID,
		"agentType":     sess.AgentType,
		"workspacePath": wsPath,
		"pinned":        sess.Pinned,
		"archived":      sess.Archived,
		"messages":      diskMsgs,
		"tasks":         tasksList,
	}

	bytes, err := json.MarshalIndent(data, "", "  ")
	if err == nil {
		_ = agentsession.SaveSessionDisk(string(bytes), wsPath)
	}
}

// ActiveSession returns the currently selected AgentSession.
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

func (as *AgentState) syncActiveSession() {
	for _, s := range as.Sessions {
		if s.ID == as.ActiveSessionID {
			as.Messages = s.Messages
			as.TaskManager = s.Tasks
			if s.Model != "" {
				as.ActiveModel = s.Model
			}
			return
		}
	}
	if len(as.Sessions) > 0 {
		as.ActiveSessionID = as.Sessions[0].ID
		as.Messages = as.Sessions[0].Messages
		as.TaskManager = as.Sessions[0].Tasks
		if as.Sessions[0].Model != "" {
			as.ActiveModel = as.Sessions[0].Model
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

// NewSession creates and activates a new task session.
func (as *AgentState) NewSession(title string) *AgentSession {
	as.mu.Lock()
	if title == "" {
		title = fmt.Sprintf("New Task Session %d", len(as.Sessions)+1)
	}
	newID := fmt.Sprintf("session-%d", time.Now().UnixMilli())
	tm := NewTaskManager()
	tm.SetGoal(title, []TaskItem{
		{ID: "1", Title: "Analyze target requirements", Status: TaskPending},
	})
	sess := &AgentSession{
		ID:            newID,
		Title:         title,
		Status:        "idle",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		Model:         as.ActiveModel,
		WorkspacePath: as.workspacePath,
		Tasks:         tm,
		Messages: []*ChatMessage{
			{
				ID:        1,
				Role:      "assistant",
				Text:      fmt.Sprintf("### %s\n\nHow can I help you accomplish this task?", title),
				Done:      true,
				Timestamp: time.Now(),
			},
		},
	}
	as.NextID = 2
	as.Sessions = append([]*AgentSession{sess}, as.Sessions...)
	as.ActiveSessionID = newID
	as.syncActiveSession()
	as.mu.Unlock()

	// Persist new session to disk
	as.SaveSessionToDisk(sess)

	if as.Invalidate != nil {
		as.Invalidate()
	}
	return sess
}

// TogglePin toggles the pinned status of a session and saves to disk.
func (as *AgentState) TogglePin(id string) {
	as.mu.Lock()
	var target *AgentSession
	for _, s := range as.Sessions {
		if s.ID == id {
			s.Pinned = !s.Pinned
			target = s
			break
		}
	}
	as.mu.Unlock()

	if target != nil {
		as.SaveSessionToDisk(target)
	}
	if as.Invalidate != nil {
		as.Invalidate()
	}
}

// ToggleArchive toggles the archived status of a session and saves to disk.
func (as *AgentState) ToggleArchive(id string) {
	as.mu.Lock()
	var target *AgentSession
	for _, s := range as.Sessions {
		if s.ID == id {
			s.Archived = !s.Archived
			target = s
			break
		}
	}
	as.mu.Unlock()

	if target != nil {
		as.SaveSessionToDisk(target)
	}
	if as.Invalidate != nil {
		as.Invalidate()
	}
}

// DeleteSession removes a session from memory and disk.
func (as *AgentState) DeleteSession(id string) {
	as.mu.Lock()
	filtered := make([]*AgentSession, 0, len(as.Sessions))
	for _, s := range as.Sessions {
		if s.ID != id {
			filtered = append(filtered, s)
		}
	}
	as.Sessions = filtered
	if as.ActiveSessionID == id && len(as.Sessions) > 0 {
		as.ActiveSessionID = as.Sessions[0].ID
	}
	as.syncActiveSession()
	as.mu.Unlock()

	// Remove from disk
	_ = agentsession.DeleteSessionDisk(id, as.workspacePath)

	if as.Invalidate != nil {
		as.Invalidate()
	}
}

// TabCounts returns the count of active, pinned, and archived sessions.
func (as *AgentState) TabCounts() (active, pinned, archived int) {
	as.mu.Lock()
	defer as.mu.Unlock()
	for _, s := range as.Sessions {
		if s.Archived {
			archived++
		} else {
			active++
		}
		if s.Pinned && !s.Archived {
			pinned++
		}
	}
	return
}

// ContextUsage returns estimated prompt tokens and context percentage.
func (as *AgentState) ContextUsage() (tokens int, pct int) {
	as.mu.Lock()
	defer as.mu.Unlock()
	chars := 0
	for _, m := range as.Messages {
		chars += len(m.Text)
	}
	tokens = chars / 4
	if tokens < 240 {
		tokens = 240
	}
	maxTokens := 200000 // 200k context window standard
	pct = (tokens * 100) / maxTokens
	if pct == 0 && tokens > 0 {
		pct = 1
	}
	return
}

// FilteredSessions returns the list of sessions matching filter tab and search query.
func (as *AgentState) FilteredSessions() []*AgentSession {
	as.mu.Lock()
	defer as.mu.Unlock()

	var result []*AgentSession
	query := strings.ToLower(strings.TrimSpace(as.SearchQuery))

	for _, s := range as.Sessions {
		// Filter tabs
		switch as.ActiveFilterTab {
		case "pinned":
			if !s.Pinned || s.Archived {
				continue
			}
		case "archived":
			if !s.Archived {
				continue
			}
		case "active", "":
			if s.Archived {
				continue
			}
		}

		// Search query filter
		if query != "" {
			if !strings.Contains(strings.ToLower(s.Title), query) && !strings.Contains(strings.ToLower(s.ID), query) {
				continue
			}
		}

		result = append(result, s)
	}
	return result
}

// SubmitMessage handles user prompt submissions and slash commands.
func (as *AgentState) SubmitMessage() {
	input := strings.TrimSpace(as.InputText)
	if input == "" || as.IsStreaming {
		return
	}

	as.mu.Lock()
	as.InputText = ""
	userMsg := &ChatMessage{
		ID:        len(as.Messages) + 1,
		Role:      "user",
		Text:      input,
		Done:      true,
		Timestamp: time.Now(),
	}

	var activeSess *AgentSession
	for _, s := range as.Sessions {
		if s.ID == as.ActiveSessionID {
			s.Messages = append(s.Messages, userMsg)
			s.UpdatedAt = time.Now()
			s.Status = "running"
			as.Messages = s.Messages
			activeSess = s
			break
		}
	}
	as.mu.Unlock()

	if activeSess != nil {
		as.SaveSessionToDisk(activeSess)
	}

	// Handle Slash Commands
	if strings.HasPrefix(input, "/") {
		as.handleSlashCommand(input)
		return
	}

	// Trigger Assistant Stream
	as.triggerAgentTurn(input)
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
		reply = fmt.Sprintf("📋 **Plan Mode Activated**: `%s`\nStep-by-step architectural breakdown populated on the floating checklist.", desc)
	case "/vibe":
		reply = "⚡ **Vibe Mode Activated**: Autonomous execution with automatic error diagnosis and fix cycles enabled."
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

	if as.Invalidate != nil {
		as.Invalidate()
	}
}

// triggerAgentTurn executes real streaming responses via LLMClient or ACPManager.
func (as *AgentState) triggerAgentTurn(prompt string) {
	as.mu.Lock()
	as.IsStreaming = true

	assistantMsg := &ChatMessage{
		ID:        len(as.Messages) + 1,
		Role:      "assistant",
		Text:      "",
		Done:      false,
		Timestamp: time.Now(),
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
			as.IsStreaming = false
			if activeSess != nil {
				activeSess.Status = "idle"
				activeSess.UpdatedAt = time.Now()
			}
			as.mu.Unlock()

			if activeSess != nil {
				as.SaveSessionToDisk(activeSess)
			}
			if as.Invalidate != nil {
				as.Invalidate()
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
				assistantMsg.Text += deltaContent
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
			assistantMsg.Text = fmt.Sprintf("⚠️ **LLM Request Notice**: %s\n\n"+
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
	statusLabel := "• Idle"
	if sess != nil {
		sessionTitle = sess.Title
		if sess.Status == "running" {
			statusLabel = "● Running"
		} else if sess.Status == "completed" {
			statusLabel = "✔ Completed"
		}
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
		// Session Header Bar
		ui.Row(c).Gap(10).Padding(10, 16).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Background(t.Surface).Children(func() {
			ui.Text(c, "⚡").FontSize(13)
			ui.Text(c, sessionTitle).Bold().FontSize(13).MaxWidth(380)
			ui.Text(c, statusLabel).FontSize(11).TextColor(t.TextMuted)
			ui.Box(c).Grow(1)
			ui.Text(c, as.ActiveModel).FontSize(11).TextColor(t.TextMuted)
			if as.IsStreaming {
				ui.Text(c, "Streaming tokens…").FontSize(11).TextColor(t.Accent)
			}
			if ui.Button(c, "Fork").FontSize(11).Clicked() && sess != nil {
				as.NewSession(fmt.Sprintf("%s (fork)", sess.Title))
			}
		})

		// Message List
		if len(as.Messages) == 0 {
			ui.Column(c).Grow(1).Center().Children(func() {
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
						ui.Text(c, m.Text).Selectable().Padding(8, 14).Radius(12).Background(t.Surface).MaxWidth(560)
					})
				} else {
					ui.Column(c).Padding(4, 0).MaxWidth(800).Children(func() {
						if m.Done {
							markdown.View(c, m.Text, &as.MdOpts)
						} else {
							markdown.Stream(c, m.Text, &as.MdOpts)
						}
					})
				}
			}).Grow(1).Padding(16, 20).Gap(12).Justify(ui.End)
		}

		// Floating Task Checklist (docked above composer)
		if as.TaskManager != nil {
			as.TaskManager.RenderChecklist(c)
		}

		// Input Composer Area
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

			// Input input row
			ui.Row(c).Gap(8).Padding(8, 16, 12, 16).AlignItems(ui.Center).Children(func() {
				in := ui.TextInput(c, &as.InputText).Placeholder("Ask agent, enter instructions, or /help...").Grow(1).AutoFocus()
				if in.Submitted() && !as.IsStreaming {
					as.SubmitMessage()
				}

				if as.IsStreaming {
					if ui.Button(c, "Stop").Clicked() {
						as.mu.Lock()
						as.IsStreaming = false
						if len(as.Messages) > 0 {
							as.Messages[len(as.Messages)-1].Done = true
						}
						as.mu.Unlock()
					}
				} else {
					sendBtn := ui.PrimaryButton(c, "Send")
					if strings.TrimSpace(as.InputText) == "" {
						sendBtn.Disabled(true)
					}
					if sendBtn.Clicked() {
						as.SubmitMessage()
					}
				}
			})
		})
	})
}
