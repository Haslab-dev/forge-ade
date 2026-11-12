package settings

import (
	"fmt"
	"strings"
	"sync"

	"github.com/hasdev/forge-ade/forge-ade-native/pkg/engine"
	"github.com/hasdev/forge-ade/forge-ade-native/pkg/theme"
)

// SupportedLLMProviders lists the primary LLM provider options.
var SupportedLLMProviders = []string{
	"openai",
	"anthropic",
	"gemini",
	"openrouter",
	"ollama",
	"deepseek",
}

// ProviderDefaultModels provides default model names per provider. Left empty so only configured models are used.
var ProviderDefaultModels = map[string]string{}

// SettingsTab Constants
const (
	SettingsTabModels     = 0 // Providers & Models
	SettingsTabAppearance = 1 // Appearance & Themes
	SettingsTabAgents     = 2 // ACP & Agent CLIs
	SettingsTabShortcuts  = 3 // Terminal & Shortcuts
	SettingsTabGeneral    = 4 // General
	SettingsTabMCP        = 5 // MCP Servers
	SettingsTabSkills     = 6 // Skills
)

// SettingsManager manages provider credentials, models, and application preferences.
type SettingsManager struct {
	mu           sync.RWMutex
	IsOpen       bool
	ActiveTab    int
	SearchQuery  string
	ShowAPIKey   bool
	AutoSave     bool
	DefaultShell string
	engine       *engine.Engine
	ThemeMgr     *theme.ThemeManager
	Provider     string
	APIKey       string
	BaseURL      string
	Model        string
	MaxTokens    int
	StatusMsg    string
	ProviderIdx  int
	OnModelSaved func(provider, model string)
}

// NewSettingsManager creates a SettingsManager initialized from backend configurations.
func NewSettingsManager(engine *engine.Engine, onSaved func(provider, model string)) *SettingsManager {
	sm := &SettingsManager{
		engine:       engine,
		Provider:     "",
		Model:        "",
		MaxTokens:    4096,
		DefaultShell: "/bin/bash",
		AutoSave:     true,
		OnModelSaved: onSaved,
	}
	sm.Load()
	return sm
}

// Load reads current settings from the native engine.
func (sm *SettingsManager) Load() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.engine == nil {
		return
	}

	cfg := sm.engine.GetLLMConfig()
	if cfg.ProviderID != "" {
		sm.Provider = cfg.ProviderID
		sm.APIKey = cfg.APIKey
		sm.BaseURL = cfg.BaseURL
		sm.Model = cfg.Model

		for i, p := range SupportedLLMProviders {
			if strings.EqualFold(p, cfg.ProviderID) {
				sm.ProviderIdx = i
				break
			}
		}
	}
}

// Save persists the active LLM profile to disk.
func (sm *SettingsManager) Save() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.ProviderIdx >= 0 && sm.ProviderIdx < len(SupportedLLMProviders) {
		sm.Provider = SupportedLLMProviders[sm.ProviderIdx]
	}

	if sm.engine != nil {
		if err := sm.engine.SaveLLMProfile(sm.Provider, sm.APIKey, sm.BaseURL, sm.Model); err != nil {
			sm.StatusMsg = fmt.Sprintf("Save error: %v", err)
			return err
		}
	}

	sm.StatusMsg = fmt.Sprintf("Saved %s (%s)", sm.Provider, sm.Model)
	if sm.OnModelSaved != nil {
		sm.OnModelSaved(sm.Provider, sm.Model)
	}
	return nil
}

// Toggle flips the visibility of the settings modal.
func (sm *SettingsManager) Toggle() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.IsOpen = !sm.IsOpen
	if sm.IsOpen {
		sm.StatusMsg = ""
	}
}
