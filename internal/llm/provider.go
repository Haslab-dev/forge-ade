package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"

	DefaultStreamIdleTimeout = 45 * time.Second
)

type contextKey string

const (
	sessionIDContextKey contextKey = "session_id"
)

// WithSessionID returns a copy of parent ctx with the agent session ID attached.
func WithSessionID(ctx context.Context, sessionID string) context.Context {
	if sessionID == "" {
		return ctx
	}
	return context.WithValue(ctx, sessionIDContextKey, sessionID)
}

// SessionIDFrom extracts the agent session ID from ctx, or empty string if not present.
func SessionIDFrom(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(sessionIDContextKey).(string); ok {
		return v
	}
	return ""
}

// ErrStreamIdleTimeout indicates that the LLM streaming connection went silent
// without any chunk arriving within the idle timeout window.
type ErrStreamIdleTimeout struct {
	Timeout time.Duration
}

func (e *ErrStreamIdleTimeout) Error() string {
	return fmt.Sprintf("stream idle timeout: no data received from model provider for %s", e.Timeout)
}

type LLMMessage struct {
	Role       Role        `json:"role"`
	Content    interface{} `json:"content"`
	Name       string      `json:"name,omitempty"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`
}

type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type ToolDefinition struct {
	Type     string       `json:"type"`
	Function FunctionSpec `json:"function"`
}

type FunctionSpec struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// MCPTool is a tool discovered from an MCP server, registered into the tool
// registry under its full "server/tool" name.
type MCPTool struct {
	ServerName  string                 `json:"server_name"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

type TokenStats struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	CachedTokens     int `json:"cached_tokens"`
	// PromptCacheHitTokens / PromptCacheMissTokens: DeepSeek's context-cache
	// accounting (hit+miss == prompt). Anthropic-style providers only set
	// CachedTokens; DeepSeek providers leave it 0.
	PromptCacheHitTokens  int `json:"prompt_cache_hit_tokens"`
	PromptCacheMissTokens int `json:"prompt_cache_miss_tokens"`
	TotalTokens           int `json:"total_tokens"`
}

type LLMResponse struct {
	Content      string     `json:"content"`
	Reasoning    string     `json:"reasoning,omitempty"`
	ToolCalls    []ToolCall `json:"tool_calls,omitempty"`
	FinishReason string     `json:"finish_reason"`
	TokenUsage   TokenStats `json:"token_usage"`
}

type ProviderConfig struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	BaseURL      string `json:"base_url"`
	EnvKey       string `json:"env_key"`
	DefaultModel string `json:"default_model"`
	RequiresKey  bool   `json:"requires_key"`
}

// isLocalBaseURL reports whether the endpoint is a local inference server
// that legitimately needs no API key (Ollama, LM Studio, llama.cpp, vLLM…).
func isLocalBaseURL(baseURL string) bool {
	u := strings.ToLower(baseURL)
	return strings.Contains(u, "localhost") || strings.Contains(u, "127.0.0.1") || strings.Contains(u, "[::1]") || strings.HasPrefix(u, "unix://")
}

type ProviderProfile struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	APIKey          string   `json:"api_key"`
	BaseURL         string   `json:"base_url"`
	Enabled         bool     `json:"enabled"`
	AvailableModels []string `json:"available_models"`
	SelectedModels  []string `json:"selected_models"`
}

var SupportedProviders = map[string]ProviderConfig{
	"openai": {
		ID:           "openai",
		Name:         "OpenAI",
		BaseURL:      "https://api.openai.com/v1",
		EnvKey:       "OPENAI_API_KEY",
		DefaultModel: "gpt-4o",
		RequiresKey:  true,
	},
	"deepseek": {
		ID:           "deepseek",
		Name:         "DeepSeek",
		BaseURL:      "https://api.deepseek.com/v1",
		EnvKey:       "DEEPSEEK_API_KEY",
		DefaultModel: "deepseek-chat",
		RequiresKey:  true,
	},
	"gemini": {
		ID:           "gemini",
		Name:         "Gemini",
		BaseURL:      "https://generativelanguage.googleapis.com/v1beta/openai",
		EnvKey:       "GEMINI_API_KEY",
		DefaultModel: "gemini-2.0-flash",
		RequiresKey:  true,
	},
	"anthropic": {
		ID:           "anthropic",
		Name:         "Anthropic",
		BaseURL:      "https://api.anthropic.com/v1",
		EnvKey:       "ANTHROPIC_API_KEY",
		DefaultModel: "claude-3-5-sonnet-latest",
		RequiresKey:  true,
	},
	"ollama": {
		ID:           "ollama",
		Name:         "Ollama (Local)",
		BaseURL:      "http://localhost:11434/v1",
		EnvKey:       "OLLAMA_API_KEY",
		DefaultModel: "llama3.1:8b",
		RequiresKey:  false,
	},
	"groq": {
		ID:           "groq",
		Name:         "Groq",
		BaseURL:      "https://api.groq.com/openai/v1",
		EnvKey:       "GROQ_API_KEY",
		DefaultModel: "llama-3.3-70b-versatile",
		RequiresKey:  true,
	},
	"openrouter": {
		ID:           "openrouter",
		Name:         "OpenRouter",
		BaseURL:      "https://openrouter.ai/api/v1",
		EnvKey:       "OPENROUTER_API_KEY",
		DefaultModel: "openai/gpt-4o",
		RequiresKey:  true,
	},
	"mistral": {
		ID:           "mistral",
		Name:         "Mistral",
		BaseURL:      "https://api.mistral.ai/v1",
		EnvKey:       "MISTRAL_API_KEY",
		DefaultModel: "mistral-large-latest",
		RequiresKey:  true,
	},
}

type Profile struct {
	ProviderID string `json:"provider_id"`
	APIKey     string `json:"api_key"`
	BaseURL    string `json:"base_url"`
	Model      string `json:"model"`
}

type LLMClient struct {
	mu              sync.RWMutex
	providerID      string
	apiKey          string
	baseURL         string
	model           string
	reasoningEffort string
	httpClient      *http.Client
	profilePath     string
	providersPath   string
	profiles        []ProviderProfile
}

func NewLLMClient(dataDir string) *LLMClient {
	c := &LLMClient{
		providerID: "openai",
		baseURL:    SupportedProviders["openai"].BaseURL,
		model:      SupportedProviders["openai"].DefaultModel,
		httpClient: &http.Client{
			Timeout: 180 * time.Second,
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
				DialContext: (&net.Dialer{
					Timeout:   15 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
				ForceAttemptHTTP2:     true,
				MaxIdleConns:          100,
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
				ResponseHeaderTimeout: 30 * time.Second,
			},
		},
		profilePath:   filepath.Join(dataDir, "profiles.json"),
		providersPath: filepath.Join(dataDir, "providers_config.json"),
		profiles:      make([]ProviderProfile, 0),
	}
	c.loadProfiles()
	return c
}

func (c *LLMClient) loadProfiles() {
	c.mu.Lock()
	defer c.mu.Unlock()

	data, err := os.ReadFile(c.providersPath)
	if err == nil {
		var list []ProviderProfile
		if json.Unmarshal(data, &list) == nil {
			c.profiles = list
			for _, p := range list {
				if p.Enabled && len(p.SelectedModels) > 0 {
					c.providerID = p.ID
					c.apiKey = p.APIKey
					c.baseURL = p.BaseURL
					c.model = p.SelectedModels[0]
					return
				}
			}
			return
		}
	}

	// No default fallback profiles; all profiles are custom user-managed providers
	c.profiles = make([]ProviderProfile, 0)
}

func (c *LLMClient) GetProviderProfiles() []ProviderProfile {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.profiles
}

func (c *LLMClient) SaveProviderProfiles(profiles []ProviderProfile) error {
	c.mu.Lock()
	c.profiles = profiles

	for _, p := range profiles {
		if p.Enabled && len(p.SelectedModels) > 0 {
			c.providerID = p.ID
			c.apiKey = p.APIKey
			c.baseURL = p.BaseURL
			c.model = p.SelectedModels[0]
			break
		}
	}
	c.mu.Unlock()

	_ = os.MkdirAll(filepath.Dir(c.providersPath), 0755)
	data, err := json.MarshalIndent(profiles, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.providersPath, data, 0644)
}

func (c *LLMClient) DeleteProviderProfile(id string) error {
	c.mu.Lock()
	filtered := make([]ProviderProfile, 0, len(c.profiles))
	for _, p := range c.profiles {
		if p.ID != id {
			filtered = append(filtered, p)
		}
	}
	c.profiles = filtered

	// If the active provider was deleted, reset active client to the first enabled profile
	if c.providerID == id {
		c.providerID = ""
		c.apiKey = ""
		c.baseURL = ""
		c.model = ""
		for _, p := range filtered {
			if p.Enabled && len(p.SelectedModels) > 0 {
				c.providerID = p.ID
				c.apiKey = p.APIKey
				c.baseURL = p.BaseURL
				c.model = p.SelectedModels[0]
				break
			}
		}
	}
	c.mu.Unlock()

	_ = os.MkdirAll(filepath.Dir(c.providersPath), 0755)
	data, err := json.MarshalIndent(filtered, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.providersPath, data, 0644)
}

// normalizeBaseURL canonicalizes a base URL for identity matching.
func normalizeBaseURL(u string) string {
	u = strings.ToLower(strings.TrimSpace(u))
	u = strings.TrimRight(u, "/")
	if strings.HasSuffix(u, "/v1") {
		u = strings.TrimSuffix(u, "/v1")
	}
	return u
}

// SyncProviderProfiles merges externally-managed provider configs (the
// frontend localStorage list) into the persisted profile list without
// clobbering profiles configured through the settings UI. Incoming profiles
// are matched to existing ones by ID first, then by base URL; matched
// profiles keep their existing ID. Empty incoming fields never overwrite
// populated stored ones (e.g. a key set via settings is not wiped by a
// keyless frontend entry). Returns the merged list.
func (c *LLMClient) SyncProviderProfiles(incoming []ProviderProfile) ([]ProviderProfile, error) {
	c.mu.Lock()

	merged := make([]ProviderProfile, len(c.profiles))
	copy(merged, c.profiles)

	byID := make(map[string]int, len(merged))
	byURL := make(map[string]int, len(merged))
	for i, p := range merged {
		if p.ID != "" {
			byID[p.ID] = i
		}
		if u := normalizeBaseURL(p.BaseURL); u != "" {
			byURL[u] = i
		}
	}

	activeAdjusted := false
	for _, in := range incoming {
		if in.ID == "" && normalizeBaseURL(in.BaseURL) == "" {
			continue
		}
		idx, ok := byID[in.ID]
		if !ok {
			idx, ok = byURL[normalizeBaseURL(in.BaseURL)]
		}
		if !ok {
			np := in
			merged = append(merged, np)
			idx = len(merged) - 1
			if np.ID != "" {
				byID[np.ID] = idx
			}
			if u := normalizeBaseURL(np.BaseURL); u != "" {
				byURL[u] = idx
			}
			continue
		}

		p := &merged[idx]
		if in.Name != "" {
			p.Name = in.Name
		}
		if in.APIKey != "" {
			p.APIKey = in.APIKey
		}
		if in.BaseURL != "" {
			p.BaseURL = in.BaseURL
		}
		p.Enabled = in.Enabled || p.Enabled
		if len(in.AvailableModels) > 0 {
			p.AvailableModels = in.AvailableModels
		}
		if in.SelectedModels != nil {
			p.SelectedModels = in.SelectedModels
		}
		if c.providerID == p.ID {
			// Keep the active client view consistent with the merged profile.
			c.apiKey = p.APIKey
			c.baseURL = p.BaseURL
			activeAdjusted = true
		}
	}

	data, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		c.mu.Unlock()
		return nil, err
	}
	_ = os.MkdirAll(filepath.Dir(c.providersPath), 0755)
	err = os.WriteFile(c.providersPath, data, 0644)
	// Adopt the merged list in memory too — the active client view was already
	// refreshed above for the profile currently in use.
	c.profiles = merged
	// Heal a keyless active profile (e.g. the pre-config default) by adopting
	// the first enabled profile that has a key and models — same policy as
	// loadProfiles — so a stale default never sends keyless requests.
	if c.apiKey == "" && !isLocalBaseURL(c.baseURL) {
		for _, p := range merged {
			if p.Enabled && p.APIKey != "" && len(p.SelectedModels) > 0 {
				c.providerID = p.ID
				c.apiKey = p.APIKey
				c.baseURL = p.BaseURL
				c.model = p.SelectedModels[0]
				activeAdjusted = true
				break
			}
		}
	}
	out := make([]ProviderProfile, len(merged))
	copy(out, merged)
	c.mu.Unlock()

	if activeAdjusted {
		c.mu.Lock()
		_ = c.saveActiveProfileLocked()
		c.mu.Unlock()
	}
	return out, err
}

// normalizeReasoningEffort maps effort strings to provider-compatible values ("low", "medium", "high", "off").
func normalizeReasoningEffort(effort string) string {
	switch strings.ToLower(strings.TrimSpace(effort)) {
	case "", "off", "none", "false", "0":
		return "off"
	case "minimal", "low":
		return "low"
	case "medium":
		return "medium"
	case "high", "xhigh", "max":
		return "high"
	default:
		return effort
	}
}

// SetActiveReasoningEffort sets the reasoning-effort hint sent with LLM
// requests ("low"/"medium"/"high"/…). Empty or "off" sends nothing — many
// gateways never stream reasoning deltas unless the request asks for them.
func (c *LLMClient) SetActiveReasoningEffort(effort string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.reasoningEffort = effort
}

func (c *LLMClient) SetActiveModel(providerID string, model string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i := range c.profiles {
		p := &c.profiles[i]
		if p.ID == providerID {
			c.providerID = p.ID
			c.apiKey = p.APIKey
			c.baseURL = p.BaseURL
			c.model = model
			// Move the selected model to the front of SelectedModels so the
			// active choice persists with the profile.
			var next []string
			next = append(next, model)
			for _, m := range p.SelectedModels {
				if m != model {
					next = append(next, m)
				}
			}
			p.SelectedModels = next
			break
		}
	}

	// Persist so the active model survives restarts.
	_ = os.MkdirAll(filepath.Dir(c.providersPath), 0755)
	if data, err := json.MarshalIndent(c.profiles, "", "  "); err == nil {
		_ = os.WriteFile(c.providersPath, data, 0644)
	}
	// Persist the active profile file too.
	_ = c.saveActiveProfileLocked()
}

// saveActiveProfileLocked writes the active profile to disk. Caller must hold c.mu.
func (c *LLMClient) saveActiveProfileLocked() error {
	p := Profile{
		ProviderID: c.providerID,
		APIKey:     c.apiKey,
		BaseURL:    c.baseURL,
		Model:      c.model,
	}
	_ = os.MkdirAll(filepath.Dir(c.profilePath), 0755)
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.profilePath, data, 0644)
}

// SetActiveModelByID sets the active provider/model from an opencode-style
// "provider/model" id (e.g. "anthropic/claude-sonnet-4-5").
func (c *LLMClient) SetActiveModelByID(fullID string) {
	providerID, model := splitModelID(fullID)
	if providerID == "" || model == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	// Match the longest provider prefix in known profiles.
	var best *ProviderProfile
	for i := range c.profiles {
		p := &c.profiles[i]
		if fullID == p.ID+"/"+model || providerID == p.ID {
			best = p
			break
		}
	}
	if best == nil {
		return
	}
	c.providerID = best.ID
	c.apiKey = best.APIKey
	c.baseURL = best.BaseURL
	c.model = model
}

// splitModelID splits "provider/model" into its parts, handling model ids that
// themselves contain slashes by matching against known provider prefixes.
func splitModelID(fullID string) (string, string) {
	idx := strings.Index(fullID, "/")
	if idx <= 0 || idx == len(fullID)-1 {
		return "", fullID
	}
	return fullID[:idx], fullID[idx+1:]
}

// FindProviderForModel returns the provider ID that configured or owns this model,
// or empty string if not found in profiles.
func (c *LLMClient) FindProviderForModel(model string) string {
	if model == "" {
		return ""
	}
	c.mu.RLock()
	defer c.mu.RUnlock()

	// 1. Check if model has a prefix matching a profile ID or known provider
	if idx := strings.Index(model, "/"); idx > 0 {
		prefix := strings.ToLower(model[:idx])
		for _, p := range c.profiles {
			if strings.ToLower(p.ID) == prefix || strings.ToLower(p.Name) == prefix {
				return p.ID
			}
		}
		if _, ok := SupportedProviders[prefix]; ok {
			return prefix
		}
	}

	// 2. Check each profile's selected or available models
	for _, p := range c.profiles {
		for _, m := range p.SelectedModels {
			if strings.EqualFold(m, model) || strings.EqualFold(p.ID+"/"+m, model) {
				return p.ID
			}
		}
		for _, m := range p.AvailableModels {
			if strings.EqualFold(m, model) || strings.EqualFold(p.ID+"/"+m, model) {
				return p.ID
			}
		}
	}

	// 3. Check active model
	if strings.EqualFold(c.model, model) && c.providerID != "" {
		return c.providerID
	}

	return ""
}

func (c *LLMClient) FetchModels(ctx context.Context, apiKey string, baseURL string) ([]string, error) {
	baseURL = strings.TrimRight(baseURL, "/")

	var reqURL string
	headers := make(map[string]string)

	isGemini := strings.Contains(baseURL, "googleapis.com") || strings.Contains(baseURL, "google") || strings.HasPrefix(apiKey, "AIza")
	isAnthropic := strings.Contains(baseURL, "anthropic.com") || strings.HasPrefix(apiKey, "sk-ant")
	isOllama := strings.Contains(baseURL, "11434") || strings.Contains(baseURL, "ollama")

	if isGemini {
		if apiKey == "" {
			return nil, fmt.Errorf("gemini api key required")
		}
		reqURL = "https://generativelanguage.googleapis.com/v1beta/models?key=" + apiKey
	} else if isAnthropic {
		reqURL = "https://api.anthropic.com/v1/models"
		headers["x-api-key"] = apiKey
		headers["anthropic-version"] = "2023-06-01"
	} else if isOllama {
		if baseURL == "" {
			baseURL = "http://localhost:11434"
		}
		reqURL = baseURL + "/api/tags"
	} else {
		if baseURL == "" {
			baseURL = "https://api.openai.com/v1"
		}
		if !strings.HasSuffix(baseURL, "/v1") && !strings.Contains(baseURL, "/v1/") && !strings.Contains(baseURL, "openrouter.ai") {
			reqURL = baseURL + "/models"
		} else {
			reqURL = baseURL + "/models"
		}
		if apiKey != "" {
			headers["Authorization"] = "Bearer " + apiKey
		}
	}

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{Timeout: 12 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		// Fallback for Anthropic if API returns network/auth error
		if isAnthropic {
			return []string{
				"claude-3-7-sonnet-20250219",
				"claude-3-5-sonnet-20241022",
				"claude-3-5-haiku-20241022",
				"claude-3-opus-20240229",
			}, nil
		}
		return nil, fmt.Errorf("fetch models failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		// Anthropic graceful fallback
		if isAnthropic {
			return []string{
				"claude-3-7-sonnet-20250219",
				"claude-3-5-sonnet-20241022",
				"claude-3-5-haiku-20241022",
				"claude-3-opus-20240229",
			}, nil
		}
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}

	var models []string

	// 1. Try Gemini structure
	var geminiResp struct {
		Models []struct {
			Name        string `json:"name"`
			DisplayName string `json:"displayName"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &geminiResp); err == nil && len(geminiResp.Models) > 0 {
		for _, m := range geminiResp.Models {
			cleanName := strings.TrimPrefix(m.Name, "models/")
			models = append(models, cleanName)
		}
		return models, nil
	}

	// 2. Try Ollama structure
	var ollamaResp struct {
		Models []struct {
			Name  string `json:"name"`
			Model string `json:"model"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &ollamaResp); err == nil && len(ollamaResp.Models) > 0 {
		for _, m := range ollamaResp.Models {
			name := m.Name
			if name == "" {
				name = m.Model
			}
			if name != "" {
				models = append(models, name)
			}
		}
		return models, nil
	}

	// 3. Try OpenAI / Anthropic / OpenRouter standard structure: { "data": [ { "id": "..." } ] }
	var openAIResp struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &openAIResp); err == nil && len(openAIResp.Data) > 0 {
		for _, m := range openAIResp.Data {
			if m.ID != "" {
				models = append(models, m.ID)
			}
		}
		return models, nil
	}

	return []string{}, nil
}

func (c *LLMClient) SaveProfile(providerID, apiKey, baseURL, model string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.providerID = providerID
	c.apiKey = apiKey
	c.baseURL = baseURL
	c.model = model

	return c.saveActiveProfileLocked()
}

func (c *LLMClient) GetConfig() Profile {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return Profile{
		ProviderID: c.providerID,
		APIKey:     c.apiKey,
		BaseURL:    c.baseURL,
		Model:      c.model,
	}
}

func (c *LLMClient) ListProviders() []ProviderConfig {
	list := make([]ProviderConfig, 0, len(SupportedProviders))
	for _, p := range SupportedProviders {
		list = append(list, p)
	}
	return list
}

func (c *LLMClient) Chat(ctx context.Context, messages []LLMMessage, tools []ToolDefinition) (*LLMResponse, error) {
	c.mu.RLock()
	providerID := c.providerID
	model := c.model
	c.mu.RUnlock()
	return c.ChatWithProvider(ctx, providerID, model, messages, tools)
}

func (c *LLMClient) ChatWithStream(ctx context.Context, messages []LLMMessage, tools []ToolDefinition, onChunk func(deltaContent string, deltaReasoning string)) (*LLMResponse, error) {
	c.mu.RLock()
	providerID := c.providerID
	model := c.model
	c.mu.RUnlock()
	return c.ChatWithProviderStream(ctx, providerID, model, messages, tools, onChunk, nil)
}

// ChatWithStreamDetailed is ChatWithStream plus a streamed tool-call fragment
// callback. Each fragment carries the accumulating index so the caller can
// track partial tool-call arguments as they arrive
// toolcall_delta events).
type ToolCallDelta struct {
	Index       int
	ID          string
	Name        string
	ArgFragment string
}

// ChatWithModelStreamDetailed performs a streaming chat call using the specified target model,
// automatically determining the matching provider profile or falling back to the client's defaults.
func (c *LLMClient) ChatWithModelStreamDetailed(
	ctx context.Context,
	targetModel string,
	messages []LLMMessage,
	tools []ToolDefinition,
	onChunk func(deltaContent string, deltaReasoning string),
	onToolCallDelta func(delta ToolCallDelta),
) (*LLMResponse, error) {
	c.mu.RLock()
	providerID := c.providerID
	model := c.model
	c.mu.RUnlock()

	if targetModel != "" {
		model = targetModel
		if pID := c.FindProviderForModel(targetModel); pID != "" {
			providerID = pID
			if strings.HasPrefix(strings.ToLower(model), strings.ToLower(pID)+"/") {
				model = model[len(pID)+1:]
			}
		} else if pID, bareModel := splitModelID(targetModel); pID != "" {
			providerID = pID
			model = bareModel
		}
	}

	return c.ChatWithProviderStream(ctx, providerID, model, messages, tools, onChunk, onToolCallDelta)
}

func (c *LLMClient) ChatWithStreamDetailed(ctx context.Context, messages []LLMMessage, tools []ToolDefinition, onChunk func(deltaContent string, deltaReasoning string), onToolCallDelta func(delta ToolCallDelta)) (*LLMResponse, error) {
	return c.ChatWithModelStreamDetailed(ctx, "", messages, tools, onChunk, onToolCallDelta)
}

func (c *LLMClient) ChatWithProvider(ctx context.Context, targetProviderID string, targetModel string, messages []LLMMessage, tools []ToolDefinition) (*LLMResponse, error) {
	return c.ChatWithProviderStream(ctx, targetProviderID, targetModel, messages, tools, nil, nil)
}

func (c *LLMClient) ChatWithProviderStream(
	ctx context.Context,
	targetProviderID string,
	targetModel string,
	messages []LLMMessage,
	tools []ToolDefinition,
	onChunk func(deltaContent string, deltaReasoning string),
	onToolCallDelta func(delta ToolCallDelta),
) (*LLMResponse, error) {
	c.mu.RLock()
	var baseURL, apiKey string
	model := targetModel
	reasoningEffort := c.reasoningEffort

	for _, p := range c.profiles {
		if p.ID == targetProviderID {
			baseURL = p.BaseURL
			apiKey = p.APIKey
			if model == "" && len(p.SelectedModels) > 0 {
				model = p.SelectedModels[0]
			}
			break
		}
	}
	if baseURL == "" {
		baseURL = c.baseURL
		apiKey = c.apiKey
		if model == "" {
			model = c.model
		}
	}
	c.mu.RUnlock()

	if apiKey == "" && SupportedProviders[targetProviderID].EnvKey != "" {
		apiKey = os.Getenv(SupportedProviders[targetProviderID].EnvKey)
	}

	// Fail fast with an actionable message instead of a remote 401: a keyless
	// request only makes sense for local servers (Ollama, LM Studio, …).
	if apiKey == "" && !isLocalBaseURL(baseURL) {
		envHint := SupportedProviders[targetProviderID].EnvKey
		return nil, fmt.Errorf(
			"no API key configured for provider %q (model %q). Add the key in Settings → Providers, or set the %s environment variable",
			targetProviderID, model, envHint)
	}

	isStreaming := onChunk != nil
	reqMessages, reqTools := PreparePayloadWithCache(targetProviderID, model, messages, tools)
	reqBody := map[string]interface{}{
		"model":    model,
		"messages": reqMessages,
		"stream":   isStreaming,
	}

	if len(tools) > 0 {
		reqBody["tools"] = reqTools
	}

	// Normalize reasoning effort ("off", "low", "medium", "high") so gateways
	// like OpenAI and OpenRouter never receive unsupported values (e.g. "max").
	normEffort := normalizeReasoningEffort(reasoningEffort)
	askReasoning := normEffort != "" && normEffort != "off"
	if askReasoning {
		reqBody["reasoning_effort"] = normEffort
		if targetProviderID == "openrouter" || strings.Contains(baseURL, "openrouter") {
			reqBody["reasoning"] = map[string]interface{}{
				"effort": normEffort,
			}
		}
	}

	reqCtx := ctx
	var cancelIdle context.CancelFunc
	var idleTimer *time.Timer
	if isStreaming {
		var idleCtx context.Context
		idleCtx, cancelIdle = context.WithCancel(ctx)
		defer cancelIdle()
		idleTimer = time.AfterFunc(DefaultStreamIdleTimeout, func() {
			cancelIdle()
		})
		defer idleTimer.Stop()
		reqCtx = idleCtx
	}

	url := strings.TrimRight(baseURL, "/") + "/chat/completions"

	doRequest := func(body map[string]interface{}) (*http.Response, error) {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request: %w", err)
		}
		req, err := http.NewRequestWithContext(reqCtx, "POST", url, bytes.NewBuffer(bodyBytes))
		if err != nil {
			return nil, fmt.Errorf("create http request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		if apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+apiKey)
		}
		if sessionID := SessionIDFrom(ctx); sessionID != "" {
			if targetProviderID == "xai" || strings.Contains(strings.ToLower(model), "grok") {
				req.Header.Set("x-grok-conv-id", sessionID)
			}
			if targetProviderID == "openrouter" {
				req.Header.Set("x-session-affinity", sessionID)
			}
		}
		return c.httpClient.Do(req)
	}

	resp, err := doRequest(reqBody)
	if err != nil {
		if reqCtx != nil && reqCtx.Err() != nil && ctx.Err() == nil {
			return nil, &ErrStreamIdleTimeout{Timeout: DefaultStreamIdleTimeout}
		}
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	if resp.StatusCode != http.StatusOK && askReasoning &&
		(resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusUnprocessableEntity) {
		// Provider rejected reasoning_effort — drop it and retry once so the
		// chat still works against strict gateways.
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		log.Printf("[llm] status %d with reasoning_effort=%q — retrying without it (provider=%q model=%q)",
			resp.StatusCode, normEffort, targetProviderID, model)
		fallbackBody := make(map[string]interface{}, len(reqBody))
		for k, v := range reqBody {
			fallbackBody[k] = v
		}
		delete(fallbackBody, "reasoning_effort")
		delete(fallbackBody, "reasoning")
		askReasoning = false
		if resp2, err2 := doRequest(fallbackBody); err2 == nil {
			if resp2.StatusCode == http.StatusOK {
				resp = resp2
			} else {
				io.Copy(io.Discard, resp2.Body)
				resp2.Body.Close()
			}
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("api error (status %d): %s", resp.StatusCode, string(respBody))
	}

	if isStreaming {
		contentType := strings.ToLower(resp.Header.Get("Content-Type"))
		br := bufio.NewReader(resp.Body)

		// Check whether this is an SSE stream (real-time token-by-token) or a plain JSON body
		// (from providers/proxies that ignore stream:true and reply with a single JSON document).
		isSSE := strings.Contains(contentType, "text/event-stream")
		if !isSSE {
			peekBytes, _ := br.Peek(512)
			trimmed := bytes.TrimSpace(peekBytes)
			// If non-empty and does not start with '{', it's SSE formatted lines (e.g. data:, :, event:)
			if len(trimmed) > 0 && trimmed[0] != '{' {
				isSSE = true
			}
		}

		if isSSE {
			// Stream tokens directly in real time from the HTTP connection!
			sseResp, sseErr := parseSSEStream(reqCtx, br, onChunk, onToolCallDelta, idleTimer)
			if sseErr != nil {
				if reqCtx.Err() != nil && ctx.Err() == nil {
					return nil, &ErrStreamIdleTimeout{Timeout: DefaultStreamIdleTimeout}
				}
				return nil, fmt.Errorf("parse stream response: %w", sseErr)
			}
			if sseResp.Content == "" && sseResp.Reasoning == "" && len(sseResp.ToolCalls) == 0 {
				log.Printf("[llm] WARNING: SSE stream produced no content; provider=%q model=%q", targetProviderID, model)
				return nil, fmt.Errorf("provider returned empty response (no content, reasoning, or tool calls)")
			}
			return sseResp, nil
		}

		// Fallback for providers that return a plain JSON chat completion body despite stream:true
		body, readErr := io.ReadAll(br)
		if readErr != nil {
			return nil, fmt.Errorf("read response body: %w", readErr)
		}
		if len(bytes.TrimSpace(body)) == 0 {
			log.Printf("[llm] WARNING: empty response body (status 200) for streaming request; provider=%q model=%q", targetProviderID, model)
			return nil, fmt.Errorf("empty response body (status 200) — provider returned nothing")
		}
		if json.Valid(body) {
			jsonResp, err := parseOpenAIJSON(body)
			if err == nil {
				if onChunk != nil && (jsonResp.Content != "" || jsonResp.Reasoning != "") {
					onChunk(jsonResp.Content, jsonResp.Reasoning)
				}
				return jsonResp, nil
			}
		}
		return parseSSEStream(ctx, bytes.NewReader(body), onChunk, onToolCallDelta)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if len(bytes.TrimSpace(respBody)) == 0 {
		// Status 200 with an empty body: some providers/proxies only answer
		// streaming requests and return an empty body to stream:false calls.
		// Surface this as an error instead of silently returning empty content.
		log.Printf("[llm] WARNING: empty response body (status 200) for non-streaming request; provider=%q model=%q", targetProviderID, model)
		return nil, fmt.Errorf("empty response body (status 200) — provider may only support streaming requests")
	}

	var openAIResp openAICompletionResp

	if err := json.Unmarshal(respBody, &openAIResp); err != nil {
		log.Printf("[llm] unmarshal failed: %v; body=%s", err, truncateBody(respBody))
		// Some OpenAI-compatible providers/proxies reply in SSE format even
		// when asked for a non-streaming response. Fall back to parsing the
		// body as an SSE stream so calls like AI commit generation don't fail
		// with "unmarshal response: unexpected json input".
		if sseResp, sseErr := parseSSEStream(ctx, bytes.NewReader(respBody), nil, nil); sseErr == nil {
			return sseResp, nil
		}
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return buildLLMResponse(openAIResp, respBody)
}

// openAICompletionResp mirrors the OpenAI chat-completions response shape
// (also used by compatible providers/proxies).
type openAICompletionResp struct {
	Choices []struct {
		Message struct {
			Content          string     `json:"content"`
			ReasoningContent string     `json:"reasoning_content"`
			Reasoning        string     `json:"reasoning"`
			Thought          string     `json:"thought"`
			Thinking         string     `json:"thinking"`
			ToolCalls        []ToolCall `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
	Usage TokenStats `json:"usage"`
}

// parseOpenAIJSON parses a plain JSON chat-completions body (non-SSE).
func parseOpenAIJSON(body []byte) (*LLMResponse, error) {
	var openAIResp openAICompletionResp
	if err := json.Unmarshal(body, &openAIResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	return buildLLMResponse(openAIResp, body)
}

// buildLLMResponse converts a parsed OpenAI-style chat completion into an
// LLMResponse, logging the raw body when choices are missing so silent empty
// responses are diagnosable.
func buildLLMResponse(openAIResp openAICompletionResp, body []byte) (*LLMResponse, error) {
	if len(openAIResp.Choices) == 0 {
		log.Printf("[llm] WARNING: response has no choices; body=%s", truncateBody(body))
		return &LLMResponse{Content: ""}, nil
	}

	choice := openAIResp.Choices[0]
	cached := openAIResp.Usage.CachedTokens
	if openAIResp.Usage.PromptCacheHitTokens > 0 {
		cached = openAIResp.Usage.PromptCacheHitTokens
	}

	reasoning := choice.Message.ReasoningContent
	if reasoning == "" {
		reasoning = choice.Message.Reasoning
	}
	if reasoning == "" {
		reasoning = choice.Message.Thought
	}
	if reasoning == "" {
		reasoning = choice.Message.Thinking
	}

	return &LLMResponse{
		Content:   choice.Message.Content,
		Reasoning: reasoning,
		ToolCalls: choice.Message.ToolCalls,
		TokenUsage: TokenStats{
			PromptTokens:          openAIResp.Usage.PromptTokens,
			CompletionTokens:      openAIResp.Usage.CompletionTokens,
			CachedTokens:          cached,
			PromptCacheHitTokens:  openAIResp.Usage.PromptCacheHitTokens,
			PromptCacheMissTokens: openAIResp.Usage.PromptCacheMissTokens,
			TotalTokens:           openAIResp.Usage.TotalTokens,
		},
	}, nil
}

// parseSSEStream reads an OpenAI-compatible SSE (server-sent events) response
// body and accumulates the deltas into an LLMResponse. onChunk/onToolCallDelta
// are optional callbacks for streaming callers.
func parseSSEStream(ctx context.Context, r io.Reader, onChunk func(deltaContent string, deltaReasoning string), onToolCallDelta func(delta ToolCallDelta), idleTimer ...*time.Timer) (*LLMResponse, error) {
	var fullContent strings.Builder
	var fullReasoning strings.Builder
	toolCallMap := make(map[int]*ToolCall)
	var toolCallOrder []int
	var usage TokenStats

	var timer *time.Timer
	if len(idleTimer) > 0 && idleTimer[0] != nil {
		timer = idleTimer[0]
	}

	scanner := bufio.NewScanner(r)
	scanBuf := make([]byte, 64*1024)
	scanner.Buffer(scanBuf, 10*1024*1024)
	for scanner.Scan() {
		if timer != nil {
			timer.Reset(DefaultStreamIdleTimeout)
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}

		var errCheck struct {
			Error struct {
				Message string `json:"message"`
				Type    string `json:"type"`
			} `json:"error"`
		}
		if err := json.Unmarshal([]byte(data), &errCheck); err == nil && errCheck.Error.Message != "" {
			return nil, fmt.Errorf("provider error: %s", errCheck.Error.Message)
		}

		var chunk struct {
			Choices []struct {
				Text      string `json:"text"`
				Reasoning string `json:"reasoning"`
				Thought   string `json:"thought"`
				Delta     struct {
					Content          string `json:"content"`
					ReasoningContent string `json:"reasoning_content"`
					Reasoning        string `json:"reasoning"`
					Thought          string `json:"thought"`
					Thinking         string `json:"thinking"`
					ToolCalls        []struct {
						Index    int    `json:"index"`
						ID       string `json:"id"`
						Type     string `json:"type"`
						Function struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						} `json:"function"`
					} `json:"tool_calls"`
				} `json:"delta"`
			} `json:"choices"`
			Usage *struct {
				PromptTokens          int `json:"prompt_tokens"`
				CompletionTokens      int `json:"completion_tokens"`
				CachedTokens          int `json:"cached_tokens"`            // Anthropic-style: cached input subset
				PromptCacheHitTokens  int `json:"prompt_cache_hit_tokens"`  // DeepSeek
				PromptCacheMissTokens int `json:"prompt_cache_miss_tokens"` // DeepSeek
				TotalTokens           int `json:"total_tokens"`
			} `json:"usage"`
		}

		if err := json.Unmarshal([]byte(data), &chunk); err == nil {
			if chunk.Usage != nil {
				if chunk.Usage.TotalTokens > 0 {
					usage.PromptTokens = chunk.Usage.PromptTokens
					usage.CompletionTokens = chunk.Usage.CompletionTokens
					usage.CachedTokens = chunk.Usage.CachedTokens
					usage.PromptCacheHitTokens = chunk.Usage.PromptCacheHitTokens
					usage.PromptCacheMissTokens = chunk.Usage.PromptCacheMissTokens
					usage.TotalTokens = chunk.Usage.TotalTokens
				}
			}
			if len(chunk.Choices) > 0 {
				choice := chunk.Choices[0]
				delta := choice.Delta
				contentDelta := delta.Content
				if contentDelta == "" && choice.Text != "" {
					contentDelta = choice.Text
				}
				reasoningDelta := delta.ReasoningContent
				if reasoningDelta == "" {
					reasoningDelta = delta.Reasoning
				}
				if reasoningDelta == "" {
					reasoningDelta = delta.Thought
				}
				if reasoningDelta == "" {
					reasoningDelta = delta.Thinking
				}
				if reasoningDelta == "" && choice.Reasoning != "" {
					reasoningDelta = choice.Reasoning
				}
				if reasoningDelta == "" && choice.Thought != "" {
					reasoningDelta = choice.Thought
				}

				if contentDelta != "" || reasoningDelta != "" {
					fullContent.WriteString(contentDelta)
					fullReasoning.WriteString(reasoningDelta)
					if onChunk != nil {
						onChunk(contentDelta, reasoningDelta)
					}
				}

				for _, tcChunk := range delta.ToolCalls {
					idx := tcChunk.Index
					existing, ok := toolCallMap[idx]
					if !ok {
						toolCallMap[idx] = &ToolCall{
							ID:   tcChunk.ID,
							Type: tcChunk.Type,
							Function: ToolFunction{
								Name:      tcChunk.Function.Name,
								Arguments: tcChunk.Function.Arguments,
							},
						}
						toolCallOrder = append(toolCallOrder, idx)
					} else {
						if tcChunk.ID != "" {
							existing.ID = tcChunk.ID
						}
						if tcChunk.Type != "" {
							existing.Type = tcChunk.Type
						}
						if tcChunk.Function.Name != "" {
							existing.Function.Name += tcChunk.Function.Name
						}
						existing.Function.Arguments += tcChunk.Function.Arguments
					}
					if onToolCallDelta != nil {
						onToolCallDelta(ToolCallDelta{
							Index:       idx,
							ID:          tcChunk.ID,
							Name:        tcChunk.Function.Name,
							ArgFragment: tcChunk.Function.Arguments,
						})
					}
				}
			}
		}
	}
	if err := scanner.Err(); err != nil && err != io.EOF {
		if fullContent.Len() == 0 && fullReasoning.Len() == 0 && len(toolCallOrder) == 0 {
			return nil, fmt.Errorf("read stream line: %w", err)
		}
		log.Printf("[llm] warning: scanner stopped with error: %v", err)
	}

	var accumulatedToolCalls []ToolCall
	for _, idx := range toolCallOrder {
		if tc, ok := toolCallMap[idx]; ok {
			accumulatedToolCalls = append(accumulatedToolCalls, *tc)
		}
	}

	return &LLMResponse{
		Content:    fullContent.String(),
		Reasoning:  fullReasoning.String(),
		ToolCalls:  accumulatedToolCalls,
		TokenUsage: usage,
	}, nil
}

// truncateBody caps a raw response body for logging so a huge or binary body
// doesn't flood the log.
func truncateBody(b []byte) string {
	const max = 2000
	if len(b) <= max {
		return string(b)
	}
	return string(b[:max]) + fmt.Sprintf("... (%d more bytes)", len(b)-max)
}

// PreparePayloadWithCache inspects the target provider and model, and if prompt caching
// with inline cache_control breakpoints is supported (Anthropic, OpenRouter Claude/Qwen, etc.),
// applies the 4-breakpoint budget:
//   - Breakpoint 1: Last tool definition (Jev-pruned tools)
//   - Breakpoint 2: Base system prompt (agent instructions)
//   - Breakpoint 3: Tail of system prompt blocks (project rules/workspace context)
//   - Breakpoint 4: Conversation tail (last user message or tool observation)
//
// Returns JSON-serializable representations of messages and tools ready for reqBody.
func PreparePayloadWithCache(providerID, model string, messages []LLMMessage, tools []ToolDefinition) (any, any) {
	if !shouldApplyCacheBreakpoints(providerID, model) {
		return messages, tools
	}

	remainingBudget := 4

	// 1. Tool breakpoint: mark the last tool definition
	var finalTools any = tools
	if len(tools) > 0 && remainingBudget > 0 {
		toolMaps := make([]map[string]any, len(tools))
		for i, t := range tools {
			tm := map[string]any{
				"type": t.Type,
				"function": map[string]any{
					"name":        t.Function.Name,
					"description": t.Function.Description,
					"parameters":  t.Function.Parameters,
				},
			}
			toolMaps[i] = tm
		}
		// Apply cache_control to the last tool definition
		toolMaps[len(toolMaps)-1]["cache_control"] = map[string]string{"type": "ephemeral"}
		remainingBudget--
		finalTools = toolMaps
	}

	// 2. System and message breakpoints
	msgMaps := make([]map[string]any, len(messages))
	for i, m := range messages {
		mm := map[string]any{
			"role": string(m.Role),
		}
		if m.Name != "" {
			mm["name"] = m.Name
		}
		if m.ToolCallID != "" {
			mm["tool_call_id"] = m.ToolCallID
		}
		if len(m.ToolCalls) > 0 {
			mm["tool_calls"] = m.ToolCalls
		}
		mm["content"] = m.Content
		msgMaps[i] = mm
	}

	// Breakpoint 2 & 3: Check system message(s)
	// If the system message is a single block or multiple, add cache_control to system content blocks
	for _, m := range msgMaps {
		if m["role"] == "system" && remainingBudget > 0 {
			contentStr, isStr := m["content"].(string)
			if isStr && contentStr != "" {
				// Convert to Anthropic / OpenRouter structured content parts
				// e.g. [{"type": "text", "text": "...", "cache_control": {"type": "ephemeral"}}]
				m["content"] = []map[string]any{
					{
						"type":          "text",
						"text":          contentStr,
						"cache_control": map[string]string{"type": "ephemeral"},
					},
				}
				remainingBudget--
			}
			break
		}
	}

	// Breakpoint 4: Conversation tail
	// Mark the last user message or tool observation so subsequent turns in the loop hit prefix cache
	if remainingBudget > 0 && len(msgMaps) > 0 {
		for i := len(msgMaps) - 1; i >= 0; i-- {
			role := msgMaps[i]["role"]
			if role == "user" || role == "tool" {
				contentVal := msgMaps[i]["content"]
				if str, ok := contentVal.(string); ok && str != "" {
					msgMaps[i]["content"] = []map[string]any{
						{
							"type":          "text",
							"text":          str,
							"cache_control": map[string]string{"type": "ephemeral"},
						},
					}
					remainingBudget--
				} else if parts, ok := contentVal.([]map[string]any); ok && len(parts) > 0 {
					// Mark the last part of this message
					lastIdx := len(parts) - 1
					parts[lastIdx]["cache_control"] = map[string]string{"type": "ephemeral"}
					remainingBudget--
				}
				break
			}
		}
	}

	return msgMaps, finalTools
}

func shouldApplyCacheBreakpoints(providerID, model string) bool {
	p := strings.ToLower(providerID)
	m := strings.ToLower(model)

	if p == "anthropic" {
		return true
	}
	if p == "openrouter" {
		// OpenRouter passes prompt caching markers directly to Anthropic and Qwen models
		return strings.Contains(m, "claude") || strings.Contains(m, "anthropic") || strings.Contains(m, "qwen")
	}
	// Direct DeepSeek / Mistral / Bedrock / etc.
	if strings.Contains(p, "anthropic") || strings.Contains(m, "claude") {
		return true
	}
	return false
}
