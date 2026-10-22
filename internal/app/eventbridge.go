package app

import (
	"os"
	"sync/atomic"

	"github.com/Haslab-dev/RakitKode/events"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// ---------------------------------------------------------------------------
// Event bridge — internal event bus → Wails frontend events
// ---------------------------------------------------------------------------

// Wails frontend event name constants
const (
	EventFSChanged           = "fs:changed"
	EventBrowserFrame        = "browser:frame"
	EventBrowserStatus       = "browser:status"
	EventAgentConfigChanged  = "agent:config:changed"
	EventSessionOutput       = "session:output"
	EventSessionOpened       = "session:opened"
	EventSessionClosed       = "session:closed"
	EventAgentSessionUpdated = "agentsession:updated"
	EventAgentSessionDeleted = "agentsession:deleted"
	EventSSHHandshake        = "ssh:handshake"
	EventSSHConnected        = "ssh:connected"
	EventSSHDisconnected     = "ssh:disconnected"
	EventSSHError            = "ssh:error"
)

// FSChangeType represents file system mutation type.
type FSChangeType string

const (
	FSChangeCreated  FSChangeType = "created"
	FSChangeModified FSChangeType = "modified"
	FSChangeDeleted  FSChangeType = "deleted"
	FSChangeRenamed  FSChangeType = "renamed"
)

// FSChangeEvent is the typed payload for "fs:changed" Wails event.
type FSChangeEvent struct {
	Type    FSChangeType `json:"type"`
	Path    string       `json:"path"`
	OldPath string       `json:"oldPath,omitempty"`
}

// emitEvent forwards an event to the frontend via the Wails v3 event system.
// Safe to call at any time: before the app exists (application.Get() == nil)
// it is a no-op, so handlers never need their own guards.
func (a *App) emitEvent(name string, data interface{}) {
	if app := application.Get(); app != nil {
		app.Event.Emit(name, data)
	}
}

// agentEventTypes are the granular agent updates (turn/message/thinking/tool)
// forwarded verbatim to the frontend so the chat can stream deltas instead of
// polling the whole session list.
var agentEventTypes = []events.EventType{
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
	"agent:schedule_update",
	"agent:available_commands",
}

// forwardedEvents map internal bus constants to Wails event names 1:1.
var forwardedEvents = []struct {
	bus  events.EventType
	wail string
}{
	{bus: "browser:frame", wail: EventBrowserFrame},
	{bus: "browser:status", wail: EventBrowserStatus},
	{bus: "agent:config:changed", wail: EventAgentConfigChanged},
	{bus: events.TerminalOutput, wail: EventSessionOutput},
	{bus: events.TerminalOpened, wail: EventSessionOpened},
	{bus: events.TerminalClosed, wail: EventSessionClosed},
	{bus: events.AgentSessionUpdated, wail: EventAgentSessionUpdated},
	{bus: events.AgentSessionDeleted, wail: EventAgentSessionDeleted},
	{bus: events.SSHHandshake, wail: EventSSHHandshake},
	{bus: events.SSHConnected, wail: EventSSHConnected},
	{bus: events.SSHDisconnected, wail: EventSSHDisconnected},
	{bus: events.SSHError, wail: EventSSHError},
}

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
	// File sync: keep the search index fresh, flag git dirtiness, and tell
	// the frontend about external changes so open tabs/editors can follow.
	a.bus.Subscribe(events.FileCreated, func(e events.Event) {
		if path, _ := e.Data["path"].(string); path != "" {
			a.searchMgr.IndexFile(path)
			atomic.StoreInt64(&gitDirtyFlag, 1)
			a.emitEvent(EventFSChanged, FSChangeEvent{Type: FSChangeCreated, Path: path})
		}
	})
	a.bus.Subscribe(events.FileChanged, func(e events.Event) {
		if path, _ := e.Data["path"].(string); path != "" {
			a.searchMgr.RemoveFile(path)
			a.searchMgr.IndexFile(path)
			atomic.StoreInt64(&gitDirtyFlag, 1)
			a.emitEvent(EventFSChanged, FSChangeEvent{Type: FSChangeModified, Path: path})
		}
	})
	a.bus.Subscribe(events.FileDeleted, func(e events.Event) {
		if path, _ := e.Data["path"].(string); path != "" {
			a.searchMgr.RemoveFile(path)
			atomic.StoreInt64(&gitDirtyFlag, 1)
			a.emitEvent(EventFSChanged, FSChangeEvent{Type: FSChangeDeleted, Path: path})
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
			a.emitEvent(EventFSChanged, FSChangeEvent{Type: FSChangeRenamed, Path: path, OldPath: oldPath})
			return
		}
		// Unpaired rename → classify by whether the path still exists.
		a.searchMgr.IndexFile(path)
		typ := FSChangeCreated
		if _, err := os.Stat(path); err != nil {
			typ = FSChangeDeleted
			a.searchMgr.RemoveFile(path)
		}
		a.emitEvent(EventFSChanged, FSChangeEvent{Type: typ, Path: path})
	})

	for _, evType := range agentEventTypes {
		evType := evType
		a.bus.Subscribe(evType, func(e events.Event) {
			a.emitEvent(string(evType), e.Data)
		})
	}

	for _, fwd := range forwardedEvents {
		fwd := fwd
		a.bus.Subscribe(fwd.bus, func(e events.Event) {
			a.emitEvent(fwd.wail, e.Data)
		})
	}
}
