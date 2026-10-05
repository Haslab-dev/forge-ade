package cordis

import (
	"fmt"
	"sync"
)

// Runtime accessors used by the loader and mount logic.

func (c *Context) Fiber() *Fiber     { return c.fiber }
func (c *Context) Runtime() *Runtime { return c.root }
func (c *Context) Realm() string     { return c.realm }

// Mount loads a plugin into this context as a child: unloading the parent
// unloads the child. This is the runtime form of a cordis.yml entry.
func (c *Context) Mount(def PluginDef) (*Fiber, error) {
	return c.root.mount(c, def, c.realm)
}

// mount resolves inject requirements, runs apply, and manages the fiber
// state machine: PENDING (missing service) → LOADING → ACTIVE, or FAILED.
func (r *Runtime) mount(parent *Context, def PluginDef, realm string) (*Fiber, error) {
	if def.ID == "" {
		return nil, fmt.Errorf("cordis: plugin entry missing id")
	}
	r.mu.Lock()
	if _, exists := r.fibers[def.ID]; exists {
		r.mu.Unlock()
		return nil, fmt.Errorf("cordis: plugin %q already mounted", def.ID)
	}
	f := &Fiber{ID: def.ID, source: def.Source, def: def, defRealm: realm}
	r.fibers[def.ID] = f
	r.mu.Unlock()

	missing := r.missingServices(def.Inject, realm)
	if len(missing) > 0 {
		f.mu.Lock()
		f.pendingFor = missing
		f.def = def
		f.defRealm = realm
		f.parent = parent
		f.mu.Unlock()
		// Registered as PENDING; resolvePending will activate when the
		// services appear.
		r.registerPending(def, realm, missing)
		return f, nil
	}

	if err := r.activate(parent, f, def, realm); err != nil {
		return f, err
	}
	return f, nil
}

// activate runs apply on a fresh child Context.
func (r *Runtime) activate(parent *Context, f *Fiber, def PluginDef, realm string) error {
	f.mu.Lock()
	f.setState(StateLoading)
	f.mu.Unlock()
	child := &Context{fiber: f, root: r, realm: realm}
	f.parent = parent

	if def.Apply == nil {
		f.mu.Lock()
		f.setState(StateActive)
		f.mu.Unlock()
		return nil
	}
	if err := def.Apply(child); err != nil {
		// FAIL, not DISPOSED: the fiber stays visible (with its error) for
		// diagnostics and the UI. Effects registered before the failure are
		// still unwound so nothing half-registered keeps running.
		f.fail(err)
		f.mu.Lock()
		effects := f.effects
		f.effects = nil
		f.mu.Unlock()
		for i := len(effects) - 1; i >= 0; i-- {
			_ = effects[i]()
		}
		return fmt.Errorf("cordis: plugin %q failed: %w", def.ID, err)
	}
	f.mu.Lock()
	f.setState(StateActive)
	f.mu.Unlock()
	return nil
}

// missingServices returns the injected names with no provider in realm.
func (r *Runtime) missingServices(inject []string, realm string) []string {
	var missing []string
	for _, name := range inject {
		if entry, ok := r.byRealm[realm][name]; !ok || entry == nil {
			missing = append(missing, name)
		}
	}
	return missing
}

// ---------------------------------------------------------------------------
// Pending registry: fibers waiting on services
// ---------------------------------------------------------------------------

type pendingEntry struct {
	def    PluginDef
	realm  string
	parent *Context
}

func (r *Runtime) registerPending(def PluginDef, realm string, missing []string) {
	r.pendMu.Lock()
	r.pending[def.ID] = pendingEntry{def: def, realm: realm}
	r.pendMu.Unlock()
	_ = missing
}

// resolvePending activates every pending fiber whose services now exist.
// Called after each Provide.
func (r *Runtime) resolvePending() {
	r.pendMu.Lock()
	var ready []string
	for id := range r.pending {
		if f, ok := r.fibers[id]; ok {
			f.mu.Lock()
			def, realm := f.def, f.defRealm
			f.mu.Unlock()
			if len(r.missingServices(def.Inject, realm)) == 0 {
				ready = append(ready, id)
			}
		} else {
			delete(r.pending, id)
		}
	}
	entries := make([]pendingEntry, 0, len(ready))
	for _, id := range ready {
		entries = append(entries, r.pending[id])
		delete(r.pending, id)
	}
	r.pendMu.Unlock()

	for _, e := range entries {
		f, ok := r.fibers[e.def.ID]
		if !ok {
			continue
		}
		if err := r.activate(e.parent, f, e.def, e.realm); err != nil {
			// apply failed: fiber is FAILED; surfaced via Fibers().
			_ = err
		}
	}
}

// ---------------------------------------------------------------------------
// Unload
// ---------------------------------------------------------------------------

// unloadFiber unwinds effects in reverse order, unregisters provided
// services, and unloads child fibers. It marks the fiber DISPOSED.
func (r *Runtime) unloadFiber(f *Fiber) {
	f.mu.Lock()
	if f.State == StateUnloading || f.State == StateDisposed {
		f.mu.Unlock()
		return
	}
	f.setState(StateUnloading)
	effects := f.effects
	f.effects = nil
	children := f.children
	f.children = nil
	provided := f.provided
	f.provided = nil
	f.mu.Unlock()

	// Children first (parent unload disposes mounted children).
	for _, child := range children {
		r.unloadFiber(child)
		delete(r.fibers, child.ID)
	}

	// Disposers run in reverse registration order.
	for i := len(effects) - 1; i >= 0; i-- {
		_ = effects[i]()
	}

	// Unregister provided services; unload dependents that injected them.
	r.mu.Lock()
	for _, name := range provided {
		if entry, ok := r.byRealm[f.defRealm][name]; ok && entry.ownerID == f.ID {
			delete(r.byRealm[f.defRealm], name)
		}
	}
	r.mu.Unlock()

	f.mu.Lock()
	f.setState(StateDisposed)
	f.mu.Unlock()

	// Dependents that injected a now-missing service unload too.
	r.unloadDependents(f.ID, provided)
}

// unloadDependents unloads fibers whose inject list names any of the lost
// services. They were ACTIVE; Cordis semantics say they unload with the
// dependency and reload if the service returns (re-PENDING until then).
func (r *Runtime) unloadDependents(lostOwner string, lostServices []string) {
	lost := map[string]bool{}
	for _, s := range lostServices {
		lost[s] = true
	}
	r.mu.Lock()
	var toUnload []*Fiber
	for _, f := range r.fibers {
		f.mu.Lock()
		injects := append([]string(nil), f.def.Inject...)
		f.mu.Unlock()
		for _, name := range injects {
			if lost[name] {
				if entry, ok := r.byRealm[f.defRealm][name]; !ok || entry == nil {
					toUnload = append(toUnload, f)
				}
				break
			}
		}
	}
	r.mu.Unlock()

	for _, f := range toUnload {
		r.unloadFiber(f)
		delete(r.fibers, f.ID)
		// Re-arm as PENDING against the same definition.
		f.mu.Lock()
		def, realm := f.def, f.defRealm
		f.mu.Unlock()
		r.registerPending(def, realm, nil)
		f.mu.Lock()
		f.setState(StatePending)
		f.mu.Unlock()
		r.fibers[f.ID] = f
	}
}

// Unload unloads one fiber by id (public handle for hot reload and disposal).
func (r *Runtime) Unload(id string) error {
	r.mu.Lock()
	f, ok := r.fibers[id]
	if !ok {
		r.mu.Unlock()
		return fmt.Errorf("cordis: plugin %q not mounted", id)
	}
	r.mu.Unlock()
	r.unloadFiber(f)
	delete(r.fibers, id)
	return nil
}

var _ = sync.Mutex{}
