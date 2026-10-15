package mcp

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// PresetEnvVar describes a configurable variable (API key, token, URL) for an MCP preset.
type PresetEnvVar struct {
	Key          string `json:"key"`
	Label        string `json:"label"`
	Placeholder  string `json:"placeholder"`
	Required     bool   `json:"required"`
	Secret       bool   `json:"secret"`
	DefaultValue string `json:"default_value,omitempty"`
	DetectedFrom string `json:"detected_from,omitempty"`
}

// PresetConfig is a ready-to-use template for setting up popular MCP servers.
type PresetConfig struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Label       string         `json:"label"`
	Description string         `json:"description"`
	Command     string         `json:"command"`
	Args        []string       `json:"args"`
	EnvVars     []PresetEnvVar `json:"env_vars"`
	WebsiteURL  string         `json:"website_url,omitempty"`
	Icon        string         `json:"icon,omitempty"`
}

// ListDefaultPresets returns curated MCP server templates, auto-detecting existing keys from
// environment variables or agent config files (e.g. Antigravity, Claude Desktop).
func ListDefaultPresets() []PresetConfig {
	detectedKeys := detectExistingKeys()

	presets := []PresetConfig{
		{
			ID:          "exa",
			Name:        "exa",
			Label:       "Exa Search",
			Description: "Neural search engine & live web retrieval designed for autonomous AI agents.",
			Command:     "npx",
			Args:        []string{"-y", "exa-mcp-server"},
			WebsiteURL:  "https://dashboard.exa.ai/api-keys",
			Icon:        "search",
			EnvVars: []PresetEnvVar{
				{
					Key:          "EXA_API_KEY",
					Label:        "Exa API Key",
					Placeholder:  "Paste your Exa API key (e.g. d329ba74-...)",
					Required:     true,
					Secret:       true,
					DefaultValue: detectedKeys["EXA_API_KEY"].Value,
					DetectedFrom: detectedKeys["EXA_API_KEY"].Source,
				},
			},
		},
		{
			ID:          "firecrawl",
			Name:        "firecrawl",
			Label:       "Firecrawl",
			Description: "Turn any website into LLM-ready clean Markdown or structured JSON.",
			Command:     "npx",
			Args:        []string{"-y", "firecrawl-mcp"},
			WebsiteURL:  "https://www.firecrawl.dev/app/api-keys",
			Icon:        "flame",
			EnvVars: []PresetEnvVar{
				{
					Key:          "FIRECRAWL_API_KEY",
					Label:        "Firecrawl API Key",
					Placeholder:  "Paste your Firecrawl API key (e.g. fc-...)",
					Required:     true,
					Secret:       true,
					DefaultValue: detectedKeys["FIRECRAWL_API_KEY"].Value,
					DetectedFrom: detectedKeys["FIRECRAWL_API_KEY"].Source,
				},
			},
		},
		{
			ID:          "figma",
			Name:        "figma",
			Label:       "Figma Developer",
			Description: "Inspect Figma design files, extract layout tokens, nodes, and UI component assets.",
			Command:     "npx",
			Args:        []string{"-y", "figma-developer-mcp", "--stdio"},
			WebsiteURL:  "https://www.figma.com/developers/api#access-tokens",
			Icon:        "palette",
			EnvVars: []PresetEnvVar{
				{
					Key:          "FIGMA_API_KEY",
					Label:        "Figma Personal Access Token",
					Placeholder:  "Paste Figma token (e.g. figd_...)",
					Required:     true,
					Secret:       true,
					DefaultValue: detectedKeys["FIGMA_API_KEY"].Value,
					DetectedFrom: detectedKeys["FIGMA_API_KEY"].Source,
				},
			},
		},
		{
			ID:          "atlassian",
			Name:        "atlassian",
			Label:       "Atlassian (Jira & Confluence)",
			Description: "Manage Jira issues, search tickets by JQL, view sprint boards, and read/write Confluence pages.",
			Command:     "uvx",
			Args:        []string{"mcp-atlassian"},
			WebsiteURL:  "https://id.atlassian.com/manage-profile/security/api-tokens",
			Icon:        "ticket",
			EnvVars: []PresetEnvVar{
				{
					Key:          "JIRA_URL",
					Label:        "Jira Instance URL",
					Placeholder:  "https://your-domain.atlassian.net",
					Required:     true,
					Secret:       false,
					DefaultValue: detectedKeys["JIRA_URL"].Value,
					DetectedFrom: detectedKeys["JIRA_URL"].Source,
				},
				{
					Key:          "JIRA_USERNAME",
					Label:        "Atlassian Account Username / Email",
					Placeholder:  "engineer@company.com",
					Required:     true,
					Secret:       false,
					DefaultValue: detectedKeys["JIRA_USERNAME"].Value,
					DetectedFrom: detectedKeys["JIRA_USERNAME"].Source,
				},
				{
					Key:          "JIRA_API_TOKEN",
					Label:        "Atlassian API Token",
					Placeholder:  "Paste Atlassian API token",
					Required:     true,
					Secret:       true,
					DefaultValue: detectedKeys["JIRA_API_TOKEN"].Value,
					DetectedFrom: detectedKeys["JIRA_API_TOKEN"].Source,
				},
				{
					Key:          "CONFLUENCE_URL",
					Label:        "Confluence URL (Optional)",
					Placeholder:  "https://your-domain.atlassian.net/wiki",
					Required:     false,
					Secret:       false,
					DefaultValue: detectedKeys["CONFLUENCE_URL"].Value,
					DetectedFrom: detectedKeys["CONFLUENCE_URL"].Source,
				},
				{
					Key:          "CONFLUENCE_USERNAME",
					Label:        "Confluence Username (Optional)",
					Placeholder:  "Defaults to Jira username / email",
					Required:     false,
					Secret:       false,
					DefaultValue: detectedKeys["CONFLUENCE_USERNAME"].Value,
					DetectedFrom: detectedKeys["CONFLUENCE_USERNAME"].Source,
				},
				{
					Key:          "CONFLUENCE_API_TOKEN",
					Label:        "Confluence API Token (Optional)",
					Placeholder:  "Defaults to Jira API token",
					Required:     false,
					Secret:       true,
					DefaultValue: detectedKeys["CONFLUENCE_API_TOKEN"].Value,
					DetectedFrom: detectedKeys["CONFLUENCE_API_TOKEN"].Source,
				},
			},
		},
		{
			ID:          "github",
			Name:        "github",
			Label:       "GitHub",
			Description: "Interact with GitHub repositories, pull requests, issues, and commit trees.",
			Command:     "npx",
			Args:        []string{"-y", "@modelcontextprotocol/server-github"},
			WebsiteURL:  "https://github.com/settings/tokens",
			Icon:        "github",
			EnvVars: []PresetEnvVar{
				{
					Key:          "GITHUB_PERSONAL_ACCESS_TOKEN",
					Label:        "GitHub Personal Access Token",
					Placeholder:  "ghp_...",
					Required:     true,
					Secret:       true,
					DefaultValue: detectedKeys["GITHUB_PERSONAL_ACCESS_TOKEN"].Value,
					DetectedFrom: detectedKeys["GITHUB_PERSONAL_ACCESS_TOKEN"].Source,
				},
			},
		},
		{
			ID:          "context7",
			Name:        "context7",
			Label:       "Context7 Docs",
			Description: "Version-accurate library documentation for popular open-source packages and frameworks.",
			Command:     "npx",
			Args:        []string{"-y", "@upstash/context7-mcp"},
			WebsiteURL:  "https://context7.com",
			Icon:        "book-open",
			EnvVars: []PresetEnvVar{
				{
					Key:          "CONTEXT7_API_KEY",
					Label:        "Context7 API Key (Optional)",
					Placeholder:  "Optional - leave blank for free tier",
					Required:     false,
					Secret:       true,
					DefaultValue: detectedKeys["CONTEXT7_API_KEY"].Value,
					DetectedFrom: detectedKeys["CONTEXT7_API_KEY"].Source,
				},
			},
		},
	}

	return presets
}

type detectedVal struct {
	Value  string
	Source string
}

func detectExistingKeys() map[string]detectedVal {
	out := make(map[string]detectedVal)

	register := func(key, val, src string) {
		val = strings.TrimSpace(val)
		if val != "" {
			if _, exists := out[key]; !exists {
				out[key] = detectedVal{Value: val, Source: src}
			}
		}
	}

	// 1. Check OS process environment first
	checkEnv := func(key string) {
		if v := os.Getenv(key); v != "" {
			register(key, v, "System environment variable")
		}
	}
	checkEnv("EXA_API_KEY")
	checkEnv("FIRECRAWL_API_KEY")
	checkEnv("FIGMA_API_KEY")
	checkEnv("JIRA_URL")
	checkEnv("JIRA_BASE_URL")
	checkEnv("JIRA_USERNAME")
	checkEnv("JIRA_EMAIL")
	checkEnv("JIRA_API_TOKEN")
	checkEnv("JIRA_TOKEN")
	checkEnv("CONFLUENCE_URL")
	checkEnv("CONFLUENCE_BASE_URL")
	checkEnv("CONFLUENCE_USERNAME")
	checkEnv("CONFLUENCE_EMAIL")
	checkEnv("CONFLUENCE_API_TOKEN")
	checkEnv("CONFLUENCE_TOKEN")
	checkEnv("GITHUB_PERSONAL_ACCESS_TOKEN")
	checkEnv("GITHUB_TOKEN")
	checkEnv("CONTEXT7_API_KEY")

	// 2. Scan agent config files
	home, _ := os.UserHomeDir()
	if home != "" {
		type serverDef struct {
			Command   string            `json:"command"`
			Args      []string          `json:"args"`
			Env       map[string]string `json:"env"`
			URL       string            `json:"url"`
			ServerURL string            `json:"serverUrl"`
		}

		type configShape struct {
			MCPServers map[string]serverDef `json:"mcpServers"`
			Projects   map[string]struct {
				MCPServers map[string]serverDef `json:"mcpServers"`
			} `json:"projects"`
		}

		candidatePaths := []struct {
			path   string
			source string
		}{
			{filepath.Join(home, ".gemini", "config", "mcp_config.json"), "Google Antigravity"},
			{filepath.Join(home, ".gemini", "antigravity", "mcp_config.json"), "Google Antigravity"},
			{filepath.Join(home, ".gemini", "antigravity-cli", "mcp_config.json"), "Google Antigravity"},
			{filepath.Join(home, ".gemini", "antigravity-ide", "mcp_config.json"), "Google Antigravity"},
			{filepath.Join(home, ".claude.json"), "Claude Code"},
			{filepath.Join(home, ".claude", "mcp.json"), "Claude Code"},
			{filepath.Join(home, "Library", "Application Support", "Claude", "claude_desktop_config.json"), "Claude Desktop"},
			{filepath.Join(home, ".cursor", "mcp.json"), "Cursor"},
		}

		for _, cand := range candidatePaths {
			data, err := os.ReadFile(cand.path)
			if err != nil {
				continue
			}
			var cfg configShape
			if json.Unmarshal(data, &cfg) != nil {
				continue
			}

			extractServer := func(name string, s serverDef) {
				src := fmt.Sprintf("Detected from %s config (%s)", cand.source, name)

				// Figma
				for _, arg := range s.Args {
					if strings.HasPrefix(arg, "--figma-api-key=") {
						register("FIGMA_API_KEY", strings.TrimPrefix(arg, "--figma-api-key="), src)
					}
				}
				if val, ok := s.Env["FIGMA_API_KEY"]; ok {
					register("FIGMA_API_KEY", val, src)
				}

				// Exa
				u := s.URL
				if u == "" {
					u = s.ServerURL
				}
				if u != "" {
					if parsed, err := url.Parse(u); err == nil {
						if key := parsed.Query().Get("exaApiKey"); key != "" {
							register("EXA_API_KEY", key, src)
						}
					}
				}
				if val, ok := s.Env["EXA_API_KEY"]; ok {
					register("EXA_API_KEY", val, src)
				}

				// Firecrawl
				if val, ok := s.Env["FIRECRAWL_API_KEY"]; ok {
					register("FIRECRAWL_API_KEY", val, src)
				}

				// GitHub
				if val, ok := s.Env["GITHUB_PERSONAL_ACCESS_TOKEN"]; ok {
					register("GITHUB_PERSONAL_ACCESS_TOKEN", val, src)
				} else if val, ok := s.Env["GITHUB_TOKEN"]; ok {
					register("GITHUB_PERSONAL_ACCESS_TOKEN", val, src)
				}

				// Context7
				if val, ok := s.Env["CONTEXT7_API_KEY"]; ok {
					register("CONTEXT7_API_KEY", val, src)
				}

				// Jira & Atlassian
				if val, ok := s.Env["JIRA_URL"]; ok {
					register("JIRA_URL", val, src)
				} else if val, ok := s.Env["JIRA_BASE_URL"]; ok {
					register("JIRA_URL", val, src)
				}

				if val, ok := s.Env["JIRA_USERNAME"]; ok {
					register("JIRA_USERNAME", val, src)
				} else if val, ok := s.Env["JIRA_EMAIL"]; ok {
					register("JIRA_USERNAME", val, src)
				}

				if val, ok := s.Env["JIRA_API_TOKEN"]; ok {
					register("JIRA_API_TOKEN", val, src)
				} else if val, ok := s.Env["JIRA_TOKEN"]; ok {
					register("JIRA_API_TOKEN", val, src)
				}

				// Confluence
				if val, ok := s.Env["CONFLUENCE_URL"]; ok {
					register("CONFLUENCE_URL", val, src)
				} else if val, ok := s.Env["CONFLUENCE_BASE_URL"]; ok {
					register("CONFLUENCE_URL", val, src)
				}

				if val, ok := s.Env["CONFLUENCE_USERNAME"]; ok {
					register("CONFLUENCE_USERNAME", val, src)
				} else if val, ok := s.Env["CONFLUENCE_EMAIL"]; ok {
					register("CONFLUENCE_USERNAME", val, src)
				}

				if val, ok := s.Env["CONFLUENCE_API_TOKEN"]; ok {
					register("CONFLUENCE_API_TOKEN", val, src)
				} else if val, ok := s.Env["CONFLUENCE_TOKEN"]; ok {
					register("CONFLUENCE_API_TOKEN", val, src)
				}
			}

			for name, s := range cfg.MCPServers {
				extractServer(name, s)
			}
			for _, p := range cfg.Projects {
				for name, s := range p.MCPServers {
					extractServer(name, s)
				}
			}
		}
	}

	// 3. Aliasing and inheritance fallbacks
	if _, ok := out["JIRA_USERNAME"]; !ok {
		if v, exists := out["JIRA_EMAIL"]; exists {
			out["JIRA_USERNAME"] = v
		}
	}
	if _, ok := out["JIRA_EMAIL"]; !ok {
		if v, exists := out["JIRA_USERNAME"]; exists {
			out["JIRA_EMAIL"] = v
		}
	}
	if _, ok := out["CONFLUENCE_USERNAME"]; !ok {
		if v, exists := out["JIRA_USERNAME"]; exists {
			out["CONFLUENCE_USERNAME"] = detectedVal{
				Value:  v.Value,
				Source: "Inherited from Jira credentials (" + v.Source + ")",
			}
		}
	}
	if _, ok := out["CONFLUENCE_API_TOKEN"]; !ok {
		if v, exists := out["JIRA_API_TOKEN"]; exists {
			out["CONFLUENCE_API_TOKEN"] = detectedVal{
				Value:  v.Value,
				Source: "Inherited from Jira credentials (" + v.Source + ")",
			}
		}
	}
	if _, ok := out["CONFLUENCE_URL"]; !ok {
		if v, exists := out["JIRA_URL"]; exists {
			clean := strings.TrimRight(v.Value, "/")
			if clean != "" && !strings.HasSuffix(clean, "/wiki") {
				out["CONFLUENCE_URL"] = detectedVal{
					Value:  clean + "/wiki",
					Source: "Inferred from Jira instance URL",
				}
			}
		}
	}

	return out
}
