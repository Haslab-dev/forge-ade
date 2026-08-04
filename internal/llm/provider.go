package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
)

type LLMMessage struct {
	Role       Role       `json:"role"`
	Content    string     `json:"content"`
	Name       string     `json:"name,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
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
	mu            sync.RWMutex
	providerID    string
	apiKey        string
	baseURL       string
	model         string
	httpClient    *http.Client
	profilePath   string
	providersPath string
	profiles      []ProviderProfile
}

func NewLLMClient(dataDir string) *LLMClient {
	c := &LLMClient{
		providerID:    "openai",
		baseURL:       SupportedProviders["openai"].BaseURL,
		model:         SupportedProviders["openai"].DefaultModel,
		httpClient:    &http.Client{Timeout: 120 * time.Second},
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

func (c *LLMClient) FetchModels(ctx context.Context, apiKey string, baseURL string) ([]string, error) {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	baseURL = strings.TrimRight(baseURL, "/")

	var reqURL string
	if strings.Contains(baseURL, "11434") || strings.Contains(baseURL, "ollama") {
		reqURL = "http://localhost:11434/api/tags"
	} else {
		reqURL = baseURL + "/models"
	}

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch models failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}

	var models []string

	// Try Ollama structure
	var ollamaResp struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &ollamaResp); err == nil && len(ollamaResp.Models) > 0 {
		for _, m := range ollamaResp.Models {
			models = append(models, m.Name)
		}
		return models, nil
	}

	// Try OpenAI structure
	var openAIResp struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &openAIResp); err == nil && len(openAIResp.Data) > 0 {
		for _, m := range openAIResp.Data {
			models = append(models, m.ID)
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

func (c *LLMClient) ChatWithStreamDetailed(ctx context.Context, messages []LLMMessage, tools []ToolDefinition, onChunk func(deltaContent string, deltaReasoning string), onToolCallDelta func(delta ToolCallDelta)) (*LLMResponse, error) {
	c.mu.RLock()
	providerID := c.providerID
	model := c.model
	c.mu.RUnlock()
	return c.ChatWithProviderStream(ctx, providerID, model, messages, tools, onChunk, onToolCallDelta)
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

	isStreaming := onChunk != nil
	reqBody := map[string]interface{}{
		"model":    model,
		"messages": messages,
		"stream":   isStreaming,
	}

	if len(tools) > 0 {
		reqBody["tools"] = tools
	}

	jsonBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := strings.TrimRight(baseURL, "/") + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, fmt.Errorf("create http request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("api error (status %d): %s", resp.StatusCode, string(respBody))
	}

	if isStreaming {
		var fullContent strings.Builder
		var fullReasoning strings.Builder
		toolCallMap := make(map[int]*ToolCall)
		var toolCallOrder []int
		var usage TokenStats

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, ":") {
				continue
			}
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				break
			}

			var chunk struct {
				Choices []struct {
					Delta struct {
						Content          string `json:"content"`
						ReasoningContent string `json:"reasoning_content"`
						Reasoning        string `json:"reasoning"`
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
					CachedTokens          int `json:"cached_tokens"` // Anthropic-style: cached input subset
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
					delta := chunk.Choices[0].Delta
					contentDelta := delta.Content
					reasoningDelta := delta.ReasoningContent
					if reasoningDelta == "" {
						reasoningDelta = delta.Reasoning
					}

					if contentDelta != "" || reasoningDelta != "" {
						fullContent.WriteString(contentDelta)
						fullReasoning.WriteString(reasoningDelta)
						onChunk(contentDelta, reasoningDelta)
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

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	var openAIResp struct {
		Choices []struct {
			Message struct {
				Content          string     `json:"content"`
				ReasoningContent string     `json:"reasoning_content"`
				Reasoning        string     `json:"reasoning"`
				ToolCalls        []ToolCall `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
		Usage TokenStats `json:"usage"`
	}

	if err := json.Unmarshal(respBody, &openAIResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if len(openAIResp.Choices) == 0 {
		return &LLMResponse{Content: ""}, nil
	}

	choice := openAIResp.Choices[0]
	cached := openAIResp.Usage.CachedTokens
	if openAIResp.Usage.PromptCacheHitTokens > 0 {
		cached = openAIResp.Usage.PromptCacheHitTokens
	}

	return &LLMResponse{
		Content:   choice.Message.Content,
		Reasoning: choice.Message.Reasoning,
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
