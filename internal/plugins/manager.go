package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/hasdev/forge-ade/internal/events"
)

var validIDRegex = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type PluginToolWithPlugin struct {
	PluginID string
	Tool     PluginToolDef
}

type PluginSkillWithPlugin struct {
	PluginID string
	Skill    PluginSkillDef
}

type Manager struct {
	mu           sync.RWMutex
	plugins      map[string]*Plugin
	globalDir    string
	workspaceDir string
	executor     *Executor
	bus          *events.Bus
}

func NewManager(dataDir string, bus *events.Bus) *Manager {
	globalDir := filepath.Join(dataDir, "plugins")
	_ = os.MkdirAll(globalDir, 0755)

	m := &Manager{
		plugins:   make(map[string]*Plugin),
		globalDir: globalDir,
		executor:  NewExecutor(),
		bus:       bus,
	}

	m.registerBuiltins()
	m.Reload()
	return m
}

func (m *Manager) SetWorkspace(dir string) {
	m.mu.Lock()
	m.workspaceDir = dir
	m.mu.Unlock()
	m.Reload()
}

func (m *Manager) registerBuiltins() {
	// Sample built-in productivity plugin: git-quick-tools
	builtinGit := &Plugin{
		ID:          "builtin-git-tools",
		Name:        "Git Quick Actions",
		Description: "Quick commit and branch inspection tools",
		Version:     "1.0.0",
		Author:      "ForgeADE",
		Enabled:     true,
		Source:      SourceBuiltin,
		Tools: []PluginToolDef{
			{
				Name:        "git_last_commit",
				Description: "View the most recent commit summary and changed files",
				Parameters: map[string]interface{}{
					"type": "object",
				},
				HandlerType: HandlerCommand,
				Command:     "git log -1 --stat",
			},
			{
				Name:        "git_show_branches",
				Description: "List local and remote git branches",
				Parameters: map[string]interface{}{
					"type": "object",
				},
				HandlerType: HandlerCommand,
				Command:     "git branch -a",
			},
		},
		Skills: []PluginSkillDef{
			{
				Name:        "git-commit-guide",
				Description: "Best practices for writing conventional git commits",
				Body:        "When authoring commits, follow conventional commits: feat, fix, docs, style, refactor, test, chore. Keep the first line under 72 characters.",
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	m.mu.Lock()
	m.plugins[builtinGit.ID] = builtinGit
	m.mu.Unlock()
}

// Reload scans the global and workspace plugin directories.
func (m *Manager) Reload() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Retain builtins and runtime registrations
	retained := make(map[string]*Plugin)
	for id, p := range m.plugins {
		if p.Source == SourceBuiltin || p.Source == SourceRuntime {
			retained[id] = p
		}
	}
	m.plugins = retained

	// 1. Scan global directory (~/.forge-ade/plugins)
	m.scanDir(m.globalDir, SourceGlobal)

	// Also check ~/.agents/plugins
	if home, err := os.UserHomeDir(); err == nil {
		agentsGlobal := filepath.Join(home, ".agents", "plugins")
		m.scanDir(agentsGlobal, SourceGlobal)
	}

	// 2. Scan workspace directories
	if m.workspaceDir != "" {
		m.scanDir(filepath.Join(m.workspaceDir, ".forge", "plugins"), SourceWorkspace)
		m.scanDir(filepath.Join(m.workspaceDir, ".agents", "plugins"), SourceWorkspace)
		m.scanDir(filepath.Join(m.workspaceDir, ".dsh", "plugins"), SourceWorkspace)
	}

	m.emitChange()
}

func (m *Manager) scanDir(dir string, source PluginSource) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pluginDir := filepath.Join(dir, e.Name())
		manifestPath := filepath.Join(pluginDir, "plugin.json")
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			continue
		}

		var p Plugin
		if err := json.Unmarshal(data, &p); err != nil {
			continue
		}

		if p.ID == "" {
			p.ID = e.Name()
		}
		if p.Name == "" {
			p.Name = p.ID
		}
		p.Source = source
		p.Path = manifestPath
		p.Dir = pluginDir

		// Default enabled to true if not explicitly set false in JSON
		var rawMap map[string]interface{}
		if err := json.Unmarshal(data, &rawMap); err == nil {
			if _, hasEnabled := rawMap["enabled"]; !hasEnabled {
				p.Enabled = true
			}
		}

		m.plugins[p.ID] = &p
	}
}

// List returns all discovered and registered plugins.
func (m *Manager) List() []*Plugin {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]*Plugin, 0, len(m.plugins))
	for _, p := range m.plugins {
		// Return copy
		pCopy := *p
		out = append(out, &pCopy)
	}
	return out
}

// Get returns a plugin by ID.
func (m *Manager) Get(id string) (*Plugin, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	p, ok := m.plugins[id]
	if !ok {
		return nil, false
	}
	pCopy := *p
	return &pCopy, true
}

// CreatePlugin creates a plugin on disk and registers it immediately.
func (m *Manager) CreatePlugin(req CreatePluginRequest, activeWorkspace string) (*Plugin, error) {
	req.ID = strings.TrimSpace(req.ID)
	if req.ID == "" {
		return nil, fmt.Errorf("plugin id cannot be empty")
	}
	req.ID = strings.ToLower(req.ID)
	if !validIDRegex.MatchString(req.ID) {
		return nil, fmt.Errorf("plugin id must be kebab-case (e.g. 'git-helpers', 'docker-tools')")
	}

	if strings.TrimSpace(req.Name) == "" {
		req.Name = req.ID
	}
	if req.Version == "" {
		req.Version = "1.0.0"
	}

	scope := req.Scope
	if scope == "" {
		scope = "workspace"
	}

	var targetDir string
	var source PluginSource

	ws := activeWorkspace
	if ws == "" {
		m.mu.RLock()
		ws = m.workspaceDir
		m.mu.RUnlock()
	}

	if scope == "workspace" && ws != "" {
		targetDir = filepath.Join(ws, ".forge", "plugins", req.ID)
		source = SourceWorkspace
	} else {
		targetDir = filepath.Join(m.globalDir, req.ID)
		source = SourceGlobal
	}

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create plugin directory %s: %w", targetDir, err)
	}

	// Write extra files (scripts, templates)
	for filename, content := range req.Files {
		filePath := filepath.Join(targetDir, filename)
		_ = os.MkdirAll(filepath.Dir(filePath), 0755)
		if err := os.WriteFile(filePath, []byte(content), 0755); err != nil {
			return nil, fmt.Errorf("failed to write plugin file %s: %w", filename, err)
		}
	}

	p := &Plugin{
		ID:           req.ID,
		Name:         req.Name,
		Description:  req.Description,
		Version:      req.Version,
		Author:       req.Author,
		Enabled:      true,
		Source:       source,
		Dir:          targetDir,
		Path:         filepath.Join(targetDir, "plugin.json"),
		SystemPrompt: req.SystemPrompt,
		Tools:        req.Tools,
		Skills:       req.Skills,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// Persist plugin.json
	manifestBytes, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to serialize plugin manifest: %w", err)
	}

	if err := os.WriteFile(p.Path, manifestBytes, 0644); err != nil {
		return nil, fmt.Errorf("failed to write plugin manifest to %s: %w", p.Path, err)
	}

	m.mu.Lock()
	m.plugins[p.ID] = p
	m.mu.Unlock()

	m.emitChange()
	return p, nil
}

// Register registers an in-memory or runtime plugin.
func (m *Manager) Register(p *Plugin) error {
	if p == nil || p.ID == "" {
		return fmt.Errorf("invalid plugin: id required")
	}
	m.mu.Lock()
	p.UpdatedAt = time.Now()
	m.plugins[p.ID] = p
	m.mu.Unlock()

	m.emitChange()
	return nil
}

// TogglePlugin toggles the enabled state of a plugin.
func (m *Manager) TogglePlugin(id string, enabled bool) error {
	m.mu.Lock()
	p, ok := m.plugins[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("plugin %s not found", id)
	}
	p.Enabled = enabled
	p.UpdatedAt = time.Now()

	// If it has a manifest path, persist the change
	manifestPath := p.Path
	var manifestData []byte
	if manifestPath != "" {
		manifestData, _ = json.MarshalIndent(p, "", "  ")
	}
	m.mu.Unlock()

	if len(manifestData) > 0 {
		_ = os.WriteFile(manifestPath, manifestData, 0644)
	}

	m.emitChange()
	return nil
}

// DeletePlugin deletes a plugin from disk and unregisters it.
func (m *Manager) DeletePlugin(id string) error {
	m.mu.Lock()
	p, ok := m.plugins[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("plugin %s not found", id)
	}
	if p.Source == SourceBuiltin {
		m.mu.Unlock()
		return fmt.Errorf("cannot delete builtin plugin")
	}
	delete(m.plugins, id)
	pluginDir := p.Dir
	m.mu.Unlock()

	if pluginDir != "" {
		_ = os.RemoveAll(pluginDir)
	}

	m.emitChange()
	return nil
}

// ExecuteTool dispatches execution of a plugin tool.
func (m *Manager) ExecuteTool(ctx context.Context, pluginID string, toolName string, args map[string]interface{}, workspaceDir string) (*PluginToolResult, error) {
	m.mu.RLock()
	p, ok := m.plugins[pluginID]
	if !ok {
		m.mu.RUnlock()
		return nil, fmt.Errorf("plugin %s not found", pluginID)
	}
	if !p.Enabled {
		m.mu.RUnlock()
		return nil, fmt.Errorf("plugin %s is disabled", pluginID)
	}

	var foundTool *PluginToolDef
	for _, t := range p.Tools {
		if t.Name == toolName {
			toolCopy := t
			foundTool = &toolCopy
			break
		}
	}
	m.mu.RUnlock()

	if foundTool == nil {
		return nil, fmt.Errorf("tool %s not found in plugin %s", toolName, pluginID)
	}

	ws := workspaceDir
	if ws == "" {
		m.mu.RLock()
		ws = m.workspaceDir
		m.mu.RUnlock()
	}

	return m.executor.Execute(ctx, p, foundTool, args, ws)
}

// ActiveSystemPrompts returns system prompts from all enabled plugins.
func (m *Manager) ActiveSystemPrompts() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var prompts []string
	for _, p := range m.plugins {
		if p.Enabled && strings.TrimSpace(p.SystemPrompt) != "" {
			prompts = append(prompts, fmt.Sprintf("### Plugin: %s\n%s", p.Name, strings.TrimSpace(p.SystemPrompt)))
		}
	}
	return prompts
}

// ActiveTools returns all tools provided by currently enabled plugins.
func (m *Manager) ActiveTools() []PluginToolWithPlugin {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var tools []PluginToolWithPlugin
	for _, p := range m.plugins {
		if p.Enabled {
			for _, t := range p.Tools {
				tools = append(tools, PluginToolWithPlugin{
					PluginID: p.ID,
					Tool:     t,
				})
			}
		}
	}
	return tools
}

// ActiveSkills returns all skills contributed by enabled plugins.
func (m *Manager) ActiveSkills() []PluginSkillWithPlugin {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var skills []PluginSkillWithPlugin
	for _, p := range m.plugins {
		if p.Enabled {
			for _, s := range p.Skills {
				skills = append(skills, PluginSkillWithPlugin{
					PluginID: p.ID,
					Skill:    s,
				})
			}
		}
	}
	return skills
}

func (m *Manager) emitChange() {
	if m.bus != nil {
		m.bus.Publish(events.Event{
			Type: "plugins/changed",
			Data: map[string]interface{}{
				"timestamp": time.Now(),
			},
		})
	}
}
