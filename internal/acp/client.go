package acp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"
)

type NotificationHandler func(method string, params json.RawMessage)
type RequestHandler func(id interface{}, method string, params json.RawMessage)
type StreamEventHandler func(event string, raw []byte)

type Client struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser

	mu          sync.Mutex
	writeMu     sync.Mutex
	requestID   int64
	pendingReqs map[string]chan *RPCResponse
	closed      bool
	lastStderr  []string

	OnNotification NotificationHandler
	OnRequest      RequestHandler
	OnStreamEvent  StreamEventHandler
	OnClose        func(err error)
}

func SpawnClient(command string, args []string, env []string, cwd string) (*Client, error) {
	cmd := exec.Command(command, args...)
	if cwd != "" {
		cmd.Dir = cwd
	}
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("acp client: stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, fmt.Errorf("acp client: stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return nil, fmt.Errorf("acp client: stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		_ = stderr.Close()
		return nil, fmt.Errorf("acp client: start process: %w", err)
	}

	c := &Client{
		cmd:         cmd,
		stdin:       stdin,
		stdout:      stdout,
		stderr:      stderr,
		pendingReqs: make(map[string]chan *RPCResponse),
	}

	go c.readLoop()
	go c.drainStderr()

	return c, nil
}

func (c *Client) readLoop() {
	scanner := bufio.NewScanner(c.stdout)
	// Buffer up to 10MB per line for large diffs or tool responses
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 10*1024*1024)

	var scanErr error
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var generic map[string]interface{}
		if err := json.Unmarshal(line, &generic); err != nil {
			continue
		}

		// Check for Stream-JSON event (e.g. Google Antigravity agy: "event", Claude stream-json: "type")
		if eventVal, hasEvent := generic["event"]; hasEvent {
			eventStr, _ := eventVal.(string)
			if c.OnStreamEvent != nil {
				c.OnStreamEvent(eventStr, line)
			}
			continue
		}
		if typeVal, hasType := generic["type"]; hasType {
			typeStr, _ := typeVal.(string)
			if typeStr == "system" || typeStr == "assistant" || typeStr == "content_block_delta" || typeStr == "result" {
				if c.OnStreamEvent != nil {
					c.OnStreamEvent(typeStr, line)
				}
				continue
			}
		}

		idVal, hasID := generic["id"]
		methodVal, hasMethod := generic["method"]

		if hasID && !hasMethod {
			// This is a response to our request
			var resp RPCResponse
			if err := json.Unmarshal(line, &resp); err == nil {
				c.routeResponse(&resp)
			}
		} else if hasMethod && hasID {
			// This is a server-initiated request (e.g. session/request_permission)
			methodStr, _ := methodVal.(string)
			var req RPCRequest
			if err := json.Unmarshal(line, &req); err == nil {
				if c.OnRequest != nil {
					c.OnRequest(idVal, methodStr, req.Params)
				}
			}
		} else if hasMethod && !hasID {
			// This is a notification (e.g. session/update)
			methodStr, _ := methodVal.(string)
			var notif RPCRequest
			if err := json.Unmarshal(line, &notif); err == nil {
				if c.OnNotification != nil {
					c.OnNotification(methodStr, notif.Params)
				}
			}
		}
	}

	scanErr = scanner.Err()
	c.Close()

	if c.OnClose != nil {
		c.OnClose(scanErr)
	}
}

func (c *Client) drainStderr() {
	scanner := bufio.NewScanner(c.stderr)
	for scanner.Scan() {
		text := scanner.Text()
		c.mu.Lock()
		c.lastStderr = append(c.lastStderr, text)
		if len(c.lastStderr) > 50 {
			c.lastStderr = c.lastStderr[len(c.lastStderr)-50:]
		}
		c.mu.Unlock()
	}
}

func (c *Client) LastStderr() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	res := make([]string, len(c.lastStderr))
	copy(res, c.lastStderr)
	return res
}

func (c *Client) WriteRawLine(data []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	if c.closed {
		return fmt.Errorf("acp client is closed")
	}

	if _, err := c.stdin.Write(data); err != nil {
		return fmt.Errorf("acp write line: %w", err)
	}
	if len(data) == 0 || data[len(data)-1] != '\n' {
		if _, err := c.stdin.Write([]byte("\n")); err != nil {
			return fmt.Errorf("acp write newline: %w", err)
		}
	}
	return nil
}

func (c *Client) routeResponse(resp *RPCResponse) {
	key := fmt.Sprintf("%v", resp.ID)
	c.mu.Lock()
	ch, ok := c.pendingReqs[key]
	if ok {
		delete(c.pendingReqs, key)
	}
	c.mu.Unlock()

	if ok && ch != nil {
		ch <- resp
	}
}

func (c *Client) nextID() int64 {
	return atomic.AddInt64(&c.requestID, 1)
}

func (c *Client) SendRequest(ctx context.Context, method string, params interface{}, result interface{}) error {
	id := c.nextID()
	key := fmt.Sprintf("%v", id)

	ch := make(chan *RPCResponse, 1)
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return fmt.Errorf("acp client is closed")
	}
	c.pendingReqs[key] = ch
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		delete(c.pendingReqs, key)
		c.mu.Unlock()
	}()

	var paramsRaw json.RawMessage
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			return fmt.Errorf("acp marshal params: %w", err)
		}
		paramsRaw = b
	}

	req := RPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  paramsRaw,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("acp marshal request: %w", err)
	}

	c.writeMu.Lock()
	_, err = c.stdin.Write(append(data, '\n'))
	c.writeMu.Unlock()
	if err != nil {
		return fmt.Errorf("acp write request: %w", err)
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case resp := <-ch:
		if resp.Error != nil {
			return fmt.Errorf("acp error %d: %s", resp.Error.Code, resp.Error.Message)
		}
		if result != nil && len(resp.Result) > 0 {
			if err := json.Unmarshal(resp.Result, result); err != nil {
				return fmt.Errorf("acp unmarshal result: %w", err)
			}
		}
		return nil
	}
}

func (c *Client) SendNotification(method string, params interface{}) error {
	var paramsRaw json.RawMessage
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			return fmt.Errorf("acp marshal notification params: %w", err)
		}
		paramsRaw = b
	}

	req := RPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  paramsRaw,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("acp marshal notification: %w", err)
	}

	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if c.closed {
		return fmt.Errorf("acp client is closed")
	}

	_, err = c.stdin.Write(append(data, '\n'))
	return err
}

func (c *Client) SendResponse(id interface{}, result interface{}, rpcErr *RPCError) error {
	var resultRaw json.RawMessage
	if result != nil {
		b, err := json.Marshal(result)
		if err != nil {
			return fmt.Errorf("acp marshal response result: %w", err)
		}
		resultRaw = b
	}

	resp := RPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  resultRaw,
		Error:   rpcErr,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("acp marshal response: %w", err)
	}

	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if c.closed {
		return fmt.Errorf("acp client is closed")
	}

	_, err = c.stdin.Write(append(data, '\n'))
	return err
}

func (c *Client) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true

	// Fail all pending requests
	for _, ch := range c.pendingReqs {
		ch <- &RPCResponse{
			Error: &RPCError{Code: -32000, Message: "client connection closed"},
		}
	}
	c.pendingReqs = make(map[string]chan *RPCResponse)
	c.mu.Unlock()

	_ = c.stdin.Close()
	_ = c.stdout.Close()
	_ = c.stderr.Close()

	if c.cmd != nil && c.cmd.Process != nil {
		// Attempt graceful interrupt first, then kill after 1 second
		done := make(chan error, 1)
		go func() {
			done <- c.cmd.Wait()
		}()

		select {
		case <-done:
		case <-time.After(1 * time.Second):
			_ = c.cmd.Process.Kill()
		}
	}

	return nil
}
