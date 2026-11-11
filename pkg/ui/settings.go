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

// SettingsTab Constants
const (
	SettingsTabModels     = 0 // 🤖 Models & Providers
	SettingsTabAppearance = 1 // 🎨 Appearance & Themes
	SettingsTabAgents     = 2 // 🔌 Agent Protocols (ACP)
	SettingsTabShortcuts  = 3 // ⌨ Keyboard Shortcuts
)

// SettingsManager manages provider credentials, models, and application preferences.
type SettingsManager struct {
	mu           sync.RWMutex
	IsOpen       bool
	ActiveTab    int
	engine       *Engine
	ThemeMgr     *ThemeManager
	ACPMgr       *ACPManager
	Provider     string
	APIKey       string
	BaseURL      string
	Model        string
	MaxTokens    int
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
		MaxTokens:    4096,
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

// RenderSettings draws the tabbed settings modal dialog.
func (sm *SettingsManager) RenderSettings(c *ui.Context) {
	sm.mu.RLock()
	open := sm.IsOpen
	sm.mu.RUnlock()

	if !open {
		return
	}

	t := c.Theme()

	ui.Modal(c, &sm.IsOpen, func() {
		ui.Column(c).Width(640).Radius(12).Background(t.Surface).BorderWidth(1).BorderColor(t.Border).Padding(16, 20).Gap(12).Children(func() {
			// Title Header
			ui.Row(c).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "⚙ Settings & Preferences").Bold().FontSize(15)
				ui.Box(c).Grow(1)
				if ui.Button(c, "✕").Clicked() {
					sm.IsOpen = false
				}
			})

			// Navigation Tabs: [Models] [Appearance] [ACP Agents] [Shortcuts]
			ui.Row(c).Gap(6).Children(func() {
				tabs := []string{"🤖 Models & Providers", "🎨 Appearance", "🔌 ACP Agents", "⌨ Shortcuts"}
				for i, name := range tabs {
					idx := i
					var btn ui.Element
					if sm.ActiveTab == idx {
						btn = ui.PrimaryButton(c, name).FontSize(11)
					} else {
						btn = ui.Button(c, name).FontSize(11)
					}
					if btn.Clicked() {
						sm.ActiveTab = idx
					}
				}
			})

			// Tab Content Area
			switch sm.ActiveTab {
			case SettingsTabModels:
				sm.renderModelsTab(c)
			case SettingsTabAppearance:
				sm.renderAppearanceTab(c)
			case SettingsTabAgents:
				sm.renderAgentsTab(c)
			case SettingsTabShortcuts:
				sm.renderShortcutsTab(c)
			}

			// Footer Actions
			ui.Row(c).Padding(10, 0, 0, 0).AlignItems(ui.Center).BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Children(func() {
				if sm.StatusMsg != "" {
					ui.Text(c, sm.StatusMsg).FontSize(11).TextColor(t.Accent)
				}
				ui.Box(c).Grow(1)
				if ui.Button(c, "Close").Clicked() {
					sm.IsOpen = false
				}
				if sm.ActiveTab == SettingsTabModels {
					if ui.PrimaryButton(c, "Save Profile").Clicked() {
						if err := sm.Save(); err == nil {
							sm.IsOpen = false
						}
					}
				}
			})
		})
	})
}

func (sm *SettingsManager) renderModelsTab(c *ui.Context) {
	t := c.Theme()

	ui.Column(c).Gap(10).Children(func() {
		ui.Text(c, "Configure LLM provider credentials for reasoning, streaming, and tool execution.").FontSize(12).TextColor(t.TextMuted)

		// Provider Selector
		ui.Column(c).Gap(4).Children(func() {
			ui.Text(c, "LLM PROVIDER").FontSize(10).Bold().TextColor(t.TextMuted)
			seg := ui.Segmented(c, &sm.providerIdx, SupportedLLMProviders...)
			if seg.Changed() && sm.providerIdx < len(SupportedLLMProviders) {
				p := SupportedLLMProviders[sm.providerIdx]
				if def, ok := ProviderDefaultModels[p]; ok && sm.Model == "" {
					sm.Model = def
				}
			}
		})

		// Model Name Input with Quick Select Pills
		ui.Column(c).Gap(4).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "MODEL ID").FontSize(10).Bold().TextColor(t.TextMuted)
				ui.Box(c).Grow(1)
				quickModels := []string{"claude-3-7-sonnet", "gpt-4o", "gemini-2.5-pro", "deepseek-chat"}
				for _, qm := range quickModels {
					thisModel := qm
					btn := ui.Button(c, thisModel).FontSize(10)
					if btn.Clicked() {
						sm.Model = thisModel
					}
				}
			})
			ui.TextInput(c, &sm.Model).Placeholder("e.g. claude-3-7-sonnet, gpt-4o, gemini-2.5-pro").Grow(1)
		})

		// API Key Input
		ui.Column(c).Gap(4).Children(func() {
			ui.Text(c, "API KEY").FontSize(10).Bold().TextColor(t.TextMuted)
			ui.TextInput(c, &sm.APIKey).Placeholder("sk-... or token (leave empty for local Ollama)").Grow(1)
		})

		// Custom Base URL Input
		ui.Column(c).Gap(4).Children(func() {
			ui.Text(c, "CUSTOM BASE URL (OPTIONAL)").FontSize(10).Bold().TextColor(t.TextMuted)
			ui.TextInput(c, &sm.BaseURL).Placeholder("e.g. http://127.0.0.1:11434/v1 for local Ollama").Grow(1)
		})
	})
}

func (sm *SettingsManager) renderAppearanceTab(c *ui.Context) {
	t := c.Theme()

	if sm.ThemeMgr == nil {
		ui.Text(c, "Theme manager unavailable.").FontSize(12).TextColor(t.TextMuted)
		return
	}

	ui.Column(c).Gap(10).Children(func() {
		ui.Text(c, "Choose your preferred theme palette, appearance, and layout options.").FontSize(12).TextColor(t.TextMuted)

		// Palette Swatches
		ui.Text(c, "THEME PALETTE").FontSize(10).Bold().TextColor(t.TextMuted)
		ui.Row(c).Gap(8).Children(func() {
			palettes := sm.ThemeMgr.AvailablePalettes()
			for _, pal := range palettes {
				p := pal
				isSelected := (sm.ThemeMgr.PaletteID == p.ID)
				btn := ui.Column(c.Key("settings-pal-"+p.ID)).Width(85).Padding(8, 6).Radius(8).AlignItems(ui.Center).Gap(4).BorderWidth(1)
				if isSelected {
					btn.BorderColor(t.Accent).Background(t.Border)
				} else {
					btn.BorderColor(t.Border).Background(t.Surface)
				}
				btn.Children(func() {
					ui.Box(c).Width(24).Height(24).Radius(12).Background(ui.Hex(p.SwatchHex))
					ui.Text(c, p.Name).FontSize(10).Bold()
				})
				if btn.Clicked() {
					sm.ThemeMgr.PaletteID = p.ID
				}
			}
		})

		// Appearance Switcher: Dark vs Light
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "APPEARANCE").FontSize(10).Bold().TextColor(t.TextMuted).Width(120)
			var darkBtn ui.Element
			if sm.ThemeMgr.Appearance == "dark" {
				darkBtn = ui.PrimaryButton(c, "🌙 Dark").FontSize(11)
			} else {
				darkBtn = ui.Button(c, "🌙 Dark").FontSize(11)
			}
			if darkBtn.Clicked() {
				sm.ThemeMgr.Appearance = "dark"
			}

			var lightBtn ui.Element
			if sm.ThemeMgr.Appearance == "light" {
				lightBtn = ui.PrimaryButton(c, "☀️ Light").FontSize(11)
			} else {
				lightBtn = ui.Button(c, "☀️ Light").FontSize(11)
			}
			if lightBtn.Clicked() {
				sm.ThemeMgr.Appearance = "light"
			}
		})

		// Chat Width Preset
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "CHAT WIDTH").FontSize(10).Bold().TextColor(t.TextMuted).Width(120)
			for _, w := range []string{"comfortable", "wide", "full"} {
				thisW := w
				var btn ui.Element
				if sm.ThemeMgr.ChatWidth == thisW {
					btn = ui.PrimaryButton(c, strings.Title(thisW)).FontSize(11)
				} else {
					btn = ui.Button(c, strings.Title(thisW)).FontSize(11)
				}
				if btn.Clicked() {
					sm.ThemeMgr.ChatWidth = thisW
				}
			}
		})

		// Diff Color Scheme
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "DIFF SCHEME").FontSize(10).Bold().TextColor(t.TextMuted).Width(120)
			for _, scheme := range []string{"classic", "modern"} {
				thisScheme := scheme
				label := "Classic (Green/Red)"
				if thisScheme == "modern" {
					label = "Modern (Blue/Orange)"
				}
				var btn ui.Element
				if sm.ThemeMgr.DiffScheme == thisScheme {
					btn = ui.PrimaryButton(c, label).FontSize(11)
				} else {
					btn = ui.Button(c, label).FontSize(11)
				}
				if btn.Clicked() {
					sm.ThemeMgr.DiffScheme = thisScheme
				}
			}
		})
	})
}

func (sm *SettingsManager) renderAgentsTab(c *ui.Context) {
	t := c.Theme()

	if sm.ACPMgr == nil {
		ui.Text(c, "ACP manager unavailable.").FontSize(12).TextColor(t.TextMuted)
		return
	}

	ui.Column(c).Gap(8).Children(func() {
		ui.Text(c, "Installed Agent Client Protocol (ACP) drivers and external CLI agents.").FontSize(12).TextColor(t.TextMuted)

		ui.Scroll(c).Height(240).Children(func() {
			ui.Column(c).Gap(6).Children(func() {
				for _, agent := range sm.ACPMgr.Agents {
					ag := agent
					ui.Row(c.Key("settings-acp-"+ag.ID)).Padding(8, 10).Radius(6).Background(t.Surface).BorderWidth(1).BorderColor(t.Border).AlignItems(ui.Center).Children(func() {
						ui.Column(c).Grow(1).Children(func() {
							ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
								ui.Text(c, ag.Name).Bold().FontSize(12)
								if ag.Available {
									ui.Text(c, "✔ On PATH").FontSize(10).TextColor(t.Accent)
								} else {
									ui.Text(c, "Not detected").FontSize(10).TextColor(t.TextMuted)
								}
							})
							ui.Text(c, ag.Description).FontSize(11).TextColor(t.TextMuted)
						})
					})
				}
			})
		})
	})
}

func (sm *SettingsManager) renderShortcutsTab(c *ui.Context) {
	t := c.Theme()

	type shortcutItem struct {
		Key  string
		Desc string
	}

	shortcuts := []shortcutItem{
		{"⌘S / Ctrl+S", "Save current editor file to disk"},
		{"⌘P / Ctrl+P", "Quick open file command palette"},
		{"⌘, / Ctrl+,", "Open settings modal dialog"},
		{"⌘B / Ctrl+B", "Toggle left workspace sidebar"},
		{"⌘N / Ctrl+N", "Create new task session"},
		{"⌘J / Ctrl+J", "Toggle collapsible right side drawer"},
		{"⌘1 / Ctrl+1", "Switch surface to ✨ Autonomous Agent Mode"},
		{"⌘2 / Ctrl+2", "Switch surface to 💻 Code Editor Mode"},
		{"⌘Enter", "Send chat prompt or commit staged git changes"},
	}

	ui.Column(c).Gap(6).Children(func() {
		ui.Text(c, "Global keyboard shortcuts for rapid navigation.").FontSize(12).TextColor(t.TextMuted)

		ui.Scroll(c).Height(240).Children(func() {
			ui.Column(c).Gap(4).Children(func() {
				for _, sc := range shortcuts {
					s := sc
					ui.Row(c.Key("sc-"+s.Key)).Padding(6, 10).Radius(6).Background(t.Surface).BorderWidth(1).BorderColor(t.Border).AlignItems(ui.Center).Children(func() {
						ui.Text(c, s.Key).Bold().FontSize(11).TextColor(t.Accent).Width(140)
						ui.Text(c, s.Desc).FontSize(12).Grow(1)
					})
				}
			})
		})
	})
}
