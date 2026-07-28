package events

import (
	"sync"
)

// EventType represents a named event in the system.
type EventType string

const (
	// Workspace events
	WorkspaceOpened  EventType = "workspace:opened"
	WorkspaceClosed  EventType = "workspace:closed"
	WorkspaceSaved   EventType = "workspace:saved"
	WorkspaceUpdated EventType = "workspace:updated"

	// File/Folder events
	FileCreated  EventType = "file:created"
	FileChanged  EventType = "file:changed"
	FileDeleted  EventType = "file:deleted"
	FileRenamed  EventType = "file:renamed"
	FolderAdded  EventType = "folder:added"
	FolderRemoved EventType = "folder:removed"

	// Editor events
	EditorOpened   EventType = "editor:opened"
	EditorClosed   EventType = "editor:closed"
	EditorModified EventType = "editor:modified"
	EditorSaved    EventType = "editor:saved"
	EditorCursor   EventType = "editor:cursor"

	// Git events
	GitCommit       EventType = "git:commit"
	GitBranchChange EventType = "git:branch:change"
	GitUpdated      EventType = "git:updated"
	GitFetch        EventType = "git:fetch"
	GitPush         EventType = "git:push"
	GitPull         EventType = "git:pull"

	// Terminal events
	TerminalOpened  EventType = "terminal:opened"
	TerminalClosed  EventType = "terminal:closed"
	TerminalOutput  EventType = "terminal:output"
	TerminalResized EventType = "terminal:resized"

	// AI/Agent events
	AgentStarted EventType = "agent:started"
	AgentStopped EventType = "agent:stopped"
	AgentOutput  EventType = "agent:output"
	AgentRefresh EventType = "agent:refresh"

	// Search events
	SearchIndexUpdated EventType = "search:index:updated"
	SearchResults      EventType = "search:results"

	// Layout events
	LayoutChanged EventType = "layout:changed"
	PanelToggled  EventType = "panel:toggled"

	// Plugin events
	PluginLoaded   EventType = "plugin:loaded"
	PluginUnloaded EventType = "plugin:unloaded"

	// Diagnostics
	DiagnosticsUpdated EventType = "diagnostics:updated"
)

// Event is the envelope for all events in the system.
type Event struct {
	Type EventType
	Data map[string]interface{}
}

// Handler is a callback for an event.
type Handler func(Event)

// Bus is the central event bus for inter-module communication.
type Bus struct {
	mu       sync.RWMutex
	handlers map[EventType][]Handler
}

// NewBus creates a new event bus.
func NewBus() *Bus {
	return &Bus{
		handlers: make(map[EventType][]Handler),
	}
}

// Subscribe registers a handler for a given event type.
func (b *Bus) Subscribe(event EventType, handler Handler) func() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.handlers[event] = append(b.handlers[event], handler)

	// Return a function to unsubscribe
	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		handlers := b.handlers[event]
		for i, h := range handlers {
			if &h == &handler {
				b.handlers[event] = append(handlers[:i], handlers[i+1:]...)
				break
			}
		}
	}
}

// Publish sends an event to all registered handlers asynchronously.
func (b *Bus) Publish(event Event) {
	b.mu.RLock()
	handlers := make([]Handler, len(b.handlers[event.Type]))
	copy(handlers, b.handlers[event.Type])
	b.mu.RUnlock()

	for _, handler := range handlers {
		handler(event)
	}
}

// PublishSync sends an event to all registered handlers synchronously (in order).
func (b *Bus) PublishSync(event Event) {
	b.mu.RLock()
	handlers := make([]Handler, len(b.handlers[event.Type]))
	copy(handlers, b.handlers[event.Type])
	b.mu.RUnlock()

	for _, handler := range handlers {
		handler(event)
	}
}
