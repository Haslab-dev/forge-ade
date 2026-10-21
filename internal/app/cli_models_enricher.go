package app

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Haslab-dev/RakitKode/agentclirc"
	"gopkg.in/yaml.v3"
)

// enrichAllAgentRuntimeConfigs ensures that OpenCode, Pi, OMP, and Command Code
// always expose their real, on-disk/CLI configured models instead of stale or empty lists.
func enrichAllAgentRuntimeConfigs(configs []agentclirc.RuntimeConfig) []agentclirc.RuntimeConfig {
	home, err := os.UserHomeDir()
	if err != nil {
		return configs
	}

	hasCmd := false
	for i := range configs {
		c := &configs[i]
		switch c.ID {
		case "opencode":
			enrichOpencode(c, home)
		case "pi":
			enrichPi(c, home)
		case "ohmypi", "omp":
			enrichOmp(c, home)
		case "commandcode", "cmd":
			hasCmd = true
		}
	}

	if !hasCmd {
		if cmdCfg := commandCodeRuntimeConfig(); cmdCfg != nil {
			configs = append(configs, *cmdCfg)
		}
	}

	return configs
}

func enrichOpencode(c *agentclirc.RuntimeConfig, home string) {
	configDirs := []string{
		filepath.Join(home, ".config", "opencode"),
		filepath.Join(home, ".opencode"),
	}

	modelSet := make(map[string]bool)
	var modelList []string
	addModel := func(m string) {
		m = strings.TrimSpace(m)
		if m != "" && !modelSet[m] {
			modelSet[m] = true
			modelList = append(modelList, m)
		}
	}

	for _, dir := range configDirs {
		for _, name := range []string{"opencode.jsonc", "opencode.json"} {
			path := filepath.Join(dir, name)
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			clean := stripComments(string(data))
			var doc struct {
				Model    string `json:"model"`
				Provider map[string]struct {
					Name    string `json:"name"`
					Options struct {
						BaseURL string `json:"baseURL"`
					} `json:"options"`
					Models map[string]interface{} `json:"models"`
				} `json:"provider"`
			}
			if err := json.Unmarshal([]byte(clean), &doc); err == nil {
				if doc.Model != "" && c.DefaultModel == "" {
					c.DefaultModel = doc.Model
					addModel(doc.Model)
				}
				for provID, prov := range doc.Provider {
					for modelID := range prov.Models {
						addModel(provID + "/" + modelID)
						addModel(modelID)
					}
				}
			}
		}
	}

	// Also execute `opencode models` with short timeout if binary exists
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cmdPath := "opencode"
	if found, err := exec.LookPath("opencode"); err == nil {
		cmdPath = found
	} else if fi, err := os.Stat(filepath.Join(home, ".opencode", "bin", "opencode")); err == nil && !fi.IsDir() {
		cmdPath = filepath.Join(home, ".opencode", "bin", "opencode")
	}

	if out, err := exec.CommandContext(ctx, cmdPath, "models").Output(); err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			line = strings.TrimSpace(line)
			if line != "" && !strings.Contains(line, " ") {
				addModel(line)
			}
		}
	}

	if len(modelList) > 0 {
		hasProv := false
		for i := range c.Providers {
			if c.Providers[i].ID == "opencode" || strings.EqualFold(c.Providers[i].Name, "opencode") {
				c.Providers[i].Models = modelList
				hasProv = true
				break
			}
		}
		if !hasProv {
			c.Providers = append(c.Providers, agentclirc.ProviderInfo{
				ID:     "opencode",
				Name:   "OpenCode",
				Models: modelList,
			})
		}
		if c.DefaultModel == "" {
			c.DefaultModel = modelList[0]
		}
	}
}

func enrichPi(c *agentclirc.RuntimeConfig, home string) {
	dir := filepath.Join(home, ".pi", "agent")
	modelsPath := filepath.Join(dir, "models.json")

	modelSet := make(map[string]bool)
	var modelList []string
	addModel := func(m string) {
		m = strings.TrimSpace(m)
		if m != "" && !modelSet[m] {
			modelSet[m] = true
			modelList = append(modelList, m)
		}
	}

	if data, err := os.ReadFile(modelsPath); err == nil {
		var doc struct {
			Providers map[string]struct {
				Name    string `json:"name"`
				BaseURL string `json:"baseUrl"`
				Models  []struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				} `json:"models"`
			} `json:"providers"`
		}
		if err := json.Unmarshal(data, &doc); err == nil {
			for provID, prov := range doc.Providers {
				for _, m := range prov.Models {
					id := m.ID
					if id == "" {
						id = m.Name
					}
					if id != "" {
						addModel(provID + "/" + id)
						addModel(id)
					}
				}
			}
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cmdPath := "pi"
	if found, err := exec.LookPath("pi"); err == nil {
		cmdPath = found
	}
	if out, err := exec.CommandContext(ctx, cmdPath, "--list-models").Output(); err == nil {
		lines := strings.Split(string(out), "\n")
		for _, l := range lines {
			fields := strings.Fields(l)
			if len(fields) >= 2 && fields[0] != "provider" && fields[1] != "model" {
				prov := fields[0]
				mod := fields[1]
				addModel(prov + "/" + mod)
				addModel(mod)
			}
		}
	}

	if len(modelList) > 0 {
		hasProv := false
		for i := range c.Providers {
			if c.Providers[i].ID == "pi" || strings.EqualFold(c.Providers[i].Name, "pi") {
				c.Providers[i].Models = modelList
				hasProv = true
				break
			}
		}
		if !hasProv {
			c.Providers = append(c.Providers, agentclirc.ProviderInfo{
				ID:     "pi",
				Name:   "Pi Agent",
				Models: modelList,
			})
		}
		if c.DefaultModel == "" {
			c.DefaultModel = modelList[0]
		}
	}
}

func enrichOmp(c *agentclirc.RuntimeConfig, home string) {
	dir := filepath.Join(home, ".omp", "agent")

	modelSet := make(map[string]bool)
	var modelList []string
	addModel := func(m string) {
		m = strings.TrimSpace(m)
		if m != "" && !modelSet[m] {
			modelSet[m] = true
			modelList = append(modelList, m)
		}
	}

	// 1. Read config.yml for modelRoles.default
	configPath := filepath.Join(dir, "config.yml")
	if data, err := os.ReadFile(configPath); err == nil {
		var doc struct {
			Model      string `yaml:"model"`
			ModelRoles struct {
				Default string `yaml:"default"`
			} `yaml:"modelRoles"`
		}
		if err := yaml.Unmarshal(data, &doc); err == nil {
			if doc.ModelRoles.Default != "" {
				c.DefaultModel = doc.ModelRoles.Default
				addModel(doc.ModelRoles.Default)
			} else if doc.Model != "" {
				c.DefaultModel = doc.Model
				addModel(doc.Model)
			}
		}
	}

	// 2. Read models.yml
	modelsPath := filepath.Join(dir, "models.yml")
	if data, err := os.ReadFile(modelsPath); err == nil {
		var doc struct {
			Providers map[string]struct {
				Models []struct {
					ID string `yaml:"id"`
				} `yaml:"models"`
			} `yaml:"providers"`
		}
		if err := yaml.Unmarshal(data, &doc); err == nil {
			for provID, prov := range doc.Providers {
				for _, m := range prov.Models {
					if m.ID != "" {
						addModel(provID + "/" + m.ID)
						addModel(m.ID)
					}
				}
			}
		}
	}

	if len(modelList) > 0 {
		hasProv := false
		for i := range c.Providers {
			if c.Providers[i].ID == "ohmypi" || c.Providers[i].ID == "omp" {
				c.Providers[i].Models = modelList
				hasProv = true
				break
			}
		}
		if !hasProv {
			c.Providers = append(c.Providers, agentclirc.ProviderInfo{
				ID:     "ohmypi",
				Name:   "OhMyPi",
				Models: modelList,
			})
		}
		if c.DefaultModel == "" {
			c.DefaultModel = modelList[0]
		}
	}
}

// stripComments removes // and /* */ comments from JSONC text.
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
