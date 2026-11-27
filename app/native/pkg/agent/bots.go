package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// BotProfile defines a persistent Local ForgeADE Bot (inspired by Orca Bots):
// a named autonomous persona with its own Home Chat, default agent/model,
// workspace scope, and system instructions.
type BotProfile struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Role          string    `json:"role"`
	SystemPrompt  string    `json:"systemPrompt,omitempty"`
	AgentID       string    `json:"agentId"`
	AgentType     string    `json:"agentType"` // "internal" or "acp"
	Model         string    `json:"model,omitempty"`
	WorkspacePath string    `json:"workspacePath,omitempty"`
	HomeSessionID string    `json:"homeSessionId,omitempty"`
	Avatar        string    `json:"avatar,omitempty"`
	Paused        bool      `json:"paused,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
}

// BotStore persists Local Bot profiles to disk (<dir>/bots.json).
type BotStore struct {
	mu   sync.RWMutex
	path string
	bots []*BotProfile
}

// NewBotStore loads or initializes the local bot catalog at <dir>/bots.json.
func NewBotStore(dir string) *BotStore {
	if dir == "" {
		if home, err := os.UserHomeDir(); err == nil {
			dir = filepath.Join(home, ".forge-ade")
		}
	}
	st := &BotStore{
		path: filepath.Join(dir, "bots.json"),
	}
	st.load()
	return st
}

func (st *BotStore) load() {
	if st == nil || st.path == "" {
		return
	}
	data, err := os.ReadFile(st.path)
	if err != nil {
		return
	}
	var list []*BotProfile
	if err := json.Unmarshal(data, &list); err == nil {
		st.mu.Lock()
		st.bots = list
		st.mu.Unlock()
	}
}

func (st *BotStore) saveLocked() {
	if st == nil || st.path == "" {
		return
	}
	_ = os.MkdirAll(filepath.Dir(st.path), 0o755)
	data, err := json.MarshalIndent(st.bots, "", "  ")
	if err != nil {
		return
	}
	tmp := st.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err == nil {
		_ = os.Rename(tmp, st.path)
	}
}

// List returns a snapshot of all configured local bots.
func (st *BotStore) List() []*BotProfile {
	if st == nil {
		return nil
	}
	st.mu.RLock()
	defer st.mu.RUnlock()
	out := make([]*BotProfile, len(st.bots))
	copy(out, st.bots)
	return out
}

// Get finds a bot by ID.
func (st *BotStore) Get(id string) *BotProfile {
	if st == nil {
		return nil
	}
	st.mu.RLock()
	defer st.mu.RUnlock()
	for _, b := range st.bots {
		if b != nil && b.ID == id {
			return b
		}
	}
	return nil
}

// Create adds a new local bot profile and persists it.
func (st *BotStore) Create(b BotProfile) *BotProfile {
	if st == nil {
		return nil
	}
	st.mu.Lock()
	defer st.mu.Unlock()

	if b.ID == "" {
		b.ID = fmt.Sprintf("bot-%d", time.Now().UnixNano())
	}
	if strings.TrimSpace(b.Name) == "" {
		b.Name = "Forge Bot"
	}
	if b.AgentID == "" {
		b.AgentID = "agent-internal"
	}
	if b.AgentType == "" {
		if b.AgentID == "agent-internal" {
			b.AgentType = "internal"
		} else {
			b.AgentType = "acp"
		}
	}
	if b.CreatedAt.IsZero() {
		b.CreatedAt = time.Now()
	}
	cp := b
	st.bots = append(st.bots, &cp)
	st.saveLocked()
	return &cp
}

// UpdateHomeSession links a bot to its Home Chat session ID and persists.
func (st *BotStore) UpdateHomeSession(botID, sessionID string) {
	if st == nil || botID == "" {
		return
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	for _, b := range st.bots {
		if b != nil && b.ID == botID {
			b.HomeSessionID = sessionID
			st.saveLocked()
			return
		}
	}
}

// UpdateAvatar sets the bot's avatar (embedded preset filename or custom image path) and persists.
func (st *BotStore) UpdateAvatar(botID, avatar string) {
	if st == nil || botID == "" {
		return
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	for _, b := range st.bots {
		if b != nil && b.ID == botID {
			b.Avatar = strings.TrimSpace(avatar)
			st.saveLocked()
			return
		}
	}
}

// Update edits a bot's name, role, standing instructions, and default harness and persists.
func (st *BotStore) Update(botID, name, role, systemPrompt, agentID, agentType string) *BotProfile {
	if st == nil || botID == "" {
		return nil
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	for _, b := range st.bots {
		if b != nil && b.ID == botID {
			if n := strings.TrimSpace(name); n != "" {
				b.Name = n
			}
			b.Role = strings.TrimSpace(role)
			b.SystemPrompt = strings.TrimSpace(systemPrompt)
			if strings.TrimSpace(agentID) != "" {
				b.AgentID = strings.TrimSpace(agentID)
			}
			if strings.TrimSpace(agentType) != "" {
				b.AgentType = strings.TrimSpace(agentType)
			}
			st.saveLocked()
			cp := *b
			return &cp
		}
	}
	return nil
}

// UpdatePersona updates a bot's editable persona fields (name, role, system prompt,
// default harness) and persists the store.
func (st *BotStore) UpdatePersona(botID, name, role, systemPrompt, agentID, agentType string) *BotProfile {
	if st == nil || botID == "" {
		return nil
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	for _, b := range st.bots {
		if b != nil && b.ID == botID {
			if trimmed := strings.TrimSpace(name); trimmed != "" {
				b.Name = trimmed
			}
			b.Role = strings.TrimSpace(role)
			b.SystemPrompt = strings.TrimSpace(systemPrompt)
			if strings.TrimSpace(agentID) != "" {
				b.AgentID = strings.TrimSpace(agentID)
				if strings.TrimSpace(agentType) != "" {
					b.AgentType = strings.TrimSpace(agentType)
				} else if b.AgentID == "agent-internal" {
					b.AgentType = "internal"
				} else {
					b.AgentType = "acp"
				}
			}
			st.saveLocked()
			cp := *b
			return &cp
		}
	}
	return nil
}

// TogglePause flips the paused flag for a bot.
func (st *BotStore) TogglePause(botID string) {
	if st == nil {
		return
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	for _, b := range st.bots {
		if b != nil && b.ID == botID {
			b.Paused = !b.Paused
			st.saveLocked()
			return
		}
	}
}

// Delete removes a bot by ID.
func (st *BotStore) Delete(botID string) {
	if st == nil {
		return
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	out := st.bots[:0]
	for _, b := range st.bots {
		if b != nil && b.ID != botID {
			out = append(out, b)
		}
	}
	st.bots = out
	st.saveLocked()
}

// OpenBotHomeSession activates (or creates) the persistent Home Chat session
// for bot, tagging it with BotID.
func (as *AgentState) OpenBotHomeSession(store *BotStore, bot *BotProfile) *AgentSession {
	if as == nil || bot == nil {
		return nil
	}
	as.mu.Lock()
	if bot.HomeSessionID != "" {
		for _, s := range as.Sessions {
			if s != nil && s.ID == bot.HomeSessionID {
				as.ActiveSessionID = s.ID
				as.syncActiveSession()
				as.mu.Unlock()
				if as.Invalidate != nil {
					as.Invalidate()
				}
				return s
			}
		}
	}

	now := time.Now()
	id := as.nextSessionIDLocked()
	ws := bot.WorkspacePath
	if ws == "" {
		ws = as.workspacePath
	}
	model := bot.Model
	if model == "" {
		model = as.ActiveModel
	}
	sess := &AgentSession{
		ID:            id,
		Title:         bot.Name + " · Home",
		Status:        "idle",
		CreatedAt:     now,
		UpdatedAt:     now,
		Model:         model,
		Provider:      as.ActiveProvider,
		AgentID:       bot.AgentID,
		AgentType:     bot.AgentType,
		WorkspacePath: ws,
		BotID:         bot.ID,
		Tasks:         NewTaskManager(),
		nextMsgID:     1,
	}
	as.Sessions = append([]*AgentSession{sess}, as.Sessions...)
	as.ActiveSessionID = id
	as.syncActiveSession()
	as.mu.Unlock()
	as.applyPendingLLMModel()

	if store != nil {
		store.UpdateHomeSession(bot.ID, id)
	}
	as.SaveSessionToDisk(sess)
	if as.Invalidate != nil {
		as.Invalidate()
	}
	return sess
}

// HandToBot hands off sourceSessionID to bot with the given goal, tagging the
// newly created session with bot.ID so it appears under the Bot in the sidebar.
func (as *AgentState) HandToBot(bot *BotProfile, sourceSessionID, goal string, autoStart bool) (*AgentSession, error) {
	if as == nil || bot == nil {
		return nil, fmt.Errorf("bot is required")
	}
	nextSteps := strings.TrimSpace(bot.SystemPrompt)
	if bot.Role != "" {
		if nextSteps != "" {
			nextSteps = fmt.Sprintf("Act as **%s** (%s).\n%s", bot.Name, bot.Role, nextSteps)
		} else {
			nextSteps = fmt.Sprintf("Act as **%s** (%s) and complete the goal autonomously.", bot.Name, bot.Role)
		}
	}
	sess, err := as.ExecuteHandoff(HandoffRequest{
		SourceSessionID: sourceSessionID,
		TargetAgentID:   bot.AgentID,
		TargetAgentType: bot.AgentType,
		TargetModel:     bot.Model,
		Goal:            goal,
		NextSteps:       nextSteps,
		AutoStart:       autoStart,
	})
	if err != nil {
		return nil, err
	}
	as.mu.Lock()
	sess.BotID = bot.ID
	as.mu.Unlock()
	as.SaveSessionToDisk(sess)
	return sess, nil
}

// BotTemplate defines a ready-to-use persona and default harness preset for
// creating a Local Bot (inspired by oh-my-hermes bot templates).
type BotTemplate struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Role         string `json:"role"`
	Summary      string `json:"summary"`
	SystemPrompt string `json:"systemPrompt"`
	AgentID      string `json:"agentId"`
	AgentType    string `json:"agentType"`
	Avatar       string `json:"avatar,omitempty"`
}

// DefaultBotTemplates returns the built-in bot templates for quick onboarding.
func DefaultBotTemplates() []BotTemplate {
	return []BotTemplate{
		{
			ID:        "pr-reviewer",
			Title:     "PR Reviewer",
			Role:      "Bug-first PR & diff reviewer",
			Summary:   "Reviews pull requests bug-first, checks diffs and tests, and delegates deep fixes to background workers.",
			AgentID:   "agent-internal",
			AgentType: "internal",
			Avatar:    "07_chrome_mecha_bot.png",
			SystemPrompt: "You are a PR Reviewer bot. Keep the main chat concise for Q&A and decisions. " +
				"When asked to review or fix code, read the diff and touched files first, lead with bugs and risks with file:line citations, and delegate heavy multi-file fixes to a background task.",
		},
		{
			ID:        "ci-watch",
			Title:     "CI & Build Triage",
			Role:      "Build failure & test triage",
			Summary:   "Diagnoses failing builds or flaky tests, isolates root causes, and proposes targeted fixes.",
			AgentID:   "agent-internal",
			AgentType: "internal",
			Avatar:    "06_cyberpunk_android.png",
			SystemPrompt: "You are a CI & Build Triage bot. Keep chat focused on root-cause summaries and next-step options. " +
				"Delegate log inspection, reproduction, and test fixes to background tasks (RakitKode or ACP worktrees).",
		},
		{
			ID:           "release-notes",
			Title:        "Release Notes",
			Role:         "Changelog & release drafter",
			Summary:      "Summarizes merged PRs and commits since the last tag into clean user-facing release notes.",
			AgentID:      "agent-internal",
			AgentType:    "internal",
			Avatar:       "14_3d_clay_robot.png",
			SystemPrompt: "You are a Release Notes bot. Group merged changes by user impact, cite PR/commit numbers, and put breaking changes first. Never invent changes not present in git history.",
		},
		{
			ID:           "research-assistant",
			Title:        "Research & Docs",
			Role:         "Codebase & architecture researcher",
			Summary:      "Answers architecture questions with exact file citations and delegates deep codebase audits to background tasks.",
			AgentID:      "agent-internal",
			AgentType:    "internal",
			Avatar:       "10_owl_knowledge_bot.png",
			SystemPrompt: "You are a Research & Docs bot. Keep answers concise and evidence-backed with exact file paths and symbol names. Delegate broad audits to background tasks.",
		},
	}
}

// BotTaskStep represents a single step or tool execution inside a Bot's background task.
type BotTaskStep struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	Detail     string `json:"detail,omitempty"`
	Status     string `json:"status"` // "running", "completed", "failed"
	DurationMs int64  `json:"durationMs,omitempty"`
}

// BotTaskSummary is a live read-model of one background task owned by a BotProfile.
type BotTaskSummary struct {
	SessionID        string            `json:"sessionId"`
	BotID            string            `json:"botId"`
	Title            string            `json:"title"`
	Status           string            `json:"status"` // "running", "needs_approval", "complete", "failed", "idle"
	HarnessLabel     string            `json:"harnessLabel"`
	AgentID          string            `json:"agentId"`
	AgentType        string            `json:"agentType"`
	CurrentStep      string            `json:"currentStep,omitempty"`
	Summary          string            `json:"summary,omitempty"`
	Steps            []BotTaskStep     `json:"steps,omitempty"`
	Changes          []*FileChange     `json:"changes,omitempty"`
	PendingApprovals []PendingApproval `json:"pendingApprovals,omitempty"`
	CreatedAt        time.Time         `json:"createdAt"`
	UpdatedAt        time.Time         `json:"updatedAt"`
}

// TurnProcessStatus summarizes what an active chat session is currently doing
// (thinking, active tool call, step count, elapsed seconds, and bot background tasks).
type TurnProcessStatus struct {
	Active              bool
	IsThinking          bool
	CurrentTool         string
	CurrentToolCommand  string
	StepCount           int
	CompletedTools      int
	ElapsedSec          int
	PendingApproval     bool
	BackgroundRunning   int
	BackgroundApprovals int
}

// SpawnBotTask launches a background task under bot using either the internal
// RakitKode harness or an external ACP agent (with optional git worktree isolation).
func (as *AgentState) SpawnBotTask(store *BotStore, bot *BotProfile, title, taskPrompt, agentID, agentType string, useWorktree bool) (*AgentSession, error) {
	if as == nil || bot == nil {
		return nil, fmt.Errorf("bot is required")
	}
	prompt := strings.TrimSpace(taskPrompt)
	if prompt == "" {
		return nil, fmt.Errorf("task prompt is required")
	}
	home := as.OpenBotHomeSession(store, bot)
	parentID := ""
	if home != nil {
		parentID = home.ID
	}
	if strings.TrimSpace(agentID) == "" {
		agentID = bot.AgentID
	}
	if strings.TrimSpace(agentType) == "" {
		agentType = bot.AgentType
	}
	wsMode := "shared"
	if useWorktree {
		wsMode = "worktree"
	}
	workerName := strings.TrimSpace(title)
	if workerName == "" {
		workerName = prompt
		if len(workerName) > 42 {
			workerName = workerName[:42] + "…"
		}
	}
	if bot.SystemPrompt != "" {
		prompt = fmt.Sprintf("Bot Persona (%s — %s):\n%s\n\nTask:\n%s", bot.Name, bot.Role, bot.SystemPrompt, prompt)
	}
	rec, err := as.SpawnWorker(WorkerSpec{
		ParentSessionID: parentID,
		Name:            workerName,
		AgentID:         agentID,
		AgentType:       agentType,
		Model:           bot.Model,
		Task:            prompt,
		WorkspaceMode:   wsMode,
		AutoStart:       true,
	})
	if err != nil {
		return nil, err
	}
	as.mu.Lock()
	var taskSess *AgentSession
	for _, s := range as.Sessions {
		if s != nil && s.ID == rec.SessionID {
			s.BotID = bot.ID
			s.Title = workerName
			taskSess = s
			break
		}
	}
	as.mu.Unlock()
	if taskSess != nil {
		as.SaveSessionToDisk(taskSess)
	}
	if as.Invalidate != nil {
		as.Invalidate()
	}
	return taskSess, nil
}

// BotTasks returns all background task sessions owned by botID (excluding its Home Chat),
// sorted newest first with live steps, tool calls, summary, and pending approvals.
func (as *AgentState) BotTasks(store *BotStore, botID string) []BotTaskSummary {
	if as == nil || botID == "" {
		return nil
	}
	homeID := ""
	if store != nil {
		if b := store.Get(botID); b != nil {
			homeID = b.HomeSessionID
		}
	}
	as.mu.Lock()
	defer as.mu.Unlock()

	var out []BotTaskSummary
	for _, s := range as.Sessions {
		if s == nil || s.Archived {
			continue
		}
		if s.ID == homeID {
			// Also surface Home Chat turns that ran tools, edits, approvals, or are
			// currently processing so hidden "Worked for" steps appear in Tasks.
			harnessLabel := "Internal · RakitKode"
			if s.AgentType == "acp" {
				harnessLabel = "ACP · " + strings.TrimPrefix(s.AgentID, "agent-")
			}
			var lastUserText string
			for _, m := range s.Messages {
				if m == nil {
					continue
				}
				if m.Role == "user" {
					lastUserText = strings.TrimSpace(m.Text)
					continue
				}
				if m.Role != "assistant" {
					continue
				}
				hasWork := !m.Done || len(m.Tools) > 0 || len(m.Changes) > 0 || (m.PendingApproval != nil && !m.PendingApproval.Resolved)
				if !hasWork {
					continue
				}
				status := "complete"
				if !m.Done {
					status = "running"
				}
				var approvals []PendingApproval
				if m.PendingApproval != nil && !m.PendingApproval.Resolved {
					approvals = append(approvals, *m.PendingApproval)
					status = "needs_approval"
				}
				var steps []BotTaskStep
				currentStep := ""
				for _, t := range m.Tools {
					if t == nil {
						continue
					}
					detail := strings.TrimSpace(t.Command)
					if len(detail) > 120 {
						detail = detail[:120] + "…"
					}
					steps = append(steps, BotTaskStep{
						ID:         t.ID,
						Label:      t.ToolName,
						Detail:     detail,
						Status:     t.Status,
						DurationMs: t.DurationMs,
					})
					if t.Status == "running" {
						currentStep = t.ToolName
						if detail != "" {
							currentStep += ": " + detail
						}
					}
					if t.Status == "failed" && status != "running" && status != "needs_approval" {
						status = "failed"
					}
				}
				if currentStep == "" && status == "running" {
					if m.IsThinking {
						currentStep = "Thinking…"
					} else {
						currentStep = "Processing…"
					}
				}
				title := lastUserText
				if title == "" {
					title = "Chat task"
				}
				if len(title) > 52 {
					title = title[:52] + "…"
				}
				ts := m.Timestamp
				if ts.IsZero() {
					ts = s.UpdatedAt
				}
				out = append(out, BotTaskSummary{
					SessionID:        fmt.Sprintf("%s#turn-%d", s.ID, m.ID),
					BotID:            botID,
					Title:            title,
					Status:           status,
					HarnessLabel:     harnessLabel,
					AgentID:          s.AgentID,
					AgentType:        s.AgentType,
					CurrentStep:      currentStep,
					Summary:          strings.TrimSpace(m.Text),
					Steps:            steps,
					Changes:          m.Changes,
					PendingApprovals: approvals,
					CreatedAt:        ts,
					UpdatedAt:        ts,
				})
			}
			continue
		}
		if s.BotID != botID && (homeID == "" || s.ParentSessionID != homeID) {
			continue
		}

		status := "idle"
		switch s.Status {
		case "running":
			status = "running"
		case "completed", "done":
			status = "complete"
		case "failed", "error":
			status = "failed"
		}

		var approvals []PendingApproval
		for _, pa := range s.PendingApprovals {
			if pa != nil {
				if mapped := pendingApprovalFromRequest(s, pa); mapped != nil {
					approvals = append(approvals, *mapped)
				}
			}
		}

		var steps []BotTaskStep
		var changes []*FileChange
		seenChanges := make(map[string]bool)
		summary := ""
		currentStep := ""

		for _, m := range s.Messages {
			if m == nil {
				continue
			}
			if m.PendingApproval != nil && !m.PendingApproval.Resolved {
				dup := false
				for _, ex := range approvals {
					if ex.ToolCallID == m.PendingApproval.ToolCallID {
						dup = true
						break
					}
				}
				if !dup {
					approvals = append(approvals, *m.PendingApproval)
				}
			}
			for _, t := range m.Tools {
				if t == nil {
					continue
				}
				detail := strings.TrimSpace(t.Command)
				if len(detail) > 120 {
					detail = detail[:120] + "…"
				}
				steps = append(steps, BotTaskStep{
					ID:         t.ID,
					Label:      t.ToolName,
					Detail:     detail,
					Status:     t.Status,
					DurationMs: t.DurationMs,
				})
				if t.Status == "running" {
					currentStep = t.ToolName
					if detail != "" {
						currentStep += ": " + detail
					}
				}
				if t.Status == "failed" && status != "running" {
					status = "failed"
				}
			}
			for _, ch := range m.Changes {
				if ch != nil && !seenChanges[ch.Path] {
					seenChanges[ch.Path] = true
					changes = append(changes, ch)
				}
			}
			if m.Role == "assistant" && strings.TrimSpace(m.Text) != "" {
				summary = strings.TrimSpace(m.Text)
			}
		}

		if len(approvals) > 0 {
			status = "needs_approval"
		} else if status == "idle" && len(steps) > 0 && summary != "" {
			status = "complete"
		}

		if currentStep == "" && status == "running" {
			currentStep = "Thinking…"
		}

		harnessLabel := "Internal · RakitKode"
		if s.AgentType == "acp" {
			harnessLabel = "ACP · " + strings.TrimPrefix(s.AgentID, "agent-")
		}

		title := strings.TrimSpace(s.Title)
		if title == "" {
			title = strings.TrimSpace(s.WorkerName)
		}
		if title == "" {
			title = "Background task"
		}

		out = append(out, BotTaskSummary{
			SessionID:        s.ID,
			BotID:            botID,
			Title:            title,
			Status:           status,
			HarnessLabel:     harnessLabel,
			AgentID:          s.AgentID,
			AgentType:        s.AgentType,
			CurrentStep:      currentStep,
			Summary:          summary,
			Steps:            steps,
			Changes:          changes,
			PendingApprovals: approvals,
			CreatedAt:        s.CreatedAt,
			UpdatedAt:        s.UpdatedAt,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	return out
}

// ActiveTurnStatus computes the live processing indicator for sessionID
// (current summary step, active tool call, elapsed time, and bot background tasks).
func (as *AgentState) ActiveTurnStatus(store *BotStore, sessionID string) TurnProcessStatus {
	var res TurnProcessStatus
	if as == nil || sessionID == "" {
		return res
	}
	as.mu.Lock()
	var sess *AgentSession
	for _, s := range as.Sessions {
		if s != nil && s.ID == sessionID {
			sess = s
			break
		}
	}
	if sess == nil {
		as.mu.Unlock()
		return res
	}

	res.Active = sess.Status == "running"
	botID := sess.BotID
	for _, pa := range sess.PendingApprovals {
		if pa != nil {
			res.PendingApproval = true
			break
		}
	}
	for i := len(sess.Messages) - 1; i >= 0; i-- {
		m := sess.Messages[i]
		if m == nil {
			continue
		}
		if m.Role == "assistant" {
			res.IsThinking = m.IsThinking
			res.StepCount = m.TotalSteps
			if !m.TurnStartedAt.IsZero() && res.Active {
				res.ElapsedSec = int(time.Since(m.TurnStartedAt).Seconds())
			}
			for _, t := range m.Tools {
				if t == nil {
					continue
				}
				if t.Status == "running" {
					res.CurrentTool = t.ToolName
					res.CurrentToolCommand = strings.TrimSpace(t.Command)
				} else if t.Status == "completed" {
					res.CompletedTools++
				}
			}
			if res.StepCount == 0 && len(m.Tools) > 0 {
				res.StepCount = len(m.Tools)
			}
			if m.PendingApproval != nil && !m.PendingApproval.Resolved {
				res.PendingApproval = true
			}
			break
		}
	}
	as.mu.Unlock()

	if botID != "" {
		for _, t := range as.BotTasks(store, botID) {
			if strings.Contains(t.SessionID, "#turn-") {
				continue
			}
			if t.Status == "running" {
				res.BackgroundRunning++
			} else if t.Status == "needs_approval" {
				res.BackgroundApprovals++
			}
		}
	}
	return res
}
