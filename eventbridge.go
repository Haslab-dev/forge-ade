package main

import (
	"os"
	"sync/atomic"

	"github.com/hasdev/forge-ade/internal/events"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// ---------------------------------------------------------------------------
// Event Bus for Frontend
// ---------------------------------------------------------------------------

// emitEvent forwards an event to the frontend via the Wails v3 event system.
func (a *App) emitEvent(name string, data interface{}) {
	if app := application.Get(); app != nil {
		app.Event.Emit(name, data)
	}
}

// ---------------------------------------------------------------------------
// File Sync — used by frontend to detect external file changes
// ---------------------------------------------------------------------------

// gitDirtyFlag is set by file-system events and swept lazily: the next git
// status consumer clears the whole status cache once (instead of per-event),
// so bursts of file changes (builds, installs) never re-run `git status`
// per event — only at most once per repo per TTL window, and only when
// something actually reads status.
var gitDirtyFlag int64

// sweepGitDirty invalidates the git status cache if any file event happened
// since the last sweep. Call at the top of every binding that consumes git
// status so reads always see fresh data without any background polling.
func (a *App) sweepGitDirty() {
	if atomic.SwapInt64(&gitDirtyFlag, 0) == 1 {
		a.gitEngine.InvalidateAll()
	}
}

func (a *App) setupEventHandlers() {
	a.bus.Subscribe(events.FileCreated, func(e events.Event) {
		path, _ := e.Data["path"].(string)
		if path != "" {
			a.searchMgr.IndexFile(path)
			atomic.StoreInt64(&gitDirtyFlag, 1)
			if a.ctx != nil {
				a.emitEvent("fs:changed", map[string]interface{}{
					"type": "created",
					"path": path,
				})
			}
		}
	})
	a.bus.Subscribe(events.FileChanged, func(e events.Event) {
		path, _ := e.Data["path"].(string)
		if path != "" {
			a.searchMgr.RemoveFile(path)
			a.searchMgr.IndexFile(path)
			atomic.StoreInt64(&gitDirtyFlag, 1)
			if a.ctx != nil {
				a.emitEvent("fs:changed", map[string]interface{}{
					"type": "modified",
					"path": path,
				})
			}
		}
	})
	a.bus.Subscribe(events.FileDeleted, func(e events.Event) {
		path, _ := e.Data["path"].(string)
		if path != "" {
			a.searchMgr.RemoveFile(path)
			atomic.StoreInt64(&gitDirtyFlag, 1)
			if a.ctx != nil {
				a.emitEvent("fs:changed", map[string]interface{}{
					"type": "deleted",
					"path": path,
				})
			}
		}
	})
	a.bus.Subscribe(events.FileRenamed, func(e events.Event) {
		path, _ := e.Data["path"].(string)
		oldPath, _ := e.Data["oldPath"].(string)
		if path == "" {
			return
		}
		atomic.StoreInt64(&gitDirtyFlag, 1)
		if oldPath != "" {
			// Paired rename (old + new known): tell the frontend to follow the
			// tab, and keep the search index in sync.
			a.searchMgr.RemoveFile(oldPath)
			a.searchMgr.IndexFile(path)
			if a.ctx != nil {
				a.emitEvent("fs:changed", map[string]interface{}{
					"type":    "renamed",
					"path":    path,
					"oldPath": oldPath,
				})
			}
			return
		}
		// Unpaired rename → classify by whether the path still exists.
		a.searchMgr.IndexFile(path)
		typ := "created"
		if _, err := os.Stat(path); err != nil {
			typ = "deleted"
			a.searchMgr.RemoveFile(path)
		}
		if a.ctx != nil {
			a.emitEvent("fs:changed", map[string]interface{}{
				"type": typ,
				"path": path,
			})
		}
	})

	// Bridge agent updates to frontend. All granular agent events
	// (turn/message/thinking/tool) are forwarded verbatim so the chat can
	// stream deltas instead of polling the whole session list.
	agentEvents := []events.EventType{
		"agent:updated",
		"agent:started",
		"agent:stopped",
		"agent:turn_start",
		"agent:turn_end",
		"agent:message_start",
		"agent:message_delta",
		"agent:message_end",
		"agent:thinking_start",
		"agent:thinking_delta",
		"agent:thinking_end",
		"agent:tool_start",
		"agent:tool_delta",
		"agent:tool_end",
		"agent:ask",
		"agent:task_update",
	}
	for _, evType := range agentEvents {
		evType := evType
		a.bus.Subscribe(evType, func(e events.Event) {
			if a.ctx != nil {
				a.emitEvent(string(evType), e.Data)
			}
		})
	}

	// Browser Use: live screencast frames + status for the Browser viewer.
	a.bus.Subscribe("browser:frame", func(e events.Event) {
		if a.ctx != nil {
			a.emitEvent("browser:frame", e.Data)
		}
	})
	a.bus.Subscribe("browser:status", func(e events.Event) {
		if a.ctx != nil {
			a.emitEvent("browser:status", e.Data)
		}
	})
	a.bus.Subscribe("agent:config:changed", func(e events.Event) {
		if a.ctx != nil {
			a.emitEvent("agent:config:changed", e.Data)
		}
	})

	// Bridge terminal output to frontend via Wails runtime events
	a.bus.Subscribe(events.TerminalOutput, func(e events.Event) {
		if a.ctx != nil {
			a.emitEvent("session:output", e.Data)
		}
	})
	a.bus.Subscribe(events.TerminalOpened, func(e events.Event) {
		if a.ctx != nil {
			a.emitEvent("session:opened", e.Data)
		}
	})
	a.bus.Subscribe(events.TerminalClosed, func(e events.Event) {
		if a.ctx != nil {
			a.emitEvent("session:closed", e.Data)
		}
	})

	// Terminal Session mode: session metadata changes (status, exit code) and
	// deletions. Raw output already flows through session:output above.
	a.bus.Subscribe(events.AgentSessionUpdated, func(e events.Event) {
		if a.ctx != nil {
			a.emitEvent("agentsession:updated", e.Data)
		}
	})
	a.bus.Subscribe(events.AgentSessionDeleted, func(e events.Event) {
		if a.ctx != nil {
			a.emitEvent("agentsession:deleted", e.Data)
		}
	})
}
