package settings

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

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
	mu              sync.RWMutex
	IsOpen          bool
	ActiveTab       int
	SearchQuery     string
	ShowAPIKey      bool
	AutoSave        bool
	DefaultShell    string
	engine          *engine.Engine
	ThemeMgr        *theme.ThemeManager
	Provider        string
	APIKey          string
	BaseURL         string
	Model           string
	MaxTokens       int
	StatusMsg       string
	ProviderIdx     int
	ProviderID      string
	OnModelSaved    func(provider, model string)
	AvailableModels []string
	FetchingModels  bool
	ModelSelection  map[string]bool
	SearchModels    string
}

// DefaultBaseURL returns the OpenAI-compatible base URL assumed per provider.
func DefaultBaseURL(provider string) string {
	switch provider {
	case "openai":
		return "https://api.openai.com/v1"
	case "anthropic":
		return "https://api.anthropic.com/v1"
	case "gemini":
		return "https://generativelanguage.googleapis.com/v1"
	case "deepseek":
		return "https://api.deepseek.com/v1"
	case "openrouter":
		return "https://openrouter.ai/api/v1"
	case "ollama":
		return "http://localhost:11434/v1"
	default:
		return ""
	}
}

// FetchModels queries the provider's OpenAI-compatible /models endpoint and
// fills AvailableModels.
func (sm *SettingsManager) FetchModels() {
	sm.mu.Lock()
	provider := sm.Provider
	apiKey := sm.APIKey
	base := sm.BaseURL
	if base == "" {
		base = DefaultBaseURL(provider)
		sm.BaseURL = base
	}
	sm.FetchingModels = true
	sm.mu.Unlock()

	if base == "" {
		sm.mu.Lock()
		sm.FetchingModels = false
		sm.StatusMsg = "Set a base URL first"
		sm.mu.Unlock()
		return
	}

	url := strings.TrimRight(base, "/") + "/models"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		sm.mu.Lock()
		sm.FetchingModels = false
		sm.StatusMsg = err.Error()
		sm.mu.Unlock()
		return
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("x-api-key", apiKey)
	}
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		sm.mu.Lock()
		sm.FetchingModels = false
		sm.StatusMsg = err.Error()
		sm.mu.Unlock()
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))

	var parsed struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	var ids []string
	if err := json.Unmarshal(body, &parsed); err == nil {
		for _, m := range parsed.Data {
			ids = append(ids, m.ID)
		}
		if len(ids) == 0 {
			for _, m := range parsed.Models {
				ids = append(ids, m.Name)
			}
		}
	}
	sm.mu.Lock()
	sm.FetchingModels = false
	if len(ids) == 0 {
		sm.StatusMsg = fmt.Sprintf("No models in response (HTTP %d)", resp.StatusCode)
	} else {
		sort.Strings(ids)
		sm.AvailableModels = ids
		if sm.ModelSelection == nil {
			sm.ModelSelection = map[string]bool{}
		}
		for _, id := range ids {
			sm.ModelSelection[id] = id == sm.Model
		}
		sm.StatusMsg = fmt.Sprintf("Fetched %d models", len(ids))
	}
	sm.mu.Unlock()
}

// ToggleModelSelection flips a fetched model's checkbox.
func (sm *SettingsManager) ToggleModelSelection(id string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sm.ModelSelection == nil {
		sm.ModelSelection = map[string]bool{}
	}
	sm.ModelSelection[id] = !sm.ModelSelection[id]
}

// SaveSelectedModel persists the provider profile with the chosen model.
func (sm *SettingsManager) SaveSelectedModel(model string) {
	sm.mu.Lock()
	sm.ProviderID = sm.Provider
	provider, apiKey, base := sm.Provider, sm.APIKey, sm.BaseURL
	sm.mu.Unlock()
	if sm.engine != nil {
		_ = sm.engine.SaveLLMProfile(provider, apiKey, base, model)
	}
	sm.mu.Lock()
	sm.Model = model
	sm.mu.Unlock()
	if sm.OnModelSaved != nil {
		sm.OnModelSaved(provider, model)
	}
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
