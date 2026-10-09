// Package agentclirc reads the on-disk configuration of external agent CLIs
// (pi, omp, opencode, codex, claude, antigravity, gemini) so ForgeADE can
// surface and manage their providers, models, MCP servers, and skills.
//
// Paths were verified against each tool's documentation for macOS and Windows
// (home-relative paths are identical on both; Windows resolves ~/.x to
// %USERPROFILE%\.x). Reading never mutates anything — editing happens through
// the editor/reveal actions on the reported file paths.
package agentclirc

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"
)

// ConfigPath is one known file/dir of a CLI's configuration.
type ConfigPath struct {
	Purpose string `json:"purpose"` // config | skills | agents | auth | data | extensions
	Path    string `json:"path"`
	Exists  bool   `json:"exists"`
}

// ProviderInfo is a model provider configured inside a CLI.
type ProviderInfo struct {
	ID      string   `json:"id"`
	Name    string   `json:"name,omitempty"`
	BaseURL string   `json:"baseUrl,omitempty"`
	Models  []string `json:"models,omitempty"`
}

// MCPServer is an MCP server entry configured inside a CLI.
type MCPServer struct {
	Name    string `json:"name"`
	Type    string `json:"type"` // local | remote
	Command string `json:"command,omitempty"`
	URL     string `json:"url,omitempty"`
}

// Skill is one skill directory (SKILL.md) or equivalent discovered for a CLI.
type Skill struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Path        string `json:"path,omitempty"`
}

// RuntimeConfig is everything ForgeADE knows about one CLI's local config.
type RuntimeConfig struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Executable   string `json:"executable"`
	Installed    bool   `json:"installed"`
	Platform     string `json:"platform"`
	DefaultModel string `json:"defaultModel,omitempty"`
	// ConfigRoot is the CLI's global config directory — the one-stop folder
	// to open in an editor when managing everything for this CLI.
	ConfigRoot string         `json:"configRoot"`
	Paths      []ConfigPath   `json:"paths"`
	Providers  []ProviderInfo `json:"providers"`
	MCPServers []MCPServer    `json:"mcpServers"`
	Skills     []Skill        `json:"skills"`
	Error      string         `json:"error,omitempty"`
}

// Runtime error wrapper so one broken config never hides the others.
func (rc *RuntimeConfig) addError(err error) {
	if err == nil {
		return
	}
	if rc.Error == "" {
		rc.Error = err.Error()
	} else {
		rc.Error += "; " + err.Error()
	}
}

func homeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}

func xdgConfigDir() string {
	if v := os.Getenv("XDG_CONFIG_HOME"); strings.TrimSpace(v) != "" {
		return v
	}
	return filepath.Join(homeDir(), ".config")
}

// opencodeConfigDir resolves opencode's global config directory, honoring
// XDG_CONFIG_HOME but falling back to the given home (the documented layout
// on both macOS and Windows).
func opencodeConfigDir(home string) string {
	xdg := xdgConfigDir()
	if xdg == filepath.Join(homeDir(), ".config") {
		return filepath.Join(home, ".config", "opencode")
	}
	return filepath.Join(xdg, "opencode")
}

// codexConfigDir honors the CODEX_HOME override documented by the Codex CLI,
// falling back to <home>/.codex. Takes the home explicitly so callers (and
// tests) can point it at any root.
func codexConfigDir(home string) string {
	if v := strings.TrimSpace(os.Getenv("CODEX_HOME")); v != "" {
		return v
	}
	return filepath.Join(home, ".codex")
}

func markExists(paths []ConfigPath) {
	for i := range paths {
		_, err := os.Stat(paths[i].Path)
		paths[i].Exists = err == nil
	}
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

func dirExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

func installed(exe string) bool {
	_, err := lookPathEnriched(exe)
	return err == nil
}

// lookPathEnriched resolves executables even when the GUI process PATH is
// minimal (macOS Finder launches): common tool bins are probed explicitly.
func lookPathEnriched(exe string) (string, error) {
	if exe == "" {
		return "", fmt.Errorf("empty executable")
	}
	if strings.ContainsAny(exe, "/\\") {
		if fileExists(exe) {
			return exe, nil
		}
		return "", fmt.Errorf("not found: %s", exe)
	}
	if p, err := execLookPath(exe); err == nil {
		return p, nil
	}
	home := homeDir()
	candidates := []string{
		"/opt/homebrew/bin", "/usr/local/bin", "/usr/bin",
		filepath.Join(home, ".cargo", "bin"),
		filepath.Join(home, "go", "bin"),
		filepath.Join(home, ".bun", "bin"),
		filepath.Join(home, ".local", "bin"),
	}
	if runtime.GOOS == "windows" {
		candidates = []string{
			filepath.Join(home, "AppData", "Local", "Programs"),
			filepath.Join(home, "go", "bin"),
		}
	}
	for _, dir := range candidates {
		p := filepath.Join(dir, exe)
		if runtime.GOOS == "windows" {
			p += ".exe"
		}
		if fileExists(p) {
			return p, nil
		}
	}
	return "", fmt.Errorf("%q not found in PATH", exe)
}

// ---------------------------------------------------------------------------
// JSON / JSONC / TOML / YAML helpers
// ---------------------------------------------------------------------------

func stripJSONComments(data string) string {
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

func readJSON(path string) (map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var v map[string]interface{}
	if err := json.Unmarshal([]byte(stripJSONComments(string(data))), &v); err != nil {
		return nil, fmt.Errorf("parse %s: %w", filepath.Base(path), err)
	}
	return v, nil
}

func readYAML(path string) (map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var v map[string]interface{}
	if err := yaml.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("parse %s: %w", filepath.Base(path), err)
	}
	return v, nil
}

func readTOML(path string) (map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var v map[string]interface{}
	if err := toml.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("parse %s: %w", filepath.Base(path), err)
	}
	return v, nil
}

func asMap(v interface{}) map[string]interface{} {
	m, _ := v.(map[string]interface{})
	return m
}

func asString(v interface{}) string {
	s, _ := v.(string)
	return s
}

func asStringSlice(v interface{}) []string {
	switch t := v.(type) {
	case []string:
		return t
	case []interface{}:
		out := make([]string, 0, len(t))
		for _, item := range t {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func sortedKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
