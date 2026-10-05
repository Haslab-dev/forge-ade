package acp

import (
	"strings"
)

// Registry maps agent configurations to their native protocol drivers.
// One module per agent, matching kybern-drivers: claude, codex, opencode,
// pi (also omp). Speak the agent's own protocol; no shared abstraction.
type Registry struct {
	drivers map[string]AgentDriver
}

func NewRegistry() *Registry {
	r := &Registry{
		drivers: map[string]AgentDriver{},
	}

	claudeDrv := NewClaudeDriver()
	codexDrv := NewCodexDriver()
	opencodeDrv := NewOpencodeDriver()
	piDrv := NewPiDriver()
	ompDrv := NewOmpDriver()

	r.Register("agent-claude", claudeDrv)
	r.Register("claude", claudeDrv)

	r.Register("agent-codex", codexDrv)
	r.Register("codex", codexDrv)

	r.Register("agent-opencode", opencodeDrv)
	r.Register("opencode", opencodeDrv)

	r.Register("agent-pi", piDrv)
	r.Register("pi", piDrv)
	r.Register("pi-acp", piDrv)

	r.Register("agent-ohmypi", ompDrv)
	r.Register("omp", ompDrv)

	return r
}

func (r *Registry) Register(key string, driver AgentDriver) {
	r.drivers[strings.ToLower(key)] = driver
}

// DriverFor returns the driver for a config, matching id and command first,
// then substring heuristics (same resolution order as before the port).
func (r *Registry) DriverFor(cfg *AgentConfig) AgentDriver {
	if cfg == nil {
		return nil
	}

	if drv, ok := r.drivers[strings.ToLower(cfg.ID)]; ok {
		return drv
	}
	if drv, ok := r.drivers[strings.ToLower(cfg.Command)]; ok {
		return drv
	}

	lowerID := strings.ToLower(cfg.ID)
	lowerCmd := strings.ToLower(cfg.Command)

	if strings.Contains(lowerID, "claude") || strings.Contains(lowerCmd, "claude") {
		return r.drivers["claude"]
	}
	if strings.Contains(lowerID, "codex") || strings.Contains(lowerCmd, "codex") {
		return r.drivers["codex"]
	}
	if strings.Contains(lowerID, "opencode") || strings.Contains(lowerCmd, "opencode") {
		return r.drivers["opencode"]
	}
	if strings.Contains(lowerID, "ohmypi") || strings.Contains(lowerCmd, "omp") {
		return r.drivers["omp"]
	}
	if strings.Contains(lowerID, "pi") || strings.Contains(lowerCmd, "pi") {
		return r.drivers["pi"]
	}

	return nil
}
