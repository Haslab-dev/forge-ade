package app

import (
	"context"
)

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
	a.buMgr.SetRect(x, y, w, h, visible)
}

// BrowserUseStart launches the browser (idempotent).
func (a *App) BrowserUseStart() error {
	return a.buMgr.Start()
}

// BrowserUseStop shuts the browser down.
func (a *App) BrowserUseStop() {
	a.buMgr.Close()
}

// BrowserUseNavigate loads a URL in the active tab (viewer URL bar).
func (a *App) BrowserUseNavigate(url string) (map[string]any, error) {
	return a.buMgr.Navigate(context.Background(), url)
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

// BrowserUseWheel scrolls the page from the viewer (cdp engine only).
func (a *App) BrowserUseWheel(deltaY float64) (map[string]any, error) {
	return a.buMgr.WheelAt(context.Background(), deltaY)
}
