package agentsession

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
)

// ConfigStore persists AgentCLIConfig records in <dataDir>/agent-clis.json.
// It merges the built-in defaults on load so a fresh install (or a new
// built-in agent added by an update) always exposes every supported CLI while
// preserving user edits.
type ConfigStore struct {
	mu   sync.RWMutex
	path string
	list []AgentCLIConfig
}

// NewConfigStore loads (or initializes) the agent CLI configuration store.
func NewConfigStore(dataDir string) *ConfigStore {
	s := &ConfigStore{path: filepath.Join(dataDir, "agent-clis.json")}
	s.load()
	return s
}

func (s *ConfigStore) load() {
	s.list = DefaultAgentCLIConfigs()
	raw, err := os.ReadFile(s.path)
	if err != nil {
		s.persist()
		return
	}
	var saved []AgentCLIConfig
	if err := json.Unmarshal(raw, &saved); err != nil {
		log.Printf("agentsession: parse %s: %v", s.path, err)
		return
	}
	byID := make(map[string]AgentCLIConfig, len(saved))
	for _, c := range saved {
		if c.ID == "" {
			continue
		}
		byID[c.ID] = c
	}
	merged := make([]AgentCLIConfig, 0, len(s.list))
	seen := make(map[string]bool, len(s.list))
	for _, def := range s.list {
		if user, ok := byID[def.ID]; ok {
			// Migrate stale built-in defaults: a saved executable that equals a
			// renamed default was never touched by the user — take the new name.
			if old, renamed := renamedDefaults[def.ID]; renamed && user.Executable == old {
				user.Executable = def.Executable
			}
			// Same for renamed resume defaults (e.g. omp now resumes via
			// --resume instead of --continue).
			if old, renamed := renamedResumeDefaults[def.ID]; renamed &&
				len(user.ResumeArgs) == 1 && user.ResumeArgs[0] == old {
				user.ResumeArgs = def.ResumeArgs
			}
			// Adopt resume defaults for configs saved before the field existed
			// (nil = never set; an explicit empty list means "disabled").
			if user.ResumeArgs == nil {
				user.ResumeArgs = def.ResumeArgs
			}
			merged = append(merged, user)
		} else {
			merged = append(merged, def)
		}
		seen[def.ID] = true
	}
	// Custom user-defined CLIs keep their place at the end.
	for _, c := range saved {
		if !seen[c.ID] {
			merged = append(merged, c)
		}
	}
	s.list = merged
}

func (s *ConfigStore) persist() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(s.list, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, raw, 0644)
}

// List returns a copy of all agent CLI configs.
func (s *ConfigStore) List() []AgentCLIConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]AgentCLIConfig, len(s.list))
	copy(out, s.list)
	return out
}

// Get returns a copy of one config by id.
func (s *ConfigStore) Get(id string) (AgentCLIConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, c := range s.list {
		if c.ID == id {
			return c, nil
		}
	}
	return AgentCLIConfig{}, fmt.Errorf("agent CLI config not found: %s", id)
}

// Save upserts a config (matched by id) and persists to disk.
func (s *ConfigStore) Save(cfg AgentCLIConfig) (AgentCLIConfig, error) {
	if cfg.ID == "" {
		return AgentCLIConfig{}, fmt.Errorf("agent CLI config id is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	replaced := false
	for i := range s.list {
		if s.list[i].ID == cfg.ID {
			// Name belongs to the built-in definition; keep it stable so UI
			// labels never drift when only executable/args change.
			if s.list[i].Executable != "" && cfg.Name == "" {
				cfg.Name = s.list[i].Name
			}
			s.list[i] = cfg
			replaced = true
			break
		}
	}
	if !replaced {
		s.list = append(s.list, cfg)
	}
	if err := s.persist(); err != nil {
		return cfg, err
	}
	return cfg, nil
}

// Reset restores one config to its built-in defaults (no-op for custom CLIs).
func (s *ConfigStore) Reset(id string) (AgentCLIConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, def := range DefaultAgentCLIConfigs() {
		if def.ID == id {
			for i := range s.list {
				if s.list[i].ID == id {
					s.list[i] = def
					break
				}
			}
			if err := s.persist(); err != nil {
				return def, err
			}
			return def, nil
		}
	}
	return AgentCLIConfig{}, fmt.Errorf("no built-in defaults for agent CLI: %s", id)
}
