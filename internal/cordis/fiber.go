// Package cordis is a Go port of the Cordis plugin runtime model: plugins as
// mountable units with fibers (state machines), effect-scoped cleanup,
// named services resolved by injection, typed events, declarative config from
// cordis.yml, and hot reload by unload-then-load.
package cordis

import (
	"fmt"
	"sync"
)

// FiberState mirrors the Cordis fiber state machine.
type FiberState int

const (
	StatePending FiberState = iota
	StateLoading
	StateActive
	StateUnloading
	StateDisposed
	StateFailed
)

func (s FiberState) String() string {
	switch s {
	case StatePending:
		return "PENDING"
	case StateLoading:
		return "LOADING"
	case StateActive:
		return "ACTIVE"
	case StateUnloading:
		return "UNLOADING"
	case StateDisposed:
		return "DISPOSED"
	case StateFailed:
		return "FAILED"
	}
	return "UNKNOWN"
}

// Disposer releases a resource acquired by an effect. Returning an error
// records a teardown failure without aborting sibling disposers.
type Disposer func() error

// PluginDef is the declarative unit a composition mounts: an id, the services
// it requires (inject), and apply — the function that registers contributions
// on a Context. Apply runs once per mount; everything it registers through
// the Context is unwound on unload.
type PluginDef struct {
	ID     string
	Inject []string
	// Apply receives a ready Context. Return an error to fail the fiber.
	Apply func(ctx *Context) error
	// Source records where the plugin came from (a cordis.yml path, "builtin",
	// a parent plugin id) for diagnostics.
	Source string
}

// Fiber is the runtime handle for one loaded plugin instance.
type Fiber struct {
	mu     sync.Mutex
	ID     string
	State  FiberState
	Error  error
	source string

	parent   *Context
	children []*Fiber
	effects  []Disposer

	// services this fiber provided (unregistered on unload)
	provided []string

	// declarative definition and mount realm (pending re-activation)
	def      PluginDef
	defRealm string
	// services the fiber waits on while PENDING
	pendingFor []string
}

func (f *Fiber) Source() string { return f.source }

// setState sets the state; caller must hold f.mu.
func (f *Fiber) setState(s FiberState) {
	f.State = s
}

// SetState is the locking public setter.
func (f *Fiber) SetState(s FiberState) {
	f.mu.Lock()
	f.State = s
	f.mu.Unlock()
}

func (f *Fiber) fail(err error) {
	f.mu.Lock()
	f.Error = err
	f.State = StateFailed
	f.mu.Unlock()
}

var _ = fmt.Sprintf // keep fmt for future diagnostics
var _ = sync.Mutex{}
