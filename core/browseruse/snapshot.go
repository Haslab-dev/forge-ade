package browseruse

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Interaction built on the DevTools Protocol. The snapshot/resolveRef scripts
// are shared with the in-app engine; input here uses trusted CDP events.

// SnapshotSession snapshots the active tab.
func (e *cdpEngine) SnapshotSession(ctx context.Context, sess string) (string, error) {
	raw, err := e.evalSession(ctx, sess, snapshotScript)
	if err != nil {
		return "", fmt.Errorf("snapshot: %w", err)
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s, nil
	}
	return string(raw), nil
}

func (e *cdpEngine) resolveRef(ctx context.Context, sess, ref string) (x, y float64, err error) {
	raw, err := e.evalSession(ctx, sess, fmt.Sprintf(resolveRefScript, ref))
	if err != nil {
		return 0, 0, err
	}
	var r refRect
	if json.Unmarshal(raw, &r) != nil || !r.Found {
		return 0, 0, fmt.Errorf("ref %s not found — the page changed since your last snapshot; take a fresh browser_snapshot", ref)
	}
	return r.X, r.Y, nil
}

func (e *cdpEngine) dispatchClick(ctx context.Context, sess string, x, y float64) error {
	for _, t := range []struct {
		typ    string
		button string
	}{
		{"mousePressed", "left"},
		{"mouseReleased", "left"},
	} {
		if _, err := e.conn.SendSession(ctx, sess, "Input.dispatchMouseEvent", map[string]any{
			"type": t.typ, "x": x, "y": y, "button": t.button,
			"clickCount": 1, "buttons": 1,
		}); err != nil {
			return fmt.Errorf("click: %w", err)
		}
	}
	return nil
}

// evalStringSession evaluates an expression that returns a string value.
func (e *cdpEngine) evalStringSession(ctx context.Context, sess, expression string) (string, error) {
	raw, err := e.evalSession(ctx, sess, expression)
	if err != nil {
		return "", err
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s, nil
	}
	return string(raw), nil
}

// loadCount reads the per-document load counter injected at document start
// (see loadCounterScript).
func (e *cdpEngine) loadCount(ctx context.Context, sess string) (int, error) {
	raw, err := e.evalSession(ctx, sess, pageLoadCountExpr)
	if err != nil {
		return 0, err
	}
	return parseLoadCount(raw), nil
}

// waitForStablePage samples the load counter to detect a reload storm — the
// page under test re-creating documents faster than actions can land (dev
// server restarting, HMR websocket death loop). Returns the load count once
// two samples 500ms apart agree; stormed=true if still climbing at deadline.
func (e *cdpEngine) waitForStablePage(ctx context.Context, sess string, max time.Duration) (loads int, stormed bool) {
	first, err := e.loadCount(ctx, sess)
	if err != nil {
		return 0, false // counter unreadable — don't block the action
	}
	deadline := time.Now().Add(max)
	for {
		select {
		case <-ctx.Done():
			return first, false
		case <-time.After(500 * time.Millisecond):
		}
		second, err := e.loadCount(ctx, sess)
		if err != nil {
			return first, false
		}
		if second == first {
			return second, false
		}
		first = second
		if time.Now().After(deadline) {
			return second, true
		}
	}
}

// Click clicks the element for a snapshot ref with a real mouse event. If the
// page reloaded right after the click (the click landed on a document that
// was replaced — reload storms do this constantly), it waits for quiet and
// re-lands the click ONCE on the fresh document, same URL only.
func (e *cdpEngine) Click(ctx context.Context, ref string) (map[string]any, error) {
	sess, err := e.ensureSession()
	if err != nil {
		return nil, err
	}
	x, y, err := e.resolveRefWithRetry(ctx, sess, ref)
	if err != nil {
		return nil, err
	}
	hrefBefore, _ := e.evalStringSession(ctx, sess, "location.href")
	loadsBefore, _ := e.loadCount(ctx, sess)
	if err := e.dispatchClick(ctx, sess, x, y); err != nil {
		return nil, err
	}

	nctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	e.waitForSettle(nctx, sess, 3*time.Second)
	loads, stormed := e.waitForStablePage(nctx, sess, 8*time.Second)

	retried := false
	if (loads > loadsBefore || stormed) && hrefBefore != "" {
		if href, herr := e.evalStringSession(nctx, sess, "location.href"); herr == nil && href == hrefBefore {
			if x, y, rerr := e.resolveRefWithRetry(nctx, sess, ref); rerr == nil {
				if derr := e.dispatchClick(nctx, sess, x, y); derr == nil {
					retried = true
					e.waitForSettle(nctx, sess, 2*time.Second)
					loads, stormed = e.waitForStablePage(nctx, sess, 4*time.Second)
				}
			}
		}
	}

	res, err := e.describePage(nctx, sess)
	if err != nil {
		return nil, err
	}
	if stormed {
		res["reload_storm"] = "the page kept reloading for the whole stability window (load " + strconv.Itoa(loads) + ") — the app under test is restarting its dev server or hot-reload loop; verify state before further actions"
	}
	if loads > loadsBefore || stormed {
		if raw, err := e.evalStringSession(nctx, sess, reloadTraceExpr); err == nil && strings.HasPrefix(raw, "[") {
			var trace []json.RawMessage
			if json.Unmarshal([]byte(raw), &trace) == nil && len(trace) > 0 {
				res["reload_trace"] = json.RawMessage(raw)
			}
		}
	}
	if retried {
		res["retried_after_reload"] = true
	}
	return res, nil
}

// resolveRefWithRetry tolerates a DOM replace between snapshot and action:
// re-snapshot once (re-tagging refs) and retry the same ref.
func (e *cdpEngine) resolveRefWithRetry(ctx context.Context, sess, ref string) (float64, float64, error) {
	x, y, err := e.resolveRef(ctx, sess, ref)
	if err == nil {
		return x, y, nil
	}
	if _, serr := e.SnapshotSession(ctx, sess); serr == nil {
		if x, y, rerr := e.resolveRef(ctx, sess, ref); rerr == nil {
			return x, y, nil
		}
	}
	return 0, 0, err
}

// Type focuses the element for a ref, inserts text (unicode-safe), and
// optionally presses Enter afterwards.
func (e *cdpEngine) Type(ctx context.Context, ref, text string, submit bool) (map[string]any, error) {
	sess, err := e.ensureSession()
	if err != nil {
		return nil, err
	}
	if _, _, err := e.resolveRefWithRetry(ctx, sess, ref); err != nil {
		return nil, err
	}
	focus := fmt.Sprintf(`(() => { const el = document.querySelector('[data-fref="%s"]'); if (!el) return 'stale'; el.focus(); return 'ok'; })()`, ref)
	state, err := e.evalString(ctx, focus)
	if err != nil || state != "ok" {
		return nil, fmt.Errorf("ref %s not focusable — take a fresh browser_snapshot", ref)
	}
	if _, err := e.conn.SendSession(ctx, sess, "Input.insertText", map[string]any{"text": text}); err != nil {
		return nil, fmt.Errorf("type: %w", err)
	}
	if submit {
		if err := e.pressKey(ctx, sess, "Enter"); err != nil {
			return nil, err
		}
	}
	e.waitForSettle(ctx, sess, 3*time.Second)
	return e.describePage(ctx, sess)
}

func (e *cdpEngine) pressKey(ctx context.Context, sess, name string) error {
	normalizedName := strings.ToLower(strings.TrimSpace(name))
	descriptor, ok := cdpKeyMap[normalizedName]
	if !ok {
		// Printable single character — synthesize text input instead.
		if len([]rune(name)) == 1 {
			if _, err := e.conn.SendSession(ctx, sess, "Input.insertText", map[string]any{"text": name}); err != nil {
				return fmt.Errorf("key %q: %w", name, err)
			}
			return nil
		}
		return fmt.Errorf("unsupported key %q (supported: enter, tab, escape, backspace, delete, arrow*, home, end, page*, or a single character)", name)
	}
	params := map[string]any{
		"type": "rawKeyDown", "code": descriptor.code, "key": descriptor.key,
		"windowsVirtualKeyCode": descriptor.windowsVK, "nativeVirtualKeyCode": descriptor.keyCode,
	}
	if _, err := e.conn.SendSession(ctx, sess, "Input.dispatchKeyEvent", params); err != nil {
		return err
	}
	params["type"] = "keyUp"
	_, err := e.conn.SendSession(ctx, sess, "Input.dispatchKeyEvent", params)
	return err
}

// Key presses a named key (Enter, ArrowDown, Escape, ...).
func (e *cdpEngine) Key(ctx context.Context, name string) (map[string]any, error) {
	sess, err := e.ensureSession()
	if err != nil {
		return nil, err
	}
	if err := e.pressKey(ctx, sess, name); err != nil {
		return nil, err
	}
	e.waitForSettle(ctx, sess, 2*time.Second)
	return e.describePage(ctx, sess)
}

// Scroll wheels the page. direction is up/down; amount is in "notches"
// (~100px each, default 5).
func (e *cdpEngine) Scroll(ctx context.Context, direction string, amount int) (map[string]any, error) {
	sess, err := e.ensureSession()
	if err != nil {
		return nil, err
	}
	if amount <= 0 {
		amount = 5
	}
	deltaY := float64(amount * 100)
	if direction == "up" {
		deltaY = -deltaY
	}
	if _, err := e.conn.SendSession(ctx, sess, "Input.dispatchMouseEvent", map[string]any{
		"type": "mouseWheel", "x": 720, "y": 450,
		"deltaX": 0, "deltaY": deltaY,
	}); err != nil {
		return nil, fmt.Errorf("scroll: %w", err)
	}
	time.Sleep(300 * time.Millisecond)
	return e.describePage(ctx, sess)
}

// Tabs manages browser tabs: list / new / select / close.
func (e *cdpEngine) Tabs(ctx context.Context, action string, index int, url string) (any, error) {
	if err := e.Start(); err != nil {
		return nil, err
	}
	e.mu.Lock()
	conn := e.conn
	e.mu.Unlock()
	if conn == nil {
		return nil, fmt.Errorf("browser is not running")
	}
	ctx2, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	switch action {
	case "", "list":
		res, err := conn.Send(ctx2, "Target.getTargets", nil)
		if err != nil {
			return nil, err
		}
		var targets struct {
			TargetInfos []struct {
				TargetID string `json:"targetId"`
				Title    string `json:"title"`
				URL      string `json:"url"`
			} `json:"targetInfos"`
		}
		json.Unmarshal(res, &targets)
		out := make([]map[string]any, 0, len(targets.TargetInfos))
		e.mu.Lock()
		active := ""
		if e.tab != nil {
			active = e.tab.targetID
		}
		e.mu.Unlock()
		i := 0
		for _, t := range targets.TargetInfos {
			if t.Title == "" && strings.HasPrefix(t.URL, "devtools") {
				continue
			}
			out = append(out, map[string]any{"index": i, "title": t.Title, "url": t.URL, "active": t.TargetID == active})
			i++
		}
		return map[string]any{"tabs": out}, nil

	case "new":
		if url == "" {
			url = "about:blank"
		}
		res, err := conn.Send(ctx2, "Target.createTarget", map[string]any{"url": url})
		if err != nil {
			return nil, fmt.Errorf("open tab: %w", err)
		}
		var created struct {
			TargetID string `json:"targetId"`
		}
		json.Unmarshal(res, &created)
		e.mu.Lock()
		err = e.attachLocked(created.TargetID)
		e.mu.Unlock()
		if err != nil {
			return nil, err
		}
		e.waitForSettle(ctx2, e.activeSession(), 10*time.Second)
		return e.describePage(ctx2, e.activeSession())

	case "select":
		res, err := conn.Send(ctx2, "Target.getTargets", nil)
		if err != nil {
			return nil, err
		}
		var targets struct {
			TargetInfos []struct {
				TargetID string `json:"targetId"`
				Type     string `json:"type"`
			} `json:"targetInfos"`
		}
		json.Unmarshal(res, &targets)
		pages := make([]string, 0)
		for _, t := range targets.TargetInfos {
			if t.Type == "page" {
				pages = append(pages, t.TargetID)
			}
		}
		if index < 0 || index >= len(pages) {
			return nil, fmt.Errorf("tab index %d out of range (%d tabs)", index, len(pages))
		}
		e.mu.Lock()
		err = e.attachLocked(pages[index])
		e.mu.Unlock()
		if err != nil {
			return nil, err
		}
		return e.describePage(ctx2, e.activeSession())

	case "close":
		e.mu.Lock()
		target := ""
		if e.tab != nil {
			target = e.tab.targetID
		}
		e.mu.Unlock()
		if target == "" {
			return nil, fmt.Errorf("no active tab")
		}
		if _, err := conn.Send(ctx2, "Target.closeTarget", map[string]any{"targetId": target}); err != nil {
			return nil, err
		}
		// Detach from the closed tab; if other pages remain, attach to one.
		e.mu.Lock()
		e.tab = nil
		e.mu.Unlock()
		if _, err := e.Tabs(ctx, "select", 0, ""); err == nil {
			return map[string]any{"closed": target, "switched_to": 0}, nil
		}
		return map[string]any{"closed": target}, nil
	}
	return nil, fmt.Errorf("unknown tabs action %q (use list, new, select, close)", action)
}

func (e *cdpEngine) activeSession() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.tab == nil {
		return ""
	}
	return e.tab.id
}

// HistoryNav goes back/forward/reloads the active tab.
func (e *cdpEngine) HistoryNav(ctx context.Context, action string) (map[string]any, error) {
	sess, err := e.ensureSession()
	if err != nil {
		return nil, err
	}
	expr := map[string]string{
		"back":    "history.back()",
		"forward": "history.forward()",
		"reload":  "location.reload()",
	}[action]
	if expr == "" {
		return nil, fmt.Errorf("unknown navigation action %q", action)
	}
	if _, err := e.evalSession(ctx, sess, expr); err != nil {
		return nil, fmt.Errorf("%s: %w", action, err)
	}
	e.waitForSettle(ctx, sess, 8*time.Second)
	return e.describePage(ctx, sess)
}
