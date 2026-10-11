package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/hasdev/forge-ade/internal/events"
	"github.com/hasdev/forge-ade/internal/jev"
	"github.com/hasdev/forge-ade/internal/llm"
	"github.com/hasdev/forge-ade/internal/mcp"
	"github.com/hasdev/forge-ade/internal/memory"
	"github.com/hasdev/forge-ade/internal/plugins"
	"github.com/hasdev/forge-ade/internal/skills"
	"github.com/hasdev/forge-ade/internal/terminal"
	"github.com/hasdev/forge-ade/internal/tools"
)

type RoleFilter string

const (
	RoleCoding   RoleFilter = "coding"
	RolePlanning RoleFilter = "planning"
	RoleResearch RoleFilter = "research"
	RoleCustom   RoleFilter = "custom"
)

type TaskStatus string

const (
	TaskPending    TaskStatus = "pending"
	TaskInProgress TaskStatus = "in_progress"
	TaskCompleted  TaskStatus = "completed"
)

type TaskItem struct {
	ID         string     `json:"id"`
	Title      string     `json:"title"`
	Status     TaskStatus `json:"status,omitempty"`
	ActiveForm string     `json:"active_form,omitempty"`
	Completed  bool       `json:"completed"`
}

// SessionProgress is durable, compact execution state (goal, done, todos) —
// injected into the system prompt each iteration so the model always knows its
// current objective instead of inferring it from a long transcript.
type SessionProgress struct {
	CurrentGoal    string   `json:"current_goal,omitempty"`
	CompletedSteps []string `json:"completed_steps,omitempty"`
	ActiveTodos    []string `json:"active_todos,omitempty"`
}

type SessionState string

const (
	StateIdle          SessionState = "idle"
	StateThinking      SessionState = "thinking"
	StateExecuting     SessionState = "executing"
	StateAwaiting      SessionState = "awaiting_approval"
	StateAwaitingInput SessionState = "awaiting_input"
)

type Session struct {
	ID               string              `json:"id"`
	Name             string              `json:"name"`
	RoleFilter       RoleFilter          `json:"role_filter"`
	State            SessionState        `json:"state"`
	Folder           string              `json:"folder"`
	ProjectName      string              `json:"project_name,omitempty"`
	Model            string              `json:"model,omitempty"`
	ParentSessionID  string              `json:"parent_session_id,omitempty"`
	SubagentIDs      []string            `json:"subagent_ids,omitempty"`
	Messages         []AgentMessage      `json:"messages"`
	Tasks            []TaskItem          `json:"tasks"`
	Progress         SessionProgress     `json:"progress,omitempty"`
	TokenUsage       llm.TokenStats      `json:"token_usage"`
	AutoApprove      bool                `json:"auto_approve"`
	PendingTools     []ContentBlock      `json:"pending_tools,omitempty"`
	PendingQuestions []tools.AskQuestion `json:"pending_questions,omitempty"`
	Dialect          string              `json:"dialect,omitempty"` // "" = native tool calling, "xml" = in-band
	// ContextTokens is the provider-reported prompt size of the last request
	// (real context window usage; 0 until the first model call of a session).
	ContextTokens int `json:"contextTokens,omitempty"`
	// CacheHitRate is the session-average prompt-cache hit rate:
	// (cumulative cached tokens) / (cumulative prompt tokens), combining
	// Anthropic-style CachedTokens and DeepSeek-style PromptCacheHitTokens.
	CacheHitRate float64   `json:"cacheHitRate,omitempty"`
	SystemPrompt string    `json:"system_prompt,omitempty"`
	CustomPrompt string    `json:"custom_prompt,omitempty"`
	CustomRules  string    `json:"custom_rules,omitempty"`
	CreatedAt    time.Time `json:"created_at,omitempty"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Manager struct {
	mu            sync.RWMutex
	sessions      map[string]*Session
	definitions   map[string]AgentDefinition
	llmClient     *llm.LLMClient
	toolReg       *tools.Registry
	skillMgr      *skills.Manager
	pluginMgr     *plugins.Manager
	memoryMgr     *memory.Manager
	mcpMgr        *mcp.Manager
	termMgr       *terminal.Manager
	jevRouter     *jev.Router
	shells        map[string]string                  // agent session id → persistent shell id
	fileStates    map[string]*tools.FileStateTracker // session id → read-state tracker
	bus           *events.Bus
	cancelFuncs   map[string]context.CancelFunc
	runningTurns  map[string]bool
	loopDetectors map[string]*LoopDetector
	subagentNotes map[string][]SubagentNotification // parent session id → queued subagent notifications
	storePath     string

	// defaultFolder is the workspace root the app is currently serving.
	// Sessions created without an explicit folder are scoped to it — never to
	// the process working directory, which is "/" for a Finder-launched app.
	defaultFolder atomic.Value // string

	// Coalesced persistence — a turn can do dozens of tool calls; rewriting
	// the whole sessions JSON on every one is wasteful.
	persistMu    sync.Mutex
	persistTimer *time.Timer
	persistDirty bool
}

type SubagentNotification struct {
	ID        string
	Name      string
	Role      string
	Output    string
	Completed time.Time
}

func NewManager(llmClient *llm.LLMClient, toolReg *tools.Registry, skillMgr *skills.Manager, pluginMgr *plugins.Manager, mcpMgr *mcp.Manager, termMgr *terminal.Manager, bus *events.Bus, dataDir string) *Manager {
	storePath := filepath.Join(dataDir, "agent_sessions.json")
	m := &Manager{
		sessions:      make(map[string]*Session),
		definitions:   make(map[string]AgentDefinition),
		llmClient:     llmClient,
		toolReg:       toolReg,
		skillMgr:      skillMgr,
		pluginMgr:     pluginMgr,
		memoryMgr:     memory.New(dataDir),
		mcpMgr:        mcpMgr,
		termMgr:       termMgr,
		jevRouter:     jev.NewRouter(""),
		shells:        make(map[string]string),
		bus:           bus,
		cancelFuncs:   make(map[string]context.CancelFunc),
		runningTurns:  make(map[string]bool),
		loopDetectors: make(map[string]*LoopDetector),
		subagentNotes: make(map[string][]SubagentNotification),
		storePath:     storePath,
	}
	m.loadSessions()
	m.loadDefinitions()
	m.syncPlugins("")

	// Stream background-task lifecycle (bash run_in_background) to the UI and
	// queue completion notifications for the owning session's next model step.
	tools.SetTaskUpdateHandler(func(task *tools.BackgroundTask) {
		data := map[string]interface{}{
			"task_id":    task.ID,
			"command":    task.Command,
			"status":     string(task.Status),
			"session_id": task.SessionID,
		}
		if task.Status != tools.TaskRunning {
			data["exit_code"] = task.ExitCode
		}
		m.emitAgentEvent(events.AgentTaskUpdate, task.SessionID, data)
	})
	return m
}

func (m *Manager) SetJevRouter(r *jev.Router) {
	m.mu.Lock()
	m.jevRouter = r
	m.mu.Unlock()
}

func (m *Manager) GetJevRouter() *jev.Router {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.jevRouter
}

func (m *Manager) SetMemoryManager(memMgr *memory.Manager) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.memoryMgr = memMgr
}

func (m *Manager) MemoryManager() *memory.Manager {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.memoryMgr
}

func (m *Manager) loadSessions() {
	m.mu.Lock()
	defer m.mu.Unlock()

	loadSessionFile := func(filePath string) {
		data, rerr := os.ReadFile(filePath)
		if rerr != nil {
			return
		}
		var s Session
		if json.Unmarshal(data, &s) != nil || s.ID == "" {
			return
		}
		s.State = StateIdle
		// Sessions persisted before CreatedAt existed default to their
		// last update time so ordering stays stable across reloads.
		if s.CreatedAt.IsZero() {
			s.CreatedAt = s.UpdatedAt
		}
		// Rebuild the system prompt so sessions created before the
		// prompt overhaul pick up style guidance and custom content.
		if !strings.Contains(s.SystemPrompt, "Response style") {
			s.SystemPrompt = buildSystemPrompt(s.RoleFilter, s.CustomPrompt, s.CustomRules)
		}
		m.sessions[s.ID] = &s
	}

	// Current layout: one file per session under sessions/ or sessions/[project-name]/
	// (lazy-loadable, crash-safe, never rewrites unrelated sessions).
	dir := filepath.Join(filepath.Dir(m.storePath), "sessions")
	entries, err := os.ReadDir(dir)
	if err == nil && len(entries) > 0 {
		for _, e := range entries {
			if e.IsDir() {
				subEntries, serr := os.ReadDir(filepath.Join(dir, e.Name()))
				if serr == nil {
					for _, se := range subEntries {
						if !se.IsDir() && strings.HasSuffix(se.Name(), ".json") && se.Name() != "info.json" {
							loadSessionFile(filepath.Join(dir, e.Name(), se.Name()))
						}
					}
				}
				continue
			}
			if !strings.HasSuffix(e.Name(), ".json") || e.Name() == "info.json" {
				continue
			}
			loadSessionFile(filepath.Join(dir, e.Name()))
		}
		_ = os.Remove(m.storePath) // legacy single-file store, if any
		return
	}

	// Legacy single-file store → migrate to sessions/.
	data, err := os.ReadFile(m.storePath)
	if err != nil {
		return
	}
	var list []*Session
	if json.Unmarshal(data, &list) != nil {
		return
	}
	kept := 0
	for _, s := range list {
		if s == nil || s.ID == "" {
			continue
		}
		// Fresh data only: drop any session whose messages aren't
		// block-shaped (legacy flat-format history from before the
		// agent engine rewrite is stale and not migrated).
		if !messagesAreBlockShaped(s.Messages) {
			continue
		}
		s.State = StateIdle
		if s.CreatedAt.IsZero() {
			s.CreatedAt = s.UpdatedAt
		}
		if !strings.Contains(s.SystemPrompt, "Response style") {
			s.SystemPrompt = buildSystemPrompt(s.RoleFilter, s.CustomPrompt, s.CustomRules)
		}
		m.sessions[s.ID] = s
		kept++
	}
	// If nothing survived the block-shape filter, start fresh: drop the old
	// store. Otherwise write per-session files and remove the legacy blob.
	if kept == 0 && len(list) > 0 {
		m.sessions = make(map[string]*Session)
	}
	if kept >= 0 {
		m.saveSessionsLocked()
	}
}

// messagesAreBlockShaped reports whether every message uses the block-based
// content model (each non-empty message has a content array of blocks).
func messagesAreBlockShaped(msgs []AgentMessage) bool {
	for _, msg := range msgs {
		if len(msg.Content) == 0 {
			return false
		}
	}
	return true
}

func (m *Manager) saveSessionLocked(id string) {
	if m.storePath == "" {
		return
	}
	s, ok := m.sessions[id]
	if !ok {
		return
	}
	dir := filepath.Join(filepath.Dir(m.storePath), "sessions")
	_ = os.MkdirAll(dir, 0755)
	data, err := json.MarshalIndent(s, "", "  ")
	if err == nil {
		_ = os.WriteFile(filepath.Join(dir, s.ID+".json"), data, 0644)
	}
}

func (m *Manager) saveSessionsLocked() {
	if m.storePath == "" {
		return
	}
	// Per-session files: each session is one small atomic write instead of a
	// giant rewritable JSON blob — lazy-loadable, crash-safe, easy to delete.
	dir := filepath.Join(filepath.Dir(m.storePath), "sessions")
	_ = os.MkdirAll(dir, 0755)
	for _, s := range m.sessions {
		data, err := json.MarshalIndent(s, "", "  ")
		if err == nil {
			_ = os.WriteFile(filepath.Join(dir, s.ID+".json"), data, 0644)
		}
	}
	// The legacy single-file store is superseded by sessions/.
	_ = os.Remove(m.storePath)
}

// scheduleSave coalesces session JSON writes. A turn can issue many tool
// calls; each would otherwise rewrite the whole sessions file. The write is
// deferred ~500ms and always flushed by finishTurn/Stop, so nothing is lost.
// Safe to call from anywhere (uses its own mutex, not m.mu).
func (m *Manager) scheduleSave() {
	m.persistMu.Lock()
	m.persistDirty = true
	if m.persistTimer != nil {
		m.persistTimer.Stop()
	}
	m.persistTimer = time.AfterFunc(persistDebounce, func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		m.persistMu.Lock()
		dirty := m.persistDirty
		m.persistDirty = false
		m.persistMu.Unlock()
		if dirty {
			m.saveSessionsLocked()
		}
	})
	m.persistMu.Unlock()
}

// SetDefaultFolder sets the workspace root that sessions without an explicit
// folder are scoped to. Called by the app whenever a workspace is opened.
func (m *Manager) SetDefaultFolder(folder string) {
	m.defaultFolder.Store(folder)
}

// resolveSessionFolder normalizes a requested folder to an absolute path.
// Empty requests fall back to the current workspace, then the home directory —
// never the process working directory (which is "/" for a launched app bundle).
func (m *Manager) resolveSessionFolder(folder string) string {
	if folder != "" {
		if abs, err := filepath.Abs(folder); err == nil {
			return abs
		}
		return folder
	}
	if def, ok := m.defaultFolder.Load().(string); ok && def != "" {
		return def
	}
	if home, err := os.UserHomeDir(); err == nil {
		return home
	}
	return ""
}

func (m *Manager) CreateSession(name string, role RoleFilter, folder string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := uuid.New().String()
	if name == "" {
		name = fmt.Sprintf("%s Agent (%s)", getRoleTitle(role), time.Now().Format("15:04:05"))
	}

	// Normalize folder to an absolute path; when empty, fall back to the
	// current workspace so sessions are always linked to a project.
	projFolder := m.resolveSessionFolder(folder)

	sysPrompt := buildSystemPrompt(role, "", "")

	now := time.Now()
	model := ""
	if m.llmClient != nil {
		model = m.llmClient.GetConfig().Model
	}
	sess := &Session{
		ID:           id,
		Name:         name,
		RoleFilter:   role,
		Model:        model,
		State:        StateIdle,
		Folder:       projFolder,
		ProjectName:  filepath.Base(projFolder),
		Messages:     make([]AgentMessage, 0),
		Tasks:        make([]TaskItem, 0),
		SystemPrompt: sysPrompt,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	m.sessions[id] = sess
	m.saveSessionsLocked()
	m.emitSessionUpdate(id)
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
	// Only overwrite custom prompt/rules when the caller supplies them; an
	// empty value means "leave unchanged" (e.g. a pure rename call).
	if customPrompt != "" {
		sess.CustomPrompt = customPrompt
	}
	if customRules != "" {
		sess.CustomRules = customRules
	}
	sess.SystemPrompt = buildSystemPrompt(sess.RoleFilter, sess.CustomPrompt, sess.CustomRules)
	sess.UpdatedAt = time.Now()
	m.saveSessionsLocked()
	m.mu.Unlock()

	m.emitSessionUpdate(id)
	return sess, nil
}

// SetDialect switches a session's tool-calling dialect. "" = native tool
// calling (default), "xml" = in-band XML tool calling (for providers without
// reliable native tools, e.g. some DeepSeek/GLM endpoints).
func (m *Manager) SetDialect(id string, dialect string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	sess, ok := m.sessions[id]
	if !ok {
		return fmt.Errorf("session %s not found", id)
	}
	if dialect != "" && dialect != "xml" {
		return fmt.Errorf("unsupported dialect %q (supported: xml)", dialect)
	}
	sess.Dialect = dialect
	sess.UpdatedAt = time.Now()
	m.saveSessionsLocked()
	return nil
}

// SetAutoApprove toggles "yolo" mode for a session: when enabled, mutating
// tool calls are approved automatically without pausing for user approval.
func (m *Manager) SetAutoApprove(id string, enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	sess, ok := m.sessions[id]
	if !ok {
		return fmt.Errorf("session %s not found", id)
	}
	sess.AutoApprove = enabled
	sess.UpdatedAt = time.Now()
	for _, subID := range sess.SubagentIDs {
		if sub, ok := m.sessions[subID]; ok {
			sub.AutoApprove = enabled
			sub.UpdatedAt = time.Now()
		}
	}
	m.saveSessionsLocked()
	return nil
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
	// Stable order by creation time then name — activity (UpdatedAt) must NOT
	// reorder panels, or the active panel visually jumps position while typing.
	sort.SliceStable(list, func(i, j int) bool {
		if !list[i].CreatedAt.Equal(list[j].CreatedAt) {
			return list[i].CreatedAt.Before(list[j].CreatedAt)
		}
		return list[i].Name < list[j].Name
	})
	return list
}

// ListSessionsForFolder returns only the sessions linked to the given project
// folder (and subfolders). Sessions are project-scoped: opening a workspace
// shows only the agent sessions that belong to it, not every project's history.
func (m *Manager) ListSessionsForFolder(folder string) []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if folder == "" {
		return nil
	}
	abs, err := filepath.Abs(folder)
	if err != nil {
		abs = folder
	}
	list := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		if s.Folder == "" {
			continue
		}
		sAbs, err := filepath.Abs(s.Folder)
		if err != nil {
			sAbs = s.Folder
		}
		// Match if the session folder is under the project folder (or equal).
		rel, err := filepath.Rel(abs, sAbs)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			list = append(list, s)
		}
	}
	sort.SliceStable(list, func(i, j int) bool {
		if !list[i].CreatedAt.Equal(list[j].CreatedAt) {
			return list[i].CreatedAt.Before(list[j].CreatedAt)
		}
		return list[i].Name < list[j].Name
	})
	return list
}

func (m *Manager) DeleteSession(id string) {
	m.mu.Lock()
	if cancel, ok := m.cancelFuncs[id]; ok {
		cancel()
		delete(m.cancelFuncs, id)
	}
	delete(m.sessions, id)
	if m.storePath != "" {
		dir := filepath.Join(filepath.Dir(m.storePath), "sessions")
		_ = os.Remove(filepath.Join(dir, id+".json"))
	}
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

	// Prepare user text with mentioned paths context. Files are inlined
	// (capped) so the model sees their content; folders are listed.
	fullContent := userContent
	if ctxText := buildMentionContext(mentionedPaths); ctxText != "" {
		fullContent += "\n\n" + ctxText
	}

	// Skill invocation: detect a leading or mid-prompt `/skill:<name>` token and
	// inject the skill's SKILL.md body as a user message
	// parseSkillInvocation + buildSkillPromptMessage).
	messages := []AgentMessage{
		{
			ID:   uuid.New().String(),
			Role: "user",
			Content: []ContentBlock{
				{Type: "text", Text: fullContent},
			},
			Timestamp: time.Now(),
		},
	}
	// Slash mentions & commands: /goal, /plan, /plan-review, /security, /vibe, /model, /clear, /skill, /mcp
	var goalOverride string
	for _, mention := range extractSlashMentions(fullContent) {
		switch mention.Kind {
		case "goal":
			goalText := strings.TrimSpace(mention.Name)
			if goalText == "" {
				goalText = userContent
			}
			goalOverride = goalText
			sess.Tasks = []TaskItem{}
			sess.Progress.ActiveTodos = nil
			sess.Progress.CompletedSteps = nil
			directive := fmt.Sprintf("<goal_directive>\nGOAL SET: %q\nA high-level goal has been defined. You MUST immediately call update_tasks at the very start of your turn to decompose this goal into a sequential task checklist (status: 'pending', IDs: '1', '2', ...).\nThen proceed to execute the tasks systematically, updating each task to 'in_progress' with active_form while working and 'completed' once verified.\nDo not skip creating tasks.\n</goal_directive>", goalText)
			messages = append(messages, AgentMessage{
				ID:   uuid.New().String(),
				Role: "user",
				Content: []ContentBlock{
					{Type: "text", Text: directive},
				},
				Timestamp: time.Now(),
			})

		case "plan":
			planText := strings.TrimSpace(mention.Name)
			if planText == "" {
				planText = userContent
			}
			goalOverride = "Plan: " + planText
			sess.Tasks = []TaskItem{}
			sess.Progress.ActiveTodos = nil
			sess.Progress.CompletedSteps = nil
			directive := fmt.Sprintf("<plan_directive>\nPLAN MODE ACTIVE for: %q\nYou are in Plan Mode. Carefully investigate the codebase, files, and architecture before making any edits. Formulate an execution plan and call update_tasks with your proposed breakdown of steps. Present the plan to the user for confirmation before altering files.\n</plan_directive>", planText)
			messages = append(messages, AgentMessage{
				ID:   uuid.New().String(),
				Role: "user",
				Content: []ContentBlock{
					{Type: "text", Text: directive},
				},
				Timestamp: time.Now(),
			})

		case "plan-review":
			directive := "<plan_review_directive>\nPLAN REVIEW MODE:\nCritically review the active execution plan, recent tool changes, and architecture. Identify edge cases, potential regressions, error handling needs, and testing gaps. If adjustments are necessary, call update_tasks with the updated plan.\n</plan_review_directive>"
			messages = append(messages, AgentMessage{
				ID:   uuid.New().String(),
				Role: "user",
				Content: []ContentBlock{
					{Type: "text", Text: directive},
				},
				Timestamp: time.Now(),
			})

		case "security":
			goalOverride = "Security Audit"
			directive := "<security_directive>\nSECURITY SCAN & AUDIT MODE:\nRun a comprehensive security inspection across the workspace. Scan for hardcoded credentials, secret leakage, injection flaws, path traversal, untrusted input, and unpatched dependencies. Call update_tasks with your audit steps, execute the checks, and provide a clear remediation report.\n</security_directive>"
			messages = append(messages, AgentMessage{
				ID:   uuid.New().String(),
				Role: "user",
				Content: []ContentBlock{
					{Type: "text", Text: directive},
				},
				Timestamp: time.Now(),
			})

		case "vibe":
			goalOverride = "Vibe Mode Execution"
			directive := "<vibe_directive>\nVIBE CODING MODE:\nExecute autonomously, rapidly, and end-to-end. Make code changes, run tests and build checks, fix compiler or lint errors directly without waiting for manual confirmation. Prioritize velocity, working software, and clean code.\n</vibe_directive>"
			messages = append(messages, AgentMessage{
				ID:   uuid.New().String(),
				Role: "user",
				Content: []ContentBlock{
					{Type: "text", Text: directive},
				},
				Timestamp: time.Now(),
			})

		case "model":
			activeModel := sess.Model
			if activeModel == "" && m.llmClient != nil {
				activeModel = m.llmClient.GetConfig().Model
			}
			directive := fmt.Sprintf("<model_info>\nSession Model: %s (Provider: %s)\nContext tokens: %d\nSession tokens: %d total (%d prompt, %d completion, %d cached)\n</model_info>",
				activeModel, inferProvider(activeModel), sess.ContextTokens, sess.TokenUsage.TotalTokens, sess.TokenUsage.PromptTokens, sess.TokenUsage.CompletionTokens, sess.TokenUsage.CachedTokens)
			messages = append(messages, AgentMessage{
				ID:   uuid.New().String(),
				Role: "user",
				Content: []ContentBlock{
					{Type: "text", Text: directive},
				},
				Timestamp: time.Now(),
			})

		case "clear":
			sess.Tasks = []TaskItem{}
			sess.Progress.CurrentGoal = ""
			sess.Progress.ActiveTodos = nil
			sess.Progress.CompletedSteps = nil
			directive := "<system_notice>\nSession tasks and active progress checklist have been cleared.\n</system_notice>"
			messages = append(messages, AgentMessage{
				ID:   uuid.New().String(),
				Role: "user",
				Content: []ContentBlock{
					{Type: "text", Text: directive},
				},
				Timestamp: time.Now(),
			})

		case "resume":
			sessions := m.ListSessionsForFolder(sess.Folder)
			var sb strings.Builder
			sb.WriteString("<session_resume>\nAvailable sessions in this workspace:\n")
			for _, s := range sessions {
				marker := " "
				if s.ID == sess.ID {
					marker = "*"
				}
				name := s.Name
				if name == "" {
					name = "Untitled session"
				}
				sb.WriteString(fmt.Sprintf("%s [%s] %s (created %s, %d messages)\n", marker, s.ID[:8], name, s.CreatedAt.Format("2006-01-02 15:04"), len(s.Messages)))
			}
			sb.WriteString("To resume or review context from a specific session, reference its ID or switch using the session sidebar.\n</session_resume>")
			messages = append(messages, AgentMessage{
				ID:   uuid.New().String(),
				Role: "user",
				Content: []ContentBlock{
					{Type: "text", Text: sb.String()},
				},
				Timestamp: time.Now(),
			})

		case "settings", "setup":
			directive := "<system_notice>\nSettings and provider setup menu requested. In ForgeADE, configure API keys, custom providers, and default models in the Settings screen (Cmd+, or Settings tab).\n</system_notice>"
			messages = append(messages, AgentMessage{
				ID:   uuid.New().String(),
				Role: "user",
				Content: []ContentBlock{
					{Type: "text", Text: directive},
				},
				Timestamp: time.Now(),
			})

		case "skill":
			if m.skillMgr == nil {
				continue
			}
			skillName := strings.TrimPrefix(mention.Name, "skill:")
			if skill, found := m.skillMgr.Get(skillName); found {
				messages = append(messages, AgentMessage{
					ID:   uuid.New().String(),
					Role: "user",
					Content: []ContentBlock{
						{Type: "text", Text: skill.InvocationMessage(mention.Args)},
					},
					Timestamp: time.Now(),
				})
			} else {
				var sb strings.Builder
				sb.WriteString(fmt.Sprintf("Skill %q was requested. Available skills:\n", skillName))
				for _, s := range m.skillMgr.List() {
					sb.WriteString(fmt.Sprintf("- %s: %s\n", s.Name, s.Description))
				}
				messages = append(messages, AgentMessage{
					ID:   uuid.New().String(),
					Role: "user",
					Content: []ContentBlock{
						{Type: "text", Text: sb.String()},
					},
					Timestamp: time.Now(),
				})
			}

		case "mcp":
			if m.mcpMgr == nil {
				continue
			}
			directive := buildMCPDirective(m.mcpMgr, mention)
			if directive != "" {
				messages = append(messages, AgentMessage{
					ID:   uuid.New().String(),
					Role: "user",
					Content: []ContentBlock{
						{Type: "text", Text: directive},
					},
					Timestamp: time.Now(),
				})
			}
		}
	}

	sess.Messages = append(sess.Messages, messages...)
	sess.State = StateThinking

	// Track the current objective so the agent's system prompt states it explicitly.
	if goalOverride != "" {
		sess.Progress.CurrentGoal = goalOverride
	} else if goal := firstLine(userContent); goal != "" {
		sess.Progress.CurrentGoal = goal
	}

	// Auto-name the session on the first real user message.
	// We set an instant smart title immediately from the prompt text so
	// the sidebar updates instantly without waiting for an LLM call.
	shouldAutoTitle := sess.Name == "" ||
		strings.HasPrefix(sess.Name, "Agent") ||
		strings.HasPrefix(sess.Name, "Coding Agent (")
	if shouldAutoTitle {
		sess.Name = formatSmartSessionTitle(userContent)
	}
	m.saveSessionLocked(sessionID)
	m.mu.Unlock()

	// EMIT IMMEDIATELY SO USER MESSAGE APPEARS INSTANTLY IN REAL-TIME
	m.emitSessionUpdate(sessionID)

	// Launch background execution loop for agent turn if not already running.
	// If already running, the new message was already appended to sess.Messages
	// and will be naturally processed by the active reasoning loop.
	m.mu.Lock()
	if m.runningTurns[sessionID] {
		m.mu.Unlock()
		return nil
	}
	execCtx, cancel := context.WithCancel(context.Background())
	m.cancelFuncs[sessionID] = cancel
	m.mu.Unlock()

	m.launchTurnGoroutine(execCtx, sessionID)

	return nil
}

// launchTurnGoroutine spawns a managed background goroutine for runAgentTurn,
// ensuring strictly one active execution loop per session.
func (m *Manager) launchTurnGoroutine(ctx context.Context, sessionID string) {
	m.mu.Lock()
	if m.runningTurns[sessionID] {
		m.mu.Unlock()
		return
	}
	m.runningTurns[sessionID] = true
	m.mu.Unlock()

	go func() {
		defer func() {
			m.mu.Lock()
			delete(m.runningTurns, sessionID)
			m.mu.Unlock()
		}()
		m.runAgentTurn(ctx, sessionID)
	}()
}

// parseSkillInvocation detects a `/skill:<name>` invocation in a user draft,
// returning the skill name, the remaining args, and whether it matched. Both
// the leading form (`/skill:foo bar`) and mid-prompt form (`fix /skill:foo`)
// are supported.
func parseSkillInvocation(text string) (string, string, bool) {
	trimmed := strings.TrimSpace(text)
	if strings.HasPrefix(trimmed, "/skill:") {
		spaceIdx := strings.Index(trimmed, " ")
		name := trimmed[len("/skill:"):]
		if spaceIdx != -1 {
			name = trimmed[len("/skill:"):spaceIdx]
		}
		name = strings.TrimSpace(name)
		if name == "" {
			return "", "", false
		}
		args := ""
		if spaceIdx != -1 {
			args = strings.TrimSpace(trimmed[spaceIdx+1:])
		}
		return name, args, true
	}
	// Mid-prompt form: /skill:<name> surrounded by prose.
	re := regexp.MustCompile(`(^|\s)/skill:([^\s/]+)(\s|$)`)
	m := re.FindStringSubmatch(text)
	if m == nil {
		return "", "", false
	}
	name := strings.TrimSpace(m[2])
	if name == "" {
		return "", "", false
	}
	leading := m[1]
	trailing := m[3]
	tokenStart := strings.Index(text, m[0]) + len(leading)
	tokenEnd := tokenStart + len(m[0]) - len(leading) - len(trailing)
	before := strings.TrimSpace(text[:tokenStart])
	after := strings.TrimSpace(text[tokenEnd:])
	var parts []string
	if before != "" {
		parts = append(parts, before)
	}
	if after != "" {
		parts = append(parts, after)
	}
	return name, strings.Join(parts, " "), true
}

// maxReasoningSteps bounds the number of LLM calls per user message so a
// runaway tool loop (e.g. a model that never finishes calling tools) cannot
// spin forever. Each iteration may carry several parallel tool calls, so this
// is generous enough for real coding tasks.
const maxReasoningSteps = 120

// maxToolBudget is the per-turn tool budget in points (cheap=1, medium=3,
// high=10), surfaced to the model each iteration as guidance.
const maxToolBudget = 300

// persistDebounce coalesces session JSON writes during a turn.
const persistDebounce = 500 * time.Millisecond

// runAgentTurn runs one iteration of the agent loop: build context, stream
// from the LLM, execute any tool calls (through the approval gate), and
// re-enter until the model stops calling tools or the iteration cap is hit.
// This is ForgeADE's agent turn loop.
func (m *Manager) runAgentTurn(ctx context.Context, sessionID string) {
	// Initialize or reset loop detector for this turn to prevent runaway loops.
	m.mu.Lock()
	if _, ok := m.loopDetectors[sessionID]; !ok {
		m.loopDetectors[sessionID] = NewLoopDetector(DefaultLoopWindowSize, DefaultLoopMaxRepeats)
	} else {
		m.loopDetectors[sessionID].Reset()
	}
	m.mu.Unlock()

	// Project/skills context is stable for the whole turn — building it per
	// iteration re-runs a git snapshot (seconds) every loop. Build once.
	m.mu.RLock()
	folder := ""
	if sess, ok := m.sessions[sessionID]; ok {
		folder = sess.Folder
	}
	m.mu.RUnlock()
	m.syncPlugins(sessionID)
	projectCtx := ""
	if folder != "" {
		projectCtx = buildProjectContext(folder)
	}
	skillsCtx := m.buildSkillsContext()

	// Route tools via TypeSafe AI Jev router if enabled.
	// Jev classifies the user's intent into focused categories (explore, read, modify, execute, git, web, none)
	// preventing tool hallucinations and excessive schema overhead.
	var routedTools []llm.ToolDefinition
	jevRouted := false
	if m.jevRouter != nil && m.jevRouter.IsEnabled() {
		m.mu.RLock()
		sess := m.sessions[sessionID]
		goal := ""
		if sess != nil {
			goal = sess.Progress.CurrentGoal
			if goal == "" {
				for i := len(sess.Messages) - 1; i >= 0; i-- {
					if sess.Messages[i].Role == "user" {
						for _, b := range sess.Messages[i].Content {
							if b.Type == "text" && b.Text != "" {
								goal = b.Text
								break
							}
						}
						if goal != "" {
							break
						}
					}
				}
			}
		}
		m.mu.RUnlock()

		allTools := m.toolReg.Definitions()
		if goal != "" {
			var decision *jev.RouteDecision
			routedTools, decision = m.jevRouter.Route(ctx, goal, allTools)
			jevRouted = true
			if decision != nil {
				log.Printf("[agent:%s] Jev router classified goal: category=%s, needs_tools=%.2f, confidence=%.2f -> active_tools=%d/%d",
					sessionID, decision.Category, decision.NeedsTools, decision.Confidence, len(routedTools), len(allTools))
			}
		}
	}

	budgetSpent := 0
	for iter := 0; iter < maxReasoningSteps; iter++ {
		turnNum := iter + 1
		if ctx.Err() != nil {
			m.finishTurn(sessionID, ctx.Err().Error())
			return
		}

		m.mu.RLock()
		sess, ok := m.sessions[sessionID]
		if !ok {
			m.mu.RUnlock()
			return
		}
		if sess.Model == "" && m.llmClient != nil {
			sess.Model = m.llmClient.GetConfig().Model
		}

		// Build LLM messages payload from block-based messages, with proper
		// tool_call_id correlation (the old loop dropped the ID on tool
		// results, which most providers reject). The transcript is windowed
		// so long sessions don't bloat the context; the stored session keeps
		// the full history for the UI.
		history, contextSummary := windowTranscriptWithSummary(sess.Messages, 40)
		sysContent := sess.SystemPrompt
		sysContent += projectCtx
		sysContent += skillsCtx
		if m.memoryMgr != nil {
			if memPrompt := m.memoryMgr.FormatPrompt(); memPrompt != "" {
				sysContent += "\n\n" + memPrompt
			}
		}
		if m.pluginMgr != nil {
			for _, pluginPrompt := range m.pluginMgr.ActiveSystemPrompts() {
				sysContent += "\n\n" + pluginPrompt
			}
		}
		// Completed background tasks (bash run_in_background) announce their
		// outcome here — the model learns results without polling task_output.
		if notes := tools.DrainTaskNotifications(sessionID); len(notes) > 0 {
			sysContent += "\n\n" + formatTaskNotifications(notes)
		}
		// Proactively announced sub-agent completions from child sessions
		if subNotes := m.drainSubagentNotifications(sessionID); len(subNotes) > 0 {
			sysContent += "\n\n" + formatSubagentNotifications(subNotes)
		}
		if contextSummary != "" {
			sysContent += "\n\n" + contextSummary
		} else if len(history) < len(sess.Messages) {
			sysContent += "\n\n(Note: older conversation history was truncated from this request to fit the context window. The original task and recent exchanges are preserved.)"
		}
		if remaining := maxToolBudget - budgetSpent; remaining > 0 {
			sysContent += fmt.Sprintf("\n\nTool budget remaining: %d points (cheap=1, medium=3, high=10). Prefer batched cheap tools (read_multiple, search) over many tiny calls; bash is expensive.", remaining)
		} else {
			sysContent += "\n\nTool budget exhausted — wrap up with a summary; no more tool calls."
		}
		if p := sess.Progress; p.CurrentGoal != "" || len(p.ActiveTodos) > 0 || len(p.CompletedSteps) > 0 {
			sysContent += "\n\n## Current objective & tasks\n"
			if p.CurrentGoal != "" {
				sysContent += "Goal: " + p.CurrentGoal + "\n"
			}
			if len(p.CompletedSteps) > 0 {
				sysContent += "Completed: " + strings.Join(p.CompletedSteps, ", ") + "\n"
			}
			if len(p.ActiveTodos) > 0 {
				sysContent += "Todos: " + strings.Join(p.ActiveTodos, ", ") + "\n"
			}
			if len(p.ActiveTodos) == 0 && len(p.CompletedSteps) > 0 {
				sysContent += "(Note: All previous tasks are completed. When the user gives new instructions or next steps, call update_tasks with a fresh task breakdown to replace this checklist with the new plan.)\n"
			}
			if p.CurrentGoal != "" && len(sess.Tasks) == 0 {
				sysContent += "(MANDATORY: A goal is active but no tasks exist yet. You MUST call update_tasks as your first tool call to decompose this goal into a sequential task checklist.)\n"
			}
		}
		messages := []llm.LLMMessage{
			{Role: llm.RoleSystem, Content: sysContent},
		}
		state := sess.State
		sess.State = StateThinking
		sess.UpdatedAt = time.Now()
		m.scheduleSave()
		m.mu.RUnlock()

		m.emitSessionUpdate(sessionID)
		if state == StateIdle {
			m.emitAgentEvent(events.AgentTurnStart, sessionID, map[string]interface{}{})
		}

		// Fetch tool definitions (built-ins + MCP tools registered into the
		// registry; filtered by Jev router if active; dialect mode will strip these and use in-band text calls).
		var toolDefs []llm.ToolDefinition
		if jevRouted {
			toolDefs = routedTools
		} else {
			toolDefs = m.toolReg.Definitions()
		}

		m.mu.RLock()
		dialect := sess.Dialect
		m.mu.RUnlock()

		var dialectScanner *xmlDialectScanner
		isPureDialect := dialect == "xml"
		if isPureDialect {
			// In-band mode: inject the dialect prompt + tool catalog, re-encode
			// the transcript as XML text, and send NO native tools.
			catalog := ""
			if len(toolDefs) > 0 {
				sysContent += "\n" + xmlDialectPrompt + toolCatalogText(toolDefs)
				catalog = toolCatalogText(toolDefs)
			}
			messages = []llm.LLMMessage{
				{Role: llm.RoleSystem, Content: sysContent},
			}
			messages = append(messages, llm.LLMMessage{
				Role:    llm.RoleUser,
				Content: renderDialectTranscript(history, catalog),
			})
			toolDefs = nil
			dialectScanner = newXMLDialectScanner()
		} else {
			messages = append(messages, m.messagesToLLM(history)...)
			dialectScanner = newXMLDialectScanner()
		}

		// Call LLM with streaming callback for real-time thinking and content.
		var lastEmit time.Time
		assistant := m.newAssistantMessage(sessionID, dialect)
		toolCallBuf := make(map[int]*llm.ToolCall)
		var toolCallOrder []int

		resp, err := m.llmClient.ChatWithStreamDetailed(ctx, messages, toolDefs,
			func(deltaContent string, deltaReasoning string) {
				m.mu.Lock()
				if deltaReasoning != "" {
					assistant.appendThinking(deltaReasoning)
					m.emitThinkingDelta(sessionID, deltaReasoning, turnNum)
				}
				if deltaContent != "" {
					if dialectScanner != nil {
						// Parse the stream for in-band tool calls.
						for _, ev := range dialectScanner.feed(deltaContent) {
							switch ev.kind {
							case "thinking":
								assistant.appendThinking(ev.text)
								m.emitThinkingDelta(sessionID, ev.text, turnNum)
							case "tool_start":
								tc := &llm.ToolCall{
									ID:   ev.toolID,
									Type: "function",
									Function: llm.ToolFunction{
										Name: ev.toolName,
									},
								}
								toolCallBuf[len(toolCallOrder)] = tc
								toolCallOrder = append(toolCallOrder, len(toolCallOrder))
								m.emitToolDelta(sessionID, len(toolCallOrder)-1, ev.toolName, "", turnNum)
							case "tool_end":
								if idx, ok := dialectToolIndex(toolCallBuf, ev.toolID); ok {
									tc := toolCallBuf[idx]
									tc.Function.Arguments = jsonStringMap(ev.args)
									m.emitToolDelta(sessionID, idx, ev.toolName, tc.Function.Arguments, turnNum)
								}
							case "text":
								assistant.appendText(ev.text)
								m.emitMessageDelta(sessionID, "text", ev.text, turnNum)
							}
						}
					} else {
						assistant.appendText(deltaContent)
						m.emitMessageDelta(sessionID, "text", deltaContent, turnNum)
					}
				}
				sess, ok := m.sessions[sessionID]
				if ok {
					sess.UpdatedAt = time.Now()
					if len(sess.Messages) > 0 && sess.Messages[len(sess.Messages)-1].Role == "assistant" {
						sess.Messages[len(sess.Messages)-1] = *assistant
					} else {
						sess.Messages = append(sess.Messages, *assistant)
					}
				}
				m.mu.Unlock()

				// Throttle full-session updates to 60ms for smooth UI; the
				// granular delta events above carry the live stream.
				if time.Since(lastEmit) > 60*time.Millisecond {
					lastEmit = time.Now()
					m.emitSessionUpdate(sessionID)
				}
			},
			func(delta llm.ToolCallDelta) {
				if isPureDialect {
					return // pure in-band mode has no native tool-call fragments
				}
				m.mu.Lock()
				tc, ok := toolCallBuf[delta.Index]
				if !ok {
					tc = &llm.ToolCall{
						ID:   delta.ID,
						Type: "function",
						Function: llm.ToolFunction{
							Name:      delta.Name,
							Arguments: delta.ArgFragment,
						},
					}
					toolCallBuf[delta.Index] = tc
					toolCallOrder = append(toolCallOrder, delta.Index)
				} else {
					if delta.ID != "" {
						tc.ID = delta.ID
					}
					if delta.Name != "" {
						tc.Function.Name += delta.Name
					}
					tc.Function.Arguments += delta.ArgFragment
				}
				m.emitToolDelta(sessionID, delta.Index, tc.Function.Name, tc.Function.Arguments, turnNum)
				m.mu.Unlock()
			},
		)
		if err != nil {
			m.finishTurn(sessionID, fmt.Sprintf("Error calling LLM: %v", err))
			return
		}

		if dialectScanner != nil {
			// Flush any trailing in-band tool calls at end of stream.
			for _, ev := range dialectScanner.flush() {
				switch ev.kind {
				case "tool_end":
					if idx, ok := dialectToolIndex(toolCallBuf, ev.toolID); ok {
						tc := toolCallBuf[idx]
						tc.Function.Arguments = jsonStringMap(ev.args)
						m.emitToolDelta(sessionID, idx, ev.toolName, tc.Function.Arguments, turnNum)
					}
				case "thinking":
					assistant.appendThinking(ev.text)
					m.emitThinkingDelta(sessionID, ev.text, turnNum)
				case "text":
					assistant.appendText(ev.text)
					m.emitMessageDelta(sessionID, "text", ev.text, turnNum)
				}
			}
		}

		// Finalize the assistant message from the accumulated stream.
		m.mu.Lock()
		for _, idx := range toolCallOrder {
			if tc, ok := toolCallBuf[idx]; ok {
				assistant.addToolCall(tc)
			}
		}
		if !assistant.isEmpty() {
			if len(sess.Messages) > 0 && sess.Messages[len(sess.Messages)-1].Role == "assistant" &&
				sess.Messages[len(sess.Messages)-1].ID == assistant.ID {
				sess.Messages[len(sess.Messages)-1] = *assistant
			} else {
				sess.Messages = append(sess.Messages, *assistant)
			}
		}
		sess.TokenUsage.PromptTokens += resp.TokenUsage.PromptTokens
		sess.TokenUsage.CompletionTokens += resp.TokenUsage.CompletionTokens
		sess.TokenUsage.CachedTokens += resp.TokenUsage.CachedTokens
		sess.TokenUsage.PromptCacheHitTokens += resp.TokenUsage.PromptCacheHitTokens
		sess.TokenUsage.PromptCacheMissTokens += resp.TokenUsage.PromptCacheMissTokens
		sess.TokenUsage.TotalTokens += resp.TokenUsage.TotalTokens
		sess.ContextTokens = resp.TokenUsage.PromptTokens
		if sess.TokenUsage.PromptTokens > 0 {
			totalCached := sess.TokenUsage.CachedTokens + sess.TokenUsage.PromptCacheHitTokens
			sess.CacheHitRate = float64(totalCached) / float64(sess.TokenUsage.PromptTokens)
		}
		// Sanitize assistant text to ensure no residual in-band markup or unclosed tags leak.
		cleaned := cleanInbandToolSyntax(assistant.Text())
		if cleaned != assistant.Text() {
			assistant.setText(cleaned)
			if len(sess.Messages) > 0 && sess.Messages[len(sess.Messages)-1].Role == "assistant" &&
				sess.Messages[len(sess.Messages)-1].ID == assistant.ID {
				sess.Messages[len(sess.Messages)-1] = *assistant
			}
		}
		m.mu.Unlock()
		m.emitMessageEnd(sessionID, assistant)

		toolCalls := assistant.ToolCallBlocks()

		// Fallback: If no native tool calls were returned, inspect the assistant text
		// for in-band tool calling patterns (e.g. Mimo <toolcall>, DeepSeek <tool_call>,
		// or Anthropic <invoke>). Strip the raw XML tags from chat and execute the tools!
		if len(toolCalls) == 0 {
			rawText := assistant.Text()
			inbandCalls, cleanedText := parseInbandToolCalls(rawText)
			if len(inbandCalls) > 0 {
				m.mu.Lock()
				assistant.setText(cleanedText)
				for _, tc := range inbandCalls {
					assistant.addToolCall(tc)
				}
				if len(sess.Messages) > 0 && sess.Messages[len(sess.Messages)-1].Role == "assistant" {
					sess.Messages[len(sess.Messages)-1] = *assistant
				}
				m.mu.Unlock()
				toolCalls = assistant.ToolCallBlocks()
				m.emitMessageEnd(sessionID, assistant)
				m.emitSessionUpdate(sessionID)
			}
		}

		// Approval gate: pause the whole batch if any mutating tool needs
		// approval (the old code gated only the first tool and dropped the rest).
		if len(toolCalls) > 0 && !sess.AutoApprove {
			needApproval := false
			for _, tc := range toolCalls {
				if isMutatingTool(tc.Name) {
					needApproval = true
					break
				}
			}
			if needApproval {
				m.mu.Lock()
				sess.State = StateAwaiting
				sess.PendingTools = toolCalls
				m.saveSessionsLocked()
				m.mu.Unlock()
				m.emitSessionUpdate(sessionID)
				return
			}
		}

		// Execute the batch under harness scheduler semantics (read-only
		// calls in parallel groups, mutating calls sequential), then loop.
		if len(toolCalls) > 0 {
			m.mu.Lock()
			sess.State = StateExecuting
			m.scheduleSave()
			m.mu.Unlock()
			m.emitSessionUpdate(sessionID)

			spent, fatal := m.executeScheduledBatch(ctx, sessionID, toolCalls, turnNum)
			budgetSpent += spent
			if fatal {
				// A tool failed fatally; surface it and stop rather than
				// feeding the error back into an infinite loop.
				m.finishTurn(sessionID, "One or more tool calls failed.")
				return
			}

			// Circuit breaker check: detect repetitive tool loops and prevent runaway token spend.
			m.mu.RLock()
			detector := m.loopDetectors[sessionID]
			m.mu.RUnlock()
			if detector != nil {
				if isLoop, loopTool, count := detector.HasLoop(); isLoop {
					warnMsg := FormatLoopWarning(loopTool, count)
					log.Printf("[agent:%s] %s", sessionID, warnMsg)
					m.finishTurn(sessionID, warnMsg)
					return
				}
			}
			// The `ask` tool paused the turn waiting for user input — yield.
			m.mu.RLock()
			paused := m.sessions[sessionID] != nil && m.sessions[sessionID].State == StateAwaitingInput
			m.mu.RUnlock()
			if paused {
				m.emitSessionUpdate(sessionID)
				return
			}
			continue // loop again for the next LLM call
		}

		// No tool calls → the turn is done.
		m.finishTurn(sessionID, "")
		return
	}

	m.finishTurn(sessionID, fmt.Sprintf("Agent loop exceeded %d reasoning steps; stopping.", maxReasoningSteps))
}

func (m *Manager) RespondApproval(ctx context.Context, sessionID string, approve bool, autoApproveAll bool) error {
	m.mu.Lock()
	sess, ok := m.sessions[sessionID]
	if !ok || len(sess.PendingTools) == 0 {
		m.mu.Unlock()
		return fmt.Errorf("no pending tool approval for session %s", sessionID)
	}

	pending := sess.PendingTools
	sess.PendingTools = nil

	if autoApproveAll {
		sess.AutoApprove = true
	}

	if !approve {
		for _, tc := range pending {
			sess.Messages = append(sess.Messages, AgentMessage{
				ID:   uuid.New().String(),
				Role: "tool",
				Content: []ContentBlock{
					{
						Type:       "tool_result",
						ToolCallID: tc.ToolCallID,
						Name:       tc.Name,
						Text:       fmt.Sprintf("Tool call %s rejected by user.", tc.Name),
						IsError:    true,
					},
				},
				Timestamp: time.Now(),
			})
		}
		sess.State = StateIdle
		m.scheduleSave()
		m.mu.Unlock()
		m.emitSessionUpdate(sessionID)
		return nil
	}

	sess.State = StateExecuting
	execCtx, cancel := context.WithCancel(context.Background())
	m.cancelFuncs[sessionID] = cancel
	m.mu.Unlock()
	m.emitSessionUpdate(sessionID)

	// Execute the whole previously-pending batch, then continue the loop.
	go func() {
		for _, tc := range pending {
			_, _ = m.executeToolCall(execCtx, sessionID, tc)
		}
		m.runAgentTurn(execCtx, sessionID)
	}()
	return nil
}

// executeToolCall runs one tool call and appends its result as a tool_result
// block linked to the original tool_call_id. Returns the model-facing text
// and an error when the tool itself fails.
func (m *Manager) executeToolCall(ctx context.Context, sessionID string, block ContentBlock) (string, error) {
	content, _, err := m.executeToolCallDetailed(ctx, sessionID, block)
	return content, err
}

// executeToolCallDetailed additionally returns the structured display payload
// (raw tool result map) the UI renders from the tool_end event.
func (m *Manager) executeToolCallDetailed(ctx context.Context, sessionID string, block ContentBlock, turn ...int) (string, map[string]any, error) {
	currentTurn := 1
	if len(turn) > 0 && turn[0] > 0 {
		currentTurn = turn[0]
	}

	// Role-gating safety check for read-only subagents (research, planning)
	var role RoleFilter
	m.mu.RLock()
	if s, ok := m.sessions[sessionID]; ok && s != nil {
		role = s.RoleFilter
	}
	m.mu.RUnlock()

	if (role == RoleResearch || role == RolePlanning) && isMutatingTool(block.Name) {
		m.emitToolStart(sessionID, block, currentTurn)
		err := fmt.Errorf("tool %q is blocked for %s agents (this role is read-only and cannot mutate workspace files or state)", block.Name, role)
		contentStr := fmt.Sprintf("<tool_use_error>%v</tool_use_error>", err)
		m.emitToolEnd(sessionID, block, contentStr, nil, true, currentTurn)
		_ = m.appendToolResult(sessionID, block, contentStr, true)
		return contentStr, nil, err
	}

	// Attach the session bridge so `todo` / `ask` tools can touch session state,
	// and the per-session read-state tracker so edit/write can enforce
	// read-before-edit.
	toolCtx := tools.WithFileState(
		tools.WithSessionBridge(ctx, m.sessionBridge(ctx, sessionID)),
		m.fileStateFor(sessionID),
	)

	// Auto-record file read state for write/edit operations on existing files so
	// models are never blocked by arbitrary "File has not been read yet" gate errors.
	if (block.Name == "write" || block.Name == "edit") && block.Arguments != nil {
		pathArg, _ := block.Arguments["path"].(string)
		if pathArg == "" {
			pathArg, _ = block.Arguments["target_file"].(string)
		}
		if pathArg == "" {
			pathArg, _ = block.Arguments["file"].(string)
		}
		if pathArg != "" {
			p := pathArg
			folder := m.getFolder(sessionID)
			if !filepath.IsAbs(p) && folder != "" {
				p = filepath.Join(folder, p)
			}
			p = filepath.Clean(p)
			if _, statErr := os.Stat(p); statErr == nil {
				m.fileStateFor(sessionID).MarkRead(p)
			}
		}
	}

	m.emitToolStart(sessionID, block, currentTurn)

	res, err := m.toolReg.Execute(toolCtx, block.Name, jsonStringMap(block.Arguments))
	var contentStr string
	var display map[string]any
	isErr := false
	if err != nil {
		contentStr = fmt.Sprintf("<tool_use_error>%v</tool_use_error>", err)
		isErr = true
	} else {
		normalized := normalizeToolResult(res)
		contentStr = truncateModelText(formatToolModelContent(block.Name, normalized))
		if dm, ok := normalized.(map[string]any); ok {
			display = dm
		}
	}

	m.emitToolEnd(sessionID, block, contentStr, display, isErr, currentTurn)
	if err := m.appendToolResult(sessionID, block, contentStr, isErr); err != nil {
		return contentStr, display, err
	}
	return contentStr, display, nil
}

// fileStateFor returns (lazily creating) the per-session read-state tracker
// used by the read/edit/write freshness gate.
func (m *Manager) fileStateFor(sessionID string) *tools.FileStateTracker {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fileStates == nil {
		m.fileStates = make(map[string]*tools.FileStateTracker)
	}
	fs, ok := m.fileStates[sessionID]
	if !ok {
		fs = tools.NewFileStateTracker()
		m.fileStates[sessionID] = fs
	}
	return fs
}

// appendToolResult stores a tool_result block for a tool call id and resets
// the session state so the loop can continue.
func (m *Manager) appendToolResult(sessionID string, block ContentBlock, contentStr string, isErr bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	sess, ok := m.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session %s not found", sessionID)
	}

	// Truncate and offload oversized tool output to disk to protect LLM context
	modelContent := m.truncateAndOffloadToolOutput(sessionID, block.ToolCallID, block.Name, contentStr)

	// Record execution signature in loop detector to detect stuck circular execution
	if detector, exists := m.loopDetectors[sessionID]; exists && detector != nil {
		argStr := ""
		if block.Arguments != nil {
			argStr = canonicalArgs(block.Arguments)
		}
		detector.Record(block.Name, argStr, modelContent)
	}

	sess.Messages = append(sess.Messages, AgentMessage{
		ID:   uuid.New().String(),
		Role: "tool",
		Content: []ContentBlock{
			{
				Type:       "tool_result",
				ToolCallID: block.ToolCallID,
				Name:       block.Name,
				Text:       modelContent,
				IsError:    isErr,
			},
		},
		Timestamp: time.Now(),
	})
	// The `ask` tool set the session to awaiting_input; keep that state so
	// the loop yields until RespondAsk resumes it.
	if sess.State != StateAwaitingInput {
		sess.State = StateThinking
	}
	sess.UpdatedAt = time.Now()
	m.scheduleSave()
	m.emitSessionUpdate(sessionID)
	return nil
}

// finishTurn sets the session back to idle, persists, emits the turn-end event,
// and (on error) appends an error message.
func (m *Manager) finishTurn(sessionID string, errMsg string) {
	m.mu.Lock()
	sess, ok := m.sessions[sessionID]
	if !ok || sess.State == StateIdle {
		m.mu.Unlock()
		return
	}
	delete(m.cancelFuncs, sessionID)
	delete(m.runningTurns, sessionID)
	if detector, ok := m.loopDetectors[sessionID]; ok && detector != nil {
		detector.Reset()
	}
	sess.State = StateIdle
	sess.UpdatedAt = time.Now()
	isCancel := errMsg != "" && strings.Contains(strings.ToLower(errMsg), "canceled")
	if errMsg != "" && !isCancel {
		if len(sess.Messages) > 0 && sess.Messages[len(sess.Messages)-1].Role == "assistant" && len(sess.Messages[len(sess.Messages)-1].Content) == 0 {
			sess.Messages[len(sess.Messages)-1].Content = []ContentBlock{
				{Type: "text", Text: errMsg},
			}
		} else {
			sess.Messages = append(sess.Messages, AgentMessage{
				ID:   uuid.New().String(),
				Role: "assistant",
				Content: []ContentBlock{
					{Type: "text", Text: errMsg},
				},
				Timestamp: time.Now(),
			})
		}
	} else if len(sess.Messages) > 0 && sess.Messages[len(sess.Messages)-1].Role == "assistant" && len(sess.Messages[len(sess.Messages)-1].Content) == 0 {
		sess.Messages = sess.Messages[:len(sess.Messages)-1]
	}
	// Defer LLM auto-titling until AFTER the first turn has finished
	// so the initial response doesn't compete with or queue behind an auto-title call.
	var autoTitlePrompt string
	if errMsg == "" && len(sess.Messages) <= 3 {
		for _, msg := range sess.Messages {
			if msg.Role == "user" && len(msg.Content) > 0 {
				autoTitlePrompt = msg.Content[0].Text
				break
			}
		}
	}
	parentID := ""
	if sess != nil {
		parentID = sess.ParentSessionID
		if parentID != "" {
			finalOutput := ""
			for i := len(sess.Messages) - 1; i >= 0; i-- {
				if sess.Messages[i].Role == "assistant" {
					for _, b := range sess.Messages[i].Content {
						if b.Type == "text" && strings.TrimSpace(b.Text) != "" {
							finalOutput = b.Text
							break
						}
					}
					if finalOutput != "" {
						break
					}
				}
			}
			m.subagentNotes[parentID] = append(m.subagentNotes[parentID], SubagentNotification{
				ID:        sess.ID,
				Name:      sess.Name,
				Role:      string(sess.RoleFilter),
				Output:    finalOutput,
				Completed: time.Now(),
			})
		}
	}
	m.saveSessionLocked(sessionID)
	m.mu.Unlock()
	if autoTitlePrompt != "" && m.llmClient != nil {
		go m.autoTitleSession(sessionID, autoTitlePrompt)
	}
	m.emitSessionUpdate(sessionID)
	if parentID != "" {
		m.emitSessionUpdate(parentID)
	}
	m.emitAgentEvent(events.AgentTurnEnd, sessionID, map[string]interface{}{
		"error": errMsg,
	})
	if errMsg != "" && !isCancel {
		m.emitMessageEnd(sessionID, &AgentMessage{
			ID:   uuid.New().String(),
			Role: "assistant",
			Content: []ContentBlock{
				{Type: "text", Text: errMsg},
			},
			Timestamp: time.Now(),
		})
	}
}

// formatSmartSessionTitle generates a fast, clean session title directly from the user prompt
// with 0 latency and 0 LLM calls, ensuring immediate UI feedback in the session list.
func formatSmartSessionTitle(text string) string {
	text = strings.TrimSpace(text)
	if idx := strings.IndexByte(text, '\n'); idx != -1 {
		text = text[:idx]
	}
	words := strings.Fields(text)
	var cleanWords []string
	for _, w := range words {
		if strings.HasPrefix(w, "/") || strings.HasPrefix(w, "@") {
			continue
		}
		w = strings.Trim(w, "*`#_~:;.,\"'()[]{}")
		if w != "" {
			cleanWords = append(cleanWords, w)
		}
		if len(cleanWords) >= 6 {
			break
		}
	}
	if len(cleanWords) == 0 {
		return "New Task"
	}
	title := strings.Join(cleanWords, " ")
	if len(title) > 50 {
		title = title[:50]
	}
	return title
}

// ---------------------------------------------------------------------------
// Auto session title — ChatGPT-style short naming from the first user message.
// ---------------------------------------------------------------------------
func (m *Manager) autoTitleSession(sessionID string, firstMessage string) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	prompt := "Give this coding task a very short title (max 6 words, no quotes, no punctuation at the end). Task: " + firstMessage
	resp, err := m.llmClient.Chat(ctx, []llm.LLMMessage{
		{Role: llm.RoleSystem, Content: "You generate concise chat session titles, like ChatGPT. Reply with only the title, max 6 words."},
		{Role: llm.RoleUser, Content: prompt},
	}, nil)
	if err != nil {
		log.Printf("[auto-title] LLM error for %s: %v", sessionID, err)
		return
	}
	if strings.TrimSpace(resp.Content) == "" {
		log.Printf("[auto-title] empty title response for %s", sessionID)
		return
	}
	title := strings.TrimSpace(resp.Content)
	title = strings.Trim(title, `"'`)
	if len(title) > 60 {
		title = title[:60]
	}

	m.mu.Lock()
	if sess, ok := m.sessions[sessionID]; ok {
		sess.Name = title
		sess.UpdatedAt = time.Now()
		m.saveSessionLocked(sessionID)
	}
	m.mu.Unlock()
	m.emitSessionUpdate(sessionID)
}

// ---------------------------------------------------------------------------
// SessionBridge — lets `todo` / `ask` tools interact with the live session.
// ---------------------------------------------------------------------------

// sessionBridge adapts the Manager to the tools package's SessionBridge.
type sessionBridge struct {
	m         *Manager
	sessionID string
	ctx       context.Context
}

func (b *sessionBridge) SessionID() string { return b.sessionID }

func (m *Manager) sessionBridge(ctx context.Context, sessionID string) tools.SessionBridge {
	return &sessionBridge{m: m, sessionID: sessionID, ctx: ctx}
}

// TerminalExec runs a command in the agent session's persistent shell (created
// lazily, one per agent session) so cwd, env, and shell state survive across
// commands — the biggest "real agent" win over spawning a shell per call.
func (b *sessionBridge) TerminalExec(command string, timeout time.Duration) (string, int, error) {
	ctx := b.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	return b.m.agentShellExecContext(ctx, b.sessionID, command, timeout)
}

// agentShellExecContext resolves (or lazily creates) the persistent shell for an
// agent session and runs a command in it, respecting context cancellation.
func (m *Manager) agentShellExecContext(ctx context.Context, sessionID, command string, timeout time.Duration) (string, int, error) {
	if m.termMgr == nil {
		return "", 1, fmt.Errorf("terminal manager unavailable")
	}
	m.mu.RLock()
	shellID := m.shells[sessionID]
	folder := ""
	if sess, ok := m.sessions[sessionID]; ok {
		folder = m.resolveSessionFolder(sess.Folder)
	}
	m.mu.RUnlock()
	if shellID == "" {
		shellName := "Agent Shell"
		if sess, ok := m.sessions[sessionID]; ok && sess.Name != "" {
			shellName = sess.Name + " Shell"
		}
		sh, err := m.termMgr.CreateShell(shellName, folder)
		if err != nil {
			return "", 1, fmt.Errorf("create shell: %w", err)
		}
		shellID = sh.ID
		m.mu.Lock()
		m.shells[sessionID] = shellID
		m.mu.Unlock()
	}
	out, code, err := m.termMgr.ExecContext(ctx, shellID, command, timeout)
	if err != nil {
		// The shell died or was restarted; drop the mapping so the next call
		// creates a fresh one.
		m.mu.Lock()
		delete(m.shells, sessionID)
		m.mu.Unlock()
	}
	return out, code, err
}

func (b *sessionBridge) GetTodos() []tools.TodoItem {
	b.m.mu.RLock()
	defer b.m.mu.RUnlock()
	sess, ok := b.m.sessions[b.sessionID]
	if !ok {
		return nil
	}
	out := make([]tools.TodoItem, 0, len(sess.Tasks))
	for _, t := range sess.Tasks {
		st := string(t.Status)
		if st == "" {
			if t.Completed {
				st = "completed"
			} else {
				st = "pending"
			}
		}
		out = append(out, tools.TodoItem{
			ID:         t.ID,
			Title:      t.Title,
			Status:     st,
			ActiveForm: t.ActiveForm,
		})
	}
	return out
}

func (b *sessionBridge) SetTodos(items []tools.TodoItem) {
	b.m.mu.Lock()
	defer b.m.mu.Unlock()
	sess, ok := b.m.sessions[b.sessionID]
	if !ok {
		return
	}
	if len(items) == 0 {
		sess.Tasks = []TaskItem{}
		sess.Progress.ActiveTodos = nil
		sess.Progress.CompletedSteps = nil
		sess.UpdatedAt = time.Now()
		b.m.saveSessionsLocked()
		b.m.emitSessionUpdate(b.sessionID)
		return
	}
	sess.Tasks = make([]TaskItem, 0, len(items))
	for _, it := range items {
		st := TaskStatus(it.Status)
		if st != TaskPending && st != TaskInProgress && st != TaskCompleted {
			if it.Status == "completed" {
				st = TaskCompleted
			} else if it.Status == "in_progress" {
				st = TaskInProgress
			} else {
				st = TaskPending
			}
		}
		sess.Tasks = append(sess.Tasks, TaskItem{
			ID:         it.ID,
			Title:      it.Title,
			Status:     st,
			ActiveForm: it.ActiveForm,
			Completed:  st == TaskCompleted,
		})
	}
	// Mirror the task list into compact progress state (todo status feeds the
	// "Current objective" section of the system prompt).
	var active, done []string
	for _, t := range sess.Tasks {
		if t.Completed {
			done = append(done, t.Title)
		} else {
			if t.ActiveForm != "" {
				active = append(active, t.ActiveForm)
			} else {
				active = append(active, t.Title)
			}
		}
	}
	if len(active) > 8 {
		active = active[:8]
	}
	if len(done) > 8 {
		done = done[len(done)-8:]
	}
	sess.Progress.ActiveTodos = active
	sess.Progress.CompletedSteps = done
	sess.UpdatedAt = time.Now()
	b.m.saveSessionsLocked()
	b.m.emitSessionUpdate(b.sessionID)
}

// Ask pauses the agent turn with structured questions. The turn loop notices
// the awaiting_input state and yields; RespondAsk resumes it with the answers.
func (b *sessionBridge) Ask(questions []tools.AskQuestion) error {
	b.m.mu.Lock()
	defer b.m.mu.Unlock()
	sess, ok := b.m.sessions[b.sessionID]
	if !ok {
		return fmt.Errorf("session not found")
	}
	sess.State = StateAwaitingInput
	sess.PendingQuestions = questions
	sess.UpdatedAt = time.Now()
	b.m.saveSessionsLocked()
	b.m.emitSessionUpdate(b.sessionID)
	b.m.emitAgentEvent(events.AgentAsk, b.sessionID, map[string]interface{}{
		"questions": questions,
	})
	return nil
}

func (b *sessionBridge) CreateSkill(name string, description string, body string, scope string, scripts map[string]string) (map[string]any, error) {
	if b.m.skillMgr == nil {
		return nil, fmt.Errorf("skills manager not initialized")
	}
	folder := b.m.getFolder(b.sessionID)
	req := skills.CreateSkillRequest{
		Name:        name,
		Description: description,
		Body:        body,
		Scope:       scope,
		Scripts:     scripts,
	}
	sk, err := b.m.skillMgr.CreateSkill(req, folder)
	if err != nil {
		return nil, err
	}
	b.m.skillMgr.Reload()
	return map[string]any{
		"name":        sk.Name,
		"description": sk.Description,
		"path":        sk.Path,
		"scope":       sk.Scope,
		"status":      "created_and_registered",
	}, nil
}

func (b *sessionBridge) LoadSkill(name string) (map[string]any, error) {
	if b.m.skillMgr == nil {
		return nil, fmt.Errorf("skills manager not initialized")
	}
	sk, ok := b.m.skillMgr.Get(name)
	if !ok {
		return nil, fmt.Errorf("skill %q not found", name)
	}
	return map[string]any{
		"name":        sk.Name,
		"description": sk.Description,
		"path":        sk.Path,
		"base_dir":    sk.BaseDir(),
		"body":        sk.Body,
		"scripts":     sk.Scripts,
	}, nil
}

func (b *sessionBridge) ListSkills() ([]map[string]any, error) {
	if b.m.skillMgr == nil {
		return nil, fmt.Errorf("skills manager not initialized")
	}
	all := b.m.skillMgr.List()
	out := make([]map[string]any, 0, len(all))
	for _, s := range all {
		out = append(out, map[string]any{
			"name":        s.Name,
			"description": s.Description,
			"path":        s.Path,
			"scope":       s.Scope,
		})
	}
	return out, nil
}

func (b *sessionBridge) CreatePlugin(payload tools.CreatePluginPayload) (map[string]any, error) {
	if b.m.pluginMgr == nil {
		return nil, fmt.Errorf("plugin manager not initialized")
	}
	folder := b.m.getFolder(b.sessionID)

	var toolDefs []plugins.PluginToolDef
	for _, t := range payload.Tools {
		name, _ := t["name"].(string)
		desc, _ := t["description"].(string)
		params, _ := t["parameters"].(map[string]interface{})
		ht, _ := t["handler_type"].(string)
		cmd, _ := t["command"].(string)
		script, _ := t["script"].(string)
		var timeout int
		if to, ok := t["timeout_seconds"].(float64); ok {
			timeout = int(to)
		}

		toolDefs = append(toolDefs, plugins.PluginToolDef{
			Name:           name,
			Description:    desc,
			Parameters:     params,
			HandlerType:    plugins.HandlerType(ht),
			Command:        cmd,
			Script:         script,
			TimeoutSeconds: timeout,
		})
	}

	var skillDefs []plugins.PluginSkillDef
	for _, s := range payload.Skills {
		sName, _ := s["name"].(string)
		sDesc, _ := s["description"].(string)
		sBody, _ := s["body"].(string)
		skillDefs = append(skillDefs, plugins.PluginSkillDef{
			Name:        sName,
			Description: sDesc,
			Body:        sBody,
		})
	}

	req := plugins.CreatePluginRequest{
		ID:           payload.ID,
		Name:         payload.Name,
		Description:  payload.Description,
		Version:      payload.Version,
		Author:       payload.Author,
		Scope:        payload.Scope,
		SystemPrompt: payload.SystemPrompt,
		Tools:        toolDefs,
		Skills:       skillDefs,
		Files:        payload.Files,
	}

	p, err := b.m.pluginMgr.CreatePlugin(req, folder)
	if err != nil {
		return nil, err
	}

	b.m.syncPlugins(b.sessionID)

	return map[string]any{
		"id":          p.ID,
		"name":        p.Name,
		"description": p.Description,
		"path":        p.Path,
		"tools_count": len(p.Tools),
		"status":      "created_and_registered",
	}, nil
}

func (b *sessionBridge) RegisterPlugin(manifest map[string]any) error {
	if b.m.pluginMgr == nil {
		return fmt.Errorf("plugin manager not initialized")
	}
	rawBytes, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	var p plugins.Plugin
	if err := json.Unmarshal(rawBytes, &p); err != nil {
		return fmt.Errorf("invalid plugin manifest: %w", err)
	}
	if err := b.m.pluginMgr.Register(&p); err != nil {
		return err
	}
	b.m.syncPlugins(b.sessionID)
	return nil
}

func (b *sessionBridge) ListPlugins() ([]map[string]any, error) {
	if b.m.pluginMgr == nil {
		return nil, fmt.Errorf("plugin manager not initialized")
	}
	list := b.m.pluginMgr.List()
	out := make([]map[string]any, 0, len(list))
	for _, p := range list {
		out = append(out, map[string]any{
			"id":          p.ID,
			"name":        p.Name,
			"description": p.Description,
			"version":     p.Version,
			"enabled":     p.Enabled,
			"source":      p.Source,
			"path":        p.Path,
			"tools_count": len(p.Tools),
		})
	}
	return out, nil
}

func (b *sessionBridge) TogglePlugin(id string, enabled bool) error {
	if b.m.pluginMgr == nil {
		return fmt.Errorf("plugin manager not initialized")
	}
	if err := b.m.pluginMgr.TogglePlugin(id, enabled); err != nil {
		return err
	}
	b.m.syncPlugins(b.sessionID)
	return nil
}

func (b *sessionBridge) LearnMemory(key string, content string, category string, scope string) error {
	if b.m.memoryMgr == nil {
		return fmt.Errorf("memory manager not initialized")
	}
	folder := b.m.getFolder(b.sessionID)
	if folder != "" {
		b.m.memoryMgr.SetWorkspace(folder)
	}
	return b.m.memoryMgr.Save(memory.Entry{
		Key:      key,
		Content:  content,
		Category: category,
		Scope:    scope,
	})
}

func (b *sessionBridge) RecallMemory(query string) ([]map[string]any, error) {
	if b.m.memoryMgr == nil {
		return nil, nil
	}
	folder := b.m.getFolder(b.sessionID)
	if folder != "" {
		b.m.memoryMgr.SetWorkspace(folder)
	}
	entries := b.m.memoryMgr.Search(query)
	out := make([]map[string]any, 0, len(entries))
	for _, e := range entries {
		out = append(out, map[string]any{
			"id":       e.ID,
			"key":      e.Key,
			"content":  e.Content,
			"category": e.Category,
			"scope":    e.Scope,
		})
	}
	return out, nil
}

func (b *sessionBridge) ListMemories() ([]map[string]any, error) {
	return b.RecallMemory("")
}

func (b *sessionBridge) GetWorkspaceFolder() string {
	return b.m.getFolder(b.sessionID)
}

func (b *sessionBridge) SpawnSubagent(task string, role string) (string, error) {
	if role == "" {
		role = "coding"
	}
	b.m.mu.Lock()
	parentSess, ok := b.m.sessions[b.sessionID]
	if !ok {
		b.m.mu.Unlock()
		return "", fmt.Errorf("parent session %s not found", b.sessionID)
	}
	folder := parentSess.Folder
	parentAutoApprove := parentSess.AutoApprove
	b.m.mu.Unlock()

	subSess, err := b.m.CreateSession(fmt.Sprintf("%s Sub-agent", strings.Title(role)), RoleFilter(role), folder)
	if err != nil {
		return "", err
	}

	b.m.mu.Lock()
	subSess.ParentSessionID = b.sessionID
	subSess.AutoApprove = parentAutoApprove
	if task != "" {
		subSess.State = StateThinking
	}
	if parentSess, ok := b.m.sessions[b.sessionID]; ok {
		parentSess.SubagentIDs = append(parentSess.SubagentIDs, subSess.ID)
	}
	b.m.saveSessionsLocked()
	b.m.mu.Unlock()

	b.m.emitSessionUpdate(b.sessionID)
	b.m.emitSessionUpdate(subSess.ID)

	if task != "" {
		go func() {
			_ = b.m.SendMessage(context.Background(), subSess.ID, task, nil)
		}()
	}

	return subSess.ID, nil
}

func (b *sessionBridge) WaitSubagent(ctx context.Context, subID string, timeout time.Duration) (map[string]any, error) {
	return b.m.WaitSubagent(ctx, subID, timeout)
}

func (b *sessionBridge) GetSubagents() []tools.SubagentSummary {
	b.m.mu.RLock()
	defer b.m.mu.RUnlock()
	parentSess, ok := b.m.sessions[b.sessionID]
	if !ok {
		return nil
	}
	var res []tools.SubagentSummary
	for _, subID := range parentSess.SubagentIDs {
		if sub, ok := b.m.sessions[subID]; ok {
			var lastText string
			for i := len(sub.Messages) - 1; i >= 0; i-- {
				if sub.Messages[i].Role == "assistant" {
					lastText = sub.Messages[i].Text()
					break
				}
			}
			res = append(res, tools.SubagentSummary{
				ID:       sub.ID,
				Name:     sub.Name,
				Role:     string(sub.RoleFilter),
				State:    string(sub.State),
				Folder:   sub.Folder,
				ParentID: sub.ParentSessionID,
				Output:   lastText,
			})
		}
	}
	return res
}

func (m *Manager) WaitSubagent(ctx context.Context, subID string, timeout time.Duration) (map[string]any, error) {
	deadline := time.Now().Add(timeout)
	for {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		m.mu.RLock()
		sub, ok := m.sessions[subID]
		if !ok {
			m.mu.RUnlock()
			return nil, fmt.Errorf("subagent session %s not found", subID)
		}
		state := sub.State
		isDone := state == StateIdle
		if state == StateAwaiting {
			var lastAssistantText string
			for i := len(sub.Messages) - 1; i >= 0; i-- {
				if sub.Messages[i].Role == "assistant" {
					lastAssistantText = sub.Messages[i].Text()
					break
				}
			}
			m.mu.RUnlock()
			return map[string]any{
				"subagent_id": subID,
				"status":      "awaiting_approval",
				"state":       string(state),
				"output":      lastAssistantText,
				"message":     "Subagent paused waiting for user approval in the sidebar. Please review and approve in the subagent tab.",
			}, nil
		}
		var lastAssistantText string
		var tokenUsage llm.TokenStats
		if isDone {
			tokenUsage = sub.TokenUsage
			for i := len(sub.Messages) - 1; i >= 0; i-- {
				if sub.Messages[i].Role == "assistant" {
					lastAssistantText = sub.Messages[i].Text()
					break
				}
			}
			m.mu.RUnlock()
			return map[string]any{
				"subagent_id": subID,
				"status":      "completed",
				"state":       string(state),
				"output":      lastAssistantText,
				"token_usage": tokenUsage,
			}, nil
		}
		m.mu.RUnlock()

		if timeout <= 0 || time.Now().After(deadline) {
			break
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(150 * time.Millisecond):
		}
	}

	m.mu.RLock()
	sub, ok := m.sessions[subID]
	if !ok {
		m.mu.RUnlock()
		return nil, fmt.Errorf("subagent session %s not found", subID)
	}
	state := sub.State
	var lastAssistantText string
	for i := len(sub.Messages) - 1; i >= 0; i-- {
		if sub.Messages[i].Role == "assistant" {
			lastAssistantText = sub.Messages[i].Text()
			break
		}
	}
	m.mu.RUnlock()

	return map[string]any{
		"subagent_id": subID,
		"status":      "running",
		"state":       string(state),
		"output":      lastAssistantText,
		"message":     "Subagent still running; poll again with get_subagent_output or check sidebar.",
	}, nil
}

func (m *Manager) GetSubagents(parentID string) []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	parent, ok := m.sessions[parentID]
	if !ok {
		return nil
	}
	var list []*Session
	for _, subID := range parent.SubagentIDs {
		if sub, ok := m.sessions[subID]; ok {
			list = append(list, sub)
		}
	}
	return list
}

func (m *Manager) syncPlugins(sessionID string) {
	if m.pluginMgr == nil || m.toolReg == nil {
		return
	}

	folder := m.getFolder(sessionID)
	activeTools := m.pluginMgr.ActiveTools()

	for _, at := range activeTools {
		pID := at.PluginID
		tDef := at.Tool
		m.toolReg.Register(tools.ToolSpec{
			Name:        tDef.Name,
			Description: tDef.Description,
			Parameters:  tDef.Parameters,
			Cost:        "medium",
			Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
				res, err := m.pluginMgr.ExecuteTool(ctx, pID, tDef.Name, args, folder)
				if err != nil {
					return nil, err
				}
				if !res.Success {
					errMsg := res.Error
					if errMsg == "" && res.Stderr != "" {
						errMsg = res.Stderr
					}
					if errMsg == "" && res.Stdout != "" {
						errMsg = res.Stdout
					}
					if errMsg == "" {
						errMsg = "command failed"
					}
					return nil, fmt.Errorf("%s (exit code %d)", errMsg, res.ExitCode)
				}
				if res.Stdout != "" {
					return res.Stdout, nil
				}
				return "Plugin tool executed successfully with no output", nil
			},
		})
	}

	if m.skillMgr != nil {
		for _, ps := range m.pluginMgr.ActiveSkills() {
			m.skillMgr.RegisterPluginSkills(ps.PluginID, []skills.Skill{
				{
					Name:        ps.Skill.Name,
					Description: ps.Skill.Description,
					Body:        ps.Skill.Body,
					Scope:       skills.ScopePlugin,
				},
			})
		}
	}
}

func (m *Manager) getFolder(sessionID string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if sess, ok := m.sessions[sessionID]; ok && sess.Folder != "" {
		return sess.Folder
	}
	return ""
}

// RespondAsk injects the user's answers to pending `ask` questions back into
// the conversation as a tool result and resumes the agent turn.
func (m *Manager) RespondAsk(sessionID string, answers map[string]any) error {
	m.mu.Lock()
	sess, ok := m.sessions[sessionID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("session %s not found", sessionID)
	}
	if sess.State != StateAwaitingInput || len(sess.PendingQuestions) == 0 {
		m.mu.Unlock()
		return fmt.Errorf("no pending questions for session %s", sessionID)
	}
	sess.PendingQuestions = nil
	b, _ := json.MarshalIndent(answers, "", "  ")

	var foundPendingResult bool
	var askToolCallID string

	// 1. Search backwards in sess.Messages for the pending ask tool result.
	for i := len(sess.Messages) - 1; i >= 0; i-- {
		msg := &sess.Messages[i]
		if msg.Role == "tool" {
			for j := range msg.Content {
				if msg.Content[j].Type == "tool_result" && msg.Content[j].Name == "ask" {
					msg.Content[j].Text = string(b)
					msg.Content[j].IsError = false
					askToolCallID = msg.Content[j].ToolCallID
					foundPendingResult = true
					break
				}
			}
			if foundPendingResult {
				break
			}
		}
	}

	// 2. If no pending ask tool result was found in sess.Messages,
	// find the assistant message with the "ask" tool call to get its real ID.
	if !foundPendingResult {
		for i := len(sess.Messages) - 1; i >= 0; i-- {
			msg := &sess.Messages[i]
			if msg.Role == "assistant" {
				for _, block := range msg.ToolCallBlocks() {
					if block.Name == "ask" {
						askToolCallID = block.ToolCallID
						break
					}
				}
				if askToolCallID != "" {
					break
				}
			}
		}
		if askToolCallID == "" {
			askToolCallID = "ask"
		}
		sess.Messages = append(sess.Messages, AgentMessage{
			ID:   uuid.New().String(),
			Role: "tool",
			Content: []ContentBlock{
				{Type: "tool_result", ToolCallID: askToolCallID, Name: "ask", Text: string(b), IsError: false},
			},
			Timestamp: time.Now(),
		})
	}

	// 3. Clean up any trailing empty assistant messages if present
	for len(sess.Messages) > 0 {
		last := sess.Messages[len(sess.Messages)-1]
		if last.Role == "assistant" && len(last.Content) == 0 {
			sess.Messages = sess.Messages[:len(sess.Messages)-1]
		} else {
			break
		}
	}

	sess.State = StateThinking
	sess.UpdatedAt = time.Now()

	execCtx, cancel := context.WithCancel(context.Background())
	m.cancelFuncs[sessionID] = cancel

	m.saveSessionLocked(sessionID)
	m.mu.Unlock()
	m.emitSessionUpdate(sessionID)
	go m.runAgentTurn(execCtx, sessionID)
	return nil
}

// windowTranscript bounds the transcript sent to the LLM so long sessions
// don't balloon the context window. The stored session (and the UI) keeps
// the full history; only the model request is windowed.
//
// It keeps the opening user message (the original task) plus the most recent
// `tail` messages, then advances the cut forward past orphaned tool results
// and incomplete tool-call groups so every tool_call/tool_result pair stays
// intact — providers reject broken pairs.
func windowTranscript(msgs []AgentMessage, tail int) []AgentMessage {
	if len(msgs) <= tail+1 {
		return msgs
	}
	cut := len(msgs) - tail
	if cut < 0 {
		cut = 0
	}
	i := cut
	for i < len(msgs) {
		switch msgs[i].Role {
		case "tool":
			// Orphaned tool result (its assistant tool_call was cut) — skip.
			i++
		case "assistant":
			if len(msgs[i].ToolCallBlocks()) == 0 {
				return withFirstUser(msgs, i)
			}
			// Assistant requested calls; include it and all its results,
			// which follow as "tool" role messages (one result per call).
			need := len(msgs[i].ToolCallBlocks())
			j := i + 1
			for j < len(msgs) && need > 0 {
				if msgs[j].Role == "tool" {
					need--
				}
				j++
			}
			return withFirstUser(msgs, i)
		default:
			return withFirstUser(msgs, i)
		}
	}
	// Every trailing message was an orphaned tool result; keep the last one
	// so the model still has context to respond to.
	return withFirstUser(msgs, len(msgs)-1)
}

// withFirstUser prepends the opening user message (the original task) so a
// truncated window doesn't lose the goal. Dedups when it's already included.
func withFirstUser(msgs []AgentMessage, i int) []AgentMessage {
	if i == 0 {
		return msgs
	}
	for j := 0; j < i; j++ {
		if msgs[j].Role == "user" {
			out := make([]AgentMessage, 0, len(msgs)-i+1)
			out = append(out, msgs[j])
			return append(out, msgs[i:]...)
		}
	}
	return msgs[i:]
}

// windowTranscriptWithSummary truncates the transcript sent to LLM for long sessions,
// but constructs an anti-amnesia context summary from the truncated messages so the LLM
// retains memory of earlier goals, decisions, files touched, and progress.
func windowTranscriptWithSummary(msgs []AgentMessage, tail int) ([]AgentMessage, string) {
	if len(msgs) <= tail+1 {
		return msgs, ""
	}
	windowed := windowTranscript(msgs, tail)
	if len(windowed) >= len(msgs) {
		return windowed, ""
	}
	summary := buildContextSummary(msgs, windowed)
	return windowed, summary
}

func buildContextSummary(allMsgs, windowed []AgentMessage) string {
	windowStartID := ""
	if len(windowed) > 1 {
		windowStartID = windowed[1].ID
	}

	var userPrompts []string
	filesTouched := make(map[string]bool)

	for _, msg := range allMsgs {
		if msg.ID == windowStartID {
			break
		}
		if msg.Role == "user" {
			for _, b := range msg.Content {
				if b.Type == "text" && strings.TrimSpace(b.Text) != "" {
					text := strings.TrimSpace(b.Text)
					if len(text) > 140 {
						text = text[:140] + "..."
					}
					userPrompts = append(userPrompts, text)
				}
			}
		} else if msg.Role == "assistant" {
			for _, b := range msg.Content {
				if b.Type == "tool_use" && b.Arguments != nil {
					for _, k := range []string{"path", "file", "target_file"} {
						if p, ok := b.Arguments[k].(string); ok && p != "" {
							filesTouched[p] = true
							break
						}
					}
				}
			}
		}
	}

	if len(userPrompts) == 0 && len(filesTouched) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("<CONTEXT_SUMMARY>\n")
	sb.WriteString("Compact summary of earlier conversation turns truncated to fit context:\n")
	if len(userPrompts) > 0 {
		sb.WriteString("### Earlier User Requests:\n")
		for i, p := range userPrompts {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, p))
		}
	}
	if len(filesTouched) > 0 {
		sb.WriteString("\n### Files Referenced in Earlier Turns:\n")
		count := 0
		for f := range filesTouched {
			sb.WriteString(fmt.Sprintf("- %s\n", f))
			count++
			if count >= 15 {
				sb.WriteString("- ... (additional files omitted)\n")
				break
			}
		}
	}
	sb.WriteString("</CONTEXT_SUMMARY>")
	return sb.String()
}

// truncateAndOffloadToolOutput bounds tool output fed to the model context.
// Large outputs are persisted to disk and replaced with a preview and file pointer.
func (m *Manager) truncateAndOffloadToolOutput(sessionID, toolCallID, toolName, content string) string {
	const maxChars = 2800
	const maxLines = 60
	lines := strings.Split(content, "\n")
	if len(content) <= maxChars && len(lines) <= maxLines {
		return content
	}

	dir := filepath.Join(filepath.Dir(m.storePath), "sessions", sessionID, "tool_outputs")
	if err := os.MkdirAll(dir, 0755); err == nil {
		safeName := strings.ReplaceAll(toolName, "/", "_")
		safeID := strings.ReplaceAll(toolCallID, "/", "_")
		if safeID == "" {
			safeID = uuid.New().String()[:8]
		}
		filePath := filepath.Join(dir, fmt.Sprintf("%s_%s.txt", safeID, safeName))
		_ = os.WriteFile(filePath, []byte(content), 0644)

		if len(lines) > maxLines {
			head := strings.Join(lines[:25], "\n")
			tail := strings.Join(lines[len(lines)-20:], "\n")
			omitted := len(lines) - 45
			return fmt.Sprintf("%s\n\n[... %d lines omitted; full output (%d bytes) saved to %s ...]\n\n%s", head, omitted, len(content), filePath, tail)
		}
		head := content[:1600]
		tail := content[len(content)-800:]
		omitted := len(content) - 2400
		return fmt.Sprintf("%s\n\n[... %d characters omitted; full output (%d bytes) saved to %s ...]\n\n%s", head, omitted, len(content), filePath, tail)
	}

	if len(lines) > maxLines {
		return strings.Join(lines[:25], "\n") + fmt.Sprintf("\n\n[... %d lines omitted ...]\n\n", len(lines)-45) + strings.Join(lines[len(lines)-20:], "\n")
	}
	return content[:maxChars] + fmt.Sprintf("\n\n[... %d characters omitted ...]", len(content)-maxChars)
}

func (m *Manager) drainSubagentNotifications(sessionID string) []SubagentNotification {
	m.mu.Lock()
	defer m.mu.Unlock()
	notes := m.subagentNotes[sessionID]
	if len(notes) > 0 {
		delete(m.subagentNotes, sessionID)
	}
	return notes
}

func formatSubagentNotifications(notes []SubagentNotification) string {
	if len(notes) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("<subagent-notification>")
	for _, n := range notes {
		sb.WriteString(fmt.Sprintf("\n- Subagent finished: name=%q, role=%q, id=%s", n.Name, n.Role, n.ID))
		if n.Output != "" {
			trimmed := strings.TrimSpace(n.Output)
			if len(trimmed) > 1000 {
				trimmed = trimmed[:1000] + "..."
			}
			sb.WriteString("\n  output summary:\n    " + strings.ReplaceAll(trimmed, "\n", "\n    "))
		}
	}
	sb.WriteString("\n</subagent-notification>")
	return sb.String()
}

func inferProvider(model string) string {
	m := strings.ToLower(model)
	switch {
	case strings.Contains(m, "claude") || strings.Contains(m, "anthropic"):
		return "anthropic"
	case strings.Contains(m, "gpt") || strings.Contains(m, "o1") || strings.Contains(m, "o3") || strings.Contains(m, "chatgpt"):
		return "openai"
	case strings.Contains(m, "gemini"):
		return "google"
	case strings.Contains(m, "deepseek"):
		return "deepseek"
	case strings.Contains(m, "qwen"):
		return "qwen"
	case strings.Contains(m, "mistral") || strings.Contains(m, "codestral"):
		return "mistral"
	case strings.Contains(m, "ollama"):
		return "ollama"
	case strings.Contains(m, "groq"):
		return "groq"
	default:
		return "default"
	}
}

// GetUsageSummary aggregates usage statistics across models, providers, and sessions.
func (m *Manager) GetUsageSummary() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	type modelAgg struct {
		model            string
		provider         string
		promptTokens     int
		completionTokens int
		cachedTokens     int
		totalTokens      int
		turns            int
		sessions         int
	}

	type providerAgg struct {
		provider         string
		promptTokens     int
		completionTokens int
		cachedTokens     int
		totalTokens      int
		models           map[string]bool
		sessions         int
	}

	var totalPrompt, totalComp, totalCached, totalTokens int
	var totalTurns int
	modelsMap := make(map[string]*modelAgg)
	providersMap := make(map[string]*providerAgg)
	sessionsList := make([]map[string]interface{}, 0, len(m.sessions))

	fallbackModel := ""
	if m.llmClient != nil {
		fallbackModel = m.llmClient.GetConfig().Model
	}
	if fallbackModel == "" {
		fallbackModel = "default-model"
	}

	for _, sess := range m.sessions {
		if sess == nil {
			continue
		}
		modelName := sess.Model
		if modelName == "" {
			modelName = fallbackModel
		}
		providerName := inferProvider(modelName)

		turnsCount := 0
		for _, msg := range sess.Messages {
			if msg.Role == "assistant" {
				turnsCount++
			}
		}

		pTokens := sess.TokenUsage.PromptTokens
		cTokens := sess.TokenUsage.CompletionTokens
		cached := sess.TokenUsage.CachedTokens + sess.TokenUsage.PromptCacheHitTokens
		tTokens := sess.TokenUsage.TotalTokens
		if tTokens == 0 && (pTokens > 0 || cTokens > 0) {
			tTokens = pTokens + cTokens
		}

		totalPrompt += pTokens
		totalComp += cTokens
		totalCached += cached
		totalTokens += tTokens
		totalTurns += turnsCount

		// Model aggregation
		ma, ok := modelsMap[modelName]
		if !ok {
			ma = &modelAgg{
				model:    modelName,
				provider: providerName,
			}
			modelsMap[modelName] = ma
		}
		ma.promptTokens += pTokens
		ma.completionTokens += cTokens
		ma.cachedTokens += cached
		ma.totalTokens += tTokens
		ma.turns += turnsCount
		ma.sessions++

		// Provider aggregation
		pa, ok := providersMap[providerName]
		if !ok {
			pa = &providerAgg{
				provider: providerName,
				models:   make(map[string]bool),
			}
			providersMap[providerName] = pa
		}
		pa.promptTokens += pTokens
		pa.completionTokens += cTokens
		pa.cachedTokens += cached
		pa.totalTokens += tTokens
		pa.models[modelName] = true
		pa.sessions++

		sessionsList = append(sessionsList, map[string]interface{}{
			"id":                sess.ID,
			"name":              sess.Name,
			"role":              string(sess.RoleFilter),
			"model":             modelName,
			"provider":          providerName,
			"prompt_tokens":     pTokens,
			"completion_tokens": cTokens,
			"cached_tokens":     cached,
			"total_tokens":      tTokens,
			"context_tokens":    sess.ContextTokens,
			"cache_hit_rate":    sess.CacheHitRate,
			"turns":             turnsCount,
			"created_at":        sess.CreatedAt.Format(time.RFC3339),
			"updated_at":        sess.UpdatedAt.Format(time.RFC3339),
		})
	}

	byModel := make([]map[string]interface{}, 0, len(modelsMap))
	for _, ma := range modelsMap {
		byModel = append(byModel, map[string]interface{}{
			"model":             ma.model,
			"provider":          ma.provider,
			"prompt_tokens":     ma.promptTokens,
			"completion_tokens": ma.completionTokens,
			"cached_tokens":     ma.cachedTokens,
			"total_tokens":      ma.totalTokens,
			"turns":             ma.turns,
			"sessions":          ma.sessions,
		})
	}

	byProvider := make([]map[string]interface{}, 0, len(providersMap))
	for _, pa := range providersMap {
		modelsList := make([]string, 0, len(pa.models))
		for mdl := range pa.models {
			modelsList = append(modelsList, mdl)
		}
		byProvider = append(byProvider, map[string]interface{}{
			"provider":          pa.provider,
			"prompt_tokens":     pa.promptTokens,
			"completion_tokens": pa.completionTokens,
			"cached_tokens":     pa.cachedTokens,
			"total_tokens":      pa.totalTokens,
			"models":            modelsList,
			"sessions":          pa.sessions,
		})
	}

	jevStats := map[string]interface{}{}
	if m.jevRouter != nil {
		jevStats = m.jevRouter.GetStats()
	}

	return map[string]interface{}{
		"totals": map[string]interface{}{
			"total_tokens":      totalTokens,
			"prompt_tokens":     totalPrompt,
			"completion_tokens": totalComp,
			"cached_tokens":     totalCached,
			"session_count":     len(m.sessions),
			"turn_count":        totalTurns,
		},
		"by_model":    byModel,
		"by_provider": byProvider,
		"by_session":  sessionsList,
		"jev":         jevStats,
	}
}

// messagesToLLM converts block-based session messages to the LLM payload.
// Tool calls are correlated by ID: each assistant message's tool_call blocks
// are sent as tool_calls, and each tool_result block is sent as a role:"tool"
// message with the matching tool_call_id.
// isReadOnlyTool reports whether a tool only reads state (safe to dedup
// identical calls within one batch without changing semantics).
var readOnlyTools = map[string]bool{
	"read":                 true,
	"read_multiple":        true,
	"read_directory_files": true,
	"search":               true,
	"glob":                 true,
	"find":                 true,
	"git_status":           true,
}

func isReadOnlyTool(name string) bool {
	return readOnlyTools[name]
}

// canonicalArgs renders tool args deterministically for dedup keys.
func canonicalArgs(args map[string]any) string {
	b, err := json.Marshal(args)
	if err != nil {
		return fmt.Sprintf("%v", args)
	}
	return string(b)
}

// toolCost returns the budget points for a tool (cheap=1, medium=3, high=10).
func (m *Manager) toolCost(name string) int {
	if spec, ok := m.toolReg.Lookup(name); ok {
		switch spec.Cost {
		case "medium":
			return 3
		case "high":
			return 10
		}
	}
	return 1
}

// firstLine returns the first line of text, trimmed and capped, or "" when
// empty. Used to track the session's current goal.
func firstLine(s string) string {
	if idx := strings.IndexByte(s, '\n'); idx > 0 {
		s = s[:idx]
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if len(s) > 160 {
		s = s[:157] + "..."
	}
	return s
}

func (m *Manager) messagesToLLM(msgs []AgentMessage) []llm.LLMMessage {
	var out []llm.LLMMessage

	// Track pending tool call IDs and their function names from assistant messages
	// so tool results can be correlated and validated against provider requirements.
	pendingToolCalls := make(map[string]string) // toolCallID -> functionName

	for _, msg := range msgs {
		switch msg.Role {
		case "assistant":
			var toolCalls []llm.ToolCall
			pendingToolCalls = make(map[string]string)
			for _, b := range msg.ToolCallBlocks() {
				toolCalls = append(toolCalls, llm.ToolCall{
					ID:   b.ToolCallID,
					Type: "function",
					Function: llm.ToolFunction{
						Name:      b.Name,
						Arguments: jsonStringMap(b.Arguments),
					},
				})
				pendingToolCalls[b.ToolCallID] = b.Name
			}
			text := strings.TrimSpace(msg.Text())
			if text == "" && len(toolCalls) == 0 {
				// Skip empty assistant messages completely; sending an assistant
				// message with empty text and no tool_calls violates LLM APIs.
				continue
			}
			if text == "" && len(toolCalls) > 0 {
				// Anthropic-style and some OpenAI-compatible providers require
				// content to be non-empty or a minimal space when tool_calls are present.
				text = " "
			}
			out = append(out, llm.LLMMessage{
				Role:      llm.RoleAssistant,
				Content:   text,
				ToolCalls: toolCalls,
			})
		case "tool":
			for _, b := range msg.ToolResultBlocks() {
				toolCallID := b.ToolCallID
				// If toolCallID doesn't match a pending tool call, check if there's
				// an unmatched pending tool call with the same function name.
				if _, ok := pendingToolCalls[toolCallID]; !ok && len(pendingToolCalls) > 0 {
					for pendingID, pendingName := range pendingToolCalls {
						if pendingName == b.Name {
							toolCallID = pendingID
							break
						}
					}
				}

				if _, ok := pendingToolCalls[toolCallID]; ok {
					delete(pendingToolCalls, toolCallID)
					out = append(out, llm.LLMMessage{
						Role:       llm.RoleTool,
						Content:    b.Text,
						ToolCallID: toolCallID,
						Name:       b.Name,
					})
				} else {
					// Orphan tool result (no preceding assistant tool call with matching ID).
					// Do not send as RoleTool because OpenAI/DeepSeek will reject with 400.
					// Send as user context instead so the information is preserved.
					out = append(out, llm.LLMMessage{
						Role:    llm.RoleUser,
						Content: fmt.Sprintf("[Observation from %s]:\n%s", b.Name, b.Text),
					})
				}
			}
		default:
			text := msg.Text()
			if text != "" {
				out = append(out, llm.LLMMessage{
					Role:    llm.Role(msg.Role),
					Content: text,
				})
			}
		}
	}
	return out
}

// newAssistantMessage creates the streaming assistant message for a turn,
// optionally in dialect (in-band text tool-calling) mode.
func (m *Manager) newAssistantMessage(sessionID string, dialect string) *AgentMessage {
	msg := &AgentMessage{
		ID:        uuid.New().String(),
		Role:      "assistant",
		Content:   make([]ContentBlock, 0),
		Timestamp: time.Now(),
	}
	m.mu.Lock()
	if sess, ok := m.sessions[sessionID]; ok {
		sess.Messages = append(sess.Messages, *msg)
	}
	m.mu.Unlock()
	m.emitAgentEvent(events.AgentMessageStart, sessionID, map[string]interface{}{
		"message_id": msg.ID,
	})
	return msg
}

func (msg *AgentMessage) appendText(delta string) {
	if len(msg.Content) > 0 && msg.Content[len(msg.Content)-1].Type == "text" {
		msg.Content[len(msg.Content)-1].Text += delta
		return
	}
	msg.Content = append(msg.Content, ContentBlock{Type: "text", Text: delta})
}

func (msg *AgentMessage) setText(newText string) {
	for i, b := range msg.Content {
		if b.Type == "text" {
			if newText == "" {
				msg.Content = append(msg.Content[:i], msg.Content[i+1:]...)
			} else {
				msg.Content[i].Text = newText
			}
			return
		}
	}
	if newText != "" {
		msg.Content = append(msg.Content, ContentBlock{Type: "text", Text: newText})
	}
}

func (msg *AgentMessage) appendThinking(delta string) {
	if len(msg.Content) > 0 && msg.Content[len(msg.Content)-1].Type == "thinking" {
		msg.Content[len(msg.Content)-1].Text += delta
		return
	}
	msg.Content = append(msg.Content, ContentBlock{Type: "thinking", Text: delta})
}

func (msg *AgentMessage) addToolCall(tc *llm.ToolCall) {
	var args map[string]any
	if tc.Function.Arguments != "" {
		_ = json.Unmarshal([]byte(tc.Function.Arguments), &args)
	}
	msg.Content = append(msg.Content, ContentBlock{
		Type:       "tool_call",
		ToolCallID: tc.ID,
		Name:       tc.Function.Name,
		Arguments:  args,
	})
}

func (msg *AgentMessage) isEmpty() bool {
	for _, b := range msg.Content {
		switch b.Type {
		case "text":
			if b.Text != "" {
				return false
			}
		case "thinking":
			if b.Text != "" {
				return false
			}
		case "tool_call":
			if b.Name != "" {
				return false
			}
		}
	}
	return true
}

func jsonStringMap(v map[string]any) string {
	if len(v) == 0 {
		return ""
	}
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
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
			if completed {
				sess.Tasks[i].Status = TaskCompleted
			} else {
				sess.Tasks[i].Status = TaskPending
			}
			break
		}
	}
	var active, done []string
	for _, t := range sess.Tasks {
		if t.Completed {
			done = append(done, t.Title)
		} else {
			if t.ActiveForm != "" {
				active = append(active, t.ActiveForm)
			} else {
				active = append(active, t.Title)
			}
		}
	}
	if len(active) > 8 {
		active = active[:8]
	}
	if len(done) > 8 {
		done = done[len(done)-8:]
	}
	sess.Progress.ActiveTodos = active
	sess.Progress.CompletedSteps = done
	sess.UpdatedAt = time.Now()
	m.saveSessionsLocked()
	m.emitSessionUpdate(sessionID)
}

func (m *Manager) ClearTasks(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	sess, ok := m.sessions[sessionID]
	if !ok {
		return
	}
	sess.Tasks = []TaskItem{}
	sess.Progress.ActiveTodos = nil
	sess.Progress.CompletedSteps = nil
	sess.UpdatedAt = time.Now()
	m.saveSessionsLocked()
	m.emitSessionUpdate(sessionID)
}

func (m *Manager) emitSessionUpdate(sessionID string) {
	if m.bus != nil {
		m.bus.Publish(events.Event{
			Type: "agent:updated",
			Data: map[string]interface{}{"session_id": sessionID},
		})
	}
}

// emitAgentEvent publishes a granular agent event to the bus (port of
// flattened to Wails event names).
func (m *Manager) emitAgentEvent(evType events.EventType, sessionID string, data map[string]interface{}) {
	if m.bus == nil {
		return
	}
	if data == nil {
		data = map[string]interface{}{}
	}
	data["session_id"] = sessionID
	m.bus.Publish(events.Event{Type: evType, Data: data})
}

func (m *Manager) emitMessageStart(sessionID string, msg *AgentMessage) {
	m.emitAgentEvent(events.AgentMessageStart, sessionID, map[string]interface{}{
		"message_id": msg.ID,
		"role":       msg.Role,
	})
}

func (m *Manager) emitMessageDelta(sessionID string, kind string, delta string, turn int) {
	m.emitAgentEvent(events.AgentMessageDelta, sessionID, map[string]interface{}{
		"kind":  kind,
		"delta": delta,
		"turn":  turn,
	})
}

func (m *Manager) emitMessageEnd(sessionID string, msg *AgentMessage) {
	m.emitAgentEvent(events.AgentMessageEnd, sessionID, map[string]interface{}{
		"message_id": msg.ID,
		"role":       msg.Role,
	})
}

func (m *Manager) emitThinkingDelta(sessionID string, delta string, turn int) {
	m.emitAgentEvent(events.AgentThinkingDelta, sessionID, map[string]interface{}{
		"delta": delta,
		"turn":  turn,
	})
}

func (m *Manager) emitToolDelta(sessionID string, index int, name string, args string, turn int) {
	m.emitAgentEvent(events.AgentToolDelta, sessionID, map[string]interface{}{
		"index": index,
		"name":  name,
		"args":  args,
		"turn":  turn,
	})
}

func (m *Manager) emitToolStart(sessionID string, block ContentBlock, turn int) {
	data := map[string]interface{}{
		"tool_call_id": block.ToolCallID,
		"name":         block.Name,
		"args":         block.Arguments,
		"turn":         turn,
	}
	m.emitAgentEvent(events.AgentToolStart, sessionID, data)
}

func (m *Manager) emitToolEnd(sessionID string, block ContentBlock, result string, display map[string]any, isErr bool, turn int) {
	data := map[string]interface{}{
		"tool_call_id": block.ToolCallID,
		"name":         block.Name,
		"args":         block.Arguments,
		"result":       result,
		"is_error":     isErr,
		"turn":         turn,
	}
	if display != nil {
		data["display"] = display
	}
	m.emitAgentEvent(events.AgentToolEnd, sessionID, data)
}

// StopTurn cancels the currently running agent turn for a session.
func (m *Manager) StopTurn(sessionID string) {
	m.mu.Lock()
	if cancel, ok := m.cancelFuncs[sessionID]; ok {
		cancel()
		delete(m.cancelFuncs, sessionID)
	}
	shellID := m.shells[sessionID]
	m.mu.Unlock()

	// If there's an active shell running a command, interrupt it immediately (send Ctrl-C)
	if shellID != "" && m.termMgr != nil {
		_, _ = m.termMgr.Write(shellID, []byte("\x03\n"))
	}

	m.finishTurn(sessionID, "context canceled")
}

func isMutatingTool(name string) bool {
	switch name {
	case "write", "write_file", "create_file", "edit", "edit_file", "replace_file_content", "bash", "run_shell", "exec", "run_command", "python_eval", "python":
		return true
	}
	return false
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
You help with coding, planning, research, and general development tasks like Claude Code, Codex, and OpenCode.

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

## Tool use & Extensibility
- Canonical Tool Parameter Schemas (MANDATORY: always use these exact argument keys):
  - read: {"path": "file/path.ext"}
  - read_multiple: {"paths": ["file1.ext", "file2.ext"]} (or use python_eval/bash loops for multi-file aggregation)
  - write: {"path": "new/file.ext", "content": "..."} (brand-new files ONLY)
  - edit: {"path": "existing/file.ext", "old_string": "...", "new_string": "..."} (surgical patch in existing files)
  - bash: {"command": "..."}
  - python_eval: {"code": "..."}
  - ask: {"questions": [{"id": "...", "question": "...", "options"?: ["..."]}]}
  - search: {"pattern": "...", "path"?: "..."}
  - find: {"path": "glob-pattern", "cwd"?: "..."}
  - git_status: {}
  - update_tasks: {"tasks": [{"id": "1", "title": "...", "status": "pending|in_progress|completed", "active_form"?: "..."}]}
  - invoke_subagent: {"task": "...", "role"?: "coding|research|planning", "block"?: false, "timeout"?: 60}
  - get_subagent_output: {"subagent_id": "...", "block"?: false, "timeout"?: 30}
  - list_subagents: {}
- CRITICAL: On any tool_use_error, NEVER repeat the identical call. Inspect the error and adapt your arguments or approach.
- Task Planning & Progress Tracking (update_tasks):
  - For ANY multi-step task, feature implementation, refactoring, or multi-phase prompt: ALWAYS call update_tasks at the start to declare your plan as sequential tasks.
  - Automatic Goal Decomposition: Whenever a goal or plan is set (or displayed under "Goal: ..."), you MUST immediately decompose it into sequential, atomic tasks by calling update_tasks as your first action before modifying code.
  - Keep tasks updated as you work: mark tasks 'in_progress' with active_form (e.g. 'Writing StatusBadge.tsx...', 'Running npm test...'), and 'completed' once verified.
  - Dynamic Task Replacement (ZCode semantics): Calling update_tasks with a new list of tasks completely replaces the active checklist. When all prior tasks are completed and the user provides follow-up instructions, changes requirements, or gives a new objective, IMMEDIATELY call update_tasks with a fresh task breakdown for the new objective (or pass tasks: [] to reset). Never let stale completed tasks from an old objective linger into a new turn.
  - This powers the live floating collapsible checklist and progress tracker above the chat in the user interface.
- Sub-agents & Delegation (invoke_subagent):
  - Delegate isolated subtasks, independent code reviews, deep research, or tests to child sub-agents using invoke_subagent.
  - When you want to wait for and incorporate the sub-agent's output immediately (such as an independent code review), pass block: true.
  - For background exploration, pass block: false. You can query status anytime with get_subagent_output or list_subagents.
  - Spawned sub-agents appear live in the UI under the Subagents tab and sidebar with dedicated minimal chat views.
- Code Editing (CRITICAL - DO NOT REWRITE WHOLE FILES):
  - NEVER rewrite an entire file or dump large walls of unchanged code. You are a precise software engineer.
  - For existing files: ALWAYS use edit with the smallest unique snippet of old_string to replace with new_string (surgical patch).
  - Use write ONLY when creating brand-new files from scratch that do not exist yet.
  - Read the file before editing it. old_string must match the file exactly (including indentation) and be unique.
  - After a successful edit or write, file state is tracked in context — do not re-read the file just to verify it.
  - Do NOT echo or paste file contents or code in your conversational chat response when calling write or edit tools. Execute the tool call directly without echoing the code in conversational text.
- Python REPL (python_eval / python):
  - Python 3 is installed and available. Use python_eval to execute code, evaluate expressions, perform calculations, inspect complex data structures, run regexes, or verify logic in REPL mode (trailing expressions print automatically).
- Reusable Skills:
  - You have an active catalog of skills under <available_skills>. Call the skill or load_skill tool with the exact name whenever a user task matches a skill domain (e.g. impeccable for frontend design) to load its full playbook before acting.
  - You can create new reusable skills using create_skill to persist domain knowledge for future turns.
- Plugin Harness Extensibility:
  - You can inspect active plugins with list_plugins, toggle them with toggle_plugin, and create/extend plugins using create_plugin.
  - Plugins contribute custom command and script tools, system prompt sections, and skills.
- Long-term Memory:
  - You can recall stored memories, user preferences, and architecture conventions with recall_memory.
  - You can persist new decisions, style rules, or project facts using learn_memory so they persist across sessions.
- Long-running commands:
  - For servers, watchers, builds, and test suites, call bash with run_in_background: true — you get a task_id
    immediately and keep working. Read output with task_output (block: true to wait), stop with task_stop,
    list with task_list. Finished tasks are announced to you automatically via <task-notification>.
- Browser (in-app browser):
  - You drive the browser embedded in the app with the browser_* tools (browser_navigate, browser_snapshot,
    browser_click, browser_type, browser_key, browser_scroll, browser_screenshot). The user sees the page live
    in the Browser viewer panel.
  - Snapshot first: navigate and every action return a page outline with element refs; act on refs, never
    guesses. Re-snapshot after actions that change the page.
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

var (
	gitSnapMu    sync.Mutex
	gitSnapCache = make(map[string]gitSnapItem)
)

type gitSnapItem struct {
	cachedAt time.Time
	output   string
}

func gitSnapshot(folder string) (string, error) {
	gitSnapMu.Lock()
	if item, ok := gitSnapCache[folder]; ok && time.Since(item.cachedAt) < 10*time.Second {
		out := item.output
		gitSnapMu.Unlock()
		return out, nil
	}
	gitSnapMu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 800*time.Millisecond)
	defer cancel()

	type cmdRes struct {
		out []byte
		err error
	}
	branchCh := make(chan cmdRes, 1)
	statusCh := make(chan cmdRes, 1)

	go func() {
		branchCmd := exec.CommandContext(ctx, "git", "branch", "--show-current")
		branchCmd.Dir = folder
		out, err := branchCmd.Output()
		branchCh <- cmdRes{out: out, err: err}
	}()

	go func() {
		statusCmd := exec.CommandContext(ctx, "git", "status", "--porcelain")
		statusCmd.Dir = folder
		out, err := statusCmd.Output()
		statusCh <- cmdRes{out: out, err: err}
	}()

	var branchRes, statusRes cmdRes
	for i := 0; i < 2; i++ {
		select {
		case r := <-branchCh:
			branchRes = r
		case r := <-statusCh:
			statusRes = r
		case <-ctx.Done():
			break
		}
	}

	if statusRes.err != nil && branchRes.err != nil {
		return "", nil
	}

	lines := strings.Split(strings.TrimSpace(string(statusRes.out)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		lines = nil
	}
	if len(lines) > 60 {
		lines = lines[:60]
	}

	var sb strings.Builder
	if b := strings.TrimSpace(string(branchRes.out)); b != "" {
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

	res := sb.String()
	gitSnapMu.Lock()
	gitSnapCache[folder] = gitSnapItem{
		cachedAt: time.Now(),
		output:   res,
	}
	gitSnapMu.Unlock()

	return res, nil
}

// buildSkillsContext appends loaded skills catalog to the system prompt.
// Uses compact catalog format (like dsh-fork) so the prompt remains small and token-efficient.
func (m *Manager) buildSkillsContext() string {
	if m.skillMgr == nil {
		return ""
	}
	return m.skillMgr.CatalogMarkdown()
}

// maxMentionFileBytes caps inlined file content per @-mention (32 KB).
const maxMentionFileBytes = 32 * 1024

// buildMentionContext renders mentioned paths as context: files inline their
// content (capped), folders list their entries.
func buildMentionContext(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			sb.WriteString(fmt.Sprintf("@%s — not found\n", p))
			continue
		}
		if info.IsDir() {
			entries, err := os.ReadDir(p)
			if err != nil {
				sb.WriteString(fmt.Sprintf("@%s — unreadable: %v\n", p, err))
				continue
			}
			sb.WriteString(fmt.Sprintf("@%s (folder):\n", p))
			for i, e := range entries {
				if i >= 100 {
					sb.WriteString("… (truncated)\n")
					break
				}
				kind := "file"
				if e.IsDir() {
					kind = "dir"
				}
				sb.WriteString(fmt.Sprintf("  %s [%s]\n", e.Name(), kind))
			}
			continue
		}
		data, err := os.ReadFile(p)
		if err != nil {
			sb.WriteString(fmt.Sprintf("@%s — unreadable: %v\n", p, err))
			continue
		}
		content := data
		if len(content) > maxMentionFileBytes {
			content = content[:maxMentionFileBytes]
		}
		sb.WriteString(fmt.Sprintf("@%s:\n%s\n", p, content))
	}
	return "Referenced context:\n" + sb.String()
}

// slashMention is one /kind:name token extracted from a prompt.
type slashMention struct {
	Kind string // "skill" | "mcp"
	Name string // skill name, server name, or server:tool
	Args string // trailing free text after the token (skills only)
}

// extractSlashMentions finds slash commands (/goal, /plan, /plan-review, /security, /vibe, /model, /clear, /skill)
// and /skill:<name> /mcp:<server> tokens in the prompt.
func extractSlashMentions(text string) []slashMention {
	var out []slashMention
	trimmed := strings.TrimSpace(text)

	// Check leading slash command (e.g. /goal, /plan, /plan-review, /security, /vibe, /model, /clear, /skill)
	if strings.HasPrefix(trimmed, "/") {
		firstLine := trimmed
		if idx := strings.IndexByte(firstLine, '\n'); idx != -1 {
			firstLine = firstLine[:idx]
		}
		parts := strings.Fields(firstLine)
		if len(parts) > 0 {
			cmd := strings.ToLower(parts[0])
			rest := strings.TrimSpace(trimmed[len(parts[0]):])

			switch {
			case cmd == "/goal":
				out = append(out, slashMention{Kind: "goal", Name: rest})
			case cmd == "/plan":
				out = append(out, slashMention{Kind: "plan", Name: rest})
			case cmd == "/plan-review":
				out = append(out, slashMention{Kind: "plan-review", Name: rest})
			case cmd == "/security":
				out = append(out, slashMention{Kind: "security", Name: rest})
			case cmd == "/vibe":
				out = append(out, slashMention{Kind: "vibe", Name: rest})
			case cmd == "/model":
				out = append(out, slashMention{Kind: "model", Name: rest})
			case cmd == "/resume":
				out = append(out, slashMention{Kind: "resume", Name: rest})
			case cmd == "/settings":
				out = append(out, slashMention{Kind: "settings", Name: rest})
			case cmd == "/setup":
				out = append(out, slashMention{Kind: "setup", Name: rest})
			case cmd == "/clear":
				out = append(out, slashMention{Kind: "clear", Name: rest})
			case cmd == "/skill" && len(parts) > 1:
				skillName := parts[1]
				skillArgs := strings.TrimSpace(strings.TrimPrefix(rest, skillName))
				out = append(out, slashMention{Kind: "skill", Name: skillName, Args: skillArgs})
			case cmd == "/skill" && len(parts) == 1:
				out = append(out, slashMention{Kind: "skill", Name: ""})
			case strings.HasPrefix(cmd, "/skill:"):
				skillName := strings.TrimPrefix(parts[0], "/skill:")
				out = append(out, slashMention{Kind: "skill", Name: skillName, Args: rest})
			}
		}
	}

	// Also extract inline /skill:name or /mcp:server tokens throughout the text
	re := regexp.MustCompile(`(?:^|\s)/(skill|mcp):([^\s]+)`)
	matches := re.FindAllStringSubmatch(text, -1)
	for _, m := range matches {
		alreadyPresent := false
		for _, ex := range out {
			if ex.Kind == m[1] && ex.Name == m[2] {
				alreadyPresent = true
				break
			}
		}
		if !alreadyPresent {
			out = append(out, slashMention{Kind: m[1], Name: m[2]})
		}
	}
	return out
}

// buildMCPDirective renders an instruction for an /mcp mention. For a bare
// server it lists the server's tools; for server:tool it directs the model to
// that tool with its input schema summary.
func buildMCPDirective(mcpMgr *mcp.Manager, mention slashMention) string {
	server := mention.Name
	toolName := ""
	if srv, tool, ok := strings.Cut(server, ":"); ok {
		server, toolName = srv, tool
	}
	var toolList []string
	for _, t := range mcpMgr.ListTools() {
		if t.ServerName != server {
			continue
		}
		if toolName != "" && t.Name != toolName {
			continue
		}
		desc := t.Description
		if len(desc) > 120 {
			desc = desc[:120]
		}
		toolList = append(toolList, fmt.Sprintf("- %s:%s — %s", t.ServerName, t.Name, desc))
	}
	if len(toolList) == 0 {
		return fmt.Sprintf("The user mentioned MCP %q but no matching tools are connected. Say so if the request depends on it.", server)
	}
	kind := fmt.Sprintf("server %q", server)
	if toolName != "" {
		kind = fmt.Sprintf("tool %q on server %q", toolName, server)
	}
	return fmt.Sprintf("The user mentioned MCP %s. Use its tools for this request:\n%s",
		kind, strings.Join(toolList, "\n"))
}
