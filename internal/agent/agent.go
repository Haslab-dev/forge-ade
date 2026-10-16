package agent

import (
	"bytes"
	"context"
	"encoding/base64"
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

	"github.com/Haslab-dev/RakitKode/events"
	"github.com/Haslab-dev/RakitKode/jev"
	"github.com/Haslab-dev/RakitKode/llm"
	"github.com/Haslab-dev/RakitKode/mcp"
	"github.com/Haslab-dev/RakitKode/memory"
	"github.com/Haslab-dev/RakitKode/sandbox"
	"github.com/Haslab-dev/RakitKode/skills"
	"github.com/google/uuid"
	"github.com/hasdev/forge-ade/internal/plugins"
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
	AllowBash        bool                `json:"allow_bash,omitempty"`
	ReadOnly         bool                `json:"read_only,omitempty"`
	Tools            []string            `json:"tools,omitempty"`
	MCPServers       []string            `json:"mcp_servers,omitempty"`
	Skills           []string            `json:"skills,omitempty"`
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
	mu              sync.RWMutex
	sessions        map[string]*Session
	definitions     map[string]AgentDefinition
	llmClient       *llm.LLMClient
	toolReg         *tools.Registry
	skillMgr        *skills.Manager
	pluginMgr       *plugins.Manager
	memoryMgr       *memory.Manager
	mcpMgr          *mcp.Manager
	termMgr         *terminal.Manager
	jevRouter       *jev.Router
	shells          map[string]string                  // agent session id → persistent shell id
	fileStates      map[string]*tools.FileStateTracker // session id → read-state tracker
	bus             *events.Bus
	cancelFuncs     map[string]context.CancelFunc
	stepCancelFuncs map[string]context.CancelFunc // session id -> active step cancel func for mid-turn steering preemption
	runningTurns    map[string]bool
	loopDetectors   map[string]*LoopDetector
	steerChannels   map[string]chan string // session id -> channel for mid-turn user steering prompts
	subagentNotesMu sync.Mutex
	subagentNotes   map[string][]SubagentNotification // parent session id → queued subagent notifications
	schedulesMu     sync.Mutex
	schedules       map[string]*AgentSchedule
	storePath       string

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

type AgentSchedule struct {
	ID        string        `json:"id"`
	SessionID string        `json:"session_id"`
	Prompt    string        `json:"prompt"`
	TaskID    string        `json:"task_id,omitempty"`
	Duration  time.Duration `json:"duration,omitempty"`
	CronExpr  string        `json:"cron_expr,omitempty"`
	CreatedAt time.Time     `json:"created_at"`
	NextRun   time.Time     `json:"next_run"`
	Active    bool          `json:"active"`
	cancel    context.CancelFunc
}

func (s *AgentSchedule) ToSnapshot() map[string]any {
	return map[string]any{
		"id":               s.ID,
		"session_id":       s.SessionID,
		"prompt":           s.Prompt,
		"task_id":          s.TaskID,
		"duration_seconds": int(s.Duration.Seconds()),
		"cron_expression":  s.CronExpr,
		"created_at":       s.CreatedAt.Format(time.RFC3339),
		"next_run":         s.NextRun.Format(time.RFC3339),
		"active":           s.Active,
	}
}

func NewManager(llmClient *llm.LLMClient, toolReg *tools.Registry, skillMgr *skills.Manager, pluginMgr *plugins.Manager, mcpMgr *mcp.Manager, termMgr *terminal.Manager, bus *events.Bus, dataDir string) *Manager {
	storePath := filepath.Join(dataDir, "agent_sessions.json")
	defaultMemory := memory.New(dataDir)
	defaultMemory.SetGlobalDir(dataDir)
	m := &Manager{
		sessions:        make(map[string]*Session),
		definitions:     make(map[string]AgentDefinition),
		llmClient:       llmClient,
		toolReg:         toolReg,
		skillMgr:        skillMgr,
		pluginMgr:       pluginMgr,
		memoryMgr:       defaultMemory,
		mcpMgr:          mcpMgr,
		termMgr:         termMgr,
		jevRouter:       jev.NewRouter("", dataDir),
		shells:          make(map[string]string),
		bus:             bus,
		cancelFuncs:     make(map[string]context.CancelFunc),
		stepCancelFuncs: make(map[string]context.CancelFunc),
		runningTurns:    make(map[string]bool),
		loopDetectors:   make(map[string]*LoopDetector),
		steerChannels:   make(map[string]chan string),
		subagentNotes:   make(map[string][]SubagentNotification),
		schedules:       make(map[string]*AgentSchedule),
		storePath:       storePath,
	}
	m.loadSessions()
	m.loadDefinitions()
	m.syncPlugins("")
	m.syncMCPTools(context.Background())

	// Stream background-task lifecycle (bash run_in_background) to the UI and
	// queue completion notifications for the owning session's next model step.
	tools.SetTaskUpdateHandler(func(task *tools.BackgroundTask) {
		data := map[string]interface{}{
			"id":          task.ID,
			"task_id":     task.ID,
			"command":     task.Command,
			"cwd":         task.Cwd,
			"status":      string(task.Status),
			"session_id":  task.SessionID,
			"started_at":  task.StartedAt.Format(time.RFC3339),
			"output_tail": task.OutputTail,
			"output_file": task.OutputFile,
		}
		if task.Status != tools.TaskRunning {
			data["exit_code"] = task.ExitCode
			if task.Finished != nil {
				data["finished_at"] = task.Finished.Format(time.RFC3339)
			}
		}
		m.emitAgentEvent(events.AgentTaskUpdate, task.SessionID, data)
	})

	tools.SetSessionFolderResolver(func(sessionID string) string {
		return m.getFolder(sessionID)
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

// DecideSubagentRole evaluates a task prompt using Jev to select the optimal sub-agent role.
func (m *Manager) DecideSubagentRole(task string) *jev.RoleDecision {
	defs := m.ListAgentDefinitions()
	var roleIDs []string
	for _, d := range defs {
		roleIDs = append(roleIDs, d.ID, string(d.Role))
	}
	m.mu.RLock()
	router := m.jevRouter
	m.mu.RUnlock()
	if router == nil {
		return jev.HeuristicRoleDecision(task, roleIDs)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	return router.DecideAgentRole(ctx, task, roleIDs)
}

// DecideSubagentForTask uses Jev to select the ideal subagent definition for a task.
func (m *Manager) DecideSubagentForTask(task string) (AgentDefinition, *jev.SubagentDecision) {
	defs := m.ListAgentDefinitions()
	var candidates []jev.SubagentCandidate
	for _, d := range defs {
		candidates = append(candidates, jev.SubagentCandidate{
			ID:          d.ID,
			Name:        d.Name,
			Role:        string(d.Role),
			Description: d.Description,
		})
	}

	m.mu.RLock()
	router := m.jevRouter
	m.mu.RUnlock()

	var dec *jev.SubagentDecision
	if router != nil && router.IsEnabled() {
		ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
		defer cancel()
		dec = router.DecideSubagent(ctx, task, candidates)
	} else {
		dec = jev.HeuristicSubagentDecision(task, candidates)
	}

	if dec != nil {
		if def, ok := m.FindDefinition(dec.SelectedID); ok {
			return def, dec
		}
		if def, ok := m.FindDefinition(dec.Role); ok {
			return def, dec
		}
	}

	// Fallback to coding definition
	if def, ok := m.FindDefinition("coding"); ok {
		return def, dec
	}
	return AgentDefinition{
		ID:   "coding",
		Name: "Coding Agent",
		Role: RoleCoding,
	}, dec
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
		stateChanged := s.State != StateIdle
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
		if stateChanged {
			m.saveSessionLocked(s.ID)
		}
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

	now := time.Now()
	model := ""
	if m.llmClient != nil {
		model = m.llmClient.GetConfig().Model
	}

	customPrompt := ""
	customRules := ""
	allowBash := false
	readOnly := false

	// Check if a definition exists for this role
	roleStr := string(role)
	var foundDef *AgentDefinition
	if d, ok := m.definitions[roleStr]; ok {
		foundDef = &d
	} else {
		for _, d := range m.definitions {
			if string(d.Role) == roleStr || strings.EqualFold(d.Name, roleStr) {
				foundDef = &d
				break
			}
		}
	}
	if foundDef == nil {
		for _, b := range builtinAgentDefs() {
			if b.ID == roleStr || string(b.Role) == roleStr || strings.EqualFold(b.Name, roleStr) {
				foundDef = &b
				break
			}
		}
	}

	var toolsList []string
	var mcpServers []string
	var skillsList []string

	if foundDef != nil {
		if foundDef.Model != "" {
			model = foundDef.Model
		}
		customPrompt = foundDef.Prompt
		customRules = foundDef.Rules
		allowBash = foundDef.AllowBash
		readOnly = foundDef.ReadOnly
		toolsList = foundDef.Tools
		mcpServers = foundDef.MCPServers
		skillsList = foundDef.Skills
	} else {
		if role == RoleResearch {
			allowBash = true
			readOnly = true
		} else if role == RolePlanning {
			readOnly = true
		} else {
			allowBash = true
		}
	}

	sysPrompt := buildSystemPrompt(role, customPrompt, customRules)

	sess := &Session{
		ID:           id,
		Name:         name,
		RoleFilter:   role,
		Model:        model,
		AllowBash:    allowBash,
		ReadOnly:     readOnly,
		Tools:        toolsList,
		MCPServers:   mcpServers,
		Skills:       skillsList,
		State:        StateIdle,
		Folder:       projFolder,
		ProjectName:  filepath.Base(projFolder),
		Messages:     make([]AgentMessage, 0),
		Tasks:        make([]TaskItem, 0),
		SystemPrompt: sysPrompt,
		CustomPrompt: customPrompt,
		CustomRules:  customRules,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	// NOTE: the sandbox is provisioned lazily in buildProjectContext on the
	// first turn — NOT here. CreateSession also runs from tests, which would
	// otherwise scatter test sandboxes into the real ~/.forge/sandbox.

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
	if cancel, ok := m.stepCancelFuncs[id]; ok {
		cancel()
		delete(m.stepCancelFuncs, id)
	}
	delete(m.sessions, id)
	if m.storePath != "" {
		dir := filepath.Join(filepath.Dir(m.storePath), "sessions")
		_ = os.Remove(filepath.Join(dir, id+".json"))
	}
	m.mu.Unlock()
	m.emitSessionUpdate(id)
}

// CompactSession compacts the conversation history of a session by summarizing
// earlier turns into a structured context summary and keeping the first prompt
// and recent turns, drastically reducing context window usage.
func (m *Manager) CompactSession(sessionID string) (*Session, error) {
	m.mu.Lock()
	sess, ok := m.sessions[sessionID]
	if !ok {
		m.mu.Unlock()
		return nil, fmt.Errorf("session %s not found", sessionID)
	}

	if len(sess.Messages) <= 4 {
		m.mu.Unlock()
		return sess, nil
	}

	// Keep the first message (initial prompt) and the last 3-4 messages
	keepTail := 4
	if len(sess.Messages) < keepTail+2 {
		keepTail = len(sess.Messages) - 2
	}
	recentMsgs := sess.Messages[len(sess.Messages)-keepTail:]
	firstMsg := sess.Messages[0]

	// Build context summary from the truncated messages
	summary := buildContextSummary(sess.Messages, recentMsgs)
	if summary == "" {
		summary = "<CONTEXT_SUMMARY>\nEarlier conversation turns were compacted to optimize context window usage.\n</CONTEXT_SUMMARY>"
	}

	summaryMsg := AgentMessage{
		ID:        fmt.Sprintf("compact-%d", time.Now().UnixMilli()),
		Role:      "assistant",
		Timestamp: time.Now(),
		Content: []ContentBlock{
			{
				Type: "text",
				Text: fmt.Sprintf("📦 **Session Compacted**\n\n%s", summary),
			},
		},
	}

	compactedMsgs := make([]AgentMessage, 0, 2+len(recentMsgs))
	compactedMsgs = append(compactedMsgs, firstMsg, summaryMsg)
	compactedMsgs = append(compactedMsgs, recentMsgs...)

	sess.Messages = compactedMsgs
	sess.UpdatedAt = time.Now()
	m.saveSessionLocked(sessionID)
	m.mu.Unlock()

	m.emitSessionUpdate(sessionID)
	return sess, nil
}

// SteerMessage sends a mid-turn instruction directly into the running agent loop,
// immediately preempting any in-flight LLM stream or tool execution so the agent
// responds to the steering directive without waiting for the prior process.
func (m *Manager) SteerMessage(sessionID string, instruction string) error {
	trimmed := strings.TrimSpace(instruction)
	if trimmed == "" {
		return fmt.Errorf("empty steering instruction")
	}

	m.mu.Lock()
	_, ok := m.sessions[sessionID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("session %s not found", sessionID)
	}
	steerCh, hasSteer := m.steerChannels[sessionID]
	running := m.runningTurns[sessionID]
	stepCancel := m.stepCancelFuncs[sessionID]
	shellID := m.shells[sessionID]
	m.mu.Unlock()

	if hasSteer && running {
		// 1. Enqueue steering instruction
		select {
		case steerCh <- trimmed:
		default:
			return fmt.Errorf("steering queue is full")
		}

		// 2. Immediately cancel in-flight step (LLM HTTP call or running tool)
		if stepCancel != nil {
			stepCancel()
		}

		// 3. If there is a persistent shell command executing, send Ctrl-C immediately
		if shellID != "" && m.termMgr != nil {
			_, _ = m.termMgr.Write(shellID, []byte("\x03\n"))
		}
		return nil
	}

	// If the session is not actively in a reasoning turn, append as standard user message
	return m.SendMessage(context.Background(), sessionID, trimmed, nil)
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
	// Attach multimodal image content blocks for vision-capable models
	for _, p := range mentionedPaths {
		ext := strings.ToLower(filepath.Ext(p))
		if ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".webp" || ext == ".gif" {
			imgBytes, err := os.ReadFile(p)
			if err == nil && len(imgBytes) > 0 && len(imgBytes) <= 4*1024*1024 {
				mime := "image/png"
				switch ext {
				case ".jpg", ".jpeg":
					mime = "image/jpeg"
				case ".webp":
					mime = "image/webp"
				case ".gif":
					mime = "image/gif"
				}
				b64 := base64.StdEncoding.EncodeToString(imgBytes)
				messages[0].Content = append(messages[0].Content, ContentBlock{
					Type: "image",
					Text: fmt.Sprintf("data:%s;base64,%s", mime, b64),
				})
			}
		}
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
			skillName = strings.Trim(skillName, `"' `)

			skill, found := m.skillMgr.Get(skillName)
			if !found && mention.Args != "" {
				// Try combining skillName with Args or words from Args (e.g. "/skill:Native SDK Discovery & Authoring")
				fullCandidate := strings.TrimSpace(skillName + " " + mention.Args)
				if sk, ok := m.skillMgr.Get(fullCandidate); ok {
					skill = sk
					found = true
				} else {
					// Check prefix matches against available skills
					for _, candidate := range m.skillMgr.List() {
						cName := strings.ToLower(candidate.Name)
						if strings.HasPrefix(strings.ToLower(fullCandidate), cName) {
							skill = candidate
							found = true
							break
						}
					}
				}
			}

			if found {
				messages = append(messages, AgentMessage{
					ID:   uuid.New().String(),
					Role: "user",
					Content: []ContentBlock{
						{Type: "text", Text: skill.InvocationMessage(mention.Args)},
					},
					Timestamp: time.Now(),
				})

				// Record and emit synthetic load_skill tool execution so UI displays the skill snippet block
				toolCallID := "skill-" + uuid.New().String()[:8]
				toolArgs := map[string]any{"skill": skill.Name}
				toolOutput := fmt.Sprintf("Skill %q loaded successfully.\nDescription: %s\nDirectory: %s\n\nInstructions (SKILL.md):\n%s", skill.Name, skill.Description, skill.BaseDir(), skill.Body)
				m.emitToolStart(sessionID, ContentBlock{
					Type:       "tool_use",
					ToolCallID: toolCallID,
					Name:       "load_skill",
					Arguments:  toolArgs,
				}, 1)
				m.emitToolEnd(sessionID, ContentBlock{
					Type:       "tool_use",
					ToolCallID: toolCallID,
					Name:       "load_skill",
					Arguments:  toolArgs,
				}, toolOutput, map[string]any{
					"skill_name":  skill.Name,
					"description": skill.Description,
					"base_dir":    skill.BaseDir(),
					"path":        skill.Path,
					"body":        skill.Body,
				}, false, 1)
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

	// Launch background execution loop for agent turn.
	// If a turn is currently active or unwinding from a cancellation, interrupt it
	// and wait for it to cleanly finish before starting the fresh turn.
	m.mu.Lock()
	if cancel, ok := m.cancelFuncs[sessionID]; ok {
		cancel()
		delete(m.cancelFuncs, sessionID)
	}
	execCtx, cancel := context.WithCancel(context.Background())
	m.cancelFuncs[sessionID] = cancel
	isAlreadyRunning := m.runningTurns[sessionID]
	m.mu.Unlock()

	if isAlreadyRunning {
		go func() {
			for i := 0; i < 60; i++ {
				m.mu.Lock()
				running := m.runningTurns[sessionID]
				m.mu.Unlock()
				if !running {
					break
				}
				time.Sleep(50 * time.Millisecond)
			}
			m.mu.Lock()
			m.runningTurns[sessionID] = false
			m.mu.Unlock()
			m.launchTurnGoroutine(execCtx, sessionID)
		}()
	} else {
		m.launchTurnGoroutine(execCtx, sessionID)
	}

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
			delete(m.cancelFuncs, sessionID)
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
	// Initialize or reset loop detector and steer channel for this turn.
	m.mu.Lock()
	if _, ok := m.loopDetectors[sessionID]; !ok {
		m.loopDetectors[sessionID] = NewLoopDetector(DefaultLoopWindowSize, DefaultLoopMaxRepeats)
	} else {
		m.loopDetectors[sessionID].Reset()
	}
	steerCh := make(chan string, 16)
	m.steerChannels[sessionID] = steerCh
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		delete(m.steerChannels, sessionID)
		delete(m.stepCancelFuncs, sessionID)
		m.mu.Unlock()
	}()

	// Project/skills context is stable for the whole turn — building it per
	// iteration re-runs a git snapshot (seconds) every loop. Build once.
	m.mu.RLock()
	folder := ""
	if sess, ok := m.sessions[sessionID]; ok {
		folder = sess.Folder
	}
	m.mu.RUnlock()

	// Route tools via TypeSafe AI Jev router if enabled.
	// Jev classifies the user's intent into focused categories (explore, read, modify, execute, git, web, none)
	// preventing tool hallucinations and excessive schema overhead. The probe
	// runs concurrently with the context build below: when the router endpoint
	// is unreachable it blocks for its full client timeout, which otherwise
	// sits entirely on the critical path before the first token.
	var routedTools []llm.ToolDefinition
	jevRouted := false
	routeDone := make(chan struct{})
	probeStarted := false
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

		m.syncPlugins(sessionID)
		m.syncMCPTools(ctx)

		if goal != "" {
			probeStarted = true
			allTools := m.toolReg.Definitions()
			go func() {
				defer close(routeDone)
				routedTools, _ = m.jevRouter.Route(ctx, goal, allTools)
				jevRouted = true
			}()
		}
	}
	if !probeStarted {
		close(routeDone)
	}

	projectCtx := ""
	if folder != "" {
		projectCtx = buildProjectContext(folder)
	}
	m.mu.RLock()
	currentSess := m.sessions[sessionID]
	m.mu.RUnlock()
	skillsCtx := m.buildSkillsContext(currentSess)

	<-routeDone

	budgetSpent := 0
	transcriptWindowSize := 40
	isFinalSummaryStep := false
	for iter := 0; iter < maxReasoningSteps; iter++ {
		turnNum := iter + 1
		if iter == maxReasoningSteps-1 {
			isFinalSummaryStep = true
		}
		if ctx.Err() != nil {
			m.finishTurn(sessionID, ctx.Err().Error())
			return
		}

		// Helper to drain mid-turn user steering prompts from the steering channel
		drainAndAppendSteering := func() bool {
			var steered []string
			draining := true
			for draining {
				select {
				case prompt := <-steerCh:
					if prompt != "" {
						steered = append(steered, prompt)
					}
				default:
					draining = false
				}
			}
			if len(steered) > 0 {
				m.mu.Lock()
				if s, ok := m.sessions[sessionID]; ok {
					for _, text := range steered {
						s.Messages = append(s.Messages, AgentMessage{
							ID:   uuid.New().String(),
							Role: "user",
							Content: []ContentBlock{
								{Type: "text", Text: text},
							},
							Timestamp: time.Now(),
						})
					}
				}
				m.mu.Unlock()
				m.emitSessionUpdate(sessionID)
				return true
			}
			return false
		}

		// Drain any mid-turn user steering instructions that arrived before this reasoning step
		drainAndAppendSteering()

		// Step-scoped context for immediate mid-turn preemption
		stepCtx, stepCancel := context.WithCancel(ctx)
		m.mu.Lock()
		m.stepCancelFuncs[sessionID] = stepCancel
		m.mu.Unlock()

		cleanupStep := func() {
			stepCancel()
			m.mu.Lock()
			delete(m.stepCancelFuncs, sessionID)
			m.mu.Unlock()
		}

		m.mu.Lock()
		sess, ok := m.sessions[sessionID]
		if !ok {
			m.mu.Unlock()
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
		history, contextSummary := windowTranscriptWithSummary(sess.Messages, transcriptWindowSize)
		sysContent := sess.SystemPrompt
		dialect := sess.Dialect
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
		m.mu.Unlock()

		m.emitSessionUpdate(sessionID)
		if state == StateIdle {
			m.emitAgentEvent(events.AgentTurnStart, sessionID, map[string]interface{}{})
		}

		// Fetch tool definitions (built-ins + MCP tools registered into the
		// registry; filtered by Jev router if active; dialect mode will strip these and use in-band text calls).
		var toolDefs []llm.ToolDefinition
		if isFinalSummaryStep {
			// Opencode graceful exhaustion: disable all tools and mandate text-only summary
			toolDefs = nil
			sysContent += "\n\n" + maxStepsPrompt
		} else if jevRouted {
			toolDefs = routedTools
			toolDefs = FlattenToolDefinitions(toolDefs)
		} else {
			toolDefs = m.toolReg.Definitions()
			toolDefs = FlattenToolDefinitions(toolDefs)
		}

		// Filter out tools that are blocked for this session's role/permissions
		if sess != nil && len(toolDefs) > 0 {
			var allowedDefs []llm.ToolDefinition
			for _, td := range toolDefs {
				if m.isToolAllowedForSession(sess, td.Function.Name) {
					allowedDefs = append(allowedDefs, td)
				}
			}
			toolDefs = allowedDefs
		}
		log.Printf("[agent:%s] turn %d: offering %d tools (jevRouted=%v)", sessionID, turnNum, len(toolDefs), jevRouted)

		var dialectScanner *xmlDialectScanner
		var thoughtScanner *thoughtStreamScanner
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
			dialectScanner = nil
			thoughtScanner = newThoughtStreamScanner()
		}

		// Call LLM with streaming callback for real-time thinking and content.
		var lastEmit time.Time
		assistant := m.newAssistantMessage(sessionID, dialect)
		toolCallBuf := make(map[int]*llm.ToolCall)
		var toolCallOrder []int

		llmCtx := llm.WithSessionID(stepCtx, sessionID)
		resp, err := m.llmClient.ChatWithModelStreamDetailed(llmCtx, sess.Model, messages, toolDefs,
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
					} else if thoughtScanner != nil {
						for _, ev := range thoughtScanner.feed(deltaContent) {
							switch ev.kind {
							case "thinking":
								assistant.appendThinking(ev.text)
								m.emitThinkingDelta(sessionID, ev.text, turnNum)
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
			cleanupStep()
			if ctx.Err() != nil {
				m.finishTurn(sessionID, ctx.Err().Error())
				return
			}
			if stepCtx.Err() != nil {
				// Mid-turn steering preemption while LLM was streaming!
				m.mu.Lock()
				if !assistant.isEmpty() {
					if len(sess.Messages) > 0 && sess.Messages[len(sess.Messages)-1].Role == "assistant" &&
						sess.Messages[len(sess.Messages)-1].ID == assistant.ID {
						sess.Messages[len(sess.Messages)-1] = *assistant
					} else {
						sess.Messages = append(sess.Messages, *assistant)
					}
				}
				m.mu.Unlock()
				if !assistant.isEmpty() {
					m.emitMessageEnd(sessionID, assistant)
				}
				drainAndAppendSteering()
				continue
			}
			if isContextOverflowError(err) && transcriptWindowSize > 6 {
				log.Printf("[agent:%s] Context overflow detected (%v); auto-compacting transcript window from %d to %d and retrying step",
					sessionID, err, transcriptWindowSize, transcriptWindowSize/2)
				transcriptWindowSize = transcriptWindowSize / 2
				iter-- // Retry the same step with smaller window
				continue
			}
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
		} else if thoughtScanner != nil {
			for _, ev := range thoughtScanner.flush() {
				switch ev.kind {
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

		if ctx.Err() != nil {
			cleanupStep()
			m.finishTurn(sessionID, ctx.Err().Error())
			return
		}
		if stepCtx.Err() != nil {
			cleanupStep()
			drainAndAppendSteering()
			continue
		}

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
		// approval (the old code gated only the first tool and dropped the
		// rest). Scratch writes inside the project sandbox are exempt —
		// that is what the sandbox is for — but bash is never exempt
		// because a command cannot be path-scoped.
		if len(toolCalls) > 0 && !sess.AutoApprove {
			needApproval := false
			for _, tc := range toolCalls {
				if isMutatingTool(tc.Name) && !isSandboxScopedCall(tc, folder) {
					needApproval = true
					break
				}
			}
			if needApproval {
				cleanupStep()
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
			// Phase 6 snapshot: capture git tree hash before mutating tool executions
			for _, tc := range toolCalls {
				if isMutatingTool(tc.Name) {
					if snap := m.captureTurnGitSnapshot(folder); snap != "" {
						log.Printf("[agent:%s] Created pre-mutation git snapshot: %s", sessionID, snap)
					}
					break
				}
			}

			m.mu.Lock()
			sess.State = StateExecuting
			m.scheduleSave()
			m.mu.Unlock()
			m.emitSessionUpdate(sessionID)

			spent, fatal := m.executeScheduledBatch(stepCtx, sessionID, toolCalls, turnNum)
			budgetSpent += spent
			cleanupStep()
			if ctx.Err() != nil {
				m.finishTurn(sessionID, "context canceled")
				return
			}
			if stepCtx.Err() != nil {
				// Preempted by steering during tool execution!
				// Ensure all tool calls have a tool_result so state remains valid for the model
				m.mu.Lock()
				if s, ok := m.sessions[sessionID]; ok {
					existingResults := make(map[string]bool)
					for _, msg := range s.Messages {
						if msg.Role == "tool" {
							for _, cb := range msg.Content {
								if cb.Type == "tool_result" {
									existingResults[cb.ToolCallID] = true
								}
							}
						}
					}
					for _, tc := range toolCalls {
						if !existingResults[tc.ToolCallID] {
							s.Messages = append(s.Messages, AgentMessage{
								ID:   uuid.New().String(),
								Role: "tool",
								Content: []ContentBlock{
									{
										Type:       "tool_result",
										ToolCallID: tc.ToolCallID,
										Name:       tc.Name,
										Text:       "Interrupted by user steering instruction.",
										IsError:    true,
									},
								},
								Timestamp: time.Now(),
							})
						}
					}
				}
				m.mu.Unlock()
				drainAndAppendSteering()
				continue
			}
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
				cleanupStep()
				m.emitSessionUpdate(sessionID)
				return
			}
			if drainAndAppendSteering() {
				continue // loop again for the next LLM call with the new steering prompt!
			}

			continue // loop again for the next LLM call
		}

		// No tool calls: check if any mid-turn steering instruction arrived while model was generating
		cleanupStep()
		if drainAndAppendSteering() {
			continue // Continue reasoning loop with user's steering directives!
		}

		// No tool calls and no pending steering → the turn is done.
		m.finishTurn(sessionID, "")
		return
	}

	m.finishTurn(sessionID, "Agent reached maximum reasoning steps and provided summary of accomplishments and remaining work.")
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

	// Role-gating safety check for subagents and permissions
	var sess *Session
	m.mu.RLock()
	if s, ok := m.sessions[sessionID]; ok && s != nil {
		sess = s
	}
	m.mu.RUnlock()

	if sess != nil && !m.isToolAllowedForSession(sess, block.Name) {
		m.emitToolStart(sessionID, block, currentTurn)
		err := fmt.Errorf("tool %q is blocked for %s agents (this role is restricted by permissions: read_only=%v, allow_bash=%v)", block.Name, sess.RoleFilter, sess.ReadOnly, sess.AllowBash)
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
		argBytes, _ := json.Marshal(block.Arguments)
		argDisplay := string(argBytes)
		if len(argDisplay) > 2000 {
			argDisplay = argDisplay[:2000] + "... (truncated)"
		}
		contentStr = fmt.Sprintf("<tool_use_error>\nTool %q failed with error:\n%v\n\nArguments provided:\n%s\n\nPlease verify tool arguments, adjust parameter values, and retry.\n</tool_use_error>", block.Name, err, argDisplay)
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
	delete(m.cancelFuncs, sessionID)
	delete(m.runningTurns, sessionID)
	sess, ok := m.sessions[sessionID]
	if !ok || sess.State == StateIdle {
		m.mu.Unlock()
		return
	}
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
	} else if isCancel {
		if len(sess.Messages) > 0 && sess.Messages[len(sess.Messages)-1].Role == "assistant" {
			if len(sess.Messages[len(sess.Messages)-1].Content) == 0 {
				sess.Messages[len(sess.Messages)-1].Content = []ContentBlock{
					{Type: "text", Text: "*(stopped by user)*"},
				}
			}
		} else {
			sess.Messages = append(sess.Messages, AgentMessage{
				ID:   uuid.New().String(),
				Role: "assistant",
				Content: []ContentBlock{
					{Type: "text", Text: "*(stopped by user)*"},
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
	var parentNotificationText string
	if sess != nil {
		parentID = sess.ParentSessionID
		if parentID != "" {
			finalOutput := ""
			for i := len(sess.Messages) - 1; i >= 0; i-- {
				if sess.Messages[i].Role == "assistant" {
					for _, b := range sess.Messages[i].Content {
						trimmed := strings.TrimSpace(b.Text)
						if b.Type == "text" && trimmed != "" && trimmed != errMsg && trimmed != "*(stopped by user)*" {
							finalOutput = trimmed
							break
						}
					}
					if finalOutput != "" {
						break
					}
				}
			}
			if finalOutput == "" && errMsg != "" {
				finalOutput = errMsg
			} else if errMsg != "" && !strings.Contains(finalOutput, errMsg) {
				finalOutput = finalOutput + fmt.Sprintf("\n\n(Execution status: %s)", errMsg)
			}

			m.subagentNotesMu.Lock()
			m.subagentNotes[parentID] = append(m.subagentNotes[parentID], SubagentNotification{
				ID:        sess.ID,
				Name:      sess.Name,
				Role:      string(sess.RoleFilter),
				Output:    finalOutput,
				Completed: time.Now(),
			})
			m.subagentNotesMu.Unlock()

			statusSummary := "completed"
			if errMsg != "" {
				statusSummary = fmt.Sprintf("completed with notice: %s", errMsg)
			}
			parentNotificationText = fmt.Sprintf(
				"[Sub-agent Notification]\nSub-agent %q (%s, ID: %s) has %s.\n\nAccomplishments & Summary of Work:\n%s",
				sess.Name, sess.RoleFilter, sess.ID, statusSummary, finalOutput,
			)
		}
	}
	m.saveSessionLocked(sessionID)
	m.mu.Unlock()
	if autoTitlePrompt != "" && m.llmClient != nil {
		go m.autoTitleSession(sessionID, autoTitlePrompt)
	}
	m.emitSessionUpdate(sessionID)
	if parentID != "" && parentNotificationText != "" {
		m.emitSessionUpdate(parentID)
		// Opencode synthetic resume: wake or steer the parent session with subagent notification
		m.mu.Lock()
		steerCh, hasSteer := m.steerChannels[parentID]
		parentRunning := m.runningTurns[parentID]
		m.mu.Unlock()
		if hasSteer && parentRunning {
			select {
			case steerCh <- parentNotificationText:
			default:
			}
		} else if !parentRunning && !isCancel {
			// Parent turn had completed and was idle waiting for background subagent to complete.
			// Automatically wake up the parent agent and send the accomplishments/result!
			go func(pID, notif string) {
				_ = m.SendMessage(context.Background(), pID, notif, nil)
			}(parentID, parentNotificationText)
		}
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
		_ = m.termMgr.Resize(shellID, 1000, 10000)
		_, _, _ = m.termMgr.Exec(shellID, "stty -echo 2>/dev/null; unsetopt zle 2>/dev/null; unsetopt prompt_cr prompt_sp 2>/dev/null; precmd() { :; }; chpwd() { :; }; PROMPT=\"\"; PS1=\"\"; RPS1=\"\"; RPROMPT=\"\"", 3*time.Second)
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

func (b *sessionBridge) SelectSubagent(task string) (map[string]any, error) {
	chosenDef, dec := b.m.DecideSubagentForTask(task)
	res := map[string]any{
		"subagent_id":   chosenDef.ID,
		"subagent_name": chosenDef.Name,
		"role":          string(chosenDef.Role),
		"model":         chosenDef.Model,
		"description":   chosenDef.Description,
		"allow_bash":    chosenDef.AllowBash,
		"read_only":     chosenDef.ReadOnly,
	}
	if dec != nil {
		res["reason"] = dec.Reason
		res["confidence"] = dec.Confidence
		res["source"] = dec.Source
	}
	return res, nil
}

// resolveModelTier maps a model tier ('weak' / 'strong' / 'inherit' / ”) or explicit model name
// to a concrete model based on the parent session's provider or configuration.
// Maki Garbage Collector pattern: Weak models (Flash, Haiku, Mini) handle cheap exploration;
// Strong models (Opus, Sonnet, Pro) retain master coordination.
func (m *Manager) resolveModelTier(tier string, role string, parentModel string) string {
	cleanTier := strings.ToLower(strings.TrimSpace(tier))
	if cleanTier == "" {
		if role == string(RoleResearch) || role == "search" || role == "explore" {
			cleanTier = "weak"
		} else {
			cleanTier = "inherit"
		}
	}

	if parentModel == "" && m.llmClient != nil {
		parentModel = m.llmClient.GetConfig().Model
	}
	pmLower := strings.ToLower(parentModel)

	switch cleanTier {
	case "weak", "cheap", "fast", "exploration", "flash", "haiku", "mini":
		if strings.Contains(pmLower, "flash") || strings.Contains(pmLower, "mini") || strings.Contains(pmLower, "haiku") || strings.Contains(pmLower, "lite") {
			return parentModel
		}
		var candidate string
		if strings.Contains(pmLower, "claude") || strings.Contains(pmLower, "anthropic") {
			candidate = "claude-3-5-haiku-20241022"
		} else if strings.Contains(pmLower, "gemini") {
			candidate = "gemini-2.5-flash"
		} else if strings.Contains(pmLower, "gpt") || strings.Contains(pmLower, "o1") || strings.Contains(pmLower, "o3") || strings.Contains(pmLower, "openai") {
			candidate = "gpt-4o-mini"
		} else if strings.Contains(pmLower, "deepseek") {
			candidate = "deepseek-v4-1-flash"
		}
		if candidate != "" && m.llmClient != nil && m.llmClient.FindProviderForModel(candidate) != "" {
			return candidate
		}
		if parentModel != "" {
			return parentModel
		}
		if candidate != "" {
			return candidate
		}
		return "gpt-4o-mini"

	case "strong", "pro", "sonnet", "opus":
		var candidate string
		if strings.Contains(pmLower, "claude") || strings.Contains(pmLower, "anthropic") {
			candidate = "claude-3-7-sonnet-20250219"
		} else if strings.Contains(pmLower, "gemini") {
			candidate = "gemini-2.5-pro"
		} else if strings.Contains(pmLower, "gpt") || strings.Contains(pmLower, "o1") || strings.Contains(pmLower, "o3") || strings.Contains(pmLower, "openai") {
			candidate = "gpt-4o"
		} else if strings.Contains(pmLower, "deepseek") {
			candidate = "deepseek-v4-pro"
		}
		if candidate != "" && m.llmClient != nil && m.llmClient.FindProviderForModel(candidate) != "" {
			return candidate
		}
		if parentModel != "" {
			return parentModel
		}
		if candidate != "" {
			return candidate
		}
		return "claude-3-7-sonnet-20250219"

	case "inherit", "auto":
		return parentModel

	default:
		// Exact model name or custom override
		return tier
	}
}

func (b *sessionBridge) SpawnSubagent(task string, role string) (string, error) {
	return b.SpawnSubagentWithModel(task, role, "")
}

func (b *sessionBridge) SpawnSubagentWithModel(task string, role string, model string) (string, error) {
	var roleDecidedBy string
	var decisionReason string
	var matchedDef AgentDefinition
	hasMatchedDef := false

	if role == "" || strings.EqualFold(role, "auto") || strings.EqualFold(role, "jev") {
		var dec *jev.SubagentDecision
		matchedDef, dec = b.m.DecideSubagentForTask(task)
		hasMatchedDef = true
		role = matchedDef.ID
		if dec != nil {
			roleDecidedBy = dec.Source
			decisionReason = dec.Reason
		}
	}
	b.m.mu.Lock()
	parentSess, ok := b.m.sessions[b.sessionID]
	if !ok {
		b.m.mu.Unlock()
		return "", fmt.Errorf("parent session %s not found", b.sessionID)
	}

	// Guard against infinite sub-agent recursion loops (depth limit = 2)
	depth := 0
	curr := parentSess
	for curr != nil && curr.ParentSessionID != "" {
		depth++
		curr = b.m.sessions[curr.ParentSessionID]
		if depth >= 2 {
			b.m.mu.Unlock()
			return "", fmt.Errorf("maximum sub-agent recursion depth (2) reached; cannot spawn further child agents")
		}
	}

	folder := parentSess.Folder
	parentAutoApprove := parentSess.AutoApprove
	parentModel := parentSess.Model
	b.m.mu.Unlock()

	// Look up configured definition for this role/name
	var def AgentDefinition
	var foundDef bool
	if hasMatchedDef {
		def = matchedDef
		foundDef = true
	} else {
		def, foundDef = b.m.FindDefinition(role)
	}
	roleFilter := RoleFilter(role)
	titleName := strings.Title(role)
	subModel := model
	customPrompt := ""
	customRules := ""
	allowBash := false
	readOnly := false

	if foundDef {
		roleFilter = def.Role
		titleName = def.Name
		if def.Model != "" {
			// A configured model on the agent definition takes precedence over tier defaults,
			// unless the caller explicitly provided a concrete model name override (not a generic tier keyword).
			isTierKeyword := strings.EqualFold(model, "weak") || strings.EqualFold(model, "strong") ||
				strings.EqualFold(model, "inherit") || strings.EqualFold(model, "auto") ||
				strings.EqualFold(model, "cheap") || strings.EqualFold(model, "fast")
			if model == "" || isTierKeyword {
				subModel = def.Model
			}
		} else if subModel == "" {
			subModel = def.Model
		}
		customPrompt = def.Prompt
		customRules = def.Rules
		allowBash = def.AllowBash
		readOnly = def.ReadOnly
	} else {
		// Built-in default permissions
		if roleFilter == RoleResearch {
			allowBash = true
			readOnly = true
		} else if roleFilter == RolePlanning {
			readOnly = true
		} else {
			allowBash = true
		}
	}

	// Model tiering resolution (Maki pattern): resolve 'weak', 'strong', or defaults
	subModel = b.m.resolveModelTier(subModel, string(roleFilter), parentModel)
	if subModel == "" && parentModel != "" {
		subModel = parentModel
	}

	subSess, err := b.m.CreateSession(fmt.Sprintf("%s Sub-agent", titleName), roleFilter, folder)
	if err != nil {
		return "", err
	}

	b.m.mu.Lock()
	if subModel != "" {
		subSess.Model = subModel
	}
	if customPrompt != "" {
		subSess.CustomPrompt = customPrompt
	}
	if customRules != "" {
		subSess.CustomRules = customRules
	}
	if roleDecidedBy != "" {
		log.Printf("[agent] Jev auto-routed sub-agent %s to role %q (source: %s, reason: %s)", subSess.ID, role, roleDecidedBy, decisionReason)
	}
	subSess.SystemPrompt = buildSystemPrompt(roleFilter, subSess.CustomPrompt, subSess.CustomRules)
	subSess.AllowBash = allowBash
	subSess.ReadOnly = readOnly
	if foundDef {
		subSess.Tools = def.Tools
		subSess.MCPServers = def.MCPServers
		subSess.Skills = def.Skills
	}
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

func (b *sessionBridge) ScheduleTimer(duration time.Duration, cronExpr string, prompt string, taskID string) (string, error) {
	return b.m.ScheduleTimer(b.sessionID, duration, cronExpr, prompt, taskID)
}

func (b *sessionBridge) CancelSchedule(scheduleID string) bool {
	return b.m.CancelSchedule(scheduleID)
}

func (b *sessionBridge) ListSchedules() []map[string]any {
	items := b.m.ListSchedules(b.sessionID)
	out := make([]map[string]any, 0, len(items))
	for _, it := range items {
		out = append(out, it.ToSnapshot())
	}
	return out
}

// ScheduleTimer creates an active timer or cron job for an agent session.
func (m *Manager) ScheduleTimer(sessionID string, duration time.Duration, cronExpr string, prompt string, taskID string) (string, error) {
	if prompt == "" {
		return "", fmt.Errorf("prompt is required")
	}
	id := fmt.Sprintf("sched-%s", uuid.New().String()[:8])
	now := time.Now()
	if duration <= 0 {
		duration = 60 * time.Second
	}
	nextRun := now.Add(duration)

	ctx, cancel := context.WithCancel(context.Background())
	item := &AgentSchedule{
		ID:        id,
		SessionID: sessionID,
		Prompt:    prompt,
		TaskID:    taskID,
		Duration:  duration,
		CronExpr:  cronExpr,
		CreatedAt: now,
		NextRun:   nextRun,
		Active:    true,
		cancel:    cancel,
	}

	m.schedulesMu.Lock()
	if m.schedules == nil {
		m.schedules = make(map[string]*AgentSchedule)
	}
	m.schedules[id] = item
	m.schedulesMu.Unlock()

	// Emit creation event
	m.emitAgentEvent(events.AgentScheduleUpdate, sessionID, item.ToSnapshot())

	go func() {
		defer cancel()
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Until(nextRun)):
		}

		m.schedulesMu.Lock()
		item.Active = false
		m.schedulesMu.Unlock()

		m.emitAgentEvent(events.AgentScheduleUpdate, sessionID, map[string]any{
			"id":         item.ID,
			"session_id": item.SessionID,
			"status":     "fired",
			"prompt":     item.Prompt,
			"task_id":    item.TaskID,
			"active":     false,
		})

		// Wake up the agent session with the prompt and task status
		wakeUpMsg := fmt.Sprintf("<schedule_notification>\n[Timer Alert - %s elapsed]: %s\n</schedule_notification>", item.Duration.String(), item.Prompt)
		targetTaskID := item.TaskID
		if targetTaskID == "" {
			for _, t := range tools.ListTasks("") {
				if t.Status == tools.TaskRunning {
					targetTaskID = t.ID
					break
				}
			}
			if targetTaskID == "" {
				all := tools.ListTasks("")
				if len(all) > 0 {
					targetTaskID = all[0].ID
				}
			}
		}
		if targetTaskID != "" {
			if t, ok := tools.GetTask(targetTaskID); ok {
				wakeUpMsg += fmt.Sprintf("\n<background_task_status task_id=%q command=%q status=%q exit_code=%d>\nLast Output:\n%s\n</background_task_status>", t.ID, t.Command, t.Status, t.ExitCode, t.OutputTail)
			}
		}

		_ = m.SendMessage(context.Background(), sessionID, wakeUpMsg, nil)
	}()

	return id, nil
}

// CancelSchedule cancels an active schedule or timer.
func (m *Manager) CancelSchedule(scheduleID string) bool {
	m.schedulesMu.Lock()
	defer m.schedulesMu.Unlock()
	item, ok := m.schedules[scheduleID]
	if !ok || !item.Active {
		return false
	}
	item.Active = false
	if item.cancel != nil {
		item.cancel()
	}
	m.emitAgentEvent(events.AgentScheduleUpdate, item.SessionID, map[string]any{
		"id":         item.ID,
		"session_id": item.SessionID,
		"status":     "cancelled",
		"active":     false,
	})
	return true
}

// ListSchedules returns all active or recent schedules for a session.
func (m *Manager) ListSchedules(sessionID string) []*AgentSchedule {
	m.schedulesMu.Lock()
	defer m.schedulesMu.Unlock()
	out := make([]*AgentSchedule, 0)
	for _, s := range m.schedules {
		if sessionID != "" && s.SessionID != sessionID {
			continue
		}
		out = append(out, s)
	}
	if len(out) == 0 && sessionID != "" {
		for _, s := range m.schedules {
			if s.Active {
				out = append(out, s)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out
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

// syncMCPTools registers tools from live connected MCP servers into the tool registry.
func (m *Manager) syncMCPTools(ctx context.Context) {
	if m.mcpMgr == nil || m.toolReg == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}

	connected := m.mcpMgr.ListConnectedTools()
	if len(connected) == 0 {
		_ = m.mcpMgr.ConnectAll(ctx)
		connected = m.mcpMgr.ListConnectedTools()
	}

	for _, t := range connected {
		m.toolReg.RegisterMCPToolWithCaller(llm.MCPTool{
			ServerName:  t.ServerName,
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
		}, m.mcpMgr)
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
	sb.WriteString("Compact structured summary of earlier conversation turns truncated to fit context:\n\n")

	sb.WriteString("## Objective\n")
	if len(userPrompts) > 0 {
		sb.WriteString(fmt.Sprintf("- %s\n", userPrompts[0]))
	} else {
		sb.WriteString("- (none)\n")
	}

	sb.WriteString("\n## Earlier User Requests\n")
	for i, p := range userPrompts {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, p))
	}

	sb.WriteString("\n## Relevant Files\n")
	if len(filesTouched) > 0 {
		count := 0
		for f := range filesTouched {
			sb.WriteString(fmt.Sprintf("- `%s`\n", f))
			count++
			if count >= 15 {
				sb.WriteString("- ... (additional files omitted)\n")
				break
			}
		}
	} else {
		sb.WriteString("- (none)\n")
	}

	sb.WriteString("\n## Important Context\n")
	sb.WriteString("- Preserved context from truncated turns; resume execution with above objectives and files.\n")
	sb.WriteString("</CONTEXT_SUMMARY>")
	return sb.String()
}

// maxStepsPrompt is presented to the agent when maxReasoningSteps is reached.
// Disables tool calls and forces the agent to summarize accomplishments and remaining work.
const maxStepsPrompt = `CRITICAL - MAXIMUM REASONING STEPS REACHED

The maximum number of reasoning steps allowed for this task has been reached. Tools are disabled until next user input. Respond with text only.

STRICT REQUIREMENTS:
1. Do NOT make any tool calls (no reads, writes, edits, searches, or terminal commands)
2. Summarize work done so far:
   - What has been accomplished
   - Which goals remain incomplete
   - Concrete next steps for the user or next turn
3. Respond with text ONLY.`

// isContextOverflowError detects whether an error from the LLM indicates a context length overflow.
func isContextOverflowError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	patterns := []string{
		"prompt is too long",
		"context window",
		"maximum context length",
		"context_length_exceeded",
		"too many tokens",
		"token limit exceeded",
		"reduce the length of the messages",
		"request entity too large",
		"payload too large",
		"request_too_large",
		"model_context_window_exceeded",
		"413",
	}
	for _, p := range patterns {
		if strings.Contains(msg, p) {
			return true
		}
	}
	return false
}

// captureTurnGitSnapshot creates a lightweight git tree snapshot hash for the session workspace
// before mutating tool execution turns, enabling clean rollback.
func (m *Manager) captureTurnGitSnapshot(folder string) string {
	if folder == "" {
		return ""
	}
	// Check if folder is in a git repository
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = folder
	if err := cmd.Run(); err != nil {
		return ""
	}
	// Create git write-tree snapshot of current state
	writeCmd := exec.Command("git", "write-tree")
	writeCmd.Dir = folder
	out, err := writeCmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// maxToolOutputBytes defines the maximum bytes of tool output passed directly to the model context.
const maxToolOutputBytes = 50 * 1024 // 50 KiB

// maxToolOutputLines defines the maximum line count of tool output passed directly to the model context.
const maxToolOutputLines = 2000

// truncateAndOffloadToolOutput bounds tool output fed to the model context.
// Large outputs exceeding 50 KiB or 2000 lines are persisted to disk and replaced with a preview and file pointer.
func (m *Manager) truncateAndOffloadToolOutput(sessionID, toolCallID, toolName, content string) string {
	lines := strings.Split(content, "\n")
	if len(content) <= maxToolOutputBytes && len(lines) <= maxToolOutputLines {
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

		if len(lines) > maxToolOutputLines {
			head := strings.Join(lines[:1200], "\n")
			tail := strings.Join(lines[len(lines)-300:], "\n")
			omitted := len(lines) - 1500
			return fmt.Sprintf("%s\n\n[... %d lines omitted; full output (%d bytes) saved to %s ...]\n\n%s", head, omitted, len(content), filePath, tail)
		}
		head := content[:35000]
		tail := content[len(content)-10000:]
		omitted := len(content) - 45000
		return fmt.Sprintf("%s\n\n[... %d characters omitted; full output (%d bytes) saved to %s ...]\n\n%s", head, omitted, len(content), filePath, tail)
	}

	if len(lines) > maxToolOutputLines {
		return strings.Join(lines[:1200], "\n") + fmt.Sprintf("\n\n[... %d lines omitted ...]\n\n", len(lines)-1500) + strings.Join(lines[len(lines)-300:], "\n")
	}
	return content[:maxToolOutputBytes] + fmt.Sprintf("\n\n[... %d characters omitted ...]", len(content)-maxToolOutputBytes)
}

func (m *Manager) drainSubagentNotifications(sessionID string) []SubagentNotification {
	m.subagentNotesMu.Lock()
	defer m.subagentNotesMu.Unlock()
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
	case strings.Contains(m, "glm") || strings.Contains(m, "zhipu"):
		return "zhipu"
	case strings.Contains(m, "mimo") || strings.Contains(m, "xiaomi"):
		return "mimo"
	case strings.Contains(m, "kenari") || strings.Contains(m, "openrouter") || strings.HasPrefix(m, "free:") || strings.HasSuffix(m, ":free"):
		return "openrouter"
	case strings.Contains(m, "grok") || strings.Contains(m, "xai"):
		return "xai"
	case strings.Contains(m, "llama") || strings.Contains(m, "meta-llama"):
		return "meta"
	case strings.Contains(m, "cohere") || strings.Contains(m, "command-r"):
		return "cohere"
	case strings.Contains(m, "ollama"):
		return "ollama"
	case strings.Contains(m, "groq"):
		return "groq"
	default:
		if idx := strings.Index(model, "/"); idx > 0 {
			return strings.ToLower(model[:idx])
		}
		return "default"
	}
}

func (m *Manager) inferProvider(model string) string {
	if m.llmClient != nil {
		if p := m.llmClient.FindProviderForModel(model); p != "" {
			return p
		}
	}
	return inferProvider(model)
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
		providerName := m.inferProvider(modelName)

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
				// Computer-use screenshots must be VISIBLE to the model, but the
				// tool role is text-only on OpenAI-compatible APIs — append the
				// capture as a user message with an image part.
				if uri := screenshotDataURI(b); uri != "" {
					if len(out) > 0 && out[len(out)-1].Role == llm.RoleUser {
						out = append(out, llm.LLMMessage{
							Role:    llm.RoleAssistant,
							Content: "Understood.",
						})
					}
					out = append(out, llm.LLMMessage{
						Role: llm.RoleUser,
						Content: []map[string]interface{}{
							{"type": "text", "text": "[Screenshot just captured by computer_screenshot — inspect it before acting]"},
							{"type": "image_url", "image_url": map[string]interface{}{"url": uri}},
						},
					})
				}
			}
		case "user":
			var hasImages bool
			for _, b := range msg.Content {
				if b.Type == "image" && b.Text != "" {
					hasImages = true
					break
				}
			}
			var userContent interface{}
			if hasImages {
				var parts []map[string]interface{}
				text := msg.Text()
				if text != "" {
					parts = append(parts, map[string]interface{}{
						"type": "text",
						"text": text,
					})
				}
				for _, b := range msg.Content {
					if b.Type == "image" && b.Text != "" {
						parts = append(parts, map[string]interface{}{
							"type": "image_url",
							"image_url": map[string]interface{}{
								"url": b.Text,
							},
						})
					}
				}
				userContent = parts
			} else {
				userContent = msg.Text()
			}
			if userContent != "" && userContent != nil {
				// Prevent consecutive user messages which cause 400 errors with strict API providers
				if len(out) > 0 && out[len(out)-1].Role == llm.RoleUser {
					out = append(out, llm.LLMMessage{
						Role:    llm.RoleAssistant,
						Content: "Understood.",
					})
				}
				out = append(out, llm.LLMMessage{
					Role:    llm.RoleUser,
					Content: userContent,
				})
			}
		default:
			text := msg.Text()
			if text != "" {
				if len(out) > 0 && out[len(out)-1].Role == llm.Role(msg.Role) && msg.Role == "user" {
					out = append(out, llm.LLMMessage{
						Role:    llm.RoleAssistant,
						Content: "Understood.",
					})
				}
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
	if cancel, ok := m.stepCancelFuncs[sessionID]; ok {
		cancel()
		delete(m.stepCancelFuncs, sessionID)
	}
	shellID := m.shells[sessionID]
	m.mu.Unlock()

	// If there's an active shell running a command, interrupt it immediately (send Ctrl-C)
	if shellID != "" && m.termMgr != nil {
		_, _ = m.termMgr.Write(shellID, []byte("\x03\n"))
	}

	m.finishTurn(sessionID, "context canceled")
}

func isBashTool(name string) bool {
	switch name {
	case "bash", "run_shell", "exec", "run_command", "python_eval", "python":
		return true
	}
	return false
}

func isFileMutationTool(name string) bool {
	switch name {
	case "write", "write_file", "create_file", "edit", "edit_file", "replace_file_content":
		return true
	}
	return false
}

func isMutatingTool(name string) bool {
	return isBashTool(name) || isFileMutationTool(name) || isComputerInputTool(name)
}

// isComputerInputTool reports whether a tool takes over the user's desktop.
// These pause for approval like other mutations — even though the registry
// cannot see their side effects — unless the session runs in Full access.
// computer_screenshot is deliberately NOT here: it is read-only.
func isComputerInputTool(name string) bool {
	switch name {
	case "computer_click", "computer_type", "computer_key", "computer_scroll", "computer_window_list":
		return true
	}
	return false
}

// screenshotDataURI extracts the computer_screenshot capture from a tool
// result payload and returns it as a data URI the model can view. Guarded
// by tool name so every other tool pays nothing; caps at 4MB like user
// image attachments.
func screenshotDataURI(b ContentBlock) string {
	if b.Name != "computer_screenshot" || b.Text == "" {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(b.Text), &m); err != nil {
		return ""
	}
	p, _ := m["screenshot_path"].(string)
	if p == "" || !strings.HasSuffix(strings.ToLower(p), ".png") {
		return ""
	}
	st, err := os.Stat(p)
	if err != nil || st.Size() > 4<<20 {
		return ""
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(data)
}

// isSandboxScopedCall reports whether a mutating tool call writes inside the
// project sandbox (the agent's own scratch area, see internal/sandbox). Only
// file-mutation tools can be path-scoped; bash is never sandbox-scoped.
func isSandboxScopedCall(tc ContentBlock, folder string) bool {
	if isBashTool(tc.Name) || !isFileMutationTool(tc.Name) {
		return false
	}
	if folder == "" {
		return false
	}
	root, err := sandbox.Ensure(folder)
	if err != nil {
		return false
	}
	for _, key := range []string{"path", "file_path", "filepath", "filePath", "target_file"} {
		if v, ok := tc.Arguments[key].(string); ok && v != "" {
			// Relative paths resolve against the workspace folder, which is
			// by definition outside the sandbox.
			if !filepath.IsAbs(v) {
				return false
			}
			return underDir(root, v)
		}
	}
	return false
}

// underDir reports whether path is inside dir (lexical containment).
func underDir(dir, path string) bool {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	return rel != ".." && filepath.IsLocal(rel)
}

func (m *Manager) isToolAllowedForSession(sess *Session, toolName string) bool {
	if sess == nil {
		return true
	}
	allowBash := sess.AllowBash
	readOnly := sess.ReadOnly
	role := sess.RoleFilter

	// Universal coordination tools are always allowed
	switch toolName {
	case "todo", "update_tasks", "invoke_subagent", "task", "select_subagent", "get_subagent_output", "list_subagents", "ask", "schedule", "cancel_schedule", "list_schedules", "create_skill", "load_skill", "skill", "call_skill", "run_skill", "invoke_skill", "list_skills", "recall_memory", "learn_memory":
		return true
	}

	// 1. Read-only gate
	if isFileMutationTool(toolName) && readOnly {
		return false
	}
	if isBashTool(toolName) && !allowBash && readOnly {
		return false
	}

	// 2. Fetch definition tools/mcp if not present directly on session
	toolsList := sess.Tools
	mcpServers := sess.MCPServers
	if len(toolsList) == 0 && len(mcpServers) == 0 {
		if def, ok := m.FindDefinition(string(role)); ok {
			toolsList = def.Tools
			mcpServers = def.MCPServers
		}
	}

	// 3. MCP tool filter
	spec, isKnown := m.toolReg.Lookup(toolName)
	isMCP := (isKnown && spec.IsMCP) || strings.HasPrefix(toolName, "mcp_") || strings.HasPrefix(toolName, "mcp__") || strings.Contains(toolName, "/") || strings.HasPrefix(toolName, "cordis_")
	if isMCP {
		if len(mcpServers) > 0 {
			serverName := spec.ServerName
			if serverName == "" {
				if strings.Contains(toolName, "/") {
					serverName = strings.SplitN(toolName, "/", 2)[0]
				} else if strings.HasPrefix(toolName, "mcp__") {
					parts := strings.Split(toolName, "__")
					if len(parts) >= 2 {
						serverName = parts[1]
					}
				}
			}
			matchedServer := false
			for _, s := range mcpServers {
				sClean := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(s)), "-", "_")
				if strings.EqualFold(s, serverName) || (serverName != "" && strings.EqualFold(sClean, strings.ReplaceAll(strings.ToLower(serverName), "-", "_"))) || strings.Contains(strings.ToLower(toolName), sClean) {
					matchedServer = true
					break
				}
			}
			if !matchedServer {
				// If user explicitly mentions this server or tool in goal or recent messages, don't block it.
				if sess != nil && serverName != "" {
					sClean := strings.ToLower(serverName)
					if strings.Contains(strings.ToLower(sess.Progress.CurrentGoal), sClean) {
						return true
					}
					for i := len(sess.Messages) - 1; i >= 0 && i >= len(sess.Messages)-4; i-- {
						if strings.Contains(strings.ToLower(sess.Messages[i].Text()), sClean) {
							return true
						}
					}
				}
				// If mcpServers only has the legacy hardcoded defaults ["context7", "github"], allow other servers too
				if len(mcpServers) == 2 && ((mcpServers[0] == "context7" && mcpServers[1] == "github") || (mcpServers[0] == "github" && mcpServers[1] == "context7")) {
					return true
				}
				return false
			}
		}
		return true
	}

	// 4. Granular tool whitelist
	if len(toolsList) > 0 {
		return isToolInCategories(toolName, toolsList)
	}

	return true
}

func isToolInCategories(toolName string, categories []string) bool {
	lowerName := strings.ToLower(toolName)
	for _, cat := range categories {
		c := strings.ToLower(strings.TrimSpace(cat))
		if c == "" {
			continue
		}
		if c == lowerName {
			return true
		}
		switch c {
		case "filesystem", "files", "fs":
			switch lowerName {
			case "read", "read_file", "view_file", "cat", "read_multiple", "read_directory_files",
				"list_dir", "ls", "write", "write_file", "create_file", "edit", "edit_file",
				"replace_file_content", "multiedit", "search", "search_workspace", "find", "glob", "index":
				return true
			}
		case "terminal", "bash", "exec", "shell":
			switch lowerName {
			case "bash", "run_shell", "exec", "run_command", "background_shell",
				"task_output", "task_stop", "task_list", "eval", "run_code", "python_eval", "python":
				return true
			}
		case "git", "git_read":
			switch lowerName {
			case "git_status", "diff":
				return true
			}
		case "web", "web_fetch", "search":
			switch lowerName {
			case "web_fetch", "web_search", "fetch", "search_web":
				return true
			}
		case "browser":
			if strings.HasPrefix(lowerName, "browser_") {
				return true
			}
		}
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
  - index: {"path": "file/path.ext"} (structural AST skeleton with exact line numbers [start-end] — use this FIRST before reading)
  - read: {"path": "file/path.ext", "start_line"?: 1, "end_line"?: 50} (read file slice or whole file)
  - read_multiple: {"paths": ["file1.ext", "file2.ext"]} (batch read multiple files)
  - write: {"path": "new/file.ext", "content": "..."} (brand-new files ONLY)
  - edit: {"path": "existing/file.ext", "old_string": "...", "new_string": "..."} (surgical patch in existing files)
  - multiedit: {"path": "existing/file.ext", "edits": [{"old_string": "...", "new_string": "..."}]} (atomic multi-patch in a single file)
  - bash: {"command": "..."}
  - python_eval: {"code": "..."}
  - ask: {"questions": [{"id": "...", "question": "...", "options"?: ["..."]}]}
  - search: {"pattern": "...", "path"?: "..."}
  - web_fetch: {"url": "https://..."} (fast HTTP fetch & markdown extractor; preferred for web documentation)
  - web_search: {"query": "...", "limit"?: 5} (fast web search engine; returns titles, links, snippets)
  - find: {"path": "glob-pattern", "cwd"?: "..."}
  - git_status: {}
  - update_tasks: {"tasks": [{"id": "1", "title": "...", "status": "pending|in_progress|completed", "active_form"?: "..."}]}
  - invoke_subagent: {"task": "...", "subagent"?: "auto|<name>", "block"?: false, "timeout"?: 60}
  - select_subagent: {"task": "..."}
  - get_subagent_output: {"subagent_id": "...", "block"?: false, "timeout"?: 30}
  - list_subagents: {}
  - background_shell: {"command": "...", "cwd"?: "..."}
  - schedule: {"duration_seconds": 60, "prompt": "...", "task_id"?: "..."}
  - cancel_schedule: {"schedule_id": "..."}
  - list_schedules: {}
- CRITICAL: On any tool_use_error, NEVER repeat the identical call. Inspect the error and adapt your arguments or approach.
- Context Efficiency (MANDATORY):
  - Use the index tool FIRST on source files to inspect their structure and exact line ranges before reading with start_line/end_line. This dramatically saves context tokens and improves reasoning accuracy.
  - For multiple edits to a single file, ALWAYS prefer multiedit over multiple sequential edit calls.
- Task Planning & Progress Tracking (update_tasks):
  - For ANY multi-step task, feature implementation, refactoring, or multi-phase prompt: ALWAYS call update_tasks at the start to declare your plan as sequential tasks.
  - Automatic Goal Decomposition: Whenever a goal or plan is set (or displayed under "Goal: ..."), you MUST immediately decompose it into sequential, atomic tasks by calling update_tasks as your first action before modifying code.
  - Keep tasks updated as you work: mark tasks 'in_progress' with active_form (e.g. 'Writing StatusBadge.tsx...', 'Running npm test...'), and 'completed' once verified.
  - Dynamic Task Replacement (ZCode semantics): Calling update_tasks with a new list of tasks completely replaces the active checklist. When all prior tasks are completed and the user provides follow-up instructions, changes requirements, or gives a new objective, IMMEDIATELY call update_tasks with a fresh task breakdown for the new objective (or pass tasks: [] to reset). Never let stale completed tasks from an old objective linger into a new turn.
  - This powers the live floating collapsible checklist and progress tracker above the chat in the user interface.
- Subagents as "Garbage Collectors" (task / invoke_subagent / select_subagent):
  - Delegate research, search, codebase exploration, or subtasks using the task tool (or invoke_subagent).
  - When delegating research or search, a subagent is spawned with its own isolated context window.
  - The subagent can run dozens of searches, greps, and dead ends.
  - When finished, the subagent's full context is discarded ("garbage collected"), returning only a concise summary or schema-validated structured output (output_schema) to the parent agent, keeping your master context clean and efficient.
  - Model tiering: Weak models (Flash, Haiku, Mini) handle cheap exploration by default (model_tier: "weak"); Strong models (Opus, Sonnet, Pro) retain master coordination (model_tier: "strong").
  - Jev Autonomous Sub-agent Selection: You do NOT need to hardcode a role or specialist name. When you call invoke_subagent or task without an explicit subagent, Jev dynamically evaluates the task and selects the optimal sub-agent from all configured built-in roles and user specialists (matching their specialized prompts, models, and safety permissions).
  - You can also call select_subagent(task: "...") to consult Jev for the recommended sub-agent before spawning.
  - For synchronous results (default with task), block: true waits for completion. For background exploration, pass block: false. You can query status anytime with get_subagent_output or list_subagents.
  - Spawned sub-agents appear live in the UI under the Subagents tab and sidebar with dedicated minimal chat views.
- Code Editing (CRITICAL - DO NOT REWRITE WHOLE FILES):
  - NEVER rewrite an entire file or dump large walls of unchanged code. You are a precise software engineer.
  - For existing files: ALWAYS use edit with the smallest unique snippet of old_string to replace with new_string (surgical patch).
  - Use write ONLY when creating brand-new files from scratch that do not exist yet.
  - Read the file before editing it. old_string must match the file exactly (including indentation) and be unique.
  - After a successful edit or write, file state is tracked in context — do not re-read the file just to verify it.
  - Do NOT echo or paste file contents or code in your conversational chat response when calling write or edit tools. Execute the tool call directly without echoing the code in conversational text.
- Sandboxed Multi-Runtime Code Evaluation (eval / run_code / python_eval / python):
  - Execute code, evaluate expressions, perform calculations, inspect complex data structures, run regexes, or verify logic.
  - Execution runtime uses multi-tiered sandboxing: Sandboxed Python runner (Monty / Python3 AST REPL), falling back to Bun for JavaScript/TypeScript, and falling back to Bash if neither is available.
  - IMPORTANT: Write self-contained evaluation code that computes and outputs the result in a SINGLE tool call. Include a trailing expression (e.g. 'fib_sequence(10)') or 'print(...) / console.log(...)' on the last line. Each eval execution runs in a fresh process, so do not split function definition and invocation across multiple tool calls.
- Reusable Skills:
  - You have an active catalog of skills under <available_skills>. Call the skill or load_skill tool with the exact name whenever a user task matches a skill domain (e.g. impeccable for frontend design) to load its full playbook before acting.
  - You can create new reusable skills using create_skill to persist domain knowledge for future turns.
- Plugin Harness Extensibility:
  - You can inspect active plugins with list_plugins, toggle them with toggle_plugin, and create/extend plugins using create_plugin.
  - Plugins contribute custom command and script tools, system prompt sections, and skills.
- Long-term Memory:
  - You can recall stored memories, user preferences, and architecture conventions with recall_memory.
  - You can persist new decisions, style rules, or project facts using learn_memory so they persist across sessions.
- Long-running commands & Background Tasks (background_shell / bash):
  - For servers, dev watchers (vite, npm run dev), or long builds: use background_shell (or bash with run_in_background: true). This immediately returns a task_id and registers the process in the floating task bar. The user can view live logs, stop the process, or open an interactive shell session in the right sidebar.
  - Read output with task_output, stop with task_stop, list with task_list. Finished tasks are announced to you automatically via <task-notification>.
- Agent Cron Jobs & Timer Schedules (schedule):
  - When operations take time (e.g. large builds, server startup, Docker compose, test suites), do NOT busy-wait or poll in a loop.
  - Use schedule(duration_seconds: 60, prompt: "Check build output and report to user", task_id: "...") or schedule with cron_expression.
  - You can cancel with cancel_schedule(schedule_id: "...") or list with list_schedules.
  - The schedule countdown displays live in the floating task tracker. When the timer triggers, you receive an automated <schedule_notification> with your prompt and background task status so you can inspect output and continue.
- Fast Web & Documentation Access (web_fetch / web_search):
  - For researching online documentation, APIs, error messages, GitHub code, or papers: ALWAYS use web_fetch and web_search. They are fast, clean, token-efficient, and execute in milliseconds.
  - Browser Automation (browser_navigate, browser_click, browser_snapshot): Use heavy browser-* tools ONLY when the user explicitly requests interactive browser actions (e.g. testing web UI, visual QA, logging into web interfaces, or clicking page elements).
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

	// Agent sandbox: a scratch area outside the user's project, provisioned
	// lazily so even old sessions get one.
	if sbRoot, err := sandbox.Ensure(folder); err == nil && folder != "" {
		sb.WriteString("\n## Sandbox workspace\n")
		sb.WriteString("Scratch area (file writes here never need approval): ")
		sb.WriteString(sbRoot)
		sb.WriteString("\n- tmp/ — throwaway scratch; entries are pruned after 7 days\n")
		sb.WriteString("- thinking/ — notes and working through problems\n")
		sb.WriteString("- coding/ — temp code experiments\n")
		sb.WriteString("- logs/ — logs you want to persist\n")
		sb.WriteString("Use absolute paths under the sandbox for scratch work; keep it out of the workspace so only real deliverables land there.\n")
	}

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
func (m *Manager) buildSkillsContext(sess *Session) string {
	if m.skillMgr == nil {
		return ""
	}
	base := m.skillMgr.CatalogMarkdown()
	if sess != nil && len(sess.Skills) > 0 {
		var sb strings.Builder
		sb.WriteString("\n## Assigned Skills for this Agent\n")
		sb.WriteString("You are specialized in and equipped with these designated skills. Proactively load and follow their instructions using `load_skill`:\n")
		for _, s := range sess.Skills {
			sb.WriteString(fmt.Sprintf("- %s\n", s))
		}
		base += sb.String()
	}
	return base
}

// maxMentionFileBytes caps inlined file content per @-mention (32 KB).
const maxMentionFileBytes = 32 * 1024

func isBinaryData(b []byte) bool {
	limit := len(b)
	if limit > 512 {
		limit = 512
	}
	return bytes.IndexByte(b[:limit], 0) != -1
}

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
		ext := strings.ToLower(filepath.Ext(p))
		if ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".webp" || ext == ".gif" || ext == ".svg" {
			sb.WriteString(fmt.Sprintf("@%s (image, %d bytes — visual data attached)\n", p, info.Size()))
			continue
		}
		data, err := os.ReadFile(p)
		if err != nil {
			sb.WriteString(fmt.Sprintf("@%s — unreadable: %v\n", p, err))
			continue
		}
		if isBinaryData(data) {
			sb.WriteString(fmt.Sprintf("@%s (binary file, %d bytes — content omitted)\n", p, len(data)))
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
				rawTarget := strings.TrimPrefix(firstLine, "/skill:")
				rawTarget = strings.TrimPrefix(rawTarget, "/SKILL:")
				rawTarget = strings.TrimSpace(rawTarget)
				skillName := ""
				skillArgs := ""
				if strings.HasPrefix(rawTarget, "\"") {
					// Quoted skill name: /skill:"Native SDK Discovery & Authoring" extra args
					if endIdx := strings.Index(rawTarget[1:], "\""); endIdx != -1 {
						skillName = rawTarget[1 : 1+endIdx]
						skillArgs = strings.TrimSpace(rawTarget[1+endIdx+1:])
					} else {
						skillName = strings.TrimPrefix(rawTarget, "\"")
					}
				} else if strings.HasPrefix(rawTarget, "'") {
					// Single-quoted skill name
					if endIdx := strings.Index(rawTarget[1:], "'"); endIdx != -1 {
						skillName = rawTarget[1 : 1+endIdx]
						skillArgs = strings.TrimSpace(rawTarget[1+endIdx+1:])
					} else {
						skillName = strings.TrimPrefix(rawTarget, "'")
					}
				} else {
					skillName = parts[0][len("/skill:"):]
					skillArgs = rest
				}
				out = append(out, slashMention{Kind: "skill", Name: skillName, Args: skillArgs})
			}
		}
	}

	// Also extract inline /skill:name or /mcp:server tokens throughout the text (including quoted: /skill:"name with spaces")
	reQuoted := regexp.MustCompile(`(?:^|\s)/(skill|mcp):["']([^"']+)["']`)
	for _, m := range reQuoted.FindAllStringSubmatch(text, -1) {
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
		baseTool := t.Name
		if strings.HasPrefix(baseTool, t.ServerName+"/") {
			baseTool = strings.TrimPrefix(baseTool, t.ServerName+"/")
		}
		cleanServer := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(server)), "-", "_")
		tServer := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(t.ServerName)), "-", "_")
		if !strings.EqualFold(t.ServerName, server) && tServer != cleanServer {
			continue
		}
		if toolName != "" && !strings.EqualFold(baseTool, toolName) && !strings.EqualFold(t.Name, toolName) {
			continue
		}
		desc := t.Description
		if len(desc) > 120 {
			desc = desc[:120]
		}
		toolList = append(toolList, fmt.Sprintf("- %s (%s) — %s", baseTool, t.ServerName, desc))
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
