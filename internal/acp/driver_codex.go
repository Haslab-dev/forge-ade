package acp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// CodexDriver speaks `codex app-server`: JSON-RPC 2.0 without the "jsonrpc"
// member, newline-delimited over stdio. Port of kybern-drivers/src/codex.rs:
// one app-server process per session hosting one Codex thread; the server
// sends requests of its own for approvals, answered by id.
type CodexDriver struct{}

func NewCodexDriver() *CodexDriver { return &CodexDriver{} }

func (d *CodexDriver) ID() string { return "codex" }

func (d *CodexDriver) CheckBinary(command string) (bool, string) {
	if command == "" {
		command = "codex"
	}
	p := resolveBinary(command)
	if p == command {
		return false, fmt.Sprintf("%q not found in PATH", command)
	}
	if _, err := os.Stat(p); err != nil {
		return false, fmt.Sprintf("%q not found", command)
	}
	return true, p
}

// codexPolicy maps a permission mode to (approvalPolicy, sandbox mode).
func codexPolicy(m PermissionMode) (string, string) {
	switch m {
	case ModeSupervised:
		return "untrusted", "workspace-write"
	case ModeAcceptEdits:
		return "on-request", "workspace-write"
	case ModeAuto:
		return "never", "workspace-write"
	case ModeFullAccess:
		return "never", "danger-full-access"
	}
	return "untrusted", "workspace-write"
}

func (d *CodexDriver) Spawn(ctx context.Context, cfg SessionConfig) (AgentSession, <-chan DriverEvent, error) {
	bin := resolveBinary(orDefault(cfg.Binary, "codex"))
	cmd := exec.Command(bin, "app-server")
	cmd.Dir = cfg.CWD
	cmd.Env = childEnv(cfg)

	child, lines, err := spawnChild(cmd)
	if err != nil {
		return nil, nil, err
	}

	evs := make(chan DriverEvent, 1024)
	s := &codexSession{
		child:     child,
		events:    evs,
		pending:   map[int64]chan codexCallResult{},
		approvals: map[string]codexApproval{},
		done:      make(chan struct{}),
	}
	s.state.mode = cfg.PermissionMode
	s.state.cwd = cfg.CWD
	s.state.model = cfg.Model

	go s.readLoop(lines)

	// initialize + initialized notification (frame has no "jsonrpc" member).
	if _, err := s.call(ctx, "initialize", map[string]any{
		"clientInfo":   map[string]any{"name": "forge-ade", "title": "forge-ade", "version": "0.8.5"},
		"capabilities": map[string]any{"experimentalApi": true},
	}); err != nil {
		child.kill()
		return nil, nil, fmt.Errorf("codex initialize: %w", err)
	}
	_ = child.write(map[string]any{"method": "initialized"})

	approval, sandbox := codexPolicy(cfg.PermissionMode)
	params := map[string]any{"cwd": cfg.CWD, "approvalPolicy": approval, "sandbox": sandbox}
	if cfg.Model != "" {
		params["model"] = cfg.Model
	}
	method := "thread/start"
	if cfg.ResumeSessionID != "" {
		params["threadId"] = cfg.ResumeSessionID
		method = "thread/resume"
		if cfg.Fork {
			method = "thread/fork"
		}
	}
	resp, err := s.call(ctx, method, params)
	if err != nil {
		child.kill()
		return nil, nil, fmt.Errorf("codex %s: %w", method, err)
	}
	threadID := jsonStringAt(resp, "thread", "id")
	if threadID == "" {
		child.kill()
		return nil, nil, fmt.Errorf("codex %s: no thread id in response", method)
	}
	s.state.mu.Lock()
	s.state.threadID = threadID
	if m := jsonString(resp, "model"); m != "" {
		s.state.model = m
	}
	s.state.mu.Unlock()
	s.emit(DriverEvent{Kind: EvSessionBound, SessionID: threadID, Model: s.state.model})
	return s, evs, nil
}

type codexCallResult struct {
	value json.RawMessage
	err   string
}

type codexApprovalKind int

const (
	codexApprovalCommand codexApprovalKind = iota
	codexApprovalFileChange
	codexApprovalUserInput
	codexApprovalElicitation
)

type codexApproval struct {
	id   json.RawMessage
	kind codexApprovalKind
}

type codexState struct {
	mu          sync.Mutex
	threadID    string
	turnID      string
	mode        PermissionMode
	model       string
	effort      string
	cwd         string
	lastTotal   *Usage
	turnUsage   Usage
	fileChanges map[string]json.RawMessage
	messageIDs  map[string]struct{}
}

type codexSession struct {
	child  *ndjsonChild
	events chan<- DriverEvent

	nextID     atomic.Int64
	pendingMu  sync.Mutex
	pending    map[int64]chan codexCallResult
	approvalMu sync.Mutex
	approvals  map[string]codexApproval
	state      codexState
	doneOnce   sync.Once
	done       chan struct{}
}

func (s *codexSession) Done() <-chan struct{} { return s.done }

func (s *codexSession) Close() error {
	s.child.close()
	return nil
}

func (s *codexSession) emit(ev DriverEvent) {
	select {
	case s.events <- ev:
	default:
	}
}

func (s *codexSession) call(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
	id := s.nextID.Add(1)
	ch := make(chan codexCallResult, 1)
	s.pendingMu.Lock()
	s.pending[id] = ch
	s.pendingMu.Unlock()
	if err := s.child.write(map[string]any{"id": id, "method": method, "params": params}); err != nil {
		s.pendingMu.Lock()
		delete(s.pending, id)
		s.pendingMu.Unlock()
		return nil, err
	}
	timer := time.NewTimer(120 * time.Second)
	defer timer.Stop()
	select {
	case res := <-ch:
		if res.err != "" {
			return nil, fmt.Errorf("%s: %s", method, res.err)
		}
		return res.value, nil
	case <-timer.C:
		s.pendingMu.Lock()
		delete(s.pending, id)
		s.pendingMu.Unlock()
		return nil, fmt.Errorf("%s: timed out", method)
	case <-ctx.Done():
		s.pendingMu.Lock()
		delete(s.pending, id)
		s.pendingMu.Unlock()
		return nil, ctx.Err()
	}
}

func (s *codexSession) respond(id json.RawMessage, result json.RawMessage, errText string) error {
	if errText != "" {
		return s.child.write(map[string]any{"id": json.RawMessage(id), "error": map[string]any{"code": -32601, "message": errText}})
	}
	if len(result) == 0 {
		result = json.RawMessage("{}")
	}
	return s.child.write(map[string]any{"id": json.RawMessage(id), "result": result})
}

func (s *codexSession) readLoop(lines <-chan json.RawMessage) {
	defer s.doneOnce.Do(func() { close(s.done) })
	for raw := range lines {
		s.handleFrame(raw)
	}
	s.failAllPending("codex exited")
	s.emit(DriverEvent{Kind: EvExited})
}

func (s *codexSession) failAllPending(msg string) {
	s.pendingMu.Lock()
	for id, ch := range s.pending {
		select {
		case ch <- codexCallResult{err: msg}:
		default:
		}
		delete(s.pending, id)
	}
	s.pendingMu.Unlock()
}

func (s *codexSession) handleFrame(raw json.RawMessage) {
	var frame struct {
		ID     *json.Number    `json:"id"`
		Method string          `json:"method"`
		Params json.RawMessage `json:"params"`
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(raw, &frame) != nil {
		return
	}
	switch {
	case frame.ID != nil && frame.Method != "":
		s.handleServerRequest(*frame.ID, frame.Method, frame.Params)
	case frame.ID != nil:
		id, err := frame.ID.Int64()
		if err != nil {
			return
		}
		s.pendingMu.Lock()
		ch, ok := s.pending[id]
		if ok {
			delete(s.pending, id)
		}
		s.pendingMu.Unlock()
		if ok {
			if frame.Error != nil {
				ch <- codexCallResult{err: frame.Error.Message}
			} else {
				ch <- codexCallResult{value: frame.Result}
			}
		}
	case frame.Method != "":
		s.handleNotification(frame.Method, frame.Params)
	}
}

func (s *codexSession) handleNotification(method string, params json.RawMessage) {
	s.state.mu.Lock()
	bound := s.state.threadID != ""
	s.state.mu.Unlock()
	if !bound {
		// Pre-bind notifications (thread/started) are folded into bind; drop the rest.
		if method == "thread/started" {
			var p struct {
				Thread struct {
					ID    string `json:"id"`
					Model string `json:"model"`
				} `json:"thread"`
			}
			if json.Unmarshal(params, &p) == nil && p.Thread.ID != "" {
				s.state.mu.Lock()
				s.state.threadID = p.Thread.ID
				s.state.mu.Unlock()
				s.emit(DriverEvent{Kind: EvSessionBound, SessionID: p.Thread.ID, Model: p.Thread.Model})
			}
		}
		return
	}

	var p struct {
		ThreadID   string          `json:"threadId"`
		ItemID     string          `json:"itemId"`
		Delta      string          `json:"delta"`
		Turn       json.RawMessage `json:"turn"`
		Item       json.RawMessage `json:"item"`
		Plan       json.RawMessage `json:"plan"`
		Error      json.RawMessage `json:"error"`
		TokenUsage json.RawMessage `json:"tokenUsage"`
	}
	_ = json.Unmarshal(params, &p)

	// Child-thread activity is folded out of the root transcript (kybern routes
	// child prose to its compact task lifecycle; we simply drop it here).
	if p.ThreadID != "" {
		s.state.mu.Lock()
		mine := s.state.threadID
		s.state.mu.Unlock()
		if mine != p.ThreadID {
			return
		}
	}

	switch method {
	case "turn/started":
		var t struct {
			Turn struct {
				ID string `json:"id"`
			} `json:"turn"`
		}
		if json.Unmarshal(params, &t) == nil && t.Turn.ID != "" {
			s.state.mu.Lock()
			s.state.turnID = t.Turn.ID
			s.state.mu.Unlock()
		}

	case "item/agentMessage/delta":
		if p.Delta != "" {
			s.emit(DriverEvent{Kind: EvTextDelta, MessageID: p.ItemID, Delta: p.Delta})
		}

	case "item/reasoning/textDelta", "item/reasoning/summaryTextDelta":
		if p.Delta != "" {
			s.emit(DriverEvent{Kind: EvThinkingDelta, MessageID: p.ItemID, Delta: p.Delta})
		}

	case "item/commandExecution/outputDelta":
		// Tool output streaming is not projected into the transcript yet.

	case "item/started":
		s.handleItem(p.Item, false)

	case "item/completed":
		s.handleItem(p.Item, true)

	case "turn/completed":
		var t struct {
			Turn struct {
				ID         string `json:"id"`
				Status     string `json:"status"`
				DurationMs uint64 `json:"durationMs"`
				Error      *struct {
					Message string `json:"message"`
				} `json:"error"`
			} `json:"turn"`
		}
		if json.Unmarshal(params, &t) != nil {
			return
		}
		s.state.mu.Lock()
		usage := s.state.turnUsage
		s.state.turnUsage = Usage{}
		s.state.lastTotal = nil
		s.state.turnID = ""
		s.state.messageIDs = map[string]struct{}{}
		s.state.fileChanges = map[string]json.RawMessage{}
		s.state.mu.Unlock()
		s.approvalMu.Lock()
		s.approvals = map[string]codexApproval{}
		s.approvalMu.Unlock()
		switch t.Turn.Status {
		case "completed":
			s.emit(DriverEvent{Kind: EvTurnCompleted, StopReason: StopCompleted, Usage: usage, DurationMs: t.Turn.DurationMs})
		case "interrupted":
			s.emit(DriverEvent{Kind: EvTurnCompleted, StopReason: StopInterrupted, Usage: usage, DurationMs: t.Turn.DurationMs})
		default:
			msg := "turn failed"
			if t.Turn.Error != nil {
				msg = t.Turn.Error.Message
			}
			s.emit(DriverEvent{Kind: EvTurnFailed, Error: msg})
		}

	case "thread/tokenUsage/updated":
		var tu struct {
			Last struct {
				TotalTokens uint64 `json:"totalTokens"`
			} `json:"last"`
			Total struct {
				InputTokens  uint64 `json:"input_tokens"`
				OutputTokens uint64 `json:"output_tokens"`
				CacheRead    uint64 `json:"cache_read_input_tokens"`
				CacheWrite   uint64 `json:"cache_creation_input_tokens"`
			} `json:"total"`
		}
		if json.Unmarshal(p.TokenUsage, &tu) == nil {
			total := Usage{InputTokens: tu.Total.InputTokens, OutputTokens: tu.Total.OutputTokens, CacheReadTokens: tu.Total.CacheRead, CacheWriteTokens: tu.Total.CacheWrite}
			s.state.mu.Lock()
			var perTurn Usage
			if s.state.lastTotal != nil {
				prev := *s.state.lastTotal
				perTurn = Usage{
					InputTokens:      saturSub(total.InputTokens, prev.InputTokens),
					OutputTokens:     saturSub(total.OutputTokens, prev.OutputTokens),
					CacheReadTokens:  saturSub(total.CacheReadTokens, prev.CacheReadTokens),
					CacheWriteTokens: saturSub(total.CacheWriteTokens, prev.CacheWriteTokens),
				}
			} else {
				perTurn = total
			}
			s.state.lastTotal = &total
			s.state.turnUsage = perTurn
			s.state.mu.Unlock()
		}

	case "error":
		var e struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
			WillRetry bool `json:"willRetry"`
		}
		if json.Unmarshal(params, &e) == nil {
			text := e.Error.Message
			if e.WillRetry {
				s.emit(DriverEvent{Kind: EvNotice, NoticeLevel: "warning", Error: text + " (retrying)"})
			} else {
				s.emit(DriverEvent{Kind: EvNotice, NoticeLevel: "error", Error: text})
			}
		}

	case "warning", "configWarning", "deprecationNotice":
		var w struct {
			Message string `json:"message"`
			Summary string `json:"summary"`
		}
		if json.Unmarshal(params, &w) == nil {
			text := w.Message
			if text == "" {
				text = w.Summary
			}
			if text != "" {
				s.emit(DriverEvent{Kind: EvNotice, NoticeLevel: "warning", Error: text})
			}
		}
	}
}

func saturSub(a, b uint64) uint64 {
	if a > b {
		return a - b
	}
	return 0
}

func (s *codexSession) handleItem(item json.RawMessage, completed bool) {
	if len(item) == 0 {
		return
	}
	var it struct {
		Type             string          `json:"type"`
		ID               string          `json:"id"`
		Status           string          `json:"status"`
		Command          string          `json:"command"`
		Path             string          `json:"path"`
		Text             string          `json:"text"`
		AggregatedOutput string          `json:"aggregatedOutput"`
		Changes          json.RawMessage `json:"changes"`
	}
	if json.Unmarshal(item, &it) != nil {
		return
	}
	switch it.Type {
	case "commandExecution":
		if !completed {
			title := it.Command
			s.emit(DriverEvent{Kind: EvToolStarted, ToolCallID: it.ID, ToolName: "bash", Input: json.RawMessage(fmt.Sprintf(`{"command":%q}`, title))})
			return
		}
		st := "completed"
		if it.Status == "failed" {
			st = "failed"
		}
		out, _ := json.Marshal(map[string]any{"output": it.AggregatedOutput})
		s.emit(DriverEvent{Kind: EvToolCompleted, ToolCallID: it.ID, ToolName: "bash", Output: out, IsError: st == "failed"})

	case "fileChange":
		if !completed {
			s.state.mu.Lock()
			if s.state.fileChanges == nil {
				s.state.fileChanges = map[string]json.RawMessage{}
			}
			s.state.fileChanges[it.ID] = it.Changes
			s.state.mu.Unlock()
			s.emit(DriverEvent{Kind: EvToolStarted, ToolCallID: it.ID, ToolName: "edit", Input: it.Changes})
			return
		}
		out, _ := json.Marshal(map[string]any{"output": "file modified"})
		s.emit(DriverEvent{Kind: EvToolCompleted, ToolCallID: it.ID, ToolName: "edit", Output: out, IsError: it.Status == "failed"})

	case "reasoning":
		if it.Text != "" {
			s.emit(DriverEvent{Kind: EvThinkingDelta, MessageID: it.ID, Delta: it.Text})
		}

	case "agentMessage":
		if completed && it.Text != "" {
			s.emit(DriverEvent{Kind: EvMessageCompleted, MessageID: it.ID, Text: it.Text})
		}
	}
}

func (s *codexSession) handleServerRequest(id json.Number, method string, params json.RawMessage) {
	var p struct {
		ItemID  string `json:"itemId"`
		Command string `json:"command"`
		CallID  string `json:"callId"`
		Tool    string `json:"tool"`
		Reason  string `json:"reason"`
	}
	_ = json.Unmarshal(params, &p)

	switch method {
	case "item/commandExecution/requestApproval":
		key := "cmd:" + id.String()
		s.approvalMu.Lock()
		s.approvals[key] = codexApproval{id: json.RawMessage(id.String()), kind: codexApprovalCommand}
		s.approvalMu.Unlock()
		input, _ := json.Marshal(map[string]any{"command": p.Command, "reason": p.Reason})
		summary := "run command"
		if p.Command != "" {
			summary = "run: " + firstLine(p.Command, 120)
		}
		s.emit(DriverEvent{Kind: EvPermissionRequest, RequestID: key, ToolCallID: p.ItemID, ToolName: "shell", Input: input, Summary: summary})

	case "item/fileChange/requestApproval":
		s.state.mu.Lock()
		auto := s.state.mode != ModeSupervised
		s.state.mu.Unlock()
		if auto {
			_ = s.respond(json.RawMessage(id.String()), json.RawMessage(`{"decision":"accept"}`), "")
			return
		}
		key := "patch:" + id.String()
		s.approvalMu.Lock()
		s.approvals[key] = codexApproval{id: json.RawMessage(id.String()), kind: codexApprovalFileChange}
		s.approvalMu.Unlock()
		s.emit(DriverEvent{Kind: EvPermissionRequest, RequestID: key, ToolCallID: p.ItemID, ToolName: "apply_patch", Input: params, Summary: "apply file changes"})

	case "item/tool/requestUserInput", "mcpServer/elicitation/request":
		kind := codexApprovalUserInput
		name := "request_user_input"
		if method == "mcpServer/elicitation/request" {
			kind = codexApprovalElicitation
			name = "mcp_elicitation"
		}
		key := "input:" + id.String()
		s.approvalMu.Lock()
		s.approvals[key] = codexApproval{id: json.RawMessage(id.String()), kind: kind}
		s.approvalMu.Unlock()
		var q struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(params, &q)
		summary := q.Message
		if summary == "" {
			summary = "Answer the agent's questions"
		}
		s.emit(DriverEvent{Kind: EvPermissionRequest, RequestID: key, ToolCallID: p.ItemID, ToolName: name, Input: params, Summary: summary})

	default:
		_ = s.respond(json.RawMessage(id.String()), nil, "forge-ade does not support "+method)
	}
}

func (s *codexSession) SendMessage(ctx context.Context, messageID, text string) error {
	s.state.mu.Lock()
	threadID := s.state.threadID
	if threadID == "" {
		s.state.mu.Unlock()
		return fmt.Errorf("no codex thread")
	}
	mode := s.state.mode
	cwd := s.state.cwd
	model := s.state.model
	effort := s.state.effort
	s.state.mu.Unlock()

	approval, _ := codexPolicy(mode)
	params := map[string]any{
		"threadId":            threadID,
		"clientUserMessageId": messageID,
		"input":               []map[string]any{{"type": "text", "text": text}},
		"approvalPolicy":      approval,
	}
	if mode == ModeFullAccess {
		params["sandboxPolicy"] = map[string]any{"type": "dangerFullAccess"}
	} else {
		params["sandboxPolicy"] = map[string]any{"type": "workspaceWrite", "writableRoots": []string{cwd}, "networkAccess": false}
	}
	if model != "" {
		params["model"] = model
	}
	if effort != "" {
		params["effort"] = effort
	}
	resp, err := s.call(ctx, "turn/start", params)
	if err != nil {
		return err
	}
	if turnID := jsonStringAt(resp, "turn", "id"); turnID != "" {
		s.state.mu.Lock()
		s.state.turnID = turnID
		s.state.mu.Unlock()
	}
	return nil
}

func (s *codexSession) Interrupt(ctx context.Context) error {
	s.state.mu.Lock()
	threadID, turnID := s.state.threadID, s.state.turnID
	s.state.mu.Unlock()
	if threadID == "" || turnID == "" {
		return nil
	}
	_, err := s.call(ctx, "turn/interrupt", map[string]any{"threadId": threadID, "turnId": turnID})
	return err
}

// SetModel stores the model for the next turn. Codex app-server expects bare
// model slugs ("glm-5-3-flash", "gpt-5.5"); provider routing comes from the
// user's config.toml `model_provider`. Strip any provider prefix the UI sent.
func (s *codexSession) SetModel(ctx context.Context, model string) error {
	if i := strings.LastIndexByte(model, '/'); i >= 0 {
		model = model[i+1:]
	}
	s.state.mu.Lock()
	s.state.model = model
	s.state.mu.Unlock()
	return nil
}

func (s *codexSession) SetEffort(ctx context.Context, effort string) error {
	s.state.mu.Lock()
	s.state.effort = effort
	s.state.mu.Unlock()
	return nil
}

// RespondPermission answers a pending approval.
// decision: "allow" (accept) | "allow_always" (acceptForSession) |
// "deny" (decline) | JSON payload for user-input/elicitation requests.
func (s *codexSession) RespondPermission(ctx context.Context, requestID, decision string) error {
	s.approvalMu.Lock()
	ap, ok := s.approvals[requestID]
	if ok {
		delete(s.approvals, requestID)
	}
	s.approvalMu.Unlock()
	if !ok {
		return fmt.Errorf("no pending approval %s", requestID)
	}
	switch ap.kind {
	case codexApprovalUserInput:
		return s.respond(ap.id, json.RawMessage(decision), "")
	case codexApprovalElicitation:
		if decision == "deny" {
			return s.respond(ap.id, json.RawMessage(`{"action":"decline"}`), "")
		}
		return s.respond(ap.id, json.RawMessage(decision), "")
	default:
		var d string
		switch decision {
		case "allow":
			d = "accept"
		case "allow_always":
			d = "acceptForSession"
		case "deny":
			d = "decline"
		default:
			return fmt.Errorf("expected permission decision, got %q", decision)
		}
		payload, _ := json.Marshal(map[string]any{"decision": d})
		return s.respond(ap.id, payload, "")
	}
}

func firstLine(s string, max int) string {
	line := s
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		line = line[:i]
	}
	if len(line) > max {
		line = line[:max]
	}
	return line
}

// --- small JSON helpers shared by drivers ---

func jsonString(v json.RawMessage, key string) string {
	var m map[string]any
	if json.Unmarshal(v, &m) != nil {
		return ""
	}
	if s, ok := m[key].(string); ok {
		return s
	}
	return ""
}

func jsonStringAt(v json.RawMessage, keys ...string) string {
	var cur any = json.RawMessage(v)
	for _, k := range keys {
		switch c := cur.(type) {
		case json.RawMessage:
			var m map[string]any
			if json.Unmarshal(c, &m) != nil {
				return ""
			}
			cur = m[k]
		case map[string]any:
			cur = c[k]
		default:
			return ""
		}
	}
	if s, ok := cur.(string); ok {
		return s
	}
	return ""
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
