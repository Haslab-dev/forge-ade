package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Haslab-dev/RakitKode/llm"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/hasdev/forge-ade/app/native/pkg/engine"
	"github.com/hasdev/forge-ade/app/native/pkg/theme"
	"github.com/hasdev/forge-ade/core/acp"
	"github.com/hasdev/forge-ade/core/agentsession"
	internalgit "github.com/hasdev/forge-ade/core/git"
)

// goSave runs fn on a new goroutine, converting a panic into a log so a
// background persistence failure never kills the process (P1-6).
func goSave(name string, fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[agent] %s panicked: %v\n%s", name, r, debugStack())
			}
		}()
		fn()
	}()
}

// nextMessageIDLocked returns a per-session monotonically increasing message
// ID. Deriving IDs from len(Messages)+1 reused IDs after /clear (messages
// are niled, the transcript restarts at 1) and collided with restored
// transcripts — snapshot caches keyed by ID went stale. Callers hold as.mu.
func nextMessageIDLocked(s *AgentSession) int {
	if s.nextMsgID <= 0 {
		s.nextMsgID = len(s.Messages) + 1
	}
	id := s.nextMsgID
	if id <= len(s.Messages) {
		// Defensive: never collide with anything already in the transcript.
		id = len(s.Messages) + 1
	}
	s.nextMsgID = id + 1
	return id
}

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
	ParsedArgs  any    `json:"-"`
}

// FileChange represents a touched file with added and deleted lines.
type FileChange struct {
	Path      string `json:"path"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
}

// PendingApproval represents a tool call awaiting user confirmation.
type PendingApproval struct {
	SessionID  string `json:"sessionId,omitempty"`
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

// PermissionMode maps the UI autonomy level onto forge-ade's
// SessionOpts.PermissionMode values.
func (m ExecutionMode) PermissionMode() string {
	switch m {
	case ModeBypass:
		return "full"
	case ModeCode:
		return "auto"
	case ModePlan:
		return "auto_edits"
	default:
		return "supervised"
	}
}

// InternalPermissionMode maps ExecutionMode onto RakitKode permission modes
// ("yolo", "edit", "plan", "build").
func (m ExecutionMode) InternalPermissionMode() string {
	switch m {
	case ModeBypass:
		return "yolo"
	case ModeCode:
		return "edit"
	case ModePlan:
		return "plan"
	default:
		return "build"
	}
}

func isInternalLLMConfigured(cfg llm.Profile) bool {
	if strings.TrimSpace(cfg.APIKey) != "" {
		return true
	}
	base := strings.TrimSpace(cfg.BaseURL)
	if base == "" {
		return false
	}
	return strings.EqualFold(cfg.ProviderID, "ollama") ||
		strings.EqualFold(cfg.ProviderID, "lmstudio") ||
		strings.Contains(base, "localhost") ||
		strings.Contains(base, "127.0.0.1") ||
		(!strings.Contains(base, "api.openai.com") &&
			!strings.Contains(base, "anthropic.com") &&
			!strings.Contains(base, "generativelanguage.googleapis.com") &&
			!strings.Contains(base, "api.deepseek.com"))
}

func hasConfiguredInternalLLM(eng *engine.Engine) bool {
	if eng == nil || eng.LLMClient == nil {
		return false
	}
	return isInternalLLMConfigured(eng.LLMClient.GetConfig())
}

func isACPAgentID(agentID, agentType string) bool {
	if agentType == "acp" {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(agentID)) {
	case "agent-agy", "agy",
		"agent-claude", "claude-code",
		"agent-codex", "codex",
		"agent-opencode", "opencode",
		"agent-commandcode", "commandcode",
		"agent-pi", "pi",
		"agent-ohmypi", "ohmypi",
		"agent-gemini", "gemini",
		"agent-kimi", "kimi",
		"agent-qwen", "qwen":
		return true
	default:
		return false
	}
}

// ReasoningEffort represents the thinking depth budget.
type ReasoningEffort string

const (
	EffortOff    ReasoningEffort = "off"
	EffortLow    ReasoningEffort = "low"
	EffortMedium ReasoningEffort = "medium"
	EffortHigh   ReasoningEffort = "high"
	EffortMax    ReasoningEffort = "max"
)

// MessageBlock is one element of an assistant turn's ordered timeline: a
// streamed text segment, thinking segment, or tool call, in the order the
// agent produced them. Blocks reference ToolExecution entries by ID; msg.Text
// remains the concatenation of all text segments for copy/search/title consumers.
type MessageBlock struct {
	Kind   string `json:"kind"` // "text" | "tool" | "thinking"
	Text   string `json:"text,omitempty"`
	ToolID string `json:"toolId,omitempty"`
	// Started/ended wall-clock stamps ("Thought 4.2s" labels). Zero values on
	// legacy sessions fall back to the message-level ThinkingTimeSec.
	StartedAtMs int64 `json:"startedAtMs,omitempty"`
	EndedAtMs   int64 `json:"endedAtMs,omitempty"`
}

// stampBlockTimesLocked closes the timeline's last open block (EndedAtMs) and
// stamps the new block's start, so per-block reasoning durations render like
// ZCode's "Thought 4.2s" rows.
func stampBlockTimesLocked(msg *ChatMessage, blk *MessageBlock) {
	now := time.Now().UnixMilli()
	if n := len(msg.Blocks); n > 0 {
		if prev := msg.Blocks[n-1]; prev != nil && prev.EndedAtMs == 0 {
			prev.EndedAtMs = now
		}
	}
	if blk != nil && blk.StartedAtMs == 0 {
		blk.StartedAtMs = now
	}
}

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
	// PendingQuestions carries a structured agent:ask (ask tool) with option
	// chips and custom answers — PendingQuestionsCard / AgentQuestionModal.
	PendingQuestions []AgentQuestion
	// Blocks is the ordered text/tool timeline for this turn. Nil for
	// sessions loaded from older disk formats — the UI falls back to the
	// legacy fixed (tools-then-text) layout when Blocks is empty.
	Blocks []*MessageBlock `json:"blocks,omitempty"`
	// SkipHarnessDumps drops further text deltas once the stream starts
	// emitting a harness protocol-state JSON blob (active_skills /
	// active_system_prompts / plugin catalogs) that some ACP CLIs print to
	// stdout — forge-ade parses these in-band; native must not show them.
	SkipHarnessDumps bool         `json:"-"`
	TotalSteps       int          // steps reported by agent:turn_end
	Feedback         string       // "up" or "down" (user message feedback)
	TurnStartedAt    time.Time    `json:"-"` // live elapsed while streaming
	cachedSnapshot   *ChatMessage `json:"-"`
}

// appendTextBlockDeltaLocked appends a streamed delta to the timeline's
// last text block, creating one when the previous block is not a text block.
func appendTextBlockDeltaLocked(msg *ChatMessage, delta string) {
	if n := len(msg.Blocks); n > 0 && msg.Blocks[n-1] != nil && msg.Blocks[n-1].Kind == "text" {
		msg.Blocks[n-1].Text += delta
		return
	}
	blk := &MessageBlock{Kind: "text", Text: delta}
	stampBlockTimesLocked(msg, blk)
	msg.Blocks = append(msg.Blocks, blk)
}

// appendThinkingBlockDeltaLocked appends a streamed reasoning delta to the
// timeline's last thinking block, creating a new block whenever a tool or
// text block has intervened.
func appendThinkingBlockDeltaLocked(msg *ChatMessage, delta string) {
	if n := len(msg.Blocks); n > 0 && msg.Blocks[n-1] != nil && msg.Blocks[n-1].Kind == "thinking" {
		msg.Blocks[n-1].Text += delta
		return
	}
	blk := &MessageBlock{Kind: "thinking", Text: delta}
	stampBlockTimesLocked(msg, blk)
	msg.Blocks = append(msg.Blocks, blk)
}

// appendToolBlockLocked records a new tool call in the timeline. Called only
// when a ToolExecution is created — tool_end updates must not add blocks.
func appendToolBlockLocked(msg *ChatMessage, toolID string) {
	blk := &MessageBlock{Kind: "tool", ToolID: toolID}
	stampBlockTimesLocked(msg, blk)
	msg.Blocks = append(msg.Blocks, blk)
}

// findToolByID returns the ToolExecution with the given ID, or nil.
func findToolByID(msg *ChatMessage, id string) *ToolExecution {
	for _, t := range msg.Tools {
		if t != nil && t.ID == id {
			return t
		}
	}
	return nil
}

// setAssistantTextLocked replaces a message's text wholesale (error
// suffixes, stop markers) and rebuilds its timeline to match.
func setAssistantTextLocked(msg *ChatMessage, text string) {
	msg.Text = text
	if strings.TrimSpace(text) == "" {
		msg.Blocks = nil
		return
	}
	msg.Blocks = []*MessageBlock{{Kind: "text", Text: text}}
}

// CompactActiveSession truncates the active session's transcript to the
// first message plus the last three, under the state lock, and persists the
// result. Returns the number of removed messages.
func (as *AgentState) CompactActiveSession() (int, error) {
	as.mu.Lock()
	sess := as.ActiveSession()
	if sess == nil {
		as.mu.Unlock()
		return 0, fmt.Errorf("no active session")
	}
	if len(sess.Messages) <= 4 {
		as.mu.Unlock()
		return 0, fmt.Errorf("conversation is already within compact bounds")
	}
	removed := len(sess.Messages) - 4
	compacted := make([]*ChatMessage, 0, 4)
	compacted = append(compacted, sess.Messages[0])
	compacted = append(compacted, sess.Messages[len(sess.Messages)-3:]...)
	sess.Messages = compacted
	if as.ActiveSessionID == sess.ID {
		as.Messages = sess.Messages
	}
	sess.UpdatedAt = time.Now()
	as.mu.Unlock()

	as.SaveSessionToDisk(sess)
	return removed, nil
}

// AgentSession represents a single task session in the ForgeADE agent workspace.
type AgentSession struct {
	ID                   string
	Title                string
	Status               string // "running", "idle", "completed"
	CreatedAt            time.Time
	UpdatedAt            time.Time
	Messages             []*ChatMessage
	Tasks                *TaskManager
	Model                string
	Provider             string
	AgentID              string
	AgentType            string
	PersonaID            string `json:"personaId,omitempty"`
	appliedPersonaKey    string `json:"-"`
	WorkspacePath        string
	Pinned               bool
	Archived             bool
	Snoozed              bool            `json:"snoozed,omitempty"`
	RemoteSessionID      string          `json:"remoteSessionId,omitempty"`      // agent conversation id
	LastCheckpointTurnID string          `json:"lastCheckpointTurnId,omitempty"` // pre-turn snapshot id
	AvailableCommands    []string        `json:"availableCommands,omitempty"`
	LastUsage            *UsageInfo      `json:"lastUsage,omitempty"`
	SteerQueue           []string        `json:"-"`
	turnStartFiles       map[string]bool // git snapshot at turn start
	nextMsgID            int             // per-session monotonic message ID; /clear does NOT reset it (P2f)

	// PendingApprovals queues unanswered permission requests so a second
	// agent:ask never hides the first one. The streaming message's
	// PendingApproval always points at the earliest unresolved entry;
	// ResolveSessionApproval pops entries as they are answered.
	PendingApprovals []*PermissionRequest `json:"-"`
}

// AgentState tracks sessions, conversation history, streaming responses, and prompt composer.
type AgentState struct {
	mu              sync.Mutex
	deletedIDs      map[string]bool
	engine          *engine.Engine
	workspacePath   string
	Sessions        []*AgentSession
	ActiveSessionID string
	ActiveFilterTab string // "active", "pinned", "snoozed", "archived"
	SearchQuery     string
	InputText       string
	ListState       ui.ListState
	MdOpts          markdown.Options
	ActiveModel     string
	ActiveProvider  string
	ActiveAgentID   string
	ActiveAgentType string
	ActivePersonaID string
	ExecutionMode   ExecutionMode
	// FetchingModels guards the live /models fetch for the model picker.
	FetchingModels  bool
	ReasoningEffort ReasoningEffort
	SteerQueue      []string
	AttachedFiles   []string
	StashedPrompts  []string

	// Prompt history (up/down at the composer), persisted like forge-ade's
	// promptHistoryStore (last 100 prompts).
	PromptHistory      []string
	PromptHistoryIdx   int    // -1 = not browsing
	PromptHistoryDraft string // in-progress text saved when history starts
	AvailableModels    []string
	NextID             int
	IsStreaming        bool
	TaskManager        *TaskManager
	Theme              *theme.ThemeManager
	ACPMgr             *ACPManager
	Messages           []*ChatMessage // alias for active session messages
	Invalidate         func()
	lastInvalidate     time.Time
	invalidatePending  bool
	invalidateMu       sync.Mutex

	// streamCancels holds the context cancel funcs of in-flight raw-LLM
	// fallback streams per session, so CancelStreaming can abort them (the
	// stream context is local to the turn goroutine).
	streamCancels      map[string]*streamCancelHandle
	internalSessionMap sync.Map // remoteManagerID -> nativeSessionID for lock-free bus routing

	// pendingLLMProvider/pendingLLMModel carry a model binding recorded by
	// syncActiveSession (under as.mu) so applyPendingLLMModel can push it to
	// the RK LLMClient after the caller releases as.mu (P0-1 layer 3).
	pendingLLMProvider string
	pendingLLMModel    string

	// acpTransportProbe lets tests fake the live ACP transport/running-state
	// lookup used by the steer path; nil uses the real ACP manager sessions.
	acpTransportProbe func(sessionID string) (protocol string, running bool)
}

// NewAgentState initializes the native agent chat state by loading persistent sessions from disk.
func NewAgentState(engine *engine.Engine, workspacePath string, invalidate func()) *AgentState {
	// Detect default agent harness: prefer installed ACP agent (e.g. agy, claude, codex)
	// when no internal LLM provider keys are configured.
	defaultAgentID := "agent-agy"
	defaultAgentType := "acp"
	defaultModel := "gemini-3.8-flash-high"

	hasInternalKey := hasConfiguredInternalLLM(engine)

	if !hasInternalKey {
		if _, err := lookPathEnriched("agy"); err == nil {
			defaultAgentID = "agent-agy"
			defaultAgentType = "acp"
			defaultModel = "gemini-3.8-flash-high"
		} else if _, err := lookPathEnriched("claude"); err == nil {
			defaultAgentID = "agent-claude"
			defaultAgentType = "claude"
			defaultModel = "sonnet"
		} else if _, err := lookPathEnriched("codex"); err == nil {
			defaultAgentID = "agent-codex"
			defaultAgentType = "codex"
			defaultModel = "deepseek-v4-1-flash"
		}
	} else {
		defaultAgentID = "agent-internal"
		defaultAgentType = "internal"
		if engine != nil && engine.LLMClient != nil {
			cfg := engine.LLMClient.GetConfig()
			if cfg.Model != "" {
				defaultModel = cfg.Model
			}
		}
	}

	normWs := agentsession.NormalizeWorkspacePath(workspacePath)

	as := &AgentState{
		engine:          engine,
		workspacePath:   normWs,
		ActiveAgentID:   defaultAgentID,
		ActiveAgentType: defaultAgentType,
		ActiveModel:     defaultModel,
		ActiveFilterTab: "active",
		ExecutionMode:   ModeBypass,
		ReasoningEffort: EffortMedium,
		SteerQueue:      make([]string, 0),
		AvailableModels: []string{},
		streamCancels:   make(map[string]*streamCancelHandle),
		Invalidate:      invalidate,
		NextID:          1,
		MdOpts: markdown.Options{
			Reveal: &markdown.Reveal{
				Unit: markdown.AtOnce,
				Fade: -1,
			},
		},
	}

	// Load real sessions from ~/.forge/sessions/ and ~/.forge-ade/sessions/
	as.LoadSessionsFromDisk(normWs)
	as.LoadPromptHistory()
	as.LoadPromptStash()
	as.LoadSteerQueues()

	return as
}

// streamCancelHandle carries the context.CancelFunc of an in-flight raw-LLM
// fallback stream. It is heap-allocated so CancelStreaming can find and call
// it under as.mu (func values are not comparable, pointers are).
type streamCancelHandle struct {
	cancel context.CancelFunc
}

func (as *AgentState) setStreamCancel(sessionID string, h *streamCancelHandle) {
	as.mu.Lock()
	if as.streamCancels == nil {
		as.streamCancels = make(map[string]*streamCancelHandle)
	}
	as.streamCancels[sessionID] = h
	as.mu.Unlock()
}

func (as *AgentState) clearStreamCancel(sessionID string, h *streamCancelHandle) {
	as.mu.Lock()
	if as.streamCancels != nil && as.streamCancels[sessionID] == h {
		delete(as.streamCancels, sessionID)
	}
	as.mu.Unlock()
}

// cancelStreamFor aborts an in-flight raw-LLM fallback stream of sessionID
// (no-op when none is running). Safe from any goroutine.
func (as *AgentState) cancelStreamFor(sessionID string) {
	as.mu.Lock()
	h := as.streamCancels[sessionID]
	as.mu.Unlock()
	if h != nil && h.cancel != nil {
		h.cancel()
	}
}

// snapshotMessageLocked returns a render-safe copy of m. Writers mutate
// ChatMessage fields in place under as.mu (msg.Text += delta), and tool
// structs in place on tool events, so the render must draw from per-message
// copies taken under the same lock.
func snapshotMessageLocked(m *ChatMessage) *ChatMessage {
	if m == nil {
		return nil
	}
	if m.Done && m.cachedSnapshot != nil {
		return m.cachedSnapshot
	}
	cp := *m
	if len(m.Tools) > 0 {
		cp.Tools = make([]*ToolExecution, 0, len(m.Tools))
		for _, t := range m.Tools {
			if t == nil {
				continue
			}
			tcp := *t
			cp.Tools = append(cp.Tools, &tcp)
		}
	}
	if len(m.Blocks) > 0 {
		cp.Blocks = make([]*MessageBlock, 0, len(m.Blocks))
		for _, b := range m.Blocks {
			if b == nil {
				continue
			}
			bcp := *b
			cp.Blocks = append(cp.Blocks, &bcp)
		}
	}
	if len(m.Changes) > 0 {
		cp.Changes = make([]*FileChange, 0, len(m.Changes))
		for _, ch := range m.Changes {
			if ch == nil {
				continue
			}
			ccp := *ch
			cp.Changes = append(cp.Changes, &ccp)
		}
	}
	if len(m.PendingQuestions) > 0 {
		cp.PendingQuestions = make([]AgentQuestion, len(m.PendingQuestions))
		for j, q := range m.PendingQuestions {
			qc := q
			qc.Options = append([]string(nil), q.Options...)
			qc.Selected = append([]string(nil), q.Selected...)
			cp.PendingQuestions[j] = qc
		}
	}
	if m.PendingApproval != nil {
		pap := *m.PendingApproval
		cp.PendingApproval = &pap
	}
	if m.Done {
		m.cachedSnapshot = &cp
	}
	return &cp
}

// MessagesSnapshot returns a per-frame consistent copy of the active
// session's transcript: slice header plus per-message struct copies taken
// under the state lock. Render code must never read the live message
// pointers while turn goroutines stream.
func (as *AgentState) MessagesSnapshot() []*ChatMessage {
	as.mu.Lock()
	defer as.mu.Unlock()
	out := make([]*ChatMessage, len(as.Messages))
	for i, m := range as.Messages {
		out[i] = snapshotMessageLocked(m)
	}
	return out
}

// ActiveTranscriptSnapshot returns the transcript snapshot plus the session
// identity fields the chat surface renders (one lock acquisition per frame).
func (as *AgentState) ActiveTranscriptSnapshot() (msgs []*ChatMessage, sessionID, checkpointTurnID string) {
	as.mu.Lock()
	defer as.mu.Unlock()
	sessionID = as.ActiveSessionID
	for _, s := range as.Sessions {
		if s.ID != sessionID {
			continue
		}
		checkpointTurnID = s.LastCheckpointTurnID
		msgs = make([]*ChatMessage, len(s.Messages))
		for i, m := range s.Messages {
			msgs[i] = snapshotMessageLocked(m)
		}
		break
	}
	return
}

// TranscriptVersion returns a cheap change key of the active transcript
// (message count + total text/thinking/tool bytes) so content-dependent UI
// such as find-in-thread re-computes while a turn streams.
func (as *AgentState) TranscriptVersion() int {
	as.mu.Lock()
	defer as.mu.Unlock()
	v := len(as.Messages)
	for _, m := range as.Messages {
		if m == nil {
			continue
		}
		v += len(m.Text) + len(m.Thinking)
		for _, t := range m.Tools {
			v += len(t.Command) + len(t.Output)
		}
	}
	return v
}

func (as *AgentState) throttledInvalidate() {
	if as.Invalidate == nil {
		return
	}
	as.invalidateMu.Lock()
	now := time.Now()
	elapsed := now.Sub(as.lastInvalidate)
	if elapsed < 33*time.Millisecond {
		if !as.invalidatePending {
			as.invalidatePending = true
			remaining := 33*time.Millisecond - elapsed
			time.AfterFunc(remaining, func() {
				as.invalidateMu.Lock()
				as.lastInvalidate = time.Now()
				as.invalidatePending = false
				inv := as.Invalidate
				as.invalidateMu.Unlock()
				if inv != nil {
					inv()
				}
			})
		}
		as.invalidateMu.Unlock()
		return
	}
	as.lastInvalidate = now
	as.invalidateMu.Unlock()
	as.Invalidate()
}

// TaskTrackerSnapshotData holds a consolidated frame snapshot of all data
// required by the top-right task tracker pill, obtained in one lock.
type TaskTrackerSnapshotData struct {
	HasWork          bool
	CurrentTask      *TaskItem
	TotalTasks       int
	CompletedTasks   int
	AllDone          bool
	RunningSubs      int
	IsSessionRunning bool
}

// TaskTrackerSnapshot grabs progress, running subagents, and active turn
// status for one UI frame. Lock-light by contract (P0-1 layer 3): as.mu is
// taken once to snapshot local state, released, and only then is the
// RakitKode AgentMgr consulted — its streaming callback holds the manager
// lock while publishing bus events whose handlers take as.mu, so calling it
// under as.mu can deadlock the whole app. Called from render every frame.
func (as *AgentState) TaskTrackerSnapshot() TaskTrackerSnapshotData {
	var data TaskTrackerSnapshotData
	var remoteID string

	as.mu.Lock()
	sessionID := as.ActiveSessionID

	if as.TaskManager != nil {
		tasks := as.TaskManager.GetTasks()
		data.TotalTasks = len(tasks)
		for i := range tasks {
			if tasks[i].Status == TaskCompleted {
				data.CompletedTasks++
			} else if data.CurrentTask == nil && tasks[i].Status == TaskInProgress {
				taskCopy := tasks[i]
				data.CurrentTask = &taskCopy
			}
		}
		data.AllDone = data.CompletedTasks == data.TotalTasks && data.TotalTasks > 0
	}

	for _, s := range as.Sessions {
		if s.ID == sessionID {
			remoteID = s.RemoteSessionID
			data.IsSessionRunning = as.IsStreaming || s.Status == "running" || s.Status == "streaming"
			break
		}
	}
	as.mu.Unlock()

	if as.engine != nil && as.engine.AgentMgr != nil && remoteID != "" {
		subs := as.engine.AgentMgr.GetSubagents(remoteID) // strictly outside as.mu
		for _, sub := range subs {
			st := strings.ToLower(fmt.Sprintf("%v", sub.State))
			if st == "running" || st == "working" || st == "thinking" || st == "executing" {
				data.RunningSubs++
			}
		}
	}

	data.HasWork = (data.CurrentTask != nil) || (data.RunningSubs > 0) || data.IsSessionRunning
	return data
}

// SteerQueueSnapshot returns a copy of the active session's steer queue.
func (as *AgentState) SteerQueueSnapshot() []string {
	as.mu.Lock()
	defer as.mu.Unlock()
	return append([]string(nil), as.SteerQueue...)
}

// StreamingSnapshot reports whether the active session is streaming.
func (as *AgentState) StreamingSnapshot() bool {
	as.mu.Lock()
	defer as.mu.Unlock()
	return as.IsStreaming
}

// SnapshotActiveSessionID returns the active session id under the state lock.
func (as *AgentState) SnapshotActiveSessionID() string {
	as.mu.Lock()
	defer as.mu.Unlock()
	return as.ActiveSessionID
}

// ActiveTaskManager returns the task tracker of the active session.
func (as *AgentState) ActiveTaskManager() *TaskManager {
	as.mu.Lock()
	defer as.mu.Unlock()
	return as.TaskManager
}

// MutateMessage applies fn to the live copy of a message under the state
// lock. Render click handlers receive snapshot copies, so every render-time
// write (show/hide toggles, feedback, undo state) must go through this.
// sessionID may be empty to search all sessions.
func (as *AgentState) MutateMessage(sessionID string, messageID int, fn func(m *ChatMessage)) {
	as.mu.Lock()
	defer as.mu.Unlock()
	for _, s := range as.Sessions {
		if sessionID != "" && s.ID != sessionID {
			continue
		}
		for _, m := range s.Messages {
			if m.ID == messageID {
				m.cachedSnapshot = nil
				fn(m)
				return
			}
		}
	}
}

// ToggleMessageFeedback sets or clears a thumbs feedback ("up"/"down") on the
// live message and persists the session asynchronously. Returns whether the
// feedback is active after the toggle.
func (as *AgentState) ToggleMessageFeedback(sessionID string, messageID int, kind string) bool {
	as.mu.Lock()
	var sess *AgentSession
	active := false
	for _, s := range as.Sessions {
		if s.ID != sessionID {
			continue
		}
		sess = s
		for _, m := range s.Messages {
			if m.ID == messageID {
				m.cachedSnapshot = nil
				if m.Feedback == kind {
					m.Feedback = ""
				} else {
					m.Feedback = kind
					active = true
				}
				break
			}
		}
		break
	}
	as.mu.Unlock()
	if sess != nil {
		goSave("SaveSessionToDisk", func() { as.SaveSessionToDisk(sess) })
	}
	return active
}

// ClearMessageChanges drops a finalized message's turn-change list (used
// after a verified checkpoint restore / turn undo).
func (as *AgentState) ClearMessageChanges(sessionID string, messageID int) {
	as.MutateMessage(sessionID, messageID, func(m *ChatMessage) {
		m.Changes = nil
	})
}

// ClearConversation wipes the active session's transcript (/clear): under the
// lock it nils both the session's messages and the active alias and resets
// the streaming flag; the emptied session is persisted outside the lock.
func (as *AgentState) ClearConversation() {
	sess := as.ActiveSession()
	if sess == nil {
		return
	}
	as.mu.Lock()
	sess.Messages = nil
	if sess.ID == as.ActiveSessionID {
		as.Messages = nil
	}
	sess.UpdatedAt = time.Now()
	as.IsStreaming = false
	as.mu.Unlock()
	goSave("SaveSessionToDisk", func() { as.SaveSessionToDisk(sess) })
	if as.Invalidate != nil {
		as.Invalidate()
	}
}

// RemoveSteerItem drops the first active-session queue entry equal to text
// (identity, not index: the tray renders a snapshot that can shift) and
// reports whether it was removed. The per-session copy stays in sync.
func (as *AgentState) RemoveSteerItem(text string) bool {
	as.mu.Lock()
	removed := false
	for i, it := range as.SteerQueue {
		if it == text {
			as.SteerQueue = append(as.SteerQueue[:i], as.SteerQueue[i+1:]...)
			removed = true
			break
		}
	}
	if removed && as.ActiveSessionID != "" {
		for _, s := range as.Sessions {
			if s.ID == as.ActiveSessionID {
				s.SteerQueue = append([]string(nil), as.SteerQueue...)
				break
			}
		}
	}
	as.mu.Unlock()
	return removed
}

// TakeSteerItem removes and returns the first active-session queue entry
// equal to text ("Edit" moves the text back into the composer); returns ""
// when the entry is gone.
func (as *AgentState) TakeSteerItem(text string) string {
	as.mu.Lock()
	taken := ""
	for i, it := range as.SteerQueue {
		if it == text {
			taken = it
			as.SteerQueue = append(as.SteerQueue[:i], as.SteerQueue[i+1:]...)
			break
		}
	}
	if taken != "" && as.ActiveSessionID != "" {
		for _, s := range as.Sessions {
			if s.ID == as.ActiveSessionID {
				s.SteerQueue = append([]string(nil), as.SteerQueue...)
				break
			}
		}
	}
	as.mu.Unlock()
	return taken
}

// ensureStreamingMessage returns the assistant message currently being
// streamed for the session, appending a fresh placeholder when the turn has
// none yet (as forge-ade's callbacks do).
func (as *AgentState) ensureStreamingMessage(sess *AgentSession) *ChatMessage {
	if len(sess.Messages) > 0 {
		last := sess.Messages[len(sess.Messages)-1]
		if last.Role == "assistant" && !last.Done {
			return last
		}
	}
	msg := &ChatMessage{
		ID:            nextMessageIDLocked(sess),
		Role:          "assistant",
		Done:          false,
		Timestamp:     time.Now(),
		TurnStartedAt: time.Now(),
		IsThinking:    true,
	}
	sess.Messages = append(sess.Messages, msg)
	return msg
}

// lateAfterTurnEndLocked reports whether s is idle with a finalized last
// message: events arriving in this state are stragglers of an
// already-completed (often cancelled) turn and must not reopen a streaming
// bubble that nothing will ever finalize. Callers hold as.mu (P2c).
func lateAfterTurnEndLocked(s *AgentSession) bool {
	if s == nil {
		return false
	}
	switch s.Status {
	case "running", "streaming", "awaiting-approval":
		return false
	}
	if len(s.Messages) == 0 {
		return false
	}
	last := s.Messages[len(s.Messages)-1]
	return last.Role == "assistant" && last.Done
}

// HandleTokenChunk applies an agent:message_delta payload.
func (as *AgentState) HandleTokenChunk(sessionID, text string) {
	if text == "" {
		return
	}
	as.mu.Lock()
	for _, s := range as.Sessions {
		if s.ID != sessionID {
			continue
		}
		if lateAfterTurnEndLocked(s) {
			// Post-cancel / post-done straggler: drop it instead of creating
			// a new never-finalized streaming message (P2c).
			break
		}
		msg := as.ensureStreamingMessage(s)
		combined := msg.Text + text
		if isHarnessDumpDelta(text, msg.Text) ||
			(len(strings.TrimSpace(msg.Text)) < 256 && harnessDumpSignature(combined) &&
				strings.HasPrefix(strings.TrimSpace(combined), "{")) {
			msg.SkipHarnessDumps = true
		}
		if msg.SkipHarnessDumps {
			// Harness protocol blob: never show it as chat text.
			if s.ID == as.ActiveSessionID {
				as.Messages = s.Messages
			}
			as.mu.Unlock()
			return
		}
		msg.Text += text
		appendTextBlockDeltaLocked(msg, text)
		msg.IsThinking = false
		if s.ID == as.ActiveSessionID {
			as.Messages = s.Messages
		}
		break
	}
	as.mu.Unlock()
	as.throttledInvalidate()
}

// harnessDumpSignature reports whether a text span is (the start of) a
// harness protocol-state JSON dump rather than chat content.
func harnessDumpSignature(text string) bool {
	// Quote-prefix matches both plain JSON keys ("active_skills") and
	// escaped ones (backslash-quote prefix) inside harness dumps.
	return strings.Contains(text, `"active_skills`) ||
		strings.Contains(text, `"active_system_prompts`) ||
		strings.Contains(text, `"plugin_id`)
}

// isHarnessDumpDelta decides whether this delta begins a protocol dump:
// the message must still be (near-)empty and the chunk must look like the
// JSON blob those harnesses print.
func isHarnessDumpDelta(delta, current string) bool {
	if len(strings.TrimSpace(current)) > 64 {
		return false // a real reply already started
	}
	trimmed := strings.TrimSpace(delta)
	// Dumps arrive either as raw JSON ({...) or string-encoded JSON ("{...
	if !strings.HasPrefix(trimmed, "{") && !strings.HasPrefix(trimmed, `"{`) {
		return false
	}
	return harnessDumpSignature(trimmed) || (len(trimmed) > 400 && strings.Contains(trimmed, "\\n"))
}

// cleanHarnessDumps removes protocol-state JSON lines from a finalized
// message. Harness dumps arrive as one huge escaped-JSON line containing
// the signature keys, so whole-line removal is exact and safe.
func cleanHarnessDumps(text string) string {
	if !harnessDumpSignature(text) {
		return text
	}
	var kept []string
	for _, line := range strings.Split(text, "\n") {
		if harnessDumpSignature(line) {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

// HandleThoughtChunk applies an agent:thinking_delta payload.
func (as *AgentState) HandleThoughtChunk(sessionID, text string) {
	if text == "" {
		return
	}
	as.mu.Lock()
	for _, s := range as.Sessions {
		if s.ID != sessionID {
			continue
		}
		if lateAfterTurnEndLocked(s) {
			// Post-cancel / post-done straggler: drop (P2c).
			break
		}
		msg := as.ensureStreamingMessage(s)
		msg.Thinking += text
		appendThinkingBlockDeltaLocked(msg, text)
		msg.IsThinking = true
		if !msg.ShowThinking && len(msg.Tools) == 0 {
			// The thinking row opens by default until tools arrive.
			msg.ShowThinking = true
		}
		if s.ID == as.ActiveSessionID {
			as.Messages = s.Messages
		}
		break
	}
	as.mu.Unlock()
	as.throttledInvalidate()
}

// HandleToolEvent applies an agent:tool_start / agent:tool_end payload.
func (as *AgentState) HandleToolEvent(ev ToolEvent) {
	as.mu.Lock()
	for _, s := range as.Sessions {
		if s.ID != ev.SessionID {
			continue
		}
		if lateAfterTurnEndLocked(s) {
			// Post-cancel / post-done straggler (P2c): refresh an existing
			// tool card of the finalized message when one matches, but never
			// reopen a streaming bubble that nothing will finalize.
			if len(s.Messages) > 0 {
				last := s.Messages[len(s.Messages)-1]
				if last.Done && last.Role == "assistant" {
					for _, t := range last.Tools {
						matched := (ev.ToolCallID != "" && t.ID == ev.ToolCallID) ||
							(ev.ToolCallID == "" && t.Status == "running" && (ev.Name == "" || t.ToolName == ev.Name))
						if !matched {
							continue
						}
						if ev.Name != "" {
							t.ToolName = ev.Name
						}
						if ev.Output != "" {
							t.Output = ev.Output
						}
						t.Status = ev.Status
						if ev.Status != "running" && t.DurationMs == 0 {
							t.DurationMs = time.Now().UnixMilli() - t.CreatedAtMs
						}
						last.cachedSnapshot = nil
						break
					}
				}
			}
			if s.ID == as.ActiveSessionID {
				as.Messages = s.Messages
			}
			break
		}
		msg := as.ensureStreamingMessage(s)

		var existing *ToolExecution
		if ev.ToolCallID != "" {
			for _, t := range msg.Tools {
				if t.ID == ev.ToolCallID {
					existing = t
					break
				}
			}
		}
		if existing == nil {
			// Fall back to a matching in-flight tool of the same name.
			for _, t := range msg.Tools {
				if t.Status == "running" && (ev.Name == "" || t.ToolName == ev.Name) {
					existing = t
					break
				}
			}
		}
		if existing == nil {
			id := ev.ToolCallID
			if id == "" {
				id = fmt.Sprintf("tool-%d", time.Now().UnixNano()%100000)
			}
			existing = &ToolExecution{
				ID:          id,
				ToolName:    ev.Name,
				Status:      ev.Status,
				CreatedAtMs: time.Now().UnixMilli(),
			}
			msg.Tools = append(msg.Tools, existing)
			appendToolBlockLocked(msg, id)
			msg.IsThinking = false
		}
		if ev.Status == "running" {
			msg.IsThinking = false
		}
		// update_tasks-style calls drive the floating tracker, like
		// forge-ade's runtime task list.
		if strings.Contains(strings.ToLower(ev.Name), "task") {
			as.applyTaskUpdatePayloadLocked(s, ev.Args)
			as.applyTaskUpdatePayloadLocked(s, ev.Output)
		}
		if ev.Name != "" {
			existing.ToolName = ev.Name
		}
		if ev.Args != "" {
			existing.Command = ev.Args
		}
		if ev.Output != "" {
			existing.Output = ev.Output
		}
		existing.Status = ev.Status
		if ev.Status != "running" {
			existing.DurationMs = time.Now().UnixMilli() - existing.CreatedAtMs
		}
		if s.ID == as.ActiveSessionID {
			as.Messages = s.Messages
		}
		break
	}
	as.mu.Unlock()
	if as.Invalidate != nil {
		as.Invalidate()
	}
}

// HandleTurnStart applies an agent:turn_start payload.
func (as *AgentState) HandleTurnStart(sessionID, prompt string) {
	as.mu.Lock()
	for _, s := range as.Sessions {
		if s.ID != sessionID {
			continue
		}
		msg := as.ensureStreamingMessage(s)
		msg.TurnStartedAt = time.Now()
		msg.IsThinking = true
		// A reused message must not carry a stale expanded trace into the new
		// turn: the Worked group always starts open-by-streaming, not by flag.
		msg.ShowThinking = false
		msg.ShowTools = false
		s.Status = "running"
		if s.ID == as.ActiveSessionID {
			as.Messages = s.Messages
			as.IsStreaming = true
		}
		break
	}
	as.mu.Unlock()
	if as.Invalidate != nil {
		as.Invalidate()
	}
}

// HandleTurnDone applies an agent:turn_end payload: finalizes the assistant
// message, maps the status vocabulary, and returns the next queued steer
// prompt, if any. It runs on the ACPManager event-consumer goroutine, so it
// must return FAST: the turn's git change list is computed on a detached
// goroutine (attachTurnChanges) — serial git subprocesses here stalled the
// queue and, once it saturated, deadlocked the streaming publisher (P0-1).
func (as *AgentState) HandleTurnDone(sessionID string, info TurnEndInfo) (nextSteer string) {
	var startFiles map[string]bool
	var sessWs string
	as.mu.Lock()
	sessWs = as.workspacePath
	for _, s := range as.Sessions {
		if s.ID == sessionID {
			startFiles = s.turnStartFiles
			if s.WorkspacePath != "" {
				sessWs = s.WorkspacePath
			}
			break
		}
	}
	as.mu.Unlock()

	as.mu.Lock()
	var sessToSave *AgentSession
	finalizedMsgID := 0
	for _, s := range as.Sessions {
		if s.ID != sessionID {
			continue
		}
		msg := as.ensureStreamingMessage(s)
		msg.Done = true
		msg.IsThinking = false
		msg.ShowThinking = false
		msg.ShowTools = false
		// Close any still-open timeline block so per-block "Thought Ns" labels
		// freeze at the turn boundary.
		if n := len(msg.Blocks); n > 0 && msg.Blocks[n-1] != nil && msg.Blocks[n-1].EndedAtMs == 0 {
			msg.Blocks[n-1].EndedAtMs = time.Now().UnixMilli()
		}
		msg.Text = cleanTaskJSONBlocks(cleanRawToolTags(cleanHarnessDumps(msg.Text)))
		for _, b := range msg.Blocks {
			if b == nil {
				continue
			}
			switch b.Kind {
			case "text":
				b.Text = cleanTaskJSONBlocks(cleanRawToolTags(cleanHarnessDumps(b.Text)))
			case "thinking":
				b.Text = cleanRawToolTags(b.Text)
			}
		}
		msg.Thinking = cleanRawToolTags(msg.Thinking)
		if !msg.TurnStartedAt.IsZero() {
			msg.ThinkingTimeSec = time.Since(msg.TurnStartedAt).Seconds()
		} else {
			msg.ThinkingTimeSec = time.Since(msg.Timestamp).Seconds()
		}
		if info.TotalSteps > 0 {
			msg.TotalSteps = info.TotalSteps
		} else if info.StepCount > 0 {
			msg.TotalSteps = info.StepCount
		}
		if info.RemoteSessionID != "" && !strings.HasPrefix(info.RemoteSessionID, "session-") && !strings.HasPrefix(info.RemoteSessionID, "acp-") {
			s.RemoteSessionID = info.RemoteSessionID
		}

		s.turnStartFiles = nil
		finalizedMsgID = msg.ID

		// Safety net: any OTHER assistant message still !Done (e.g. orphaned
		// by a mid-turn steer bubble inserted above it) must not linger as a
		// perpetually-"in progress" bubble.
		for _, m := range s.Messages {
			if m != nil && m != msg && m.Role == "assistant" && !m.Done {
				m.Done = true
				m.IsThinking = false
				m.ShowThinking = false
				m.ShowTools = false
			}
		}

		// Status vocabulary of the original app.
		switch {
		case info.Error != "":
			s.Status = "error"
			if strings.TrimSpace(msg.Text) == "" {
				setAssistantTextLocked(msg, fmt.Sprintf("⚠️ **Error**: %s", info.Error))
			} else {
				msg.Text += fmt.Sprintf("\n\n⚠️ **Error**: %s", info.Error)
				appendTextBlockDeltaLocked(msg, fmt.Sprintf("\n\n⚠️ **Error**: %s", info.Error))
			}
		case info.Status == "completed":
			s.Status = "completed"
		case info.Status == "stopped":
			s.Status = "idle"
			if strings.TrimSpace(msg.Text) == "" {
				setAssistantTextLocked(msg, "*(stopped by user)*")
			}
		default:
			s.Status = "idle"
		}
		s.UpdatedAt = time.Now()
		if s.ID == as.ActiveSessionID {
			as.Messages = s.Messages
		}
		sessToSave = s
		break
	}
	// Clear the global streaming flag only when the visible active session completed.
	if as.ActiveSessionID == sessionID || sessionID == "" {
		as.IsStreaming = false
	}
	// Drain the completing session's own steer queue even if it completed in
	// the background — but never when the user stopped the turn: draining
	// would hand the queue to HandleTurnDone's caller, which relaunches the
	// session and defeats Stop. Queued prompts stay in the tray (and on the
	// session) so the next real turn consumes them. The global queue is only
	// drained when the completing session still exists: a gone session must
	// not swallow prompts that belong to the replacement session (P2e).
	turnStopped := info.Status == "stopped"
	var steerSnapshot []string
	saveSteer := false
	if !turnStopped && sessToSave != nil && len(sessToSave.SteerQueue) > 0 {
		nextSteer = sessToSave.SteerQueue[0]
		sessToSave.SteerQueue = sessToSave.SteerQueue[1:]
		steerSnapshot = append([]string(nil), sessToSave.SteerQueue...)
		saveSteer = true
		if sessToSave.ID == as.ActiveSessionID {
			as.SteerQueue = append([]string(nil), sessToSave.SteerQueue...)
		}
		sessToSave.Messages = append(sessToSave.Messages, &ChatMessage{
			ID:        nextMessageIDLocked(sessToSave),
			Role:      "user",
			Text:      nextSteer,
			Done:      true,
			Timestamp: time.Now(),
		})
		if sessToSave.ID == as.ActiveSessionID {
			as.Messages = sessToSave.Messages
		}
	} else if !turnStopped && sessToSave != nil && (as.ActiveSessionID == sessionID || sessionID == "") && len(as.SteerQueue) > 0 {
		nextSteer = as.SteerQueue[0]
		as.SteerQueue = as.SteerQueue[1:]
		steerSnapshot = append([]string(nil), as.SteerQueue...)
		saveSteer = true
		sessToSave.SteerQueue = append([]string(nil), as.SteerQueue...)
		sessToSave.Messages = append(sessToSave.Messages, &ChatMessage{
			ID:        nextMessageIDLocked(sessToSave),
			Role:      "user",
			Text:      nextSteer,
			Done:      true,
			Timestamp: time.Now(),
		})
		if sessToSave.ID == as.ActiveSessionID {
			as.Messages = sessToSave.Messages
		}
	}
	as.mu.Unlock()

	if sessToSave != nil {
		as.SaveSessionToDisk(sessToSave)
	}
	if saveSteer {
		as.saveSteerQueuesSlice(steerSnapshot)
	}
	if as.Invalidate != nil {
		as.Invalidate()
	}

	// Turn changes: git status + per-file diffs run OFF the event consumer
	// and are attached to the finalized message under as.mu afterwards.
	if sessToSave != nil && finalizedMsgID > 0 {
		goSave("attachTurnChanges", func() {
			as.attachTurnChanges(sessionID, finalizedMsgID, sessWs, startFiles)
		})
	}
	return nextSteer
}

// attachTurnChanges computes the turn's file-change list (git status snapshot
// + bounded per-file diffs) and attaches it to the finalized assistant
// message under as.mu, then re-persists the session. Runs detached from
// HandleTurnDone so the event consumer never waits on git subprocesses
// (P0-1 layer 2).
func (as *AgentState) attachTurnChanges(sessionID string, msgID int, sessWs string, startFiles map[string]bool) {
	turnEndFiles := as.gitStatusPathsForWorkspace(sessWs)
	var turnChanges []*FileChange
	for path := range turnEndFiles {
		if startFiles == nil || !startFiles[path] {
			turnChanges = append(turnChanges, &FileChange{Path: path, Additions: -1, Deletions: -1})
		}
	}
	as.computeChangeStatsForWorkspace(sessWs, turnChanges)
	if len(turnChanges) == 0 {
		return
	}

	as.mu.Lock()
	var sess *AgentSession
	for _, s := range as.Sessions {
		if s.ID == sessionID {
			sess = s
			break
		}
	}
	// The session may have been deleted while the git subprocesses ran:
	// never resurrect it in memory or on disk (P2b).
	if sess != nil {
		for _, m := range sess.Messages {
			if m.ID == msgID {
				m.cachedSnapshot = nil
				m.Changes = turnChanges
				break
			}
		}
	}
	exists := sess != nil
	as.mu.Unlock()

	if exists {
		as.SaveSessionToDisk(sess)
	}
	if as.Invalidate != nil {
		as.Invalidate()
	}
}

// pendingApprovalFromRequest derives the render-facing approval card from a
// queued permission request. Callers must hold as.mu.
func pendingApprovalFromRequest(s *AgentSession, r *PermissionRequest) *PendingApproval {
	if r == nil {
		return nil
	}
	return &PendingApproval{
		SessionID:  s.ID,
		ToolCallID: r.RequestID,
		ToolName:   r.ToolName,
		Summary:    r.Title,
		Arguments:  r.Arguments,
	}
}

// HandleAsk applies an agent:ask permission request: it is appended to the
// session's pending-approval queue (a second request must not hide the first
// unanswered one), the streaming message points at the earliest unresolved
// entry, and the session flips to awaiting-approval.
func (as *AgentState) HandleAsk(req *PermissionRequest) {
	if req == nil {
		return
	}
	as.mu.Lock()
	for _, s := range as.Sessions {
		if s.ID != req.SessionID {
			continue
		}
		s.Status = "awaiting-approval"
		s.PendingApprovals = append(s.PendingApprovals, req)
		msg := as.ensureStreamingMessage(s)
		msg.PendingApproval = pendingApprovalFromRequest(s, s.PendingApprovals[0])
		if s.ID == as.ActiveSessionID {
			as.Messages = s.Messages
		}
		break
	}
	as.mu.Unlock()
	if as.Invalidate != nil {
		as.Invalidate()
	}
}

// ResolveSessionApproval pops the answered request (matched by request ID)
// from the session's pending-approval queue after a successful respond, and
// re-points the approval bar at the earliest remaining request — or clears
// it when the queue is empty. Call only after the backend accepted the
// decision; on error the queue is left untouched so the user can retry.
func (as *AgentState) ResolveSessionApproval(sessionID, requestID string) {
	as.mu.Lock()
	for _, s := range as.Sessions {
		if s.ID != sessionID {
			continue
		}
		for i, r := range s.PendingApprovals {
			if r != nil && r.RequestID == requestID {
				s.PendingApprovals = append(s.PendingApprovals[:i], s.PendingApprovals[i+1:]...)
				break
			}
		}
		for i := len(s.Messages) - 1; i >= 0; i-- {
			msg := s.Messages[i]
			if msg.PendingApproval == nil || msg.PendingApproval.Resolved {
				continue
			}
			if len(s.PendingApprovals) > 0 {
				msg.PendingApproval = pendingApprovalFromRequest(s, s.PendingApprovals[0])
			} else {
				msg.PendingApproval = nil
			}
			break
		}
		if s.ID == as.ActiveSessionID {
			as.Messages = s.Messages
		}
		break
	}
	as.mu.Unlock()
	if as.Invalidate != nil {
		as.Invalidate()
	}
}

func (as *AgentState) applyTaskUpdatePayloadLocked(sess *AgentSession, payload string) bool {
	payload = strings.TrimSpace(payload)
	if sess == nil || payload == "" || payload[0] != '{' {
		return false
	}
	var parsed struct {
		Tasks []struct {
			Title      string `json:"title"`
			Status     string `json:"status"`
			ActiveForm string `json:"active_form"`
		} `json:"tasks"`
	}
	if err := json.Unmarshal([]byte(payload), &parsed); err != nil || len(parsed.Tasks) == 0 {
		return false
	}
	if sess.Tasks == nil {
		sess.Tasks = NewTaskManager()
	}
	items := make([]TaskItem, 0, len(parsed.Tasks))
	for i, tk := range parsed.Tasks {
		status := TaskPending
		switch strings.ToLower(tk.Status) {
		case "completed", "done":
			status = TaskCompleted
		case "in_progress", "active", "running":
			status = TaskInProgress
		}
		items = append(items, TaskItem{
			ID:         fmt.Sprintf("%d", i+1),
			Title:      tk.Title,
			Status:     status,
			ActiveForm: tk.ActiveForm,
		})
	}
	sess.Tasks.SetTasks(items)
	if sess.ID == as.ActiveSessionID {
		as.TaskManager = sess.Tasks
		as.Messages = sess.Messages
	}
	return true
}

// applyTaskUpdatePayload parses an update_tasks payload (args or result JSON
// with a tasks array) into the active session's floating tracker.
func (as *AgentState) applyTaskUpdatePayload(payload string) {
	as.mu.Lock()
	var active *AgentSession
	for _, s := range as.Sessions {
		if s.ID == as.ActiveSessionID {
			active = s
			break
		}
	}
	updated := as.applyTaskUpdatePayloadLocked(active, payload)
	as.mu.Unlock()
	if updated && as.Invalidate != nil {
		as.Invalidate()
	}
}

// HandleUsage records a usage report for the context gauge and quota modal.
func (as *AgentState) HandleUsage(sessionID string, usage UsageInfo) {
	as.mu.Lock()
	for _, s := range as.Sessions {
		if s.ID != sessionID {
			continue
		}
		u := usage
		s.LastUsage = &u
		break
	}
	as.mu.Unlock()
}

// HandleCommands stores the slash commands the agent advertises.
func (as *AgentState) HandleCommands(sessionID string, commands []string) {
	as.mu.Lock()
	for _, s := range as.Sessions {
		if s.ID != sessionID {
			continue
		}
		s.AvailableCommands = commands
		break
	}
	as.mu.Unlock()
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
		return "Medium"
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

// AttachFiles adds several files, resolving workspace-relative paths to
// absolute ones so the ACP manager's image extraction finds them.
func (as *AgentState) AttachFiles(paths []string) {
	for _, p := range paths {
		if p == "" {
			continue
		}
		if !filepath.IsAbs(p) {
			p = filepath.Join(as.workspacePath, p)
		}
		as.AttachFile(p)
	}
}

var (
	rawToolOnce = sync.OnceValue(func() []*regexp.Regexp {
		patterns := []string{
			// 1. Complete in-band tool call and container blocks.
			`(?i)<toolcall[\s\S]*?</toolcall>`,
			`(?i)<tool_call[\s\S]*?</tool_call>`,
			`(?i)<[|\s\x{FF5C}]*(?:DSML[|\s\x{FF5C}]*)?calls>[\s\S]*?</[|\s\x{FF5C}]*(?:DSML[|\s\x{FF5C}]*)?calls>`,
			`(?i)<[|\s\x{FF5C}]*(?:DSML[|\s\x{FF5C}]*)?invoke[\s\S]*?</[|\s\x{FF5C}]*(?:DSML[|\s\x{FF5C}]*)?invoke>`,
			`(?i)<function_call[\s\S]*?</function_call>`,
			// 2. Unclosed in-band tool block at the end (streaming params).
			`(?i)<[|\s\x{FF5C}]*(?:DSML[|\s\x{FF5C}]*)?(?:calls|invoke)[\s\S]*$`,
			`(?i)<(?:toolcall|tool_call|function_call)[\s\S]*$`,
			// 3. Standalone or unclosed parameter blocks.
			`(?i)<[|\s\x{FF5C}]*(?:DSML[|\s\x{FF5C}]*)?parameter[\s\S]*?</[|\s\x{FF5C}]*(?:DSML[|\s\x{FF5C}]*)?parameter>`,
			`(?i)<[|\s\x{FF5C}]*(?:DSML[|\s\x{FF5C}]*)?parameter[\s\S]*$`,
			// 4. Residual tool tags.
			`(?i)</?(?:toolcall|tool_call|function_call|[|\s\x{FF5C}]*(?:DSML[|\s\x{FF5C}]*)?(?:invoke|parameter|calls|call_begin|call_end))(?:=[^>]+|[^>]*)>`,
		}
		res := make([]*regexp.Regexp, 0, len(patterns))
		for _, p := range patterns {
			res = append(res, regexp.MustCompile(p))
		}
		return res
	})
)

// cleanRawToolTags removes in-band tool-call blocks that models echo into
// their text (DeepSeek DSML tags, toolcall/function_call blocks), exactly as
// forge-ade's cleanRawToolTags does before a reply is shown.
func cleanRawToolTags(text string) string {
	if text == "" {
		return ""
	}
	s := text
	for _, re := range rawToolOnce() {
		s = re.ReplaceAllString(s, "")
	}
	return strings.TrimSpace(s)
}

// cleanTaskJSONBlocks strips standalone update_tasks JSON blobs (tool
// arguments/results the model echoes into its reply) from the final text.
// A blob is a brace-balanced object carrying an update_tasks signature.
func cleanTaskJSONBlocks(text string) string {
	if !strings.Contains(text, `"tasks"`) &&
		!strings.Contains(text, `"active_form"`) &&
		!strings.Contains(text, `"just_completed"`) {
		return text
	}
	signatures := []string{`"tasks"`, `"active_form"`, `"just_completed"`}
	var out strings.Builder
	i := 0
	for i < len(text) {
		if text[i] == '{' {
			headEnd := i + 200
			if headEnd > len(text) {
				headEnd = len(text)
			}
			head := text[i:headEnd]
			isTask := false
			for _, sig := range signatures {
				if strings.Contains(head, sig) {
					isTask = true
					break
				}
			}
			if isTask {
				if end := braceEnd(text, i); end != -1 {
					i = end + 1
					continue
				}
			}
		}
		out.WriteByte(text[i])
		i++
	}
	return strings.TrimRight(out.String(), " \t\n")
}

// braceEnd returns the index of the '}' matching the '{' at start, honoring
// strings and escapes, or -1.
func braceEnd(text string, start int) int {
	depth := 0
	inStr, esc := false, false
	for j := start; j < len(text); j++ {
		c := text[j]
		if esc {
			esc = false
			continue
		}
		if inStr {
			switch {
			case c == '\\':
				esc = true
			case c == '"':
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return j
			}
		}
	}
	return -1
}

// CleanRawToolTags exposes the cleaner for the UI layer.
func CleanRawToolTags(text string) string { return cleanRawToolTags(text) }

// AttachFile adds a file path to the pending composer attachments.
func (as *AgentState) AttachFile(path string) {
	for _, p := range as.AttachedFiles {
		if p == path {
			return
		}
	}
	as.AttachedFiles = append(as.AttachedFiles, path)
}

// SavePastedImage writes a clipboard PNG into the workspace's attachments
// folder (where forge-ade's prompt enrichment looks) and returns its path.
func (as *AgentState) SavePastedImage(png []byte) (string, error) {
	if len(png) == 0 {
		return "", fmt.Errorf("empty image")
	}
	dir := filepath.Join(as.workspacePath, ".forge", "attachments")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, fmt.Sprintf("pasted-%d.png", time.Now().UnixMilli()))
	if err := os.WriteFile(path, png, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// RemoveAttachedFile deletes an attachment by index.
func (as *AgentState) RemoveAttachedFile(idx int) {
	if idx >= 0 && idx < len(as.AttachedFiles) {
		as.AttachedFiles = append(as.AttachedFiles[:idx], as.AttachedFiles[idx+1:]...)
	}
}

// ClearAttachedFiles clears all pending composer attachments.
func (as *AgentState) ClearAttachedFiles() {
	as.AttachedFiles = nil
}

// maxStashedPrompts is the documented capacity of the prompt stash.
const maxStashedPrompts = 20

// StashPrompt stashes the current composer draft text into memory and disk,
// keeping at most maxStashedPrompts entries (oldest dropped first).
func (as *AgentState) StashPrompt() {
	as.mu.Lock()
	txt := strings.TrimSpace(as.InputText)
	stashed := false
	if txt != "" {
		as.StashedPrompts = append(as.StashedPrompts, txt)
		if len(as.StashedPrompts) > maxStashedPrompts {
			as.StashedPrompts = append([]string(nil), as.StashedPrompts[len(as.StashedPrompts)-maxStashedPrompts:]...)
		}
		as.InputText = ""
		stashed = true
	}
	as.mu.Unlock()
	if stashed {
		goSave("SavePromptStash", func() { as.SavePromptStash() })
	}
}

func agentDataDir(e *engine.Engine) (string, error) {
	if e != nil && e.DataDir != "" {
		return e.DataDir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".forge-ade"), nil
}

// draftStore paths for per-session composer drafts and steer queues.
func (as *AgentState) draftStorePath(name string) string {
	dir, err := agentDataDir(as.engine)
	if err != nil {
		return ""
	}
	return filepath.Join(dir, name)
}

func readStringMap(path string) map[string]string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		return nil
	}
	return m
}

func writeStringMap(path string, m map[string]string) {
	if data, err := json.MarshalIndent(m, "", "  "); err == nil {
		_ = agentsession.WriteFileAtomic(path, data, 0o644)
	}
}

// SaveComposerDraft persists the in-progress prompt of a session
// (forge:composer-drafts parity) and restores the target session's draft.
func (as *AgentState) SwapComposerDraft(fromSessionID, toSessionID string) {
	if fromSessionID == toSessionID {
		return
	}
	path := as.draftStorePath("composer-drafts.json")
	if path == "" {
		return
	}
	m := readStringMap(path)
	if m == nil {
		m = map[string]string{}
	}
	if cur := strings.TrimSpace(as.InputText); cur != "" {
		m[fromSessionID] = cur
	} else {
		delete(m, fromSessionID)
	}
	as.InputText = m[toSessionID]
	writeStringMap(path, m)
}

func (as *AgentState) saveSteerQueuesSlice(q []string) {
	path := as.draftStorePath("steer-queues.json")
	if path == "" {
		return
	}
	if data, err := json.MarshalIndent(q, "", "  "); err == nil {
		_ = agentsession.WriteFileAtomic(path, data, 0o644)
	}
}

// SaveSteerQueues persists the active session's steer queue and keeps its per-session copy in sync.
func (as *AgentState) SaveSteerQueues() {
	as.mu.Lock()
	q := append([]string(nil), as.SteerQueue...)
	if as.ActiveSessionID != "" {
		for _, s := range as.Sessions {
			if s.ID == as.ActiveSessionID {
				s.SteerQueue = append([]string(nil), q...)
				break
			}
		}
	}
	as.mu.Unlock()
	as.saveSteerQueuesSlice(q)
}

// LoadSteerQueues restores the persisted global steer queue.
func (as *AgentState) LoadSteerQueues() {
	path := as.draftStorePath("steer-queues.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var q []string
	if err := json.Unmarshal(data, &q); err != nil {
		return
	}
	as.mu.Lock()
	as.SteerQueue = q
	as.mu.Unlock()
}

// LoadPromptStash reads persisted stashed prompts (max 20).
func (as *AgentState) LoadPromptStash() {
	path := as.draftStorePath("prompt-stash.json")
	if path == "" {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var stash []string
	if err := json.Unmarshal(data, &stash); err != nil || len(stash) == 0 {
		return
	}
	as.mu.Lock()
	as.StashedPrompts = stash
	as.mu.Unlock()
}

// SavePromptStash persists the stash list.
func (as *AgentState) SavePromptStash() {
	as.mu.Lock()
	stash := append([]string(nil), as.StashedPrompts...)
	as.mu.Unlock()
	path := as.draftStorePath("prompt-stash.json")
	if path == "" {
		return
	}
	if data, err := json.Marshal(stash); err == nil {
		_ = agentsession.WriteFileAtomic(path, data, 0644)
	}
}

// RestoreStash restores a stashed prompt back into the composer.
func (as *AgentState) RestoreStash(idx int) {
	as.mu.Lock()
	restored := false
	if idx >= 0 && idx < len(as.StashedPrompts) {
		as.InputText = as.StashedPrompts[idx]
		as.StashedPrompts = append(as.StashedPrompts[:idx], as.StashedPrompts[idx+1:]...)
		restored = true
	}
	as.mu.Unlock()
	if restored {
		goSave("SavePromptStash", func() { as.SavePromptStash() })
	}
}

type diskSessionRaw struct {
	ID        string          `json:"id"`
	Title     string          `json:"title"`
	Name      string          `json:"name"`
	Goal      string          `json:"goal"`
	Status    string          `json:"status"`
	State     string          `json:"state"`
	CreatedAt json.RawMessage `json:"createdAt"`
	UpdatedAt json.RawMessage `json:"updatedAt"`
	// Engine-internal sub-agent schema markers (RakitKode agent.Session
	// dumps): presence means this file is a child-agent session, never a
	// user task. Their timestamps live in created_at/updated_at (snake_case)
	// which the fields above never read.
	ParentSessionID      string           `json:"parent_session_id"`
	RoleFilter           string           `json:"role_filter"`
	Model                string           `json:"model"`
	Provider             string           `json:"provider"`
	AgentID              string           `json:"agentId"`
	AgentType            string           `json:"agentType"`
	ExecutionMode        string           `json:"executionMode"`
	ReasoningLevel       string           `json:"reasoningLevel"`
	WorkspacePath        string           `json:"workspacePath"`
	Folder               string           `json:"folder"`
	Pinned               bool             `json:"pinned"`
	Archived             bool             `json:"archived"`
	Snoozed              bool             `json:"snoozed"`
	RemoteSessionID      string           `json:"remoteSessionId"`
	LastCheckpointTurnID string           `json:"lastCheckpointTurnId"`
	LastUsage            *UsageInfo       `json:"lastUsage"`
	Messages             []diskMessageRaw `json:"messages"`
	Tasks                []TaskItem       `json:"tasks"`
}

type diskMessageRaw struct {
	ID              string            `json:"id"`
	Role            string            `json:"role"`
	Content         json.RawMessage   `json:"content"`
	Text            string            `json:"text"`
	Timestamp       json.RawMessage   `json:"timestamp"`
	TurnTexts       map[string]string `json:"turnTexts"`
	IsThinking      bool              `json:"isThinking"`
	Thoughts        []string          `json:"thoughts"`
	ThinkingTimeSec float64           `json:"thinkingTimeSec"`
	TotalSteps      int               `json:"totalSteps"`
	Feedback        string            `json:"feedback"`
	Changes         []FileChange      `json:"changes"`
	ToolExecutions  []ToolExecution   `json:"toolExecutions"`
	Blocks          []MessageBlock    `json:"blocks"`
	TextBlocks      []struct {
		Text string `json:"text"`
	} `json:"textBlocks"`
}

func parseTimeRaw(raw json.RawMessage) time.Time {
	if len(raw) == 0 {
		return time.Time{}
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
	// Missing or unparsable timestamps must stay zero, never "now": a now
	// fallback made every session with a foreign/absent updatedAt render
	// "now" in the sidebar forever (the timestamp is re-derived each launch).
	// SaveSessionToDisk stamps zero UpdatedAt with time.Now() on write.
	return time.Time{}
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
		// Engine-internal sub-agent sessions must never load as user tasks:
		// they render in the Subagent drawer, and their snake_case
		// timestamps parse as zero (previously: time.Now(), which is what
		// made the sidebar show "now" for all of them on every launch).
		if diskSess.ParentSessionID != "" || diskSess.RoleFilter != "" {
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
		sessAgentID := diskSess.AgentID
		sessAgentType := diskSess.AgentType

		hasInternalKey := hasConfiguredInternalLLM(as.engine)

		if !hasInternalKey {
			if sessAgentID == "" || sessAgentID == "agent-monolith" || sessAgentType == "internal" || sessAgentType == "" {
				sessAgentID = as.ActiveAgentID
				sessAgentType = as.ActiveAgentType
			}
			if sessModel == "" || sessModel == "gpt-4o" || strings.Contains(sessModel, "gpt-4o") || strings.Contains(sessModel, "Sonnet") {
				sessModel = as.ActiveModel
			}
		} else if sessModel == "" {
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
			// Clean raw tool tags and echoed task JSON, as forge-ade does
			// for loaded sessions.
			txt := cleanTaskJSONBlocks(cleanRawToolTags(cleanHarnessDumps(extractMessageText(m))))
			if txt == "" && len(m.ToolExecutions) == 0 && len(m.Thoughts) == 0 {
				continue
			}

			var toolsList []*ToolExecution
			for _, t := range m.ToolExecutions {
				tCopy := t
				toolsList = append(toolsList, &tCopy)
			}

			var blocks []*MessageBlock
			if len(m.Blocks) > 0 {
				for _, b := range m.Blocks {
					bCopy := b
					if bCopy.Kind == "text" {
						bCopy.Text = cleanTaskJSONBlocks(cleanRawToolTags(cleanHarnessDumps(bCopy.Text)))
					}
					blocks = append(blocks, &bCopy)
				}
			}

			var thinkingStr string
			if len(m.Thoughts) > 0 {
				thinkingStr = strings.Join(m.Thoughts, "\n\n")
			}

			var changes []*FileChange
			if len(m.Changes) > 0 {
				for _, ch := range m.Changes {
					chCopy := ch
					changes = append(changes, &chCopy)
				}
			} else {
				changes = extractChangesFromTools(toolsList)
			}

			thinkTime := m.ThinkingTimeSec
			if thinkTime <= 0 && len(thinkingStr) > 0 {
				thinkTime = float64(len(thinkingStr)) / 100.0
			}

			msgs = append(msgs, &ChatMessage{
				ID:              msgID,
				Role:            role,
				Text:            txt,
				Done:            true,
				Timestamp:       parseTimeRaw(m.Timestamp),
				Thinking:        thinkingStr,
				IsThinking:      m.IsThinking,
				ThinkingTimeSec: thinkTime,
				TotalSteps:      m.TotalSteps,
				Feedback:        m.Feedback,
				Tools:           toolsList,
				Blocks:          blocks,
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
		// A process kill mid-turn persists "running"/"streaming"; nothing is
		// actually streaming after a restart — show Idle, never a phantom
		// Running pill (forge-ade marks interrupted sessions the same way).
		if status == "running" || status == "streaming" || status == "working" {
			status = "idle"
		}

		wsPath := diskSess.WorkspacePath
		if wsPath == "" {
			wsPath = diskSess.Folder
		}
		wsPath = agentsession.NormalizeWorkspacePath(wsPath)

		loaded = append(loaded, &AgentSession{
			ID:                   diskSess.ID,
			Title:                sessTitle,
			Status:               status,
			CreatedAt:            createdTime,
			UpdatedAt:            updatedTime,
			Model:                sessModel,
			Provider:             diskSess.Provider,
			AgentID:              sessAgentID,
			AgentType:            sessAgentType,
			WorkspacePath:        wsPath,
			Pinned:               diskSess.Pinned,
			Archived:             diskSess.Archived,
			Snoozed:              diskSess.Snoozed,
			RemoteSessionID:      diskSess.RemoteSessionID,
			LastCheckpointTurnID: diskSess.LastCheckpointTurnID,
			LastUsage:            diskSess.LastUsage,
			Messages:             msgs,
			Tasks:                tm,
			nextMsgID:            msgID, // msgs consumed IDs 1..msgID-1; continue above them
		})
	}

	if len(loaded) > 0 {
		// Sort by UpdatedAt descending (newest first), tie-broken deterministically by ID.
		sort.Slice(loaded, func(i, j int) bool {
			if !loaded[i].UpdatedAt.Equal(loaded[j].UpdatedAt) {
				return loaded[i].UpdatedAt.After(loaded[j].UpdatedAt)
			}
			return loaded[i].ID > loaded[j].ID
		})
		as.Sessions = loaded
		as.ActiveSessionID = ""
		as.mu.Lock()
		as.syncActiveSession()
		as.mu.Unlock()
		as.applyPendingLLMModel()
	}
}

// SaveSessionToDisk writes the given session to ~/.forge/sessions/[project-name]/session-<id>.json.
func (as *AgentState) SaveSessionToDisk(sess *AgentSession) {
	if sess == nil {
		return
	}

	// Snapshot everything the disk format needs under the state lock: turn
	// goroutines mutate messages and tool structs in place, and the previous
	// unlocked iteration raced them. Per-message struct copies keep the
	// encoded payload off the live backing arrays; disk I/O stays outside.
	as.mu.Lock()
	if as.deletedIDs != nil && as.deletedIDs[sess.ID] {
		as.mu.Unlock()
		return
	}
	// In-flight turn goroutines hold session pointers across DeleteSession:
	// a session that no longer exists in memory must NOT be resurrected on
	// disk (P2b). All callers snapshot from as.Sessions, so presence here is
	// a reliable liveness signal.
	live := false
	for _, s := range as.Sessions {
		if s == sess {
			live = true
			break
		}
	}
	if !live {
		as.mu.Unlock()
		return
	}
	var diskMsgs []map[string]interface{}
	for _, m := range sess.Messages {
		if m == nil {
			continue
		}
		cp := snapshotMessageLocked(m)
		role := cp.Role
		if role == "assistant" {
			role = "agent"
		}
		msgMap := map[string]interface{}{
			"id":        fmt.Sprintf("msg-%d", cp.ID),
			"role":      role,
			"content":   cp.Text,
			"timestamp": cp.Timestamp.Format(time.RFC3339),
		}
		if cp.Thinking != "" {
			msgMap["thoughts"] = []string{cp.Thinking}
			msgMap["isThinking"] = cp.IsThinking
		}
		if cp.ThinkingTimeSec > 0 {
			msgMap["thinkingTimeSec"] = cp.ThinkingTimeSec
		}
		if cp.TotalSteps > 0 {
			msgMap["totalSteps"] = cp.TotalSteps
		}
		if cp.Feedback != "" {
			msgMap["feedback"] = cp.Feedback
		}
		if len(cp.Changes) > 0 {
			msgMap["changes"] = cp.Changes
		}
		if len(cp.Tools) > 0 {
			msgMap["toolExecutions"] = cp.Tools
		}
		if len(cp.Blocks) > 0 {
			msgMap["blocks"] = cp.Blocks
		}
		diskMsgs = append(diskMsgs, msgMap)
	}

	var tasksList []TaskItem
	goal := sess.Title
	if sess.Tasks != nil {
		tasksList = sess.Tasks.GetTasks()
		if g := sess.Tasks.GoalText(); g != "" {
			goal = g
		}
	}

	// Preserve empty string for General sessions instead of re-binding them to
	// whichever workspace happens to be active during a background turn completion.
	wsPath := agentsession.NormalizeWorkspacePath(sess.WorkspacePath)
	curWs := as.workspacePath
	execMode := as.ExecutionMode
	effort := as.ReasoningEffort
	var lastUsage *UsageInfo
	if sess.LastUsage != nil {
		lu := *sess.LastUsage
		lastUsage = &lu
	}

	updatedAt := sess.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = time.Now()
	}

	// Build the payload under the lock (cheap map writes) so every session
	// scalar is read consistently; only the encode + disk write run outside.
	data := map[string]interface{}{
		"id":             sess.ID,
		"title":          sess.Title,
		"goal":           goal,
		"status":         sess.Status,
		"createdAt":      sess.CreatedAt.Format(time.RFC3339),
		"updatedAt":      updatedAt.Format(time.RFC3339),
		"model":          sess.Model,
		"provider":       sess.Provider,
		"agentId":        sess.AgentID,
		"agentType":      sess.AgentType,
		"executionMode":  string(execMode),
		"reasoningLevel": string(effort),
		"workspacePath":  wsPath,
		"pinned":         sess.Pinned,
		"archived":       sess.Archived,
		"snoozed":        sess.Snoozed,
		"messages":       diskMsgs,
		"tasks":          tasksList,
	}
	remoteID := sess.RemoteSessionID
	checkpointID := sess.LastCheckpointTurnID
	as.mu.Unlock()

	if remoteID != "" {
		data["remoteSessionId"] = remoteID
	}
	if checkpointID != "" {
		data["lastCheckpointTurnId"] = checkpointID
	}
	if lastUsage != nil {
		data["lastUsage"] = lastUsage
	}

	diskWs := wsPath
	if diskWs == "" && agentsession.IsTempTestPath(curWs) {
		diskWs = curWs
	}

	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return
	}

	_ = agentsession.SaveSessionDisk(string(bytes), diskWs)
}

// ActiveSession returns the currently selected agent session, or nil if on the New Agent UI / landing screen.
func (as *AgentState) ActiveSession() *AgentSession {
	as.mu.Lock()
	defer as.mu.Unlock()

	if as.ActiveSessionID == "" {
		return nil
	}
	for _, s := range as.Sessions {
		if s.ID == as.ActiveSessionID {
			return s
		}
	}
	return nil
}

// FindSession returns the session with the matching ID, or nil if not found.
func (as *AgentState) FindSession(sessionID string) *AgentSession {
	as.mu.Lock()
	defer as.mu.Unlock()
	for _, s := range as.Sessions {
		if s.ID == sessionID {
			return s
		}
	}
	return nil
}

// GetLastUserPrompt returns the text of the most recent user prompt in the session.
func (as *AgentState) GetLastUserPrompt(sessionID string) string {
	as.mu.Lock()
	defer as.mu.Unlock()
	for _, s := range as.Sessions {
		if s.ID == sessionID {
			for i := len(s.Messages) - 1; i >= 0; i-- {
				if s.Messages[i] != nil && s.Messages[i].Role == "user" {
					return s.Messages[i].Text
				}
			}
			break
		}
	}
	return ""
}

// RestartACPSession restarts the backend ACP client process for the given session
// so it immediately adopts new credentials from Keychain/environment, while preserving
// the full conversation transcript and state.
func (as *AgentState) RestartACPSession(sessionID string) error {
	as.mu.Lock()
	var targetSess *AgentSession
	for _, s := range as.Sessions {
		if s.ID == sessionID {
			targetSess = s
			break
		}
	}
	if targetSess == nil {
		as.mu.Unlock()
		return fmt.Errorf("session not found: %s", sessionID)
	}
	as.mu.Unlock()

	var newSess *acp.Session
	var err error
	if as.ACPMgr != nil {
		newSess, err = as.ACPMgr.RestartSession(sessionID)
	} else if as.engine != nil && as.engine.ACPMgr != nil {
		newSess, err = as.engine.ACPMgr.RestartSession(sessionID)
	} else {
		return fmt.Errorf("no ACP manager available")
	}

	if err == nil && newSess != nil {
		as.syncACPSessionState(targetSess, newSess.Model, newSess.RemoteSessionID)
	}
	return err
}

func (as *AgentState) nextSessionIDLocked() string {
	base := fmt.Sprintf("session-%d", time.Now().UnixMilli())
	candidate := base
	for {
		conflict := false
		for _, s := range as.Sessions {
			if s.ID == candidate {
				conflict = true
				break
			}
		}
		if !conflict {
			return candidate
		}
		as.NextID++
		candidate = fmt.Sprintf("%s-%d", base, as.NextID)
	}
}

// ReplaceSessions atomically replaces the session slice (thread-safe).
func (as *AgentState) ReplaceSessions(sessions []*AgentSession) {
	as.mu.Lock()
	defer as.mu.Unlock()
	as.Sessions = sessions
}

// HasRunningSession reports whether the active session or any background session is streaming.
func (as *AgentState) HasRunningSession() bool {
	as.mu.Lock()
	defer as.mu.Unlock()
	if as.IsStreaming {
		return true
	}
	for _, s := range as.Sessions {
		if s.Status == "running" || s.Status == "streaming" {
			return true
		}
	}
	return false
}

// ResolveModelForAgent returns currentModel if supported by agentID, or that agent's default model.
func (as *AgentState) ResolveModelForAgent(agentID, currentModel string) string {
	if agentID == "" || agentID == "agent-internal" {
		return strings.TrimSpace(currentModel)
	}
	if as != nil && as.ACPMgr != nil {
		return as.ACPMgr.EnsureValidModel(agentID, currentModel)
	}
	return EnsureValidModelForACPAgent(agentID, currentModel)
}

// syncACPSessionState persists the effective ACP session state after a turn
// started: the remote conversation ID, and — the real current model — the
// model the agent process was actually configured with. StartSessionWithID
// normalizes unsupported selections back to the harness default, so the
// requested model and the effective one can differ; writing the effective
// value back keeps the UI's ActiveModel honest (it shows what the agent runs,
// never a guessed default).
func (as *AgentState) syncACPSessionState(sess *AgentSession, effModel, remoteSessionID string) {
	if sess == nil {
		return
	}
	effModel = strings.TrimSpace(effModel)
	as.mu.Lock()
	remote := strings.TrimSpace(remoteSessionID)
	modelChanged := false
	if remote != "" && !strings.HasPrefix(remote, "session-") && !strings.HasPrefix(remote, "acp-") && remote != sess.RemoteSessionID {
		sess.RemoteSessionID = remote
	}
	if effModel != "" && effModel != sess.Model {
		sess.Model = effModel
		modelChanged = true
	}
	if modelChanged && sess.ID == as.ActiveSessionID {
		as.ActiveModel = effModel
	}
	as.mu.Unlock()

	if as.Invalidate != nil {
		as.Invalidate()
	}
	goSave("SaveSessionToDisk", func() { as.SaveSessionToDisk(sess) })
}

// SessionModel returns the model recorded for a session — for ACP sessions
// the model actually configured on the agent process (written back on every
// turn start via syncACPSessionState), for internal sessions the bound
// provider model. UI consumers that address a specific (possibly background)
// session should prefer this over AgentState.ActiveModel, which only tracks
// the foreground session.
func (as *AgentState) SessionModel(sessionID string) string {
	if as == nil {
		return ""
	}
	as.mu.Lock()
	defer as.mu.Unlock()
	for _, s := range as.Sessions {
		if s.ID == sessionID {
			return s.Model
		}
	}
	return ""
}

// SelectACPAgent switches the active harness to agentID and auto-selects its default model and effort.
func (as *AgentState) SelectACPAgent(agentID string) string {
	if as == nil {
		return ""
	}
	normID := normalizeACPAgentLookupID(agentID)
	var model string
	var effort ReasoningEffort
	isInternal := normID == "agent-internal" || strings.EqualFold(agentID, "internal")
	if isInternal {
		normID = "agent-internal"
		if as.engine != nil && as.engine.LLMClient != nil {
			model = as.engine.LLMClient.GetConfig().Model
		}
	} else {
		if as.ACPMgr != nil {
			model, _, effort = as.ACPMgr.ResolveDefaultModel(normID)
		}
		if model == "" {
			model, _, effort = DefaultModelForACPAgentWithType(normID)
		}
	}

	as.mu.Lock()
	as.ActiveAgentID = normID
	if isInternal {
		as.ActiveAgentType = "internal"
	} else {
		as.ActiveAgentType = "acp"
	}
	if model != "" {
		as.ActiveModel = model
	}
	if effort != "" {
		as.ReasoningEffort = effort
	}
	outModel := as.ActiveModel
	as.mu.Unlock()

	if as.Invalidate != nil {
		as.Invalidate()
	}
	return outModel
}

// syncActiveSession updates the ActiveSessionID and points Messages/IsStreaming to the active session.
// Callers hold as.mu. Any RK LLMClient call is deferred to applyPendingLLMModel
// which the caller invokes AFTER releasing as.mu (P0-1 layer 3: never call
// into the RakitKode engine while holding as.mu).
func (as *AgentState) syncActiveSession() {
	as.PromptHistoryIdx = -1
	as.PromptHistoryDraft = ""
	if as.ActiveSessionID == "" {
		as.Messages = nil
		as.IsStreaming = false
		return
	}
	var active *AgentSession
	for _, s := range as.Sessions {
		if s.ID == as.ActiveSessionID {
			active = s
			break
		}
	}
	if active != nil {
		as.Messages = active.Messages
		as.workspacePath = active.WorkspacePath
		as.IsStreaming = active.Status == "running" || active.Status == "streaming"
		as.SteerQueue = append([]string(nil), active.SteerQueue...)
		if active.Tasks != nil {
			as.TaskManager = active.Tasks
		} else {
			as.TaskManager = NewTaskManager()
		}
		if active.Model != "" {
			as.ActiveModel = active.Model
		} else if as.ActiveModel != "" {
			active.Model = as.ActiveModel
		}
		if active.Provider != "" {
			as.ActiveProvider = active.Provider
		} else if as.ActiveProvider != "" {
			active.Provider = as.ActiveProvider
		}
		if (active.AgentType == "internal" || active.AgentType == "") && as.engine != nil && as.engine.LLMClient != nil {
			if as.ActiveProvider != "" && as.ActiveModel != "" {
				as.pendingLLMProvider = as.ActiveProvider
				as.pendingLLMModel = as.ActiveModel
			}
		}
		if active.AgentID != "" {
			as.ActiveAgentID = active.AgentID
		}
		if active.AgentType != "" {
			as.ActiveAgentType = active.AgentType
		}
	} else {
		as.Messages = nil
		as.IsStreaming = false
		as.SteerQueue = nil
	}
}

// applyPendingLLMModel flushes a model/provider binding recorded by
// syncActiveSession. Must be called by syncActiveSession's callers AFTER
// they release as.mu: SetActiveModel takes RakitKode's LLM client lock,
// which the streaming path also holds (P0-1 layer 3).
func (as *AgentState) applyPendingLLMModel() {
	as.mu.Lock()
	prov := as.pendingLLMProvider
	model := as.pendingLLMModel
	as.pendingLLMProvider = ""
	as.pendingLLMModel = ""
	client := (*llm.LLMClient)(nil)
	if as.engine != nil {
		client = as.engine.LLMClient
	}
	as.mu.Unlock()
	if client != nil && prov != "" && model != "" {
		client.SetActiveModel(prov, model)
	}
}

// ClearActiveSession resets active session to empty (displaying the New Agent UI)
// without interrupting or mutating any background session that is currently running.
func (as *AgentState) ClearActiveSession() {
	as.mu.Lock()
	prevID := as.ActiveSessionID
	if prevID != "" {
		for _, s := range as.Sessions {
			if s.ID == prevID {
				s.SteerQueue = append([]string(nil), as.SteerQueue...)
				break
			}
		}
	}
	as.ActiveSessionID = ""
	as.Messages = nil
	as.IsStreaming = false
	as.InputText = ""
	as.SteerQueue = nil
	as.TaskManager = NewTaskManager()
	as.mu.Unlock()

	if prevID != "" {
		as.SwapComposerDraft(prevID, "")
	}

	if as.Invalidate != nil {
		as.Invalidate()
	}
}

// ForkSession duplicates the given session into a new active session.
func (as *AgentState) ForkSession(sess *AgentSession) *AgentSession {
	if sess == nil {
		return nil
	}
	as.mu.Lock()

	now := time.Now()
	id := as.nextSessionIDLocked()
	forkTitle := fmt.Sprintf("%s (fork)", sess.Title)

	msgsCopy := make([]*ChatMessage, 0, len(sess.Messages))
	for _, m := range sess.Messages {
		if m == nil {
			continue
		}
		cp := *m
		cp.PendingApproval = nil
		if len(m.Tools) > 0 {
			cp.Tools = make([]*ToolExecution, 0, len(m.Tools))
			for _, t := range m.Tools {
				if t == nil {
					continue
				}
				tc := *t
				cp.Tools = append(cp.Tools, &tc)
			}
		}
		if len(m.Changes) > 0 {
			cp.Changes = make([]*FileChange, 0, len(m.Changes))
			for _, ch := range m.Changes {
				if ch == nil {
					continue
				}
				cc := *ch
				cp.Changes = append(cp.Changes, &cc)
			}
		}
		if len(m.PendingQuestions) > 0 {
			cp.PendingQuestions = make([]AgentQuestion, len(m.PendingQuestions))
			for i, q := range m.PendingQuestions {
				qc := q
				qc.Options = append([]string(nil), q.Options...)
				qc.Selected = append([]string(nil), q.Selected...)
				cp.PendingQuestions[i] = qc
			}
		}
		msgsCopy = append(msgsCopy, &cp)
	}

	tasksCopy := NewTaskManager()
	if sess.Tasks != nil && len(sess.Tasks.Tasks) > 0 {
		items := append([]TaskItem(nil), sess.Tasks.Tasks...)
		tasksCopy.SetGoal(sess.Tasks.Goal, items)
	}

	forked := &AgentSession{
		ID:            id,
		Title:         forkTitle,
		Status:        "idle",
		CreatedAt:     now,
		UpdatedAt:     now,
		Model:         sess.Model,
		Provider:      sess.Provider,
		AgentID:       sess.AgentID,
		AgentType:     sess.AgentType,
		WorkspacePath: sess.WorkspacePath,
		Pinned:        sess.Pinned,
		Messages:      msgsCopy,
		Tasks:         tasksCopy,
		nextMsgID:     sess.nextMsgID, // copied IDs stay unique across the fork
	}

	as.Sessions = append([]*AgentSession{forked}, as.Sessions...)
	as.ActiveSessionID = id
	as.Messages = forked.Messages
	as.TaskManager = forked.Tasks
	as.IsStreaming = false
	as.mu.Unlock()
	return forked
}

// ForkSessionByID forks the session with the given id (sidebar fork action).
func (as *AgentState) ForkSessionByID(sessionID string) {
	as.mu.Lock()
	var target *AgentSession
	for _, s := range as.Sessions {
		if s.ID == sessionID {
			target = s
			break
		}
	}
	as.mu.Unlock()
	if target != nil {
		if forked := as.ForkSession(target); forked != nil {
			as.SaveSessionToDisk(forked)
		}
		if as.Invalidate != nil {
			as.Invalidate()
		}
	}
}

// SessionDiffStats sums every turn's file-change counts, powering the +n/−n
// badge the sidebar TaskRow shows, as forge-ade's diff-stats chip does.
func (as *AgentState) SessionDiffStats(sessionID string) (added, removed int) {
	as.mu.Lock()
	defer as.mu.Unlock()
	for _, s := range as.Sessions {
		if s.ID != sessionID {
			continue
		}
		for _, m := range s.Messages {
			for _, ch := range m.Changes {
				if ch.Additions > 0 {
					added += ch.Additions
				}
				if ch.Deletions > 0 {
					removed += ch.Deletions
				}
			}
		}
		break
	}
	return added, removed
}

// CreateSessionWithInitialPrompt creates a new session with an initial user prompt and starts the turn.
func (as *AgentState) CreateSessionWithInitialPrompt(prompt, agentID string) *AgentSession {
	now := time.Now()
	title := prompt
	if len(title) > 42 {
		title = strings.TrimSpace(title[:42]) + "…"
	}
	if title == "" {
		title = "New Task"
	}
	userMsg := &ChatMessage{
		ID:        1,
		Role:      "user",
		Text:      prompt,
		Done:      true,
		Timestamp: now,
	}

	as.mu.Lock()
	if prevID := as.ActiveSessionID; prevID != "" {
		for _, s := range as.Sessions {
			if s.ID == prevID {
				s.SteerQueue = append([]string(nil), as.SteerQueue...)
				break
			}
		}
	}
	id := as.nextSessionIDLocked()
	if agentID == "" || agentID == "agent-monolith" {
		agentID = as.ActiveAgentID
	}
	if agentID == "" {
		agentID = "agent-agy"
	}
	agentType := as.ActiveAgentType
	if agentType == "" {
		if agentID == "agent-internal" {
			agentType = "internal"
		} else {
			agentType = "acp"
		}
	}
	model := as.ActiveModel
	if isACPAgentID(agentID, agentType) {
		model = as.ResolveModelForAgent(agentID, model)
		if model != "" {
			as.ActiveModel = model
		}
	}
	if model == "" {
		model = "gemini-3.8-flash-high"
	}

	prov := as.ActiveProvider
	if prov == "" && as.engine != nil && as.engine.LLMClient != nil {
		prov = as.engine.LLMClient.GetConfig().ProviderID
	}

	sess := &AgentSession{
		ID:            id,
		Title:         title,
		Status:        "idle",
		CreatedAt:     now,
		UpdatedAt:     now,
		Model:         model,
		Provider:      prov,
		AgentID:       agentID,
		AgentType:     agentType,
		WorkspacePath: as.workspacePath,
		Messages:      []*ChatMessage{userMsg},
		Tasks:         NewTaskManager(),
	}

	as.Sessions = append([]*AgentSession{sess}, as.Sessions...)
	as.ActiveSessionID = id
	as.Messages = sess.Messages
	as.TaskManager = sess.Tasks
	as.SteerQueue = nil
	as.IsStreaming = false
	as.mu.Unlock()

	as.SaveSessionToDisk(sess)
	as.triggerAgentTurnForSession(id, prompt)

	if as.Invalidate != nil {
		as.Invalidate()
	}
	return sess
}

// TriggerTurn begins an AI agent turn with the given prompt for the active session.
func (as *AgentState) TriggerTurn(prompt string) {
	as.mu.Lock()
	targetSessID := as.ActiveSessionID
	as.mu.Unlock()
	as.triggerAgentTurnForSession(targetSessID, prompt)
}

// TriggerTurnForSession begins an AI agent turn for a specific session ID (safe when running in parallel).
func (as *AgentState) TriggerTurnForSession(sessionID, prompt string, attached ...string) {
	as.triggerAgentTurnForSession(sessionID, prompt, attached...)
}

// SelectSession activates the given session by ID.
func (as *AgentState) SelectSession(id string) {
	as.SwapComposerDraft(as.ActiveSessionID, id)
	as.mu.Lock()
	if prevID := as.ActiveSessionID; prevID != "" && prevID != id {
		for _, s := range as.Sessions {
			if s.ID == prevID {
				s.SteerQueue = append([]string(nil), as.SteerQueue...)
				break
			}
		}
	}
	as.ActiveSessionID = id
	as.syncActiveSession()
	as.mu.Unlock()
	as.applyPendingLLMModel()

	if as.Invalidate != nil {
		as.Invalidate()
	}
}

// NewSession creates and activates a new empty session with a real TaskManager.
func (as *AgentState) NewSession(title string) *AgentSession {
	as.mu.Lock()
	defer as.mu.Unlock()

	if prevID := as.ActiveSessionID; prevID != "" {
		for _, s := range as.Sessions {
			if s.ID == prevID {
				s.SteerQueue = append([]string(nil), as.SteerQueue...)
				break
			}
		}
	}

	now := time.Now()
	id := as.nextSessionIDLocked()

	agentID := as.ActiveAgentID
	if agentID == "" || agentID == "agent-monolith" {
		agentID = "agent-agy"
	}
	agentType := as.ActiveAgentType
	if agentType == "" {
		if agentID == "agent-internal" {
			agentType = "internal"
		} else {
			agentType = "acp"
		}
	}
	model := as.ActiveModel
	if isACPAgentID(agentID, agentType) {
		model = as.ResolveModelForAgent(agentID, model)
		if model != "" {
			as.ActiveModel = model
		}
	}

	prov := as.ActiveProvider
	if prov == "" && as.engine != nil && as.engine.LLMClient != nil {
		prov = as.engine.LLMClient.GetConfig().ProviderID
	}

	sess := &AgentSession{
		ID:            id,
		Title:         title,
		Status:        "idle",
		CreatedAt:     now,
		UpdatedAt:     now,
		Model:         model,
		Provider:      prov,
		AgentID:       agentID,
		AgentType:     agentType,
		WorkspacePath: as.workspacePath,
		Messages:      nil,
		Tasks:         NewTaskManager(),
	}

	as.Sessions = append([]*AgentSession{sess}, as.Sessions...)
	as.ActiveSessionID = id
	as.Messages = sess.Messages
	as.TaskManager = sess.Tasks
	as.SteerQueue = nil
	as.IsStreaming = false

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

// ToggleSnooze snoozes or unsnoozes the given session.
func (as *AgentState) ToggleSnooze(id string) {
	as.mu.Lock()
	var updated *AgentSession
	for _, s := range as.Sessions {
		if s.ID == id {
			s.Snoozed = !s.Snoozed
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
	if as.deletedIDs == nil {
		as.deletedIDs = make(map[string]bool)
	}
	as.deletedIDs[id] = true
	var deletedWs string
	newSessions := make([]*AgentSession, 0, len(as.Sessions))
	for _, s := range as.Sessions {
		if s.ID == id {
			deletedWs = s.WorkspacePath
			continue
		}
		newSessions = append(newSessions, s)
	}
	as.Sessions = newSessions
	if as.ActiveSessionID == id {
		nextID := ""
		for _, s := range as.Sessions {
			if !s.Archived && !s.Snoozed && s.WorkspacePath == deletedWs {
				nextID = s.ID
				break
			}
		}
		as.ActiveSessionID = nextID
	}
	curWs := as.workspacePath
	as.syncActiveSession()
	as.mu.Unlock()
	as.applyPendingLLMModel()

	diskWs := deletedWs
	if diskWs == "" && agentsession.IsTempTestPath(curWs) {
		diskWs = curWs
	}
	_ = agentsession.DeleteSessionDisk(id, diskWs)

	if as.Invalidate != nil {
		as.Invalidate()
	}
}

// DeleteSessionsByProject removes all ACP sessions belonging to a workspace project
// across all tabs (active, pinned, snoozed, archived) and deletes its on-disk folder.
func (as *AgentState) DeleteSessionsByProject(projectName, folderPath string) {
	normFolder := agentsession.NormalizeWorkspacePath(folderPath)
	if strings.EqualFold(projectName, "General") || (normFolder == "" && projectName == "") {
		return
	}

	as.mu.Lock()
	if as.deletedIDs == nil {
		as.deletedIDs = make(map[string]bool)
	}
	curWs := as.workspacePath
	newSessions := make([]*AgentSession, 0, len(as.Sessions))
	var deleted []struct {
		id string
		ws string
	}
	activeDeleted := false
	for _, s := range as.Sessions {
		if s.WorkspacePath == "" {
			newSessions = append(newSessions, s)
			continue
		}
		match := (normFolder != "" && s.WorkspacePath == normFolder) ||
			(normFolder == "" && projectName != "" && filepath.Base(filepath.Clean(s.WorkspacePath)) == projectName)
		if match {
			as.deletedIDs[s.ID] = true
			deleted = append(deleted, struct {
				id string
				ws string
			}{id: s.ID, ws: s.WorkspacePath})
			if as.ActiveSessionID == s.ID {
				activeDeleted = true
			}
			continue
		}
		newSessions = append(newSessions, s)
	}
	as.Sessions = newSessions
	if activeDeleted {
		as.ActiveSessionID = ""
		for _, s := range as.Sessions {
			if !s.Archived && !s.Snoozed && s.WorkspacePath == "" {
				as.ActiveSessionID = s.ID
				break
			}
		}
	}
	as.syncActiveSession()
	as.mu.Unlock()
	as.applyPendingLLMModel()

	for _, d := range deleted {
		diskWs := d.ws
		if diskWs == "" && agentsession.IsTempTestPath(curWs) {
			diskWs = curWs
		}
		_ = agentsession.DeleteSessionDisk(d.id, diskWs)
	}
	_ = agentsession.DeleteProjectSessionsDisk(projectName, normFolder)

	if as.Invalidate != nil {
		as.Invalidate()
	}
}

// RenameSession updates a session's title and persists it to disk.
func (as *AgentState) RenameSession(id, title string) {
	title = strings.TrimSpace(title)
	if id == "" || title == "" {
		return
	}
	as.mu.Lock()
	var renamed *AgentSession
	for _, s := range as.Sessions {
		if s.ID == id {
			s.Title = title
			renamed = s
			break
		}
	}
	as.syncActiveSession()
	as.mu.Unlock()
	as.applyPendingLLMModel()

	if renamed != nil {
		as.SaveSessionToDisk(renamed)
		if as.Invalidate != nil {
			as.Invalidate()
		}
	}
}

// AllSessions returns a thread-safe snapshot of all loaded ACP sessions.
func (as *AgentState) AllSessions() []*AgentSession {
	as.mu.Lock()
	defer as.mu.Unlock()
	out := make([]*AgentSession, len(as.Sessions))
	copy(out, as.Sessions)
	return out
}

// TabCounts returns session counts for active, pinned, snoozed, and archived tabs.
func (as *AgentState) TabCounts() (active, pinned, snoozed, archived int) {
	as.mu.Lock()
	defer as.mu.Unlock()

	for _, s := range as.Sessions {
		switch {
		case s.Archived:
			archived++
		case s.Snoozed:
			snoozed++
		default:
			active++
			if s.Pinned {
				pinned++
			}
		}
	}
	return
}

// ContextUsage calculates the approximate tokens used and percentage of standard 200k window.
// ContextWindowTokens returns the context window the ACTIVE session's
// harness actually offers. Defaults to 200k; Antigravity-class agents run a
// 1M-token window (the hardcoded 200k made the gauge hit red at ~20% real
// usage on agy).
func (as *AgentState) ContextWindowTokens() int {
	as.mu.Lock()
	defer as.mu.Unlock()
	for _, s := range as.Sessions {
		if s.ID != as.ActiveSessionID {
			continue
		}
		id := strings.ToLower(s.AgentID + " " + s.AgentType)
		switch {
		case strings.Contains(id, "agy") || strings.Contains(id, "antigravity"):
			return 1_000_000
		case strings.Contains(id, "codex"):
			return 272_000
		}
		return 200_000
	}
	return 200_000
}

func (as *AgentState) ContextUsage() (tokens int, pct int) {
	as.mu.Lock()
	defer as.mu.Unlock()

	maxContext := as.contextWindowLocked()

	// Prefer the usage report the agent streams (agent:usage / turn_end).
	for _, s := range as.Sessions {
		if s.ID == as.ActiveSessionID && s.LastUsage != nil && s.LastUsage.TotalTokens > 0 {
			tokens = s.LastUsage.TotalTokens
			pct = (tokens * 100) / maxContext
			if pct == 0 {
				pct = 1
			}
			return
		}
	}

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
	pct = (tokens * 100) / maxContext
	if pct == 0 && tokens > 0 {
		pct = 1
	}
	return
}

// contextWindowLocked is ContextWindowTokens for callers already holding
// as.mu. Callers without the lock use ContextWindowTokens.
func (as *AgentState) contextWindowLocked() int {
	for _, s := range as.Sessions {
		if s.ID != as.ActiveSessionID {
			continue
		}
		id := strings.ToLower(s.AgentID + " " + s.AgentType)
		switch {
		case strings.Contains(id, "agy") || strings.Contains(id, "antigravity"):
			return 1_000_000
		case strings.Contains(id, "codex"):
			return 272_000
		}
		return 200_000
	}
	return 200_000
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
			if !s.Snoozed || s.Archived {
				continue
			}
		default: // "active"
			if s.Archived || s.Snoozed {
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
	if text == "" && len(as.AttachedFiles) == 0 {
		return
	}

	// Snapshot attached files before clearing so triggers still pass mentions.
	attached := append([]string(nil), as.AttachedFiles...)
	if len(attached) > 0 {
		var b strings.Builder
		b.WriteString("Attached files:\n")
		for _, f := range attached {
			b.WriteString(fmt.Sprintf("@%s\n", f))
		}
		b.WriteString("\n")
		if text != "" {
			b.WriteString(text)
		} else {
			b.WriteString("Please review the attached files.")
		}
		text = b.String()
		as.AttachedFiles = nil
	}

	// Determine whether the currently viewed session is streaming; when on the
	// New Task screen (ActiveSessionID == ""), always launch a fresh parallel session.
	as.mu.Lock()
	var activeSess *AgentSession
	for _, s := range as.Sessions {
		if s.ID == as.ActiveSessionID {
			activeSess = s
			break
		}
	}
	isActiveStreaming := activeSess != nil && (activeSess.Status == "running" || activeSess.Status == "streaming" || as.IsStreaming)
	as.mu.Unlock()

	// Mid-turn submissions ALWAYS queue onto the session's steer queue: the
	// prompt shows up in the steer tray (queued text + Steer button + edit /
	// delete) for review, and pressing Steer injects it into the running
	// RakitKode loop via SteerInternal (RemoveSteerItem pops the entry).
	// Injecting directly here bypassed the queue and silently removed the
	// whole tray UX (user-reported regression).
	if isActiveStreaming {
		as.mu.Lock()
		activeSess.SteerQueue = append(activeSess.SteerQueue, text)
		if activeSess.ID == as.ActiveSessionID {
			as.SteerQueue = append([]string(nil), activeSess.SteerQueue...)
		}
		steerSnapshot := append([]string(nil), activeSess.SteerQueue...)
		as.InputText = ""
		as.mu.Unlock()
		as.saveSteerQueuesSlice(steerSnapshot)
		if as.Invalidate != nil {
			as.Invalidate()
		}
		return
	}

	as.InputText = ""

	as.mu.Lock()
	activeSess = nil
	for _, s := range as.Sessions {
		if s.ID == as.ActiveSessionID {
			activeSess = s
			break
		}
	}
	if activeSess == nil {
		now := time.Now()
		id := as.nextSessionIDLocked()
		title := text
		if len(title) > 42 {
			title = strings.TrimSpace(title[:42]) + "…"
		}
		if title == "" {
			title = "New Task"
		}
		agentID := as.ActiveAgentID
		if agentID == "" || agentID == "agent-monolith" {
			agentID = "agent-agy"
		}
		agentType := as.ActiveAgentType
		if agentType == "" {
			if agentID == "agent-internal" {
				agentType = "internal"
			} else {
				agentType = "acp"
			}
		}
		model := as.ActiveModel
		if isACPAgentID(agentID, agentType) {
			model = as.ResolveModelForAgent(agentID, model)
			if model != "" {
				as.ActiveModel = model
			}
		}
		if model == "" {
			model = "gemini-3.8-flash-high"
		}
		activeSess = &AgentSession{
			ID:            id,
			Title:         title,
			Status:        "idle",
			CreatedAt:     now,
			UpdatedAt:     now,
			Model:         model,
			Provider:      "",
			AgentID:       agentID,
			AgentType:     agentType,
			WorkspacePath: as.workspacePath,
			Messages:      nil,
			Tasks:         NewTaskManager(),
		}
		as.Sessions = append([]*AgentSession{activeSess}, as.Sessions...)
		as.ActiveSessionID = id
		as.TaskManager = activeSess.Tasks
		as.SteerQueue = nil
	}

	targetSessID := activeSess.ID
	userMsg := &ChatMessage{
		ID:        nextMessageIDLocked(activeSess),
		Role:      "user",
		Text:      text,
		Done:      true,
		Timestamp: time.Now(),
	}
	activeSess.Messages = append(activeSess.Messages, userMsg)
	if activeSess.ID == as.ActiveSessionID {
		as.Messages = activeSess.Messages
	}
	activeSess.UpdatedAt = time.Now()
	as.mu.Unlock()

	if activeSess != nil {
		as.SaveSessionToDisk(activeSess)
	}
	as.AppendPromptHistory(text)

	// Slash commands: /clear and /help are native; every other command goes
	// to the agent, exactly as forge-ade sends them.
	if strings.HasPrefix(text, "/") {
		root := text
		if i := strings.IndexAny(text, " \n"); i >= 0 {
			root = text[:i]
		}
		switch root {
		case "/clear":
			// Shared locked implementation (nils session + alias, resets
			// streaming, persists async) — same path as the slash menu.
			as.ClearConversation()
			return
		case "/help":
			as.mu.Lock()
			for _, s := range as.Sessions {
				if s.ID == targetSessID {
					help := "### ForgeADE Commands\n" +
						"- `/clear` — clear this conversation\n" +
						"- `/help` — this reference\n\n" +
						"Everything else is sent to the agent. The agent's own commands"
					if len(s.AvailableCommands) > 0 {
						help += " (from the agent):"
						for _, c := range s.AvailableCommands {
							help += fmt.Sprintf("\n- `/%s`", c)
						}
					} else {
						help += " arrive as they are advertised (`/goal`, `/plan`, `/compact`, ...)."
					}
					s.Messages = append(s.Messages, &ChatMessage{
						ID:        nextMessageIDLocked(s),
						Role:      "assistant",
						Text:      help,
						Done:      true,
						Timestamp: time.Now(),
					})
					if s.ID == as.ActiveSessionID {
						as.Messages = s.Messages
					}
					break
				}
			}
			as.mu.Unlock()
			if as.Invalidate != nil {
				as.Invalidate()
			}
			return
		}
	}

	// Trigger streaming AI turn scoped specifically to targetSessID
	as.triggerAgentTurnForSession(targetSessID, text, attached...)

	if as.Invalidate != nil {
		as.Invalidate()
	}
}

// CancelStreaming aborts the active session's running turn without disturbing
// other parallel sessions: it interrupts the ACP agent, stops the internal
// RakitKode agent turn, and finalizes that session's streaming message.
func (as *AgentState) CancelStreaming() {
	as.mu.Lock()
	as.IsStreaming = false
	sessionID := as.ActiveSessionID
	var cancelSess *AgentSession
	isInternal := false
	for _, s := range as.Sessions {
		if s.ID == sessionID {
			cancelSess = s
			if s.AgentType == "internal" && s.RemoteSessionID != "" {
				isInternal = true
			}
			break
		}
	}
	as.mu.Unlock()

	if sessionID == "" {
		return
	}

	// Abort an in-flight raw-LLM fallback stream first: its context is local
	// to the turn goroutine and is otherwise unreachable (tokens would keep
	// burning after Stop).
	as.cancelStreamFor(sessionID)

	if isInternal {
		as.StopInternalTurn(sessionID)
	} else if as.ACPMgr != nil {
		_ = as.ACPMgr.Cancel(sessionID)
	}

	// The internal LLM path has no event to finalize; close only this session's message.
	as.mu.Lock()
	if cancelSess != nil {
		if len(cancelSess.Messages) > 0 {
			msg := cancelSess.Messages[len(cancelSess.Messages)-1]
			if msg.Role == "assistant" && !msg.Done {
				msg.Done = true
				msg.IsThinking = false
				msg.ShowThinking = false
				msg.ShowTools = false
				if !msg.TurnStartedAt.IsZero() {
					msg.ThinkingTimeSec = time.Since(msg.TurnStartedAt).Seconds()
				}
				if strings.TrimSpace(msg.Text) == "" {
					setAssistantTextLocked(msg, "*(stopped by user)*")
				}
			}
			// A stopped turn can no longer answer its approvals: mark them
			// resolved and drop the queue so the bar does not outlive the
			// turn (the requests are moot once the harness is cancelled).
			for _, m := range cancelSess.Messages {
				if m.PendingApproval != nil && !m.PendingApproval.Resolved {
					m.PendingApproval.Resolved = true
				}
			}
			cancelSess.PendingApprovals = nil
		}
		if cancelSess.Status == "running" || cancelSess.Status == "streaming" {
			cancelSess.Status = "idle"
		}
		if cancelSess.ID == as.ActiveSessionID {
			as.Messages = cancelSess.Messages
		}
	}
	as.mu.Unlock()

	if cancelSess != nil {
		as.SaveSessionToDisk(cancelSess)
	}
	if as.Invalidate != nil {
		as.Invalidate()
	}
}

// gitStatusPaths returns the set of files git reports as changed, for the
// turn-change diff (files modified during one turn).
func (as *AgentState) gitStatusPaths() map[string]bool {
	as.mu.Lock()
	ws := as.workspacePath
	as.mu.Unlock()
	return as.gitStatusPathsForWorkspace(ws)
}

func (as *AgentState) gitStatusPathsForWorkspace(wsPath string) map[string]bool {
	out := map[string]bool{}
	if as.engine == nil || as.engine.GitEngine == nil || wsPath == "" {
		return out
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	res, err := as.engine.GitEngine.GetStatus(ctx, wsPath)
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

// Lock acquires the agent state mutex.
func (as *AgentState) Lock() {
	as.mu.Lock()
}

// Unlock releases the agent state mutex.
func (as *AgentState) Unlock() {
	as.mu.Unlock()
}

// SetWorkspacePath updates the active workspace path.
func (as *AgentState) SetWorkspacePath(dir string) {
	norm := agentsession.NormalizeWorkspacePath(dir)
	as.mu.Lock()
	as.workspacePath = norm
	as.mu.Unlock()
}

// WorkspacePath returns the active workspace path.
func (as *AgentState) WorkspacePath() string {
	as.mu.Lock()
	defer as.mu.Unlock()
	return as.workspacePath
}

// Engine returns the engine instance.
func (as *AgentState) Engine() *engine.Engine {
	return as.engine
}

// ComputeChangeStats fills real +N/−N counts from the file's git diff.
func (as *AgentState) ComputeChangeStats(changes []*FileChange) {
	as.mu.Lock()
	ws := as.workspacePath
	as.mu.Unlock()
	as.computeChangeStatsForWorkspace(ws, changes)
}

// computeChangeStats fills real +N/−N counts from the file's git diff.
func (as *AgentState) computeChangeStats(changes []*FileChange) {
	as.ComputeChangeStats(changes)
}

// maxChangeStatsFiles caps the per-file git diffs computed for one turn: a
// 500-file refactor would otherwise serially spawn 500 git subprocesses
// (10s timeout each) in the turn-completion path (P0-1 layer 2).
const maxChangeStatsFiles = 50

func (as *AgentState) computeChangeStatsForWorkspace(wsPath string, changes []*FileChange) {
	if as.engine == nil || wsPath == "" {
		return
	}
	computed := 0
	for _, ch := range changes {
		if ch.Additions >= 0 {
			continue
		}
		if computed >= maxChangeStatsFiles {
			break // remaining entries stay at -1 (rendered as changed, no counts)
		}
		computed++
		patch := as.engine.GetGitDiff(wsPath, ch.Path)
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

// triggerAgentTurnForSession executes real streaming responses via LLMClient or ACPManager
// scoped to targetSessID so multiple sessions can stream concurrently without interference.
func (as *AgentState) triggerAgentTurnForSession(targetSessID, prompt string, attached ...string) {
	as.mu.Lock()
	if targetSessID == "" {
		targetSessID = as.ActiveSessionID
	}
	var targetSess *AgentSession
	for _, s := range as.Sessions {
		if s.ID == targetSessID {
			targetSess = s
			break
		}
	}
	if targetSess == nil {
		as.mu.Unlock()
		return
	}
	sessWs := targetSess.WorkspacePath
	if sessWs == "" {
		sessWs = as.workspacePath
	}
	as.mu.Unlock()

	turnStartFiles := as.gitStatusPathsForWorkspace(sessWs)

	as.mu.Lock()
	targetSess = nil
	for _, s := range as.Sessions {
		if s.ID == targetSessID {
			targetSess = s
			break
		}
	}
	if targetSess == nil {
		as.mu.Unlock()
		return
	}

	startTime := time.Now()
	assistantMsg := &ChatMessage{
		ID:              nextMessageIDLocked(targetSess),
		Role:            "assistant",
		Text:            "",
		Done:            false,
		Timestamp:       startTime,
		TurnStartedAt:   startTime,
		IsThinking:      true,
		ThinkingTimeSec: 0,
		ShowThinking:    false,
		ShowTools:       false,
	}

	// A stale approval gate from the previous turn must not leak into this one.
	if len(targetSess.Messages) > 0 {
		if prev := targetSess.Messages[len(targetSess.Messages)-1]; prev.PendingApproval != nil {
			prev.PendingApproval.Resolved = true
		}
	}
	targetSess.PendingApprovals = nil
	targetSess.Messages = append(targetSess.Messages, assistantMsg)
	targetSess.Status = "running"
	targetSess.turnStartFiles = turnStartFiles
	if targetSess.ID == as.ActiveSessionID {
		as.Messages = targetSess.Messages
		as.IsStreaming = true
	}

	permissionMode := as.ExecutionMode.PermissionMode()
	hasInternalKey := hasConfiguredInternalLLM(as.engine)

	// 1. Check if ACP session can handle this turn
	isACPAgent := isACPAgentID(targetSess.AgentID, targetSess.AgentType)

	// If no internal LLM provider is configured, automatically route turn to installed ACP agent
	if !isACPAgent && !hasInternalKey {
		targetSess.AgentID = "agent-agy"
		targetSess.AgentType = "acp"
		if targetSess.Model == "" || targetSess.Model == "gpt-4o" || strings.Contains(targetSess.Model, "gpt-4o") {
			targetSess.Model = "gemini-3.8-flash-high"
		}
		isACPAgent = true
	}
	as.mu.Unlock()

	if isACPAgent {
		go func() {
			// A panicking turn goroutine must not kill the app (P1-6): log
			// and leave the session to the user's Stop button.
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[agent] ACP turn goroutine panicked (session %s): %v\n%s", targetSessID, r, debugStack())
				}
			}()
			turnID := fmt.Sprintf("turn-%d", time.Now().UnixMilli())
			as.mu.Lock()
			targetSess.LastCheckpointTurnID = turnID
			runWs := targetSess.WorkspacePath
			if runWs == "" {
				runWs = sessWs
			}
			sessID := targetSess.ID
			agentID := targetSess.AgentID
			model := targetSess.Model
			remoteID := targetSess.RemoteSessionID
			as.mu.Unlock()

			if as.engine != nil && as.engine.GitEngine != nil && runWs != "" {
				ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
				_, _ = internalgit.CreateCheckpoint(ctx, runWs, sessID, turnID, prompt)
				cancel()
			}

			var startErr error
			var acpSess *acp.Session
			var effModel, effRemote string
			if as.ACPMgr != nil {
				var err error
				acpSess, err = as.ACPMgr.StartSessionWithID(sessID, agentID, runWs, model, permissionMode, remoteID)
				startErr = err
				if startErr == nil {
					startErr = as.ACPMgr.Prompt(sessID, prompt)
				}
			} else if as.engine != nil && as.engine.ACPMgr != nil {
				var err error
				acpSess, err = as.engine.ACPMgr.StartSession(acp.SessionOpts{
					SessionID:       sessID,
					AgentID:         agentID,
					Cwd:             runWs,
					Model:           model,
					PermissionMode:  permissionMode,
					ResumeSessionID: remoteID,
				})
				startErr = err
				if startErr == nil {
					startErr = as.engine.ACPMgr.Prompt(sessID, prompt)
				}
			} else {
				startErr = fmt.Errorf("no ACP manager available")
			}
			if startErr == nil && acpSess != nil {
				// Snapshot the two fields by value immediately after start:
				// the manager wrote them under its own lock during creation,
				// and later protocol events may rewrite them on another
				// goroutine — the live-pointer read would race.
				effModel = strings.TrimSpace(acpSess.Model)
				effRemote = strings.TrimSpace(acpSess.RemoteSessionID)
				as.syncACPSessionState(targetSess, effModel, effRemote)
			}

			if startErr != nil {
				as.mu.Lock()
				assistantMsg.Done = true
				assistantMsg.IsThinking = false
				assistantMsg.ShowThinking = false
				assistantMsg.ShowTools = false
				if as.ActiveSessionID == targetSessID {
					as.IsStreaming = false
				}
				targetSess.Status = "error"
				targetSess.turnStartFiles = nil
				assistantMsg.Text = fmt.Sprintf("ACP agent **%s** error: %v\n\nPlease ensure the CLI executable is installed on PATH (or select another agent harness in ⌘M).", agentID, startErr)
				as.mu.Unlock()
				if as.Invalidate != nil {
					as.Invalidate()
				}
				return
			}
			// ACP agent is now executing asynchronously.
			// Chunks stream via events -> HandleTokenChunk; completion is
			// handled by HandleTurnDone (status, changes, steer queue).
		}()
		return
	}

	// 2. Execute via the RakitKode agent engine (tool loop, approvals, asks,
	// subagents, schedules) — the same backend forge-ade's webview uses.
	// Streaming arrives via bus events bridged into Handle*; HandleTurnDone
	// finalizes. Falls back to the raw LLM stream when the engine is absent.
	if as.engine != nil && as.engine.AgentMgr != nil {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[agent] internal turn goroutine panicked (session %s): %v\n%s", targetSessID, r, debugStack())
				}
			}()
			if err := as.TriggerInternalTurn(targetSess, prompt, attached...); err != nil {
				as.mu.Lock()
				assistantMsg.Done = true
				assistantMsg.IsThinking = false
				assistantMsg.ShowThinking = false
				assistantMsg.ShowTools = false
				if as.ActiveSessionID == targetSessID {
					as.IsStreaming = false
				}
				targetSess.Status = "error"
				targetSess.turnStartFiles = nil
				assistantMsg.Text = fmt.Sprintf("**Agent engine error**: %v", err)
				as.mu.Unlock()
				if as.Invalidate != nil {
					as.Invalidate()
				}
			}
			// Success: events drive the UI from here (HandleTurnStart,
			// HandleTokenChunk, HandleToolEvent, HandleTurnDone) exactly as
			// they do for ACP turns.
		}()
		return
	}

	// 3. Fallback: raw LLMClient stream (no agent engine wired).
	go func() {
		turnCancelled := false
		// Declared first so it runs LAST: the finalization defer completes
		// the message, then a panic is logged instead of killing the app
		// (P1-6).
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[agent] raw-LLM turn goroutine panicked (session %s): %v\n%s", targetSessID, r, debugStack())
			}
		}()
		defer func() {
			// Compute git diff stats outside as.mu so subprocess calls never block UI renders.
			turnEndFiles := as.gitStatusPathsForWorkspace(sessWs)
			var turnChanges []*FileChange
			for path := range turnEndFiles {
				if !turnStartFiles[path] {
					turnChanges = append(turnChanges, &FileChange{Path: path, Additions: -1, Deletions: -1})
				}
			}
			as.computeChangeStatsForWorkspace(sessWs, turnChanges)

			as.mu.Lock()
			assistantMsg.Done = true
			assistantMsg.IsThinking = false
			assistantMsg.ShowThinking = false
			assistantMsg.ShowTools = false
			assistantMsg.ThinkingTimeSec = time.Since(startTime).Seconds()
			if as.ActiveSessionID == targetSessID {
				as.IsStreaming = false
			}
			targetSess.Status = "idle"
			targetSess.UpdatedAt = time.Now()
			targetSess.turnStartFiles = nil
			assistantMsg.Changes = turnChanges
			if turnCancelled && strings.TrimSpace(assistantMsg.Text) == "" {
				assistantMsg.Text = "*(stopped by user)*"
			}

			// Stop must not hand the steer queue to the relaunch: like
			// HandleTurnDone, drain only for turns that finished on their own.
			var nextSteer string
			var steerSnapshot []string
			saveSteer := false
			if !turnCancelled && len(targetSess.SteerQueue) > 0 {
				nextSteer = targetSess.SteerQueue[0]
				targetSess.SteerQueue = targetSess.SteerQueue[1:]
				steerSnapshot = append([]string(nil), targetSess.SteerQueue...)
				saveSteer = true
				if targetSess.ID == as.ActiveSessionID {
					as.SteerQueue = append([]string(nil), targetSess.SteerQueue...)
				}
				targetSess.Messages = append(targetSess.Messages, &ChatMessage{
					ID:        nextMessageIDLocked(targetSess),
					Role:      "user",
					Text:      nextSteer,
					Done:      true,
					Timestamp: time.Now(),
				})
				if targetSess.ID == as.ActiveSessionID {
					as.Messages = targetSess.Messages
				}
			} else if !turnCancelled && as.ActiveSessionID == targetSessID && len(as.SteerQueue) > 0 {
				nextSteer = as.SteerQueue[0]
				as.SteerQueue = as.SteerQueue[1:]
				targetSess.SteerQueue = append([]string(nil), as.SteerQueue...)
				steerSnapshot = append([]string(nil), as.SteerQueue...)
				saveSteer = true
				targetSess.Messages = append(targetSess.Messages, &ChatMessage{
					ID:        nextMessageIDLocked(targetSess),
					Role:      "user",
					Text:      nextSteer,
					Done:      true,
					Timestamp: time.Now(),
				})
				if targetSess.ID == as.ActiveSessionID {
					as.Messages = targetSess.Messages
				}
			}
			as.mu.Unlock()

			as.SaveSessionToDisk(targetSess)
			if saveSteer {
				as.saveSteerQueuesSlice(steerSnapshot)
			}
			if as.Invalidate != nil {
				as.Invalidate()
			}

			if nextSteer != "" {
				as.triggerAgentTurnForSession(targetSessID, nextSteer)
			}
		}()

		// 2. Execute via internal LLMClient
		if as.engine != nil && as.engine.LLMClient != nil {
			var llmMsgs []llm.LLMMessage
			as.mu.Lock()
			for _, m := range targetSess.Messages {
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
			// Register the cancel func so CancelStreaming can reach this
			// turn-local context; the callback stops appending once aborted.
			handle := &streamCancelHandle{cancel: cancel}
			as.setStreamCancel(targetSessID, handle)
			defer func() {
				as.clearStreamCancel(targetSessID, handle)
				cancel()
			}()

			_, err := as.engine.LLMClient.ChatWithStream(ctx, llmMsgs, nil, func(deltaContent, deltaReasoning string) {
				if ctx.Err() != nil {
					return // Stop was pressed: stop appending tokens
				}
				as.mu.Lock()
				if deltaReasoning != "" {
					assistantMsg.Thinking += deltaReasoning
					appendThinkingBlockDeltaLocked(assistantMsg, deltaReasoning)
					assistantMsg.IsThinking = true
				}
				if deltaContent != "" {
					assistantMsg.Text += deltaContent
					appendTextBlockDeltaLocked(assistantMsg, deltaContent)
					assistantMsg.IsThinking = false
				}
				as.mu.Unlock()
				if as.Invalidate != nil {
					as.Invalidate()
				}
			})

			if ctx.Err() != nil {
				turnCancelled = true
				return // the defer finalizes this as a user-stopped turn
			}

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

// LastTurnChangedFiles returns the files the session's last assistant turn
// changed, most significant first.
func (as *AgentState) LastTurnChangedFiles(sessionID string) []FileChange {
	as.mu.Lock()
	defer as.mu.Unlock()
	for _, s := range as.Sessions {
		if s.ID != sessionID {
			continue
		}
		for i := len(s.Messages) - 1; i >= 0; i-- {
			m := s.Messages[i]
			if m.Role == "assistant" && len(m.Changes) > 0 {
				out := make([]FileChange, 0, len(m.Changes))
				for _, ch := range m.Changes {
					if ch == nil {
						continue
					}
					out = append(out, *ch)
				}
				return out
			}
		}
		break
	}
	return nil
}

// LastTurnChangedFile returns the first file the session's last turn changed
// (Follow Agent auto-jump source).
func (as *AgentState) LastTurnChangedFile(sessionID string) (string, bool) {
	as.mu.Lock()
	defer as.mu.Unlock()
	for _, s := range as.Sessions {
		if s.ID != sessionID {
			continue
		}
		for i := len(s.Messages) - 1; i >= 0; i-- {
			m := s.Messages[i]
			if m.Role == "assistant" && len(m.Changes) > 0 {
				return m.Changes[0].Path, true
			}
		}
		break
	}
	return "", false
}

// writeToolPathArgKeys mirrors the path-argument keys the engine's file tools
// accept (RakitKode write/edit families).
var writeToolPathArgKeys = []string{
	"path", "file_path", "filepath", "filePath", "file", "filename", "target_file", "targetFile",
}

// writeToolPath extracts the target file path from a write/edit tool call's
// raw argument JSON.
func writeToolPath(argsJSON string) string {
	if argsJSON == "" {
		return ""
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return ""
	}
	for _, key := range writeToolPathArgKeys {
		if v, ok := args[key].(string); ok {
			if s := strings.TrimSpace(v); s != "" {
				return s
			}
		}
	}
	return ""
}

// LastTurnWrittenFiles returns the files the session's last assistant turn
// wrote or edited, newest first — derived from the turn's file tool calls so
// it works outside git workspaces, where the git-based change list stays
// empty.
func (as *AgentState) LastTurnWrittenFiles(sessionID string) []string {
	as.mu.Lock()
	defer as.mu.Unlock()
	for _, s := range as.Sessions {
		if s.ID != sessionID {
			continue
		}
		for i := len(s.Messages) - 1; i >= 0; i-- {
			m := s.Messages[i]
			if m.Role != "assistant" {
				continue
			}
			var out []string
			for j := len(m.Tools) - 1; j >= 0; j-- {
				t := m.Tools[j]
				if t == nil {
					continue
				}
				name := strings.ToLower(t.ToolName)
				isFileTool := strings.Contains(name, "write") || strings.Contains(name, "edit") ||
					strings.Contains(name, "create") || strings.Contains(name, "save") ||
					strings.Contains(name, "apply")
				if !isFileTool {
					continue
				}
				if p := writeToolPath(t.Command); p != "" {
					out = append(out, p)
				}
			}
			return out
		}
		break
	}
	return nil
}

// SessionWorkspacePath returns the workspace folder a session runs in.
func (as *AgentState) SessionWorkspacePath(sessionID string) string {
	as.mu.Lock()
	defer as.mu.Unlock()
	for _, s := range as.Sessions {
		if s.ID == sessionID {
			return s.WorkspacePath
		}
	}
	return ""
}
