//go:build darwin

package browseruse

// iabEngine hosts a real WKWebView as a native subview of the app's main
// window, positioned over the Browser viewer panel — the page renders inside
// the app (the Wails equivalent of ZCode's Electron WebContentsView in-app
// browser). The agent drives it through evaluateJavaScript; no external
// browser process exists.

/*
#cgo darwin CFLAGS: -x objective-c -fobjc-arc
#cgo darwin LDFLAGS: -framework Cocoa -framework WebKit

#include <stdint.h>
#include <stdlib.h>

#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>
#import <objc/runtime.h>

extern void forgeIABEvalDone(int64_t reqID, char *result, char *errStr);
extern void forgeIABShotDone(int64_t reqID, unsigned char *png, long len, char *errStr);

static void iab_install_crash_guard(void *wk);

static NSString *iabLoadScript = nil;

static void iab_set_load_script(const char *js);

static void *iab_create_impl(void *nswindow) {
	NSWindow *win = (__bridge NSWindow *)nswindow;
	if (!win.contentView) return NULL;
	WKWebViewConfiguration *cfg = [[WKWebViewConfiguration alloc] init];
	cfg.websiteDataStore = [WKWebsiteDataStore nonPersistentDataStore];

	// Per-load counter + reload tracer, sourced from Go (loadCounterScript):
	// survives reloads via sessionStorage, so the tools can tell "the page
	// reloaded" from "nothing happened", and JS-initiated reloads leave a
	// stack trace in __forgeReloadTrace.
	WKUserContentController *ucc = [[WKUserContentController alloc] init];
	if (iabLoadScript) {
		WKUserScript *loadCounter = [[WKUserScript alloc]
			initWithSource:iabLoadScript
			injectionTime:WKUserScriptInjectionTimeAtDocumentStart
			forMainFrameOnly:YES];
		[ucc addUserScript:loadCounter];
	}
	cfg.userContentController = ucc;

	WKWebView *wv = [[WKWebView alloc] initWithFrame:NSMakeRect(0, 0, 200, 200) configuration:cfg];
	wv.hidden = YES;
	iab_install_crash_guard((__bridge void *)wv);
	[win.contentView addSubview:wv];
	return (__bridge_retained void *)wv;
}

static void iab_set_load_script(const char *js) {
	if (!js) return;
	NSString *s = [NSString stringWithUTF8String:js];
	if (![NSThread isMainThread]) {
		dispatch_sync(dispatch_get_main_queue(), ^{ iabLoadScript = s; });
		return;
	}
	iabLoadScript = s;
}

// WebContent process death (jetsam, renderer crash) leaves WKWebView showing
// a blank page with no error surfaced to Go — the agent's next evals just
// fail mysteriously. Observe the termination notification and reload so the
// view is self-healing.
static void iab_install_crash_guard(void *wk) {
	if (!wk) return;
	if (![NSThread isMainThread]) {
		dispatch_async(dispatch_get_main_queue(), ^{ iab_install_crash_guard(wk); });
		return;
	}
	WKWebView *wv = (__bridge WKWebView *)wk;
	__weak WKWebView *weakWV = wv;
	id token = [[NSNotificationCenter defaultCenter]
		addObserverForName:@"WKWebViewWebContentProcessDidTerminateNotification"
		object:wv
		queue:[NSOperationQueue mainQueue]
		usingBlock:^(NSNotification *note) {
			[weakWV reload];
		}];
	// Tie the observer's lifetime to the webview; the block only holds a
	// weak reference, so tearing down the view stays clean.
	objc_setAssociatedObject(wv, &iab_install_crash_guard, token, OBJC_ASSOCIATION_RETAIN);
}

static void *iab_create(void *nswindow) {
	if (!nswindow) return NULL;
	if ([NSThread isMainThread]) return iab_create_impl(nswindow);
	__block void *out = NULL;
	dispatch_sync(dispatch_get_main_queue(), ^{ out = iab_create_impl(nswindow); });
	return out;
}

static void iab_set_frame(void *wk, double x, double y, double w, double h, int visible) {
	if (!wk) return;
	WKWebView *wv = (__bridge WKWebView *)wk;
	if (![NSThread isMainThread]) {
		dispatch_async(dispatch_get_main_queue(), ^{ iab_set_frame(wk, x, y, w, h, visible); });
		return;
	}
	NSView *content = wv.superview;
	if (!content) return;
	double ch = content.bounds.size.height;
	// CSS coordinates are top-left based; Cocoa is bottom-left.
	wv.frame = NSMakeRect(x, ch - y - h, w, h);
	wv.hidden = !visible;
}

static void iab_load_inner(void *wk, NSString *u) {
	WKWebView *wv = (__bridge WKWebView *)wk;
	NSURL *ns = [NSURL URLWithString:u];
	if (!ns) {
		// Percent-encode stray characters (spaces, unicode) and retry.
		NSString *enc = [u stringByAddingPercentEncodingWithAllowedCharacters:[NSCharacterSet URLFragmentAllowedCharacterSet]];
		ns = [NSURL URLWithString:enc];
	}
	if (!ns) ns = [NSURL URLWithString:@"about:blank"];
	[wv loadRequest:[NSURLRequest requestWithURL:ns]];
}

static void iab_load_html_inner(void *wk, NSString *html) {
	WKWebView *wv = (__bridge WKWebView *)wk;
	[wv loadHTMLString:html baseURL:nil];
}

static void iab_load(void *wk, const char *url) {
	if (!wk) return;
	NSString *u = url ? [NSString stringWithUTF8String:url] : @"about:blank";
	if (![NSThread isMainThread]) {
		dispatch_async(dispatch_get_main_queue(), ^{ iab_load_inner(wk, u); });
		return;
	}
	iab_load_inner(wk, u);
}

static void iab_load_html(void *wk, const char *html) {
	if (!wk) return;
	NSString *h = [NSString stringWithUTF8String:html ? html : ""];
	if (![NSThread isMainThread]) {
		dispatch_async(dispatch_get_main_queue(), ^{ iab_load_html_inner(wk, h); });
		return;
	}
	iab_load_html_inner(wk, h);
}

static void iab_nav(void *wk, int action) {
	if (!wk) return;
	if (![NSThread isMainThread]) {
		dispatch_async(dispatch_get_main_queue(), ^{ iab_nav(wk, action); });
		return;
	}
	WKWebView *wv = (__bridge WKWebView *)wk;
	switch (action) {
		case 0: [wv goBack]; break;
		case 1: [wv goForward]; break;
		case 2: [wv reload]; break;
	}
}

static void iab_eval_js_inner(void *wk, int64_t reqID, NSString *js) {
	WKWebView *wv = (__bridge WKWebView *)wk;
	[wv evaluateJavaScript:js completionHandler:^(id result, NSError *error) {
		NSString *res = nil;
		NSString *err = error.localizedDescription;
		if (!error && result) {
			if ([result isKindOfClass:[NSString class]]) {
				res = result;
			} else if ([result isKindOfClass:[NSNull class]]) {
				res = @"null";
			} else if ([NSJSONSerialization isValidJSONObject:result]) {
				NSData *d = [NSJSONSerialization dataWithJSONObject:result options:0 error:nil];
				if (d) res = [[NSString alloc] initWithData:d encoding:NSUTF8StringEncoding];
				else res = [NSString stringWithFormat:@"%@", result];
			} else {
				// NaN/Infinity numbers, Dates, host objects… — top-level
				// scalars that dataWithJSONObject would THROW on. Describe
				// them; an eval result must never crash the app.
				if ([result isKindOfClass:[NSNumber class]]) {
					res = @"null";
				} else if ([result isKindOfClass:[NSDate class]]) {
					res = [NSString stringWithFormat:@"%.0f", [result timeIntervalSince1970] * 1000.0];
				} else {
					res = [NSString stringWithFormat:@"%@", result];
				}
			}
		}
		forgeIABEvalDone(reqID, res ? (char *)res.UTF8String : NULL, err ? (char *)err.UTF8String : NULL);
	}];
}

static void iab_eval_js(void *wk, int64_t reqID, const char *code) {
	if (!wk) return;
	NSString *js = [NSString stringWithUTF8String:code ? code : ""];
	if (![NSThread isMainThread]) {
		dispatch_async(dispatch_get_main_queue(), ^{ iab_eval_js_inner(wk, reqID, js); });
		return;
	}
	iab_eval_js_inner(wk, reqID, js);
}

static void iab_take_shot(void *wk, int64_t reqID) {
	if (!wk) return;
	if (![NSThread isMainThread]) {
		dispatch_async(dispatch_get_main_queue(), ^{ iab_take_shot(wk, reqID); });
		return;
	}
	WKWebView *wv = (__bridge WKWebView *)wk;
	WKSnapshotConfiguration *cfg = [[WKSnapshotConfiguration alloc] init];
	[wv takeSnapshotWithConfiguration:cfg completionHandler:^(NSImage *img, NSError *error) {
		if (!img) {
			NSString *msg = error.localizedDescription ?: @"snapshot failed";
			forgeIABShotDone(reqID, NULL, 0, (char *)msg.UTF8String);
			return;
		}
		NSBitmapImageRep *rep = [NSBitmapImageRep imageRepWithData:[img TIFFRepresentation]];
		NSData *png = [rep representationUsingType:NSBitmapImageFileTypePNG properties:@{}];
		forgeIABShotDone(reqID, (unsigned char *)png.bytes, (long)png.length, NULL);
	}];
}

static void iab_destroy(void *wk) {
	if (!wk) return;
	WKWebView *wv = (__bridge_transfer WKWebView *)wk;
	if (![NSThread isMainThread]) {
		dispatch_async(dispatch_get_main_queue(), ^{ [wv removeFromSuperview]; });
		return;
	}
	[wv removeFromSuperview];
}
*/
import "C"

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/google/uuid"
)

type iabEngine struct {
	mu       sync.Mutex
	emit     func(event string, data map[string]any)
	dataDir  string
	windowFn func() unsafe.Pointer

	view    unsafe.Pointer // retained WKWebView
	rect    [4]float64     // css x,y,w,h relative to the window content view
	visible bool
	started bool
	closed  bool

	lastURL   string
	lastTitle string

	// lastLoadCount tracks the page-load counter (see iab_create) so actions
	// can report "the page reloaded since the last action".
	lastLoadCount int

	// lastFrame caches the last applied frame/visibility; identical SetRect
	// calls are skipped — re-assigning the frame on a layer-backed WKWebView
	// every poll cycle forces layout and visibly flickers the page.
	lastFrame      [4]float64
	lastVisible    bool
	hasAppliedOnce bool

	pollStop chan struct{}
}

func newIABEngine(dataDir string, windowFn func() unsafe.Pointer, emit func(event string, data map[string]any)) *iabEngine {
	return &iabEngine{
		dataDir:  dataDir,
		windowFn: windowFn,
		emit:     emit,
		visible:  true,
		pollStop: make(chan struct{}),
	}
}

func (e *iabEngine) EngineName() string { return "in-app" }
func (e *iabEngine) HasTabs() bool      { return false }

// ViewerVisible reports whether the native view is currently shown in the
// viewer panel.
func (e *iabEngine) ViewerVisible() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.view != nil && e.visible
}

func (e *iabEngine) setEmitter(fn func(event string, data map[string]any)) { e.emit = fn }

// ---------------------------------------------------------------------------
// Pending-request registry shared by the C callbacks.
// ---------------------------------------------------------------------------

type iabResult struct {
	result string
	err    string
	png    []byte
}

var iabPending = struct {
	mu   sync.Mutex
	next int64
	m    map[int64]chan iabResult
}{m: make(map[int64]chan iabResult)}

func iabRegister() (int64, chan iabResult) {
	iabPending.mu.Lock()
	defer iabPending.mu.Unlock()
	iabPending.next++
	id := iabPending.next
	ch := make(chan iabResult, 1)
	iabPending.m[id] = ch
	return id, ch
}

func iabDeliver(id int64, res iabResult) {
	iabPending.mu.Lock()
	ch, ok := iabPending.m[id]
	if ok {
		delete(iabPending.m, id)
	}
	iabPending.mu.Unlock()
	if ok {
		select {
		case ch <- res:
		default:
		}
	}
}

//export forgeIABEvalDone
func forgeIABEvalDone(reqID C.int64_t, result *C.char, errStr *C.char) {
	res := iabResult{}
	if result != nil {
		res.result = C.GoString(result)
	}
	if errStr != nil {
		res.err = C.GoString(errStr)
	}
	iabDeliver(int64(reqID), res)
}

//export forgeIABShotDone
func forgeIABShotDone(reqID C.int64_t, png *C.uchar, length C.long, errStr *C.char) {
	res := iabResult{}
	if errStr != nil {
		res.err = C.GoString(errStr)
	}
	if png != nil && length > 0 {
		res.png = C.GoBytes(unsafe.Pointer(png), C.int(length))
	}
	iabDeliver(int64(reqID), res)
}

// ---------------------------------------------------------------------------
// Engine interface
// ---------------------------------------------------------------------------

func (e *iabEngine) Start() error {
	e.mu.Lock()
	if e.started && e.view != nil {
		e.mu.Unlock()
		return nil
	}
	fn, rect, visible := e.windowFn, e.rect, e.visible
	e.mu.Unlock()
	return e.attach(fn, rect, visible)
}

func (e *iabEngine) attach(windowFn func() unsafe.Pointer, rect [4]float64, visible bool) error {
	if windowFn == nil {
		return fmt.Errorf("main window is not available yet")
	}
	win := windowFn()
	if win == nil {
		return fmt.Errorf("main window is not available yet")
	}
	cScript := C.CString(loadCounterScript)
	C.iab_set_load_script(cScript)
	C.free(unsafe.Pointer(cScript))
	view := C.iab_create(win)
	if view == nil {
		return fmt.Errorf("failed to create the in-app browser view")
	}
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		C.iab_destroy(view)
		return fmt.Errorf("engine closed")
	}
	e.view = view
	e.started = true
	e.mu.Unlock()
	C.iab_set_frame(view, C.double(rect[0]), C.double(rect[1]), C.double(rect[2]), C.double(rect[3]), cInt(visible))
	C.iab_load(view, nil) // about:blank
	go e.pollStatusLoop()
	e.emitStatus("running", "")
	return nil
}

func cInt(b bool) C.int {
	if b {
		return 1
	}
	return 0
}

// SetRect repositions the native view over the viewer panel. Identical
// repeated calls are no-ops (the viewer polls its rect on an interval).
func (e *iabEngine) SetRect(x, y, w, h float64, visible bool) {
	e.mu.Lock()
	e.rect = [4]float64{x, y, w, h}
	e.visible = visible
	view := e.view
	if e.hasAppliedOnce &&
		e.lastFrame == e.rect && e.lastVisible == visible {
		e.mu.Unlock()
		return
	}
	e.lastFrame = e.rect
	e.lastVisible = visible
	e.hasAppliedOnce = true
	e.mu.Unlock()
	if view != nil {
		C.iab_set_frame(view, C.double(x), C.double(y), C.double(w), C.double(h), cInt(visible))
	}
}

func (e *iabEngine) emitStatus(state, message string) {
	if e.emit == nil {
		return
	}
	data := map[string]any{"state": state, "engine": "in-app"}
	if message != "" {
		data["message"] = message
	}
	e.mu.Lock()
	url, title := e.lastURL, e.lastTitle
	e.mu.Unlock()
	if url != "" {
		data["url"] = url
	}
	if title != "" {
		data["title"] = title
	}
	e.emit("browser:status", data)
}

// pollStatusLoop keeps URL/title fresh for the viewer's URL bar and drives
// the auto-open signal. It runs while the view exists — even when the panel
// is closed — otherwise nothing tells the frontend to open the panel when
// the agent navigates.
func (e *iabEngine) pollStatusLoop() {
	ticker := time.NewTicker(800 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-e.pollStop:
			return
		case <-ticker.C:
			e.mu.Lock()
			alive := e.view != nil
			e.mu.Unlock()
			if !alive {
				continue
			}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			raw, err := e.eval(ctx, pageStatusScript)
			cancel()
			if err != nil {
				continue
			}
			var st struct {
				URL   string `json:"url"`
				Title string `json:"title"`
			}
			if json.Unmarshal(raw, &st) != nil {
				continue
			}
			e.mu.Lock()
			changed := st.URL != e.lastURL || st.Title != e.lastTitle
			e.lastURL, e.lastTitle = st.URL, st.Title
			e.mu.Unlock()
			if changed && st.URL != "" {
				e.emitStatus("navigated", "")
			}
		}
	}
}

func (e *iabEngine) Close() {
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return
	}
	e.closed = true
	view := e.view
	e.view = nil
	e.started = false
	e.mu.Unlock()
	if e.pollStop != nil {
		select {
		case <-e.pollStop:
		default:
			close(e.pollStop)
		}
	}
	if view != nil {
		C.iab_destroy(view)
	}
	e.emitStatus("stopped", "")
}

// eval runs JavaScript in the page and returns the raw JSON-encoded result.
func (e *iabEngine) eval(ctx context.Context, code string) (json.RawMessage, error) {
	e.mu.Lock()
	view := e.view
	e.mu.Unlock()
	if view == nil {
		if err := e.Start(); err != nil {
			return nil, err
		}
		e.mu.Lock()
		view = e.view
		e.mu.Unlock()
		if view == nil {
			return nil, fmt.Errorf("in-app browser view is not available")
		}
	}
	id, ch := iabRegister()
	cCode := C.CString(code)
	defer C.free(unsafe.Pointer(cCode))
	C.iab_eval_js(view, C.int64_t(id), cCode)

	select {
	case res := <-ch:
		if res.err != "" {
			return nil, fmt.Errorf("page evaluation failed: %s", res.err)
		}
		if res.result == "" {
			return json.RawMessage("null"), nil
		}
		return json.RawMessage(res.result), nil
	case <-ctx.Done():
		return nil, fmt.Errorf("page evaluation timed out")
	}
}

func (e *iabEngine) evalString(ctx context.Context, code string) (string, error) {
	raw, err := e.eval(ctx, code)
	if err != nil {
		return "", err
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s, nil
	}
	return string(raw), nil
}

func (e *iabEngine) pageState(ctx context.Context) (url, title, ready string) {
	raw, err := e.eval(ctx, pageStatusScript)
	if err != nil {
		return "", "", ""
	}
	var st struct {
		URL   string `json:"url"`
		Title string `json:"title"`
		Ready string `json:"ready"`
	}
	if json.Unmarshal(raw, &st) == nil {
		return st.URL, st.Title, st.Ready
	}
	return "", "", ""
}

func (e *iabEngine) waitForSettle(ctx context.Context, max time.Duration) {
	deadline := time.Now().Add(max)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return
		default:
		}
		_, _, ready := e.pageState(ctx)
		if ready == "complete" {
			time.Sleep(400 * time.Millisecond)
			return
		}
		time.Sleep(150 * time.Millisecond)
	}
}

func (e *iabEngine) describePage(ctx context.Context) (map[string]any, error) {
	url, title, _ := e.pageState(ctx)
	out := map[string]any{"url": url, "title": title}
	if n, err := e.loadCount(ctx); err == nil {
		out["loads"] = n
	}
	snap, err := e.Snapshot(ctx)
	if err != nil {
		out["snapshot_error"] = err.Error()
	} else if snap != "" {
		out["snapshot"] = snap
	}
	return out, nil
}

// loadCount reads the per-document load counter (see loadCounterScript).
func (e *iabEngine) loadCount(ctx context.Context) (int, error) {
	raw, err := e.evalString(ctx, pageLoadCountExpr)
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n < 1 {
		return 1, nil
	}
	return n, nil
}

// waitForStablePage samples the load counter to detect a reload storm —
// the page under test re-creating documents faster than actions can land
// (dev server restarting, HMR websocket death loop). It returns the load
// count once two samples 500ms apart agree, or the latest count and
// stormed=true if it was still climbing at the deadline.
func (e *iabEngine) waitForStablePage(ctx context.Context, max time.Duration) (loads int, stormed bool) {
	first, err := e.loadCount(ctx)
	if err != nil {
		return 0, false // counter unreadable (about:blank etc.) — don't block
	}
	deadline := time.Now().Add(max)
	for {
		select {
		case <-ctx.Done():
			return first, false
		case <-time.After(500 * time.Millisecond):
		}
		second, err := e.loadCount(ctx)
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

func (e *iabEngine) Navigate(ctx context.Context, rawURL string) (map[string]any, error) {
	if err := e.Start(); err != nil {
		return nil, err
	}
	e.mu.Lock()
	view := e.view
	e.mu.Unlock()
	if view == nil {
		return nil, fmt.Errorf("in-app browser view is not available")
	}
	if strings.HasPrefix(rawURL, "data:text/html,") {
		// NSURL rejects unescaped HTML; use the dedicated HTML loader.
		html := strings.TrimPrefix(rawURL, "data:text/html,")
		if strings.HasPrefix(html, "charset=utf-8,") {
			html = strings.TrimPrefix(html, "charset=utf-8,")
		}
		cHTML := C.CString(html)
		C.iab_load_html(view, cHTML)
		C.free(unsafe.Pointer(cHTML))
	} else {
		cURL := C.CString(normalizeURL(rawURL))
		C.iab_load(view, cURL)
		C.free(unsafe.Pointer(cURL))
	}

	nctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	e.waitForSettle(nctx, 10*time.Second)
	// Announce immediately (don't wait for the poll tick) so the frontend's
	// auto-open can bring the viewer panel up while the tool is returning.
	if url, title, _ := e.pageState(nctx); url != "" {
		e.mu.Lock()
		e.lastURL, e.lastTitle = url, title
		e.mu.Unlock()
		e.emitStatus("navigated", "")
	}
	res, err := e.describePage(nctx)
	if err != nil {
		return nil, err
	}
	// Baseline the reload detector: a navigate legitimately bumps the load
	// counter, so the first click after it must not warn about this load.
	if n, err := e.loadCount(nctx); err == nil {
		e.mu.Lock()
		e.lastLoadCount = n
		e.mu.Unlock()
	}
	return res, nil
}

func (e *iabEngine) Snapshot(ctx context.Context) (string, error) {
	if err := e.Start(); err != nil {
		return "", err
	}
	s, err := e.evalString(ctx, snapshotScript)
	if err != nil {
		return "", fmt.Errorf("snapshot: %w", err)
	}
	return s, nil
}

func (e *iabEngine) resolveRef(ctx context.Context, ref string) error {
	out, err := e.evalString(ctx, fmt.Sprintf(resolveRefScript, ref))
	if err != nil {
		return err
	}
	var r refRect
	if json.Unmarshal([]byte(out), &r) != nil || !r.Found {
		// Include page diagnostics — a React remount or HMR full reload wipes
		// data-fref attributes; the count tells stale-ref from broken-eval.
		href, _ := e.evalString(ctx, "location.href")
		n, _ := e.evalString(ctx, "String(document.querySelectorAll('[data-fref]').length)")
		return fmt.Errorf("ref %s not found (page: %s, refs in dom: %s) — take a fresh browser_snapshot and retry", ref, href, n)
	}
	return nil
}

// resolveRefWithRetry tolerates a DOM replace between snapshot and action:
// re-snapshot once (re-tagging refs) and retry the same ref.
func (e *iabEngine) resolveRefWithRetry(ctx context.Context, ref string) error {
	err := e.resolveRef(ctx, ref)
	if err == nil {
		return nil
	}
	if _, serr := e.Snapshot(ctx); serr == nil {
		if retryErr := e.resolveRef(ctx, ref); retryErr == nil {
			return nil
		}
	}
	return err
}

// clickResult is the JSON the page-side clickScript returns.
type clickResult struct {
	Found        bool   `json:"found"`
	ClickedLabel string `json:"clicked_label"`
	Href         string `json:"href"`
	Loads        int    `json:"loads"`
}

func (e *iabEngine) Click(ctx context.Context, ref string) (map[string]any, error) {
	if err := e.Start(); err != nil {
		return nil, err
	}
	if err := e.resolveRefWithRetry(ctx, ref); err != nil {
		return nil, err
	}
	clickOut, err := e.evalString(ctx, fmt.Sprintf(clickScript, ref))
	if err != nil {
		return nil, fmt.Errorf("click: %w", err)
	}
	var click clickResult
	_ = json.Unmarshal([]byte(clickOut), &click)

	nctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	e.waitForSettle(nctx, 3*time.Second)

	// Reload-storm guard: sample the load counter; if the page reloaded
	// after the click (the click landed on a document that was replaced) or
	// reloads are still churning, let it go quiet and re-land the click ONCE
	// on the fresh document — same URL only (a navigated click must not be
	// repeated).
	loads, stormed := e.waitForStablePage(nctx, 8*time.Second)
	retried := false
	if (loads > click.Loads || stormed) && click.Href != "" {
		if href, herr := e.evalString(nctx, "location.href"); herr == nil && href == click.Href {
			if rerr := e.resolveRefWithRetry(nctx, ref); rerr == nil {
				if _, cerr := e.evalString(nctx, fmt.Sprintf(clickScript, ref)); cerr == nil {
					retried = true
					e.waitForSettle(nctx, 2*time.Second)
					loads, stormed = e.waitForStablePage(nctx, 4*time.Second)
				}
			}
		}
	}

	res, err := e.describePage(nctx)
	if err != nil {
		return nil, err
	}
	// Surface what the click actually did and what the page did afterwards.
	if click.Found {
		if click.ClickedLabel != "" {
			res["clicked_label"] = click.ClickedLabel
		}
		e.mu.Lock()
		prev := e.lastLoadCount
		e.lastLoadCount = loads
		e.mu.Unlock()
		if prev > 0 && loads > prev {
			res["reload_warning"] = fmt.Sprintf("the page reloaded since the last action (load %d → %d) — in-page state (forms, counters) was reset", prev, loads)
		}
	}
	if stormed {
		res["reload_storm"] = "the page kept reloading for the whole stability window (load " + strconv.Itoa(loads) + ") — the app under test is restarting its dev server or hot-reload loop; verify state before further actions"
	}
	if loads > click.Loads || stormed {
		if raw, err := e.evalString(nctx, reloadTraceExpr); err == nil && strings.HasPrefix(raw, "[") {
			var trace []json.RawMessage
			if json.Unmarshal([]byte(raw), &trace) == nil && len(trace) > 0 {
				res["reload_trace"] = json.RawMessage(raw)
			} else if raw != "" {
				res["reload_trace_raw"] = raw
			}
		}
	}
	if retried {
		res["retried_after_reload"] = true
	}
	return res, nil
}

func (e *iabEngine) Type(ctx context.Context, ref, text string, submit bool) (map[string]any, error) {
	if err := e.Start(); err != nil {
		return nil, err
	}
	if err := e.resolveRefWithRetry(ctx, ref); err != nil {
		return nil, err
	}
	if _, err := e.evalString(ctx, fmt.Sprintf(typeScript, ref, text)); err != nil {
		return nil, fmt.Errorf("type: %w", err)
	}
	if submit {
		if _, err := e.evalString(ctx, submitScript); err != nil {
			return nil, fmt.Errorf("submit: %w", err)
		}
	}
	nctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	e.waitForSettle(nctx, 3*time.Second)
	return e.describePage(nctx)
}

func (e *iabEngine) Key(ctx context.Context, name string) (map[string]any, error) {
	if err := e.Start(); err != nil {
		return nil, err
	}
	normalized := strings.ToLower(strings.TrimSpace(name))
	if descriptor, ok := cdpKeyMap[normalized]; ok {
		if _, err := e.evalString(ctx, fmt.Sprintf(keyScript, descriptor.key, descriptor.code, descriptor.windowsVK)); err != nil {
			return nil, fmt.Errorf("key: %w", err)
		}
	} else if len([]rune(name)) == 1 {
		if _, err := e.evalString(ctx, fmt.Sprintf(insertTextScript, name)); err != nil {
			return nil, fmt.Errorf("key: %w", err)
		}
	} else {
		return nil, fmt.Errorf("unsupported key %q (supported: enter, tab, escape, backspace, delete, arrow*, home, end, page*, or a single character)", name)
	}
	nctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	e.waitForSettle(nctx, 2*time.Second)
	return e.describePage(nctx)
}

func (e *iabEngine) Scroll(ctx context.Context, direction string, amount int) (map[string]any, error) {
	if err := e.Start(); err != nil {
		return nil, err
	}
	if amount <= 0 {
		amount = 5
	}
	delta := amount * 100
	if direction == "up" {
		delta = -delta
	}
	if _, err := e.evalString(ctx, "window.scrollBy(0, "+strconv.Itoa(delta)+"); 'ok'"); err != nil {
		return nil, fmt.Errorf("scroll: %w", err)
	}
	time.Sleep(300 * time.Millisecond)
	return e.describePage(ctx)
}

func (e *iabEngine) Screenshot(ctx context.Context) (string, error) {
	if err := e.Start(); err != nil {
		return "", err
	}
	e.mu.Lock()
	view := e.view
	e.mu.Unlock()
	if view == nil {
		return "", fmt.Errorf("in-app browser view is not available")
	}
	id, ch := iabRegister()
	C.iab_take_shot(view, C.int64_t(id))
	select {
	case res := <-ch:
		if res.err != "" {
			return "", fmt.Errorf("screenshot: %s", res.err)
		}
		if len(res.png) == 0 {
			return "", fmt.Errorf("screenshot returned no image")
		}
		_ = os.MkdirAll(filepath.Join(e.dataDir, "screenshots"), 0755)
		path := filepath.Join(e.dataDir, "screenshots", fmt.Sprintf("browser-%s.png", uuid.New().String()[:8]))
		if err := os.WriteFile(path, res.png, 0644); err != nil {
			return "", err
		}
		return path, nil
	case <-ctx.Done():
		return "", fmt.Errorf("screenshot timed out")
	}
}

func (e *iabEngine) HistoryNav(ctx context.Context, action string) (map[string]any, error) {
	if err := e.Start(); err != nil {
		return nil, err
	}
	e.mu.Lock()
	view := e.view
	e.mu.Unlock()
	if view == nil {
		return nil, fmt.Errorf("in-app browser view is not available")
	}
	var navAction C.int
	switch action {
	case "back":
		navAction = 0
	case "forward":
		navAction = 1
	case "reload":
		navAction = 2
	default:
		return nil, fmt.Errorf("unknown navigation action %q (use back, forward, reload)", action)
	}
	C.iab_nav(view, navAction)
	nctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	e.waitForSettle(nctx, 8*time.Second)
	return e.describePage(nctx)
}

// PickElement arms the element picker (viewer's "select element for chat");
// the user's next click in the in-app browser resolves to a selector.
func (e *iabEngine) PickElement(ctx context.Context) (map[string]any, error) {
	if err := e.Start(); err != nil {
		return nil, err
	}
	armed, err := e.evalString(ctx, pickElementScript)
	if err != nil {
		return nil, err
	}
	if armed != "armed" && armed != "already-armed" {
		return nil, fmt.Errorf("failed to arm the element picker")
	}
	deadline := time.Now().Add(35 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		raw, err := e.evalString(ctx, "window.__forgePickResult ? window.__forgePickResult : ''")
		if err == nil && raw != "" {
			return map[string]any{"picked": json.RawMessage(raw)}, nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return nil, fmt.Errorf("element picking timed out (30s)")
}

func (e *iabEngine) Status() map[string]any {
	e.mu.Lock()
	running := e.view != nil
	url, title := e.lastURL, e.lastTitle
	e.mu.Unlock()
	out := map[string]any{"running": running, "engine": "in-app"}
	if url != "" {
		out["url"] = url
	}
	if title != "" {
		out["title"] = title
	}
	return out
}
