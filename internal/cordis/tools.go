package cordis

// Bridge between the Cordis runtime and the personal-plugin tool executor:
// cordis.yml entries can declare tools with JSON-schema parameters; the
// schema is passed through to the existing declarative executor unchanged
// (command/script handlers stay in internal/plugins).

// ToolDef is the declarative tool shape a cordis.yml entry (or its referenced
// module's config) declares. It mirrors internal/plugins.PluginToolDef but
// lives here so the runtime has no reverse dependency on the plugin manager.
type ToolDef struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
	// Handler is the command or script executed by the plugin executor.
	// Empty means the tool is registration-only (provided by a service).
	Handler string `json:"handler,omitempty"`
	// TimeoutSeconds bounds handler execution; 0 uses the executor default.
	TimeoutSeconds int `json:"timeout_seconds,omitempty"`
}

// RegisterTool attaches a declarative tool to the calling fiber. The
// registration is an effect: unloading the fiber unregisters the tool.
func (c *Context) RegisterTool(def ToolDef) {
	c.root.mu.Lock()
	c.root.entryTools[c.fiber.ID] = append(c.root.entryTools[c.fiber.ID], def)
	c.root.mu.Unlock()
	c.OnDispose(func() error {
		c.root.mu.Lock()
		defer c.root.mu.Unlock()
		kept := c.root.entryTools[c.fiber.ID][:0]
		for _, t := range c.root.entryTools[c.fiber.ID] {
			if t.Name != def.Name {
				kept = append(kept, t)
			}
		}
		c.root.entryTools[c.fiber.ID] = kept
		return nil
	})
}

// EntryTools returns the tools registered under a fiber id.
func (r *Runtime) EntryTools(fiberID string) []ToolDef {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := append([]ToolDef(nil), r.entryTools[fiberID]...)
	return out
}
