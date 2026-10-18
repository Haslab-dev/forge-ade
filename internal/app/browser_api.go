package app

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"
)

func logToIAB(format string, args ...any) {
	f, err := os.OpenFile("/tmp/forge_iab.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		fmt.Fprintf(f, "[%s] %s\n", time.Now().UTC().Format(time.RFC3339), fmt.Sprintf(format, args...))
		_ = f.Close()
	}
}

// Browser Use API — drives the managed browser from the Browser viewer panel
// in the right sidebar (the agent uses the browser_* tools directly).

// BrowserUseStatus reports whether the browser is running plus page info.
func (a *App) BrowserUseStatus() map[string]any {
	return a.buMgr.Status()
}

// BrowserUseEngine reports the active backend: "in-app" (WKWebView embedded
// in the window) or "cdp" (headless screencast fallback).
func (a *App) BrowserUseEngine() string {
	return a.buMgr.EngineName()
}

// BrowserUseSetRect positions the in-app browser view over the viewer panel.
// x/y/w/h are CSS pixels relative to the window content view; visible=false
// hides the view (tab closed / pane collapsed).
func (a *App) BrowserUseSetRect(x, y, w, h float64, visible bool) {
	logToIAB("[BrowserUseSetRect] x=%.1f y=%.1f w=%.1f h=%.1f visible=%v", x, y, w, h, visible)
	log.Printf("[BrowserUseSetRect] x=%.1f y=%.1f w=%.1f h=%.1f visible=%v", x, y, w, h, visible)
	a.buMgr.SetRect(x, y, w, h, visible)
}

// BrowserUseStart launches the browser (idempotent).
func (a *App) BrowserUseStart() error {
	logToIAB("[BrowserUseStart] called")
	return a.buMgr.Start()
}

// BrowserUseStop shuts the browser down.
func (a *App) BrowserUseStop() {
	logToIAB("[BrowserUseStop] called")
	a.buMgr.Close()
}

// BrowserUseNavigate loads a URL in the active tab (viewer URL bar).
func (a *App) BrowserUseNavigate(url string) (map[string]any, error) {
	logToIAB("[BrowserUseNavigate] url=%s", url)
	log.Printf("[BrowserUseNavigate] url=%s", url)
	res, err := a.buMgr.Navigate(context.Background(), url)
	logToIAB("[BrowserUseNavigate] url=%s DONE res=%v err=%v", url, res != nil, err)
	return res, err
}

// BrowserUseHistory performs back/forward/reload from the viewer controls.
func (a *App) BrowserUseHistory(action string) (map[string]any, error) {
	return a.buMgr.HistoryNav(context.Background(), action)
}

// BrowserUseScreenshot captures the current page; the path is returned.
func (a *App) BrowserUseScreenshot() (string, error) {
	return a.buMgr.Screenshot(context.Background())
}

// BrowserUsePickElement arms the "select element for chat" picker and waits
// for the user's click inside the in-app browser.
func (a *App) BrowserUsePickElement() (map[string]any, error) {
	return a.buMgr.PickElement(context.Background())
}

// BrowserUseClickAt forwards a click on the viewer's rendered frame to the
// page (cdp engine only; the in-app browser is clicked directly).
func (a *App) BrowserUseClickAt(x, y float64) (map[string]any, error) {
	return a.buMgr.ClickAt(context.Background(), x, y)
}

// BrowserUseCancelPick cancels any active element picker.
func (a *App) BrowserUseCancelPick() error {
	return a.buMgr.CancelPick(context.Background())
}

// BrowserUseSetZoom sets page zoom factor (1.0 = 100%).
func (a *App) BrowserUseSetZoom(zoom float64) error {
	return a.buMgr.SetZoom(zoom)
}

// BrowserUseWheel scrolls the page from the viewer (cdp engine only).
func (a *App) BrowserUseWheel(deltaY float64) (map[string]any, error) {
	return a.buMgr.WheelAt(context.Background(), deltaY)
}
