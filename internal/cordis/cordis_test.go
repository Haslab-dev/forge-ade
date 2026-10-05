package cordis

import (
	"os"
	"path/filepath"
	"testing"
)

func writeComposition(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "cordis.yml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLifecycleUnloadUnwindsEffects(t *testing.T) {
	rt := NewRuntime()
	c := &Context{fiber: &Fiber{ID: "host"}, root: rt}

	unloaded := false
	f, err := c.Mount(PluginDef{
		ID: "lifecycle-demo",
		Apply: func(ctx *Context) error {
			ctx.OnDispose(func() error {
				unloaded = true
				return nil
			})
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if f.State != StateActive {
		t.Fatalf("state = %s, want ACTIVE", f.State)
	}
	if err := rt.Unload("lifecycle-demo"); err != nil {
		t.Fatal(err)
	}
	if !unloaded {
		t.Fatal("disposer never ran")
	}
	if _, ok := rt.fibers["lifecycle-demo"]; ok {
		t.Fatal("fiber still registered after unload")
	}
}

func TestServicesInjectHoldsPendingThenResolves(t *testing.T) {
	rt := NewRuntime()
	c := &Context{fiber: &Fiber{ID: "host"}, root: rt}

	greeted := ""
	consumerMounted := false
	_, err := c.Mount(PluginDef{
		ID:     "consumer",
		Inject: []string{"greeter"},
		Apply: func(ctx *Context) error {
			consumerMounted = true
			if g, ok := ctx.Get("greeter"); ok {
				if s, ok := g.(interface{ Greet() string }); ok {
					greeted = s.Greet()
				}
			}
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if consumerMounted {
		t.Fatal("consumer mounted without its service")
	}
	pending := rt.PendingNames()
	if len(pending) == 0 {
		t.Fatal("consumer should be PENDING")
	}

	// Provide the service; the PENDING fiber resolves.
	_, err = c.Mount(PluginDef{
		ID: "greeter",
		Apply: func(ctx *Context) error {
			ctx.Provide("greeter", greeterStub{})
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !consumerMounted {
		t.Fatal("consumer never resolved after Provide")
	}
	if greeted != "hello stub" {
		t.Fatalf("greeted = %q", greeted)
	}
	_ = greeted
}

type greeterStub struct{}

func (greeterStub) Greet() string { return "hello stub" }

func TestServiceLossUnloadsDependents(t *testing.T) {
	rt := NewRuntime()
	c := &Context{fiber: &Fiber{ID: "host"}, root: rt}

	if _, err := c.Mount(PluginDef{
		ID: "provider",
		Apply: func(ctx *Context) error {
			ctx.Provide("shell", "stub-shell")
			return nil
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Mount(PluginDef{
		ID:     "dependent",
		Inject: []string{"shell"},
		Apply:  func(ctx *Context) error { return nil },
	}); err != nil {
		t.Fatal(err)
	}
	if !rt.isMounted("dependent") {
		t.Fatal("dependent not mounted")
	}

	// Unload the provider: the dependent unloads and re-arms PENDING.
	if err := rt.Unload("provider"); err != nil {
		t.Fatal(err)
	}
	if rt.isMounted("dependent") {
		t.Fatal("dependent should have unloaded with its service")
	}
	f := rt.fibers["dependent"]
	if f == nil {
		t.Fatal("dependent fiber missing from registry")
	}
	f.mu.Lock()
	st := f.State
	f.mu.Unlock()
	if st != StatePending {
		t.Fatalf("dependent state = %s, want PENDING", st)
	}
}

func TestEventsWaterfallShortCircuit(t *testing.T) {
	rt := NewRuntime()
	c := &Context{fiber: &Fiber{ID: "host"}, root: rt}

	order := []string{}
	var secondRan bool
	if _, err := c.Mount(PluginDef{
		ID: "listener-a",
		Apply: func(ctx *Context) error {
			ctx.On("tools/result", func(p any) any {
				order = append(order, "a")
				return "replaced-by-a" // short-circuit
			})
			return nil
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Mount(PluginDef{
		ID: "listener-b",
		Apply: func(ctx *Context) error {
			ctx.On("tools/result", func(p any) any {
				secondRan = true
				order = append(order, "b")
				return nil
			})
			return nil
		},
	}); err != nil {
		t.Fatal(err)
	}

	res := c.Emit("tools/result", "original")
	if res.Payload != "replaced-by-a" || !res.ShortCircuited {
		t.Fatalf("waterfall result = %+v", res)
	}
	if secondRan {
		t.Fatal("listener-b ran after short-circuit")
	}
	if len(order) != 1 || order[0] != "a" {
		t.Fatalf("order = %v", order)
	}

	// Unloading listener-a removes its listener: b now runs.
	_ = rt.Unload("listener-a")
	res = c.Emit("tools/result", "original")
	if res.Payload != "original" || res.ShortCircuited {
		t.Fatalf("after unload result = %+v", res)
	}
	if !secondRan {
		t.Fatal("listener-b did not run after listener-a unloaded")
	}
}

func TestConfigValidationFailsLoud(t *testing.T) {
	dir := t.TempDir()
	path := writeComposition(t, dir, `- id: config-demo
  name: './config-demo.ts'
  config:
    targets: 'not-an-array'
  configSchema:
    targets:
      type: array
      required: true
`)
	rt := NewRuntime()
	cm := newCompositionManager(rt, path)
	err := cm.Load()
	if err == nil {
		t.Fatal("expected config validation error")
	}
	f := rt.fibers["config-demo"]
	if f == nil {
		t.Fatal("fiber missing")
	}
	f.mu.Lock()
	st, e := f.State, f.Error
	f.mu.Unlock()
	if st != StateFailed {
		t.Fatalf("state = %s, want FAILED", st)
	}
	if e == nil {
		t.Fatal("fiber error empty")
	}
}

func TestDisabledEntryNeverMounts(t *testing.T) {
	dir := t.TempDir()
	path := writeComposition(t, dir, `- id: off
  name: './off.ts'
  disabled: true
`)
	rt := NewRuntime()
	cm := newCompositionManager(rt, path)
	if err := cm.Load(); err != nil {
		t.Fatal(err)
	}
	if _, ok := rt.fibers["off"]; ok {
		t.Fatal("disabled entry mounted")
	}
}

func TestDiffImplementsHotReload(t *testing.T) {
	dir := t.TempDir()
	path := writeComposition(t, dir, `- id: hello
  name: './hello.ts'
  configSchema:
    greeting:
      type: string
`)
	rt := NewRuntime()
	cm := newCompositionManager(rt, path)
	if err := cm.Load(); err != nil {
		t.Fatal(err)
	}
	if !rt.isMounted("hello") {
		t.Fatal("hello not mounted after Load")
	}

	// Change the config: Diff unmounts and remounts the entry.
	if err := os.WriteFile(path, []byte(`- id: hello
  name: './hello.ts'
  configSchema:
    greeting:
      type: string
  config:
    greeting: hi
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := cm.Diff(); err != nil {
		t.Fatal(err)
	}
	if !rt.isMounted("hello") {
		t.Fatal("hello not remounted after Diff")
	}

	// Remove the entry: Diff unmounts it.
	if err := os.WriteFile(path, []byte(`- id: other
  name: './other.ts'
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := cm.Diff(); err != nil {
		t.Fatal(err)
	}
	if rt.isMounted("hello") {
		t.Fatal("hello should be unmounted after removal")
	}
	if _, ok := rt.fibers["other"]; !ok {
		t.Fatal("other not mounted after addition")
	}
}

func TestToolsRegisterAndUnloadWithFiber(t *testing.T) {
	rt := NewRuntime()
	c := &Context{fiber: &Fiber{ID: "host"}, root: rt}

	if _, err := c.Mount(PluginDef{
		ID: "tool-owner",
		Apply: func(ctx *Context) error {
			ctx.RegisterTool(ToolDef{Name: "greet", Description: "Greet"})
			return nil
		},
	}); err != nil {
		t.Fatal(err)
	}
	if got := len(rt.EntryTools("tool-owner")); got != 1 {
		t.Fatalf("tools = %d, want 1", got)
	}
	_ = rt.Unload("tool-owner")
	if got := len(rt.EntryTools("tool-owner")); got != 0 {
		t.Fatalf("tools after unload = %d, want 0", got)
	}
}
