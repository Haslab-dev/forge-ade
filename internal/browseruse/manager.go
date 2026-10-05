package browseruse

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync"
	"unsafe"
)

// engine abstracts the browser backend. Two implementations:
//   - iabEngine  (in-app browser): a real WKWebView hosted as a native subview
//     of the main window, positioned over the Browser viewer panel — the page
//     renders inside the app (ZCode's WebContentsView architecture). macOS.
//   - cdpEngine: a headless Chromium driven over the DevTools Protocol with a
//     screencast streamed to the viewer (fallback for other platforms,
//     tests, or FORGE_ADE_BROWSER_ENGINE=cdp).
type engine interface {
	Start() error
	// Attach tells the engine where the viewer panel lives, so the in-app
	// engine can position its native view. x/y/w/h are CSS pixels relative
	// to the window content view; visible=false hides the view.
	SetRect(x, y, w, h float64, visible bool)
	EngineName() string
	HasTabs() bool

	Navigate(ctx context.Context, rawURL string) (map[string]any, error)
	Snapshot(ctx context.Context) (string, error)
	Click(ctx context.Context, ref string) (map[string]any, error)
	Type(ctx context.Context, ref, text string, submit bool) (map[string]any, error)
	Key(ctx context.Context, name string) (map[string]any, error)
	Scroll(ctx context.Context, direction string, amount int) (map[string]any, error)
	Screenshot(ctx context.Context) (string, error)
	HistoryNav(ctx context.Context, action string) (map[string]any, error)
	Status() map[string]any
	Close()

	// PickElement arms the viewer's "select element for chat" picker.
	PickElement(ctx context.Context) (map[string]any, error)
	setEmitter(fn func(event string, data map[string]any))
}

// Manager is the shared front door for the agent tools and the viewer API.
type Manager struct {
	mu       sync.Mutex
	dataDir  string
	windowFn func() unsafe.Pointer
	engine   engine
	emit     func(event string, data map[string]any)
}

// NewManager creates a browser-use manager. dataDir holds screenshots; the
// engine is chosen lazily on first use.
func NewManager(dataDir string) *Manager {
	return &Manager{dataDir: dataDir}
}

// SetEmitter installs the event sink:
//
//	browser:status {state, engine?, url?, title?, message?}
//	browser:frame  {data, format, width, height}   (cdp engine only)
func (m *Manager) SetEmitter(fn func(event string, data map[string]any)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.emit = fn
	if m.engine != nil {
		m.engine.setEmitter(fn)
	}
}

// SetWindowProvider installs the accessor for the main NSWindow handle
// (required by the in-app engine; called from the app layer).
func (m *Manager) SetWindowProvider(fn func() unsafe.Pointer) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.windowFn = fn
}

// current returns the active engine, building the platform default on first
// use: in-app WKWebView on macOS (when a window provider is registered),
// headless CDP otherwise.
func (m *Manager) current() (engine, error) {
	m.mu.Lock()
	if m.engine != nil {
		eng := m.engine
		m.mu.Unlock()
		return eng, nil
	}
	dataDir, windowFn, emit := m.dataDir, m.windowFn, m.emit
	m.mu.Unlock()

	eng := newPlatformEngine(dataDir, windowFn, emit)
	eng.setEmitter(emit)

	m.mu.Lock()
	if m.engine == nil {
		m.engine = eng
	}
	eng = m.engine
	m.mu.Unlock()
	return eng, nil
}

func newPlatformEngine(dataDir string, windowFn func() unsafe.Pointer, emit func(event string, data map[string]any)) engine {
	if os.Getenv("FORGE_ADE_BROWSER_ENGINE") != "cdp" &&
		runtime.GOOS == "darwin" && windowFn != nil {
		return newIABEngine(dataDir, windowFn, emit)
	}
	return newCDPEngine(dataDir, emit)
}

// EngineName reports the active backend ("in-app" or "cdp").
func (m *Manager) EngineName() string {
	eng, _ := m.current()
	if eng == nil {
		return "none"
	}
	return eng.EngineName()
}

// HasTabs reports whether the active engine supports multiple tabs.
func (m *Manager) HasTabs() bool {
	eng, _ := m.current()
	return eng != nil && eng.HasTabs()
}

// ViewerVisible reports whether the browser view is currently shown in the
// viewer panel.
func (m *Manager) ViewerVisible() bool {
	eng, err := m.current()
	if err != nil {
		return false
	}
	type viewerChecker interface{ ViewerVisible() bool }
	if vc, ok := eng.(viewerChecker); ok {
		return vc.ViewerVisible()
	}
	return true
}

// ---------------------------------------------------------------------------
// Engine delegation (agent tools + viewer API)
// ---------------------------------------------------------------------------

func (m *Manager) Start() error {
	eng, _ := m.current()
	return eng.Start()
}

func (m *Manager) SetRect(x, y, w, h float64, visible bool) {
	if eng, err := m.current(); err == nil {
		eng.SetRect(x, y, w, h, visible)
	}
}

func (m *Manager) Navigate(ctx context.Context, rawURL string) (map[string]any, error) {
	eng, err := m.current()
	if err != nil {
		return nil, err
	}
	return eng.Navigate(ctx, normalizeURL(rawURL))
}

func (m *Manager) Snapshot(ctx context.Context) (string, error) {
	eng, err := m.current()
	if err != nil {
		return "", err
	}
	return eng.Snapshot(ctx)
}

func (m *Manager) Click(ctx context.Context, ref string) (map[string]any, error) {
	eng, err := m.current()
	if err != nil {
		return nil, err
	}
	return eng.Click(ctx, ref)
}

func (m *Manager) Type(ctx context.Context, ref, text string, submit bool) (map[string]any, error) {
	eng, err := m.current()
	if err != nil {
		return nil, err
	}
	return eng.Type(ctx, ref, text, submit)
}

func (m *Manager) Key(ctx context.Context, name string) (map[string]any, error) {
	eng, err := m.current()
	if err != nil {
		return nil, err
	}
	return eng.Key(ctx, name)
}

func (m *Manager) Scroll(ctx context.Context, direction string, amount int) (map[string]any, error) {
	eng, err := m.current()
	if err != nil {
		return nil, err
	}
	return eng.Scroll(ctx, direction, amount)
}

func (m *Manager) Screenshot(ctx context.Context) (string, error) {
	eng, err := m.current()
	if err != nil {
		return "", err
	}
	return eng.Screenshot(ctx)
}

func (m *Manager) HistoryNav(ctx context.Context, action string) (map[string]any, error) {
	eng, err := m.current()
	if err != nil {
		return nil, err
	}
	return eng.HistoryNav(ctx, action)
}

func (m *Manager) Status() map[string]any {
	eng, err := m.current()
	if err != nil {
		return map[string]any{"running": false, "error": err.Error()}
	}
	return eng.Status()
}

func (m *Manager) Close() {
	m.mu.Lock()
	eng := m.engine
	m.mu.Unlock()
	if eng != nil {
		eng.Close()
	}
}

// PickElement arms the element picker and waits for the user's pick.
func (m *Manager) PickElement(ctx context.Context) (map[string]any, error) {
	eng, err := m.current()
	if err != nil {
		return nil, err
	}
	return eng.PickElement(ctx)
}

// Tabs manages tabs (cdp engine only — the in-app engine is single-tab).
func (m *Manager) Tabs(ctx context.Context, action string, index int, url string) (any, error) {
	eng, err := m.current()
	if err != nil {
		return nil, err
	}
	cdp, ok := eng.(*cdpEngine)
	if !ok {
		return nil, fmt.Errorf("tabs are not supported by the %s browser engine", eng.EngineName())
	}
	return cdp.Tabs(ctx, action, index, url)
}

// ClickAt clicks viewport coordinates (cdp engine's frame canvas).
func (m *Manager) ClickAt(ctx context.Context, x, y float64) (map[string]any, error) {
	eng, err := m.current()
	if err != nil {
		return nil, err
	}
	cdp, ok := eng.(*cdpEngine)
	if !ok {
		return nil, fmt.Errorf("canvas clicks are not needed for the %s engine (click the page directly)", eng.EngineName())
	}
	return cdp.ClickAt(ctx, x, y)
}

// WheelAt scrolls by deltaY (cdp engine's frame canvas).
func (m *Manager) WheelAt(ctx context.Context, deltaY float64) (map[string]any, error) {
	eng, err := m.current()
	if err != nil {
		return nil, err
	}
	cdp, ok := eng.(*cdpEngine)
	if !ok {
		return nil, fmt.Errorf("canvas wheel is not needed for the %s engine (scroll the page directly)", eng.EngineName())
	}
	return cdp.WheelAt(ctx, deltaY)
}
