package app

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Haslab-dev/RakitKode/agentclirc"
	"github.com/hasdev/forge-ade/internal/agentsession"
)

func commandCodeRuntimeConfig() *agentclirc.RuntimeConfig {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	configRoot := filepath.Join(home, ".commandcode")

	installed := agentsession.DetectExecutable("cmd")
	rootExists := false
	if fi, err := os.Stat(configRoot); err == nil && fi.IsDir() {
		rootExists = true
	}

	if !installed && !rootExists {
		return nil
	}

	// 1. Read default/current model from config.json
	defaultModel := "deepseek/deepseek-v4-flash"
	configPath := filepath.Join(configRoot, "config.json")
	if data, err := os.ReadFile(configPath); err == nil {
		var doc struct {
			Model string `json:"model"`
		}
		if err := json.Unmarshal(data, &doc); err == nil && doc.Model != "" {
			defaultModel = doc.Model
		}
	}

	// 2. Discover configuration paths
	knownPaths := []struct {
		purpose string
		path    string
	}{
		{"config", filepath.Join(configRoot, "config.json")},
		{"settings", filepath.Join(configRoot, "settings.json")},
		{"mcp", filepath.Join(configRoot, "mcp.json")},
		{"auth", filepath.Join(configRoot, "auth.json")},
		{"skills", filepath.Join(configRoot, "skills")},
		{"plans", filepath.Join(configRoot, "plans")},
	}
	var paths []agentclirc.ConfigPath
	for _, kp := range knownPaths {
		_, statErr := os.Stat(kp.path)
		paths = append(paths, agentclirc.ConfigPath{
			Purpose: kp.purpose,
			Path:    kp.path,
			Exists:  statErr == nil,
		})
	}

	// 3. Extract real model catalog
	models := discoverCommandCodeModels(configRoot)

	var providers []agentclirc.ProviderInfo
	if len(models) > 0 {
		providers = append(providers, agentclirc.ProviderInfo{
			ID:     "commandcode",
			Name:   "Command Code Catalog",
			Models: models,
		})
	}

	// 4. Extract MCP servers from mcp.json
	var mcpServers []agentclirc.MCPServer
	mcpPath := filepath.Join(configRoot, "mcp.json")
	if mcpData, err := os.ReadFile(mcpPath); err == nil {
		var doc struct {
			MCPServers map[string]struct {
				Command   string `json:"command"`
				Transport string `json:"transport"`
				URL       string `json:"url"`
			} `json:"mcpServers"`
		}
		if err := json.Unmarshal(mcpData, &doc); err == nil {
			for name, s := range doc.MCPServers {
				sType := "local"
				if s.Transport == "sse" || s.Transport == "http" || s.URL != "" {
					sType = "remote"
				}
				mcpServers = append(mcpServers, agentclirc.MCPServer{
					Name:    name,
					Type:    sType,
					Command: s.Command,
					URL:     s.URL,
				})
			}
		}
	}

	// 5. Extract skills from skills dir
	var skills []agentclirc.Skill
	skillsDir := filepath.Join(configRoot, "skills")
	if entries, err := os.ReadDir(skillsDir); err == nil {
		for _, e := range entries {
			skills = append(skills, agentclirc.Skill{
				Name: e.Name(),
				Path: filepath.Join(skillsDir, e.Name()),
			})
		}
	}

	return &agentclirc.RuntimeConfig{
		ID:           "commandcode",
		Name:         "Command Code",
		Executable:   "cmd",
		Installed:    installed,
		Platform:     "local",
		DefaultModel: defaultModel,
		ConfigRoot:   configRoot,
		Paths:        paths,
		Providers:    providers,
		MCPServers:   mcpServers,
		Skills:       skills,
	}
}

// discoverCommandCodeModels finds real models from catalog cache or by running `cmd --list-models`.
func discoverCommandCodeModels(configRoot string) []string {
	seen := make(map[string]bool)
	var result []string

	addModel := func(m string) {
		m = strings.TrimSpace(m)
		if m != "" && !seen[m] {
			seen[m] = true
			result = append(result, m)
		}
	}

	// Strategy A: Read cached catalog JSON files from ~/.commandcode/cache/model-catalog/v1/
	cacheDir := filepath.Join(configRoot, "cache", "model-catalog", "v1")
	if entries, err := os.ReadDir(cacheDir); err == nil {
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), "catalog-") && strings.HasSuffix(e.Name(), ".json") {
				catalogPath := filepath.Join(cacheDir, e.Name())
				if content, err := os.ReadFile(catalogPath); err == nil {
					var cat struct {
						Body struct {
							Data []struct {
								ID string `json:"id"`
							} `json:"data"`
						} `json:"body"`
					}
					if err := json.Unmarshal(content, &cat); err == nil {
						for _, item := range cat.Body.Data {
							addModel(item.ID)
						}
					}
				}
			}
		}
	}

	if len(result) > 0 {
		return result
	}

	// Strategy B: Run `cmd --list-models` if available
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "cmd", "--list-models")
	out, err := cmd.Output()
	if err == nil && len(out) > 0 {
		lines := strings.Split(string(out), "\n")
		// Match lines that start with model identifier (e.g. deepseek/deepseek-v4-flash, claude-sonnet-5-5, etc.)
		modelLineRe := regexp.MustCompile(`^([a-zA-Z0-9_\-\.\:\/]+)\s+`)
		for _, l := range lines {
			l = strings.TrimSpace(l)
			if l == "" || strings.HasPrefix(l, "Available models") || strings.HasPrefix(l, "Pass the full id") || strings.HasPrefix(l, "Docs:") || strings.HasPrefix(l, "cmd --model") {
				continue
			}
			matches := modelLineRe.FindStringSubmatch(l)
			if len(matches) > 1 {
				m := matches[1]
				if !strings.HasPrefix(m, "http") && !strings.Contains(l, "Decision models") {
					addModel(m)
				}
			}
		}
	}

	return result
}
