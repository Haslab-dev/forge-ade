package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int64  `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int64          `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// Client is a single JSON-RPC 2.0 stdio connection to a language server.
type Client struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	writeMu sync.Mutex

	pendingMu sync.Mutex
	pending   map[int64]chan rpcMessage
	nextID    atomic.Int64

	onNotification func(method string, params json.RawMessage)
	closed         atomic.Bool
	done           chan struct{}
}

// StartProcessClient spawns cmd and performs the LSP initialize handshake.
func StartProcessClient(ctx context.Context, command string, args []string, rootDir string, onNotify func(method string, params json.RawMessage)) (*Client, error) {
	cmd := exec.Command(command, args...)
	if rootDir != "" {
		cmd.Dir = rootDir
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("lsp stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("lsp stdout: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("lsp start %s: %w", command, err)
	}

	c := newStreamClient(cmd, stdin, stdout, onNotify)

	initCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	if err := c.initialize(initCtx, rootDir); err != nil {
		c.Shutdown()
		return nil, err
	}
	return c, nil
}

// NewStreamClient creates a Client over arbitrary streams (used by tests).
func NewStreamClient(stdin io.WriteCloser, stdout io.Reader, onNotify func(method string, params json.RawMessage)) *Client {
	return newStreamClient(nil, stdin, stdout, onNotify)
}

func newStreamClient(cmd *exec.Cmd, stdin io.WriteCloser, stdout io.Reader, onNotify func(method string, params json.RawMessage)) *Client {
	c := &Client{
		cmd:            cmd,
		stdin:          stdin,
		pending:        make(map[int64]chan rpcMessage),
		onNotification: onNotify,
		done:           make(chan struct{}),
	}
	go c.readLoop(stdout)
	return c
}

func (c *Client) initialize(ctx context.Context, rootDir string) error {
	rootURI := PathToURI(rootDir)
	params := map[string]any{
		"processId": nil,
		"rootUri":   rootURI,
		"capabilities": map[string]any{
			"textDocument": map[string]any{
				"synchronization": map[string]any{
					"didSave": true,
				},
				"completion": map[string]any{
					"completionItem": map[string]any{
						"snippetSupport": false,
					},
				},
				"hover": map[string]any{
					"contentFormat": []string{"markdown", "plaintext"},
				},
				"publishDiagnostics": map[string]any{},
			},
		},
	}
	if err := c.Request(ctx, "initialize", params, nil); err != nil {
		return fmt.Errorf("lsp initialize: %w", err)
	}
	return c.Notify("initialized", struct{}{})
}

// Request sends a JSON-RPC request and waits for its response or ctx cancellation.
func (c *Client) Request(ctx context.Context, method string, params any, out any) error {
	if c.closed.Load() {
		return fmt.Errorf("lsp client closed")
	}
	id := c.nextID.Add(1)
	ch := make(chan rpcMessage, 1)
	c.pendingMu.Lock()
	c.pending[id] = ch
	c.pendingMu.Unlock()

	defer func() {
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
	}()

	if err := c.send(rpcRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}); err != nil {
		return err
	}

	select {
	case msg, ok := <-ch:
		if !ok {
			return fmt.Errorf("lsp connection closed")
		}
		if msg.Error != nil {
			return fmt.Errorf("lsp error %d: %s", msg.Error.Code, msg.Error.Message)
		}
		if out != nil && len(msg.Result) > 0 && string(msg.Result) != "null" {
			if err := json.Unmarshal(msg.Result, out); err != nil {
				return fmt.Errorf("lsp decode %s: %w", method, err)
			}
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-c.done:
		return fmt.Errorf("lsp process exited")
	}
}

// Notify sends a fire-and-forget JSON-RPC notification.
func (c *Client) Notify(method string, params any) error {
	if c.closed.Load() {
		return fmt.Errorf("lsp client closed")
	}
	return c.send(rpcRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	})
}

func (c *Client) send(req rpcRequest) error {
	body, err := json.Marshal(req)
	if err != nil {
		return err
	}
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))

	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if c.closed.Load() {
		return fmt.Errorf("lsp client closed")
	}
	if _, err := io.WriteString(c.stdin, header); err != nil {
		return err
	}
	_, err = c.stdin.Write(body)
	return err
}

func (c *Client) readLoop(r io.Reader) {
	defer func() {
		c.closed.Store(true)
		c.pendingMu.Lock()
		for id, ch := range c.pending {
			close(ch)
			delete(c.pending, id)
		}
		c.pendingMu.Unlock()
		close(c.done)
	}()

	br := bufio.NewReaderSize(r, 64*1024)
	for {
		contentLen := -1
		for {
			line, err := br.ReadString('\n')
			if err != nil {
				return
			}
			line = strings.TrimRight(line, "\r\n")
			if line == "" {
				break
			}
			if idx := strings.IndexByte(line, ':'); idx > 0 {
				k := strings.TrimSpace(line[:idx])
				v := strings.TrimSpace(line[idx+1:])
				if strings.EqualFold(k, "Content-Length") {
					if n, err := strconv.Atoi(v); err == nil {
						contentLen = n
					}
				}
			}
		}
		if contentLen <= 0 {
			continue
		}
		payload := make([]byte, contentLen)
		if _, err := io.ReadFull(br, payload); err != nil {
			return
		}
		var msg rpcMessage
		if err := json.Unmarshal(payload, &msg); err != nil {
			continue
		}
		if msg.ID != nil && msg.Method == "" {
			c.pendingMu.Lock()
			ch := c.pending[*msg.ID]
			c.pendingMu.Unlock()
			if ch != nil {
				ch <- msg
			}
			continue
		}
		if msg.Method != "" {
			// Server-to-client request: reply with null result so servers don't stall.
			if msg.ID != nil {
				_ = c.sendRawResponse(*msg.ID)
			} else if c.onNotification != nil {
				c.onNotification(msg.Method, msg.Params)
			}
		}
	}
}

func (c *Client) sendRawResponse(id int64) error {
	body := fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"result":null}`, id)
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if c.closed.Load() {
		return nil
	}
	if _, err := io.WriteString(c.stdin, header); err != nil {
		return err
	}
	_, err := io.WriteString(c.stdin, body)
	return err
}

// Alive reports whether the client connection is still open.
func (c *Client) Alive() bool {
	return c != nil && !c.closed.Load()
}

// Shutdown gracefully stops the language server and closes its pipes.
func (c *Client) Shutdown() {
	if c == nil || c.closed.Swap(true) {
		return
	}
	// Best-effort LSP shutdown/exit before closing stdin.
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	_ = c.send(rpcRequest{JSONRPC: "2.0", ID: c.nextID.Add(1), Method: "shutdown"})
	_ = ctx
	cancel()
	_ = c.send(rpcRequest{JSONRPC: "2.0", Method: "exit"})
	if c.stdin != nil {
		_ = c.stdin.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		select {
		case <-c.done:
		case <-time.After(300 * time.Millisecond):
			_ = c.cmd.Process.Kill()
		}
		_ = c.cmd.Wait()
	}
}
