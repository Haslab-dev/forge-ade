package ui

import (
	"fmt"
	"strings"
	"sync"

	"github.com/egoist/mygo/ui"
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

// ProviderDefaultModels provides popular default model names.
var ProviderDefaultModels = map[string]string{
	"openai":     "gpt-4o",
	"anthropic":  "claude-3-5-sonnet-20241022",
	"gemini":     "gemini-2.5-pro",
	"openrouter": "anthropic/claude-3.5-sonnet",
	"ollama":     "llama3.3:latest",
	"deepseek":   "deepseek-chat",
}

// SettingsManager manages provider credentials, models, and application preferences.
type SettingsManager struct {
	mu           sync.RWMutex
	IsOpen       bool
	engine       *Engine
	Provider     string
	APIKey       string
	BaseURL      string
	Model        string
	StatusMsg    string
	providerIdx  int
	OnModelSaved func(provider, model string)
}

// NewSettingsManager creates a SettingsManager initialized from backend configurations.
func NewSettingsManager(engine *Engine, onSaved func(provider, model string)) *SettingsManager {
	sm := &SettingsManager{
		engine:       engine,
		Provider:     "anthropic",
		Model:        "claude-3-5-sonnet-20241022",
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
				sm.providerIdx = i
				break
			}
		}
	}
}

// Save persists the active LLM profile to disk.
func (sm *SettingsManager) Save() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.providerIdx >= 0 && sm.providerIdx < len(SupportedLLMProviders) {
		sm.Provider = SupportedLLMProviders[sm.providerIdx]
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

// Toggle flips settings visibility.
func (sm *SettingsManager) Toggle() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.IsOpen = !sm.IsOpen
	if sm.IsOpen {
		sm.StatusMsg = ""
	}
}

// RenderSettings draws the settings modal dialog.
func (sm *SettingsManager) RenderSettings(c *ui.Context) {
	sm.mu.RLock()
	open := sm.IsOpen
	sm.mu.RUnlock()

	if !open {
		return
	}

	t := c.Theme()

	ui.Modal(c, &sm.IsOpen, func() {
		ui.Column(c).Width(580).Radius(12).Background(t.Surface).BorderWidth(1).BorderColor(t.Border).Padding(16, 20).Gap(12).Children(func() {
			// Title Header
			ui.Row(c).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "Settings & Model Configuration").Bold().FontSize(15)
				ui.Box(c).Grow(1)
				if ui.Button(c, "✕").Clicked() {
					sm.IsOpen = false
				}
			})

			ui.Text(c, "Configure your LLM credentials for autonomous reasoning and agent tool execution.").FontSize(12).TextColor(t.TextMuted)

			// Provider Selector
			ui.Column(c).Gap(4).Children(func() {
				ui.Text(c, "LLM PROVIDER").FontSize(11).Bold().TextColor(t.TextMuted)
				seg := ui.Segmented(c, &sm.providerIdx, SupportedLLMProviders...)
				if seg.Changed() && sm.providerIdx < len(SupportedLLMProviders) {
					p := SupportedLLMProviders[sm.providerIdx]
					if def, ok := ProviderDefaultModels[p]; ok && sm.Model == "" {
						sm.Model = def
					}
				}
			})

			// Model Name Input
			ui.Column(c).Gap(4).Children(func() {
				ui.Text(c, "MODEL ID").FontSize(11).Bold().TextColor(t.TextMuted)
				ui.TextInput(c, &sm.Model).Placeholder("e.g. claude-3-5-sonnet-20241022, gpt-4o").Grow(1)
			})

			// API Key Input
			ui.Column(c).Gap(4).Children(func() {
				ui.Text(c, "API KEY").FontSize(11).Bold().TextColor(t.TextMuted)
				ui.TextInput(c, &sm.APIKey).Placeholder("sk-... or leave empty for local Ollama").Grow(1)
			})

			// Custom Base URL Input
			ui.Column(c).Gap(4).Children(func() {
				ui.Text(c, "CUSTOM BASE URL (OPTIONAL)").FontSize(11).Bold().TextColor(t.TextMuted)
				ui.TextInput(c, &sm.BaseURL).Placeholder("e.g. http://127.0.0.1:11434/v1 for Ollama").Grow(1)
			})

			// Footer Actions
			ui.Row(c).Padding(8, 0, 0, 0).AlignItems(ui.Center).Children(func() {
				if sm.StatusMsg != "" {
					ui.Text(c, sm.StatusMsg).FontSize(12).TextColor(t.Accent)
				}
				ui.Box(c).Grow(1)
				if ui.Button(c, "Cancel").Clicked() {
					sm.IsOpen = false
				}
				if ui.PrimaryButton(c, "Save Profile").Clicked() {
					if err := sm.Save(); err == nil {
						sm.IsOpen = false
					}
				}
			})
		})
	})
}
