//go:build !darwin

package computeruse

import "errors"

// AXTrusted is always false on non-macOS builds.
func (m *Manager) AXTrusted() bool { return false }

// ScreenCaptureTrusted is always false on non-macOS builds.
func (m *Manager) ScreenCaptureTrusted() bool { return false }

// RequestScreenCapture is unsupported on non-macOS builds.
func (m *Manager) RequestScreenCapture() bool { return false }

// ScreenSize is unsupported on non-macOS builds.
func (m *Manager) ScreenSize() (w, h int) { return 0, 0 }

// Click is unsupported on non-macOS builds.
func (m *Manager) Click(spec ClickSpec) error { return ErrUnsupportedOS }

// TypeText is unsupported on non-macOS builds.
func (m *Manager) TypeText(text string) error { return ErrUnsupportedOS }

// Key is unsupported on non-macOS builds.
func (m *Manager) Key(chord string) error { return ErrUnsupportedOS }

// Scroll is unsupported on non-macOS builds.
func (m *Manager) Scroll(spec ScrollSpec) error { return ErrUnsupportedOS }

// WindowList is unsupported on non-macOS builds.
func (m *Manager) WindowList() ([]WindowInfo, error) { return nil, ErrUnsupportedOS }

// Screenshot is unsupported on non-macOS builds.
func (m *Manager) Screenshot(projectPath string, region Region) (string, int, int, error) {
	return "", 0, 0, ErrUnsupportedOS
}

var errStub = errors.New("unreachable")
var _ = errStub
