package ui

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
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
	ID        string
	Title     string
	Status    string // "running", "idle", "completed"
	CreatedAt time.Time
	UpdatedAt time.Time
	Messages  []*ChatMessage
	Tasks     *TaskManager
	Model     string
	Pinned    bool
	Archived  bool
}

// AgentState tracks sessions, conversation history, streaming responses, and prompt composer.
type AgentState struct {
	mu              sync.Mutex
	engine          *Engine
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

// NewAgentState initializes the native agent chat state with default sessions.
func NewAgentState(engine *Engine, invalidate func()) *AgentState {
	as := &AgentState{
		engine:          engine,
		ActiveModel:     "Claude 3.7 Sonnet / gpt-4o",
		ActiveFilterTab: "active",
		Invalidate:      invalidate,
		NextID:          10,
		MdOpts: markdown.Options{
			Reveal: &markdown.Reveal{Unit: markdown.ByWord},
		},
	}

	// Session 1: Port ForgeADE to Native Go UI (active)
	sess1Tasks := NewTaskManager()
	sess1Tasks.SetGoal("Port ForgeADE to Golang Native UI", []TaskItem{
		{ID: "1", Title: "Initialize forge-ade-native repository", Status: TaskCompleted},
		{ID: "2", Title: "Port Wails + React to pure Go MyGo native engine", Status: TaskCompleted},
		{ID: "3", Title: "Implement WorkspaceHeader, Agent Mode, Editor Mode parity", Status: TaskInProgress, ActiveForm: "Building layout components..."},
		{ID: "4", Title: "Compile and run on Linux display with <50MB RAM", Status: TaskPending},
	})
	sess1 := &AgentSession{
		ID:        "sess-1",
		Title:     "Port ForgeADE to Native Go UI",
		Status:    "running",
		CreatedAt: time.Now().Add(-2 * time.Hour),
		UpdatedAt: time.Now().Add(-1 * time.Minute),
		Model:     "Claude 3.7 Sonnet",
		Pinned:    true,
		Tasks:     sess1Tasks,
		Messages: []*ChatMessage{
			{
				ID:        1,
				Role:      "assistant",
				Text:      "### Welcome to ForgeADE Native\n\nI am your native autonomous coding assistant running with **pure Go runtime** and **GPU acceleration**.\n\nType `/help` to view slash commands, or enter an instruction below to begin.",
				Done:      true,
				Timestamp: time.Now().Add(-2 * time.Hour),
			},
			{
				ID:        2,
				Role:      "user",
				Text:      "Buat layout, sidebar, agent mode, editor mode yang sama dengan ForgeADE.",
				Done:      true,
				Timestamp: time.Now().Add(-10 * time.Minute),
			},
			{
				ID:        3,
				Role:      "assistant",
				Text:      "Menerapkan arsitektur **Dual-Surface (Agent Mode + Editor Mode)** identik dengan ForgeADE:\n\n1. **Top Header (`WorkspaceHeader`)**: Mode Switcher `[✨ Agent]` / `[💻 Editor]`, project badge, git branch selector, quick search (⌘P), terminal toggle, side drawer toggle.\n2. **Agent Mode (`ModeAgent`)**: Full-height sidebar dengan task session cards, filter tabs (Active/Pinned/Archived), bottom profile row, agent chat stream, docked task checklist, dan slash commands.\n3. **Editor Mode (`ModeEditor`)**: Activity bar (Files/Git/DB/Settings), Workspace File Tree dengan dirty dots (`•`), Tab bar dengan multi-pane split & diff viewer, bottom terminal drawer, dan Zed-style status bar.\n\nMemproses perubahan komponen...",
				Done:      true,
				Timestamp: time.Now().Add(-8 * time.Minute),
			},
		},
	}

	// Session 2: GPU Rendering Optimization
	sess2Tasks := NewTaskManager()
	sess2Tasks.SetGoal("Evaluate pure Go vs GTK3 memory consumption", []TaskItem{
		{ID: "1", Title: "Measure RSS memory on Linux", Status: TaskCompleted},
		{ID: "2", Title: "Verify zero-copy texture upload", Status: TaskCompleted},
	})
	sess2 := &AgentSession{
		ID:        "sess-2",
		Title:     "GPU Rendering & Memory Profiling",
		Status:    "completed",
		CreatedAt: time.Now().Add(-24 * time.Hour),
		UpdatedAt: time.Now().Add(-5 * time.Hour),
		Model:     "gpt-4o",
		Pinned:    false,
		Tasks:     sess2Tasks,
		Messages: []*ChatMessage{
			{
				ID:        4,
				Role:      "user",
				Text:      "Berapa memory footprint saat idle?",
				Done:      true,
				Timestamp: time.Now().Add(-6 * time.Hour),
			},
			{
				ID:        5,
				Role:      "assistant",
				Text:      "Memory consumption idle adalah **55.1 MB** (turun drastis dari 180MB+ pada WebView/Chromium). Idle CPU adalah **0.7%**.",
				Done:      true,
				Timestamp: time.Now().Add(-5 * time.Hour),
			},
		},
	}

	as.Sessions = []*AgentSession{sess1, sess2}
	as.ActiveSessionID = sess1.ID
	as.syncActiveSession()

	return as
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
			return
		}
	}
	if len(as.Sessions) > 0 {
		as.ActiveSessionID = as.Sessions[0].ID
		as.Messages = as.Sessions[0].Messages
		as.TaskManager = as.Sessions[0].Tasks
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
	newID := fmt.Sprintf("sess-%d", time.Now().UnixNano()%100000)
	tm := NewTaskManager()
	tm.SetGoal(title, []TaskItem{
		{ID: "1", Title: "Analyze target requirements", Status: TaskPending},
	})
	sess := &AgentSession{
		ID:        newID,
		Title:     title,
		Status:    "idle",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Model:     as.ActiveModel,
		Tasks:     tm,
		Messages: []*ChatMessage{
			{
				ID:        as.NextID,
				Role:      "assistant",
				Text:      fmt.Sprintf("### %s\n\nHow can I help you accomplish this task?", title),
				Done:      true,
				Timestamp: time.Now(),
			},
		},
	}
	as.NextID++
	as.Sessions = append([]*AgentSession{sess}, as.Sessions...)
	as.ActiveSessionID = newID
	as.syncActiveSession()
	as.mu.Unlock()

	if as.Invalidate != nil {
		as.Invalidate()
	}
	return sess
}

// TogglePin toggles the pinned status of a session.
func (as *AgentState) TogglePin(id string) {
	as.mu.Lock()
	for _, s := range as.Sessions {
		if s.ID == id {
			s.Pinned = !s.Pinned
			break
		}
	}
	as.mu.Unlock()
	if as.Invalidate != nil {
		as.Invalidate()
	}
}

// ToggleArchive toggles the archived status of a session.
func (as *AgentState) ToggleArchive(id string) {
	as.mu.Lock()
	for _, s := range as.Sessions {
		if s.ID == id {
			s.Archived = !s.Archived
			break
		}
	}
	as.mu.Unlock()
	if as.Invalidate != nil {
		as.Invalidate()
	}
}

// DeleteSession removes a session from history.
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
	if as.Invalidate != nil {
		as.Invalidate()
	}
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
		ID:        as.NextID,
		Role:      "user",
		Text:      input,
		Done:      true,
		Timestamp: time.Now(),
	}
	as.NextID++

	// Update active session
	for _, s := range as.Sessions {
		if s.ID == as.ActiveSessionID {
			s.Messages = append(s.Messages, userMsg)
			s.UpdatedAt = time.Now()
			s.Status = "running"
			as.Messages = s.Messages
			break
		}
	}
	as.mu.Unlock()

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
	switch rootCmd {
	case "/clear":
		for _, s := range as.Sessions {
			if s.ID == as.ActiveSessionID {
				s.Messages = s.Messages[:0]
				as.Messages = s.Messages
				break
			}
		}
		reply = "Chat history cleared."
	case "/goal":
		desc := strings.TrimSpace(strings.TrimPrefix(cmd, "/goal"))
		if desc == "" {
			desc = "Decompose and execute objective"
		}
		if as.TaskManager != nil {
			as.TaskManager.SetGoal(desc, []TaskItem{
				{ID: "1", Title: "Analyze target requirements and code symbols", Status: TaskInProgress, ActiveForm: "Scanning symbols..."},
				{ID: "2", Title: "Formulate code changes and dependency updates", Status: TaskPending},
				{ID: "3", Title: "Execute regression tests and static validation", Status: TaskPending},
				{ID: "4", Title: "Review diff hunks and commit changes", Status: TaskPending},
			})
		}
		reply = fmt.Sprintf("Declared Goal: **%s**\nDecomposed into 4 atomic tasks on the floating checklist.", desc)
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
			"- **Editor**: Zed-style virtualized buffer with live diff gutters"
	default:
		reply = fmt.Sprintf("Unknown command `%s`. Type `/help` for available commands.", rootCmd)
	}

	replyMsg := &ChatMessage{
		ID:        as.NextID,
		Role:      "assistant",
		Text:      reply,
		Done:      true,
		Timestamp: time.Now(),
	}
	as.NextID++

	for _, s := range as.Sessions {
		if s.ID == as.ActiveSessionID {
			s.Messages = append(s.Messages, replyMsg)
			as.Messages = s.Messages
			s.Status = "idle"
			break
		}
	}

	if as.Invalidate != nil {
		as.Invalidate()
	}
}

func (as *AgentState) triggerAgentTurn(prompt string) {
	as.mu.Lock()
	as.IsStreaming = true
	assistantMsg := &ChatMessage{
		ID:        as.NextID,
		Role:      "assistant",
		Text:      "",
		Done:      false,
		Timestamp: time.Now(),
	}
	as.NextID++

	for _, s := range as.Sessions {
		if s.ID == as.ActiveSessionID {
			s.Messages = append(s.Messages, assistantMsg)
			as.Messages = s.Messages
			s.Status = "running"
			break
		}
	}
	as.mu.Unlock()

	go func() {
		defer func() {
			as.mu.Lock()
			assistantMsg.Done = true
			as.IsStreaming = false
			for _, s := range as.Sessions {
				if s.ID == as.ActiveSessionID {
					s.Status = "idle"
					s.UpdatedAt = time.Now()
					break
				}
			}
			as.mu.Unlock()
			if as.Invalidate != nil {
				as.Invalidate()
			}
		}()

		chunks := []string{
			"Analyzing your request...\n\n",
			"Here is an overview of the implementation:\n\n",
			"```go\n",
			"// Native zero-copy UI bridge\n",
			"func StreamResponse(token string) {\n",
			"    buffer.Append(token)\n",
			"    window.Invalidate()\n",
			"}\n",
			"```\n\n",
			"The task has been decomposed and verified natively without browser webview latency.",
		}

		for _, chunk := range chunks {
			time.Sleep(100 * time.Millisecond)
			as.mu.Lock()
			assistantMsg.Text += chunk
			as.mu.Unlock()
			if as.Invalidate != nil {
				as.Invalidate()
			}
		}
	}()
}

// RenderChat renders the conversation view, message bubbles, task checklist, and prompt composer.
func (as *AgentState) RenderChat(c *ui.Context) {
	t := c.Theme()
	sess := as.ActiveSession()

	sessionTitle := "Agent Assistant"
	statusLabel := "⚪ Idle"
	if sess != nil {
		sessionTitle = sess.Title
		if sess.Status == "running" {
			statusLabel = "🟢 Running"
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
			ui.Text(c, "✨").FontSize(14)
			ui.Text(c, sessionTitle).Bold().FontSize(13)
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

		// Floating Task Checklist (docked above composer)
		if as.TaskManager != nil {
			as.TaskManager.RenderChecklist(c)
		}

		// Input Composer Area
		ui.Column(c).BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Background(t.Surface).Children(func() {
			// Slash commands shortcuts bar
			ui.Row(c).Gap(6).Padding(6, 16, 2, 16).AlignItems(ui.Center).Children(func() {
				cmdPills := []string{"/goal", "/tasks", "/model", "/clear", "/status", "/help"}
				for _, cmd := range cmdPills {
					thisCmd := cmd
					btn := ui.Button(c, thisCmd).FontSize(11)
					if btn.Clicked() {
						as.InputText = thisCmd + " "
					}
				}
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
