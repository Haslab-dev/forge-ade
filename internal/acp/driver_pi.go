package acp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// PiDriver drives pi and Oh My Pi (omp): JSONL RPC over stdio (`--mode rpc`).
// Port of kybern-drivers/src/pi.rs: omp is a fork of pi with a superset
// protocol, so one module drives both with a flavor switch. Differences that
// matter: omp announces itself with a `ready` frame and supports chunked
// frames after protocol negotiation; omp approval tiers come from spawn-time
// `--approval-mode`; pi signals run completion with `agent_settled`, omp with
// `agent_end.isTerminal`.
type PiDriver struct {
	flavor string // "pi" | "omp"
}

func NewPiDriver() *PiDriver  { return &PiDriver{flavor: "pi"} }
func NewOmpDriver() *PiDriver { return &PiDriver{flavor: "omp"} }

func (d *PiDriver) ID() string { return d.flavor }

func (d *PiDriver) CheckBinary(command string) (bool, string) {
	if d.flavor == "pi" {
		for _, c := range []string{command, "pi"} {
			if c == "" {
				continue
			}
			if p := resolveBinary(c); p != c {
				if _, err := os.Stat(p); err == nil {
					return true, p
				}
			}
		}
		return false, fmt.Sprintf("%q not found in PATH", command)
	}
	p := resolveBinary(orDefault(command, "omp"))
	if p == orDefault(command, "omp") {
		return false, fmt.Sprintf("%q not found in PATH", command)
	}
	if _, err := os.Stat(p); err != nil {
		return false, fmt.Sprintf("%q not found", command)
	}
	return true, p
}

func ompApprovalTier(m PermissionMode) string {
	switch m {
	case ModeSupervised:
		return "always-ask"
	case ModeAcceptEdits:
		return "write"
	default:
		return "yolo"
	}
}

func (d *PiDriver) Spawn(ctx context.Context, cfg SessionConfig) (AgentSession, <-chan DriverEvent, error) {
	defaultBin := "pi"
	installHint := "npm install -g @earendil-works/pi-coding-agent"
	if d.flavor == "omp" {
		defaultBin = "omp"
		installHint = "npm install -g @oh-my-pi/pi-coding-agent"
	}
	// "pi-acp" is the ACP adapter, not the RPC binary this driver speaks; the
	// legacy seed config names it. The real binary is pi/omp.
	effective := cfg.Binary
	if effective == "pi-acp" || effective == "agent-pi" {
		effective = ""
	}
	bin := resolveBinary(orDefault(effective, defaultBin))
	if bin == "" || bin == orDefault(effective, defaultBin) {
		if _, err := exec.LookPath(bin); err != nil {
			return nil, nil, fmt.Errorf("%s binary not found; install with: %s", defaultBin, installHint)
		}
	}
	newSessionID := newUUID()

	args := []string{"--mode", "rpc"}
	if d.flavor == "pi" {
		if cfg.ResumeSessionID != "" {
			args = append(args, "--session", cfg.ResumeSessionID)
		} else {
			args = append(args, "--session-id", newSessionID)
		}
	} else {
		args = append(args, "--cwd", cfg.CWD)
		if cfg.ResumeSessionID != "" {
			args = append(args, "--resume", cfg.ResumeSessionID)
		}
		args = append(args, "--approval-mode", ompApprovalTier(cfg.PermissionMode))
	}
	if cfg.Model != "" {
		args = append(args, "--model", cfg.Model)
	}
	if cfg.Effort != "" {
		args = append(args, "--thinking", cfg.Effort)
	}

	cmd := exec.Command(bin, args...)
	cmd.Dir = cfg.CWD
	env := childEnv(cfg)
	if d.flavor == "omp" {
		if profile, err := resolveOmpProfile(cfg.Env); err == nil && profile != "" {
			env = append(env, "OMP_PROFILE="+profile)
		} else if err != nil {
			return nil, nil, err
		}
	}
	cmd.Env = env

	child, lines, err := spawnChild(cmd)
	if err != nil {
		return nil, nil, err
	}

	evs := make(chan DriverEvent, 1024)
	s := &piSession{
		flavor:   d.flavor,
		child:    child,
		events:   evs,
		pending:  map[string]chan piCallResult{},
		provider: newSessionID,
		done:     make(chan struct{}),
	}
	if d.flavor == "omp" {
		s.ready = make(chan struct{})
	}
	go s.readLoop(lines)

	if err := s.initialize(ctx, cfg); err != nil {
		child.kill()
		return nil, nil, err
	}
	return s, evs, nil
}

// resolveOmpProfile mirrors omp_profile.rs: normalize OMP_PROFILE (or legacy
// PI_PROFILE) against OMP's bootstrap rules. Empty/"default" resolves to "".
func resolveOmpProfile(env map[string]string) (string, error) {
	value := env["OMP_PROFILE"]
	if v, ok := os.LookupEnv("OMP_PROFILE"); ok && env["OMP_PROFILE"] == "" {
		value = v
	}
	if value == "" {
		value = os.Getenv("PI_PROFILE")
	}
	profile := strings.TrimSpace(value)
	if profile == "" || profile == "default" {
		return "", nil
	}
	// omp bootstrap rules: 1–64 chars, every byte lowercase alnum/._- (the
	// first byte must be alphanumeric, covered by the same rule), no trailing
	// dot, no reserved device names.
	if len(profile) > 64 || profile[len(profile)-1] == '.' || !isASCIIAlnum(profile[0]) {
		return "", fmt.Errorf("invalid OMP profile %q: use 1–64 lowercase letters, digits, dots, underscores or hyphens", profile)
	}
	for i := range len(profile) {
		c := profile[i]
		if !(c >= 'a' && c <= 'z') && !(c >= '0' && c <= '9') && c != '.' && c != '_' && c != '-' {
			return "", fmt.Errorf("invalid OMP profile %q: use 1–64 lowercase letters, digits, dots, underscores or hyphens", profile)
		}
	}
	return profile, nil
}

func isASCIIAlnum(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z')
}

type piCallResult struct {
	value json.RawMessage
	err   string
}

type piState struct {
	mu             sync.Mutex
	active         bool
	settling       bool
	promptPending  bool
	aborted        bool
	closed         bool
	currentMessage string
	currentText    strings.Builder
	currentThink   strings.Builder
	turnUsage      Usage
	turnCost       float64
	turnStarted    time.Time
	turnError      string
	chunks         map[string][][]*string
}

type piSession struct {
	flavor   string
	child    *ndjsonChild
	events   chan<- DriverEvent
	provider string

	pendingMu sync.Mutex
	pending   map[string]chan piCallResult

	readyOnce sync.Once
	ready     chan struct{}

	state piState

	doneOnce sync.Once
	done     chan struct{}
}

func (s *piSession) Done() <-chan struct{} { return s.done }

func (s *piSession) Close() error {
	s.state.mu.Lock()
	s.state.closed = true
	s.state.mu.Unlock()
	s.child.close()
	return nil
}

func (s *piSession) emit(ev DriverEvent) {
	select {
	case s.events <- ev:
	default:
	}
}

// initialize negotiates protocol and resolves the provider session id.
func (s *piSession) initialize(ctx context.Context, cfg SessionConfig) error {
	if s.flavor == "omp" {
		// Wait for `ready`, then negotiate chunked framing so big frames survive.
		select {
		case <-s.ready:
		case <-time.After(30 * time.Second):
		case <-ctx.Done():
			return ctx.Err()
		}
		_, _ = s.call(ctx, "negotiate_protocol", map[string]any{"protocolVersion": 2}, 60*time.Second)
	}
	state, err := s.call(ctx, "get_state", map[string]any{}, 60*time.Second)
	if err != nil {
		return fmt.Errorf("pi get_state: %w", err)
	}
	sessionID := jsonString(state, "sessionId")
	if sessionID == "" {
		sessionID = s.provider
	}
	model := piModelName(state)
	s.state.mu.Lock()
	s.provider = sessionID
	s.state.mu.Unlock()
	s.emit(DriverEvent{Kind: EvSessionBound, SessionID: sessionID, Model: model})
	return nil
}

func piModelName(state json.RawMessage) string {
	id := jsonStringAt(state, "model", "id")
	if id == "" {
		return ""
	}
	provider := jsonStringAt(state, "model", "provider")
	if provider == "" {
		return id
	}
	return provider + "/" + id
}

func (s *piSession) call(ctx context.Context, ty string, params map[string]any, timeout time.Duration) (json.RawMessage, error) {
	id := newUUID()
	ch := make(chan piCallResult, 1)
	s.pendingMu.Lock()
	s.pending[id] = ch
	s.pendingMu.Unlock()
	params["id"] = id
	params["type"] = ty
	if err := s.child.write(params); err != nil {
		s.pendingMu.Lock()
		delete(s.pending, id)
		s.pendingMu.Unlock()
		return nil, err
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case res := <-ch:
			if res.err != "" {
				return nil, fmt.Errorf("%s: %s", ty, res.err)
			}
			return res.value, nil
		case <-timer.C:
			s.pendingMu.Lock()
			_, live := s.pending[id]
			delete(s.pending, id)
			s.pendingMu.Unlock()
			if live {
				if ty == "prompt" {
					s.child.kill()
					return nil, fmt.Errorf("%s: timed out; send again to resume the saved conversation", ty)
				}
				return nil, fmt.Errorf("%s: timed out", ty)
			}
			// Answered while we were removing; loop to drain the buffered value.
			<-ch
		case <-ctx.Done():
			s.pendingMu.Lock()
			delete(s.pending, id)
			s.pendingMu.Unlock()
			return nil, ctx.Err()
		}
	}
}

func (s *piSession) readLoop(lines <-chan json.RawMessage) {
	defer s.doneOnce.Do(func() { close(s.done) })
	for raw := range lines {
		s.handleFrame(raw)
	}
	s.emit(DriverEvent{Kind: EvExited})
}

// reassemble reassembles omp v2 `rpc_chunk` frames (1 MiB splits).
func (s *piSession) reassemble(raw json.RawMessage) (json.RawMessage, bool) {
	var v struct {
		Type    string `json:"type"`
		ChunkID string `json:"chunkId"`
		Index   int    `json:"index"`
		Count   int    `json:"count"`
		Data    string `json:"data"`
	}
	if json.Unmarshal(raw, &v) != nil || v.Type != "rpc_chunk" {
		return raw, true
	}
	decoded, err := base64Decode(v.Data)
	if err != nil {
		return nil, false
	}
	s.state.mu.Lock()
	defer s.state.mu.Unlock()
	if s.state.chunks == nil {
		s.state.chunks = map[string][][]*string{}
	}
	parts, ok := s.state.chunks[v.ChunkID]
	if !ok {
		parts = make([][]*string, 0)
	}
	_ = parts
	entry := s.state.chunks[v.ChunkID]
	if entry == nil {
		entry = make([][]*string, 0)
		// entry is [partsHolder]; we use a 1-row slice-of-slice to keep a pointer cell.
		holder := make([]*string, v.Count)
		entry = append(entry, holder)
		s.state.chunks[v.ChunkID] = entry
	}
	holder := entry[0]
	if v.Index >= 0 && v.Index < len(holder) {
		piece := string(decoded)
		holder[v.Index] = &piece
	}
	for _, p := range holder {
		if p == nil {
			return nil, false
		}
	}
	var sb strings.Builder
	for _, p := range holder {
		sb.WriteString(*p)
	}
	delete(s.state.chunks, v.ChunkID)
	return json.RawMessage(sb.String()), true
}

func (s *piSession) handleFrame(raw json.RawMessage) {
	raw, ok := s.reassemble(raw)
	if !ok {
		return
	}
	var v struct {
		Type    string          `json:"type"`
		ID      string          `json:"id"`
		Success bool            `json:"success"`
		Error   string          `json:"error"`
		Data    json.RawMessage `json:"data"`
	}
	if json.Unmarshal(raw, &v) != nil {
		return
	}
	switch v.Type {
	case "ready":
		if s.ready != nil {
			s.readyOnce.Do(func() { close(s.ready) })
		}
	case "response":
		s.pendingMu.Lock()
		ch, live := s.pending[v.ID]
		if live {
			delete(s.pending, v.ID)
		}
		s.pendingMu.Unlock()
		if live {
			if v.Success {
				ch <- piCallResult{value: v.Data}
			} else {
				ch <- piCallResult{err: v.Error}
			}
		}
	case "agent_start":
		s.state.mu.Lock()
		if !s.state.active {
			s.state.active = true
			s.state.turnStarted = time.Now()
			s.state.turnUsage = Usage{}
			s.state.turnCost = 0
			s.state.turnError = ""
			s.state.aborted = false
		}
		aborted := s.state.aborted
		s.state.mu.Unlock()
		if aborted {
			_ = s.cancelNativeWork()
		}
	case "message_start":
		var m struct {
			Message struct {
				Role string `json:"role"`
			} `json:"message"`
		}
		if json.Unmarshal(raw, &m) == nil && m.Message.Role == "assistant" {
			s.state.mu.Lock()
			s.state.currentMessage = newUUID()
			s.state.currentText.Reset()
			s.state.currentThink.Reset()
			s.state.mu.Unlock()
		}
	case "message_update":
		var m struct {
			AssistantMessageEvent struct {
				Type  string `json:"type"`
				Delta string `json:"delta"`
			} `json:"assistantMessageEvent"`
		}
		if json.Unmarshal(raw, &m) != nil {
			return
		}
		s.state.mu.Lock()
		messageID := s.state.currentMessage
		s.state.mu.Unlock()
		if messageID == "" {
			return
		}
		switch m.AssistantMessageEvent.Type {
		case "text_delta":
			if m.AssistantMessageEvent.Delta != "" {
				s.emit(DriverEvent{Kind: EvTextDelta, MessageID: messageID, Delta: m.AssistantMessageEvent.Delta})
			}
		case "thinking_delta":
			if m.AssistantMessageEvent.Delta != "" {
				s.emit(DriverEvent{Kind: EvThinkingDelta, MessageID: messageID, Delta: m.AssistantMessageEvent.Delta})
			}
		}
	case "message_end":
		s.handleMessageEnd(raw)
	case "tool_execution_start":
		var t struct {
			ToolCallID string          `json:"toolCallId"`
			ToolName   string          `json:"toolName"`
			Args       json.RawMessage `json:"args"`
		}
		if json.Unmarshal(raw, &t) == nil {
			name := t.ToolName
			if name == "" {
				name = "tool"
			}
			s.emit(DriverEvent{Kind: EvToolStarted, ToolCallID: t.ToolCallID, ToolName: name, Input: t.Args})
		}
	case "tool_execution_end":
		var t struct {
			ToolCallID string `json:"toolCallId"`
			IsError    bool   `json:"isError"`
			Result     struct {
				Content json.RawMessage `json:"content"`
				Details json.RawMessage `json:"details"`
			} `json:"result"`
		}
		if json.Unmarshal(raw, &t) == nil {
			out, _ := json.Marshal(map[string]any{"content": json.RawMessage(t.Result.Content), "details": json.RawMessage(t.Result.Details)})
			s.emit(DriverEvent{Kind: EvToolCompleted, ToolCallID: t.ToolCallID, Output: out, IsError: t.IsError})
		}
	case "agent_end":
		terminal := true
		if s.flavor == "omp" {
			var e struct {
				IsTerminal *bool `json:"isTerminal"`
			}
			if json.Unmarshal(raw, &e) == nil && e.IsTerminal != nil {
				terminal = *e.IsTerminal
			}
		}
		if terminal {
			s.finishTurn()
		}
	case "agent_settled":
		if s.flavor == "pi" {
			s.finishTurn()
		}
	case "extension_ui_request":
		s.handleUIRequest(raw)
	case "auto_retry_start":
		var e struct {
			ErrorMessage string `json:"errorMessage"`
		}
		_ = json.Unmarshal(raw, &e)
		s.emit(DriverEvent{Kind: EvNotice, NoticeLevel: "warning", Error: "retrying (" + e.ErrorMessage + ")"})
	case "auto_retry_end":
		var e struct {
			Success    bool   `json:"success"`
			FinalError string `json:"finalError"`
		}
		_ = json.Unmarshal(raw, &e)
		s.state.mu.Lock()
		if e.Success {
			s.state.turnError = ""
		} else if !s.state.aborted {
			msg := e.FinalError
			if msg == "" {
				msg = "pi exhausted its retries"
			}
			s.state.turnError = msg
		}
		s.state.mu.Unlock()
	case "compaction_start", "auto_compaction_start":
		s.emit(DriverEvent{Kind: EvNotice, NoticeLevel: "info", Error: "compacting context"})
	case "compaction_end", "auto_compaction_end":
		var e struct {
			ErrorMessage string `json:"errorMessage"`
			Aborted      bool   `json:"aborted"`
		}
		_ = json.Unmarshal(raw, &e)
		text := "Context compacted"
		level := "info"
		if e.ErrorMessage != "" {
			text = "Context compaction failed: " + e.ErrorMessage
			level = "warning"
		} else if e.Aborted {
			text = "Compaction interrupted"
		}
		s.emit(DriverEvent{Kind: EvNotice, NoticeLevel: level, Error: text})
	case "model_changed":
		if state, err := s.call(context.Background(), "get_state", map[string]any{}, 10*time.Second); err == nil {
			s.emit(DriverEvent{Kind: EvSessionBound, SessionID: s.providerID(), Model: piModelName(state)})
		}
	}
}

func (s *piSession) providerID() string {
	s.state.mu.Lock()
	defer s.state.mu.Unlock()
	return s.provider
}

func (s *piSession) handleMessageEnd(raw json.RawMessage) {
	var m struct {
		Message struct {
			Role         string `json:"role"`
			StopReason   string `json:"stopReason"`
			ErrorMessage string `json:"errorMessage"`
			Usage        struct {
				Input      uint64 `json:"input"`
				Output     uint64 `json:"output"`
				CacheRead  uint64 `json:"cacheRead"`
				CacheWrite uint64 `json:"cacheWrite"`
				Cost       struct {
					Total float64 `json:"total"`
				} `json:"cost"`
			} `json:"usage"`
			Content []struct {
				Type     string `json:"type"`
				Text     string `json:"text"`
				Thinking string `json:"thinking"`
			} `json:"content"`
		} `json:"message"`
	}
	if json.Unmarshal(raw, &m) != nil || m.Message.Role != "assistant" {
		return
	}
	s.state.mu.Lock()
	messageID := s.state.currentMessage
	s.state.currentMessage = ""
	text, thinking := "", ""
	for _, block := range m.Message.Content {
		switch block.Type {
		case "text":
			text += block.Text
		case "thinking":
			thinking += block.Thinking
		}
	}
	s.state.turnUsage.InputTokens += m.Message.Usage.Input
	s.state.turnUsage.OutputTokens += m.Message.Usage.Output
	s.state.turnUsage.CacheReadTokens += m.Message.Usage.CacheRead
	s.state.turnUsage.CacheWriteTokens += m.Message.Usage.CacheWrite
	s.state.turnCost += m.Message.Usage.Cost.Total
	switch m.Message.StopReason {
	case "error":
		msg := m.Message.ErrorMessage
		if msg == "" {
			msg = "model error"
		}
		s.state.turnError = msg
	case "aborted":
		s.state.aborted = true
	}
	s.state.mu.Unlock()
	if text != "" || thinking != "" {
		s.emit(DriverEvent{Kind: EvMessageCompleted, MessageID: messageID, Text: text, Thinking: thinking})
	}
}

// finishTurn waits briefly for the settle boundary then publishes completion.
func (s *piSession) finishTurn() {
	s.state.mu.Lock()
	if !s.state.active || s.state.settling || s.state.closed {
		s.state.mu.Unlock()
		return
	}
	s.state.settling = true
	s.state.mu.Unlock()

	go func() {
		// A settled callback can coincide with retries/compaction; small delay
		// mirrors kybern's idle-verification loop.
		time.Sleep(150 * time.Millisecond)
		s.state.mu.Lock()
		if !s.state.active || s.state.closed {
			s.state.mu.Unlock()
			return
		}
		s.state.active = false
		s.state.settling = false
		usage := s.state.turnUsage
		cost := s.state.turnCost
		duration := uint64(0)
		if !s.state.turnStarted.IsZero() {
			duration = uint64(time.Since(s.state.turnStarted).Milliseconds())
		}
		errText := s.state.turnError
		aborted := s.state.aborted
		s.state.turnError = ""
		s.state.mu.Unlock()

		c := cost
		var ev DriverEvent
		switch {
		case aborted:
			ev = DriverEvent{Kind: EvTurnCompleted, StopReason: StopInterrupted, Usage: usage, CostUSD: &c, DurationMs: duration}
		case errText != "":
			ev = DriverEvent{Kind: EvTurnFailed, Error: errText}
		default:
			ev = DriverEvent{Kind: EvTurnCompleted, StopReason: StopCompleted, Usage: usage, CostUSD: &c, DurationMs: duration}
		}
		s.emit(ev)
	}()
}

func (s *piSession) cancelNativeWork() error {
	if s.flavor == "pi" {
		_ = s.child.write(map[string]any{"type": "clear_queue", "id": newUUID()})
		_ = s.child.write(map[string]any{"type": "abort_retry", "id": newUUID()})
	}
	return s.child.write(map[string]any{"type": "abort", "id": newUUID()})
}

func (s *piSession) handleUIRequest(raw json.RawMessage) {
	var v struct {
		ID     string `json:"id"`
		Method string `json:"method"`
		Title  string `json:"title"`
	}
	if json.Unmarshal(raw, &v) != nil || v.ID == "" || len(v.ID) > 128 {
		return
	}
	switch {
	case v.Method == "select" && strings.HasPrefix(v.Title, "Allow tool:"):
		toolName := strings.TrimSpace(strings.TrimPrefix(v.Title, "Allow tool:"))
		var input strings.Builder
		input.WriteString("{")
		first := true
		for line := range strings.Lines(v.Title) {
			line = strings.TrimRight(line, "\n")
			if k, val, ok := strings.Cut(line, ":"); ok && !strings.HasPrefix(k, "Allow tool") {
				if !first {
					input.WriteString(",")
				}
				first = false
				key, _ := json.Marshal(strings.ToLower(strings.TrimSpace(k)))
				value, _ := json.Marshal(strings.TrimSpace(val))
				input.Write(key)
				input.WriteString(":")
				input.Write(value)
			}
		}
		input.WriteString("}")
		s.emit(DriverEvent{
			Kind: EvPermissionRequest, RequestID: v.ID, ToolName: toolName,
			Input: json.RawMessage(input.String()), Summary: toolName + ": " + v.Title,
		})
	case v.Method == "select" || v.Method == "confirm" || v.Method == "input" || v.Method == "editor":
		s.emit(DriverEvent{
			Kind: EvPermissionRequest, RequestID: v.ID,
			ToolName: "ui_" + v.Method, Input: raw, Summary: v.Title,
		})
	case v.Method == "notify":
		var n struct {
			NotifyType string `json:"notifyType"`
			Message    string `json:"message"`
		}
		if json.Unmarshal(raw, &n) == nil {
			level := "info"
			switch n.NotifyType {
			case "error":
				level = "error"
			case "warning":
				level = "warning"
			}
			s.emit(DriverEvent{Kind: EvNotice, NoticeLevel: level, Error: n.Message})
		}
	}
}

func (s *piSession) SendMessage(ctx context.Context, messageID, text string) error {
	params := map[string]any{"message": text}
	_, err := s.call(ctx, "prompt", params, 600*time.Second)
	return err
}

func (s *piSession) Interrupt(ctx context.Context) error {
	s.state.mu.Lock()
	s.state.aborted = true
	s.state.mu.Unlock()
	return s.cancelNativeWork()
}

// SetModel accepts composite model names. pi-family RPC wants
// {provider, modelId}; installs differ in how the id is prefixed (e.g.
// provider=myairouter, modelId="glm-coding/glm-5.3-flash" vs provider=
// myairouter_cloud, modelId="kenari/glm-5-3-flash"). Try the natural split
// first, then fall back to the running provider with the full id.
func (s *piSession) SetModel(ctx context.Context, model string) error {
	provider, modelID, ok := strings.Cut(model, "/")
	if !ok {
		return fmt.Errorf("models are named provider/model, got %s", model)
	}
	_, err := s.call(ctx, "set_model", map[string]any{"provider": provider, "modelId": modelID}, 60*time.Second)
	if err == nil {
		return nil
	}
	state, stateErr := s.call(ctx, "get_state", map[string]any{}, 30*time.Second)
	if stateErr != nil {
		return err
	}
	if cur := jsonStringAt(state, "model", "provider"); cur != "" && cur != provider {
		_, err2 := s.call(ctx, "set_model", map[string]any{"provider": cur, "modelId": model}, 60*time.Second)
		if err2 == nil {
			return nil
		}
	}
	return err
}

func (s *piSession) SetEffort(ctx context.Context, effort string) error {
	_, err := s.call(ctx, "set_thinking_level", map[string]any{"level": effort}, 60*time.Second)
	return err
}

// RespondPermission answers a pending extension UI request.
// decision: "allow" (Approve) | "allow_always" (Approve) | "deny" (Deny) |
// "cancel" for withdrawn requests.
func (s *piSession) RespondPermission(ctx context.Context, requestID, decision string) error {
	value := "Deny"
	switch decision {
	case "allow", "allow_always":
		value = "Approve"
	case "cancel":
		return s.child.write(map[string]any{"type": "extension_ui_response", "id": requestID, "cancelled": true})
	}
	return s.child.write(map[string]any{"type": "extension_ui_response", "id": requestID, "value": value})
}

func base64Decode(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}
