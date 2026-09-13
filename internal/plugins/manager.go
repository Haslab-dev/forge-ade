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
	"gopkg.in/yaml.v3"
)

var validIDRegex = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

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
		bus:       bus,
	}
	m.executor = NewExecutor(m)

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
	// 1. Builtin Git Workflow
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

	// 2. DeepSeek PTC / Code Mode
	builtinCodeMode := &Plugin{
		ID:          "dsh-code-mode",
		Name:        "DeepSeek PTC / Code Mode",
		Description: "Programmatic Tool Calling mode from DeepSeek Harness. Lets the agent execute TypeScript, JavaScript, Python, or Bash to batch operations in a single turn.",
		Version:     "1.0.0",
		Author:      "DeepSeek / Forge",
		Enabled:     true,
		Source:      SourceBuiltin,
		SystemPrompt: `You have DeepSeek PTC (Programmatic Tool Calling) Code Mode enabled.
When a task requires searching, reading, analyzing, or modifying multiple files, prefer executing programmatic code via 'run_code' rather than issuing dozens of serial tool calls.
Use Node.js (JavaScript/TypeScript) or Python scripts to filter, aggregate, and transform workspace data in a single turn.
Always print clear, structured JSON or concise text results to stdout.`,
		Tools: []PluginToolDef{
			{
				Name:        "run_code",
				Description: "Execute a JavaScript, TypeScript, Python, or Bash script against the workspace to batch complex operations in one step.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"code": map[string]interface{}{
							"type":        "string",
							"description": "Script code to execute in workspace context",
						},
						"language": map[string]interface{}{
							"type":        "string",
							"description": "Language: javascript, typescript, python, bash",
							"default":     "javascript",
						},
						"args": map[string]interface{}{
							"type":        "object",
							"description": "Optional arguments passed to the script environment",
						},
					},
					"required": []interface{}{"code"},
				},
				HandlerType:    HandlerInline,
				TimeoutSeconds: 60,
			},
			{
				Name:        "eval_expression",
				Description: "Quickly evaluate a code expression in JavaScript or Python and return the evaluated result.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"expression": map[string]interface{}{
							"type":        "string",
							"description": "Expression to evaluate",
						},
						"language": map[string]interface{}{
							"type":        "string",
							"description": "Language: javascript or python",
							"default":     "javascript",
						},
					},
					"required": []interface{}{"expression"},
				},
				HandlerType:    HandlerInline,
				TimeoutSeconds: 15,
			},
		},
		Skills: []PluginSkillDef{
			{
				Name:        "ptc-orchestration",
				Description: "Guide to Programmatic Tool Calling and batch scripting",
				Body:        "When inspecting repositories with dozens of files, write a short Node.js or Python snippet with run_code to query files with fs or os.walk, filter by regex, and return only the relevant lines. This avoids round-trip latency and context bloat.",
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// 3. DeepSeek Minimal Mode
	builtinMinimalMode := &Plugin{
		ID:          "dsh-minimal-mode",
		Name:        "DeepSeek Minimal Mode",
		Description: "High-speed minimal coding mode from DeepSeek Harness. Single-tool persistent shell and zero token waste.",
		Version:     "1.0.0",
		Author:      "DeepSeek / Forge",
		Enabled:     false,
		Source:      SourceBuiltin,
		SystemPrompt: `You are operating in DeepSeek Minimal Mode.
Focus on high-speed, direct implementation. Eliminate conversational filler, pleasantries, and unsolicited architectural lectures.
Execute shell commands and precise edits. Test your changes immediately and return clean status summaries.`,
		Tools: []PluginToolDef{
			{
				Name:        "persistent_shell",
				Description: "Run shell commands in the workspace environment with environment persistence and timeout control.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"command": map[string]interface{}{
							"type":        "string",
							"description": "Shell command to execute in workspace",
						},
					},
					"required": []interface{}{"command"},
				},
				HandlerType:    HandlerInline,
				TimeoutSeconds: 120,
			},
		},
		Skills: []PluginSkillDef{
			{
				Name:        "minimal-engineering",
				Description: "High-speed concise coding guidelines",
				Body:        "1. Inspect exact lines.\n2. Apply minimal atomic edits.\n3. Run build/test to verify.\n4. Conclude with single summary sentence.",
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// 4. DeepSeek Creator Mode (Cordis)
	builtinCreatorMode := &Plugin{
		ID:          "dsh-creator-mode",
		Name:        "DeepSeek Creator Mode (Cordis)",
		Description: "Plugin and agent preset authoring mode from DeepSeek Harness. Inspects Cordis plugins, runtime services, validates manifests, and guides creation of new plugins and skills.",
		Version:     "1.0.0",
		Author:      "DeepSeek / Forge",
		Enabled:     true,
		Source:      SourceBuiltin,
		SystemPrompt: `You are in DeepSeek Creator Mode. You can inspect, author, and extend Forge & Cordis plugins and agent presets.
Remember the Two Planes rule from DeepSeek Harness:
- HOST composition: registries, sandbox, persistence, cross-session services.
- AGENT PRESET: session-scoped tools, persona, prompt sections, and skills.
Use 'cordis_inspect' to check active runtime plugins and tools. Use 'validate_plugin' to check plugin health. Use 'create_plugin' to register new capabilities.`,
		Tools: []PluginToolDef{
			{
				Name:        "cordis_inspect",
				Description: "Inspect currently loaded Cordis & Forge plugins, active tools, system prompt contributions, and harness status.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"target": map[string]interface{}{
							"type":        "string",
							"description": "Target: all, plugins, tools, skills, system_prompt",
							"default":     "all",
						},
					},
				},
				HandlerType:    HandlerInline,
				TimeoutSeconds: 10,
			},
			{
				Name:        "validate_plugin",
				Description: "Validate a plugin directory, verifying plugin.json, preset.yml, agent.cordis.yml, or cordis.yml syntax and required scripts.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{
							"type":        "string",
							"description": "Path to plugin directory or manifest file",
						},
					},
					"required": []interface{}{"path"},
				},
				HandlerType:    HandlerInline,
				TimeoutSeconds: 15,
			},
		},
		Skills: []PluginSkillDef{
			{
				Name:        "cordis-plugin-development",
				Description: "How to develop Cordis plugins with apply(ctx: Context)",
				Body: `In DeepSeek Harness and Cordis, every plugin is a module that exports:
export const name = 'my-plugin'
export function apply(ctx: Context) {
  // register tools, services, or prompt sections
}
Presets are composed via cordis.yml or agent.cordis.yml.`,
			},
			{
				Name:        "editing-cordis-compositions",
				Description: "Best practices for editing Cordis agent presets",
				Body: `When authoring presets:
1. Always copy from an existing preset rather than starting from scratch.
2. Put session-specific tools and persona in the agent preset.
3. Keep global state in the host composition.
4. Mount-validate with validate_plugin before deploying.`,
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// 5. DeepSeek Coder Reasoning Protocol
	builtinDeepSeekCoder := &Plugin{
		ID:          "dsh-deepseek-coder",
		Name:        "DeepSeek Coder Reasoning Mode",
		Description: "DeepSeek state-of-the-art coding harness protocol: Grounding & Exploration -> Test-Driven Reproduction -> Minimal Diff Refactoring -> Automated Verification.",
		Version:     "1.0.0",
		Author:      "DeepSeek / Forge",
		Enabled:     true,
		Source:      SourceBuiltin,
		SystemPrompt: `You adhere to the DeepSeek Coder Protocol:
1. Grounding: Never assume file structures or APIs. Always inspect actual code and imports first.
2. TDD / Reproduction: Write a reproduction test script or inspect existing tests before editing.
3. Minimal Diff: Modify only what is necessary. Preserve existing styling and comments.
4. Verification: Run tests and linters via 'verify_changes' or 'test_runner' before concluding.`,
		Tools: []PluginToolDef{
			{
				Name:        "verify_changes",
				Description: "Verify workspace changes: inspects git diff --stat and automatically triggers project tests and linter.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"run_tests": map[string]interface{}{
							"type":        "boolean",
							"description": "Whether to run test suite (default true)",
							"default":     true,
						},
					},
				},
				HandlerType:    HandlerInline,
				TimeoutSeconds: 60,
			},
			{
				Name:        "test_runner",
				Description: "Auto-detects project test framework (go test, npm test, pytest, cargo test) and runs test suite with optional filters.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{
							"type":        "string",
							"description": "Test path or package",
						},
						"filter": map[string]interface{}{
							"type":        "string",
							"description": "Test name regex or filter",
						},
					},
				},
				HandlerType:    HandlerInline,
				TimeoutSeconds: 60,
			},
		},
		Skills: []PluginSkillDef{
			{
				Name:        "deepseek-coding-protocol",
				Description: "DeepSeek 4-phase verified coding protocol",
				Body: `Phase 1: Explore & Ground
Phase 2: Reproduce & Test
Phase 3: Minimal Edit
Phase 4: Verify with verify_changes`,
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	m.mu.Lock()
	m.plugins[builtinGit.ID] = builtinGit
	m.plugins[builtinCodeMode.ID] = builtinCodeMode
	m.plugins[builtinMinimalMode.ID] = builtinMinimalMode
	m.plugins[builtinCreatorMode.ID] = builtinCreatorMode
	m.plugins[builtinDeepSeekCoder.ID] = builtinDeepSeekCoder
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
		p := m.loadPluginFromDir(pluginDir, e.Name(), source)
		if p != nil {
			m.plugins[p.ID] = p
		}
	}
}

func (m *Manager) loadPluginFromDir(pluginDir, folderName string, source PluginSource) *Plugin {
	manifestJSON := filepath.Join(pluginDir, "plugin.json")
	presetYAML := filepath.Join(pluginDir, "preset.yml")
	agentCordisYAML := filepath.Join(pluginDir, "agent.cordis.yml")
	cordisYAML := filepath.Join(pluginDir, "cordis.yml")

	var p *Plugin

	// 1. Check plugin.json
	if data, err := os.ReadFile(manifestJSON); err == nil {
		var parsed Plugin
		if err := json.Unmarshal(data, &parsed); err == nil {
			p = &parsed
			p.Path = manifestJSON
			var rawMap map[string]interface{}
			if err := json.Unmarshal(data, &rawMap); err == nil {
				if _, hasEnabled := rawMap["enabled"]; !hasEnabled {
					p.Enabled = true
				}
			}
		}
	}

	// 2. Check Cordis preset.yml / agent.cordis.yml
	if p == nil {
		presetData, errPreset := os.ReadFile(presetYAML)
		agentData, errAgent := os.ReadFile(agentCordisYAML)
		if errPreset == nil || errAgent == nil {
			p = &Plugin{
				ID:      folderName,
				Enabled: true,
			}
			if errPreset == nil {
				p.Path = presetYAML
				var py struct {
					Name        string `yaml:"name"`
					Description string `yaml:"description"`
				}
				if err := yaml.Unmarshal(presetData, &py); err == nil {
					p.Name = py.Name
					p.Description = py.Description
				}
			}
			if errAgent == nil {
				if p.Path == "" {
					p.Path = agentCordisYAML
				}
				p.SystemPrompt = parseCordisSystemPrompt(agentData)
				p.Tools = parseCordisTools(agentData, folderName)
			}
		}
	}

	// 3. Check cordis.yml
	if p == nil {
		if _, err := os.ReadFile(cordisYAML); err == nil {
			p = &Plugin{
				ID:          folderName,
				Name:        folderName,
				Description: "Cordis plugin module",
				Path:        cordisYAML,
				Enabled:     true,
			}
		}
	}

	if p == nil {
		return nil
	}

	if p.ID == "" {
		p.ID = folderName
	}
	if p.Name == "" {
		p.Name = p.ID
	}
	p.Source = source
	p.Dir = pluginDir

	// Auto-load skills from <pluginDir>/skills/
	skillsDir := filepath.Join(pluginDir, "skills")
	discoveredSkills := loadSkillsFromDir(skillsDir)
	if len(discoveredSkills) > 0 {
		existing := make(map[string]bool)
		for _, s := range p.Skills {
			existing[s.Name] = true
		}
		for _, ds := range discoveredSkills {
			if !existing[ds.Name] {
				p.Skills = append(p.Skills, ds)
				existing[ds.Name] = true
			}
		}
	}

	return p
}

func parseCordisSystemPrompt(data []byte) string {
	var entries []map[string]interface{}
	if err := yaml.Unmarshal(data, &entries); err != nil {
		return ""
	}

	var parts []string
	for _, entry := range entries {
		id, _ := entry["id"].(string)
		name, _ := entry["name"].(string)

		if id == "persona" || strings.Contains(name, "persona") {
			if cfg, ok := entry["config"].(map[string]interface{}); ok {
				if prefix, ok := cfg["prefix"].(string); ok && prefix != "" {
					parts = append(parts, prefix)
				}
				if suffix, ok := cfg["suffix"].(string); ok && suffix != "" {
					parts = append(parts, suffix)
				}
			}
		}

		if id == "planning" || strings.Contains(name, "plan") {
			if cfgList, ok := entry["config"].([]interface{}); ok {
				for _, item := range cfgList {
					if itemMap, ok := item.(map[string]interface{}); ok {
						if subCfg, ok := itemMap["config"].(map[string]interface{}); ok {
							if section, ok := subCfg["section"].(string); ok && section != "" {
								parts = append(parts, section)
							}
						}
					}
				}
			}
		}
	}
	return strings.Join(parts, "\n\n")
}

func parseCordisTools(data []byte, folderName string) []PluginToolDef {
	var tools []PluginToolDef
	content := string(data)
	lowerName := strings.ToLower(folderName)

	if strings.Contains(content, "ptc") || strings.Contains(content, "run_code") || strings.Contains(lowerName, "ptc") || strings.Contains(lowerName, "code") {
		tools = append(tools, PluginToolDef{
			Name:        "run_code",
			Description: "Execute a JavaScript, TypeScript, Python, or Bash script against the workspace",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"code":     map[string]interface{}{"type": "string", "description": "Code to run"},
					"language": map[string]interface{}{"type": "string", "description": "Language", "default": "javascript"},
				},
				"required": []interface{}{"code"},
			},
			HandlerType:    HandlerInline,
			TimeoutSeconds: 60,
		}, PluginToolDef{
			Name:        "eval_expression",
			Description: "Quick expression evaluation in JavaScript or Python",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"expression": map[string]interface{}{"type": "string", "description": "Expression"},
					"language":   map[string]interface{}{"type": "string", "description": "Language", "default": "javascript"},
				},
				"required": []interface{}{"expression"},
			},
			HandlerType:    HandlerInline,
			TimeoutSeconds: 15,
		})
	}

	if strings.Contains(content, "persistent") || strings.Contains(content, "pty") || strings.Contains(lowerName, "minimal") {
		tools = append(tools, PluginToolDef{
			Name:        "persistent_shell",
			Description: "Run commands in persistent shell environment",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{"type": "string", "description": "Shell command"},
				},
				"required": []interface{}{"command"},
			},
			HandlerType:    HandlerInline,
			TimeoutSeconds: 120,
		})
	}

	if strings.Contains(content, "cordis") || strings.Contains(lowerName, "cordis") || strings.Contains(lowerName, "creator") {
		tools = append(tools, PluginToolDef{
			Name:        "cordis_inspect",
			Description: "Inspect loaded Cordis plugins, active tools, and prompt sections",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"target": map[string]interface{}{"type": "string", "description": "all, plugins, tools, skills", "default": "all"},
				},
			},
			HandlerType:    HandlerInline,
			TimeoutSeconds: 10,
		}, PluginToolDef{
			Name:        "validate_plugin",
			Description: "Validate a plugin directory or manifest",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{"type": "string", "description": "Path to plugin"},
				},
				"required": []interface{}{"path"},
			},
			HandlerType:    HandlerInline,
			TimeoutSeconds: 15,
		})
	}

	return tools
}

func loadSkillsFromDir(skillsDir string) []PluginSkillDef {
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return nil
	}

	var out []PluginSkillDef
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		skillFile := filepath.Join(skillsDir, e.Name(), "SKILL.md")
		data, err := os.ReadFile(skillFile)
		if err != nil {
			continue
		}

		content := string(data)
		var name, desc, body string
		name = e.Name()
		body = content

		if strings.HasPrefix(content, "---") {
			parts := strings.SplitN(content, "---", 3)
			if len(parts) >= 3 {
				var meta struct {
					Name        string `yaml:"name"`
					Description string `yaml:"description"`
				}
				if err := yaml.Unmarshal([]byte(parts[1]), &meta); err == nil {
					if meta.Name != "" {
						name = meta.Name
					}
					desc = meta.Description
				}
				body = strings.TrimSpace(parts[2])
			}
		}

		out = append(out, PluginSkillDef{
			Name:        name,
			Description: desc,
			Body:        body,
		})
	}
	return out
}

// List returns all discovered and registered plugins.
func (m *Manager) List() []*Plugin {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]*Plugin, 0, len(m.plugins))
	for _, p := range m.plugins {
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

	// If it has a JSON manifest path, persist the change
	manifestPath := p.Path
	var manifestData []byte
	if manifestPath != "" && strings.HasSuffix(manifestPath, ".json") {
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
