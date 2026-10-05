package cordis

import (
	"os"
	"path/filepath"
	"testing"
)

// Mirrors a real personal plugin dir: cordis.yml declaring a config-validated
// entry plus a plugin.json-less layout like .dsh/plugins/hello-cordis.
func TestIntegrationPersonalPluginComposition(t *testing.T) {
	dir := t.TempDir()
	path := writeComposition(t, dir, `- id: my-tools
  name: './my-tools.ts'
  inject: [tools]
  config:
    greeting: Hello
    targets:
      - world
      - you
  configSchema:
    greeting:
      type: string
      required: true
    targets:
      type: array
      required: true
`)
	rt := NewRuntime()
	// The host (plugin manager) provides the standard "tools" service.
	if host, ok := rt.EnsureRealm(""); ok {
		host.Provide("tools", "forge-ade-plugin-executor")
	}
	cm := NewComposition(rt, path)
	if err := cm.Load(); err != nil {
		t.Fatalf("load: %v", err)
	}
	f := rt.fibers["my-tools"]
	if f == nil {
		t.Fatal("fiber missing")
	}
	f.mu.Lock()
	st := f.State
	f.mu.Unlock()
	if st != StateActive {
		t.Fatalf("state = %s", st)
	}
	// Config defaults filled? greeting defaulted... it was provided; targets kept.
	if !rt.isMounted("my-tools") {
		t.Fatal("not mounted")
	}
	// HMR: disable the entry, Diff unmounts.
	if err := os.WriteFile(path, []byte(`- id: my-tools
  name: './my-tools.ts'
  disabled: true
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := cm.Diff(); err != nil {
		t.Fatal(err)
	}
	if rt.isMounted("my-tools") {
		t.Fatal("still mounted after disable")
	}
	_ = filepath.Join
}
