package forgemcp

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// AgentMCPInstallStatus represents detection and Forge MCP installation state for an external agent.
type AgentMCPInstallStatus struct {
	AgentID    string `json:"agentId"`    // "claude", "antigravity", "codex", "opencode", "ohmypi", "pi", "cursor", "windsurf", "gemini", "commandcode"
	AgentName  string `json:"agentName"`  // "Claude Code", "Google Antigravity", etc.
	Executable string `json:"executable"` // "claude", "agy", "codex", etc.
	ConfigPath string `json:"configPath"` // resolved target configuration file path
	Found      bool   `json:"found"`      // whether CLI binary or config directory is found
	Installed  bool   `json:"installed"`  // whether Forge MCP is configured in this agent
	ServerName string `json:"serverName"` // "forge"
}

// AgentMCPInstallResult returns the outcome of an install or uninstall operation.
type AgentMCPInstallResult struct {
	AgentID    string `json:"agentId"`
	AgentName  string `json:"agentName"`
	ConfigPath string `json:"configPath"`
	Success    bool   `json:"success"`
	Message    string `json:"message"`
}

type agentSpec struct {
	id         string
	name       string
	executable string
	kind       string // "mcpServers" | "opencode" | "codex" | "claude"
	resolve    func(home string) (configPath string, found bool)
}

// ResolveForgeExecutable finds the preferred binary path or command name for Forge MCP.
func ResolveForgeExecutable() string {
	// If "forge-ade" is found in PATH, use "forge-ade"
	if _, err := exec.LookPath("forge-ade"); err == nil {
		return "forge-ade"
	}
	// Fall back to current running binary if available
	if current, err := os.Executable(); err == nil && current != "" {
		return current
	}
	return "forge-ade"
}

func getAgentSpecs() []agentSpec {
	return []agentSpec{
		{
			id:         "claude",
			name:       "Claude Code",
			executable: "claude",
			kind:       "claude",
			resolve: func(home string) (string, bool) {
				p1 := filepath.Join(home, ".claude.json")
				p2 := filepath.Join(home, ".claude", "mcp.json")
				cDir := filepath.Join(home, ".claude")

				hasExec := isExecutableOnPath("claude")
				if fileExists(p1) {
					return p1, true
				}
				if fileExists(p2) {
					return p2, true
				}
				if dirExists(cDir) || hasExec {
					return p1, true
				}
				return p1, false
			},
		},
		{
			id:         "antigravity",
			name:       "Google Antigravity",
			executable: "agy",
			kind:       "mcpServers",
			resolve: func(home string) (string, bool) {
				candidates := []string{
					filepath.Join(home, ".gemini", "antigravity-cli", "mcp_config.json"),
					filepath.Join(home, ".gemini", "antigravity", "mcp_config.json"),
					filepath.Join(home, ".gemini", "config", "mcp_config.json"),
				}
				for _, c := range candidates {
					if fileExists(c) {
						return c, true
					}
				}
				hasExec := isExecutableOnPath("agy")
				dCli := filepath.Join(home, ".gemini", "antigravity-cli")
				dAgy := filepath.Join(home, ".gemini", "antigravity")
				if dirExists(dCli) || hasExec {
					return candidates[0], true
				}
				if dirExists(dAgy) {
					return candidates[1], true
				}
				if dirExists(filepath.Join(home, ".gemini")) {
					return candidates[0], true
				}
				return candidates[0], false
			},
		},
		{
			id:         "opencode",
			name:       "OpenCode",
			executable: "opencode",
			kind:       "opencode",
			resolve: func(home string) (string, bool) {
				configDir := os.Getenv("XDG_CONFIG_HOME")
				if configDir == "" {
					configDir = filepath.Join(home, ".config")
				}
				candidates := []string{
					filepath.Join(configDir, "opencode", "opencode.json"),
					filepath.Join(configDir, "opencode", "opencode.jsonc"),
					filepath.Join(home, ".opencode", "opencode.json"),
				}
				for _, c := range candidates {
					if fileExists(c) {
						return c, true
					}
				}
				hasExec := isExecutableOnPath("opencode")
				if dirExists(filepath.Join(configDir, "opencode")) || hasExec {
					return candidates[0], true
				}
				if dirExists(filepath.Join(home, ".opencode")) {
					return candidates[2], true
				}
				return candidates[0], false
			},
		},
		{
			id:         "codex",
			name:       "OpenAI Codex",
			executable: "codex",
			kind:       "codex",
			resolve: func(home string) (string, bool) {
				p := filepath.Join(home, ".codex", "config.toml")
				cDir := filepath.Join(home, ".codex")
				hasExec := isExecutableOnPath("codex")
				if fileExists(p) {
					return p, true
				}
				if dirExists(cDir) || hasExec {
					return p, true
				}
				return p, false
			},
		},
		{
			id:         "ohmypi",
			name:       "Oh My Pi (OMP)",
			executable: "omp",
			kind:       "mcpServers",
			resolve: func(home string) (string, bool) {
				p1 := filepath.Join(home, ".omp", "agent", "mcp.json")
				p2 := filepath.Join(home, ".omp", "mcp.json")
				hasExec := isExecutableOnPath("omp")
				if fileExists(p1) {
					return p1, true
				}
				if fileExists(p2) {
					return p2, true
				}
				if dirExists(filepath.Join(home, ".omp", "agent")) || dirExists(filepath.Join(home, ".omp")) || hasExec {
					return p1, true
				}
				return p1, false
			},
		},
		{
			id:         "pi",
			name:       "Pi",
			executable: "pi",
			kind:       "mcpServers",
			resolve: func(home string) (string, bool) {
				p := filepath.Join(home, ".pi", "agent", "mcp.json")
				hasExec := isExecutableOnPath("pi") || isExecutableOnPath("pi-acp")
				if fileExists(p) {
					return p, true
				}
				if dirExists(filepath.Join(home, ".pi")) || hasExec {
					return p, true
				}
				return p, false
			},
		},
		{
			id:         "cursor",
			name:       "Cursor",
			executable: "cursor",
			kind:       "mcpServers",
			resolve: func(home string) (string, bool) {
				p := filepath.Join(home, ".cursor", "mcp.json")
				hasExec := isExecutableOnPath("cursor")
				if fileExists(p) {
					return p, true
				}
				if dirExists(filepath.Join(home, ".cursor")) || hasExec {
					return p, true
				}
				return p, false
			},
		},
		{
			id:         "windsurf",
			name:       "Windsurf",
			executable: "windsurf",
			kind:       "mcpServers",
			resolve: func(home string) (string, bool) {
				p := filepath.Join(home, ".codeium", "windsurf", "mcp_config.json")
				hasExec := isExecutableOnPath("windsurf")
				if fileExists(p) {
					return p, true
				}
				if dirExists(filepath.Join(home, ".codeium", "windsurf")) || hasExec {
					return p, true
				}
				return p, false
			},
		},
		{
			id:         "gemini",
			name:       "Gemini CLI",
			executable: "gemini",
			kind:       "mcpServers",
			resolve: func(home string) (string, bool) {
				p := filepath.Join(home, ".gemini", "settings.json")
				hasExec := isExecutableOnPath("gemini")
				if fileExists(p) {
					return p, true
				}
				if dirExists(filepath.Join(home, ".gemini")) || hasExec {
					return p, true
				}
				return p, false
			},
		},
		{
			id:         "commandcode",
			name:       "Command Code",
			executable: "cmd",
			kind:       "mcpServers",
			resolve: func(home string) (string, bool) {
				p := filepath.Join(home, ".commandcode", "mcp.json")
				hasExec := isExecutableOnPath("cmd")
				if fileExists(p) {
					return p, true
				}
				if dirExists(filepath.Join(home, ".commandcode")) || hasExec {
					return p, true
				}
				return p, false
			},
		},
	}
}

// NormalizeAgentID normalizes variant agent IDs (e.g. "agent-claude" -> "claude").
func NormalizeAgentID(rawID string) string {
	id := strings.TrimPrefix(rawID, "agent-")
	id = strings.TrimPrefix(id, "cli-")
	switch strings.ToLower(id) {
	case "agy", "antigravity", "gemini-antigravity":
		return "antigravity"
	case "claude", "claude-code":
		return "claude"
	case "codex", "openai-codex":
		return "codex"
	case "opencode":
		return "opencode"
	case "omp", "ohmypi", "oh-my-pi":
		return "ohmypi"
	case "pi", "pi-acp":
		return "pi"
	case "cursor":
		return "cursor"
	case "windsurf":
		return "windsurf"
	case "gemini", "gemini-cli":
		return "gemini"
	case "cmd", "commandcode", "command-code":
		return "commandcode"
	default:
		return strings.ToLower(id)
	}
}

// GetAgentMCPInstallStatuses returns the detection and installation state for all supported agents.
func GetAgentMCPInstallStatuses() []AgentMCPInstallStatus {
	home, _ := os.UserHomeDir()
	specs := getAgentSpecs()
	out := make([]AgentMCPInstallStatus, 0, len(specs))

	for _, s := range specs {
		cfgPath, found := s.resolve(home)
		installed := false
		if fileExists(cfgPath) {
			installed = checkForgeInstalled(cfgPath, s.kind)
		}

		out = append(out, AgentMCPInstallStatus{
			AgentID:    s.id,
			AgentName:  s.name,
			Executable: s.executable,
			ConfigPath: cfgPath,
			Found:      found,
			Installed:  installed,
			ServerName: "forge",
		})
	}
	return out
}

// InstallForgeMCPToAgent configures Forge MCP into the designated agent's config file.
func InstallForgeMCPToAgent(rawAgentID string) (*AgentMCPInstallResult, error) {
	agentID := NormalizeAgentID(rawAgentID)
	specs := getAgentSpecs()
	var matched *agentSpec
	for i := range specs {
		if specs[i].id == agentID {
			matched = &specs[i]
			break
		}
	}
	if matched == nil {
		return nil, fmt.Errorf("unsupported agent %q", rawAgentID)
	}

	home, _ := os.UserHomeDir()
	cfgPath, _ := matched.resolve(home)
	execPath := ResolveForgeExecutable()

	var err error
	switch matched.kind {
	case "codex":
		err = installToCodex(cfgPath, execPath)
	case "opencode":
		err = installToOpencode(cfgPath, execPath)
	case "claude":
		err = installToClaude(cfgPath, execPath)
	default: // "mcpServers"
		err = installToGenericMCPServers(cfgPath, execPath)
	}

	if err != nil {
		return &AgentMCPInstallResult{
			AgentID:    matched.id,
			AgentName:  matched.name,
			ConfigPath: cfgPath,
			Success:    false,
			Message:    err.Error(),
		}, err
	}

	return &AgentMCPInstallResult{
		AgentID:    matched.id,
		AgentName:  matched.name,
		ConfigPath: cfgPath,
		Success:    true,
		Message:    fmt.Sprintf("Installed Forge MCP into %s configuration (%s)", matched.name, cfgPath),
	}, nil
}

// UninstallForgeMCPFromAgent removes Forge MCP from the specified agent's configuration file.
func UninstallForgeMCPFromAgent(rawAgentID string) (*AgentMCPInstallResult, error) {
	agentID := NormalizeAgentID(rawAgentID)
	specs := getAgentSpecs()
	var matched *agentSpec
	for i := range specs {
		if specs[i].id == agentID {
			matched = &specs[i]
			break
		}
	}
	if matched == nil {
		return nil, fmt.Errorf("unsupported agent %q", rawAgentID)
	}

	home, _ := os.UserHomeDir()
	cfgPath, _ := matched.resolve(home)

	if !fileExists(cfgPath) {
		return &AgentMCPInstallResult{
			AgentID:    matched.id,
			AgentName:  matched.name,
			ConfigPath: cfgPath,
			Success:    true,
			Message:    "Configuration file does not exist, nothing to remove",
		}, nil
	}

	var err error
	switch matched.kind {
	case "codex":
		err = uninstallFromCodex(cfgPath)
	case "opencode":
		err = uninstallFromOpencode(cfgPath)
	case "claude":
		err = uninstallFromClaude(cfgPath)
	default:
		err = uninstallFromGenericMCPServers(cfgPath)
	}

	if err != nil {
		return &AgentMCPInstallResult{
			AgentID:    matched.id,
			AgentName:  matched.name,
			ConfigPath: cfgPath,
			Success:    false,
			Message:    err.Error(),
		}, err
	}

	return &AgentMCPInstallResult{
		AgentID:    matched.id,
		AgentName:  matched.name,
		ConfigPath: cfgPath,
		Success:    true,
		Message:    fmt.Sprintf("Removed Forge MCP from %s configuration (%s)", matched.name, cfgPath),
	}, nil
}

// InstallForgeMCPToAllAgents installs Forge MCP into all found external agents.
func InstallForgeMCPToAllAgents() ([]AgentMCPInstallResult, error) {
	statuses := GetAgentMCPInstallStatuses()
	var results []AgentMCPInstallResult

	for _, st := range statuses {
		if st.Found && !st.Installed {
			res, _ := InstallForgeMCPToAgent(st.AgentID)
			if res != nil {
				results = append(results, *res)
			}
		}
	}
	return results, nil
}

// ---------------------------------------------------------------------------
// Format-specific readers / writers
// ---------------------------------------------------------------------------

func checkForgeInstalled(path string, kind string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	content := string(data)

	if kind == "codex" {
		return strings.Contains(content, "[mcp_servers.forge]")
	}

	if kind == "opencode" {
		var doc struct {
			MCP map[string]interface{} `json:"mcp"`
		}
		if json.Unmarshal([]byte(stripComments(content)), &doc) == nil {
			if _, ok := doc.MCP["forge"]; ok {
				return true
			}
		}
		return false
	}

	// claude or generic mcpServers
	var doc struct {
		MCPServers map[string]interface{} `json:"mcpServers"`
	}
	if json.Unmarshal([]byte(stripComments(content)), &doc) == nil {
		if _, ok := doc.MCPServers["forge"]; ok {
			return true
		}
	}
	return false
}

func installToGenericMCPServers(path string, execPath string) error {
	return mutateJSONMap(path, func(doc map[string]interface{}) {
		mcpServers := getOrCreateMap(doc, "mcpServers")
		mcpServers["forge"] = map[string]interface{}{
			"command": execPath,
			"args":    []string{"mcp"},
		}
	})
}

func uninstallFromGenericMCPServers(path string) error {
	return mutateJSONMap(path, func(doc map[string]interface{}) {
		if mcpServers, ok := doc["mcpServers"].(map[string]interface{}); ok {
			delete(mcpServers, "forge")
		}
	})
}

func installToClaude(path string, execPath string) error {
	return mutateJSONMap(path, func(doc map[string]interface{}) {
		mcpServers := getOrCreateMap(doc, "mcpServers")
		mcpServers["forge"] = map[string]interface{}{
			"type":    "stdio",
			"command": execPath,
			"args":    []string{"mcp"},
		}
	})
}

func uninstallFromClaude(path string) error {
	return mutateJSONMap(path, func(doc map[string]interface{}) {
		if mcpServers, ok := doc["mcpServers"].(map[string]interface{}); ok {
			delete(mcpServers, "forge")
		}
	})
}

func installToOpencode(path string, execPath string) error {
	return mutateJSONMap(path, func(doc map[string]interface{}) {
		mcp := getOrCreateMap(doc, "mcp")
		mcp["forge"] = map[string]interface{}{
			"type":    "local",
			"command": []string{execPath, "mcp"},
		}
	})
}

func uninstallFromOpencode(path string) error {
	return mutateJSONMap(path, func(doc map[string]interface{}) {
		if mcp, ok := doc["mcp"].(map[string]interface{}); ok {
			delete(mcp, "forge")
		}
	})
}

func installToCodex(path string, execPath string) error {
	if dir := filepath.Dir(path); dir != "" {
		_ = os.MkdirAll(dir, 0o755)
	}

	content := ""
	if fileExists(path) {
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		content = string(b)
	}

	block := fmt.Sprintf("\n[mcp_servers.forge]\ncommand = %q\nargs = [\"mcp\"]\n", execPath)

	if strings.Contains(content, "[mcp_servers.forge]") {
		// Replace existing [mcp_servers.forge] block
		re := regexp.MustCompile(`(?s)\[mcp_servers\.forge\].*?(\n\[|\z)`)
		if re.MatchString(content) {
			content = re.ReplaceAllString(content, strings.TrimPrefix(block, "\n")+"$1")
		} else {
			content += block
		}
	} else {
		content += block
	}

	return atomicWriteFile(path, []byte(content))
}

func uninstallFromCodex(path string) error {
	if !fileExists(path) {
		return nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	content := string(b)
	if !strings.Contains(content, "[mcp_servers.forge]") {
		return nil
	}

	re := regexp.MustCompile(`(?s)\n?\[mcp_servers\.forge\].*?(\n\[|\z)`)
	content = re.ReplaceAllString(content, "$1")
	return atomicWriteFile(path, []byte(content))
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func mutateJSONMap(path string, mutate func(doc map[string]interface{})) error {
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	doc := make(map[string]interface{})
	if fileExists(path) {
		data, err := os.ReadFile(path)
		if err == nil && len(strings.TrimSpace(string(data))) > 0 {
			_ = json.Unmarshal([]byte(stripComments(string(data))), &doc)
		}
	}

	mutate(doc)

	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')
	return atomicWriteFile(path, out)
}

func getOrCreateMap(parent map[string]interface{}, key string) map[string]interface{} {
	if existing, ok := parent[key].(map[string]interface{}); ok && existing != nil {
		return existing
	}
	created := make(map[string]interface{})
	parent[key] = created
	return created
}

func atomicWriteFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tmp-*.json")
	if err != nil {
		// Fallback to direct write if temp file creation fails
		return os.WriteFile(path, data, 0o644)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func isExecutableOnPath(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func stripComments(data string) string {
	var b strings.Builder
	b.Grow(len(data))
	inString := false
	escaped := false
	for i := 0; i < len(data); i++ {
		c := data[i]
		if inString {
			b.WriteByte(c)
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == '"' {
				inString = false
			}
			continue
		}
		switch {
		case c == '"':
			inString = true
			b.WriteByte(c)
		case c == '/' && i+1 < len(data) && data[i+1] == '/':
			for i < len(data) && data[i] != '\n' {
				i++
			}
			if i < len(data) {
				b.WriteByte('\n')
			}
		case c == '/' && i+1 < len(data) && data[i+1] == '*':
			i += 2
			for i+1 < len(data) && !(data[i] == '*' && data[i+1] == '/') {
				i++
			}
			i++
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}
