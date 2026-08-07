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
	"time"

	"github.com/google/uuid"
	"github.com/hasdev/forge-ade/internal/events"
	"github.com/hasdev/forge-ade/internal/llm"
	"github.com/hasdev/forge-ade/internal/mcp"
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

type TaskItem struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Completed bool   `json:"completed"`
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
	StateIdle           SessionState = "idle"
	StateThinking       SessionState = "thinking"
	StateExecuting      SessionState = "executing"
	StateAwaiting       SessionState = "awaiting_approval"
	StateAwaitingInput  SessionState = "awaiting_input"
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
	Progress     SessionProgress `json:"progress,omitempty"`
	TokenUsage   llm.TokenStats `json:"token_usage"`
	AutoApprove  bool           `json:"auto_approve"`
	PendingTools []ContentBlock `json:"pending_tools,omitempty"`
	PendingQuestions []tools.AskQuestion `json:"pending_questions,omitempty"`
	Dialect      string         `json:"dialect,omitempty"` // "" = native tool calling, "xml" = in-band
	SystemPrompt string         `json:"system_prompt,omitempty"`
	CustomPrompt string         `json:"custom_prompt,omitempty"`
	CustomRules  string         `json:"custom_rules,omitempty"`
	CreatedAt    time.Time      `json:"created_at,omitempty"`
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
	termMgr     *terminal.Manager
	shells      map[string]string // agent session id → persistent shell id
	bus         *events.Bus
	cancelFuncs map[string]context.CancelFunc
	storePath   string

	// Coalesced persistence — a turn can do dozens of tool calls; rewriting
	// the whole sessions JSON on every one is wasteful.
	persistMu    sync.Mutex
	persistTimer *time.Timer
	persistDirty bool
}

func NewManager(llmClient *llm.LLMClient, toolReg *tools.Registry, skillMgr *skills.Manager, mcpMgr *mcp.Manager, termMgr *terminal.Manager, bus *events.Bus, dataDir string) *Manager {
	storePath := filepath.Join(dataDir, "agent_sessions.json")
	m := &Manager{
		sessions:    make(map[string]*Session),
		definitions: make(map[string]AgentDefinition),
		llmClient:   llmClient,
		toolReg:     toolReg,
		skillMgr:    skillMgr,
		mcpMgr:      mcpMgr,
		termMgr:     termMgr,
		shells:      make(map[string]string),
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

	// Current layout: one file per session under sessions/ (lazy-loadable,
	// crash-safe, never rewrites unrelated sessions).
	dir := filepath.Join(filepath.Dir(m.storePath), "sessions")
	entries, err := os.ReadDir(dir)
	if err == nil && len(entries) > 0 {
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			data, rerr := os.ReadFile(filepath.Join(dir, e.Name()))
			if rerr != nil {
				continue
			}
			var s Session
			if json.Unmarshal(data, &s) != nil || s.ID == "" {
				continue
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
	// Prune files whose sessions were deleted.
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".json")
		if _, ok := m.sessions[id]; !ok {
			_ = os.Remove(filepath.Join(dir, e.Name()))
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

	now := time.Now()
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
	if name, args, ok := parseSkillInvocation(fullContent); ok && m.skillMgr != nil {
		if skill, found := m.skillMgr.Get(name); found {
			messages = append(messages, AgentMessage{
				ID:   uuid.New().String(),
				Role: "user",
				Content: []ContentBlock{
					{Type: "text", Text: skill.InvocationMessage(args)},
				},
				Timestamp: time.Now(),
			})
		}
	}

	sess.Messages = append(sess.Messages, messages...)
	sess.State = StateThinking

	// Track the current objective (first line of the message) so the agent's
	// system prompt states it explicitly instead of inferring it from history.
	if goal := firstLine(userContent); goal != "" {
		sess.Progress.CurrentGoal = goal
	}

	// Auto-name the session (ChatGPT-style short title) on the first real user
	// message, using the LLM to summarize it. The frontend creates sessions
	// with a bare default name ("Agent", "Agent 2", ...) or the backend's
	// "Coding Agent (hh:mm:ss)" — any of these placeholders triggers auto-title.
	shouldAutoTitle := sess.Name == "" ||
		strings.HasPrefix(sess.Name, "Agent") ||
		strings.HasPrefix(sess.Name, "Coding Agent (")
	m.saveSessionsLocked()
	m.mu.Unlock()

	// EMIT IMMEDIATELY SO USER MESSAGE APPEARS INSTANTLY IN REAL-TIME
	m.emitSessionUpdate(sessionID)

	if shouldAutoTitle && m.llmClient != nil {
		go m.autoTitleSession(sessionID, fullContent)
	}

	// Launch background execution loop for agent turn
	execCtx, cancel := context.WithCancel(context.Background())
	m.mu.Lock()
	m.cancelFuncs[sessionID] = cancel
	m.mu.Unlock()

	go m.runAgentTurn(execCtx, sessionID)

	return nil
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
	// Project/skills context is stable for the whole turn — building it per
	// iteration re-runs a git snapshot (seconds) every loop. Build once.
	m.mu.RLock()
	folder := ""
	if sess, ok := m.sessions[sessionID]; ok {
		folder = sess.Folder
	}
	m.mu.RUnlock()
	projectCtx := ""
	if folder != "" {
		projectCtx = buildProjectContext(folder)
	}
	skillsCtx := m.buildSkillsContext()

	budgetSpent := 0
	for iter := 0; iter < maxReasoningSteps; iter++ {
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

		// Build LLM messages payload from block-based messages, with proper
		// tool_call_id correlation (the old loop dropped the ID on tool
		// results, which most providers reject). The transcript is windowed
		// so long sessions don't bloat the context; the stored session keeps
		// the full history for the UI.
		history := windowTranscript(sess.Messages, 40)
		sysContent := sess.SystemPrompt
		sysContent += projectCtx
		sysContent += skillsCtx
		if len(history) < len(sess.Messages) {
			sysContent += "\n\n(Note: older conversation history was truncated from this request to fit the context window. The original task and recent exchanges are preserved.)"
		}
		if remaining := maxToolBudget - budgetSpent; remaining > 0 {
			sysContent += fmt.Sprintf("\n\nTool budget remaining: %d points (cheap=1, medium=3, high=10). Prefer batched cheap tools (read_multiple, search) over many tiny calls; bash is expensive.", remaining)
		} else {
			sysContent += "\n\nTool budget exhausted — wrap up with a summary; no more tool calls."
		}
		if p := sess.Progress; p.CurrentGoal != "" || len(p.ActiveTodos) > 0 {
			sysContent += "\n\n## Current objective\n"
			if p.CurrentGoal != "" {
				sysContent += "Goal: " + p.CurrentGoal + "\n"
			}
			if len(p.CompletedSteps) > 0 {
				sysContent += "Completed: " + strings.Join(p.CompletedSteps, ", ") + "\n"
			}
			if len(p.ActiveTodos) > 0 {
				sysContent += "Todos: " + strings.Join(p.ActiveTodos, ", ") + "\n"
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
		// registry; dialect mode will strip these and use in-band text calls).
		toolDefs := m.toolReg.Definitions()

		m.mu.RLock()
		dialect := sess.Dialect
		m.mu.RUnlock()

		var dialectScanner *xmlDialectScanner
		if dialect == "xml" {
			// In-band mode: inject the dialect prompt + tool catalog, re-encode
			// the transcript as XML text, and send NO native tools.
			sysContent += "\n" + xmlDialectPrompt + toolCatalogText(toolDefs)
			messages = []llm.LLMMessage{
				{Role: llm.RoleSystem, Content: sysContent},
			}
			messages = append(messages, llm.LLMMessage{
				Role:    llm.RoleUser,
				Content: renderDialectTranscript(history, toolCatalogText(toolDefs)),
			})
			toolDefs = nil
			dialectScanner = newXMLDialectScanner()
		} else {
			messages = append(messages, m.messagesToLLM(history)...)
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
					m.emitThinkingDelta(sessionID, deltaReasoning)
				}
				if deltaContent != "" {
					if dialectScanner != nil {
						// Parse the stream for in-band tool calls.
						for _, ev := range dialectScanner.feed(deltaContent) {
							switch ev.kind {
							case "thinking":
								assistant.appendThinking(ev.text)
								m.emitThinkingDelta(sessionID, ev.text)
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
								m.emitToolDelta(sessionID, len(toolCallOrder)-1, ev.toolName, "")
							case "tool_end":
								if idx, ok := dialectToolIndex(toolCallBuf, ev.toolID); ok {
									tc := toolCallBuf[idx]
									tc.Function.Arguments = jsonStringMap(ev.args)
									m.emitToolDelta(sessionID, idx, ev.toolName, tc.Function.Arguments)
								}
							case "text":
								assistant.appendText(ev.text)
								m.emitMessageDelta(sessionID, "text", ev.text)
							}
						}
					} else {
						assistant.appendText(deltaContent)
						m.emitMessageDelta(sessionID, "text", deltaContent)
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
				if dialectScanner != nil {
					return // in-band mode has no native tool-call fragments
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
				m.emitToolDelta(sessionID, delta.Index, tc.Function.Name, tc.Function.Arguments)
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
						m.emitToolDelta(sessionID, idx, ev.toolName, tc.Function.Arguments)
					}
				case "thinking":
					assistant.appendThinking(ev.text)
					m.emitThinkingDelta(sessionID, ev.text)
				case "text":
					assistant.appendText(ev.text)
					m.emitMessageDelta(sessionID, "text", ev.text)
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
		m.mu.Unlock()
		m.emitMessageEnd(sessionID, assistant)

		toolCalls := assistant.ToolCallBlocks()

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

		// Execute the batch (sequential), then loop again.
		if len(toolCalls) > 0 {
			m.mu.Lock()
			sess.State = StateExecuting
			m.scheduleSave()
			m.mu.Unlock()
			m.emitSessionUpdate(sessionID)

			allExecuted := true
			// Dedup: identical read-only calls in one batch run once and the
			// result is mirrored to every caller id (each id still gets its own
			// tool_result — providers reject missing results).
			type dedupEntry struct {
				content string
				isErr   bool
			}
			dedup := make(map[string]dedupEntry)
			for _, tc := range toolCalls {
				if isReadOnlyTool(tc.Name) {
					key := tc.Name + "\x00" + canonicalArgs(tc.Arguments)
					if ent, ok := dedup[key]; ok {
						m.emitToolEnd(sessionID, tc, ent.content, ent.isErr)
						if err := m.appendToolResult(sessionID, tc, ent.content, ent.isErr); err != nil {
							allExecuted = false
						}
						budgetSpent += m.toolCost(tc.Name)
						continue
					}
				}
				content, execErr := m.executeToolCall(ctx, sessionID, tc)
				if execErr != nil {
					allExecuted = false
				}
				if isReadOnlyTool(tc.Name) {
					dedup[tc.Name+"\x00"+canonicalArgs(tc.Arguments)] = dedupEntry{content: content, isErr: execErr != nil}
				}
				budgetSpent += m.toolCost(tc.Name)
			}
			if !allExecuted {
				// A tool failed fatally; surface it and stop rather than
				// feeding the error back into an infinite loop.
				m.finishTurn(sessionID, "One or more tool calls failed.")
				return
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
	m.mu.Unlock()
	m.emitSessionUpdate(sessionID)

	// Execute the whole previously-pending batch, then continue the loop.
	go func() {
		for _, tc := range pending {
			_, _ = m.executeToolCall(ctx, sessionID, tc)
		}
		m.runAgentTurn(ctx, sessionID)
	}()
	return nil
}

// executeToolCall runs one tool call and appends its result as a tool_result
// block linked to the original tool_call_id. Returns an error when the tool
// itself fails.
func (m *Manager) executeToolCall(ctx context.Context, sessionID string, block ContentBlock) (string, error) {
	// Attach the session bridge so `todo` / `ask` tools can touch session state.
	toolCtx := tools.WithSessionBridge(ctx, m.sessionBridge(sessionID))

	res, err := m.toolReg.Execute(toolCtx, block.Name, jsonStringMap(block.Arguments))
	var contentStr string
	isErr := false
	if err != nil {
		contentStr = fmt.Sprintf("Tool Error: %v", err)
		isErr = true
	} else {
		b, _ := json.MarshalIndent(res, "", "  ")
		contentStr = string(b)
	}

	m.emitToolEnd(sessionID, block, contentStr, isErr)
	if err := m.appendToolResult(sessionID, block, contentStr, isErr); err != nil {
		return contentStr, err
	}
	return contentStr, nil
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
	sess.Messages = append(sess.Messages, AgentMessage{
		ID:   uuid.New().String(),
		Role: "tool",
		Content: []ContentBlock{
			{
				Type:       "tool_result",
				ToolCallID: block.ToolCallID,
				Name:       block.Name,
				Text:       contentStr,
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
	if ok {
		sess.State = StateIdle
		sess.UpdatedAt = time.Now()
		if errMsg != "" {
			sess.Messages = append(sess.Messages, AgentMessage{
				ID:   uuid.New().String(),
				Role: "assistant",
				Content: []ContentBlock{
					{Type: "text", Text: errMsg},
				},
				Timestamp: time.Now(),
			})
		}
		m.saveSessionsLocked()
	}
	m.mu.Unlock()
	m.emitSessionUpdate(sessionID)
	m.emitAgentEvent(events.AgentTurnEnd, sessionID, map[string]interface{}{
		"error": errMsg,
	})
	if errMsg != "" {
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
		m.saveSessionsLocked()
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
}

func (m *Manager) sessionBridge(sessionID string) tools.SessionBridge {
	return &sessionBridge{m: m, sessionID: sessionID}
}

// TerminalExec runs a command in the agent session's persistent shell (created
// lazily, one per agent session) so cwd, env, and shell state survive across
// commands — the biggest "real agent" win over spawning a shell per call.
func (b *sessionBridge) TerminalExec(command string, timeout time.Duration) (string, int, error) {
	return b.m.agentShellExec(b.sessionID, command, timeout)
}

// agentShellExec resolves (or lazily creates) the persistent shell for an
// agent session and runs a command in it.
func (m *Manager) agentShellExec(sessionID, command string, timeout time.Duration) (string, int, error) {
	if m.termMgr == nil {
		return "", 1, fmt.Errorf("terminal manager unavailable")
	}
	m.mu.RLock()
	shellID := m.shells[sessionID]
	folder := ""
	if sess, ok := m.sessions[sessionID]; ok {
		folder = sess.Folder
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
	out, code, err := m.termMgr.Exec(shellID, command, timeout)
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
		status := "pending"
		if t.Completed {
			status = "completed"
		}
		out = append(out, tools.TodoItem{ID: t.ID, Title: t.Title, Status: status})
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
	sess.Tasks = sess.Tasks[:0]
	for _, it := range items {
		sess.Tasks = append(sess.Tasks, TaskItem{
			ID:        it.ID,
			Title:     it.Title,
			Completed: it.Status == "completed",
		})
	}
	// Mirror the task list into compact progress state (todo status feeds the
	// "Current objective" section of the system prompt).
	var active, done []string
	for _, t := range sess.Tasks {
		if t.Completed {
			done = append(done, t.Title)
		} else {
			active = append(active, t.Title)
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
	sess.Messages = append(sess.Messages, AgentMessage{
		ID:   uuid.New().String(),
		Role: "tool",
		Content: []ContentBlock{
			{Type: "tool_result", ToolCallID: "ask", Name: "ask", Text: string(b), IsError: false},
		},
		Timestamp: time.Now(),
	})
	sess.State = StateThinking
	sess.UpdatedAt = time.Now()
	m.saveSessionsLocked()
	m.mu.Unlock()
	m.emitSessionUpdate(sessionID)
	go m.runAgentTurn(context.Background(), sessionID)
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
	for _, msg := range msgs {
		switch msg.Role {
		case "assistant":
			var toolCalls []llm.ToolCall
			for _, b := range msg.ToolCallBlocks() {
				toolCalls = append(toolCalls, llm.ToolCall{
					ID:   b.ToolCallID,
					Type: "function",
					Function: llm.ToolFunction{
						Name:      b.Name,
						Arguments: jsonStringMap(b.Arguments),
					},
				})
			}
			text := msg.Text()
			if text == "" && len(toolCalls) > 0 {
				// Anthropic-style providers require content to be present; use
				// a minimal non-empty string so the tool_calls ride along.
				text = " "
			}
			out = append(out, llm.LLMMessage{
				Role:      llm.RoleAssistant,
				Content:   text,
				ToolCalls: toolCalls,
			})
		case "tool":
			for _, b := range msg.ToolResultBlocks() {
				out = append(out, llm.LLMMessage{
					Role:       llm.RoleTool,
					Content:    b.Text,
					ToolCallID: b.ToolCallID,
					Name:       b.Name,
				})
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

func (m *Manager) emitMessageDelta(sessionID string, kind string, delta string) {
	m.emitAgentEvent(events.AgentMessageDelta, sessionID, map[string]interface{}{
		"kind":  kind,
		"delta": delta,
	})
}

func (m *Manager) emitMessageEnd(sessionID string, msg *AgentMessage) {
	m.emitAgentEvent(events.AgentMessageEnd, sessionID, map[string]interface{}{
		"message_id": msg.ID,
		"role":       msg.Role,
	})
}

func (m *Manager) emitThinkingDelta(sessionID string, delta string) {
	m.emitAgentEvent(events.AgentThinkingDelta, sessionID, map[string]interface{}{
		"delta": delta,
	})
}

func (m *Manager) emitToolDelta(sessionID string, index int, name string, args string) {
	m.emitAgentEvent(events.AgentToolDelta, sessionID, map[string]interface{}{
		"index": index,
		"name":  name,
		"args":  args,
	})
}

func (m *Manager) emitToolEnd(sessionID string, block ContentBlock, result string, isErr bool) {
	m.emitAgentEvent(events.AgentToolEnd, sessionID, map[string]interface{}{
		"tool_call_id": block.ToolCallID,
		"name":         block.Name,
		"args":         block.Arguments,
		"result":       result,
		"is_error":     isErr,
	})
}

// StopTurn cancels the currently running agent turn for a session (port of
// cancels the running turn).
func (m *Manager) StopTurn(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cancel, ok := m.cancelFuncs[sessionID]; ok {
		cancel()
	}
}

func isMutatingTool(name string) bool {
	switch name {
	case "write", "write_file", "create_file", "edit", "edit_file", "bash", "run_shell", "exec", "run_command":
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
