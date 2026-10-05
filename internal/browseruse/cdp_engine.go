package browseruse

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
)

// cdpEngine drives a headless Chromium over the DevTools Protocol and streams
// a screencast to the viewer — the fallback backend when the in-app WKWebView
// engine is unavailable (non-macOS, tests, or FORGE_ADE_BROWSER_ENGINE=cdp).
type cdpEngine struct {
	mu       sync.Mutex
	emit     func(event string, data map[string]any)
	dataDir  string
	profile  string
	headless bool

	cmd     *exec.Cmd
	conn    *cdpConn
	tab     *tab
	quit    chan struct{}
	running bool
}

type tab struct {
	targetID string
	id       string // attached session id
}

func newCDPEngine(dataDir string, emit func(event string, data map[string]any)) *cdpEngine {
	return &cdpEngine{
		dataDir:  dataDir,
		profile:  filepath.Join(dataDir, "browser-profile"),
		headless: os.Getenv("FORGE_ADE_BROWSER_HEADED") == "",
		quit:     make(chan struct{}),
		emit:     emit,
	}
}

func (e *cdpEngine) EngineName() string { return "cdp" }
func (e *cdpEngine) HasTabs() bool      { return true }

// ViewerVisible: the cdp viewer shows screencast frames whenever its panel
// is open; the engine can't see the panel, so report true (the frontend
// auto-opens the panel on navigation).
func (e *cdpEngine) ViewerVisible() bool { return true }

func (e *cdpEngine) emitStatus(state, message string) {
	e.mu.Lock()
	emit := e.emit
	url, title := "", ""
	if e.tab != nil {
		e.mu.Unlock()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		url, title = e.pageInfo(ctx)
		e.mu.Lock()
	}
	e.mu.Unlock()
	if emit != nil {
		data := map[string]any{"state": state}
		if message != "" {
			data["message"] = message
		}
		if url != "" {
			data["url"] = url
		}
		if title != "" {
			data["title"] = title
		}
		emit("browser:status", data)
	}
}

// Status reports whether the browser is running plus the current page info.
func (e *cdpEngine) Status() map[string]any {
	e.mu.Lock()
	running := e.running && e.conn != nil && e.tab != nil
	e.mu.Unlock()
	out := map[string]any{"running": running}
	if running {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		url, title := e.pageInfo(ctx)
		out["url"] = url
		out["title"] = title
	}
	return out
}

// ---------------------------------------------------------------------------
// Lifecycle
// ---------------------------------------------------------------------------

var browserCandidates = [][]string{
	{"FORGE_ADE_BROWSER"}, // env override, checked first
	{"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"},
	{"/Applications/Chromium.app/Contents/MacOS/Chromium"},
	{"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge"},
	{"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser"},
	// Arc is deliberately excluded: it ignores --headless=new (opens a visible
	// window) and it's likely the user's personal browser — never hijack it.
	{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "chrome"}, // PATH
	{"/usr/bin/google-chrome", "/usr/bin/chromium"},
}

func findBrowser() (string, error) {
	for _, cand := range browserCandidates {
		if len(cand) == 1 && strings.HasPrefix(cand[0], "FORGE_") {
			if p := os.Getenv(cand[0]); p != "" {
				if _, err := os.Stat(p); err == nil {
					return p, nil
				}
			}
			continue
		}
		for _, p := range cand {
			if strings.Contains(p, "/") {
				if _, err := os.Stat(p); err == nil {
					return p, nil
				}
			} else if lp, err := exec.LookPath(p); err == nil {
				return lp, nil
			}
		}
	}
	return "", fmt.Errorf("no Chromium-based browser found; install Google Chrome (or set FORGE_ADE_BROWSER to a Chromium binary)")
}

// Start launches (or reuses) the browser and attaches to its active tab with
// the screencast running. It is idempotent.
func (e *cdpEngine) Start() error {
	e.mu.Lock()
	if e.running && e.conn != nil && e.tab != nil {
		e.mu.Unlock()
		return nil
	}
	bin, err := findBrowser()
	if err != nil {
		e.mu.Unlock()
		return err
	}
	port, err := freePort()
	if err != nil {
		e.mu.Unlock()
		return fmt.Errorf("find free port: %w", err)
	}
	e.mu.Unlock()
	return e.launch(bin, port)
}

func (e *cdpEngine) launch(bin string, port int) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	_ = os.MkdirAll(e.profile, 0700)
	_ = os.MkdirAll(filepath.Join(e.dataDir, "screenshots"), 0755)

	args := []string{
		"--remote-debugging-port=" + strconv.Itoa(port),
		"--user-data-dir=" + e.profile,
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-background-networking",
		"--disable-features=Translate,AcceptCHFrame",
		"--disable-breakpad",
		"--window-size=1440,900",
		"--disable-session-crashed-bubble",
		"about:blank",
	}
	if e.headless {
		args = append([]string{"--headless=new", "--hide-scrollbars", "--mute-audio"}, args...)
	}

	cmd := exec.Command(bin, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("launch browser: %w", err)
	}
	e.cmd = cmd
	e.running = true

	// Poll the DevTools HTTP endpoint until the browser is ready — works for
	// every Chromium (some, like Arc, log nothing to stderr).
	wsURL, err := waitForDevToolsEndpoint(port, 20*time.Second)
	if err != nil {
		e.running = false
		e.stopProcessLocked()
		return err
	}

	conn, err := newCDPConn(context.Background(), wsURL)
	if err != nil {
		e.running = false
		e.stopProcessLocked()
		return err
	}
	e.conn = conn

	// Attach to the initial about:blank tab.
	res, err := conn.Send(context.Background(), "Target.getTargets", nil)
	if err != nil {
		e.teardownLocked()
		return fmt.Errorf("list targets: %w", err)
	}
	var targets struct {
		TargetInfos []struct {
			TargetID string `json:"targetId"`
			Type     string `json:"type"`
		} `json:"targetInfos"`
	}
	json.Unmarshal(res, &targets)
	var targetID string
	for _, t := range targets.TargetInfos {
		if t.Type == "page" {
			targetID = t.TargetID
			break
		}
	}
	if targetID == "" {
		e.teardownLocked()
		return fmt.Errorf("no page target found in browser")
	}
	if err := e.attachLocked(targetID); err != nil {
		e.teardownLocked()
		return err
	}

	go e.watchExit()
	e.emitStatus("running", "")
	return nil
}

func (e *cdpEngine) attachLocked(targetID string) error {
	res, err := e.conn.Send(context.Background(), "Target.attachToTarget", map[string]any{
		"targetId": targetID,
		"flatten":  true,
	})
	if err != nil {
		return fmt.Errorf("attach to tab: %w", err)
	}
	var attached struct {
		SessionID string `json:"sessionId"`
	}
	json.Unmarshal(res, &attached)
	if attached.SessionID == "" {
		return fmt.Errorf("attach returned no sessionId")
	}
	t := &tab{targetID: targetID, id: attached.SessionID}
	e.tab = t
	// Enable the Page domain so navigation/dialog events fire, and inject the
	// same document-start load counter as the in-app engine (see
	// loadCounterScript) — the click flow uses it to detect reload storms.
	_, _ = e.conn.SendSession(context.Background(), t.id, "Page.enable", nil)
	_, _ = e.conn.SendSession(context.Background(), t.id, "Page.addScriptToEvaluateOnNewDocument", map[string]any{
		"source": loadCounterScript,
	})
	e.wirePageHandlers(t)
	return e.startScreencast(t)
}

func (e *cdpEngine) wirePageHandlers(t *tab) {
	ctx := context.Background()
	// Keep the viewer's URL bar in sync.
	e.conn.On(t.id, "Page.frameNavigated", func(params json.RawMessage) {
		var p struct {
			Frame struct {
				ParentID string `json:"parentId"`
				URL      string `json:"url"`
			} `json:"frame"`
		}
		if json.Unmarshal(params, &p) == nil && p.Frame.ParentID == "" {
			e.emitStatus("navigated", "")
		}
	})
	// Auto-dismiss JS dialogs so the page can never wedge the agent; the
	// status event tells the model what happened.
	e.conn.On(t.id, "Page.javascriptDialogOpening", func(params json.RawMessage) {
		var p struct {
			Message string `json:"message"`
			Type    string `json:"type"`
		}
		_ = json.Unmarshal(params, &p)
		_, _ = e.conn.SendSession(ctx, t.id, "Page.handleJavaScriptDialog", map[string]any{"accept": false})
		e.emitStatus("dialog", fmt.Sprintf("auto-dismissed %s dialog: %s", p.Type, p.Message))
	})
	// Browser died — drop state so the next tool call relaunches cleanly.
	go func() {
		select {
		case <-e.conn.Done():
			e.mu.Lock()
			was := e.running
			e.conn = nil
			e.tab = nil
			e.running = false
			e.mu.Unlock()
			if was {
				e.emitStatus("stopped", "browser exited")
			}
		case <-e.quit:
		}
	}()
}

func (e *cdpEngine) startScreencast(t *tab) error {
	_, err := e.conn.SendSession(context.Background(), t.id, "Page.startScreencast", map[string]any{
		"format":        "jpeg",
		"quality":       55,
		"maxWidth":      1600,
		"maxHeight":     1000,
		"everyNthFrame": 1,
	})
	if err != nil {
		return fmt.Errorf("start screencast: %w", err)
	}
	e.conn.On(t.id, "Page.screencastFrame", func(params json.RawMessage) {
		var p struct {
			Data      string `json:"data"`
			SessionID int64  `json:"sessionId"`
			Metadata  struct {
				PageScaleFactor float64 `json:"pageScaleFactor"`
				DeviceWidth     int     `json:"deviceWidth"`
				DeviceHeight    int     `json:"deviceHeight"`
				ScrollOffsetX   float64 `json:"scrollOffsetX"`
				ScrollOffsetY   float64 `json:"scrollOffsetY"`
			} `json:"metadata"`
		}
		if json.Unmarshal(params, &p) != nil || p.Data == "" {
			return
		}
		e.mu.Lock()
		emit := e.emit
		e.mu.Unlock()
		if emit != nil {
			emit("browser:frame", map[string]any{
				"data":   p.Data,
				"format": "jpeg",
				"width":  p.Metadata.DeviceWidth,
				"height": p.Metadata.DeviceHeight,
			})
		}
		// Ack so Chrome sends the next frame (sessionId here is the frame
		// counter, not the CDP session).
		_, _ = e.conn.SendSession(context.Background(), t.id, "Page.screencastFrameAck", map[string]any{
			"sessionId": p.SessionID,
		})
	})
	return nil
}

func (e *cdpEngine) watchExit() {
	e.mu.Lock()
	cmd := e.cmd
	e.mu.Unlock()
	if cmd == nil {
		return
	}
	_ = cmd.Wait()
}

func (e *cdpEngine) teardownLocked() {
	if e.conn != nil {
		e.conn.Close()
		e.conn = nil
	}
	e.tab = nil
	e.stopProcessLocked()
	e.running = false
}

func (e *cdpEngine) stopProcessLocked() {
	if e.cmd != nil && e.cmd.Process != nil {
		pid := e.cmd.Process.Pid
		_ = syscall.Kill(-pid, syscall.SIGTERM)
		go func() {
			time.Sleep(3 * time.Second)
			_ = syscall.Kill(-pid, syscall.SIGKILL)
		}()
	}
	e.cmd = nil
}

// Close shuts the browser down (browser_close tool / app teardown).
func (e *cdpEngine) Close() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.teardownLocked()
	select {
	case <-e.quit:
	default:
		close(e.quit)
	}
	e.emitStatus("stopped", "")
}

// ensureSession returns the active page session, launching the browser on
// first use and re-attaching after a crash.
func (e *cdpEngine) ensureSession() (string, error) {
	if err := e.Start(); err != nil {
		return "", err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.conn == nil || e.tab == nil {
		return "", fmt.Errorf("browser is not running")
	}
	return e.tab.id, nil
}

// ---------------------------------------------------------------------------
// Page primitives
// ---------------------------------------------------------------------------

func (e *cdpEngine) eval(ctx context.Context, expression string) (json.RawMessage, error) {
	sess, err := e.ensureSession()
	if err != nil {
		return nil, err
	}
	return e.evalSession(ctx, sess, expression)
}

func (e *cdpEngine) evalSession(ctx context.Context, sess, expression string) (json.RawMessage, error) {
	res, err := e.conn.SendSession(ctx, sess, "Runtime.evaluate", map[string]any{
		"expression":    expression,
		"returnByValue": true,
		"awaitPromise":  true,
	})
	if err != nil {
		return nil, err
	}
	var out struct {
		Result struct {
			Type  string          `json:"type"`
			Value json.RawMessage `json:"value"`
			Descr string          `json:"description"`
		} `json:"result"`
		ExceptionDetails *struct {
			Text string `json:"text"`
		} `json:"exceptionDetails,omitempty"`
	}
	if err := json.Unmarshal(res, &out); err != nil {
		return nil, err
	}
	if out.ExceptionDetails != nil {
		return nil, fmt.Errorf("page evaluation failed: %s", out.ExceptionDetails.Text)
	}
	if out.Result.Type == "string" && len(out.Result.Value) >= 2 {
		var s string
		if json.Unmarshal(out.Result.Value, &s) == nil {
			return json.RawMessage(strconv.Quote(s)), nil
		}
	}
	return out.Result.Value, nil
}

// evalString evaluates an expression that returns a string value.
func (e *cdpEngine) evalString(ctx context.Context, expression string) (string, error) {
	raw, err := e.eval(ctx, expression)
	if err != nil {
		return "", err
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s, nil
	}
	return "", nil
}

func (e *cdpEngine) pageInfo(ctx context.Context) (string, string) {
	e.mu.Lock()
	sess := ""
	if e.tab != nil {
		sess = e.tab.id
	}
	conn := e.conn
	e.mu.Unlock()
	if conn == nil || sess == "" {
		return "", ""
	}
	cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	res, err := conn.SendSession(cctx, sess, "Runtime.evaluate", map[string]any{
		"expression":    "JSON.stringify({url: location.href, title: document.title})",
		"returnByValue": true,
	})
	if err != nil {
		return "", ""
	}
	var out struct {
		Result struct {
			Value struct {
				URL   string `json:"url"`
				Title string `json:"title"`
			} `json:"value"`
		} `json:"result"`
	}
	if json.Unmarshal(res, &out) == nil {
		return out.Result.Value.URL, out.Result.Value.Title
	}
	return "", ""
}

// Navigate loads a URL in the active tab and waits (bounded) for the page to
// settle before snapshotting.
func (e *cdpEngine) Navigate(ctx context.Context, rawURL string) (map[string]any, error) {
	sess, err := e.ensureSession()
	if err != nil {
		return nil, err
	}
	if !strings.Contains(rawURL, "://") {
		rawURL = "https://" + rawURL
	}
	nctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if _, err := e.conn.SendSession(nctx, sess, "Page.navigate", map[string]any{"url": rawURL}); err != nil {
		return nil, fmt.Errorf("navigate: %w", err)
	}
	e.waitForSettle(nctx, sess, 10*time.Second)
	return e.describePage(nctx, sess)
}

// waitForSettle polls document.readyState + a short idle delay.
func (e *cdpEngine) waitForSettle(ctx context.Context, sess string, max time.Duration) {
	deadline := time.Now().Add(max)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return
		default:
		}
		if ready, err := e.evalString(ctx, "document.readyState"); err == nil && ready == "complete" {
			time.Sleep(400 * time.Millisecond) // let late rendering settle
			return
		}
		time.Sleep(150 * time.Millisecond)
	}
}

func (e *cdpEngine) describePage(ctx context.Context, sess string) (map[string]any, error) {
	url, title := e.pageInfo(ctx)
	out := map[string]any{"url": url, "title": title}
	if raw, err := e.evalSession(ctx, sess, pageLoadCountExpr); err == nil {
		out["loads"] = parseLoadCount(raw)
	}
	snap, err := e.SnapshotSession(ctx, sess)
	if err != nil {
		snap = ""
	}
	if snap != "" {
		out["snapshot"] = snap
	}
	return out, nil
}

// Snapshot outlines the page with stable element refs (see snapshot.go).
func (e *cdpEngine) Snapshot(ctx context.Context) (string, error) {
	sess, err := e.ensureSession()
	if err != nil {
		return "", err
	}
	return e.SnapshotSession(ctx, sess)
}

// Screenshot captures a PNG of the viewport, saves it under
// <dataDir>/screenshots, and returns the path.
func (e *cdpEngine) Screenshot(ctx context.Context) (string, error) {
	sess, err := e.ensureSession()
	if err != nil {
		return "", err
	}
	res, err := e.conn.SendSession(ctx, sess, "Page.captureScreenshot", map[string]any{
		"format": "png",
	})
	if err != nil {
		return "", fmt.Errorf("screenshot: %w", err)
	}
	var out struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal(res, &out); err != nil || out.Data == "" {
		return "", fmt.Errorf("screenshot returned no image")
	}
	data, err := base64.StdEncoding.DecodeString(out.Data)
	if err != nil {
		return "", err
	}
	path := filepath.Join(e.dataDir, "screenshots", fmt.Sprintf("browser-%s.png", uuid.New().String()[:8]))
	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", err
	}
	return path, nil
}

// ClickAt dispatches a real mouse click at viewport coordinates (used by the
// viewer's frame canvas in cdp mode).
func (e *cdpEngine) ClickAt(ctx context.Context, x, y float64) (map[string]any, error) {
	sess, err := e.ensureSession()
	if err != nil {
		return nil, err
	}
	if err := e.dispatchClick(ctx, sess, x, y); err != nil {
		return nil, err
	}
	e.waitForSettle(ctx, sess, 3*time.Second)
	return e.describePage(ctx, sess)
}

// WheelAt scrolls at the viewport center by deltaY pixels (viewer wheel).
func (e *cdpEngine) WheelAt(ctx context.Context, deltaY float64) (map[string]any, error) {
	sess, err := e.ensureSession()
	if err != nil {
		return nil, err
	}
	if _, err := e.conn.SendSession(ctx, sess, "Input.dispatchMouseEvent", map[string]any{
		"type": "mouseWheel", "x": 720, "y": 450,
		"deltaX": 0, "deltaY": deltaY,
	}); err != nil {
		return nil, fmt.Errorf("wheel: %w", err)
	}
	return map[string]any{"scrolled": true}, nil
}

// SetRect is a no-op for the cdp engine (the viewer renders screencast frames).
func (e *cdpEngine) SetRect(x, y, w, h float64, visible bool) {}

func (e *cdpEngine) setEmitter(fn func(event string, data map[string]any)) { e.emit = fn }

// PickElement arms the page-side picker and waits for the pick (the viewer
// relays the user's click on the frame canvas via ClickAt).
func (e *cdpEngine) PickElement(ctx context.Context) (map[string]any, error) {
	sess, err := e.ensureSession()
	if err != nil {
		return nil, err
	}
	if _, err := e.evalSession(ctx, sess, pickElementScript); err != nil {
		return nil, err
	}
	deadline := time.Now().Add(35 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		raw, err := e.evalSession(ctx, sess, "window.__forgePickResult ? window.__forgePickResult : ''")
		if err == nil {
			var s string
			if json.Unmarshal(raw, &s) == nil && s != "" {
				return map[string]any{"picked": json.RawMessage(s)}, nil
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return nil, fmt.Errorf("element picking timed out (30s)")
}

// shortTimeoutHTTP is the dialer used for the DevTools websocket handshake.
func shortTimeoutHTTP() *http.Client {
	return &http.Client{Timeout: 10 * time.Second}
}

// freePort reserves an ephemeral port for the browser's DevTools listener.
func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// waitForDevToolsEndpoint polls /json/version until the browser exposes its
// browser-scoped websocket URL.
func waitForDevToolsEndpoint(port int, max time.Duration) (string, error) {
	deadline := time.Now().Add(max)
	endpoint := fmt.Sprintf("http://127.0.0.1:%d/json/version", port)
	client := &http.Client{Timeout: 1200 * time.Millisecond}
	for time.Now().Before(deadline) {
		if resp, err := client.Get(endpoint); err == nil {
			data, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			var v struct {
				WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
			}
			if json.Unmarshal(data, &v) == nil && v.WebSocketDebuggerURL != "" {
				return v.WebSocketDebuggerURL, nil
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	return "", fmt.Errorf("timed out waiting for the browser DevTools endpoint on port %d", port)
}
