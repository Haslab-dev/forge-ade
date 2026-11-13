// Package computeruse gives ForgeADE's built-in agent native macOS desktop
// control — its own computer-use tools, with no external MCP server: screen
// capture into the project sandbox, mouse/keyboard input via CoreGraphics,
// and a window list for targeting.
//
// macOS requires two TCC permissions: Accessibility (input events) and
// Screen Recording (screenshots, window titles). Manager surfaces both so
// tools can fail with actionable errors.
package computeruse

import (
	"errors"
	"fmt"
	"image"
	"os"
	"strings"
)

// Errors returned when macOS permissions are missing.
var (
	ErrAccessibilityDenied = errors.New("computer use: Accessibility permission not granted — enable ForgeADE under System Settings → Privacy & Security → Accessibility")
	ErrScreenCaptureDenied = errors.New("computer use: Screen Recording permission not granted — enable ForgeADE under System Settings → Privacy & Security → Screen Recording")
	ErrUnsupportedOS       = errors.New("computer use: only supported on macOS")
)

// Manager provides the desktop-control operations exposed as agent tools.
// It is stateless; permission checks run per call so a user granting the
// TCC prompt takes effect immediately.
type Manager struct{}

// NewManager returns a computer-use manager.
func NewManager() *Manager { return &Manager{} }

// PermissionStatus is the live macOS TCC state for the two permissions the
// computer-use tools need.
type PermissionStatus struct {
	Accessibility bool `json:"accessibility"` // input events (click/type/key/scroll)
	ScreenCapture bool `json:"screenCapture"` // screenshots + window titles
}

// SettingsInfo is everything the Settings card and the agent need to know.
type SettingsInfo struct {
	Supported  bool             `json:"supported"`
	ScreenW    int              `json:"screenW"`
	ScreenH    int              `json:"screenH"`
	Permission PermissionStatus `json:"permission"`
}

// WindowInfo describes one on-screen window.
type WindowInfo struct {
	Title  string `json:"title"`
	App    string `json:"app"`
	PID    int    `json:"pid"`
	X      int    `json:"x"`
	Y      int    `json:"y"`
	Width  int    `json:"w"`
	Height int    `json:"h"`
}

// Settings returns the live permission state and main-display size.
func (m *Manager) Settings() SettingsInfo {
	info := SettingsInfo{}
	info.ScreenW, info.ScreenH = m.ScreenSize()
	info.Permission.Accessibility = m.AXTrusted()
	info.Permission.ScreenCapture = m.ScreenCaptureTrusted()
	if info.ScreenW > 0 {
		info.Supported = true
	}
	return info
}

// pngSize decodes a PNG's pixel dimensions without loading the full image.
func pngSize(path string) (int, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return 0, 0, fmt.Errorf("read screenshot: %w", err)
	}
	return cfg.Width, cfg.Height, nil
}

// ClickSpec is a validated mouse click request.
type ClickSpec struct {
	X, Y   int
	Button string // left | right | middle
	Double bool
}

// ScrollSpec is a validated scroll request.
type ScrollSpec struct {
	X, Y      int
	Direction string // up | down | left | right
	Amount    int    // lines/rows (1-50)
}

// Region is a validated screen region for a partial screenshot.
type Region struct {
	X, Y, W, H int
}

var validButtons = map[string]bool{"left": true, "right": true, "middle": true}
var validDirections = map[string]bool{"up": true, "down": true, "left": true, "right": true}

// ValidateClick normalises a click request; empty button defaults to left.
func ValidateClick(x, y int, button string, double bool) (ClickSpec, error) {
	if button == "" {
		button = "left"
	}
	button = strings.ToLower(button)
	if !validButtons[button] {
		return ClickSpec{}, fmt.Errorf("invalid button %q (left|right|middle)", button)
	}
	if x < 0 || y < 0 {
		return ClickSpec{}, fmt.Errorf("coordinates must be non-negative (got %d,%d)", x, y)
	}
	return ClickSpec{X: x, Y: y, Button: button, Double: double}, nil
}

// ValidateScroll normalises a scroll request; empty direction defaults to
// down and amount is clamped to 1-50.
func ValidateScroll(x, y int, direction string, amount int) (ScrollSpec, error) {
	if direction == "" {
		direction = "down"
	}
	direction = strings.ToLower(direction)
	if !validDirections[direction] {
		return ScrollSpec{}, fmt.Errorf("invalid direction %q (up|down|left|right)", direction)
	}
	if x < 0 || y < 0 {
		return ScrollSpec{}, fmt.Errorf("coordinates must be non-negative (got %d,%d)", x, y)
	}
	if amount == 0 {
		amount = 3
	}
	if amount < 0 {
		amount = -amount
	}
	if amount > 50 {
		amount = 50
	}
	return ScrollSpec{X: x, Y: y, Direction: direction, Amount: amount}, nil
}

// ValidateRegion normalises an optional screenshot region.
func ValidateRegion(x, y, w, h int) (Region, error) {
	if w == 0 && h == 0 {
		return Region{}, nil // full screen
	}
	if w <= 0 || h <= 0 || x < 0 || y < 0 {
		return Region{}, fmt.Errorf("invalid region x=%d y=%d w=%d h=%d", x, y, w, h)
	}
	return Region{X: x, Y: y, W: w, H: h}, nil
}

// ParseKeyChord splits "cmd+shift+t" into normalized modifier flags and the
// base key name. Modifiers: cmd, ctrl, alt/option, shift. The base key is
// lowercased; unknown keys are returned as-is and resolved by the caller
// (single characters fall back to unicode text input).
func ParseKeyChord(chord string) (mods []string, base string, err error) {
	chord = strings.TrimSpace(chord)
	if chord == "" {
		return nil, "", fmt.Errorf("empty key chord")
	}
	parts := strings.Split(chord, "+")
	for _, p := range parts[:len(parts)-1] {
		switch strings.ToLower(strings.TrimSpace(p)) {
		case "cmd", "command", "meta":
			mods = append(mods, "cmd")
		case "ctrl", "control":
			mods = append(mods, "ctrl")
		case "alt", "option", "opt":
			mods = append(mods, "alt")
		case "shift":
			mods = append(mods, "shift")
		default:
			return nil, "", fmt.Errorf("unknown modifier %q in %q", p, chord)
		}
	}
	base = strings.ToLower(strings.TrimSpace(parts[len(parts)-1]))
	if base == "" {
		return nil, "", fmt.Errorf("missing base key in %q", chord)
	}
	return mods, base, nil
}
