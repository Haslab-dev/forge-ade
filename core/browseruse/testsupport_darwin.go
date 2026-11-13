//go:build darwin && forge_iabtest

package browseruse

// Test-only support: creates a real NSWindow so the in-app engine's
// end-to-end test (iab_darwin_test.go) can host its WKWebView without the
// Wails main window. Built only under `-tags forge_iabtest`.

/*
#cgo darwin CFLAGS: -x objective-c -fobjc-arc
#cgo darwin LDFLAGS: -framework Cocoa

#import <Cocoa/Cocoa.h>

static void *test_make_window_impl(void) {
	[NSApplication sharedApplication];
	NSWindow *win = [[NSWindow alloc] initWithContentRect:NSMakeRect(100, 100, 1200, 800)
		styleMask:NSWindowStyleMaskTitled | NSWindowStyleMaskResizable
		backing:NSBackingStoreBuffered
		defer:NO];
	[win setTitle:@"forge-ade iab test"];
	[win makeKeyAndOrderFront:nil];
	return (__bridge_retained void *)win;
}

static void *test_make_window(void) {
	if ([NSThread isMainThread]) return test_make_window_impl();
	__block void *out = NULL;
	dispatch_sync(dispatch_get_main_queue(), ^{ out = test_make_window_impl(); });
	return out;
}
*/
import "C"

import (
	"runtime"
	"unsafe"
)

func makeTestWindow() unsafe.Pointer {
	return C.test_make_window()
}

// runLoopForever pumps the Cocoa main runloop on the calling (locked) OS
// thread and blocks forever — lets WKWebView completion handlers fire while
// tests run on other goroutines.
func runLoopForever() {
	runtime.LockOSThread()
	C.CFRunLoopRun()
}
