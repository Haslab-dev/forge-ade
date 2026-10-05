package acp

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/hasdev/forge-ade/internal/agent"
	"github.com/hasdev/forge-ade/internal/events"
)

// PermissionMode mirrors kybern's driver-level permission mode.
type PermissionMode int

const (
	ModeSupervised PermissionMode = iota
	ModeAcceptEdits
	ModeAuto
	ModeFullAccess
)

// DriverEventKind is the normalized event stream every driver publishes.
// One session = one live process = one stream, like kybern-drivers.
type DriverEventKind int

const (
	EvSessionBound DriverEventKind = iota
	EvTextDelta
	EvThinkingDelta
	EvMessageCompleted
	EvToolStarted
	EvToolCompleted
	EvToolOutputDelta
	EvPermissionRequest
	EvPermissionWithdrawn
	EvTurnCompleted
	EvTurnFailed
	EvNotice
	EvExited
)

// StopReason explains why a turn ended.
type StopReason int

const (
	StopCompleted StopReason = iota
	StopInterrupted
	StopMaxTurns
)

// DriverEvent is the provider-neutral event the Rust drivers emit; here it is
// consumed directly by the session adapter in acp.go.
type DriverEvent struct {
	Kind DriverEventKind

	// SessionBound
	SessionID string
	Model     string

	// Deltas and completions
	MessageID string
	Delta     string
	Text      string
	Thinking  string

	// Tool lifecycle
	ToolCallID string
	ToolName   string
	Input      json.RawMessage
	Output     json.RawMessage
	IsError    bool

	// Permissions
	RequestID   string
	Summary     string
	Suggestions []json.RawMessage

	// Turn completion
	StopReason StopReason
	Usage      Usage
	CostUSD    *float64
	DurationMs uint64

	// Notices / exit
	NoticeLevel string // info | warning | error
	Error       string
	ExitCode    *int
}

// Usage counts tokens for a turn.
type Usage struct {
	InputTokens      uint64
	OutputTokens     uint64
	CacheReadTokens  uint64
	CacheWriteTokens uint64
}

func (u *Usage) add(o Usage) {
	u.InputTokens += o.InputTokens
	u.OutputTokens += o.OutputTokens
	u.CacheReadTokens += o.CacheReadTokens
	u.CacheWriteTokens += o.CacheWriteTokens
}

// SessionConfig is everything a driver needs to start (or resume) a session.
type SessionConfig struct {
	CWD             string
	Binary          string
	Env             map[string]string
	ResumeSessionID string
	Fork            bool
	Model           string
	Effort          string
	PermissionMode  PermissionMode
}

// AgentSession is one live provider session. The Go Manager owns one per
// ACPSession, matching kybern's one-process-per-thread shape.
type AgentSession interface {
	// SendMessage submits one user turn. Completion/failure arrives as an
	// EvTurnCompleted/EvTurnFailed event on the session's event channel.
	SendMessage(ctx context.Context, messageID string, text string) error
	Interrupt(ctx context.Context) error
	SetModel(ctx context.Context, model string) error
	SetEffort(ctx context.Context, effort string) error
	// RespondPermission answers a pending request. decision is driver-specific:
	// "allow", "allow_always", "deny", or a JSON payload for dialogs.
	RespondPermission(ctx context.Context, requestID, decision string) error
	Close() error
	Done() <-chan struct{}
}

// AgentDriver spawns sessions speaking the provider's own protocol.
type AgentDriver interface {
	// ID returns the driver kind: claude | codex | opencode | pi | omp.
	ID() string
	// Spawn launches one process (or server) and performs the handshake.
	// The returned session's Done() channel closes when the process exits.
	Spawn(ctx context.Context, cfg SessionConfig) (AgentSession, <-chan DriverEvent, error)
	// CheckBinary reports whether the provider's binary can be found.
	CheckBinary(command string) (bool, string)
}

// ---------------------------------------------------------------------------
// ndjsonChild: line-delimited JSON over a child's stdio (port of ndjson.rs).
// ---------------------------------------------------------------------------

type ndjsonChild struct {
	cmd    *exec.Cmd
	stdin  writeCloser
	stderr *stderrTee

	mu       sync.Mutex
	stdinMu  sync.Mutex
	closed   bool
	waitCh   chan struct{}
	waitCode *int
}

type writeCloser interface {
	Write(p []byte) (int, error)
	Close() error
}

type stderrTee struct {
	mu    sync.Mutex
	lines []string
}

func (t *stderrTee) add(line string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.lines = append(t.lines, line)
	if len(t.lines) > 200 {
		t.lines = t.lines[len(t.lines)-200:]
	}
}

func (t *stderrTee) tail(n int) string {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.lines) > n {
		t.lines = t.lines[len(t.lines)-n:]
	}
	return strings.Join(t.lines, "\n")
}

func spawnChild(cmd *exec.Cmd) (*ndjsonChild, <-chan json.RawMessage, error) {
	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, err
	}
	tee := &stderrTee{}
	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("spawn %s: %w", cmd.Path, err)
	}
	child := &ndjsonChild{
		cmd:    cmd,
		stdin:  stdinPipe,
		stderr: tee,
		waitCh: make(chan struct{}),
	}

	lines := make(chan json.RawMessage, 1024)
	go func() {
		sc := bufio.NewScanner(stdout)
		sc.Buffer(make([]byte, 1024*1024), 64*1024*1024)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line == "" {
				continue
			}
			if !json.Valid([]byte(line)) {
				continue
			}
			select {
			case lines <- json.RawMessage(line):
			case <-child.waitCh:
				return
			}
		}
	}()
	go func() {
		sc := bufio.NewScanner(stderr)
		sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
		for sc.Scan() {
			tee.add(sc.Text())
		}
	}()
	go func() {
		err := cmd.Wait()
		child.mu.Lock()
		code := cmd.ProcessState.ExitCode()
		child.waitCode = &code
		child.closed = true
		child.mu.Unlock()
		if err != nil && code == 0 {
			// Signaled or otherwise failed; report non-zero.
			c := 1
			child.waitCode = &c
		}
		close(child.waitCh)
		// Unblock writers after exit.
	}()
	return child, lines, nil
}

// Write sends one JSON frame followed by a newline.
func (c *ndjsonChild) write(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	c.stdinMu.Lock()
	defer c.stdinMu.Unlock()
	c.mu.Lock()
	closed := c.closed
	c.mu.Unlock()
	if closed {
		return fmt.Errorf("session is closed; send again to resume")
	}
	if _, err := c.stdin.Write(append(data, '\n')); err != nil {
		return err
	}
	// No explicit flush: os/exec pipes are unbuffered *os.File.
	return nil
}

// kill terminates the process tree (process group on unix).
func (c *ndjsonChild) kill() {
	c.mu.Lock()
	already := c.closed
	c.mu.Unlock()
	if already || c.cmd.Process == nil {
		return
	}
	killProcessGroup(c.cmd)
}

// close waits up to 5s for graceful exit after stdin EOF, then kills.
func (c *ndjsonChild) close() {
	c.stdinMu.Lock()
	_ = c.stdin.Close()
	c.stdinMu.Unlock()
	select {
	case <-c.waitCh:
	case <-time.After(5 * time.Second):
		c.kill()
	}
}

// exitCode returns the recorded exit code once the process is gone.
func (c *ndjsonChild) exitCode() (int, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.waitCode == nil {
		return 0, false
	}
	return *c.waitCode, true
}

// killProcessGroup signals the whole group on unix so shell wrappers die too.
func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	_ = cmd.Process.Kill()
}

// ---------------------------------------------------------------------------
// Shared binary resolution (port of binary.rs + conn.go resolveBinary).
// ---------------------------------------------------------------------------

func resolveBinary(command string) string {
	if command == "" {
		return ""
	}
	if filepath.IsAbs(command) {
		if _, err := os.Stat(command); err == nil {
			return command
		}
		return ""
	}
	if p, err := exec.LookPath(command); err == nil {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return command
	}
	candidates := []string{
		filepath.Join(home, ".local", "bin", command),
		filepath.Join(home, ".opencode", "bin", command),
		filepath.Join(home, ".bun", "bin", command),
		filepath.Join(home, ".cargo", "bin", command),
		"/opt/homebrew/bin/" + command,
		"/opt/homebrew/sbin/" + command,
		"/usr/local/bin/" + command,
		"/usr/bin/" + command,
		"/bin/" + command,
	}
	if nvmBin := os.Getenv("NVM_BIN"); nvmBin != "" {
		candidates = append(candidates, filepath.Join(nvmBin, command))
	}
	if nvmNodes, _ := filepath.Glob(filepath.Join(home, ".nvm", "versions", "node", "*", "bin", command)); len(nvmNodes) > 0 {
		for i := len(nvmNodes) - 1; i >= 0; i-- {
			candidates = append(candidates, nvmNodes[i])
		}
	}
	candidates = append(candidates,
		filepath.Join(home, ".fnm", "current", "bin", command),
		filepath.Join(home, ".local", "share", "fnm", "current", "bin", command),
		filepath.Join(home, ".asdf", "shims", command),
		filepath.Join(home, ".volta", "bin", command),
		filepath.Join(home, ".npm-global", "bin", command),
		filepath.Join(home, "Library", "pnpm", command),
	)
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return command
}

// childEnv builds the process environment: inherited env, a broad PATH (so
// node-managed CLIs resolve), ~/.zshrc exports, then the caller's overrides.
func childEnv(cfg SessionConfig) []string {
	envMap := map[string]string{}
	for _, kv := range os.Environ() {
		if i := strings.IndexByte(kv, '='); i > 0 {
			envMap[kv[:i]] = kv[i+1:]
		}
	}
	home, _ := os.UserHomeDir()
	currentPath := envMap["PATH"]
	extraDirs := []string{
		"/opt/homebrew/bin", "/opt/homebrew/sbin", "/usr/local/bin", "/usr/bin", "/bin",
		filepath.Join(home, ".local", "bin"),
		filepath.Join(home, ".opencode", "bin"),
		filepath.Join(home, ".bun", "bin"),
		filepath.Join(home, ".cargo", "bin"),
	}
	if nvmBin := os.Getenv("NVM_BIN"); nvmBin != "" {
		extraDirs = append(extraDirs, nvmBin)
	}
	if nvmNodes, _ := filepath.Glob(filepath.Join(home, ".nvm", "versions", "node", "*", "bin")); len(nvmNodes) > 0 {
		for i := len(nvmNodes) - 1; i >= 0; i-- {
			extraDirs = append(extraDirs, nvmNodes[i])
		}
	}
	if fnm := filepath.Join(home, ".fnm", "current", "bin"); fnm != "" {
		extraDirs = append(extraDirs, fnm)
	}
	for _, d := range extraDirs {
		if d != "" && !strings.Contains(currentPath, d) {
			currentPath = d + ":" + currentPath
		}
	}
	envMap["PATH"] = currentPath
	loadShellEnv(home, envMap)
	for k, v := range cfg.Env {
		envMap[k] = v
	}
	out := make([]string, 0, len(envMap))
	for k, v := range envMap {
		out = append(out, k+"="+v)
	}
	return out
}

func loadShellEnv(home string, envMap map[string]string) {
	if home == "" {
		return
	}
	data, err := os.ReadFile(filepath.Join(home, ".zshrc"))
	if err != nil {
		return
	}
	for _, l := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(l)
		if !strings.HasPrefix(trimmed, "export ") {
			continue
		}
		rest := strings.TrimSpace(strings.TrimPrefix(trimmed, "export "))
		key, val, ok := strings.Cut(rest, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.Trim(strings.TrimSpace(val), `"'`)
		if key != "" && envMap[key] == "" {
			envMap[key] = val
		}
	}
}

// ---------------------------------------------------------------------------
// Session-side helpers shared by the driver sessions: emitting turn results
// into the Manager's session state (replaces conn.mutateSession etc.).
// ---------------------------------------------------------------------------

// sessionSink adapts raw DriverEvents into Manager session state + UI events.
// Each spawned session gets one bound to its ACPSession id.
type sessionSink struct {
	m                 *Manager
	sessionID         string
	providerSessionID string
	msgID             string // assistant message receiving the turn's output
	usage             Usage
	started           time.Time

	streamMu   sync.Mutex
	streamedBy map[string]string // driver message id -> text already streamed

	turnMu   sync.Mutex
	turnDone chan struct{} // closed when the active turn reaches a terminal event
}

func (s *sessionSink) mutate(fn func(sess *ACPSession)) { s.m.mutateLockedSession(s.sessionID, fn) }

// beginTurn binds the sink to the assistant message receiving this turn's
// output and arms the turn-completion gate that Send waits on.
func (s *sessionSink) beginTurn(assistantMsgID string) {
	s.msgID = assistantMsgID
	s.turnMu.Lock()
	s.turnDone = make(chan struct{})
	s.turnMu.Unlock()
}

// waitTurnDone blocks until the active turn reaches a terminal event
// (TurnCompleted / TurnFailed / Exited) or ctx is cancelled.
func (s *sessionSink) waitTurnDone(ctx context.Context) error {
	s.turnMu.Lock()
	done := s.turnDone
	s.turnMu.Unlock()
	if done == nil {
		return nil
	}
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// signalTurnDone releases a pending waitTurnDone, if any.
func (s *sessionSink) signalTurnDone() {
	s.turnMu.Lock()
	if s.turnDone != nil {
		close(s.turnDone)
		s.turnDone = nil
	}
	s.turnMu.Unlock()
}

// codexMetadataBanner matches the notice codex app-server emits when a model
// is missing from its price catalog. It is provider noise, not assistant text.
var codexMetadataBanner = regexp.MustCompile(`(?s)^Model metadata for ` + "`[^`]*`" + ` not found\. Defaulting to fallback metadata; this can degrade performance and cause issues\.`)

// stripHarnessBanner removes known provider banner prefixes from a delta.
func stripHarnessBanner(delta string) string {
	if loc := codexMetadataBanner.FindStringIndex(delta); loc != nil && loc[0] == 0 {
		return delta[loc[1]:]
	}
	return delta
}

// recordStreamed remembers text streamed for a driver message id.
func (s *sessionSink) recordStreamed(driverMsgID, delta string) {
	if driverMsgID == "" {
		return
	}
	s.streamMu.Lock()
	if s.streamedBy == nil {
		s.streamedBy = map[string]string{}
	}
	s.streamedBy[driverMsgID] += delta
	s.streamMu.Unlock()
}

// unstreamedTail returns the portion of the final text the deltas did not
// already stream (false when everything was streamed or there is no final).
func (s *sessionSink) unstreamedTail(driverMsgID, finalText string) (string, bool) {
	if finalText == "" {
		return "", false
	}
	s.streamMu.Lock()
	streamed := s.streamedBy[driverMsgID]
	s.streamMu.Unlock()
	if driverMsgID == "" || !strings.HasPrefix(finalText, streamed) {
		return finalText, true
	}
	return finalText[len(streamed):], true
}

// mutateLockedSession applies fn to the session under the Manager lock.
func (m *Manager) mutateLockedSession(sessionID string, fn func(sess *ACPSession)) {
	m.mu.Lock()
	if s, ok := m.sessions[sessionID]; ok {
		fn(s)
		s.UpdatedAt = time.Now()
	}
	m.mu.Unlock()
}

// extractOutputText pulls human-readable text out of a provider tool output
// payload (shared by the sinks of every driver).
func extractOutputText(rawOutput json.RawMessage) string {
	if len(rawOutput) == 0 {
		return ""
	}
	var o struct {
		Output  string          `json:"output"`
		Content json.RawMessage `json:"content"`
		Text    string          `json:"text"`
	}
	if err := json.Unmarshal(rawOutput, &o); err == nil {
		if o.Output != "" {
			return o.Output
		}
		if o.Text != "" {
			return o.Text
		}
		if len(o.Content) > 0 {
			var s string
			if json.Unmarshal(o.Content, &s) == nil {
				return s
			}
		}
	}
	return string(rawOutput)
}

func (s *sessionSink) appendText(text string, thinking bool) {
	if text == "" {
		return
	}
	kind := "text"
	if thinking {
		kind = "thinking"
	}
	s.mutate(func(sess *ACPSession) {
		for i := range sess.Messages {
			if sess.Messages[i].ID != s.msgID {
				continue
			}
			blocks := sess.Messages[i].Content
			if n := len(blocks); n > 0 && blocks[n-1].Type == kind {
				blocks[n-1].Text += text
			} else {
				sess.Messages[i].Content = append(blocks, agent.ContentBlock{Type: kind, Text: text})
			}
			if sess.State == "thinking" && !thinking {
				sess.State = "executing"
			}
		}
	})
}

func (s *sessionSink) addToolCall(toolCallID, name string, input json.RawMessage) {
	s.mutate(func(sess *ACPSession) {
		for i := range sess.Messages {
			if sess.Messages[i].ID != s.msgID {
				continue
			}
			args := map[string]any{}
			if len(input) > 0 {
				_ = json.Unmarshal(input, &args)
			}
			sess.Messages[i].Content = append(sess.Messages[i].Content, agent.ContentBlock{
				Type: "tool_call", ToolCallID: toolCallID, Name: name, Arguments: args,
			})
		}
	})
}

func (s *sessionSink) completeToolCall(toolCallID, status, title string, output json.RawMessage) string {
	resolved := title
	s.mutate(func(sess *ACPSession) {
		for i := range sess.Messages {
			if sess.Messages[i].ID != s.msgID {
				continue
			}
			if resolved == "" {
				for _, blk := range sess.Messages[i].Content {
					if blk.Type == "tool_call" && blk.ToolCallID == toolCallID && blk.Name != "" {
						resolved = blk.Name
						break
					}
				}
			}
			out := extractOutputText(output)
			if resolved != "" && out == "" {
				out = resolved
			}
			sess.Messages[i].Content = append(sess.Messages[i].Content, agent.ContentBlock{
				Type: "tool_result", ToolCallID: toolCallID, Name: resolved, Text: out, IsError: status == "failed",
			})
		}
	})
	return resolved
}

func (s *sessionSink) setPendingPermission(p *PendingPermission) {
	s.mutate(func(sess *ACPSession) {
		sess.PendingPermission = p
		if p != nil {
			sess.State = "awaiting_approval"
		} else if sess.State == "awaiting_approval" {
			sess.State = "executing"
		}
	})
}

func (s *sessionSink) emit(evType events.EventType, extra map[string]interface{}) {
	data := map[string]interface{}{"session_id": s.sessionID}
	for k, v := range extra {
		data[k] = v
	}
	s.m.emitEvent(evType, s.sessionID, data)
}

// drain consumes normalized driver events until the session ends, projecting
// them into Manager state. This is the Go counterpart of the daemon's
// orchestrator fold over DriverEvents.
func (s *sessionSink) drain(evs <-chan DriverEvent, done <-chan struct{}) {
	for {
		select {
		case ev, ok := <-evs:
			if !ok {
				return
			}
			s.apply(ev)
		case <-done:
			return
		}
	}
}

func (s *sessionSink) apply(ev DriverEvent) {
	switch ev.Kind {
	case EvSessionBound:
		s.mutate(func(sess *ACPSession) { sess.ProviderID = ev.SessionID })
		s.providerSessionID = ev.SessionID
		s.m.mu.Lock()
		s.m.saveSessionsLocked()
		s.m.mu.Unlock()
	case EvTextDelta:
		stripped := stripHarnessBanner(ev.Delta)
		s.recordStreamed(ev.MessageID, stripped) // stripped: matches cleaned final
		if stripped != "" {
			s.appendText(stripped, false)
			s.emit(events.AgentMessageDelta, map[string]interface{}{"message_id": s.msgID, "delta": stripped, "kind": "text"})
		}
		s.m.emitUpdate(s.sessionID)
	case EvThinkingDelta:
		s.appendText(ev.Delta, true)
		s.emit(events.AgentThinkingDelta, map[string]interface{}{"message_id": s.msgID, "delta": ev.Delta})
		s.m.emitUpdate(s.sessionID)
	case EvMessageCompleted:
		// Deltas already streamed most text; append only the unstreamed tail
		// so the settled block matches the provider's exact final text. The
		// final text carries provider banners that deltas already stripped.
		clean := stripHarnessBanner(ev.Text)
		if tail, ok := s.unstreamedTail(ev.MessageID, clean); ok && tail != "" {
			s.appendText(tail, false)
		}
	case EvToolStarted:
		s.addToolCall(ev.ToolCallID, ev.ToolName, ev.Input)
		s.emit(events.AgentToolStart, map[string]interface{}{
			"message_id": s.msgID, "tool_call_id": ev.ToolCallID, "title": ev.ToolName, "kind": ev.ToolName, "raw_input": string(ev.Input),
		})
		s.m.emitUpdate(s.sessionID)
	case EvToolCompleted:
		resolved := s.completeToolCall(ev.ToolCallID, statusOf(!ev.IsError), ev.ToolName, ev.Output)
		out := extractOutputText(ev.Output)
		if out == "" && resolved != "" {
			out = resolved
		}
		s.emit(events.AgentToolEnd, map[string]interface{}{
			"message_id": s.msgID, "tool_call_id": ev.ToolCallID, "status": statusOf(!ev.IsError), "title": resolved, "raw_output": out,
		})
		s.m.emitUpdate(s.sessionID)
	case EvPermissionRequest:
		opts := []PermissionOption{}
		for _, sugg := range ev.Suggestions {
			opts = append(opts, PermissionOption{OptionID: string(sugg), Name: string(sugg), Kind: "allow"})
		}
		if len(opts) == 0 {
			opts = []PermissionOption{
				{OptionID: "allow", Name: "Allow", Kind: "allow_once"},
				{OptionID: "deny", Name: "Deny", Kind: "reject_once"},
			}
		}
		toolCall := map[string]any{"tool": ev.ToolName, "summary": ev.Summary}
		if len(ev.Input) > 0 {
			var in map[string]any
			_ = json.Unmarshal(ev.Input, &in)
			toolCall["input"] = in
		}
		s.setPendingPermission(&PendingPermission{RequestIDStr: ev.RequestID, ToolCall: toolCall, Options: opts})
		s.m.emitUpdate(s.sessionID)
	case EvPermissionWithdrawn:
		s.setPendingPermission(nil)
		s.m.emitUpdate(s.sessionID)
	case EvNotice:
		// Notices are provider chatter (metadata warnings, retry hints). They
		// are not model output and not user-actionable thinking; drop them
		// from the UI stream entirely.
		_ = ev.Error
	case EvTurnCompleted:
		s.m.finishTurn(s.sessionID, s.msgID, "", false)
		s.signalTurnDone()
	case EvTurnFailed:
		s.m.finishTurn(s.sessionID, s.msgID, ev.Error, true)
		s.signalTurnDone()
	case EvExited:
		s.signalTurnDone()
	}
}

func statusOf(ok bool) string {
	if ok {
		return "completed"
	}
	return "failed"
}

func newUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
