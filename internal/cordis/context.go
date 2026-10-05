package cordis

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Context is one plugin instance's view of the runtime. Every registration
// method records an effect owned by the fiber, so unloading the fiber unwinds
// everything the plugin contributed — listeners, child plugins, services.
type Context struct {
	mu    sync.Mutex
	fiber *Fiber
	root  *Runtime
	// scoped service realm: groups with isolate get their own Context realm.
	realm string
}

// Runtime is the shared application runtime: service registry, event bus,
// and the fiber registry.
type Runtime struct {
	mu sync.Mutex

	services map[string]*serviceEntry // realm → per-realm map below
	byRealm  map[string]map[string]*serviceEntry

	listeners map[string][]*listener

	fibers map[string]*Fiber // by plugin id (root-level registry view)

	entryTools map[string][]ToolDef // fiber id → declarative tools

	pendMu  sync.Mutex
	pending map[string]pendingEntry

	nextOrdinal int
}

type serviceEntry struct {
	name    string
	ownerID string // owning fiber id
	value   any
}

type listener struct {
	ownerID string
	fn      func(payload any) any // return value used by waterfall dispatch
}

// NewRuntime creates a fresh Cordis runtime.
func NewRuntime() *Runtime {
	return &Runtime{
		byRealm:    map[string]map[string]*serviceEntry{"": {}},
		listeners:  map[string][]*listener{},
		fibers:     map[string]*Fiber{},
		pending:    map[string]pendingEntry{},
		entryTools: map[string][]ToolDef{},
	}
}

// ---------------------------------------------------------------------------
// Effects
// ---------------------------------------------------------------------------

// Effect registers a resource with a disposer. The body runs immediately; the
// disposer runs when the owning plugin unloads. Disposers unwind in reverse
// registration order.
func (c *Context) Effect(acquire func() (any, Disposer), use func(any) error) error {
	if acquire == nil {
		return nil
	}
	_, disposer := acquire()
	if disposer == nil {
		disposer = func() error { return nil }
	}
	c.mu.Lock()
	c.fiber.effects = append(c.fiber.effects, disposer)
	c.mu.Unlock()
	if use != nil {
		return use(nil)
	}
	return nil
}

// OnDispose registers a bare disposer to run at unload.
func (c *Context) OnDispose(disposer Disposer) {
	if disposer == nil {
		disposer = func() error { return nil }
	}
	c.mu.Lock()
	c.fiber.effects = append(c.fiber.effects, disposer)
	c.mu.Unlock()
}

// ---------------------------------------------------------------------------
// Services
// ---------------------------------------------------------------------------

// Provide publishes a named service in this context's realm. Registration is
// an effect: unloading the providing fiber removes the service and unloads
// every fiber that injected it.
func (c *Context) Provide(name string, value any) {
	c.root.mu.Lock()
	realm := c.root.realmMap(c.realm)
	realm[name] = &serviceEntry{name: name, ownerID: c.fiber.ID, value: value}
	c.fiber.mu.Lock()
	c.fiber.provided = append(c.fiber.provided, name)
	c.fiber.mu.Unlock()
	c.root.mu.Unlock()
	// Resolve any PENDING fibers that were waiting on this service. Runs
	// without r.mu: activation (apply) may call back into the runtime.
	c.root.resolvePending()
}

func (r *Runtime) realmMap(realm string) map[string]*serviceEntry {
	m, ok := r.byRealm[realm]
	if !ok {
		m = map[string]*serviceEntry{}
		r.byRealm[realm] = m
	}
	return m
}

// Get returns a service by name, or nil when no provider exists in this
// realm. Optional dependencies probe here instead of declaring inject.
func (c *Context) Get(name string) (any, bool) {
	c.root.mu.Lock()
	defer c.root.mu.Unlock()
	entry, ok := c.root.byRealm[c.realm][name]
	if !ok || entry == nil {
		return nil, false
	}
	return entry.value, true
}

// ---------------------------------------------------------------------------
// Events
// ---------------------------------------------------------------------------

// On subscribes to an event. The listener is removed when the plugin unloads.
// A listener may return a replacement payload; when any listener returns a
// non-nil value, dispatch stops and that value becomes the waterfall result
// (short-circuit).
func (c *Context) On(event string, fn func(payload any) any) {
	c.root.mu.Lock()
	c.root.listeners[event] = append(c.root.listeners[event], &listener{ownerID: c.fiber.ID, fn: fn})
	c.root.mu.Unlock()
	c.OnDispose(func() error {
		c.root.mu.Lock()
		defer c.root.mu.Unlock()
		kept := c.root.listeners[event][:0]
		for _, l := range c.root.listeners[event] {
			if l.ownerID != c.fiber.ID {
				kept = append(kept, l)
			}
		}
		c.root.listeners[event] = kept
		return nil
	})
}

// DispatchResult carries the waterfall outcome of an event dispatch.
type DispatchResult struct {
	// Payload is the (possibly replaced) final payload.
	Payload any
	// ShortCircuited is true when a listener returned a non-nil replacement
	// and later listeners were skipped.
	ShortCircuited bool
}

// Emit broadcasts an event to listeners in registration order. Any listener
// may return a non-nil value to short-circuit: dispatch stops and that value
// becomes the result.
func (c *Context) Emit(event string, payload any) DispatchResult {
	c.root.mu.Lock()
	ls := make([]*listener, len(c.root.listeners[event]))
	copy(ls, c.root.listeners[event])
	c.root.mu.Unlock()

	for _, l := range ls {
		if out := l.fn(payload); out != nil {
			return DispatchResult{Payload: out, ShortCircuited: true}
		}
	}
	return DispatchResult{Payload: payload}
}

// ---------------------------------------------------------------------------
// Registry enumeration (diagnostics)
// ---------------------------------------------------------------------------

// FiberInfo is a serializable fiber snapshot for diagnostics and the UI.
type FiberInfo struct {
	ID       string   `json:"id"`
	State    string   `json:"state"`
	Source   string   `json:"source,omitempty"`
	Error    string   `json:"error,omitempty"`
	Inject   []string `json:"inject,omitempty"`
	Provided []string `json:"provided,omitempty"`
}

// Fibers returns snapshots for all fibers in the runtime, sorted by id.
func (r *Runtime) Fibers() []FiberInfo {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]FiberInfo, 0, len(r.fibers))
	for _, f := range r.fibers {
		f.mu.Lock()
		info := FiberInfo{ID: f.ID, State: f.State.String(), Source: f.source, Error: ""}
		if f.Error != nil {
			info.Error = f.Error.Error()
		}
		prov := append([]string(nil), f.provided...)
		inject := append([]string(nil), f.def.Inject...)
		f.mu.Unlock()
		info.Provided = prov
		info.Inject = inject
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// PendingNames lists fibers stuck in PENDING with the missing services, the
// chapter-6 diagnostic: "why does my plugin print nothing?"
func (r *Runtime) PendingNames() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []string
	for id, f := range r.fibers {
		f.mu.Lock()
		pending := f.State == StatePending
		f.mu.Unlock()
		if pending {
			out = append(out, fmt.Sprintf("%s is PENDING — a required service is missing", id))
		}
	}
	sort.Strings(out)
	_ = strings.TrimSpace
	return out
}
