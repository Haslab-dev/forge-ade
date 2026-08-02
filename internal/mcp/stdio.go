package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
)

// JSON-RPC 2.0 message types used by MCP. This is the Go port of
// the MCP stdio transport (JSON-RPC 2.0 over a subprocess).

// jsonRpcError is a JSON-RPC 2.0 error object.
type jsonRpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (e *jsonRpcError) Error() string {
	return fmt.Sprintf("MCP error %d: %s", e.Code, e.Message)
}

// jsonRpcRequest is a client→server request.
type jsonRpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

// jsonRpcResponse is a server→client response.
type jsonRpcResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      any           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonRpcError `json:"error,omitempty"`
}

// jsonRpcNotification is a one-way message (no id).
type jsonRpcNotification struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

// requestIDAllocator is a monotonic request id allocator.
type requestIDAllocator struct {
	n atomic.Int64
}

func (a *requestIDAllocator) next() int64 {
	return a.n.Add(1)
}

// pendingRequest is an in-flight request awaiting a response.
type pendingRequest struct {
	resolve chan json.RawMessage
	reject  chan error
}

// stdioTransport implements JSON-RPC 2.0 over a subprocess's stdin/stdout,
// using newline-delimited JSON frames.
type stdioTransport struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	mu     sync.Mutex
	closed bool

	ids      requestIDAllocator
	pending  map[int64]*pendingRequest
	pendingMu sync.Mutex

	onNotification func(method string, params json.RawMessage)
	onClose        func()
}

// newStdioTransport spawns the MCP server subprocess and starts the read loop.
func newStdioTransport(ctx context.Context, command string, args []string, env map[string]string) (*stdioTransport, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Env = mergeEnv(env)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("mcp stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("mcp stdout pipe: %w", err)
	}
	cmd.Stderr = nil // server logs are discarded (MCP spec allows this)

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("mcp spawn %s: %w", command, err)
	}

	t := &stdioTransport{
		cmd:     cmd,
		stdin:   stdin,
		pending: make(map[int64]*pendingRequest),
	}
	go t.readLoop(stdout)
	return t, nil
}

// request sends a request and waits for the matching response.
func (t *stdioTransport) request(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := t.ids.next()
	req := jsonRpcRequest{JSONRPC: "2.0", ID: id, Method: method, Params: params}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("mcp marshal request: %w", err)
	}

	pending := &pendingRequest{
		resolve: make(chan json.RawMessage, 1),
		reject:  make(chan error, 1),
	}
	t.pendingMu.Lock()
	if t.closed {
		t.pendingMu.Unlock()
		return nil, errors.New("mcp transport not connected")
	}
	t.pending[id] = pending
	t.pendingMu.Unlock()

	// Cleanup on context cancel.
	cleanup := func() {
		t.pendingMu.Lock()
		if t.pending[id] == pending {
			delete(t.pending, id)
		}
		t.pendingMu.Unlock()
	}

	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		cleanup()
		return nil, errors.New("mcp transport not connected")
	}
	_, writeErr := t.stdin.Write(append(body, '\n'))
	t.mu.Unlock()
	if writeErr != nil {
		cleanup()
		return nil, fmt.Errorf("mcp write: %w", writeErr)
	}

	select {
	case <-ctx.Done():
		cleanup()
		return nil, ctx.Err()
	case res := <-pending.resolve:
		cleanup()
		return res, nil
	case err := <-pending.reject:
		cleanup()
		return nil, err
	}
}

// notify sends a one-way notification (fire and forget).
func (t *stdioTransport) notify(method string, params any) error {
	body, err := json.Marshal(jsonRpcNotification{JSONRPC: "2.0", Method: method, Params: params})
	if err != nil {
		return err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return errors.New("mcp transport not connected")
	}
	_, err = t.stdin.Write(append(body, '\n'))
	return err
}

// close terminates the subprocess and rejects pending requests.
func (t *stdioTransport) close() {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return
	}
	t.closed = true
	_ = t.stdin.Close()
	t.mu.Unlock()

	t.pendingMu.Lock()
	for _, p := range t.pending {
		p.reject <- errors.New("mcp transport closed")
	}
	t.pending = make(map[int64]*pendingRequest)
	t.pendingMu.Unlock()

	if t.cmd.Process != nil {
		_ = t.cmd.Process.Kill()
	}
	if t.onClose != nil {
		t.onClose()
	}
}

// readLoop reads newline-delimited JSON frames from the subprocess stdout.
func (t *stdioTransport) readLoop(stdout io.Reader) {
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		t.handleFrame([]byte(line))
	}
	t.close()
}

func (t *stdioTransport) handleFrame(frame []byte) {
	// Distinguish response (has id) from notification (has method, no id).
	var envelope struct {
		ID     any             `json:"id"`
		Method string          `json:"method"`
		Result json.RawMessage `json:"result"`
		Error  *jsonRpcError   `json:"error"`
	}
	if err := json.Unmarshal(frame, &envelope); err != nil {
		return // skip malformed frames
	}

	if envelope.ID != nil {
		// Response to one of our requests.
		var id int64
		switch v := envelope.ID.(type) {
		case float64:
			id = int64(v)
		default:
			return
		}
		t.pendingMu.Lock()
		p := t.pending[id]
		if p != nil {
			delete(t.pending, id)
		}
		t.pendingMu.Unlock()
		if p == nil {
			return
		}
		if envelope.Error != nil {
			p.reject <- envelope.Error
		} else {
			p.resolve <- envelope.Result
		}
		return
	}

	if envelope.Method != "" && t.onNotification != nil {
		var params json.RawMessage
		// params may be omitted; re-extract leniently
		_ = json.Unmarshal(frame, &struct {
			Params json.RawMessage `json:"params"`
		}{Params: params})
		// Unmarshal params properly:
		var withParams struct {
			Params json.RawMessage `json:"params"`
		}
		_ = json.Unmarshal(frame, &withParams)
		t.onNotification(envelope.Method, withParams.Params)
	}
}

// mergeEnv overlays env over the current process environment.
func mergeEnv(extra map[string]string) []string {
	env := os.Environ()
	for k, v := range extra {
		env = append(env, k+"="+v)
	}
	return env
}
