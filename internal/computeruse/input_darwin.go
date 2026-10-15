//go:build darwin

package computeruse

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework CoreGraphics -framework ApplicationServices -framework AppKit

#include <CoreGraphics/CoreGraphics.h>
#include <ApplicationServices/ApplicationServices.h>
#include <stdlib.h>

typedef struct {
	char title[256];
	char app[256];
	int pid;
	int x, y, w, h;
} CUWindowInfo;

static int cu_ax_trusted(void) {
	return AXIsProcessTrusted() ? 1 : 0;
}

static int cu_screen_capture_prechecked(void) {
	return CGPreflightScreenCaptureAccess() ? 1 : 0;
}

static int cu_request_screen_capture(void) {
	return CGRequestScreenCaptureAccess() ? 1 : 0;
}

static void cu_screen_size(int* w, int* h) {
	CGDirectDisplayID d = CGMainDisplayID();
	*w = (int)CGDisplayPixelsWide(d);
	*h = (int)CGDisplayPixelsHigh(d);
}

// cu_post_mouse posts one mouse event; kind is a CGEventType, button a
// kCGMouseButton value, clicks the click-state (2 = double).
static void cu_post_mouse(int kind, double x, double y, int button, int clicks) {
	CGPoint p = CGPointMake(x, y);
	CGEventRef e = CGEventCreateMouseEvent(NULL, (CGEventType)kind, p, (CGMouseButton)button);
	if (!e) return;
	if (clicks > 1) CGEventSetIntegerValueField(e, kCGMouseEventClickState, clicks);
	CGEventPost(kCGHIDEventTap, e);
	CFRelease(e);
}

// cu_post_key posts a raw virtual-key press or release with modifier flags.
static void cu_post_key(int down, unsigned short vk, unsigned long flags) {
	CGEventRef e = CGEventCreateKeyboardEvent(NULL, vk, down ? true : false);
	if (!e) return;
	if (flags) CGEventSetFlags(e, (CGEventFlags)flags);
	CGEventPost(kCGHIDEventTap, e);
	CFRelease(e);
}

// cu_post_text types a UTF-8 string via the keyboard unicode payload.
static void cu_post_text(const char* s) {
	CFStringRef cf = CFStringCreateWithCString(NULL, s, kCFStringEncodingUTF8);
	if (!cf) return;
	CFIndex n = CFStringGetLength(cf);
	if (n <= 0) { CFRelease(cf); return; }
	UniChar* chars = (UniChar*)malloc((size_t)n * sizeof(UniChar));
	if (!chars) { CFRelease(cf); return; }
	CFStringGetCharacters(cf, CFRangeMake(0, n), chars);

	CGEventRef down = CGEventCreateKeyboardEvent(NULL, 0, true);
	CGEventRef up = CGEventCreateKeyboardEvent(NULL, 0, false);
	if (down && up) {
		CGEventKeyboardSetUnicodeString(down, n, chars);
		CGEventKeyboardSetUnicodeString(up, n, chars);
		CGEventPost(kCGHIDEventTap, down);
		CGEventPost(kCGHIDEventTap, up);
	}
	if (down) CFRelease(down);
	if (up) CFRelease(up);
	free(chars);
	CFRelease(cf);
}

// cu_scroll moves the pointer to (x,y) then posts a line-unit scroll with
// two wheels (vertical, horizontal). Positive vertical scrolls up; positive
// horizontal scrolls right.
static void cu_scroll(double x, double y, int linesV, int linesH) {
	CGPoint p = CGPointMake(x, y);
	CGEventRef mv = CGEventCreateMouseEvent(NULL, kCGEventMouseMoved, p, kCGMouseButtonLeft);
	if (mv) { CGEventPost(kCGHIDEventTap, mv); CFRelease(mv); }
	CGEventRef e = CGEventCreateScrollWheelEvent2(
		NULL, kCGScrollEventUnitLine, 2, linesV, linesH, 0);
	if (!e) return;
	CGEventPost(kCGHIDEventTap, e);
	CFRelease(e);
}

// cu_window_list fills out (capacity max) with on-screen normal-layer
// windows. Returns the number filled. Window titles require Screen
// Recording; without it title is empty but bounds/pid still work.
static int cu_window_list(CUWindowInfo* out, int max) {
	CFArrayRef list = CGWindowListCopyWindowInfo(
		kCGWindowListOptionOnScreenOnly | kCGWindowListExcludeDesktopElements,
		kCGNullWindowID);
	if (!list) return 0;
	CFIndex n = CFArrayGetCount(list);
	int count = 0;
	for (CFIndex i = 0; i < n && count < max; i++) {
		CFDictionaryRef d = (CFDictionaryRef)CFArrayGetValueAtIndex(list, i);
		if (!d) continue;

		CFNumberRef layerRef = (CFNumberRef)CFDictionaryGetValue(d, kCGWindowLayer);
		long layer = -1;
		if (layerRef) CFNumberGetValue(layerRef, kCFNumberLongType, &layer);
		if (layer != 0) continue; // normal windows only, skip menu/dock overlays

		CFDictionaryRef bRef = (CFDictionaryRef)CFDictionaryGetValue(d, kCGWindowBounds);
		CGRect b;
		if (!bRef || !CGRectMakeWithDictionaryRepresentation(bRef, &b)) continue;
		if (b.size.width < 40 || b.size.height < 40) continue; // skip specks

		CUWindowInfo* info = &out[count];
		memset(info, 0, sizeof(*info));
		info->x = (int)b.origin.x;
		info->y = (int)b.origin.y;
		info->w = (int)b.size.width;
		info->h = (int)b.size.height;

		CFNumberRef pidRef = (CFNumberRef)CFDictionaryGetValue(d, kCGWindowOwnerPID);
		if (pidRef) CFNumberGetValue(pidRef, kCFNumberIntType, &info->pid);

		CFStringRef name = (CFStringRef)CFDictionaryGetValue(d, kCGWindowName);
		if (name) CFStringGetCString(name, info->title, sizeof(info->title), kCFStringEncodingUTF8);

		CFStringRef owner = (CFStringRef)CFDictionaryGetValue(d, kCGWindowOwnerName);
		if (owner) CFStringGetCString(owner, info->app, sizeof(info->app), kCFStringEncodingUTF8);

		count++;
	}
	CFRelease(list);
	return count;
}
*/
import "C"

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/hasdev/forge-ade/internal/sandbox"
)

// AXTrusted reports whether the app may post input events (Accessibility).
func (m *Manager) AXTrusted() bool { return C.cu_ax_trusted() != 0 }

// ScreenCaptureTrusted reports whether the app may capture the screen.
func (m *Manager) ScreenCaptureTrusted() bool { return C.cu_screen_capture_prechecked() != 0 }

// RequestScreenCapture triggers the macOS Screen Recording consent flow.
func (m *Manager) RequestScreenCapture() bool { return C.cu_request_screen_capture() != 0 }

// ScreenSize returns the main display dimensions in points.
func (m *Manager) ScreenSize() (w, h int) {
	var cw, ch C.int
	C.cu_screen_size(&cw, &ch)
	return int(cw), int(ch)
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

func (m *Manager) requireAX() error {
	if !m.AXTrusted() {
		return ErrAccessibilityDenied
	}
	return nil
}

// Click posts a mouse click at (x, y).
func (m *Manager) Click(spec ClickSpec) error {
	if err := m.requireAX(); err != nil {
		return err
	}
	p := C.double(spec.X)
	py := C.double(spec.Y)
	kindDown, kindUp, btn := mouseConstants(spec.Button)
	clicks := C.int(1)
	if spec.Double {
		clicks = 2
	}
	C.cu_post_mouse(5, p, py, 0, 1) // mouse moved first so hover states settle
	C.cu_post_mouse(kindDown, p, py, btn, clicks)
	time.Sleep(25 * time.Millisecond)
	C.cu_post_mouse(kindUp, p, py, btn, clicks)
	return nil
}

// TypeText types text into the focused control.
func (m *Manager) TypeText(text string) error {
	if err := m.requireAX(); err != nil {
		return err
	}
	if text == "" {
		return nil
	}
	for _, line := range splitKeepNewlines(text) {
		if line.text != "" {
			C.cu_post_text(C.CString(line.text))
		}
		if line.newline {
			C.cu_post_key(1, 0x24, 0) // Return
			C.cu_post_key(0, 0x24, 0)
			time.Sleep(15 * time.Millisecond)
		}
	}
	return nil
}

type textLine struct {
	text    string
	newline bool
}

func splitKeepNewlines(s string) []textLine {
	var out []textLine
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, textLine{text: s[start:i], newline: true})
			start = i + 1
		}
	}
	out = append(out, textLine{text: s[start:]})
	return out
}

// Key presses a key or chord ("Return", "cmd+t", "ctrl+shift+esc").
func (m *Manager) Key(chord string) error {
	if err := m.requireAX(); err != nil {
		return err
	}
	mods, base, err := ParseKeyChord(chord)
	if err != nil {
		return err
	}
	vk, ok := keyCodeFor(base)
	if !ok {
		// Single printable characters without modifiers go via the unicode
		// path; anything else with an unknown key is an error.
		if len(mods) == 0 && []rune(base)[0] >= 32 {
			C.cu_post_text(C.CString(base))
			return nil
		}
		return fmt.Errorf("unknown key %q", base)
	}
	var flags C.ulong
	for _, mod := range mods {
		flags |= modifierFlag(mod)
	}
	if flags != 0 { // hold modifiers, tap key, release modifiers
		for _, mod := range mods {
			C.cu_post_key(1, 0, modifierFlag(mod))
		}
		C.cu_post_key(1, C.ushort(vk), flags)
		time.Sleep(15 * time.Millisecond)
		C.cu_post_key(0, C.ushort(vk), flags)
		for _, mod := range mods {
			C.cu_post_key(0, 0, modifierFlag(mod))
		}
		return nil
	}
	C.cu_post_key(1, C.ushort(vk), 0)
	time.Sleep(15 * time.Millisecond)
	C.cu_post_key(0, C.ushort(vk), 0)
	return nil
}

// Scroll scrolls at (x, y).
func (m *Manager) Scroll(spec ScrollSpec) error {
	if err := m.requireAX(); err != nil {
		return err
	}
	var linesV, linesH C.int
	switch spec.Direction {
	case "up":
		linesV = C.int(spec.Amount)
	case "down":
		linesV = C.int(-spec.Amount)
	case "left":
		linesH = C.int(-spec.Amount)
	case "right":
		linesH = C.int(spec.Amount)
	}
	C.cu_scroll(C.double(spec.X), C.double(spec.Y), linesV, linesH)
	return nil
}

// WindowList returns the on-screen normal-layer windows of the main display.
func (m *Manager) WindowList() ([]WindowInfo, error) {
	if !m.ScreenCaptureTrusted() {
		return nil, ErrScreenCaptureDenied
	}
	const max = 64
	out := make([]C.CUWindowInfo, max)
	n := C.cu_window_list(&out[0], max)
	windows := make([]WindowInfo, 0, n)
	for i := 0; i < int(n); i++ {
		r := &out[i]
		windows = append(windows, WindowInfo{
			Title:  C.GoString(&r.title[0]),
			App:    C.GoString(&r.app[0]),
			PID:    int(r.pid),
			X:      int(r.x),
			Y:      int(r.y),
			Width:  int(r.w),
			Height: int(r.h),
		})
	}
	return windows, nil
}

// Screenshot captures the main display (or the given region) to the project
// sandbox tmp/ and returns path, width, height.
func (m *Manager) Screenshot(projectPath string, region Region) (path string, w, h int, err error) {
	if !m.ScreenCaptureTrusted() {
		if m.RequestScreenCapture() {
			return "", 0, 0, ErrScreenCaptureDenied // consent flow started; retry next turn
		}
		return "", 0, 0, ErrScreenCaptureDenied
	}
	dir, err := sandbox.Sub(projectPath, sandbox.DirTmp)
	if err != nil {
		return "", 0, 0, err
	}
	path = filepath.Join(dir, fmt.Sprintf("screen-%d.png", time.Now().UnixMilli()))
	args := []string{"-x"}
	if region.W > 0 {
		args = append(args, "-R", fmt.Sprintf("%d,%d,%d,%d", region.X, region.Y, region.W, region.H))
	}
	args = append(args, path)
	out, err := exec.Command(screencaptureBin(), args...).CombinedOutput()
	if err != nil {
		return "", 0, 0, fmt.Errorf("screencapture: %v: %s", err, string(out))
	}
	w, h, err = pngSize(path)
	if err != nil {
		return "", 0, 0, err
	}
	return path, w, h, nil
}

// screencaptureBin locates the macOS screencapture CLI (moved to /usr/sbin
// on recent macOS releases; older images keep it in /usr/bin).
func screencaptureBin() string {
	if p, err := exec.LookPath("screencapture"); err == nil {
		return p
	}
	for _, c := range []string{"/usr/sbin/screencapture", "/usr/bin/screencapture"} {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return "screencapture"
}

func mouseConstants(button string) (down, up, btn C.int) {
	switch button {
	case "right":
		return 3, 4, 1 // kCGEventRightMouseDown/Up, kCGMouseButtonRight
	case "middle":
		return 25, 26, 2 // kCGEventOtherMouseDown/Up, kCGMouseButtonCenter
	default:
		return 1, 2, 0 // kCGEventLeftMouseDown/Up, kCGMouseButtonLeft
	}
}

func modifierFlag(mod string) C.ulong {
	switch mod {
	case "cmd":
		return C.ulong(1 << 20) // kCGEventFlagMaskCommand
	case "shift":
		return C.ulong(1 << 17) // kCGEventFlagMaskShift
	case "ctrl":
		return C.ulong(1 << 18) // kCGEventFlagMaskControl
	case "alt":
		return C.ulong(1 << 19) // kCGEventFlagMaskAlternate
	}
	return 0
}
