package cordis

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// Entry is one cordis.yml row: a declarative plugin mount with optional
// config, inject requirements, and a disabled flag.
type Entry struct {
	ID       string                 `yaml:"id" json:"id"`
	Name     string                 `yaml:"name" json:"name"`
	Inject   []string               `yaml:"inject" json:"inject,omitempty"`
	Disabled bool                   `yaml:"disabled" json:"disabled,omitempty"`
	Config   map[string]interface{} `yaml:"config" json:"config,omitempty"`
	// ConfigSchema declares field types/required for validation of Config.
	ConfigSchema map[string]interface{} `yaml:"configSchema" json:"config_schema,omitempty"`

	// loaded TS module path (relative to the config dir), informational —
	// personal plugins execute declaratively on the Go side.
	ResolvedPath string `yaml:"-" json:"resolved_path,omitempty"`
}

// ConfigValidation is one schema violation for an entry's config block.
type ConfigValidationError struct {
	Field  string `json:"field"`
	Detail string `json:"detail"`
}

func (e ConfigValidationError) String() string {
	return fmt.Sprintf("$.%s %s (at %s)", e.Field, e.Detail, e.Field)
}

// loadComposition reads a cordis.yml and returns its entries with stable ids
// (id, else the name without extension, else positional "entry-N").
func loadComposition(path string) ([]Entry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", filepath.Base(path), err)
	}
	var raw []struct {
		ID           string                 `yaml:"id"`
		Name         string                 `yaml:"name"`
		Inject       []string               `yaml:"inject"`
		Disabled     bool                   `yaml:"disabled"`
		Config       map[string]interface{} `yaml:"config"`
		ConfigSchema map[string]interface{} `yaml:"configSchema"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse %s: %w", filepath.Base(path), err)
	}
	entries := make([]Entry, 0, len(raw))
	for i, r := range raw {
		e := Entry{
			ID:           r.ID,
			Name:         r.Name,
			Inject:       r.Inject,
			Disabled:     r.Disabled,
			Config:       r.Config,
			ConfigSchema: r.ConfigSchema,
		}
		if e.ID == "" {
			if e.Name != "" {
				base := filepath.Base(e.Name)
				e.ID = strings.TrimSuffix(base, filepath.Ext(base))
			} else {
				e.ID = fmt.Sprintf("entry-%d", i+1)
			}
		}
		if e.Name != "" && !filepath.IsAbs(e.Name) && strings.HasPrefix(e.Name, "./") {
			e.ResolvedPath = filepath.Join(filepath.Dir(path), e.Name)
		}
		entries = append(entries, e)
	}
	return entries, nil
}

// compositionManager owns one cordis.yml composition: it mounts entries,
// applies config, diffs the file on change (HMR), and reports states.
type compositionManager struct {
	mu        sync.Mutex
	path      string
	runtime   *Runtime
	entries   []Entry           // last read composition
	ids       map[string]bool   // entry id → mounted as PluginDef?
	dirty     map[string]*Fiber // entry id → fiber (mount handles)
	groupMu   sync.Mutex
	groups    map[string][]string // group id → entry ids (isolate realms)
	watching  bool
	stopWatch chan struct{}

	onChange func() // UI hook: called after any state change
}

func newCompositionManager(rt *Runtime, path string) *compositionManager {
	return &compositionManager{
		path:      path,
		runtime:   rt,
		ids:       map[string]bool{},
		groups:    map[string][]string{},
		stopWatch: make(chan struct{}),
	}
}

// Load reads the composition and mounts every enabled entry. Config is
// validated before apply; a failing entry becomes a FAILED fiber with the
// error attached — loud, not skipped.
func (cm *compositionManager) Load() error {
	entries, err := loadComposition(cm.path)
	if err != nil {
		return err
	}
	cm.mu.Lock()
	cm.entries = entries
	cm.mu.Unlock()

	for _, e := range entries {
		if e.Disabled {
			continue
		}
		if err := cm.mountEntry(e); err != nil {
			return err
		}
	}
	return nil
}

// mountEntry converts one entry into a PluginDef and mounts it. The entry's
// config is validated against required fields derived from the module
// (declarative personal plugins validate via the entry's declared schema in
// configSchema; unknown keys pass through).
func (cm *compositionManager) mountEntry(e Entry) error {
	def := PluginDef{
		ID:     e.ID,
		Inject: e.Inject,
		Source: cm.path,
		Apply: func(ctx *Context) error {
			// Declarative execution: the personal-plugin executor consumes
			// the entry's tools/skills; runtime-side apply validates config.
			return e.validateConfig()
		},
	}
	cm.mu.Lock()
	cm.ids[e.ID] = true
	cm.mu.Unlock()
	_, err := cm.runtime.mount(nil, def, "")
	return err
}

// validateEntryConfig fails the load when required config keys declared in
// the entry's own configSchema are missing, or when values have obviously
// wrong types. Schema-declared defaults are filled in on the entry.
func (e Entry) validateConfig() error {
	schema := e.ConfigSchema
	if len(schema) == 0 {
		return nil // no declared schema: nothing to validate
	}
	for field, specRaw := range schema {
		spec, ok := specRaw.(map[string]interface{})
		if !ok {
			continue
		}
		required := false
		if r, ok := spec["required"].(bool); ok {
			required = r
		}
		value, present := e.Config[field]
		if required && (!present || value == nil || value == "") {
			return fmt.Errorf("invalid config: $.%s is required (at %s)", field, field)
		}
		if !present {
			if def, ok := spec["default"]; ok {
				e.Config[field] = def
			}
			continue
		}
		want, _ := spec["type"].(string)
		if want == "" {
			continue
		}
		if err := checkType(field, want, value); err != nil {
			return err
		}
	}
	return nil
}

func checkType(field, want string, value interface{}) error {
	ok := false
	switch want {
	case "string":
		_, ok = value.(string)
	case "number":
		switch value.(type) {
		case float64, int, int64:
			ok = true
		}
	case "boolean", "bool":
		_, ok = value.(bool)
	case "array":
		_, ok = value.([]interface{})
	case "object":
		_, ok = value.(map[string]interface{})
	}
	if !ok {
		return fmt.Errorf("invalid config: $.%s expected %s but got %T (at %s)", field, want, value, field)
	}
	return nil
}

// Diff implements hot reload: re-read the composition, unmount removed or
// changed entries, mount added or changed ones. Entries carry stable ids so
// an untouched row is left alone.
func (cm *compositionManager) Diff() error {
	newEntries, err := loadComposition(cm.path)
	if err != nil {
		return err
	}
	cm.mu.Lock()
	oldByID := map[string]Entry{}
	for _, e := range cm.entries {
		oldByID[e.ID] = e
	}
	cm.entries = newEntries
	cm.mu.Unlock()

	newByID := map[string]Entry{}
	for _, e := range newEntries {
		newByID[e.ID] = e
	}

	// Unmount: removed entries, disabled entries, and changed configs.
	for id := range oldByID {
		old := oldByID[id]
		newE, exists := newByID[id]
		if !exists || newE.Disabled || !sameEntry(old, newE) {
			_ = cm.runtime.Unload(id)
		}
	}

	// Mount: new or changed enabled entries.
	for _, e := range newEntries {
		if e.Disabled {
			continue
		}
		old, existed := oldByID[e.ID]
		if !existed || !sameEntry(old, e) || !cm.runtime.isMounted(e.ID) {
			if err := cm.mountEntry(e); err != nil {
				return err
			}
		}
	}
	return nil
}

func sameEntry(a, b Entry) bool {
	if a.ID != b.ID || a.Name != b.Name || a.Disabled != b.Disabled {
		return false
	}
	if len(a.Inject) != len(b.Inject) {
		return false
	}
	for i := range a.Inject {
		if a.Inject[i] != b.Inject[i] {
			return false
		}
	}
	return reflect.DeepEqual(a.Config, b.Config)
}

// isMounted reports whether an entry's fiber is currently mounted.
func (r *Runtime) isMounted(id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	f, ok := r.fibers[id]
	return ok && func() bool {
		f.mu.Lock()
		defer f.mu.Unlock()
		return f.State == StateActive || f.State == StateLoading
	}()
}

// StartWatching enables HMR: poll the composition file (and its directory for
// added files) on a simple ticker — the file watcher dependency of the app
// already exists, but cordis keeps its own tiny loop to stay self-contained.
func (cm *compositionManager) StartWatching(interval time.Duration) {
	if cm.watching {
		return
	}
	cm.watching = true
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		var lastMod time.Time
		if fi, err := os.Stat(cm.path); err == nil {
			lastMod = fi.ModTime()
		}
		for {
			select {
			case <-cm.stopWatch:
				return
			case <-ticker.C:
				fi, err := os.Stat(cm.path)
				if err != nil {
					continue
				}
				if fi.ModTime().After(lastMod) {
					lastMod = fi.ModTime()
					_ = cm.Diff()
					if cm.onChange != nil {
						cm.onChange()
					}
				}
			}
		}
	}()
}

// StopWatching ends HMR polling.
func (cm *compositionManager) StopWatching() {
	if cm.stopWatch != nil {
		select {
		case <-cm.stopWatch:
		default:
			close(cm.stopWatch)
		}
	}
	cm.watching = false
}

var _ = sync.Mutex{}

// NewComposition creates a composition manager bound to a cordis.yml path.
func NewComposition(rt *Runtime, path string) *compositionManager {
	return newCompositionManager(rt, path)
}

// EnsureRealm returns a host Context for a realm, letting the harness provide
// standard services (e.g. "tools") before entries mount.
func (r *Runtime) EnsureRealm(realm string) (*Context, bool) {
	r.mu.Lock()
	if _, ok := r.fibers["__host__"]; !ok {
		r.fibers["__host__"] = &Fiber{ID: "__host__", source: "host", State: StateActive}
	}
	r.mu.Unlock()
	return &Context{fiber: r.fibers["__host__"], root: r, realm: realm}, true
}
