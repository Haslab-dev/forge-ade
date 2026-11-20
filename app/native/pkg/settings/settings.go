package settings

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Haslab-dev/RakitKode/llm"
	"github.com/hasdev/forge-ade/app/native/pkg/engine"
	"github.com/hasdev/forge-ade/app/native/pkg/terminal"
	"github.com/hasdev/forge-ade/app/native/pkg/theme"
	"github.com/hasdev/forge-ade/core/quota"
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
	SettingsTabModels           = 0  // Providers & Models
	SettingsTabAppearance       = 1  // Appearance & Themes
	SettingsTabAgents           = 2  // ACP & Agent CLIs
	SettingsTabShortcuts        = 3  // Terminal & Shortcuts
	SettingsTabGeneral          = 4  // General
	SettingsTabMCP              = 5  // MCP Servers
	SettingsTabSkills           = 6  // Skills
	SettingsTabPlugins          = 7  // Plugins
	SettingsTabMemory           = 8  // Memory
	SettingsTabAutos            = 9  // Automations (bots)
	SettingsTabSubagents        = 10 // Sub-Agents & Roles
	SettingsTabBrowser          = 11 // Browser Use
	SettingsTabComputer         = 12 // Computer Use
	SettingsTabCommands         = 13 // Slash Commands
	SettingsTabHooks            = 14 // Lifecycle Hooks
	SettingsTabIndexing         = 15 // Codebase Indexing
	SettingsTabAntigravityQuota = 16 // Antigravity Quota Tracker
)

// AddAccountMode constants
const (
	AddAccountModeOAuth  = 0
	AddAccountModeManual = 1
)

// CustomSlashCommand represents a user-defined prompt expansion.
type CustomSlashCommand struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Prompt      string `json:"prompt"`
}

// HookEntry represents a configured lifecycle hook.
type HookEntry struct {
	ID      string `json:"id"`
	Event   string `json:"event"`
	Matcher string `json:"matcher"`
	Command string `json:"command"`
}

// SettingsManager manages provider credentials, models, and application preferences.
type SettingsManager struct {
	mu           sync.RWMutex
	IsOpen       bool
	ActiveTab    int
	SearchQuery  string
	ShowAPIKey   bool
	AutoSave     bool
	DefaultShell string

	// Integration form state (MCP / Memory / Automations sections).
	MCPFormOpen bool
	MCPName     string
	MCPCommand  string
	MCPType     string
	MemoryNew   string
	AutoName    string
	AutoCron    string
	AutoPrompt  string

	// Skills form state
	SkillFormOpen    bool
	SkillName        string
	SkillDescription string
	SkillPrompt      string
	SkillScope       string

	// Role form state
	RoleFormOpen  bool
	RoleEditingID string
	RoleName      string
	RoleDesc      string
	RoleFilter    string
	RoleModel     string
	RolePrompt    string
	RoleRules     string
	RoleReadOnly  bool
	RoleAllowBash bool

	// Custom slash commands state
	CmdFormOpen    bool
	CmdName        string
	CmdDescription string
	CmdPrompt      string
	CustomCommands []CustomSlashCommand

	// Lifecycle hooks state
	HookFormOpen bool
	HookEvent    string
	HookMatcher  string
	HookCommand  string
	HooksList    []HookEntry

	// Browser Use state
	BrowserHeadless bool
	BrowserAllowJS  bool
	BrowserEngine   string
	BrowserViewport string

	// Computer Use state
	ComputerPermissionMode string

	// Indexing state
	IndexingStatus  string
	IndexedFiles    int
	IndexedSymbols  int
	ReindexBusy     bool
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

	// Custom provider profiles state
	SelectedProviderID      string
	EditingProfileID        string
	EditProfileName         string
	EditBaseURL             string
	EditAPIKey              string
	IsAddingProvider        bool
	NewProviderProtocol     string // "openai" or "anthropic"
	NewProviderName         string
	NewProviderBaseURL      string
	NewProviderAPIKey       string
	NewProviderInitialModel string
	IsAddingModel           bool
	NewModelName            string
	ModelSearchQuery        string
	IsFetchingModels        bool

	// Antigravity Quota Tracker state
	accountMgr           *quota.AntigravityAccountManager
	IsRefreshingQuota    bool
	RefreshingAccountID  string
	IsAddingAccount      bool
	AddAccountMode       int // 0: OAuth (automatic), 1: Manual (token/json)
	IsOAuthFlowActive    bool
	OAuthFlowURL         string
	OAuthFlowStatus      string
	NewAccountInput      string
	NewAccountLabel      string
	AccountStatusMessage string
}

// AntigravityAccountMgr returns the active AntigravityAccountManager instance.
func (sm *SettingsManager) AntigravityAccountMgr() *quota.AntigravityAccountManager {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sm.accountMgr != nil {
		return sm.accountMgr
	}
	if sm.engine != nil && sm.engine.AntigravityAccounts != nil {
		sm.accountMgr = sm.engine.AntigravityAccounts
		return sm.accountMgr
	}
	dataDir := ""
	if sm.engine != nil && sm.engine.DataDir != "" {
		dataDir = sm.engine.DataDir
	}
	sm.accountMgr = quota.NewAccountManager(dataDir)
	return sm.accountMgr
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

// GetProfiles returns the custom provider profiles from LLMClient.
func (sm *SettingsManager) GetProfiles() []llm.ProviderProfile {
	if sm.engine != nil && sm.engine.LLMClient != nil {
		return sm.engine.LLMClient.GetProviderProfiles()
	}
	return nil
}

// CopyProfiles deep-copies provider profiles. GetProviderProfiles hands back
// the LLMClient's live internal slice; UI layers copy it before editing so
// concurrent fetch/save goroutines never observe mid-mutation state, and
// persistence happens only through SaveProfiles (atomic slice swap).
func CopyProfiles(profiles []llm.ProviderProfile) []llm.ProviderProfile {
	if profiles == nil {
		return nil
	}
	out := make([]llm.ProviderProfile, len(profiles))
	for i, p := range profiles {
		p.AvailableModels = append([]string(nil), p.AvailableModels...)
		p.SelectedModels = append([]string(nil), p.SelectedModels...)
		out[i] = p
	}
	return out
}

// SaveProfiles saves the updated custom provider profiles to LLMClient.
func (sm *SettingsManager) SaveProfiles(profiles []llm.ProviderProfile) error {
	if sm.engine != nil && sm.engine.LLMClient != nil {
		return sm.engine.LLMClient.SaveProviderProfiles(profiles)
	}
	return nil
}

// DeleteProfile deletes a custom provider profile by ID.
func (sm *SettingsManager) DeleteProfile(id string) error {
	if sm.engine != nil && sm.engine.LLMClient != nil {
		return sm.engine.LLMClient.DeleteProviderProfile(id)
	}
	return nil
}

// NewSettingsManager creates a SettingsManager initialized from backend configurations.
func NewSettingsManager(engine *engine.Engine, onSaved func(provider, model string)) *SettingsManager {
	sm := &SettingsManager{
		engine:              engine,
		Provider:            "",
		Model:               "",
		MaxTokens:           4096,
		DefaultShell:        terminal.DefaultUserShell(),
		AutoSave:            true,
		OnModelSaved:        onSaved,
		NewProviderProtocol: "openai",
	}
	sm.Load()
	sm.loadIntegrationLists()
	if sm.SelectedProviderID == "" {
		profiles := sm.GetProfiles()
		if len(profiles) > 0 {
			sm.SelectedProviderID = profiles[0].ID
		}
	}
	return sm
}

// integrationLists is the persisted shape for custom slash commands and
// lifecycle hooks (they round-trip through the settings data directory).
type integrationLists struct {
	CustomCommands []CustomSlashCommand `json:"custom_commands"`
	Hooks          []HookEntry          `json:"hooks"`
}

func (sm *SettingsManager) integrationPath() string {
	if sm.engine != nil && sm.engine.DataDir != "" {
		return filepath.Join(sm.engine.DataDir, "integrations.json")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".forge-ade", "integrations.json")
}

// loadIntegrationLists reads persisted custom commands and hooks at startup.
func (sm *SettingsManager) loadIntegrationLists() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	path := sm.integrationPath()
	if path == "" {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var lists integrationLists
	if err := json.Unmarshal(data, &lists); err != nil {
		return
	}
	sm.CustomCommands = lists.CustomCommands
	sm.HooksList = lists.Hooks
}

// persistIntegrationListsLocked writes the current lists (caller holds mu).
func (sm *SettingsManager) persistIntegrationListsLocked() {
	path := sm.integrationPath()
	if path == "" {
		return
	}
	payload := integrationLists{CustomCommands: sm.CustomCommands, Hooks: sm.HooksList}
	if data, err := json.MarshalIndent(payload, "", "  "); err == nil {
		_ = os.MkdirAll(filepath.Dir(path), 0o755)
		_ = os.WriteFile(path, data, 0o644)
	}
}

// SetCustomCommands replaces the custom slash command list and persists it.
func (sm *SettingsManager) SetCustomCommands(list []CustomSlashCommand) {
	sm.mu.Lock()
	sm.CustomCommands = list
	sm.persistIntegrationListsLocked()
	sm.mu.Unlock()
}

// SetHooks replaces the lifecycle hook list and persists it.
func (sm *SettingsManager) SetHooks(list []HookEntry) {
	sm.mu.Lock()
	sm.HooksList = list
	sm.persistIntegrationListsLocked()
	sm.mu.Unlock()
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
