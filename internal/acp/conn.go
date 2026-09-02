package acp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/hasdev/forge-ade/internal/agent"
)

// conn is a live JSON-RPC connection to one spawned ACP agent process.
type conn struct {
	cmd     *exec.Cmd
	stdout  io.ReadCloser
	stdin   chan []byte
	pending map[uint64]chan rpcMessage
	mu      sync.Mutex
	nextID  uint64
	dead    bool
	manager *Manager

	// stream state for the in-flight prompt turn
	streamMu      sync.Mutex
	streamSession string // our session id receiving updates, "" when idle
	streamMsgID   string // assistant message being appended to
}

type rpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *uint64         `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *rpcError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("rpc error %d: %s", e.Code, e.Message)
}

// spawn launches the agent subprocess with stdin/stdout pipes.
func spawn(cfg *AgentConfig) (*conn, error) {
	cmd := exec.Command(cfg.Command, cfg.Args...)
	if len(cfg.Env) > 0 {
		cmd.Env = append(os.Environ(), flattenEnv(cfg.Env)...)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("spawn %s: %w", cfg.Command, err)
	}
	c := &conn{
		cmd:     cmd,
		stdout:  stdout,
		stdin:   make(chan []byte, 32),
		pending: map[uint64]chan rpcMessage{},
	}
	go func() {
		w := bufio.NewWriter(stdin)
		for line := range c.stdin {
			if _, err := w.Write(line); err != nil {
				break
			}
			if err := w.WriteByte('\n'); err != nil {
				break
			}
			if err := w.Flush(); err != nil {
				break
			}
		}
		stdin.Close()
	}()
	return c, nil
}

func flattenEnv(env map[string]string) []string {
	out := make([]string, 0, len(env))
	for k, v := range env {
		out = append(out, k+"="+v)
	}
	return out
}

// close kills the process and fails pending calls.
func (c *conn) close() {
	c.mu.Lock()
	if c.dead {
		c.mu.Unlock()
		return
	}
	c.dead = true
	for id, ch := range c.pending {
		close(ch)
		delete(c.pending, id)
	}
	c.mu.Unlock()
	close(c.stdin)
	if c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
	}
}

func (c *conn) nextCallID() uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.nextID++
	return c.nextID
}

// call sends a JSON-RPC request and waits for the matching response.
func (c *conn) call(ctx context.Context, method string, params any, out any) (json.RawMessage, error) {
	id := c.nextCallID()
	ch := make(chan rpcMessage, 1)
	c.mu.Lock()
	if c.dead {
		c.mu.Unlock()
		return nil, fmt.Errorf("connection closed")
	}
	c.pending[id] = ch
	c.mu.Unlock()

	if err := c.write(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params}); err != nil {
		return nil, err
	}

	select {
	case msg, ok := <-ch:
		if !ok {
			return nil, fmt.Errorf("connection closed")
		}
		if msg.Error != nil {
			return nil, msg.Error
		}
		if out != nil && len(msg.Result) > 0 {
			if err := json.Unmarshal(msg.Result, out); err != nil {
				return msg.Result, fmt.Errorf("decode %s result: %w", method, err)
			}
		}
		return msg.Result, nil
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, ctx.Err()
	}
}

// notify sends a notification (no id, no response expected).
func (c *conn) notify(method string, params any) {
	_ = c.write(map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
}

// respond answers a request from the agent.
func (c *conn) respond(id uint64, result any) {
	_ = c.write(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
}

func (c *conn) write(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	select {
	case c.stdin <- data:
		return nil
	default:
		return fmt.Errorf("agent stdin backlog")
	}
}

// readLoop parses newline-delimited JSON-RPC messages from the agent.
func (c *conn) readLoop() {
	sc := bufio.NewScanner(c.stdout)
	sc.Buffer(make([]byte, 1024*1024), 16*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var msg rpcMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}
		switch {
		case msg.ID != nil && msg.Method != "":
			c.handleRequest(*msg.ID, msg.Method, msg.Params)
		case msg.ID != nil:
			c.mu.Lock()
			ch, ok := c.pending[*msg.ID]
			if ok {
				delete(c.pending, *msg.ID)
			}
			c.mu.Unlock()
			if ok {
				ch <- msg
			}
		case msg.Method != "":
			c.handleNotification(msg.Method, msg.Params)
		}
	}
	c.mu.Lock()
	c.dead = true
	for id, ch := range c.pending {
		close(ch)
		delete(c.pending, id)
	}
	c.mu.Unlock()
}

// ---------------------------------------------------------------------------
// Agent → client requests
// ---------------------------------------------------------------------------

func (c *conn) handleRequest(id uint64, method string, params json.RawMessage) {
	m := c.manager
	switch method {
	case "fs/read_text_file":
		var p struct {
			Path  string `json:"path"`
			Line  *int   `json:"line"`
			Limit *int   `json:"limit"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			c.respondErr(id, -32602, err.Error())
			return
		}
		data, err := os.ReadFile(p.Path)
		if err != nil {
			c.respondErr(id, -32000, err.Error())
			return
		}
		content := string(data)
		if p.Line != nil || p.Limit != nil {
			content = sliceLines(content, p.Line, p.Limit)
		}
		c.respond(id, map[string]any{"content": content})

	case "fs/write_text_file":
		var p struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			c.respondErr(id, -32602, err.Error())
			return
		}
		if dir := filepath.Dir(p.Path); dir != "" {
			_ = os.MkdirAll(dir, 0755)
		}
		if err := os.WriteFile(p.Path, []byte(p.Content), 0644); err != nil {
			c.respondErr(id, -32000, err.Error())
			return
		}
		m.emitEvent("fs:changed", "", map[string]interface{}{"type": "modified", "path": p.Path})
		c.respond(id, map[string]any{})

	case "session/request_permission":
		var p struct {
			SessionID string         `json:"sessionId"`
			ToolCall  map[string]any `json:"toolCall"`
			Options   []struct {
				OptionID string `json:"optionId"`
				Name     string `json:"name"`
				Kind     string `json:"kind"`
			} `json:"options"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			c.respondErr(id, -32602, err.Error())
			return
		}
		opts := make([]PermissionOption, 0, len(p.Options))
		for _, o := range p.Options {
			opts = append(opts, PermissionOption{OptionID: o.OptionID, Name: o.Name, Kind: o.Kind})
		}
		pending := &PendingPermission{RequestID: id, ToolCall: p.ToolCall, Options: opts}

		m.mu.Lock()
		s, ok := m.sessions[p.SessionID]
		if !ok {
			m.mu.Unlock()
			c.respondErr(id, -32000, "unknown session")
			return
		}
		s.PendingPermission = pending
		if s.AutoApprove {
			// pick the first allow option immediately
			choice := ""
			for _, o := range opts {
				if strings.HasPrefix(o.Kind, "allow") {
					choice = o.OptionID
					break
				}
			}
			if choice == "" && len(opts) > 0 {
				choice = opts[0].OptionID
			}
			s.PendingPermission = nil
			m.mu.Unlock()
			c.respond(id, map[string]any{
				"outcome": map[string]any{"optionId": choice, "outcome": "selected"},
			})
			return
		}
		s.State = "awaiting_approval"
		s.UpdatedAt = time.Now()
		m.mu.Unlock()
		m.emitUpdate(p.SessionID)

	default:
		c.respondErr(id, -32601, "method not supported: "+method)
	}
}

func (c *conn) respondErr(id uint64, code int, msg string) {
	_ = c.write(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"error":   map[string]any{"code": code, "message": msg},
	})
}

func sliceLines(content string, line, limit *int) string {
	lines := strings.Split(content, "\n")
	start := 0
	if line != nil && *line > 0 {
		start = *line - 1
	}
	if start > len(lines) {
		start = len(lines)
	}
	end := len(lines)
	if limit != nil && start+*limit < end {
		end = start + *limit
	}
	return strings.Join(lines[start:end], "\n")
}

// ---------------------------------------------------------------------------
// session/update notifications
// ---------------------------------------------------------------------------

func (c *conn) handleNotification(method string, params json.RawMessage) {
	if method != "session/update" {
		return
	}
	var p struct {
		SessionID string `json:"sessionId"`
		Update    struct {
			SessionUpdate string          `json:"sessionUpdate"`
			Content       json.RawMessage `json:"content"`
			ToolCallID    string          `json:"toolCallId"`
			Title         string          `json:"title"`
			Kind          string          `json:"kind"`
			Status        string          `json:"status"`
			RawInput      json.RawMessage `json:"rawInput"`
			RawOutput     json.RawMessage `json:"rawOutput"`
			Entries       []struct {
				Content  string `json:"content"`
				Priority string `json:"priority"`
				Status   string `json:"status"`
			} `json:"entries"`
		} `json:"update"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return
	}

	c.streamMu.Lock()
	sessionID := c.streamSession
	msgID := c.streamMsgID
	c.streamMu.Unlock()
	if sessionID == "" {
		return
	}

	m := c.manager
	switch p.Update.SessionUpdate {
	case "agent_message_chunk":
		c.appendBlock(sessionID, msgID, textOf(p.Update.Content), false)
	case "agent_thought_chunk":
		c.appendBlock(sessionID, msgID, textOf(p.Update.Content), true)
	case "tool_call":
		c.addToolCall(sessionID, msgID, p.Update.ToolCallID, p.Update.Title, p.Update.Kind, p.Update.RawInput)
	case "tool_call_update":
		c.completeToolCall(sessionID, msgID, p.Update.ToolCallID, p.Update.Status, p.Update.Title, p.Update.RawOutput)
	case "plan":
		c.appendPlan(sessionID, msgID, p.Update.Entries)
	}
	m.emitUpdate(sessionID)
}

func textOf(raw json.RawMessage) string {
	var blk struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &blk); err != nil {
		return ""
	}
	return blk.Text
}

// beginStream marks the connection as streaming updates into the given
// session/assistant message.
func (c *conn) beginStream(sessionID, assistantMsgID string) {
	c.streamMu.Lock()
	c.streamSession = sessionID
	c.streamMsgID = assistantMsgID
	c.streamMu.Unlock()
}

func (c *conn) endStream() {
	c.streamMu.Lock()
	c.streamSession = ""
	c.streamMsgID = ""
	c.streamMu.Unlock()
}

func (c *conn) mutateSession(sessionID string, fn func(s *ACPSession)) {
	m := c.manager
	m.mu.Lock()
	if s, ok := m.sessions[sessionID]; ok {
		fn(s)
		s.UpdatedAt = time.Now()
	}
	m.mu.Unlock()
}

func (c *conn) appendBlock(sessionID, msgID, text string, thinking bool) {
	if text == "" {
		return
	}
	kind := "text"
	if thinking {
		kind = "thinking"
	}
	c.mutateSession(sessionID, func(s *ACPSession) {
		for i := range s.Messages {
			if s.Messages[i].ID != msgID {
				continue
			}
			blocks := s.Messages[i].Content
			if n := len(blocks); n > 0 && blocks[n-1].Type == kind {
				blocks[n-1].Text += text
			} else {
				s.Messages[i].Content = append(blocks, agent.ContentBlock{Type: kind, Text: text})
			}
			if s.State == "thinking" && !thinking {
				s.State = "executing"
			}
		}
	})
}

func (c *conn) addToolCall(sessionID, msgID, toolCallID, title, kind string, rawInput json.RawMessage) {
	c.mutateSession(sessionID, func(s *ACPSession) {
		for i := range s.Messages {
			if s.Messages[i].ID != msgID {
				continue
			}
			args := map[string]any{}
			if len(rawInput) > 0 {
				_ = json.Unmarshal(rawInput, &args)
			}
			s.Messages[i].Content = append(s.Messages[i].Content, agent.ContentBlock{
				Type:       "tool_call",
				ToolCallID: toolCallID,
				Name:       title,
				Arguments:  args,
			})
		}
	})
}

func (c *conn) completeToolCall(sessionID, msgID, toolCallID, status, title string, rawOutput json.RawMessage) {
	c.mutateSession(sessionID, func(s *ACPSession) {
		for i := range s.Messages {
			if s.Messages[i].ID != msgID {
				continue
			}
			output := ""
			if len(rawOutput) > 0 {
				var o struct {
					Output  string `json:"output"`
					Content string `json:"content"`
					Kind    string `json:"kind"`
					Title   string `json:"title"`
					Text    string `json:"text"`
				}
				if err := json.Unmarshal(rawOutput, &o); err == nil {
					output = o.Output
					if output == "" {
						output = o.Text
					}
					if output == "" {
						output = o.Content
					}
					if output == "" && o.Title != "" {
						output = o.Title
					}
				} else {
					output = string(rawOutput)
				}
			}
			if title != "" && output == "" {
				output = title
			}
			s.Messages[i].Content = append(s.Messages[i].Content, agent.ContentBlock{
				Type:       "tool_result",
				ToolCallID: toolCallID,
				Name:       title,
				Text:       output,
				IsError:    status == "failed",
			})
		}
	})
}

func (c *conn) appendPlan(sessionID, msgID string, entries []struct {
	Content  string `json:"content"`
	Priority string `json:"priority"`
	Status   string `json:"status"`
}) {
	if len(entries) == 0 {
		return
	}
	var sb strings.Builder
	sb.WriteString("\n\nPlan:\n")
	for _, e := range entries {
		mark := "[ ]"
		switch e.Status {
		case "completed":
			mark = "[x]"
		case "in_progress":
			mark = "[~]"
		}
		sb.WriteString(fmt.Sprintf("- %s %s (%s)\n", mark, e.Content, e.Priority))
	}
	c.appendBlock(sessionID, msgID, sb.String(), false)
}
