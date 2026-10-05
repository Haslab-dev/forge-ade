//go:build !darwin

package browseruse

import (
	"context"
	"fmt"
	"unsafe"
)

// iabEngine is unavailable off macOS; the CDP engine is used instead.
type iabEngine struct{}

func newIABEngine(dataDir string, windowFn func() unsafe.Pointer, emit func(event string, data map[string]any)) *iabEngine {
	return &iabEngine{}
}

func (e *iabEngine) EngineName() string                                    { return "in-app" }
func (e *iabEngine) HasTabs() bool                                         { return false }
func (e *iabEngine) ViewerVisible() bool                                   { return false }
func (e *iabEngine) setEmitter(fn func(event string, data map[string]any)) {}

func (e *iabEngine) Start() error                             { return fmt.Errorf("the in-app browser requires macOS") }
func (e *iabEngine) SetRect(x, y, w, h float64, visible bool) {}
func (e *iabEngine) Navigate(ctx context.Context, rawURL string) (map[string]any, error) {
	return nil, fmt.Errorf("the in-app browser requires macOS")
}
func (e *iabEngine) Snapshot(ctx context.Context) (string, error) {
	return "", fmt.Errorf("the in-app browser requires macOS")
}
func (e *iabEngine) Click(ctx context.Context, ref string) (map[string]any, error) {
	return nil, fmt.Errorf("the in-app browser requires macOS")
}
func (e *iabEngine) Type(ctx context.Context, ref, text string, submit bool) (map[string]any, error) {
	return nil, fmt.Errorf("the in-app browser requires macOS")
}
func (e *iabEngine) Key(ctx context.Context, name string) (map[string]any, error) {
	return nil, fmt.Errorf("the in-app browser requires macOS")
}
func (e *iabEngine) Scroll(ctx context.Context, direction string, amount int) (map[string]any, error) {
	return nil, fmt.Errorf("the in-app browser requires macOS")
}
func (e *iabEngine) Screenshot(ctx context.Context) (string, error) {
	return "", fmt.Errorf("the in-app browser requires macOS")
}
func (e *iabEngine) HistoryNav(ctx context.Context, action string) (map[string]any, error) {
	return nil, fmt.Errorf("the in-app browser requires macOS")
}
func (e *iabEngine) PickElement(ctx context.Context) (map[string]any, error) {
	return nil, fmt.Errorf("the in-app browser requires macOS")
}
func (e *iabEngine) Status() map[string]any {
	return map[string]any{"running": false, "engine": "unavailable"}
}
func (e *iabEngine) Close() {}
