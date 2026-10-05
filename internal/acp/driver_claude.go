package acp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// ClaudeDriver speaks Claude Code's native stream-json protocol over stdio.
// Port of kybern-drivers/src/claude.rs: one process per session, spawn shape
// mirrors the official Agent SDK, each stdin user message starts a turn that
// ends with a `result` frame. Permission prompts arrive as
// control_request{subtype:"can_use_tool"} and are answered on stdin.
type ClaudeDriver struct{}

func NewClaudeDriver() *ClaudeDriver { return &ClaudeDriver{} }

func (d *ClaudeDriver) ID() string { return "claude" }

func (d *ClaudeDriver) CheckBinary(command string) (bool, string) {
	if command == "" {
		command = "claude"
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

func (d *ClaudeDriver) Spawn(ctx context.Context, cfg SessionConfig) (AgentSession, <-chan DriverEvent, error) {
	bin := resolveBinary(orDefault(cfg.Binary, "claude"))
	if bin == "" || bin == orDefault(cfg.Binary, "claude") {
		if _, err := exec.LookPath(bin); err != nil {
			return nil, nil, fmt.Errorf("claude binary not found; install with: npm install -g @anthropic-ai/claude-code")
		}
	}
	sessionID := cfg.ResumeSessionID
	if sessionID == "" || cfg.Fork {
		sessionID = newUUID()
	}

	args := []string{
		"--output-format", "stream-json",
		"--input-format", "stream-json",
		"--verbose",
		"--include-partial-messages",
		"--permission-prompt-tool", "stdio",
	}
	if cfg.ResumeSessionID != "" && !cfg.Fork {
		args = append(args, "--resume="+cfg.ResumeSessionID)
	} else if cfg.ResumeSessionID != "" && cfg.Fork {
		args = append(args, "--resume="+cfg.ResumeSessionID, "--fork-session", "--session-id="+sessionID)
	} else {
		args = append(args, "--session-id="+sessionID)
	}
	args = append(args, "--permission-mode", claudeModeArg(cfg.PermissionMode))
	if cfg.Model != "" {
		args = append(args, "--model", cfg.Model)
	}
	cmd := exec.Command(bin, args...)
	cmd.Dir = cfg.CWD
	cmd.Env = childEnv(cfg)
	cmd.Env = append(cmd.Env, "CLAUDE_CODE_ARTIFACT_AUTO_OPEN=0")

	child, lines, err := spawnChild(cmd)
	if err != nil {
		return nil, nil, err
	}

	evs := make(chan DriverEvent, 1024)
	s := &claudeSession{
		child:    child,
		provider: sessionID,
		events:   evs,
		pending:  map[string]chan controlResult{},
		state:    &claudeTurnState{},
	}
	done := make(chan struct{})
	go s.readLoop(lines, done)
	// Claude Code emits no startup frame; system/init only arrives with the
	// first user turn. Bind immediately to the spawn-time session id (same as
	// kybern), and adopt the provider's id if init later reports a different one.
	evs <- DriverEvent{Kind: EvSessionBound, SessionID: sessionID, Model: cfg.Model}
	return s, evs, nil
}

func claudeModeArg(m PermissionMode) string {
	switch m {
	case ModeSupervised:
		return "default"
	case ModeAcceptEdits:
		return "acceptEdits"
	case ModeAuto:
		return "auto"
	case ModeFullAccess:
		return "bypassPermissions"
	}
	return "default"
}

type controlResult struct {
	value json.RawMessage
	err   string
}

type claudeTurnState struct {
	mu                sync.Mutex
	text              map[string]string
	thinking          map[string]string
	currentMessage    string
	lastAssistantUUID string
	currentUserUUID   string
	queuedUsage       Usage
	queuedCost        float64
	lastTotalCost     float64
	lastContextModel  string
	lastContextUsed   uint64
}

type claudeSession struct {
	child     *ndjsonChild
	provider  string
	events    chan<- DriverEvent
	pendingMu sync.Mutex
	pending   map[string]chan controlResult
	state     *claudeTurnState
	doneOnce  sync.Once
	done      chan struct{}
	closed    bool
}

func (s *claudeSession) Done() <-chan struct{} { return s.done }

func (s *claudeSession) Close() error {
	s.pendingMu.Lock()
	s.closed = true
	s.pendingMu.Unlock()
	s.child.close()
	return nil
}

func (s *claudeSession) emit(ev DriverEvent) {
	select {
	case s.events <- ev:
	default:
	}
}

func (s *claudeSession) SendMessage(ctx context.Context, messageID, text string) error {
	s.state.mu.Lock()
	s.state.currentUserUUID = messageID
	sessionID := s.provider
	s.state.mu.Unlock()
	frame := map[string]any{
		"type":       "user",
		"uuid":       messageID,
		"session_id": sessionID,
		"message": map[string]any{
			"role":    "user",
			"content": []map[string]any{{"type": "text", "text": text}},
		},
		"parent_tool_use_id": nil,
	}
	return s.child.write(frame)
}

func (s *claudeSession) Interrupt(ctx context.Context) error {
	_, err := s.sendControl(ctx, "interrupt", map[string]any{})
	return err
}

func (s *claudeSession) SetModel(ctx context.Context, model string) error {
	_, err := s.sendControl(ctx, "set_model", map[string]any{"model": model})
	return err
}

func (s *claudeSession) SetEffort(ctx context.Context, effort string) error {
	// Claude Code effort is fixed at spawn; kybern rejects the switch too.
	return fmt.Errorf("claude effort is fixed when the session starts; start a new session to change it")
}

// RespondPermission answers a can_use_tool control request.
// decision: "allow" | "allow_always" | "deny"[:reason]
func (s *claudeSession) RespondPermission(ctx context.Context, requestID, decision string) error {
	kind := decision
	reason := ""
	if i := strings.IndexByte(decision, ':'); i >= 0 {
		kind, reason = decision[:i], decision[i+1:]
	}
	var response map[string]any
	switch kind {
	case "allow", "allow_always":
		response = map[string]any{"behavior": "allow"}
	case "deny":
		if reason == "" {
			reason = "The user declined this action."
		}
		response = map[string]any{"behavior": "deny", "message": reason, "interrupt": false}
	default:
		return fmt.Errorf("unsupported claude permission decision %q", decision)
	}
	return s.respondControl(requestID, response)
}

func (s *claudeSession) sendControl(ctx context.Context, subtype string, request map[string]any) (json.RawMessage, error) {
	id := newUUID()
	request["subtype"] = subtype
	ch := make(chan controlResult, 1)
	s.pendingMu.Lock()
	if s.closed {
		s.pendingMu.Unlock()
		return nil, fmt.Errorf("session is closed; send again to resume")
	}
	s.pending[id] = ch
	s.pendingMu.Unlock()
	if err := s.child.write(map[string]any{"type": "control_request", "request_id": id, "request": request}); err != nil {
		s.pendingMu.Lock()
		delete(s.pending, id)
		s.pendingMu.Unlock()
		return nil, err
	}
	timer := time.NewTimer(30 * time.Second)
	defer timer.Stop()
	select {
	case res := <-ch:
		if res.err != "" {
			return nil, fmt.Errorf("%s: %s", subtype, res.err)
		}
		return res.value, nil
	case <-timer.C:
		s.pendingMu.Lock()
		delete(s.pending, id)
		s.pendingMu.Unlock()
		return nil, fmt.Errorf("%s: timed out", subtype)
	case <-ctx.Done():
		s.pendingMu.Lock()
		delete(s.pending, id)
		s.pendingMu.Unlock()
		return nil, ctx.Err()
	}
}

func (s *claudeSession) respondControl(requestID string, response map[string]any) error {
	return s.child.write(map[string]any{
		"type": "control_response",
		"response": map[string]any{
			"subtype":    "success",
			"request_id": requestID,
			"response":   response,
		},
	})
}

func (s *claudeSession) readLoop(lines <-chan json.RawMessage, done chan struct{}) {
	defer s.doneOnce.Do(func() { close(done) })
	for raw := range lines {
		s.handleFrame(raw)
	}
	code, ok := s.child.exitCode()
	var ev DriverEvent
	if ok && code != 0 {
		tail := s.child.stderr.tail(5)
		msg := fmt.Sprintf("exit code %d", code)
		if tail != "" {
			msg = tail
		}
		ev = DriverEvent{Kind: EvExited, ExitCode: &code, Error: msg}
	} else {
		ev = DriverEvent{Kind: EvExited}
	}
	s.emit(ev)
	// Fail any outstanding control calls.
	s.pendingMu.Lock()
	for id, ch := range s.pending {
		select {
		case ch <- controlResult{err: "process exited"}:
		default:
		}
		delete(s.pending, id)
	}
	s.pendingMu.Unlock()
}

func (s *claudeSession) handleFrame(raw json.RawMessage) {
	var frame struct {
		Type            string          `json:"type"`
		RequestID       string          `json:"request_id"`
		Request         json.RawMessage `json:"request"`
		Response        json.RawMessage `json:"response"`
		Message         json.RawMessage `json:"message"`
		Event           json.RawMessage `json:"event"`
		Subtype         string          `json:"subtype"`
		Usage           json.RawMessage `json:"usage"`
		IsReplay        bool            `json:"isReplay"`
		UUID            string          `json:"uuid"`
		ParentToolUseID *string         `json:"parent_tool_use_id"`
	}
	if err := json.Unmarshal(raw, &frame); err != nil {
		return
	}
	switch frame.Type {
	case "control_response":
		var resp struct {
			RequestID string          `json:"request_id"`
			Subtype   string          `json:"subtype"`
			Response  json.RawMessage `json:"response"`
			Error     string          `json:"error"`
		}
		if json.Unmarshal(frame.Response, &resp) != nil {
			return
		}
		s.pendingMu.Lock()
		ch, ok := s.pending[resp.RequestID]
		if ok {
			delete(s.pending, resp.RequestID)
		}
		s.pendingMu.Unlock()
		if ok {
			if resp.Subtype == "success" {
				ch <- controlResult{value: resp.Response}
			} else {
				ch <- controlResult{err: resp.Error}
			}
		}

	case "control_request":
		s.handleControlRequest(frame.RequestID, frame.Request)

	case "system":
		s.handleSystem(raw, frame.Subtype)

	case "stream_event":
		// Only main-thread text is the transcript; subagent output stays in its tool call.
		if frame.ParentToolUseID != nil {
			return
		}
		var ev struct {
			Type    string `json:"type"`
			Message struct {
				ID string `json:"id"`
			} `json:"message"`
			Delta struct {
				Type     string `json:"type"`
				Text     string `json:"text"`
				Thinking string `json:"thinking"`
			} `json:"delta"`
		}
		if json.Unmarshal(frame.Event, &ev) != nil {
			return
		}
		switch ev.Type {
		case "message_start":
			s.state.mu.Lock()
			s.state.currentMessage = ev.Message.ID
			s.state.mu.Unlock()
			s.emit(DriverEvent{Kind: EvTextDelta}) // ResponseStarted signal unused by sink
		case "content_block_delta":
			s.state.mu.Lock()
			messageID := s.state.currentMessage
			s.state.mu.Unlock()
			if messageID == "" {
				return
			}
			switch ev.Delta.Type {
			case "text_delta":
				if ev.Delta.Text != "" {
					s.emit(DriverEvent{Kind: EvTextDelta, MessageID: messageID, Delta: ev.Delta.Text})
				}
			case "thinking_delta":
				if ev.Delta.Thinking != "" {
					s.emit(DriverEvent{Kind: EvThinkingDelta, MessageID: messageID, Delta: ev.Delta.Thinking})
				}
			}
		}

	case "assistant":
		s.handleAssistant(raw, frame.ParentToolUseID, frame.Message)

	case "user":
		if frame.IsReplay {
			return
		}
		s.handleUserToolResults(frame.Message)

	case "result":
		s.handleResult(raw, frame.Subtype)
	}
}

func (s *claudeSession) handleControlRequest(requestID string, req json.RawMessage) {
	var r struct {
		Subtype   string          `json:"subtype"`
		ToolName  string          `json:"tool_name"`
		Input     json.RawMessage `json:"input"`
		ToolUseID string          `json:"tool_use_id"`
		Message   string          `json:"message"`
	}
	_ = json.Unmarshal(req, &r)
	switch r.Subtype {
	case "can_use_tool":
		summary := r.ToolName
		var input map[string]any
		_ = json.Unmarshal(r.Input, &input)
		if d, ok := input["description"].(string); ok && d != "" {
			summary = r.ToolName + ": " + d
		} else {
			summary = summarizeToolCall(r.ToolName, r.Input)
		}
		s.emit(DriverEvent{
			Kind: EvPermissionRequest, RequestID: requestID, ToolCallID: r.ToolUseID,
			ToolName: r.ToolName, Input: r.Input, Summary: summary,
		})
	case "hook_callback":
		_ = s.respondControl(requestID, map[string]any{"continue": true})
	default:
		_ = s.child.write(map[string]any{
			"type": "control_response",
			"response": map[string]any{
				"subtype":    "error",
				"request_id": requestID,
				"error":      "forge-ade does not support " + r.Subtype,
			},
		})
	}
}

func (s *claudeSession) handleSystem(raw json.RawMessage, subtype string) {
	switch subtype {
	case "init":
		var v struct {
			SessionID string `json:"session_id"`
			Model     string `json:"model"`
		}
		if json.Unmarshal(raw, &v) == nil && v.SessionID != "" {
			s.provider = v.SessionID
			s.emit(DriverEvent{Kind: EvSessionBound, SessionID: v.SessionID, Model: v.Model})
		}
	case "compact_boundary":
		s.emit(DriverEvent{Kind: EvNotice, NoticeLevel: "info", Error: "Context compacted"})
	}
}

func (s *claudeSession) handleAssistant(raw json.RawMessage, parent *string, msg json.RawMessage) {
	var m struct {
		ID    string `json:"id"`
		Model string `json:"model"`
		Usage struct {
			InputTokens              uint64 `json:"input_tokens"`
			OutputTokens             uint64 `json:"output_tokens"`
			CacheReadInputTokens     uint64 `json:"cache_read_input_tokens"`
			CacheCreationInputTokens uint64 `json:"cache_creation_input_tokens"`
		} `json:"usage"`
		Content []struct {
			Type     string          `json:"type"`
			ID       string          `json:"id"`
			Name     string          `json:"name"`
			Input    json.RawMessage `json:"input"`
			Text     string          `json:"text"`
			Thinking string          `json:"thinking"`
		} `json:"content"`
	}
	if json.Unmarshal(msg, &m) != nil {
		return
	}
	if parent == nil {
		s.state.mu.Lock()
		s.state.lastContextModel = m.Model
		s.state.lastContextUsed = m.Usage.InputTokens + m.Usage.OutputTokens + m.Usage.CacheReadInputTokens + m.Usage.CacheCreationInputTokens
		s.state.mu.Unlock()
	}
	for _, block := range m.Content {
		switch block.Type {
		case "text":
			if parent != nil {
				continue
			}
			s.state.mu.Lock()
			if s.state.text == nil {
				s.state.text = map[string]string{}
			}
			s.state.text[m.ID] += block.Text
			full := s.state.text[m.ID]
			thinking := s.state.thinking[m.ID]
			s.state.mu.Unlock()
			s.emit(DriverEvent{Kind: EvMessageCompleted, MessageID: m.ID, Text: full, Thinking: thinking})
		case "thinking":
			if parent != nil {
				continue
			}
			s.state.mu.Lock()
			if s.state.thinking == nil {
				s.state.thinking = map[string]string{}
			}
			s.state.thinking[m.ID] += block.Thinking
			s.state.mu.Unlock()
		case "tool_use":
			s.emit(DriverEvent{Kind: EvToolStarted, ToolCallID: block.ID, ToolName: block.Name, Input: block.Input})
		}
	}
}

func (s *claudeSession) handleUserToolResults(msg json.RawMessage) {
	var m struct {
		Content []struct {
			Type      string          `json:"type"`
			ToolUseID string          `json:"tool_use_id"`
			Content   json.RawMessage `json:"content"`
			IsError   bool            `json:"is_error"`
		} `json:"content"`
	}
	if json.Unmarshal(msg, &m) != nil {
		return
	}
	for _, block := range m.Content {
		if block.Type != "tool_result" {
			continue
		}
		output, _ := json.Marshal(map[string]any{"content": json.RawMessage(block.Content)})
		s.emit(DriverEvent{Kind: EvToolCompleted, ToolCallID: block.ToolUseID, Output: output, IsError: block.IsError})
	}
}

func (s *claudeSession) handleResult(raw json.RawMessage, subtype string) {
	s.state.mu.Lock()
	contextModel := s.state.lastContextModel
	contextUsed := s.state.lastContextUsed
	var v struct {
		TotalCostUSD float64 `json:"total_cost_usd"`
		DurationMs   uint64  `json:"duration_ms"`
	}
	_ = json.Unmarshal(raw, &v)
	delta := v.TotalCostUSD - s.state.lastTotalCost
	if delta < 0 {
		delta = 0
	}
	s.state.lastTotalCost = v.TotalCostUSD
	s.state.text = map[string]string{}
	s.state.thinking = map[string]string{}
	s.state.currentMessage = ""
	cost := delta
	s.state.mu.Unlock()

	var u struct {
		InputTokens              uint64 `json:"input_tokens"`
		OutputTokens             uint64 `json:"output_tokens"`
		CacheReadInputTokens     uint64 `json:"cache_read_input_tokens"`
		CacheCreationInputTokens uint64 `json:"cache_creation_input_tokens"`
	}
	var wrapper struct {
		Usage json.RawMessage `json:"usage"`
	}
	if json.Unmarshal(raw, &wrapper) == nil && len(wrapper.Usage) > 0 {
		_ = json.Unmarshal(wrapper.Usage, &u)
	}
	usage := Usage{InputTokens: u.InputTokens, OutputTokens: u.OutputTokens, CacheReadTokens: u.CacheReadInputTokens, CacheWriteTokens: u.CacheCreationInputTokens}

	_ = contextModel
	_ = contextUsed
	c := cost
	switch subtype {
	case "success":
		s.emit(DriverEvent{Kind: EvTurnCompleted, StopReason: StopCompleted, Usage: usage, CostUSD: &c, DurationMs: v.DurationMs})
	case "error_max_turns":
		s.emit(DriverEvent{Kind: EvTurnCompleted, StopReason: StopMaxTurns, Usage: usage, CostUSD: &c, DurationMs: v.DurationMs})
	default:
		var e struct {
			Errors []string `json:"errors"`
		}
		_ = json.Unmarshal(raw, &e)
		errText := fmt.Sprintf("claude: %s", subtype)
		if len(e.Errors) > 0 {
			errText = strings.Join(e.Errors, "; ")
		}
		s.emit(DriverEvent{Kind: EvTurnFailed, Error: errText})
	}
}

func summarizeToolCall(name string, input json.RawMessage) string {
	var in map[string]any
	if json.Unmarshal(input, &in) == nil {
		for _, key := range []string{"command", "file_path", "path", "pattern", "url", "query"} {
			if v, ok := in[key].(string); ok && v != "" {
				line := v
				if i := strings.IndexByte(line, '\n'); i >= 0 {
					line = line[:i]
				}
				if len(line) > 120 {
					line = line[:120]
				}
				return fmt.Sprintf("%s: %s", name, line)
			}
		}
	}
	return name
}
