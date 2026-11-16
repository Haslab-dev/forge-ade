//go:build e2e

package main

// The automation plugin lets agents and tests drive the app's native UI
// over a local socket (MYGO_AUTOMATION). Release builds leave it out.
import _ "github.com/egoist/mygo/plugins/automation"
