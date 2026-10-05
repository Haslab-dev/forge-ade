// Package browseruse drives a real Chrome/Chromium browser over the Chrome
// DevTools Protocol so the internal agent can navigate, inspect, and interact
// with web pages (forge-ade's native equivalent of ZCode's browser-use plugin,
// which rides on its node-repl host + Electron broker — not portable to a Go
// host, so the same capability is implemented directly against CDP).
package browseruse

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// cdpConn is a minimal DevTools Protocol client: JSON-RPC over the browser
// websocket with flatten-mode target sessions.
type cdpConn struct {
	ctx    context.Context
	cancel context.CancelFunc
	ws     *websocket.Conn

	mu      sync.Mutex
	nextID  int64
	pending map[int64]chan cdpResp

	evtMu    sync.Mutex
	handlers map[string][]func(params json.RawMessage) // keyed "method" and "method\x00sessionId"
}

type cdpResp struct {
	ID     int64           `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type cdpEvent struct {
	Method    string          `json:"method"`
	SessionID string          `json:"sessionId,omitempty"`
	Params    json.RawMessage `json:"params,omitempty"`
}

func newCDPConn(ctx context.Context, wsURL string) (*cdpConn, error) {
	cctx, cancel := context.WithCancel(ctx)
	ws, _, err := websocket.Dial(cctx, wsURL, &websocket.DialOptions{
		HTTPClient: shortTimeoutHTTP(),
	})
	if err != nil {
		cancel()
		return nil, fmt.Errorf("connect to browser devtools: %w", err)
	}
	c := &cdpConn{
		ctx:      cctx,
		cancel:   cancel,
		ws:       ws,
		pending:  make(map[int64]chan cdpResp),
		handlers: make(map[string][]func(json.RawMessage)),
	}
	go c.readLoop()
	return c, nil
}

func (c *cdpConn) readLoop() {
	defer c.cancel()
	for {
		_, data, err := c.ws.Read(c.ctx)
		if err != nil {
			c.mu.Lock()
			for _, ch := range c.pending {
				close(ch)
			}
			c.pending = make(map[int64]chan cdpResp)
			c.mu.Unlock()
			return
		}
		var base struct {
			ID        int64  `json:"id"`
			Method    string `json:"method"`
			SessionID string `json:"sessionId"`
		}
		if err := json.Unmarshal(data, &base); err != nil {
			continue
		}
		if base.ID != 0 {
			var resp cdpResp
			if json.Unmarshal(data, &resp) == nil {
				c.mu.Lock()
				ch, ok := c.pending[base.ID]
				delete(c.pending, base.ID)
				c.mu.Unlock()
				if ok {
					select {
					case ch <- resp:
					default:
					}
				}
			}
			continue
		}
		if base.Method != "" {
			var ev cdpEvent
			if json.Unmarshal(data, &ev) == nil {
				c.dispatch(ev)
			}
		}
	}
}

func (c *cdpConn) dispatch(ev cdpEvent) {
	c.evtMu.Lock()
	defer c.evtMu.Unlock()
	// Session-scoped handlers take precedence over domain-wide ones.
	if ev.SessionID != "" {
		for _, h := range c.handlers[ev.Method+"\x00"+ev.SessionID] {
			go h(ev.Params)
		}
	}
	for _, h := range c.handlers[ev.Method] {
		go h(ev.Params)
	}
}

// On subscribes to a CDP method. When sessionID is non-empty the handler only
// fires for that session.
func (c *cdpConn) On(sessionID, method string, fn func(params json.RawMessage)) {
	c.evtMu.Lock()
	defer c.evtMu.Unlock()
	key := method
	if sessionID != "" {
		key = method + "\x00" + sessionID
	}
	c.handlers[key] = append(c.handlers[key], fn)
}

// Send invokes a CDP command without a session (browser-level domain).
func (c *cdpConn) Send(ctx context.Context, method string, params any) (json.RawMessage, error) {
	return c.sendSession(ctx, "", method, params)
}

// SendSession invokes a CDP command scoped to an attached target session.
func (c *cdpConn) SendSession(ctx context.Context, sessionID, method string, params any) (json.RawMessage, error) {
	return c.sendSession(ctx, sessionID, method, params)
}

func (c *cdpConn) sendSession(ctx context.Context, sessionID, method string, params any) (json.RawMessage, error) {
	if ctx == nil {
		ctx = c.ctx
	}
	c.mu.Lock()
	c.nextID++
	id := c.nextID
	ch := make(chan cdpResp, 1)
	c.pending[id] = ch
	c.mu.Unlock()

	msg := map[string]any{"id": id, "method": method}
	if params != nil {
		msg["params"] = params
	}
	if sessionID != "" {
		msg["sessionId"] = sessionID
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}

	writeCtx, cancelWrite := context.WithTimeout(c.ctx, 10*time.Second)
	defer cancelWrite()
	if err := c.ws.Write(writeCtx, websocket.MessageText, data); err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, fmt.Errorf("cdp write %s: %w", method, err)
	}

	// Commands get a bounded timeout so a wedged tab can't stall the agent.
	cmdCtx := ctx
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		cmdCtx, cancel = context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
	}
	select {
	case resp, ok := <-ch:
		if !ok {
			return nil, fmt.Errorf("cdp connection closed (%s)", method)
		}
		if resp.Error != nil {
			return nil, fmt.Errorf("cdp %s error %d: %s", method, resp.Error.Code, resp.Error.Message)
		}
		return resp.Result, nil
	case <-cmdCtx.Done():
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, fmt.Errorf("cdp %s timed out", method)
	}
}

func (c *cdpConn) Close() {
	c.cancel()
	_ = c.ws.Close(websocket.StatusNormalClosure, "")
}

// Done reports when the connection is closed (browser exited).
func (c *cdpConn) Done() <-chan struct{} {
	return c.ctx.Done()
}
