package plugins

import (
	"time"
)

// PluginSource indicates where the plugin was discovered or created.
type PluginSource string

const (
	SourceWorkspace PluginSource = "workspace"
	SourceGlobal    PluginSource = "global"
	SourceRuntime   PluginSource = "runtime"
	SourceBuiltin   PluginSource = "builtin"
)

// HandlerType specifies how a plugin tool executes.
type HandlerType string

const (
	HandlerCommand HandlerType = "command" // Executes a shell command template
	HandlerScript  HandlerType = "script"  // Executes a script file in the plugin directory
	HandlerInline  HandlerType = "inline"  // Built-in or inline code
)

// PluginToolDef defines a tool exposed by a plugin to the agent.
type PluginToolDef struct {
	Name           string                 `json:"name"`
	Description    string                 `json:"description"`
	Parameters     map[string]interface{} `json:"parameters,omitempty"`
	HandlerType    HandlerType            `json:"handler_type"`
	Command        string                 `json:"command,omitempty"`
	Script         string                 `json:"script,omitempty"`
	TimeoutSeconds int                    `json:"timeout_seconds,omitempty"`
}

// PluginSkillDef defines a skill bundled within a plugin.
type PluginSkillDef struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Body        string `json:"body"`
}

// PluginToolWithPlugin pairs an exposed tool with its parent plugin ID.
type PluginToolWithPlugin struct {
	PluginID string
	Tool     PluginToolDef
}

// PluginSkillWithPlugin pairs a bundled skill with its parent plugin ID.
type PluginSkillWithPlugin struct {
	PluginID string
	Skill    PluginSkillDef
}

// Inspector allows components like Executor to inspect live runtime plugin state.
type Inspector interface {
	List() []*Plugin
	ActiveTools() []PluginToolWithPlugin
	ActiveSkills() []PluginSkillWithPlugin
	ActiveSystemPrompts() []string
}

// Plugin represents a registered or discovered agent plugin.
type Plugin struct {
	ID           string           `json:"id"`
	Name         string           `json:"name"`
	Description  string           `json:"description"`
	Version      string           `json:"version"`
	Author       string           `json:"author,omitempty"`
	Enabled      bool             `json:"enabled"`
	Source       PluginSource     `json:"source"`
	Path         string           `json:"path,omitempty"` // Path to plugin.json
	Dir          string           `json:"dir,omitempty"`  // Directory holding plugin files
	SystemPrompt string           `json:"system_prompt,omitempty"`
	Tools        []PluginToolDef  `json:"tools,omitempty"`
	Skills       []PluginSkillDef `json:"skills,omitempty"`
	CreatedAt    time.Time        `json:"created_at,omitempty"`
	UpdatedAt    time.Time        `json:"updated_at,omitempty"`
}

// CreatePluginRequest is the payload used by the agent or UI to create a new plugin.
type CreatePluginRequest struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	Version      string            `json:"version,omitempty"`
	Author       string            `json:"author,omitempty"`
	Scope        string            `json:"scope,omitempty"` // "workspace" or "global"
	SystemPrompt string            `json:"system_prompt,omitempty"`
	Tools        []PluginToolDef   `json:"tools,omitempty"`
	Skills       []PluginSkillDef  `json:"skills,omitempty"`
	Files        map[string]string `json:"files,omitempty"` // filename -> content
}

// PluginToolResult is returned after executing a plugin tool.
type PluginToolResult struct {
	Stdout   string `json:"stdout,omitempty"`
	Stderr   string `json:"stderr,omitempty"`
	ExitCode int    `json:"exit_code"`
	Success  bool   `json:"success"`
	Error    string `json:"error,omitempty"`
}
