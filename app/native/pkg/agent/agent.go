package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Haslab-dev/RakitKode/llm"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/hasdev/forge-ade/forge-ade-native/pkg/engine"
	"github.com/hasdev/forge-ade/forge-ade-native/pkg/icons"
	"github.com/hasdev/forge-ade/forge-ade-native/pkg/theme"
	"github.com/hasdev/forge-ade/internal/acp"
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
	TotalSteps      int       // steps reported by agent:turn_end
	TurnStartedAt   time.Time `json:"-"` // live elapsed while streaming
}

// AgentSession represents a single task session in the ForgeADE agent workspace.
type AgentSession struct {
	ID                string
	Title             string
	Status            string // "running", "idle", "completed"
	CreatedAt         time.Time
	UpdatedAt         time.Time
	Messages          []*ChatMessage
	Tasks             *TaskManager
	Model             string
	Provider          string
	AgentID           string
	AgentType         string
	WorkspacePath     string
	Pinned            bool
	Archived          bool
	Snoozed           bool            `json:"snoozed,omitempty"`
	RemoteSessionID   string          `json:"remoteSessionId,omitempty"` // agent conversation id
	AvailableCommands []string        `json:"availableCommands,omitempty"`
	LastUsage         *UsageInfo      `json:"lastUsage,omitempty"`
	turnStartFiles    map[string]bool // git snapshot at turn start
}

// AgentState tracks sessions, conversation history, streaming responses, and prompt composer.
type AgentState struct {
	mu              sync.Mutex
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
	ActiveAgentID   string
	ActiveAgentType string
	ExecutionMode   ExecutionMode
	ReasoningEffort ReasoningEffort
	AutoVerify      bool
	SteerQueue      []string
	AttachedFiles   []string
	StashedPrompts  []string
	ShowModelPicker bool
	AvailableModels []string
	NextID          int
	IsStreaming     bool
	TaskManager     *TaskManager
	Theme           *theme.ThemeManager
	ACPMgr          *ACPManager
	statsPending    map[string]bool
	Messages        []*ChatMessage // alias for active session messages
	Invalidate      func()
}

// NewAgentState initializes the native agent chat state by loading persistent sessions from disk.
func NewAgentState(engine *engine.Engine, workspacePath string, invalidate func()) *AgentState {
	// Detect default agent harness: prefer installed ACP agent (e.g. agy, claude, codex)
	// when no internal LLM provider keys are configured.
	defaultAgentID := "agent-agy"
	defaultAgentType := "acp"
	defaultModel := "gemini-3.8-flash-high"

	hasInternalKey := false
	if engine != nil && engine.LLMClient != nil {
		cfg := engine.LLMClient.GetConfig()
		if strings.TrimSpace(cfg.APIKey) != "" {
			hasInternalKey = true
		}
	}

	if !hasInternalKey {
		if _, err := exec.LookPath("agy"); err == nil {
			defaultAgentID = "agent-agy"
			defaultAgentType = "acp"
			defaultModel = "gemini-3.8-flash-high"
		} else if _, err := exec.LookPath("claude"); err == nil {
			defaultAgentID = "agent-claude"
			defaultAgentType = "claude"
			defaultModel = "sonnet"
		} else if _, err := exec.LookPath("codex"); err == nil {
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

	as := &AgentState{
		engine:          engine,
		workspacePath:   workspacePath,
		ActiveAgentID:   defaultAgentID,
		ActiveAgentType: defaultAgentType,
		ActiveModel:     defaultModel,
		ActiveFilterTab: "active",
		ExecutionMode:   ModeBypass,
		ReasoningEffort: EffortMedium,
		AutoVerify:      false,
		SteerQueue:      make([]string, 0),
		AvailableModels: []string{},
		Invalidate:      invalidate,
		NextID:          1,
		MdOpts: markdown.Options{
			Reveal: &markdown.Reveal{Unit: markdown.ByWord},
		},
	}

	// Load real sessions from ~/.forge/sessions/ and ~/.forge-ade/sessions/
	as.LoadSessionsFromDisk(workspacePath)

	return as
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
		ID:            len(sess.Messages) + 1,
		Role:          "assistant",
		Done:          false,
		Timestamp:     time.Now(),
		TurnStartedAt: time.Now(),
		IsThinking:    true,
	}
	sess.Messages = append(sess.Messages, msg)
	return msg
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
		msg := as.ensureStreamingMessage(s)
		msg.Text += text
		msg.IsThinking = false
		as.Messages = s.Messages
		s.UpdatedAt = time.Now()
		break
	}
	as.mu.Unlock()
	if as.Invalidate != nil {
		as.Invalidate()
	}
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
		msg := as.ensureStreamingMessage(s)
		msg.Thinking += text
		msg.IsThinking = true
		if !msg.ShowThinking && len(msg.Tools) == 0 {
			// The thinking row opens by default until tools arrive.
			msg.ShowThinking = true
		}
		as.Messages = s.Messages
		s.UpdatedAt = time.Now()
		break
	}
	as.mu.Unlock()
	if as.Invalidate != nil {
		as.Invalidate()
	}
}

// HandleToolEvent applies an agent:tool_start / agent:tool_end payload.
func (as *AgentState) HandleToolEvent(ev ToolEvent) {
	as.mu.Lock()
	for _, s := range as.Sessions {
		if s.ID != ev.SessionID {
			continue
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
		}
		// update_tasks-style calls drive the floating tracker, like
		// forge-ade's runtime task list.
		if strings.Contains(strings.ToLower(ev.Name), "task") {
			as.applyTaskUpdatePayload(ev.Args)
			as.applyTaskUpdatePayload(ev.Output)
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
		as.Messages = s.Messages
		s.UpdatedAt = time.Now()
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
		s.Status = "running"
		as.Messages = s.Messages
		break
	}
	as.mu.Unlock()
	if as.Invalidate != nil {
		as.Invalidate()
	}
}

// HandleTurnDone applies an agent:turn_end payload: finalizes the assistant
// message, maps the status vocabulary, records the turn's file changes, and
// returns the next queued steer prompt, if any.
func (as *AgentState) HandleTurnDone(sessionID string, info TurnEndInfo) (nextSteer string) {
	start := time.Now()
	as.mu.Lock()
	for _, s := range as.Sessions {
		if s.ID != sessionID {
			continue
		}
		msg := as.ensureStreamingMessage(s)
		msg.Done = true
		msg.IsThinking = false
		msg.Text = cleanTaskJSONBlocks(cleanRawToolTags(msg.Text))
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

		// Turn changes: files git newly reports as changed since the turn
		// started, with real +/- counts from their diffs.
		turnEndFiles := as.gitStatusPaths()
		var turnChanges []*FileChange
		for path := range turnEndFiles {
			if s.turnStartFiles == nil || !s.turnStartFiles[path] {
				turnChanges = append(turnChanges, &FileChange{Path: path, Additions: -1, Deletions: -1})
			}
		}
		s.turnStartFiles = nil
		as.computeChangeStats(turnChanges)
		if len(turnChanges) > 0 {
			msg.Changes = turnChanges
		}

		// Status vocabulary of the original app.
		switch {
		case info.Error != "":
			s.Status = "error"
			if strings.TrimSpace(msg.Text) == "" {
				msg.Text = fmt.Sprintf("⚠️ **Error**: %s", info.Error)
			} else {
				msg.Text += fmt.Sprintf("\n\n⚠️ **Error**: %s", info.Error)
			}
		case info.Status == "completed":
			s.Status = "completed"
		case info.Status == "stopped":
			s.Status = "idle"
			if strings.TrimSpace(msg.Text) == "" {
				msg.Text = "*(stopped by user)*"
			}
		default:
			s.Status = "idle"
		}
		s.UpdatedAt = time.Now()
		as.SaveSessionToDisk(s)
		break
	}
	// Only the visible session drives the global streaming flag.
	if as.ActiveSessionID == sessionID || sessionID == "" {
		as.IsStreaming = false
	}
	// ACP turns never drained the steer queue before; do it here.
	if len(as.SteerQueue) > 0 {
		nextSteer = as.SteerQueue[0]
		as.SteerQueue = as.SteerQueue[1:]
	}
	as.mu.Unlock()
	_ = start
	if as.Invalidate != nil {
		as.Invalidate()
	}
	return nextSteer
}

// HandleAsk applies an agent:ask permission request: surfaces it on the
// streaming message and flips the session to awaiting-approval.
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
		msg := as.ensureStreamingMessage(s)
		msg.PendingApproval = &PendingApproval{
			SessionID:  s.ID,
			ToolCallID: req.RequestID,
			ToolName:   req.ToolName,
			Summary:    req.Title,
			Arguments:  req.Arguments,
		}
		as.Messages = s.Messages
		break
	}
	as.mu.Unlock()
	if as.Invalidate != nil {
		as.Invalidate()
	}
}

// applyTaskUpdatePayload parses an update_tasks payload (args or result JSON
// with a tasks array) into the active session's floating tracker.
func (as *AgentState) applyTaskUpdatePayload(payload string) {
	payload = strings.TrimSpace(payload)
	if payload == "" || payload[0] != '{' {
		return
	}
	var parsed struct {
		Tasks []struct {
			Title      string `json:"title"`
			Status     string `json:"status"`
			ActiveForm string `json:"active_form"`
		} `json:"tasks"`
	}
	if err := json.Unmarshal([]byte(payload), &parsed); err != nil || len(parsed.Tasks) == 0 {
		return
	}
	as.mu.Lock()
	sess := as.ActiveSession()
	if sess == nil {
		as.mu.Unlock()
		return
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
	as.TaskManager = sess.Tasks
	as.Messages = sess.Messages
	as.mu.Unlock()
	if as.Invalidate != nil {
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

// ExecutionModeIcon returns the Tabler SVG icon for the active execution mode.
func (as *AgentState) ExecutionModeIcon() *ui.SVG {
	switch as.ExecutionMode {
	case ModeAsk:
		return icons.TablerHand
	case ModeCode:
		return icons.TablerPencil
	case ModePlan:
		return icons.TablerShieldCheck
	default:
		return icons.TablerShieldAlert
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

// StashPrompt stashes the current composer draft text into memory.
func (as *AgentState) StashPrompt() {
	txt := strings.TrimSpace(as.InputText)
	if txt != "" {
		as.StashedPrompts = append(as.StashedPrompts, txt)
		as.InputText = ""
	}
}

// RestoreStash restores a stashed prompt back into the composer.
func (as *AgentState) RestoreStash(idx int) {
	if idx >= 0 && idx < len(as.StashedPrompts) {
		as.InputText = as.StashedPrompts[idx]
		as.StashedPrompts = append(as.StashedPrompts[:idx], as.StashedPrompts[idx+1:]...)
	}
}

type diskSessionRaw struct {
	ID              string           `json:"id"`
	Title           string           `json:"title"`
	Name            string           `json:"name"`
	Goal            string           `json:"goal"`
	Status          string           `json:"status"`
	State           string           `json:"state"`
	CreatedAt       json.RawMessage  `json:"createdAt"`
	UpdatedAt       json.RawMessage  `json:"updatedAt"`
	Model           string           `json:"model"`
	Provider        string           `json:"provider"`
	AgentID         string           `json:"agentId"`
	AgentType       string           `json:"agentType"`
	ExecutionMode   string           `json:"executionMode"`
	ReasoningLevel  string           `json:"reasoningLevel"`
	WorkspacePath   string           `json:"workspacePath"`
	Folder          string           `json:"folder"`
	Pinned          bool             `json:"pinned"`
	Archived        bool             `json:"archived"`
	Snoozed         bool             `json:"snoozed"`
	RemoteSessionID string           `json:"remoteSessionId"`
	LastUsage       *UsageInfo       `json:"lastUsage"`
	Messages        []diskMessageRaw `json:"messages"`
	Tasks           []TaskItem       `json:"tasks"`
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
		sessAgentID := diskSess.AgentID
		sessAgentType := diskSess.AgentType

		hasInternalKey := false
		if as.engine != nil && as.engine.LLMClient != nil {
			cfg := as.engine.LLMClient.GetConfig()
			if strings.TrimSpace(cfg.APIKey) != "" {
				hasInternalKey = true
			}
		}

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
			txt := cleanTaskJSONBlocks(cleanRawToolTags(extractMessageText(m)))
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
			ID:              diskSess.ID,
			Title:           sessTitle,
			Status:          status,
			CreatedAt:       createdTime,
			UpdatedAt:       updatedTime,
			Model:           sessModel,
			Provider:        diskSess.Provider,
			AgentID:         sessAgentID,
			AgentType:       sessAgentType,
			WorkspacePath:   wsPath,
			Pinned:          diskSess.Pinned,
			Archived:        diskSess.Archived,
			Snoozed:         diskSess.Snoozed,
			RemoteSessionID: diskSess.RemoteSessionID,
			LastUsage:       diskSess.LastUsage,
			Messages:        msgs,
			Tasks:           tm,
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
		"snoozed":        sess.Snoozed,
		"messages":       diskMsgs,
		"tasks":          tasksList,
	}
	if sess.RemoteSessionID != "" {
		data["remoteSessionId"] = sess.RemoteSessionID
	}
	if sess.LastUsage != nil {
		data["lastUsage"] = sess.LastUsage
	}

	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return
	}

	_ = agentsession.SaveSessionDisk(string(bytes), wsPath)
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

// syncActiveSession updates the ActiveSessionID and points Messages to the active session.
func (as *AgentState) syncActiveSession() {
	if as.ActiveSessionID == "" {
		as.Messages = nil
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
		if active.Tasks != nil {
			as.TaskManager = active.Tasks
		}
		if active.Model != "" {
			as.ActiveModel = active.Model
		}
		if active.AgentID != "" {
			as.ActiveAgentID = active.AgentID
		}
		if active.AgentType != "" {
			as.ActiveAgentType = active.AgentType
		}
	} else {
		as.Messages = nil
	}
}

// ClearActiveSession resets active session to empty (displaying the New Agent UI).
func (as *AgentState) ClearActiveSession() {
	as.mu.Lock()
	as.ActiveSessionID = ""
	as.Messages = nil
	as.InputText = ""
	as.SteerQueue = nil
	if as.TaskManager != nil {
		as.TaskManager.Clear()
	}
	as.mu.Unlock()

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
	defer as.mu.Unlock()

	now := time.Now()
	id := fmt.Sprintf("session-%d", now.UnixMilli())
	forkTitle := fmt.Sprintf("%s (fork)", sess.Title)

	var msgsCopy []*ChatMessage
	for _, m := range sess.Messages {
		cp := *m
		msgsCopy = append(msgsCopy, &cp)
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
		Messages:      msgsCopy,
		Tasks:         NewTaskManager(),
	}

	as.Sessions = append([]*AgentSession{forked}, as.Sessions...)
	as.ActiveSessionID = id
	as.Messages = forked.Messages
	as.TaskManager = forked.Tasks
	return forked
}

// CreateSessionWithInitialPrompt creates a new session with an initial user prompt and starts the turn.
func (as *AgentState) CreateSessionWithInitialPrompt(prompt, agentID string) *AgentSession {
	now := time.Now()
	id := fmt.Sprintf("session-%d", now.UnixMilli())
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
	if model == "" {
		model = "gemini-3.8-flash-high"
	}

	sess := &AgentSession{
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
		Messages:      []*ChatMessage{userMsg},
		Tasks:         NewTaskManager(),
	}

	as.mu.Lock()
	as.Sessions = append([]*AgentSession{sess}, as.Sessions...)
	as.ActiveSessionID = id
	as.Messages = sess.Messages
	as.TaskManager = sess.Tasks
	as.mu.Unlock()

	as.SaveSessionToDisk(sess)
	as.triggerAgentTurn(prompt)

	if as.Invalidate != nil {
		as.Invalidate()
	}
	return sess
}

// TriggerTurn begins an AI agent turn with the given prompt for the active session.
func (as *AgentState) TriggerTurn(prompt string) {
	as.triggerAgentTurn(prompt)
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

	sess := &AgentSession{
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
func (as *AgentState) ContextUsage() (tokens int, pct int) {
	as.mu.Lock()
	defer as.mu.Unlock()

	// Prefer the usage report the agent streams (agent:usage / turn_end).
	for _, s := range as.Sessions {
		if s.ID == as.ActiveSessionID && s.LastUsage != nil && s.LastUsage.TotalTokens > 0 {
			tokens = s.LastUsage.TotalTokens
			const maxContext = 200000
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

	// Format attached files if present
	if len(as.AttachedFiles) > 0 {
		var b strings.Builder
		b.WriteString("Attached files:\n")
		for _, f := range as.AttachedFiles {
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

	as.mu.Lock()
	var activeSess *AgentSession
	for _, s := range as.Sessions {
		if s.ID == as.ActiveSessionID {
			activeSess = s
			break
		}
	}
	if activeSess == nil {
		now := time.Now()
		id := fmt.Sprintf("session-%d", now.UnixMilli())
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
	}

	userMsg := &ChatMessage{
		ID:        len(activeSess.Messages) + 1,
		Role:      "user",
		Text:      text,
		Done:      true,
		Timestamp: time.Now(),
	}
	activeSess.Messages = append(activeSess.Messages, userMsg)
	as.Messages = activeSess.Messages
	activeSess.UpdatedAt = time.Now()
	as.mu.Unlock()

	if activeSess != nil {
		as.SaveSessionToDisk(activeSess)
	}

	// Slash commands: /clear and /help are native; every other command goes
	// to the agent, exactly as forge-ade sends them.
	if strings.HasPrefix(text, "/") {
		root := text
		if i := strings.IndexAny(text, " \n"); i >= 0 {
			root = text[:i]
		}
		switch root {
		case "/clear":
			as.mu.Lock()
			for _, s := range as.Sessions {
				if s.ID == as.ActiveSessionID {
					s.Messages = s.Messages[:0]
					as.Messages = s.Messages
					s.UpdatedAt = time.Now()
					as.SaveSessionToDisk(s)
					break
				}
			}
			as.mu.Unlock()
			if as.Invalidate != nil {
				as.Invalidate()
			}
			return
		case "/help":
			as.mu.Lock()
			for _, s := range as.Sessions {
				if s.ID == as.ActiveSessionID {
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
						ID:        len(s.Messages) + 1,
						Role:      "assistant",
						Text:      help,
						Done:      true,
						Timestamp: time.Now(),
					})
					as.Messages = s.Messages
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

	// Trigger streaming AI turn
	as.triggerAgentTurn(text)

	if as.Invalidate != nil {
		as.Invalidate()
	}
}

// CancelStreaming aborts the active turn: it interrupts the ACP agent
// (process-group SIGINT for stream CLIs, protocol cancel for standard ACP)
// and stops the internal LLM stream. HandleTurnDone finalizes the message
// when the agent acknowledges the stop.
func (as *AgentState) CancelStreaming() {
	as.mu.Lock()
	as.IsStreaming = false
	sessionID := as.ActiveSessionID
	last := as.Messages
	as.mu.Unlock()

	if sessionID != "" && as.ACPMgr != nil {
		_ = as.ACPMgr.Cancel(sessionID)
	}

	// The internal LLM path has no event to finalize; close it here.
	if len(last) > 0 {
		msg := last[len(last)-1]
		if msg.Role == "assistant" && !msg.Done {
			msg.Done = true
			msg.IsThinking = false
			msg.ThinkingTimeSec = time.Since(msg.TurnStartedAt).Seconds()
			if strings.TrimSpace(msg.Text) == "" {
				msg.Text = "*(stopped by user)*"
			}
		}
	}
	as.mu.Lock()
	for _, s := range as.Sessions {
		if s.ID == sessionID {
			if s.Status == "running" {
				s.Status = "idle"
			}
			as.SaveSessionToDisk(s)
			break
		}
	}
	as.mu.Unlock()
	if as.Invalidate != nil {
		as.Invalidate()
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
	as.mu.Lock()
	defer as.mu.Unlock()
	as.workspacePath = dir
}

// WorkspacePath returns the active workspace path.
func (as *AgentState) WorkspacePath() string {
	return as.workspacePath
}

// Engine returns the engine instance.
func (as *AgentState) Engine() *engine.Engine {
	return as.engine
}

// ComputeChangeStats fills real +N/−N counts from the file's git diff.
func (as *AgentState) ComputeChangeStats(changes []*FileChange) {
	as.computeChangeStats(changes)
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
		TurnStartedAt:   startTime,
		IsThinking:      true,
		ThinkingTimeSec: 0,
		ShowThinking:    false,
		ShowTools:       false,
	}

	var activeSess *AgentSession
	for _, s := range as.Sessions {
		if s.ID == as.ActiveSessionID {
			// A stale approval gate from the previous turn must not leak
			// into this one.
			if len(s.Messages) > 0 {
				if prev := s.Messages[len(s.Messages)-1]; prev.PendingApproval != nil {
					prev.PendingApproval.Resolved = true
				}
			}
			s.Messages = append(s.Messages, assistantMsg)
			as.Messages = s.Messages
			s.Status = "running"
			s.turnStartFiles = turnStartFiles
			activeSess = s
			break
		}
	}
	permissionMode := as.ExecutionMode.PermissionMode()
	as.mu.Unlock()

	hasInternalKey := false
	if as.engine != nil && as.engine.LLMClient != nil {
		cfg := as.engine.LLMClient.GetConfig()
		if strings.TrimSpace(cfg.APIKey) != "" {
			hasInternalKey = true
		}
	}

	// 1. Check if ACP session can handle this turn
	isACPAgent := activeSess != nil && (activeSess.AgentType == "acp" ||
		activeSess.AgentID == "agent-agy" ||
		activeSess.AgentID == "agent-claude" ||
		activeSess.AgentID == "agent-codex" ||
		activeSess.AgentID == "agent-opencode" ||
		activeSess.AgentID == "agent-commandcode" ||
		activeSess.AgentID == "agent-pi" ||
		activeSess.AgentID == "agent-ohmypi" ||
		activeSess.AgentID == "agent-gemini")

	// If no internal provider API key is configured, automatically route turn to installed ACP agent
	if !isACPAgent && !hasInternalKey {
		if activeSess != nil {
			activeSess.AgentID = "agent-agy"
			activeSess.AgentType = "acp"
			if activeSess.Model == "" || activeSess.Model == "gpt-4o" || strings.Contains(activeSess.Model, "gpt-4o") {
				activeSess.Model = "gemini-3.8-flash-high"
			}
		}
		isACPAgent = true
	}

	if isACPAgent {
		go func() {
			var startErr error
			if as.ACPMgr != nil {
				_, startErr = as.ACPMgr.StartSessionWithID(activeSess.ID, activeSess.AgentID, as.workspacePath, activeSess.Model, permissionMode)
				if startErr == nil {
					startErr = as.ACPMgr.Prompt(activeSess.ID, prompt)
				}
			} else if as.engine != nil && as.engine.ACPMgr != nil {
				_, startErr = as.engine.ACPMgr.StartSession(acp.SessionOpts{
					SessionID:      activeSess.ID,
					AgentID:        activeSess.AgentID,
					Cwd:            as.workspacePath,
					Model:          activeSess.Model,
					PermissionMode: permissionMode,
				})
				if startErr == nil {
					startErr = as.engine.ACPMgr.Prompt(activeSess.ID, prompt)
				}
			} else {
				startErr = fmt.Errorf("no ACP manager available")
			}

			if startErr != nil {
				as.mu.Lock()
				assistantMsg.Done = true
				assistantMsg.IsThinking = false
				as.IsStreaming = false
				if activeSess != nil {
					activeSess.Status = "error"
					activeSess.turnStartFiles = nil
				}
				assistantMsg.Text = fmt.Sprintf("ACP agent **%s** error: %v\n\nPlease ensure the CLI executable is installed on PATH (or select another agent harness in ⌘M).", activeSess.AgentID, startErr)
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

	// 2. Execute via internal LLMClient
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
			if activeSess != nil {
				activeSess.turnStartFiles = nil
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
