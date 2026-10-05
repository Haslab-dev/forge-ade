package acp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// OpencodeDriver runs `opencode serve` per session and speaks its HTTP API,
// consuming the server-sent event stream for progress (port of
// kybern-drivers/src/opencode.rs, adapted to the v2 HTTP API):
//   - v2 serve always enables HTTP Basic auth and prints the one-line password
//     to stdout ("server password <pw>"); username is "opencode".
//   - Session create:  POST /api/session                 (JSON body optional)
//     Prompt:           POST /api/session/{id}/prompt    {"text": ...} (sync submit;
//     the assistant turn streams over SSE)
//     Events:           GET  /api/event (SSE; frames data:{...})
//   - Turn boundaries:   session.execution.succeeded / .failed (or .interrupted)
//   - Deltas:            session.text.delta / session.reasoning.delta
//     (data.assistantMessageID, data.delta)
//   - Permissions:       GET /api/session/{id}/permission polls pending requests;
//     POST answers {"id":"per...","action":"once|always|reject"}.
type OpencodeDriver struct{}

func NewOpencodeDriver() *OpencodeDriver { return &OpencodeDriver{} }

func (d *OpencodeDriver) ID() string { return "opencode" }

func (d *OpencodeDriver) CheckBinary(command string) (bool, string) {
	if command == "" {
		command = "opencode"
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

// opencodeRuleset is the per-session permission ruleset for a mode; last
// matching rule wins (port of ruleset() in opencode.rs). v2 accepts rules on
// session create under "permission"; unknown server versions ignore them.
func opencodeRuleset(m PermissionMode) []map[string]any {
	rule := func(permission, action string) map[string]any {
		return map[string]any{"permission": permission, "pattern": "*", "action": action}
	}
	switch m {
	case ModeSupervised:
		return []map[string]any{
			rule("*", "ask"), rule("read", "allow"), rule("glob", "allow"), rule("grep", "allow"),
			rule("list", "allow"), rule("todowrite", "allow"), rule("question", "allow"),
			rule("skill", "allow"), rule("task", "allow"), rule("lsp", "allow"),
		}
	case ModeAcceptEdits:
		return []map[string]any{
			rule("*", "allow"), rule("bash", "ask"), rule("webfetch", "ask"),
			rule("websearch", "ask"), rule("external_directory", "ask"), rule("doom_loop", "ask"),
		}
	case ModeAuto:
		return []map[string]any{rule("*", "allow"), rule("external_directory", "ask"), rule("doom_loop", "ask")}
	case ModeFullAccess:
		return []map[string]any{rule("*", "allow")}
	}
	return []map[string]any{rule("*", "ask")}
}

func (d *OpencodeDriver) Spawn(ctx context.Context, cfg SessionConfig) (AgentSession, <-chan DriverEvent, error) {
	bin := resolveBinary(orDefault(cfg.Binary, "opencode"))
	cmd := exec.Command(bin, "serve", "--port", "0", "--hostname", "127.0.0.1")
	cmd.Dir = cfg.CWD
	cmd.Env = childEnv(cfg)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("spawn opencode: %w", err)
	}

	// Wait for the announce lines, capturing both URL and Basic-auth password.
	// v2 prints exactly: "server listening on http://…" then "server password …".
	// One pump owns the scanner; it signals ready once both landed, then hands
	// the scanner to a drainer so the pipe never blocks the server.
	var base, password string
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
	ready := make(chan struct{})
	go func() {
		defer close(ready)
		for sc.Scan() {
			line := sc.Text()
			if i := strings.Index(line, "http://"); i >= 0 && base == "" {
				base = strings.TrimSuffix(strings.TrimSpace(line[i:]), "/")
			}
			if pw, ok := strings.CutPrefix(strings.TrimSpace(line), "server password "); ok && password == "" {
				password = strings.TrimSpace(pw)
			}
			if base != "" && password != "" {
				go func() {
					for sc.Scan() {
					}
				}()
				return
			}
		}
	}()
	select {
	case <-ready:
		if base == "" {
			_ = cmd.Process.Kill()
			return nil, nil, fmt.Errorf("opencode serve did not announce a listening address")
		}
	case <-time.After(60 * time.Second):
		_ = cmd.Process.Kill()
		return nil, nil, fmt.Errorf("opencode serve did not announce a listening address (timeout)")
	}
	// password == "" means a v1 server (≤1.x) with no Basic auth; the empty
	// password disables the Authorization header.

	evs := make(chan DriverEvent, 1024)
	s := &opencodeSession{
		client: &http.Client{Timeout: 30 * time.Minute},
		base:   base,
		dir:    cfg.CWD,
		user:   "opencode",
		pass:   password,
		events: evs,
		state: opencodeState{
			mode:    cfg.PermissionMode,
			model:   cfg.Model,
			parts:   map[string]opencodePart{},
			pending: map[string]string{},
		},
		cmd:  cmd,
		done: make(chan struct{}),
	}

	// Create or adopt the session.
	var sessionID string
	switch {
	case cfg.ResumeSessionID != "" && !cfg.Fork:
		var v struct {
			Data struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if err := s.getJSON(ctx, "/api/session/"+cfg.ResumeSessionID, &v); err != nil {
			_ = s.Close()
			return nil, nil, fmt.Errorf("opencode session %s not found in %s", cfg.ResumeSessionID, cfg.CWD)
		}
		sessionID = cfg.ResumeSessionID
	case cfg.ResumeSessionID != "" && cfg.Fork:
		body, _ := json.Marshal(map[string]any{})
		var v struct {
			Data struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if err := s.doJSON(ctx, http.MethodPost, "/api/session/"+cfg.ResumeSessionID+"/fork", body, &v); err != nil {
			_ = s.Close()
			return nil, nil, fmt.Errorf("opencode fork: %w", err)
		}
		sessionID = v.Data.ID
		if sessionID == "" {
			_ = s.Close()
			return nil, nil, fmt.Errorf("opencode fork returned no id")
		}
	default:
		body := map[string]any{"title": "forge-ade", "permission": opencodeRuleset(cfg.PermissionMode)}
		if p, m, ok := splitModelID(cfg.Model); ok {
			body["model"] = map[string]any{"providerID": p, "id": m}
		}
		payload, _ := json.Marshal(body)
		var v struct {
			Data struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if err := s.doJSONRaw(ctx, http.MethodPost, "/api/session", payload, &v); err != nil {
			_ = s.Close()
			return nil, nil, fmt.Errorf("opencode session create failed: %w", err)
		}
		sessionID = v.Data.ID
		if sessionID == "" {
			_ = s.Close()
			return nil, nil, fmt.Errorf("opencode session create returned no id")
		}
	}
	s.state.sessionID = sessionID
	s.emit(DriverEvent{Kind: EvSessionBound, SessionID: sessionID, Model: cfg.Model})

	go s.eventLoop()
	return s, evs, nil
}

func splitModelID(s string) (string, string, bool) {
	if s == "" {
		return "", "", false
	}
	p, m, ok := strings.Cut(s, "/")
	return p, m, ok
}

type opencodePart struct {
	Kind      string
	MessageID string
}

type opencodeState struct {
	mu        sync.Mutex
	sessionID string
	model     string
	mode      PermissionMode
	active    bool
	turnStart time.Time
	parts     map[string]opencodePart
	pending   map[string]string // permission id -> session id
}

type opencodeSession struct {
	client *http.Client
	base   string
	dir    string
	user   string
	pass   string
	events chan<- DriverEvent
	cmd    *exec.Cmd

	state opencodeState
	once  sync.Once
	done  chan struct{}
}

func (s *opencodeSession) Done() <-chan struct{} { return s.done }

func (s *opencodeSession) Close() error {
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	return nil
}

func (s *opencodeSession) emit(ev DriverEvent) {
	select {
	case s.events <- ev:
	default:
	}
}

func (s *opencodeSession) newRequest(ctx context.Context, method, path string, body []byte) (*http.Request, error) {
	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, s.base+path, rd)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if s.pass != "" {
		req.SetBasicAuth(s.user, s.pass)
	}
	return req, nil
}

func (s *opencodeSession) doJSONRaw(ctx context.Context, method, path string, body []byte, out any) error {
	req, err := s.newRequest(ctx, method, path, body)
	if err != nil {
		return err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("opencode http: %w", err)
	}
	defer resp.Body.Close()
	var buf bytes.Buffer
	io.Copy(&buf, resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("opencode http %d: %s", resp.StatusCode, strings.TrimSpace(buf.String()))
	}
	if out != nil && buf.Len() > 0 {
		return json.Unmarshal(buf.Bytes(), out)
	}
	return nil
}

func (s *opencodeSession) doJSON(ctx context.Context, method, path string, body any, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	return s.doJSONRaw(ctx, method, path, payload, out)
}

func (s *opencodeSession) getJSON(ctx context.Context, path string, out any) error {
	return s.doJSONRaw(ctx, http.MethodGet, path, nil, out)
}

func (s *opencodeSession) eventLoop() {
	defer s.once.Do(func() { close(s.done) })
	backoff := 200 * time.Millisecond
	for {
		req, err := s.newRequest(context.Background(), http.MethodGet, "/api/event", nil)
		if err == nil {
			resp, err := s.client.Do(req)
			if err == nil && resp.StatusCode == http.StatusOK {
				backoff = 200 * time.Millisecond
				s.consumeSSE(resp.Body)
			}
			if resp != nil {
				resp.Body.Close()
			}
		}
		// Stream dropped: is the server still alive?
		if s.cmd.ProcessState != nil {
			break
		}
		select {
		case <-time.After(backoff):
		case <-s.done:
			return
		}
		backoff *= 2
		if backoff > 5*time.Second {
			backoff = 5 * time.Second
		}
	}
	code := 0
	if s.cmd.ProcessState != nil {
		code = s.cmd.ProcessState.ExitCode()
	}
	if code != 0 {
		s.emit(DriverEvent{Kind: EvExited, ExitCode: &code, Error: fmt.Sprintf("opencode serve exited with code %d", code)})
	} else {
		s.emit(DriverEvent{Kind: EvExited})
	}
}

func (s *opencodeSession) consumeSSE(body io.ReadCloser) {
	sc := bufio.NewScanner(body)
	sc.Buffer(make([]byte, 1024*1024), 64*1024*1024)
	for sc.Scan() {
		line := sc.Text()
		data, ok := strings.CutPrefix(line, "data:")
		if !ok {
			continue
		}
		var v struct {
			Type string          `json:"type"`
			Data json.RawMessage `json:"data"`
		}
		if json.Unmarshal([]byte(strings.TrimSpace(data)), &v) != nil {
			continue
		}
		s.handleEvent(v.Type, v.Data)
	}
}

func (s *opencodeSession) handleEvent(ty string, p json.RawMessage) {
	switch ty {
	case "session.text.delta":
		var e struct {
			AssistantMessageID string `json:"assistantMessageID"`
			Delta              string `json:"delta"`
		}
		if json.Unmarshal(p, &e) == nil && e.Delta != "" {
			s.emit(DriverEvent{Kind: EvTextDelta, MessageID: e.AssistantMessageID, Delta: e.Delta})
		}

	case "session.reasoning.delta":
		var e struct {
			AssistantMessageID string `json:"assistantMessageID"`
			Delta              string `json:"delta"`
		}
		if json.Unmarshal(p, &e) == nil && e.Delta != "" {
			s.emit(DriverEvent{Kind: EvThinkingDelta, MessageID: e.AssistantMessageID, Delta: e.Delta})
		}

	case "session.permission.created", "session.permission.updated":
		var e struct {
			ID         string          `json:"id"`
			SessionID  string          `json:"sessionID"`
			Type       string          `json:"type"`
			Title      string          `json:"title"`
			Metadata   json.RawMessage `json:"metadata"`
			Patterns   json.RawMessage `json:"patterns"`
			Permission string          `json:"permission"`
		}
		if json.Unmarshal(p, &e) == nil && e.ID != "" {
			s.state.mu.Lock()
			sid := e.SessionID
			if sid == "" {
				sid = s.state.sessionID
			}
			s.state.pending[e.ID] = sid
			s.state.mu.Unlock()
			name := e.Permission
			if name == "" {
				name = e.Type
			}
			if name == "" {
				name = "permission"
			}
			summary := e.Title
			if summary == "" {
				summary = name
			}
			input, _ := json.Marshal(map[string]any{"permission": name, "patterns": json.RawMessage(e.Patterns), "metadata": json.RawMessage(e.Metadata)})
			s.emit(DriverEvent{Kind: EvPermissionRequest, RequestID: e.ID, ToolName: name, Input: input, Summary: summary})
		}

	case "session.execution.succeeded":
		s.endTurn(StopCompleted)

	case "session.execution.failed":
		var e struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(p, &e)
		s.state.mu.Lock()
		active := s.state.active
		s.state.active = false
		s.state.mu.Unlock()
		if active {
			s.emit(DriverEvent{Kind: EvTurnFailed, Error: e.Error})
		} else {
			s.emit(DriverEvent{Kind: EvNotice, NoticeLevel: "error", Error: e.Error})
		}

	case "session.execution.interrupted":
		s.endTurn(StopInterrupted)
	}
}

func (s *opencodeSession) endTurn(stop StopReason) {
	s.state.mu.Lock()
	active := s.state.active
	s.state.active = false
	duration := uint64(0)
	if !s.state.turnStart.IsZero() {
		duration = uint64(time.Since(s.state.turnStart).Milliseconds())
	}
	s.state.mu.Unlock()
	if active {
		s.emit(DriverEvent{Kind: EvTurnCompleted, StopReason: stop, DurationMs: duration})
	}
}

func (s *opencodeSession) SendMessage(ctx context.Context, messageID, text string) error {
	s.state.mu.Lock()
	sessionID := s.state.sessionID
	s.state.mu.Unlock()
	if sessionID == "" {
		return fmt.Errorf("no opencode session")
	}
	body := map[string]any{"text": text, "id": "msg_" + strings.ReplaceAll(messageID, "-", "")}
	payload, _ := json.Marshal(body)
	s.state.mu.Lock()
	s.state.active = true
	s.state.turnStart = time.Now()
	s.state.mu.Unlock()
	// v2 prompt submits the turn; completion streams over SSE as
	// session.execution.*. Use a detached context: the HTTP call returning
	// does not end the turn.
	err := s.doJSONRaw(context.WithoutCancel(ctx), http.MethodPost, "/api/session/"+sessionID+"/prompt", payload, nil)
	if err != nil {
		s.state.mu.Lock()
		s.state.active = false
		s.state.mu.Unlock()
	}
	return err
}

func (s *opencodeSession) Interrupt(ctx context.Context) error {
	s.state.mu.Lock()
	sessionID := s.state.sessionID
	s.state.mu.Unlock()
	if sessionID == "" {
		return nil
	}
	err := s.doJSON(ctx, http.MethodPost, "/api/session/"+sessionID+"/interrupt", map[string]any{}, nil)
	if err == nil {
		s.endTurn(StopInterrupted)
	}
	return err
}

// SetModel switches the live session's model via POST /api/session/{id}/model.
// Composite "provider/model" is split per the Model.Ref schema.
func (s *opencodeSession) SetModel(ctx context.Context, model string) error {
	provider, id, ok := strings.Cut(model, "/")
	if !ok {
		return fmt.Errorf("opencode models are named provider/model, got %s", model)
	}
	body := map[string]any{"model": map[string]any{"providerID": provider, "id": id}}
	if err := s.doJSON(ctx, http.MethodPost, "/api/session/"+s.state.sessionID+"/model", body, nil); err != nil {
		return err
	}
	s.state.mu.Lock()
	s.state.model = model
	s.state.mu.Unlock()
	return nil
}

func (s *opencodeSession) SetEffort(ctx context.Context, effort string) error {
	return fmt.Errorf("opencode did not report effort controls for this model")
}

// RespondPermission answers a pending permission request.
// decision: "allow" (once) | "allow_always" (always) | "deny" (reject).
func (s *opencodeSession) RespondPermission(ctx context.Context, requestID, decision string) error {
	s.state.mu.Lock()
	sessionID, ok := s.state.pending[requestID]
	if ok {
		delete(s.state.pending, requestID)
	}
	if sessionID == "" {
		sessionID = s.state.sessionID
	}
	s.state.mu.Unlock()
	if !ok && sessionID == "" {
		return fmt.Errorf("no pending permission %s", requestID)
	}
	action := "once"
	switch decision {
	case "allow":
		action = "once"
	case "allow_always":
		action = "always"
	case "deny":
		action = "reject"
	default:
		return fmt.Errorf("expected permission decision, got %q", decision)
	}
	body := map[string]any{"id": requestID, "action": action}
	if err := s.doJSON(ctx, http.MethodPost, "/api/session/"+sessionID+"/permission", body, nil); err != nil {
		return err
	}
	s.emit(DriverEvent{Kind: EvPermissionWithdrawn, RequestID: requestID})
	return nil
}
